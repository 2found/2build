# Changelog

## 1.108.1 — 2026-10-25

### Added

- **`todo.md`** — backlog for re-measuring Foreman token cost after the
  prompt-cache changes, with the baseline pointer and expected metrics.


## 1.108.0 — 2026-10-25

### Added

- **Prompt-cache-aware Foreman waits** — new `foreman_cache_ttl` config
  (auto: claude 3600s, others 300s). `bbs foreman task ask`/await blocking
  returns land a minute inside the agent's prompt-cache lifetime so the
  follow-up run reads a warm cache; the watcher keeps an idle Foreman's
  cache warm through long batches. `foreman_status_interval` default drops
  to 3300s to stay under the one-hour cache window.
- **Cache-warmth internals** — `agent.PromptCacheTTL()` per-agent profile
  field and `promptCacheTTL`/`blockingWaitSeconds` helpers; the batch
  watcher sends a keep-warm nudge before the cache expires, and
  `scripts/foreman-cache-report.py` reports cache economics.
- **`token-audit` skill** — audits token spend against the foreman cache
  baseline (`docs/data/foreman-cache-baseline-2026-10-25.json`), shipped
  in the 2build pack with `scripts/foreman-cache-report.py`.
- **Docs** — `docs/foreman-cache-economics.md` and
  `docs/token-economics.md`.

### Changed

- **autopilot verification, review-pr, plan-draft, foreman skills** —
  routing, phases, tickets, topology and worker-lifecycle references
  refreshed for the cache-aware lifecycle.

## 1.107.0 — 2026-10-25

### Added

- **QA executor mode** — autopilot's `qa` gate now keeps judgment in the
  session (matrix, triage, fixes, verdict) and hands the mechanical runs —
  browser driving, checks, screenshots, logs — to one mid-tier executor
  subagent that reports evidence back. `packs/2build/qa/skill.md` gains an
  Executor mode contract; autopilot's Current-session gates describe the split.
- **`bbs foreman task ask` / `task done`** — workers report to Foreman through
  `bbs foreman task` instead of raw Orca messages; autopilot's Foreman ticket
  assignment and skill docs updated.
- **`foreman.ErrTaskNotFound`** — `LoadTask` wraps it so callers asking "is a
  worker running for this task" read a missing record as "not dispatched yet".

### Changed

- **Dispatch launch mode renamed** `worker-start` → `agent-launch` in
  Foreman routing evidence; docs and skill references follow the Foreman
  Dispatch naming throughout.

### Fixed

- **Capacity reserved before a Task record exists no longer errors** —
  `taskDispatchState` treats a missing record as "no Dispatch yet".

## 1.106.0 — 2026-10-25

### Fixed

- **Foreman rebinds a dead terminal on batch handover** — when a batch is handed
  to a new coordinator whose recorded terminal no longer exists, Foreman rebinds
  to a live one instead of stalling, and the watcher reports `ORPHANED-BATCH`
  when it finds a batch with no live owner.
- **A settled worker whose terminal was lost in an Orca restart counts as cleaned
  up** — Orca reports such a dispatch as `release_unknown`, which made finish
  readiness refuse forever. A completed or failed dispatch has nothing left to
  run, so it now counts as exited like retained/released.

## 1.105.0 — 2026-10-24

### Changed

- **Foreman closes a worker's terminal as soon as its Dispatch settles** — the
  batch watcher now shuts down the worker's agent terminal at done instead of
  leaving it idle. Follow-ups go to a fresh session; `worker-retain` is for
  debugging only. Set `foreman_keep_settled_workers: true` to keep the old
  behaviour (default false). Foreman skill docs describe the new rule.

## 1.104.0 — 2026-10-24

### Added

- **Proportional gates** — a ticket whose `review-pr` or `qa` gate is missing or
  stale can be sealed with `bbs foreman seal <ticket>... --waive <review-pr|qa>[,...]
  --reason "<text>"`; the waiver is recorded and shown as a note, never a blocker.
- **`bbs foreman evidence <parent> --phase <id> --carry --from <attempt-id>[,...]
  --reason "<text>" [--except <AC-ids>]`** carries a phase's passed checks forward
  to a re-integrated phase subject. Phase readiness now counts the union of current
  passed records. Autopilot and Foreman skill docs describe proportional gates.

