"""Sound for a directed project: the SFX track from sfx.json, a procedural mood bed,
and a mix check against the measured voice.

Sources, first match wins, per cue kind:
1. `<campaign>/sfx/<kind>.(wav|mp3|flac)`: the user's own or licensed one-shots.
2. a recorded library one-shot (LIBRARY, royalty-free, commercial use allowed),
   when present on this machine.
3. `synth` (procedural, seeded).
A cue's `db` is its peak level; SFX dip SFX_UNDER_VO_DB under speech so the voice
always reads. Numbers stand in for ears: the report says PENDING LISTEN."""
from __future__ import annotations

import subprocess
import wave
from pathlib import Path

import numpy as np

from . import synth
from .common import dump, load

SR = synth.SR
LIB_DIR = Path.home() / ".claude/skills/media-use/audio/assets/sfx"
LIBRARY = {"whoosh": "whoosh.mp3", "whoosh-short": "whoosh-short.mp3", "whoosh-cinematic": "whoosh-cinematic.mp3",
           "riser": "riser.mp3", "typing": "typing.mp3", "impact": "impact-bass-1.mp3", "impact-2": "impact-bass-2.mp3",
           "chime": "chime.mp3", "sparkle": "sparkle.mp3", "pop": "pop.mp3", "click": "click.mp3", "ping": "ping.mp3"}
SFX_UNDER_VO_DB = 6.0
MIX_MARGIN_DB = 8.0       # voice over (bed + sfx) inside every spoken span
MOODS = sorted(synth.MOODS)


def decode(path: Path) -> np.ndarray:
    # decode as stereo and average: ffmpeg's own 2→1 downmix adds 3 dB to a centred signal
    raw = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", str(path), "-f", "f32le", "-ac", "2",
                          "-ar", str(SR), "-"], check=True, capture_output=True).stdout
    return np.frombuffer(raw, dtype=np.float32).astype(np.float64).reshape(-1, 2).mean(axis=1)


def write(path: Path, x: np.ndarray) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    peak = np.max(np.abs(x)) if len(x) else 0
    if peak > 0.98:                         # never clip; a cue over full scale is a level bug, report it upstream
        x = x * (0.98 / peak)
    st = np.repeat((np.clip(x, -1, 1) * 32767).astype("<i2")[:, None], 2, axis=1)
    with wave.open(str(path), "wb") as w:
        w.setnchannels(2)
        w.setsampwidth(2)
        w.setframerate(SR)
        w.writeframes(st.tobytes())


def sample(kind: str, seed: int, dur: float | None, campaign: Path | None) -> tuple[np.ndarray, str]:
    for ext in ("wav", "mp3", "flac"):
        p = campaign / "sfx" / f"{kind}.{ext}" if campaign else None
        if p and p.is_file():
            return synth.norm(decode(p)), f"campaign:{p.name}"
    if kind in LIBRARY and (LIB_DIR / LIBRARY[kind]).is_file():
        return synth.norm(decode(LIB_DIR / LIBRARY[kind])), f"library:{LIBRARY[kind]}"
    if kind in synth.AMBIENT:
        return synth.AMBIENT[kind](max(dur or 4.0, 0.5), seed), "synth"
    if kind in synth.TIMED:
        return synth.norm(synth.TIMED[kind](dur or 1.2, seed)), "synth"
    if kind in synth.ONE_SHOTS:
        return synth.norm(synth.ONE_SHOTS[kind](seed)), "synth"
    raise KeyError(kind)


def speech_spans(tl: dict) -> list[tuple[float, float]]:
    return [(b["vo_start"], b["vo_end"]) for b in tl["beats"] if b.get("vo_start") is not None]


def duck_curve(n: int, spans, db: float, ramp: float = 0.12) -> np.ndarray:
    t = np.arange(n) / SR
    m = np.zeros(n)
    for s, e in spans:
        m = np.maximum(m, np.clip(np.minimum((t - (s - ramp)) / ramp, ((e + ramp) - t) / ramp), 0, 1))
    return 1 - (1 - 10 ** (-db / 20)) * m


