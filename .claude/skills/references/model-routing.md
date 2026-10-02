# Model routing

The pack's canonical task/phase routing contract. Foreman obtains a new
worker's agent from explicit intent, a compatible phase pin, or destination-host
Orca discovery; `bbs foreman model` selects its configured model tier.
The CLI owns defaults and settings merging; never copy model tables into skills.

Foreman splits work into phase assignments, selects each model from task
complexity plus phase, then launches its worker. Foreman launches Plan, Build,
Review and QA as phase-scoped supervised sessions.
`autopilot` does not select models: every step in one invocation runs in the
session opened by the human or Foreman. Neither skill switches a running worker's model.

## Route inputs

New worker routes select the explicit agent first, then a compatible phase pin,
then Orca's effective default on the destination host. Model and effort come
from an explicit phase override, a persisted route on resume, or the configured
policy returned by `bbs foreman model`. Provider selection belongs to the agent's
native configuration unless Orca advertises a supported explicit contract.

`bbs agent detect --json` identifies the current harness without Orca;
`bbs agent list --json` reports installed CLIs. Neither command chooses a new
Foreman worker.

Legacy BBS YAML agent/provider/model/effort preferences and their worker/foreman
environment selectors are retired: new launches ignore them, existing YAML
bytes remain unchanged, and `bbs config set` rejects new writes. `bbs agent resolve`
has been removed; use Orca settings for new worker routes.

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

Classify the phase using this table, then query `bbs foreman model` with the
task complexity and selected agent. The CLI owns complexity-to-tier routing;
settings can override it. `normal` names both a task complexity and a phase
class; neither is a model tier.

| Phase                                                                            | Phase class |
| -------------------------------------------------------------------------------- | ----------- |
| Parent/child planning, decomposition, design, design feedback                    | `critical`  |
| Code review and review diagnosis                                                 | `critical`  |
| Implementation and code repairs                                                  | `normal`    |
| Per-ticket QA, integration QA and product acceptance checks                      | `normal`    |
| Finish audits, merges, composition, authorized delivery, restoration and cleanup | `normal`    |


## Model tiers

Read the effective policy (defaults merged with global then repo settings):

```bash
bbs foreman model --dir "$REPO" --json
bbs foreman model --dir "$REPO" --agent <selected-agent> \
  --complexity <simple|normal|hard> --phase-class <normal|critical> --json
```

The lookup returns `agent`, `complexity`, `phaseClass`, `selectedTier`, `model`
and optional `effort`. Use these fields for `bbs foreman route`; do not infer a
model from a tier name. Without selection flags the command prints the whole
effective policy. It needs no ticket or Orca connection. `--dir` defaults to
cwd and resolves the Git root, including from a worktree subdirectory; outside
Git it reads that directory's `.babysit/settings.json`.

Users override policy in `~/.babysit/settings.json` or
`<repo>/.babysit/settings.json` (`BABYSIT_STATE_DIR` relocates the global file).
Precedence is repo > global > built-in defaults, merged per field. Example:

```json
{
  "foreman": {
    "models": {
      "routing": {
        "hard": { "normal": "max" }
      },
      "tiers": {
        "flash": {
          "codex": { "model": "my-model", "effort": "high" }
        }
      }
    }
  }
}
```

`routing` maps `simple|normal|hard` × `normal|critical` to `flash|pro|max`.
`tiers` maps each tier and agent ID to `model` and optional `effort`. Omitted
fields inherit; `"effort": ""` clears an inherited effort. Model IDs and effort
values are opaque native identifiers, validated against discovery at launch.
Missing files use defaults; malformed settings, unknown policy keys or empty
models fail with the settings path. Other settings namespaces are ignored.
These are routing policy, not vendor capability or price claims.

Resolve configured aliases against the live native binding. For OMP, read the
selected role's binding; its provider/model/effort belong to that role.
Do not invent an OMP effort or assume provider-selection support. New routes
send selected model/effort only through advertised launch transport and verify
`launch.effective`. If a launcher cannot honor a selection, stop with `BLOCKED`;
do not substitute.

Resolve an explicit phase-specific user selection first, then a valid persisted
route on resume, otherwise the CLI policy lookup. Explicit phase
model/effort overrides replace the named fields, not the task/phase classification;
record the override separately from the policy tier. Provider remains native
configuration unless Orca advertises a supported route field.
Never invent a model ID; model support and role bindings must be verified
independently.

An unmapped agent needs an explicit phase route. An unavailable model/role or
unknown binding needs `NEEDS_CONTEXT`; unsupported launcher forwarding is
`BLOCKED` before dispatch. Never silently downgrade, upgrade, or substitute a
native default. Applying this policy is Mechanical; a choice outside it follows
the Auto-Decision Framework.

## Autopilot session boundary

A Foreman-dispatched Autopilot honors the supplied phase and model, records the
routing metadata with observed launch settings, and reports conflicting launch
evidence to Foreman before work. It does not classify tasks, select tiers or
repair a route by spawning a worker. Foreman recovers missing legacy routing
evidence from the durable handoff before its next launch.

Standalone Autopilot runs all steps in the human-opened session. It does not
load this routing policy, recommend a tier, or change models between phases.
Record the actual session model (unknown if unobserved) as evidence; use the
existing capability/NEEDS\_CONTEXT handling if the session cannot carry the work.

## Resume and changing a route

Persist task complexity, phase class, tier, override provenance, requested
settings and effective launch receipt together with each phase's existing route.
On resume, reuse the persisted route when the assignment and policy still match;
config changes alone do not change it. Legacy strong/normal routes lacking the
task/phase/tier evidence must be resolved through the CLI policy before the next
launch. A changed task scope or routing policy is reclassified at a settled
boundary and the previous route remains in the handoff.

Release a settled worker when the next phase needs a different route; start a
fresh worker on the same checkout. Reuse only when agent, provider, model,
effort and resource profile match the next assigned phase. Persist requested
settings and observed launch results separately; label an unobserved native
default as `native-default`, never as a known effective model. That label is
bookkeeping, never a CLI model argument or evidence that a tier was honored.

A failed attempt that came back short is evidence to reconsider the approach,
not permission to buy a more expensive model. A requested route change is a
Taste decision: record the reason and any observed cost difference in the
handoff, preserve the failure evidence, and start a fresh
worker. Never replace a live writer or silently substitute an unsupported route.