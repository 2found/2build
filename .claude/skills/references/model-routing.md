# Model routing

The pack's canonical launch-settings contract. `foreman` routes each supervised
Dispatch it launches through `bbs agent resolve --role worker --json`.
Provider names, model IDs and effort come from user configuration or the agent's
live model listing. There is no bundled model catalog or price table. Never
invent a model ID or read an agent type as a model name.

`autopilot` deliberately does not route models: every step in one invocation
runs in the session it was launched in. Foreman
launches Plan, Build, Review and QA as phase-scoped supervised sessions;
it never asks a running worker to change models mid-run.

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

An empty provider/model/effort means native default: omit its launch flag.
An unknown default is not proof of the required model class below. Do not
replace an empty value with a model from a remembered table. Models are
opaque identifiers, including OMP roles such as `@slow` or `provider/model`.
Use the agent's own model listing when a user needs available choices; do not
store credentials or scrape auth files to choose a provider.

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

## Tiers

Classify from the requirement, plan, and acceptance commands. Weak evidence
stays `normal`. Tiers guide phase ownership and verification, not a fixed model.

| Tier | Use for |
|---|---|
| `simple` | an obvious local docs/config edit, or a tiny isolated change with no new contract and no new state |
| `normal` | ordinary implementation work |
| `critical` / `hard` | security, auth, money, irreversible or live-data migration, distributed concurrency, a cross-system architecture decision |

## Phase routing

Model class follows the work phase, independently of ticket complexity:

| Phase | Required model class |
|---|---|
| Parent/child planning, decomposition, design, design feedback | **strong** |
| Code review and review diagnosis | **strong** |
| Implementation and code repairs | **normal** |
| Per-ticket QA, integration QA and product acceptance checks | **normal** |
| Merges, composition and authorized delivery handlers | **normal** |

Resolve a concrete strong and normal route for the selected agent before their
first dispatch. Use explicit phase-specific user selections first. Otherwise
use configured models when their class is known, and the agent's live model
listing/capability descriptions to select supported routes for any missing
class. Log that selection as Taste and pass the selected model explicitly to
`bbs agent resolve --role worker --model <model> --json` before launch. These
are phase choices, not new config keys or literal `--model strong|normal` flags.
Do not treat a generic `worker_model` as an override for both classes or infer
capability from an opaque alias. If the class cannot be established, route the
missing selection through `NEEDS_CONTEXT`; never silently use one unknown model
for every phase. A higher effort setting alone does not make a normal model strong.

Release a settled worker when the next phase needs a different route; start a
fresh worker on the same checkout. Reuse only when agent, provider, model,
effort and resource profile match the next assigned phase. Persist requested
settings and observed launch results separately; label an unobserved native
default as `native-default`, never as a known effective model. That label is
bookkeeping, never a CLI model argument or evidence of strong/normal capability.

## Changing a route

An explicit model or effort the user names for this run wins over configuration.
Otherwise retain the persisted phase route. A failed attempt that came back short is
evidence to reconsider the approach, not permission to buy a more expensive
model. A requested route change is a Taste decision: log the reason and any
observed cost difference, preserve the failure evidence, and start a fresh
worker. Never replace a live writer or silently substitute an unsupported route.
