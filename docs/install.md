# Install 2build

For agent-assisted installation, copy the [one installation prompt in the README](../README.md#install-with-one-prompt).
2build needs **both** the `bbs` CLI and the skill pack in your coding agent.
No 2build checkout or Orca installation is needed for Autopilot.

## Installation contract for agents

1. Identify the OS/architecture and the **current harness from session context**.
   Installed executables alone do not identify the agent running this session.
   Target that harness, rather than installing into every agent on the machine.
2. Check `command -v bbs`, `bbs --version` and `bbs install --help`. Reuse a
   working released CLI that supports installation. If an older CLI lacks
   `install`, use `bbs update` for a supported install shape or replace it with
   a current release archive, then recheck.
   For a missing CLI, use Homebrew below if available, otherwise a verified
   release archive. Do not require Go, Node, a source checkout or Orca for a
   release install. Windows users should run the CLI and coding agent in WSL.
3. For Claude Code/Codex, check the harness CLI is on PATH and supports
   `plugin` commands. A desktop app alone is insufficient. If missing, report
   the exact prerequisite and let the user enable/install that harness's CLI;
   do not silently install the pack for a different agent.
4. Run `bbs install claude`, `bbs install codex`, or `bbs install antigravity`
   for the current harness. Installation must exit successfully. If a command
   fails, fix a recoverable local issue and retry for that target, or report the
   failed command and the missing prerequisite.
5. Verify **both layers** before reporting installed:

   | Layer | Check |
   |-------|-------|
   | CLI | `bbs --version` reports a release version and `bbs install --help` works from the user's shell. |
   | Claude Code | `claude plugin list --json` shows `bbs@babysit` installed and enabled. |
   | Codex | `codex plugin list --json` shows 2build installed and enabled; do not confuse an available marketplace listing with an installed plugin. |
   | Antigravity | Its managed plugin has `plugin.json`, `skills/autopilot/SKILL.md` and `skills/references/preamble.md` in the locations below. |

6. Tell the user to restart the affected agent and supply its first invocation:
   Claude Code `/bbs:autopilot`, Codex `$bbs:autopilot`; in Antigravity ask it to
   use the installed `autopilot` skill. After restart, confirm that the agent
   can discover the skill before claiming it is loaded. Explain the
   [first-ticket plan → `/goal` → build handoff](../README.md#try-your-first-ticket).

Completion means the CLI is runnable and the pack is installed for the intended
agent. A restart still pending means **installed, restart required**, not a
verified first run. The user's existing agent/model account supplies inference;
2build adds no separate model service requirement. Normal agent usage costs apply.

## Install into your harness

```bash
bbs install                 # detect and install for all supported harnesses
bbs install claude          # Claude Code only
bbs install codex           # Codex only
bbs install antigravity     # Antigravity only
```

Detection checks CLI executables, Antigravity's `.gemini/antigravity*`
directories, and Codex / Antigravity macOS apps. Leftover `~/.claude` or
`~/.gemini/config` directories are not treated as an installed harness.
Claude Code and Codex require their CLI on PATH:
`bbs install` registers the 2build marketplace and installs `bbs@babysit`.
The marketplace keeps its legacy ID `babysit` for compatibility; the source is
`2found/2build`. An existing marketplace is refreshed, so rerunning the command is supported.
Failures name the harness and return a nonzero exit code; installation still
continues for other detected harnesses. No detected harness is an error with
instructions, not a silent success. Restart affected harnesses afterwards.

Antigravity receives a native skill plugin at `~/.gemini/config/plugins/bbs`;
when its CLI is detected, also at `~/.gemini/antigravity-cli/plugins/bbs`.
These are the [documented Antigravity plugin locations](https://antigravity.google/docs/plugins/).
The installer uses the CLI checkout's skills when available; otherwise it
downloads the matching release tag using Git. It preserves sibling references,
workflows, scripts and data, and only replaces bundles marked as managed by
`bbs install`. This installs skills; it does not port Claude's hooks or add
Antigravity worker dispatch to Foreman.

For 2build development, `go run ./cmd/bbs setup --full` builds the CLI and
prints the commands for registering the checkout as a local marketplace.

## What `bbs` gives you today

`bbs` is a [multicall binary](#how-the-aliases-work): one executable that
behaves differently depending on the name it's invoked as. Most of babysit's
core bins are now Go and ship inside this one binary, reachable as `bbs <sub>`:

| You run | Runs | What it does |
|---------|------|--------------|
| `bbs install [claude\|codex\|antigravity]` | `install` | install the skill pack for one harness or all detected harnesses |
| `bbs config …` (alias `bbs-config`) | `config` | read/write `~/.babysit/config.yaml` |
| `bbs ticket …` | `ticket` | ticket identity core (`env`, `resolve`, `set-verdict`, `verdict-status`, `session`, `board`) — see the strangler note below |
| `bbs update` | `update` | update the active CLI installation, an additional Homebrew copy when one is installed, and installed harness plugins; `update check` probes for a newer release |
| `bbs secrets …` (alias `bbs-env`) | `secrets` | project-local `.babysit/.env` credential loader (`load` / `seed` / `ensure-gitignore`), env resolution with `.env.base` auto-load (`resolve` / `is-set` / `list-prefix` / `prompt`), and `.babysit/qa.yaml` fields (`qa probe` / `qa list` / …) |
| `bbs design …` | `design` | design-intelligence broker (`tokens` / `suggest` / `components` / `ux-check`) — the CSV/DESIGN.md data files ship with the skill pack |

**Strangler note on `ticket`:** the Go `ticket` command owns the identity core
(resolve, verdicts, session, board), the index.json state-accessors
(`env`, `get`, `set-status`, `set-phase`, `set-parent`, `set-sibling`,
`set-pointer`, `ensure-size`, `append-history`), the file-only
manifest.yaml ops (`init`, `get-manifest`, `set-branch`), the base-ops family
(`refresh`, `surface`, `serve`, `land`), `ensure`, and
`path`/`list`/`reconcile`. `bbs ticket` is now entirely
self-contained in the binary — a brew-only install runs every subcommand
without the skill pack.

**Note on `dashboard`:** released binaries carry the built SPA inside them
(`internal/webui`, staged by `scripts/build-webui.sh` in goreleaser's
before-hooks), so `bbs dashboard` on a brew-only install serves the real
dashboard with no checkout and no npm. A checkout's own `web/dist` takes
precedence when it exists, so `bbs dashboard build` still does what it always
did. `--snapshot` needs real files next to `index.html`, so it unpacks the
embedded copy into `~/.babysit/cache/dashboard` and writes `data.js` there.

**Note on `design`:** the `design` command itself is Go (ships in the binary),
but its CSV/DESIGN.md data files live in the skill pack, so a brew-only
`bbs design suggest` needs `--data <dir>` pointed at a skill-pack checkout.

The release binary contains the CLI. `bbs install` supplies the skill pack
(skills, workflows, DESIGN.md/CSV data) through each harness's plugin system.

`bbs --version` (or `-v`) prints the version. A release binary reports the tag
it was built from, injected at build time; a clone install has no injected value
and reads `VERSION` from the checkout instead, so it stays accurate after a
`git pull` without a rebuild. `unknown` means neither source was available.

## macOS — Homebrew (primary)

```bash
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs install
```

The explicit tap URL is required because the repository is `2found/2build`
rather than the conventional `homebrew-2build` name. The fully qualified formula
selects this tap's package. See [Homebrew's tap documentation](https://docs.brew.sh/Taps).

Verify:

```bash
bbs --help          # babysit CLI
bbs --version       # installed CLI version
```

Upgrade / uninstall:

```bash
bbs update
brew uninstall bbs
```

`bbs update` follows the running binary: a checkout is pulled and rebuilt;
a Homebrew installation is upgraded with `brew upgrade bbs`. When running
from a checkout, it also checks `brew list --versions bbs` and upgrades that
copy if present. A failed Homebrew upgrade returns a failure instead of
claiming the entire update succeeded. Other standalone binary copies must
be replaced manually. Use `type -a bbs` to inspect PATH precedence.

## Release archives (macOS or Linux)

Homebrew works on macOS and Linux. Without it, download the archive for your OS
and architecture plus `checksums.txt` from the **same**
[release](https://github.com/2found/2build/releases/latest).
Use the platform matrix below to choose the archive. Verify its SHA-256 with
`sha256sum` on Linux or `shasum -a 256` on macOS against the matching row in
`checksums.txt` before extracting. Keep the published filename so it matches
that row. Do not install an archive with a missing or mismatched checksum.

After verification, extract `bbs` and install it:

```bash
mkdir -p ~/.local/bin
install -m 0755 bbs ~/.local/bin/bbs
export PATH="$HOME/.local/bin:$PATH"
bbs --version
bbs install
```

Add the PATH entry to your shell's startup file if it is missing, then verify
`bbs --version` in a new shell. The `bbs-config` / `bbs-env` compatibility aliases
are optional; current skills call `bbs <subcommand>` directly.

## Troubleshooting the first install

| Symptom | Next step |
|---------|-----------|
| `brew` is missing | Use the release archive path above; Homebrew is optional. |
| `bbs: command not found` after extracting | Check `~/.local/bin/bbs --version`, add `~/.local/bin` to PATH and restart the shell/agent. |
| No supported harness detected | Run `bbs install <harness>` for the current supported agent. Claude Code/Codex still require their CLI on PATH. |
| Harness has no `plugin` subcommand | Update/enable its CLI using that harness's own installation instructions, then retry the selected target. |
| Installation exits nonzero | Read the named harness's error; another harness succeeding does not mean this one installed. Retry `bbs install <harness>`. |
| Skill is missing after installation | Restart the agent, check the plugin is enabled, and verify installation in the same user/environment where the agent runs. |

For a source/development install, use `go run ./cmd/bbs setup --full` in a
checkout, then run the local marketplace registration commands it prints.

## Platform matrix

Released artifacts (no Windows — see below):

| OS | arch | artifact |
|----|------|----------|
| macOS | arm64 (Apple Silicon) | `bbs_<version>_darwin_arm64.tar.gz` |
| macOS | amd64 (Intel) | `bbs_<version>_darwin_amd64.tar.gz` |
| Linux | arm64 | `bbs_<version>_linux_arm64.tar.gz` |
| Linux | amd64 | `bbs_<version>_linux_amd64.tar.gz` |

**Windows:** `bbs` is cross-compiled for Windows in CI purely as a regression
check (so a change that breaks the Windows build fails a PR). **No Windows
artifact is published.** The runtime hooks are compiled into `bbs`
(`bbs hooks pre-tool-gate` / `bbs hooks session-writer`), so they run on any
OS with no bash or jq on PATH. For native PowerShell/cmd, the skill pack
ships `.claude/skills/references/preamble.ps1` (the shared preamble — same
state-echo contract as the bash block) and `bbs setup` (the installer — `go
run ./cmd/bbs setup` from a checkout); skill bodies still assume a POSIX
commands, so Git-Bash/WSL remains the fully supported path.

## How the aliases work

`bbs` inspects `argv[0]`: the alias `bbs-config` runs the `config` subcommand,
and the alias `bbs-env` runs the env resolver that now also answers to
`bbs secrets resolve`. The Homebrew formula installs the real binary once and
adds `bbs-config` / `bbs-env` as symlinks to it — so the `bbs-*` names work
exactly like the in-repo dev symlinks, without a separate build per bin.

`env` and `qa-config` are still reachable as top-level commands, but hidden
from `bbs --help`: they moved under `secrets`, and the old spellings stay only
so a brew-updated binary keeps working for a skill pack that hasn't upgraded
yet. New callers should use `bbs secrets …`.

## For maintainers: cutting a release

Changing `VERSION` and pushing to `main` triggers the release pipeline.

1. Bump `VERSION`, both fields in `.claude-plugin/marketplace.json`
   (`metadata.version`, `plugins[0].version`) and `.codex-plugin/plugin.json`
   (`version`) — the 4-field rule in
   [CLAUDE.md](../CLAUDE.md#releasing--version-bumps).
2. Push the approved version change to `main`. The workflow creates its tag
   inside the release job; a manual `v*` tag remains a supported alternative.
3. `.github/workflows/release.yml` validates all four versions, builds the four
   archives plus checksums, commits the real checksums into `Formula/bbs.rb`,
   then publishes the GitHub Release.

Validate the pipeline locally without tagging:

```bash
goreleaser check                       # config is valid
goreleaser build --snapshot --clean    # 4 binaries
brew style Formula/bbs.rb              # formula lint
```
