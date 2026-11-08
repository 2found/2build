"""The stick-figure rig: a character drawn in code (SVG), so every limb moves on a word.

A rig layer (`"rig": "stick"` instead of `"art"`) rides the same layer wrappers as a cut-out (placement,
parallax, enter, moves, hits, idle, tweens) and adds a body: joints whose angles the timeline tweens.

Angles are degrees. A limb at 0 hangs straight down (an arm, a thigh); the torso and head at 0 stand up.
Positive turns the far end toward screen-right — forward for an unflipped figure in side view — so a walk
reads naturally and `flip` faces it left. Elbows and knees are relative to their parent bone.

The pose library below is a set of shortcuts. Any pose may be a joint dict instead (merged over the
previous pose), and a raw tween on part `rig` can drive any `--<joint>` CSS variable directly.
"""
from __future__ import annotations

import html
import math
import re

JOINTS = ("lean", "head", "armL", "elbowL", "armR", "elbowR", "legL", "kneeL", "legR", "kneeR")
# body proportions (units; the feet stand at y = 100). `stick` is the classic figure; `doodle` is the chibi
# explainer figure: a big squarish head, a shirt shape, mitten hands and shoes on stick limbs. `dims` overrides any.
# `chibi` is the comic stick-figure look: a bold even outline, a head with a jaw and ears, a T-shirt with short
# sleeves on stick arms, round hands, a face set low with brows, blinking eyes.
_COMMON = {"side_fx": 3.5, "stand": None, "arms_front": False, "ears": False, "sleeve": 0, "face_y": 0.0, "face_line": 1.0, "blink": False, "face_set": "plain"}
BODIES = {
    "stick": {**_COMMON, "thigh": 22, "shin": 22, "torso": 30, "neck": 4, "head_r": 9, "upper": 17, "fore": 16,
              "shoulder": 0, "hand": 0, "foot": 0, "shirt": None, "head": "round", "width": 0.022},
    "doodle": {**_COMMON, "thigh": 15, "shin": 15, "torso": 22, "neck": 1, "head_r": 17, "upper": 13, "fore": 12,
               "shoulder": 5.5, "hand": 3.4, "foot": 3.6, "shirt": [7, 9], "head": "square", "width": 0.014},
    "chibi": {**_COMMON, "thigh": 12, "shin": 12, "torso": 20, "neck": 1.5, "head_r": 18.5, "upper": 12, "fore": 11,
              "shoulder": 6.6, "hand": 3.5, "foot": 3.8, "shirt": [7.4, 9.2], "head": "jaw", "width": 0.019,
              "side_fx": 4.6, "stand": {"armL": -17, "elbowL": 10, "armR": 17, "elbowR": -10},
              "arms_front": True, "ears": True, "sleeve": 6.5, "face_y": 2.4, "face_line": 0.8, "blink": True, "face_set": "chibi"},
}
# head outlines in the head's frame (centre at the origin, half-width 10). A rig may give its own path d instead.
HEADS = {
    "round": '<circle cx="0" cy="0" r="10"',
    "square": '<rect x="-10" y="-10.6" width="20" height="21.2" rx="5"',
    "jaw": '<path d="M -10 -5.5 C -10 -9.6 -7.6 -11 -4 -11 L 4 -11 C 7.6 -11 10 -9.6 10 -5.5 L 10 2.4 C 10 4.4 9.2 5.6 '
           '7.8 7 L 4.2 10.2 C 1.8 11.9 -1.8 11.9 -4.2 10.2 L -7.8 7 C -9.2 5.6 -10 4.4 -10 2.4 Z"',
}
HEADS_SIDE = {  # seen from the side (facing right): the jaw toward the face, the back of the skull round
    "jaw": '<path d="M -10 -5.5 C -10 -9.6 -7.6 -11 -4 -11 L 4 -11 C 7.6 -11 10 -9.6 10 -5.5 L 10 3 C 10 5.6 9 7.4 '
           '7.2 9 L 4.6 10.8 C 2 11.8 -2.6 11.8 -5.4 10.8 C -8.6 9.4 -10 6.6 -10 3 Z"',
}
# hair shortcuts in the head's frame, for the jaw head: {view: (behind the head, over it)}; {c} is the colour, {t} a
# thin strand line. Any other hair is the rig's own parts.
_HAIR = {
    "spiky": {
        "front": ("", '<path d="M -10.4 -1.2 L -12.6 -7 L -10.6 -8.4 L -12 -13.6 L -7 -11.8 L -6 -17.4 L -2 -13.4 L 1 -18.4 '
                      'L 3.6 -13.6 L 7.6 -17 L 7.8 -12.2 L 12.6 -13.2 L 10.8 -8.6 L 12.8 -6 L 10.4 -1.6 L 9.4 -5.4 L 7.6 -2.6 '
                      'L 6 -6.6 L 3.8 -3.2 L 2 -7 L -0.4 -3.6 L -2.4 -7 L -4.6 -3.4 L -6.4 -6.8 L -8.2 -3.2 Z" fill="{c}"/>'
                      '<path d="M -3.8 -12.6 L -2.6 -9.4 M 2.8 -12.8 L 2 -9.6 M 7.4 -11 L 6 -8.6" stroke-width="{t}"/>'),
        "side": ("", '<path d="M 10.4 -2 L 12.4 -6.6 L 10.6 -8.6 L 12.2 -13.4 L 7.6 -12.4 L 6.6 -17.6 L 2.6 -13.8 L -1 -18.2 '
                     'L -3 -13.6 L -7.6 -16.4 L -7.6 -12 L -12.6 -12.6 L -11 -8 L -13.2 -5.6 L -10.6 -2.8 L -10.4 3.2 '
                     'C -8.8 1.4 -7.4 -0.6 -6.6 -2.8 L -4.6 -1.8 L -3.6 -5.4 L -1 -2.6 L 0.6 -6.4 L 3 -3 L 4.8 -6.6 L 6.8 -3 '
                     'L 8.4 -6 Z" fill="{c}"/><path d="M -2 -12.8 L -1.2 -9.6 M 4 -12.6 L 3.4 -9.6" stroke-width="{t}"/>'),
    },
    "crop": {
        "front": ("", '<path d="M -10.6 -0.6 C -11.8 -8 -8.6 -12.8 -2 -13.2 C 5 -13.6 11.4 -10.2 10.8 -0.8 L 9.4 -4.6 '
                      'C 6 -5.2 3.4 -6 1.6 -7.6 C -0.6 -5.8 -4.6 -4.8 -9 -4.6 Z" fill="{c}"/>'
                      '<path d="M 1.6 -7.6 C 2.6 -9.6 4.4 -10.6 6.4 -11" stroke-width="{t}"/>'),
        "side": ("", '<path d="M 10.6 -3 C 11 -10 6.6 -13.4 -0.4 -13.4 C -8 -13.4 -11.8 -9 -11 -1 L -10.6 4.4 '
                     'C -8.8 2.4 -8 -0.4 -7.4 -3.4 C -2 -4.4 4.4 -5 8.4 -6.6 Z" fill="{c}"/>'),
    },
    "messy": {
        "front": ("", '<path d="M -10.8 0 L -12.4 -4.6 L -11.2 -9.6 L -8.4 -12.6 L -9.2 -15.2 L -5 -14.2 L -3 -17 L -0.4 -14.6 '
                      'L 3.6 -16.6 L 5 -13.6 L 9.4 -14.6 L 9 -11.4 L 12.4 -9.4 L 11 -6 L 12.2 -2.6 L 10.4 -0.8 L 9.4 -5.6 '
                      'L 7.6 -3.8 L 6.8 -7.4 L 4.4 -5 L 3.6 -8.4 L 1 -5.4 L -0.6 -8.6 L -2.8 -5.2 L -4.2 -8.2 L -6.6 -4.6 '
                      'L -7.6 -7.6 L -9.2 -3.2 Z" fill="{c}"/>'
                      '<path d="M -6 -12.4 L -4.6 -9.4 M 0.6 -13.4 L 1.2 -10.2 M 6.4 -12 L 5.4 -9" stroke-width="{t}"/>'),
        "side": ("", '<path d="M 10.6 -2.6 L 12 -6.4 L 10.8 -10 L 8.4 -12.8 L 9 -15.4 L 4.8 -14.4 L 2.6 -17.2 L 0 -14.6 '
                     'L -4 -16.4 L -5.4 -13.4 L -9.8 -14 L -9.4 -11 L -12.6 -9 L -11.4 -5.8 L -12.8 -2.6 L -10.6 3.8 '
                     'C -9 1.6 -7.8 -0.8 -7 -3.4 L -4.8 -2.2 L -3.4 -6 L -1 -3 L 1 -6.6 L 3.2 -3.4 L 5.4 -7 L 7.4 -3.4 '
                     'L 8.8 -6.4 Z" fill="{c}"/>'),
    },
    "bun": {
        "front": ('<circle cx="0" cy="-15.6" r="4.6" fill="{c}"/>',
                  '<path d="M -10.8 0.6 C -12 -8.4 -8 -12.8 0 -12.8 C 8 -12.8 12 -8.4 10.8 0.6 L 9.6 -4.4 C 6.4 -4.6 3 -5.8 '
                  '0 -8.4 C -3 -5.8 -6.4 -4.6 -9.6 -4.4 Z" fill="{c}"/>'
                  '<path d="M -3.4 -12.4 C -2.2 -10.4 -1 -9.4 0 -8.4 M 3.4 -12.4 C 2.2 -10.4 1 -9.4 0 -8.4" stroke-width="{t}"/>'),
        "side": ('<circle cx="-4.6" cy="-14.6" r="4.6" fill="{c}"/>',
                 '<path d="M 10.6 -3 C 10.6 -10 6 -12.8 -1 -12.8 C -8.4 -12.8 -11.6 -8.6 -11 -1 L -10.6 5 '
                 'C -9 2.6 -8 -0.6 -7.4 -3.2 C -2.6 -4 3.6 -4.6 8.4 -6.4 Z" fill="{c}"/>'),
    },
    "long": {
        "front": ('<path d="M -11.6 -2 C -13 6 -13 13 -11.6 19 L 11.6 19 C 13 13 13 6 11.6 -2 Z" fill="{c}"/>',
                  '<path d="M -11 3.4 C -12.2 -8.4 -8 -12.8 0 -12.8 C 8 -12.8 12.2 -8.4 11 3.4 L 9.4 -3.6 C 5 -4.4 1.6 -6.4 '
                  '-0.4 -9 C -2.4 -6 -5.6 -4.2 -9.4 -3.6 Z" fill="{c}"/>'
                  '<path d="M -0.4 -9 C 1.6 -10.6 4 -11.4 6.4 -11.4" stroke-width="{t}"/>'),
        "side": ('<path d="M -11.4 -3 C -13.2 6 -13 13 -11 19 L -1 19 C -2.6 12 -3 6 -2.6 -1 Z" fill="{c}"/>',
                 '<path d="M 10.6 -3 C 10.6 -10 6 -12.8 -1 -12.8 C -8.4 -12.8 -11.8 -8.6 -11.2 -1 L -10.6 7 '
                 'C -8.6 3.6 -7.6 0 -7.2 -3.4 C -2.6 -4 3.6 -4.6 8.4 -6.4 Z" fill="{c}"/>'),
    },
}
HAIRS = tuple(_HAIR)
STICK = BODIES["stick"]
FIG_H = 96.0  # the stick figure's height in units (feet to the top of its head)


