"""Direction: the look and the shots of one script, as data (`07-video/<sid>/direction.json`).

A beat is either directed here — a list of shots (an approved art plate, a real screen,
footage, a colour) with a camera move, a transition in, word-anchored punch-ins and
effects — or "custom", a scene authored by hand. `mvf scaffold` resolves every anchor
against the measured voice and writes each directed beat as a generated scene
(`data-mvf="generated"`, rewritten every run) that plays on the mvkit runtime
(templates/kit/mvkit.js). Re-voicing moves every cut, punch-in and slam with its word.

Anchors (beat-local unless noted):
  1.25            seconds from the beat's start      start | end | 40%
  word:mười       start of the first word "mười" (accents and case folded; a phrase
                  "word:mười năm" matches the run); #2 = the second match; ±0.1 shifts it
  wordend:năm     end of that word (or phrase)       w:3 / wend:3  the 4th word's start / end
  B03@word:mưa    (sound, chip) a beat-prefixed anchor; plain seconds are absolute there
"""
from __future__ import annotations

import html
import json
import math
import random
import re
import unicodedata
import zlib
from pathlib import Path

from . import rig
from .common import STYLES, Report, campaign_style, load

DIRECTION = "direction.json"

# transition -> default seconds
TRANSITIONS = {"cut": 0.0, "flash": 0.28, "dip": 0.18, "whip": 0.26, "zoom": 0.32, "fade": 0.4, "blur": 0.3,
               "bloom": 0.6, "wipe": 0.45, "iris": 0.32, "push": 0.34}
OVERLAP = {"fade": 1.0, "blur": 0.5, "bloom": 1.0, "wipe": 1.0}  # starts before the cut point by this share of its length
HOLD_PREV = {"whip", "iris", "push"}  # the outgoing shot stays under the incoming until it covers the frame
OUT_FOR = {"whip": ("whip", 0.12), "zoom": ("zoom", 0.14)}
HARD = {"cut"}
CAMS = {"push", "pull", "pan-left", "pan-right", "tilt-up", "tilt-down", "drift", "hold"}
FX_DUR = {"slam": 1.2, "title": 1.8, "type": 2.0, "stamp": 1.1, "label": 2.2, "counter": 1.8, "flash": 0.3, "black": 0.12,
          "shake": 0.4, "punch": 0.43, "dim": 0.4, "weather": None, "endtag": None,
          # explain and show: a region called out on a screen, a tap, steps, a two-way compare, bars, a quote, a name
          "callout": 2.4, "tap": 0.7, "steps": None, "compare": None, "bars": None, "quote": None, "lower": 3.0,
          # anime: manga speed lines, radial (focus lines on an impact) or linear (motion streaks)
          "speedlines": 0.6,
          # comic: a hand-lettered label with a leader line drawn on to the point it names
          "pointer": 2.4,
          # devices that tell a number: a grid of dots (lit of total), two ladders whose rungs do not line up
          "dots": None, "ladder": None,
          # raw: any markup the agent writes (HTML or inline SVG), timed and faded like the rest
          "html": 2.0}
BLOCK_FX = {"steps", "compare", "bars", "quote", "dots", "ladder"}  # read blocks: on a wide canvas they keep a readable measure
POS_KEYS = ("rect", "x", "y", "size")  # any of these may be {canvas: value} when one number cannot fit every frame
TEXT_FX = {"slam", "title", "type", "stamp", "label", "counter", "endtag", "steps", "compare", "bars", "quote", "lower",
           "dots", "ladder"}
TEXT_SIZE = {"slam": 128, "title": 92, "type": 54, "stamp": 96, "label": 60, "counter": 190, "endtag": 104,
             "steps": 58, "compare": 56, "bars": 46, "quote": 70, "lower": 48, "dots": 48, "ladder": 40}
TEXT_Y = {"slam": 0.24, "title": 0.2, "type": 0.62, "stamp": 0.3, "label": 0.16, "counter": 0.28, "endtag": None,
          "steps": 0.2, "compare": 0.2, "bars": 0.22, "quote": 0.22, "lower": 0.6, "dots": 0.2, "ladder": 0.18}
WEATHER = {"rain": 90, "snow": 60, "embers": 40, "petals": 22}
OVER = 0.06  # .mvk-kb overhang per side (share of the frame)
EVENT_GAP_WARN = 3.5

DEFAULT_LOOK = {
    # a warm paper ground, not the dark-navy void every generated video falls into
    "colors": {"ground": "#efe7d6", "ink": "#16130f", "accent": "#c2412d", "hot": "#e9b44c", "on_plate": "#fbf7ef"},
    "fonts": {"display": "Montserrat", "body": "Inter", "caption": "Montserrat", "mono": "IBM Plex Mono"},
    "grade": {"grain": 0.06, "vignette": 0.35},
    "chip": None, "letterbox": False, "slam_case": "uppercase",
}
DEFAULT_CAPTIONS = {"style": "stroke", "size": None, "words": 3, "y": None, "font": None, "weight": 850}

# sound placed for what the picture does (sfx.py renders the kinds)
SFX_FOR_IN = {"whip": ("whip", -18, -0.10), "zoom": ("whoosh", -21, -0.18), "flash": ("hit", -17, 0.0),
              "dip": ("thump", -22, 0.0), "bloom": ("brush", -24, -0.25), "wipe": ("brush", -24, -0.2),
              "iris": ("whoosh-short", -22, -0.05), "push": ("whoosh", -22, -0.12), "blur": ("air", -27, -0.15)}
SFX_FOR_FX = {"slam": ("hit", -17, -0.02), "stamp": ("stamp", -18, 0.0), "counter": ("ticks", -26, 0.0),
              "label": ("swish", -26, -0.05), "title": ("air", -27, -0.05), "type": ("typing", -26, 0.0),
              "shake": ("rumble", -20, 0.0), "punch": ("thump", -20, -0.02), "endtag": ("bell", -22, 0.05),
              "flash": ("hit", -18, 0.0), "callout": ("swish", -26, -0.05), "tap": ("click", -20, 0.0),
              "steps": ("pop", -24, 0.0), "compare": ("swish", -26, -0.05), "bars": ("swish", -27, 0.0),
              "quote": ("air", -27, -0.05), "lower": ("swish", -27, -0.05), "speedlines": ("whoosh-short", -24, -0.04),
              "dots": ("ticks", -27, 0.0), "ladder": ("pop", -25, 0.0)}
PER_ITEM_SFX = {"steps", "compare"}  # one sound per item that lands, not one for the block

# 2.5D layers: cut-outs over a shot's plate, each with its own depth and motion
LAYER_IN = {"fade": 0.4, "pop": 0.45, "slide-left": 0.6, "slide-right": 0.6, "rise": 0.6, "drop": 0.55}
IDLE = {"breathe": 3.2, "bob": 1.6, "sway": 3.0}  # seconds per cycle
DEVICE_FX = {"counter", "bars", "compare", "steps", "dots", "ladder", "html", "callout", "pointer"}  # show a number or relation
STEADY_CAMS = {"drift", "hold", "custom", "none"}  # repeating these is the absence of a move, not a repeat
ANTIC = 0.08  # a move first leans back this share of its travel, then overshoots and settles (opt out: antic: false)
KEY_SFX = ("hit", -19)  # a key line's impact, at its last word, unless a cue is already there
KEY_SFX_NEAR = 0.3
HIT_FROM = {"left": (1, 0, 2.5), "right": (-1, 0, -2.5), "front": (0, -1, 0.0)}  # recoil away: x sign, y sign, lean deg
LAYER_PARTS = {"layer": ".mvk-ly", "move": ".mvk-lym", "hit": ".mvk-lyh", "idle": ".mvk-lyi", "img": ".mvk-lyi > .mvk-lyart",
               "rig": ".mvk-rig"}
BLENDS = {"normal", "multiply", "screen", "overlay", "darken", "lighten", "color-dodge", "color-burn", "hard-light",
          "soft-light", "difference", "exclusion", "hue", "saturation", "color", "luminosity"}
LAYER_Z = 0.25  # default depth: 0 moves with the plate, 1 moves twice as far (near the lens)


def path(cd: Path, sid: str) -> Path:
    return cd / "07-video" / sid / DIRECTION


def load_direction(cd: Path, sid: str) -> dict | None:
    """direction.json, its look taking the campaign's `style` when it names no preset of its own."""
    p = path(cd, sid)
    if not p.is_file():
        return None
    from . import formats
    d = formats.under(load(p), formats.defaults(cd))  # a saved format's look/captions/ambience under the direction's own
    st = campaign_style(cd)
    if st and isinstance(d, dict) and "preset" not in (d.get("look") or {}):
        d["look"] = {**(d.get("look") or {}), "preset": st}
    return d


def look_of(d: dict | None, brand: dict) -> dict:
    """The resolved look: direction.look over the campaign brand over the defaults."""
    lk = json.loads(json.dumps(DEFAULT_LOOK))
    bc = brand.get("colors") or {}
    # a brand palette maps onto the look's roles when the direction does not set them
    for role, keys in (("ground", ("ground", "bg")), ("ink", ("ink", "fg")), ("accent", ("accent",)), ("hot", ("hot",))):
        for k in keys:
            if k in (brand.get("colors_set") or {}):
                lk["colors"][role] = bc[k]
                break
    own = dict((d or {}).get("look") or {})
    pre = (STYLES.get(own.pop("preset", None)) or {}).get("look") or {}
    # a style preset sits over the brand; whatever the direction's look writes itself wins
    for src in (pre, own):
        _merge_look(lk, src)
    return lk


def _merge_look(lk: dict, src: dict) -> None:
    for k, v in src.items():
        if isinstance(v, dict) and isinstance(lk.get(k), dict):
            lk[k] = {**lk[k], **v}
        else:
            lk[k] = v


def preset_of(d: dict | None) -> dict:
    """The styles.json entry the direction's look names, or {}."""
    return STYLES.get((((d or {}).get("look") or {}).get("preset"))) or {}


def captions_of(d: dict | None) -> dict:
    return {**DEFAULT_CAPTIONS, **(preset_of(d).get("captions") or {}), **((d or {}).get("captions") or {})}


# ---------- anchors ----------

def fold(s: str) -> str:
    s = unicodedata.normalize("NFD", s.lower().replace("đ", "d").replace("Đ", "d"))
    return re.sub(r"[^\w]", "", "".join(c for c in s if unicodedata.category(c) != "Mn"))


ANCHOR = re.compile(r"^(word|wordend|w|wend):(.+?)(?:#(\d+))?(?:\s*([+-]\s*\d+(?:\.\d+)?))?$")


class AnchorError(ValueError):
    pass


def resolve(anchor, beat: dict, lead: float) -> float:
    """A beat-local anchor -> seconds on the scene's own clock (0 = the host's start = beat start - lead)."""
    dur = beat["end"] - beat["start"]
    if isinstance(anchor, (int, float)):
        return round(float(anchor) + lead, 3)
    a = str(anchor).strip()
    tail = re.match(r"^(.*?[^\s+-])((?:\s*[+-]\s*\d+(?:\.\d+)?){2,})$", a)
    if tail:  # several shifts in a row (word:x-0.36+0.02): their sum
        return round(resolve(tail.group(1), beat, lead)
                     + sum(float(v.replace(" ", "")) for v in re.findall(r"[+-]\s*\d+(?:\.\d+)?", tail.group(2))), 3)
    off = 0.0
    m = re.match(r"^(start|end|\d+(?:\.\d+)?%)\s*([+-]\s*\d+(?:\.\d+)?)?$", a)
    if m:
        off = float(m.group(2).replace(" ", "")) if m.group(2) else 0.0
        base = 0.0 if m.group(1) == "start" else dur if m.group(1) == "end" else dur * float(m.group(1)[:-1]) / 100
        return round(base + off + lead, 3)
    m = ANCHOR.match(a)
    if not m:
        try:
            return round(float(a) + lead, 3)
        except ValueError:
            raise AnchorError(f"anchor {anchor!r}: seconds, start, end, N%, word:<text>[#n][±s], wordend:, w:<k>, wend:<k>")
    kind, body, nth, shift = m.group(1), m.group(2).strip(), int(m.group(3) or 1), m.group(4)
    off = float(shift.replace(" ", "")) if shift else 0.0
    ws = beat["words"]
    if not ws:
        raise AnchorError(f"anchor {anchor!r}: beat {beat['id']} has no words")
    if kind in {"w", "wend"}:
        k = int(body)
        if not 0 <= k < len(ws):
            raise AnchorError(f"anchor {anchor!r}: beat {beat['id']} has words 0..{len(ws) - 1}")
        t = ws[k]["s"] if kind == "w" else ws[k]["e"]
        return round(t - beat["start"] + off + lead, 3)
    want = [fold(x) for x in body.split() if fold(x)]
    have = [fold(w["w"]) for w in ws]
    hits = [i for i in range(len(have) - len(want) + 1) if have[i:i + len(want)] == want]
    if len(hits) < nth:
        raise AnchorError(f"anchor {anchor!r}: {'no' if not hits else f'only {len(hits)}'} match in {beat['id']}'s words: "
                          + " ".join(w["w"] for w in ws))
    i = hits[nth - 1]
    t = ws[i]["s"] if kind == "word" else ws[i + len(want) - 1]["e"]
    return round(t - beat["start"] + off + lead, 3)


