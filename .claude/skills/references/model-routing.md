# Model routing

The pack's canonical task/phase routing and launch-settings contract. `foreman`
routes each supervised Dispatch it launches through
`bbs agent resolve --role worker --json`. This resolver carries settings; the
skill selects the tier. Keep the model table here, not in each consuming skill.

Foreman splits work into phase assignments, selects each model from task
complexity plus phase, then launches its worker. Foreman launches Plan, Build,
Review and QA as phase-scoped supervised sessions.
`autopilot` does not select models: every step in one invocation runs in the
session opened by the human or Foreman. Neither skill switches a running worker's model.

## Configuration

Resolve each field independently: explicit `--agent` / `--provider` / `--model` /
`--effort`, then `BABYSIT_WORKER_*`, then shared `BABYSIT_AGENT`,
`BABYSIT_PROVIDER`, `BABYSIT_MODEL`, `BABYSIT_EFFORT`, then the machine-wide
`~/.babysit/config.yaml`. The keys are `worker_agent`, `worker_provider`,
`worker_model`, `worker_effort`. The corresponding `foreman_*` keys are
independent.

An absent agent or `auto` detects the current harness first, then an installed
CLI. `bbs agent detect --json` identifies the current harness with its evidence;
`bbs agent list --json` reports installed CLIs. Installed does not mean current,
authenticated, or ready to resolve babysit's skills.

An empty provider/model/effort means native default at the resolver level: omit
its launch flag. For a routed launch, select model and effort from the tier
table below first; an unknown native default does not satisfy a route. Models
are opaque identifiers to the resolver. Use the agent's own model listing to
verify support; do not store credentials or scrape auth files to choose a provider.

```bash
bbs config set worker_agent omp
bbs config set worker_provider openai
bbs config set worker_model '<your-model-id>'
bbs config set worker_effort high
bbs agent resolve --role worker --dir '<worker-repo>' --json
```

For OMP roles whose binding already names a provider, leave `worker_provider`
empty. Codex providers must exist in its native configuration. Claude accepts
`anthropic`, `bedrock`, `vertex`, or `foundry`; Grok and Cursor use their native
`xai` and `cursor` providers. Unsupported combinations fail at resolution.
`bbs foreman worker-command` and `spawn` translate these settings to native CLI
flags. A separate launcher must explicitly support forwarding them; a resolved
preference is not evidence that the worker received it.

## Task complexity

Classify each task from its requirement, plan, and acceptance commands; weak
evidence stays `normal`. Classify parent planning and composed integration or
delivery tasks from their own scope, not whichever child finished last. Ticket
size and repository profile do not select model tiers. Normalize legacy task
complexity `critical` to `hard`; reserve `critical` for the phase class below.


| Complexity | Use for                                                                                                                   |
| ---------- | ------------------------------------------------------------------------------------------------------------------------- |
| `simple`   | an obvious local docs/config edit, or a tiny isolated change with no new contract and no new state                        |
| `normal`   | ordinary implementation work                                                                                              |
| `hard`     | security, auth, money, irreversible or live-data migration, distributed concurrency, a cross-system architecture decision |


## Phase routing

First choose the task's ordered pair of tiers, then select by phase class:


| Complexity | Tier pair      | Normal phase | Critical phase |
| ---------- | -------------- | ------------ | -------------- |
| `simple`   | `[flash, pro]` | `flash`      | `pro`          |
| `normal`   | `[flash, pro]` | `flash`      | `pro`          |
| `hard`     | `[pro, max]`   | `pro`        | `max`          |


Normal phases select index 0; critical phases select index 1. `normal` names
both a task complexity and a phase class; neither is a model tier.


| Phase                                                                            | Phase class |
| -------------------------------------------------------------------------------- | ----------- |
| Parent/child planning, decomposition, design, design feedback                    | `critical`  |
| Code review and review diagnosis                                                 | `critical`  |
| Implementation and code repairs                                                  | `normal`    |
| Per-ticket QA, integration QA and product acceptance checks                      | `normal`    |
| Finish audits, merges, composition, authorized delivery, restoration and cleanup | `normal`    |


## Model tiers

