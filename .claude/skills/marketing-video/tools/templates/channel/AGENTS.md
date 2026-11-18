# AGENTS.md: {{CHANNEL}}

Videos for {{PRODUCT}}, made with the `marketing-video` skill (`mvf`). CLAUDE.md is a symlink to this file.

## Read first

1. [docs/01-bible.md](docs/01-bible.md): who the channel is for, what it promises, how it looks and sounds. Every script matches it.
2. [docs/02-production-workflow.md](docs/02-production-workflow.md): how an episode is opened and what this channel adds to the mvf stages.
3. The `marketing-video` skill: the shared standard. Where it and these docs disagree, the skill's FAIL checks win; content direction follows these docs.

## Rules

- **Open an episode with `mvf init <slug> --product "{{PRODUCT}}" --channel {{CHANNEL}} [--series NAME]`** (or this channel's own script in `bin/`, if it has one).
- **Edit brand, colours, fonts and pronunciations in `pack/campaign.json`**, never in the mvf library: every new episode overwrites the library's copy with this file.
- **Open decisions are in [docs/03-decisions.md](docs/03-decisions.md).** Follow the Proposed option until a Decided line exists; never decide for the owner.
- **The agent does not post, log in or spend.** The owner publishes.

## Layout

```text
docs/                bible, production workflow, decisions (series bibles beside them)
pack/campaign.json   the channel's product pack: brand, colours, fonts, pronunciations
pack/series/*.json   per-series overlay merged into each new campaign.json (`--series NAME`)
campaigns/<slug>/    one mvf campaign per episode
brand/               logo, avatar, banner, character sheets
metrics/             videos.csv (one row per posted video), weekly.md
```
