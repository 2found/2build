#!/usr/bin/env python3
"""mvf — the marketing-video flow's tool. Every stage writes one artifact into the
campaign dir; every artifact has a check; every video is measured against the
platform it ships to. Run from anywhere inside a campaign (or pass --campaign).

  init <slug> --product NAME [--format F]   new campaign from templates (default <repo>/marketing/campaigns/<slug>)
  format save <name> --from <sid> | ls      keep a series' look, captions, ambience, rigs, props and voice by name
  status                            stage by stage: present, FAIL/WARN counts, what is next
  next [--json]                     the one next action, and whether the agent or the owner takes it
  sheet <sid>                       the script as the owner reads it; approve <sid> --verdict … records their word
  extensions [ls|init]              where extensions load from and what they add; init the local dir
  sources                           every data source: what it yields, its params
  gather <source> [k=v …]           pull items from a source into 00-material/index.json
  material [ls|show ID…] [--kind K] [-q TEXT]   read the pack as short cards, strongest first
  predict | overlap                 briefs for fresh helpers: the obvious videos, then each angle against them
  check [stage]                     validate brief|material|product|audience|trends|research|strategy|art|pretest|all
  new-script <sid>                  05-scripts/<sid>.json from the template
  lint <script> [--hook H2]         hook timing, first value, length, claims→facts, brand, on-screen load, platform copy
  trend-video URL… [--frames]       public metadata (+ hook frames) of reference posts into trends.json
  top-videos "QUERY" | --url URL…   the most-viewed videos and what they say: transcripts into 03-research/top/
  pretest-brief <script>… [--round r1]   pre-test brief with the material verbatim
  coldread <script> [--hooks H1 …]  second-by-second feed view for fresh-context swipe tests
  feed <script>… --control <sid>    shuffled feed of candidates, a rejected control and real benchmark videos
  voice <script> [--engine E]       a take: every line + the script's chosen `hook` as audio with word timing
  voice-design --name --instruct …  candidate voices for a designed narrator (an engine with `design`)
  voice-pick <cand> --as NAME       a candidate becomes the campaign's reference voice
  art [ls|check|draw|approve] [ID…]  generated plates, backgrounds, insets, stickers (07-video/art/art.json)
  scaffold <script> [--hook H2]     one HyperFrames project per canvas, on the chosen hook and measured timing
  score <proj> --bed FILE|auto:MOOD|--none  SFX from the direction + a bed (yours or procedural), ducked, mix-checked
  export <proj> [--platforms …]     render, master −14 LUFS, encode per platform, QA, copy, AI-label notes
  viewer <proj>                     brief for a fresh helper who sees only the cut: frames at 1 fps + the voice
  thumbnail <sid> [--canvas 16x9]  custom thumbnail/cover still: approved art + the thumbnail line, ≤ 2 MB JPEG
  qa <video> --platform P --proj D  measure one file against a platform
  doctor [--json]                   is this machine ready (HyperFrames, OmniVoice, Qwen+RMBG, Micro Fish)
  judge setup|scripts|video …       Micro Fish on the user's Soot judges the scripts, then the cut

Voice engines: `files` (built in — your own recordings, one per line) and any
extension (`mvf extensions`; `omnivoice` ships with the skill). Data sources:
`manual` (built in) and any extension. Image engines: `qwen` (built in — the
image-gen skill, local and free) and any extension; a paid one needs --spend. Extensions live in <campaign>/extensions/
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
from mvflib import checks, extensions, flow, material, timing  # noqa: E402
from mvflib.common import (PLATFORMS, STAGES, TOOLS, Report, config, dump, find_campaign,  # noqa: E402
                           heavy_prefix, load, local_dir, project_root, say_map, today, user_dir)

TEMPLATES = TOOLS / "templates/campaign"


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
            "ideas": ({x["id"]: x for x in opt("03-research/research.json", "ideas") if "id" in x}
                      if (cd / "03-research/research.json").is_file() else None),
            "lore_terms": (load(cd / "03-research/research.json").get("lore_terms") or []
                           if (cd / "03-research/research.json").is_file() else []),
            "material": material.load_pack(cd)["items"], "recipes": recipes(cd),
            "art": {j.get("id"): j for j in opt("07-video/art/art.json", "jobs")},
            "thesis": flow.thesis_picks(cd), "past": past_scripts(cd)}


def past_scripts(cd: Path) -> list[tuple[str, str]]:
    """The voice-over of every script in the other campaigns beside this one: what the owner already made."""
    out = []
    for f in sorted(cd.resolve().parent.glob("*/05-scripts/*.json")):
        if f.parents[1] == cd.resolve() or not (f.parents[1] / "campaign.json").is_file():
            continue
        try:
            d = load(f)
        except (ValueError, OSError):
            continue
        beats = (d.get("beats") if isinstance(d, dict) else None) or []
        out.append((f"{f.parents[1].name}/{f.stem}", " ".join(str(b.get("vo") or "") for b in beats if isinstance(b, dict))))
    return out


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


def voice_library() -> Path:
    """This OS user's voices, for every repo and campaign: ~/.babysit/marketing-video/voices/. Never in a repo."""
    return user_dir() / "voices"


