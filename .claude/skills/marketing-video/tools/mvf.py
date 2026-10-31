#!/usr/bin/env python3
"""mvf — the marketing-video flow's tool. Every stage writes one artifact into the
campaign dir; every artifact has a check; every video is measured against the
platform it ships to. Run from anywhere inside a campaign (or pass --campaign).

  init <slug> --product NAME        new campaign from templates (default <repo>/marketing/campaigns/<slug>)
  status                            stage by stage: present, FAIL/WARN counts, what is next
  extensions [ls|init]              where extensions load from and what they add; init the local dir
  sources                           every data source: what it yields, its params
  gather <source> [k=v …]           pull items from a source into 00-material/index.json
  material [ls|show ID…] [--kind K] [-q TEXT]   read the pack as short cards, strongest first
  check [stage]                     validate brief|material|product|audience|trends|strategy|pretest|all
  new-script <sid>                  05-scripts/<sid>.json from the template
  lint <script> [--hook H2]         hook timing, first value, length, claims→facts, brand, on-screen load, platform copy
  trend-video URL… [--frames]       public metadata (+ hook frames) of reference posts into trends.json
  pretest-brief <script>… [--round r1]   pre-test brief with the material verbatim
  voice <script> [--engine E]       a take: every line (+ hook variants) as audio with word timing
  voice-design --name --instruct …  candidate voices for a designed narrator (an engine with `design`)
  voice-pick <cand> --as NAME       a candidate becomes the campaign's reference voice
  scaffold <script> [--hook H1|all] HyperFrames project per canvas × hook on the measured timing
  score <proj> --bed FILE | --none  your music bed, ducked and levelled against the voice
  export <proj> [--platforms …]     render, master −14 LUFS, encode per platform, QA, copy
  qa <video> --platform P --proj D  measure one file against a platform

Voice engines: `files` (built in — your own recordings, one per line) and any
extension (`mvf extensions`; `omnivoice` ships with the skill). Data sources:
`manual` (built in) and any extension. Extensions live in <campaign>/extensions/
or <repo>/.babysit/marketing-video/extensions/, both gitignored. Model and render
jobs run through $MVF_HEAVY (or config "heavy") when set. Nothing here uploads
or publishes.
"""
from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from mvflib import checks, extensions, material, timing  # noqa: E402
from mvflib.common import (PLATFORMS, STAGES, TOOLS, Report, config, dump, find_campaign,  # noqa: E402
                           heavy_prefix, load, local_dir, project_root, say_map, today)

TEMPLATES = TOOLS / "templates/campaign"
REFERENCE = {"brief": "00-brief-material.md", "material": "00-brief-material.md", "product": "01-product.md",
             "audience": "02-audience.md", "trends": "03-trends.md", "strategy": "04-strategy.md"}


# ---------- campaign helpers ----------

def campaign_dir(a) -> Path:
    return find_campaign(Path(a.campaign) if getattr(a, "campaign", None) else Path.cwd())


def maybe_campaign(a) -> Path | None:
    try:
        return campaign_dir(a)
    except SystemExit:
        return None


def recipes(cd: Path | None) -> dict:
    from mvflib.lint import RECIPES
    return {**RECIPES, **extensions.load(cd).recipes}


def refs(cd: Path) -> dict:
    def opt(rel, key):
        p = cd / rel
        return load(p).get(key, []) if p.is_file() else []
    return {"facts": {f["id"] for f in opt("01-product/facts.json", "facts") if "id" in f},
            "segments": {s["id"]: s for s in opt("02-audience/segments.json", "segments") if "id" in s},
            "trends": {t["id"]: t for t in opt("03-trends/trends.json", "trends") if "id" in t},
            "angles": {g["id"]: g for g in opt("04-strategy/angles.json", "angles") if "id" in g},
            "material": material.load_pack(cd)["items"], "recipes": recipes(cd)}


def script_path(cd: Path, s: str) -> Path:
    p = Path(s)
    for c in (p, cd / p, cd / "05-scripts" / p, cd / "05-scripts" / f"{s}.json"):
        if c.is_file():
            return c.resolve()
    raise SystemExit(f"no script {s} (looked in {cd / '05-scripts'})")


def proj_path(cd: Path, s: str) -> Path:
    p = Path(s)
    for c in (p, cd / p, cd / "07-video" / p):
        if (c / "mvf.json").is_file():
            return c.resolve()
    hits = sorted((cd / "07-video").glob(f"*/{s}")) if (cd / "07-video").is_dir() else []
    if len(hits) == 1 and (hits[0] / "mvf.json").is_file():
        return hits[0].resolve()
    raise SystemExit(f"no scaffolded project {s}; run `mvf scaffold` first")


