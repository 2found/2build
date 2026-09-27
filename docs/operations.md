# Operations

Day-2 configuration, telemetry, and upgrade handling.

## Configuration

```bash
bbs config set telemetry local       # off | local
bbs config set update_check true     # false silences upgrade notifications
bbs config set auto_upgrade false    # true runs bbs update on session start
bbs config set proactive true        # false = only run skills typed explicitly
bbs config set foreman_status_interval 3600  # seconds between foreman reconciliation ticks
bbs config set parallel_max_workers 4     # balanced laptop-safe ceiling
bbs config set parallel_global_units auto  # global weighted Foreman capacity
bbs config list                      # show all keys + annotated docs
```

### Machine-global worker admission

`parallel_max_workers` remains a per-Foreman ceiling. Its default of four keeps
headroom for the coordinator and OS while still exposing useful parallelism.
It cannot protect one machine running several Foremen: three coordinators with
a ceiling of four could otherwise launch 12 workers. Every Foreman therefore
also reserves from one atomic weighted pool under `~/.babysit/resources/`.

With `parallel_global_units: auto`, the pool uses the smaller of the
machine's CPUs and one unit per GiB after a 6 GiB OS reserve, with a minimum
of one unit. A configured positive value can lower, but not raise, that
host-derived budget. New work also queues while available memory is below 20%
or one-minute load reaches 80% of the machine's CPUs. Running workers are never
preempted.

| Profile | Units | Exclusive host resources |
|---------|------:|--------------------------|
| `plan` | 1 | — |
| `standard` | 2 | — |
| `android-simulator` | 4 | mobile simulator, GPU |
| `ios-simulator` | 4 | mobile simulator, GPU |
| `local-ml` | 4 | GPU |

Reservations are global across repositories and Foremen sharing the same
`BABYSIT_HOME`. They are keyed by Foreman and Orca Task, making a retry
idempotent. The broker enforces the per-Foreman worker ceiling atomically as
well as the global weighted budget. Both `reserve` and `status`, and the detached
watcher, recover leases across all owners without waiting for a dead Foreman:

- Terminal Dispatches release capacity immediately.
- Agents proven exited by Orca's fleet view are stopped by exact Dispatch id;
  capacity is reclaimed only after settlement is confirmed.
- Reservations with no new Dispatch are reclaimed after ten minutes if the
  owner's heartbeat is stale or its record is missing. Resume must heartbeat
  and repeat `reserve` immediately before launching, saving the returned id.
- Live workers survive owner interruption and laptop sleep. Unverifiable
  workers remain held, with `RESOURCE_HELD` diagnostics; contact loss alone
  cannot safely authorize another worker in their slot.

Recovery prints `RELEASED_LEASE` for each reclaimed slot. Reconciliation has a
15-second overall deadline and two-second per-command deadlines; Orca probes
never hold the admission lock. A persisted probe cursor rotates past slow
workers so they cannot starve later leases on every tick. The OS releases
that lock on process exit,
including SIGKILL. Replacement leases have new ids, protecting them from late
cleanup of the previous attempt. Old mkdir lock directories are ignored by the
new broker; do not run old and new broker binaries concurrently during upgrade.

```bash
bbs foreman resource status
bbs foreman resource reserve fm-project \
  --ticket bs-child --task orca-task-id --profile ios-simulator
bbs foreman resource release rsc-0123456789abcdef
```

`ADMISSION=queued` is backpressure, not a failed Task. Foreman may dispatch
other admitted work and retries the queued Task on its next reconcile tick.

### Project review and delivery evidence

Foreman prepares one parent plan, design/prototype and proposed ticket map
before creating child worktrees or dispatching production work. Review that
project checkpoint once; Foreman reviews the child plans against it. Product
scope/design changes return to the parent checkpoint. Approval is bound to
the artifact revision, so changed documents require a new review.

To delegate the human design reviews too, explicitly pass `--auto`:

```text
Claude Code  /bbs:foreman --auto <large project>
OMP          /foreman --auto <large project>
Codex        $bbs:foreman --auto <large project>
```

`bbs foreman spawn fm-project --auto` and direct adoption with `--auto` persist
the choice on that Foreman record across resumes. It still creates and
reviews the artifacts; hold/grant bounds, non-delegable decisions, code review,
QA, and `finish:` authorization remain in force. Existing records without
`auto: true` use human project review; child plan autonomy is unchanged.