def resolve_global(anchor, beats: dict) -> float:
    """Absolute seconds: a number, or "<BEAT>@<beat anchor>"."""
    if isinstance(anchor, (int, float)):
        return float(anchor)
    a = str(anchor)
    if "@" in a:
        bid, rest = a.split("@", 1)
        if bid not in beats:
            raise AnchorError(f"anchor {anchor!r}: no beat {bid}")
        b = beats[bid]
        return round(resolve(rest, b, 0.0) + b["start"], 3)
    if a == "end":
        return max(b["end"] for b in beats.values())
    try:
        return float(a)
    except ValueError:
        raise AnchorError(f"anchor {anchor!r}: absolute seconds or <BEAT>@<anchor>")


# ---------- camera ----------

def _clamp(v, lim):
    return max(-lim, min(lim, v))


def drift_range(s: float, over: float = OVER) -> float:
    """Largest |translate| (share of the frame) that keeps a .mvk-kb at scale s covering the frame."""
    return max(0.0, ((1 + 2 * over) * s - 1) / 2)


def camera(cam, focus) -> dict | None:
    """A preset name (or {from, to, ease}) -> clamped {from:{s,x,y}, to:{s,x,y}, ease}."""
    if cam in (None, False):
        return None
    fx, fy = focus or (0.5, 0.5)
    if isinstance(cam, dict):
        f = {"s": 1.0, "x": 0.0, "y": 0.0, **(cam.get("from") or {})}
        t = {"s": 1.0, "x": 0.0, "y": 0.0, **(cam.get("to") or {})}
        ease = cam.get("ease", "sine.inOut")
    else:
        amt = 1.0
        m = re.match(r"^([a-z-]+)(?::(\d+(?:\.\d+)?))?$", str(cam))
        if not m or m.group(1) not in CAMS:
            raise ValueError(f"cam {cam!r} not in {sorted(CAMS)} (optionally :<strength>, e.g. push:1.5)")
        name, amt = m.group(1), float(m.group(2) or 1.0)
        ease = "sine.inOut" if name in {"drift", "hold"} else "none"
        z = 0.09 * amt
        if name == "push":
            f, t = {"s": 1.0}, {"s": 1.0 + z, "x": -(fx - 0.5) * z * 1.6, "y": -(fy - 0.5) * z * 1.6}
        elif name == "pull":
            f, t = {"s": 1.0 + z, "x": -(fx - 0.5) * z * 1.6, "y": -(fy - 0.5) * z * 1.6}, {"s": 1.0}
        elif name in {"pan-left", "pan-right"}:
            s = 1.0 + 0.05 * amt
            r = drift_range(s) * 0.85
            f, t = ({"s": s, "x": r}, {"s": s, "x": -r}) if name == "pan-left" else ({"s": s, "x": -r}, {"s": s, "x": r})
        elif name in {"tilt-up", "tilt-down"}:
            s = 1.0 + 0.05 * amt
            r = drift_range(s) * 0.85
            f, t = ({"s": s, "y": -r}, {"s": s, "y": r}) if name == "tilt-up" else ({"s": s, "y": r}, {"s": s, "y": -r})
        elif name == "drift":
            f, t = {"s": 1.02, "x": 0.01 * amt}, {"s": 1.02 + 0.035 * amt, "x": -0.012 * amt}
        else:
            f, t = {"s": 1.0}, {"s": 1.0}
    out = {}
    for k, v in (("from", f), ("to", t)):
        s = max(1.0, float(v.get("s", 1.0)))
        lim = drift_range(s)
        out[k] = {"s": round(s, 4), "x": round(_clamp(float(v.get("x", 0.0)), lim), 4),
                  "y": round(_clamp(float(v.get("y", 0.0)), lim), 4)}
    out["ease"] = ease
    return out


def key_to(k: dict) -> dict:
    """A punch-in on .mvk-kbo: scale s about the centre, the focus point pulled toward it, never past an edge."""
    s = max(1.0, float(k.get("s", 1.15)))
    fx, fy = k.get("focus") or (0.5, 0.5)
    lim = (s - 1) / 2
    return {"s": round(s, 4), "x": round(_clamp(-(fx - 0.5) * s, lim), 4), "y": round(_clamp(-(fy - 0.5) * s, lim), 4)}


# ---------- resolve a direction against a timeline ----------

def _tr(v) -> dict:
    if v is None:
        return {"type": "cut", "dur": 0.0}
    t = {"type": v} if isinstance(v, str) else dict(v)
    if t.get("type") not in TRANSITIONS:
        raise ValueError(f"transition {t.get('type')!r} not in {sorted(TRANSITIONS)}")
    t.setdefault("dur", TRANSITIONS[t["type"]])
    return t


def _lead(tr: dict) -> float:
    return round(tr["dur"] * OVERLAP.get(tr["type"], 0.0), 3)


def _hot(text: str) -> str:
    """*word* marks the hot words of a text effect."""
    return re.sub(r"\*([^*]+)\*", r'<span class="hot">\1</span>', html.escape(text))


def plan(d: dict, tl: dict, geo: dict, art: dict, media: dict) -> tuple[dict, Report]:
    """Every directed beat resolved to a scene spec on its own clock, plus host lead/tail, sound cues and the
    visual events the checks read. Raises nothing for a bad beat: it is reported and left as a placeholder.

    art: id -> {"path": project-relative src, "approved": bool}; media: campaign-relative src -> project src."""
    rep = Report()
    if captions_of(d).get("y"):  # captions moved for the whole video: the text and face checks read that band
        geo = {**geo, "cap_top": int(captions_of(d)["y"])}
    beats = {b["id"]: b for b in tl["beats"]}
    order = [b["id"] for b in tl["beats"]]
    spec_beats = (d or {}).get("beats") or {}
    for bid in spec_beats:
        if bid not in beats:
            rep.fail("D0", f"direction.{bid}", f"no beat {bid} in the script ({', '.join(order)})")
    W, H = geo["W"], geo["H"]
    out = {"beats": {}, "sfx": [], "events": [], "transitions": [], "custom": [], "missing": [], "draft_art": [], "shotlog": [],
           "preset": preset_of(d), "rigs": (d or {}).get("rigs") or {}, "props": (d or {}).get("props") or {}}
    # pass 1: each beat's shots with their own transitions; the beat lead comes from its first shot's way in
    for i, bid in enumerate(order):
        bs = spec_beats.get(bid)
        if bs is None:
            out["missing"].append(bid)
            continue
        if bs == "custom" or (isinstance(bs, dict) and bs.get("custom")):
            out["custom"].append(bid)
            continue
        b = beats[bid]
        shots = bs.get("shots") if isinstance(bs, dict) else bs
        if not shots:
            rep.fail("D1", f"direction.{bid}", "a directed beat needs shots: [...] (or \"custom\")")
            continue
        try:
            first = _tr(shots[0].get("in"))
        except ValueError as e:
            rep.fail("D2", f"direction.{bid}.s0", str(e))
            continue
        if i == 0 and first["type"] != "cut":
            rep.warn("D2", f"direction.{bid}.s0", f"the first frame is the thumbnail: '{first['type']}' in from nothing; cut in")
            first = {"type": "cut", "dur": 0.0}
        lead = _lead(first) if i else 0.0
        out["beats"][bid] = {"lead": lead, "tail": 0.0, "first_in": first, "raw": bs, "shots": shots}
    # pass 2: resolve each beat (the tail a next beat's whip/iris/push needs is known now)
    for i, bid in enumerate(order):
        if bid not in out["beats"]:
            continue
        nxt = order[i + 1] if i + 1 < len(order) else None
        if nxt in out["beats"] and out["beats"][nxt]["first_in"]["type"] in HOLD_PREV:
            out["beats"][bid]["tail"] = out["beats"][nxt]["first_in"]["dur"]
        _resolve_beat(bid, beats[bid], out["beats"][bid], out["beats"].get(nxt), geo, art, media, rep, out)
    out["events"].sort()
    _sound_globals(d, beats, rep, out)
    _key_cues(spec_beats, tl, out)
    return out, rep


def _key_cues(spec_beats, tl, out):
    """A key line lands on one sound at its last word, unless a cue is already there or the beat says key_sfx: false."""
    for b in tl["beats"]:
        bs = spec_beats.get(b["id"])
        if not b.get("key") or b.get("vo_end") is None or (isinstance(bs, dict) and bs.get("key_sfx") is False):
            continue
        t = b["vo_end"]
        if any(abs(c["t"] - t) <= KEY_SFX_NEAR for c in out["sfx"] if not c.get("bed")):
            continue
        kind, db = KEY_SFX
        if isinstance(bs, dict) and isinstance(bs.get("key_sfx"), str):
            kind = bs["key_sfx"]
        out["sfx"].append({"t": round(t, 3), "kind": kind, "db": db, "src": f"{b['id']}.key"})


