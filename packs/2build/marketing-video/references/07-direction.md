# Stage 7b - Direction (optional helper)

Output: `07-video/<sid>/direction.json`, the shots and effects of one script as
data. It is one way to build scenes, not a style guide: the look, the shot
choice and the pacing are yours. Skip it and author `compositions/<beat>.html`
by hand (SCENE_GUIDE.md), or use another editor entirely.

When the file exists, `mvf scaffold` resolves every anchor against the measured
voice and writes each directed beat as a generated scene (`data-mvf="generated"`,
rewritten on every run) on the mvkit runtime (`tools/templates/kit/`). A
re-voice then moves every cut and effect with its word. A beat set to
`"custom"` is authored by hand, and scaffold keeps it.

You direct; the keys are shortcuts. Raw markup and raw GSAP are first-class at
every level, so nothing here caps what a scene can show. A shot plate can be
`html`, and an effect can be `{type: "html"}`. A beat, a layer or a rig can take
`tweens` with any selector, a camera can be a dict, and `look.css`/`look.html`
add rules or markup to index.html. Checks FAIL only on what would break the
render, the timing or the platform. Pacing and composition are WARN advice that
you may overrule.

```json
{ "storyline": "the picked idea in one line",
  "look": { "colors": {"ground": "#…", "ink": "#…", "accent": "#…", "hot": "#…", "on_plate": "#…"},
            "fonts": {"display": "…", "body": "…", "caption": "…", "files": {"…": "brand/fonts/….ttf"}},
            "grade": {"grain": 0.0, "vignette": 0.0}, "letterbox": false, "slam_case": "none",
            "chip": {"text": "series or product card", "sub": "", "until": "B02@end"} },
  "captions": {"style": "stroke|shadow|box|comic", "words": 3, "weight": 850, "size": null, "y": null,
               "css": "", "custom": false},
  "sound": { "ambience": [{"kind": "room", "from": 0, "to": "end", "db": -32}],
             "cues": [{"at": "B04@word:saved", "kind": "impact", "db": -16}] },
  "beats": {
    "B01": {"shots": [
      {"image": "01-product/screens/inbox.png", "card": true, "cam": "push:1.3",
       "fx": [{"type": "slam", "text": "3 *HOURS*", "at": "word:hours", "shown": true}]},
      {"art": "P02", "at": "word:every", "in": "whip"}]},
    "B05": "custom" } }
```
A beat may set `captions_y` (a frame share) to lift its captions off a face.

`look.preset` (`stickman`, `ink`, `anime`, or `none`; see
`tools/mvflib/styles.json`) sets these under whatever the look and `captions`
write themselves:
- the colours, grade, fonts and caption style;
- `paper` (text effects in ink on a light ground instead of white with a glow);
- its rig's body, ink, line and boil.

A direction that names no preset takes the campaign's `style` (campaign.json,
`stickman` for a new campaign). `"preset": "none"` opts out.
- `captions.css` appends your own rules to the generated captions.
- `captions.custom: true` keeps a hand-authored `compositions/captions.html`
  instead of regenerating it, so retiming it is then yours.
- `look.css` and `look.html` append raw CSS (it wins over the kit) and raw
  markup (above the scenes, under the captions) to index.html.
`look.chip` is a corner card (`text`, `sub`, `until`). It covers that corner of
every plate it overlaps, frame 0 included.

## Anchors
Every `at` and `until` is an anchor, local to its beat:
- **Time:** `1.25` is seconds from the beat's start; `start`, `end` and `40%`
  also work.
- **Word:** `word:saved` is the start of the first matching word. Accents, case
  and punctuation are folded, and a phrase matches a run of words. `#2` picks
  the second match and `±0.1` shifts it. `wordend:` is the word's end, and
  `w:3` / `wend:3` are the fourth word's start and end. Offsets chain
  (`word:nhặt đá+0.4-0.1`).
- **Shot:** inside a shot after the first, `start` and `end` are that shot's
  own bounds, so its layers and keys can say `start+0.3`.
