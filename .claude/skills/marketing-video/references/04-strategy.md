# Stage 4 - Strategy (angles)

Output: `04-strategy/angles.json`. An angle = **segment x insight x promise x
format**, standing on at least one fact and using the trends that fit.

```json
{ "angles": [{ "id": "G1", "segment": "A1", "insight": "the true, slightly funny or painful thing the segment already knows",
  "promise": "what the viewer gets, provable with facts", "why_watch": "why a stranger keeps watching past 3 s",
  "format": "problem-solution", "facts": ["F1"], "trends": ["T1"], "platforms": ["tiktok", "x"],
  "hooks": ["three openers, each a different mechanism: result first, stake, contradiction, POV"] }],
  "plan": [{"script": "s01-name", "angle": "G1", "why": "what this video tests"}] }
```
- `why_watch` answering "to learn about the product" means it is an ad: rethink.
- At least 2 different formats across angles: test formats, not synonyms.
  Formats: problem-solution, demo, pov, before-after, story, listicle,
  myth-buster, reaction, tutorial, testimonial, behind-the-scenes, comparison,
  launch, meme, skit.
- One script per angle. Hook variants (H2, H3) replace only beat B01, so the
  body is shared and the test isolates the hook.

| | TikTok | X |
| --- | --- | --- |
| Hook | <= 2.5 s, spoken and on screen | <= 2.0 s, on screen (autoplays muted) |
| Length | 15-60 s (sweet spot 20-40) | 10-45 s |
| Copy | caption with search terms first, 3-5 hashtags | post <= 280 weighted chars, 0-2 hashtags; CTA/link in a reply |
| Sound | trend or library sound added in-app, or a bed | bed only; captions carry it |
| Proof | the real screen | the real screen, readable muted |

Gate: 2-3 angles, each with all fields, a fact id that exists, a trend id that
exists (or none), at least 2 formats overall, a `plan` entry per script.