## 1.103.1 — 2026-10-24

### Fixed

- **Foreman woke on every worker heartbeat** — Orca's injected preamble tells
  each supervised worker to send a `heartbeat` every 5 minutes to the Run
  mailbox, and the Foreman worker transport told workers to follow it, so Orca
  typed a mail pointer into Foreman for each one and every heartbeat cost a
  full Foreman wake. The worker transport now forbids `heartbeat`/`status`
  mail under Foreman: liveness is the bbs watcher's, progress goes to
  `bbs foreman progress`, and a worker reaches Foreman only through `ask` for
  a decision outside the accepted plan, `escalation` for a blocker, and its one
  `worker_done`, which `bbs foreman batch` collects into one wake per batch.
  A stray mail pointer mid-batch gets `batch show` only — no `check`, no ack,
  since acking re-arms Orca's pointer.

## 1.103.0 — 2026-10-23

### Changed

- **`bbs foreman land` closes the phase's worker agents** — the Orca
  terminals in the worktrees it removes (ticket sessions, phase QA worker) are
  closed before removal and listed as `closed_terminals`, so claude/codex/omp
  sessions kept for repairs stop holding memory once the phase lands. Kept
  worktrees and the caller's own terminal are untouched; without Orca it is a
  no-op.

## 1.102.3 — 2026-10-21

### Fixed

- **`docs/install.md` was one unclosed code block** — the 1.101.0 edit that
  added `bbs install omp` dropped the fence closing the harness command block,
  so GitHub and the 2build site rendered everything after it as source text and
  the page lost its heading anchors. The fence is restored; the docs artifact
  is rebuilt.

## 1.102.2 — 2026-10-21

### Fixed

- **Provider-mismatch error named the wrong providers** — `agent.ValidateSettings`
  printed the agent's native provider twice and dropped the caller's request
  (`provider "xai"; its native provider is anthropic` for a grok profile asked
  for anthropic). The message now names what was requested and what the agent
  supports, in that order.

## 1.102.1 — 2026-10-21

### Fixed

- **`--help` ran the command or errored** — `bbs foreman <verb> --help` answered
  `--help needs a value` (exit 1) because the verb flags are hand-parsed, and
  `bbs ticket <sub> --help` ignored the flag and executed the subcommand: a
  help probe on `ensure` created a ticket, and one on `approval` published an
  approval a Foreman then had to drop. Both dispatchers now answer help before
  any verb runs.

- **OMP `normal` workers launched dead** — the `flash` tier bound OMP to
  `@normal`, which is not one of omp's role aliases (`@smol`, `@slow`,
  `@plan`). Every normal-complexity ticket worker printed
  `Model "@normal" not found` and exited with `No model selected`. The `flash`
  binding for OMP is now empty, so `--model` is omitted and omp uses its own
  configured default; `agent.ValidateSettings` rejects any other unknown
  `@role` before the terminal is created instead of letting the pane die.

- **`bbs foreman spawn --dir` through a symlink** — Orca keys a worktree by the
  path it indexed, so `repo add` registered `/private/tmp/x` while
  `terminal create --worktree path:/tmp/x` selected nothing and answered
  `selector_not_found`. `orca.Create` now resolves the directory before
  registering and selecting it.

- **The tool floor blocked `git checkout`** — `checkout` alone matched the
  floor's payment vocabulary, so a Foreman (or worker) reading a git command
  was refused. The phrase now needs payment context (`checkout page`, `flow`,
  `form`, …), the same shape as the other floor entries.

## 1.102.0 — 2026-10-20

### Changed

- **Explicit Foreman worker harnesses** — `bbs foreman spawn` / `adopt`
  take `--plan-worker`, `--implement-worker`, and `--qa-worker`
  (`claude|codex|omp|grok|cursor`). Omitted means this Foreman's own
  harness. Quota-driven OMP offload (`bbs foreman model --quota`,
  `foreman.models.offload`, usage > 70%) is removed; leftover `offload`
  settings are ignored.

## 1.101.0 — 2026-10-20

### Added