def _resolve_beat(bid, b, slot, nxt_slot, geo, art, media, rep, out):
    W, H = geo["W"], geo["H"]
    lead, tail = slot["lead"], slot["tail"]
    dur = round(b["end"] - b["start"] + lead + tail, 3)  # the scene's own clock runs over the host
    end_local = round(b["end"] - b["start"] + lead, 3)
    raw = slot["raw"] if isinstance(slot["raw"], dict) else {"shots": slot["raw"]}
    shots, fxs, sounds = [], [], []
    where = lambda k: f"direction.{bid}.s{k}"

    def at(a, w, default=None):
        try:
            return resolve(a if a is not None else default, b, lead)
        except AnchorError as e:
            rep.fail("D3", w, str(e))
            return None

    # shots
    starts = []
    for k, sh in enumerate(slot["shots"]):
        t0 = lead if k == 0 else at(sh.get("at"), where(k))
        if k and sh.get("at") is None:
            rep.fail("D3", where(k), "every shot after the first needs at: (a word anchor lands the cut on its word)")
        starts.append(t0)
    for k in range(1, len(starts)):
        if starts[k] is None or starts[k - 1] is None:
            continue
        if starts[k] <= starts[k - 1]:
            rep.fail("D4", where(k), f"starts at {starts[k]:.2f}s, not after shot {k - 1} ({starts[k - 1]:.2f}s)")
        elif starts[k] <= starts[k - 1] + 0.2:
            rep.warn("D4", where(k), f"starts {starts[k] - starts[k - 1]:.2f}s after shot {k - 1}: a flash cut, "
                                     f"unreadable unless that is the point")
    if any(s is None for s in starts):
        slot["error"] = True
        return
    for k, sh in enumerate(slot["shots"]):
        sid = f"{bid}-s{k}"
        try:
            tr = slot["first_in"] if k == 0 else _tr(sh.get("in"))
        except ValueError as e:
            rep.fail("D2", where(k), str(e))
            tr = {"type": "cut", "dur": 0.0}
        start = starts[k]
        end = starts[k + 1] if k + 1 < len(starts) else round(end_local + tail, 3)

        def at_s(a, w, default=None, _k=k, _s=start, _e=end):
            """Inside a shot, start and end are its own window (the last shot keeps the beat's end)."""
            v = a if a is not None else default
            mm = re.match(r"^\s*(start|end)((?:\s*[+-]\s*\d+(?:\.\d+)?)*)\s*$", v) if isinstance(v, str) else None
            if mm and (mm.group(1) == "start" and _k or mm.group(1) == "end" and _k + 1 < len(starts)):
                return round((_s if mm.group(1) == "start" else _e)
                             + sum(float(x.replace(" ", "")) for x in re.findall(r"[+-]\s*\d+(?:\.\d+)?", mm.group(2))), 3)
            return at(a, w, default)
        m = _media(sh, k, where(k), art, media, rep, out)
        try:
            cam = camera(sh.get("cam", "drift"), sh.get("focus"))
        except ValueError as e:
            rep.fail("D5", where(k), str(e))
            cam = None
        s = {"id": sid, "start": start, "end": end, "in": tr, "lead": _lead(tr) if k else lead,  # beat 0's way in runs in the host lead
             "cam": cam, "keys": [], "media": m, "dim": sh.get("dim"), "scrim": sh.get("scrim")}
        if tr["type"] in {"bloom", "iris"}:
            s["origin"] = sh.get("origin") or sh.get("focus") or [0.5, 0.45]
        busy_until = start + (tr["dur"] if tr["type"] in {"zoom", "whip", "push"} else 0)
        prev_to = None
        for j, kk in enumerate(sh.get("keys") or []):
            ka = at_s(kk.get("at"), f"{where(k)}.keys{j}")
            if ka is None:
                continue
            if ka < busy_until:
                rep.warn("D6", f"{where(k)}.keys{j}", f"punch-in at {ka:.2f}s lands inside the '{tr['type']}' in; moved to {busy_until:.2f}s")
                ka = round(busy_until, 3)
            if not start <= ka < end - 0.1:
                rep.fail("D6", f"{where(k)}.keys{j}", f"punch-in at {ka:.2f}s is outside its shot ({start:.2f}–{end:.2f}s)")
                continue
            to = key_to(kk)
            s["keys"].append({"at": ka, "to": to, "dur": float(kk.get("dur", 0.22)), "ease": kk.get("ease", "power3.out")})
            prev_to = to
            out["events"].append(round(b["start"] - lead + ka, 3))
            snd = kk.get("sfx", "thump")
            if snd:
                sounds.append({"t": ka, "kind": snd if isinstance(snd, str) else "thump", "db": float(kk.get("db", -24)),
                               "src": f"{sid}.key{j}"})
        s["s_last"] = prev_to["s"] if prev_to else 1.0
        cap_band = (geo["cap_top"] - 12, geo["cap_top"] + 2 * geo["cap_size"] + 40)
        if raw.get("captions_y") is not None:
            ct = float(raw["captions_y"]) * H
            cap_band = (round(ct) - 12, round(ct + 2 * geo["cap_size"] + 40))
        ctx = {"preset": out.get("preset") or {}, "rigs": out.get("rigs") or {}, "props": out.get("props") or {}, "media": media,
               "words": [(w["s"] - b["start"] + lead, w["e"] - b["start"] + lead) for w in b.get("words") or []]}
        s["layers"] = _layers(sh, sid, start, end, cam, at_s, where(k), art, W, H, rep, sounds, out, b["start"] - lead, cap_band, ctx)
        shots.append(s)
        cn = sh.get("cam", "drift")
        out["shotlog"].append({"at": round(b["start"] - lead + start, 3), "end": round(b["start"] - lead + end, 3),
                               "where": sid, "cam": "custom" if isinstance(cn, dict) else
                               ("none" if cn in (None, False) else str(cn).split(":")[0])})
        out["events"].append(round(b["start"] - lead + start, 3))
        if k:
            out["transitions"].append({"at": round(b["start"] - lead + start, 3), "type": tr["type"], "where": sid})
        if tr["type"] in SFX_FOR_IN and sh.get("sfx", True) is not False:
            kind, db, off = SFX_FOR_IN[tr["type"]]
            sounds.append({"t": start + off - (s["lead"] if tr["type"] in OVERLAP else 0), "kind": kind,
                           "db": db + float(sh.get("sfx_db", 0)), "src": f"{sid}.in:{tr['type']}"})
        for j, e in enumerate(sh.get("sfx") if isinstance(sh.get("sfx"), list) else []):
            ea = at_s(e.get("at", "start"), f"{where(k)}.sfx{j}")
            if ea is not None:
                sounds.append({"t": ea, "kind": e["kind"], "db": float(e.get("db", -22)), "src": f"{sid}.sfx{j}"})
        for j, f in enumerate(sh.get("fx") or []):
            fxs.append((f, f"{where(k)}.fx{j}", start, end))
    # transitions: hide each shot once the next covers the frame, and give it the matching exit
    for k, s in enumerate(shots):
        if k + 1 < len(shots):
            n = shots[k + 1]
            s["hideAt"] = round(n["start"] + (n["in"]["dur"] if n["in"]["type"] in HOLD_PREV else 0), 3)
            o = OUT_FOR.get(n["in"]["type"])
            if o:
                s["out"] = {"type": o[0], "at": round(n["start"] - o[1], 3), "dur": o[1], "s0": s["s_last"],
                            **({"dir": n["in"].get("dir")} if n["in"].get("dir") else {})}
            elif n["in"]["type"] == "push":
                s["out"] = {"type": "push", "at": n["start"], "dur": n["in"]["dur"]}
    if shots and nxt_slot and not nxt_slot.get("error"):
        nt = nxt_slot["first_in"]
        o = OUT_FOR.get(nt["type"])
        last = shots[-1]
        if o:  # the exit plays at the end of this scene, the way in at the start of the next
            last["out"] = {"type": o[0], "at": round(end_local - o[1], 3), "dur": o[1], "s0": last["s_last"],
                           **({"dir": nt.get("dir")} if nt.get("dir") else {})}
        elif nt["type"] == "push":
            last["out"] = {"type": "push", "at": end_local, "dur": nt["dur"]}
    # effects (beat-level and per-shot)
    for j, f in enumerate(raw.get("fx") or []):
        fxs.append((f, f"direction.{bid}.fx{j}", 0.0, dur))
    resolved_fx = []
    for n, (f, w, s0, s1) in enumerate(fxs):
        r = _fx(f, n, bid, b, lead, dur, end_local, geo, w, rep)
        if r is None:
            continue
        resolved_fx.append(r)
        if f.get("type") not in {"dim", "weather"}:
            for o in (item_times(r) if f["type"] in PER_ITEM_SFX else [0.0]):  # each item landing is something new
                out["events"].append(round(b["start"] - lead + r["at"] + o, 3))
        if f.get("type") in SFX_FOR_FX and f.get("sfx", True) is not False:
            kind, db, off = SFX_FOR_FX[f["type"]]
            if isinstance(f.get("sfx"), str):
                kind = f["sfx"]
            offs = item_times(r) if f["type"] in PER_ITEM_SFX else [0.0]
            for k, o in enumerate(offs):
                sounds.append({"t": r["at"] + o + off, "kind": kind, "db": db + float(f.get("sfx_db", 0)),
                               "src": f"{bid}.fx{n}:{f['type']}" + (f"#{k}" if len(offs) > 1 else ""),
                               **({"dur": r["dur"]} if f["type"] in {"counter", "type", "shake"} else {})})
        if f.get("type") == "weather" and f.get("sfx", True) is not False and f.get("kind") in {"rain", "snow"}:
            sounds.append({"t": r["at"], "kind": "rain" if f["kind"] == "rain" else "wind", "db": float(f.get("sfx_db", -30)),
                           "dur": r["dur"], "src": f"{bid}.fx{n}:weather", "bed": True})
    for j, e in enumerate(raw.get("sfx") or []):
        ea = at(e.get("at", "start"), f"direction.{bid}.sfx{j}")
        if ea is not None:
            sounds.append({"t": ea, "kind": e["kind"], "db": float(e.get("db", -22)), "src": f"{bid}.sfx{j}",
                           **({"dur": float(e["dur"])} if e.get("dur") else {})})
    origin = b["start"] - lead
    for snd in sounds:
        out["sfx"].append({**snd, "t": round(origin + snd["t"], 3)})
    tweens = []
    for t_i, tw in enumerate(raw.get("tweens") or []):  # raw GSAP over anything in the scene: the last word on motion
        tw_w = f"direction.{bid}.tweens{t_i}"
        if not isinstance(tw.get("sel"), str) or not isinstance(tw.get("to"), dict):
            rep.fail("D12", tw_w, "a beat tween is {at, sel: CSS selector (scoped to the scene; #id as-is), from?, to: {GSAP vars}, "
                                  "dur, ease}")
            continue
        ta = at(tw.get("at", "start"), tw_w)
        if ta is None:
            continue
        d, to = float(tw.get("dur", 0.5)), dict(tw["to"])
        if to.get("repeat") == -1:  # the frame must stay a function of time
            to["repeat"] = max(0, math.ceil((dur - ta) / max(0.05, d)) - 1)
        tweens.append({"at": round(ta, 3), "dur": d, "ease": tw.get("ease", "power2.inOut"), "sel": tw["sel"],
                       "from": tw.get("from"), "to": to})
        if tw.get("event", True):
            out["events"].append(round(b["start"] - lead + ta, 3))
    slot["spec"] = {"id": bid, "W": W, "H": H, "dur": dur, "tweens": tweens, "shots": [{k: v for k, v in s.items() if k not in {"media", "s_last", "scrim"}}
                                                                    for s in shots], "fx": resolved_fx}
    slot["shots_resolved"] = shots
    slot["dur"] = dur
    slot["moods"] = raw.get("mood")


def _layer_cam(cam: dict | None, z: float) -> dict | None:
    """The shot's camera as seen by a layer at depth z: a nearer layer scales and travels further (parallax)."""
    if not cam:
        return None
    f = lambda c: {"s": round(1 + (c["s"] - 1) * (1 + z), 4), "x": round(c["x"] * (1 + z), 4), "y": round(c["y"] * (1 + z), 4)}
    return {"from": f(cam["from"]), "to": f(cam["to"]), "ease": cam.get("ease", "none")}