def voice_ref(cd: Path, name: str) -> tuple[Path, Path]:
    """A campaign's own voice first, then this machine's project voices. Both are gitignored; no voices ship."""
    dirs = (cd / "voices", local_dir(cd) / "voices")
    for d in dirs:
        w, t = d / f"{name}.wav", d / f"{name}.txt"
        if w.is_file() and t.is_file():
            return w, t
    raise SystemExit(f"reference voice {name!r} not found in {', '.join(map(str, dirs))} (.wav + .txt)")


def cue_list(script: dict) -> list[dict]:
    """Spoken lines. A beat's own `voice` (a reference name) speaks that line instead of the script voice:
    a demo of another voice, a second character."""
    def cue(b):
        c = {"id": b["id"], "text": b["vo"].strip()}
        if isinstance(b.get("voice"), str) and b["voice"].strip():
            c["voice"] = b["voice"].strip()
        return c
    cues = [cue(b) for b in script["beats"] if (b.get("vo") or "").strip()]
    cues += [cue(v) for v in script.get("variants") or [] if (v.get("vo") or "").strip()]
    return cues


def voice_key(script: dict, engine: str | None = None) -> dict:
    v = script.get("voice") or {}
    return {"engine": engine, "mode": v.get("mode"), "ref": v.get("ref"), "name": v.get("name"),
            "speed": float(v.get("speed", 1.0)), "language": script.get("language")}


def takes(cd: Path, sid: str) -> list[Path]:
    d = cd / "07-video" / sid / "takes"
    return sorted(p for p in d.glob("t[0-9][0-9]") if (p / "manifest.json").is_file()) if d.is_dir() else []


def take_state(script: dict, take: Path) -> tuple[set, set]:
    """(cue ids the take voices as written, cue ids that are stale or missing)."""
    m = load(take / "manifest.json")
    have = {c["id"]: c for c in m["cues"]}
    same_voice = m.get("mvf_voice") == voice_key(script, (m.get("mvf_voice") or {}).get("engine"))
    ok, stale = set(), set()
    for c in cue_list(script):
        h = have.get(c["id"])
        same = same_voice and h and h["text"] == c["text"] and h.get("voice") == c.get("voice")
        (ok if same else stale).add(c["id"])
    return ok, stale


def pick_take(cd: Path, script: dict, want: str | None) -> Path | None:
    if want == "none":
        return None
    if want and want != "latest":
        p = Path(want)
        p = p if p.is_absolute() else (cd / "07-video" / script["id"] / "takes" / want)
        if not (p / "manifest.json").is_file():
            raise SystemExit(f"no take manifest at {p}")
        return p
    ts = takes(cd, script["id"])
    if not ts:
        return None
    ok, stale = take_state(script, ts[-1])
    if stale:
        print(f"WARN take {ts[-1].name} does not match the script for {sorted(stale)}: run `mvf voice` "
              f"(changed lines only are re-voiced); using estimated timing")
        return None
    return ts[-1]


# ---------- commands ----------

def cmd_init(a):
    root = Path(a.root) if a.root else project_root() / "marketing/campaigns"
    cd = root / a.slug
    if (cd / "campaign.json").exists():
        raise SystemExit(f"{cd} already has a campaign")
    for rel, tmpl in (("campaign.json", "campaign.json"), ("01-product/facts.json", "facts.json"),
                      ("02-audience/segments.json", "segments.json"), ("03-trends/trends.json", "trends.json"),
                      ("04-strategy/angles.json", "angles.json")):
        text = (TEMPLATES / tmpl).read_text(encoding="utf-8").replace("{{SLUG}}", a.slug).replace("{{PRODUCT}}", a.product)
        (cd / rel).parent.mkdir(parents=True, exist_ok=True)
        (cd / rel).write_text(text, encoding="utf-8")
    for d in ("01-product/screens", "05-scripts", "06-pretest", "07-video", "08-delivery", "brand", "voices", "extensions"):
        (cd / d).mkdir(parents=True, exist_ok=True)
    (cd / ".gitignore").write_text(CAMPAIGN_GITIGNORE, encoding="utf-8")
    print(f"campaign {cd}\nnext: fill campaign.json, then stage 0 (references/00-brief-material.md); `mvf status` shows the way")