def render_sfx(proj: Path, campaign: Path | None = None) -> dict:
    """assets/music/sfx.wav from sfx.json; returns the report (counts, sources, unknown kinds)."""
    tl = load(proj / "timeline.json")
    spec = load(proj / "sfx.json") if (proj / "sfx.json").is_file() else {"cues": []}
    n = int((tl["duration"] + 0.05) * SR)
    buf = np.zeros(n)
    used, unknown = {}, []
    for i, c in enumerate(spec.get("cues") or []):
        try:
            sig, src = sample(c["kind"], 101 + i * 7, c.get("dur"), campaign)
        except KeyError:
            unknown.append(c["kind"])
            continue
        if c.get("dur") and c["kind"] not in synth.AMBIENT and c["kind"] not in synth.TIMED:
            sig = sig[: int((c["dur"] + 0.4) * SR)]
        if c["kind"] in synth.AMBIENT:
            sig = sig * synth.env(len(sig), 0.6, 0.8)
        synth.place(buf, sig, float(c["t"]), 10 ** (float(c.get("db", -22)) / 20))
        used[src] = used.get(src, 0) + 1
    buf *= duck_curve(n, speech_spans(tl), SFX_UNDER_VO_DB)
    out = proj / "assets/music/sfx.wav"
    if not spec.get("cues"):
        out.unlink(missing_ok=True)
    else:
        write(out, buf)
    return {"cues": len(spec.get("cues") or []), "sources": used, "unknown": sorted(set(unknown)),
            "peak_db": round(20 * np.log10(max(np.max(np.abs(buf)), 1e-9)), 1) if n else None}


def auto_bed(proj: Path, mood: str, seed: int = 7) -> Path:
    """A procedural bed in one mood at the project's runtime, written as the bed source for mix.score."""
    if mood not in synth.MOODS:
        raise SystemExit(f"mood {mood!r}: one of {MOODS}")
    tl = load(proj / "timeline.json")
    x = synth.reverb(synth.mood_bed(mood, tl["duration"] + 0.5, seed), 2.4, 0.22, seed)
    p = proj / "assets/music/bed.src.wav"
    write(p, synth.norm(x, 0.7))
    return p


PRODUCT = "product-audio.json"   # <proj>/: {"clip": path, "in": s, "at": [[s, e], …]}
PRODUCT_OPEN_S = 5.0             # with no `at`, the product is heard in the pauses of the first seconds
PRODUCT_MIN_GAP = 0.4
PRODUCT_AUDIBLE_DB = -40.0


def product_track(proj: Path, campaign: Path | None = None) -> dict | None:
    """assets/music/product.wav: the product's own recorded sound (a narration, a player, a game),
    at the voice's level inside its windows and silent under every spoken line. The clip plays on
    from window to window, so a second window continues the recording instead of restarting it."""
    out = proj / "assets/music/product.wav"
    if not (proj / PRODUCT).is_file():
        return None
    spec, tl = load(proj / PRODUCT), load(proj / "timeline.json")
    clip = Path(spec["clip"])
    clip = clip if clip.is_absolute() else (campaign or proj) / clip
    if not clip.is_file():
        raise SystemExit(f"product audio {clip} is missing (named in {proj / PRODUCT})")
    n = int((tl["duration"] + 0.05) * SR)
    spans = sorted(speech_spans(tl))
    wins = [tuple(w) for w in spec.get("at") or []]
    if not wins:
        edges = [0.0] + [x for s, e in spans for x in (s, e)] + [tl["duration"]]
        wins = [(a, min(b, PRODUCT_OPEN_S)) for a, b in zip(edges[::2], edges[1::2])
                if a < PRODUCT_OPEN_S and min(b, PRODUCT_OPEN_S) - a >= PRODUCT_MIN_GAP]
    x = decode(clip)[int(float(spec.get("in") or 0) * SR):]
    voice = _voice(proj, tl, n)
    said = np.concatenate([voice[int(s * SR):int(e * SR)] for s, e in spans]) if spans else np.zeros(0)
    used = x[: int(sum(e - s for s, e in wins) * SR)]
    gain = 10 ** ((_db(said) - _db(used)) / 20) if said.any() and used.any() else 1.0
    buf, pos = np.zeros(n), 0
    for s, e in wins:
        seg = x[pos:pos + int((e - s) * SR)]
        pos += len(seg)
        synth.place(buf, seg * synth.env(len(seg), 0.04, 0.08), s, gain)
    buf *= duck_curve(n, spans, 60.0, 0.06)
    write(out, buf)
    rep = {"clip": str(clip), "windows": [[round(s, 2), round(e, 2)] for s, e in wins], "gain_db": round(20 * np.log10(gain), 1)}
    dump(out.with_suffix(".json"), rep)
    return rep