def _layers(sh, sid, start, end, cam, at, w, art, W, H, rep, sounds, out, origin, cap_band, ctx=None) -> list[dict]:
    """A shot's `layers`: cut-out art (or a code-drawn rig) over its plate, each placed by its foot point, at a
    depth, with an entrance, moves, an idle loop and hits on words. Times are the scene's own seconds; moves are
    px offsets. `anchor: fill` is a full-frame plane (multiplane depth), overscanned so its parallax shows no edge."""
    ctx = ctx or {}
    pre_art = (ctx.get("preset") or {}).get("art") or {}
    res = []
    for j, L in enumerate(sh.get("layers") or []):
        lw, lid = f"{w}.layers{j}", f"{sid}-L{j}"
        if L.get("prop"):  # a set piece drawn once in the props sheet; the layer's own keys win
            props = ctx.get("props") or {}
            if L["prop"] not in props:
                rep.fail("D12", lw, f"prop {L['prop']!r}: not in props ({', '.join(sorted(props)) or 'none'}; "
                                    f"07-video/props.json or the direction's props)")
                continue
            L = {**props[L["prop"]], **{k: v for k, v in L.items() if k != "prop"}}
        anchor = L.get("anchor", "bottom")
        if anchor not in {"bottom", "center", "fill"}:
            rep.fail("D12", lw, f"anchor {anchor!r} not in bottom, center, fill")
            continue
        z = float(L.get("z", LAYER_Z))
        if z < -1:
            rep.warn("D12", lw, f"z {z:g} < -1: -1 already stands still while the plate moves; lower runs against it")
        blend = L.get("blend")
        if blend is not None and blend not in BLENDS:
            rep.fail("D12", lw, f"blend {blend!r} is not a CSS mix-blend-mode ({', '.join(sorted(BLENDS))})")
            continue
        srcs, swaps, shown_h, rg, vec = [], [], None, None, None
        if L.get("rig"):
            rg = _rig(L, lw, start, end, at, rep, out, origin, ctx)
            if rg is None:
                continue
        elif L.get("svg"):  # a prop drawn in code: raw SVG markup in the rig's ink, line and boil
            try:
                vb = [float(v) for v in str(L.get("viewBox", "0 0 100 100")).replace(",", " ").split()]
                assert len(vb) == 4 and vb[2] > 0 and vb[3] > 0
            except (ValueError, AssertionError):
                rep.fail("D12", lw, f"viewBox {L.get('viewBox')!r}: four numbers, min-x min-y width height")
                continue
            pre_rig = (ctx.get("preset") or {}).get("rig") or {}
            svg, missing = str(L["svg"]), []
            for ref in dict.fromkeys(re.findall(r'"media:([^"]+)"', svg)):  # a campaign file placed inside the drawing
                if ref in (ctx.get("media") or {}):
                    svg = svg.replace(f'"media:{ref}"', f'"{ctx["media"][ref]}"')
                else:
                    missing.append(ref)
            if missing:
                rep.fail("D12", lw, f"media:{missing[0]} is not a file in the campaign")
                continue
            vec = {"svg": svg, "vb": vb, "ink": L.get("ink", pre_rig.get("ink", "var(--mv-ink)")),
                   "fill": L.get("fill", "none"), "stroke": float(L.get("stroke", pre_rig.get("width", 0.022) * 0.4)),
                   "boil": float(L.get("boil", pre_rig.get("boil", 0))), "boil_fps": float(L.get("boil_fps", 8))}
        else:
            names = [L.get("art")] + [s.get("art") for s in (L.get("swaps") or [])]
            got_all, ok = [], True
            for n_i, a in enumerate(names):
                got = art.get(a) if a else None
                aw = lw if n_i == 0 else f"{lw}.swaps{n_i - 1}"
                if not got:
                    rep.fail("D12", aw, f"layer art {a!r}: nothing drawn or approved (a cut-out art job, use: sticker; "
                                        f"or rig: stick for a figure drawn in code)")
                    ok = False
                    continue
                if not got["approved"]:
                    rep.warn("D7", aw, f"art {a!r} is not approved: the layer shows its newest draft (a rough cut only; export refuses it)")
                    out["draft_art"].append(a)
                elif got.get("cutout") is False and not blend:
                    if pre_art.get("sticker") == "multiply":
                        blend = "multiply"  # the preset draws props as line art on white: the white drops into the paper
                    else:
                        rep.warn("D12", aw, f"art {a!r} is opaque and hides the plate behind it: fine for a card or a "
                                            f"full-frame picture; otherwise set transparent: true on its job, or blend: multiply")
                got_all.append(got)
            if not ok:
                continue
            srcs = list(dict.fromkeys(g["path"] for g in got_all))
            shown_h = got_all[0].get("shown_h")
            for s_i, s in enumerate(L.get("swaps") or []):
                sa = at(s.get("at"), f"{lw}.swaps{s_i}")
                if sa is None:
                    continue
                if not start <= sa < end:
                    rep.fail("D12", f"{lw}.swaps{s_i}", f"swap at {sa:.2f}s is outside its shot ({start:.2f}–{end:.2f}s)")
                    continue
                swaps.append({"at": sa, "k": srcs.index(art[s["art"]]["path"])})
                out["events"].append(round(origin + sa, 3))
        lcam = _layer_cam(cam, z)
        if anchor == "fill":
            need = 0.0
            for e in ((lcam["from"], lcam["to"]) if lcam else ()):
                need = max(need, (0.5 + abs(e["x"])) / e["s"] - 0.5, (0.5 + abs(e["y"])) / e["s"] - 0.5)
            ov = round(max(0.0, need) + 0.03, 3)  # + room for a hit's recoil
            x, y, h = 0.5, 0.5, round(1 + 2 * ov, 3)
        else:
            ov = 0.0
            x, y, h = float(L.get("x", 0.5)), float(L.get("y", 0.92)), float(L.get("h", 0.5))
        if shown_h and h * H > shown_h * 1.1:
            rep.warn("D13", lw, f"shown {round(h * H)} px tall, approved for {shown_h} px: set its job's display to the "
                                f"size it is shown at and approve again, or the cut-out goes soft")
        if anchor != "fill" and vec is None:  # a drawn prop has no face to cover
            top = (y - h if anchor == "bottom" else y - h / 2) * H
            cap0, cap1 = cap_band
            if h >= 0.25 and top < cap1 and top + 0.25 * h * H > cap0:
                rep.warn("D14", lw, f"its top quarter ({top:.0f}–{top + 0.25 * h * H:.0f}px, a face?) sits in the caption band "
                                    f"({cap0}–{cap1}px): move the layer, or set the beat's captions_y")
        r = {"id": lid, "src": srcs[0] if srcs else None, "srcs": srcs, "swaps": swaps, "x": x, "y": y, "h": h, "z": z,
             "ov": ov, "anchor": anchor, "blend": blend, "flip": bool(L.get("flip")), "shadow": bool(L.get("shadow")),
             "cam": lcam, "enter": None, "moves": [], "idle": None, "hits": [], "rig": rg, "vec": vec}
        busy = start
        if L.get("enter"):
            e = L["enter"] if isinstance(L["enter"], dict) else {"type": L["enter"]}
            if e.get("type") not in LAYER_IN:
                rep.fail("D12", f"{lw}.enter", f"enter {e.get('type')!r} not in {sorted(LAYER_IN)} (or write it as tweens)")
            else:
                ea = at(e.get("at"), f"{lw}.enter", "start")
                if ea is not None:
                    ea = max(ea, start)
                    d = float(e.get("dur", LAYER_IN[e["type"]]))
                    r["enter"] = {"type": e["type"], "at": ea, "dur": d}
                    busy = ea + d
                    out["events"].append(round(origin + ea, 3))
        cur = {"x": 0.0, "y": 0.0, "s": 1.0, "rot": 0.0}
        for m_i, m in enumerate(L.get("moves") or []):
            mw = f"{lw}.moves{m_i}"
            ma = at(m.get("at"), mw)
            if ma is None:
                continue
            if ma < busy:
                rep.warn("D12", mw, f"move at {ma:.2f}s starts before the last one ends; moved to {busy:.2f}s")
                ma = round(busy, 3)
            d = float(m.get("dur", 0.6))
            if not start <= ma < end - 0.1:
                rep.fail("D12", mw, f"move at {ma:.2f}s is outside its shot ({start:.2f}–{end:.2f}s)")
                continue
            to = dict(cur)
            if "x" in m:
                to["x"] = round((float(m["x"]) - x) * W, 1)
            if "y" in m:
                to["y"] = round((float(m["y"]) - y) * H, 1)
            if "s" in m:
                to["s"] = float(m["s"])
            if "rot" in m:
                to["rot"] = float(m["rot"])
            mv = {"at": ma, "dur": d, "ease": m.get("ease", "power2.inOut"), "from": cur, "to": to}
            far = abs(to["x"] - cur["x"]) + abs(to["y"] - cur["y"]) >= 0.03 * W or abs(to["s"] - cur["s"]) >= 0.06 \
                or abs(to["rot"] - cur["rot"]) >= 6
            if m.get("antic", True) is not False and "ease" not in m and d >= 0.35 and far:
                mv["antic"] = float(m["antic"]) if isinstance(m.get("antic"), (int, float)) and m["antic"] is not True else ANTIC
            r["moves"].append(mv)
            cur, busy = to, ma + d
            out["events"].append(round(origin + ma, 3))
        if L.get("idle"):
            i = L["idle"] if isinstance(L["idle"], dict) else {"type": L["idle"]}
            if i.get("type") not in IDLE:
                rep.fail("D12", f"{lw}.idle", f"idle {i.get('type')!r} not in {sorted(IDLE)} (or a tween with repeat: -1)")
            else:
                r["idle"] = {"type": i["type"], "amt": float(i.get("amt", 1.0)), "period": float(i.get("period", IDLE[i["type"]]))}
        for h_i, hit in enumerate(L.get("hits") or []):
            hw = f"{lw}.hits{h_i}"
            ha = at(hit.get("at"), hw)
            if ha is None:
                continue
            if not start <= ha < end - 0.1:
                rep.fail("D12", hw, f"hit at {ha:.2f}s is outside its shot ({start:.2f}–{end:.2f}s)")
                continue
            frm = hit.get("from", "left")
            if frm not in HIT_FROM:
                rep.fail("D12", hw, f"hit from {frm!r} not in {sorted(HIT_FROM)}")
                continue
            sx, sy, lean = HIT_FROM[frm]
            amt = float(hit.get("amt", 1.0))
            r["hits"].append({"at": ha, "dx": round(sx * 0.018 * W * amt, 1), "dy": round(sy * 0.012 * H * amt, 1),
                              "rot": round(lean * amt, 2)})
            out["events"].append(round(origin + ha, 3))
            if hit.get("sfx"):
                sounds.append({"t": ha, "kind": hit["sfx"], "db": float(hit.get("db", -22)), "src": f"{lid}.hit{h_i}"})
        r["tweens"] = []
        for t_i, tw in enumerate(L.get("tweens") or []):  # raw GSAP: anything the shortcuts above do not cover
            tw_w = f"{lw}.tweens{t_i}"
            sel = tw.get("sel")
            if (sel is None and tw.get("part", "move") not in LAYER_PARTS) or (sel is not None and not isinstance(sel, str)) \
                    or not isinstance(tw.get("to"), dict):
                rep.fail("D12", tw_w, f"a tween is {{at, part: {'|'.join(LAYER_PARTS)} (or sel: any CSS selector inside the "
                                      f"layer), from?, to: {{GSAP vars}}, dur, ease}}")
                continue
            ta = at(tw.get("at"), tw_w)
            if ta is None:
                continue
            if not start <= ta < end:
                rep.fail("D12", tw_w, f"tween at {ta:.2f}s is outside its shot ({start:.2f}–{end:.2f}s)")
                continue
            d = float(tw.get("dur", 0.5))
            to = dict(tw["to"])
            if to.get("repeat") == -1:  # the frame must stay a function of time: loop over the rest of the shot
                to["repeat"] = max(0, math.ceil((end - ta) / max(0.05, d)) - 1)
            r["tweens"].append({"at": ta, "dur": d, "ease": tw.get("ease", "power2.inOut"), "part": tw.get("part", "move"),
                                "sel": sel, "from": tw.get("from"), "to": to})
            if tw.get("event", True):
                out["events"].append(round(origin + ta, 3))
        res.append(r)
    return res


def _rig(L, lw, start, end, at, rep, out, origin, ctx) -> dict | None:
    """A code-drawn figure: its pose timeline (named poses, joint dicts, walk cycles), faces, a mouth that talks
    on the beat's words, emotes, and the line boil. Every joint stays reachable by a raw tween on part `rig`."""
    rigs = ctx.get("rigs") or {}
    R = L["rig"]
    if isinstance(R, str):
        R = dict(rigs[R]) if R in rigs else {"type": R}
    elif R.get("use"):
        if R["use"] not in rigs:
            rep.fail("D12", f"{lw}.rig", f"rig use {R['use']!r}: not in rigs ({', '.join(sorted(rigs)) or 'none'}; "
                                         f"07-video/rigs.json or the direction's rigs)")
            return None
        base = rigs[R["use"]]
        R = {**base, **{k: v for k, v in R.items() if k != "use"}}
        for k in ("faces", "poses"):  # an override adds to the sheet's faces and poses ...
            if isinstance(base.get(k), dict) and isinstance(R.get(k), dict) and R[k] is not base[k]:
                R[k] = {**base[k], **R[k]}
        if base.get("parts") and R.get("parts") is not base.get("parts"):  # ... and its parts (a prop in hand)
            R["parts"] = list(base["parts"]) + list(R.get("parts") or [])
    if R.get("type", "stick") != "stick":
        rep.fail("D12", f"{lw}.rig", f"rig {R.get('type')!r}: not a character in rigs, and the runtime draws 'stick' "
                                     f"(anything else: a cut-out or a custom scene)")
        return None
    pre = (ctx.get("preset") or {}).get("rig") or {}
    R = {**{k: v for k, v in pre.items() if k == "body"}, **R}
    if R.get("body", "stick") not in rig.BODIES:
        rep.fail("D12", f"{lw}.rig.body", f"body {R['body']!r} not in {sorted(rig.BODIES)} (or set dims yourself)")
        return None
    dm = rig.body(R)
    if R.get("hair"):  # a hair shortcut goes under the rig's own parts
        try:
            R = {**R, "parts": rig.hair_parts(R["hair"]) + list(R.get("parts") or [])}
        except rig.RigError as e:
            rep.fail("D12", f"{lw}.rig.hair", str(e))
            return None
    bad = [pt.get("on") for pt in R.get("parts") or [] if pt.get("on") not in rig.PART_BONES or not pt.get("svg")
           or pt.get("view", "front") not in {"front", "side"}]
    if bad:
        rep.fail("D12", f"{lw}.rig.parts", f"a part is {{on: {'|'.join(rig.PART_BONES)}, svg, back?, view?: front|side}}; got on={bad}")
        return None
    extra = R.get("poses") or {}
    custom_faces = R.get("faces") or {}
    face_names = set(rig.face_lib(dm)) | set(custom_faces)
    try:
        p0 = rig.pose(L.get("pose", "stand"), None, extra, dm)
    except rig.RigError as e:
        rep.fail("D12", f"{lw}.pose", str(e))
        return None
    face0 = L.get("face", "calm")
    if face0 not in face_names:
        rep.fail("D12", f"{lw}.face", f"face {face0!r} not in {sorted(face_names)} (or add it under rig.faces as SVG)")
        return None
    steps = []  # (at, kind, entry, where)
    for i, p in enumerate(L.get("poses") or []):
        steps.append((at(p.get("at"), f"{lw}.poses{i}"), "pose", p, f"{lw}.poses{i}"))
    for i, wk in enumerate(L.get("walk") or []):
        steps.append((at(wk.get("at"), f"{lw}.walk{i}"), "walk", wk, f"{lw}.walk{i}"))
    tweens, facesets, used = [], [], {face0}
    cur, busy = p0, start
    for ta, kind, e, ew in sorted((s for s in steps if s[0] is not None), key=lambda s: s[0]):
        if not start <= ta < end - 0.05:
            rep.fail("D12", ew, f"at {ta:.2f}s is outside its shot ({start:.2f}–{end:.2f}s)")
            continue
        if ta < busy and kind == "walk" or (kind == "pose" and ta < busy and (e.get("pose") or e.get("set"))):
            rep.warn("D12", ew, f"starts at {ta:.2f}s before the last pose ends; moved to {busy:.2f}s")
            ta = round(busy, 3)
        if kind == "walk":
            until = at(e.get("until"), ew, "end")
            if until is None or until <= ta:
                rep.fail("D12", ew, "a walk needs until: after its at")
                continue
            seq = rig.walk(ta, min(until, end), float(e.get("speed", 1.8)), float(e.get("amt", 1.0)), cur, dm)
            if seq:
                tweens += seq
                cur, busy = seq[-1]["to"], seq[-1]["at"] + seq[-1]["dur"]
                out["events"].append(round(origin + ta, 3))
            continue
        if e.get("face"):
            if e["face"] not in face_names:
                rep.fail("D12", ew, f"face {e['face']!r} not in {sorted(face_names)}")
                continue
            facesets.append({"at": ta, "face": e["face"]})
            used.add(e["face"])
            out["events"].append(round(origin + ta, 3))
        if e.get("pose") or e.get("set"):
            try:
                to = rig.pose(e.get("pose"), cur, extra, dm) if e.get("pose") else cur
                if e.get("set"):
                    to = rig.pose(e["set"], to, extra, dm)
            except rig.RigError as err:
                rep.fail("D12", ew, str(err))
                continue
            d = float(e.get("dur", 0.3))
            tw = {"at": ta, "dur": d, "ease": e.get("ease", "power2.inOut"), "from": cur, "to": to}
            swing = max([abs(float(to[k]) - float(cur[k])) for k in to if k in cur and isinstance(to[k], (int, float))
                         and isinstance(cur[k], (int, float)) and k != "drop"] or [0])
            if e.get("antic", True) is not False and "ease" not in e and d >= 0.25 and swing >= 15:
                tw["antic"] = ANTIC
            tweens.append(tw)
            cur, busy = to, ta + d
            out["events"].append(round(origin + ta, 3))
    flaps = []
    talk = L.get("talk")
    if talk:
        spans = [(start, end)] if talk is True else [
            (at(s.get("at"), f"{lw}.talk{i}", "start"), at(s.get("until"), f"{lw}.talk{i}", "end"))
            for i, s in enumerate(talk if isinstance(talk, list) else [talk])]
        for a, b in spans:
            if a is not None and b is not None:
                flaps += rig.flaps(ctx.get("words") or [], max(a, start), min(b, end))
    emotes = []
    for i, em in enumerate(L.get("emotes") or []):
        ew = f"{lw}.emotes{i}"
        ea = at(em.get("at"), ew)
        if ea is None:
            continue
        if em.get("svg"):
            mark = {"svg": str(em["svg"])}
        elif em.get("text"):
            mark = {"anchor": [13, -24], "svg": (
                f'<text x="0" y="3" font-size="{float(em.get("size", 10)):g}" text-anchor="middle" stroke="none" '
                f'class="rig-dot" style="font-family:var(--mv-display-font);font-weight:900">{html.escape(str(em["text"]))}</text>')}
        elif em.get("type") in rig.EMOTES:
            ax, ay, svg_ = rig.EMOTES[em["type"]]
            mark = {"anchor": [ax, ay], "svg": svg_}
        else:
            rep.fail("D12", ew, f"emote {em.get('type')!r} not in {sorted(rig.EMOTES)} (or give text: or svg:)")
            continue
        if "anchor" in mark:  # nudged in the stick head's units, clear of a tall hairdo
            mark["anchor"] = [mark["anchor"][0] + float(em.get("dx", 0)), mark["anchor"][1] + float(em.get("dy", 0))]
        emotes.append({"at": ea, "dur": float(em.get("dur", 1.2)), **mark})
        out["events"].append(round(origin + ea, 3))
    blinks = []
    if dm.get("blink"):  # eyes in .rig-ey blink every few seconds, on a seed of the layer's id (the same every render)
        rnd = random.Random(zlib.crc32(lw.encode()))
        t = start + rnd.uniform(0.5, 1.6)
        while t < end - 0.3:
            if all(abs(t - f["at"]) > 0.3 for f in facesets):
                blinks.append(round(t, 3))
            t += rnd.uniform(2.3, 4.2)
    return {"pose0": p0, "face0": face0, "faces_used": sorted(used), "faces": custom_faces, "tweens": tweens,
            "facesets": facesets, "flaps": flaps, "emotes": emotes, "blinks": blinks,
            "boil": float(R.get("boil", pre.get("boil", 0))), "boil_fps": float(R.get("boil_fps", 8)),
            "ink": R.get("ink", pre.get("ink", "var(--mv-ink)")),
            "width": float(R.get("width", pre.get("width", dm["width"]) if R.get("body", "stick") == pre.get("body", "stick")
                                 else dm["width"])),
            "view": R.get("view", "front"), "dims": dm, "parts": R.get("parts") or [],
            **{k: R[k] for k in ("skin", "shirt", "pants", "sleeves", "hands", "shoes", "arms", "blush") if R.get(k)},
            **({"skin": R["fill"]} if R.get("fill") and not R.get("skin") else {})}


