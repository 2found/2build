"""Pre-test brief: the real material verbatim (every hook variant with its measured
or estimated timestamps, the on-screen text, the visuals and the platform copy),
the audience segments as researched, and the decision. Hand it to a simulated
panel (any persona-simulation tool) or read it to real people."""
from __future__ import annotations

from pathlib import Path

from . import timing
from .common import PLATFORMS, load

SIZES = {"quick": 24, "standard": 48, "deep": 96}


def _script_block(script: dict, hook: str, take: Path | None) -> str:
    sv = timing.variant(script, hook)
    tl = timing.build(sv, timing.load_manifest(take) if take else None)
    lines = [f"### Variant {script['id']}/{hook} — {tl['duration']:.1f}s ({tl['timing']} timing), format {script.get('format')}"]
    for b, t in zip(sv["beats"], tl["beats"]):
        vo = (b.get("vo") or "").strip() or "(no voice)"
        on = (b.get("onscreen") or "").strip()
        lines.append(f"- [{t['start']:05.2f}–{t['end']:05.2f}] {b.get('role')}: VOICE \"{vo}\""
                     + (f" | ON-SCREEN \"{on}\"" if on else "") + f" | PICTURE: {(b.get('visual') or '').strip()}")
    for p, c in (script.get("platforms") or {}).items():
        if p == "tiktok" and c.get("caption"):
            lines.append(f"- TikTok caption: \"{c['caption'].strip()}\"")
        if p == "x" and c.get("post"):
            lines.append(f"- X post: \"{c['post'].strip()}\"")
            for i, t in enumerate(c.get("thread") or [], 1):
                lines.append(f"  - reply {i}: \"{t.strip()}\"")
    return "\n".join(lines)


def brief(campaign_dir: Path, scripts: list[Path], hooks: dict[str, list[str]], size: str, question: str | None,
          takes: dict[str, Path | None]) -> str:
    c = load(campaign_dir / "campaign.json")
    segs = load(campaign_dir / "02-audience/segments.json").get("segments", [])
    facts = load(campaign_dir / "01-product/facts.json")
    blocks, ids = [], []
    for sp in scripts:
        s = load(sp)
        for h in hooks.get(s["id"]) or ["H1"] + [v["id"] for v in s.get("variants") or []]:
            blocks.append(_script_block(s, h, takes.get(s["id"])))
            ids.append(f"{s['id']}/{h}")
    plats = ", ".join(PLATFORMS[p]["label"] for p in c.get("platforms", []))
    seg_txt = []
    for x in sorted(segs, key=lambda x: x.get("priority", 9)):
        pl = "; ".join(f"{k}: {', '.join(v.get('habits') or [])}" for k, v in (x.get("platforms") or {}).items())
        seg_txt.append(f"- {x['id']} (priority {x.get('priority')}) {x.get('name')}: {x.get('who')}. Pains: "
                       f"{'; '.join(x.get('pains') or [])}. Platform habits: {pl}")
    q = question or (f"Which video ({' vs '.join(ids)}) holds viewers longest, makes them want to share it and "
                     f"understand {facts.get('one_liner', 'the product')} correctly? Which one gets swiped in the first "
                     f"2 seconds, and why?")
    return f"""# Short-video pre-test — {c.get('product')} ({c.get('slug')})

**Decision to make:** {q}

**Content:** short video (kind: video) on {plats}. Measure per person: stops scrolling in the first second
(attention), watches to the end (watch_through), wants to share/save (share), and the comment they would write.
Viewers scroll a feed and most watch X muted: judge each variant on its on-screen text alone too.

**Markets:** {', '.join(c.get('markets') or [])} · **Languages:** {', '.join(c.get('languages') or [])}
**Size:** {size} ({SIZES[size]} personas)

## Audience (from research, verbatim)
{chr(10).join(seg_txt)}

## Product (one line)
{facts.get('one_liner', '')}

## Variants (verbatim, with timestamps)
{chr(10).join(blocks)}

## Report
- Rank the variants by attention in the first 2 seconds and by watch_through; say whether the gap is clear.
- For each variant: the second viewers leave and the line that made them leave; the first comment they would write.
- For each variant: what a viewer who leaves at 3 s and at 6 s got from it; who they would send it to and with
  what words; whether they can name the product without the CTA.
- Experts (a TikTok creator, an X growth marketer, someone in the priority-1 segment) score the hook, how much it
  feels like an ad, how clear the value is, and suggest concrete wording fixes.
- Which lines sound like an ad or are hard to believe.
"""
