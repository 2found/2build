# Companion CLI

There is **one binary**: `bbs`. `bbs setup` builds it and symlinks it onto
your `PATH` at `~/.local/bin/bbs` (`brew install bbs` gets you the same binary
with no checkout). Every command below is a subcommand of it — call them as
`bbs <sub>`. Run `bbs <sub> --help` for full usage.

`bbs` is a multicall binary: it also dispatches on `argv[0]`, so a `bbs-<name>`
argv0 alias still works (`bbs-config` → `bbs config`). Those are **legacy
aliases only** — a checkout install ships none, and a Homebrew install ships
just two (`bbs-config`, `bbs-env`) — which is why skills and docs always use
the space form.

Every subcommand is now native Go, with retained behavior guarded by
the differential harnesses in `tests/`. `ticket` was the
last strangler: identity/verdict/session/board, the index.json state-accessors
(`get`/`set-*`/`add-child`/`add-relation`/`remove-relation`/`ensure-size`/
`append-history`/`env`), the file-only
manifest.yaml ops (`init`/`get-manifest`/`set-branch`), the git-mutating
base-ops (`refresh`/`surface`/`serve`/`land`),
`ensure`, and `path`/`list`/`reconcile`. No bash remains in
production; a frozen byte-identical copy of the old script lives at
`tests/fixtures/bbs-ticket.reference` purely as the differential oracle.

| Command | Purpose |
|---------|---------|
| `bbs bootstrap` / `bbs starter check` | Create a verified project from a versioned starter; report applicable releases from committed provenance without changing application files. [Starter commands and update contract](starters.md). |
| `bbs autopilot` | `snapshot --json` reads canonical state, mode, policy and gate evidence; `recover --json` adds bounded artifact excerpts for recovery. `checkpoint`, `attempt` and `verification` persist execution state; `clear`, `base-branch`, `git-flow` and `lint-workflow` support lifecycle and policy. |
| `bbs ticket` | Ticket-layout broker and state-probe surface. `env` derives `SLUG`/`BRANCH`/`TICKET`/`BABYSIT_PROJECT_HOME` through the identity ladder — `BABYSIT_TICKET` env → `manifest.yaml` cwd-match → branch regex — which is what every skill preamble evals and what autopilot resume relies on; `path <kind>` resolves Layout C file paths; `verdict-status --skill <n>` reads the latest verdict for a sub-skill (used by autopilot's Probe and Verify-post) |
| `bbs config` | `get` / `set` / `list` plus `workspace` operations, all in `~/.babysit/config.yaml` |
| `bbs semantic-decision` | Bounded judgment step: `--input <request.json> --kind <kind>` routes to the current LLM by default or configured Cloudflare; `--llm-answers <answers.json>` records the LLM decision. [Configuration and human-review policy](operations.md#settings-without-a-dashboard-control). |
| `bbs foreman model` | Prints the effective model policy as JSON; add `--agent codex --complexity normal --phase-class critical` to select a model/effort. Defaults are built into bbs; override individual fields in `~/.babysit/settings.json` or `<repo>/.babysit/settings.json` (repo wins). `--dir` selects the assignment repo/worktree. See [model settings](../.claude/skills/foreman/references/model-routing.md#model-tiers). |
| `bbs update` | `git pull` + `bbs setup`, then refreshes installed Claude Code and Codex plugins; writes a `JUST_UPGRADED` marker. `bbs update check` is the cached probe — prints `UPGRADE_AVAILABLE <old> <new>` when a new release exists |
| `bbs secrets` | Everything a skill reads to reach a running app. `load` (emit `export KEY='…'` for `.babysit/.env` keys not already in shell env) / `seed` / `ensure-gitignore` — project-local credential auto-loader; `resolve` / `is-set` / `list-prefix` / `prompt` — env resolution with `.env.base` auto-load; `qa <probe\|list\|default-env\|check\|leak-check>` — named-environment fields (`url`, `start`, `check`, `flows`, `prepare`/`revert`) from `.babysit/qa.yaml` |
| `bbs dashboard` | Serves the dashboard + JSON API on `127.0.0.1` and opens it. The SPA is embedded in released binaries, so a brew-only install needs no checkout and no npm; a checkout's own `web/dist` wins when it exists. `--snapshot` writes `web/dist/data.js` and opens the `file://` build instead, `build` rebuilds `web/`, `--no-open` for CI, `--dev` for vite + HMR |

CLI simplification removes these redundant entry points:

| Removed | Replacement | Reason |
|---------|-------------|--------|
| `bbs design` (`tokens`, `suggest`, `components`, `ux-check`) | Read the project's design doc and component/token source directly. | No workflow needs the parser or generic CSV recommendations; `design-ui` handles project context and visual iteration. |
| `bbs autopilot probe`, `bbs autopilot explain` | `bbs autopilot snapshot --json` | The skill already uses the canonical snapshot; the old probe initialized ticket state and maintained a second routing implementation. Mode is in `data.run.mode`, with policy, artifacts and gates alongside it. |
| `bbs autopilot context` | `bbs autopilot recover --json` | No skill, hook or dashboard consumes the cursor/delta cache. Recovery supplies the bounded artifact view without writing a cache. |
| `bbs ticket get-pointer <key>` | `bbs ticket get pointers.<key>` | Same field read through the existing dotted-path accessor. |
| `bbs agent resolve` | Orca settings; `bbs agent detect` for the current harness | The resolver was already a failure-only retirement stub. |

The removed commands fail as unknown commands. Snapshot and recovery do not
choose a workflow for the model: workflow selection remains in the autopilot
skill. Existing checkpoint/attempt formats and QA/release gates are unchanged.
Legacy root aliases (`env`, `slug`, `qa-config`, `workspace`, `update-check`)
remain for older installed packs and the frozen differential fixtures; `env`
and `config` argv0 aliases also remain part of the Homebrew install.
