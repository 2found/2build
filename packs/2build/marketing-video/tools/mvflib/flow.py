"""The way through a campaign for someone who only wants the video: `mvf next` names the one next action and
who takes it (the agent, or the owner at one of their three touchpoints: the brief, the script, the cut);
`mvf sheet` is the script as the owner reads it; `mvf approve` records the owner's word on it."""
from __future__ import annotations

import hashlib
from pathlib import Path

from .common import PLATFORMS, STAGES, dump, load, today

REF = {"brief": "00-brief-material.md", "material": "00-brief-material.md", "product": "01-product.md",
       "audience": "02-audience.md", "trends": "03-trends.md", "research": "03b-research.md",
       "strategy": "04-strategy.md"}
OWNER = "06-pretest/owner.json"


def sha(p: Path) -> str:
    return hashlib.sha256(p.read_bytes()).hexdigest()[:16]


def is_control(sid: str) -> bool:
    return sid.startswith("ctl-")


def script_ids(cd: Path) -> tuple[list[str], list[str]]:
    ids = sorted(p.stem for p in (cd / "05-scripts").glob("*.json")) if (cd / "05-scripts").is_dir() else []
    return [i for i in ids if not is_control(i)], [i for i in ids if is_control(i)]


def decision(cd: Path) -> tuple[Path, dict] | None:
    """The latest pre-test decision that names a winner (by decided_at, then file time)."""
    ds = [p for p in (cd / "06-pretest").glob("*/decision.json")] if (cd / "06-pretest").is_dir() else []
    rows = []
    for p in ds:
        d = load(p)
        if d.get("winner") or d.get("method") == "skipped":
            rows.append((str(d.get("decided_at") or ""), p.stat().st_mtime, p, d))
    if not rows:
        return None
    _, _, p, d = max(rows, key=lambda r: (r[0], r[1]))
    return p, d


def owner(cd: Path) -> dict:
    p = cd / OWNER
    return load(p) if p.is_file() else {"approvals": []}


def owner_verdict(cd: Path, sid: str) -> dict | None:
    rows = [a for a in owner(cd).get("approvals") or [] if a.get("script") == sid]
    return rows[-1] if rows else None


def approve(cd: Path, sid: str, verdict: str, note: str, hook: str | None) -> dict:
    sp = cd / "05-scripts" / f"{sid}.json"
    if not sp.is_file():
        raise SystemExit(f"no script {sid}")
    if verdict == "rejected" and not note.strip():
        raise SystemExit("a rejection carries the owner's reason (--note): it is what the rewrite works from")
    rec = {"script": sid, "hook": hook or load(sp).get("hook") or "H1", "verdict": verdict, "note": note,
           "sha": sha(sp), "at": today().isoformat()}
    o = owner(cd)
    o.setdefault("approvals", []).append(rec)
    dump(cd / OWNER, o)
    return rec


def owner_gate(cd: Path, sid: str, skip: str | None) -> str | None:
    """None when production may start; else why not. The owner reads a campaign's first script; once one video
    is delivered they have seen the voice of the campaign, and later scripts wait only on a rejection."""
    if skip and skip.strip():
        return None
    v = owner_verdict(cd, sid)
    delivered = (cd / "08-delivery").is_dir() and any(p.is_dir() for p in (cd / "08-delivery").iterdir())
    if v is None and delivered:
        return None
    if v is None:
        return (f"the owner has not read {sid}: `mvf sheet {sid}` is the page they read; record their word with "
                f"`mvf approve {sid} --verdict approved|rejected|delegated --note …` (or --owner-skip REASON)")
    if v["verdict"] == "rejected":
        return f"the owner rejected {sid} on {v['at']}: {v['note']}. Rewrite it, pre-test again, ask again"
    return None


def owner_drift(cd: Path, sid: str) -> str | None:
    v = owner_verdict(cd, sid)
    sp = cd / "05-scripts" / f"{sid}.json"
    if v and v["verdict"] != "rejected" and sp.is_file() and v.get("sha") and v["sha"] != sha(sp):
        return f"{sid} changed after the owner's {v['verdict']} on {v['at']}: if a line they read changed, ask again"
    return None


# ---------- the sheet ----------