- **`bbs install omp`** — OMP is a first-class plugin harness. `package.json`
  declares `hooks/omp.ts` as an extension and `skills/` symlinks the canonical
  pack so [omp plugins](https://omp.sh/docs/plugins) load both. Auto-detect
  includes `omp` on PATH; `bbs update` refreshes an installed `bbs@babysit`
  marketplace plugin. Invoke `/autopilot` (no `bbs:` prefix).

## 1.100.0 — 2026-10-18

### Changed

- **Flash-first Foreman ticket routing** — ticket workers on `simple` and
  `normal` tasks now run on the `flash` tier (Claude `sonnet`, Codex
  `gpt-6.1-sol`, OMP `@normal`); only `hard` tickets reach `max`. Codex `flash`
  now binds `gpt-6.1-sol` (was `gpt-6-luna`, which stays on `lite`), and OMP
  `lite` binds `@smol`. Codex integration, QA and release work on `normal`
  tasks therefore also moves from luna to sol; override `tiers.flash.codex` to
  keep luna.
- **Strict `hard` rubric** — complexity is classified from the ticket's own
  diff, not its project or domain. `bbs foreman route --complexity hard` now
  requires `--hard-reason money|auth|security|live-data|concurrency` and
  rejects anything else; the reason is persisted with the route. Exact-session
  resumes of recorded routes are unaffected. Expect about one hard ticket in seven.

### Added

- **`design` phase class** — basic design (architecture, prototype, phase
  plan) routes on its own class: `max` for `normal`/`hard`, `pro` for `simple`.
  Overriding `routing.normal.critical` no longer moves project planning.
- **OMP quota offload** — `bbs foreman model --quota [--host <id>]` reads the
  destination host's Orca quota snapshots and moves a new `lite`/`flash`
  Claude or Codex worker to OMP when the authoritative weekly window is above
  70%. Unknown quota never offloads; `pro`/`max` work stays put. Configure with
  `foreman.models.offload` (`agent`, `from`, `tiers`, `weeklyUsedPercent`);
  `"agent": ""` disables it.

## 1.99.1 — 2026-10-17

### Changed

- **Cheaper Claude Foreman tiers** — the default `pro` tier for Claude is now
  `opus` at `medium` effort (was `high`), and `max` is `opus` at `high`
  (was `max`). Codex and omp bindings are unchanged. Override per tier with
  `foreman.models.tiers` in `settings.json`.
- **Browser hygiene** — `browse` gains a section: use the automation's own
  browser, quit it with its own close (never `kill`), and check what still uses
  a profile before reaping it. `setup-project` writes the pointer bullet and
  `harness-audit` checks for it.

## 1.99.0 — 2026-10-17

### Added

- **Phase-based Foreman projects** — Foreman splits a project into release
  phases. Each ticket runs its whole autopilot loop (plan, approve, implement,
  review, QA) in one worker session, and Foreman wakes once per phase batch.
  Plans sized S/M are self-approved by the worker; L plans are approved by Foreman.
- **One git flow, no configuration** — autopilot never commits, pushes or opens
  a PR. `bbs foreman seal` snapshots each ticket on `refs/bbs/tickets/<id>`.
  `bbs foreman await` and `bbs ticket adopt` carry sealed prerequisites into a ticket.
  `bbs foreman integrate` builds one commit per phase.
  `bbs foreman land` fast-forwards local base to the QA'd commit. It never
  stashes or resets, and it refuses when base moved or local edits overlap.
- **Advisor toolkit** — `bbs contract list|pin|status|check` versions interface
  contracts by content digest. `bbs foreman next` suggests the next steps.
  `batch show` lists each owned project's `NEXT=` steps.
  The watcher coalesces early attention (`--attention-window`).
  `bbs foreman seal` accepts several tickets.

### Changed

- **Ticket contract v2 only** — v1 checkpoints are no longer read.
  `recover` refuses one with `CHECKPOINT_V1` (exit 3), and `snapshot` marks it
  with a checkpoint-refresh obligation. Docs, hooks and READMEs (including
  translations) were swept to match HEAD.
- **The watcher no longer compacts Foreman** at batch boundaries; the agent
  compacts itself when its context fills.

### Removed

- Git-flow profiles and `BBS_FINISH` / `RIGOR` / `REVIEW_EFFORT`.
- `bbs ticket land` / `refresh` / `serve`, `surface compose` / `revert` / `clear`
  and branch mode. Use the phase flow above instead.

### Fixed

- Watcher messages are sent from the Foreman's terminal (`--from`).
  Previously the launchd watcher's batch holds and wake messages failed with
  `no_active_sender_terminal`, so Orca typed a mail pointer into Foreman for
  every worker report.
- The watcher reaches Foreman by its recorded terminal handle, because Claude
  Code retitles its terminal. It no longer reports a live worker whose terminal
  the user took over as lost.
- `bbs foreman land` resolves a submodule's working tree, so a clean
  submodule checkout is no longer refused with "local changes overlap".
- Finish readiness no longer reopens a landed child's seal when its phase
  rewrote a shared workflow file or repaired a dependency.
- `bbs foreman next` waits on a ticket whose worker holds a lease, instead of
  suggesting a duplicate dispatch. Worktree manifests record an absolute
  canonical path, and `slug` honors `BABYSIT_HOME`.

## 1.98.1 — 2026-10-16

### Fixed

- **Fix misleading shared-reference links in `autopilot`** — link labels
  in `autopilot/SKILL.md` for `finding-unknowns` and `archetypes` no longer
  prefix the label with `references/`, preventing models from attempting to
  read non-existent skill-local files under `autopilot/references/`.

## 1.98.0 — 2026-10-13

### Added

- **`setup-project` seeds Claude Code subagent defaults** — when Claude Code is
  a harness, setup merges `env.CLAUDE_CODE_SUBAGENT_MODEL: "sonnet"` and
  `subagentPromptCacheTtl: "1h"` into the project's committed
  `.claude/settings.json` if absent, so subagents run on Sonnet and keep their
  prompt cache warm across long tool waits. Existing values and every other key
  are preserved; `settings.local.json` and `~/.claude/` are never written.
- **Search rule in the 2build pointer section** — the `## 2build` template that
  setup writes into `AGENTS.md`/`CLAUDE.md` now tells agents to use `rg` instead
  of `grep`, with `rg -uu` to include gitignored and hidden files.

### Changed

- **Release no longer notifies the 2found website** — `release.yml` drops the
  repository-dispatch step; the website is synced and deployed explicitly from
  a local `2found.dev` checkout. `RELEASE.md` and `docs/install.md` updated.

## 1.97.0 — 2026-10-12

### Added

- **`bbs foreman batch`** — Foreman hands a batch of launched Dispatches to the
  detached watcher and ends its turn instead of waiting. The watcher nudges a
  worker whose pane sits idle with its Dispatch open (`--worker-idle`,
  `--worker-nudges`), holds the Foreman's own status prompts, compacts the
  session at the batch boundary, and wakes the Foreman once — Orca message plus
  pane prompt — when every member settles, or early for a question, an exited
  worker or a stall. `bbs foreman batch show <id>` prints the batch state.
  While a batch runs it keeps the Foreman's Orca Delivery unacknowledged, which
  stops Orca typing a mail pointer into the Foreman for every `worker_done`.
- **`lite` model tier** — simple tasks in normal phases route to it; Claude
  routing is now haiku (lite), sonnet (flash), opus (pro), opus at max effort
  (max).
- **`release-prd` skill** — per-repo production delivery from version/changelog
  and main sync through GitHub/package distribution, app deployment, hypercare
  and local CLI updates. `--local` runs the full delivery from the operator's
  machine when CI quota is exhausted, with workflow suspension/restoration;
  `--dry-run`, `--prepare` and `--resume` support planning and recovery.
- **2build release profile** — normal pipeline ownership and equivalent local
  CLI/pack/docs/formula delivery, with post-release install verification.

### Changed

- **Foreman parallelism** — the default worker cap is `min(16, max(2, CPUs-2))`
  (`parallel_max_workers: auto`), a standard worker costs one resource unit,
  and simulator/local-ML checks are deferred and merged into one consolidated
  heavy QA per profile so they never displace light parallel work.
- **Batch supervision of native-terminal workers** — context-only Dispatches
  (OMP) report a retained terminal while working; they are now nudged too,
  while released or user-taken-over terminals are left alone.
- **`setup-project` and `harness-audit`** — setup prepares and links a release
  profile from repository facts; audit checks release-stage coverage, command
  accuracy, local execution, workflow restoration and post-release evidence.
  Existing inline/runbook profiles remain valid without a new filename.

## 1.96.0 — 2026-10-09

### Added

- **`test` skill** — behavioral test creation and suite optimization, with an
  `optimize` reference for shrinking weak suites.
- **`harness-audit` skill** — split out of `setup-project`: audits the coding
  harness itself, while `setup-project` keeps configuring a repository for
  babysit and points at project architecture and applicable design/deploy
  links.

### Removed

- **`bbs design` and `internal/design`** — unused compiled CLI plus the bundled
  design CSV/template guidance; `design-ui` now works from project context and
  an iterating spec instead.
- **`reason`, `social-content`, `office-hours`, `conversion-fix`,
  `copy-rewrite` and `growth-experiment` skills** — generic shortcuts removed;
  `triage` picks up the routing contracts that mattered.

### Changed

- **`plan-draft`, `implement` and `qa`** — reviewable plans are preserved and
  each skill records acceptance evidence; workflow contracts tightened.
- **Release pipeline** — the release workflow reuses the main test
  verification once and publishes the verified tag with generated notes,
  `DOWNLOADS.md`, `release.json` and checksum assets.

### Fixed

- **2build pack** — `design-ui` resolves its design data from the installed
  Soot pack instead of a checkout-relative path.

## 1.95.0 — 2026-10-08

### Added

- **Project starters** — `bbs starter` scaffolds a new project from the
  starter catalog; onboarding docs point at the stable release.

## 1.94.3 — 2026-10-08

### Changed

- **Rebranded to `2found/2build`** — repository moved under the 2found org;
  READMEs rewritten around the 2build name in all five languages with the new
  mascot.

## 1.93.7 — 2026-09-29

### Changed

- **The `bin/` directory is gone** — the last shell scripts moved into the
  Go multicall binary: `bin/setup-skills` → `bbs setup` (install/uninstall/
  build/relink, plus the pre-commit hook), `bin/bbs-ticket-lint` →
  `bbs ticket lint`, `bin/hooks/pre-commit` → `bbs hooks pre-commit`, and
  `bin/bbs-ticket-test` → the Go e2e suite in
  `internal/cmd/ticket_path_e2e_test.go`. `bin/lib/lock.sh` moved to
  `tests/fixtures/lib/` next to the bash oracle that still sources it. The
  compiled binary now lives at the checkout root (`./bbs`, still gitignored);
  `go build -o bbs ./cmd/bbs` replaces `bin/setup-skills` for a plain build.
- **`bbs update` runs setup in-process** — the checkout path no longer execs
  `bin/setup-skills`; the relink step is the compiled `setup` with stdout
  discarded, so upgrading on Windows no longer needs bash.

## 1.94.2 — 2026-10-06

### Changed

- **Docs/READMEs name all three OSes** — install instructions now state
  Homebrew covers macOS and Linux, the per-arch tarball covers Linux, and
  Windows publishes no binary (WSL/Git-Bash or `go run ./cmd/bbs setup`).
- **2build pack ships a `bbs` prerequisite** — `packs/2build` README gains a
  `## Prerequisites` section and the same note is injected into every packed
  `skill.md`, since soot inlines the skill body but never resolves the
  `../references/` links.
- **Unshipped stale one-shot docs** — removed `docs/autopilot-design/`, the
  `orca-agent-ownership` planning docs, and investigation/spike write-ups with
  no inbound references.


## 1.94.1 — 2026-10-06

### Fixed

- `bbs update` refreshes the Homebrew tap (`brew update`) before upgrading a
  coexisting or primary keg. `brew upgrade` never fetches the formula itself,
  so a same-day release previously installed the previous version.


## 1.94.0 — 2026-10-06

### Added

- **`bbs install [claude|codex|antigravity]`** installs the skill pack into
  detected harnesses (Claude Code, Codex, Antigravity). `bbs setup` now points
  at this command for published skills, and still prints the local-checkout
  marketplace registration path for development.

### Fixed

- Auto-detect no longer treats leftover `~/.claude` or `~/.gemini/config` as an
  installed harness, so `bbs setup` followed by `bbs install` on a Codex-only
  machine does not fail looking for the Claude CLI.
- `bbs update` still refreshes plugins and clears upgrade markers when a
  coexisting Homebrew copy or Antigravity plugin fails, and upgrades a Homebrew
  keg even when the running binary is a tarball/manual install.
- Runtime command failures no longer append `Run 'bbs --help'`.

## 1.93.12 — 2026-10-05

### Fixed

- Foreman preserves selected OMP models and role aliases through native idle
  terminal startup followed by Orca-supervised terminal reuse, instead of sending
  unsupported OMP model/effort overrides to `worker-start`.
- Launch verification consumes Orca's current `dispatchId` and
  `launch.effective.agent` receipt fields, rejects unready/unaccepted launches,
  and distinguishes exact-terminal transport proof from unknown native settings.

## 1.93.1 — 2026-09-24

### Added

- **Ticket DAG relationship mutations** — `bbs ticket add-child` and
  `add-relation` persist parent membership and dependency edges through locked
  ticket mutations. `remove-relation` removes a specific edge, allowing Foreman
  to update the dependency graph without editing `index.json` directly.

### Fixed

- Relationship commands reject malformed ticket indexes without overwriting
  them and preserve `duplicate_of null` as the clear sentinel.

## 1.56.0 — 2026-07-29

### Changed

- **Skills call the Go CLI as `bbs <sub>`** — every skill, workflow, and
  reference now invokes the multicall binary in its space form instead of a
  hyphenated `bbs-<sub>` alias. Only `bbs-config` and `bbs-env` ship with
  `brew install bbs`, so alias-form calls were broken for brew-only installs;
  the space form works on every install shape.
- **Preamble bootstrap collapsed to one guarantee** — the `_bbs_resolve`
  helper and its twelve `BBS_*_BIN` exports are gone (no skill body ever read
  them). In their place the preamble prepends the absolute install dirs to
  `PATH` and probes once with `bbs ticket --help`, emitting `BBS_DEGRADED` on
  stderr when no working binary is reachable. Measured ~0.025s per skill
  invocation against 0.117–0.291s for the old resolver, and 52 lines of shell
  removed. Probing rather than path-checking is deliberate: `root.go` sets
  cobra's `SilenceErrors`, so a binary predating a subcommand exits 1 silently,
  indistinguishable from a legitimate "no ticket" exit.
- **Help text follows the invocation** — usage and diagnostic strings now
  render `bbs ticket` when called as `bbs`, and `bbs-ticket` when called
  through a compat symlink, so help never hands a brew-only user a command
  that is not on their `PATH`. The symlink path stays byte-identical, which is
  what keeps the differential harnesses diffing cleanly against the frozen
  bash oracles in `tests/fixtures/`.

### Added

- **`setup-skills` puts `bbs` on your `PATH`** — the installer now builds the
  binary and symlinks `~/.local/bin/bbs`, warning when that directory is not on
  `PATH`. It also removes the pre-Go bun shim left at that path by older
  installs. Previously `bbs` landed only in `~/.claude/`, which is not a `PATH`
  directory, so a bare `bbs` in a shell worked only for users who had added it
  by hand.
- **`tests/test_preamble_bin_reachability.sh`** — runs the extracted preamble
  under `env -i` across a bash×zsh matrix covering the three install shapes and
  two failure modes, including a stale binary that exits 1 silently (a path
  check alone reports that one healthy). Replaces
  `test_preamble_bbs_resolve.sh`. Its repo-wide guard fails the build if a
  skill or doc reintroduces an alias-form call.

### Fixed

- **`update-check` and `upgrade` were dead on every real install** —
  `babysitDir()` walked up from `argv[0]` without resolving symlinks, but no
  install invokes `<checkout>/bin/bbs` directly: `setup-skills` links
  `~/.local/bin/bbs` and `~/.claude/bbs` at it and the preamble puts both on
  `PATH`. The lexical parent of whichever won was `~/.local` or `$HOME`,
  neither holding a `VERSION` — so `update-check` exited 0 silently (the
  preamble's upgrade notification never fired, and `auto_upgrade: true` did
  nothing) and `upgrade` told a healthy clone it was "not installed via git
  clone". `babysitDir()` now calls `EvalSymlinks`. A Homebrew install still
  resolves outside a checkout, which is correct: there is no clone to pull.
  This was inherited from the bash original (`$0` without `readlink -f`) and
  ported faithfully, so it is the one place the port now deliberately diverges
  from the retired oracle — the three `tests/test_bbs_update_check.sh` cases
  that pinned the bug became absolute `g0` assertions, joined by a new
  `TestBabysitDir`. It survived this long because every existing case in
  `test_bbs_update_check.sh` and `test_bbs_upgrade.sh` pins `BABYSIT_DIR` for
  sandbox safety, short-circuiting the `argv[0]` path that real installs take.
- **`--uninstall` left `bbs-analytics-cron` behind** — the bin was in the
  install list but missing from the uninstall list, so it survived as a
  dangling symlink.
- **Docs described commands that never existed** — `docs/operations.md`
  documented `bbs analytics` and `bbs doctor`, and `setup-skills --help`
  advertised a doctor step that `--full` never ran. Neither command was ever
  implemented in Go or bash; the docs now describe what actually exists.

## 1.55.0 — 2026-07-11

### Changed

- **Thin plan.md contract** — `plan-draft` now writes a **5–15 line** plan with
  inline bold labels (`**Goal:** / **Out of scope:** / **Approach:** /
  **Unknowns:** / **Verify:** / **Design:**`), replacing the old
  `## Goal/Scope/Approach/Reuse/Files/Verification/Risks/Next` headings. The
  plan carries only what a human must react to and what `implement` can't
  re-derive; task order, file lists, and step detail move to `implement`.
  - `implement` derives files and task order from the code (the plan no longer
    lists them) and reads reuse notes from the plan's `**Approach**`.
  - `qa` and the `builder` workflow read the plan's `**Verify:**` line (was
    `## Verification`).

