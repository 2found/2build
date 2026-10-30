# Skill index

Skills carry specialized workflows; `semantic-decision` isolates bounded
judgment with the current LLM by default and optional Cloudflare inference.

## Workflows

| Command | Use when | Stops at |
|---------|----------|----------|
| `/bbs:autopilot "<requirement>"` | Build one production ticket — feature or bug fix (auto-selects build/implement/child/verify) | QA-verified change, left uncommitted |
| `/bbs:team "<project>"` | Hand a whole project to an autonomous product team on Orca: you confirm the spec and approve the basic design, then review the handover | A reviewed product, one commit per phase with cumulative QA, released on your go at handover; in-scope feedback fixed by the team |

`builder` is autopilot's only built-in workflow; a project may add its own under
`.claude/workflows/`.

## Which skill when

Skills marked *manual* set `disable-model-invocation: true`: they are not in the
model's skill listing, so the agent never picks them up on its own — type the
command.

| Need | Skill |
|------|-------|
| End-to-end checkpointed work through QA handoff | `/bbs:autopilot` |
| Plan without coding | `/bbs:plan-draft` |
| Implement a scoped change | `/bbs:implement` |
| Add missing behavioral coverage | `/bbs:test create` |
| Audit weak, redundant or unrun tests | `/bbs:test audit` |
| Measure and optimize test lanes | `/bbs:test optimize` |
| Root-cause a bug | `/bbs:investigate` |
| Make a bounded judgment, including whether human input is needed | `/bbs:semantic-decision` |
| Focused browser check | `/bbs:browse` |
| Full test/fix browser loop | `/bbs:qa` |
| Pre-landing code review | `/bbs:review-pr` |
| UI design and prototype | `/bbs:design-ui` |
| Release to production through distribution, deploy and hypercare | `/bbs:release-prd` |
| Create a pull request after human review | `/bbs:create-pr` *(manual)* |
| Classify and unblock a stalled/BLOCKED run | `/bbs:triage` *(manual)* |
| Turn babysit telemetry into ticket-ready findings | `/bbs:analytics-review` *(manual)* |
| Find where session tokens and background context go, rank savings | `/bbs:token-audit` *(manual)* |
| Evaluate a running app's UX by using it as a persona; inventory work flows | `/bbs:ui-audit` *(manual)* |
| Configure repo | `/bbs:setup-project` *(manual)* |
| Audit AGENTS.md, CLAUDE.md and related harness files | `/bbs:harness-audit` *(manual)* |
| Evaluate external code | `/bbs:recon` *(manual)* |
| Docs, quick start or marketing page built around one agent prompt; agent-readable Markdown | `/bbs:agent-first-docs` *(manual)* |
| Short marketing videos for your own product: facts, audience, trends, script with A/B hooks, per-platform files you post | `/bbs:marketing-video` *(manual)* |

## Skill boundary

`release-prd` reads the target repo's release profile (`RELEASE.md`,
`.babysit/release.md`, or the linked inline instructions). Use
`/bbs:release-prd --local` when CI quota is exhausted: checks and builds run
locally, required artifacts are published manually, and affected automatic
workflows are guarded/restored. `/bbs:release-prd --local --dry-run` reports
the plan without mutations; `--prepare` stops before remote delivery and
`--resume <tag>` recovers a partial release. Options are skill instructions,
not a new CLI subcommand. Codex uses `$bbs:release-prd` with the same options.
Autopilot's existing QA checkpoint does not grant release authority.

A standalone skill earns its place by owning a 2build contract, evidence format,
repo convention or workflow boundary. Ordinary reasoning, brainstorming, copy
and script writing are direct agent tasks.

Removed shortcuts: `conversion-fix`, `copy-rewrite`, `growth-experiment`,
`office-hours`, `reason`, `social-content`. Ask the agent directly for those
jobs, and use `agent-first-docs` for the product activation/docs standard.

Removed with the archetype model: the `prototype`, `sweep` and
`maintain` skills and the `prototyper`, `sweeper`, `grower` and `maintainer`
workflows (telemetry: `builder` ran 396 times, the four others 7 in total). A
spike is an ordinary agent task; simplify with Claude Code's `/simplify` or
`review-pr`; audit security with `/security-review`; fix a bug with
`autopilot` (`investigate` inside it). `fix-pr` is gone too — ask the agent to
address the PR's review comments; `product-marketing-page` merged into
`agent-first-docs`. The reasoning benchmark remains archived
under `tests/reason-bench`; it is not an installed skill.

`create-pr` retains PR and ticket-state protocols; `analytics-review`
retains telemetry interpretation; `triage` retains bounded checkpoint recovery.
Low personal usage alone is not a reason to remove these contracts.