def _media(sh, k, w, art, media, rep, out) -> dict:
    if sh.get("art"):
        a = sh["art"]
        got = art.get(a)
        if not got:
            rep.fail("D7", w, f"art {a!r}: nothing drawn or approved (mvf art draw/approve {a}); the shot shows the ground")
            return {"kind": "color", "color": "ground"}
        if not got["approved"]:
            rep.warn("D7", w, f"art {a!r} is not approved: the scene shows its newest draft (a rough cut only; export refuses it)")
            out["draft_art"].append(a)
        return {"kind": "art", "id": a, "path": got["path"], "card": bool(sh.get("card"))}
    for kind in ("video", "image"):
        if sh.get(kind):
            src = media.get(sh[kind])
            if not src:
                rep.fail("D7", w, f"{kind} {sh[kind]!r} not found (a path inside the campaign)")
                return {"kind": "color", "color": "ground"}
            return {"kind": kind, "path": src, "card": bool(sh.get("card")), "trim": float(sh.get("trim", 0)), "rate": sh.get("rate")}
    if sh.get("html"):  # raw: any HTML or inline SVG drawn as the plate (the camera still moves it)
        return {"kind": "raw", "html": str(sh["html"])}
    if sh.get("color"):
        return {"kind": "color", "color": sh["color"]}
    rep.fail("D7", w, "a shot shows art:, video:, image:, color: or html: (raw markup)")
    return {"kind": "color", "color": "ground"}


def _fx(f, n, bid, b, lead, dur, end_local, geo, w, rep) -> dict | None:
    f = {**f, **{k: f[k][geo["canvas"]] for k in POS_KEYS if isinstance(f.get(k), dict) and geo["canvas"] in f[k]}}
    for k in POS_KEYS:
        if isinstance(f.get(k), dict):
            rep.fail("D8", w, f"{k} gives {sorted(f[k])} but not this canvas {geo['canvas']}")
            return None
    ty = f.get("type")
    if ty not in FX_DUR:
        rep.fail("D8", w, f"fx type {ty!r} not in {sorted(FX_DUR)}")
        return None
    try:
        t = resolve(f.get("at", "start"), b, lead)
        if f.get("until") is not None:
            d = resolve(f["until"], b, lead) - t
        elif f.get("dur") is not None:
            d = float(f["dur"])
        else:
            d = FX_DUR[ty] if FX_DUR[ty] is not None else (end_local + 0.0 - t)
    except AnchorError as e:
        rep.fail("D3", w, str(e))
        return None
    if ty in {"weather", "endtag"} and f.get("until") is None and f.get("dur") is None:
        d = dur - t
    d = round(max(0.1, min(d, dur - t)), 3)
    r = {"type": ty, "at": round(t, 3), "dur": d, "el": f"{bid}-fx{n}"}
    if f.get("shown") and ty in TEXT_FX:
        r["shown"] = True  # already on screen at its `at` (frame 0 is the cover): no entrance, only the exit
    for k in ("from", "to", "suffix", "locale", "run", "peak", "amp", "amt", "seed", "kind", "rot", "stagger", "step", "hold",
              "in", "out", "opacity", "heads", "focus", "spot", "max"):
        if k in f:
            r[k] = f[k]
    if ty in {"steps", "compare", "bars"}:
        if not _items(ty, f, r, w, rep):
            return None
        if f.get("ats"):  # each item lands on its own word: the list keeps pace with the voice
            try:
                ts = [round(resolve(a, b, lead) - t, 3) for a in f["ats"]]
            except AnchorError as e:
                rep.fail("D3", w, str(e))
                return None
            if any(x < 0 or x > d for x in ts):
                rep.fail("D3", w, f"{ty} ats {ts} fall outside the effect (0–{d}s after its start)")
                return None
            r["times"] = ts
    if ty in {"dots", "ladder"} and not _device(ty, f, r, w, rep):
        return None
    if ty in {"callout", "tap"}:
        if not _region(ty, f, r, geo, w, rep):
            return None
    if ty == "html":
        if not isinstance(f.get("html"), str):
            rep.fail("D8", w, "an html fx carries html: \"<markup>\" (any HTML or inline SVG; style: positions its box)")
            return None
        r.update(html=f["html"], style=str(f.get("style") or "inset:0"))
    if ty == "pointer" and not _pointer(f, r, geo, w, rep):
        return None
    if ty == "dim":
        r["el"] = f"{bid}-s{int(f.get('shot', 0))}-d"
    if ty == "weather":
        if f.get("kind") not in WEATHER:
            rep.fail("D8", w, f"weather kind {f.get('kind')!r} not in {sorted(WEATHER)}")
            return None
        r.update(W=geo["W"], H=geo["H"], n=int(f.get("count", WEATHER[f["kind"]])))
    if ty == "speedlines":
        if f.get("kind", "radial") not in {"radial", "linear"}:
            rep.fail("D8", w, f"speedlines kind {f.get('kind')!r} not in radial, linear")
            return None
        r.update(W=geo["W"], H=geo["H"], fps=float(f.get("fps", 12)), polys=_speedlines(f, geo["W"], geo["H"]),
                 color=_color(f.get("color", "on_plate")))
    if ty in TEXT_FX:
        r["text"] = f.get("text", "")
        r["sub"] = f.get("sub")
        _place_text(r, f, geo, w, rep)
    return r


def _color(c: str) -> str:
    """A look role (ground, ink, accent, hot, on_plate) as its scene variable, or the colour as written."""
    c = str(c)
    return f"var(--mv-{c.replace('_', '-')})" if re.match(r"^[a-z_]+$", c) and c not in {"white", "black"} else c


def _speedlines(f: dict, W: int, H: int) -> list[str]:
    """Seeded line polygons, in frames to flicker through (`variants`): radial wedges that stop short of a focus
    point, or tapered streaks along `angle`. The same seed draws the same lines on every render."""
    import random
    kind, n = f.get("kind", "radial"), int(f.get("count", 56 if f.get("kind", "radial") == "radial" else 30))
    wd, var = float(f.get("width", 1.0)), max(1, int(f.get("variants", 3)))
    out = []
    for v in range(var):
        rnd = random.Random(int(f.get("seed", 7)) * 101 + v)
        pts = []
        if kind == "radial":
            cx, cy = float(f.get("x", 0.5)) * W, float(f.get("y", 0.45)) * H
            R = math.hypot(W, H)
            ri0 = float(f.get("inner", 0.32)) * math.hypot(W, H) / 2
            for i in range(n):
                th = (i + rnd.random()) / n * 2 * math.pi
                ri = ri0 * (0.85 + rnd.random() * 0.6)
                b = W * 0.006 * wd * (0.4 + rnd.random())
                ux, uy = math.cos(th), math.sin(th)
                tip = (cx + ri * ux, cy + ri * uy)
                p1 = (cx + R * ux - b * uy * 4, cy + R * uy + b * ux * 4)
                p2 = (cx + R * ux + b * uy * 4, cy + R * uy - b * ux * 4)
                pts.append((tip, p1, p2))
        else:
            a = math.radians(float(f.get("angle", 0)))
            ux, uy, D = math.cos(a), math.sin(a), math.hypot(W, H) / 2
            for i in range(n):
                off = ((i + rnd.random()) / n * 2 - 1) * D
                mid = (rnd.random() * 2 - 1) * D * 0.7
                ln = W * (0.25 + rnd.random() * 0.6) / 2
                t = H * 0.0035 * wd * (0.4 + rnd.random())
                cx, cy = W / 2 - uy * off + ux * mid, H / 2 + ux * off + uy * mid
                pts.append(((cx - ux * ln, cy - uy * ln), (cx - uy * t, cy + ux * t), (cx + ux * ln, cy + uy * ln),
                            (cx + uy * t, cy - ux * t)))
        out.append("".join('<polygon points="' + " ".join(f"{x:.0f},{y:.0f}" for x, y in poly) + '"/>' for poly in pts))
    return out


def item_times(r: dict) -> list[float]:
    """When each item of a steps/compare block lands, in seconds after the block starts: its `ats`, else spread
    over the first 60% of the block (the same default mvkit.js plays)."""
    if r.get("times"):
        return list(r["times"])
    if r["type"] == "compare":
        return [0.0, round(min(1.2, r["dur"] * 0.4), 3)]
    n = len(r.get("items") or [])
    gap = min(0.9, r["dur"] * 0.6 / max(1, n))
    return [round(i * gap, 3) for i in range(n)]