def voice_ref(cd: Path, name: str) -> tuple[Path, Path]:
    """A campaign's own voice first, then this repo's, then the user's library. No voices ship. A voice found in a
    campaign or repo goes into the library the first time it is used, so the user gives each voice once."""
    lib = voice_library()
    dirs = (cd / "voices", local_dir(cd) / "voices", lib)
    for d in dirs:
        w, t = d / f"{name}.wav", d / f"{name}.txt"
        if w.is_file() and t.is_file():
            if d != lib:
                cache_voice(w, t)
            return w, t
    raise SystemExit(f"reference voice {name!r} not found in {', '.join(map(str, dirs))} (.wav + .txt): put it in "
                     f"{lib} once and every campaign finds it")


def cache_voice(wav: Path, txt: Path) -> None:
    """Copy a voice into the library unless it holds that name already; a different voice under the name is kept."""
    lib = voice_library()
    cw, ct = lib / wav.name, lib / txt.name
    if cw.is_file() and ct.is_file():
        if cw.read_bytes() != wav.read_bytes():
            print(f"INFO voice {wav.stem}: {wav} is not the library's {cw}; this campaign uses its own")
        return
    lib.mkdir(parents=True, exist_ok=True)
    shutil.copy2(wav, cw)
    shutil.copy2(txt, ct)
    print(f"INFO voice {wav.stem} saved to {lib}: every campaign on this machine can use it as voice.ref")


def cue_list(script: dict) -> list[dict]:
    """Spoken lines. A beat's own `voice` (a reference name) speaks that line instead of the script voice:
    a demo of another voice, a second character."""
    def cue(b):
        c = {"id": b["id"], "text": b["vo"].strip()}
        if isinstance(b.get("voice"), str) and b["voice"].strip():
            c["voice"] = b["voice"].strip()
        return c
    cues = [cue(b) for b in script["beats"] if (b.get("vo") or "").strip()]
    # One video: only the hook it ships with is voiced. Other variants live in the script for the pre-test.
    hook = script.get("hook") or "H1"
    cues += [cue(v) for v in script.get("variants") or [] if v.get("id") == hook and (v.get("vo") or "").strip()]
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


def current_take(cd: Path, script: dict, stored: Path | None) -> Path | None:
    """The take a re-scaffold should use: the project's own, unless the script has moved past it."""
    if stored is None or not take_state(script, stored)[1]:
        return stored
    newer = pick_take(cd, script, "latest")
    if newer is not None and newer.resolve() != stored.resolve():
        print(f"INFO take {stored.name} no longer voices the script as written: using {newer.name}")
        return newer
    print(f"WARN take {stored.name} no longer voices the script as written: run `mvf voice`, then `mvf scaffold`")
    return stored


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
    if a.format:
        from mvflib import formats
        formats.plain(a.format)
    if a.format and not (root / "_formats" / a.format / "format.json").is_file():
        have = ", ".join(r["name"] for r in formats.listing(root)) or "none"
        raise SystemExit(f"no format {a.format!r} in {root / '_formats'} (have: {have})")
    for rel, tmpl in (("campaign.json", "campaign.json"), ("01-product/facts.json", "facts.json"),
                      ("02-audience/segments.json", "segments.json"), ("03-trends/trends.json", "trends.json"),
                      ("03-research/research.json", "research.json"), ("04-strategy/angles.json", "angles.json")):
        text = (TEMPLATES / tmpl).read_text(encoding="utf-8").replace("{{SLUG}}", a.slug).replace("{{PRODUCT}}", a.product)
        (cd / rel).parent.mkdir(parents=True, exist_ok=True)
        (cd / rel).write_text(text, encoding="utf-8")
    for d in ("01-product/screens", "05-scripts", "06-pretest", "07-video", "08-delivery", "brand", "voices", "extensions"):
        (cd / d).mkdir(parents=True, exist_ok=True)
    (cd / ".gitignore").write_text(CAMPAIGN_GITIGNORE, encoding="utf-8")
    if a.format:
        from mvflib import formats
        f = formats.apply(cd, a.format)
        v = (f.get("voice") or {}).get("ref")
        print(f"format {a.format} (from {f.get('from')}): style {f.get('style') or 'none'}, look/captions/ambience in "
              f"{formats.APPLIED}" + (f"; set each script's voice.ref = \"{v}\"" if v else ""))
    print(f"campaign {cd}\nnext: `mvf next` (it walks the stages and says who acts)")


def cmd_format(a):
    from mvflib import formats
    if a.action == "save":
        if not a.name or not a.sid:
            raise SystemExit("mvf format save <name> --from <sid>")
        cd = campaign_dir(a)
        out = formats.save(cd, a.name, a.sid, force=a.force)
        print(f"format {a.name} → {out}\nstart the next episode with `mvf init <slug> --product … --format {a.name}`")
        return
    camps = campaign_dir(a).parent if maybe_campaign(a) else (Path(a.root) if a.root else project_root() / "marketing/campaigns")
    rows = formats.listing(camps)
    for r in rows:
        print(f"{r['name']}: broken format.json" if r.get("broken") else
              f"{r['name']}: from {r['from']} ({r['saved']}), style {r['style'] or 'none'}, voice {r['voice'] or 'none'}"
              + (f", {' '.join(r['sheets'])}" if r["sheets"] else ""))
    if not rows:
        print(f"no formats in {formats.root(camps)}: `mvf format save <name> --from <sid>` inside a campaign")