def _stage_report(cd: Path, stage: str, c: dict) -> Report | None:
    plats = c.get("platforms") or []
    r = refs(cd)
    if stage == "brief":
        return checks.check_campaign(c)
    p = cd / dict(STAGES)[stage]
    if stage == "material":
        return material.check_pack(material.load_pack(cd), material.kinds(cd)) if p.is_file() else None
    if stage in {"product", "audience", "trends", "strategy"} and not p.is_file():
        return None
    if stage == "product":
        return checks.check_facts(load(p))
    if stage == "audience":
        return checks.check_segments(load(p), plats)
    if stage == "trends":
        return checks.check_trends(load(p), plats, set(r["segments"]))
    if stage == "strategy":
        return checks.check_angles(load(p), r["facts"], set(r["segments"]), set(r["trends"]), plats)
    if stage == "pretest":
        ds = sorted(p.glob("*/decision.json")) if p.is_dir() else []
        if not ds:
            return None
        rep = Report()
        for d in ds:
            rep.extend(checks.check_pretest(d))
        return rep
    return None


def cmd_sources(a):
    from mvflib.sources import registry
    for name, src in sorted(registry(maybe_campaign(a)).items()):
        print(f"{name:14} {src.about}")
        print(f"{'':14} yields {', '.join(src.kinds)}" + (f" · needs {', '.join(src.needs)}" if src.needs else ""))
        for p, h in src.params.items():
            print(f"{'':16}{p.lstrip('*')}={'(required) ' if p.startswith('*') else ''}{h}")


def cmd_gather(a):
    from mvflib import sources
    cd = campaign_dir(a)
    params = {}
    for kv in a.params:
        if "=" not in kv:
            raise SystemExit(f"param {kv!r}: write key=value")
        k, v = kv.split("=", 1)
        params[k] = v
    items = sources.run(a.source, params, cd)
    added, updated, rep = material.merge(cd, items, {"source": a.source, "params": params, "at": today().isoformat()})
    if rep.items:
        rep.print(f"rejected from {a.source}")
    print(f"{a.source}: {added} new, {updated} updated, {len(items) - added - updated} rejected → {cd / material.INDEX}")
    sys.exit(1 if rep.fails else 0)


def cmd_material(a):
    cd = campaign_dir(a)
    items = material.load_pack(cd)["items"]
    if a.action == "show":
        for iid in a.ids:
            if iid not in items:
                raise SystemExit(f"no item {iid}")
            it = items[iid]
            print(material.card(it, items, width=4000))
            for k in ("before", "after"):
                if (it.get("meta") or {}).get(k):
                    print(f"  [{k}] {it['meta'][k]}")
            print()
        return
    if a.kind and a.kind not in material.kinds(cd):
        raise SystemExit(f"unknown kind {a.kind}; known {sorted(material.kinds(cd))}")
    xs = material.rank(list(items.values()), a.kind, a.q)
    for it in xs[: a.limit]:
        print(material.card(it, items, width=160))
    print(f"-- {min(len(xs), a.limit)} of {len(xs)} items" + (f" (kind {a.kind})" if a.kind else ""))


def cmd_check(a):
    cd = campaign_dir(a)
    c = load(cd / "campaign.json")
    stages = ["brief", "material", "product", "audience", "trends", "strategy", "pretest"] if a.stage == "all" else [a.stage]
    total = Report()
    for st in stages:
        r = _stage_report(cd, st, c)
        if r is None:
            print(f"== {st}: not started ({dict(STAGES)[st]})")
            continue
        r.print(st)
        total.extend(r)
    sys.exit(1 if total.fails else 0)


def cmd_status(a):
    cd = campaign_dir(a)
    c = load(cd / "campaign.json")
    print(f"campaign {c.get('slug')} — {c.get('product')} · {', '.join(c.get('platforms') or [])} · today {today()}")
    nxt = None
    for st, rel in STAGES[:6]:
        r = _stage_report(cd, st, c)
        mark = "—" if r is None else ("FAIL" if r.fails else "ok")
        extra = ""
        if st == "material" and r is not None:
            items = material.load_pack(cd)["items"].values()
            kinds = {}
            for it in items:
                kinds[it["kind"]] = kinds.get(it["kind"], 0) + 1
            extra = " · " + " ".join(f"{k} {n}" for k, n in sorted(kinds.items()))
        print(f"  {st:9} {rel:28} {mark:5}" + ("" if r is None else f" {r.fails} FAIL {r.warns} WARN") + extra)
        if nxt is None and (r is None or r.fails):
            nxt = f"stage {st}: references/{REFERENCE[st]}" if r is None else f"fix FAILs: mvf check {st}"
    scripts = sorted((cd / "05-scripts").glob("*.json"))
    rf = refs(cd)
    from mvflib.lint import lint
    for sp in scripts:
        s = load(sp)
        t = pick_take(cd, s, "latest") if takes(cd, s["id"]) else None
        rep = lint(s, c, rf, timing.load_manifest(t) if t else None)
        projs = sorted((cd / "07-video" / s["id"]).glob("*-H*")) if (cd / "07-video" / s["id"]).is_dir() else []
        deliv = sorted((cd / "08-delivery" / s["id"]).glob("*/*/delivery.json")) if (cd / "08-delivery" / s["id"]).is_dir() else []
        print(f"  script    {sp.name:28} {'FAIL' if rep.fails else 'ok':5} {rep.fails} FAIL {rep.warns} WARN · take "
              f"{t.name if t else '—'} · projects {len(projs)} · deliveries {len(deliv)}")
        if nxt is None and rep.fails:
            nxt = f"fix script: mvf lint {sp.name}"
    pre = _stage_report(cd, "pretest", c)
    print(f"  pretest   06-pretest/*/decision.json  " + ("—" if pre is None else f"{pre.fails} FAIL {pre.warns} WARN"))
    if nxt is None:
        nxt = "write scripts (references/05-script.md)" if not scripts else \
              "pre-test (references/06-pretest.md), then mvf voice → scaffold → scenes → score → export"
    print(f"next: {nxt}")


