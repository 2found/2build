"""The cold viewer: a fresh helper who sees only the cut (frames at 1 fps, the voice as heard, the logline and
what the video is for), never the script or the plan, and writes a verdict with red items. A reviewer who read
the plan sees the plan in the frames; one who did not sees the video."""
from __future__ import annotations

import re
import subprocess
from pathlib import Path

from .common import Report, load, tile_width

VERDICTS = ("APPROVE", "FIX", "UNVERIFIABLE")
COLS, ROWS = 6, 3
NEAR_S = 1.5  # a red item is in review.json when an issue sits this close to its second


def _version(proj: Path) -> int:
    ns = [int(m.group(1)) for p in (proj / "review").glob("viewer-brief-v*.md")
          if (m := re.match(r"viewer-brief-v(\d+)\.md$", p.name))]
    return max(ns, default=0) + 1


def sheets(video: Path, out: Path, dur: float, W: int, H: int) -> list[Path]:
    """1 fps strips, 18 frames a sheet, each under ~1 MP, with the second stamped on every frame."""
    per = COLS * ROWS
    w = tile_width(W, H, COLS, ROWS)
    paths = []
    for k in range(int(dur // per) + (1 if dur % per > 0.5 else 0) or 1):
        p = out.parent / f"{out.stem}-{k + 1}.jpg"
        subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-ss", str(k * per), "-t", str(per), "-i", str(video),
                        "-vf", f"fps=1,scale={w}:-2,drawtext=text='%{{eif\\:t+{k * per}\\:d}}s':x=4:y=4:fontsize={max(12, w // 10)}:"
                        f"fontcolor=white:box=1:boxcolor=black@0.6,tile={COLS}x{ROWS}:padding=4:margin=4",
                        "-frames:v", "1", str(p)], check=True)
        paths.append(p)
    return paths


def brief(cd: Path, proj: Path) -> Path:
    name = f"{proj.parent.name}/{proj.name}"  # <sid>/<canvas>, what `mvf export` takes
    video = proj / "renders/master.mp4"
    if not video.is_file():
        raise SystemExit(f"no {video}: render the cut first (`mvf export {name} --draft`)")
    meta, tl = load(proj / "mvf.json"), load(proj / "timeline.json")
    script, c = load(cd / meta["script"]), load(cd / "campaign.json")
    n = _version(proj)
    (proj / "review").mkdir(exist_ok=True)
    from .common import PLATFORMS
    canvas = PLATFORMS["canvases"][meta["canvas"]]
    strips = sheets(video, proj / "review" / f"viewer-v{n}-strip", tl["duration"], canvas["width"], canvas["height"])
    rows = [f"| {b['vo_start']:.1f}–{b['vo_end']:.1f} | {' '.join(w['w'] for w in b.get('words') or [])} |"
            for b in tl["beats"] if b.get("vo_start") is not None]
    out = proj / "review" / f"viewer-brief-v{n}.md"
    rel = lambda p: p.relative_to(proj)  # noqa: E731
    out.write_text(f"""# Cold viewer: {name} (v{n}, {tl['duration']:.0f} s)

You review a short video you have never seen, for a {c.get('goal', '')} post on {', '.join(script.get('platforms') or {})}.
You get only what a viewer gets: the frames (1 per second, the second stamped on each), the voice as heard and
one line on what it is for. Do not open any other file in the project: no script, no plan, no direction.

**Logline:** {script.get('value', '')}

Frames, in order: {', '.join(f'`{rel(p)}`' for p in strips)}

| s | voice |
| --- | --- |
{chr(10).join(rows) or '| — | (no voice) |'}

Watch it as a viewer first, then as an editor:
1. Retell it in one or two sentences as you would to a friend. If you cannot, say so.
2. The second you would swipe, and why. Every place you did not follow (a name, a "he", a "so").
3. Each frame: does the picture show what the voice says at that second? Is anything broken (text over a face,
   cut-off words, a black or frozen stretch, a mangled hand, stray text)?
4. Could this frame belong to another video? Name any frame that is a stock look (glowing particles over a
   dark gradient, a centred title on a blurred photo, a floating phone mock-up, a generic city at night, a
   lens flare) instead of something only this video has.
5. Where the eye goes in each shot, and whether that is where the voice points.

Budget: at most 15 tool calls. Write the full notes to `review/viewer-v{n}.md` (in this project folder), ending
with exactly these three lines:
Verdict: APPROVE | FIX | UNVERIFIABLE
Red items: <second>s <what>; <second>s <what>  (or none)
Retell: <your one or two sentences>
Reply with at most 6 lines: the file path, the verdict and the red items.
""", encoding="utf-8")
    return out


