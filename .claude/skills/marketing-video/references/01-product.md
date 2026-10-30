# Stage 1 - Product facts

Output: `01-product/facts.json`, `01-product/screens/`, `brand/` (logo).
The facts file is the only place a script may take a claim from.

```json
{ "one_liner": "how a viewer would explain it to a friend, not the tagline",
  "category": "", "jobs": ["the job a viewer hires it for, in their words"],
  "alternatives": [{"name": "", "why_switch": "", "why_stay": ""}],
  "facts": [{"id": "F1", "kind": "feature|benefit|proof|price|number|limit|positioning",
             "claim": "", "source": {"type": "url|file|measured|user", "ref": "", "quote": "verbatim"},
             "checked": "YYYY-MM-DD"}],
  "demo_moments": [{"what": "the 3 seconds of real product that prove the promise", "screen": "path", "facts": ["F1"]}],
  "screens": [{"path": "01-product/screens/x.png", "provenance": "prod app, demo account", "facts": ["F1"]}] }
```

Kinds: `feature` (the demo), `benefit` (the payoff line), `proof` (a real result
or public sample), `price` (the offer), `limit` (what it does NOT do; keeps the
script honest and pre-empts the top comment), `number` (users, speed; only if
sourced), `positioning` (category, who it is for, what it replaces).

Sources: `url` (live page plus verbatim quote and read date), `file` (repo path
plus quote), `measured` (something you did: a timing run, a screenshot of the
real flow), `user` (the owner said it; record who and when).

**Competitor check, always.** Search the category on TikTok and the web; write
down what competitors already claim. A claim they also make is not "the only"
(add to `constraints`). Their ads go to stage 3 as `avoid` or `adapt`.

**Real screens.** Capture at delivery size: for 9:16 use a phone-width viewport
(390-430 CSS px at 3x DPR) so text survives a 1.6-2x push-in. Best first: the
production app with a demo account the user provides (never guess passwords),
a public demo page, a recent QA screenshot set. Crop test-account names. Note
the sample data on the screens (names, titles): the script's jokes and examples
use that same data so demo and story are one. Never draw a UI.

Gate (`mvf check product`, plus by eye for screens and competitors): every fact has a source with quote and date; no `price`
or `number` fact older than 30 days; at least one `limit`; one demo moment has a
real screen file; competitor claims recorded.