def cmd_new_script(a):
    cd = campaign_dir(a)
    p = cd / "05-scripts" / f"{a.sid}.json"
    if p.exists():
        raise SystemExit(f"{p} exists")
    c = load(cd / "campaign.json")
    text = (TEMPLATES / "script.json").read_text(encoding="utf-8").replace("{{SID}}", a.sid)
    d = json.loads(text)
    d["language"] = (c.get("languages") or ["en"])[0]
    d["platforms"] = {p: d["platforms"][p] for p in c.get("platforms") or [] if p in d["platforms"]}
    dump(p, d)
    print(p)


def cmd_lint(a):
    from mvflib.lint import lint
    cd = campaign_dir(a)
    c = load(cd / "campaign.json")
    bad = 0
    for s_ in a.script:
        sp = script_path(cd, s_)
        s = load(sp)
        t = pick_take(cd, s, a.take)
        rep = lint(s, c, refs(cd), timing.load_manifest(t) if t else None, a.hook)
        rep.print(f"{sp.name}" + (f" (take {t.name})" if t else " (estimated timing)"))
        bad += rep.fails
    sys.exit(1 if bad else 0)


def cmd_trend_video(a):
    from mvflib.trendref import add
    add(campaign_dir(a), a.urls, a.frames, a.note)


def cmd_pretest_brief(a):
    from mvflib.pretest import brief
    cd = campaign_dir(a)
    sps = [script_path(cd, s) for s in a.script]
    hooks = {}
    for h in a.hooks or []:
        sid, hv = h.split("/")
        hooks.setdefault(sid, []).append(hv)
    tk = {load(p)["id"]: pick_take(cd, load(p), "latest") for p in sps}
    text = brief(cd, sps, hooks, a.size, a.question, tk)
    out = cd / "06-pretest" / a.round / "brief.md"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(text, encoding="utf-8")
    dec = out.parent / "decision.json"
    if not dec.exists():
        dump(dec, {"method": "simulated-panel", "run": None, "winner": None, "evidence": "",
                   "objections": [], "changes": [], "decided_at": None})
    print(f"{out}\nhand it to a simulated panel (any persona-simulation tool) or read it to real people;\n"
          f"then fill {dec} (method, run id, winner, evidence, changes)")


# ---------- voice engines ----------

CAMPAIGN_GITIGNORE = """# Machine-local or heavy: never committed.
extensions/
voices/
03-trends/refs/
07-video/*/takes/
07-video/*/recordings/
07-video/*/*/renders/
07-video/*/*/assets/vo/
08-delivery/
"""


def voices(cd: Path | None) -> dict:
    from mvflib import voice_files
    return {"files": voice_files.VOICE, **extensions.load(cd).voices}


def engine_for(cd: Path, c: dict, s: dict | None, want: str | None) -> tuple[str, extensions.Voice]:
    """--engine, then script voice.engine, campaign voice_engine, local config voice_engine; `recorded` means files."""
    v = (s or {}).get("voice") or {}
    name = want or v.get("engine") or c.get("voice_engine") or config(cd).get("voice_engine")
    if not name and v.get("mode") in (None, "recorded"):
        name = "files"
    reg = voices(cd)
    if not name:
        raise SystemExit(f"voice.mode {v.get('mode')!r} needs an engine: set voice_engine in campaign.json "
                         f"(or voice.engine, or --engine); engines: {', '.join(sorted(reg))} (`mvf extensions`)")
    if name not in reg:
        raise SystemExit(f"unknown voice engine {name!r}; engines: {', '.join(sorted(reg))} (`mvf extensions`)")
    return name, reg[name]