def _latest(proj: Path) -> Path | None:
    vs = [(int(m.group(1)), p) for p in (proj / "review").glob("viewer-v*.md")
          if (m := re.match(r"viewer-v(\d+)\.md$", p.name))] if (proj / "review").is_dir() else []
    return max(vs)[1] if vs else None


def read(path: Path) -> dict:
    text = path.read_text(encoding="utf-8")
    last = lambda pat: (re.findall(pat, text, re.M | re.I) or [None])[-1]  # noqa: E731
    v = last(r"^\s*Verdict:\s*([A-Z]+)\s*$")  # the brief's `APPROVE | FIX | ...` template line does not match
    red = last(r"^\s*Red items:\s*(.*)$")
    retell = last(r"^\s*Retell:\s*(.*)$")
    items = []
    if red and red.strip().lower() not in {"none", "-", ""}:
        for part in red.split(";"):
            m = re.match(r"\s*(\d+(?:\.\d+)?)(?:\s*[-–]\s*(\d+(?:\.\d+)?))?\s*s\b\s*(.*)", part)  # 6.5s or 27-35s
            if m:
                it = {"at": float(m.group(1)), "what": m.group(3).strip()}
                items.append({**it, "to": float(m.group(2))} if m.group(2) else it)
            elif part.strip():
                items.append({"at": None, "what": part.strip()})
    return {"verdict": v.upper() if v else None, "red": items, "retell": retell.strip() if retell else ""}


def check(proj: Path, rep: Report) -> None:
    p = _latest(proj)
    w = f"{proj.name}.viewer"
    if p is None:
        rep.warn("VW1", w, "no cold viewer: `mvf viewer <proj>`, give the brief to a fresh helper, and fix what it marks red")
        return
    n = int(re.match(r"viewer-v(\d+)", p.name).group(1))
    if _version(proj) - 1 > n:
        rep.warn("VW1", w, f"{p.name} answers an older brief than viewer-brief-v{_version(proj) - 1}.md: "
                 "give the newest brief to a fresh helper")
        return
    v = read(p)
    if v["verdict"] not in VERDICTS:
        rep.warn("VW1", w, f"{p.name} has no `Verdict: APPROVE|FIX|UNVERIFIABLE` line: the helper did not finish")
        return
    if v["verdict"] == "UNVERIFIABLE":
        rep.warn("VW1", w, f"{p.name}: the viewer could not judge from the frames; give it what it was missing and rerun")
        return
    if v["red"]:
        rv = load(proj / "review.json") if (proj / "review.json").is_file() else {}
        ats = [float(i["at"]) for i in rv.get("issues") or [] if isinstance(i.get("at"), (int, float))]
        loose = [it for it in v["red"] if it["at"] is None
                 or not any(it["at"] - NEAR_S <= a <= it.get("to", it["at"]) + NEAR_S for a in ats)]
        if loose:
            rep.warn("VW2", w, f"{p.name} marks red items review.json does not carry: "
                     + "; ".join(f"{it['at']}s {it['what']}" if it["at"] is not None else it["what"] for it in loose[:5])
                     + ". Record each as an issue (fixed, or accepted with a reason)")
    rep.info("VW0", w, f"{p.name}: {v['verdict']}; retell: {v['retell'][:160]}")


SCORE_SCALE = "5 a student's cut, 6 a template, 7 a working pro, 8 an agency, 9 an award entry"


def score_check(proj: Path, rep: Report) -> None:
    """review.json carries an honest 1-10 score on a fixed scale, so a 'looks fine' review has to say how fine."""
    p = proj / "review.json"
    if not p.is_file():
        return
    s = load(p).get("score")
    if not isinstance(s, (int, float)) or isinstance(s, bool) or not 1 <= s <= 10:
        rep.warn("RV2", f"{proj.name}.review", f"review.json has no score (1-10; {SCORE_SCALE}): score the cut against "
                 "that scale and name what keeps it from the next step")
    elif s < 7:
        rep.info("RV2", f"{proj.name}.review", f"scored {s:g}: under a working pro's cut ({SCORE_SCALE})")