def _stage_report(cd: Path, stage: str, c: dict) -> Report | None:
    plats = c.get("platforms") or []
    r = refs(cd)
    if stage == "brief":
        return checks.check_campaign(c)
    p = cd / dict(STAGES)[stage]
    if stage == "material":
        return material.check_pack(material.load_pack(cd), material.kinds(cd), c.get("material_skip")) if p.is_file() else None
    if stage in {"product", "audience", "trends", "research", "strategy"} and not p.is_file():
        return None
    if stage == "product":
        return checks.check_facts(load(p))
    if stage == "audience":
        return checks.check_segments(load(p), plats, c.get("goal"))
    if stage == "trends":
        return checks.check_trends(load(p), plats, set(r["segments"]))
    if stage == "research":
        from mvflib import research
        return research.check(load(p), c.get("benefit"), int((c.get("budget") or {}).get("scripts") or 3),
                              r["facts"])
    if stage == "strategy":
        from mvflib import fresh
        a = load(p)
        rep = checks.check_angles(a, r["facts"], set(r["segments"]), set(r["trends"]), plats, r.get("ideas"),
                                  c.get("benefit"), r.get("lore_terms"), c.get("goal"))
        rep.extend(fresh.check(cd, [g for g in a.get("angles") or [] if isinstance(g, dict)]))
        return rep
    if stage == "art":
        from mvflib import art
        return art.check(cd, persons(cd)) if (cd / art.ART / "art.json").is_file() else None
    if stage == "pretest":
        ds = sorted(p.glob("*/decision.json")) if p.is_dir() else []
        if not ds:
            return None
        from mvflib import flow
        rep = Report()
        cur = flow.decision(cd)
        for d in ds:
            if cur and d != cur[0]:  # a newer round decided: an older one is history, not a gate
                rep.info("PRE", f"pretest.{d.parent.name}", f"superseded by {cur[0].parent.name}")
                continue
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
            print(material.card(it, items, width=0))
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
    stages = ["brief", "material", "product", "audience", "trends", "research", "strategy", "art", "pretest"] if a.stage == "all" else [a.stage]
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
    for st, rel in STAGES[:7]:
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
    scripts = sorted((cd / "05-scripts").glob("*.json"))
    rf = refs(cd)
    from mvflib.lint import lint
    for sp in scripts:
        s = load(sp)
        t = pick_take(cd, s, "latest") if takes(cd, s["id"]) else None
        rep = lint(s, c, rf, timing.load_manifest(t) if t else None)
        projs = sorted((cd / "07-video" / s["id"]).glob("*/mvf.json")) if (cd / "07-video" / s["id"]).is_dir() else []
        deliv = sorted((cd / "08-delivery" / s["id"]).glob("*/delivery.json")) if (cd / "08-delivery" / s["id"]).is_dir() else []
        print(f"  script    {sp.name:28} {'FAIL' if rep.fails else 'ok':5} {rep.fails} FAIL {rep.warns} WARN · take "
              f"{t.name if t else '—'} · projects {len(projs)} · deliveries {len(deliv)}")
    ar = _stage_report(cd, "art", c)
    if ar is not None:
        from mvflib import art
        st = art.status(cd)
        print(f"  art       07-video/art/art.json        {'FAIL' if ar.fails else 'ok':5} {ar.fails} FAIL {ar.warns} WARN · "
              f"jobs {len(st)} · drafted {sum(1 for x in st if x['drafts'])} · approved {sum(x['approved'] for x in st)}")
    pre = _stage_report(cd, "pretest", c)
    print(f"  pretest   06-pretest/*/decision.json  " + ("—" if pre is None else f"{pre.fails} FAIL {pre.warns} WARN"))
    from mvflib import flow
    flow.print_action(next_of(cd, c))


def next_of(cd: Path, c: dict) -> dict:
    from mvflib import flow
    from mvflib.lint import lint
    rf = refs(cd)

    def lint_fails(sid):
        sc = load(cd / "05-scripts" / f"{sid}.json")
        t = pick_take(cd, sc, "latest") if takes(cd, sc["id"]) else None
        return lint(sc, c, rf, timing.load_manifest(t) if t else None).fails
    return flow.next_action(cd, lambda st: _stage_report(cd, st, c), lint_fails)


def cmd_next(a):
    from mvflib import flow
    cd = campaign_dir(a)
    x = next_of(cd, load(cd / "campaign.json"))
    if a.json:
        print(json.dumps(x, ensure_ascii=False))
    else:
        flow.print_action(x)


def cmd_sheet(a):
    from mvflib import flow
    cd = campaign_dir(a)
    sid = load(script_path(cd, a.sid))["id"]
    text = flow.sheet(cd, sid)
    out = cd / "06-pretest" / f"sheet-{sid}.md"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(text, encoding="utf-8")
    print(text + f"\n→ {out}")


def cmd_approve(a):
    from mvflib import flow
    cd = campaign_dir(a)
    sid = load(script_path(cd, a.sid))["id"]
    rec = flow.approve(cd, sid, a.verdict, a.note or "", a.hook, a.thesis)
    print(f"owner {rec['verdict']} {sid}/{rec['hook']} on {rec['at']}"
          + (f", thesis {rec['thesis']!r}" if rec.get("thesis") else "") + f" → {cd / flow.OWNER}")


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
    angles = {g.get("id"): g for g in (load(cd / "04-strategy/angles.json").get("angles") or [])} \
        if (cd / "04-strategy/angles.json").is_file() else {}
    if a.angle and a.angle not in angles:
        raise SystemExit(f"no angle {a.angle} in 04-strategy/angles.json (have {', '.join(angles) or 'none'})")
    g = angles.get(a.angle) or (next(iter(angles.values())) if len(angles) == 1 else None)
    d["angle"] = g["id"] if g else None
    if g and g.get("format"):
        d["format"] = g["format"]
    d["recipe"] = (g or {}).get("recipe")  # the template's product-demo is an example, not every script's build
    dump(p, d)
    print(f"{p}  (angle {d['angle']}, format {d['format']})" if d["angle"] else f"{p}  (no angle set: --angle G…)")


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
        v = s.get("voice") or {}
        if v.get("mode") in {"clone", "design"} and v.get("ref"):
            try:
                voice_ref(cd, v["ref"])
            except SystemExit as e:
                rep.warn("S2", f"{s.get('id')}.voice.ref", f"{e}: the take will fail; add the voice or change voice.ref")
        rep.print(f"{sp.name}" + (f" (take {t.name})" if t else " (estimated timing)"))
        bad += rep.fails
    sys.exit(1 if bad else 0)


