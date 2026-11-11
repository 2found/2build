# Stage 7d - Review

Output: `07-video/<sid>/<canvas>/review.json`, written after you have looked at
the draft. The checks catch a broken file. They cannot see a dull shot, a
caption over a face or a plate that melts at full size.

## Bar
- Look at the draft's frames before a delivery export. Every place that could
  break needs a still: frame 0, the hook's end, half-way, the CTA, the last
  frame, and each moment an effect or punch-in lands.
- Record what you saw, fix it, re-render, and mark it fixed.
- A delivery export refuses while a P0 is `open`. It WARNs (RV1) when there is
  no `review.json`. Use `accepted` only for a P1 or P2 kept on purpose, with the
  reason in `fix`.
- Audio stays `PENDING LISTEN` after this review: the numbers stand in for ears
  until a person listens.
- **Judge the cut with strangers** before delivery: `mvf judge video <proj>`
  ([Micro Fish](06-pretest.md#micro-fish)). Your eyes
  find broken frames; the panel finds where people leave and whether they can
  still retell it once the picture, pace and captions are real. Each expert
  P0 goes into `review.json` as an issue. Export WARNs (JV1) when a delivered
  cut was never judged.
- **Judge once, fix once, ship.** After the panel has judged a cut, fix what it
  found (or accept it in `review.json` with a reason), check the fix with your
  own eyes and `mvf score`, and deliver. Do not run `mvf judge video` again on
  the fixed cut: a second round costs the owner another ~10 minutes and tokens
  for a verdict they did not ask for. Re-judge only when the owner asks, or
  when the fix changed what the video says end to end (a new script, not a
  corrected line).

- **Look at motion on a strip, not a still.** A still shows a frame; motion is
  in the frames between. For each shot that moves, take a strip of 6–8 frames
  over the move (`ffmpeg -ss <t> -t <dur> -vf fps=8,scale=180:-2,tile=8x1`) and
  ask: does one thing lead and the rest follow? Does the cause come before its
  effect? Does a move lean back before it goes and settle after it lands, and
  does it hold long enough to be read? Does an exit take less time than its
  entrance? Is a third of the frame or less moving at once? Do the eyes look
  where the next thing happens? ([07-craft](07-craft.md#motion))
- **Could this frame belong to another video?** Name any frame that a stock
  look would fill as well: see the cliché list in
  [07-craft](07-craft.md#clichés). Replace it with something only this
  video has (its character, its object, its number).
- **Score it** in `review.json` `score` (1–10), on this scale: 5 a student's
  cut, 6 a template, 7 a working pro, 8 an agency, 9 an award entry. Say what
  keeps it from the next step. Export WARNs without a score (RV2).

## Cold viewer
You wrote the plan, so you see the plan in the frames. A fresh helper who never
read it sees the video. `mvf viewer <proj>` writes `review/viewer-brief-vN.md`:
1 fps frame sheets (each under 1 MP, the second stamped on each frame), the
voice as heard with its times, the logline and what the video is for, and
nothing else: no script, plan or direction path. Give it to a fresh helper with
no other context. It retells the video, names where it would swipe, checks each
frame against the voice and for breaks and stock looks, and writes
`review/viewer-vN.md` ending in three lines:
```
Verdict: APPROVE | FIX | UNVERIFIABLE
Red items: 3s caption over the face; 6.5s frozen hand   (or none)
Retell: one or two sentences
```
Each red item goes into `review.json` as an issue, fixed or accepted with a
reason. A delivery export WARNs when no viewer ran, the latest says
UNVERIFIABLE (VW1), or red items have no issue near their second (VW2). One
viewer per cut; after the fixes, check them yourself and ship, as with the
panel.

| Sev | Means | Examples |
| --- | --- | --- |
| P0 | a viewer would notice and leave, or the post is wrong | a black or frozen frame, a caption over a face or the subject, stray text or a mangled hand in art, an uncited claim on screen, a wrong name or number, a draft plate |
| P1 | it costs attention | a soft or stretched plate, a shot that does not show what the line names, a dead stretch, an effect louder than a word |
| P2 | polish | a transition that fights the beat, a colour off the look |

```json
{"reviewed_at": "YYYY-MM-DD", "looked_at": ["contact.jpg", "spot-0.jpg", "12.4s still"], "score": 7,
 "seen": [{"at": 0.0, "what": "a hand sets a phone on the desk; '3 HOURS' over the screen"}],
 "issues": [{"sev": "P0", "at": 3.1, "what": "caption covers the product screen", "status": "open|fixed|accepted",
             "fix": "captions y up 80 px on B02"}]}
```
`seen` records what the frames actually show, which can differ from the
script's `visual`. `mvf judge video` adds each note to its beat's picture, so
the panel judges the cut rather than the plan.

## Tool
`mvf export <proj> --draft` renders a rough cut and writes `contact.jpg` and
`spot-*.jpg` in `08-delivery/`. For other moments, render stills with
`npx hyperframes snapshot` or `ffmpeg -ss`, or take frames from your own editor.
