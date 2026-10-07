# Project starters

`bbs bootstrap` creates a runnable project from the separately versioned
`2found/2build-starters` catalog. The initial template is `hono-bun`. These commands
are available in CLI 1.95.0+; the first stable starter release is `0.1.0`.
Bootstrap uses published releases by default; `--source` supports local development.

```bash
bbs bootstrap my-api --template hono-bun --profile startup
bbs bootstrap another-api --template hono-bun --version 0.1.0
bbs bootstrap local-api --source /path/to/2build-starters --template hono-bun
bbs starter check --dir my-api --json
```

The parent directory must already exist, the target must not exist, and its name
must be lowercase kebab-case. Profiles: `pet`, `startup` (default), `enterprise`.
Bootstrap resolves a published stable `vX.Y.Z` release to a Git commit, validates
the catalog's CLI requirement, composes common/template files in a temporary
directory, runs the template's verification commands, and publishes the project
only when they pass. Commands execute directly, not through shell interpolation.
It never initializes Git, commits, pushes, or deploys.

`--json` prints the generated directory, dev argv, provenance and `verified` status;
verification logs go to stderr. `--no-verify` writes files without installing or
checking and reports `verified: false`. Local `--source` uses a content digest
instead of misreporting a Git release revision; `--version` must match its catalog.

## Update contract

`.babysit/starter.lock.yaml` is committed with the generated project. It records
schema version, source, release, template ID/revision and source revision. It is
separate from machine-local workspace `harness_version` and the CLI/plugin version.

`bbs starter check` searches the selected directory and its parents for that lock.
It reads the latest published stable release, pins its catalog to a commit and
reports `update_available` only when the catalog version and selected template
revision are newer. JSON statuses: `up_to_date`, `update_available`, `ahead`,
`template_missing`, `unknown`. `latest` is the catalog release; an unrelated
template release can therefore differ while status remains `up_to_date`.

Responses include current/latest versions, template, applicable summary, pinned
upgrade guide, minimum CLI version, checked time, cache/stale flags and warning.
Malformed project locks/catalogs return an error; offline release checks are
advisory and return cached data with a warning or `unknown` without cached evidence.

The machine-state `starter/catalog-cache.json` caches release data for 24 hours;
failed requests are throttled for 15 minutes. `--force` retries immediately.
`--source /path/to/catalog` compares a local catalog without reading/writing that
cache. Neither mode modifies project files.

Standard `bbs skill enter --name ...` runs the check on stderr when a starter lock
exists. Ordinary repos incur no starter network work. `update_check: false` in user
config suppresses automatic checks. JSON-only `skill enter --json` stays a telemetry
API; its caller runs `bbs starter check --json` when it owns workflow bootstrap.

The agent reads the upgrade guide, preserves application edits, applies the relevant
changes, verifies, then updates the lock to the release actually applied. There is
no automatic file sync. `bbs update` upgrades CLI/plugins separately and does not
advance starter provenance or application dependencies.
