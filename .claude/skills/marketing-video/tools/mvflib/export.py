"""Render once, master once, encode per platform, and prove each file meets its
platform: codec, size, duration, loudness, first frame, frozen spans, gaps."""
from __future__ import annotations

import json
import re
import shutil
import subprocess
from pathlib import Path

from .common import HYPERFRAMES, PLATFORMS, Report, dump, heavy_prefix, load
from .scaffold import placeholders


def run(cmd: list[str], cwd: Path | None = None, check: bool = True) -> subprocess.CompletedProcess:
    r = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    if check and r.returncode:
        raise SystemExit(f"command failed ({r.returncode}): {' '.join(map(str, cmd))}\n{r.stdout[-1500:]}\n{r.stderr[-1500:]}")
    return r


def render(proj: Path, draft: bool = False) -> Path:
    raw = proj / "renders/raw.mp4"
    raw.parent.mkdir(exist_ok=True)
    q = "draft" if draft else "delivery"
    log = proj / "renders/render.log"
    with open(log, "w") as fh:
        r = subprocess.run([*heavy_prefix(proj), "npx", "--yes", f"hyperframes@{HYPERFRAMES}", "render", "-q", q, "-w", "2",
                            "-o", "renders/raw.mp4"], cwd=proj, stdout=fh, stderr=subprocess.STDOUT)
    if r.returncode or not raw.is_file():
        tail = log.read_text()[-2000:]
        raise SystemExit(f"render failed (see {log}):\n{tail}")
    return master_audio(raw, proj / "renders/master.mp4")


def master_audio(src: Path, out: Path, lufs: float = -14.0) -> Path:
    """Static gain to `lufs`, then an oversampled brickwall limiter at -2.4 dBFS (AAC overshoots ~0.5-1 dB,
    so the file peaks near -1.5); video copied untouched. Feeds turn loud uploads down but leave quiet ones
    quiet, so an un-mastered -20 LUFS render plays softer than the feed. Static gain (not loudnorm's dynamic
    mode) keeps the voice/bed balance `mvf score` set; only the impacts get limited."""
    i, _ = loudness(src)
    if i != i:
        raise SystemExit(f"cannot measure loudness of {src} (no audio?)")
    gain = round(lufs - i, 2)
    run(["ffmpeg", "-hide_banner", "-nostats", "-loglevel", "error", "-y", "-i", str(src), "-c:v", "copy", "-af",
         f"aresample=192000,volume={gain}dB,alimiter=limit=0.76:attack=3:release=60:level=disabled,aresample=48000",
         "-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", str(out)])
    return out


def target_size(canvas_wh: tuple[int, int], enc: dict, hard: dict) -> tuple[int, int]:
    """Scale the canvas into the platform's encode box, keeping its aspect (even pixels)."""
    w, h = canvas_wh
    long_box = max(enc["width"], enc["height"])
    if "max_long_side" in hard:
        long_box = min(long_box, hard["max_long_side"])
    k = long_box / max(w, h)
    if "max_short_side" in hard:
        k = min(k, hard["max_short_side"] / min(w, h))
    k = min(k, 1.0)  # never upscale
    ev = lambda x: int(round(x * k / 2) * 2)
    return ev(w), ev(h)


def encode(master: Path, out: Path, platform: str, canvas_wh: tuple[int, int]) -> Path:
    enc, hard = PLATFORMS[platform]["encode"], PLATFORMS[platform]["hard"]
    w, h = target_size(canvas_wh, enc, hard)
    v = ["-c:v", enc["vcodec"], "-profile:v", enc["profile"], "-pix_fmt", enc["pix_fmt"], "-r", str(enc["fps"]),
         "-preset", "slow", "-vf", f"scale={w}:{h}:flags=lanczos"]
    if "bitrate" in enc:
        v += ["-b:v", enc["bitrate"], "-maxrate", enc["bitrate"], "-bufsize", "12M"]
    else:
        v += ["-crf", str(enc["crf"]), "-maxrate", enc["maxrate"], "-bufsize", "24M"]
    a = ["-c:a", "aac", "-profile:a", "aac_low", "-b:a", enc["abitrate"], "-ar", str(enc["ar"]), "-ac", "2"]
    out.parent.mkdir(parents=True, exist_ok=True)
    run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(master), *v, *a, "-movflags", "+faststart", str(out)])
    return out


