# Stage 0 and 0.5 - Brief and material

## Brief: `campaign.json`
```json
{ "slug": "launch-v1", "product": "Name", "goal": "awareness|signups|activation",
  "kpi": "3-second view rate >= 35%; 50 profile visits per 10k views",
  "markets": ["US"], "languages": ["en"], "platforms": ["tiktok", "x"],
  "brand": { "names": ["Name"], "say": {"Name": "how it is pronounced"},
             "handle": {"tiktok": "@...", "x": "@..."}, "url": "https://...",
             "colors": {"bg": "", "fg": "", "accent": ""}, "fonts": {"display": "", "body": ""} },
  "constraints": ["no competitor names", "no prices unless checked this week"],
  "budget": {"videos": 3, "hook_variants_per_video": 3} }
```
`mvf init <slug> --product NAME` writes it from the template (plus every
later stage file and the campaign `.gitignore`). Optional `"voice_engine"`
names the default voice engine (stage 7).

Gate (`mvf check brief`): product, goal, one measurable KPI, at least one
platform, language and brand names are set; `_template` is gone. Read the product's own branding
rules (spelling, case, words to avoid) before filling `brand`. Handles and any
paid spend are the user's call: ask, never guess.

## Material: `00-material/index.json`
Material is what the video will stand on that already has an audience: the
feature users praise most, a public changelog moment, reviews, a ranking the
product appears in, a meme the niche shares, official assets. Without it,
angles drift into what the product wants to say.

Item envelope (`mvf gather` validates it; `mvf material` reads the pack as cards):
```json
{ "id": "ns:kind:key", "kind": "product|feature|quote|review|post|ranking|metric|person|asset|trend",
  "title": "", "body": "the text a card shows", "signals": {"stars": 5, "likes": 1200},
  "source": {"name": "who read it", "url": "https://...", "fetched_at": "YYYY-MM-DD"},
  "rights": "own|licensed|public-domain|official-promo|unknown", "credit": "", "links": [], "meta": {} }
```
Ids are stable. Research by hand (search, the product's own analytics, store
reviews, community threads), save the items as a JSON list and
`mvf gather manual file=items.json`. A recurring source (the product's API, a
store export) becomes a **source extension** (`mvf extensions`, SKILL.md).

**Rights policy.** On screen: `own`, `licensed`, `public-domain`,
`official-promo` (posters, stills, trailers released to promote a title; use
briefly, credited on screen and in the caption, with your own commentary, never
replacing the original). `unknown` (fan edits, reposts, screen-recorded clips)
may inform the script, never appears in a beat. A real person appears only in
official pictures supplied by the rights holder.

Gate (`mvf check material`): at least 3 items with a signal and read date; every item has `rights`;
anything a beat will show is showable; a signal older than 30 days is re-read
or dropped. A script cites item ids in `beats[].material` the way it cites fact
ids. Skipping material is allowed for a pure feature demo; say so.
