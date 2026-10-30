# Stage 7 - Production

Output: one project per hook under `07-video/<sid>/<hook>/` (or one edit file
for a screen-recording workflow). Use what the user has; do not install
anything without asking.

## Route
- **HyperFrames** (HTML to video) if available (`npx hyperframes`) and a
  `hyperframes` skill is installed: read its entrypoint first. Scenes are pure
  functions of time; one file per beat; frame 0 of B01 is the cover.
- **Otherwise** screen recording plus any editor the user has (CapCut, iMovie,
  DaVinci, ffmpeg). Write a `STORYBOARD.md`: beat id, start and end seconds,
  picture, on-screen text, voice line.

## 1. Voice
The user's own recording (best, nothing to clear), or a TTS voice the user has
rights to (the service's licence permits commercial use; never clone a real
person's voice without their consent). Record or synthesise per beat so timing
is per scene. Note the voice source and rights in the script `voice.ref`.

Brand names and numbers: spell phonetically in the TTS input (`brand.say`),
keep digits in captions, write lists as "15, 30, 45 minutes". Listen to every
line; a dropped or mangled word means rewrite the line or fix its spelling.

## 2. Timing from the audio
Measure, do not estimate: get word times from the audio (Whisper or any aligner;
`ffprobe` for durations). Scene slots are the measured beat spans. Mark the
timing source (`measured` or `estimated`) in the storyboard; delivery needs
`measured`. Re-run the script checks on the real hook length.

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
Optional low bed that you may use (own, generated or royalty-free with licence
noted), ducked under speech: voice at least 8 dB above the bed (11 is better),
measured. X plays muted first, so captions carry it. No licensed track baked in.

Gate: no placeholders left (or each is a labelled, reported placeholder), timing
`measured`, voice-to-bed gap measured, stills reviewed for caption collisions
and mid-effect covers.
