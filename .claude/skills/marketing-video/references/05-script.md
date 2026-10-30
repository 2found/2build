# Stage 5 - Script

Output: `05-scripts/<sid>.json`. One script carries the video, the hook
variants, the X post and thread and the cover text.

```json
{ "id": "s01-name", "angle": "G1", "format": "problem-solution", "language": "en",
  "value": "what the viewer walks away with",
  "voice": {"mode": "own-recording|tts|none", "ref": "who/what voice, with rights note", "speed": 1.0},
  "beats": [{"id": "B01", "role": "hook", "vo": "spoken line", "onscreen": "<= 5 words per line",
             "visual": "what the picture does", "facts": [], "material": [], "keys": ["words to emphasise"]}],
  "variants": [{"id": "H2", "vo": "", "onscreen": "", "visual": ""}],
  "platforms": {"tiktok": {"caption": "", "cover_text": "", "sound": "in-app|bed"},
                "x": {"post": "", "thread": [], "cover_text": "", "facts": []}} }
```
Roles: hook, context, problem, tension, turn, demo, proof, payoff, reaction,
beat, cta, end. One beat = one scene. Variants H2..H9 replace B01 only, so B01
is a pure hook; any setup the body needs lives in B02+. Re-read the body after
each variant.

## Checks (apply each; FAIL blocks, WARN gets a note)
- **Structure:** at least 3 beats; B01 is the hook; unique ids; `value`, `format`,
  `angle` set and the angle exists.
- **Hook:** first spoken line ends within 2.5 s (TikTok) / 2.0 s (X) of frame 0
  (estimate at about 3 words/s until you have audio; more than 1 s over is a
  FAIL, cut words, never speed up); on-screen text exists at frame 0 (FAIL for X);
  <= 7 words on screen; no warm-up opener ("hey guys", "today we", "did you
  know"); frame 0 shows the subject in motion, not a logo or black.
- **Value before brand:** the product name is not in the hook, appears at the
  turn, is not repeated through the body. Selling words ("download now", "link
  in bio", "promo code", "limited time") only in the CTA. One CTA, said once,
  at 70% or later of the runtime, shown with the URL or handle.
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

Write all hooks before picking one; read aloud at speed and cut until every beat
earns its seconds. Re-check after real audio exists: the real hook is often
longer than the estimate.
