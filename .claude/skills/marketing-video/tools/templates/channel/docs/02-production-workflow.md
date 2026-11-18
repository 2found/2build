# Production workflow

Only what this channel adds to the `marketing-video` skill. The stages and checks come from the skill; do not copy them here.

## Open an episode

```bash
mvf init <slug> --product "{{PRODUCT}}" --channel {{CHANNEL}} [--series NAME] [--format NAME]
```

`mvf` is `python3 <marketing-video skill>/tools/mvf.py`. Init copies `pack/campaign.json` into the library's product pack, writes `campaigns/<slug>/` and merges `pack/series/NAME.json` into its `campaign.json`. From then on: `mvf --campaign campaigns/<slug> next`.

- Slug: _naming rule, e.g. `<series code><number>-<keyword>`._

## Per stage

| Stage | This channel |
| --- | --- |
| brief | _defaults the owner fixed once_ |
| script | _house rules for hooks, length, call to action_ |
| delivery | _where it is posted, then one row in `metrics/videos.csv`_ |
