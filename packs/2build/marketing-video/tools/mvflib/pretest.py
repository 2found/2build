"""Pre-test brief: the real material verbatim (every hook variant with its measured
or estimated timestamps, the on-screen text, the visuals and the platform copy),
the audience segments as researched, and the decision. Hand it to a simulated
panel (any persona-simulation tool) or read it to real people."""
from __future__ import annotations

from pathlib import Path

from . import timing
from .common import PLATFORMS, copy_kind, load

SIZES = {"quick": 24, "standard": 48, "deep": 96}


def caption_of(script: dict) -> str:
    """The caption a viewer reads under the video: the first caption platform's, else the X post, else the
    YouTube title."""
    pl = script.get("platforms") or {}
    for p, c in pl.items():
        if p in PLATFORMS and copy_kind(p) == "caption" and (c or {}).get("caption"):
            return c["caption"]
    for p, c in pl.items():
        if (c or {}).get("post") or (c or {}).get("title"):
            return c.get("post") or c.get("title")
    return ""


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
        if p not in PLATFORMS:
            continue
        if copy_kind(p) == "caption" and c.get("caption"):
            lines.append(f"- {PLATFORMS[p]['label']} caption: \"{c['caption'].strip()}\"")
        if p == "x" and c.get("post"):
            lines.append(f"- X post: \"{c['post'].strip()}\"")
            for i, t in enumerate(c.get("thread") or [], 1):
                lines.append(f"  - reply {i}: \"{t.strip()}\"")
        if p.startswith("youtube") and c.get("title"):
            lines.append(f"- {PLATFORMS[p]['label']} title: \"{c['title'].strip()}\""
                         + (f" | thumbnail: \"{c['thumbnail_text'].strip()}\"" if c.get("thumbnail_text") else ""))
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


def coldread(script: dict, hook: str, take: Path | None) -> str:
    """What a stranger gets, second by second, and nothing else: no value line, no spine,
    no angle, no product pitch. A fresh-context reader who sees the writer's intent grades
    the intent; one who sees only the feed grades the video."""
    sv = timing.variant(script, hook)
    tl = timing.build(sv, timing.load_manifest(take) if take else None)
    rows = []
    for b, t in zip(sv["beats"], tl["beats"]):
        vo = (b.get("vo") or "").strip()
        on = (b.get("onscreen") or "").strip().replace("\n", " / ")
        rows.append(f"| {t['start']:4.1f}–{t['end']:4.1f} | {(b.get('visual') or '').strip()} | {on} | {vo} |")
    cap = caption_of(script)
    return f"""# Cold read — {script['id']}/{hook} ({tl['duration']:.0f} s, {tl['timing']} timing)

You are scrolling the TikTok For You feed at night, thumb ready, with dozens of videos you like competing for
the next second. You did not ask for this video and know nothing about who made it or why. Read the table
top to bottom as if it plays: PICTURE is what you see, ON SCREEN the text, VOICE what you hear.
Be the viewer, not a critic: most real viewers swipe in the first 2 s, so say so if you would.
You hear each VOICE line once, at speaking pace: do not re-read a line to work out what it meant.

| s | PICTURE | ON SCREEN | VOICE |
| --- | --- | --- | --- |
{chr(10).join(rows)}

Caption under the video: "{cap}"

Answer as JSON only:
{{"swipe_at": <second you swipe, or null if you watch to the end>,
  "why": "the exact line or moment that lost you (or kept you)",
  "after_3s_i_expect": "what you think you will get if you stay",
  "retell": "if you stayed: in one or two sentences, what this video told you, as you would tell a friend;
             empty if you could not say",
  "unclear": ["each word or step you did not follow"],
  "who_or_what": ["each 'he', 'it', 'that' or name you could not place when you heard it"],
  "because_gaps": ["each 'so' or 'because' step you did not see follow from what came before"],
  "ending_premise": "what earlier line the ending rests on, or 'none' if it came from nowhere",
  "hanging": ["each line that raised something the video never answered"],
  "numbers": ["each number as you heard it, once, at speaking pace; mark (?) any you could have heard as another
              number or had lost by the next line"],
  "numbers_dont_fit": ["after it ends, put its numbers side by side once: each total, count or time that does not
                       add up or changes between lines"],
  "who_made_it": "who you think made this and what they want from you, and the second you found out; 'never' if
                  it asks something of you without saying who is asking",
  "tell_a_friend": "the one thing from it you would tell a friend tomorrow (a scene, a twist, a line, a rule, a
                   fact you did not know), in your words; 'nothing' if there is nothing worth retelling",
  "learned_or_felt": "what you walk away with, in your words; empty if nothing",
  "use": "one sentence: what you could do, decide or think differently in your own life after this, or the
          answer you now have to a question you really had; write 'nothing' if it only told you a story or
          facts you did not care about",
  "use_when": "the last time something like that happened to you, or none",
  "save": true|false,
  "dead_seconds": ["s-s: why it dragged"], "best_moment": "s: what",
  "send_to": "who you would send it to and the words you would send it with, or no",
  "comment": "the comment you would write, or none", "reads_like_an_ad": true|false}}
"""