def cmd_trend_video(a):
    from mvflib.trendref import add
    add(campaign_dir(a), a.urls, a.frames, a.note)


def cmd_top_videos(a):
    from mvflib.topvideos import study
    study(campaign_dir(a), a.query, a.url, a.n, a.seconds, a.scope, a.max_min, a.platform)


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


def cmd_predict(a):
    from mvflib import fresh
    cd = campaign_dir(a)
    out = cd / "04-strategy" / "predict-brief.md"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(fresh.predict_brief(load(cd / "campaign.json")), encoding="utf-8")
    print(f"{out}\ngive it to a fresh helper (no other context); it writes {cd / fresh.PREDICT}. Then `mvf overlap`.")


def cmd_overlap(a):
    from mvflib import fresh
    cd = campaign_dir(a)
    if not (cd / "04-strategy/angles.json").is_file():
        raise SystemExit("no 04-strategy/angles.json: write the angles first")
    out = cd / "04-strategy" / "overlap-brief.md"
    out.write_text(fresh.overlap_brief(cd), encoding="utf-8")
    print(f"{out}\ngive it to a fresh helper (no other context); it writes {cd / fresh.OVERLAP}. "
          "Then `mvf check strategy`.")


def cmd_coldread(a):
    from mvflib.pretest import coldread
    cd = campaign_dir(a)
    sp = script_path(cd, a.script)
    s = load(sp)
    take = pick_take(cd, s, "latest") if takes(cd, s["id"]) else None
    hooks = a.hooks or ["H1"] + [v["id"] for v in s.get("variants") or []]
    out = cd / "06-pretest" / a.round / "coldread"
    out.mkdir(parents=True, exist_ok=True)
    for h in hooks:
        f = out / f"{s['id']}-{h}.md"
        f.write_text(coldread(s, h, take), encoding="utf-8")
        print(f)
    print(f"give each file to {a.readers}+ fresh-context readers (no campaign files, no other context); save each JSON\n"
          f"answer as {out}/<sid>-<hook>.<reader>.json, fix what they flag, re-run (references/06-pretest.md#cold-read)")


def cmd_feed(a):
    import json as _json
    from mvflib.pretest import bench_clip, feed, script_clip
    cd = campaign_dir(a)
    clips = []
    for kind, names in (("candidate", a.script), ("control", a.control or [])):
        for n in names:
            sid, _, h = n.partition("/")
            s = load(script_path(cd, sid))
            clips.append(script_clip(s, h or "H1", pick_take(cd, s, "latest") if takes(cd, s["id"]) else None, kind))
    bench = sorted((cd / "06-pretest" / "bench").glob("*.json"))
    clips += [bench_clip(load(p)) for p in bench]
    if not a.control:
        print("WARN no --control: add a script the team already rejected; a feed with no known loser cannot tell a "
              "generous reader from a good script")
    if not bench:
        print("WARN no 06-pretest/bench/*.json: transcribe 2-3 real videos with known views (references/06-pretest.md#feed)")
    out = cd / "06-pretest" / a.round / "feed"
    out.mkdir(parents=True, exist_ok=True)
    for i in range(a.readers):
        text, key = feed(clips, a.seed + i)
        (out / f"feed-{i + 1}.md").write_text(text, encoding="utf-8")
        (out / f"feed-{i + 1}.key.json").write_text(_json.dumps(key, ensure_ascii=False, indent=1), encoding="utf-8")
        print(out / f"feed-{i + 1}.md")
    print(f"give feed-N.md (never the key) to reader N, a fresh-context agent with a one-line persona; save the answer\n"
          f"as {out}/feed-N.<reader>.json, then read it against feed-N.key.json (references/06-pretest.md#feed)")


# ---------- voice engines ----------

