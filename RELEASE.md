# 2build production release

Use `release-prd` from [.claude/skills/release-prd/SKILL.md](.claude/skills/release-prd/SKILL.md).
This profile covers `2found/2build`, base `main`, with the existing
[git policy](.babysit/git-flow.yaml). It does not release sibling repos.

## Version, notes and normal delivery

`VERSION`, `.claude-plugin/marketplace.json` (`metadata.version` and
`plugins[0].version`) and `.codex-plugin/plugin.json` (`version`) change in
one commit. Use a prepared version if present; otherwise derive semver from
the public CLI/skill contract changes. Tag `v<version>`. Update `CHANGELOG.md`
from the previous published stable tag, then regenerate the Soot pack with
`python3 scripts/build-2build-pack.py` before final verification.

Normal mode: [.github/workflows/release.yml](.github/workflows/release.yml)
owns checks → tag → archives/pack/docs → GitHub draft → formula commit →
public release. A version-changing main push triggers it; an unchanged
already tagged version does not. Follow this SHA's workflow and verify outputs.
See [installation/release contract](docs/install.md#for-maintainers-cutting-a-release).

## Local mode

An explicit `--local` production release permits temporary suspension of
affected automatic workflows using the skill's guard. Discover live states;
`release.yml`, `build.yml`, `lint-workflows.yml` are candidates, subject to
their event/path filters at remote main and the release candidate. Restore
only workflows this invocation disabled, after the final metadata push and
GitHub publication. Never dispatch/rerun Actions or download CI artifacts.

Prerequisites: Go from `go.mod`, Node/npm for the embedded dashboard, Python 3,
GitHub CLI with repo write/workflow management, GoReleaser v2, tar, and the
SHA-256 tool required by `scripts/gen-formula.sh`. Inspect GoReleaser hooks;
resolve any `go mod tidy` changes before freezing the source tag.

Run required checks locally from this repo root:

```sh
node --test scripts/release-plan.test.mjs scripts/release-assets.test.mjs
python3 tests/test_codex_plugin.py
python3 scripts/build-2build-pack.py --check-tree
go build -o bbs ./cmd/bbs
go test ./...
goreleaser check
```

Also run every `tests/test_*.sh` suite with this repo's `bbs` on PATH and
test-local git identity (preserve global git config). Mirror the six
OS/architecture cross-compiles in `build.yml`: darwin/linux/windows ×
arm64/amd64, `CGO_ENABLED=0`; Windows is verified but not distributed. Run
`./bbs autopilot lint-workflow` on existing builtin/project workflow files
when the lint-workflows path filter applies. A required failure blocks release.

After the source commit is on remote main and its immutable local/remote tag
is verified, build locally from that exact clean source. Bind the following
variables to the resolved identity, not arbitrary caller text:

```sh
goreleaser release --clean --skip=publish
python3 scripts/build-2build-pack.py --version "$RELEASE_VERSION" --commit "$RELEASE_SHA" --out dist/2build --tarball "dist/2build-pack_${RELEASE_VERSION}.tar.gz"
GITHUB_REPOSITORY=2found/2build node scripts/release-assets.mjs "$RELEASE_VERSION" dist
```

Produce the pack's `.sha256` alongside its archive before `release-assets.mjs`,
matching `release.yml`'s checksum filename format. Use clean `dist/`; inspect
and upload the explicit expected assets to a GitHub draft with the prepared
changelog notes plus generated `DOWNLOADS.md`:

- Four `bbs_<version>_<darwin|linux>_<amd64|arm64>.tar.gz` CLI archives,
  containing the embedded dashboard and the injected release version.
- `2build-pack_<version>.tar.gz` and its `.sha256`.
- `2build-docs_<version>.tar.gz`, `DOWNLOADS.md`, `release.json`, `checksums.txt`.

Download from the draft and compare recorded checksums. In the main checkout,
ensure main's `VERSION` still equals this release, then generate
`Formula/bbs.rb` using `scripts/gen-formula.sh "$RELEASE_VERSION" <checksums-file>`.
Commit/push only that metadata change if needed; do not move the source tag.
Publish the complete draft last. Formula updates are distribution; no npm
package belongs to this repo. `scripts/release-plan.mjs` is CI-only, so use its
tests/policy as guidance rather than forging CI environment variables.

## Post-release, local CLI and hypercare

App deploy: N/A. The dashboard is embedded in the CLI. Verify a downloaded
host-platform archive in an isolated directory, without relying on a checkout:
`bbs --version`, `bbs install --help`, and a harmless invalid subcommand that
must return nonzero. Verify dashboard assets are present/servable.

Inspect `command -v bbs`, all PATH copies and installation ownership. Update
the selected local installation using [the install contract](docs/install.md):
Homebrew via `brew upgrade 2found/2build/bbs`, an installed checkout via its
supported update path, or the exact released archive. A moving upgrade channel
must be confirmed to select this version; otherwise install the pinned archive
without overwriting a package-manager-owned binary. Verify the active version
and smoke checks; refresh installed skills via the supported install/update
path and report that the coding agent needs a restart to load the new pack.

Hypercare: sample release downloads/formula and local smoke at start and after
2 minutes. Any missing/corrupt asset, wrong version, broken formula or CLI
startup failure blocks completion. A public artifact error needs a new version;
restore the local CLI to its recorded previous artifact if necessary.
The 2found website consumes releases hourly; an explicit local release does
not send the workflow's repository-dispatch notification. Inspect its consumer
status and record pending hourly propagation as a concern; do not deploy the
website unless that sibling target is explicitly selected.