def _script_rows(script: dict, hook: str, take: Path | None) -> tuple[list[tuple], float]:
    sv = timing.variant(script, hook)
    tl = timing.build(sv, timing.load_manifest(take) if take else None)
    return ([(t["start"], t["end"], (b.get("visual") or "").strip(), (b.get("onscreen") or "").strip().replace("\n", " / "),
              (b.get("vo") or "").strip()) for b, t in zip(sv["beats"], tl["beats"])], tl["duration"])


def bench_clip(d: dict) -> dict:
    """A real video, transcribed: {id, url, views, caption, rows: [{s: "0-2.5", picture, onscreen, voice}]}."""
    rows = []
    for x in d.get("rows") or []:
        a, _, b = str(x.get("s", "0-0")).partition("-")
        rows.append((float(a), float(b or a), x.get("picture", ""), x.get("onscreen", ""), x.get("voice", "")))
    return {"id": d["id"], "kind": "bench", "rows": rows, "caption": d.get("caption", ""),
            "duration": rows[-1][1] if rows else 0.0, "views": d.get("views"), "url": d.get("url")}


def script_clip(script: dict, hook: str, take: Path | None, kind: str) -> dict:
    rows, dur = _script_rows(script, hook, take)
    cap = caption_of(script)
    return {"id": f"{script['id']}/{hook}", "kind": kind, "rows": rows, "caption": cap, "duration": dur}


def feed(clips: list[dict], seed: int) -> tuple[str, dict]:
    """A feed of anonymous clips: the candidates, a control the team already rejected, and real videos with known
    views. A reader asked 'would you watch this?' about one clip says yes to almost anything; a reader who ranks it
    against videos people really watched, and against one they really did not, cannot."""
    import random
    order = list(clips)
    random.Random(seed).shuffle(order)
    key, parts = {}, []
    for i, c in enumerate(order):
        lab = chr(ord("A") + i)
        key[lab] = {k: c.get(k) for k in ("id", "kind", "views", "url")}
        rows = "\n".join(f"| {a:4.1f}–{b:4.1f} | {p} | {o} | {v} |" for a, b, p, o, v in c["rows"])
        parts.append(f"## Video {lab} ({c['duration']:.0f} s)\n\n| s | PICTURE | ON SCREEN | VOICE |\n| --- | --- | --- | --- |\n"
                     f"{rows}\n\nCaption: \"{c['caption']}\"\n")
    labs = ", ".join(key)
    text = f"""# Feed — {len(order)} videos

You are scrolling the TikTok For You feed at night. These {len(order)} videos come up one after another. You did
not ask for any of them and know nothing about who made them. Read each table top to bottom as if it plays:
PICTURE is what you see, ON SCREEN the text, VOICE what you hear. Be the viewer, not a critic. Most real
viewers swipe in the first 2 s; say so when you would. You hear each VOICE line once, at speaking pace: do
not re-read a line to work out what it meant. Judge each video against the others, not on its own.

{chr(10).join(parts)}
Answer as JSON only:
{{"videos": {{"<letter>": {{"swipe_at": <second, or null if you watch to the end>, "why": "the line or moment",
               "retell": "in one or two sentences, what it told you, as you would tell a friend; empty if you
                          could not say",
               "unclear": ["each word or step you did not follow"],
               "learned_or_felt": "what you walk away with, or empty",
               "use": "what you could do, decide or think differently in your own life after it, or the
                       answer you now have to a question you really had; 'nothing' if it only told you a
                       story or facts you did not care about",
               "save": true|false, "send_to": "who and the words, or no",
               "comment": "what you would comment, or none"}}}},
  "rank": ["the letters {labs}, the one you would most want to save or send first"],
  "why_first": "why the first one beats the rest", "why_last": "why the last one loses"}}
"""
    return text, key
