# Stage 1 - Product facts

Output: `01-product/facts.json`, real screens in `01-product/screens/` and the
logo in `brand/`. The facts file is the only place a script may take a claim
from.

## Bar
- `one_liner` says what the product is in the viewer's words, not as a
  tagline.
- Every fact has a unique id, a claim, a source and the date it was checked.
- A `url` or `file` source carries the verbatim quote the claim rests on, and
  a `url` source is a real http(s) link.

`mvf check product` FAILs on these. It WARNs on fewer than 3 facts, no
`proof`, no `limit`, or a price or number checked over 30 days ago.

## Direction
- A promise needs a proof (a real result, a number, a quote, a screen). A
  `limit` keeps the script honest and pre-empts the top comment.
- Know what competitors already claim. A claim they share is not "the only"
  (add it to `constraints`).
- Show the real thing: screens captured at delivery size from the real product
  (a demo account the user provides, never guessed passwords), footage of a
  physical product in use, the passage of a book, a service's real before and
  after. Never a mock-up. Crop test-account names, and let the script's
  examples use the sample data on the screens.

## Tool
`facts.json` keys:
- `one_liner`, `category`, `jobs`
- `alternatives: [{name, why_switch, why_stay}]`
- `facts: [{id: "F1", kind, claim, source: {type: url|file|measured|user, ref, quote}, checked}]`
- `demo_moments: [{what, screen, facts}]`
- `screens` or `assets: [{path, …}]`

The kinds the checks read are `proof`, `limit`, `price` and `number`. Others,
such as feature, benefit and positioning, are free labels.

A script cites fact ids in `beats[].facts` and in its platform copy.
