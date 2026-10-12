---
name: foreman
description: Autonomous Orca orchestrator for large projects made of multiple tickets or dependent features. Decomposes the project, owns branches and worktrees, supervises autopilot workers, coordinates per-ticket and integration QA, and applies the configured finish policy. Requires Orca and its orchestration skill; use autopilot directly for one serial ticket.
---
# foreman

Coordinate the project; workers do the heavy work. Foreman owns the ticket DAG,
phase dispatch, light evidence checks, delivery and cleanup. Autopilot executes
one assigned phase in the session Foreman opened; it never selects or switches
models. Planning, implementation, review, QA and merges all belong to workers.
Use [worker routing](references/worker-routing.md) before **every worker launch**,
including general planning and general finish: task complexity, model tier,
launch settings and receipt checks. Classify each parent assignment from its
own scope; do not inherit the last child's complexity. All stages use the same
report-driven wait and light verification below.

Follow [the preamble](../references/preamble.md) and
[Auto-Decision Framework](../references/auto-decision-framework.md). Shared refs
(`../references/*.md`) are filesystem paths beside this skill's directory, so
read them by path, not as `skill://`. Decide Taste choices; escalate missing
authority/context and honor explicit holds.

## 0. Initialize

Read [runtime](references/runtime.md) for the exact bootstrap and cold-resume
protocol. Load the live Orca orchestration guide, adopt the current Foreman
session, initialize or recover the parent ticket, claim it, and bind its Orca
Run. Use the supported `bbs ticket ensure` / `bbs ticket init` commands; there
is no top-level `bbs init`. Persist identifiers and checkpoints as work advances.

Resolve repository policy with `bbs autopilot git-flow`: `BBS_FINISH` selects
`review`, `land` or `pr`; profile sets review/QA breadth. Rebuild the task list
from durable state. On resume, continue the first unmet gate without replacing
live workers or replaying completed work.

## 1. General plan and prototype

Read [project contract](references/project-contract.md) and
[execution evidence](references/execution.md). Dispatch a planning/design worker
on the **critical phase** route in the parent checkout to produce one general
plan and prototype covering **all proposed ticket scopes**, dependencies,
shared interfaces and parent acceptance
criteria. Non-UI projects use a workflow/interface design with prototype N/A.

Present the plan, prototype and proposed tickets for **human review before child
worktrees or production dispatch**. Only explicit `--auto` delegates this review;
persist that choice across resumes. `--auto` still produces and reviews the same
artifacts and approval record through a separate **critical phase** design-review
worker; Foreman checks its evidence and records the decision. A material
scope/design change returns here; child implementation details are reviewed
against the accepted parent design.

## 2. Execute the ticket DAG

Read [topology](references/topology.md) to create/reuse child tickets, branches,
worktrees and dependency edges, then dispatch the admitted ready wave as one
batch within resource limits. Keep one writer per ticket worktree.
Never force-push, overwrite user work or bypass readiness.

For each ticket: **split phases → select model → launch worker → await report →
lightly verify evidence → next phase**. Keep separate phase Dispatches even when
both routes select the same model. Preserve the ticket's `pointers.workflow`.

| Phase | Assignment / stop boundary | Routing phase |
|---|---|---|
| Plan | Autopilot `<workflow> <ticket> --stop-after=plan`; plan/design artifacts only | Critical |
| Implement | Autopilot scoped to implementation, focused checks and local commits | Normal |
| Review | Autopilot scoped to `review-pr --fix`; dispatch passes `medium` effort (`low` only for `simple` tickets) regardless of profile-derived effort; applies and commits verified fixes, reports skips/unfixable findings to Foreman | Critical |
| QA | Autopilot scoped to `qa` without code fixes; return checks and evidence | Normal |
| Finish | Delivery worker runs only the authorized merge/PR handler | Normal |

