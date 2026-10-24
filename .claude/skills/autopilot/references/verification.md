# Typed verification producer

The only writer of `review-pr` and `qa` gate evidence and of their
`verdicts/<gate>.md` projections. A checkpoint from before `schema_version: 2`
is refused with `CHECKPOINT_V1` (`recover` included; `snapshot` still renders
its facts but lists a lone `checkpoint` obligation); migrate it once with
`bbs autopilot checkpoint --refresh --ticket <id>` and never convert old
evidence. Run the real review-pr and qa skills in the current session; QA's mechanical
execution may go to its executor subagent (SKILL.md § Current-session gates),
whose `run-<n>.json` feeds `record`.

Never commit for a gate: the subject is the working tree's content, untracked
files included. Capture it **before** verification:

```sh
BABYSIT_TICKET="$TICKET" bbs autopilot verification begin --gate review-pr \
  --owner <actual-worker-id> --handle <actual-session-or-dispatch-id> \
  --transport <harness-or-orca> --harness <harness>
```

Use `--gate qa` for QA. The producer creates and binds a v2 attempt and returns
its ID. Keep that ID across compaction. Never fabricate runtime handles. The
attempt's assignment preserves the starting subject and its lifecycle preserves
the one-writer boundary. `attempt show`/`attempt update` remain the recovery path
for interrupted attempts; never cancel an unproven live worker.

Execute the skill and required checks, retaining real logs. Then record:

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

For review, include its written analysis as evidence alongside executed checks;
do not pretend reading a diff ran a test. The producer owns subject hashes,
attempt completion, immutable receipts, archived logs and verdict projections.
It records failures as failures. A successful `record` command means persisted,
not passed: re-read `bbs ticket readiness --action review --json` and require
`data.ready: true` after both gates.

If review/QA repairs changed the captured tree, begin fresh gates on the repaired tree.
The stale attempt becomes failed; it cannot bless a changed revision. A crash
after attempt completion can retry `record` with the same ID: the durable pending
envelope is reused. Neither worker nor coordinator reconstructs old evidence as
current. Emit structured progress at milestones/waits via `bbs foreman progress`;
the parent Task supplies its criterion IDs and integration obligations.