`bbs foreman report <parent-ticket>` reads the last durable `report.md`, even
after Orca closes. Each reconciliation writes the observation time, meaningful
ticket titles, worker/phase, branch/head, gate evidence, PR/merge state and
cleanup/blockers. It is explicitly a saved snapshot; ask the running Foreman
to check status for a fresh reconciliation. Execution, delivery and cleanup
are separate: PR_READY does not mean merged, and LANDED_LOCAL does not mean
pushed. Missing evidence stays UNKNOWN.

Final project integration QA runs after finish handlers and before Foreman
reports done: on the actual landed `<base>` for `finish: land`, or a retained
`qa/<parent>` branch composed from verified heads for `pr`/`review`. It records
the tested branch/SHA and acceptance evidence on the parent. Independent
code-bearing tickets still require this final check. QA branch preparation
and restoration never reset local base; `surface compose/revert` are only
for the earlier scratch/per-ticket lifecycle. Failures return to child repair
and verification. The full protocol is in
[the Foreman project contract](../.claude/skills/foreman/references/project-contract.md).

### Which coding agent runs the work

Open `bbs dashboard` → **Settings** to configure machine-wide agent, provider,
model, and effort defaults separately for workers and foremen. Changes apply to
new runs. Agent choices show which CLIs are installed. Model and provider
identifiers are free text, so custom providers and new models do not need a
babysit update. Authentication stays in the agent's own config. Static
dashboard snapshots cannot edit settings.

`foreman` dispatches workers as coding-agent CLI sessions. Two keys in the
single `~/.babysit/config.yaml` file pick which CLI:

| Key | Selects | Default |
|-----|---------|---------|
| `worker_agent` | the per-ticket workers | `auto` |
| `foreman_agent` | the foreman session itself | `auto` |

Supported: `claude`, `omp`, `grok`, `codex`, `cursor`. `auto` detects the current
agent, then selects the first installed CLI in alphabetical order; with no
signal or installation it falls back to Claude and preflight reports the missing
binary. Explicit selections never fall back to another agent.

```bash
bbs agent detect --json                 # current harness and detection evidence
bbs agent list --json                   # supported CLIs and installed paths
bbs agent resolve --role worker --json  # resolved launch settings
bbs agent resolve --role foreman --dir /path/to/repo --json
```

Detection uses an explicit `BABYSIT_CURRENT_AGENT` marker, nearest recognized
parent process, then native session markers. It does not infer the current agent
from installed binaries, API keys, or config directories. OMP needs a recognized
`omp` parent process or an explicit current-agent marker when embedded behind an
unrecognized wrapper. `BABYSIT_AGENT` is a launch override, not runtime identity.

Selection precedence for each field is CLI flag, role-specific environment
(`BABYSIT_WORKER_AGENT` or `BABYSIT_FOREMAN_AGENT`), shared `BABYSIT_AGENT`,
repository config, global config, then detection. `claude-code`/`claude code`,
`cursor-agent`, and `oh-my-pi` are accepted aliases. Cursor launches through
`cursor-agent`; a generic `agent` binary is not assumed to be Cursor (Grok also
installs that name).
`foreman_agent` selects only a session created or recovered by `spawn`; a
directly invoked Foreman naturally runs in the CLI where its skill was invoked.

Direct skill invocation is the normal entrypoint:

```text
Claude Code  /bbs:foreman <large project>
OMP          /foreman <large project>
Codex        $bbs:foreman <large project>
```

The skill's first operation is `bbs foreman adopt <id>`. It detects the agent,
reads and renames the active Orca terminal, records the actual agent dialect,
and makes dashboard/watchdog wakes addressable. Re-adoption after compaction is
idempotent. It refuses to steal an id, terminal, or repo already bound to a
different Foreman. `bbs foreman spawn` is optional convenience and recovery,
not the required launch path. Direct invocation inherits the current CLI's
permission mode, so configure that session for unattended tool use before
leaving a multi-day run.

The two keys do not inherit from each other on purpose. A foreman reviews design
gates and QA evidence from its workers, so moving workers to another agent is a
throughput choice that must not silently relocate that audit — `worker_agent:
omp` alone leaves a directly invoked foreman in its current CLI and a managed
foreman on its independently selected `foreman_agent`.

Adding an agent is a registry entry in `internal/agent`, which owns the binary
name, the flag that suppresses tool approval, how (or whether) a conversation
can be given a durable handle, and how that agent namespaces babysit's skills.
Two things it does not own:

