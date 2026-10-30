"""The built-in `files` voice engine: lines you recorded yourself become a take.

One file per cue, named by its id — `<from>/<cue>.{wav,mp3,m4a,aac,flac,ogg}`;
`from` defaults to 07-video/<script>/recordings/ (`mvf voice --opt from=DIR`).
Each is converted to 48 kHz mono WAV, its speech span found by silence detection,
and word times spread across the span by length. Nothing hears it back: the ASR
check is empty until an engine with `rehear` (e.g. `--rehear --engine omnivoice`)
aligns the take.
"""
from __future__ import annotations

import json
import re
import subprocess
from pathlib import Path

from .common import dump, words
from .extensions import Voice

EXTS = (".wav", ".mp3", ".m4a", ".aac", ".flac", ".ogg")


def _run(cmd: list[str]) -> subprocess.CompletedProcess:
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode:
        raise SystemExit(f"command failed ({r.returncode}): {' '.join(cmd)}\n{r.stderr[-1500:]}")
    return r


def duration(path: Path) -> float:
    r = _run(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "json", str(path)])
    return float(json.loads(r.stdout)["format"]["duration"])


def speech_span(path: Path, dur: float) -> tuple[float, float]:
    """Leading and trailing silence trimmed (−40 dB for ≥ 0.12 s)."""
    r = subprocess.run(["ffmpeg", "-hide_banner", "-i", str(path), "-af", "silencedetect=noise=-40dB:d=0.12", "-f", "null", "-"],
                       capture_output=True, text=True)
    starts = [float(x) for x in re.findall(r"silence_start: (-?[\d.]+)", r.stderr)]
    ends = [float(x) for x in re.findall(r"silence_end: ([\d.]+)", r.stderr)]
    s, e = 0.0, dur
    if starts and starts[0] <= 0.01 and ends:
        s = ends[0]
    if starts and (len(ends) < len(starts) or ends[-1] >= dur - 0.02) and starts[-1] > s:
        e = starts[-1]
    return round(max(0.0, s), 3), round(min(dur, max(e, s + 0.05)), 3)


def spread(text: str, s: float, e: float) -> list[dict]:
    ws = words(text)
    if not ws:
        return []
    weight = [len(w) + 1 for w in ws]
    total, t, out = sum(weight), s, []
    for w, k in zip(ws, weight):
        d = (e - s) * k / total
        out.append({"w": w, "s": round(t, 3), "e": round(t + d, 3)})
        t += d
    return out


def find(src: Path, cue: str) -> Path | None:
    return next((src / f"{cue}{x}" for x in EXTS if (src / f"{cue}{x}").is_file()), None)


def run(mode: str, job: dict, out: Path, opts: dict) -> None:
    src = Path(opts.get("from") or Path(job["workdir"]) / "recordings").expanduser()
    missing = [c["id"] for c in job["cues"] if not find(src, c["id"])]
    if missing:
        lines = "\n".join(f"  {c['id']}: {c['text']}" for c in job["cues"] if c["id"] in missing)
        raise SystemExit(f"record these lines into {src}/<id>.wav (or .mp3/.m4a), then re-run `mvf voice`:\n{lines}")
    out.mkdir(parents=True, exist_ok=True)
    cues = []
    for c in job["cues"]:
        wav = out / f"{c['id']}.wav"
        _run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(find(src, c["id"])), "-ac", "1", "-ar", "48000", str(wav)])
        dur = duration(wav)
        s, e = speech_span(wav, dur)
        row = {"id": c["id"], "text": c["text"], "path": wav.name, "duration_s": round(dur, 3), "speech": [s, e],
               "words": spread(c["text"], s, e), "asr": {"heard": None, "dropped": [], "added": []}}
        if c.get("voice"):
            row["voice"] = c["voice"]
        cues.append(row)
    dump(out / "manifest.json", {"engine": "files", "language": job.get("language"), "voice": job.get("voice"),
                                 "settings": {"from": str(src)}, "asr_model": None, "voice_reference": None,
                                 "cues": cues, "asr_problems": [], "qa": "PENDING LISTEN"})


VOICE = Voice("Your own recordings, one file per line (07-video/<script>/recordings/<cue>.wav)", modes=("take",), run=run,
              opts={"from": "directory holding <cue>.wav|mp3|m4a (default 07-video/<script>/recordings)"})
