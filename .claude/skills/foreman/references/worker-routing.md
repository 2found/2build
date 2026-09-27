# Worker launch and route persistence

Read the canonical launch-settings contract in
[model routing](../references/model-routing.md) for task/phase classification,
tiers, provider/OMP bindings, overrides and resume policy. It applies to every
parent and child Dispatch; Foreman's own `foreman_*` settings are independent.

## Resolve and launch

1. Resolve the worker agent/provider in its destination repository:
   `bbs agent resolve --role worker --dir "$WORKTREE" --json`.
   Select the phase's model/effort through the canonical contract, then resolve
   again with those explicit settings. Generic defaults do not override tiers.
   For OMP, pass `--model @normal`, `--model @slow` or `--model @plan`
   for the selected tier. Never replace the role with its resolved model name
   or a model from the Codex column. Persist the role as the requested selector;
   record its resolved provider/model/effort separately as launch evidence.
2. Read the live Orca launcher capabilities. For supported model/effort forwarding:

   ```bash
   orca orchestration worker-start --task "$ORCA_TASK_ID" --worktree current \
     --agent <worker agent> --model <model> --effort <effort> --json
   ```

   `--effort` requires `--model`; neither combines with `--terminal`. Omit unset
   fields. Do not assume `--provider` support or support for every agent.
   `bbs foreman worker-command --dir "$WORKTREE" --prompt <text>` renders a
   native terminal command, not a `worker-start` argument unless supported.
3. If a launcher cannot carry a required setting, stop before dispatch and name
   the unsupported field. Do not drop it and launch on a default. A provider
   established by the worker server needs evidence from that server; coordinator
   shell exports do not establish worker configuration.
4. Verify `launch.effective` in the launch receipt. A requested/effective mismatch
   is a failed launch. Record unobserved settings as `native-default` (unknown),
   never as proof of a tier or a value to pass as `--model`.

## Persist and resume

Record task complexity, phase class, selected tier, override provenance,
requested agent/provider/model/effort and effective receipt in each Dispatch's
handoff, including parent planning, finish audits, integration and delivery.
Keep phase routes separate using `bbs ticket set-pointer`:

| Phase | Pointer prefix |
|---|---|
| Plan | `planner_` |
| Implement | `worker_` |
| Review | `reviewer_` |
| QA | `qa_` |

Each prefix has `agent`, `provider`, `model`, `effort` fields, for example:
`BABYSIT_TICKET="$TICKET" bbs ticket set-pointer planner_model "<model-or-role>"`.
Persist other assignments in their Task/Dispatch handoffs rather than overwriting
another phase's route.

Apply the canonical resume policy before the next launch: reuse matching durable
routes; resolve missing legacy routing evidence; never change a live worker's
model. After Plan, release its worker and resource lease, then start Build on
the normal phase route. Other phase reuse requires matching agent, provider,
model/role, effort and resource profile; each new Dispatch reserves its own lease.
