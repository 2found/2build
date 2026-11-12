"""The cold viewer: a fresh helper who sees only the cut (frames at 1 fps, the voice as heard, the logline and
what the video is for), never the script or the plan, and writes a verdict with red items. A reviewer who read
the plan sees the plan in the frames; one who did not sees the video."""
from __future__ import annotations

import re
import shutil
import subprocess
from pathlib import Path

from .common import HYPERFRAMES, Report, heavy_prefix, load, tile_width

VERDICTS = ("APPROVE", "FIX", "UNVERIFIABLE")
SCORE_SCALE = "5 a student's cut, 6 a template, 7 a working pro, 8 an agency, 9 an award entry"
COLS, ROWS = 6, 3
NEAR_S = 1.5  # a red item is in review.json when an issue sits this close to its second
REFS = 4      # most-viewed research videos whose opening the cut is laid beside


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


def openers(video: Path, out: Path) -> Path:
    """The cut's first two seconds, three frames wide: the hook at a size where the framing can be judged."""
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-t", "2.1", "-i", str(video), "-vf",
                    "fps=1,scale=-2:560,tile=3x1:padding=6:margin=6", "-frames:v", "1", str(out)], check=True)
    return out


def references(cd: Path, out: Path) -> tuple[Path | None, list[str]]:
    """One frame, 2 s in, of the most-viewed research videos that were downloaded (03-research/top/<id>/video.mp4):
    what the audience's feed already looks like. Never shown in the video; a reviewer's yardstick only."""
    rows = []
    for v in (cd / "03-research/top").glob("*/video.mp4"):
        meta = v.parent.parent / f"{v.parent.name}.json"
        m = load(meta) if meta.is_file() else {}
        rows.append((int(m.get("views") or m.get("view_count") or 0), v))
    rows = sorted(rows, key=lambda x: -x[0])[:REFS]
    tiles, notes = [], []
    for k, (views, v) in enumerate(rows):
        t = out.parent / f"{out.stem}-{k}.jpg"
        r = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-ss", "2", "-i", str(v), "-vf",
                            "scale=-2:560", "-frames:v", "1", str(t)], capture_output=True)
        if r.returncode == 0 and t.is_file():
            tiles.append(t)
            notes.append(f"{views:,} views" if views else "views unknown")
    if not tiles:
        return None, []
    cmd = ["ffmpeg", "-hide_banner", "-loglevel", "error", "-y"]
    for t in tiles:
        cmd += ["-i", str(t)]
    if len(tiles) > 1:
        cmd += ["-filter_complex", f"hstack=inputs={len(tiles)}"]
    subprocess.run([*cmd, str(out)], check=True)
    for t in tiles:
        t.unlink()
    return out, notes


KEYFRAME = "review/keyframe.jpg"
KEY_W, KEY_H = 1600, 960  # the sheet's width and this video's frame height: large enough to judge a face


def beside(frame: Path, cd: Path, out: Path) -> tuple[Path, list[str]]:
    """A frame of this video, large, over a strip of the reference frames."""
    out.parent.mkdir(parents=True, exist_ok=True)
    ff = ["ffmpeg", "-hide_banner", "-loglevel", "error", "-y"]
    refs, notes = references(cd, out.parent / f"{out.stem}-refs.jpg")
    if refs:
        subprocess.run([*ff, "-i", str(frame), "-i", str(refs), "-filter_complex",
                        f"[0]scale=-2:{KEY_H},pad={KEY_W}:{KEY_H}:(ow-iw)/2:0[a];[1]scale={KEY_W}:-2[b];[a][b]vstack",
                        "-frames:v", "1", str(out)], check=True)
        refs.unlink()
    else:
        subprocess.run([*ff, "-i", str(frame), "-vf", f"scale=-2:{KEY_H}", "-frames:v", "1", str(out)], check=True)
    return out, notes