These are the configured routing policy, not vendor capability or price claims.
The order is `flash` &lt; `pro` &lt; `max`; model tier and reasoning effort are separate
fields (flash uses max effort on Codex).


| Tier    | Codex model   | Codex effort | Claude model | Claude effort | OMP role  |
| ------- | ------------- | ------------ | ------------ | ------------- | --------- |
| `flash` | `gpt-6-luna`  | `high`       | opus         | `high`        | `@normal` |
| `pro`   | `gpt-5.6-sol` | `high`       | opus         | `high`        | `@slow`   |
| `max`   | `gpt-6-astra` | `high`       | opus         | `high`        | `@plan`   |


For Claude, resolve a supported native identifier for **Opus 5.5**. An `opus`
alias is usable only if the live binding identifies that version. For OMP,
read the selected role's binding; its provider/model/effort belong to that role.
Do not invent an OMP effort or let generic worker provider/effort defaults
override the binding. Pass the role as the model selector using supported native
launch transport; omission of a flag is not evidence that a conflicting default
was cleared. A launcher that cannot honor the binding is unsupported.
In particular, `bbs agent resolve` and `bbs foreman worker-command` inherit
generic provider/effort settings even when those flags are omitted or empty.
For a launcher supporting role-only selection, pass the role without forwarding
those generic fields and verify the native binding in the launch receipt.
For `worker-command`, explicitly override inherited fields with the observed
role binding's values where supported. If a binding has an unset field that
cannot be cleared by that launcher, stop with `BLOCKED`; do not mutate shared
configuration or launch with a conflicting inherited setting.

Resolve an explicit phase-specific user selection first, then a valid persisted
route on resume, otherwise the selected tier's table entry. Explicit phase
model/effort overrides replace the named fields, not the task/phase classification;
record the override separately from the policy tier. Generic `worker_model` and
`worker_effort` defaults do not override the tier table. Supply the selected model
and nonempty effort explicitly to `bbs agent resolve --role worker --model <model> --effort <effort> --json`, omitting effort for an OMP role unless explicitly
overridden for that phase. Never invent a model ID. Model support and role bindings
must be verified independently of this resolver, which accepts opaque IDs.

An unmapped agent needs an explicit phase route. An unavailable model/role or
unknown binding needs `NEEDS_CONTEXT`; unsupported launcher forwarding is
`BLOCKED` before dispatch. Never silently downgrade, upgrade, or substitute a
native default. Applying this table is Mechanical; a choice outside it follows
the Auto-Decision Framework and logs Taste decisions.

## Autopilot session boundary

A Foreman-dispatched Autopilot honors the supplied phase and model, records the
routing metadata with observed launch settings, and reports conflicting launch
evidence to Foreman before work. It does not classify tasks, select tiers or
repair a route by spawning a worker. Foreman recovers missing legacy routing
evidence from the durable handoff before its next launch.

Standalone Autopilot runs all steps in the human-opened session. It does not
load this routing table, recommend a tier, or change models between phases.
Record the actual session model (unknown if unobserved) as evidence; use the
existing capability/NEEDS\_CONTEXT handling if the session cannot carry the work.

## Resume and changing a route

Persist task complexity, phase class, tier, override provenance, requested
settings and effective launch receipt together with each phase's existing route.
On resume, reuse the persisted route when the assignment and policy still match;
config changes alone do not change it. Legacy strong/normal routes lacking the
task/phase/tier evidence must be resolved through this matrix before the next
launch. A changed task scope or routing policy is reclassified at a settled
boundary and the previous route remains in the handoff.

Release a settled worker when the next phase needs a different route; start a
fresh worker on the same checkout. Reuse only when agent, provider, model,
effort and resource profile match the next assigned phase. Persist requested
settings and observed launch results separately; label an unobserved native
default as `native-default`, never as a known effective model. That label is
bookkeeping, never a CLI model argument or evidence that a tier was honored.

A failed attempt that came back short is evidence to reconsider the approach,
not permission to buy a more expensive
model. A requested route change is a Taste decision: log the reason and any
observed cost difference, preserve the failure evidence, and start a fresh
worker. Never replace a live writer or silently substitute an unsupported route.