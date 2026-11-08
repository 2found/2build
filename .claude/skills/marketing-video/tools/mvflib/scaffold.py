"""Script + timeline → a Hyperframes project, one per canvas, built on the one hook the video ships with.

Generated files (rewritten on every run, never hand-edit): index.html,
compositions/captions.html (unless direction captions.custom keeps a hand-authored one), timeline.json,
STORYBOARD.md, SCENE_GUIDE.md, mvf.json.
Authored files (created once as placeholders, never overwritten):
compositions/<beat>.html — except a body scene copied from another project (--from),
which follows its source until edited here. Switching the project to another hook
sets the old opening scene aside as <beat>.<old hook>.html. Re-running after a re-voice moves every scene slot,
voice clip and caption to the new timing while keeping the scenes' work."""
from __future__ import annotations

import hashlib
import html
import json
import re
import shutil
from pathlib import Path

from . import direction as dirn
from . import timing
from .common import HYPERFRAMES, PLATFORMS, TOOLS, dump, load, words

PLACEHOLDER = 'data-mvf="placeholder"'
GENERATED = 'data-mvf="generated"'
KIT = TOOLS / "templates/kit"
# A warm paper ground and dark ink: the old near-black default made every video a dark-navy void.
DEFAULT_COLORS = {"bg": "#efe7d6", "fg": "#16130f", "accent": "#c2412d", "muted": "#6b6358"}
DEFAULT_FONTS = {"display": "Montserrat", "body": "Inter", "caption": "Montserrat"}


def geometry(canvas: str, platforms: list[str]) -> dict:
    c = PLATFORMS["canvases"][canvas]
    W, H = c["width"], c["height"]
    if canvas == "9x16":
        # The strictest platform UI wins: TikTok's top bar, its right rail, and its caption/handle block. A platform's
        # safe area is measured on its own canvas: a 16x9 YouTube upload says nothing about a vertical frame.
        platforms = [p for p in platforms if PLATFORMS[p].get("canvas") == "9x16"] or ["tiktok"]
        safe = {"top": max(PLATFORMS[p]["safe"]["top"] for p in platforms),
                "bottom": min(PLATFORMS[p]["safe"]["bottom_text"] for p in platforms),
                "right": max(PLATFORMS[p]["safe"]["right_rail"] for p in platforms),
                "left": max(PLATFORMS[p]["safe"]["left"] for p in platforms)}
        cap_top, cap_size, cap_w = safe["bottom"] - 230, 74, 860
    else:
        m = round(min(W, H) * 0.06)
        safe = {"top": m, "bottom": H - m, "right": m, "left": m}
        cap_top = H - m - (190 if canvas == "16x9" else 230)
        cap_size, cap_w = (64, 1500) if canvas == "16x9" else (66, 900)
    return {"W": W, "H": H, "canvas": canvas, "safe": safe, "cap_top": cap_top, "cap_size": cap_size, "cap_w": cap_w}


def _brand(campaign: dict) -> dict:
    b = campaign.get("brand") or {}
    return {"colors": {**DEFAULT_COLORS, **(b.get("colors") or {})}, "fonts": {**DEFAULT_FONTS, **(b.get("fonts") or {})},
            "names": b.get("names") or [], "logo": b.get("logo"), "say": b.get("say") or {},
            "colors_set": dict(b.get("colors") or {})}


def _family(f) -> str:
    return f["family"] if isinstance(f, dict) else str(f)


def font_faces(look: dict) -> str:
    """@font-face for every look font that ships as a file (copied to assets/fonts/)."""
    out, seen = [], set()
    for f in (look.get("fonts") or {}).values():
        if isinstance(f, dict) and f.get("file") and (f["family"], f["file"]) not in seen:
            seen.add((f["family"], f["file"]))
            out.append(f'@font-face {{ font-family: "{f["family"]}"; src: url("assets/fonts/{Path(f["file"]).name}"); '
                       f'font-weight: {f.get("weight", "100 900")}; font-style: {f.get("style", "normal")}; }}')
    return "\n      ".join(out)


# look.paper: the text effects sit on paper, not on a picture: ink letters, no dark outline or glow
PAPER_CSS = ("html .mvk-slam, html .mvk-title, html .mvk-counter, html .mvk-end-title { color: var(--mv-ink); "
             "-webkit-text-stroke: 0; text-shadow: 0.05em 0.05em 0 var(--mv-ground); } "
             "html .mvk-type { color: var(--mv-ink); -webkit-text-stroke: 0; text-shadow: none; } "
             "html .mvk-slam .hot, html .mvk-title .hot { color: var(--mv-accent); }")


