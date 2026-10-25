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
base-ops (`surface` lease, `adopt`),
`ensure`, and `path`/`list`/`reconcile`. No bash remains in
production; a frozen byte-identical copy of the old script lives at
`tests/fixtures/bbs-ticket.reference` purely as the differential oracle.

| Command | Purpose |
|---------|---------|
| `bbs bootstrap` / `bbs starter check` | Create a verified project from a versioned starter; report applicable releases from committed provenance without changing application files. [Starter commands and update contract](starters.md). |
| `bbs autopilot` | `snapshot --json` reads canonical state, mode, policy and gate evidence; `recover --json` adds bounded artifact excerpts for recovery. `checkpoint`, `attempt` and `verification` persist execution state; `clear`, `base-branch` and `lint-workflow` support lifecycle. |
| `bbs ticket` | Ticket-layout broker and state-probe surface. `env` derives `SLUG`/`BRANCH`/`TICKET`/`BABYSIT_PROJECT_HOME` through the identity ladder — `BABYSIT_TICKET` env → `manifest.yaml` cwd-match → branch regex — which is what every skill preamble evals and what autopilot resume relies on; `path <kind>` resolves Layout C file paths; `verdict-status --skill <n>` reads the latest verdict for a sub-skill (used by autopilot's Probe and Verify-post) |
| `bbs config` | `get` / `set` / `list` plus `workspace` operations, all in `~/.babysit/config.yaml` |
| `bbs semantic-decision` | Bounded judgment step: `--input <request.json> --kind <kind>` routes to the current LLM by default or configured Cloudflare; `--llm-answers <answers.json>` records the LLM decision. [Configuration and human-review policy](operations.md#settings-without-a-dashboard-control). |
| `bbs foreman model` | Prints the effective model policy as JSON; add `--agent codex --complexity normal --phase-class critical` to select a model/effort. Pin worker harnesses with `bbs foreman spawn --plan-worker|--implement-worker|--qa-worker` (default: the Foreman's own agent). Defaults are built into bbs; override individual fields in `~/.babysit/settings.json` or `<repo>/.babysit/settings.json` (repo wins). `--dir` selects the assignment repo/worktree. See [model settings](../.claude/skills/foreman/references/model-routing.md#model-tiers). |
| `bbs foreman task create\|launch\|show\|list\|done\|ask\|answer\|abandon` | The Foreman worker lifecycle, owned by bbs on disk (`$BABYSIT_HOME/tasks/<task>/`): `create` stores a Task spec, `launch` opens a Dispatch and starts its agent through `orca agent launch --surface terminal` (or into `--terminal`) and records verified launch evidence; the worker reports once with `done` and asks blocking questions with `ask` (exit 4 = still pending, `--resume <id>`), which Foreman answers with `answer`. No Orca orchestration, mailbox or heartbeat. [Worker lifecycle](../.claude/skills/foreman/references/worker-lifecycle.md). |
| `bbs foreman await --prereqs <ticket>:<task>,...` | Run by a Foreman ticket worker after its plan is approved: blocks until each prerequisite ticket's Foreman Dispatch has completed, its ticket passes readiness (review, or push when `pointers.deferred_heavy` is set) and it is sealed, then adopts each sealed snapshot into the worktree as uncommitted changes (`PREREQ=… SNAPSHOT=<sha>`). Exit 0 ready, 5 conflict (`CONFLICT=<file>` lines to resolve), 3 not-ready, 4 timeout (default 540 s; run it again). |
| `bbs ticket adopt <ticket>...` | Adopts sealed tickets' snapshots into the current worktree on demand (same three-way merge as `await`); never commits. |
| `bbs foreman seal <ticket>...` | Snapshots each verified ticket's uncommitted tree (one not ready does not hold back the rest) on the hidden ref `refs/bbs/tickets/<id>`; HEAD, index and branches are untouched. |
| `bbs foreman integrate <parent> --phase <id>` | Composes the phase's sealed snapshots into **one commit** on local base (base itself does not move) and checks it out detached in `<repo>/.babysit/worktrees/<parent>_phase-<id>` for phase QA. A conflict names the ticket and files to repair. |
| `bbs foreman land <parent> --phase <id>` | After phase QA passed on that exact commit, fast-forwards local base to it and removes the phase's worktrees. Refuses if base moved since `integrate`, or if a checkout of base has local edits to a file the phase changes; never stashes or resets. |
| `bbs foreman next <parent> [--json]` | Suggests what Foreman does now — publish or revise the design, bind or open a phase, dispatch, seal, refresh, integrate, verify, land, release, complete — each with its command, or who it is waiting on. Advice derived from the same snapshot and gates the verbs enforce; it changes nothing. |
| `bbs contract list\|pin\|status\|check` | Interface contracts: repo files indexed in `.babysit/contracts.yaml` (optional `owner`, consumers' `checks`), versioned by content digest. A ticket pins what it builds against; a changed pin makes its readiness stale (`contract:<id>:changed`); `integrate` refuses a phase that changes a contract another Foreman owns and records the changed ones for phase QA; `check` runs their checks. [Contracts](../.claude/skills/references/contracts.md). |
| `bbs foreman readiness <parent> --action dispatch\|integrate\|land\|release --phase <id> --json` | Gates each step of a phased project: opening phase `k` needs phase `k-1` landed; integrating, landing or releasing it needs every earlier phase released and no failed release upstream. Prints `ready` and reasons; a zero exit only means the read succeeded. |
| `bbs foreman release <parent> --phase <id> --request` / `--file receipt.json` | `--request` publishes the phase's human release approval (never self-resolvable); `--file` records a `release-prd` verdict (`RELEASED`, `ROLLED_BACK`, `PARTIAL`) after checking the released SHA contains every ticket of the phase and earlier ones. |
| `bbs foreman supervisor status\|install\|uninstall` | The launchd/systemd unit that keeps `bbs foreman watch` alive while a batch is open; `bbs foreman batch` installs it. `BBS_WATCH_SUPERVISOR=off` disables it. |
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