def body(R: dict) -> dict:
    dm = {**BODIES.get(R.get("body", "stick"), STICK), **(R.get("dims") or {})}
    for k in ("ears", "sleeve", "blink", "head"):  # the commonest dims also read from the rig itself
        if k in R:
            dm[k] = R[k]
    dm["fig_h"] = dm["thigh"] + dm["shin"] + dm["torso"] + dm["neck"] + 2 * dm["head_r"]
    return dm


STAND = {"lean": 0, "head": 0, "armL": -10, "elbowL": 6, "armR": 10, "elbowR": -6, "legL": -5, "kneeL": 0, "legR": 5, "kneeR": 0}
POSES = {
    "stand": {},  # a body may rest wider (its `stand`); every other pose is a change from rest
    "wave": {"armR": 140, "elbowR": 30},
    "cheer": {"armL": -145, "elbowL": -20, "armR": 145, "elbowR": 20},
    "point": {"armR": 90, "elbowR": 0},
    "shrug": {"head": 8, "armL": -35, "elbowL": -95, "armR": 35, "elbowR": 95},
    "think": {"head": 10, "armR": 25, "elbowR": 140},
    "hands-up": {"armL": -120, "elbowL": -40, "armR": 120, "elbowR": 40},
    "crouch": {"lean": 0, "armL": -30, "elbowL": -30, "armR": 30, "elbowR": 30,
               "legL": -35, "kneeL": 70, "legR": 35, "kneeR": -70},
    # side view (facing right; flip faces left)
    "sit": {"lean": 5, "head": 0, "armL": 50, "elbowL": 50, "armR": 55, "elbowR": 45,
            "legL": 85, "kneeL": -85, "legR": 80, "kneeR": -80},
    "bow": {"lean": 40, "head": 15, "armL": 20, "elbowL": 0, "armR": 25, "elbowR": 0},
    "slump": {"lean": 18, "head": 28, "armL": 8, "elbowL": 0, "armR": 12, "elbowR": 0},
}
WALK = [  # one stride cycle, side view facing right: contact, passing, contact, passing
    {"lean": 4, "head": 0, "legL": 25, "kneeL": -5, "legR": -20, "kneeR": -25, "armL": -25, "elbowL": 15, "armR": 25, "elbowR": 20},
    {"lean": 4, "head": 0, "legL": 0, "kneeL": 0, "legR": 10, "kneeR": -50, "armL": 0, "elbowL": 10, "armR": 0, "elbowR": 10},
    {"lean": 4, "head": 0, "legR": 25, "kneeR": -5, "legL": -20, "kneeL": -25, "armR": -25, "elbowR": 15, "armL": 25, "elbowL": 20},
    {"lean": 4, "head": 0, "legR": 0, "kneeR": 0, "legL": 10, "kneeL": -50, "armR": 0, "elbowR": 10, "armL": 0, "elbowL": 10},
]

