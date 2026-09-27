## Worker model and effort routing

Read the canonical launch-settings contract in
[model routing](../references/model-routing.md) before dispatch. Resolve the
worker's settings in its destination repository:

```bash
bbs agent resolve --role worker --dir "$WORKTREE" --json
```

Use the returned `agent` and `provider` as launch defaults, then
resolve model and effort from task complexity and phase through the canonical
routing contract; generic model/effort defaults do not override its tier table.
Never infer the agent by splitting a shell command. Explicit phase-specific choices go in
the resolver's `--agent`, `--provider`, `--model`, and `--effort` flags.
Never invent a model ID. Empty values mean native defaults at the resolver,
not a verified tier route. Foreman's own `foreman_*` settings do not select workers.

Classify the ticket once from its requirement, plan, and acceptance commands,
then select a model tier for each phase using the shared matrix:

- Plan and design-feedback Dispatches use the **critical phase** route.
- Code Review Dispatches use the **critical phase** route, including simple tickets.
- Build, repairs, per-ticket QA, Integration QA and merge/delivery Dispatches
  use the **normal phase** route. Integration QA scopes checks to the composed surface.
- Classify parent planning and integration/delivery assignments from their own
  scope. Record task complexity, phase class and model tier in every Task spec,
  along with any explicit phase override and its effective launch settings.
  Resolve concrete routes from the canonical table or explicit selections;
  pass the selected route explicitly and persist it before dispatch.

For a launcher that supports the selected preferences, pass nonempty model and
effort on a fresh-worker start:

```bash
orca orchestration worker-start --task "$ORCA_TASK_ID" --worktree current \
  --agent <worker agent> --model <model> --effort <effort> --json
```

Omit unset options. Orca's `--effort` requires `--model`; neither combines with
`--terminal`. Read the live orchestration capability contract before forwarding
preferences. Do not assume it accepts `--provider` or that every agent accepts
model/effort forwarding. `bbs foreman worker-command --dir "$WORKTREE" --prompt
<text>` renders native settings for terminal launchers; its command is not an
argument to `worker-start` unless that launcher explicitly supports it.

If the launcher cannot carry an explicitly configured preference, stop before
dispatch with the unsupported field and required native-launch capability named.
Do not drop it and launch on a default. A provider already established by the
worker server's native configuration is usable only with evidence from that
server; a coordinator shell export is not such evidence.

Read the receipt: `launch.effective` is evidence of the model actually used.
A mismatch with an explicit request is a failed launch, not a successful route.
When no model was requested and the receipt does not identify one, record
`native-default` as unknown effective model. This sentinel is never passed as
`--model`. Do not fabricate a model from a remembered catalog or an OMP role's
name. Record unsupported or unobserved effort the same way.

Persist Plan, Build, Review and QA routes separately so resume and retry cannot silently
change the settings:

```bash
BABYSIT_TICKET="$TICKET" bbs ticket set-pointer planner_agent "<agent>"
BABYSIT_TICKET="$TICKET" bbs ticket set-pointer planner_provider "<provider-or-native-default>"
BABYSIT_TICKET="$TICKET" bbs ticket set-pointer planner_model "<model-or-role-or-native-default>"
BABYSIT_TICKET="$TICKET" bbs ticket set-pointer planner_effort "<effort-or-native-default>"
BABYSIT_TICKET="$TICKET" bbs ticket set-pointer worker_agent "<agent>"
BABYSIT_TICKET="$TICKET" bbs ticket set-pointer worker_provider "<provider-or-native-default>"
BABYSIT_TICKET="$TICKET" bbs ticket set-pointer worker_model "<model-or-role-or-native-default>"
BABYSIT_TICKET="$TICKET" bbs ticket set-pointer worker_effort "<effort-or-native-default>"
```

Use the same `set-pointer` fields with `reviewer_` and `qa_` prefixes for Review
and QA. Record each Dispatch's task complexity, phase class, selected tier,
override provenance, requested agent/provider/model/effort and observed launch
receipt in its durable handoff, including
parent planning and integration/delivery Tasks.

On resume, reuse the persisted route if the assignment and routing policy still
match; changes to config affect new routes. An old route with no task/phase/tier
evidence must be resolved before the next launch; never interrupt a live writer
to change its model. After Plan,
release the settled planner and Plan resource lease; Foreman
starts a fresh Build worker on the normal phase route. Reuse for other phases
only when exact
agent, provider, model/role, effort, and resource profile match the required
route. Preserve the failure evidence
when a user changes a route and never replace a live writer. Choosing a route
outside explicit settings is a Taste decision; log its rationale and observed
cost, and never silently upgrade a model.

Foreman's own launch preferences come from `foreman_*`, with explicit flags
winning on `bbs foreman spawn`. They are pinned on its durable record and
reused during recovery; changing defaults does not alter a running foreman.