def _items(ty, f, r, w, rep) -> bool:
    """steps: items (or text lines); compare: left + right lists, heads; bars: [{label, value}]."""
    if ty == "steps":
        items = f.get("items") or [x for x in str(f.get("text", "")).split("\n") if x.strip()]
        r["items"] = [str(x) for x in items]
        n = len(items)
    elif ty == "compare":
        r["left"], r["right"] = [str(x) for x in f.get("left") or []], [str(x) for x in f.get("right") or []]
        n = len(r["left"]) + len(r["right"])
        if not r["left"] or not r["right"]:
            rep.fail("D8", w, "compare needs both left and right lists (and heads [left, right])")
            return False
    else:
        try:
            r["items"] = [{"label": str(x["label"]), "value": float(x["value"])} for x in f.get("items") or []]
        except (KeyError, TypeError, ValueError):
            rep.fail("D8", w, "bars items are [{\"label\": …, \"value\": number}]")
            return False
        n = len(r["items"])
    if not n:
        rep.fail("D8", w, f"{ty} has no items")
        return False
    if n > {"steps": 6, "compare": 10, "bars": 7}[ty]:
        rep.warn("D8", w, f"{ty} carries {n} items: more than a viewer reads in one pass; split it across beats")
    return True


def _device(ty, f, r, w, rep) -> bool:
    """dots: total (≤ 400) dots in a grid, the first `lit` of them lit; ladder: left and right rungs listed bottom
    to top, each column spread over the same height so unequal counts do not line up, `mark` [side, rung] lit."""
    if ty == "dots":
        try:
            total, lit = int(f.get("total", 0)), int(f.get("lit", 0))
        except (TypeError, ValueError):
            rep.fail("D8", w, "dots: total and lit are whole numbers")
            return False
        if not 1 <= total <= 400 or not 0 <= lit <= total:
            rep.fail("D8", w, f"dots: total {total} must be 1–400 and lit {lit} 0–total (show a big number as a "
                     "share: 1 dot = 100 people, said in sub)")
            return False
        cols = int(f.get("cols") or min(20, max(1, math.ceil(math.sqrt(total * 1.6)))))
        r.update(total=total, lit=lit, cols=cols, rows=math.ceil(total / cols))
        return True
    left, right = [str(x) for x in f.get("left") or []], [str(x) for x in f.get("right") or []]
    if not left or not right:
        rep.fail("D8", w, "ladder needs left and right rungs, bottom to top (and heads [left, right])")
        return False
    if max(len(left), len(right)) > 16:
        rep.warn("D8", w, "a ladder over 16 rungs reads as texture: group rungs or show part of it")
    mark = f.get("mark")
    if mark is not None:
        ok = isinstance(mark, list) and len(mark) == 2 and mark[0] in {"l", "r", "left", "right"} and isinstance(mark[1], int)
        if not ok or not 0 <= mark[1] < len(left if str(mark[0])[0] == "l" else right):
            rep.fail("D8", w, "ladder mark is [\"l\"|\"r\", rung index from the bottom, 0-based]")
            return False
        r["mark"] = [str(mark[0])[0], mark[1]]
    r.update(left=left, right=right, heads=f.get("heads") or ["", ""], offset=float(f.get("offset", 0)))
    return True


def _region(ty, f, r, geo, w, rep) -> bool:
    """callout: rect [x, y, w, h] in frame shares (or px) around what the viewer must look at; tap: point x, y."""
    W, H = geo["W"], geo["H"]

    def px(v, full):
        return float(v) * full if float(v) <= 1 else float(v)
    try:
        if ty == "callout":
            x, y, ww, hh = (px(v, full) for v, full in zip(f["rect"], (W, H, W, H)))
        else:
            x, y, ww, hh = px(f["x"], W), px(f["y"], H), 0.0, 0.0
    except (KeyError, TypeError, ValueError):
        rep.fail("D8", w, "callout needs rect [x, y, w, h]; tap needs x and y (shares of the frame, or px)")
        return False
    if x < 0 or y < 0 or x + ww > W or y + hh > H:
        rep.warn("D8", w, f"{ty} at {x:.0f},{y:.0f} {ww:.0f}x{hh:.0f}px leaves the {W}x{H} frame: clamped to it")
        x, y = min(max(x, 0.0), W - ww if ww < W else 0.0), min(max(y, 0.0), H - hh if hh < H else 0.0)
        ww, hh = min(ww, W - x), min(hh, H - y)
    r["rect"] = [round(x), round(y), round(ww), round(hh)]
    if ty == "callout" and f.get("text"):
        size = float(f.get("size") or 46)
        below = y + hh + 18 + size * 1.5 < geo["cap_top"] - 12
        top = y + hh + 18 if below else y - 18 - size * 1.4
        r["text"] = str(f["text"])
        r["label"] = {"top": round(top), "size": round(size), "left": round(max(geo["safe"]["left"], min(x, W - 600)))}
        cap0, cap1 = geo["cap_top"] - 12, geo["cap_top"] + 2 * geo["cap_size"] + 40
        if top < cap1 and top + size * 1.4 > cap0 and not f.get("allow_caption_zone"):
            rep.fail("D9", w, f"callout label at y {top:.0f}px sits in the caption band: move the rect or drop the label")
    cap0, cap1 = geo["cap_top"] - 12, geo["cap_top"] + 2 * geo["cap_size"] + 40
    y0, y1 = (y, y + hh) if ty == "callout" else (y - 75, y + 75)  # a tap's ring is 150 px across
    if y0 < cap1 and y1 > cap0:
        rep.warn("D9", w, f"{ty} y {y0:.0f}–{y1:.0f}px meets the caption band ({cap0}–{cap1}px): the captions will "
                 "cover what it points at; move the picture (focus/keys) or set the beat's captions_y")
    if y + hh > geo["safe"]["bottom"] or x + ww > W - geo["safe"]["right"]:
        rep.warn("D9", w, f"{ty} reaches under the platform UI (right rail / bottom text): the viewer may not see it")
    return True


def _pointer(f, r, geo, w, rep) -> bool:
    """pointer: text lettered at `label` [x, y] with a leader line drawn on to `point` [x, y] (shares or px), a dot at its end."""
    W, H = geo["W"], geo["H"]
    try:
        px = lambda v, full: float(v) * full if float(v) <= 1 else float(v)
        x1, y1 = px(f["point"][0], W), px(f["point"][1], H)
        lab = f.get("label") or [x1 / W - 0.18 if x1 > W / 2 else x1 / W + 0.18, y1 / H - 0.12]
        x0, y0 = px(lab[0], W), px(lab[1], H)
    except (KeyError, TypeError, ValueError, IndexError):
        rep.fail("D8", w, "pointer needs point [x, y] (what it names) and text; label [x, y] places the lettering")
        return False
    size = float(f.get("size") or 54)
    dx, dy = x1 - x0, y1 - y0
    bend = float(f.get("bend", 0.18))  # a hand-drawn line bows a little
    cx, cy = x0 + dx / 2 - dy * bend, y0 + dy / 2 + dx * bend
    sx, sy = x0, y0 + (size * 0.7 if dy > 0 else -size * 0.7)  # the line leaves the lettering's edge, not its middle
    r.update(text=str(f.get("text", "")), size=round(size), W=W, H=H, lx=round(x0), ly=round(y0),
             d=f"M{sx:.0f} {sy:.0f} Q{cx:.0f} {cy:.0f} {x1:.0f} {y1:.0f}", dot=[round(x1), round(y1)],
             color=_color(f.get("color", "ink")))
    cap0 = geo["cap_top"] - 12
    if y0 + size > cap0 and y0 - size < geo["cap_top"] + 2 * geo["cap_size"] + 40:
        rep.warn("D9", w, f"pointer lettering at y {y0:.0f}px meets the caption band: move label")
    return True


def _place_text(r, f, geo, w, rep):
    """Where a text effect sits, kept out of the caption band and the platform UI unless it says otherwise."""
    W, H, s = geo["W"], geo["H"], geo["safe"]
    ty = r["type"]
    size = float(f.get("size") or TEXT_SIZE[ty])
    text = str(f.get("text", "")).replace("*", "")  # *hot* markup takes no width
    lines = max(1, len(text.split("\n")), math.ceil(max(map(len, text.split("\n"))) * size * 0.62 / (W - s["left"] - s["right"] - 40)))
    h = size * 1.08 * lines + (size * 0.5 if f.get("sub") else 0)
    if ty == "steps":
        h = len(r["items"]) * size * 1.6
    elif ty == "compare":
        h = (max(len(r["left"]), len(r["right"])) + (1 if r.get("heads") else 0)) * size * 1.55
    elif ty == "bars":
        h = len(r["items"]) * size * 2.2
    elif ty == "dots":
        size = min(size, (W - s["left"] - s["right"] - 40) / (r["cols"] * 1.25))
        h = r["rows"] * size * 1.25 + (size * 1.6 if text else 0) + (size * 1.2 if f.get("sub") else 0)
    elif ty == "ladder":
        n = max(len(r["left"]), len(r["right"])) * 1.5 + 1.6
        if f.get("size") is None and not f.get("allow_caption_zone"):  # a tall ladder shrinks to end above the captions
            y0 = float(f.get("y", TEXT_Y[ty]))
            room = geo["cap_top"] - 12 - 20 - (y0 * H if y0 <= 1 else y0)
            size = max(24.0, min(size, room / n)) if room > 0 else size
        h = n * size
    elif ty == "quote":
        h = size * 1.15 * lines + size * 1.0 + (size * 0.7 if f.get("sub") else 0)
    elif ty == "lower":
        h = size * 1.2 + (size * 0.85 if f.get("sub") else 0) + size * 0.5
    y = f.get("y", TEXT_Y[ty])
    if ty == "endtag" and f.get("y") is None:  # clear of the captions: above them, or below when they sit on top
        cap1 = geo["cap_top"] + 2 * geo["cap_size"] + 40
        top = geo["cap_top"] - h - 200 if geo["cap_top"] > H / 2 else cap1 + 220
    else:
        top = float(y) * H if float(y) <= 1 else float(y)
    r["box"] = {"left": s["left"] + 20, "width": W - s["left"] - s["right"] - 40, "top": round(top), "size": round(size),
                "h": round(h)}
    if ty in BLOCK_FX and geo["W"] > geo["H"]:
        r["box"]["width"] = min(r["box"]["width"], round(W * 0.58))
        if ty == "quote" or f.get("x") is None:
            r["box"]["left"] = round((W - r["box"]["width"]) / 2) if ty == "quote" else r["box"]["left"]
    if ty in {"stamp", "label", "lower"} and f.get("x") is not None:
        r["box"]["left"] = round(float(f["x"]) * W if float(f["x"]) <= 1 else float(f["x"]))
        r["box"]["width"] = None
    cap0, cap1 = geo["cap_top"] - 12, geo["cap_top"] + 2 * geo["cap_size"] + 40
    if not f.get("allow_caption_zone") and top < cap1 and top + h > cap0:
        rep.fail("D9", w, f"{ty} spans y {top:.0f}–{top + h:.0f}px, into the caption band ({cap0}–{cap1}px): move it (y) "
                 "or set allow_caption_zone after looking")
    if top < s["top"] or top + h > s["bottom"]:
        rep.warn("D9", w, f"{ty} spans y {top:.0f}–{top + h:.0f}px, outside the safe area ({s['top']}–{s['bottom']}px)")


def _sound_globals(d, beats, rep, out):
    snd = (d or {}).get("sound") or {}
    for j, a in enumerate(snd.get("ambience") or []):
        try:
            t0 = resolve_global(a.get("from", 0), beats)
            t1 = resolve_global(a.get("to", "end"), beats)
        except AnchorError as e:
            rep.fail("D3", f"direction.sound.ambience{j}", str(e))
            continue
        out["sfx"].append({"t": round(t0, 3), "dur": round(t1 - t0, 3), "kind": a.get("kind", "wind"), "db": float(a.get("db", -32)),
                           "src": f"sound.ambience{j}", "bed": True})
    for j, e in enumerate(snd.get("cues") or []):
        try:
            t = resolve_global(e["at"], beats)
        except (AnchorError, KeyError) as x:
            rep.fail("D3", f"direction.sound.cues{j}", str(x))
            continue
        out["sfx"].append({"t": round(t, 3), "kind": e["kind"], "db": float(e.get("db", -22)), "src": f"sound.cues{j}",
                           **({"dur": float(e["dur"])} if e.get("dur") else {})})


# ---------- checks over the resolved plan ----------