# faces in head coordinates (the head's centre is (0, -13)); eyes and a mouth (.rig-m, hidden while talking)
FACES = {
    "calm": ('<circle cx="-3.2" cy="-14" r="1.1" class="rig-dot"/><circle cx="3.2" cy="-14" r="1.1" class="rig-dot"/>',
             '<path d="M -2.5 -9.6 Q 0 -8.2 2.5 -9.6"/>'),
    "happy": ('<path d="M -4.4 -13.6 Q -3.2 -15.4 -2 -13.6"/><path d="M 2 -13.6 Q 3.2 -15.4 4.4 -13.6"/>',
              '<path d="M -3.6 -10.2 Q 0 -6.4 3.6 -10.2"/>'),
    "sad": ('<circle cx="-3.2" cy="-13.6" r="1.1" class="rig-dot"/><circle cx="3.2" cy="-13.6" r="1.1" class="rig-dot"/>'
            '<path d="M -5 -16.2 L -1.8 -17.2"/><path d="M 5 -16.2 L 1.8 -17.2"/>',
            '<path d="M -3 -8.2 Q 0 -10.6 3 -8.2"/>'),
    "angry": ('<circle cx="-3.2" cy="-13.6" r="1.1" class="rig-dot"/><circle cx="3.2" cy="-13.6" r="1.1" class="rig-dot"/>'
              '<path d="M -5 -17.6 L -1.6 -15.8"/><path d="M 5 -17.6 L 1.6 -15.8"/>',
              '<path d="M -2.6 -8.8 L 2.6 -8.8"/>'),
    "surprised": ('<circle cx="-3.2" cy="-14.2" r="1.6" class="rig-dot"/><circle cx="3.2" cy="-14.2" r="1.6" class="rig-dot"/>',
                  '<ellipse cx="0" cy="-8.6" rx="1.8" ry="2.2"/>'),
    "laugh": ('<path d="M -4.6 -13.4 Q -3.2 -15.6 -1.8 -13.4"/><path d="M 1.8 -13.4 Q 3.2 -15.6 4.6 -13.4"/>',
              '<path d="M -3.8 -10.4 Q 0 -4.6 3.8 -10.4 Z" class="rig-dot"/>'),
    "blank": ('<circle cx="-3.2" cy="-14" r="1.1" class="rig-dot"/><circle cx="3.2" cy="-14" r="1.1" class="rig-dot"/>', ''),
}
MOUTH_OPEN = '<ellipse cx="0" cy="-9" rx="2.1" ry="1.7" class="rig-dot"/>'
# the chibi set: the same coordinates; eyes sit in .rig-ey (they blink), {b} is a brow's line, {t} a thin one
_RED, _TONGUE = "#b8333d", "#ef8a8f"
_DOTS = '<g class="rig-ey"><circle cx="-3.4" cy="{y}" r="1.25" class="rig-dot"/><circle cx="3.4" cy="{y}" r="1.25" class="rig-dot"/></g>'
_BROWS = '<path d="M -5.2 -17.6 L -2 -17.9" stroke-width="{b}"/><path d="M 2 -17.9 L 5.2 -17.6" stroke-width="{b}"/>'
_OPEN = lambda w, top, bot: (f'<path d="M {-w:g} {top:g} L {w:g} {top:g} Q {w * 0.92:g} {bot:g} 0 {bot:g} Q {-w * 0.92:g} {bot:g} '
                             f'{-w:g} {top:g} Z" fill="{_RED}"/><path d="M {-w * 0.55:g} {bot - 0.7:g} Q 0 {bot - 2.2:g} '
                             f'{w * 0.55:g} {bot - 0.7:g} Q 0 {bot + 0.1:g} {-w * 0.55:g} {bot - 0.7:g} Z" fill="{_TONGUE}" stroke="none"/>')