def probe(path: Path) -> dict:
    r = run(["ffprobe", "-v", "error", "-show_streams", "-show_format", "-of", "json", str(path)])
    return json.loads(r.stdout)


def loudness(path: Path) -> tuple[float, float]:
    r = run(["ffmpeg", "-hide_banner", "-nostats", "-i", str(path), "-af", "ebur128=peak=true", "-f", "null", "-"], check=False)
    tail = r.stderr[r.stderr.rfind("Summary:"):]
    i = re.search(r"I:\s+(-?[\d.]+) LUFS", tail)
    p = re.search(r"Peak:\s+(-?[\d.inf]+) dBFS", tail)
    return (float(i.group(1)) if i else float("nan"), float(p.group(1)) if p and p.group(1) != "-inf" else -99.0)


def first_frame_luma(path: Path) -> float:
    r = run(["ffmpeg", "-hide_banner", "-i", str(path), "-frames:v", "1", "-vf", "signalstats,metadata=print", "-f", "null", "-"], check=False)
    m = re.search(r"lavfi\.signalstats\.YAVG=([\d.]+)", r.stderr)
    return float(m.group(1)) if m else -1.0


def frozen_spans(path: Path, min_s: float = 2.5) -> list[tuple[float, float]]:
    r = run(["ffmpeg", "-hide_banner", "-i", str(path), "-vf", f"freezedetect=n=-60dB:d={min_s}", "-map", "0:v", "-f", "null", "-"], check=False)
    starts = [float(x) for x in re.findall(r"freeze_start: ([\d.]+)", r.stderr)]
    ends = [float(x) for x in re.findall(r"freeze_end: ([\d.]+)", r.stderr)]
    return list(zip(starts, ends + [float("inf")] * (len(starts) - len(ends))))


def silences(path: Path, min_s: float = 0.8) -> list[tuple[float, float]]:
    r = run(["ffmpeg", "-hide_banner", "-i", str(path), "-af", f"silencedetect=noise=-45dB:d={min_s}", "-f", "null", "-"], check=False)
    starts = [float(x) for x in re.findall(r"silence_start: (-?[\d.]+)", r.stderr)]
    ends = [float(x) for x in re.findall(r"silence_end: ([\d.]+)", r.stderr)]
    return list(zip(starts, ends + [float("inf")] * (len(starts) - len(ends))))


