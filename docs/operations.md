# Operations

Day-2 configuration, telemetry, and upgrade handling.

## Configuration

```bash
bbs config set telemetry local       # off | local
bbs config set update_check true     # false silences upgrade notifications
bbs config set auto_upgrade false    # true runs bbs update on session start
bbs config set proactive true        # false = only run skills typed explicitly
bbs config set foreman_status_interval 3300  # seconds between team lead reconciliation ticks
bbs config set foreman_cache_ttl auto  # prompt-cache lifetime (auto: claude 3600, others 300)
bbs config set parallel_max_workers auto  # min(16, max(2, CPUs-2)) per Team
bbs config set parallel_global_units auto  # global weighted Team capacity
bbs config list                      # show saved YAML (not all effective defaults)
```

### Settings without a dashboard control

These settings are available through the CLI or files; no UI feature flag is
needed. Inspect saved user settings with `bbs config list` or
`bbs config get <key>`. An absent key prints nothing and uses its built-in
default; `list` does not enumerate every default or refresh old header comments.

To open the user configuration directly:

```bash
BBS_CONFIG_DIR="${BABYSIT_STATE_DIR:-$HOME/.babysit}"
mkdir -p "$BBS_CONFIG_DIR"
nano "$BBS_CONFIG_DIR/config.yaml"
```

`bbs config set <key> <value>` creates/updates that YAML without needing an
editor. A key being writable does not mean it is supported: use documented
keys below and the configuration examples above.