def variety(p: dict, tl: dict, rep: Report) -> None:
    """Pro-cut rules: few hard cuts, 3–6 kinds of transition, something new on screen every few seconds."""
    trs = p["transitions"]
    if len(trs) >= 4:
        hard = sum(1 for t in trs if t["type"] in HARD)
        if hard > len(trs) / 3:
            rep.warn("V1", "direction", f"{hard}/{len(trs)} transitions are plain cuts: keep it ≤ 1/3 (whip, zoom, flash, bloom…)")
        kinds = {t["type"] for t in trs}
        if len(trs) >= 6 and len(kinds) < 3:
            rep.warn("V2", "direction", f"only {len(kinds)} kinds of transition ({', '.join(sorted(kinds))}): use 3–5")
        if len(kinds) > 6:
            rep.warn("V2", "direction", f"{len(kinds)} kinds of transition: pick 3–5 so the cut has a grammar")
    covered = [(b["start"], b["end"]) for b in tl["beats"] if b["id"] in p["beats"] and not p["beats"][b["id"]].get("error")]
    ev = sorted(set([0.0] + p["events"] + [tl["duration"]]))
    for a, c in zip(ev, ev[1:]):
        if c - a <= EVENT_GAP_WARN:
            continue
        mid = (a + c) / 2
        if not any(s <= mid < e for s, e in covered):
            continue  # a custom or undirected beat answers for itself
        rep.warn("V3", "direction", f"{a:.2f}–{c:.2f}s: {c - a:.1f}s with nothing new on screen (a cut, punch-in or effect "
                 f"every ≤ {EVENT_GAP_WARN:g}s, unless the hold is the point)")
    for bid in p["missing"]:
        rep.info("V4", f"direction.{bid}", "not in direction.json: the scene is a placeholder or hand-authored")
    log = sorted(p.get("shotlog") or [], key=lambda x: x["at"])
    for a, b in zip(log, log[1:]):
        if a["cam"] == b["cam"] and a["cam"] not in STEADY_CAMS:
            rep.warn("V5", f"direction.{b['where']}", f"'{a['cam']}' again right after {a['where']}: two shots in a row "
                     "with the same camera move read as one; change the move, or hold one")
    lens = [x["end"] - x["at"] for x in log if x["end"] > x["at"]]
    if len(lens) >= 5:
        mean = sum(lens) / len(lens)
        cv = (sum((x - mean) ** 2 for x in lens) / len(lens)) ** 0.5 / mean
        if cv < 0.30:
            rep.warn("V6", "direction", f"{len(lens)} shots all about {mean:.1f}s long (spread {cv:.2f}): an even cut "
                     "ticks like a metronome; let a key line hold longer and a run of quick shots cut faster")


def check(cd: Path, sid: str, tl: dict, geo: dict, art: dict, media: dict) -> tuple[dict | None, Report]:
    d = load_direction(cd, sid)
    if d is None:
        r = Report()
        r.fail("D0", "direction", f"no {path(cd, sid).relative_to(cd)} (references/07-direction.md)")
        return None, r
    for k in ("rigs", "props"):  # the campaign's characters and set pieces, shared by every script; the direction's own win
        sp = cd / "07-video" / f"{k}.json"
        if sp.is_file():
            d = {**d, k: {**(load(sp).get(k) or {}), **(d.get(k) or {})}}
    p, rep = plan(d, tl, geo, art, media)
    _cast_rigs(cd, d, rep)
    pre = (d.get("look") or {}).get("preset")
    if pre and pre != "none" and pre not in STYLES:
        rep.fail("D0", "direction.look.preset", f"{pre!r} not in {sorted(STYLES)} (or drop it and set the look yourself)")
    if not str(d.get("storyline", "")).strip():
        rep.warn("D10", "direction", "no storyline: one line naming the picked storyline and why a stranger follows it")
    variety(p, tl, rep)
    _onscreen(cd, sid, d, p, rep)
    _devices(cd, sid, d, p, rep)
    return p, rep


def _devices(cd: Path, sid: str, d: dict, p: dict, rep: Report) -> None:
    """A beat whose line carries a number or a comparison shows it with a device (DV1 in lint asks for the name)."""
    from .craft import needs_device
    sp = cd / "05-scripts" / f"{sid}.json"
    if not sp.is_file():
        return
    for b in load(sp).get("beats") or []:
        slot = p["beats"].get(b.get("id"))
        if not slot or not needs_device(b) or str(b.get("device") or "").strip().lower() == "none":
            continue
        shown = {f["type"] for f in (slot.get("spec") or {}).get("fx") or []}
        if not shown & DEVICE_FX:
            rep.warn("DV1", f"direction.{b['id']}", f"the line carries a number or a comparison"
                     + (f" (device: {b['device']})" if b.get("device") else "")
                     + f" and no shot shows it: add one of {sorted(DEVICE_FX)}, or answer why the picture tells it")


def _cast_rigs(cd: Path, d: dict, rep: Report) -> None:
    """A character drawn in code that plays someone from the adaptation must read as them at a glance: it names
    its `cast` (07-video/art/art.json) and carries drawn parts (the hair, the costume, the prop that identify them)."""
    ap = cd / "07-video" / "art" / "art.json"
    cast = (load(ap).get("cast") or {}) if ap.is_file() else {}
    for name, R in (d.get("rigs") or {}).items():
        c = R.get("cast")
        if c and c not in cast:
            rep.warn("D15", f"rigs.{name}", f"cast {c!r} is not in art.json cast: name the adaptation's design there")
        if c and not R.get("parts"):
            rep.warn("D15", f"rigs.{name}", "plays a cast character with no drawn parts: give it the hair silhouette, "
                                            "costume and colours of its cast image, or nobody recognises them")


def _onscreen(cd: Path, sid: str, d: dict, p: dict, rep: Report) -> None:
    """A directed beat shows text only through captions and text effects: frame 0 (the cover) needs its line
    already on screen (`shown: true`), and a beat with captions off loses its `onscreen` line without a text fx.
    A captioned body beat needs none: a second text layer repeating the captions is clutter."""
    sp = cd / "05-scripts" / f"{sid}.json"
    if not sp.is_file():
        return
    first = True
    for b in load(sp).get("beats") or []:
        slot = p["beats"].get(b["id"])
        if slot is None or not str(b.get("onscreen") or "").strip():
            first = False
            continue
        fx = [f for f in slot["spec"]["fx"] if f["type"] in TEXT_FX] if slot.get("spec") else []
        if not fx and (first or b.get("captions") is False):
            rep.warn("D11", f"direction.{b['id']}", f"the script's on-screen line {b['onscreen']!r} is not shown: add a text fx "
                     "(title, slam, label) with it" + (" `shown: true`" if first else "") + ", or drop it from the script")
        elif first and not any(f.get("shown") and f["at"] <= 0.05 for f in fx):
            rep.warn("D11", f"direction.{b['id']}", "frame 0 is the cover and has no on-screen line yet: give the hook's text "
                     "fx `shown: true` at start")
        first = False


# ---------- HTML ----------

def _media_html(m: dict, sid: str, start: float, end: float, track: int) -> str:
    if m["kind"] == "raw":
        return m["html"]
    if m["kind"] == "color":
        c = m["color"]
        c = f"var(--mv-{c.replace('_', '-')})" if re.match(r"^[a-z_]+$", c) else c
        return f'<div class="mvk-color" style="background:{html.escape(c)}"></div>'
    src = html.escape(m["path"])
    if m["kind"] == "video":
        rate = f' data-playback-rate="{m["rate"]}"' if m.get("rate") else ""
        tag = (f'<video id="{sid}-v" class="clip" src="{src}" muted playsinline data-start="{start}" data-duration="{round(end - start, 3)}"'
               f' data-media-start="{m.get("trim", 0)}" data-track-index="{track}"{rate}></video>')
        return tag
    if m.get("card"):
        return f'<img class="mvk-card-bg" src="{src}" alt="" /><img class="mvk-card-fg" src="{src}" alt="" />'
    return f'<img src="{src}" alt="" />'


def _title_words(line: str) -> list[str]:
    """One span per word for the per-word reveal; *hot* runs keep their colour word by word."""
    out = []
    for tok in re.findall(r"\*[^*]+\*\S*|\S+", line):
        m = re.match(r"\*([^*]+)\*(\S*)$", tok)
        hot, body, tail = (True, m.group(1), m.group(2)) if m else (False, tok, "")
        ws = body.split()
        for k, w in enumerate(ws):
            # punctuation right after a *hot* run stays on its word ("*một quyển*." is not "một quyển .")
            out.append(f'<span class="mvk-w{" hot" if hot else ""}">{html.escape(w + (tail if k == len(ws) - 1 else ""))}</span>')
    return out


def _fx_html(f: dict) -> str:
    ty, el = f["type"], f["el"]
    if ty in {"flash", "black", "shake", "punch", "dim"}:
        return ""
    if ty == "html":
        return (f'      <div id="{el}" class="mvk-fx mvk-raw" style="{html.escape(f["style"])}" '
                f'data-layout-allow-overlap>{f["html"]}</div>')
    if ty == "pointer":
        x, y = f["dot"]
        return (f'      <div id="{el}" class="mvk-fx mvk-pointer" style="inset:0;--pc:{html.escape(f["color"])};--pw:{max(8, round(f["size"] * 0.2))}px" data-layout-allow-overlap>'
                f'<svg viewBox="0 0 {f["W"]} {f["H"]}" preserveAspectRatio="none"><path class="mvk-pt-line" d="{f["d"]}" '
                f'pathLength="1"/><circle class="mvk-pt-dot" cx="{x}" cy="{y}" r="{max(11, round(f["size"] * 0.27))}"/></svg>'
                f'<div class="mvk-pt-text" style="left:{f["lx"]}px;top:{f["ly"]}px;font-size:{f["size"]}px"><span>{_hot(f["text"])}</span></div></div>')
    if ty in {"callout", "tap"}:
        x, y, ww, hh = f["rect"]
        if ty == "tap":
            return (f'      <div id="{el}" class="mvk-fx mvk-tap" style="left:{x}px;top:{y}px">'
                    '<i class="mvk-tap-ring"></i><i class="mvk-tap-dot"></i></div>')
        lab = f.get("label")
        label = (f'<div class="mvk-callout-label" style="left:{lab["left"] - x}px;top:{lab["top"] - y}px;'
                 f'font-size:{lab["size"]}px">{_hot(f["text"])}</div>') if lab else ""
        spot = '<div class="mvk-callout-spot"></div>' if f.get("spot") else ""
        return (f'      <div id="{el}" class="mvk-fx mvk-callout" style="left:{x}px;top:{y}px;width:{ww}px;height:{hh}px" '
                f'data-layout-allow-overlap>{spot}<div class="mvk-callout-box"></div>{label}</div>')
    if ty == "speedlines":
        gs = "".join(f'<g class="mvk-sv"{"" if i == 0 else " style=\"opacity:0\""}>{g}</g>' for i, g in enumerate(f["polys"]))
        return (f'      <svg id="{el}" class="mvk-speed" viewBox="0 0 {f["W"]} {f["H"]}" preserveAspectRatio="none" '
                f'fill="{html.escape(f["color"])}" style="opacity:0" data-layout-ignore>{gs}</svg>')
    if ty == "weather":
        parts = "".join('<i class="mvk-p"></i>' for _ in range(f["n"]))
        return f'      <div id="{el}" class="mvk-weather {f["kind"]}" data-layout-ignore>{parts}</div>'
    b = f["box"]
    pos = f'left:{b["left"]}px;top:{b["top"]}px;font-size:{b["size"]}px;' + (f'width:{b["width"]}px;' if b.get("width") else "")
    text = f.get("text") or ""
    sub = f.get("sub")
    lines = [_hot(x) for x in text.split("\n")]
    if ty == "slam":
        inner = "<br>".join(lines) + (f"<small>{_hot(sub)}</small>" if sub else "")
    elif ty == "title":
        inner = "<br>".join(" ".join(_title_words(ln)) for ln in text.split("\n"))
    elif ty == "type":
        inner = "".join(f'<span class="mvk-c">{html.escape(c)}</span>' for c in text)
    elif ty == "stamp":
        inner = "<br>".join(lines)
        pos += "width:auto;display:inline-block;"
    elif ty == "label":
        inner = "<br>".join(lines) + (f"<small>{_hot(sub)}</small>" if sub else "")
        pos += "width:auto;"
    elif ty == "counter":
        inner = f'<span class="mvk-num">{html.escape(str(f.get("from", 0)))}</span>' + (f"<small>{_hot(sub)}</small>" if sub else "")
    elif ty == "steps":
        inner = "".join(f'<div class="mvk-step"><b>{i}</b><span>{_hot(x)}</span></div>' for i, x in enumerate(f["items"], 1))
    elif ty == "compare":
        hd = f.get("heads") or ["", ""]
        cols = []
        for side, items, head in (("l", f["left"], hd[0]), ("r", f["right"], hd[1] if len(hd) > 1 else "")):
            cols.append(f'<div class="mvk-cmp-col mvk-cmp-{side}">' + (f'<h4>{_hot(head)}</h4>' if head else "")
                        + "".join(f'<p>{_hot(x)}</p>' for x in items) + "</div>")
        inner = "".join(cols)
    elif ty == "bars":
        top = max(it["value"] for it in f["items"]) or 1
        mx = float(f.get("max") or top)
        suf = html.escape(str(f.get("suffix") or ""))
        inner = "".join(f'<div class="mvk-bar"><span>{_hot(it["label"])}</span><div class="mvk-bar-track">'
                        f'<i style="--w:{max(2.0, min(100.0, 100 * it["value"] / mx)):.1f}%"></i></div>'
                        f'<em>{it["value"]:g}{suf}</em></div>' for it in f["items"])
    elif ty == "dots":
        head = f'<div class="mvk-dots-head">{"<br>".join(lines)}</div>' if text else ""
        grid = "".join('<i class="lit"><b></b></i>' if k < f["lit"] else "<i></i>" for k in range(f["total"]))
        inner = (head + f'<div class="mvk-dot-grid" style="grid-template-columns:repeat({f["cols"]},1fr)">{grid}</div>'
                 + (f"<small>{_hot(sub)}</small>" if sub else ""))
    elif ty == "ladder":
        hd, mk = f.get("heads") or ["", ""], f.get("mark")
        cols = []
        for side, rungs, head in (("l", f["left"], hd[0]), ("r", f["right"], hd[1] if len(hd) > 1 else "")):
            off = f' style="transform:translateY({-f["offset"] * 100 / len(rungs):.1f}%)"' if side == "r" and f["offset"] else ""
            ps = "".join(f'<p{" class=\"hot\"" if mk and mk[0] == side and mk[1] == k else ""}>{_hot(x)}</p>'
                         for k, x in enumerate(rungs))
            cols.append(f'<div class="mvk-lad-col mvk-lad-{side}"><h4>{_hot(head)}</h4><div class="mvk-rungs"{off}>{ps}</div></div>')
        pos += f'height:{b["h"]}px;'
        inner = "".join(cols)
    elif ty == "quote":
        inner = ('<div class="mvk-q-mark">\u201c</div><div class="mvk-q-text">' + "<br>".join(" ".join(_title_words(ln)) for ln in text.split("\n"))
                 + "</div>" + (f"<small>{_hot(sub)}</small>" if sub else ""))
    elif ty == "lower":
        inner = f'<div class="mvk-lower-name">{"<br>".join(lines)}</div>' + (f'<div class="mvk-lower-sub">{_hot(sub)}</div>' if sub else "")
        pos += "width:auto;"
    elif ty == "endtag":
        return (f'      <div id="{el}" class="mvk-endtag" data-layout-allow-overlap>'
                f'<div class="mvk-end-fade"></div>'
                f'<div class="mvk-end-title" style="top:{b["top"]}px;font-size:{b["size"]}px;padding:0 {b["left"]}px">{"<br>".join(lines)}</div>'
                f'<div class="mvk-end-line" style="top:{b["top"] + b["h"] + 24}px;font-size:{round(b["size"] * 0.5)}px">{_hot(sub or "")}</div></div>')
    else:
        inner = "<br>".join(lines)
    shown = " mvk-shown" if f.get("shown") else ""
    return f'      <div id="{el}" class="mvk-fx mvk-{ty}{shown}" style="{pos}">{inner}</div>'