### Added

- **PR reviewer quiz** — `create-pr` ends the PR body with 2–3 questions
  probing what the diff alone can't show (behavior riding on existing code
  paths, the consequence of a deviation, what else the change can reach), with
  answers collapsed in a `<details>` block so the reviewer self-checks before
  merging. `NEXT` reminds the human to pass the quiz first.
- **Token-skinned design prototypes** — `design-ui` mocks now inline the
  project's real tokens and imitate inventory components copied from the
  nearest real screen, so the prototype looks like the product `implement`
  builds to. Every element maps to a named `DESIGN.md` component or is flagged
  `NEW:` with a one-clause why; free-form styling only for projects with no UI
  yet.
- **review-pr living rulebook** — the review's learned rules
  (`<repo>/.babysit/review-pr.md`) now dedup against paraphrases, score by
  hits/misses with recency decay, and compact/expire when the file grows,
  sharing Swatter's entry format so both books stay mutually readable.

## 1.54.0 — 2026-07-09

### Added

- **Native task list mirroring** — multi-step skills now mirror their driving
  artifact into Claude Code's native task list (`TaskCreate`/`TaskUpdate`) as
  the visible progress view, while disk artifacts stay the durable state. New
  §Native task list in `references/preamble.md` sets the contract: seed tasks
  from the skill's artifact (`plan.md`, the QA flow matrix, workflow
  milestones), mark each `in_progress` on start and `completed` only when its
  check passes, and on cold resume rebuild the list from disk — never the
  reverse.
  - `autopilot` mirrors workflow milestones at loop entry (rebuilt from
    checkpoint + `plan.md` on cold re-entry); step skills add finer tasks to
    the same list.
  - `implement` derives its task list from `plan.md` (one task per verifiable
    unit) instead of an ad-hoc list beside the plan; deviations update both
    the task list and `## Deviations`.
  - `qa` mirrors its flow matrix one task per case, closed only when evidence
    lands.
