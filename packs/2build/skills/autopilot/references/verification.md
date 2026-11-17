# Typed verification producer

The only writer of `review-pr` and `qa` gate evidence and of their
`verdicts/<gate>.md` projections. A checkpoint from before `schema_version: 2`
is refused with `CHECKPOINT_V1` (`recover` included; `snapshot` still renders
its facts but lists a lone `checkpoint` obligation); migrate it once with
`bbs autopilot checkpoint --refresh --ticket <id>` and never convert old
evidence. Run the real review-pr and qa skills in the current session; QA's mechanical
execution may go to its executor subagent (SKILL.md § Current-session gates),
whose `run-<n>.json` feeds `record`.

Never commit for a gate. **Nothing stales a recorded gate**: the subject
records head, base, tree, requirement, plan and policy for the audit trail
only, and editing code or docs while a gate runs, or after, is normal work.
Begin the gate first:

```sh
BABYSIT_TICKET="$TICKET" bbs autopilot verification begin --gate review-pr \
  --owner <actual-worker-id> --handle <actual-session-or-dispatch-id> \
  --transport <harness-or-orca> --harness <harness>
```

`--gate` is exactly `review-pr` or `qa`. The producer creates and binds a v2
attempt and returns its ID as `data.id`; while it is open the checkpoint keeps
it as `active_attempt_id`, so it survives compaction. Never fabricate runtime handles. The
attempt's assignment preserves the starting subject and its lifecycle preserves
the one-writer boundary: one open attempt per ticket, so record review before
beginning QA. `attempt show`/`attempt update` remain the recovery path
for interrupted attempts; never cancel an unproven live worker.

Execute the skill and required checks, retaining real logs. Then record, from
the ticket's checkout:

```sh
BABYSIT_TICKET="$TICKET" bbs autopilot verification record \
  --attempt <id> --file /absolute/results.json
```

```json
{
  "checks":[{"argv":["actual","check"],"cwd":"/absolute/repo","exit_code":0,"log_path":"/absolute/check.log"}],
  "unresolved_findings":[],
  "limitations":[]
}
```

The file takes exactly these three keys; any other key is rejected.
`limitations` names required coverage that could not run: any entry records
the gate as limited and readiness stays false. A note that blocks nothing
belongs in the review or QA body. Each check
needs an absolute `cwd` inside this checkout or the ticket directory, its real
`exit_code`, and a non-empty log file; at least one check is required. Review's
written analysis is not a field here: persist it with `set-review --body-file`,
and list the executed checks (type-check, tests) beside it; do not pretend
reading a diff ran a test. The producer owns subject hashes,
attempt completion, immutable receipts, archived logs and verdict projections.
It records failures as failures. A successful `record` command means persisted,
not passed: re-read `bbs ticket readiness --action review --json` and require
`data.ready: true` after both gates.

**One `review-pr --fix` pass is the review gate.** Run the checks on the
post-fix tree and `record` the same attempt. Never re-review a review's own
fixes, a QA fix or a repair, and never loop review until it finds nothing.
`active attempt <id> already owns this ticket` names an open attempt: if it is
this session's, record it or `attempt show --id <id>` and
fail it with `attempt update --id <id> --expect-revision <rev> --json
'{"state":"failed","failure":{"reason":"<why>"}}'`; never begin a second gate
beside it. A crash
after attempt completion can retry `record` with the same ID: the durable pending
envelope is reused. Neither worker nor coordinator reconstructs old evidence as
current. Emit structured progress at milestones/waits via `bbs team progress`;
the parent Task supplies its criterion IDs and integration obligations.