- **Skills must be reachable by that agent**, and a binary on PATH is not that.
  Each CLI has its own mechanism, and getting it wrong is silent — the worker
  launches fine and then cannot resolve its prompt:

  | agent | how it finds babysit's skills | prompt shape |
  |-------|-------------------------------|--------------|
  | `claude` | the plugin marketplace | `/bbs:autopilot` |
  | `grok` | `grok plugin install https://github.com/lohi-ai/babysit` | `/bbs:autopilot` |
  | `omp` | `omp config set skills.customDirectories '["$HOME/.claude/plugins/marketplaces/babysit/.claude/skills"]'` | `/autopilot` |
  | `codex` | `codex plugin marketplace add lohi-ai/babysit && codex plugin add bbs@babysit` | `$bbs:autopilot` |
  | `cursor` | make the skills available under `.cursor/skills` or `.agents/skills` | `/autopilot` |

  `bbs foreman worker-command` preflights the binary and names the per-agent
  fix; the install itself is on the operator. Pass `--skill autopilot` rather
  than writing the prompt by hand — **omp reaches its skills through a flat
  directory list, so they have no `bbs:` namespace**, while Codex uses a `$`
  sigil. A hard-coded `/bbs:autopilot` resolves incorrectly for both. (`omp plugin install <git-url>`
  looks like the fix and is not: it reports success under `--dry-run` and then
  fails for real, being an npm-shaped installer rather than a plugin store.)
  A flat list also bounds what `skill://` can address: one skill directory, no
  `..`. Anything a skill names outside its own directory — the shared
  `references/` one level up, a sibling skill, a pack-level doc — is silently
  retargeted inside the skill (`skill://qa/../references/worktrees.md` becomes
  `skill://qa/references/worktrees.md`) and dies as `File not found`. SKILL.md
  files therefore say to read those by path, and
  `tests/test_skill_reference_links.sh` guards both halves: that the targets
  resolve, and that every skill which names one says how to read it.
- **A foreman's session is pinned to the agent that minted it.** `spawn` records
  it and reuses it on resume, because a conversation handle means nothing to a
  different CLI. Changing `foreman_agent` takes effect on the next *new*
  foreman, not on a resume; `bbs foreman spawn <id> --agent <other>` refuses
  rather than guess.

  What the handle *is* depends on the agent, and only `claude` and `grok` can
  be told to use one we chose: they take `--session-id <uuid>`. `omp` has no
  such flag, so a foreman on omp gets a private session directory
  (`--session-dir`) and resumes with `--continue` — unambiguous because nothing
  else writes to that directory. `codex` and `cursor` have neither, so either closed foreman
  starts a fresh conversation and cold-resumes from ticket + Orca state. It
  deliberately does not use repo-wide `resume --last`: several foremen may
  share one repo, and "last" could attach the wrong project's goal. A uuid is
  never recorded against an agent that cannot be told to use it.

  For multi-day runs, schedule `bbs foreman ensure <id>` to recreate a missing
  Orca terminal. The watcher needs no scheduling: `adopt` and `spawn` start a
  detached `bbs foreman watch` automatically. The unscoped watcher takes one
  global flock, while a scoped `watch <id>` takes an id-specific flock, so
  repeat check-ins are no-ops without blocking other foremen. The watcher exits
  on its own once no foreman has an open Orca terminal. `bbs foreman watch <id>
  --once` remains available to refresh an idle one by hand. Both prompts reload
  the Foreman skill and carry `--foreman-id <id>` so compaction or a cold start
  cannot erase coordinator identity.

**grok needs the directory trusted first.** grok keeps a per-folder trust record
in `~/.grok/trusted_folders.toml`, and it is *separate* from `permission_mode` —
with `always-approve` set globally, a first run in an unlisted directory still
stops on "Do you trust the contents of this directory?", which `--always-approve`
does not answer. An unattended worker parked on that prompt reads as a hung
ticket. Both spawn paths preflight it and refuse with the fix named, so the
failure is loud instead of silent. Grant it once per repo:

```bash
cd <repo> && grok      # answer the trust prompt, then quit
```

Workers launch with `--cwd <repo>`, so this is one decision per repo, not per
worktree.

### Provider, model and effort

Each role has independent `*_provider`, `*_model`, and `*_effort` settings.
Empty settings preserve the CLI's native configuration. Babysit does not ship a
model catalog or choose a more expensive model from ticket difficulty.

