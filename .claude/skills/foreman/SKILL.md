---
name: foreman
description: Autonomous Orca orchestrator for large projects. Clarifies the requirement, has a worker produce the basic design (architecture, prototype, plan) split into release phases for one human review, then runs each phase as parallel autopilot tickets — one worker session per ticket, plan through QA — integrates and QAs the phase cumulatively, and releases it to production before the next phase. Wakes only when a batch settles or a decision needs it. Requires Orca and its orchestration skill; use autopilot directly for one serial ticket.
---
# foreman

Foreman keeps the project goal, confirms the basic design, coordinates and
verifies; workers do everything else. A project ships in **phases**, each a
production release increment; a phase is built by **tickets**, each one child
ticket with its own worktree run end to end by one worker session:

```text
requirement → basic design ⇒ human review → phase 1: tickets ∥ → integrate → cumulative QA → release
                                            phase 2: tickets ∥ → integrate → cumulative QA → release …
```

**One ticket is one worker session:** a single autopilot Dispatch runs the
ticket's whole loop (plan → plan approval → await and adopt prerequisites →
implement → review → QA) on one model without stopping between steps, and
every later follow-up for that ticket (repair, prerequisite refresh)
returns to the same session while it lives. A fresh worker re-reads the
requirement, plan, code and history on a cold prompt cache; keeping the ticket
in one session is what this design buys, so never split a ticket's steps across
workers. Autopilot never selects or switches models.

**Foreman stays light.** Its context holds the goal, the accepted design, the
phase plan, the DAG and durable ids — never worker transcripts, diffs or test
logs. It judges work from readiness JSON, verdicts and handoff paths, and it
wakes only when a batch settles or something needs it: an `L` plan review, a
worker's mid-run question, or a worker that cannot report.

## Human control points

The run is autonomous between these points; nothing else waits on a human.

| Point | Who decides | Mechanism |
|---|---|---|
| Kickoff questions that change architecture or the phase plan | human, once, batched | step 1; none under `--auto` except a User Challenge |
| Basic design + phase/release plan | **human**; `--auto` delegates to Foreman | `project-plan` approval (step 2) |
| Production release of an `ask` phase | **human only**, under every posture | `bbs foreman release --request`; self-resolve refuses it |
| A failed release twice, or a stopped line | human | the CLI demands a new release go |
| Money/auth/irreversible-data plans, `bbs foreman hold` | human | the ticket parks; the rest continues |

An `auto` phase releases on the approved design: approving the design with
`release: auto` phases *is* the durable authorization for those releases.

Use [worker routing](references/worker-routing.md) before **every worker launch
or session reuse**: task complexity, model tier, launch settings and receipt
checks. Classify each assignment from its own scope; do not inherit the last
ticket's complexity.

Follow [the preamble](../references/preamble.md) and
[Auto-Decision Framework](../references/auto-decision-framework.md). Shared refs
(`../references/*.md`) are filesystem paths beside this skill's directory, so
read them by path, not as `skill://`. Decide Taste choices; escalate missing
authority/context and honor explicit holds. A decision only a human can make
parks the one ticket or phase that needs it while independent work continues.

## 0. Initialize

Read [runtime](references/runtime.md) for the exact bootstrap and cold-resume
protocol. Load the live Orca orchestration guide, adopt the current Foreman
session, initialize or recover the parent ticket, claim it, and bind its Orca
Run. Use the supported `bbs ticket ensure` / `bbs ticket init` commands; there
is no top-level `bbs init`. Persist identifiers and checkpoints as work advances.

Git has one flow and no policy to resolve ([git flow](../references/git-flow.md)):
workers never commit, Foreman lands **one commit per phase** on local base,
and only a release pushes. Committing a single ticket is an unfinished job,
never a delivery. Find the release profile release-prd would use (`RELEASE.md`, `.babysit/release.md` or the one
linked from `AGENTS.md`/`CLAUDE.md`); without one every phase is `release:
none`. Rebuild the task list from durable state. On resume, continue the first
unmet gate — `bbs foreman snapshot "$PARENT" --json` names each phase's state —
without replacing live workers or replaying completed work.

## 1. Clarify the requirement

Foreman does this itself; it is the goal it keeps. Write the parent
`requirement.md`: outcome, audience, scope and non-goals, constraints
(deadline, platforms, data, compliance), assumptions, and open questions. Read
the codebase and docs enough to answer what they can. Classify each remaining
question with the Auto-Decision Framework: answer Mechanical and Taste
questions as recorded assumptions; ask the human only what would change the
architecture or the phase plan, in **one batch**, now — never mid-run. Under
`--auto` ask nothing but a User Challenge; record the rest as assumptions the
design review sees.