def _voice(proj: Path, tl: dict, n: int) -> np.ndarray:
    v = np.zeros(n)
    for b in tl["beats"]:
        if b.get("vo_start") is None or not b.get("cue"):
            continue
        clips = list((proj / "assets/vo").glob(f"{b['cue']}.*"))
        if not clips:
            continue
        x = decode(clips[0])
        synth.place(v, x[int(b.get("vo_trim", 0) * SR):], b["vo_start"])
    return v


def _db(x: np.ndarray) -> float:
    return 20 * np.log10(max(np.sqrt(np.mean(x ** 2)) if len(x) else 0, 1e-9))


SPEECH_BODY_DB = 12.0      # a window this far under the line's loudest is a word tail or a breath, not speech


def span_margin(voice: np.ndarray, mus: np.ndarray) -> float | None:
    """Worst voice-over-music margin in 100 ms windows of one spoken line. A single hit under one word
    matters and the span average hides it; a window under SPEECH_BODY_DB below the line's loudest is a
    word tail or a breath, and counting it reports a bed as burying a voice that is not speaking."""
    w = SR // 10
    rows = [(_db(voice[k:k + w]), _db(mus[k:k + w])) for k in range(0, len(voice) - w + 1, w)]
    if not rows:
        return None
    floor = max(v for v, _ in rows) - SPEECH_BODY_DB
    m = [v - b for v, b in rows if v >= floor]
    return min(m) if m else None


def mixcheck(proj: Path) -> dict:
    """Voice over (bed + sfx) RMS in every spoken span; the worst margin and the spans under MIX_MARGIN_DB."""
    tl = load(proj / "timeline.json")
    n = int((tl["duration"] + 0.05) * SR)
    mus = np.zeros(n)
    prod = None
    for name in ("bed.wav", "sfx.wav", "product.wav"):
        p = proj / "assets/music" / name
        if p.is_file():
            x = decode(p)
            mus[: min(n, len(x))] += x[:n]
            if name == "product.wav":
                prod = np.pad(x[:n], (0, max(0, n - len(x))))
    voice = _voice(proj, tl, n)
    rows, bad = [], []
    for b in tl["beats"]:
        if b.get("vo_start") is None:
            continue
        i, j = int(b["vo_start"] * SR), int(b["vo_end"] * SR)
        if j - i < SR // 10:
            continue
        worst = span_margin(voice[i:j], mus[i:j]) if voice[i:j].any() else None
        rows.append({"beat": b["id"], "margin_db": None if worst is None else round(float(worst), 1)})
        if worst is not None and worst < MIX_MARGIN_DB:
            bad.append(b["id"])
    rep = {"spans": rows, "under": bad, "min_db": min((r["margin_db"] for r in rows if r["margin_db"] is not None), default=None),
           "target_db": MIX_MARGIN_DB, "status": "PENDING LISTEN"}
    if prod is not None:
        # the first 100 ms where the product's own sound is audible and louder than the voice
        w = SR // 10
        rep["product_first_s"] = next((round(k / SR, 1) for k in range(0, n - w + 1, w)
                                       if _db(prod[k:k + w]) > max(PRODUCT_AUDIBLE_DB, _db(voice[k:k + w]))), None)
    dump(proj / "assets/music/mixcheck.json", rep)
    return rep
