# Model routing

The pack's canonical task/phase routing contract. The lead obtains a new
worker's agent from explicit intent, a compatible ticket pin, or destination-host
Orca discovery; `bbs team model` selects its configured model tier.
The CLI owns defaults and settings merging; never copy model tables into skills.

The lead splits each release phase into tickets and selects one route per ticket from
its task complexity plus the assignment class; that one supervised session runs
the ticket's whole loop (plan, build, review, QA) and every follow-up on it.
Single-purpose assignments (basic design, diagnosis, phase integration,
heavy QA, release, delivery in a fresh session) get their own routes.
`autopilot` does not select models: every step in one invocation runs in the
session opened by the human or the lead. Neither skill switches a running worker's model.

## Route inputs

New worker routes select the explicit agent first, then a compatible ticket pin,
then Orca's effective default on the destination host. Model and effort come
from an explicit ticket override, a persisted route on resume, or the configured
policy returned by `bbs team model`. Provider selection belongs to the agent's
native configuration unless Orca advertises a supported explicit contract.

`bbs agent detect --json` identifies the current harness without Orca;
`bbs agent list --json` reports installed CLIs. Neither command chooses a new
Team worker.

BBS global agent/provider/model/effort preferences and their worker/foreman
environment selectors are retired (the legacy BBS YAML keys): new launches
ignore them, existing YAML
bytes remain unchanged, and `bbs config set` rejects new writes. `bbs agent resolve`
has been removed; use Orca settings for new worker routes.

## Task complexity

Use `../semantic-decision/SKILL.md` kind `task-complexity` with this rubric to
classify each assignment from its requirement, plan, and acceptance commands.
Classify the assignment's **own diff**, not its project, domain or parent: a UI,
docs, onboarding, SDK or test ticket inside a billing project is still `normal`.
A child or recheck does not inherit the parent's `hard` label. Weak evidence
stays `normal`. Ticket size and repository profile do not select model tiers.
Normalize legacy task complexity `critical` to `hard`; reserve `critical` for
the phase class below.

| Complexity | Use for                                                                                              |
| ---------- | ---------------------------------------------------------------------------------------------------- |
| `simple`   | an obvious local docs/config edit, or a tiny isolated change with no new contract and no new state   |
| `normal`   | ordinary implementation work, including new modules, APIs, schemas, UI and work spanning services     |
| `hard`     | the diff itself changes one named invariant below, where a mistake ships a real loss                  |

`hard` needs exactly one `--hard-reason` on `bbs team route`, and the CLI
rejects anything else (money, auth, security, live-data or concurrency):

| `--hard-reason` | The assignment's own diff…                                                         |
| --------------- | ---------------------------------------------------------------------------------- |
| `money`         | debits, credits, prices, refunds or a ledger                                        |
| `auth`          | login, sessions, tokens, OAuth or permission checks                                 |
| `security`      | secrets, sandboxing, SSRF/injection boundaries or tenant isolation                  |
| `live-data`     | migrates, backfills or deletes production data irreversibly                         |
| `concurrency`   | locks, idempotency, retries or ordering across processes                             |

The invariant must be the ticket's core deliverable, not a touchpoint. Calling
an existing payment, auth or lock API, rendering ledger data, adding a field to
a guarded table, or writing a reversible additive migration is `normal`.
"Touches several services", "new architecture", "important to the product",
"the project is about billing" and "large" are not reasons: architecture is
decided in basic design, and size is split by topology.