def parse_opts(kvs: list[str]) -> dict:
    out = {}
    for kv in kvs or []:
        if "=" not in kv:
            raise SystemExit(f"--opt {kv!r}: write key=value")
        k, v = kv.split("=", 1)
        out[k] = v
    return out


def run_voice(cd: Path, name: str, eng: extensions.Voice, mode: str, job: Path, out: Path, opts: dict) -> None:
    if mode not in eng.modes:
        raise SystemExit(f"engine {name} cannot {mode} (it does {', '.join(eng.modes)})")
    import os
    missing = [e for e in eng.needs if not os.environ.get(e)]
    if missing:
        raise SystemExit(f"engine {name} needs {', '.join(missing)} in the environment")
    if eng.run:
        eng.run(mode, load(job), out, opts)
        return
    cmd = [*heavy_prefix(cd), *eng.command, mode, "--job", str(job), "--out", str(out)]
    for k, v in opts.items():
        cmd += [f"--{k}", v]
    print(f"voicing with {name}…", flush=True)
    try:
        p = subprocess.Popen(cmd, cwd=eng.cwd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    except FileNotFoundError as e:
        raise SystemExit(f"engine {name}: {e.filename} not found ({extensions.load(cd).origin.get(('VOICES', name))})")
    for line in p.stdout:
        if "it/s]" in line or "s/it]" in line:
            continue
        print(line, end="", flush=True)
    if p.wait():
        raise SystemExit(f"voice engine {name} failed ({p.returncode})")


def cmd_voice(a):
    cd = campaign_dir(a)
    c = load(cd / "campaign.json")
    sp = script_path(cd, a.script)
    s = load(sp)
    v = s.get("voice") or {}
    mode = v.get("mode") or "recorded"
    if mode == "none":
        raise SystemExit("voice.mode is none: nothing to voice (scaffold uses on-screen timing)")
    name, eng = engine_for(cd, c, s, a.engine)
    opts = parse_opts(a.opt)
    say = say_map((c.get("brand") or {}).get("say"))
    tdir = cd / "07-video" / s["id"] / "takes"
    prev = takes(cd, s["id"])
    if a.rehear:
        if not prev:
            raise SystemExit("no take to re-hear")
        job = tdir / f"{prev[-1].name}.rehear.json"
        dump(job, {"language": s.get("language", "en"), "say": say})
        run_voice(cd, name, eng, "rehear", job, prev[-1], opts)
        return
    voice = {"mode": mode, "speed": float(v.get("speed", 1.0))}
    if mode in ("clone", "design"):
        if not v.get("ref"):
            raise SystemExit("voice.ref is empty. A designed voice must be fixed first, or every line sounds like a different "
                             "person: `mvf voice-design …`, listen, `mvf voice-pick <cand> --as <name>`, set voice.ref")
        wav, txt = voice_ref(cd, v["ref"])
        voice.update(mode="clone", ref_wav=str(wav), ref_txt=str(txt))
    elif mode == "preset":
        if not v.get("name"):
            raise SystemExit("voice.mode preset needs voice.name (the engine's stock voice)")
        voice["name"] = v["name"]
    cues = cue_list(s)
    if not cues:
        raise SystemExit("no spoken lines")
    reuse = set()
    if prev and not a.all:
        old = load(prev[-1] / "manifest.json")
        if (old.get("mvf_voice") or {}).get("engine") == name:
            reuse, _ = take_state(s, prev[-1])
    todo = [x for x in cues if x["id"] not in reuse]
    for x in todo:
        if x.get("voice") and mode != "recorded":
            w, t = voice_ref(cd, x["voice"])
            x.update(ref_wav=str(w), ref_txt=str(t))
    if not todo:
        print(f"take {prev[-1].name} already voices every line as written")
        return
    n = int(prev[-1].name[1:]) + 1 if prev else 1
    new = tdir / f"t{n:02d}"
    part = tdir / f"t{n:02d}.part"
    if part.exists():
        shutil.rmtree(part)
    job = tdir / f"t{n:02d}.job.json"
    dump(job, {"language": s.get("language", "en"), "say": say, "voice": voice, "cues": todo,
               "workdir": str(cd / "07-video" / s["id"])})
    print(f"{len(todo)} line(s) to voice with {name}" + (f", {len(reuse)} reused from {prev[-1].name}" if reuse else ""))
    run_voice(cd, name, eng, "take", job, part, opts)
    m = load(part / "manifest.json")
    if reuse:
        old = load(prev[-1] / "manifest.json")
        for oc in old["cues"]:
            if oc["id"] in reuse:
                shutil.copy2(prev[-1] / oc["path"], part / oc["path"])
                m["cues"].append(oc)
        order = {x["id"]: i for i, x in enumerate(cues)}
        m["cues"].sort(key=lambda x: order.get(x["id"], 99))
        m["reused_from"] = {prev[-1].name: sorted(reuse)}
    m["asr_problems"] = [x["id"] for x in m["cues"] if x["asr"]["dropped"] or x["asr"]["added"]]
    m["mvf_voice"] = voice_key(s, name)
    m["script_sha256"] = __import__("hashlib").sha256(sp.read_bytes()).hexdigest()[:16]
    dump(part / "manifest.json", m)
    part.rename(new)
    print(f"take {new.relative_to(cd)}" + (f"\nWARN ASR problems left in {m['asr_problems']}: listen, rewrite the line "
                                           f"or re-run `mvf voice` (new seeds)" if m["asr_problems"] else ""))
    if all(x["asr"].get("heard") is None for x in m["cues"]):
        print("INFO nothing heard this take back: listen to every line (an engine with rehear can check it: --rehear --engine …)")
    for x in m["cues"]:
        print(f"  {x['id']:4} {x['speech'][1] - x['speech'][0]:5.2f}s  {x['text'][:70]}")


def cmd_voice_design(a):
    cd = campaign_dir(a)
    c = load(cd / "campaign.json")
    name, eng = engine_for(cd, c, {"voice": {"mode": "design"}}, a.engine)
    out = cd / "voices/candidates"
    job = out / f"{a.name}.design.json"
    dump(job, {"name": a.name, "instruct": a.instruct, "language": a.language, "sample": a.sample, "n": a.n,
               "say": say_map((c.get("brand") or {}).get("say"))})
    run_voice(cd, name, eng, "design", job, out, parse_opts(a.opt))
    print(f"listen to {out}/{a.name}.cand*.wav, then `mvf voice-pick {a.name}.candN --as {a.name}`")


def cmd_voice_pick(a):
    cd = campaign_dir(a)
    src = cd / "voices/candidates"
    w, t = src / f"{a.cand}.wav", src / f"{a.cand}.txt"
    if not w.is_file():
        raise SystemExit(f"no candidate {w}")
    # A clone reference that says the wrong words teaches every line to slur: the
    # Whisper check from voice-design must be clean unless the person overrides it.
    rep = src / f"{a.cand.rsplit('.cand', 1)[0]}.candidates.json"
    row = next((r for r in (load(rep)["candidates"] if rep.is_file() else []) if r["name"] == a.cand), None)
    if row and (row["dropped"] or row["added"]) and not a.force:
        raise SystemExit(f"{a.cand} failed its hearing check (dropped={row['dropped']} added={row['added']}); "
                         f"pick a clean candidate, re-run voice-design, or --force after listening")
    shutil.copy2(w, cd / "voices" / f"{a.as_}.wav")
    shutil.copy2(t, cd / "voices" / f"{a.as_}.txt")
    print(f"voices/{a.as_}.wav is the reference; set script voice.ref = \"{a.as_}\" (mode clone or design)")


def cmd_scaffold(a):
    from mvflib.scaffold import placeholders, scaffold
    cd = campaign_dir(a)
    c = load(cd / "campaign.json")
    sp = script_path(cd, a.script)
    s = load(sp)
    take = pick_take(cd, s, a.take)
    hooks = ["H1"] + [v["id"] for v in s.get("variants") or []] if a.hook == "all" else a.hook.split(",")
    plats = [p for p in (s.get("platforms") or {}) if p in PLATFORMS] or c.get("platforms")
    canvases = a.canvas.split(",") if a.canvas else sorted({(s["platforms"][p] or {}).get("canvas", PLATFORMS[p]["canvas"]) for p in plats})
    base = None
    for cv in canvases:
        for h in hooks:
            src = Path(a.from_).resolve() if a.from_ else base
            proj = scaffold(cd, sp, cv, h, take, plats, from_dir=src)
            base = base or proj  # later hooks/canvases start from the first project's scenes
    left = placeholders(base) if base else []
    if left:
        print(f"placeholder scenes in {base.relative_to(cd)}: {left}\nnext: author them (SCENE_GUIDE.md, references/07-production.md)")


def cmd_score(a):
    from mvflib.mix import score
    from mvflib.scaffold import scaffold
    cd = campaign_dir(a)
    proj = proj_path(cd, a.proj)
    if not a.bed and not a.none:
        raise SystemExit("--bed FILE (a track you own or licensed) or --none")
    rep = score(proj, None if a.none else Path(a.bed).expanduser().resolve())
    for it in rep["items"]:
        print(f"{it['level']:4} MIX  {it['msg']}")
    # index.html picks the stems up on re-scaffold (same script, hook, canvas, take)
    m = load(proj / "mvf.json")
    take = Path(cd / m["take"]) if m.get("take") else None
    scaffold(cd, cd / m["script"], m["canvas"], m["hook"], take, m["platforms"])
    print(f"SCORED {proj.name} → {proj / 'assets/music'}")


def cmd_extensions(a):
    cd = maybe_campaign(a)
    if a.action == "init":
        d = extensions.init_local(cd)
        readme = d / "README.md"
        if not readme.exists():
            readme.write_text(LOCAL_README, encoding="utf-8")
        print(f"{d}  (gitignored: extensions/, voices/, config.json)\nnext: drop <name>.py into {d / 'extensions'}; "
              f"`mvf extensions` shows what loaded")
        return
    for d in extensions.dirs(cd):
        print(f"{'  ' if d.is_dir() else '- '}{d}" + ("" if d.is_dir() else "  (absent)"))
    ext = extensions.load(cd)
    print("voices:")
    for name, v in sorted(voices(cd).items()):
        where = ext.origin.get(("VOICES", name), "built in")
        print(f"  {name:12} {v.about}\n  {'':12} modes {', '.join(v.modes)} · {where}")
        for k, h in v.opts.items():
            print(f"  {'':14}--opt {k}=… {h}")
    print("sources:")
    from mvflib.sources import registry
    for name, src in sorted(registry(cd).items()):
        print(f"  {name:12} {src.about} · {ext.origin.get(('SOURCES', name), 'built in')}")
    for label, table, key in (("kinds", ext.kinds, "KINDS"), ("recipes", ext.recipes, "RECIPES")):
        if table:
            print(f"{label} added:")
            for name in sorted(table):
                print(f"  {name:12} {ext.origin[(key, name)]}")
    for kind, name, f in ext.shadowed:
        print(f"note: {kind} {name} in {f} is shadowed by {ext.origin[(kind, name)]}")


LOCAL_README = """# .babysit/marketing-video — machine-local, never committed

extensions/  <name>.py or <name>/extension.py: data sources, voice engines, kinds, recipes
voices/      <name>.wav + <name>.txt clone references for every campaign in this repo
config.json  {"voice_engine": "omnivoice", "heavy": ["path/to/lock.sh"]}  (both optional)

A source extension:

    from mvflib.extensions import Source

    def fetch(p):  # p: the k=v params given to `mvf gather`
        return [{"id": "store:review:123", "kind": "review", "title": "…", "body": "…",
                 "signals": {"stars": 5}, "rights": "unknown",
                 "source": {"name": "store.reviews", "url": "https://…", "fetched_at": "2026-10-31"}}]

    SOURCES = {"store.reviews": Source("App-store reviews", ["review"], fetch, {"*app": "app id"}, needs=["STORE_TOKEN"])}

A voice engine: VOICES = {"name": Voice(about, modes=("take",), command=[…])} — see
mvflib/extensions.py for the job and manifest contract (references/07-production.md).
"""


def cmd_export(a):
    from mvflib.export import export
    cd = campaign_dir(a)
    plats = a.platforms.split(",")
    for p in plats:
        if p not in PLATFORMS:
            raise SystemExit(f"unknown platform {p}")
    for pj in a.proj:
        proj = proj_path(cd, pj)
        rep = export(cd, proj, plats, draft=a.draft, skip_render=a.skip_render)
        rep.print(f"QA {proj.name}")
        print(f"delivered → {cd / '08-delivery'}  (listen and watch every file before it is called done; nothing was uploaded)")
        if rep.fails:
            sys.exit(1)


def cmd_qa(a):
    from mvflib.export import qa
    cd = campaign_dir(a)
    proj = proj_path(cd, a.proj)
    rep = qa(Path(a.video).resolve(), a.platform, load(proj / "timeline.json"))
    rep.print(f"QA {Path(a.video).name} for {a.platform}")
    sys.exit(1 if rep.fails else 0)


def main():
    ap = argparse.ArgumentParser(prog="mvf", description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--campaign", help="campaign dir (default: the one containing the cwd)")
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("init")
    p.add_argument("slug")
    p.add_argument("--product", required=True)
    p.add_argument("--root")
    p.set_defaults(fn=cmd_init)
    sub.add_parser("status").set_defaults(fn=cmd_status)
    sub.add_parser("sources").set_defaults(fn=cmd_sources)
    p = sub.add_parser("extensions")
    p.add_argument("action", nargs="?", default="ls", choices=["ls", "init"])
    p.set_defaults(fn=cmd_extensions)
    p = sub.add_parser("gather")
    p.add_argument("source")
    p.add_argument("params", nargs="*", help="key=value …")
    p.set_defaults(fn=cmd_gather)
    p = sub.add_parser("material")
    p.add_argument("action", nargs="?", default="ls", choices=["ls", "show"])
    p.add_argument("ids", nargs="*")
    p.add_argument("--kind")
    p.add_argument("-q")
    p.add_argument("--limit", type=int, default=30)
    p.set_defaults(fn=cmd_material)
    p = sub.add_parser("check")
    p.add_argument("stage", nargs="?", default="all", choices=["all", "brief", "material", "product", "audience", "trends", "strategy", "pretest"])
    p.set_defaults(fn=cmd_check)
    p = sub.add_parser("new-script")
    p.add_argument("sid")
    p.set_defaults(fn=cmd_new_script)
    p = sub.add_parser("lint")
    p.add_argument("script", nargs="+")
    p.add_argument("--hook")
    p.add_argument("--take", default="latest", help="latest (default) | none | tNN")
    p.set_defaults(fn=cmd_lint)
    p = sub.add_parser("trend-video")
    p.add_argument("urls", nargs="+")
    p.add_argument("--frames", action="store_true", help="download and cut hook frames + a contact sheet")
    p.add_argument("--note")
    p.set_defaults(fn=cmd_trend_video)
    p = sub.add_parser("pretest-brief")
    p.add_argument("script", nargs="+")
    p.add_argument("--round", default="r1")
    p.add_argument("--size", default="quick", choices=["quick", "standard", "deep"])
    p.add_argument("--hooks", nargs="*", help="limit to sid/H1 sid/H2 …")
    p.add_argument("--question")
    p.set_defaults(fn=cmd_pretest_brief)
    p = sub.add_parser("voice")
    p.add_argument("script")
    p.add_argument("--engine", help="voice engine (default: voice.engine, campaign voice_engine, config, files)")
    p.add_argument("--opt", action="append", help="engine option key=value (repeatable; `mvf extensions` lists them)")
    p.add_argument("--all", action="store_true", help="re-voice every line, not only changed ones")
    p.add_argument("--rehear", action="store_true", help="re-run ASR + alignment on the latest take (same audio)")
    p.set_defaults(fn=cmd_voice)
    p = sub.add_parser("voice-design")
    p.add_argument("--name", required=True)
    p.add_argument("--instruct", required=True, help="e.g. 'female, young adult, moderate pitch, american accent'")
    p.add_argument("--sample", required=True, help="a 10-15 s line in the campaign's register")
    p.add_argument("--language", default="en")
    p.add_argument("--n", type=int, default=3)
    p.add_argument("--engine")
    p.add_argument("--opt", action="append")
    p.set_defaults(fn=cmd_voice_design)
    p = sub.add_parser("voice-pick")
    p.add_argument("cand")
    p.add_argument("--as", dest="as_", required=True)
    p.add_argument("--force", action="store_true", help="adopt a candidate that failed the hearing check")
    p.set_defaults(fn=cmd_voice_pick)
    p = sub.add_parser("scaffold")
    p.add_argument("script")
    p.add_argument("--hook", default="H1", help="H1 | H2,H3 | all")
    p.add_argument("--canvas", help="9x16,1x1,… (default: the script's platform canvases)")
    p.add_argument("--take", default="latest", help="latest | none | tNN")
    p.add_argument("--from", dest="from_", help="copy authored scenes from another project")
    p.set_defaults(fn=cmd_scaffold)
    p = sub.add_parser("score")
    p.add_argument("proj")
    p.add_argument("--bed", help="music bed: a track you own or licensed")
    p.add_argument("--none", action="store_true", help="no bed (removes a previous one)")
    p.set_defaults(fn=cmd_score)
    p = sub.add_parser("export")
    p.add_argument("proj", nargs="+")
    p.add_argument("--platforms", default="tiktok,x")
    p.add_argument("--draft", action="store_true", help="rough cut: allows placeholders and estimated timing")
    p.add_argument("--skip-render", action="store_true", help="reuse renders/master.mp4")
    p.set_defaults(fn=cmd_export)
    p = sub.add_parser("qa")
    p.add_argument("video")
    p.add_argument("--platform", required=True, choices=list(k for k in PLATFORMS if k not in {"canvases", "speech_wps", "_about"}))
    p.add_argument("--proj", required=True)
    p.set_defaults(fn=cmd_qa)
    a = ap.parse_args()
    a.fn(a)


if __name__ == "__main__":
    main()
