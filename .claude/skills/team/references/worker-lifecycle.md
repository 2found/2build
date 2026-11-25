# Team worker transport

The lead owns this adapter; member skills do not load this reference. A
Task spec (`bbs team task create --spec-file`) carries only the assignment
and its stop boundary: `bbs team task show <task> --spec` adds the
transport header (`SPAWNED=true`, `AGENT_ROLE=orca`, Task, ticket, lead, the
`BABYSIT_PROJECT_HOME`/`BABYSIT_TICKET` export), the lifecycle below in
short, and — on a relaunched Task — a Resume section naming the earlier
Dispatches. The outer worker applies the lifecycle once around its assigned
skill, including on resume or failure. The launch prompt names the Task and
Dispatch ids; `bbs team task show <task>` is the authority for them, never
env or ticket guesses.

## Worker lifecycle

The worker talks to bbs, never to an Orca mailbox. Orca only hosts its
terminal and worktree: there is no orchestration skill to load, no Run to
join, and no heartbeat.

- **Ask** with `bbs team task ask <task> --body "<question>"`; no
  `AskUserQuestion` in a worker. It blocks until the lead answers (default 540s,
  shorter for an agent whose prompt cache lives five minutes, so each re-run
  resumes warm).
  Exit 4 with `QUESTION_PENDING=<id>` leaves the question pending: keep
  waiting with `--resume <id>`, never a new ask. Ask only for a decision
  outside the accepted plan or assignment (an escalated plan, an out-of-plan
  finding, a scope or contract change, a blocker it cannot resolve); anything
  the plan already decides, the worker decides and records in its handoff.
- **Send no status messages.** The bbs watcher owns liveness (pane plus
  Dispatch state) and wakes the lead once per batch; milestones go to
  `bbs team progress` ([shape](#progress-reports)) only when the assignment asks.
- **Report once** for the whole Dispatch, not each nested skill: persist
  handoff/verdicts, then `bbs team task done <task> --dispatch <id>
  --outcome succeeded|failed --summary "<did; found; left>"`. Use `succeeded`
  for DONE/DONE_WITH_CONCERNS, `failed` when ending as BLOCKED/NEEDS_CONTEXT.
  A pending ask is not a report. `--report-path` / `--files-modified` name
  real artifacts. A retried identical report is harmless; a contradicting one
  is refused. For a ticket session (Task kind `ticket`, `repair`,
  `redirect`) a `succeeded` report is refused with `GATE=not-ready` and the
  missing readiness reasons until the ticket passes readiness: record the
  missing gates on the final tree and report again. `--no-gate "<why>"` only for a gate that cannot
  apply; `--outcome failed` when the work did not land. A succeeded report of
  a project ticket snapshots its uncommitted tree for the phase (an
  artifact-only ticket adds `--artifacts <paths.json>`); a snapshot error
  refuses the report — fix the worktree (on its ticket branch, one repo) and
  report again.
- A **phase session** (Task kind `phase`) reports twice: `bbs team task
  built <task> --dispatch <id>` once its focused checks pass (the ticket gate
  and snapshot above, the Dispatch stays open), then — after it integrated,
  landed, reviewed, QA'd and fixed its phase — `task done`, which is refused
  with `GATE=not-ready` until the phase is verified.
- A `Continue Task <id>` prompt is the watcher's idle nudge: resume the
  assignment, or run its pending `task done` if it is already complete.
- A worker is shut down when it reports
  ([tickets](tickets.md#ticket-session-ownership)), so a follow-up Dispatch
  starts a fresh session that reads the ticket's handoffs. A session that
  survives (the debugging opt-out) and receives a new launch prompt carries a
  new Dispatch id: apply this lifecycle afresh, keep its context, re-read the
  durable state the follow-up names, and never report for a settled Dispatch.
- After reporting, end the turn and idle. The lead reads
  `bbs ticket verdict-status`; the report does not substitute for a verdict.
  Any other `task done` error is a reporting failure: keep the artifacts and
  return the shared preamble's structured NEEDS_CONTEXT, never invent a
  Dispatch.

The `worker-report-gate` Stop hook reminds once per stop while a Dispatch
launched into this terminal is still open (or its question is unanswered)
and names the command to run; the stop after it ends freely, and the watcher
settles a worker that never reports. It neither reports for the worker nor
waits for the lead.

## Progress reports

Write `bbs team progress <ticket> --file <report.json>` at a meaningful
milestone or wait transition. Use the actual Task/Dispatch/attempt IDs
(`run_id` carries the bbs Task id):

```json
{"producer":"worker-id","phase":"qa","summary":"Task survives reload",
 "run_id":"run-id","dispatch_id":"dispatch-id","attempt_id":"attempt-id",
 "evidence":"/absolute/durable/check.log"}
```

`phase` names the ticket's loop step (`plan`, `approve`, `build`, `review`,
`qa`, `repair`), not a release phase. An `approve` wait on the lead is
`wait_kind: approval`.
Only changed evidence advances `progress_at`; summary rewrites and extra edits
do not. Waits add `wait_kind` (`test`, `approval`, `resource`),
`wait_reason` and `wait_until` at most 15 minutes ahead. Refresh from actual
ongoing waits, never a timer. Expiry is not failure or a reason to inspect a
worker; Orca owns runtime state and runtime.md defines reconciliation triggers.
