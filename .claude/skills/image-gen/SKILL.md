---
name: image-gen
description: Generate raster images locally with Qwen Image 2.1 Turbo (Metal, no account, no network at generation time) and cut subjects out of their background with RMBG-1.4. Use for draft plates, backgrounds, covers, stickers and reference-guided edits; not pixel-exact edits, UI mock-ups or likenesses of real people.
disable-model-invocation: true
---

# Local image generation and cutouts

Qwen Image 2.1 Turbo Q4_K_M (CFG 1, a fixed 8-step schedule) through a pinned
`sd-cli` build on a Mac with Apple silicon, plus Bria RMBG-1.4 (ONNX, CPU,
~1 s) for cutouts. Weights come from HuggingFace at the revisions, sizes and
SHA-256 pinned in `scripts/models.json`. No account, no paid API, no network
when drawing. `marketing-video` draws its art through this skill (`mvf art`);
call it directly for a one-off image.

**Heavy job.** One generation peaks near 3.5 GiB at any size (weights are read
from disk a segment at a time) and takes about 7 min a megapixel on an M1 Pro
16 GB. Start it only when the machine is idle (`uptime` load well
below the core count, `memory_pressure` free >= ~40%). A non-blocking lock
refuses a second concurrent run; on a shared machine queue it behind the same
lock as renders and TTS (`MVF_HEAVY` in marketing-video).

## Host preparation (once)

`<skill-dir>/scripts/setup [--with-vision] [--no-cutout] [--check]` builds
`sd-cli` at the pinned stable-diffusion.cpp commit, creates a hash-locked venv
(numpy, onnxruntime, Pillow) and downloads ~9.9 GB of Qwen weights (+1.2 GB
vision projector with `--with-vision`, needed for `--ref`; +176 MB RMBG-1.4
unless `--no-cutout`), all under `$IMAGE_GEN_HOME` (default
`~/.local/share/image-gen`). Needs `git`, `uv` and the Xcode command-line
tools. It is a download and a native build: run it only when the user has
authorised it and the machine is idle. `--check` verifies offline.

## Generate

```sh
<skill-dir>/scripts/generate-image "<prompt>" -o <new.png|.webp|.jpg> \
  --size <WxH> [--seed N] [--ref <reference.png>] [--transparent]
```

Sizes are multiples of 32, each side 256 to 1920, within the machine's pixel
budget. `generate-image --limits` prints it as JSON: 1088x1920 (a whole
vertical frame) with 16 GB of memory or more, 512x512 below that, where nothing
larger was measured. Measured on an M1 Pro 16 GB, 9:16:

| Size | Time | Shown full-frame at 1080x1920 |
| --- | --- | --- |
| 384x672 | 2 min | 2.8x enlarged: soft, fine under text or as a small inset |
| 576x1024 | 4.5 min | 1.9x: soft |
| 768x1344 | 7.5 min | 1.4x: reads sharp |
| 1088x1920 | 15.5 min | as drawn |

You choose the size: draw at the size the picture is shown, smaller where
softness is fine (a background under text, dimmed or blurred) and when time
matters more. Other ratios take the same pixel counts (1024x1024, 1344x768, …).

- The output must be a new path; nothing is overwritten.
- `--ref`: PNG/JPEG <= 2 MiB; name its role (subject, style, composition) in
  the prompt. Needs `setup --with-vision`. A character sheet as `--ref` keeps a
  face and costume across poses.
- `--transparent` gives a cutout. The model is asked for alpha; when under 5%
  of its render is clear (often, e.g. on a coloured background or with
  `--ref`), RMBG-1.4 cuts it out. The receipt's `cutout` is `model-alpha`,
  `rmbg-1.4` or `none` (RMBG not installed). Prompt a plain flat background in
  a neutral colour the subject lacks; check the edge on dark and on white.
- Prompts <= 4000 chars; no negative prompts, sampler changes or batches.
- Text in the image is unreliable (diacritics most of all): put
  titles and captions in the layout layer (HTML/CSS) and prompt "no text".
- Never prompt with a real person's name or ask for a real person's likeness,
  and never draw a product UI; use the real screen.

## Cut out an existing image

```sh
<skill-dir>/scripts/remove-bg <input> -o <new.png> [--mask <new-matte.png>]
```

The receipt reports `transparent_pct` and `opaque_pct`; near-zero opaque means
the subject was lost. Flat line art whose inside matches the background loses
its enclosed areas: draw it with `--transparent` on a contrasting colour instead.

## Licences

Qwen-Image weights: Apache-2.0. RMBG-1.4: bria-rmbg-1.4, source-available and
non-commercial; commercial use of its cutouts needs a Bria agreement. Use
RMBG cutouts in marketing only when the operator holds that agreement; without
it, set up with `--no-cutout`.

Inspect every saved image yourself (anatomy, identity, text, artifacts, edges):
the receipt proves the file, not its quality. One generation plus at most one
targeted correction per asset; no speculative batches. On timeout, OOM or
"busy", stop and report; do not raise budgets or retry in a loop.

Return the saved path, size, seed and the prompt used (the JSON receipt).
