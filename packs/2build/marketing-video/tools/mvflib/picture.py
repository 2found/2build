"""Measured picture and sound checks on the rendered master, once per export. Small grey frames and a mono
track are enough: does the hook move, does something act in each beat once the camera's own move is taken out,
does a picture come back, does a single frame flash, is a beat louder or harsher than the rest. All WARN:
numbers stand in for eyes and ears, and the cut is still watched and listened to."""
from __future__ import annotations

import json
import subprocess
from pathlib import Path

import numpy as np

from .common import Report

W_PX = 72                # analysis frame width; height follows the aspect
HOOK_S = 2.0
HOOK_MOTION = 0.5        # % of the frame changing per quarter second under which the open reads as a still
ACTION_FLOOR = 0.25      # % of the frame changed after the best global zoom/pan: under this nothing in the picture acts
ACTION_SHARE = 0.3       # ... or under this share of the video's median beat
REPEAT_GAP_S = 6.0
REPEAT_DIFF = 2.5        # beat middles this alike (0-255 grey, normalised) are the same picture
POP_JUMP, POP_BACK = 25.0, 6.0
LOUD_LU, HARSH_DB = 3.0, 3.0
FLAT_VIDEO, FLAT_BEAT = 0.6, 0.75  # share of the frame in one flat colour: the video's median, one beat's mean
SR = 24000


def _probe(path: Path) -> tuple[int, int, float]:
    r = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height,r_frame_rate",
                        "-of", "json", str(path)], capture_output=True, text=True, check=True)
    s = json.loads(r.stdout)["streams"][0]
    num, den = map(int, s["r_frame_rate"].split("/"))
    if not den or not num:
        raise ValueError(f"frame rate {s['r_frame_rate']!r}")
    return int(s["width"]), int(s["height"]), num / den


def frames(path: Path) -> tuple[np.ndarray, float]:
    """Every frame as small grey pixels (n, h, w) and the frame rate."""
    W, H, fps = _probe(path)
    h = max(2, round(W_PX * H / W / 2) * 2)
    raw = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", str(path), "-vf", f"scale={W_PX}:{h},format=gray",
                          "-f", "rawvideo", "-"], capture_output=True, check=True).stdout
    return np.frombuffer(raw, dtype=np.uint8).reshape(-1, h, W_PX).astype(np.float32), fps


def audio(path: Path) -> np.ndarray:
    raw = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", str(path), "-f", "f32le", "-ac", "1",
                          "-ar", str(SR), "-"], capture_output=True, check=True).stdout
    return np.frombuffer(raw, dtype=np.float32).astype(np.float64)


def flat_shares(path: Path, fps: float = 1.0) -> np.ndarray:
    """Per sampled frame, the share of it that is one flat colour (the commonest of 4096 colour bins): how much
    of the frame is empty ground. 13 of 14 most-viewed references of two campaigns measured a median of 0.07-0.49; three
    cuts whose figures stood under a third of the frame height measured 0.61-0.81 (2026-11-11)."""
    w, h = 54, 96
    raw = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", str(path), "-vf", f"fps={fps},scale={w}:{h}",
                          "-pix_fmt", "rgb24", "-f", "rawvideo", "-"], capture_output=True, check=True).stdout
    q = (np.frombuffer(raw, dtype=np.uint8).reshape(-1, h * w, 3) >> 4).astype(np.int32)
    keys = (q[..., 0] << 8) | (q[..., 1] << 4) | q[..., 2]
    return np.array([np.bincount(k, minlength=4096).max() / k.size for k in keys])


def _diff(a, b) -> float:
    return float(np.abs(a - b).mean())


_WARPS: dict = {}


def _warps(h: int, w: int) -> list[np.ndarray]:
    """Flat source indices for every small zoom (±3%) and pan (±3 px) of an h×w frame."""
    if (h, w) not in _WARPS:
        ys, xs = np.mgrid[0:h, 0:w]
        cy, cx = (h - 1) / 2, (w - 1) / 2
        out = []
        for s in (0.97, 0.985, 1.0, 1.015, 1.03):
            for dy in range(-3, 4):
                for dx in range(-3, 4):
                    sy = np.clip(np.rint(cy + (ys - cy) / s + dy), 0, h - 1).astype(int)
                    sx = np.clip(np.rint(cx + (xs - cx) / s + dx), 0, w - 1).astype(int)
                    out.append((sy * w + sx).ravel())
        _WARPS[(h, w)] = out
    return _WARPS[(h, w)]


