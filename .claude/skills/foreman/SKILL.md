---
name: foreman
description: Autonomous Orca orchestrator for large projects made of multiple tickets or dependent features. Splits the project into phases, owns branches and worktrees, runs each phase as one autopilot worker session end to end — plan, approval, build, review, QA — launches the whole phase DAG in one batch, wakes only when it settles or a phase needs a decision, and applies the configured finish policy. Requires Orca and its orchestration skill; use autopilot directly for one serial ticket.
---
# foreman

Foreman keeps the project goal, confirms the general plan, coordinates the
phases and verifies the final result. Workers do everything else. Foreman
splits a project into **phases** — independently verifiable slices, each one
child ticket with its own branch and worktree — and owns the phase DAG,
dispatch, light evidence checks, delivery and cleanup.

**One phase is one worker session:** a single autopilot Dispatch runs the
phase's whole loop (plan → plan approval → await prerequisites → merge them →
implement → review → QA) on one model without stopping between steps, and
every later follow-up for that phase (repair, prerequisite refresh, delivery)
returns to the same session while it lives. A fresh worker re-reads the
requirement, plan, code and history on a cold prompt cache; keeping the phase
in one session is what this design buys, so never split a phase's steps across
workers. Autopilot never selects or switches models.

**Foreman stays light.** Its context holds the goal, the accepted parent
artifacts, the DAG and durable ids — never worker transcripts, diffs or test
logs. It judges phases from readiness JSON, verdicts and handoff paths, and it
wakes only when the batch settles or a phase needs it: an `L` plan review, a
worker's mid-run question, or a worker that cannot report.

Use [worker routing](references/worker-routing.md) before **every worker launch
or session reuse**, including general planning and general finish:
task complexity, model tier, launch settings and receipt checks. Classify each
assignment from its own scope; do not inherit the last phase's complexity.