- **Native plan mode handoff** — `plan-draft` in a developer session already
  in plan mode presents the finished draft through `ExitPlanMode` (native
  approval is the "plan accepted" checkpoint) and defers `plan.md` /
  `set-pointer` writes until after approval. Unattended runs never enter plan
  mode; `plan.md` on disk remains the accepted plan.

## 1.53.0 — 2026-07-09

### Changed

- **Builder bootstrap gate** — an unconfigured repo
  (`state_repo_configured=0`) no longer dead-ends in `NEEDS_CONTEXT`
  pointing at the developer-only `setup-project`. The builder workflow now
  seeds `.babysit/git-flow.yaml` with detected defaults (base from
  `origin/HEAD` → local `main`/`master` → current branch; `push` only when
  a remote exists; `mode: branch`) and keeps going, recommending
  `/bbs:setup-project` in the handoff for the QA harness only.
  `test_autopilot_readiness_gate.sh` now executes the seed block.
- **Git is autopilot's job, end to end** — step skills are infra-isolated:
  `implement` and `qa` never branch, commit, or push (QA fixes edit the
  checkout; the workflow commits and lands them). Autopilot init `git
  init`s a bare folder, and the builder workflow commits each skill's
  output itself. `CLAUDE.md` over-strict pattern #5 extended to cover all
  git mutations.