```bash
bbs config set worker_agent omp
bbs config set worker_provider openai
bbs config set worker_model '<your-model-id>'
bbs config set worker_effort high
bbs config set foreman_agent codex
bbs config set foreman_model '<your-coordinator-model-id>'
bbs agent resolve --role worker --json
```

The CLI quotes values such as `@slow` correctly; quote them in hand-written
YAML too. For an OMP role bound to a provider, leave `worker_provider` empty
and set only `worker_model`, for example `@slow`. Clear a setting with
`bbs config set worker_model ''` to use native defaults when no higher-priority
setting applies.

All four fields follow the same precedence: explicit flag, role-specific
`BABYSIT_WORKER_*` / `BABYSIT_FOREMAN_*`, shared `BABYSIT_PROVIDER` /
`BABYSIT_MODEL` / `BABYSIT_EFFORT` / `BABYSIT_AGENT`, then
`~/.babysit/config.yaml`. `--dir` selects the launch directory; configuration
remains machine-wide.

| Agent | Provider setting | Model / effort |
|---|---|---|
| Claude Code | `anthropic`, `bedrock`, `vertex`, `foundry` via native environment selectors | `--model`, `--effort` |
| Codex | native provider identifier via `-c model_provider=…`; configure that provider in Codex first | `--model`, `-c model_reasoning_effort=…` |
| OMP | native `--provider` selector; alternatively use a `provider/model` model with provider unset | `--model`, `--thinking` |
| Grok | unset or `xai`; other providers are rejected | `--model`, `--reasoning-effort` |
| Cursor | unset or `cursor`; other providers are rejected | `--model`; separate effort is unsupported |

Provider credentials and endpoints remain in each agent's own configuration.
Model IDs stay opaque, so a newly available model needs no babysit release.
Unsupported provider/effort combinations fail before a terminal is created.

```bash
bbs foreman worker-command --agent omp --provider openai \
  --model '<your-model-id>' --effort high --skill autopilot --prompt 'Build the ticket'
bbs foreman spawn fm-demo --agent codex --model '<your-model-id>'
```

Managed foreman records pin the requested provider/model/effort alongside the
agent. Recovery reuses them even if configuration changes; contradictory
explicit flags fail. Empty recorded settings continue to use native CLI config,
so they cannot freeze an unobserved native default across later native changes.

Foreman skills read `bbs agent resolve` before supervised dispatch and persist
Plan/Build routes on each child. The external launcher must support carrying the
requested settings: check its live capabilities and `launch.effective` receipt.
In particular, do not invent an Orca `--provider` flag or assume an OMP model
flag is forwarded. A launcher that cannot honor explicit configuration is a
named dispatch blocker, never permission to silently use another model. Direct
`worker-command` and managed `spawn` render the native options themselves.
See [model routing](../.claude/skills/references/model-routing.md).

## Telemetry

Skill runs append JSON Lines to `~/.babysit/analytics/skill-usage.jsonl`. Because babysit runs unattended, telemetry is the *primary* feedback channel — treat it as load-bearing, not decoration. Local-only by default; nothing leaves the machine.

Nothing summarizes that file on demand — the reader is the `/bbs:analytics-review` skill. Dispatch it by hand (`/bbs:analytics-review`) when you want a report; to look at the raw rows, read the JSONL directly.

The Auto-Decision Framework's audit trail is the companion file, `~/.babysit/analytics/decisions.jsonl` — one line per Taste/Mechanical decision. `investigate` can read it for prior-learnings context; grep or `jq` it directly.

## Auto-update

`bbs update check` compares the local `VERSION` against `main` on GitHub, with cache-friendly TTLs (60 min when up-to-date, 12 h when an upgrade is pending). Typical preamble wiring:

```bash
UPD="$(bbs update check 2>/dev/null || true)"
case "$UPD" in
  "UPGRADE_AVAILABLE "*) echo "babysit update available — run bbs update";;
  "JUST_UPGRADED "*)     echo "babysit upgraded: $UPD";;
esac
```

Snooze a pending update: `bbs update --snooze 1` (24 h), `2` (48 h), `3` (7 d).

## Workflow linting

Every workflow file must declare `needs-state:` frontmatter so the autopilot orchestrator can route mechanically. `bbs autopilot lint-workflow <path>` validates this and checks for missing `> produces:` directives.