| Setting / file | How to inspect or change | Default / scope |
|----------------|--------------------------|-----------------|
| `semantic_decision_provider` | `bbs config get semantic_decision_provider`; `bbs config set semantic_decision_provider cloudflare` | `llm`; user scope only |
| `semantic_decision_model` | `bbs config set semantic_decision_model clef-flash` or `clef` | `clef-flash`, used only with `cloudflare` |
| Semantic credentials | Edit `$BBS_CONFIG_DIR/.env` with `nano`; set `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_API_TOKEN` | Environment wins, then user `.env`; project `.env` is ignored |
| Project inference limits | Edit `<repo>/.babysit/semantic-decision.yaml` | Optional; can restrict external inference, not enable it or select provider/model |
| Team model/effort routes | `bbs team model --json`; edit `~/.babysit/settings.json` or `<repo>/.babysit/settings.json` | Built-in policy, then user, then repo overrides; [schema](../.claude/skills/team/references/model-routing.md#model-tiers) |
| Base branch | `bbs autopilot base-branch`; `BBS_BASE_BRANCH` or `bbs config set base_branch <name>` | `origin/HEAD`, then `main`; the only git setting ([git flow](../.claude/skills/references/git-flow.md)) |

The semantic-decision step always exists. Selecting Cloudflare is the explicit
user opt-in to external inference; credentials alone do not select it. Switch
back with `bbs config set semantic_decision_provider llm`. No separate
`semantic_decision_enabled` switch is needed. The CLI reads config each call;
there is no daemon restart or rebuild for settings changes.

Project restrictions, for example:

```yaml
# <repo>/.babysit/semantic-decision.yaml
allowed_kinds: [task-size, task-complexity, testcase]
# enabled: false  # force the LLM provider for this project
```

Other decision kinds fall back to LLM. `allowed_kinds: []` permits no external
inference; omitted permits all kinds under the user's chosen provider.
Malformed policy also falls back. `enabled: true` grants no access the user
has not configured. Do not run `bbs secrets load` to source project credentials
for this provider; its credentials belong in the user file or environment.

Cloudflare's 20-second timeout and 0.75 confidence threshold are currently code
constants, not hidden config keys. Request/response format, all decision kinds
and telemetry fields live in the
[semantic-decision contract](../.claude/skills/references/semantic-decision.md).

### Decision coverage and human review

Bounded judgments now use one `semantic-decision` step, with the current LLM
as default. This coverage audit separates judgments from enforced policy:

| Decision | Consumer / kind | What still governs execution |
|----------|-----------------|------------------------------|
| Mechanical, Taste, or User Challenge | Auto-Decision Framework / `decision-tier` | Known User Challenges and missing authority cannot be downgraded |
| Size, complexity, workflow and decomposition | Plan-draft, Team routing, Autopilot / `task-size`, `task-complexity`, `skill-route`, `orchestration` | Explicit user selections, rubric floors, workflow prerequisites |
| Impacted tests and coverage | Implement, QA / `testcase` | Required criteria, regression reproducers and repo checks |
| Finding validity and product severity | Review-pr, Fix-pr, Autopilot repair, Team product evaluator / `review-finding` | Cited disproof before refuting; broken criteria remain material |
| Extra review worth doing | Review-pr / `deep-review` | Mandatory phases of the selected effort still run |
| Plan evidence sufficient; human input needed | Team parent/child review, Autopilot / `human-review` | Artifact freshness, delegation, holds, grants, safety floors and approval CLI |
| Recoverable blocker or missing human input | Triage / `recovery` | Retry budget, ownership and dispatch authority |

Team's parent plan requires human review unless explicit, persisted `--auto`
delegates it. Ticket plans are approved by size within the accepted parent plan
and current bounds: `XS`/`S` have none, `M` by the ticket worker as Team's delegate, `L` by
Team itself; one that neither may approve parks that ticket only. A phase's
production release go is never delegated: `approval self-resolve` refuses kind
`release` under every posture and grant. Semantic judgment evaluates the rubric; `approval
self-resolve` still enforces floor → rubric → authority → approval. A model's
`proceed` answer is not an approval record.

Autopilot `--auto` (implied for every Team worker) never prints a `/goal`
paste block: the agent sets its own goal when it has a tool for that, otherwise
it works the loop directly in the same session, so no human or Team wake is
needed to start execution. Autopilot honors `--stop-after` and invoker-held approvals, repairs routine
findings itself, and escalates non-derivable User Challenges. Composed Taste
choices go in its handoff; it never gains permission to push or release from a
semantic answer. Direct skills retain the Auto-Decision Framework's own final
reporting gate. The shared [human-review contract](../.claude/skills/references/semantic-decision.md#human-review)
distinguishes `proceed`, `revise`, `needs-human`, and `blocked`.

Open-ended design, hypothesis generation and experiment ideation still belong
to their domain skills; once they have evidence and bounded alternatives they
can use the shared `custom` kind. Deterministic branch/DAG checks, retry counts,
resource admission, artifact hashes and release gates do not need inference.

### Machine-global worker admission

`parallel_max_workers` is a per-Team ceiling and batch size. Its default,
`auto`, is min(16, max(2, CPUs - 2)): two CPUs stay free for the coordinator and
OS. `bbs team resource status` prints the effective value as
`FOREMAN_MAX_WORKERS`. It cannot protect one machine running several teams:
three coordinators with a ceiling of six could otherwise launch 18 workers. Every team lead therefore
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
| `standard` | 1 | — |
| `android-simulator` | 4 | mobile simulator, GPU |
| `ios-simulator` | 4 | mobile simulator, GPU |
| `local-ml` | 4 | GPU |

Reservations are global across repositories and teams sharing the same
`BABYSIT_HOME`. They are keyed by Team and bbs Task, making a retry
idempotent. The broker enforces the per-Team worker ceiling atomically as
well as the global weighted budget. Both `reserve` and `status`, and the detached
watcher, recover leases across all owners without waiting for a dead Team:

- Settled Dispatches release capacity immediately.
- An open Dispatch whose terminal shows its agent has exited (Orca's
  `terminal list` no longer names an agent there) is settled `failed` by exact
  Dispatch id, then its capacity is reclaimed.
- Reservations with no new Dispatch are reclaimed after ten minutes if the
  owner's heartbeat is stale or its record is missing. Resume must heartbeat
  and repeat `reserve` immediately before launching, saving the returned id.
- Live workers survive owner interruption and laptop sleep. Unverifiable
  workers (Orca unreachable, terminal vanished) remain held, with `RESOURCE_HELD` diagnostics; contact loss alone
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
bbs team resource status
bbs team resource reserve fm-project \
  --ticket bs-child --task orca-task-id --profile ios-simulator
bbs team resource release rsc-0123456789abcdef
```

`ADMISSION=queued` is backpressure, not a failed Task. Team may dispatch
other admitted work and retries the queued Task on its next reconcile tick.

### Project review and delivery evidence

Team clarifies the requirement, then has one worker prepare the basic
design — architecture, whole-product prototype, plan, ticket manifest and the
release-phase plan in `project.json` — before creating any phase ticket or
dispatching production work. Review that project checkpoint once; ticket plans
are approved against it — `M` by its own worker, `L` by Team, `XS`/`S` have none — without
stopping the ticket session. Product scope/design changes return to the parent
checkpoint. Approval is bound to the artifact revision, so changed documents
require a new review.

The phase plan names, per phase, its seeds and a release mode: `ask` (a human
gives the go), `auto` (the approved design is the go) or `none` (ships with the
next phase). The CLI enforces the order:

```bash
bbs team next <parent>                       # what to do now (advice; verbs keep their gates)
bbs team readiness <parent> --action dispatch|integrate|land|release --phase <id> --json
bbs team integrate <parent> --phase <id>     # one phase commit, detached worktree
bbs team land <parent> --phase <id>          # after phase QA: fast-forward base
bbs team bind <parent> --phase <id> --child <phase-ticket>
bbs team release <parent> --phase <id> --request --note "<what ships>"   # ask: human go
bbs team release <parent> --phase <id> --file receipt.json              # release-prd verdict
```

Phase `k` opens once phase `k-1` has landed, so it builds
while `k-1` waits on its release; it integrates only after every earlier phase
is released. A `ROLLED_BACK` or `PARTIAL` receipt stops the line until the
phase is fixed forward and released; from the second failure every phase needs
a human go. To give an `ask` go, approve the release record on the phase
ticket like any approval (`BABYSIT_TICKET=<phase-ticket> bbs ticket approval
resolve --action approve`, or the dashboard). An approved contract records each
seed's release mode as a floor; a revised contract that lowers one needs human
approval even under `--auto`.

To delegate the human design reviews too, explicitly pass `--auto`:

```text
Claude Code  /bbs:team --auto <large project>
OMP          /team --auto <large project>
Codex        $bbs:team --auto <large project>
```

`bbs team spawn fm-project --auto` and direct adoption with `--auto` persist
the choice on that lead record across resumes. It still creates and
reviews the artifacts; hold/grant bounds, non-delegable decisions, code review,
QA, and release modes remain in force. Existing records without
`auto: true` use human project review; ticket plan approval is unchanged.
`--auto` never gives a release go.

`bbs team recap <parent-ticket>` renders that report with project state into
`tickets/<parent>/recap.html` (also written by `complete`).
`bbs team report <parent-ticket>` reads the last durable `report.md`, even
after Orca closes. Each reconciliation writes the observation time, meaningful
ticket titles, each phase's state and release, worker/loop step, sealed
snapshot, gate evidence, phase commit and cleanup/blockers. It is explicitly a
saved snapshot; ask the running Team to check status for a fresh
reconciliation. Execution, delivery and cleanup are separate: LANDED does not
mean pushed or released. Missing evidence stays UNKNOWN.

Workers never commit. Each phase becomes **one commit** on local base
(`bbs team integrate`), built from the tickets' sealed snapshots in a
detached integration worktree, and one integration QA runs on exactly that
commit. It tests every criterion of that phase and all earlier ones, so the
last phase's run is the project's final QA. `bbs team land` then
fast-forwards local base to the tested commit; nothing is ever stashed or
reset. Failures return to ticket repair, reseal and a fresh phase commit. The full protocol is in
[the Team project contract](../.claude/skills/team/references/project-contract.md)
and [phases](../.claude/skills/team/references/phases.md).

### Which coding agent runs the work

Configure enabled coding agents and the default agent in Orca. The dashboard's
teams page shows agent ownership guidance (`#/settings` redirects there);
no BBS form or config file controls agent preferences. Team uses Orca
discovery on the destination host when
starting new workers. Existing workers and team leads resume with their recorded
agent/model rather than today's default.

`bbs agent detect --json` identifies the current harness with evidence;
`bbs agent list --json` reports supported CLIs and installed paths. Detection
uses `BABYSIT_CURRENT_AGENT`, the nearest recognized parent process, then native
session markers. It does not query Orca, installed binaries, credentials or
config; standalone skills remain usable without Orca. `bbs agent resolve` is
removed; configure new worker routes in Orca.

Direct skill invocation is the normal entrypoint:

```text
Claude Code  /bbs:team <large project>
OMP          /team <large project>
Codex        $bbs:team <large project>
```

The skill's first operation is `bbs team adopt <id>`. It detects the agent,
reads and renames the active Orca terminal, records the actual agent dialect,
and makes dashboard/watchdog wakes addressable. Re-adoption after compaction is
idempotent. It refuses to steal an id, terminal, or repo already bound to a
different Team. `bbs team spawn` is optional convenience and recovery,
not the required launch path. Direct invocation inherits the current CLI's
permission mode, so configure that session for unattended tool use before
leaving a multi-day run.

New Team terminals use an explicit `--agent` when requested, otherwise the
current harness. Plan, implement, and QA workers follow `--plan-worker`,
`--implement-worker`, and `--qa-worker` (default: that team lead's harness).
Old BBS worker/foreman agent/provider/model/effort settings no longer affect
launches. Legacy YAML bytes remain untouched; each launch command emits one
warning when such values are present, and `bbs config set` rejects new writes.
Providers stay in agent-native configuration unless Orca advertises a supported
explicit contract.

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
  | `grok` | `grok plugin install https://github.com/2found/2build` | `/bbs:autopilot` |
  | `omp` | `omp plugin marketplace add 2found/2build && omp plugin install bbs@babysit` | `/autopilot` |
  | `codex` | `codex plugin marketplace add 2found/2build && codex plugin add bbs@babysit` | `$bbs:autopilot` |
  | `cursor` | make the skills available under `.cursor/skills` or `.agents/skills` | `/autopilot` |

  `bbs team worker-command` preflights the binary and names the per-agent
  fix; the install itself is on the operator. Pass `--skill autopilot` rather
  than writing the prompt by hand — **omp plugin skills are a flat list, so they
  have no `bbs:` namespace**, while Codex uses a `$`
  sigil. A hard-coded `/bbs:autopilot` resolves incorrectly for both.
  A flat list also bounds what `skill://` can address: one skill directory, no
  `..`. Anything a skill names outside its own directory — the shared
  `references/` one level up, a sibling skill, a pack-level doc — is silently
  retargeted inside the skill (`skill://qa/../references/worktrees.md` becomes
  `skill://qa/references/worktrees.md`) and dies as `File not found`. SKILL.md
  files therefore say to read those by path, and
  `tests/test_skill_reference_links.sh` guards both halves: that the targets
  resolve, and that every skill which names one says how to read it.
  - **A lead's session is pinned to the agent that minted it.** `spawn` records
    it and uses it on resume; contradictory explicit `--agent` requests refuse.
    Recorded model, effort, and historical provider values remain available for
    exact-session recovery. New sessions do not read retired BBS preferences.

  What the handle *is* depends on the agent, and only `claude` and `grok` can
  be told to use one we chose: they take `--session-id <uuid>`. `omp` has no
  such flag, so a team lead on omp gets a private session directory
  (`--session-dir`) and resumes with `--continue` — unambiguous because nothing
  else writes to that directory. `codex` and `cursor` have neither, so either closed team lead
  starts a fresh conversation and cold-resumes from ticket + Orca state. It
  deliberately does not use repo-wide `resume --last`: several team leads may
  share one repo, and "last" could attach the wrong project's goal. A uuid is
  never recorded against an agent that cannot be told to use it.

  For multi-day runs, schedule `bbs team ensure <id>` to recreate a missing
  Orca terminal. The watcher needs no scheduling: `adopt` and `spawn` start a
  detached `bbs team watch` automatically. The unscoped watcher takes one
  global flock, while a scoped `watch <id>` takes an id-specific flock, so
  repeat check-ins are no-ops without blocking other team leads. The watcher exits
  on its own once no team lead has an open Orca terminal. `bbs team watch <id>
  --once` remains available to refresh an idle one by hand. Both prompts reload
  the team skill and carry `--team-id <id>` so compaction or a cold start
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

Orca owns enabled-agent and default-agent selection for new workers. Team
applies the ticket/tier routing table and destination-host discovery. Explicit
per-dispatch `--agent`, `--model`, and `--effort` requests remain available
through route and worker-command; managed `spawn` also accepts explicit fields
for a new Team. Provider selection belongs in the agent's native
configuration unless Orca advertises a supported selector. BBS global and
role-specific environment fallbacks are retired; `bbs config set` rejects
retired preferences.

```bash
bbs team worker-command --agent omp --model '<your-model-id>' --effort high \
  --skill autopilot --prompt 'Build the ticket'
bbs team spawn fm-demo --agent codex --model '<your-model-id>'
```

OMP worker routes return `launchMode: native-terminal`: render an idle command
with `bbs team worker-command --startup-only --agent omp --model <selection>`,
create an Orca terminal with that exact command, then call
`bbs team task launch <task> --agent omp --worktree <path> --terminal <handle> --created-terminal`
without launch overrides.
`--startup-only` rejects prompts and skill selectors, so supervision delivers
the assignment once. This preserves native aliases such as `@slow` without
changing global OMP configuration or falling back to the native default.

Managed Team records pin the selected agent, model and effort. Provider
values recorded by earlier versions remain readable and are applied only when
recovering that pinned session; new launches use native provider configuration.
Changed explicit settings are refused for a pinned session.

Team routes new supervised workers through `bbs team route` using the
destination host's Orca facts and persists the selected route, then launches
with `bbs team task launch`, which starts the agent through `orca agent
launch --surface terminal` and verifies the agent Orca detects in the returned
terminal (`matched`). Launching into an existing terminal (a native OMP
terminal, or a follow-up reusing a live session) returns `transport-matched`.
Orca reports no effective model or effort, so the worker verifies its actual
session settings before accepting ticket work. After settlement the watcher
closes the worker's terminal only if its Dispatch created it. Direct `worker-command` uses the same route policy.
See [model routing](../.claude/skills/team/references/model-routing.md) and the
[worker launch reference](../.claude/skills/team/references/worker-routing.md).

## Telemetry

Skill runs append JSON Lines to `~/.babysit/analytics/skill-usage.jsonl`. Because babysit runs unattended, telemetry is the *primary* feedback channel — treat it as load-bearing, not decoration. Local-only by default; nothing leaves the machine.

Nothing summarizes that file on demand — the reader is the `/bbs:analytics-review` skill. Dispatch it by hand (`/bbs:analytics-review`) when you want a report; to look at the raw rows, read the JSONL directly.

The companion file `~/.babysit/analytics/decisions.jsonl` is the audit trail for ticket-level events (size resizes, approval self-resolves) — one line per decision. Grep or `jq` it directly. It is machine-wide telemetry; a Team project's own record of the judgments it took is `<parent>/decisions.jsonl`, written by `bbs team log` and rendered in the recap.

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

`bbs setup` installs a pre-commit hook that auto-lints staged workflow files. To install or reinstall:

```bash
go run ./cmd/bbs setup
```

### CI

The `Lint Workflows` GitHub Action runs on pushes and PRs that touch workflow `.md` files. See `.github/workflows/lint-workflows.yml`.

## Watching a team

A team lead drives its batch from its own terminal, so its worst failure is the
quiet one: the session finishes a thought, prints nothing more, and sits at an
idle prompt while its workers wait for a design gate. Nothing detects that
today — the record's heartbeat is written by the team lead itself, so a team lead
that stopped working also stopped reporting that it stopped.

`bbs team watch` is the outside observer. It captures the last N lines of the
lead's Orca terminal on an interval; if those bytes are identical for longer than
`--idle`, it types a nudge into the pane — the same "check status" a human would
send — and if the nudges stop landing, it says so and gives up rather than
poking forever. Independently of the pane, every `--status-interval` it sends
the same skill prompt as an active status check, so a busy team lead still gets
asked; a team lead whose record says `done` leaves the watch set even while its
terminal stays open. The status interval defaults to the configured
`foreman_status_interval` (3300 seconds, just inside a one-hour prompt cache so
the prompt reads the cache instead of rewriting it). The idle threshold defaults to that
same interval; `--status-interval` overrides it for that watcher and `--idle`
can independently override the idle threshold. Status reminders trigger full
reconciliation.

While a batch handed over with `bbs team batch <id> --tasks a,b` is open, the
watcher holds those team lead prompts and supervises the batch instead: it types
`Continue Task <id>…` into a worker pane idle for `--worker-idle` with its
Dispatch open, up to `--worker-nudges` times, then reports it stalled. It wakes
the team lead (one pane prompt) once when every Dispatch is settled, or early for
a new question (tracked by question id, so a second question after the first
was answered still wakes), an exited worker (its terminal shows no agent), a
stall, or a lost worker — an open Dispatch with no live terminal for the whole
stall budget. Early attention is held for `--attention-window` (default 120 s)
from the first item, so plan reviews and questions from tickets that started
together cost one wake; `batch show` lists held items too. A wake that cannot be delivered stays owed and is retried each tick;
a failed member the team lead retries is followed to its new Dispatch, so the
batch does not settle under a running retry. Workers report into the bbs task
store, never into the lead's terminal, so nothing else types into it while
a batch runs. `bbs team batch show <id>` prints the batch state the wake refers to — each
settled member with its ticket, kind, gate verdict, summary and report — and
then the `NEXT=` steps of every project the team lead owns, so one read covers a
wake. A question counts as delivered only once `batch show` printed it: a wake
typed while the team lead is busy (an open dialog) can be swallowed, so
re-arming the batch wakes again for any pending question it never read.

A batch outlasting the team lead's prompt cache would make its wake a cold one:
the whole context rewritten at up to twice the input price instead of read at a
tenth. When the cache lifetime (`foreman_cache_ttl`, default the agent's own:
claude 3600 s, others 300 s) is at least 30 minutes, the watcher therefore
types one plain keep-warm line (not a skill prompt) asking for one `bbs team
batch show` — answer any pending question, handle any stalled, exited or lost
worker, else reply ok and end the turn — once the batch has gone that lifetime minus ten minutes without
prompting the team lead, at most 8 per batch. Shorter caches get none. The same
lifetime caps the default block of `bbs team await` and `bbs team task
ask` (540 s, or a minute inside the lifetime) so a worker's re-run after a
timeout reads its cache.

The watcher is built to outlive what goes wrong around it: every Orca call is
bounded (60 s), a failing tick is logged as `TICK-FAILED` rather than ending
the process, and while any batch is open an unreachable Orca is retried
(`ORCA-DOWN`) instead of exiting. If the process dies anyway, the next `bbs`
session-writer hook (at most once a minute per working agent) or `bbs team
await` call restarts it; the watch lock keeps that to one watcher.

While a batch is open, `bbs team batch` also installs an OS supervisor for
the watcher — a launchd agent on macOS
(`~/Library/LaunchAgents/dev.2found.bbs.foreman-watch.<hash>.plist`) or a
systemd user unit on Linux (`bbs-foreman-watch-<hash>.service`), one per
`BABYSIT_HOME`. It runs `bbs team watch --supervised`, which stands by
behind the live watcher's lock and takes over if that process dies, exits 0
when no batch is open (so the OS never restart-loops it), and restarts at most
once a minute after a crash. It carries only `PATH`, `HOME`, `BABYSIT_HOME` and
the Orca CLI variables it was installed with. Inspect or manage it with `bbs team supervisor
status|install|uninstall`; set `BBS_WATCH_SUPERVISOR=off` to never install one.
Windows has no supervisor; the hook restart above still applies.

A batch normally holds every ticket of the open phases: a dependent ticket's worker waits for
its prerequisites inside its own session with `bbs team await --prereqs
<ticket>:<task>,...`, and an `L` ticket waits for Team's plan approval through
`bbs team task ask`. Both run as foreground commands bounded below the
default 10-minute `--worker-idle` (540 s per call, then repeated), so a waiting
worker is neither nudged nor reported stalled. `await` exits 0 with the verified
heads to merge, 3 when a prerequisite settled without passing readiness (the
dependent reports `BLOCKED` and Team later sends it a `refresh`), and 4 on
timeout. A prerequisite Task not launched yet is still waiting, and a readiness read that fails
is retried twice before it counts as not-ready.

```bash
bbs team watch                       # every team lead with an open workspace
bbs team watch fm-acme               # just this one
bbs team watch --idle 300 --nudge "check status and report the board"
bbs team watch --once                # one pass, for cron
```

| flag | default | what it does |
|---|---|---|
| `--interval <sec>` | 60 | how often to capture the pane |
| `--idle <sec>` | effective `--status-interval` (3300) | unchanged for this long → nudge |
| `--status-interval <sec>` | `foreman_status_interval` config (3300) | periodic status prompt, even while the pane moves |
| `--lines <n>` | 40 | how much of the pane forms the fingerprint |
| `--nudge <text>` | `check status` | what gets typed in |
| `--max-nudges <n>` | 3 | budget before it reports `STALLED` and stops |
| `--worker-idle <sec>` | 600 | batch worker pane unchanged this long, Dispatch open → nudge |
| `--worker-nudges <n>` | 2 | worker nudges before the batch reports it `stalled:<task>` |
| `--attention-window <sec>` | 120 | early wakes arriving within this window of the first share one wake |
| `--supervised` | off | the launchd/systemd form: stands by behind a running watcher, exits 0 once no batch is open |
| `--once` | off | single pass then exit; prints a line per team lead |

It is a foreground loop, not a daemon — but you rarely start it yourself:
`bbs team adopt` and `bbs team spawn` launch an unscoped detached copy
on every check-in. Its global flock keeps exactly one unscoped watcher running;
scoped watchers use separate per-foreman flocks and cannot block other team leads.
It writes only its own clock and `watch.log` under `~/.babysit/watch/`, and
exits when no team lead has an open Orca terminal. Output is events only — a
team lead that is working produces no output at all.

```text
NUDGED fm-acme after 12m (1/3) — sent "check status"
STATUS fm-acme after 60m — sent "check status"
STALLED fm-acme — 3 nudges, no change in 41m; open "bbs team"
GONE fm-acme — terminal "bbs team" is closed
```

Two behaviours worth knowing. It selects team leads by **open Orca terminal, not
by liveness** — a team lead wedged long enough to need a nudge is exactly the one
whose heartbeat has gone stale, so selecting on `Live()` would drop every
team lead this exists to catch. And the nudge's own echo in the pane does not
refund the budget: real progress changes the pane on more than one tick, which
is what keeps `--max-nudges` binding on a dead session. The status clock is
separate from the idle clock on purpose: a status prompt neither spends a
nudge nor resets the idle window, so it can never let an unresponsive terminal
slip past the stall bound.

## Health checks

There is no health-check command. `bbs setup` reports what it linked and warns when `~/.local/bin` is missing from your `PATH`; beyond that, the preamble is the live check — it emits `BBS_DEGRADED` on stderr at the top of every skill run when no working `bbs` is reachable, which is the failure that actually matters.

To verify an install by hand:

```bash
bbs ticket --help     # exit 0 = the binary is present and serving subcommands
bbs config list       # reads ~/.babysit/config.yaml
```
