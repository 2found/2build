# Worker launch and route persistence

Read the canonical contract in [model routing](../references/model-routing.md)
for task/phase classification, tiers, phase overrides and OMP role bindings.
For Orca-supervised Foreman Dispatches, this reference owns worker agent
selection and resume. New routes use explicit intent, a compatible phase pin,
then the destination-host Orca default; BBS preferences do not select an agent.

## Resolve and launch

Classify task complexity and phase class using the canonical contract. Select
an agent from explicit intent, a compatible phase pin, or destination-host Orca
discovery, then obtain model/effort and tier with
`bbs foreman model --dir "$REPO" --agent <selected-agent> --complexity <simple|normal|hard> --phase-class <normal|critical> --json`.
Use the assignment repo/worktree, not the coordinator repo in a multi-repo run.
Explicit phase overrides and valid persisted resume routes take precedence;
skip the policy lookup when resuming an exact recorded session. Pass the
resulting model, effort and selected tier into route validation before resource
admission:

```bash
bbs foreman route --ticket "$TICKET" --task "$ORCA_TASK_ID" \
  [--agent <explicit-agent>] [--pinned-agent <phase-agent>] \
  [--pinned-model <phase-model>] [--pinned-effort <phase-effort>] \
  [--model <selected-model>] [--effort <selected-effort>] [--host <host-id>] \
  --complexity <simple|normal|hard> --phase-class <normal|critical> \
  --selected-tier <flash|pro|max> [--override-provenance <source|none>] \
  [--pinned-model-provenance <phase-pointer>] [--pinned-effort-provenance <phase-pointer>]
```

The route order is explicit intent, compatible phase pin, then the
destination-host `agentDiscovery.effectiveDefaultAgent`. The command validates
enabled/runnable state when discovery is available and persists the requested
route and pin provenance. It also returns `launchMode`: `worker-start` validates
Orca's advertised model/effort forwarding; OMP uses `native-terminal` because
Orca does not forward OMP launch overrides. Never substitute coordinator-local
detection, retired BBS preferences, or a generic profile default. If discovery
is unavailable, explicit and pinned routes remain usable; a new unpinned route
must stop with its actionable Orca upgrade/configuration error.

Pass the selected complexity, phase class and tier on every route. Use
`none` when no phase-specific model/effort override was chosen; otherwise name
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
spec. Require the worker to compare actual session metadata before phase work;
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

Keep phase routes separate using `bbs ticket set-pointer`:

| Phase | Pointer prefix |
|---|---|
| Plan | `planner_` |
| Implement | `worker_` |
| Review | `reviewer_` |
| QA | `qa_` |

Each prefix has `agent`, `provider`, `model`, `effort` fields, for example:
`BABYSIT_TICKET="$TICKET" bbs ticket set-pointer planner_model "<model-or-role>"`.
Pass the phase's agent/model/effort pins to `bbs foreman route`; provider is not
a `worker-start` selector unless Orca explicitly advertises support. Persist
other assignments in their Task/Dispatch handoffs rather than overwriting
another phase's route.

Apply the canonical resume policy before the next launch: exact native-session
resume requires the recorded agent and settings plus `--exact-session`; never
route a recorded session through today's effective default. For a new Dispatch,
reuse a compatible phase pin; otherwise resolve the destination-host default.
Never change a live worker's model. After Plan, release its worker and resource
lease before starting Build; each new Dispatch reserves its own lease.