FACES_CHIBI = {
    "calm": (_DOTS.format(y=-14) + _BROWS, '<path d="M -1.5 -9.5 Q 0 -8.6 1.5 -9.5"/>'),
    "blank": (_DOTS.format(y=-14) + _BROWS, ''),
    "happy": ('<g class="rig-ey"><path d="M -4.9 -13.5 Q -3.4 -15.7 -1.9 -13.5" stroke-width="{b}"/>'
              '<path d="M 1.9 -13.5 Q 3.4 -15.7 4.9 -13.5" stroke-width="{b}"/></g>'
              '<path d="M -5.2 -18.4 Q -3.6 -19.3 -2 -18.6" stroke-width="{b}"/><path d="M 2 -18.6 Q 3.6 -19.3 5.2 -18.4" stroke-width="{b}"/>',
              _OPEN(2.8, -10.4, -6.6)),
    "laugh": ('<path d="M -5.2 -15.6 L -2.2 -14.1 L -5.2 -12.6" stroke-width="{b}"/><path d="M 5.2 -15.6 L 2.2 -14.1 L 5.2 -12.6" stroke-width="{b}"/>'
              '<path d="M -5.2 -18.6 Q -3.6 -19.6 -2 -18.8" stroke-width="{b}"/><path d="M 2 -18.8 Q 3.6 -19.6 5.2 -18.6" stroke-width="{b}"/>',
              _OPEN(3.6, -10.8, -5.4)),
    "angry": (_DOTS.format(y=-13.8) + '<path d="M -5.6 -18.6 L -1.8 -16.4" stroke-width="{b}"/><path d="M 5.6 -18.6 L 1.8 -16.4" stroke-width="{b}"/>',
              '<path d="M -2 -8.8 Q 0 -10.3 2 -8.8"/>'),
    "shout": (_DOTS.format(y=-13.8) + '<path d="M -5.6 -18.6 L -1.8 -16.4" stroke-width="{b}"/><path d="M 5.6 -18.6 L 1.8 -16.4" stroke-width="{b}"/>',
              _OPEN(3.0, -11, -5.6)),
    "shocked": ('<g class="rig-ey"><circle cx="-3.5" cy="-14" r="2.3" fill="#ffffff"/><circle cx="3.5" cy="-14" r="2.3" fill="#ffffff"/>'
                '<circle cx="-3.5" cy="-14" r="0.95" class="rig-dot"/><circle cx="3.5" cy="-14" r="0.95" class="rig-dot"/></g>'
                '<path d="M -5.6 -19.4 Q -3.8 -20.8 -2 -19.8" stroke-width="{b}"/><path d="M 2 -19.8 Q 3.8 -20.8 5.6 -19.4" stroke-width="{b}"/>',
                f'<ellipse cx="0" cy="-8.4" rx="1.5" ry="2.1" fill="{_RED}"/>'),
    "surprised": (_DOTS.format(y=-14.2) + '<path d="M -5.4 -19 Q -3.6 -20.2 -2 -19.4" stroke-width="{b}"/>'
                  '<path d="M 2 -19.4 Q 3.6 -20.2 5.4 -19" stroke-width="{b}"/>',
                  f'<ellipse cx="0" cy="-8.8" rx="1.2" ry="1.5" fill="{_RED}"/>'),
    "sad": (_DOTS.format(y=-13.6) + '<path d="M -5.4 -16.6 L -2 -18.2" stroke-width="{b}"/><path d="M 5.4 -16.6 L 2 -18.2" stroke-width="{b}"/>',
            '<path d="M -1.8 -8.6 Q 0 -10 1.8 -8.6"/>'),
    "cry": ('<g class="rig-ey"><path d="M -5 -13.4 Q -3.4 -15.2 -1.8 -13.4" stroke-width="{b}"/>'
            '<path d="M 1.8 -13.4 Q 3.4 -15.2 5 -13.4" stroke-width="{b}"/></g>'
            '<path d="M -3.6 -12.6 Q -4.4 -9.4 -3.8 -6" stroke="#5fb4ea" stroke-width="{b}"/>'
            '<path d="M 3.6 -12.6 Q 4.4 -9.4 3.8 -6" stroke="#5fb4ea" stroke-width="{b}"/>'
            '<path d="M -5.4 -16.6 L -2 -18.2" stroke-width="{b}"/><path d="M 5.4 -16.6 L 2 -18.2" stroke-width="{b}"/>',
            f'<path d="M -2.4 -9.4 Q 0 -11 2.4 -9.4 Q 1.6 -7.2 0 -7.4 Q -1.6 -7.2 -2.4 -9.4 Z" fill="{_RED}"/>'),
    "smug": ('<g class="rig-ey"><path d="M -5.3 -14.6 L -1.7 -14.6"/><path d="M 1.7 -14.6 L 5.3 -14.6"/>'
             '<path d="M -4.4 -14.6 A 1.1 1.1 0 0 0 -2.2 -14.6 Z" class="rig-dot"/><path d="M 2.6 -14.6 A 1.1 1.1 0 0 0 4.8 -14.6 Z" class="rig-dot"/></g>'
             '<path d="M -5.2 -17.8 L -2 -18.2" stroke-width="{b}"/><path d="M 2 -19.2 Q 3.6 -20 5.2 -19" stroke-width="{b}"/>',
             '<path d="M -1.8 -9.6 Q 1 -8.6 2.8 -10.8"/>'),
    "disgust": ('<path d="M -5.2 -15.4 L -2.2 -14 L -5.2 -12.6" stroke-width="{b}"/>'
                '<g class="rig-ey"><path d="M 1.8 -14.8 L 5.2 -14.8"/><path d="M 2.4 -14.8 A 1.1 1.1 0 0 0 4.6 -14.8 Z" class="rig-dot"/></g>'
                '<path d="M -5.4 -16.6 L -2 -16.8" stroke-width="{b}"/><path d="M 2 -18.8 L 5.4 -19.4" stroke-width="{b}"/>',
                '<path d="M -2.6 -9 Q -1.7 -10 -0.9 -9 Q 0 -8 0.9 -9 Q 1.7 -10 2.6 -9"/>'),
    "think": ('<g class="rig-ey"><circle cx="-2.4" cy="-15" r="1.2" class="rig-dot"/><circle cx="4.4" cy="-15" r="1.2" class="rig-dot"/></g>'
              '<path d="M -5 -18.2 L -1.8 -18.2" stroke-width="{b}"/><path d="M 2.2 -19.4 Q 3.8 -20.4 5.4 -19.4" stroke-width="{b}"/>',
              '<path d="M 0.2 -9.4 L 2.2 -9.8"/>'),
    "worried": (_DOTS.format(y=-13.8) + '<path d="M -5.2 -17 L -2 -18.4" stroke-width="{b}"/><path d="M 5.2 -17 L 2 -18.4" stroke-width="{b}"/>',
                '<path d="M -2 -9.2 Q -1 -10 0 -9.2 Q 1 -8.4 2 -9.2"/>'),
    "focus": ('<g class="rig-ey"><ellipse cx="-3.4" cy="-14" rx="1.25" ry="0.9" class="rig-dot"/>'
              '<ellipse cx="3.4" cy="-14" rx="1.25" ry="0.9" class="rig-dot"/></g>'
              '<path d="M -5.4 -16.6 L -1.8 -16.2" stroke-width="{b}"/><path d="M 1.8 -16.2 L 5.4 -16.6" stroke-width="{b}"/>',
              '<path d="M -1.4 -9.4 L 1.4 -9.4"/>'),
}
MOUTH_OPEN_CHIBI = _OPEN(2.1, -10.4, -7.2)
# marks beside the head: (anchor x, anchor y, mark around the anchor). The anchor is in the stick head's coordinates
# (centre (0,-13), r 9) and moves out with a bigger head; the mark keeps a stick-figure size (grows only as sqrt(r/9))
EMOTES = {
    "!": (13, -25, '<path d="M 0 -6 L 0 2"/><circle cx="0" cy="5.5" r="0.9" class="rig-dot"/>'),
    "?": (13.5, -26, '<path d="M -3.5 -2 Q -3.5 -6 0 -6 Q 3.5 -6 3.5 -2.5 Q 3.5 0.5 0 1.5 L 0 3.5"/>'
                     '<circle cx="0" cy="6.5" r="0.9" class="rig-dot"/>'),
    "sweat": (-13, -19.5, '<path d="M 0 -3.5 Q -2.6 1.9 0 2.9 Q 2.6 1.9 0 -3.5 Z"/>'),
    "anger": (11.5, -26.2, '<g class="rig-hot"><path d="M -3.5 1.2 Q -1.5 1.7 -1.1 3.8"/><path d="M 3.5 1.2 Q 1.5 1.7 1.1 3.8"/>'
                           '<path d="M -3.5 -1.2 Q -1.5 -1.8 -1.1 -3.8"/><path d="M 3.5 -1.2 Q 1.5 -1.8 1.1 -3.8"/></g>'),
    "spark": (12, -22, '<path d="M -2 1 L 2.5 -3.5"/><path d="M 0 5.5 L 5.5 4.5"/><path d="M -5 -1.5 L -4 -7"/>'),
}


