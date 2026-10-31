# Worker launch and route persistence

Read the canonical contract in [model routing](references/model-routing.md)
for task/phase classification, tiers, ticket overrides and OMP role bindings.
For the lead Dispatches, this reference owns worker agent selection, launch
and resume. New routes use explicit intent (`--plan-worker` / `--implement-worker` /
`--qa-worker`, default this lead's harness) or a compatible ticket pin; BBS
preferences do not select an agent.

## Resolve and launch

Classify task complexity and phase class using the canonical contract. Select
an agent from `--plan-worker` (basic design), `--implement-worker` (ticket
sessions), or `--qa-worker` (phase QA / integration). Omitted means this
the lead's own harness. Then obtain model/effort and tier with
`bbs team model --dir "$REPO" --agent <selected-agent> --complexity <simple|normal|hard> --phase-class <normal|critical|design> --json`.
Do not pass `--quota` and do not replace the selected agent from usage.
Use the assignment repo/worktree, not the coordinator repo in a multi-repo run.
Explicit ticket overrides and valid persisted resume routes take precedence;
skip the policy lookup when resuming an exact recorded session. Pass the
resulting model, effort and selected tier into route validation before resource
admission:

```bash
```bash
bbs team route --ticket "$TICKET" --task "$TASK" \
  [--agent <explicit-agent>] [--pinned-agent <ticket-agent>] \
  [--pinned-model <ticket-model>] [--pinned-effort <ticket-effort>] \
  [--model <selected-model>] [--effort <selected-effort>] [--host <host-id>] \
  --complexity <simple|normal|hard> --phase-class <normal|critical|design> \
  [--hard-reason <money|auth|security|live-data|concurrency>] \
  --selected-tier <lite|flash|pro|max> [--override-provenance <source|none>] \
  [--pinned-model-provenance <ticket-pointer>] [--pinned-effort-provenance <ticket-pointer>]
```

`$TASK` is the bbs Task the worker will run: create it first with
`bbs team task create "$FOREMAN_ID" --ticket "$TICKET" --kind <kind> --spec-file <spec>`
(the spec carries only the assignment; bbs adds the transport and lifecycle of
[worker-lifecycle.md](worker-lifecycle.md) when the worker reads it).
`--kind` is required: `ticket` for a ticket session and `repair|refresh|redirect`
for its follow-ups (their `task done` is refused until the ticket's readiness
passes); any other name — `phase-qa`, `diagnosis`, `product-review`, `release`
— is ungated.

The route order is explicit intent, compatible ticket pin, then the
destination-host `agentDiscovery.effectiveDefaultAgent`. The command validates
enabled/runnable state when discovery is available and persists the requested
route and pin provenance. It also returns `launchMode`: `agent-launch` validates
Orca's advertised model/effort overrides; OMP uses `native-terminal` because
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
Reserve before launching. Omit unset fields in the examples below.

### Launch: `agent-launch`

```bash
bbs team task launch "$TASK" --agent <selected-agent> \
  --worktree <assignment-worktree> [--model <selected-model>] [--effort <selected-effort>] \
  [--host <host-id>] [--skill autopilot]
```

`task launch` opens the Task's next Dispatch, starts the agent with
`orca agent launch --surface terminal` (no orchestration preamble), types the
short bootstrap prompt that points the worker at its spec, then checks the
terminal Orca shows: `matched` requires the requested agent (and host, when
given) running in the returned terminal. Orca does not report the effective
model or effort, so the spec tells the worker to confirm its session's
model/effort before ticket work and report a conflict, never switch the live
model. It persists the sanitized launch evidence as a ticket handoff and
prints `TASK`, `DISPATCH`, `TERMINAL`, `VERIFICATION`. A mismatch exits
non-zero with the Dispatch still open on that terminal: `bbs team task
abandon`, close the terminal, release the lease, then relaunch. A launch that
started no terminal settles its Dispatch as failed by itself.

### OMP launch: `native-terminal`

Resolve the selected alias on the execution host before launch, using its live
OMP role binding. Put the expected resolved provider/model/effort in the Task
spec. Require the worker to compare actual session metadata before ticket work;
unknown or conflicting settings must be reported, never repaired by switching
the live model. An omitted effort uses the selected role's native binding.

Render an idle native command, create an Orca terminal in the assignment
worktree, wait for the agent to come up, then launch the Task into that exact
terminal:

```bash
bbs team worker-command --startup-only --dir "$REPO" \
  --agent omp --model <selected-model> [--effort <selected-effort>]

orca terminal create --worktree <assignment-worktree> \
  --command "<exact rendered startup command>" --json
orca terminal wait --terminal "$TERMINAL" --for tui-idle --json

bbs team task launch "$TASK" --agent omp --worktree <assignment-worktree> \
  --terminal "$TERMINAL" --created-terminal
```

`--startup-only` rejects `--prompt` and `--skill`. Do not type the assignment
by hand. Execute native preflight and startup on the selected execution host;
remote placement must be supported explicitly; never fall back to a
coordinator-local launch. A launch into `--terminal` verifies the agent Orca
detects there and reports `transport-matched`: it does not attest native
settings, so persist the worker's observed agent/model/effort separately.
`--created-terminal` records that this Dispatch owns the terminal, so the
watcher closes it at settle; without it the terminal is never closed by
the lead. Never close a preexisting user's terminal or an uncertain live
worker.

### Reuse a ticket session

A settled worker's terminal is closed by the watcher, so this applies only to a
session that is still live (watcher opted out with `foreman_keep_settled_workers`
for debugging). Otherwise start a fresh worker in
the ticket worktree and point its spec at the ticket's handoffs. A follow-up on
a ticket (`repair`, `refresh`, `redirect`, `deliver`) goes to that ticket's
existing session only when its previous Dispatch settled with an accepted
report, the handle is the exact one recorded for this ticket (`bbs team task
show`), and `orca terminal show` still names its agent. It inherits the ticket
route, so skip model lookup and route resolution:

```bash
bbs team task create "$FOREMAN_ID" --ticket "$TICKET" --kind <repair|refresh|redirect> --spec-file <follow-up spec>
# reserve the ticket's resource profile for the new Task first (topology.md)
TICKET_WORKTREE=$(BABYSIT_TICKET="$TICKET" bbs ticket path worktree --read)
bbs team task launch "$FOLLOWUP_TASK" --agent <ticket-agent> \
  --worktree "$TICKET_WORKTREE" --terminal "$TICKET_TERMINAL" [--created-terminal]
```

`--terminal` takes no model or effort: the live session keeps the ticket route.
Pass `--created-terminal` when an earlier Dispatch of this lead created that
terminal, so cleanup ownership moves to the new Dispatch. Require
`transport-matched`. A refused reuse, a session with no agent, or a ticket whose
route must change starts a fresh worker in the ticket worktree on the recorded
route instead.

A mismatch is a failed launch. If Orca reports rate limiting, re-read quota
snapshots before retrying, and never switch accounts or downgrade tiers.

## Persist and resume

Record task complexity, phase class, selected tier, override provenance,
requested settings, model/effort pins and their pointer provenance, and the
exact-session marker in the numbered ticket handoff written by `bbs team route`.
Include the route result, sanitized launch evidence, terminal ownership and
separately observed session settings in the corresponding Task spec or handoff,
including planning, finish audits, integration and delivery.
`task launch` also writes a numbered ticket handoff.

A ticket has one route, persisted on it under the `worker_` prefix with
`bbs ticket set-pointer`: `worker_agent`, `worker_provider`, `worker_model`,
`worker_effort`, for example
`BABYSIT_TICKET="$TICKET" bbs ticket set-pointer worker_model "<model-or-role>"`.
Pass these pins to `bbs team route` for a fresh ticket worker; provider is not
a launch selector unless Orca explicitly advertises support. Legacy
`planner_`, `reviewer_` and `qa_` pointers from split-step tickets are read only
to resume their exact recorded sessions. Persist other assignments (basic
design, phase integration, heavy QA, release, diagnosis, fresh delivery) in their Task
handoffs rather than overwriting a ticket route.

Apply the canonical resume policy before the next launch: exact native-session
resume requires the recorded agent and settings plus `--exact-session`; never
route a recorded session through today's effective default. For a new Dispatch,
reuse a compatible ticket pin; otherwise resolve the destination-host default.
Never change a live worker's model. Every Dispatch, including a follow-up on a
reused session, reserves its own lease and releases it at settlement; whether
the session is then closed is automatic: the watcher shuts a worker down when
its Dispatch settles (SKILL.md).
