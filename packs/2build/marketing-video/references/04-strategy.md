# Stage 4 - Strategy (angles)

Output: `04-strategy/angles.json`. An angle = **segment x insight x promise x
format x share reason**, standing on at least one fact and using the trends
that fit.

```json
{ "angles": [{ "id": "G1", "segment": "A1", "insight": "the true, slightly funny or painful thing the segment already knows",
  "promise": "what the viewer gets, provable with facts", "why_watch": "why a stranger keeps watching past 3 s",
  "why_share": "who the viewer sends it to and the line they send it with",
  "format": "problem-solution", "facts": ["F1"], "trends": ["T1"], "platforms": ["tiktok", "x"],
  "hooks": ["three openers, each a different mechanism (table below)"] }],
  "plan": [{"script": "s01-name", "angle": "G1", "why": "what this video tests"}] }
```
- `why_watch` answering "to learn about the product" means it is an ad: rethink.
- `why_share` is the organic reach. A video nobody forwards stops at the views
  the first push gives it. It names one trigger: **identity** ("this is so
  you", said to one friend), **utility** (saved or sent for later), **debate**
  (people comment to disagree or rank), **emotion** (a laugh or a gasp they want
  to pass on). "It is a good product" is not a reason to share.
- At least 2 different formats across angles: test formats, not synonyms.
  Formats: problem-solution, demo, pov, before-after, story, listicle,
  myth-buster, reaction, tutorial, testimonial, behind-the-scenes, comparison,
  launch, meme, skit.
- One script per angle. Hook variants (H2, H3) replace only beat B01, so the
  body is shared and the test isolates the hook.

## Hooks
A viewer decides in about 1 s from the picture, then by 3 s from the promise.
A hook therefore works in three layers at once: **frame 0** (the most
interesting picture in the video, already moving: the result, the surprise, a
face reacting, the absurd number on a real screen), the **on-screen line** (the
specific promise or tension, readable muted) and the **first spoken words**
(content from the first word, no greeting). Each layer should hook on its own.

| Mechanism | Shape | Note |
| --- | --- | --- |
| Result first | show the end state, then how | the payoff becomes a reveal of the steps |
| Call-out | "If you <specific situation>…" | names the segment in its own words |
| Contradiction | "Stop doing X" / "X is wrong" | must be true and proved in the body |
| Stake | what the viewer is losing now | concrete: time, money, the chapter |
| POV / scene | "POV: 3 am, chapter 1200" | the situation the segment lives |
| Countdown | "3 <things> that <result>" | the best item last; it holds them |
| Curiosity gap | open a specific question | the payoff answers exactly it |

Specific beats generic: a number, a name or a scene the segment recognises. A
hook promises only what the body delivers; clickbait buys the 3 s and loses
the watch-through, the share and the comments.

## Brand and CTA by goal
The product is the visible tool of the value (on screen where the value
happens), not the topic of the video. Brand recall comes from seeing it work
and from the account itself: a recurring format and a series name earn the
follow, and the follow is the reach the next video starts with.

| Goal | The one CTA | Where |
| --- | --- | --- |
| awareness / follow | follow for part 2, or save / send it | spoken over the payoff |
| signups / activation | search the product's name, or link in bio | last beat, name + handle on screen |

| | TikTok | X |
| --- | --- | --- |
| Hook | <= 2.5 s, spoken and on screen | <= 2.0 s, on screen (autoplays muted) |
| First value | <= 6 s | <= 6 s |
| Length | 15-60 s (sweet spot 20-40) | 10-45 s |
| Copy | caption with search terms first, 3-5 hashtags | post <= 280 weighted chars, 0-2 hashtags; CTA/link in a reply |
| Sound | trend or library sound added in-app, or a bed | bed only; captions carry it |
| Proof | the real screen | the real screen, readable muted |

Gate (`mvf check strategy`): 2-3 angles, each with all fields (`why_share`
included), a fact id that exists, a trend id that exists (or none), at least 2
formats overall, a `plan` entry per script.
