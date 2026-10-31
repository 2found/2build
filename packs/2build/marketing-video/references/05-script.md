# Stage 5 - Script

Output: `05-scripts/<sid>.json` (`mvf new-script <sid>`). One script carries
the video, the hook variants, the X post and thread and the cover text. `recipe`
names what it is built from (`tools/mvflib/recipes.json`: product-demo,
social-proof, stat-hook, listicle, quote, community; extensions add more).

```json
{ "id": "s01-name", "angle": "G1", "format": "problem-solution", "recipe": "product-demo", "language": "en",
  "value": "what the viewer walks away with",
  "voice": {"mode": "recorded|clone|design|preset|none", "ref": "clone reference", "name": "preset voice",
            "engine": "optional override", "speed": 1.0},
  "beats": [{"id": "B01", "role": "hook", "vo": "spoken line", "onscreen": "<= 5 words per line",
             "visual": "what the picture does", "facts": [], "material": [], "keys": ["words to emphasise"]}],
  "variants": [{"id": "H2", "vo": "", "onscreen": "", "visual": ""}],
  "platforms": {"tiktok": {"caption": "", "cover_text": "", "sound": "original|trend:T1"},
                "x": {"post": "", "thread": [], "cover_text": "", "facts": []}} }
```
Roles: hook, context, problem, tension, turn, demo, proof, payoff, reaction,
beat, cta, end. One beat = one scene. Variants H2..H9 replace B01 only, so B01
is a pure hook; any setup the body needs lives in B02+. Re-read the body after
each variant.

## Retention shape
Write for the viewer who leaves at 3 s, at 6 s and at half-way. Each of them
should leave with something, and the one who stays gets the most.

| Time | Beat | Does |
| --- | --- | --- |
| 0-2.5 s | hook | frame 0 + on-screen line + first words, each hooking alone; opens one question |
| by 6 s | first value (demo, proof, payoff, turn, reaction, beat) | the first tip, result or punchline, with the product on screen where it happens |
| about half-way | re-hook | the next, bigger thing ("the third one is the one nobody uses") |
| end | payoff (+ CTA) | answers the hook's question exactly; the CTA rides on it in one line |

- No setup between hook and value. The hook states the problem; `context` and
  `problem` beats, when needed, are under 2 s together. Cut the intro, the
  backstory and the "here is how it works".
- Value density: every beat gives something new (a step, a result, a laugh).
  A beat that only explains the previous one is cut.
- End on the payoff, not on a goodbye or a logo card. An ending that flows back
  into frame 0 earns rewatches.
- The brand is shown, not said: the real screen with the product's own name or
  logo in frame from the first value beat, optionally a small on-screen label
  the first time it appears (write the name into that beat's `visual` or
  `onscreen` so the lint sees it). Spoken at most once, at the payoff or CTA.

## Checks (`mvf lint <sid>`; FAIL blocks, WARN gets a note)
The lint measures on the latest take's timing once one exists, else on an
estimate it labels as such.
- **Structure:** at least 3 beats; B01 is the hook; unique ids; `value`, `format`,
  `angle` set and the angle exists.
- **Hook:** first spoken line ends within 2.5 s (TikTok) / 2.0 s (X) of frame 0
  (estimate at about 3 words/s until you have audio; more than 1 s over is a
  FAIL, cut words, never speed up); on-screen text exists at frame 0 (FAIL for X);
  <= 7 words on screen; no warm-up opener ("hey guys", "today we", "did you
  know"); frame 0 shows the subject in motion, not a logo or black.
- **Retention:** the first value beat starts by 6 s (WARN; more than 2 s late
  is a FAIL).
- **Brand:** the product name is not spoken or written in the hook, is seen
  (`visual` or `onscreen`) before half the runtime, and is spoken in at most
  one body beat. Selling words ("download now", "link in bio", "promo code",
  "limited time") only in the CTA. One CTA, said once, at 70% or later of the
  runtime, shown with the URL or handle.
- **Claims:** any number with a unit, price, "free", superlative or guarantee
  in a beat or in platform copy cites a fact id (or a material signal read within
  30 days). Name the honest limit as a beat when the audience would doubt it.
- **Material:** hook and payoff stand on cited items; every item shown on screen
  has showable rights; the credit is on screen and in the caption.
- **Length:** inside the platform's sweet spot; one idea per video.
- **Picture:** every beat says what is seen; a beat over 3.5 s has a visible
  change inside it; on-screen text <= 3 words/s; the on-screen text alone tells
  the story muted.
- **One cast:** the joke's example data is the data on your real screens.
- **Copy:** X post <= 280 weighted chars (emoji and many non-Latin or accented
  characters weigh 2, URLs count 23); X hashtags 0-2; TikTok caption's first
  line carries a segment search term, 3-5 hashtags; cover text <= 6 words.

Write all hooks before picking one, each a different mechanism
([04](04-strategy.md#hooks)); read aloud at speed and cut until every beat
earns its seconds. Check the hook by covering everything after 3 s: would the
segment know what they get by staying? Re-check after real audio exists: the real hook is often
longer than the estimate.