def look_vars(look: dict, brand: dict) -> str:
    c, f = look["colors"], look["fonts"]
    v = {**{f"--mv-{k}": val for k, val in brand["colors"].items()},
         **{f"--mv-{k.replace('_', '-')}": val for k, val in c.items()},
         "--mv-ink-on-plate": c.get("on_plate", "#fbf7ef"),
         "--mv-display-font": f'"{_family(f.get("display", "Montserrat"))}"', "--mv-body-font": f'"{_family(f.get("body", "Inter"))}"',
         "--mv-mono-font": f'"{_family(f.get("mono", "IBM Plex Mono"))}"', "--mv-slam-case": look.get("slam_case", "uppercase")}
    return " ".join(f"{k}: {val};" for k, val in v.items())


# A caption group should not end on a word that leans on the next one.
WEAK_END = {"a", "an", "the", "to", "of", "at", "in", "into", "on", "for", "with", "by", "from", "my", "his", "her",
            "their", "your", "our", "its", "and", "or", "but", "so", "i", "i'm", "is", "are", "was", "not", "no",
            "every", "this", "that", "these", "those", "after", "before", "as", "những", "các", "một", "của", "và", "là", "thì", "mà", "cho", "với", "ở"}
NUMBER = {"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "twenty", "fifty", "hundred",
          "thousand", "million", "một", "hai", "ba", "mười", "trăm", "nghìn", "ngàn", "triệu"}


def _chunk(phrase: list[dict], max_words: int) -> list[list[dict]]:
    """Split one phrase into ≤ max_words groups: balanced, no orphan word, no group
    ending on a weak word or inside a spelled-out number."""
    n = len(phrase)
    if n <= max_words:
        return [phrase]
    bare = [re.sub(r"[^\w']", "", t["text"]).lower() for t in phrase]

    def cost(i, j):  # group phrase[i:j]
        c = 1.0 + (1.5 if j - i == 1 else 0)
        if j < n and bare[j - 1] in WEAK_END:
            c += 2
        if j < n and bare[j - 1] in NUMBER and bare[j] in NUMBER:
            c += 3
        if i > 0 and bare[i] in WEAK_END:  # a function word naturally opens a group
            c -= 0.3
        return c

    best = [(0.0, [])] + [(float("inf"), [])] * n
    for j in range(1, n + 1):
        for i in range(max(0, j - max_words), j):
            c = best[i][0] + cost(i, j)
            sizes = [len(g) for g in best[i][1]] + [j - i]
            c += 0.3 * (max(sizes) - min(sizes))
            if c < best[j][0]:
                best[j] = (c, best[i][1] + [phrase[i:j]])
    return best[n][1]


def caption_groups(tl: dict, script: dict, max_words: int = 3, pause: float = 0.22) -> list[dict]:
    """Whitespace tokens (punctuation kept for display) carrying their words' times,
    cut into phrases at punctuation and pauses, then into readable groups."""
    keys = {k.lower() for b in script["beats"] for k in (b.get("keys") or [])}
    groups = []
    for b, t in zip(script["beats"], tl["beats"]):
        if b.get("captions") is False or not t["words"]:
            continue
        ws, k, cur, phrases = t["words"], 0, [], []
        for tok in re.findall(r"\S+", b.get("vo") or ""):
            n = len(words(tok))
            if n == 0:
                if cur:
                    cur[-1]["text"] += " " + tok
                continue
            span = ws[k:k + n]
            k += n
            if not span:
                break
            item = {"text": tok, "s": span[0]["s"], "e": span[-1]["e"],
                    "key": any(w["w"].lower() in keys for w in span)}
            if cur and item["s"] - cur[-1]["e"] >= pause:
                phrases.append(cur)
                cur = []
            cur.append(item)
            if re.search(r"[.!?…:;,]$", tok):
                phrases.append(cur)
                cur = []
        if cur:
            phrases.append(cur)
        for ph in phrases:
            groups.extend(_chunk(ph, max_words))
    out = []
    for i, g in enumerate(groups):
        s = max(0.05, g[0]["s"] - 0.04)
        e = max(g[-1]["e"] + 0.08, s + 0.3)
        if i + 1 < len(groups):  # linger into a pause, never into the next group: one line on screen at a time
            e = min(e + 0.25, max(0.05, groups[i + 1][0]["s"] - 0.04))
        out.append({"s": round(s, 3), "e": round(e, 3), "tokens": g})
    # ASR gives two words the same start when a number is read fast ("ba mươi tư"): the
    # group between them inherits a zero-width window, never renders, and writes an SRT cue
    # that starts and ends on the same millisecond. Fold such a group into the next one so
    # every caption is actually on screen for a readable span.
    folded: list[dict] = []
    for g in out:
        if folded and folded[-1]["e"] - folded[-1]["s"] < 0.12:
            folded[-1] = {"s": folded[-1]["s"], "e": max(g["e"], folded[-1]["s"] + 0.12),
                          "tokens": folded[-1]["tokens"] + g["tokens"]}
        else:
            folded.append(g)
    if folded and folded[-1]["e"] - folded[-1]["s"] < 0.12:
        folded[-1]["e"] = round(folded[-1]["s"] + 0.12, 3)
    # Nothing overlaps: a start is clamped to the previous end.
    prev_end = 0.0
    for g in folded:
        g["s"] = round(max(g["s"], prev_end), 3)
        g["e"] = round(max(g["e"], g["s"] + 0.12), 3)
        prev_end = g["e"]
    return folded