- **Plain-language guidance for humans** — with `INVOKER=developer`,
  autopilot leads every stop (handoff, `NEEDS_CONTEXT`, final status) with
  one plain sentence plus the exact next command to paste, so a
  non-technical user can drive a build end to end without knowing git.

## 1.52.1 — 2026-07-08

### Changed

- **Skill/reference condensation** — `autopilot` SKILL + `builder` workflow and
  the shared references (`archetypes`, `auto-decision-framework`, `git-flow`,
  `handoff-contracts`, `preamble`, `ticket-layout`, `ticket-size-rubric`)
  rewritten much shorter (net ≈ −850 lines) with the same contracts: gates,
  statuses, and file schemas are unchanged; prose and examples trimmed.
- New over-strict pattern #5 in `CLAUDE.md` (git-flow protocol belongs to
  workflows, not skills); `qa` operates on the current checkout as-is.

### Added

- **finding-unknowns reference** (`references/finding-unknowns.md`) — deriving
  unstated requirements from code and git history; wired into `plan-draft`,
  `investigate`, and `qa`. Companion blog posts under `blogs/`.

### Fixed

- **Inline `# comments` in YAML values** — `bbs-ticket` (git-flow `mode:`),
  `bbs-autopilot` (`base_branch:` / `branches.develop:`), and all
  `bbs-qa-config` scalar parsers (including the `credentials:` block, whose
  documented template carries an inline comment) now strip trailing comments
  before validating values. Covered by `test_bbs_ticket_git_flow_mode.sh` and
  `test_qa_config_loader.sh`.