def qa(path: Path, platform: str, tl: dict) -> Report:
    r = Report()
    where = f"{platform}:{path.name}"
    P = PLATFORMS[platform]
    info = probe(path)
    v = next((s for s in info["streams"] if s["codec_type"] == "video"), None)
    a = next((s for s in info["streams"] if s["codec_type"] == "audio"), None)
    dur = float(info["format"]["duration"])
    mb = path.stat().st_size / 1e6
    if not v:
        r.fail("Q1", where, "no video stream")
        return r
    if v["codec_name"] != "h264" or "High" not in v.get("profile", ""):
        r.fail("Q1", where, f"video {v['codec_name']} {v.get('profile')}: needs H.264 High")
    if v.get("pix_fmt") != "yuv420p":
        r.fail("Q1", where, f"pix_fmt {v.get('pix_fmt')}: needs yuv420p")
    num, den = map(int, v["r_frame_rate"].split("/"))
    if num / den > 60.01:
        r.fail("Q1", where, f"{num / den:.2f} fps > 60")
    W, H = int(v["width"]), int(v["height"])
    hard = P["hard"]
    if "max_long_side" in hard and (max(W, H) > hard["max_long_side"] or min(W, H) > hard["max_short_side"]):
        r.fail("Q1", where, f"{W}x{H} exceeds {hard['max_long_side']}x{hard['max_short_side']}")
    ratio = max(W, H) / min(W, H)
    if ratio > 3.0:
        r.fail("Q1", where, f"aspect {W}:{H} outside 1:3–3:1")
    if platform == "tiktok" and abs(H / W - 16 / 9) > 0.02:
        r.warn("Q1", where, f"{W}x{H} is not 9:16: TikTok letterboxes it")
    if not a:
        r.fail("Q2", where, "no audio stream")
    else:
        if a["codec_name"] != "aac" or a.get("profile") not in (None, "LC"):
            r.fail("Q2", where, f"audio {a['codec_name']} {a.get('profile')}: needs AAC LC")
        if int(a.get("channels", 2)) > 2:
            r.fail("Q2", where, f"{a['channels']} audio channels: mono or stereo only")
    if not hard["min_s"] <= dur <= hard["max_s"]:
        r.fail("Q3", where, f"{dur:.2f}s outside {hard['min_s']}–{hard['max_s']}s")
    if abs(dur - tl["duration"]) > 0.2:
        r.fail("Q3", where, f"{dur:.2f}s but the timeline is {tl['duration']:.2f}s")
    if mb > hard["max_mb"]:
        r.fail("Q3", where, f"{mb:.1f} MB > {hard['max_mb']} MB")
    if a:
        lufs, peak = loudness(path)
        want = P["encode"]["lufs"]
        if not abs(lufs - want) <= 1.5:
            r.warn("Q4", where, f"{lufs:.1f} LUFS; master to {want} (feeds turn loud files down and leave quiet ones quiet)")
        if peak > -1.0:
            r.warn("Q4", where, f"sample peak {peak:.1f} dBFS > -1.0: AAC may clip on playback")
        r.info("Q4", where, f"{lufs:.1f} LUFS, peak {peak:.1f} dBFS")
    y = first_frame_luma(path)
    if 0 <= y < 18:
        r.fail("Q5", where, f"frame 0 is black (luma {y:.0f}): the first frame is the thumbnail")
    for s, e in frozen_spans(path):
        r.warn("Q6", where, f"picture frozen {s:.1f}–{min(e, dur):.1f}s: something must move at least every 3.5 s")
    vo = [(b["vo_start"], b["vo_end"]) for b in tl["beats"] if b.get("vo_start") is not None]
    if vo and a:
        lo, hi = vo[0][0], vo[-1][1]
        for s, e in silences(path):
            if s > lo and min(e, dur) < hi:
                r.warn("Q7", where, f"{min(e, dur) - s:.1f}s of silence at {s:.1f}s inside the voice: a dead gap")
    r.info("Q0", where, f"{W}x{H} {num / den:.0f}fps {dur:.2f}s {mb:.1f}MB")
    return r


def srt(groups: list[dict]) -> str:
    def ts(t):
        ms = int(round(t * 1000))
        return f"{ms // 3600000:02}:{ms // 60000 % 60:02}:{ms // 1000 % 60:02},{ms % 1000:03}"
    out = []
    for i, g in enumerate(groups, 1):
        out.append(f"{i}\n{ts(g['s'])} --> {ts(g['e'])}\n{' '.join(t['text'] for t in g['tokens'])}\n")
    return "\n".join(out)


def contact_sheet(path: Path, out: Path, cols: int = 6) -> Path:
    info = probe(path)
    dur = float(info["format"]["duration"])
    n = cols * 3
    step = dur / n
    run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(path), "-vf",
         # select (not fps): fps relabels a later frame as 00:00:00, so the sheet showed mid-effect frames as frame 0
         f"select='isnan(prev_selected_t)+gte(t-prev_selected_t\\,{step:.3f})',scale=240:-2,drawtext=text='%{{pts\\:hms}}':x=6:y=6:fontsize=18:fontcolor=white:box=1:boxcolor=black@0.5,"
         f"tile={cols}x3:padding=4:margin=4", "-frames:v", "1", str(out)])
    return out


