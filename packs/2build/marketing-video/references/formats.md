# Formats and channels

Read this when picking a script's `format` (stage 5) and its canvases (stage 7).
The atlas names, for each kind of video, the value a viewer gets and what
usually breaks it. The last column lists mvf shortcuts that exist for that
kind of picture; it is an index, not a recommendation, and it says nothing
about how the video should look. A `story` drawn in code, a `listicle` on
footage or a `demo` with no `callout` is as valid as the row's shortcuts
(SKILL.md: every shortcut has a raw form). It never prescribes a structure:
the standards in SKILL.md hold for every row, and the shape is yours. A format
not listed here is fine; name it as what it is, and lint does not ask you to
rename it.

## The atlas
| `format` | The viewer gets | What breaks it | Shortcuts that exist |
| --- | --- | --- | --- |
| `demo` | the result they want, seen working on the real product | a feature tour; UI before the problem; a drawn screen | real `image`/`video` shots, `callout` and `tap` on the screen, `counter` for a measured number |
| `tutorial` | a task they can do after one viewing | a missing step; steps faster than a viewer can follow; "it's easy" instead of the step | `steps` (one item per spoken step, `ats` on its word), `callout`, `lower` for the tool's name |
| `problem-solution` | their own problem named, then a fix they can try | a problem too general to recognise; a fix that is only the product's name | `compare` (before/after), `slam` on the problem's word |
| `before-after` | the change, seen side by side | an "after" that is not measured or not real | `compare`, `bars`, a `counter` |
| `comparison` | a pick: which one, for whom, why | a draw with no verdict ("both are good"); unsourced numbers | `compare`, `bars` (every value names a fact) |
| `listicle` | each item worth keeping on its own | ten thin items; the best item last when most viewers leave by half-way | `steps` as a ranked list, a `counter`; put the strongest item early |
| `myth-buster` | the wrong belief, the right one, and why | the myth repeated louder than the correction; the correction without its reason | `stamp`/`label` ("MYTH", "FACT"), `quote` for the source |
| `explainer` | an answer to one question, in plain words | the tease; terms kept unexplained; the question asked back as the ending | `title`, `steps`, `compare`, `quote` for the cited line |
| `story` | a feeling they can name, or a lesson they can use | the world before a person and what they want; no turn | art plates, `cam` moves, `weather`, `quote` for the line that lands |
| `testimonial` | someone like them, and what changed for them | a paid-sounding script; no specific detail | `quote` with the real words, `lower` for the person, their real photo cut out (an art job with `image`) |
| `pov` / `skit` | a situation they know, made funny or sharp | an inside joke only fans get; a product plug as the punchline | fast cuts, `slam`, `punch`, a voice per character (beat `voice`) |
| `reaction` | the reaction plus the context to share it | the clip without the context; rights the clip does not have | `official-promo` or own clips only; `lower`, `quote` |
| `behind-the-scenes` | how something real gets made | polish that hides the process | real footage, `lower`, `counter` |
| `launch` | what is new and who it is for, in one line | a list of features; hype words | real screens, `callout`, one `endtag` |
| `challenge` / `meme` | a format they can join | a trend that has gone stale (re-check stage 3) | trend sound added in the app, never baked in |

`recipe` is separate from `format`. It names the material a script stands on
(`mvf material`, `tools/mvflib/recipes.json`), so a `story` can stand on a
`researched-story` and a `demo` on a `product-demo`.

## Explaining on screen
The direction file's effects carry an explanation when the picture alone
cannot (`07-direction.md` has the keys):
- `callout`: a box with an optional spotlight on one region of a real screen,
  with a label. `tap`: a ring where a finger would press.
- `steps`: numbered items that appear on their spoken words, and `compare`:
  two columns.
- `bars`: a measured comparison in which every value names a fact. `quote`: one
  cited line, word by word. `lower`: a name and a role.

A rect or position can be given per canvas (`{"9x16": [...], "16x9": [...]}`),
so one direction file serves both canvases. Text stays out of the caption band
(D9), and a box under the captions WARNs.

## Channels
One script feeds several platforms, and mvf builds one project per canvas.
The specs and copy limits live in `tools/mvflib/platforms.json`, which
[08](08-delivery.md) renders as a table. What changes the *script*:

| | Sound | Canvas | What it means for the script |
| --- | --- | --- | --- |
| TikTok, YouTube Shorts, Instagram Reels, Facebook Reels | on | 9x16 | the spoken hook carries it; captions back it up |
| X, LinkedIn | off (autoplay muted) | 9x16 | on-screen text tells the video muted; frame 0 is a statement |
| YouTube long-form | on | 16x9 | a longer arc: the first value by about 15 s, chapters, a custom thumbnail |

- Pick channels from where the segment watches (stage 2), not from all of them.
  Two channels done well beat five uploads of one file.
- A short and a long-form version of one idea are two scripts that share
  research. The short is not a trimmed long-form.
- `platforms.<p>` in the script carries that channel's copy, and lint checks
  each one against its limits.
