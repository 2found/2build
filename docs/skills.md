# Skill index

Skills carry specialized workflows; `semantic-decision` isolates bounded
judgment with the current LLM by default and optional Cloudflare inference.
The pack is organized around the [five archetypes](../.claude/skills/references/archetypes.md)
of a product-building team — pick by the shape of the work, not the job title.

| Archetype | Skills |
|-----------|--------|
| Prototyper | `recon`, `prototype` |
| Builder | `autopilot`, `plan-draft`, `design-ui`, `implement`, `review-pr`, `qa`, `browse`, `create-pr`, `semantic-decision`, `agent-first-docs` |
| Sweeper | `sweep`, `review-pr`, `qa` |
| Grower | `product-marketing-page` |
| Maintainer | `maintain`, `investigate`, `qa`, `analytics-review`, `triage` |
| Setup | `setup-project` |

## Archetype workflows

`autopilot` runs exactly one workflow per archetype. Name the archetype, or let
`autopilot` route production work to `builder` from ticket state.

| Workflow | Use when | Stops at |
|----------|----------|----------|
| `/bbs:autopilot prototyper "<idea>"` | Validate a risky assumption with a throwaway spike | learning verdict |
| `/bbs:autopilot builder "<requirement>"` | Build one production ticket (auto-selects build/implement/child/verify) | QA-verified local commit |
| `/bbs:foreman "<project>"` | Decompose and orchestrate a multi-ticket project through Orca | Project-wide QA and configured finish |
| `/bbs:autopilot sweeper` | Simplify / unship / optimize without changing behavior | QA-verified branch |
| `/bbs:autopilot grower "<metric>"` | Rank or scaffold a growth experiment | ranked plan or scaffolded variant |
| `/bbs:autopilot maintainer` | Audit security/deps/reliability/scale, or root-cause a bug | hardened/fixed branch |

## Which skill when

| Need | Skill |
|------|-------|
| End-to-end checkpointed work through QA handoff | `/bbs:autopilot` |
| Plan without coding | `/bbs:plan-draft` |
| Implement a scoped change | `/bbs:implement` |
| Validate a risky idea with a throwaway spike | `/bbs:prototype` |
| Simplify, unship, or optimize without changing behavior | `/bbs:sweep` |
| Audit security, deps, reliability, or scale and harden | `/bbs:maintain` |
| Root-cause a bug | `/bbs:investigate` |
| Turn babysit telemetry into ticket-ready findings | `/bbs:analytics-review` |
| Make a bounded judgment, including whether human input is needed | `/bbs:semantic-decision` |
| Classify and unblock a stalled/BLOCKED run | `/bbs:triage` |
| Focused browser check | `/bbs:browse` |
| Full test/fix browser loop | `/bbs:qa` |
| Pre-landing code review | `/bbs:review-pr` |
| Create a pull request after human review | `/bbs:create-pr` |
| UI design and prototype | `/bbs:design-ui` |
| Configure repo | `/bbs:setup-project` |
| Evaluate external code | `/bbs:recon` |
| Measurable growth workflow | `/bbs:autopilot grower` |
| Agent-first onboarding docs and Markdown discovery | `/bbs:agent-first-docs` |
| Product value, CTAs and a marketing quick start | `/bbs:product-marketing-page` |

## Skill boundary

A standalone skill earns its place by owning a 2build contract, evidence format,
repo convention or workflow boundary. Ordinary reasoning, brainstorming, copy
and script writing are direct agent tasks.

Removed shortcuts: `conversion-fix`, `copy-rewrite`, `growth-experiment`,
`office-hours`, `reason`, `social-content`. Ask the agent directly for those
jobs; use `autopilot grower` for a persisted experiment workflow, `prototype`
for a quarantined technical spike, and `product-marketing-page` for the
product activation/docs standard. The reasoning benchmark remains archived
under `tests/reason-bench`; it is not an installed skill.

`create-pr`/`fix-pr` retain PR and ticket-state protocols; `analytics-review`
retains telemetry interpretation; `triage` retains bounded checkpoint recovery.
Low personal usage alone is not a reason to remove these contracts.
