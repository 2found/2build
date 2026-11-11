# Stage 7 - Production

Output: one video, made from the pre-test's winning script on its winning hook,
as one project per canvas in `07-video/<sid>/<canvas>/` (or the edit file of
whatever editor you used). The other candidates and hooks are never voiced,
drawn or rendered.

How the video is built (the look, the shots, the motion, the edit) is your
call, made for this product, audience and platform. This page sets the bar and
describes the helpers.

## Bar
- **Real product.** Any UI on screen is a captured screenshot or recording,
  never drawn or generated. A missing screen is a labelled placeholder and a
  line in the report.
- **Rights and credits.** Show only material with showable rights (`own`,
  `licensed`, `public-domain`, `official-promo`), with the credit on screen
  while it shows. Clone only a voice whose owner consented. A cloud voice or
  image engine must allow commercial use.
- **Generated art** fills what a camera or the product cannot give. It is never
  the product UI, a logo, readable words or a real person.
  - Only approved art is shown.
  - Art is gated by its upscale (the size shown over the size drawn). A
    full-frame plate or thumbnail passes at 1.5x and fails past 2x. A
    background under text passes at 3x. An inset or sticker is measured at its
    `display` size. A reference is never shown.
  - Never prompt with a real person's name, and never produce their likeness.
- **Draw who the audience already pictures** (see Cast below).
- **Measured timing.** Slot lengths, word times and caption times come from
  the recorded or synthesised audio, not from estimates. A delivery needs
  measured timing.
- **Sound.**
  - Never bake a licensed or trend track into the file. Trend sounds are added
    in the app, and a bed is one the user owns or licensed, or a generated one.
  - The voice stays clear over the bed and the effects. The mix check targets
    8 dB of margin inside every spoken span, and its status stays
    `PENDING LISTEN` until a person has heard it.
- **Looked at before delivery.** Review the draft by looking at its frames
  ([07-review](07-review.md)). The checks cannot see a dull shot or a caption
  over a face.
- **One video at a time.** Heavy jobs (models, renders) run one at a time on a
  shared machine.
- **No spending without a yes.** An engine that costs money runs only with
  `--spend`, and you pass that flag only after the user agreed to that spend.

What the checks refuse:
- `mvf art check` FAILs:
  - a duplicate or malformed job id, an unknown `use`, `look` or `canvas`;
  - an inset or sticker without `display`;
  - an empty prompt;
  - a prompt that asks for a product UI;
  - a prompt that names a real person from the material pack;
  - a `ref` that is neither a job nor a campaign file;
  - the cast errors below.

  It WARNs on text or a logo in the picture, and on a sticker that is not
  transparent.
- `mvf art approve` refuses an upscale past the fail line unless you looked at
  it and pass `--force`.
- `mvf art draw` refuses a paid engine without `--spend`, a missing env
  secret, or a transparency or reference request the engine cannot do.
- A delivery `mvf export` refuses while:
  - a placeholder scene remains;
  - timing is estimated;
  - the script changed after scaffold;
  - the script shows unapproved art;
  - `review.json` has an open P0.

  `mvf qa` and the thumbnail gates are in [08](08-delivery.md).

## Direction
- The picture tells the beat with the sound off. X autoplays muted, and many
  viewers on every platform watch that way.
- Something visibly changes every few seconds. A held frame is where thumbs
  move.
- Show what the line names, when it names it. An edit feels intentional when
  the cut, reveal or punch-in lands on the word.
- Frame 0 is the cover. It shows the subject and the hook line, never black, a
  logo or a title card.
- Keep faces, the subject and on-screen text out of the caption band and the
  platform UI zones.
- Sound effects serve a moment and sit off the spoken words. The bed sets the
  mood under the voice and never competes with it.
- Match the look to the product and audience; brand colours and fonts are a
  default, not a cage.
- Motion, devices, cut rhythm, key moments and the cliché test are in
  [07-craft](07-craft.md).

## Art
`07-video/art/art.json` is `{preset?, style, cutout_style?, engine, cast, jobs: [{id, use, prompt |
image, canvas?, display?, ref?, cast?, transparent?, look?, style?}]}`.
- `style` goes on every job; `cutout_style` replaces it on a sticker or a
  `transparent` job (a subject alone on a flat ground, no scenery); a job's own
  `style` beats both.
