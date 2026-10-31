# Team worker transport

The lead owns this adapter. Include these instructions in every worker Task
spec (`bbs team task create --spec-file`); member skills do not load this
reference. The outer worker applies the lifecycle once around its assigned
skill, including on resume or failure.

Declare `SPAWNED=true` and `AGENT_ROLE=orca` in the assignment, with the Task
id, the ticket and stop boundary, and this escalation channel. The launch
prompt names the Task and Dispatch ids; `bbs team task show <task>` is the
authority for them, never env or ticket guesses.

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
  `bbs team progress` only when the assignment asks.
- **Report once** for the whole Dispatch, not each nested skill: persist
  handoff/verdicts, then `bbs team task done <task> --dispatch <id>
  --outcome succeeded|failed --summary "<did; found; left>"`. Use `succeeded`
  for DONE/DONE_WITH_CONCERNS, `failed` when ending as BLOCKED/NEEDS_CONTEXT.
  A pending ask is not a report. `--report-path` / `--files-modified` name
  real artifacts. A retried identical report is harmless; a contradicting one
  is refused. For a ticket session (Task kind `ticket`, `repair`, `refresh`,
  `redirect`) a `succeeded` report is refused with `GATE=not-ready` and the
  missing readiness reasons until the ticket passes readiness: you still hold
  the context, so record the missing gates (`review-pr`, `qa`) on the final
  tree and report again. `--no-gate "<why>"` only for a gate that cannot
  apply; `--outcome failed` when the work did not land.
- A `Continue Task <id>` prompt is the watcher's idle nudge: resume the
  assignment, or run its pending `task done` if it is already complete.
- A worker is shut down when it reports (the watcher closes its terminal at
  settle), so a follow-up Dispatch (`repair`, `refresh`, `redirect`,
  `deliver`) normally starts a fresh session in the ticket worktree and reads
  the ticket's handoffs. Should a session survive (the
  `foreman_keep_settled_workers` debugging opt-out) and receive a new launch prompt, it carries a new Dispatch
  id: apply this lifecycle afresh, keep the context it holds, re-read the
  durable state the follow-up names, and never report for a settled Dispatch.
- After reporting, end the turn and idle. The lead reads
  `bbs ticket verdict-status`; the report does not substitute for a verdict.
  Any other `task done` error is a reporting failure: keep the artifacts and
  return the shared preamble's structured NEEDS_CONTEXT, never invent a
  Dispatch.

The `worker-report-gate` Stop hook blocks a Stop while a Dispatch launched
into this terminal is still open (or its question is unanswered) and names
the command to run; it neither reports for the worker nor waits for the lead.