CAPTION_STYLES = {
    # stroke: a heavy outline that reads on any plate (story, art); shadow: the light legacy style; box: a solid block
    "stroke": "-webkit-text-stroke: {sw}px rgba(0,0,0,.92); paint-order: stroke fill; text-shadow: 0 6px 18px rgba(0,0,0,.55);",
    "shadow": "text-shadow: 0 5px 0 rgba(0,0,0,.55), 0 0 22px rgba(0,0,0,.55);",
    "box": "background: rgba(0,0,0,.78); padding: 0.04em 0.16em; border-radius: 0.06em;",
    # comic: ink letters in a white label with a bold ink outline and an offset shadow, the spoken word in the accent
    "comic": ("background: #ffffff; border: {bw}px solid {ink}; box-shadow: {bw}px {bw}px 0 {ink}; "
              "padding: 0.02em 0.2em 0.06em; border-radius: 0.18em;"),
}


def captions_html(groups: list[dict], geo: dict, brand: dict, duration: float, cap: dict | None = None,
                  look: dict | None = None, beat_tops: list[tuple[float, float, int]] | None = None) -> str:
    """beat_tops: (start, end, top px) for beats whose direction moves the captions off their picture."""
    c, f = dict(brand["colors"]), dict(brand["fonts"])
    cap = cap or {"style": "shadow"}
    if look:  # a directed video: captions sit on pictures, in the look's own colours and type
        c["fg"], c["accent"] = look["colors"].get("on_plate", "#fbf7ef"), look["colors"].get("hot", c["accent"])
        if cap.get("style") == "comic":
            c["fg"], c["accent"] = look["colors"].get("ink", "#1b1b1b"), look["colors"].get("accent", c["accent"])
        f["caption"] = _family(cap.get("font") or look["fonts"].get("caption") or look["fonts"].get("display"))
    size = int(cap.get("size") or geo["cap_size"])
    top = int(cap.get("y") or geo["cap_top"])
    deco = CAPTION_STYLES.get(cap.get("style") or "shadow", CAPTION_STYLES["shadow"]).format(
        sw=max(6, round(size * 0.12)), bw=max(3, round(size * 0.06)), ink=(look or {}).get("colors", {}).get("ink", "#1b1b1b"))
    weight = int(cap.get("weight") or 800)
    faces = font_faces(look) if look else ""
    rows, js = [], []
    for i, g in enumerate(groups):
        # A fixed gap between words: the spoken word lifts (scale) and must not eat the space beside it.
        spans = '<span class="gap"> </span>'.join(
            f'<span id="cap{i}-{j}" class="w{" key" if t["key"] else ""}">{html.escape(t["text"])}</span>'
            for j, t in enumerate(g["tokens"]))
        w0 = g["tokens"][0]["s"]  # the group shows a little before its first word: place it by that word's beat
        own = next((t for b0, b1, t in beat_tops or [] if b0 <= w0 < b1), None)
        rows.append(f'    <div id="cap{i}" class="cap"' + (f' style="top:{own}px"' if own is not None else "") + f'>{spans}</div>')
        cut = i + 1 < len(groups) and groups[i + 1]["s"] - g["e"] < 0.05  # the next group replaces this one
        js.append(f'  show("#cap{i}", {g["s"]}, {g["e"]}, {json.dumps([[t["s"], t["e"]] for t in g["tokens"]])}, {str(cut).lower()});')
    return f"""<template>
  <style>
    {faces}
    #root {{ position: absolute; inset: 0; pointer-events: none; }}
    .cap {{ position: absolute; left: 0; right: 0; top: {top}px; text-align: center;
           font-size: {size}px;  /* the gap span sizes its width in em off this */
           opacity: 0; visibility: hidden; padding: 0 {max(geo['safe']['left'], geo['safe']['right'])}px; }}
    .cap .w {{ display: inline-block; font-family: "{f['caption']}"; font-weight: {weight}; font-size: {size}px;
              line-height: 1.12; color: {c['fg']}; letter-spacing: -0.01em;
              {deco} margin: 0 0.07em; }}
    .cap .gap {{ display: inline-block; width: 0.26em; }}
    .cap .w.key {{ color: {c['accent']}; }}
    {cap.get("css") or ""}
  </style>
  <div id="root" data-composition-id="captions" data-width="{geo['W']}" data-height="{geo['H']}" data-duration="{duration}">
{chr(10).join(rows)}
  </div>
  <script>
    const tl = gsap.timeline({{ paused: true }});
    const ACCENT = "{c['accent']}", FG = "{c['fg']}";
    // One group on screen at a time; the spoken word lifts and takes the accent.
    // A group the next one replaces is cut, not faded: two half-visible groups read as one garbled line.
    function show(sel, s, e, ws, cut) {{
      tl.set(sel, {{ visibility: "visible" }}, s);
      tl.fromTo(sel, {{ opacity: 0, y: 16, scale: 0.94 }}, {{ opacity: 1, y: 0, scale: 1, duration: 0.12, ease: "power2.out" }}, s);
      ws.forEach(([a, b], j) => {{
        const w = `${{sel}}-${{j}}`;
        const key = document.querySelector(w)?.classList.contains("key");
        tl.fromTo(w, {{ scale: 1, color: key ? ACCENT : FG }}, {{ scale: 1.06, color: ACCENT, duration: 0.07, ease: "power2.out" }}, a);
        tl.to(w, {{ scale: 1, color: key ? ACCENT : FG, duration: 0.1, ease: "power1.out" }}, Math.max(b, a + 0.08));
      }});
      if (!cut) tl.to(sel, {{ opacity: 0, duration: 0.1, ease: "power2.in" }}, e - 0.1);
      tl.set(sel, {{ opacity: 0, visibility: "hidden" }}, e);
    }}
{chr(10).join(js)}
    window.__timelines["captions"] = tl;
  </script>
</template>
"""