class RigError(ValueError):
    pass


def face_lib(dm: dict) -> dict:
    """The built-in faces for this body (the chibi set has brows and colour; both share every plain name)."""
    return FACES_CHIBI if dm.get("face_set") == "chibi" else FACES


def hair_parts(spec) -> list[dict]:
    """A hair shortcut (`"spiky"` or `{"style": "spiky", "color": "#e8507a"}`) as head parts, front and side."""
    st, c = (spec, "#1f1f24") if isinstance(spec, str) else (spec.get("style"), spec.get("color", "#1f1f24"))
    if st not in _HAIR:
        raise RigError(f"hair {st!r} not in {', '.join(HAIRS)} (or draw it: parts on head)")
    out = []
    for view, (back, front) in _HAIR[st].items():
        for svg_, is_back in ((back, True), (front, False)):
            if svg_:  # the back of long hair falls behind the shirt: it rides the torso, drawn in the head's frame
                behind = is_back and st == "long"
                out.append({"on": "torso" if behind else "head", "view": view, "back": is_back, "hair": True,
                            "svg": svg_.replace("{c}", html.escape(str(c))), **({"head_frame": True} if behind else {})})
    return out


def scope_ids(markup: str, prefix: str) -> str:
    """Raw SVG from the agent (a dot pattern, a gradient, a clip) names its ids per layer: every scene shares one
    document, so a set piece reused in two shots would otherwise point at the first copy, hidden in another beat."""
    ids = set(re.findall(r'\bid="([^"]+)"', markup))
    if not ids:
        return markup
    pat = "|".join(re.escape(i) for i in sorted(ids, key=len, reverse=True))
    markup = re.sub(rf'\bid="({pat})"', lambda m: f'id="{prefix}-{m.group(1)}"', markup)
    return re.sub(rf'(url\(#|href="#)({pat})(?=[)"])', lambda m: f'{m.group(1)}{prefix}-{m.group(2)}', markup)