## 2. Basic design and phase plan

Read [project contract](references/project-contract.md),
[execution evidence](references/execution.md) and
[phases](references/phases.md). Dispatch one basic-design worker on the
**critical** route in the parent checkout to produce the architecture
(`design.md`), one prototype of the whole product (`prototype.html`, N/A with
interfaces and examples for non-UI work), `plan.md`, `manifest.md` and
`project.json` with its `phases` — sized, ordered and release-safe per
[the phase plan rules](references/phases.md#the-phase-plan), and tickets per
[topology](references/topology.md#decompose-and-prepare-topology).

Present it for **human review before any phase ticket or production
dispatch**. Only explicit `--auto` delegates this review; persist that choice
across resumes. Under `--auto`, Foreman reviews the same artifacts itself and
records the approval with evidence. A material scope/design change returns
here; ticket plans are reviewed against the accepted design.

## 3. Run each phase

Phases run in contract order through the [phase loop](references/phases.md#phase-loop),
each step gated by `bbs foreman readiness "$PARENT" --action
<dispatch|integrate|land|release> --phase <id> --json`:

1. **Open** — create the phase ticket and its tickets, then launch all of the
   phase's tickets as one batch (below). Tickets inside a phase run in
   parallel; an in-phase dependency waits in its own session.
2. **Integrate** — once every ticket is sealed, `bbs foreman integrate
   "$PARENT" --phase <id>` composes their snapshots into **one phase commit**
   on local base, checked out in the phase's integration worktree. A ticket
   pair that does not compose is a `repair` on the later ticket.
3. **Cumulative QA** — one integration worker runs the `qa` skill's Foreman
   integration mode in that worktree, on that exact commit: every criterion
   of this and all earlier phases, plus product review when the contract
   asks; failures are `repair`s on the owning ticket sessions.
4. **Land** — `bbs foreman land "$PARENT" --phase <id>` fast-forwards local
   base to the tested commit and removes the phase's worktrees.
5. **Release** — `auto`: `release-prd` with hypercare, in the integration
   session once the phase landed, else a release worker; `ask`: Foreman
   requests the human go first. Record the verdict with `bbs
   foreman release --file`. A failed release stops the line until it is fixed
   forward.

**Pipeline one phase deep:** while phase `k` waits on its release, open phase
`k+1` (readiness allows it once `k` has landed), so a human release decision
never idles the workers. Phase `k+1` cannot integrate until `k` is released.
A wake handles every phase that is ready — open, integrate, land, release — before
handing over the next batch.

## Tickets inside a phase

Read [topology](references/topology.md) to create/reuse tickets, worktrees
and dependency edges, then launch a phase's tickets as one batch
within resource limits. Keep one writer per ticket worktree — its ticket
session. Never commit a ticket, force-push, overwrite user work or bypass
readiness. Preserve
each ticket's `pointers.workflow`.

For each ticket: **select its route once → launch its ticket worker → await the
batch → lightly verify → reuse, retain or release the session → finish**. The
ticket worker follows autopilot's **Foreman ticket assignment** contract:

| Loop step (one session) | Boundary |
|---|---|
| Plan | `plan-draft` scoped by the ticket seed and accepted parent artifacts; XS skips |
| Plan approval | By ticket size — see [plan approval](#plan-approval-by-ticket-size) |
| Await prerequisites | `bbs foreman await` until every prerequisite ticket settles; no-op for a root ticket |
| Adopt prerequisites | `await` adopts each sealed prerequisite snapshot into the worktree as uncommitted changes; resolve `CONFLICT=` files, then reconcile the plan with the adopted code |
| Implement | Code and focused checks, left uncommitted |
| Review | `review-pr --fix` at `medium` effort (`low` only for `simple` tickets) |
| QA | `qa` with the repair loop; deferred heavy checks only recorded |
| Report | Readiness, handoff and exactly one `worker_done`; no commit, push or PR |

The route is chosen once per ticket (phase class `critical`, because the session
owns its own planning and review) and reused by every follow-up; see
[model routing](references/model-routing.md#assignment-routing). Each assignment
includes the accepted parent artifacts, the ticket requirement/seed, size and
acceptance IDs, its prerequisites as `<ticket>:<orca-task>` pairs, owned files
and contracts, review effort, deferred heavy checks, selected route, resource
lease and required evidence. Use the injected Orca lifecycle: the worker is
already spawned, executes immediately, and reports exactly one `worker_done`
per Dispatch. Missing inputs come through Orca `ask`, answered into the same
live session — not a local human prompt.

### Plan approval by ticket size

The plan step flows into implementation inside the same session; only an `L`
plan waits, and it waits on Foreman, not a human. Size is the larger of the
manifest's and the plan's own `ticket_size`:

| Size | Plan | Approval |
|---|---|---|
| `XS` | none | nothing to approve |
| `S`, `M` | ticket `plan.md` | the worker self-approves as Foreman's delegate and continues |
| `L` | ticket `plan.md` | the worker publishes it and asks Foreman; **Foreman reviews and approves**, and the worker continues in the same session |

The exact commands and refusal handling are in
[project contract](references/project-contract.md#project-design-checkpoint).
To review an `L` plan, Foreman reads that ticket's `plan.md` (and `design.md`
for UI) against the parent goal, the ticket's acceptance IDs and the shared
contracts in the manifest — not the code — then either self-resolves the
approval and answers `approved`, or resolves it `redirect` and answers with
the exact changes; the worker revises in place and asks again. After two
redirects Foreman decides: approve with named conditions, or park the ticket.

A plan the worker cannot self-approve — the CLI's money/auth/irreversible-data
floor, a `bbs foreman hold`, a spent grant bound, or a material deviation from
accepted scope or a shared contract — also goes to Foreman. Foreman answers
whatever its own authority covers (scope it accepted, contract choices within
the parent design). What only a human can approve **parks** that ticket: the
worker reports `NEEDS_CONTEXT` with the published approval, Foreman retains
its session, records the decision under `Remaining` in the parent report and
relays it through the preamble's channel, and the rest of the DAG continues.
When the approval resolves, a `redirect` follow-up resumes the parked session.

### Ticket session ownership

The ticket session is the ticket's single writer for the ticket's whole life.
After every accepted report, decide its next owner before acknowledging the
Delivery (Orca completion accounting):

1. **Reuse** for an immediate follow-up on the same ticket — a `repair`
   (verification gap, review/QA finding, heavy/integration failure, a phase
   that does not compose), a `refresh` (a prerequisite changed or settled
   after this ticket stopped waiting for it) or a `redirect` (a parked or
   redirected plan was answered): create the
   follow-up Task and start it on the session's exact terminal
   ([reuse launch](references/worker-routing.md#reuse-a-ticket-session)).
2. **Retain** (`worker-retain`) when the ticket is parked or passed but still
   waits on something that will send it back — a human approval, a
   prerequisite's repair after `AWAIT=not-ready`, consolidated heavy QA, or
   its phase's integration and cumulative QA, whose failures come back to it
   as `repair`s. Keep at
   most `FOREMAN_MAX_WORKERS` retained idle ticket sessions; release the
   oldest beyond that.
3. **Release** otherwise, and always at ticket close-out.

Release the Dispatch's resource lease on every settlement; a reused session
reserves a fresh one. A follow-up for a ticket whose session is gone or not
provably live starts a fresh worker in the ticket worktree on the ticket's
recorded route, recovering from durable state. Never reuse one ticket's session
for another ticket, and never send a follow-up to a session whose Dispatch is
still open.

### Batch dispatch: one wake per phase

Every coordinator wake re-reads the whole Foreman context, so wakes, not worker
count, dominate cost. Dependent tickets therefore launch **with** their
prerequisites and wait inside their own sessions — planning and approving
while the prerequisites build — instead of waking Foreman once per DAG wave.
The bbs watcher waits and supervises:

1. A batch is every ticket of the open phases not yet launched, in
   dependency order (a ticket's
   prerequisites are already in the batch or done), up to
   `FOREMAN_MAX_WORKERS` ([topology](references/topology.md#resource-admission));
   ready tickets and light profiles take the slots first, so a waiting
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
   each session's owner, advance every phase that is ready (phases.md) and
   hand the next batch — tickets, integration, QA or release workers — over in
   the same wake.
   Otherwise it names attention — `messages:<n>` (an `L` plan review or a
   worker question), `exited:<task>`, `stalled:<task>` or `lost:<task>` (an
   open Dispatch with no live worker terminal through the stall budget):
   consume the Delivery and handle only that — answer the question, or follow
   the recovery contract for that worker (a lost Dispatch is a concrete
   failure: fence it with `worker-stop`/`worker-abandon` before relaunching)
   — then re-run `bbs foreman batch` with the same Tasks plus any new ones,
   and end the turn. The watcher follows a retried Task to its new Dispatch,
   outlives Orca restarts, and any `bbs` call made by a working agent restarts
   it if it died, so never babysit it.
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

Ticket workers run on `plan`/`standard` profiles. A check needing a heavy profile
(`ios-simulator`, `android-simulator`, `local-ml`) is not run per ticket: the
assignment names it deferred, the worker runs every other check, records
`DEFERRED_HEAVY=<profile>: <checks>` in its handoff and
`pointers.deferred_heavy`, and leaves the `qa` verdict open; dependents may
build on it once its review passes. Retain that ticket session; Foreman then
dispatches one **consolidated heavy QA** per profile covering every ticket
waiting on it:

- dispatch it only when no light ticket is ready, or when it is the last gate
  before waiting tickets can finish; it never displaces light work;
- one lease and one simulator boot: check each covered ticket in its exact
  worktree, on its current tree, in dependency order and write that ticket's own `qa` verdict
  and evidence with `BABYSIT_TICKET=<ticket>`;
- a covered ticket is not finish-ready until its heavy checks pass; a failure is
  a `repair` follow-up on that ticket's session, and its re-check joins the next
  consolidated run.

### Light verification and ticket close-out

Check the expected Task/Dispatch and accepted lifecycle settlement, then the
ticket's artifacts, plan approval record, revision and verdicts. A message alone
is not a passing gate. For code tickets, read current review/QA evidence,
`qa-evidence` and `bbs ticket readiness --action review --json`;
require `ok` and `data.ready`, and confirm each adopted prerequisite snapshot
is still the one that prerequisite was sealed at. For evidence-only tickets,
check their artifacts and acceptance coverage. Foreman does not repeat code
review, rerun tests or implement fixes; any gap is a `repair` follow-up naming
the exact evidence. Changed inputs invalidate gates.

| Trigger | Next owner |
|---|---|
| Light verification gap, heavy QA or phase QA failure | `repair` on the ticket session (fresh ticket worker if gone) |
| `integrate` reports the phase does not compose | `repair` on the later ticket named: `bbs ticket adopt <earlier>`, resolve, re-run its gates |
| A prerequisite was resealed after a dependent adopted it, or a dependent stopped on `AWAIT=not-ready` and the prerequisite now passes | `refresh` on each affected dependent session, in dependency order |
| A parked plan approval was resolved | `redirect` on the parked session |
| Ticket reported `BLOCKED` after its own retry budget | Fresh read-only **critical** diagnosis worker, then `repair` with its findings |

As soon as a ticket passes, seal it (`bbs foreman seal`): its uncommitted tree
becomes the snapshot its phase is built from. Nothing about a ticket is
committed, pushed or PR'd on its own; a ticket reaches base only inside its
phase commit ([delivery](references/delivery.md)).

Once a ticket's phase has landed, archive worker output, release the ticket
session and resource leases, and close the ticket's Orca terminals/worktree
surfaces; `bbs foreman land` removed its worktree, or reported it kept because
it changed after the seal. Cleanup need not wait for the release. Never close
an active/unverifiable worker or delete kept work.

## 4. Finish

1. The last phase's cumulative QA is the project's final QA; nothing is
   re-tested by a separate final pass unless a later repair invalidated it.
2. Dispatch a read-only finish-audit worker on the **normal** route to confirm
   every phase is released (or `none` and landed), every ticket's cleanup,
   and that cancelled scope went through an accepted contract change.
3. Release settled workers and leases, persist the parent report, and require
   `bbs foreman readiness "$PARENT" --action finish --json` to report ready.
   Only then run `bbs foreman complete "$PARENT" --foreman "$FOREMAN_ID"`. No
   unreleased phase, missing gate, blocked ticket or unfinished cleanup can
   become DONE.

Report the result per phase — phase commit, release tag and verdict, QA
evidence — and remaining human action. Use `STATUS: DONE | IN_PROGRESS |
NEEDS_CONTEXT | BLOCKED`, link the parent `report.md`, and end with `Next:`
(a pending release go, kept worktrees, parked approvals, or none).
Distinguish landed-on-local-base from released (pushed and deployed).
Normal worker activity stays in Orca; report state changes, requests and
terminal evidence.
