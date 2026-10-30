"""One timeline for a script: estimated from word rates before a voice exists,
measured from the take's manifest after `mvf voice`. Lint, scaffold and export
all read this, so a re-voiced line moves its scene, captions and checks together."""
from __future__ import annotations

import copy
from pathlib import Path

from .common import PLATFORMS, load, words

LEAD_IN = 0.10   # voice starts by 0.3 s: frame 0 already speaks
GAP = 0.12       # between beats unless a beat sets "gap"
TAIL = 0.10      # breath after a beat's last word before its scene may cut
END_HOLD = 0.8   # last frame held after the final beat
SILENT_BEAT = 1.6


def variant(script: dict, hook: str | None) -> dict:
    """The script with beat 0 replaced by hook variant `hook` (H1 = beat 0 itself)."""
    s = copy.deepcopy(script)
    if not hook or hook == "H1":
        s["beats"][0].setdefault("variant", "H1")
        return s
    for v in s.get("variants") or []:
        if v.get("id") == hook:
            b0 = s["beats"][0]
            for k in ("vo", "onscreen", "visual", "facts", "trends", "min_s"):
                if k in v:
                    b0[k] = v[k]
            b0["variant"] = hook
            return s
    raise SystemExit(f"unknown hook variant {hook}; have H1 + {[v.get('id') for v in s.get('variants') or []]}")


def wps(script: dict) -> float:
    lang = script.get("language", "default")
    base = PLATFORMS["speech_wps"].get(lang, PLATFORMS["speech_wps"]["default"])
    return base * float((script.get("voice") or {}).get("speed", 1.0))


def _estimate_words(text: str, start: float, rate: float) -> list[dict]:
    """Spread words over the line by character weight (a word ~ its letters + 1)."""
    ws = words(text)
    if not ws:
        return []
    total = len(ws) / rate
    weights = [len(w) + 1 for w in ws]
    unit = total / sum(weights)
    out, t = [], start
    for w, k in zip(ws, weights):
        out.append({"w": w, "s": round(t, 3), "e": round(t + k * unit, 3)})
        t += k * unit
    return out


def cue_id(beat: dict) -> str:
    """The take stores hook variants under their own id (H2…), beat 0's H1 under B01."""
    v = beat.get("variant")
    return v if v and v != "H1" else beat["id"]


def build(script: dict, manifest: dict | None = None) -> dict:
    """Timeline of an already-varianted script. Times in seconds from frame 0."""
    cues = {c["id"]: c for c in (manifest or {}).get("cues", [])}
    rate = wps(script)
    t = LEAD_IN
    out = []
    for i, b in enumerate(script["beats"]):
        vo = (b.get("vo") or "").strip()
        cue = cues.get(cue_id(b))
        if vo and cue:
            sp0, sp1 = cue.get("speech", [0.0, cue["duration_s"]])
            vo_dur = sp1 - sp0
            # ASR often starts the first word at 0, before the trimmed speech: never before the slot's voice.
            ws = [{"w": w["w"], "s": round(t + max(0.0, w["s"] - sp0), 3), "e": round(t + max(0.05, w["e"] - sp0), 3)}
                  for w in cue.get("words", [])]
            source = "measured"
        elif vo:
            vo_dur = len(words(vo)) / rate
            ws = _estimate_words(vo, t, rate)
            sp0, source = 0.0, "estimated"
        else:
            vo_dur, ws, sp0, source = 0.0, [], 0.0, "silent"
        dur = max(vo_dur + (TAIL if vo else 0.0), float(b.get("min_s") or (0 if vo else SILENT_BEAT)))
        dur += float(b.get("hold") or 0.0)
        start = t if i else 0.0  # beat 0 owns frame 0; its voice starts at LEAD_IN
        if out:
            out[-1]["end"] = round(t, 3)  # scene slots are contiguous: the inter-beat gap belongs to the outgoing scene
        end = t + dur
        out.append({
            "id": b["id"], "role": b.get("role"), "variant": b.get("variant"),
            "start": round(start, 3), "end": round(end, 3),
            "vo_start": round(t, 3) if vo else None, "vo_end": round(t + vo_dur, 3) if vo else None,
            "vo_trim": round(sp0, 3), "cue": cue_id(b) if vo else None, "timing": source, "words": ws,
        })
        gap = b.get("gap")
        t = end + (GAP if gap is None else float(gap))
    duration = round(out[-1]["end"] + END_HOLD, 3) if out else 0.0
    if out:
        out[-1]["end"] = duration  # the last scene holds to the end
    kinds = {b["timing"] for b in out} - {"silent"}
    return {"duration": duration, "timing": "measured" if kinds == {"measured"} else ("mixed" if "measured" in kinds else "estimated"),
            "beats": out}


def load_manifest(take: Path) -> dict | None:
    m = Path(take) / "manifest.json"
    return load(m) if m.is_file() else None