def spot(path: Path, t: float, out: Path) -> Path:
    run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-ss", f"{max(0.0, t):.3f}", "-i", str(path), "-frames:v", "1", "-q:v", "2", str(out)])
    return out


def post_copy(script: dict, platform: str) -> str:
    c = (script.get("platforms") or {}).get("x" if platform.startswith("x") else platform) or {}
    if platform == "tiktok":
        return c.get("caption", "").strip() + "\n"
    parts = [c.get("post", "").strip()] + [f"[reply {i}] {t.strip()}" for i, t in enumerate(c.get("thread") or [], 1)]
    return "\n\n".join(p for p in parts if p) + "\n"


def export(campaign_dir: Path, proj: Path, platforms: list[str], draft: bool = False, skip_render: bool = False) -> Report:
    meta, tl = load(proj / "mvf.json"), load(proj / "timeline.json")
    script = load(campaign_dir / meta["script"])
    holes = placeholders(proj)
    if holes and not draft:
        raise SystemExit(f"placeholder scenes left: {holes}. Author them (SCENE_GUIDE.md) or pass --draft for a rough cut")
    if meta.get("timing") != "measured" and not draft:
        raise SystemExit("timing is estimated: run `mvf voice` (or set voice.mode none) and re-scaffold before a delivery export")
    if script.get("id") and meta.get("script_sha256"):
        from .scaffold import _sha
        if _sha(campaign_dir / meta["script"]) != meta["script_sha256"]:
            raise SystemExit("the script changed after scaffold: re-run `mvf scaffold` (and `mvf voice` if lines changed)")
    master = proj / "renders/master.mp4"
    if not (skip_render and master.is_file()):
        print(f"rendering {proj.name} ({'draft' if draft else 'delivery'})…", flush=True)
        master = render(proj, draft)
    canvas = PLATFORMS["canvases"][meta["canvas"]]
    wh = (canvas["width"], canvas["height"])
    out_root = campaign_dir / "08-delivery" / script["id"] / meta["hook"] / meta["canvas"]
    rep = Report()
    summary = {"script": script["id"], "hook": meta["hook"], "canvas": meta["canvas"], "draft": draft, "files": {}}
    for p in platforms:
        d = out_root / p
        f = encode(master, d / f"{script['id']}-{meta['hook']}-{p}.mp4", p, wh)
        r = qa(f, p, tl)
        rep.extend(r)
        (d / "captions.srt").write_text(srt(tl.get("captions", [])), encoding="utf-8")
        (d / ("caption.txt" if p == "tiktok" else "post.txt")).write_text(post_copy(script, p), encoding="utf-8")
        hook_end = tl["beats"][0]["end"]
        # Frame 0 is the designed still (H0: readable at frame 0, no caption yet); later hook frames may be mid-effect.
        spot(f, 0.0, d / "cover.jpg")
        contact_sheet(f, d / "contact.jpg")
        cta = next((b for b in tl["beats"] if b["role"] == "cta"), tl["beats"][-1])
        for k, t in enumerate([0.0, hook_end - 0.1, tl["duration"] / 2, cta["start"] + 0.6, tl["duration"] - 0.2]):
            spot(f, t, d / f"spot-{k}.jpg")
        summary["files"][p] = {"video": str(f.relative_to(campaign_dir)), "fails": r.fails, "warns": r.warns, "qa": r.items}
        dump(d / "qa.json", {"platform": p, "file": f.name, "items": r.items, "listen": "PENDING LISTEN"})
    dump(out_root / "delivery.json", summary)
    shutil.copy2(proj / "STORYBOARD.md", out_root / "STORYBOARD.md")
    return rep
