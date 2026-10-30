"""Score a scaffolded project with a music bed you supply: looped or trimmed to the
runtime, levelled against the measured voice, ducked under every spoken line,
faded in and out. Numbers stand in for ears; listen before calling it done.

Writes <proj>/assets/music/bed.wav and score.json. Bring the bed yourself: a
track you own or licensed (royalty-free library, your own recording). A TikTok
trend sound is added in the app at upload, never baked here — a business account
may only use Commercial Music Library sounds, and the in-app sound carries the
trend's discovery. Cut SFX are optional: drop a full-length `sfx.wav` into
assets/music/ and re-scaffold.
"""
from __future__ import annotations

import json
from pathlib import Path

from .common import dump, load
from .export import loudness, run

OPEN_DB = 7.0        # bed this far under the voice between lines
UNDER_VO_DB = 15.0   # and this far under while the voice speaks
RAMP = 0.15          # seconds the duck takes to move


def voice_level(proj: Path, tl: dict) -> tuple[float | None, list[tuple[float, float]]]:
    """Integrated loudness of the spoken clips (duration-weighted) and the speech spans on the timeline."""
    spans, acc, total = [], 0.0, 0.0
    for b in tl["beats"]:
        if b.get("vo_start") is None:
            continue
        spans.append((b["vo_start"], b["vo_end"]))
        clip = proj / "assets/vo" / f"{b['cue']}.wav"
        if not clip.is_file():
            continue
        lufs, _ = loudness(clip)
        d = b["vo_end"] - b["vo_start"]
        if lufs == lufs and lufs > -70:  # NaN / silent clip carry no level
            acc, total = acc + lufs * d, total + d
    return (acc / total if total else None), spans


def duck_expr(spans: list[tuple[float, float]], low: float) -> str:
    """ffmpeg volume expression: 1 between lines, `low` inside a span, linear ramps of RAMP at each edge."""
    if not spans:
        return "1"
    r = RAMP
    inside = [f"clip(min((t-{s - r:.3f})/{r},({e + r:.3f}-t)/{r}),0,1)" for s, e in spans]
    m = inside[0]
    for x in inside[1:]:
        m = f"max({m},{x})"
    return f"1-{1 - low:.4f}*{m}"


def score(proj: Path, bed: Path | None) -> dict:
    tl = load(proj / "timeline.json")
    music = proj / "assets/music"
    music.mkdir(parents=True, exist_ok=True)
    if bed is None:
        (music / "bed.wav").unlink(missing_ok=True)
        report = {"bed": None, "items": [{"level": "INFO", "msg": "no bed: voice and scenes only"}]}
        dump(music / "score.json", report)
        return report
    if not bed.is_file():
        raise SystemExit(f"no bed file {bed}")
    dur = tl["duration"]
    vo_lufs, spans = voice_level(proj, tl)
    raw = music / "bed.raw.wav"
    run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-stream_loop", "-1", "-i", str(bed),
         "-t", f"{dur:.3f}", "-ac", "2", "-ar", "48000", str(raw)])
    bed_lufs, _ = loudness(raw)
    target = (vo_lufs if vo_lufs is not None else -16.0) - OPEN_DB
    gain = target - bed_lufs
    low = 10 ** ((OPEN_DB - UNDER_VO_DB) / 20)
    fade = min(0.8, dur / 4)
    af = (f"volume={gain:.2f}dB,volume='{duck_expr(spans, low)}':eval=frame,"
          f"afade=t=in:d=0.25,afade=t=out:st={max(0.0, dur - fade):.3f}:d={fade:.3f}")
    run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(raw), "-af", af, str(music / "bed.wav")])
    raw.unlink(missing_ok=True)
    items = []
    if vo_lufs is None:
        items.append({"level": "WARN", "msg": "no voice clips measured: bed set to -23 LUFS open; check it by ear"})
    items.append({"level": "INFO", "msg": f"bed {OPEN_DB:g} dB under the voice between lines, {UNDER_VO_DB:g} dB under while "
                                          f"it speaks ({len(spans)} spans)"})
    report = {"bed": str(bed), "voice_lufs": None if vo_lufs is None else round(vo_lufs, 1),
              "bed_lufs_raw": round(bed_lufs, 1), "gain_db": round(gain, 2), "open_db": OPEN_DB,
              "under_voice_db": UNDER_VO_DB, "items": items}
    (music / "score.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return report