def sheet(cd: Path, sid: str) -> str:
    from . import timing
    c = load(cd / "campaign.json")
    vi = (c.get("languages") or ["en"])[0].startswith("vi")
    L = (lambda v, e: v if vi else e)
    s = load(cd / "05-scripts" / f"{sid}.json")
    hook = s.get("hook") or "H1"
    dec = decision(cd)
    if dec and str(dec[1].get("winner") or "").startswith(sid + "/"):
        hook = dec[1]["winner"].split("/", 1)[1]
    sv = timing.variant(s, hook)
    tl = timing.build(sv)
    sp = s.get("spine") or {}
    out = [f"# {sid} · {hook} · ~{tl['duration']:.0f} s · {s.get('format', '')}", ""]
    out += [f"**{L('Người xem được gì', 'What the viewer gets')}:** {sp.get('use') or s.get('value', '')}",
            f"**{L('Người lạ kể lại được', 'A stranger retells')}:** {sp.get('takeaway', '')}", ""]
    out += [f"| s | {L('Hình', 'Picture')} | {L('Chữ', 'Text')} | {L('Lời', 'Voice')} |", "| --- | --- | --- | --- |"]
    for b, t in zip(sv["beats"], tl["beats"]):
        pic = (b.get("visual") or "").strip().replace("|", "/")
        pic = pic if len(pic) <= 110 else pic[:109] + "…"
        out.append(f"| {t['start']:.0f}–{t['end']:.0f} | {pic} | {(b.get('onscreen') or '').strip().replace(chr(10), ' / ')} | "
                   f"{(b.get('vo') or '').strip()} |")
    for p, pc in (s.get("platforms") or {}).items():
        if p in PLATFORMS and isinstance(pc, dict):
            text = pc.get("caption") or pc.get("post") or pc.get("title")
            if text:
                out.append(f"\n**{PLATFORMS[p]['label']}:** {text}")
    won = bool(dec and str(dec[1].get("winner") or "").startswith(sid + "/"))
    if dec and not won:
        out.append(f"\n**Pre-test ({dec[0].parent.name}):** {L('kịch bản thắng là', 'the winner is')} {dec[1].get('winner')}")
    mf = dec[0].parent / "microfish" / "judge.json" if won else None
    if won and mf.is_file():
        out += _panel_table(load(mf), sid, L)
    elif won:
        out.append(f"\n**Pre-test ({dec[0].parent.name}, {dec[1].get('method')}):** {dec[1].get('evidence', '')}")
    if won and dec[1].get("retell"):
        out.append(f"\n**{L('Người xem đã kể lại', 'What viewers retold')}:** {dec[1]['retell']}")
    if won and dec[1].get("changes"):
        out.append(f"\n**{L('Đã/sẽ sửa theo pre-test', 'Changes from the pre-test')}:**")
        out += [f"- {x}" for x in dec[1]["changes"]]
    if mf and mf.is_file():
        for x in (load(mf).get("stimuli") or {}).values():
            if x.get("id", "").startswith(sid + "/") and x.get("expert_blocking"):
                out.append(f"\n**{L('Chuyên gia lưu ý', 'Experts flagged')}:**")
                out += [f"- {e}" for e in x["expert_blocking"]]
    out.append("\n" + L(f"Duyệt: `mvf approve {sid} --verdict approved` · không được: `--verdict rejected --note \"lý do\"` · "
                        f"giao cho agent tự quyết: `--verdict delegated`",
                        f"Approve: `mvf approve {sid} --verdict approved` · no: `--verdict rejected --note \"why\"` · "
                        f"let the agent decide: `--verdict delegated`"))
    return "\n".join(out) + "\n"


def _panel_table(j: dict, sid: str, L) -> list[str]:
    """The simulated panel in one small table: this script in bold, the control named, so the owner sees the
    margin without reading the evidence string."""
    pct = lambda v: "—" if v is None else f"{v:.0f}%"
    out = [f"\n**{L('Khán giả giả lập', 'Simulated panel')} (Micro Fish, n={next(iter(j['stimuli'].values()))['n']}):** "
           + (L("dẫn rõ", "a clear lead") if j.get("clear") else L("dẫn nhưng chưa chắc (khoảng tin cậy chồng nhau)",
                                                                  "leads, not clearly (intervals overlap)"))
           + (L(", hơn bản đối chứng", ", beats the control") if j.get("beats_control")
              else L(", KHÔNG hơn bản đối chứng", ", does NOT beat the control")), "",
           f"| {L('Kịch bản', 'Script')} | {L('Chọn', 'Preferred')} | {L('Xem hết', 'Watched')} | {L('Hiểu', 'Clear')} | "
           f"{L('Muốn xem tiếp', 'Intent')} | {L('Thấy như quảng cáo', 'Felt like an ad')} |", "| --- | ---: | ---: | ---: | ---: | ---: |"]
    for x in sorted(j["stimuli"].values(), key=lambda x: -(x.get("preference_pct") or 0)):
        name = x["id"].split("/")[0]
        name = f"**{name}**" if name == sid else name + (L(" (đối chứng)", " (control)") if x.get("kind") == "control" else "")
        out.append(f"| {name} | {pct(x.get('preference_pct'))} | {pct(x.get('watch_through'))} | {pct(x.get('clarity_top2'))} | "
                   f"{pct(x.get('intent_top2'))} | {x.get('ad_feel', 0)}/{x.get('n')} |")
    return out


# ---------- next ----------

def _act(who, what, run=None, ref=None, why=None):
    return {"who": who, "what": what, "run": run, "ref": ref, "why": why}


