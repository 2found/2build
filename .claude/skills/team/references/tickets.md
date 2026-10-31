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
batch → lightly verify → finish (the worker is shut down when it reports done; a follow-up starts a fresh session)**. The
ticket worker follows autopilot's **Team ticket assignment** contract:

| Loop step (one session) | Boundary |
|---|---|
| Plan | `XS`/`S` build from the ticket brief and record `ticket_size`; `M`/`L` run `plan-draft` for only what the brief leaves open |
| Plan approval | The worker self-approves; see [plan approval](#plan-approval) |
| Await prerequisites | `bbs team await` until every prerequisite ticket settles; no-op for a root ticket |
| Adopt prerequisites | `await` adopts each sealed prerequisite snapshot into the worktree as uncommitted changes; resolve `CONFLICT=` files, then reconcile the plan with the adopted code |
| Implement | Code and focused checks (typecheck, lint, unit tests of the touched code), left uncommitted; report as soon as they pass |
| Review, QA | Left to the phase, which reviews its diff once and QAs every criterion on the phase commit. Only a ticket whose paths hit the money/auth/irreversible-data floor runs `review-pr --fix` on its own diff; deferred heavy checks only recorded |
| Report | Readiness, handoff and exactly one `bbs team task done`, which refuses success until readiness passes; no commit, push or PR |

The route is chosen once per ticket (phase class `critical`, because the session
owns its own planning and review) and reused by every follow-up; see
[model routing](model-routing.md#assignment-routing). Create its Task with
`--kind ticket` (`repair`, `refresh` or `redirect` for a follow-up) so `task
done` holds the session to its gates. Each assignment includes the ticket's
**slice** of the accepted parent artifacts — its ticket brief
(`design.md#<seed key>`), the prototype ids it owns, and the plan and
manifest rows it owns, by path and heading, with the whole documents linked
for lookup rather than read up front: whatever a worker reads first it re-reads
from cache on every later turn, and whole parent documents measured about 35k
tokens of each worker's base context — the ticket requirement/seed, size and
acceptance IDs, its prerequisites as `<ticket>:<task>` pairs (bbs Task ids),
owned files and contracts, review effort, deferred heavy checks, selected
route, resource lease and required evidence, plus the
[worker lifecycle](worker-lifecycle.md): the worker is already spawned,
executes immediately, and reports exactly one `bbs team task done` per
Dispatch. Missing inputs come through `bbs team task ask`, answered with
`task answer` while the same live session waits — not a local human prompt.

### Plan approval

The plan step flows into implementation inside the same session; `XS` and
`S` tickets have no plan step (an `S` touching money, auth or irreversible
data plans as `M`), and every `M`/`L` plan is self-approved by its worker as
the lead's delegate. A plan reaches the lead only on the floor (`self-resolve`
exit 3/5), a third refusal, or a material deviation from the brief or a
shared contract; the lead then reads `plan.md` against the brief, acceptance
IDs and contracts — not the code — and answers `approved` or the exact
change. After two redirects the lead approves with named conditions; a
disagreement is never a reason to wait on the human. The exact commands and
refusal handling are in
[project contract](project-contract.md#project-design-checkpoint).

### Ticket session ownership

The ticket session is the ticket's single writer while it works: questions and
plan reviews go to the live session. A worker that reports `task done` is shut
down at once — the watcher closes its terminal when the Dispatch settles
(succeeded, failed or abandoned), because a follow-up to a finished worker
almost never happens and an idle agent costs memory and tokens. Opt out with
config `foreman_keep_settled_workers: true` (debugging only). Land closes any terminal still left in the phase's worktrees.

After every accepted report, the next owner is therefore always a **fresh
session**:

1. **Follow-up** — a `repair` (verification gap, review/QA finding,
   heavy/integration failure, a phase that does not compose), a `refresh` (a
   prerequisite changed or settled after this ticket stopped waiting for it)
   or a `redirect` (a parked or redirected plan was answered): create the
   follow-up Task (`--kind repair|refresh|redirect`) and start a fresh worker
   in the ticket worktree on the ticket's recorded route. The spec points at
   the ticket's handoffs (numbered handoff paths, `plan.md`, the last report
   and verdicts) so the new session recovers from durable state without
   re-reading the codebase. Several triggers for one ticket in the same wake
   — a review finding and a prerequisite refresh — go into **one** follow-up,
   so the ticket pays one cold start, not one per trigger.
2. **Nothing** otherwise; the ticket closes with its phase.

Release the Dispatch's resource lease on every settlement; a reused session
reserves a fresh one. A follow-up for a ticket whose session is gone or not
provably live starts a fresh worker in the ticket worktree on the ticket's
recorded route, recovering from durable state. Never reuse one ticket's session
for another ticket, and never send a follow-up to a session whose Dispatch is
still open.

### Batch dispatch: one wake per phase

Every coordinator wake re-reads the whole lead context, so wakes, not worker
count, dominate cost. Dependent tickets therefore launch **with** their
prerequisites and wait inside their own sessions — planning and approving
while the prerequisites build — instead of waking the lead once per DAG wave.
The bbs watcher waits and supervises:

1. A batch is every ticket of the open phases not yet launched, in
   dependency order (a ticket's
   prerequisites are already in the batch or done), up to
   `FOREMAN_MAX_WORKERS` ([topology](topology.md#resource-admission));
   ready tickets and light profiles take the slots first, so a waiting
   dependent never displaces runnable work. Never hold back ready work to
   align finish times.
2. Launch every member, run `bbs team batch "$FOREMAN_ID" --tasks
   <task,task,...>`, then **end the turn**. Do not wait, actively check
   terminals, checkpoints, git state, inbox, resource status or
   project snapshots for progress, start a polling timer, invoke `watch --once`
   or restart a live worker. The detached `bbs team watch` nudges a worker
   whose pane sits idle with its Dispatch open (bounded), holds the lead's own
   status/stall prompts while the batch is open, and wakes the lead with one
   prompt naming the batch. Workers report into the bbs task store, never
   into the lead's terminal, so nothing else types into the lead mid-batch.
3. On that prompt run `bbs team batch show "$FOREMAN_ID"`; it ends with
   each owned project's `NEXT=` steps.
   `BATCH_STATE=settled` (failures included): read each member's report
   from its line (ticket, kind, gate, summary, report path), then verify, decide
   each session's owner, advance every phase that is ready (phases.md) and
   hand the next batch — tickets, integration, QA or release workers — over in
   the same wake.
   Otherwise it names attention — `messages:<n>` (an escalated plan or a
   worker question), `exited:<task>`, `stalled:<task>` or `lost:<task>` (an
   open Dispatch with no live worker terminal through the stall budget);
   attention arriving within the watcher's `--attention-window` (2 min) is
   delivered as one wake:
   handle only that — answer each `QUESTION=` with the `ANSWER_WITH=` command
   it prints, or recover that worker (an exited, stalled or lost Dispatch is a
   concrete failure: `bbs team task abandon` it and close its terminal
   before relaunching)
   — then re-run `bbs team batch` with the same Tasks plus any new ones,
   and end the turn. The watcher follows a retried Task to its new Dispatch,
   outlives Orca restarts, and any `bbs` call made by a working agent restarts
   it if it died, so never babysit it.
4. The watcher never compacts the lead; the agent compacts itself when its
   context fills. Keep durable state on disk anyway, so a wake after
   compaction recovers per [runtime](runtime.md).

Judge each member from
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
build on it once its review passes. The lead then
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

Check the expected Task/Dispatch settled `completed` for the current
Dispatch, then the ticket's artifacts, plan approval record, revision and
verdicts. A report alone is not a passing gate. For code tickets, read current review/QA evidence
and `bbs ticket readiness --action review --json`;
require `ok` and `data.ready`, and confirm each adopted prerequisite snapshot
is still the one that prerequisite was sealed at. For evidence-only tickets,
check their artifacts and acceptance coverage. The lead does not repeat code
review, rerun tests or implement fixes; any gap is a `repair` follow-up naming
the exact evidence. Changed inputs invalidate gates.

| Trigger | Next owner |
|---|---|
| Light verification gap, heavy QA or phase QA failure | `repair` on the ticket session (fresh ticket worker if gone) |
| `integrate` reports the phase does not compose | `repair` on the later ticket named: `bbs ticket adopt <earlier>`, resolve, re-run its gates |
| A prerequisite was resealed after a dependent adopted it, or a dependent stopped on `AWAIT=not-ready` and the prerequisite now passes | `refresh` on each affected dependent session, in dependency order |
| A contract a ticket pinned changed under it (`contract:<id>:changed`) | `refresh` on that ticket's session: re-read the contract, adapt, re-verify, re-pin |
| `integrate` refuses: the phase changes a contract another team owns | ask the owner — a ticket assigned to it ([contracts](../references/contracts.md)) — and repair the ticket to build against the version on base |
| A parked plan approval was resolved | `redirect` on the parked session |
| Ticket reported `BLOCKED` after its own retry budget | Fresh read-only **critical** diagnosis worker, then `repair` with its findings |

A `repair` or other follow-up spec should end with "record the `review-pr` and
`qa` gates on the final tree", so a waiver (`bbs team seal --waive`,
[delivery](delivery.md)) stays the exception rather than the way a worker
skips its gates; `task done` refuses a gated kind's success until they pass,
so a missing gate is fixed by the warm session instead of a cold one. A member
whose line shows `GATE="skipped: …"` or `GATE="unverified: …"` gets the
light verification below before it is sealed. If a worker still did not
re-record them but its handoff shows the verification, seal with `--waive` and
a reason instead of launching a worker only to re-stamp gates.

As soon as a ticket passes, seal it (`bbs team seal`, several tickets in
one call): its uncommitted tree
becomes the snapshot its phase is built from. Nothing about a ticket is
committed, pushed or PR'd on its own; a ticket reaches base only inside its
phase commit ([delivery](delivery.md)).

Once a ticket's phase has landed, archive worker output, release the ticket
session and resource leases, and close the ticket's Orca terminals/worktree
surfaces; `bbs team land` removed its worktree, or reported it kept because
it changed after the seal. Cleanup need not wait for the release. Never close
an active/unverifiable worker or delete kept work.

