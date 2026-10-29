# Lead initialization and recovery

Read at entry and cold resume; consult recovery only on a concrete trigger.
This is operational detail for the four-step Team flow, not a second loop.

## Initialize Babysit and Orca

1. Confirm the Orca runtime is reachable (`orca status --json`). The lead uses
   Orca only for worktrees and terminals (`orca agent launch`, `terminal
   list/show/read/send/close/wait`); the Task/Dispatch lifecycle is bbs's own
   (`bbs team task`), so no orchestration skill is loaded. If Orca is
   unavailable, report `BLOCKED`; no generic subagent substitute.
2. Direct skill invocation in an Orca terminal is the default entrypoint;
   `bbs team spawn` is optional, not a prerequisite. Resolve `FOREMAN_ID`
   from `--team-id <id>`, or choose a stable project id for a fresh run.
   Run `bbs team adopt "$FOREMAN_ID"` before other mutations; babysit detects
   its actual agent. If detection is unavailable, pass `--agent <current-agent>`.
   Pass `--plan-worker`, `--implement-worker`, and `--qa-worker` when the
   invocation named them; omitted means this lead's own harness. A bare
   `bbs team adopt` recovers the existing binding on continuation.
   Adoption failure is `BLOCKED`; never use `register` or borrow a live id.
3. Append `--auto` only when explicitly requested. Adoption persists it; read
   recorded `auto` on resume. It delegates project design review, not scope,
   holds, artifacts, QA or delivery authority. The current harness must already
   permit unattended tool use if no human will be present.
4. For free text, run `bbs ticket ensure --no-branch`, parse its parent ticket
   id (never eval the output), persist `requirement.md`, and initialize ticket
   state with `BABYSIT_TICKET="$PARENT" bbs ticket init`. A list of independent
   requests still gets one parent. For an existing child, recover its parent;
   for a bare wake, recover assignments from `bbs team inbox "$FOREMAN_ID"`.
5. Before launching or changing topology, claim the parent with
   `BABYSIT_TICKET="$PARENT" bbs ticket claim "$FOREMAN_ID"`. Another owner is
   a hard fence: remain read-only and report the owner. Different parents may
   run concurrently; one parent has one mutating lead.
6. Save Task, Dispatch, worktree and lease ids immediately after successful
   mutations; `bbs team task list "$FOREMAN_ID"` lists the Tasks bbs holds.
   Terminal handles are routing metadata, never recovery identity. Initialize
   the native task list from parent, children and DAG; rebuild it from
   ticket + task state on cold resume.

## Persistent run and bounded reconciliation

Use the harness's persistent goal facility when available. Its objective names
the lead id, parent, this skill and the final completion condition. Reuse a
compatible goal. A timeout, idle prompt, compaction or rate-limit pause is not
project completion. Compaction is a cold-resume boundary: reload the skill
and preamble, then recover durable state.

The detached `bbs team watch` is the external missed-event/restart backup;
adopt/spawn start it automatically. The lead never schedules its own status timer
or polls on empty waits; it hands each batch to `bbs team batch` and ends
its turn, and that watcher wakes it (SKILL.md). An external status/recovery reminder is a bounded wake,
not permission to start a new polling loop. `bbs team ensure <id>` can recover
a missing terminal; never resume an ambiguous "last" conversation.

Reconcile only for cold resume, an explicit status request, an external recovery
reminder, a concrete failure, or final completion:

1. Re-adopt the current session, heartbeat, re-claim the parent, read inbox and
   controls. A pause/cancellation stops new dispatch. On a cold resume, log a
   `--phase start` row to the [decision trail](delivery.md#decision-trail).
2. Read the relevant Tasks/Dispatches (`bbs team task list --open`) and
   actual worker terminals. On full recovery use `bbs team resource status`
   to reconcile leases. Silence never proves worker exit: only a terminal whose
   agent left does; retry only proven failed attempts (`task abandon` first).
   Never report `task done` for a worker or synthesize a PASS to repair
   inconsistent state.
3. Read parent approval/artifact revision, relations, manifests, checkpoints and
   verdicts. Check `bbs team snapshot` delivery rows before accessing removed
   worktrees. Preserve live writers; recreate only missing safe topology.
   A stale approval returns to the project checkpoint before new production work.
4. Refresh the parent `report.md` using the project contract. For a status request,
   show every project Task and supervised worker, delivery/QA/cleanup state,
   resource use and the DAG. Release settled orphans, launch every unlaunched
   ticket (dependents included) as one batch, hand it to `bbs team batch` and end the turn. Do not repeat heavy review or QA yourself.

Carry failure counts across retries/resumes. After three failed attempts at the
same blocker, mark that ticket blocked and continue independent ready work.

Worker reports and questions are durable bbs records: a missed wake is
replayed from them, never lost (`bbs team batch show` lists pending
questions with their answer command). A report for a superseded Dispatch is
refused and completes nothing. On `stop <project|ticket>`, `task abandon` the
open Dispatches, close the terminals they created and preserve work unless
removal was separately authorized.

## Intake and DAG

New accepted scope becomes durable ticket state before dispatch. A change after
work began gets a `change-request` child with explicit dependencies; do not rewrite
a settled ticket or silently replace a live assignment. Pause superseded work,
preserve its checkout, and continue unaffected work. Scope or revision changes
invalidate affected child gates and prior Integration QA.

Show `bbs ticket dag "$PARENT" --mermaid` when topology is built/changed or the
user requests status; print its output, never re-type the edges. Read-only: it
never writes ticket state. See topology.md for the bidirectional relations.

## Completion reporting

Use project-contract.md for the durable report and execution.md for typed evidence
and the final completion receipt. A `done` heartbeat alone is not completion.
The parent becomes `done` once every phase landed (and released, where its mode
asks). After successful `bbs team complete`, the external watcher drops the
the lead from its set and leaves its terminal open and idle — never closed,
nudged or prompted — so the human can read the recap and ask follow-ups there.