def next_action(cd: Path, stage_report, lint_fails) -> dict:
    """The first thing not done, in order. `stage_report(stage)` is mvf's checker (None = not started);
    `lint_fails(sid)` counts a script's lint FAILs."""
    from . import judge
    from .export import open_review
    from .scaffold import placeholders
    c = load(cd / "campaign.json")
    if c.get("_template"):
        return _act("agent", "write the brief into campaign.json from the owner's request (product, goal, benefit, "
                    "markets, languages, platforms); ask the owner once, only for what the request leaves out",
                    ref="references/00-brief-material.md", why="campaign.json still holds the template (`_template`)")
    for st, _ in STAGES[:7]:
        r = stage_report(st)
        if r is None:
            return _act("agent", f"stage {st}", ref=f"references/{REF[st]}")
        if r.fails:
            return _act("agent", f"fix {r.fails} FAIL in stage {st}", run=f"mvf check {st}", ref=f"references/{REF[st]}")
    cands, ctls = script_ids(cd)
    want = int((c.get("budget") or {}).get("scripts") or 3)
    if len(cands) < want:
        return _act("agent", f"write {want - len(cands)} more script(s): {want} different scripts, one video",
                    run="mvf new-script <sid>", ref="references/05-script.md")
    bad = [sid for sid in cands if lint_fails(sid)]  # a control is written to lose: only candidates must pass
    if bad:
        return _act("agent", f"fix lint FAILs in {', '.join(bad)}", run=f"mvf lint {' '.join(bad)}", ref="references/05-script.md")
    dec = decision(cd)
    pre = stage_report("pretest")
    if dec is None:
        if judge.choice() is None:
            return _act("owner", "choose the evaluator: which Soot AI provider and model Micro Fish runs on (once)",
                        run="mvf judge status", ref="references/06-pretest.md#micro-fish",
                        why="first use: the owner signs in (`soot login <provider>`) and names the model")
        ctl = f" --control {ctls[0]}" if ctls else " --control <a rejected script>"
        return _act("agent", "pre-test the scripts on Micro Fish (10-30 min: run it in the background)",
                    run=f"mvf judge scripts {' '.join(cands[:3])}{ctl}", ref="references/06-pretest.md")
    if pre is not None and pre.fails:
        return _act("agent", "finish the pre-test decision (read the answer, write the retell)",
                    run="mvf check pretest", ref="references/06-pretest.md")
    win = str(dec[1].get("winner") or "")
    sid = win.split("/")[0] if win else (cands[0] if cands else "")
    gate = owner_gate(cd, sid, None)
    if gate:
        v = owner_verdict(cd, sid)
        if v and v["verdict"] == "rejected":
            return _act("agent", f"rewrite {sid} from the owner's note", ref="references/05-script.md", why=gate)
        return _act("owner", f"read the winning script {win or sid} and say yes or no",
                    run=f"mvf sheet {sid}", why="production starts only on the owner's word")
    s = load(cd / "05-scripts" / f"{sid}.json")
    vdir = cd / "07-video" / sid
    silent = (s.get("voice") or {}).get("mode") == "none"
    if not silent and not any((vdir / "takes").glob("t[0-9][0-9]/manifest.json")):
        return _act("agent", f"voice {sid} (OmniVoice; a designed voice is fixed once with voice-design → voice-pick)",
                    run=f"mvf voice {sid}", ref="references/07-production.md#tool")
    projs = sorted(p.parent for p in vdir.glob("*/mvf.json")) if vdir.is_dir() else []
    if not projs:
        return _act("agent", f"scaffold {sid} on each canvas", run=f"mvf scaffold {sid}", ref="references/07-production.md")
    for proj in projs:
        rel = proj.relative_to(cd / "07-video")
        holes = placeholders(proj)
        if holes:
            return _act("agent", f"direct and author the scenes {holes} of {rel}", ref="references/07-direction.md")
        if not (proj / "assets/music").is_dir() or not any((proj / "assets/music").iterdir()):
            return _act("agent", f"score {rel}", run=f"mvf score {rel} --bed auto:<mood>", ref="references/07-production.md#tool")
        if open_review(proj) is None:
            return _act("agent", f"review {rel}: draft export, look at the frames, write review.json",
                        run=f"mvf export {rel} --draft", ref="references/07-review.md")
        if open_review(proj):
            return _act("agent", f"fix the open P0 in {rel}/review.json", ref="references/07-review.md")
        if judge.video_judged(proj) is None:
            return _act("agent", f"judge the cut {rel} on Micro Fish (background)", run=f"mvf judge video {rel}",
                        ref="references/06-pretest.md#micro-fish")
    for proj in projs:
        rel = proj.relative_to(cd / "07-video")
        if not (cd / "08-delivery" / sid / proj.name / "delivery.json").is_file():
            return _act("agent", f"export {rel} for its platforms", run=f"mvf export {rel}", ref="references/08-delivery.md")
    return _act("owner", f"watch and listen to every file in 08-delivery/{sid}/, then post it yourself (mvf never "
                "uploads)", ref="references/08-delivery.md")


def print_action(x: dict) -> None:
    tag = "OWNER" if x["who"] == "owner" else "AGENT"
    print(f"next [{tag}]: {x['what']}")
    if x.get("why"):
        print(f"  why:  {x['why']}")
    if x.get("run"):
        print(f"  run:  {x['run']}")
    if x.get("ref"):
        print(f"  read: {x['ref']}")
