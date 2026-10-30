---
name: marketing-video
description: Plan and produce short marketing videos (TikTok and X first) for your own product, from product facts and audience research to a script with A/B hooks, production and per-platform files; you post them.
disable-model-invocation: true
---
# marketing-video
Product to platform-ready short video, done by an agent for a founder or indie
dev who has no marketing team. Manual stopgap until 2found's marketing product
(2market) ships. Follow [preamble](../references/preamble.md) for bootstrap and
telemetry; shared refs are filesystem paths beside this skill's directory, so
read them by path, not as `skill://`.

One campaign = one folder, suggested `marketing/campaigns/<slug>/`. Gitignore
large media (`07-video/`, `08-delivery/`, `*.mp4`, `*.wav`); commit the
Markdown/JSON artifacts. Each stage writes one artifact, each artifact has a
gate checklist you apply to it yourself, and nothing moves forward on a FAIL.
There is no tool to run: artifacts are plain JSON/Markdown whose shapes are in
the stage references. Ask the user for the product, goal and where it lives if
the request does not say; otherwise decide and note the assumption.

## The flow
| # | Stage -> artifact | Do | Reference |
| --- | --- | --- | --- |
| 0 | brief -> `campaign.json` | goal + KPI, markets, languages, platforms, brand (names, pronunciations, colors, font, handles), constraints | [00](references/00-brief-material.md) |
| 0.5 | material -> `00-material/index.json` | gather what people already like or know: best-loved feature, public posts, reviews, official assets, ranked lists | [00](references/00-brief-material.md) |
| 1 | product -> `01-product/facts.json` | every claim the video may make, quoted from a source with a check date; real screens in `01-product/screens/` | [01](references/01-product.md) |
| 2 | audience -> `02-audience/segments.json` | 1-3 segments from evidence, pains in their words, search terms, anti-segments | [02](references/02-audience.md) |
| 3 | trends -> `03-trends/trends.json` | formats, hooks, sounds observed this fortnight with URL + metric + decision | [03](references/03-trends.md) |
| 4 | strategy -> `04-strategy/angles.json` | 2-3 angles = segment x insight x promise x format, each standing on facts | [04](references/04-strategy.md) |
| 5 | script -> `05-scripts/<sid>.json` | beats (one beat = one scene), 2-3 hook variants, platform copy | [05](references/05-script.md) |
| 6 | pre-test -> `06-pretest/<round>/decision.json` | simulated panel, honest small human panel, or skipped with a reason | [06](references/06-pretest.md) |
| 7 | video -> `07-video/<sid>/<hook>/` | voice, scenes, captions, sound bed | [07](references/07-production.md) |
| 8 | delivery -> `08-delivery/<sid>/<hook>/<platform>/` | render, master, encode per platform, QA, captions, cover, copy | [08](references/08-delivery.md) |

Open only the reference of the stage you are in. Stages 1-4 are research: a
guess is written down as `inference`, never as a fact. Iterate backwards freely:
a claim FAIL sends you to stage 1, a dead trend to stage 3. Only run the stages
the user needs (research only, or script only) and say where you stopped.

## Rules that cost people when broken
- **Claims stand on facts.** Every quantity, price, "free", superlative or
  guarantee in a beat or in platform copy names a fact id. Price and limits are
  re-read before every export (older than 30 days: re-check).
- **Real product on screen.** UI is a captured screenshot or recording, never a
  drawn mock-up. A missing screen is a labelled placeholder and a report line.
- **Material before angles.** Choose the angle from what the audience already
  likes or recognises, not from what the product wants to say.
- **Native, not an ad.** Hook <= 2.5 s on TikTok, <= 2.0 s on X, on screen from
  frame 0 (X autoplays muted). No selling words before the CTA. One CTA.
- **Measured, not hand-typed.** Scene slots, word times and loudness come from
  the audio and the rendered file, not from estimates. Estimates are labelled.
- **Rights.** Show only `own`, `licensed`, `public-domain` or `official-promo`
  (brief, credited, with your own commentary). `unknown` may inform the script,
  never appear on screen.
- **No AI likeness of a real person**, and never prompt an image model with a
  real person's name.
- **Trend sounds are added in the app** at upload. Never bake a licensed track
  into the file; the fallback is a royalty-free or generated bed you may use.

## Boundary
Never log in to, upload to or post on any platform, and never spend money
(ads, boosts, paid APIs) without the user's explicit say. Delivery ends at local
files plus caption/post text; the user posts. No secrets in campaign files.
Observe platforms logged out only; do not like, follow or comment.

## Report
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
CAMPAIGN: <folder> | STAGES: <run / skipped and why>
FILES: <delivered paths with duration, size, loudness, QA verdict>
OPEN: <unverified trends, placeholder screens, untested hooks, what the user must watch and post>
```
