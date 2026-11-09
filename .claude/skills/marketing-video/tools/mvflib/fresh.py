"""Is an angle fresh? Two fresh-context helpers answer it. `mvf predict` writes a brief with only the campaign
brief's words: the helper lists the five videos anyone would make from it. `mvf overlap` writes a brief with
the angles, that list and this owner's past videos: the helper marks each angle CLEAR / SHARES / OVERLAP.
Freshness is about the angle and format; the benefit may be a known one. All advice is WARN."""
from __future__ import annotations

import json
from pathlib import Path

from .common import Report, load

PREDICT = "04-strategy/predict.md"
OVERLAP = "04-strategy/overlap.json"
VERDICTS = {"CLEAR", "SHARES", "OVERLAP"}
BUDGET = ("Budget: at most 15 tool calls. Write the answer to the file named below and reply with at most "
          "6 lines: the file path and one line per item.")


def _cut(s, n=160) -> str:
    s = " ".join(str(s or "").split())
    return s if len(s) <= n else s[:n - 1] + "…"


def predict_brief(c: dict) -> str:
    rows = [f"- product: {c.get('product', '')}", f"- goal: {c.get('goal', '')}",
            f"- what the viewer gets: {c.get('benefit', '')}", f"- format asked for: {_cut(c.get('format'), 300)}",
            f"- platforms: {', '.join(c.get('platforms') or [])}", f"- markets: {', '.join(c.get('markets') or [])}",
            f"- language: {', '.join(c.get('languages') or [])}"]
    # the subject (a novel, a feature) is usually only in the owner's notes; without it the helper predicts
    # videos about the product in general
    rows += [f"- note: {_cut(x, 240)}" for x in (c.get("constraints") or [])[:8]]
    return f"""# Prediction test

You are a working short-video creator. Here is a campaign brief, nothing else:

{chr(10).join(rows)}

List the 5 videos anyone in this niche would make first from this brief: the obvious ones, not the clever
ones. For each: a title, the hook line, the format, and the one thing it says. Number them P1–P5.

{BUDGET}
Write them to `{PREDICT}` (inside the campaign folder you were given).
"""


def past_videos(cd: Path, limit: int = 60) -> list[str]:
    """What this owner already made: sibling campaigns' angles and script payoffs, plus campaign.past."""
    out = []
    c = load(cd / "campaign.json") if (cd / "campaign.json").is_file() else {}
    out += [f"(this owner) {_cut(x)}" for x in c.get("past") or []]
    def product(d):
        try:
            return load(d / "campaign.json").get("product")
        except (ValueError, OSError, AttributeError):
            return None
    sibs = [p for p in cd.parent.iterdir() if p.is_dir() and p != cd and not p.name.startswith("_")]
    mine = set(cd.name.split("-"))
    # the same product's campaigns first, then the same topic (shared slug words), then the newest: those are
    # the videos this audience has most likely seen from this owner
    for sib in sorted(sibs, key=lambda d: (product(d) != c.get("product"), -len(mine & set(d.name.split("-"))),
                                           -d.stat().st_mtime)):
        ap = sib / "04-strategy/angles.json"
        try:
            angles = load(ap).get("angles") or [] if ap.is_file() else []
        except (ValueError, OSError, AttributeError):
            angles = []
        for g in angles if isinstance(angles, list) else []:
            if not isinstance(g, dict):
                continue
            out.append(f"{sib.name}/{g.get('id')} (angle): {_cut(g.get('promise'))} [{g.get('format', '')}]")
        for sp in sorted((sib / "05-scripts").glob("s*.json")) if (sib / "05-scripts").is_dir() else []:
            try:
                s = load(sp)
            except (ValueError, OSError):
                continue
            if not isinstance(s, dict):
                continue
            pay = (s.get("spine") or {}).get("payoff") or s.get("value")
            if pay:
                # a script that never reached delivery was never seen by anyone
                made = "made" if (sib / "08-delivery" / sp.stem).is_dir() else "script only, never made"
                out.append(f"{sib.name}/{s.get('id')} ({made}): {_cut(pay)}")
        if len(out) >= limit:
            break
    return out[:limit]


def overlap_brief(cd: Path) -> str:
    a = load(cd / "04-strategy/angles.json").get("angles") or []
    rows = [f"- {g.get('id')}: {_cut(g.get('promise'), 220)} [format {g.get('format', '')}]; hooks: "
            f"{' / '.join(_cut(h, 80) for h in (g.get('hooks') or [])[:3])}" for g in a]
    pred = (cd / PREDICT).read_text(encoding="utf-8").strip() if (cd / PREDICT).is_file() else \
        "(no prediction yet: run `mvf predict` first and give it to a fresh helper)"
    past = past_videos(cd)
    ex = {g.get("id", "G1"): {"verdict": "CLEAR|SHARES|OVERLAP", "with": ["P2", "<campaign>/<id>"],
                              "what": "what is the same: the angle, the format, the hook"} for g in a[:1]}
    return f"""# Overlap check

You judge whether each new angle is one the viewer has already seen. Compare each angle with the obvious
videos below and with what this owner already made. The same benefit is fine; the same angle or format is not.

CLEAR: nothing below makes the same point the same way. SHARES: a video below shares the topic or format but
not the point. OVERLAP: a video below makes the same point in the same way.

## New angles
{chr(10).join(rows) or '(none)'}

## The obvious videos (prediction test)
{pred}

## This owner's past videos
Only an item marked `made` was posted. An `angle` or a `script only, never made` is a plan nobody saw: it
cannot make a new angle OVERLAP, though a new angle that repeats it is worth a note.
{chr(10).join('- ' + x for x in past) or '(none found)'}

{BUDGET}
Write `{OVERLAP}` (inside the campaign folder you were given) as JSON, one key per angle id:
{json.dumps(ex, ensure_ascii=False)}
"""


def check(cd: Path, angles: list[dict]) -> Report:
    r = Report()
    p = cd / OVERLAP
    marks = None
    if p.is_file():
        try:
            marks = load(p)
            if not isinstance(marks, dict):
                raise ValueError("not an object")
        except ValueError as e:
            r.warn("OV1", OVERLAP, f"unreadable ({e}): it is " '{"G1": {"verdict": "CLEAR|SHARES|OVERLAP", '
                   '"with": [...], "what": "..."}}; rerun `mvf overlap` with a fresh helper')
            return r
    for g in angles:
        w = f"angles.{g.get('id')}"
        if not str(g.get("swap") or "").strip():
            r.warn("SW1", w, "swap: put the closest competitor in place of the product; write what is left. If the "
                   "video still works, the angle is about the category, not about this product")
        if marks is None:
            continue
        m = marks.get(g.get("id"))
        v = str((m or {}).get("verdict", "")).upper() if isinstance(m, dict) else ""
        if v not in VERDICTS:
            r.warn("OV1", w, f"{OVERLAP} has no CLEAR/SHARES/OVERLAP verdict for this angle: rerun `mvf overlap`")
        elif v == "OVERLAP" and not str(g.get("overlap_note") or "").strip():
            r.warn("OV2", w, f"overlaps {m.get('with') or '?'} ({_cut(m.get('what'), 100)}): change the angle or "
                   "format, or say in overlap_note why it is still worth making")
    if marks is None:
        r.warn("OV1", "angles", f"no {OVERLAP}: run `mvf predict`, then `mvf overlap`, each read by a fresh helper, "
               "to see whether these angles are the ones anyone would make")
    return r