- **Global:** in `sound` and `chip`, use `B03@word:rain`; plain seconds there
  are absolute.

## Shots
A shot shows one of these:
- `art`: an approved id. A draft shows with a WARN, and export refuses it.
- `image` or `video`: a path inside the campaign. A video takes `trim` and
  `rate`.
- `color`: a look role or a hex.
- `html`: raw HTML or inline SVG drawn as the plate. The camera still moves it,
  and the layers sit on it.

`card: true` floats the shot over a blurred copy of itself.

| Key | Values |
| --- | --- |
| `at` | when it cuts in; required on every shot after the first; after the shot before (within 0.2 s WARNs: a flash cut) |
| `in` | `cut`, `flash`, `dip`, `whip`, `zoom`, `fade`, `blur`, `bloom`, `wipe`, `iris`, `push`, or `{type, dur, dir}`; beat 1 always cuts |
| `cam` | `push`, `pull`, `pan-left`, `pan-right`, `tilt-up`, `tilt-down`, `drift`, `hold`, optionally `:strength`, or `{from:{s,x,y}, to:{s,x,y}, ease}` where `x`/`y` shift the scaled plate as a share of the frame (0 = centred; to centre point `[px, py]` at scale `s` use `x = -(px-0.5)*s`, `y = -(py-0.5)*s`); clamped so no edge shows |
| `focus` | `[x, y]` (0–1), the point the camera and punch-ins favour |
| `keys` | punch-ins `{at, s, focus, dur, ease, sfx, db}`, inside their shot; `focus` is on the framed (post-`cam`) picture |
| `dim`, `scrim` | darken the plate (0–1), or `top` / `bottom` / `both` gradients for text |
| `fx` | effects bound to this shot's time |
| `sfx` | `false` mutes its automatic sound, or `[{at, kind, db}]` |

## Layers (2.5D)
A shot's `layers` stack cut-outs over its plate, so the character and the
world move apart. A nearer layer travels further when the camera moves
(parallax), and each layer can enter, move, idle and take a hit on a word. It
is livelier than one flat plate when the beat has an action: something
arrives, is struck or crosses the frame. A held talking head gains little.

```json
{"art": "P04-BG", "cam": "push:1.2", "layers": [
  {"art": "HERO-SIT", "x": 0.5, "y": 0.93, "h": 0.55, "z": 0.3, "shadow": true, "idle": "breathe",
   "hits": [{"at": "word:ngực", "from": "left"}, {"at": "word:trán", "from": "left", "amt": 0.6}]},
  {"art": "PEBBLE", "x": -0.1, "y": 0.45, "h": 0.05, "z": 0.6, "anchor": "center",
   "moves": [{"at": "word:ném", "x": 0.47, "y": 0.62, "dur": 0.35, "ease": "power2.in"}]}]}
```