def scene_placeholder(b: dict, t: dict, geo: dict, brand: dict, logo: str | None) -> str:
    bid, c, f = b["id"], brand["colors"], brand["fonts"]
    on = html.escape((b.get("onscreen") or "").strip())
    vis = html.escape((b.get("visual") or "").strip())
    s = geo["safe"]
    size = 104 if geo["canvas"] == "9x16" else 84
    logo_html = (f'\n    <img id="{bid}-logo" src="{logo}" alt="" style="position:absolute;left:50%;top:{round(geo["H"] * 0.18)}px;'
                 f'width:{round(geo["W"] * 0.22)}px;transform:translateX(-50%)" />') if logo and b.get("role") in {"cta", "end"} else ""
    return f"""<template>
  <!-- PLACEHOLDER scene for {bid} ({b.get('role')}). Replace everything below with the real scene, then delete
       the data-mvf attribute. Brief: STORYBOARD.md row {bid}; rules: SCENE_GUIDE.md. -->
  <style>
    #root {{ position: absolute; inset: 0; overflow: hidden; }}
    #{bid}-bg {{ position: absolute; inset: -4%; background: radial-gradient(120% 80% at 50% 30%, {c['accent']}22, {c['bg']} 60%); }}
    #{bid}-text {{ position: absolute; left: {s['left'] + 30}px; right: {s['right'] + 30}px; top: {s['top'] + 220}px;
                  text-align: center; font-family: "{f['display']}"; font-weight: 800; font-size: {size}px;
                  line-height: 1.04; color: {c['fg']}; letter-spacing: -0.02em; }}
    #{bid}-brief {{ position: absolute; left: {s['left'] + 30}px; right: {s['right'] + 30}px; top: {round(geo['H'] * 0.45)}px;
                   font-family: "IBM Plex Mono"; font-size: 30px; line-height: 1.35; color: {c['muted']}; text-align: center; }}
  </style>
  <div id="root" {PLACEHOLDER} data-composition-id="{bid}" data-width="{geo['W']}" data-height="{geo['H']}">
    <div id="{bid}-bg"></div>{logo_html}
    <div id="{bid}-text">{on}</div>
    <div id="{bid}-brief" data-layout-ignore>PLACEHOLDER {bid} · {vis}</div>
  </div>
  <script>
    // Beat timing: MV.beat("{bid}") → {{dur, words:[{{w, s, e}}]}} in this scene's local seconds.
    const B = (window.MV && window.MV.beat("{bid}")) || {{ dur: {round(t['end'] - t['start'], 3)}, words: [] }};
    const tl = gsap.timeline({{ paused: true }});
    tl.fromTo("#{bid}-text", {{ opacity: 0, y: 40, scale: 0.94 }}, {{ opacity: 1, y: 0, scale: 1, duration: 0.32, ease: "back.out(1.7)" }}, 0.02);
    tl.fromTo("#{bid}-bg", {{ scale: 1 }}, {{ scale: 1.08, duration: B.dur, ease: "none" }}, 0);
    window.__timelines["{bid}"] = tl;
  </script>
</template>
"""


