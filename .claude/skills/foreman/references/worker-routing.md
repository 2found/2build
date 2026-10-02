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
enabled/runnable state and supported model/effort overrides when the advertised
discovery contract is available, then persists the requested route and pin
provenance in the ticket handoff. Never substitute coordinator-local detection,
retired BBS preferences, or a generic profile default. If discovery is
unavailable, explicit and pinned routes remain usable; a new unpinned route
must stop with its actionable Orca upgrade/configuration error.

Pass the selected complexity, phase class and tier on every route. Use
`none` when no phase-specific model/effort override was chosen; otherwise name
the override source. Pin provenance identifies the pointer that supplied each
pin, such as `worker_model` or `reviewer_effort`.

Use the returned `agent`, `model` and `effort`, plus observed `hostId` or the
requested `destinationHost`, for matching resource reservation and launch.

```bash
orca orchestration worker-start --task "$ORCA_TASK_ID" --worktree current \
  --agent <selected-agent> --model <selected-model> --effort <selected-effort> --json
```

`--effort` requires `--model`; neither combines with `--terminal`. Omit unset
fields. Do not assume `--provider` support or support for every agent.
`bbs foreman worker-command` uses the same route policy when rendering a
native terminal command.

Verify the actual receipt, never a successful exit alone:

```bash
bbs foreman route verify --ticket "$TICKET" --task "$ORCA_TASK_ID" \
  --agent <selected-agent> --host <host-id> [--model <model>] [--effort <effort>] \
  --receipt-file "$WORKER_START_RECEIPT"
```

This persists only sanitized `launch.effective` fields. A mismatch is a failed
launch. If `worker-start` reports rate limiting, persist it with
`bbs foreman route verify ... --agent <selected-agent> --rate-limited`, re-read
quota snapshots before retrying, and never switch accounts or downgrade tiers.

## Persist and resume

Record task complexity, phase class, selected tier, override provenance,
requested settings, model/effort pins and their pointer provenance, and the
exact-session marker in the numbered ticket handoff written by `bbs foreman route`.
Include the route result and sanitized `launch.effective` receipt in the
corresponding Orca Task/Dispatch handoff, including planning, finish audits,
integration and delivery. `route verify` also writes a numbered ticket handoff.

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