```bash
# Lint a single workflow
bbs autopilot lint-workflow .claude/skills/autopilot/workflows/builder.md

# Lint all workflows
for wf in .claude/skills/autopilot/workflows/*.md .claude/workflows/*.md; do
  [ -f "$wf" ] && bbs autopilot lint-workflow "$wf"
done
```

### Pre-commit hook

`setup-skills` installs a pre-commit hook that auto-lints staged workflow files. To install or reinstall:

```bash
./bin/setup-skills
```

### CI

The `Lint Workflows` GitHub Action runs on pushes and PRs that touch workflow `.md` files. See `.github/workflows/lint-workflows.yml`.

## Watching a foreman

A foreman drives its batch from its own terminal, so its worst failure is the
quiet one: the session finishes a thought, prints nothing more, and sits at an
idle prompt while its workers wait for a design gate. Nothing detects that
today — the record's heartbeat is written by the foreman itself, so a foreman
that stopped working also stopped reporting that it stopped.

`bbs foreman watch` is the outside observer. It captures the last N lines of the
foreman's Orca terminal on an interval; if those bytes are identical for longer than
`--idle`, it types a nudge into the pane — the same "check status" a human would
send — and if the nudges stop landing, it says so and gives up rather than
poking forever. Independently of the pane, every `--status-interval` it sends
the same skill prompt as an active status check, so a busy foreman still gets
asked; a foreman whose record says `done` leaves the watch set even while its
terminal stays open. The status interval defaults to the configured
`foreman_status_interval` (3600 seconds). The idle threshold defaults to that
same interval; `--status-interval` overrides it for that watcher and `--idle`
can independently override the idle threshold. Foreman blocks awaiting worker
reports between reminders. Empty `check --wait` timeouts only renew the wait;
they do not trigger project audits. Worker reports trigger focused verification
of affected tickets, while status reminders trigger full reconciliation.

```bash
bbs foreman watch                       # every foreman with an open workspace
bbs foreman watch fm-acme               # just this one
bbs foreman watch --idle 300 --nudge "check status and report the board"
bbs foreman watch --once                # one pass, for cron
```

| flag | default | what it does |
|---|---|---|
| `--interval <sec>` | 60 | how often to capture the pane |
| `--idle <sec>` | effective `--status-interval` (3600) | unchanged for this long → nudge |
| `--status-interval <sec>` | `foreman_status_interval` config (3600) | periodic status prompt, even while the pane moves |
| `--lines <n>` | 40 | how much of the pane forms the fingerprint |
| `--nudge <text>` | `check status` | what gets typed in |
| `--max-nudges <n>` | 3 | budget before it reports `STALLED` and stops |
| `--once` | off | single pass then exit; prints a line per foreman |

It is a foreground loop, not a daemon — but you rarely start it yourself:
`bbs foreman adopt` and `bbs foreman spawn` launch an unscoped detached copy
on every check-in. Its global flock keeps exactly one unscoped watcher running;
scoped watchers use separate per-foreman flocks and cannot block other foremen.
It writes only its own clock and `watch.log` under `~/.babysit/watch/`, and
exits when no foreman has an open Orca terminal. Output is events only — a
foreman that is working produces no output at all.

```text
NUDGED fm-acme after 12m (1/3) — sent "check status"
STATUS fm-acme after 60m — sent "check status"
STALLED fm-acme — 3 nudges, no change in 41m; open "bbs foreman"
GONE fm-acme — terminal "bbs foreman" is closed
```

Two behaviours worth knowing. It selects foremen by **open Orca terminal, not
by liveness** — a foreman wedged long enough to need a nudge is exactly the one
whose heartbeat has gone stale, so selecting on `Live()` would drop every
foreman this exists to catch. And the nudge's own echo in the pane does not
refund the budget: real progress changes the pane on more than one tick, which
is what keeps `--max-nudges` binding on a dead session. The status clock is
separate from the idle clock on purpose: a status prompt neither spends a
nudge nor resets the idle window, so it can never let an unresponsive terminal
slip past the stall bound.

## Health checks

There is no health-check command. `./bin/setup-skills` reports what it linked and warns when `~/.local/bin` is missing from your `PATH`; beyond that, the preamble is the live check — it emits `BBS_DEGRADED` on stderr at the top of every skill run when no working `bbs` is reachable, which is the failure that actually matters.

To verify an install by hand:

```bash
bbs ticket --help     # exit 0 = the binary is present and serving subcommands
bbs config list       # reads ~/.babysit/config.yaml
```