def index_html(tl: dict, geo: dict, brand: dict, audio: list[dict], music: list[dict], title: str,
               look: dict | None = None, slots: dict | None = None, overlay: bool = False) -> str:
    c = brand["colors"]
    W, H = geo["W"], geo["H"]
    look = look or dirn.look_of(None, brand)
    slots = slots or {}
    hosts = []
    for i, b in enumerate(tl["beats"]):
        lead, tail = slots.get(b["id"], (0.0, 0.0))  # a directed beat's way in starts early / its way out ends late
        d = round(b["end"] - b["start"] + lead + tail, 3)
        hosts.append(f'      <div id="{b["id"]}" data-composition-id="{b["id"]}" data-composition-src="compositions/{b["id"]}.html"'
                     f' data-start="{round(b["start"] - lead, 3)}" data-duration="{d}" data-track-index="{1 + i % 2}" data-track-kind="graphics"'
                     f' data-width="{W}" data-height="{H}" style="z-index:{10 + i}"></div>')
    if overlay:
        hosts.append(f'      <div id="overlay" data-composition-id="overlay" data-composition-src="compositions/overlay.html" data-start="0"'
                     f' data-duration="{tl["duration"]}" data-track-index="8" data-track-kind="graphics" data-width="{W}" data-height="{H}"'
                     f' style="z-index:850"></div>')
    kit_css = (KIT / "mvkit.css").read_text(encoding="utf-8")
    kit_js = (KIT / "mvkit.js").read_text(encoding="utf-8")
    aud = []
    for k, a in enumerate(audio):
        aud.append(f'      <audio id="vo-{a["beat"]}" class="clip" src="{a["src"]}" data-start="{a["start"]}" data-duration="{a["dur"]}"'
                   f' data-media-start="{a["trim"]}" data-track-index="{20 + k % 2}" data-volume="1"></audio>')
    for k, m in enumerate(music):
        aud.append(f'      <audio id="{m["id"]}" class="clip" src="{m["src"]}" data-start="0" data-duration="{tl["duration"]}"'
                   f' data-track-index="{30 + k}" data-volume="{m["vol"]}" data-fade-out="0.6"></audio>')
    mv = {b["id"]: {"start": b["start"], "dur": round(b["end"] - b["start"], 3),
                    "words": [{"w": w["w"], "s": round(w["s"] - b["start"], 3), "e": round(w["e"] - b["start"], 3)} for w in b["words"]]}
          for b in tl["beats"]}
    return f"""<!doctype html>
<!-- GENERATED by mvf scaffold: re-run it instead of editing. Scenes live in compositions/<beat>.html. -->
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width={W}, height={H}" />
    <title>{html.escape(title)}</title>
    <script src="https://cdn.jsdelivr.net/npm/gsap@3.14.2/dist/gsap.min.js"></script>
    <script>
      // Measured beat timing for scenes: MV.beat(id) → {{start, dur, words:[{{w, s, e}}]}}, times local to the beat.
      window.MV = {{ beats: {json.dumps(mv, ensure_ascii=False)}, beat(id) {{ return this.beats[id]; }} }};
    </script>
    <script>
{kit_js}
    </script>
    <style>
      {font_faces(look)}
      :root {{ {look_vars(look, brand)} }}
      {PAPER_CSS if look.get("paper") else ""}
      * {{ margin: 0; padding: 0; box-sizing: border-box; }}
      html, body {{ margin: 0; width: {W}px; height: {H}px; overflow: hidden; background: {look['colors']['ground']}; }}
      #root {{ position: relative; width: 100%; height: 100%; }}
      #ground {{ position: absolute; inset: 0; background: {look['colors']['ground']}; }}
{kit_css}
      {look.get("css") or ""}
    </style>
  </head>
  <body>
    <div id="root" data-composition-id="main" data-start="0" data-duration="{tl['duration']}" data-width="{W}" data-height="{H}">
      <div id="ground"></div>
{chr(10).join(hosts)}
      <div id="captions" data-composition-id="captions" data-composition-src="compositions/captions.html" data-start="0"
           data-duration="{tl['duration']}" data-track-index="9" data-track-kind="captions" data-width="{W}" data-height="{H}"
           style="z-index:900"></div>
      {look.get("html") or ""}
{chr(10).join(aud)}
    </div>
    <script>
      window.__timelines["main"] = gsap.timeline({{ paused: true }});
    </script>
  </body>
</html>
"""


def storyboard_md(script: dict, tl: dict, geo: dict, platforms: list[str], hook: str) -> str:
    rows = []
    for b, t in zip(script["beats"], tl["beats"]):
        cell = lambda s: (s or "").replace("|", "\\|").replace("\n", " ")
        rows.append(f"| {b['id']} | {b.get('role')} | {t['start']:.2f}–{t['end']:.2f} | {cell(b.get('vo'))} | "
                    f"{cell(b.get('onscreen'))} | {cell(b.get('visual'))} | {', '.join(b.get('facts') or [])} |")
    labels = ", ".join(PLATFORMS[p]["label"] for p in platforms)
    return f"""# Storyboard — {script['id']} ({hook}, {geo['canvas']})

GENERATED by `mvf scaffold`; edit the script and re-run, never this file.

- **Value:** {script.get('value', '')}
- **Format:** {script.get('format')} · **Platforms:** {labels} · **Canvas:** {geo['W']}×{geo['H']} · **Runtime:** {tl['duration']:.1f}s ({tl['timing']} timing)
- **Language / voice:** {script.get('language')} / {(script.get('voice') or {}).get('mode')}

| Beat | Role | Time (s) | Voice | On-screen | Visual | Facts |
| --- | --- | --- | --- | --- | --- | --- |
{chr(10).join(rows)}

Every on-screen claim traces to its fact in `01-product/facts.json`; a beat with
no fact shows no number, superlative or "free".
"""