- `preset` (`stickman`, `ink`, `anime`, from `tools/mvflib/styles.json`) fills
  `style` and `cutout_style` when the file leaves them empty. Without one, it
  takes the campaign's `style` (campaign.json, `stickman` for a new campaign),
  and `"none"` opts out. `stickman` and `ink` draw stickers as line art on white,
  shown with `blend: multiply` (07 Direction), so they do not ask for
  `transparent`. An unknown preset FAILs (A0). An unknown `look` WARNs and is
  passed through.
- `use` is `plate`, `background`, `inset`, `sticker`, `thumbnail` or
  `reference`. `look` is `illustration` (the default), `texture` or
  `photoreal`; photoreal needs the AI label.
- `mvf art check` validates the file.
- `mvf art draw [ID…] [--seed N] [--spend]` draws references first. Drafts go
  to `draft/<ID>-s<seed>.png` with a receipt and are never overwritten.
- `mvf art approve ID [--draft P] [--focus x,y] [--upscaler
  esrgan|blend|lanczos] [--force]` fits a draft to its display size. For the
  gate, `esrgan` counts as 2x, `blend` as 1.6x and `lanczos` as 1x.
- `transparent: true` asks for a cut-out. An opaque result is cleared from the
  flat border background at approve, and approve refuses one less than 5%
  clear.
- `image: "<path in the campaign>"` in place of `prompt` starts from a picture
  the campaign already has, such as the founder, a product photo or a real
  cover. With `transparent: true`, `draw` cuts it out with RMBG-1.4 (image-gen
  `remove-bg`, about 2 s on CPU); otherwise it takes the picture as it is.
  Approve fits it like any draft. Nothing is generated, so it never counts
  toward the AI label.
- **RMBG-1.4 is licensed for non-commercial use.** A product video needs a Bria
  agreement. The owner records it once in config as `rmbg_license:
  bria-agreement` (or `non-commercial`). Export WARNs (LIC1) when shown art was
  cut by RMBG and no agreement is recorded. Qwen's own alpha (`cutout:
  model-alpha` in the receipt) is not affected.
- The `qwen` engine is the local `image-gen` skill (Qwen Image 2.1 Turbo).
  `$MVF_IMAGE_GEN` points at another generator, and any other engine is an
  `IMAGES` extension.

### Layered art (2.5D)
A layered shot needs its parts drawn apart, in the same `style` and light:
- the plate (`use: plate`) with the place and nobody in it, the prompt naming
  the empty spot where the character will stand;
- each character or prop as a `sticker` with `transparent: true` and the
  `display` it is shown at, on a flat background far from its colours, in the
  pose and camera angle the plate implies, with the `cast` it shows. Give it
  `cutout_style` (or its own `style`): the shared one describes a whole
  picture, so the model paints scenery and the cut leaves its colour on the
  edges (A9 WARNs).
- a character that changes pose or expression mid-shot as one sticker per
  drawing (same `cast`, `display` and framing, the feet at the same spot),
  shown through the layer's `swaps`.
- a small prop with a large `display` (a pebble at 400x400, shown at about a
  fifth of the frame): the cut-out keeps its margin, so a tight display makes
  it a dot.

Draw the plate first and look at its horizon and light before prompting the
cut-outs. A cut-out lit from the other side, or seen from another height,
reads as pasted on.

Per style. A preset is a starting point, not a requirement, and no check
asks for one:
- **Stickman comic** (`preset: stickman`, the default). This is the look of
  Vietnamese webcomic explainers:
  - a bold, even black outline on everything;
  - big chibi heads with an angular jaw and C ears;
  - a strong-coloured hair silhouette;
  - flat T-shirts, single-line arms with round hands;
  - faces that act: thick brows, dot eyes, red open mouths, blush.

  The characters are not art jobs. They are `rig` layers with `body: chibi`
  drawn in code (07 Direction), so every limb moves, the eyes blink and the
  mouth talks. Props and places are `svg` layers in the same ink and line, or
  line-art stickers multiplied onto the paper. A plate can be a plain
  `color: ground`. What makes it read as a comic:
  - **Frame close.** Bust and waist shots carry the face. A full figure is for
    a walk or a set-up.
  - **One loud trait per character.** That is the hair silhouette and its
    colour, then one costume colour or prop. Two characters in one frame never
    share a hair shape.
  - **Push the face.** Every beat has an expression, and the brows do most of
    the work. `shout`, `shocked` and `laugh` open the red mouth wide.
  - **Letter the frame.** `pointer` names a thing in uppercase with a leader
    line and a dot, `stamp` gives a sound effect, and the `comic` captions box
    each word in ink.
  - Draw set pieces once in `07-video/props.json` and place them by name
    (`prop`), so every shot of a place is the same place. A chibi head hides
    an arm raised straight up, so a reach or a hand-over goes forward and up
    (about 110°), clear of the head.
- **Ink** (`preset: ink`): the plain stick figure on warm paper. It has a round
  head and bare lines (`body: stick`), for a minimal or a calligraphic tone.
- **Anime** (`preset: anime`): painted plates and cel-shaded cut-outs. Limited
  animation sells it: hold, then a fast action on the word, `speedlines` and a
  `flash` on the hit, and a held pose `swap` instead of a tween between
  drawings.
- **Anything else.** Set `"preset": "none"` (or a campaign `style` of `none`)
  and write the look yourself (`look.colors`, `fonts`, `css`), or author the
  scenes by hand.

A rig that plays a cast character must be recognisable without its name:
someone who knows the adaptation names them from one frame. Take two or three
signature traits from the cast image and draw them as parts and colours. The
hair silhouette comes first (it reads at thumbnail size), then the costume
colours and one mark or prop. Draw the character once in `07-video/rigs.json`
with its `cast`, and compare a rendered frame with the cast image side by side
before the full render. A generic figure in the right colours is not enough
(D15).

### Cast: follow the adaptation
When a character, mascot or world already has a look its audience knows (an
animation, a comic, a game, a drama, official art), art drawn from text alone
reads as the wrong person and a cheap video. Research lists the `adaptations`
(03b). `art.json` then carries a `cast`, and every job that shows a character
lists it (`"cast": ["HERO"]`):

```json
"cast": {"HERO": {"name": "the character as the audience calls them",
                  "source": {"kind": "animation", "title": "the adaptation", "url": "https://…",
                             "image": "07-video/art/cast/hero.jpg"},
                  "design": "build, age, hair, eyes, costume, colours and props as the adaptation shows them"}}