MASK_CLASS = {"bloom": "mvk-mask-bloom", "wipe": "mvk-mask-wipe"}


def _layer_html(L: dict, W: int, H: int) -> str:
    """#<layer> (parallax camera, full frame) > .mvk-ly (placed at its foot point) > .mvk-lym (entrance + moves)
    > [shadow] .mvk-lyh (hits) > .mvk-lyi (idle) > the art (one img per swap, stacked) or the rig's svg.
    One wrapper per motion so no two tweens write one property."""
    e = L.get("enter") or {}
    ly_style = "opacity:0;" if e.get("type") in {"fade", "pop"} else ""
    lym = {"slide-left": f"transform:translate({0.6 * W:g}px,0)", "slide-right": f"transform:translate({-0.6 * W:g}px,0)",
           "rise": f"transform:translate(0,{0.35 * H:g}px)", "drop": f"transform:translate(0,{-0.5 * H:g}px)",
           "pop": "transform:scale(0.6)"}.get(e.get("type"), "")
    if L.get("rig"):
        inner = rig.svg(L, H)
    elif L.get("vec"):
        inner = rig.vec_svg(L, H)
    else:
        flip = ";transform:scaleX(-1)" if L["flip"] else ""
        size = "" if L["anchor"] == "fill" else f"height:{round(L['h'] * H)}px"
        inner = "".join(f'<img class="mvk-lyart" src="{html.escape(src)}" alt="" style="{size}{flip}'
                        + (";opacity:0" if i else "") + '" />' for i, src in enumerate(L["srcs"]))
    shadow = '<div class="mvk-lysh"></div>' if L["shadow"] else ""
    blend = f' style="mix-blend-mode:{L["blend"]}"' if L.get("blend") else ""
    if L["anchor"] == "fill":  # a full-frame plane, overscanned so its parallax never shows an edge
        o = L["ov"]
        place = (f'class="mvk-ly mvk-lyc-center mvk-lyfill" style="{ly_style}left:{-o * W:.0f}px;top:{-o * H:.0f}px;'
                 f'width:{(1 + 2 * o) * W:.0f}px;height:{(1 + 2 * o) * H:.0f}px"')
    else:
        shift = "-100%" if L["anchor"] == "bottom" else "-50%"
        cls = "mvk-ly" if L["anchor"] == "bottom" else "mvk-ly mvk-lyc-center"  # a flying prop turns about its middle
        place = (f'class="{cls}" style="{ly_style}left:{round(L["x"] * W)}px;top:{round(L["y"] * H)}px;'
                 f'transform:translate(-50%,{shift})"')
    return (f'<div id="{L["id"]}" class="mvk-lyc"{blend}><div {place}><div class="mvk-lym" style="{lym}">{shadow}'
            f'<div class="mvk-lyh"><div class="mvk-lyi">{inner}</div></div></div></div></div>')


def _spec_for_js(sp: dict) -> dict:
    """The spec without the markup already written into the HTML (speed-line polygons, a rig's faces and emotes)."""
    fx = [{k: v for k, v in f.items() if k != "polys"} for f in sp["fx"]]
    shots = []
    for s in sp["shots"]:
        ls = []
        for L in s.get("layers") or []:
            if L.get("rig"):
                R = {k: v for k, v in L["rig"].items() if k not in {"faces", "parts", "dims"}}
                R["emotes"] = [{"at": e["at"], "dur": e["dur"]} for e in R["emotes"]]
                L = {**L, "rig": R}
            if L.get("vec"):
                L = {**L, "vec": {k: v for k, v in L["vec"].items() if k != "svg"}}
            ls.append(L)
        shots.append({**s, "layers": ls})
    return {**sp, "fx": fx, "shots": shots}


def _init_style(s: dict) -> str:
    """The state a way in starts from, inline, so the first frame of the shot's window is already right."""
    t, o = s["in"]["type"], s.get("origin") or [0.5, 0.45]
    css = {"fade": "opacity:0", "blur": "opacity:0;filter:blur(18px)",
           "bloom": f"--mvk-r:0%;--mvk-ox:{o[0] * 100:g}%;--mvk-oy:{o[1] * 100:g}%", "wipe": "--mvk-w:-30%",
           "iris": f"clip-path:circle(0% at {o[0] * 100:g}% {o[1] * 100:g}%)"}.get(t)
    return f' style="{css}"' if css else ""


def scene_html(bid: str, slot: dict, geo: dict) -> str:
    W, H = geo["W"], geo["H"]
    rows = []
    for k, s in enumerate(slot["shots_resolved"]):
        m = s["media"]
        scrim = s.get("scrim")
        scr = ("".join(f'<div class="mvk-scrim-{x}"></div>' for x in (["top", "bottom"] if scrim == "both" else [scrim]))
               if scrim else "")
        dim = f'<div id="{s["id"]}-d" class="mvk-dim" style="opacity:{s.get("dim") or 0}"></div>'
        t0 = round(s["start"] - s.get("lead", 0), 3)
        t1 = round(s.get("hideAt", s["end"]), 3)
        cls, style = "clip mvk-shot", _init_style(s)
        if s["in"]["type"] in MASK_CLASS:
            cls += " " + MASK_CLASS[s["in"]["type"]]
        rows.append(f'      <div id="{s["id"]}" class="{cls}" data-start="{t0}" data-duration="{round(t1 - t0, 3)}" '
                    f'data-track-index="{2 + k}"{style}><div class="mvk-kbo"><div class="mvk-kb" data-layout-allow-overflow>'
                    f'{_media_html(m, s["id"], t0, t1, 40 + k)}</div>{"".join(_layer_html(L, W, H) for L in s.get("layers") or [])}'
                    f'</div>{dim}{scr}</div>')
    fx = [x for x in (_fx_html(f) for f in slot["spec"]["fx"]) if x]
    spec = json.dumps(_spec_for_js(slot["spec"]), ensure_ascii=False, separators=(",", ":")).replace("</", "<\\/")
    return f"""<template>
  <!-- GENERATED by mvf scaffold from direction.json ({bid}): rewritten every run. Change the beat in
       direction.json, or set it to "custom" there to author this scene by hand. -->
  <div id="root" class="mvk-scene" data-mvf="generated" data-composition-id="{bid}" data-width="{W}" data-height="{H}"
       data-duration="{slot['dur']}" style="--mvk-over:{OVER * 100:g}%">
    <div id="{bid}-world" class="mvk-world">
{chr(10).join(rows)}
{chr(10).join(fx)}
    </div>
    <div id="{bid}-flash" class="mvk-flash"></div>
    <div id="{bid}-black" class="mvk-black"></div>
  </div>
  <script>
    (function () {{
      const tl = gsap.timeline({{ paused: true }});
      window.MVK.build(tl, {spec});
      window.__timelines["{bid}"] = tl;
    }})();
  </script>
</template>
"""


def overlay_html(look: dict, geo: dict, duration: float, chip_until: float | None) -> str:
    W, H, s = geo["W"], geo["H"], geo["safe"]
    g = look.get("grade") or {}
    grain, vig = float(g.get("grain", 0) or 0), float(g.get("vignette", 0) or 0)
    chip = look.get("chip")
    parts = []
    if vig > 0:
        parts.append(f'    <div id="overlay-vig" style="position:absolute;inset:0;background:radial-gradient(130% 95% at 50% 42%, '
                     f'rgba(0,0,0,0) 52%, rgba(0,0,0,{vig:.2f}) 100%)"></div>')
    if grain > 0:
        parts.append(f'    <div id="overlay-grain" style="position:absolute;inset:0;opacity:{grain:.3f};mix-blend-mode:overlay;'
                     f'background-image:url(assets/kit/grain.png);background-size:512px 512px"></div>')
    if look.get("letterbox"):
        lb = round(H * 0.06)
        parts.append(f'    <div style="position:absolute;left:0;right:0;top:0;height:{lb}px;background:#000"></div>'
                     f'<div style="position:absolute;left:0;right:0;bottom:0;height:{lb}px;background:#000"></div>')
    if chip:
        parts.append(f'    <div id="overlay-chip" style="position:absolute;left:{s["left"] + 20}px;top:{s["top"] + 24}px;max-width:'
                     f'{W - s["left"] - s["right"] - 40}px;display:flex;flex-direction:column;gap:6px;padding:16px 26px 18px 22px;'
                     f'border-left:8px solid var(--mv-hot);background:rgba(0,0,0,.6)">'
                     f'<b style="font:900 52px/1.02 var(--mv-display-font);color:var(--mv-on-plate)">{_hot(chip.get("text") or chip.get("title", ""))}</b>'
                     + (f'<span style="font:700 32px/1.1 var(--mv-body-font);color:var(--mv-hot)">{_hot(chip["sub"])}</span>'
                        if chip.get("sub") else "") + "</div>")
    o = {"dur": duration, "grain": grain > 0, "seed": 3, "chip": ({"until": chip_until} if chip else None)}
    return f"""<template>
  <!-- GENERATED by mvf scaffold from direction.json look: grade (grain, vignette), letterbox, series chip. -->
  <div id="root" data-composition-id="overlay" data-width="{W}" data-height="{H}" data-duration="{duration}"
       style="position:absolute;inset:0;pointer-events:none" data-layout-ignore>
{chr(10).join(parts)}
  </div>
  <script>
    (function () {{
      const tl = gsap.timeline({{ paused: true }});
      window.MVK.overlay(tl, {json.dumps(o)});
      window.__timelines["overlay"] = tl;
    }})();
  </script>
</template>
"""


def grain_png(path: Path, size: int = 256, seed: int = 9) -> None:
    """A tileable grey noise tile (stdlib PNG writer)."""
    import random
    import struct
    import zlib
    r = random.Random(seed)
    raw = b"".join(b"\x00" + bytes(max(0, min(255, int(r.gauss(128, 46)))) for _ in range(size)) for _ in range(size))
    def chunk(t, d):
        return struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d) & 0xFFFFFFFF)
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", size, size, 8, 0, 0, 0, 0)) + \
        chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(png)
