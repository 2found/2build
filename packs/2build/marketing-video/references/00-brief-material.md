# Stage 0 - Brief and material

Output: `campaign.json` and `00-material/index.json`.

## Bar
- **The brief** names the slug, the product, a goal, the platforms, the
  languages, the markets and every spelling of the brand. A measurable KPI is
  expected.
- **The benefit** is what a viewer can use after the video: `lesson`,
  `decide`, `answer`, `match` or `how-to`. Each angle picks its own
  ([04](04-strategy.md#benefit)); write `benefit` into the brief only when the
  owner names one, and do not ask for it. Handles and any paid spend are the user's call
  too; ask, and never guess them.
- **The material pack** is not empty (or `material_skip` says why). Every item has a kind, a title, a source
  with a read date, and rights. Anything from outside has a URL, and
  `official-promo` carries a credit.

`mvf check brief` and `mvf check material` FAIL when any of this is missing
or invalid.

## Direction
- Material is what already has an audience: an app's best store review, a
  clip players share, a book's most-quoted line, a ranking, an official asset.
  Without it, angles drift into what the product wants to say. A pure feature
  demo may skip it; say so in `campaign.json` `material_skip: "<why>"`, and an
  empty pack then passes `mvf check material`.
- Some of it should measure what people like (reviews, rankings, metrics). A
  signal older than 30 days is re-read before a script cites it.
- Fill `brand` from the product's own branding rules.

**Rights.** On screen: `own`, `licensed`, `public-domain`, and `official-promo`
(brief, credited on screen and in the caption, with your own commentary).
`unknown` may inform a script, never appear in it. A real person appears only
in official pictures from the rights holder.

## Tool
- `mvf init <slug> --product NAME [--root DIR]` writes every stage file from
  the templates, plus the campaign `.gitignore`.
- `campaign.json` holds `slug`, `product`, `goal` (awareness, follow, signup,
  install, purchase, retention or launch), `benefit`, `kpi`, `markets`,
  `languages`, `platforms` (tiktok, x, youtube, youtube-long, instagram, facebook,
  linkedin),
  `brand.{names, say, handle, url, logo, colors, fonts}`, `constraints`,
  `budget` (`scripts`: candidates to write, default 3; `videos`;
  `hook_variants_per_video`), and optionally `voice_engine` and `known_words`
  (words this audience already knows, such as a country or the title they
  search for, which the stranger checks then skip).
  `style` is `none` for a new campaign: each video designs its own look. Set
  it to a preset (`stickman`, `ink`, `anime`) only for a series that keeps
  one. A direction or art.json that names its own preset wins.
- `mvf gather <source> [k=v …]` validates and merges items. Use
  `mvf gather manual file=items.json` for hand research. `mvf sources` lists
  the sources; a recurring one becomes a source extension (`mvf extensions`).
- `mvf material [ls|show ID…] [--kind K] [-q TEXT]` reads the pack as cards,
  strongest first.
- An item is `{id: "<ns>:<kind>:<key>", kind, title, body, signals: {name: number},
  source: {name, url, fetched_at}, rights, credit, links: [ids], meta}`. The kinds
  are product, feature, quote, review, post, ranking, metric, person, asset and
  trend, plus any that extensions add.
- A script cites item ids in `beats[].material`.