def keyframe(cd: Path, proj: Path, at: float) -> tuple[Path, list[str]]:
    """The look is settled on one finished frame before every scene is built: snapshot it and lay it beside the
    most-viewed references, so the level is compared and not remembered."""
    tmp = proj / "review/.keyframe"
    shutil.rmtree(tmp, ignore_errors=True)
    log = proj / "snapshot.log"
    with open(log, "w") as fh:
        r = subprocess.run([*heavy_prefix(cd), "npx", "--yes", f"hyperframes@{HYPERFRAMES}", "snapshot", ".", "--at", str(at),
                            "--no-end", "--describe", "false", "-o", "review/.keyframe"], cwd=proj, stdout=fh,
                           stderr=subprocess.STDOUT)
    pngs = sorted(tmp.rglob("*.png")) if tmp.is_dir() else []
    if r.returncode or not pngs:
        raise SystemExit(f"snapshot failed (see {log}):\n{log.read_text()[-1500:]}")
    res = beside(pngs[0], cd, proj / KEYFRAME)
    shutil.rmtree(tmp, ignore_errors=True)
    return res


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
    opener = openers(video, proj / "review" / f"viewer-v{n}-open.jpg")
    refs, ref_notes = references(cd, proj / "review" / f"viewer-v{n}-refs.jpg")
    rows = [f"| {b['vo_start']:.1f}–{b['vo_end']:.1f} | {' '.join(w['w'] for w in b.get('words') or [])} |"
            for b in tl["beats"] if b.get("vo_start") is not None]
    out = proj / "review" / f"viewer-brief-v{n}.md"
    rel = lambda p: p.relative_to(proj)  # noqa: E731
    if refs:
        ref_line = (f"For comparison only, one frame each from the most-viewed videos on this topic "
                    f"({'; '.join(ref_notes)}): `{rel(refs)}`")
        ref_q = ("Beside the reference frames: does this video look made to the same level? Name what they do with the "
                 "frame (the size of the subject, depth, light, how full it is) that this one does not. Their style is "
                 "theirs; the question is the level, not the look.")
    else:
        ref_line = "(Research downloaded no reference videos: judge the framing on its own.)"
        ref_q = "Would this opening stop your thumb among the videos you usually see on this topic? Say why or why not."
    out.write_text(f"""# Cold viewer: {name} (v{n}, {tl['duration']:.0f} s)

You review a short video you have never seen, for a {c.get('goal', '')} post on {', '.join(script.get('platforms') or {})}.
You get only what a viewer gets: the frames (1 per second, the second stamped on each), the voice as heard and
one line on what it is for. Do not open any other file in the project: no script, no plan, no direction.

**Logline:** {script.get('value', '')}

Frames, in order: {', '.join(f'`{rel(p)}`' for p in strips)}
The first two seconds, larger: `{rel(opener)}`
Each frame is one instant of a moving picture, so lettering or a figure caught mid-fade looks faint or doubled. Call
that a fault only when it shows in two frames running.
{ref_line}

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
6. Framing, at phone size: in each shot, how much of the frame is the subject? Name every shot where the people
   or the thing the voice names are small in an empty frame, where a face is too small to read its expression,
   where the subject is cut by the frame edge, or where it sits under the captions. A shot like that is a red item.
   Also name any band of the frame (a strip of scenery, a pattern, a border) that looks the same in shot after
   shot while the subject stays small beside it: that area is empty, however much is drawn in it.
7. {ref_q}
8. Score the cut from 1 to 10 on this scale: {SCORE_SCALE}. Say what keeps it from the next step.
9. The best moment (its second): does it land harder than everything around it? The weakest five seconds, and
   why (still, repeated, unclear, too long). Then at most three fixes, the biggest first, each with its second and
   what changes; at least one makes the best moment stronger instead of repairing a fault.

Budget: at most 15 tool calls. Write the full notes to `review/viewer-v{n}.md` (in this project folder), ending
with exactly these four lines:
Verdict: APPROVE | FIX | UNVERIFIABLE
Red items: <second>s <what>; <second>s <what>  (or none)
Score: <1-10>
Retell: <your one or two sentences>
Reply with at most 6 lines: the file path, the verdict, the score and the red items.
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
    sc = last(r"^\s*Score:\s*(\d+(?:\.\d+)?)\b")
    items = []
    if red and red.strip().lower() not in {"none", "-", ""}:
        for part in red.split(";"):
            m = re.match(r"\s*(\d+(?:\.\d+)?)(?:\s*[-–]\s*(\d+(?:\.\d+)?))?\s*s\b\s*(.*)", part)  # 6.5s or 27-35s
            if m:
                it = {"at": float(m.group(1)), "what": m.group(3).strip()}
                items.append({**it, "to": float(m.group(2))} if m.group(2) else it)
            elif part.strip():
                items.append({"at": None, "what": part.strip()})
    return {"verdict": v.upper() if v else None, "red": items, "retell": retell.strip() if retell else "",
            "score": float(sc) if sc else None}


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
    rep.info("VW0", w, f"{p.name}: {v['verdict']}" + (f", scored {v['score']:g}" if v["score"] is not None else "")
             + f"; retell: {v['retell'][:160]}")


def score_check(proj: Path, rep: Report) -> None:
    """review.json carries an honest 1-10 score on a fixed scale, so a 'looks fine' review has to say how fine."""
    p = proj / "review.json"
    if not p.is_file():
        return
    s = load(p).get("score")
    if not isinstance(s, (int, float)) or isinstance(s, bool) or not 1 <= s <= 10:
        rep.warn("RV2", f"{proj.name}.review", f"review.json has no score (1-10; {SCORE_SCALE}): score the cut against "
                 "that scale and name what keeps it from the next step")
        return
    lp = _latest(proj)
    vs = read(lp)["score"] if lp else None
    if vs is not None and s > vs:
        rep.warn("RV2", f"{proj.name}.review", f"review.json scores the cut {s:g}, the cold viewer {vs:g}: whoever made a "
                 "cut sees the plan in it. Record the stranger's score")
    elif s < 7:
        rep.info("RV2", f"{proj.name}.review", f"scored {s:g}: under a working pro's cut ({SCORE_SCALE})")