def scene_guide_md(script: dict, tl: dict, geo: dict, brand: dict, platforms: list[str]) -> str:
    tmpl = (TOOLS / "templates/SCENE_GUIDE.md").read_text(encoding="utf-8")
    s, c, f = geo["safe"], brand["colors"], brand["fonts"]
    vals = {
        "W": geo["W"], "H": geo["H"], "CANVAS": geo["canvas"], "SAFE_TOP": s["top"], "SAFE_BOTTOM": s["bottom"],
        "SAFE_LEFT": s["left"], "SAFE_RIGHT": s["right"], "CAP_TOP": geo["cap_top"],
        "CAP_BOTTOM": geo["cap_top"] + 2 * geo["cap_size"] + 40,
        "BG": c["bg"], "FG": c["fg"], "ACCENT": c["accent"], "MUTED": c["muted"],
        "FONT_DISPLAY": f["display"], "FONT_BODY": f["body"], "BRAND": ", ".join(brand["names"]) or "the brand",
        "PLATFORMS": ", ".join(PLATFORMS[p]["label"] for p in platforms), "DURATION": tl["duration"],
        "SCRIPT": script["id"], "VALUE": script.get("value", ""),
    }
    return re.sub(r"\{\{(\w+)\}\}", lambda m: str(vals[m.group(1)]), tmpl)


