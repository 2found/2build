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

| Sev | Means | Examples |
| --- | --- | --- |
| P0 | a viewer would notice and leave, or the post is wrong | a black or frozen frame, a caption over a face or the subject, stray text or a mangled hand in art, an uncited claim on screen, a wrong name or number, a draft plate |
| P1 | it costs attention | a soft or stretched plate, a shot that does not show what the line names, a dead stretch, an effect louder than a word |
| P2 | polish | a transition that fights the beat, a colour off the look |

```json
{"reviewed_at": "YYYY-MM-DD", "looked_at": ["contact.jpg", "spot-0.jpg", "12.4s still"],
 "seen": [{"at": 0.0, "what": "the potter's hands lift a white paperweight; 'đập vỡ vận mệnh con' over his chest"}],
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