```
- `design` is required (20+ characters) and goes into the prompt of every job
  that shows the character.
- `source.kind` is one of `animation`, `comic`, `game`, `official-art`,
  `live-action` or `none`. Every kind except `none` needs a `title` and an
  http `url`.
  - For `animation`, `comic`, `game` and `official-art`, the `image` (a
    reference still, never shown in the video) is passed to the engine as
    pixels when the job has no `ref` of its own. Without an image this WARNs,
    and an engine without reference support falls back to the design text.
  - For `live-action`, the still is an actor's face. It is never passed to a
    model (A11 FAIL), and the actor is never named. The design keeps costume,
    hair, colours and props only.
- A job naming an unknown cast id, a bad kind, a short design or a missing
  image file FAILs (A10). Research listing an adaptation while the art has no
  cast WARNs.

## Tool
The tooled route is HyperFrames (HTML to video, `npx hyperframes`, Node 22+).
Read the `hyperframes` skill first if it is installed. Use, adapt or skip any
step. Another editor (CapCut, DaVinci, ffmpeg) is fine if the bar is met and
timing is measured.

- **Voice.** `mvf voice <sid>` makes a take in `07-video/<sid>/takes/tNN/`: one
  WAV per line and `manifest.json` (speech spans and word times). It voices
  only changed lines, plus the script's `hook` variant; `--all` voices every
  line again.
  - The engine comes from `--engine`, then `voice.engine`, then the campaign's
    `voice_engine`, then `.babysit/marketing-video/config.json`.
  - `files` (built in) uses the user's recordings in
    `07-video/<sid>/recordings/<cue>.wav|mp3|m4a`. Run it without recordings to
    list the lines to record.
  - `omnivoice` (shipped extension, no voices included) clones `voice.ref` or
    designs a voice (`mvf voice-design`, `mvf voice-pick`), and Whisper times
    each word. `--rehear --engine omnivoice` re-times any take.
  - Reference voices (`<name>.wav` with a `.txt` of its words) are found in
    `<campaign>/voices/`, then the repo's `.babysit/marketing-video/voices/`,
    then the user's library `~/.babysit/marketing-video/voices/`. A voice
    found in a campaign or repo is copied into the library the first time it
    is used (and `mvf voice-pick` saves there too), so the user gives each
    voice once. A different voice under a library name stays in its campaign.
  - OmniVoice has no stock voices: `mode: preset` fails on it. Use `clone`
    with `voice.ref`.
  - Spoken spellings go in `brand.say`; captions keep the display spelling.
    Listen to every line.
- **Scaffold.** `mvf scaffold <sid> [--hook H2] [--canvas 9x16] [--from DIR]`
  writes one project per canvas on the latest matching take:
  - generated files: `index.html`, `compositions/captions.html`,
    `timeline.json`, `STORYBOARD.md`, `SCENE_GUIDE.md` and `mvf.json`;
  - authored scenes: `compositions/<beat>.html`, created as placeholders and
    never overwritten.

  Re-run it after a re-voice: every slot, clip and caption moves and your
  scenes are kept. A project switched to another hook keeps the old opening as
  `<beat>.<hook>.html`. `--from` copies body scenes from another project, and
  a copy follows its source until you edit it.
- **Direction** (optional). `07-video/<sid>/direction.json` describes shots
  and effects as data, and scaffold renders them as generated scenes
  ([07-direction](07-direction.md)). Hand-authored scenes need none of it.
- **Sound.** `mvf score <proj> [--bed FILE | --bed auto:<mood> | --none]
  [--sfx none] [--seed N]` builds the track and re-scaffolds:
  - effect cues come from the direction (campaign `sfx/<kind>.wav`, then a
    library one-shot, then a synth), dipped under speech;
  - the bed is looped to the runtime, 7 dB under the voice between lines and
    ducked under speech;
  - it writes `assets/music/mixcheck.json`, which names any beat under the
    8 dB margin.
- **Thumbnail.** `mvf thumbnail <sid> [--canvas 16x9] [--art T1] [--text LINE]` writes
  `07-video/<sid>/thumb-<canvas>/index.html` once, then snapshots it to
  `thumbnail-<canvas>.jpg`. Edit the page and re-run. YouTube long-form needs
  one; a TikTok or Shorts cover is frame 0.
- **Draft and export.** `mvf export <proj> --draft`, then `mvf export <proj>`
  and `mvf qa` ([08](08-delivery.md)).

## Extensions
`mvf extensions init` creates `.babysit/marketing-video/` (`extensions/`,
`voices/` and `config.json`, all gitignored). A campaign's own `extensions/`
is gitignored too. An extension is `<name>.py`, or `<name>/extension.py` when it
has helper files. It exports any of:

- `SOURCES = {"ns.name": Source(about, kinds, fetch, params, needs)}`. Its
  `fetch(params)` returns material Items (stage 0.5) and only reads.
- `VOICES = {"name": Voice(about, modes, command=[…] | run=fn, cwd, opts,
  needs)}`.
  - A command is run as `[*command, take|design|rehear, --job job.json,
    --out DIR, --opt-name value…]`, and `run(mode, job, out, opts)` runs in
    process.
  - The job is `{language, say, voice: {mode, ref_wav?, ref_txt?, name?,
    speed}, cues: [{id, text, voice?}], workdir}`.
  - The engine writes `<cue>.wav` and `manifest.json`, which is `{cues: [{id,
    text, path, duration_s, speech: [s, e], words: [{w, s, e}]}]}`.
- `IMAGES = {"name": Image(about, command=[…] | run=fn, max_pixels, step,
  sides, refs, transparent, paid, heavy, needs)}`.
  - A command is run as `[*command, prompt, -o out.png, --size WxH, --seed N,
    [--ref path], [--transparent]]`. It may print a JSON receipt (`seconds`,
    `cost_usd`) as its last stdout line.
  - `run(req, out)` gets `{prompt, size, seed, ref, transparent}`.
  - The draw size is the largest `step` multiple inside `sides` and
    `max_pixels` at the canvas aspect. An engine marked `paid` runs only with
    `--spend`.
- `KINDS` (extra material kinds) and `RECIPES` (the same shape as
  `recipes.json`).

Search order, where the first definition wins: the campaign, then
`.babysit/marketing-video`, then `$MVF_EXTENSIONS`, then the skill's own.
`mvf extensions` lists what loaded and from where. Secrets come from the
environment variables named in `needs`, never from files. Set `$MVF_HEAVY` (or
config `"heavy"`) to a queue or lock command so model and render jobs run one at
a time. Extensions read; they never post, log in or spend.