| Key | Values |
| --- | --- |
| `art` | an approved cut-out (`use: sticker`, `transparent: true`); an opaque one WARNs (it hides the plate) unless it has a `blend` |
| `rig` | `stick` (or `{type: stick, body, hair, view: front|side, ink, width, boil, dims, parts, poses, faces}`) in place of `art`: a figure drawn in code (below). A puppet of your own is an `svg` layer plus `tweens` on its `sel`s |
| `svg`, `viewBox` | raw SVG markup in place of `art`: a prop the agent draws (a step, a door, a stone), in the rig's ink, line (`stroke`, a frame share) and boil; `fill`, `class="rig-dot"` fills with the ink. Its `id`s are scoped to the layer, so a reused `pattern` or `clipPath` cannot collide; `href="media:<campaign path>"` places a campaign file (a real screen inside a drawn phone) |
| `prop` | a name in `07-video/props.json` (`{"props": {NAME: {svg, viewBox, z, anchor, …}}}`, under the direction's own `props`): a set piece drawn once and placed many times; the layer's keys override the sheet's |
| `x`, `y` | where its foot point sits, as frame shares (default 0.5, 0.92) |
| `anchor` | `bottom` (feet on the ground, the default), `center` (a flying prop) or `fill` (a full-frame plane, overscanned past its parallax: foreground grass, mist, a far ridge) |
| `h` | height as a share of the frame; keep it within the job's approved `display` |
| `z` | depth: 0 moves with the plate, 0.25 by default, 1 twice as far, below 0 less than the plate (-1 still, lower WARNs: it runs against the plate); layers paint in list order whatever their z. Whatever touches the floor (a house, a desk, the actors) shares the floor's z, or its feet slide when the camera drifts |
| `blend` | a CSS `mix-blend-mode` (`multiply` drops line art's white into the paper; `screen` lays light) |
| `swaps` | `[{at, art}]`: show another drawing of the same layer from that word (a pose, an expression) |
| `flip`, `shadow` | mirror it; a soft contact shadow under the feet |
| `enter` | `fade`, `pop`, `slide-left`, `slide-right`, `rise` or `drop`, or `{type, at, dur}` |
| `moves` | `[{at, x?, y?, s?, rot?, dur, ease, antic?}]`: to a frame point, scale or angle; each starts where the last ended. A far move (≥ 3% of the width, Δs ≥ 0.06 or ≥ 6°) of ≥ 0.35 s with no `ease` leans back first and settles past the mark (`antic`, default 0.08 of the travel); `antic: false` or any `ease` turns it off |
| `idle` | `breathe`, `bob` or `sway`, or `{type, amt, period}`: a loop over the whole shot |
| `hits` | `[{at, from: left|right|front, amt, sfx?}]`: a recoil away from the blow, silent unless `sfx` names a sound |
| `ghosts` | `[{at, until, count (1–6, default 3), gap (s, 0.05), opacity (0.45), style?}]`: after-images on a dash, copies of the layer drawn behind it, each `gap` later and fainter; `style` is raw CSS on the copies (`filter:brightness(0)` makes ink silhouettes) |
| `tweens` | `[{at, part | sel, from?, to, dur, ease}]`: raw GSAP vars on one `part` (`layer`, `move`, `hit`, `idle`, `img`, `rig`) or any CSS selector `sel` inside the layer; `repeat: -1` becomes a finite loop over the shot |

The named moves are shortcuts, not the limit of the cut. Write `tweens` for
anything else (a squash, a blur, a colour shift, a walk cycle as keyframes),
and give a tween the part no shortcut animates at the same moment, or the two
fight. When the beat needs more than data can say, set it `"custom"` and write
the scene; `window.MVK.layer(tl, shot, layer, W, H)` plays a resolved layer
there, and the rest is your own GSAP.

### Stick rig
A `rig: stick` layer is a stick figure in SVG whose joints the timeline turns:
`lean`, `head`, `armL`, `elbowL`, `armR`, `elbowR`, `legL`, `kneeL`, `legR`,
`kneeR`, in degrees. A limb at 0 hangs down and the torso stands up; positive
turns the far end toward screen-right, which is forward for an unflipped
figure in side view (`flip` faces it left). Elbows and knees are relative to
their bone. The hips sink by themselves so the lower foot stays on the ground.

| Key | Values |
| --- | --- |
| `pose` | the first pose: `stand`, `wave`, `cheer`, `point`, `shrug`, `think`, `hands-up`, `crouch`, `bow`, `slump`, `sit` (side view), a `rig.poses` name, or a joint dict |
| `poses` | `[{at, pose?, set?, face?, dur, ease, antic?, snap?}]`: `pose` names one, `set` changes the joints it names over the last pose, `face` changes the face. A swing of ≥ 15° over ≥ 0.25 s with no `ease` gets the same lean-back and settle (`antic: false` opts out). `snap: true` holds the last pose and jumps to this one on the key: the release frame of a blow |
| `walk` | `[{at, until, speed, amt}]`: a stride cycle (side view; pair it with a `moves` across), back to the pose before it |
| `face` | `calm`, `happy`, `sad`, `angry`, `surprised`, `blank`; the `chibi` body adds `laugh`, `shout`, `shocked`, `cry`, `smug`, `disgust`, `think`, `worried`, `focus`. Or one of `rig.faces` (`{name: [eyes svg, mouth svg]}` in head coordinates, centre `(0, -13)`, radius 9; on chibi `{b}` is a brow's line width and `{t}` a thin one, and eyes inside `<g class="rig-ey">` blink) |
| `talk` | `true` or `[{at, until}]`: the mouth opens on each spoken word of the beat (the character is the voice) |
| `emotes` | `[{at, type | text | svg, dur, dx?, dy?}]`: `!`, `?`, `sweat`, `anger`, `spark` or any text, beside the head at a stick-figure size (`dx`/`dy` nudge it clear of the hair); raw SVG is drawn in head coordinates and scales with the head |

The pose names are shortcuts. A raw tween on part `rig` drives any `--<joint>`
variable (`{"part": "rig", "to": {"--elbowR": 70, "yoyo": true, "repeat": -1}}`
works the hands), and `sel` reaches any element of the figure (`.rig-f-happy`,
`.rig-mo`). A raw tween and a pose that turn one joint at the same moment
fight. In a custom scene `window.MVK.rig(tl, shot, layer)` plays a resolved
rig.

**A character.** The rig object (or a name in `07-video/rigs.json`
`{"rigs": {NAME: {...}}}`, the campaign's character sheet, under the
direction's own `rigs`) carries the look; a layer names it (`"rig": "AN"`) or
overrides it (`"rig": {"use": "AN", "view": "side"}`). With `use`, the
layer's `faces` and `poses` add to the sheet's and its `parts` append to the
sheet's (a knife in the hand for one shot).

| Key | Values |
| --- | --- |
| `body` | `chibi` (the `stickman` default: an angular jaw head about 45% of the figure, C ears, sleeved T-shirt, single-line arms with round hands in front of the body, a lower face with thick brows and red open mouths, eyes that blink), `doodle` (square head, shirt, mitten hands) or `stick` (round head, bare lines). `dims` overrides any length (`thigh`, `shin`, `torso`, `neck`, `head_r`, `upper`, `fore`, `shoulder`, `hand`, `foot`, `shirt: [shoulder, hem]` half-widths) and any trait: `head` (`round`, `square`, `jaw` or a raw path `d` in the head frame), `ears`, `sleeve` (length), `face_y` (how low the face sits), `blink`, `arms_front`, `stand` (rest arm joints) |
| `view` | `front` (default) or `side` (the face turns, the shirt narrows, the near arm comes in front) |
| `hair` | `spiky`, `crop`, `messy`, `bun`, `long`, or `{style, color}`: a hair silhouette in front and side views (`long` falls behind the shirt). Raw hair is `parts` on `head` |
| `skin`, `shirt`, `pants`, `sleeves`, `hands`, `shoes`, `arms`, `blush` | fill and line colours; `arms` colours a sleeved arm's line, `blush` draws cheeks in that colour |
| `parts` | `[{on, svg, back?, view?}]`: raw SVG riding a bone (`hip`, `torso`, `chest`, `head`, `armL`, `handL`, `armR`, `handR`, `legL`, `footL`, `legR`, `footR`); `back` draws it behind the bone; `view` keeps it to one view |
| `cast` | the `art.json` cast id it plays |
| `steps` | drawings a second for its pose tweens (12 is "on twos" at 24 fps): each pose holds, then steps, the way hand-drawn frames do; 0 (default) tweens smoothly |

Coordinates: `head` is centred with a radius of 10 at any `head_r` (`x`
right, `y` down); `torso` has the hip at the origin and the neck at
`(0, -torso)`; a limb bone has its joint at the origin and runs down `+y`.
Hair is two head parts: the back mass (`back: true`) and the bangs in front of
the face. A costume detail (collar, stripes, belt) is a `torso` part, a
carried prop a `handR` part. A side view with a front-view fringe looks wrong:
give the hair a `view: side` variant.

The plate under the layers is drawn without them: the empty lane, not the lane
with the boy in it (07 Art). Entrances, moves and hits count as something new
on screen for V3; an idle loop does not.

## Effects
An effect is `{type, at, until | dur, …}`, at beat level or on a shot:

| Type | Keys |
| --- | --- |
| `slam` (a word hits the frame) | `text` (`*hot*` words coloured), `sub`, `size`, `y`, `shown` |
| `title` (words rise one by one) | `text` (`\n` breaks), `size`, `y`, `shown` |
| `type` (typed characters) | `text`, `size`, `y` |
| `stamp` / `label` | `text`, `x`, `y`, `rot` |
| `counter` | `from`, `to`, `suffix`, `locale` |
| `flash` / `black` | — |
| `shake` / `punch` | `amp` |
| `freeze` (hit-stop on contact) | `dur` (default 0.12 s), `amp` (the frame shakes in real time while the scene holds). It holds the frame two frames after contact, where the release pose and the recoil have landed, and a `flash` or `speedlines` in the beat plays in real time over the hold; the scene then plays faster for 0.3 s and is back on the voice, so later word anchors stay on their words. Give the shake here, not as a `shake` at the same moment |
| `dim` | `shot`, `amt` |
| `weather` (`rain`, `snow`, `embers`, `petals`) | `count`, `seed` |
| `speedlines` (manga lines on an impact or a dash) | `kind` (`radial` toward `x`, `y`, stopping at `inner`; `linear` along `angle`), `count`, `width`, `color` (a look role or a colour), `opacity`, `fps`, `variants`, `seed` |
| `endtag` (the last card, on a scrim for a dark plate) | `text`, `sub`, `y` (its top in px; by default above the captions, or below them when `captions.y` puts them in the top half) |
| `callout` (a box on one region of a real screen) | `rect` `[x, y, w, h]` (frame shares or px), `text` (its label, placed below the box or above it), `spot: true` dims the rest |
| `tap` (a ring where a finger presses) | `x`, `y` |
| `steps` (numbered items, one per spoken step) | `items` (or `text` lines), `ats` (one anchor per item), `focus: true` dims the earlier ones |
| `compare` (two columns) | `heads` `[left, right]`, `left`, `right` (lists), `ats` |
| `bars` (a measured comparison) | `items` `[{label, value}]`, `max`, `suffix`; every value names a fact in the beat |
| `dots` (a share you can count: 3 in 100) | `total` (1–400), `lit`, `cols` (default about a 1.6:1 grid), `text` (`*hot*`), `sub` (what one dot is); the lit dots light one by one |
| `ladder` (two ranks side by side: levels of one against the other) | `left`, `right` (rung labels, bottom first; over 16 WARNs), `heads` `[left, right]`, `mark` `["l"|"r", index]` (the rung the line names), `offset` (how far the right column sits up, a share of a rung, so equal rungs do not read as equal ranks) |
| `quote` (one cited line, word by word) | `text`, `sub` (who said it, where) |
| `lower` (a name strip) | `text` (the name), `sub` (the role) |
| `pointer` (comic lettering naming a point) | `text` (uppercase, `*hot*`), `point` `[x, y]` (what it names), `label` `[x, y]` (where the lettering sits), `size`, `bend`, `color`; the line draws on from the lettering and ends in a dot |
| `html` (anything else) | `html` (raw HTML or inline SVG), `style` (its box, e.g. `left:80px;top:300px;width:600px`), `in` `fade`/`pop`/`draw`/`none` (`draw` strokes every `[pathLength]` path on, `stagger`, `run`), `out: none` |

A beat's `tweens` (`[{at, sel, from?, to, dur, ease}]`) are raw GSAP over
anything in its scene: `sel` is scoped to the scene, and `#id` is used as is.
`#B03-s1` is a shot, so a tween on it is a transition of your own (`in: cut`
plus your tween). `#B03-fx0` is an effect.

`steps`, `compare`, `bars`, `dots`, `ladder` and `quote` stay until the beat ends unless `until`
or `dur` is set. `rect`, `x`, `y` and `size` may be per canvas, such as
`{"9x16": [0.1, 0.3, 0.8, 0.12], "16x9": [0.3, 0.2, 0.4, 0.15]}`, because a
share of a tall frame is not the same spot on a wide one. A missing canvas
FAILs (D8). A callout box or tap ring under the caption band WARNs (D9).
Which kinds of video these suit is in [formats](formats.md).

**Camera.** Frame a layered shot with its `cam` (a `{from, to}` that holds
the framing and drifts in a touch gives the far layers their parallax) and
punch in with `keys`. A key scales every layer as one picture, around the
framed frame, so it has no parallax and a key near `s: 1` cannot pan (the
shift is clamped to `(s-1)/2`). On a light paper plate, a title drawn as an
`svg` layer in the ink reads better than `endtag`, which is white on a scrim.

Entrances start from nothing, so frame 0 is blank unless a text effect sets
`shown: true`. A script beat's `captions: false` hides its captions where an
effect carries the words.

## Sound
Transitions, punch-ins and text effects place their own cues in the project's
`sfx.json`. A cue that lands on a spoken word gets `sfx: false`, and the mix
check flags it otherwise. `sound.ambience` lays wind, rain or room tone.
`sound.cues` adds named sounds: `sword`, `gong`, `heartbeat`, `bell`, `taiko`,
`riser`, `boom`, `impact`, `pop`, `click` and `ding`. `mvf score` renders them (07 Tool).
A script beat with `key_moment: true` gets one `hit` at its line's last word
(a shot's `key_sfx` names another kind, or `false` drops it) unless a cue
already sits within 0.3 s, and `mvf score` ducks the bed a further 6 dB over
the hold after it ([07-craft](07-craft.md#key-moments)).

## Checks (printed by `mvf scaffold`)
- **FAIL (the file cannot be built):**
  - D0: the beat exists.
  - D1: a directed beat has shots.
  - D2: the transition exists.
  - D3: the anchor resolves.
  - D4: shots are in order (one at or before the shot before it).
  - D5: the camera exists.
  - D6: punch-ins stay in their shot.
  - D7: the media exists.
  - D8: the effect exists and has what it needs (a region past the frame edge
    WARNs and is clamped).
  - D9: text stays out of the caption band, unless `allow_caption_zone` is set.
  - D12: a layer's art exists, and its rig, body, hair, parts, poses, faces and
    emotes exist. Its enter, idle, moves, hits, swaps and tweens exist and stay
    in their shot.
- **Advice (WARN):**
  - V1 and V2: transition variety.
  - V3: something changes every 3.5 s, unless the hold is the point.
  - V5: two adjacent shots make the same camera move (a steady `drift`,
    `hold` or `custom` is fine).
  - V6: over five or more shots, the shot lengths are too even (spread under
    30% of the mean): the cut ticks like a metronome.
  - DV1: a script beat whose line carries a number or a comparison shows no
    device effect (`counter`, `bars`, `compare`, `steps`, `dots`, `ladder`,
    `html`, `callout`, `pointer`); the script's `device: none` answers it.
  - D4 flash cuts (≤ 0.2 s), D12 opaque art without a blend, and z below -1.
  - D10: a storyline is named.
  - D11: the hook's line is on screen at frame 0, and a captionless beat still
    shows its line.
  - D13: a layer shown taller than its art was approved for.
  - D14: a character-sized layer's top quarter (its face) under the captions;
    a beat's `captions_y` (a frame share) moves that beat's captions.
  - D15: a rig with a `cast` that is not in `art.json`, or that has no drawn
    parts.
  - Text outside the platform safe area.

The checks catch a static or broken edit, not a weak one. Look at the draft
([07-review](07-review.md)).
