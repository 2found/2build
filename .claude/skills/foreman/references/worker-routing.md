# Worker launch and route persistence

Read the canonical contract in [model routing](references/model-routing.md)
for task/phase classification, tiers, ticket overrides and OMP role bindings.
For Orca-supervised Foreman Dispatches, this reference owns worker agent
selection and resume. New routes use explicit intent (`--plan-worker` / `--implement-worker` /
`--qa-worker`, default this Foreman's harness) or a compatible ticket pin; BBS
preferences do not select an agent.

## Resolve and launch

Classify task complexity and phase class using the canonical contract. Select
an agent from `--plan-worker` (basic design), `--implement-worker` (ticket
sessions), or `--qa-worker` (phase QA / integration). Omitted means this
Foreman's own harness. Then obtain model/effort and tier with
`bbs foreman model --dir "$REPO" --agent <selected-agent> --complexity <simple|normal|hard> --phase-class <normal|critical|design> --json`.
Do not pass `--quota` and do not replace the selected agent from usage.
Use the assignment repo/worktree, not the coordinator repo in a multi-repo run.
Explicit ticket overrides and valid persisted resume routes take precedence;
skip the policy lookup when resuming an exact recorded session. Pass the
resulting model, effort and selected tier into route validation before resource
admission:

```bash
bbs foreman route --ticket "$TICKET" --task "$ORCA_TASK_ID" \
  [--agent <explicit-agent>] [--pinned-agent <ticket-agent>] \
  [--pinned-model <ticket-model>] [--pinned-effort <ticket-effort>] \
  [--model <selected-model>] [--effort <selected-effort>] [--host <host-id>] \
  --complexity <simple|normal|hard> --phase-class <normal|critical|design> \
  [--hard-reason <money|auth|security|live-data|concurrency>] \
  --selected-tier <lite|flash|pro|max> [--override-provenance <source|none>] \
  [--pinned-model-provenance <ticket-pointer>] [--pinned-effort-provenance <ticket-pointer>]
```

The route order is explicit intent, compatible ticket pin, then the
destination-host `agentDiscovery.effectiveDefaultAgent`. The command validates
enabled/runnable state when discovery is available and persists the requested
route and pin provenance. It also returns `launchMode`: `worker-start` validates
Orca's advertised model/effort forwarding; OMP uses `native-terminal` because
Orca does not forward OMP launch overrides. Never substitute coordinator-local
detection, retired BBS preferences, or a generic profile default. If discovery
is unavailable, explicit and pinned routes remain usable; a new unpinned route
must stop with its actionable Orca upgrade/configuration error.

Pass the selected complexity, phase class and tier on every route. Use
`none` when no ticket-specific model/effort override was chosen; otherwise name
the override source. Pin provenance identifies the pointer that supplied each
pin, such as `worker_model` or `reviewer_effort`.

Use the returned `agent`, `model`, `effort` and `launchMode`, plus observed
`hostId` or requested `destinationHost`, for resource reservation and launch.
Reserve before creating a terminal. Omit unset fields in the examples below.

### Managed launch: `worker-start`

```bash
orca orchestration worker-start --task "$ORCA_TASK_ID" --worktree current \
  --agent <selected-agent> --model <selected-model> --effort <selected-effort> --json

bbs foreman route verify --ticket "$TICKET" --task "$ORCA_TASK_ID" \
  --agent <selected-agent> [--host <host-id>] [--model <model>] [--effort <effort>] \
  --receipt-file "$WORKER_START_RECEIPT"
```

`--effort` requires `--model`; neither combines with `--terminal`. Do not
assume `--provider` support or support for every agent. Verify the actual
receipt, not a successful exit alone: `matched` requires ready/accepted input
and matching effective agent and every requested override.

### OMP launch: `native-terminal`

Resolve the selected alias on the execution host before launch, using its live
OMP role binding. Put the expected resolved provider/model/effort in the Task
spec. Require the worker to compare actual session metadata before ticket work;
unknown or conflicting settings must be reported, never repaired by switching
the live model. An omitted effort uses the selected role's native binding.

Render an idle native command, create an Orca-owned terminal in the assignment
worktree, then supervise that exact terminal:

```bash
bbs foreman worker-command --startup-only --dir "$REPO" \
  --agent omp --model <selected-model> [--effort <selected-effort>]

orca terminal create --worktree <assignment-worktree> \
  --command "<exact rendered startup command>" --json

orca orchestration worker-start --task "$ORCA_TASK_ID" \
  --worktree <assignment-worktree> --terminal "$TERMINAL" --json

bbs foreman route verify --ticket "$TICKET" --task "$ORCA_TASK_ID" \
  --agent omp --terminal "$TERMINAL" --receipt-file "$WORKER_START_RECEIPT"
```

`--startup-only` rejects `--prompt` and `--skill`. Do not send the assignment
early, use `dispatch --inject`, or pass agent/model/effort to terminal reuse.
Execute native preflight and startup on the selected execution host; verify the
created terminal's worktree and execution host before reuse. Remote placement
must be supported explicitly; never fall back to a coordinator-local launch.

Orca reports null effective agent/model/effort for a reused terminal.
`transport-matched` verifies ready/accepted input, reuse of the exact recorded
terminal, and accepted dispatch input to that terminal; it does not attest
native settings. Preserve null fields and separately persist the worker's
observed agent/model/effort. Passing an unreported model/effort/host to receipt
verification fails; do not synthesize effective fields from the startup argv.

Record the exact startup command and returned terminal handle, with whether
this Foreman created it. After settlement, call `worker-release`. Orca can retain
reused terminals as `external_terminal`; close the exact retained terminal only
when this Foreman created it and the Dispatch is settled. Never close a
preexisting user's terminal or an uncertain live worker. Apply the same ownership
rule after a failed launch, then release the resource lease.

### Reuse a ticket session

A settled worker's terminal is closed by the watcher, so this applies only to a
session that is still live (watcher opted out with `foreman_keep_settled_workers`,
or an explicit `worker-retain` for debugging). Otherwise start a fresh worker in
the ticket worktree and point its spec at the ticket's handoffs. A follow-up on
a ticket (`repair`, `refresh`, `redirect`, `deliver`) goes to
that ticket's existing session when Orca proves it reusable; it inherits the
ticket route, so skip model lookup and route resolution. Reuse only when the
previous Dispatch on that terminal settled with an accepted report, the handle
is the exact one recorded for this ticket, and `worker-list` shows it `live`:

```bash
orca orchestration worker-show --dispatch "$LAST_DISPATCH" --json
orca orchestration task-create --spec "<follow-up assignment>" --json
# reserve the ticket's resource profile for the new Task first (topology.md)
TICKET_WORKTREE=$(BABYSIT_TICKET="$TICKET" bbs ticket path worktree --read)
orca orchestration worker-start --task "$FOLLOWUP_TASK" \
  --worktree "path:$TICKET_WORKTREE" --terminal "$TICKET_TERMINAL" --json

bbs foreman route verify --ticket "$TICKET" --task "$FOLLOWUP_TASK" \
  --agent <ticket-agent> --terminal "$TICKET_TERMINAL" --receipt-file "$WORKER_START_RECEIPT"
```

`--terminal` takes no model or effort: the live session keeps the ticket route.
It does need `--worktree` naming that terminal's ticket worktree; without it
Orca binds the Dispatch to the coordinator's (`current`) worktree.
Require `transport-matched`. A refused reuse, a non-live or unverifiable
session, or a ticket whose route must change starts a fresh worker in the ticket
worktree on the recorded route instead; never type a follow-up into a pane by
hand. The reused terminal's cleanup ownership moves to the new Dispatch.

Both launch modes persist sanitized receipt evidence. A mismatch is a failed
launch. If `worker-start` reports rate limiting, persist it with
`bbs foreman route verify ... --agent <selected-agent> --rate-limited`, re-read
quota snapshots before retrying, and never switch accounts or downgrade tiers.

## Persist and resume

Record task complexity, phase class, selected tier, override provenance,
requested settings, model/effort pins and their pointer provenance, and the
exact-session marker in the numbered ticket handoff written by `bbs foreman route`.
Include the route result, sanitized receipt, native-terminal ownership record
and separately observed session settings in the corresponding Orca Task/Dispatch
handoff, including planning, finish audits, integration and delivery.
`route verify` also writes a numbered ticket handoff.

A ticket has one route, persisted on it under the `worker_` prefix with
`bbs ticket set-pointer`: `worker_agent`, `worker_provider`, `worker_model`,
`worker_effort`, for example
`BABYSIT_TICKET="$TICKET" bbs ticket set-pointer worker_model "<model-or-role>"`.
Pass these pins to `bbs foreman route` for a fresh ticket worker; provider is not
a `worker-start` selector unless Orca explicitly advertises support. Legacy
`planner_`, `reviewer_` and `qa_` pointers from split-step tickets are read only
to resume their exact recorded sessions. Persist other assignments (basic
design, phase integration, heavy QA, release, diagnosis, fresh delivery) in their Task/Dispatch
handoffs rather than overwriting a ticket route.

Apply the canonical resume policy before the next launch: exact native-session
resume requires the recorded agent and settings plus `--exact-session`; never
route a recorded session through today's effective default. For a new Dispatch,
reuse a compatible ticket pin; otherwise resolve the destination-host default.
Never change a live worker's model. Every Dispatch, including a follow-up on a
reused session, reserves its own lease and releases it at settlement; whether
the session is then closed is automatic: the watcher shuts a worker down when
its Dispatch settles (SKILL.md).