Hard is rare by design: expect about one ticket in seven, never more than one
in five. When a phase plan has more, split each hard invariant into its own
small ticket so the surrounding work (UI, API surface, wiring, tests) stays
`normal` and runs on `pro` — but only where the invariant owns its own files
behind a frozen interface; one that shares files with its surroundings stays
one `hard` ticket, since the second session would reload the same context
([split by context](topology.md#split-by-context)). Record the split in the
phase plan. When unsure
between `normal` and `hard`, choose `normal`: review and QA still gate it.

## Assignment routing

Classify the assignment using this table, then query `bbs team model` with
the task complexity, its phase class and the selected agent.
The CLI owns complexity-to-tier routing; settings can override it. `normal` names both a
task complexity and a phase class; neither is a model tier.

Routine work should stay in `lite`/`flash`/`pro`; `max` is for `hard`
tickets and `hard` basic designs only. A `critical` phase class alone does
not imply `max`. Use `normal` for an unclassified assignment; with no difficulty or
importance signal, the built-in policy falls back to `pro`.

| Assignment                                                                         | Phase class |
| ---------------------------------------------------------------------------------- | ----------- |
| Basic design: architecture, prototype, release-phase plan, design review           | `design`    |
| Ticket worker: one session for the ticket's whole loop and every follow-up on it   | `critical`  |
| Diagnosis of a ticket's repeated failure                                           | `critical`  |
| Mechanical re-verification of recorded checks on unchanged code                    | `normal`    |
| Phase integration: QA of the phase commit, cumulative criteria, then land         | `normal`    |
| Consolidated heavy QA and product acceptance checks                                | `normal`    |
| Phase release: `release-prd` and its hypercare                                     | `normal`    |
| Finish audits, restoration and cleanup in a fresh session                          | `normal`    |

A ticket worker is `critical` because it owns its ticket plan and landing review;
implementation rides the same session instead of buying a cold start. Built in,
`simple` and `normal` tickets run on `pro` (Claude `opus` at medium effort): a
stronger model that lands the ticket in one session is cheaper than a
follow-up, which costs a cold start. Phase integration (`normal`/`normal`)
also runs on `pro`, because it reviews and fixes the small tickets that
deferred their gates to it. Only `hard` tickets — security, money, live
data, cross-system logic — reach `max`. Basic design is its own `design` class
(`pro` unless the design is `hard`, then `max`), so overriding `routing.normal.critical`
no longer moves project planning. The mechanical re-verification exception applies only to
executing recorded checks. Reviewing changed code, assessing new findings or
issuing a fresh judgment remains code review, even when the dispatch is called
a recheck.

## Model tiers

Read the effective policy (defaults merged with global then repo settings):

```bash
bbs team model --dir "$REPO" --json
bbs team model --dir "$REPO" --agent <selected-agent> \
  --complexity <simple|normal|hard> --phase-class <normal|critical|design> --json
```

The lookup returns `agent`, `complexity`, `phaseClass`, `selectedTier`, `model`
and optional `effort`. Use these fields for `bbs team route`; do not infer a
model from a tier name. Without selection flags the command prints the whole
effective policy. It needs no ticket or Orca connection. `--dir` defaults to
cwd and resolves the Git root, including from a worktree subdirectory; outside
Git it reads that directory's `.babysit/settings.json`.

With `--agent`, omitted complexity and phase class each default to `normal`;
`bbs team model --agent codex --json` therefore selects the configured
`normal`/`normal` binding (`pro` in the built-in policy). Invalid explicit
values still fail. `bbs team route --selected-tier` records the lookup's tier;
it does not select or override a model. Only `lite`, `flash`, `pro`, and `max` are valid;
`ultra` is not a tier and is rejected. Pass the lookup's model/effort as well.

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

`routing` maps `simple|normal|hard` × `normal|critical|design` to `lite|flash|pro|max`.
`tiers` maps each tier and agent ID to `model` and optional `effort`. Omitted
fields inherit; `"effort": ""` clears an inherited effort. Model IDs and effort
values are opaque native identifiers, validated by the selected launch transport.
Missing files use defaults; malformed settings, unknown policy keys or empty
models fail with the settings path. Other settings namespaces are ignored.
These are routing policy, not vendor capability or price claims.

Do not switch a worker's agent from quota or usage. Pin the harness at spawn
or adopt: `--plan-worker`, `--implement-worker`, `--qa-worker` (default: this
the lead's own agent). `bbs team model --quota` is refused.

Resolve configured aliases against the live native binding. For OMP, read the
selected role's binding; its provider/model/effort belong to that role.
Do not invent an OMP effort or assume provider-selection support. Use the
route's `launchMode`: `agent-launch` forwards only advertised overrides and
verifies the launched agent; OMP launches use the native startup command
followed by `task launch --terminal` into that exact terminal. Orca attests
neither model nor effort, so the worker compares its observed session
settings with the resolved route or native binding before ticket work. Unknown receipt fields remain unknown. If neither
transport can honor a selection, stop with `BLOCKED`; do not substitute.

Resolve an explicit ticket-specific user selection first, then a valid persisted
route on resume, otherwise the CLI policy lookup. Explicit ticket
model/effort overrides replace the named fields, not the task/phase classification;
record the override separately from the policy tier. Provider remains native
configuration unless Orca advertises a supported route field.
Never invent a model ID; model support and role bindings must be verified
independently.

An unmapped agent needs an explicit ticket route. An unavailable model/role or
unknown binding needs `NEEDS_CONTEXT`; unsupported launcher forwarding is
`BLOCKED` before dispatch. Never silently downgrade, upgrade, or substitute a
native default. Applying this policy is Mechanical; a choice outside it follows
the Auto-Decision Framework.

## Autopilot session boundary

A Team-dispatched Autopilot honors the supplied assignment and model, records the
routing metadata with observed launch settings, and reports conflicting launch
evidence to the lead before work. It does not classify tasks, select tiers or
repair a route by spawning a worker. The lead recovers missing legacy routing
evidence from the durable handoff before its next launch.

Standalone Autopilot runs all steps in the human-opened session. It does not
load this routing table, recommend a tier, or change models between steps.
Record the actual session model (unknown if unobserved) as evidence; use the
existing capability/NEEDS\_CONTEXT handling if the session cannot carry the work.

## Resume and changing a route

Persist task complexity, phase class, tier, override provenance, requested
settings and effective launch receipt together with each ticket's existing route.
On resume, reuse the persisted route when the assignment and policy still match;
config changes alone do not change it. Legacy strong/normal routes lacking the
task/phase/tier evidence must be resolved through the CLI policy before the next
launch. A changed task scope or routing policy is reclassified at a settled
boundary and the previous route remains in the handoff.

Follow-ups on a ticket reuse its live session, whose route is the ticket route
by construction. A ticket whose route must change (reclassified scope, explicit
override) releases its settled session and starts a fresh worker on the same
checkout; never reuse a session for an assignment with a different agent,
provider, model, effort or resource profile. Persist requested
settings and observed launch results separately; label an unobserved native
default as `native-default`, never as a known effective model. That label is
bookkeeping, never a CLI model argument or evidence that a tier was honored.

A failed attempt that came back short is evidence to reconsider the approach,
not permission to buy a more expensive model. A requested route change is a
Taste decision: record the reason and any observed cost difference in the
handoff, preserve the failure evidence, and start a fresh
worker. Never replace a live writer or silently substitute an unsupported route.