def pose(spec, prev: dict | None = None, extra: dict | None = None, dm: dict = STICK) -> dict:
    """A named pose, or a joint dict merged over the previous one (or over `stand`)."""
    stand = {**STAND, **(dm.get("stand") or {})}  # a body may rest its arms wider (a chibi's clear its shirt)
    base = dict(prev or stand)
    lib = {**POSES, "stand": stand, **(extra or {})}
    if isinstance(spec, str):
        if spec not in lib:
            raise RigError(f"pose {spec!r} not in {sorted(lib)} (or give the joints: {{armR: 120, ...}})")
        base = {**stand, **lib[spec]}
    elif isinstance(spec, dict):
        if spec.get("pose"):
            base = pose(spec["pose"], base, extra, dm)
        bad = [k for k in spec if k not in JOINTS and k not in {"pose", "drop"}]
        if bad:
            raise RigError(f"unknown joint(s) {bad}: joints are {', '.join(JOINTS)}")
        base.update({k: float(v) for k, v in spec.items() if k in JOINTS})
        if "drop" in spec:
            base["drop"] = float(spec["drop"])
            return base
    else:
        raise RigError("a pose is a name or a joint dict")
    base.pop("drop", None)
    base["drop"] = drop(base, dm)
    return base


def drop(p: dict, dm: dict = STICK) -> float:
    """How far the hips sink so the lower foot stays on the ground (a crouch, a sit, the bob of a walk)."""
    def foot(leg, knee):
        a, b = math.radians(p.get(leg, 0)), math.radians(p.get(leg, 0) + p.get(knee, 0))
        return dm["thigh"] * math.cos(a) + dm["shin"] * math.cos(b)
    return round(dm["thigh"] + dm["shin"] - max(foot("legL", "kneeL"), foot("legR", "kneeR")), 2)


def scaled(p: dict, amt: float, rest: dict) -> dict:
    return {k: round(rest.get(k, 0) + (v - rest.get(k, 0)) * amt, 2) for k, v in p.items()}


def walk(at: float, until: float, speed: float, amt: float, prev: dict, dm: dict = STICK) -> list[dict]:
    """A stride cycle from `at` to `until` as pose tweens, then back to the pose before it. speed: steps a second."""
    phase = 1.0 / (2.0 * max(0.3, speed))
    out, t, k, cur = [], at, 0, prev
    while t + phase <= until + 1e-6:
        to = pose(scaled(WALK[k % 4], amt, STAND), cur, None, dm)
        out.append({"at": round(t, 3), "dur": round(phase, 3), "ease": "sine.inOut", "from": cur, "to": to})
        cur, t, k = to, t + phase, k + 1
    if out:
        back = {**prev}
        out.append({"at": round(t, 3), "dur": 0.25, "ease": "power2.out", "from": cur, "to": back})
    return out


def flaps(words: list[tuple[float, float]], a: float, b: float) -> list[tuple[float, float]]:
    """Mouth open/close times over the spoken words in [a, b]: open on each word, closed in the gaps,
    a long word closes once in its middle."""
    out = []
    for s, e in words:
        if e <= a or s >= b:
            continue
        s, e = max(s, a), min(e, b)
        if e - s > 0.42:
            m = (s + e) / 2
            out += [(s, m - 0.04), (m + 0.04, e - 0.03)]
        elif e - s > 0.08:
            out.append((s, e - 0.03))
    return [(round(x, 3), round(y, 3)) for x, y in out if y - x >= 0.05]


def _css(p: dict) -> str:
    return ";".join(f"--{k}:{p[k]:g}" for k in (*JOINTS, "drop"))


PART_BONES = ("hip", "torso", "chest", "head", "armL", "handL", "armR", "handR", "legL", "footL", "legR", "footR")


def view_box(dm: dict) -> tuple[float, float, float, float]:
    """Room for raised arms, hair and emotes above the head and a stride either side; the bottom is the feet."""
    top = 100 - dm["fig_h"] - max(16.0, dm["head_r"] * 1.3)
    return (-60.0, round(top, 2), 120.0, round(100 - top + max(2.0, dm["foot"]), 2))