Follow [the preamble](../references/preamble.md) and
[Auto-Decision Framework](../references/auto-decision-framework.md). Shared refs
(`../references/*.md`) are filesystem paths beside this skill's directory, so
read them by path, not as `skill://`. Decide Taste choices; escalate missing
authority/context and honor explicit holds. A run is autonomous: nobody is
expected at the keyboard after the general plan, so a decision only a human can
make parks the one phase that needs it while the rest of the DAG keeps going.

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
on the **critical** route in the parent checkout to produce one general plan and
prototype covering **all proposed phases**, dependencies, shared interfaces and
parent acceptance criteria. Phases are sized and ordered per
[topology](references/topology.md#decompose-and-prepare-topology). Non-UI
projects use a workflow/interface design with prototype N/A.

Present the plan, prototype and proposed phases for **human review before child
worktrees or production dispatch**. Only explicit `--auto` delegates this review;
persist that choice across resumes. Under `--auto`, Foreman reviews the same
artifacts itself and records the approval with evidence. This is the one
checkpoint where a human is expected. A material scope/design change returns
here; phase plans are reviewed against the accepted parent design.

## 2. Execute the phase DAG

Read [topology](references/topology.md) to create/reuse phase tickets, branches,
worktrees and dependency edges, then launch the DAG as one batch within
resource limits. Keep one writer per phase worktree — its phase session. Never
force-push, overwrite user work or bypass readiness. Preserve each phase's
`pointers.workflow`.

For each phase: **select its route once → launch its phase worker → await the
batch → lightly verify → reuse, retain or release the session → finish**. The
phase worker follows autopilot's **Foreman phase assignment** contract:

| Loop step (one session) | Boundary |
|---|---|
| Plan | `plan-draft` scoped by the phase seed and accepted parent artifacts; XS skips |
| Plan approval | By phase size — see [plan approval](#plan-approval-by-phase-size) |
| Await prerequisites | `bbs foreman await` until every prerequisite phase settles; no-op for a root phase |
| Merge prerequisites | Exactly the heads `await` verified, in its order; the post-merge HEAD is the phase's review base; then reconcile the plan with the merged code |
| Implement | Code, focused checks and local commits |
| Review | `review-pr --fix` at `medium` effort (`low` only for `simple` phases) regardless of profile-derived effort |
| QA | `qa` with the repair loop; deferred heavy checks only recorded |
| Report | Readiness, handoff and exactly one `worker_done`; no push, land or PR |

The route is chosen once per phase (phase class `critical`, because the session
owns its own planning and review) and reused by every follow-up; see
[model routing](references/model-routing.md#assignment-routing). Each assignment
includes the accepted parent artifacts, the phase requirement/seed, size and
acceptance IDs, its prerequisites as `<ticket>:<orca-task>` pairs, owned files
and contracts, review effort, deferred heavy checks, selected route, resource
lease and required evidence. Use the injected Orca lifecycle: the worker is
already spawned, executes immediately, and reports exactly one `worker_done`
per Dispatch. Missing inputs come through Orca `ask`, answered into the same
live session — not a local human prompt.

### Plan approval by phase size

The plan step flows into implementation inside the same session; only an `L`
plan waits, and it waits on Foreman, not a human. Size is the larger of the
manifest's and the plan's own `ticket_size`:

| Size | Plan | Approval |
|---|---|---|
| `XS` | none | nothing to approve |
| `S`, `M` | phase `plan.md` | the worker self-approves as Foreman's delegate and continues |
| `L` | phase `plan.md` | the worker publishes it and asks Foreman; **Foreman reviews and approves**, and the worker continues in the same session |

The exact commands and refusal handling are in
[project contract](references/project-contract.md#project-design-checkpoint).
To review an `L` plan, Foreman reads that phase's `plan.md` (and `design.md`
for UI) against the parent goal, the phase's acceptance IDs and the shared
contracts in the manifest — not the code — then either self-resolves the
approval and answers `approved`, or resolves it `redirect` and answers with
the exact changes; the worker revises in place and asks again. After two
redirects Foreman decides: approve with named conditions, or park the phase.

A plan the worker cannot self-approve — the CLI's money/auth/irreversible-data
floor, a `bbs foreman hold`, a spent grant bound, or a material deviation from
accepted scope or a shared contract — also goes to Foreman. Foreman answers
whatever its own authority covers (scope it accepted, contract choices within
the parent design). What only a human can approve **parks** that phase: the
worker reports `NEEDS_CONTEXT` with the published approval, Foreman retains
its session, records the decision under `Remaining` in the parent report and
relays it through the preamble's channel, and the rest of the DAG continues.
When the approval resolves, a `redirect` follow-up resumes the parked session.

### Phase session ownership

The phase session is the phase's single writer for the phase's whole life.
After every accepted report, decide its next owner before acknowledging the
Delivery (Orca completion accounting):

1. **Reuse** for an immediate follow-up on the same phase — a `repair`
   (verification gap, review/QA finding, heavy/integration failure, merge
   conflict), a `refresh` (a prerequisite changed or settled after this phase
   stopped waiting for it), a `redirect` (a parked or redirected plan was
   answered) or a `deliver` (its authorized finish handler): create the
   follow-up Task and start it on the session's exact terminal
   ([reuse launch](references/worker-routing.md#reuse-a-phase-session)).
2. **Retain** (`worker-retain`) when the phase is parked or passed but still
   waits on something that will send it back — a human approval, a
   prerequisite's repair after `AWAIT=not-ready`, consolidated heavy QA,
   pre-land integration QA, or its turn in the land order. Keep at
   most `FOREMAN_MAX_WORKERS` retained idle phase sessions; release the
   oldest beyond that.
3. **Release** otherwise, and always at phase close-out.

Release the Dispatch's resource lease on every settlement; a reused session
reserves a fresh one. A follow-up for a phase whose session is gone or not
provably live starts a fresh worker in the phase worktree on the phase's
recorded route, recovering from durable state. Never reuse one phase's session
for another phase, and never send a follow-up to a session whose Dispatch is
still open.

### Batch dispatch: one wake per project

Every coordinator wake re-reads the whole Foreman context, so wakes, not worker
count, dominate cost. Dependent phases therefore launch **with** their
prerequisites and wait inside their own sessions — planning and approving
while the prerequisites build — instead of waking Foreman once per DAG wave.
The bbs watcher waits and supervises:

1. A batch is every phase not yet launched, in dependency order (a phase's
   prerequisites are already in the batch or done), up to
   `FOREMAN_MAX_WORKERS` ([topology](references/topology.md#resource-admission));
   ready phases and light profiles take the slots first, so a waiting
   dependent never displaces runnable work. Never hold back ready work to
   align finish times.
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
   of up to 50), processing every message before each ack, then verify, decide
   each session's owner, finish and hand the next batch over in the same wake.
   Otherwise it names attention — `messages:<n>` (an `L` plan review or a
   worker question), `exited:<task>` or `stalled:<task>`: consume the Delivery
   and handle only that — answer the question, or follow the recovery contract
   for that worker — then re-run `bbs foreman batch` with the same Tasks plus
   any new ones, and end the turn.
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

### Keep phases light: defer and merge heavy checks

Phase workers run on `plan`/`standard` profiles. A check needing a heavy profile
(`ios-simulator`, `android-simulator`, `local-ml`) is not run per phase: the
assignment names it deferred, the worker runs every other check, records
`DEFERRED_HEAVY=<profile>: <checks>` in its handoff and
`pointers.deferred_heavy`, and leaves the `qa` verdict open; dependents may
build on it once its review passes. Retain that phase session; Foreman then
dispatches one **consolidated heavy QA** per profile covering every phase
waiting on it:

- dispatch it only when no light phase is ready, or when it is the last gate
  before waiting phases can finish; it never displaces light work;
- one lease and one simulator boot: check each covered phase on its exact
  worktree head in dependency order and write that phase's own `qa` verdict
  and evidence with `BABYSIT_TICKET=<ticket>`;
- a covered phase is not finish-ready until its heavy checks pass; a failure is
  a `repair` follow-up on that phase's session, and its re-check joins the next
  consolidated run.

### Light verification and phase finish

Check the expected Task/Dispatch and accepted lifecycle settlement, then the
phase's artifacts, plan approval record, revision and verdicts. A message alone
is not a passing gate. For code phases, read current review/QA evidence,
`qa-evidence` and `bbs ticket readiness --action <review|land|pr> --json`;
require `ok` and `data.ready`, and confirm each merged prerequisite head is
still the one that prerequisite was verified at. For evidence-only phases,
check their artifacts and acceptance coverage. Foreman does not repeat code
review, rerun tests or implement fixes; any gap is a `repair` follow-up naming
the exact evidence. Changed inputs invalidate gates. Interacting phases require
pre-land integration QA before their landing.

| Trigger | Next owner |
|---|---|
| Light verification gap, heavy/integration QA failure, land/PR conflict | `repair` on the phase session (fresh phase worker if gone) |
| A prerequisite was repaired after a dependent merged it, or a dependent stopped on `AWAIT=not-ready` and the prerequisite now passes | `refresh` on each affected dependent session, in dependency order |
| A parked plan approval was resolved | `redirect` on the parked session |
| Phase reported `BLOCKED` after its own retry budget | Fresh read-only **critical** diagnosis worker, then `repair` with its findings |
| Phase passed; finish authorized now | `deliver` on the phase session, or Foreman's own `review` close-out |

As soon as a phase's finish prerequisites pass, seal its evidence and apply
[delivery](references/delivery.md) in dependency order: merge to the authorized
base/wave branch or create the PR. With `review`, retain work for human review.
The configured handler determines the destination; do not invent a wave-branch
merge command. Local lands must wait while remaining shared-surface operations
could reset base, as the delivery contract requires.

Verify the finish receipt, archive worker output, release the phase session and
resource leases, and close the phase's Orca terminals/worktree surfaces. Remove
only eligible clean Git worktrees; keep branches and recoverable work. Cleanup
need not wait for the project. Never close an active/unverifiable worker or use
force removal.

## 3. General finish

1. Dispatch a read-only finish-audit worker on the **normal** route to confirm
   every required phase completed execution or was explicitly cancelled, and
   verify merge/PR/review receipts and cleanup. Cancellation removes scope
   only through an accepted contract change. An open PR may be execution-complete
   while its ticket remains `in_review`; do not claim merged.
2. Dispatch integration/delivery workers on the **normal** route for any
   remaining authorized handlers, final surface preparation and safe cleanup.
   Await their receipts before QA; delegate restoration after QA likewise.
3. Dispatch final project QA and any required product acceptance worker on the
   **normal** route, on the exact delivered base or retained QA/wave
   composition, following [final integration QA](references/project-contract.md).
   Workers check the complete parent scope and cross-phase flows. Await reports;
   failures return to the owning phase as `repair` (a fresh phase worker when
   its session is released), then affected gates and delivery, then QA.
4. Foreman lightly verifies final evidence and cleanup, releases settled workers
   and leases, persists the parent report, and requires
   `bbs foreman readiness "$PARENT" --action finish --json` to report ready.
   Only then run `bbs foreman complete "$PARENT" --foreman "$FOREMAN_ID"`.
   No missing gate, blocked required phase or unfinished cleanup can become DONE.

Report the result, delivery state, final QA evidence and remaining human action.
Use `STATUS: DONE | IN_PROGRESS | NEEDS_CONTEXT | BLOCKED`, link the parent
`report.md`, and end with `Next:` (review retained work/PRs, parked approvals,
or none). Distinguish local merges, open PRs and verified remote merges. Normal
worker activity stays in Orca; report state changes, requests and terminal
evidence.