def scaffold(campaign_dir: Path, script_path: Path, canvas: str, hook: str, take: Path | None,
             platforms: list[str], from_dir: Path | None = None, music: Path | None = None) -> Path:
    from .common import load
    campaign = load(campaign_dir / "campaign.json")
    script = load(script_path)
    sv = timing.variant(script, hook)
    manifest = timing.load_manifest(take) if take else None
    tl = timing.build(sv, manifest)
    geo = geometry(canvas, platforms)
    brand = _brand(campaign)
    proj = campaign_dir / "07-video" / script["id"] / canvas  # one video per script: the hook is in mvf.json
    (proj / "compositions").mkdir(parents=True, exist_ok=True)
    (proj / "assets/vo").mkdir(parents=True, exist_ok=True)
    (proj / "assets/media").mkdir(parents=True, exist_ok=True)

    # Brand files are copied in (Hyperframes resolves assets inside the project only).
    logo = None
    bdir = campaign_dir / "brand"
    if bdir.is_dir():
        shutil.copytree(bdir, proj / "assets/brand", dirs_exist_ok=True)
        if brand["logo"]:
            logo = f"assets/brand/{Path(brand['logo']).name}"
    # Approved art (mvf art approve). A directed shot whose art is not approved yet shows its newest
    # draft from assets/art/draft/ — a rough cut only: export refuses a project that still does.
    dr = dirn.load_direction(campaign_dir, script["id"])
    look = dirn.look_of(dr, brand)
    art_dir = campaign_dir / "07-video/art"
    art_map = {}
    for a in sorted(art_dir.glob("*.png")) if art_dir.is_dir() else []:
        if a.with_suffix(".json").is_file():
            (proj / "assets/art").mkdir(exist_ok=True)
            shutil.copy2(a, proj / "assets/art" / a.name)
            rec = load(a.with_suffix(".json"))
            shown = str(rec.get("shown") or "")
            art_map[a.stem] = {"path": f"assets/art/{a.name}", "approved": True, "cutout": rec.get("clear") is not None,
                               "shown_h": int(shown.split("x")[1]) if re.match(r"^\d+x\d+$", shown) else None}
    media_map = {}
    if dr:
        shutil.rmtree(proj / "assets/art/draft", ignore_errors=True)
        for bs in (dr.get("beats") or {}).values():
            for sh in (bs.get("shots") or []) if isinstance(bs, dict) else (bs if isinstance(bs, list) else []):
                for a in [sh.get("art")] + [L.get("art") for L in sh.get("layers") or []]:
                    if not a or a in art_map:
                        continue
                    ds = sorted((art_dir / "draft").glob(f"{a}-s*.png"), key=lambda x: x.stat().st_mtime) if (art_dir / "draft").is_dir() else []
                    if ds:
                        (proj / "assets/art/draft").mkdir(parents=True, exist_ok=True)
                        shutil.copy2(ds[-1], proj / "assets/art/draft" / f"{a}.png")
                        art_map[a] = {"path": f"assets/art/draft/{a}.png", "approved": False}
                for kind in ("video", "image"):
                    if sh.get(kind) and (campaign_dir / sh[kind]).is_file():
                        name = Path(sh[kind]).name
                        shutil.copy2(campaign_dir / sh[kind], proj / "assets/media" / name)
                        media_map[sh[kind]] = f"assets/media/{name}"
        # a drawn prop or rig part may place a campaign file inside its SVG: href="media:<campaign path>"
        sheets = [json.dumps(dr, ensure_ascii=False)] + [(campaign_dir / "07-video" / f"{k}.json").read_text()
                                                         for k in ("props", "rigs") if (campaign_dir / "07-video" / f"{k}.json").is_file()]
        for ref in dict.fromkeys(re.findall(r'media:([^"\\]+)', "".join(sheets))):
            if (campaign_dir / ref).is_file() and ref not in media_map:
                shutil.copy2(campaign_dir / ref, proj / "assets/media" / Path(ref).name)
                media_map[ref] = f"assets/media/{Path(ref).name}"
        for f in (look.get("fonts") or {}).values():
            if isinstance(f, dict) and f.get("file"):
                src = campaign_dir / f["file"]
                if not src.is_file():
                    raise SystemExit(f"direction look font {f['file']} not found in the campaign")
                (proj / "assets/fonts").mkdir(parents=True, exist_ok=True)
                shutil.copy2(src, proj / "assets/fonts" / src.name)

    audio = []
    if manifest:
        cues = {c["id"]: c for c in manifest["cues"]}
        for b in tl["beats"]:
            if not b["cue"]:
                continue
            cue = cues[b["cue"]]
            src = proj / "assets/vo" / cue["path"]
            shutil.copy2(take / cue["path"], src)
            dur = round(cue["speech"][1] - b["vo_trim"] + 0.06, 3)
            audio.append({"beat": b["id"], "src": f"assets/vo/{cue['path']}", "start": b["vo_start"], "trim": b["vo_trim"], "dur": dur})
    mus = []
    for name, vol in (("bed.wav", 1), ("sfx.wav", 1)):
        p = (music / name) if music else proj / "assets/music" / name
        if p.is_file():
            (proj / "assets/music").mkdir(exist_ok=True)
            if p.parent != proj / "assets/music":
                shutil.copy2(p, proj / "assets/music" / name)
            mus.append({"id": name.split(".")[0], "src": f"assets/music/{name}", "vol": vol})

    # Body scenes shared with another hook's project are copies; a copy follows its
    # source on every run until it is edited here (its sha no longer matches the
    # one recorded in mvf.json), then it is this project's own scene.
    prev = json.loads((proj / "mvf.json").read_text()).get("copies") or {} if (proj / "mvf.json").is_file() else {}
    old = _hook_of(proj)
    first = proj / "compositions" / f"{sv['beats'][0]['id']}.html"
    if old and old != hook and first.is_file():
        first.rename(first.with_name(f"{first.stem}.{old}.html"))  # the old hook's opening scene, kept, not shown
        prev.pop(sv["beats"][0]["id"], None)
    created, copied, copies, generated, kept = [], [], {}, [], []
    plan, prep = (dirn.check(campaign_dir, script["id"], tl, geo, art_map, media_map) if dr else (None, None))
    slots = {}
    if plan:
        for bid, slot in plan["beats"].items():
            if slot.get("error") or "spec" not in slot:
                continue
            dst = proj / "compositions" / f"{bid}.html"
            if dst.exists() and not any(m in dst.read_text(encoding="utf-8") for m in (PLACEHOLDER, GENERATED)):
                kept.append(bid)  # authored by hand: never overwritten (set the beat "custom" to say so)
                continue
            dst.write_text(dirn.scene_html(bid, slot, geo), encoding="utf-8")
            slots[bid] = (slot["lead"], slot["tail"])
            generated.append(bid)
    for b, t in zip(sv["beats"], tl["beats"]):
        if b["id"] in generated:
            continue
        dst = proj / "compositions" / f"{b['id']}.html"
        src = (from_dir / "compositions" / f"{b['id']}.html") if from_dir else None
        same_beat0 = not (b is sv["beats"][0] and from_dir and _hook_of(from_dir) != hook)
        usable = bool(src and src.is_file() and same_beat0 and src.resolve() != dst.resolve())
        if dst.exists():
            follows = PLACEHOLDER in dst.read_text(encoding="utf-8") or prev.get(b["id"]) == _sha(dst)
            if usable and follows and _sha(src) != _sha(dst):
                shutil.copy2(src, dst)
                copied.append(b["id"])
        elif usable:
            shutil.copy2(src, dst)
            copied.append(b["id"])
        else:
            dst.write_text(scene_placeholder(b, t, geo, brand, logo), encoding="utf-8")
            created.append(b["id"])
        if usable and _sha(src) == _sha(dst):
            copies[b["id"]] = _sha(dst)
        elif not from_dir and b["id"] in prev:
            copies[b["id"]] = prev[b["id"]]  # a run without a source (e.g. `mvf score`) keeps the record
    if from_dir:
        for d in ("media", "fonts"):
            if (from_dir / "assets" / d).is_dir():
                shutil.copytree(from_dir / "assets" / d, proj / "assets" / d, dirs_exist_ok=True)

    cap = dirn.captions_of(dr) if dr else None
    groups = caption_groups(tl, sv, max_words=int((cap or {}).get("words") or 3))
    tops = [(b["start"], b["end"], round(float(bs["captions_y"]) * geo["H"])) for b in tl["beats"]
            for bs in [((dr or {}).get("beats") or {}).get(b["id"])] if isinstance(bs, dict) and bs.get("captions_y") is not None]
    cap_file = proj / "compositions/captions.html"
    if not ((cap or {}).get("custom") and cap_file.exists()):  # captions.custom: true keeps a hand-authored captions.html
        cap_file.write_text(captions_html(groups, geo, brand, tl["duration"], cap, look if dr else None, tops), encoding="utf-8")
    overlay = bool(dr)
    if overlay:
        chip = look.get("chip")
        until = None
        if chip and chip.get("until") is not None:
            until = dirn.resolve_global(chip["until"], {b["id"]: b for b in tl["beats"]})
        dirn.grain_png(proj / "assets/kit/grain.png")
        (proj / "compositions/overlay.html").write_text(dirn.overlay_html(look, geo, tl["duration"], until), encoding="utf-8")
    else:
        (proj / "compositions/overlay.html").unlink(missing_ok=True)
    if plan:
        dump(proj / "sfx.json", {"cues": sorted(plan["sfx"], key=lambda c: c["t"]), "sound": (dr or {}).get("sound") or {}})
        dump(proj / "direction.plan.json", {"events": plan["events"], "transitions": plan["transitions"], "custom": plan["custom"],
                                            "missing": plan["missing"], "generated": generated, "kept": kept,
                                            "draft_art": sorted(set(plan["draft_art"])), "items": prep.items,
                                            "beats": {k: {"lead": v["lead"], "tail": v["tail"]} for k, v in plan["beats"].items()}})
    title = f"{campaign.get('product', '')} {script['id']} {canvas} {hook}".strip()
    (proj / "index.html").write_text(index_html(tl, geo, brand, audio, mus, title, look, slots, overlay), encoding="utf-8")
    (proj / "STORYBOARD.md").write_text(storyboard_md(sv, tl, geo, platforms, hook), encoding="utf-8")
    (proj / "SCENE_GUIDE.md").write_text(scene_guide_md(sv, tl, geo, brand, platforms), encoding="utf-8")
    dump(proj / "timeline.json", {**tl, "canvas": canvas, "hook": hook, "captions": groups})
    pkg = {"name": f"{script['id']}-{canvas}-{hook}".lower(), "private": True, "type": "module",
           "scripts": {"dev": f"npx --yes hyperframes@{HYPERFRAMES} preview", "check": f"npx --yes hyperframes@{HYPERFRAMES} check",
                       "render": f"npx --yes hyperframes@{HYPERFRAMES} render"}}
    dump(proj / "package.json", pkg)
    if not (proj / "hyperframes.json").exists():
        dump(proj / "hyperframes.json", {"$schema": "https://hyperframes.heygen.com/schema/hyperframes.json",
                                         "paths": {"blocks": "compositions", "components": "compositions/components", "assets": "assets"},
                                         "media": {"autoProxy": True}})
    if not (proj / "meta.json").exists():
        dump(proj / "meta.json", {"id": pkg["name"], "name": pkg["name"]})
    dump(proj / "mvf.json", {"script": str(script_path.relative_to(campaign_dir)), "script_sha256": _sha(script_path),
                             "hook": hook, "canvas": canvas, "platforms": platforms,
                             "take": str(take.relative_to(campaign_dir)) if take else None, "timing": tl["timing"],
                             "copies": copies, "generated": generated,
                             "draft_art": sorted(set(plan["draft_art"])) if plan else []})
    print(f"scaffold {proj.relative_to(campaign_dir)}: {tl['duration']:.2f}s ({tl['timing']}), {len(tl['beats'])} scenes"
          + (f"; generated {generated}" if generated else "")
          + (f"; new placeholders {created}" if created else "") + (f"; copied {copied} from {from_dir.name}" if copied else "")
          + (f"; kept hand-authored {kept} (direction.json directs them: set them \"custom\")" if kept else ""))
    if prep and (prep.fails or prep.warns):
        prep.print("direction")
    return proj


def _hook_of(proj: Path) -> str | None:
    m = proj / "mvf.json"
    return json.loads(m.read_text())["hook"] if m.is_file() else None


def _sha(p: Path) -> str:
    return hashlib.sha256(Path(p).read_bytes()).hexdigest()[:16]


def placeholders(proj: Path) -> list[str]:
    return sorted(p.stem for p in (proj / "compositions").glob("B*.html") if PLACEHOLDER in p.read_text(encoding="utf-8"))