For mechanical re-verification on unchanged code, use the exception in
[model routing](references/model-routing.md#phase-routing). Classify the
assignment's own scope before selecting its tier.

Each assignment includes the accepted parent artifacts, ticket requirement/plan,
exact prerequisite revisions, owned files, phase and stop boundary, selected
route, resource lease and required evidence. Use the injected Orca lifecycle:
the worker is already spawned, executes immediately, and reports exactly one
`worker_done`. Missing inputs come through Orca `ask`, not a local human prompt.
Foreman dispatches repairs and rechecks affected gates before advancing.

### Batch dispatch: one wake per batch

Every coordinator wake re-reads the whole Foreman context, so wakes, not worker
count, dominate cost. The bbs watcher waits and supervises; Foreman wakes once
per batch:

1. A batch is every admitted ready phase up to `FOREMAN_MAX_WORKERS`
   ([topology](references/topology.md#resource-admission)), light profiles
   first. Prefer the same phase across tickets so members finish together; never
   hold back a ready phase only to align it.
2. Launch every member, run `bbs foreman batch "$FOREMAN_ID" --tasks
   <task,task,...>`, then **end the turn**. Do not wait, `check --wait`,
   actively check terminals, checkpoints, git state, inbox, resource status or
   project snapshots for progress, start a polling timer, invoke `watch --once`
   or restart a live worker. The detached `bbs foreman watch` nudges a worker
   whose pane sits idle with its Dispatch open (bounded), holds Foreman's own
   status/stall prompts while the batch is open, and wakes Foreman with an Orca
   message plus a prompt naming the batch. To stop Orca typing a mail pointer
   into Foreman per worker report, it keeps Foreman's Delivery unacknowledged
   until that wake; an Orca mail pointer that still arrives mid-batch gets
   `batch show` and, with no attention, the turn ends without consuming.
3. On that prompt run `bbs foreman batch show "$FOREMAN_ID"`.
   `BATCH_STATE=settled` (failures included): consume with Orca `check` until
   it returns no messages (`check --ack <delivery>` returns the next Delivery
   of up to 50), processing every message before each ack, then verify, finish
   and hand the next batch over in the same wake.
   Otherwise it names attention (`messages:<n>`, `exited:<task>`,
   `stalled:<task>`): consume the Delivery, handle only that (answer, or follow
   the recovery contract for that worker), re-run `bbs foreman batch` with every
   Task still to watch, including repairs, and end the turn.
4. The watcher compacts the session (`/compact`, where the agent has one)
   before a settled wake, so every batch wake is a cold-resume boundary:
   recover from durable state per [runtime](references/runtime.md).

Process every message before acknowledging its Delivery. Judge each member from
`bbs ticket verdict-status`, readiness JSON and the reported handoff paths; read
worker transcripts only when those disagree. Read controls/intake before new
dispatch or finish; `paused` or `cancelled` prevents new work. A user/status
request, external recovery reminder, cold resume or concrete failure uses the
bounded reconciliation in [runtime](references/runtime.md), not a competing
polling loop.

### Keep tickets light: defer and merge heavy checks

Ticket phases run on `plan`/`standard` profiles. A check needing a heavy profile
(`ios-simulator`, `android-simulator`, `local-ml`) is not run per ticket: the
QA assignment names it deferred, the worker runs every other check, records
`DEFERRED_HEAVY=<profile>: <checks>` in its handoff and leaves the `qa`
verdict open. Foreman then dispatches one **consolidated heavy QA** per profile
covering every ticket waiting on it:

- dispatch it only when no light phase is ready, or when it is the last gate
  before waiting tickets can finish; it never displaces light work;
- one lease and one simulator boot: check each covered ticket on its exact
  worktree head in dependency order and write that ticket's own `qa` verdict
  and evidence with `BABYSIT_TICKET=<ticket>`;
- a covered ticket is not finish-ready until its heavy checks pass; a failure
  repairs only that ticket and its re-check joins the next consolidated run.

### Light verification and ticket finish

Check the expected Task/Dispatch and accepted lifecycle settlement, then the
phase's artifacts, revision and verdict. A message alone is not a passing gate.
For code tickets, read current review/QA evidence, `qa-evidence` and
`bbs ticket readiness --action <review|land|pr> --json`; require `ok` and
`data.ready`. For evidence-only tickets, check their artifacts and acceptance
coverage. Foreman does not repeat code review, rerun tests or implement fixes;
dispatch missing checks or repairs to workers. Changed inputs invalidate gates.
Interacting tickets require pre-land integration QA before their landing.

As soon as a ticket's finish prerequisites pass, seal its evidence and apply
[delivery](references/delivery.md) in dependency order: merge to the authorized
base/wave branch or create the PR. With `review`, retain work for human review.
The configured handler determines the destination; do not invent a wave-branch
merge command. Local lands must wait while remaining shared-surface operations
could reset base, as the delivery contract requires.

Verify the finish receipt, archive worker output, release settled workers and
resource leases, and close the ticket's Orca terminals/worktree surfaces. Remove
only eligible clean Git worktrees; keep branches and recoverable work. Release
settled phase workers between phases too; cleanup need not wait for the project.
Never close an active/unverifiable worker or use force removal.

## 3. General finish

1. Dispatch a read-only finish-audit worker on the **normal phase** route to
   confirm every required ticket completed execution or was explicitly cancelled,
   and verify merge/PR/review receipts and cleanup. Cancellation removes scope
   only through an accepted contract change. An open PR may be execution-complete
   while its ticket remains `in_review`; do not claim merged.
2. Dispatch integration/delivery workers on the **normal phase** route for any
   remaining authorized handlers, final surface preparation and safe cleanup.
   Await their receipts before QA; delegate restoration after QA likewise.
3. Dispatch final project QA and any required product acceptance worker on the
   **normal phase** route, on the exact delivered base or retained QA/wave
   composition, following [final integration QA](references/project-contract.md).
   Workers check the complete parent scope and cross-ticket flows. Await reports;
   failures return to owning repair workers, affected gates and delivery, then QA.
4. Foreman lightly verifies final evidence and cleanup, releases settled workers
   and leases, persists the parent report, and requires
   `bbs foreman readiness "$PARENT" --action finish --json` to report ready.
   Only then run `bbs foreman complete "$PARENT" --foreman "$FOREMAN_ID"`.
   No missing gate, blocked required ticket or unfinished cleanup can become DONE.

Report the result, delivery state, final QA evidence and remaining human action.
Use `STATUS: DONE | IN_PROGRESS | NEEDS_CONTEXT | BLOCKED`, link the parent
`report.md`, and end with `Next:` (review retained work/PRs, or none). Distinguish
local merges, open PRs and verified remote merges. Normal worker activity stays
in Orca; report state changes, requests and terminal evidence.
