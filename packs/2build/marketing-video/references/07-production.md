# Stage 7 - Production

Output: one project per canvas x hook under `07-video/<sid>/<canvas>-<hook>/` (or one edit file
for a screen-recording workflow). Use what the user has; do not install
anything without asking.

## Route
- **HyperFrames** (HTML to video, `npx hyperframes`, Node 22+) is the tooled
  route: `mvf scaffold <sid> [--hook all]` writes one project per canvas x hook
  on the take's timing (index.html, captions, STORYBOARD.md, SCENE_GUIDE.md,
  placeholder `compositions/<beat>.html`). You author the beat scenes; re-run
  scaffold after any re-voice and it moves every slot while keeping your scenes.
  Read the `hyperframes` skill's entrypoint first if it is installed.
- **Otherwise** screen recording plus any editor the user has (CapCut, iMovie,
  DaVinci, ffmpeg). Write a `STORYBOARD.md`: beat id, start and end seconds,
  picture, on-screen text, voice line.

## 1. Voice
`mvf voice <sid>` makes a take (`07-video/<sid>/takes/tNN/`): one WAV per line
plus `manifest.json` with speech spans and word times. Only lines that changed
since the last take are re-voiced. The engine comes from `--engine`, then the
script's `voice.engine`, then campaign `voice_engine`, then
`.babysit/marketing-video/config.json`. A script in `recorded` mode uses `files`.

- **`files`** (built in): the user's own recordings, the best choice with
  nothing to clear. Running it without recordings lists the lines to record
  into `07-video/<sid>/recordings/<cue>.wav|mp3|m4a`. Speech spans come from
  silence detection; word times are spread by length.
- **`omnivoice`** (ships with the skill, `tools/extensions/omnivoice/`): local
  TTS that clones `voice.ref` or designs a voice (`mvf voice-design`, then
  `mvf voice-pick`). Whisper hears every line back. No voices ship with it, and
  it needs `uv` or `$MVF_OMNIVOICE_PYTHON` (see its README). `--rehear --engine
  omnivoice` Whisper-aligns any take, recorded ones included.
- **Any other engine** (a cloud TTS, a local model) is a voice extension (see below).

References live in `<campaign>/voices/` or `.babysit/marketing-video/voices/`
(`<name>.wav` plus `.txt` of its words). Both are gitignored. Clone only a
voice whose owner consented. For a cloud voice the service's licence must
permit commercial use.

Brand names and numbers: write how they are said in `brand.say` (the engine
speaks it, captions keep the display spelling). Keep digits in captions and
write lists as "15, 30, 45 minutes". Listen to every line. A dropped or mangled
word means rewriting the line or fixing its `say`.

## 2. Timing from the audio
Measure, do not estimate. Scaffold and lint read the latest take whose lines
match the script. Otherwise they fall back to an estimate and say so.
Delivery needs `measured`. Re-run `mvf lint` on the real hook length. Off the
tooled route, get word times from any aligner and durations from `ffprobe`,
and mark the timing source in the storyboard.

## 3. Scenes
- Real screens cropped and upscaled for push-in; logo from the product's brand
  folder. Pictures from `00-material` with showable rights first, credit on
  screen while shown. Never draw a UI or a real person.
- Frame 0 of B01: hook text plus the subject, never black.
- Captions: word-by-word, 1-3 words per group, keys highlighted, never over the
  subject, inside the platform safe zone (TikTok: keep text roughly between 168
  and 1330 px of a 1920 px canvas and 140 px clear of the right rail; X: top 120
  px clear).
- Effects resolve on the word: a glitch or typing effect runs before its word and
  lands as it is spoken. Clamp events to the beat's duration; re-check after the
  voice changes.
- Check at each slot's first frame, each landing word and the last frame (render
  stills with `npx hyperframes snapshot` or ffmpeg) and look at the images.

## 4. Sound bed
Optional low bed the user may use (own or royalty-free, licence noted):
`mvf score <proj> --bed FILE` loops or trims it to the runtime, sets it 7 dB
under the measured voice and ducks it 15 dB under speech. It writes
`assets/music/bed.wav` and `score.json`, then re-scaffolds; `--none` removes it.
Cut SFX are optional: a full-length `assets/music/sfx.wav` you supply is picked
up on re-scaffold. X plays muted first, so captions carry it. Never bake in a
licensed track.

## Extensions (gitignored, per project or per campaign)
`mvf extensions init` creates `.babysit/marketing-video/` (`extensions/`,
`voices/`, `config.json`, all ignored). A campaign's own `extensions/` is
ignored by its `.gitignore`. An extension is `<name>.py`, or
`<name>/extension.py` when it has helper files. It exports any of:

- `SOURCES = {"ns.name": Source(about, kinds, fetch, params, needs)}`: `fetch(params)`
  returns material Items (stage 0.5) and only reads.
- `VOICES = {"name": Voice(about, modes, command=[…] | run=fn, opts, needs)}`. A
  command is run as `[*command, take|design|rehear, --job job.json, --out DIR,
  --opt-name value…]`. The job is `{language, say, voice: {mode, ref_wav?,
  ref_txt?, name?, speed}, cues: [{id, text, voice?}], workdir}`. The engine
  writes `<cue>.wav` and `manifest.json` `{cues: [{id, text, path, duration_s,
  speech: [s, e], words: [{w, s, e}], asr: {heard, dropped, added}}]}`.
- `KINDS` (extra material kinds) and `RECIPES` (same shape as `recipes.json`).

Search order (the first definition wins): campaign, `.babysit/marketing-video`,
`$MVF_EXTENSIONS`, then the skill's own. `mvf extensions` lists what loaded and
from where. Secrets come from environment variables named in `needs`, never
from files. Set `$MVF_HEAVY` (or config `"heavy"`) to a queue or lock command
to run model and render jobs one at a time on a shared machine.

Gate: no placeholders left (or each is a labelled, reported placeholder), timing
`measured`, voice-to-bed gap measured, stills reviewed for caption collisions
and mid-effect covers.