CAMPAIGN_GITIGNORE = """# Machine-local or heavy: never committed.
extensions/
voices/
03-trends/refs/
07-video/*/takes/
07-video/*/recordings/
07-video/*/*/renders/
07-video/*/*/assets/vo/
07-video/*/*/assets/art/
07-video/art/draft/
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
    from mvflib import flow
    if not a.rehear:
        gate = flow.owner_gate(cd, s["id"], a.owner_skip)
        if gate:
            raise SystemExit(f"BLOCKED owner: {gate}")
        gate = flow.pretest_gate(cd, s["id"]) if not (a.owner_skip and a.owner_skip.strip()) else None
        if gate:
            raise SystemExit(f"BLOCKED pretest: {gate}")
        if a.owner_skip and a.owner_skip.strip():
            flow.record_skip(cd, s["id"], a.owner_skip)
        drift = flow.owner_drift(cd, s["id"])
        if drift:
            print(f"WARN {drift}")
    name, eng = engine_for(cd, c, s, a.engine)
    opts = parse_opts(a.opt)
    # a script in another language overrides how brand names are said (script.say wins)
    say = say_map({**((c.get("brand") or {}).get("say") or {}), **(s.get("say") or {})})
    tdir = cd / "07-video" / s["id"] / "takes"
    prev = takes(cd, s["id"])
    if a.rehear:
        if not prev:
            raise SystemExit("no take to re-time")
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
    m["mvf_voice"] = voice_key(s, name)
    m["script_sha256"] = __import__("hashlib").sha256(sp.read_bytes()).hexdigest()[:16]
    dump(part / "manifest.json", m)
    part.rename(new)
    print(f"take {new.relative_to(cd)}: listen to every line")
    if not m.get("asr_model"):
        print("INFO word times are spread by length: an engine with rehear times them from the audio (--rehear --engine …)")
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
    shutil.copy2(w, cd / "voices" / f"{a.as_}.wav")
    shutil.copy2(t, cd / "voices" / f"{a.as_}.txt")
    cache_voice(cd / "voices" / f"{a.as_}.wav", cd / "voices" / f"{a.as_}.txt")
    print(f"voices/{a.as_}.wav is the reference; set script voice.ref = \"{a.as_}\" (mode clone or design)")


def cmd_scaffold(a):
    from mvflib.scaffold import placeholders, scaffold
    cd = campaign_dir(a)
    c = load(cd / "campaign.json")
    sp = script_path(cd, a.script)
    s = load(sp)
    take = pick_take(cd, s, a.take)
    hook = a.hook or s.get("hook") or "H1"
    if hook != "H1" and hook not in {v.get("id") for v in s.get("variants") or []}:
        raise SystemExit(f"no hook {hook} in {sp.name}: one video is made from one hook (set the script's `hook` to "
                         "the pre-test winner)")
    plats = [p for p in (s.get("platforms") or {}) if p in PLATFORMS] or c.get("platforms")
    canvases = a.canvas.split(",") if a.canvas else sorted({(s["platforms"][p] or {}).get("canvas", PLATFORMS[p]["canvas"]) for p in plats})
    firsts = []
    for cv in canvases:
        src = Path(a.from_).resolve() if a.from_ else None
        firsts.append(scaffold(cd, sp, cv, hook, take, plats, from_dir=src))
    for base in firsts:
        left = placeholders(base)
        if left:
            print(f"placeholder scenes in {base.relative_to(cd)}: {left}\nnext: author them (SCENE_GUIDE.md, references/07-production.md)")


def cmd_score(a):
    from mvflib import sound
    from mvflib.mix import score
    from mvflib.scaffold import scaffold
    cd = campaign_dir(a)
    proj = proj_path(cd, a.proj)
    if not a.bed and not a.none:
        raise SystemExit("--bed FILE (a track you own or licensed), --bed auto:<mood> "
                         f"({'|'.join(sound.MOODS)}, procedural) or --none")
    m = load(proj / "mvf.json")
    take = Path(cd / m["take"]) if m.get("take") else None
    take = current_take(cd, load(cd / m["script"]), take) if take else None
    # sfx.json and timeline.json come from direction.json: refresh them first, or an edited
    # direction is mixed against the previous cues
    scaffold(cd, cd / m["script"], m["canvas"], m["hook"], take, m["platforms"])
    if a.sfx != "none":
        sr = sound.render_sfx(proj, cd)
        print(f"INFO SFX  {sr['cues']} cues → assets/music/sfx.wav · " + ", ".join(f"{k} {v}" for k, v in sr["sources"].items())
              + (f" · unknown kinds {sr['unknown']}" if sr["unknown"] else ""))
    else:
        (proj / "assets/music/sfx.wav").unlink(missing_ok=True)
    bed = None
    if not a.none:
        bed = sound.auto_bed(proj, a.bed.split(":", 1)[1], a.seed) if a.bed.startswith("auto:") \
            else Path(a.bed).expanduser().resolve()
    rep = score(proj, bed)
    if bed is not None and bed.name == "bed.src.wav":
        bed.unlink(missing_ok=True)
    for it in rep["items"]:
        print(f"{it['level']:4} MIX  {it['msg']}")
    mc = sound.mixcheck(proj)
    lvl = "WARN" if mc["under"] else "INFO"
    print(f"{lvl:4} MIX  voice over bed+sfx: worst {mc['min_db']} dB (target ≥ {mc['target_db']:g})"
          + (f"; under in {', '.join(mc['under'])}: lower those cues or the bed" if mc["under"] else "") + " · PENDING LISTEN")
    # index.html picks the stems up on re-scaffold (same script, hook, canvas, take)
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
             (~/.babysit/marketing-video/voices/ holds them for every repo; a voice used once lands there)
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


def project_platforms(cd: Path, proj: Path) -> list[str]:
    """The script's platforms whose canvas is this project's canvas (a 16x9 project is not a TikTok file)."""
    m = load(proj / "mvf.json")
    s = load(cd / m["script"])
    return [p for p, c in (s.get("platforms") or {}).items()
            if p in PLATFORMS and (c or {}).get("canvas", PLATFORMS[p]["canvas"]) == m["canvas"]]


def cmd_export(a):
    from mvflib.export import export
    cd = campaign_dir(a)
    for pj in a.proj:
        proj = proj_path(cd, pj)
        plats = a.platforms.split(",") if a.platforms else project_platforms(cd, proj)
        for p in plats:
            if p not in PLATFORMS or p in {"canvases", "speech_wps", "_about"}:
                raise SystemExit(f"unknown platform {p}")
        if not plats:
            raise SystemExit(f"{proj.name}: no script platform uses canvas {load(proj / 'mvf.json')['canvas']}; pass --platforms")
        rep = export(cd, proj, plats, draft=a.draft, skip_render=a.skip_render)
        rep.print(f"QA {proj.name}")
        print(f"delivered → {cd / '08-delivery'}  (listen and watch every file before it is called done; nothing was uploaded)")
        if rep.fails:
            sys.exit(1)


def cmd_viewer(a):
    from mvflib import viewer
    cd = campaign_dir(a)
    proj = proj_path(cd, a.proj)
    out = viewer.brief(cd, proj)
    n = out.stem.rsplit("-v", 1)[1]
    print(f"{out}\ngive it to a fresh helper (no other context: not the script, not the plan); it writes "
          f"{proj / 'review' / f'viewer-v{n}.md'}. Record each red item in review.json, fix, re-export.")


def persons(cd: Path) -> list[str]:
    """Names of the real people in the material pack: an art prompt never names them."""
    return [it.get("title", "") for it in material.load_pack(cd)["items"].values() if it.get("kind") == "person"]


def cmd_art(a):
    from mvflib import art
    cd = campaign_dir(a)
    if a.action == "ls":
        if not (cd / art.ART / "art.json").is_file():
            print(f"no {art.ART / 'art.json'} (references/07-production.md#art)")
            return
        for x in art.status(cd):
            print(f"  {x['id']:10} {x['use']:10} {x['look']:12} drafts {x['drafts']} · "
                  + (f"approved {x['verdict']}" + (f" ({x['upscale']}x)" if x['upscale'] else "") if x["approved"] else "not approved"))
        name, eng = art.engine_for(cd, a.engine)
        print(f"engine {name}: {eng.about}" + (" · PAID (--spend)" if eng.paid else ""))
        return
    if a.action == "check":
        rep = art.check(cd, persons(cd))
        rep.print("art")
        sys.exit(1 if rep.fails else 0)
    if a.action == "draw":
        rep = art.draw(cd, a.ids, a.engine, a.seed, a.spend, persons(cd))
        rep.print("art draw")
        print("next: open each draft and look at it (anatomy, text, artifacts, likeness), then mvf art approve ID")
        sys.exit(1 if rep.fails else 0)
    if len(a.ids) != 1:
        raise SystemExit("mvf art approve ID [--draft PATH]")
    fx, fy = (float(v) for v in a.focus.split(","))
    rec = art.approve(cd, a.ids[0], a.draft, (fx, fy), a.force, a.upscaler)
    print(f"{rec['verdict']:4} {rec['id']}: {rec['drawn']} → {rec.get('shown', 'native')}"
          + (f" ({rec['upscale']}x, {rec.get('upscaler')})" if rec.get("upscale") else "") + f" → {art.ART / (rec['id'] + '.png')}"
          + ("  FORCED past the upscale gate" if rec.get("forced") else ""))
    print("re-run mvf scaffold so projects pick it up (assets/art/)")


def cmd_thumbnail(a):
    from mvflib.thumbnail import thumbnail
    cd = campaign_dir(a)
    out, rep = thumbnail(cd, a.sid, a.canvas, a.art, a.text)
    rep.print(f"thumbnail {out.name}")
    print(f"{out.relative_to(cd)}  (edit {Path('07-video') / a.sid / f'thumb-{a.canvas}' / 'index.html'} and re-run; look at it at 320 px wide)")
    sys.exit(1 if rep.fails else 0)


def cmd_qa(a):
    from mvflib.export import qa
    cd = campaign_dir(a)
    proj = proj_path(cd, a.proj)
    rep = qa(Path(a.video).resolve(), a.platform, load(proj / "timeline.json"))
    rep.print(f"QA {Path(a.video).name} for {a.platform}")
    sys.exit(1 if rep.fails else 0)


def cmd_doctor(a):
    from mvflib import doctor
    sys.exit(doctor.run(maybe_campaign(a), a.json, a.quick))


def cmd_judge(a):
    from mvflib import judge
    if a.action in ("status", "setup"):
        if a.action == "setup" and a.provider and a.model:
            judge.setup(a.provider, a.model, a.effort, a.soot_forge)
            return
        if a.action == "setup" and (a.provider or a.model):
            raise SystemExit("--provider and --model go together: the user names both on first use")
        st = judge.status()
        judge.print_status(st)
        sys.exit(0 if not st["problems"] else judge.NEEDS_CONTEXT)
    cd = campaign_dir(a)
    c = load(cd / "campaign.json")
    plat = a.platform or (c.get("platforms") or ["tiktok"])[0]
    if plat not in PLATFORMS:
        raise SystemExit(f"unknown platform {plat}")
    if a.action == "scripts":
        items, manifests = [], {}
        for kind, names in (("candidate", a.names), ("control", a.control or [])):
            for n in names:
                sid, _, h = n.partition("/")
                s = load(script_path(cd, sid))
                t = pick_take(cd, s, "latest") if takes(cd, s["id"]) else None
                manifests[s["id"]] = timing.load_manifest(t) if t else None
                items.append((s, h or s.get("hook") or "H1", kind))
        if not a.control:
            print("WARN no --control: add a script the team already rejected; without a known loser the panel's "
                  "enthusiasm cannot be told from a good script")
        bench = [load(cd / "06-pretest/bench" / f"{b}.json") for b in a.bench or []]
        judge.judge_scripts(cd, items, a.round, a.size, plat, manifests, bench, a.dry, not a.no_interview)
    elif a.action == "video":
        if len(a.names) != 1:
            raise SystemExit("mvf judge video <proj>")
        judge.judge_video(cd, proj_path(cd, a.names[0]), a.round if a.round != "r1" else "v1", a.size, plat, a.dry)
    elif a.action in ("resume", "collect"):
        out = Path(a.names[0]).resolve() if a.names else cd / "06-pretest" / a.round / "microfish"
        (judge.resume if a.action == "resume" else judge.collect)(cd, out)


def main():
    ap = argparse.ArgumentParser(prog="mvf", description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--campaign", help="campaign dir (default: the one containing the cwd)")
    sub = ap.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("init")
    p.add_argument("slug")
    p.add_argument("--product", required=True)
    p.add_argument("--root")
    p.add_argument("--format", help="start from a saved format (`mvf format ls`)")
    p.set_defaults(fn=cmd_init)
    p = sub.add_parser("format", help="save a series' look by name, or list saved formats")
    p.add_argument("action", choices=["save", "ls"])
    p.add_argument("name", nargs="?")
    p.add_argument("--from", dest="sid", help="the script whose direction to keep")
    p.add_argument("--force", action="store_true")
    p.add_argument("--root", help="campaigns folder for `ls` outside a campaign")
    p.set_defaults(fn=cmd_format)
    sub.add_parser("status").set_defaults(fn=cmd_status)
    p = sub.add_parser("next", help="the one next action and who takes it (agent or owner)")
    p.add_argument("--json", action="store_true")
    p.set_defaults(fn=cmd_next)
    p = sub.add_parser("sheet", help="the script as the owner reads it (06-pretest/sheet-<sid>.md)")
    p.add_argument("sid")
    p.set_defaults(fn=cmd_sheet)
    p = sub.add_parser("approve", help="record the owner's word on a script")
    p.add_argument("sid")
    p.add_argument("--verdict", required=True, choices=["approved", "rejected", "delegated"])
    p.add_argument("--note", help="the owner's words (required for rejected and delegated)")
    p.add_argument("--hook", help="the hook they approved (default: the script's hook)")
    p.add_argument("--thesis", type=int, help="the thesis wording they picked (1-based, as numbered on the sheet)")
    p.set_defaults(fn=cmd_approve)
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
    p.add_argument("stage", nargs="?", default="all", choices=["all", "brief", "material", "product", "audience", "trends", "research", "strategy", "art", "pretest"])
    p.set_defaults(fn=cmd_check)
    p = sub.add_parser("new-script")
    p.add_argument("sid")
    p.add_argument("--angle", help="the angle it is written from (04-strategy/angles.json id); fills angle and format")
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
    p = sub.add_parser("top-videos")
    p.add_argument("query", nargs="*", help="search words as the audience types them (logged out)")
    p.add_argument("--url", nargs="*", default=[], help="videos to transcribe directly (TikTok, YouTube, X)")
    p.add_argument("--n", type=int, default=6, help="how many of the most-viewed results to transcribe")
    p.add_argument("--seconds", type=float, default=300, help="how much of each transcript to keep (0 = all)")
    p.add_argument("--max-min", type=float, default=30, help="skip search results longer than this many minutes "
                   "(compilations, livestreams); 0 = no limit")
    p.add_argument("--scope", choices=["topic", "benefit"], default="topic",
                   help="topic = videos about the subject; benefit = videos that give this audience the same kind of benefit")
    p.add_argument("--platform", choices=["youtube", "tiktok"], default="youtube",
                   help="where to search; tiktok prints the browse steps for the agent (mvf cannot search it)")
    p.set_defaults(fn=cmd_top_videos)
    p = sub.add_parser("pretest-brief")
    p.add_argument("script", nargs="+")
    p.add_argument("--round", default="r1")
    p.add_argument("--size", default="quick", choices=["quick", "standard", "deep"])
    p.add_argument("--hooks", nargs="*", help="limit to sid/H1 sid/H2 …")
    p.add_argument("--question")
    p.set_defaults(fn=cmd_pretest_brief)
    sub.add_parser("predict", help="brief for a fresh helper: the 5 videos anyone would make from this brief"
                   ).set_defaults(fn=cmd_predict)
    sub.add_parser("overlap", help="brief for a fresh helper: mark each angle CLEAR/SHARES/OVERLAP"
                   ).set_defaults(fn=cmd_overlap)
    p = sub.add_parser("coldread")
    p.add_argument("script")
    p.add_argument("--hooks", nargs="*", help="H1 H2 … (default: all)")
    p.add_argument("--round", default="r1")
    p.add_argument("--readers", type=int, default=3)
    p.set_defaults(fn=cmd_coldread)
    p = sub.add_parser("feed")
    p.add_argument("script", nargs="+", help="candidate scripts, sid or sid/H2")
    p.add_argument("--control", nargs="*", help="a script already rejected (the known loser)")
    p.add_argument("--round", default="r1")
    p.add_argument("--readers", type=int, default=4, help="one shuffled feed per reader")
    p.add_argument("--seed", type=int, default=1)
    p.set_defaults(fn=cmd_feed)
    p = sub.add_parser("voice")
    p.add_argument("script")
    p.add_argument("--engine", help="voice engine (default: voice.engine, campaign voice_engine, config, files)")
    p.add_argument("--opt", action="append", help="engine option key=value (repeatable; `mvf extensions` lists them)")
    p.add_argument("--all", action="store_true", help="re-voice every line, not only changed ones")
    p.add_argument("--rehear", action="store_true", help="re-time the words of the latest take from its audio (same audio)")
    p.add_argument("--owner-skip", metavar="REASON", help="voice before the owner read the script or the pre-test picked it (a test take, a demo); the reason is recorded in 06-pretest/owner.json")
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
    p.set_defaults(fn=cmd_voice_pick)
    p = sub.add_parser("art")
    p.add_argument("action", nargs="?", default="ls", choices=["ls", "check", "draw", "approve"])
    p.add_argument("ids", nargs="*", help="job ids (draw: default every job with no draft yet)")
    p.add_argument("--engine", help="image engine (default: art.json engine, else qwen)")
    p.add_argument("--seed", type=int, help="a redraw is a new seed; drafts are never overwritten")
    p.add_argument("--spend", action="store_true", help="allow a paid engine: only when the user said yes to this spend")
    p.add_argument("--draft", help="approve this draft (default: the newest)")
    p.add_argument("--focus", default="0.5,0.5", help="crop focus x,y in 0..1 when the aspect differs")
    p.add_argument("--force", action="store_true", help="approve past the upscale gate (after looking at it)")
    p.add_argument("--upscaler", choices=["esrgan", "blend", "lanczos"],
                   help="approve: esrgan (default for illustration when installed), blend, or lanczos")
    p.set_defaults(fn=cmd_art)
    p = sub.add_parser("thumbnail")
    p.add_argument("sid")
    p.add_argument("--canvas", default="16x9", choices=["16x9", "9x16", "1x1"])
    p.add_argument("--art", help="approved art id for the plate (default: the approved thumbnail job on this canvas)")
    p.add_argument("--text", help="the line (default: youtube-long thumbnail_text, else a cover_text)")
    p.set_defaults(fn=cmd_thumbnail)
    p = sub.add_parser("scaffold")
    p.add_argument("script")
    p.add_argument("--hook", help="the one hook this video ships with (default: the script's `hook`, else H1)")
    p.add_argument("--canvas", help="9x16,1x1,… (default: the script's platform canvases)")
    p.add_argument("--take", default="latest", help="latest | none | tNN")
    p.add_argument("--from", dest="from_", help="copy authored scenes from another project")
    p.set_defaults(fn=cmd_scaffold)
    p = sub.add_parser("score")
    p.add_argument("proj")
    p.add_argument("--bed", help="music bed: a track you own or licensed, or auto:<mood> (procedural)")
    p.add_argument("--none", action="store_true", help="no bed (removes a previous one)")
    p.add_argument("--sfx", default="auto", choices=["auto", "none"], help="render sfx.json cues (default) or drop them")
    p.add_argument("--seed", type=int, default=7, help="auto bed seed: another seed, another melody")
    p.set_defaults(fn=cmd_score)
    p = sub.add_parser("export")
    p.add_argument("proj", nargs="+")
    p.add_argument("--platforms", help="comma list (default: the script's platforms on this project's canvas)")
    p.add_argument("--draft", action="store_true", help="rough cut: allows placeholders and estimated timing")
    p.add_argument("--skip-render", action="store_true", help="reuse renders/master.mp4")
    p.set_defaults(fn=cmd_export)
    p = sub.add_parser("viewer", help="brief for a fresh helper who sees only the cut (1 fps frames + the voice)")
    p.add_argument("proj")
    p.set_defaults(fn=cmd_viewer)
    p = sub.add_parser("doctor", help="is this machine ready: HyperFrames, OmniVoice, Qwen+RMBG, Micro Fish")
    p.add_argument("--json", action="store_true")
    p.add_argument("--quick", action="store_true", help="skip verifying the image weights")
    p.set_defaults(fn=cmd_doctor)
    p = sub.add_parser("judge", help="Micro Fish (Soot) judges the scripts or the cut")
    p.add_argument("action", choices=["status", "setup", "scripts", "video", "resume", "collect"])
    p.add_argument("names", nargs="*", help="scripts: sid or sid/H2 …; video: proj; resume/collect: the microfish dir")
    p.add_argument("--provider", help="setup: the Soot provider the user chose (soot providers)")
    p.add_argument("--model", help="setup: the model the user chose")
    p.add_argument("--effort", help="setup: reasoning effort, if the model takes one")
    p.add_argument("--soot-forge", help="setup: install society-sim from this soot-forge checkout instead of the release")
    p.add_argument("--control", nargs="*", help="scripts: one script the team already rejected (the known loser)")
    p.add_argument("--bench", nargs="*", help="scripts: 06-pretest/bench/<id>.json real videos to rank against")
    p.add_argument("--round", default="r1")
    p.add_argument("--size", default="quick", choices=["quick", "standard", "deep"])
    p.add_argument("--platform", help="the feed the crowd scrolls (default: the campaign's first platform)")
    p.add_argument("--dry", action="store_true", help="write the brief and key only; run nothing")
    p.add_argument("--no-interview", action="store_true", help="skip the follow-up retell interview")
    p.set_defaults(fn=cmd_judge)
    p = sub.add_parser("qa")
    p.add_argument("video")
    p.add_argument("--platform", required=True, choices=list(k for k in PLATFORMS if k not in {"canvases", "speech_wps", "_about"}))
    p.add_argument("--proj", required=True)
    p.set_defaults(fn=cmd_qa)
    a = ap.parse_args()
    a.fn(a)


if __name__ == "__main__":
    main()
