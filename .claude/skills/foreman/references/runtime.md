# Foreman initialization and recovery

Read at entry and cold resume; consult recovery only on a concrete trigger.
This is operational detail for the four-step Foreman flow, not a second loop.

## Initialize Babysit and Orca

1. Read `~/.claude/skills/orchestration/SKILL.md` and resolve its executable.
   Run `ORCA skills get orchestration` with that executable before other Orca
   commands; follow the version-matched guide. Confirm runtime reachability and
   orchestration support. If unavailable, report `BLOCKED`; no pane polling or
   generic subagent substitute. Recover lost mutations through request recovery
   receipts before retrying them.
2. Direct skill invocation in an Orca terminal is the default entrypoint;
   `bbs foreman spawn` is optional, not a prerequisite. Resolve `FOREMAN_ID`
   from `--foreman-id <id>`, or choose a stable project id for a fresh run.
   Run `bbs foreman adopt "$FOREMAN_ID"` before other mutations; babysit detects
   its actual agent. If detection is unavailable, pass `--agent <current-agent>`.
   A bare `bbs foreman adopt` recovers the existing binding on continuation.
   Adoption failure is `BLOCKED`; never use `register` or borrow a live id.
3. Append `--auto` only when explicitly requested. Adoption persists it; read
   recorded `auto` on resume. It delegates project design review, not scope,
   holds, artifacts, QA or delivery authority. The current harness must already
   permit unattended tool use if no human will be present.
4. For free text, run `bbs ticket ensure --no-branch`, parse its parent ticket
   id (never eval the output), persist `requirement.md`, and initialize ticket
   state with `BABYSIT_TICKET="$PARENT" bbs ticket init`. A list of independent
   requests still gets one parent. For an existing child, recover its parent;
   for a bare wake, recover assignments from `bbs foreman inbox "$FOREMAN_ID"`.
5. Before binding a Run or changing topology, claim the parent with
   `BABYSIT_TICKET="$PARENT" bbs ticket claim "$FOREMAN_ID"`. Another owner is
   a hard fence: remain read-only and report the owner. Different parents may
   run concurrently; one parent has one mutating Foreman.
6. Create or bind one Orca Run and persist `pointers.orca_run`. Save Task,
   Dispatch, branch/worktree and lease ids immediately after successful mutations.
   Terminal handles are routing metadata, never recovery identity. Resolve
   `bbs autopilot git-flow` for `BBS_FINISH=review | land | pr` and profile-derived
   review/QA breadth. Initialize the native task list from parent, children and
   DAG; rebuild it from ticket + Orca state on cold resume.

## Persistent run and bounded reconciliation

Use the harness's persistent goal facility when available. Its objective names
the Foreman id, parent, this skill and the final completion condition. Reuse a
compatible goal. A timeout, idle prompt, compaction or rate-limit pause is not
project completion. Compaction is a cold-resume boundary: reload the skill,
preamble and live Orca guide, then recover durable state.

The detached `bbs foreman watch` is the external missed-event/restart backup;
adopt/spawn start it automatically. Foreman never schedules its own status timer
or polls on empty waits. An external status/recovery reminder is a bounded wake,
not permission to start a new polling loop. `bbs foreman ensure <id>` can recover
a missing terminal; never resume an ambiguous "last" conversation.

Reconcile only for cold resume, an explicit status request, an external recovery
reminder, a concrete failure, or final completion:

1. Re-adopt the current session, heartbeat, re-claim the parent, read inbox and
   controls, then bind the recorded Run. A pause/cancellation stops new dispatch.
2. Read the relevant Tasks/Dispatches and actual worker state. On full recovery
   use `bbs foreman resource status` to reconcile leases. Silence or an expired
   heartbeat never proves worker exit; retry only proven failed/stopped attempts.
   Never synthesize `worker_done` or a PASS to repair inconsistent state.
3. Read parent approval/artifact revision, relations, manifests, checkpoints and
   verdicts. Check finish receipts before accessing removed worktrees. Preserve
   live writers; recreate only missing safe topology from recorded branches.
   A stale approval returns to the project checkpoint before new production work.
4. Refresh the parent `report.md` using the project contract. For a status request,
   show every project Task and supervised worker, delivery/QA/cleanup state,
   resource use and the DAG. Release settled orphans, dispatch newly ready work
   and return to the blocking wait. Do not repeat heavy review or QA yourself.

Carry failure counts across retries/resumes. After three failed attempts at the
same blocker, mark that ticket blocked and continue independent ready work.

Orca enqueue is durable; its attention nudge is best-effort, not proof Foreman
read the message. Process every message in a Delivery before acknowledging its
exact id; let unacknowledged deliveries replay on recovery. A `worker_done` needs
accepted lifecycle settlement for the expected Dispatch. Rejected lifecycle
reports do not complete work. On `stop <project|ticket>`, follow the live Orca
stop/retain protocol and preserve work unless removal was separately authorized.

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
Under local land, the parent becomes `done`; under retained review or open PRs it
remains `in_review` while the coordinator can complete. Never infer remote merge
from a local ancestor test. After successful `bbs foreman complete`, the external
watcher closes only the adopted Foreman terminal after its delivery grace.