- **qa/browse credential fallback drift** — `qa`'s snippet now falls back to
  the standard `QA_USER` / `QA_PASS` names like `browse`, instead of clobbering
  the values `bbs-secrets load` just exported.

## 1.51.2 — 2026-07-06

### Added

- **Standard QA credentials** — optional `credentials:` block in the qa.yaml
  simple shape naming the standard login env vars (`QA_USER` / `QA_PASS`), whose
  values live in the gitignored `.babysit/.env`. `bbs-qa-config probe` surfaces
  them as `QA_ENV_{USERNAME,PASSWORD}_ENV`; the `qa` and `browse` skills resolve
  login creds via `bbs-secrets load` and `BLOCK` (naming `.babysit/.env`) when a
  required credential resolves empty; `setup-project` seeds placeholders with
  `bbs-secrets seed`. Covered by `test_qa_config_loader.sh`.

## 1.51.0 — 2026-07-04

### Removed

- **Product mode** — deleted `bin/bbs-product`, `qa-product-mode.md`, the
  product-folder docs, `setup-workspace` templates, and the product-mode test
  suite (`test_bbs_product_*`, `test_qa_merge_*`, `test_qa_deploy_*`, the
  concurrency harness). Multi-repo work now runs through the trunk-worktree /
  merge-mode flow.
- **`babysitter` submodule** and its `.gitmodules` entry.

### Added

- **Merge mode / git-flow** — `bbs-ticket` and `bbs-slug` reworked around
  `bbs-ticket merge-base` plus cross-repo dispatch (`RELATED_*_REPO`), replacing
  product mode. New coverage: `test_bbs_ticket_git_flow_mode.sh`,
  `test_bbs_ticket_merge_base.sh`, `test_bbs_ticket_switch.sh`.

### Changed

- **review-pr** moves to a multi-agent finder/validator model; `qa` and
  `browse` SKILL docs updated; added `blogs/review-pr.md`.