def svg(L: dict, H: int) -> str:
    """The figure: nested bones, each a <g> turned by a CSS variable on the <svg> (so one tween moves a pose).
    Parts (raw SVG the agent drew: hair, a costume, a prop) ride their bone; the head's frame has its centre at
    the origin and a radius of 10, every other bone its joint at the origin and the limb along +y."""
    R, dm = L["rig"], L["rig"]["dims"]
    vx, vy, vw, vh = view_box(dm)
    hpx = L["h"] * H * vh / dm["fig_h"]
    wpx = hpx * vw / vh
    ink, sw = html.escape(R["ink"]), R["width"] * dm["fig_h"]
    col = lambda k, d: html.escape(str(R.get(k) or d))
    skin, shoes = col("skin", "none" if dm["head"] == "round" else "#ffffff"), col("shoes", R["ink"])
    hands = col("hands", R.get("skin") or "#ffffff")
    side = R.get("view") == "side"
    parts = {b: ("", "") for b in PART_BONES}
    for pt in R.get("parts") or []:
        if pt.get("view") and (pt["view"] == "side") != side:
            continue  # a part drawn for the other view (side hair parted toward the face, say)
        back, front = parts.get(pt.get("on"), ("", ""))
        mk = pt["svg"].replace("{t}", f'{R["width"] * dm["fig_h"] * 10 / dm["head_r"] * 0.55:.2f}') if pt.get("hair") else pt["svg"]
        if pt.get("head_frame"):  # drawn in the head's frame, wherever the bone is (the head at rest)
            hy = -(dm["torso"] + dm["neck"] + dm["head_r"])
            mk = (f'<g stroke-width="{R["width"] * dm["fig_h"] * 10 / dm["head_r"]:.2f}" style="transform:translate(0px,'
                  f'{hy:g}px) scale({dm["head_r"] / 10:g})">{mk}</g>')
        g = f'<g class="rig-p {html.escape(str(pt.get("class", "")))}">{scope_ids(mk, L["id"])}</g>'
        parts[pt["on"]] = (back + g, front) if pt.get("back") else (back, front + g)
    rot = lambda j, sign: f'style="transform:rotate(calc(var(--{j}) * {sign}1deg))"'
    tr = lambda x, y: f'style="transform:translate({x:g}px,{y:g}px)"'

    def limb(j1, j2, l1, l2, top, low, end, stroke="", over=""):
        st = f' stroke="{stroke}"' if stroke else ""
        return (f'<g {rot(j1, "-")}>{parts[top][0]}<line x1="0" y1="0" x2="0" y2="{l1:g}"{st}/>{over}{parts[top][1]}'
                f'<g {tr(0, l1)}><g {rot(j2, "-")}>{parts[low][0]}<line x1="0" y1="0" x2="0" y2="{l2:g}"{st}/>'
                f'{end}{parts[low][1]}</g></g></g>')

    hand = lambda: (f'<circle cx="0" cy="{dm["fore"]:g}" r="{dm["hand"]:g}" fill="{hands}"/>' if dm["hand"] else "")
    foot = lambda: (f'<ellipse cx="{1.4 if side else 0:g}" cy="{dm["shin"] + dm["foot"] * 0.3:g}" rx="{dm["foot"]:g}" '
                    f'ry="{dm["foot"] * 0.6:g}" fill="{shoes}"/>' if dm["foot"] else "")
    pants = col("pants", "")
    legs = (limb("legL", "kneeL", dm["thigh"], dm["shin"], "legL", "footL", foot(), pants)
            + limb("legR", "kneeR", dm["thigh"], dm["shin"], "legR", "footR", foot(), pants))
    sx = 0 if side else dm["shoulder"]
    sl = min(float(dm["sleeve"] or 0), dm["upper"]) if dm["shirt"] else 0
    # a short sleeve: the shirt's colour around the top of the upper arm, turning with it (the torso hides its inside)
    sleeve = (f'<path d="M -3.1 -2.4 Q 0 -3.6 3.1 -2.4 L 3.7 {sl:g} Q 0 {sl + 0.9:g} -3.7 {sl:g} Z" '
              f'fill="{col("sleeves", R.get("shirt") or "#ffffff")}"/>' if sl else "")
    arm_line = col("arms", "") if sl else col("sleeves", "")
    arm = lambda s_, j1, j2, top, low: (f'<g {tr(s_, 2 if dm["shirt"] else 0)}>'
                                        f'{limb(j1, j2, dm["upper"], dm["fore"], top, low, hand(), arm_line, sleeve)}</g>')
    armL, armR = arm(-sx, "armL", "elbowL", "armL", "handL"), arm(sx, "armR", "elbowR", "armR", "handR")
    # the head: its frame is centred, radius 10; the faces are drawn in the stick head's coordinates (centre (0,-13), r 9)
    r = dm["head_r"]
    hd = str(dm["head"])
    shape = (f'<path d="{html.escape(hd)}"' if hd[:1] in "Mm" else (HEADS_SIDE if side else {}).get(hd) or HEADS.get(hd, HEADS["square"]))
    shape += f' fill="{skin}"/>'
    hsw = sw * 10 / r  # the outline in the head's frame
    ear = ""
    if dm["ears"]:  # a C ear: behind the head from the front (its outer half shows), over it from the side
        curl = lambda x, d: f'<path d="M {x:g} 0 Q {x + d * 1.5:g} 1.6 {x:g} 3.2" stroke-width="{hsw * 0.6:.2f}"/>'
        ear = (f'<ellipse cx="-6.4" cy="3.6" rx="1.7" ry="2.4" fill="{skin}"/>{curl(-6.8, 0.7)}' if side else
               "".join(f'<ellipse cx="{x:g}" cy="1.6" rx="2.6" ry="3.4" fill="{skin}"/>{curl(x * 1.03, x / abs(x))}'
                       for x in (-10.4, 10.4)))
    fx = dm["side_fx"] if side else 0.0
    sq = 0.6 if side else 1.0
    fill_in = lambda m: m.replace("{b}", f"{hsw * 0.9 * 1.5:.2f}").replace("{t}", f"{hsw * 0.9 * 0.55:.2f}")
    faces = {**face_lib(dm), **{k: (v, "") if isinstance(v, str) else tuple(v) for k, v in (R.get("faces") or {}).items()}}
    fgs = "".join(f'<g class="rig-f rig-f-{html.escape(n)}" style="opacity:{1 if n == R["face0"] else 0}">{fill_in(eyes)}'
                  f'<g class="rig-m">{fill_in(mouth)}</g></g>' for n, (eyes, mouth) in faces.items() if n in R["faces_used"])
    if R.get("blush"):
        fgs = (f'<g stroke="none" fill="{col("blush", "")}" opacity="0.55"><ellipse cx="-6.3" cy="-10.6" rx="2.1" ry="1.1"/>'
               f'<ellipse cx="6.3" cy="-10.6" rx="2.1" ry="1.1"/></g>') + fgs
    # a raw-svg emote is drawn in the stick head's coordinates and scales with the head; a built-in or text one is
    # anchored beside the head at a stick-figure size
    ek = math.sqrt(r / 9)
    emote = lambda i, e: (f'<g class="rig-e rig-e{i}" style="opacity:0"><g class="rig-ei">{e["svg"]}</g></g>' if "anchor" not in e else
                          f'<g class="rig-e rig-e{i}" stroke-width="{sw / ek:.2f}" style="opacity:0;transform:translate('
                          f'{e["anchor"][0] * r / 9:g}px,{(e["anchor"][1] + 13) * r / 9 - dm["neck"] - r:g}px) '
                          f'scale({ek:.3f})"><g class="rig-ei">{e["svg"]}</g></g>')
    emotes = "".join(emote(i, e) for i, e in enumerate(R["emotes"]) if "anchor" not in e)
    anchored = "".join(emote(i, e) for i, e in enumerate(R["emotes"]) if "anchor" in e)
    mo = MOUTH_OPEN_CHIBI if dm.get("face_set") == "chibi" else MOUTH_OPEN
    fl = f' stroke-width="{hsw * dm["face_line"]:.2f}"' if dm["face_line"] != 1 else ""
    face = (f'<g style="transform:translate(0px,{dm["face_y"]:g}px) scale({10 / 9:.4f}) translate(0px,13px)">'
            f'<g{fl} style="transform:translate({fx:g}px,-13px) scale({sq:g},1) translate(0px,13px)">{fgs}'
            f'<g class="rig-mo" style="opacity:0">{mo}</g></g>{emotes}</g>')
    head = (f'<g {rot("head", "")}><line x1="0" y1="0" x2="0" y2="{-dm["neck"]:g}"/>'
            f'<g stroke-width="{hsw:.2f}" style="transform:translate(0px,{-(dm["neck"] + r):g}px) scale({r / 10:g})">'
            f'{parts["head"][0]}{"" if side else ear}{shape}{face}{parts["head"][1]}{ear if side else ""}</g>{anchored}</g>')
    T = dm["torso"]
    if dm["shirt"]:
        ws, wh = dm["shirt"]
        k = 0.72 if side else 1.0
        trunk = (f'<path d="M {-wh * k:g} 0 L {wh * k:g} 0 L {ws * k:g} {-T:g} L {-ws * k:g} {-T:g} Z" '
                 f'fill="{col("shirt", "#ffffff")}"/>')
    else:
        trunk = f'<line x1="0" y1="0" x2="0" y2="{-T:g}"/>'
    # front view: both arms behind the shirt, or (arms_front) over it and under the head, so a hand reaches the chin
    front = armR if side else (armL + armR if dm["arms_front"] else "")
    chest = f'<g {tr(0, -T)}>{parts["chest"][0]}{front}{head}{parts["chest"][1]}</g>'
    behind = armL if side else ("" if dm["arms_front"] else armL + armR)
    narrow = f'style="transform:scale({0.72 if side and dm["shirt"] else 1:g},1)"'  # a costume turns with the shirt
    lean = (f'<g {rot("lean", "")}><g {tr(0, -T)}>{behind}</g><g {narrow}>{parts["torso"][0]}</g>{trunk}'
            f'<g {narrow}>{parts["torso"][1]}</g>{chest}</g>')
    hip_y = 100 - dm["thigh"] - dm["shin"]
    bodyg = (f'<g style="transform:translate(0px,calc({hip_y:g}px + var(--drop) * 1px))">{parts["hip"][0]}{legs}'
             f'{lean}{parts["hip"][1]}</g>')
    filt, use = boil_filter(f'{L["id"]}-boil', R["boil"])
    flip = "transform:scaleX(-1);" if L["flip"] else ""
    return (f'<svg class="mvk-lyart mvk-rig" viewBox="{vx:g} {vy:g} {vw:g} {vh:g}" width="{wpx:.0f}" height="{hpx:.0f}" '
            f'style="{flip}{_css(R["pose0"])};--rig-ink:{ink}" overflow="visible">{filt}'
            f'<g{use} stroke="{ink}" stroke-width="{sw:.2f}" fill="none" stroke-linecap="round" stroke-linejoin="round">'
            f'{bodyg}</g></svg>')


