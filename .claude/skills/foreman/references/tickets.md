# Tickets inside a phase — details

The wake loop in the skill is the summary; this is the detail behind it.

## Tickets inside a phase

Read [topology](topology.md) to create/reuse tickets, worktrees
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
[model routing](model-routing.md#assignment-routing). Each assignment
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
[project contract](project-contract.md#project-design-checkpoint).
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
   ([reuse launch](worker-routing.md#reuse-a-ticket-session)).
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
   `FOREMAN_MAX_WORKERS` ([topology](topology.md#resource-admission));
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
   `batch show` and, with no attention, the turn ends without consuming —
   no `check`, no ack, no reply to a stray heartbeat.
3. On that prompt run `bbs foreman batch show "$FOREMAN_ID"`; it ends with
   each owned project's `NEXT=` steps.
   `BATCH_STATE=settled` (failures included): consume with Orca `check` until
   it returns no messages (`check --ack <delivery>` returns the next Delivery
   of up to 50), processing every message before each ack, then verify, decide
   each session's owner, advance every phase that is ready (phases.md) and
   hand the next batch — tickets, integration, QA or release workers — over in
   the same wake.
   Otherwise it names attention — `messages:<n>` (an `L` plan review or a
   worker question), `exited:<task>`, `stalled:<task>` or `lost:<task>` (an
   open Dispatch with no live worker terminal through the stall budget);
   attention arriving within the watcher's `--attention-window` (2 min) is
   delivered as one wake:
   consume the Delivery and handle only that — answer the question, or follow
   the recovery contract for that worker (a lost Dispatch is a concrete
   failure: fence it with `worker-stop`/`worker-abandon` before relaunching)
   — then re-run `bbs foreman batch` with the same Tasks plus any new ones,
   and end the turn. The watcher follows a retried Task to its new Dispatch,
   outlives Orca restarts, and any `bbs` call made by a working agent restarts
   it if it died, so never babysit it.
4. The watcher never compacts Foreman; the agent compacts itself when its
   context fills. Keep durable state on disk anyway, so a wake after
   compaction recovers per [runtime](runtime.md).

Process every message before acknowledging its Delivery. Judge each member from
`bbs ticket verdict-status`, readiness JSON and the reported handoff paths; read
worker transcripts only when those disagree. Read controls/intake before new
dispatch or finish; `paused` or `cancelled` prevents new work. A user/status
request, external recovery reminder, cold resume or concrete failure uses the
bounded reconciliation in [runtime](runtime.md), not a competing
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
is not a passing gate. For code tickets, read current review/QA evidence
and `bbs ticket readiness --action review --json`;
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
| A contract a ticket pinned changed under it (`contract:<id>:changed`) | `refresh` on that ticket's session: re-read the contract, adapt, re-verify, re-pin |
| `integrate` refuses: the phase changes a contract another Foreman owns | ask the owner — a ticket assigned to it ([contracts](../references/contracts.md)) — and repair the ticket to build against the version on base |
| A parked plan approval was resolved | `redirect` on the parked session |
| Ticket reported `BLOCKED` after its own retry budget | Fresh read-only **critical** diagnosis worker, then `repair` with its findings |

A `repair` or other follow-up spec should end with "record the `review-pr` and
`qa` gates on the final tree", so a waiver (`bbs foreman seal --waive`,
[delivery](delivery.md)) stays the exception rather than the way a worker
skips its gates. If a worker still did not re-record them but its handoff
shows the verification, seal with `--waive` and a reason instead of launching
a worker only to re-stamp gates.

As soon as a ticket passes, seal it (`bbs foreman seal`, several tickets in
one call): its uncommitted tree
becomes the snapshot its phase is built from. Nothing about a ticket is
committed, pushed or PR'd on its own; a ticket reaches base only inside its
phase commit ([delivery](delivery.md)).

Once a ticket's phase has landed, archive worker output, release the ticket
session and resource leases, and close the ticket's Orca terminals/worktree
surfaces; `bbs foreman land` removed its worktree, or reported it kept because
it changed after the seal. Cleanup need not wait for the release. Never close
an active/unverifiable worker or delete kept work.