def _residual(a: np.ndarray, b: np.ndarray, mask: np.ndarray) -> float:
    """Share of the frame (%) that changed from a to b once the best small zoom and pan is taken out: the
    subject's change, not the camera's. A pixel counts when it moved more than its own edge contrast (so a
    sub-pixel misfit of the camera on a sharp edge does not count) plus a noise floor."""
    h, w = a.shape
    inner = np.zeros_like(mask)
    inner[4:-4, 4:-4] = True
    keep = (mask & inner).ravel()
    fa, fb = a.ravel(), b.ravel()[keep]
    best = min(_warps(h, w), key=lambda ix: float(np.abs(fa[ix][keep] - fb).mean()))
    g = np.zeros_like(b)
    g[:, :-1] = np.abs(np.diff(b, axis=1))
    g[:-1, :] = np.maximum(g[:-1, :], np.abs(np.diff(b, axis=0)))
    return float((np.abs(fa[best][keep] - fb) > 20 + g.ravel()[keep]).mean() * 100)


def check(path: Path, tl: dict, cap_band: tuple[float, float] | None = None) -> Report:
    """cap_band: the caption rows as shares of the frame height (captions change every word, they are not action)."""
    r = Report()
    where = Path(path).name
    try:
        fr, fps = frames(path)
    except (subprocess.CalledProcessError, OSError, ValueError, KeyError, IndexError) as e:
        r.warn("PX0", where, f"could not read the frames for the picture checks: {e}")
        return r
    n, h, w = fr.shape
    if n < 3:
        return r
    t = lambda i: i / fps  # noqa: E731
    idx = lambda s: max(0, min(n - 1, int(round(s * fps))))  # noqa: E731
    mask = np.ones((h, w), dtype=bool)
    if cap_band:
        mask[int(cap_band[0] * h):int(np.ceil(cap_band[1] * h)), :] = False
    steps = np.array([_diff(fr[i][mask], fr[i + 1][mask]) for i in range(n - 1)])

    # PX1: the hook moves
    k, q = idx(HOOK_S), max(1, int(round(fps / 4)))
    moved = [float((np.abs(fr[i] - fr[i + q]) > 15)[mask].mean() * 100) for i in range(0, max(0, k - q) + 1, q)]
    if k > q and np.mean(moved) < HOOK_MOTION:
        r.warn("PX1", where, f"0–{HOOK_S:g}s barely moves ({np.mean(moved):.2f}% of the frame changes per quarter second): "
               "the open reads as a still; move the subject, punch in on the hook word, or land the hook text on it")

    # PX2: something acts in each beat, beyond the camera's move
    beats = [b for b in tl.get("beats") or [] if b["end"] - b["start"] >= 1.5]
    act = {}
    for b in beats:
        i0, i1 = idx(b["start"] + 0.2), idx(b["end"] - 0.2)
        stride = max(1, int(round(fps / 6)))
        pairs = [(i, i + stride) for i in range(i0, i1 - stride, stride)]
        if pairs:
            act[b["id"]] = float(np.mean([_residual(fr[i], fr[j], mask) for i, j in pairs]))
    if len(act) >= 2:
        med = float(np.median(list(act.values())))
        bar = max(ACTION_FLOOR, ACTION_SHARE * med)
        still = [f"{bid} {v:.2f}%" for bid, v in act.items() if v < bar]
        if still:  # one line for the video: answer it once in warn_notes, or fix the beats it names
            r.warn("PX2", where, f"only the camera moves in {', '.join(still)} of the frame acting (under {bar:.2f}%, "
                   f"set by the video median {med:.2f}%, so it rises as the other beats gain action): give the subject one action on a word (a turn, a gesture, a prop), or a device that lands")

    # PX3: a picture comes back
    mids = [(b["id"], (b["start"] + b["end"]) / 2) for b in tl.get("beats") or []]
    seen = 0
    for i, (ba, ta) in enumerate(mids):
        for bb, tb in mids[i + 1:]:
            if tb - ta < REPEAT_GAP_S:
                continue
            fa, fb = fr[idx(ta)], fr[idx(tb)]
            za, zb = (fa - fa.mean()) / (fa.std() + 1e-6), (fb - fb.mean()) / (fb.std() + 1e-6)
            if float(np.abs(za - zb)[mask].mean()) * 32 < REPEAT_DIFF and fa.std() > 3 and seen < 3:
                seen += 1
                r.warn("PX3", where, f"{bb} at {tb:.1f}s looks like {ba} at {ta:.1f}s: a picture the viewer already saw "
                       "reads as a rerun; change the framing, the pose or the set (or say why the callback is the point)")

    # PX4: one-frame pops
    pops = [i for i in range(1, n - 1) if steps[i - 1] > POP_JUMP and steps[i] > POP_JUMP
            and _diff(fr[i - 1][mask], fr[i + 1][mask]) < POP_BACK]
    for i in pops[:5]:
        r.warn("PX4", where, f"one frame at {t(i):.2f}s differs from both neighbours: a flash frame from a scene edge or a "
               "missing asset; scrub it")

    # PX6: how much of the frame is empty ground
    try:
        fl = flat_shares(path)
    except (subprocess.CalledProcessError, OSError, ValueError):
        fl = np.zeros(0)
    if len(fl) >= 3 and float(np.median(fl)) >= FLAT_VIDEO:
        worst = []
        for b in tl.get("beats") or []:
            seg = fl[int(b["start"]):max(int(b["start"]) + 1, int(np.ceil(b["end"])))]
            if len(seg) and float(seg.mean()) >= FLAT_BEAT:
                worst.append(f"{b['id']} {seg.mean():.0%}")
        r.warn("PX6", where, f"{np.median(fl):.0%} of the frame is one flat colour through most of the video"
               + (f" ({', '.join(worst[:8])})" if worst else "") + ": the subject is small in an empty frame, and at phone "
               "size the faces and the action do not read. Give the room to the subject: frame closer (a bust or waist "
               "shot, a punch-in to the thing the line names) or draw it larger. Decoration added over the gap leaves "
               "the subject as small as it was")

    # PX5: per-beat loudness and 2-5 kHz harshness
    try:
        x = audio(path)
    except (subprocess.CalledProcessError, OSError):
        x = np.zeros(0)
    rows = []
    for b in tl.get("beats") or []:
        a, c = int(b["start"] * SR), int(b["end"] * SR)
        seg = x[a:c]
        if b["end"] - b["start"] < 1.0 or len(seg) < SR // 2 or not np.any(seg):
            continue
        rms = 20 * np.log10(np.sqrt(np.mean(seg ** 2)) + 1e-9)
        spec = np.abs(np.fft.rfft(seg * np.hanning(len(seg)))) ** 2
        f = np.fft.rfftfreq(len(seg), 1 / SR)
        harsh = 10 * np.log10((spec[(f >= 2000) & (f <= 5000)].sum() + 1e-12) / (spec[f >= 60].sum() + 1e-12))
        rows.append((b["id"], rms, harsh))
    if len(rows) >= 3:
        mr, mh = float(np.median([x_[1] for x_ in rows])), float(np.median([x_[2] for x_ in rows]))
        for bid, rms, harsh in rows:
            if abs(rms - mr) > LOUD_LU:
                r.warn("PX5", f"{where}.{bid}", f"{rms - mr:+.1f} dB against the other beats: the viewer reaches for the "
                       "volume; level the line or the cue under it")
            if harsh - mh > HARSH_DB:
                r.warn("PX5", f"{where}.{bid}", f"{harsh - mh:+.1f} dB more 2–5 kHz than the other beats: harsh on phone "
                       "speakers; soften the cue or the sibilance")
    r.info("PX0", where, f"picture checks on {n} frames at {fps:.0f} fps"
           + (f", flat ground {np.median(fl):.0%}" if len(fl) else "")
           + (f", action per beat {', '.join(f'{k} {v:.1f}' for k, v in act.items())}" if act else ""))
    return r