def boil_filter(fid: str, boil: float) -> tuple[str, str]:
    """The hand-drawn wobble: a displacement whose noise re-seeds on fixed steps (mvkit.js). (defs, filter attr)"""
    if boil <= 0:
        return "", ""
    return (f'<defs><filter id="{fid}" x="-20%" y="-20%" width="140%" height="140%">'
            f'<feTurbulence type="fractalNoise" baseFrequency="0.07" numOctaves="2" seed="1"/>'
            f'<feDisplacementMap in="SourceGraphic" scale="{boil * 2:g}" xChannelSelector="R" yChannelSelector="G"/>'
            f'</filter></defs>', f' filter="url(#{fid})"')


def vec_svg(L: dict, H: int) -> str:
    """A prop the agent drew as raw SVG (a step, a stool, a ground line), in the rig's ink, line and boil."""
    V = L["vec"]
    vx, vy, vw, vh = V["vb"]
    hpx = L["h"] * H
    wpx = hpx * vw / vh
    sw = V["stroke"] * H * vh / hpx  # px on screen -> viewBox units
    flip = "transform:scaleX(-1);" if L["flip"] else ""
    # the boil is in viewBox units: wobble as many px as it does on a figure 0.4 of the frame tall, at any prop size
    defs, use = boil_filter(f'{L["id"]}-boil', V["boil"] * (0.4 * H / FIG_H) * vh / hpx)
    return (f'<svg class="mvk-lyart mvk-vec" viewBox="{vx:g} {vy:g} {vw:g} {vh:g}" width="{wpx:.0f}" height="{hpx:.0f}" '
            f'style="{flip}--rig-ink:{html.escape(V["ink"])}" overflow="visible">{defs}'
            f'<g{use} stroke="{html.escape(V["ink"])}" stroke-width="{sw:.2f}" fill="{html.escape(V["fill"])}" '
            f'stroke-linecap="round" stroke-linejoin="round">{scope_ids(V["svg"], L["id"])}</g></svg>')
