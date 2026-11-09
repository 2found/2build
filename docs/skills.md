# Skill index

You type a handful of commands, one per step of the 2found loop. Autopilot and
Team run the other skills for you as steps of their work.

## Commands you type

| Step | Command | Stops at |
|------|---------|----------|
| Connect a project | `/bbs:setup-project`, then `/bbs:harness-audit` | 2build config, QA target and project docs; AGENTS.md checked against the code |
| Prototype | `/bbs:design-ui "<screens>"` | A clickable prototype for you to approve |
| Build one ticket | `/bbs:autopilot "<requirement>"` — feature or bug fix (auto-selects build/implement/child/verify) | QA-verified change, left uncommitted |
| Build a whole project | `/bbs:team "<project>"` — an autonomous product team on Orca: you confirm the spec and approve the basic design, then review the handover | A reviewed product, one commit per phase with cumulative QA, released on your go at handover; in-scope feedback fixed by the team |
| Check performance | `/bbs:perf-audit "<feature>"`, or no target for the whole system | A plain-language report: what users feel, the proven causes, fixes ranked easiest and safest first, each a ready `/bbs:autopilot` requirement with before/after acceptance |
| Release | `/bbs:release` (`--local`, `--dry-run`, `--prepare`, `--resume <tag>`) | Verified distribution, deploy and hypercare |
| Market test and launch | `/bbs:marketing-video "<goal>"` | Per-platform video files (TikTok, YouTube, Reels, X, LinkedIn) from a script a simulated panel could retell; you read the script, then post them yourself |

`builder` is autopilot's only built-in workflow; a project may add its own under
`.claude/workflows/`.

`marketing-video` and `agent-first-docs` belong to the marketing step of the
loop, not to product engineering. When 2market ships they move to its namespace
and keep a `bbs:` alias.

### Roles → commands

The start guide names five kinds of work. They are roles you play, not
commands; each maps to a command you already have.

| Role | Work | Command |
|------|------|---------|
| Prototyper | Try a risky idea before building it | `/bbs:design-ui`; a throwaway code spike is a direct agent task |
| Builder | A feature or bug fix through review and QA | `/bbs:autopilot` (`/bbs:team` for a phased project) |
| Sweeper | Simplify without changing behaviour | Claude Code `/simplify`, or `/bbs:review-pr` |
| Grower | Copy, conversion, a growth experiment | `/bbs:marketing-video`, `/bbs:agent-first-docs`; a copy or flow change goes through `/bbs:autopilot` |
| Maintainer | Failures, security, dependencies | `/bbs:autopilot` (it investigates first); `/bbs:investigate` for root cause only; Claude Code `/security-review` |

## Steps the workflows run

Autopilot and Team call these on their own. Type one only for a focused job
outside a workflow.

| Need | Skill |
|------|-------|
| Plan without coding | `/bbs:plan-draft` |
| Implement a scoped change | `/bbs:implement` |
| Add missing behavioral coverage | `/bbs:test create` |
| Audit weak, redundant or unrun tests | `/bbs:test audit` |
| Measure and optimize test lanes | `/bbs:test optimize` |
| Root-cause a bug | `/bbs:investigate` |
| Baseline and verify a performance change | `/bbs:perf-audit` (`verify <report>`) |
| Focused browser check | `/bbs:browse` |
| Full test/fix browser loop | `/bbs:qa` |
| Pre-landing code review | `/bbs:review-pr` |

`semantic-decision` makes bounded judgments inside the workflows (the current
LLM by default, optional Cloudflare inference). It sets `user-invocable: false`,
so it is not in the `/` menu.

## Occasional commands

These set `disable-model-invocation: true`: they are not in the model's skill
listing, so the agent never picks them up on its own — type the command.

| Need | Skill |
|------|-------|
| Create a pull request after human review | `/bbs:create-pr` |
| Classify and unblock a stalled/BLOCKED run | `/bbs:triage` |
| Turn 2build's own telemetry into ticket-ready findings | `/bbs:telemetry-review` |
| Find where session tokens and background context go, rank savings | `/bbs:token-audit` |
| Evaluate a running app's UX by using it as a persona; inventory work flows | `/bbs:ui-audit` |
| Map the current architecture and design the next generation: diagrams, headroom, a scaling roadmap with measured triggers | `/bbs:architect` |
| Evaluate external code | `/bbs:recon` |
| Docs, quick start or marketing page built around one agent prompt; agent-readable Markdown | `/bbs:agent-first-docs` |
| Draw an image or cut one out locally (Qwen Image, RMBG-1.4; no account); marketing-video's `mvf art` uses it | `/bbs:image-gen` |

Renamed, old commands still work: `release-prd` → `release`,
`analytics-review` → `telemetry-review`, `foreman` → `team`.

## Skill boundary

`release` reads the target repo's release profile (`RELEASE.md`,
`.babysit/release.md`, or the linked inline instructions). Use
`/bbs:release --local` when CI quota is exhausted: checks and builds run
locally, required artifacts are published manually, and affected automatic
workflows are guarded/restored. `/bbs:release --local --dry-run` reports
the plan without mutations; `--prepare` stops before remote delivery and
`--resume <tag>` recovers a partial release. Options are skill instructions,
not a new CLI subcommand. Codex uses `$bbs:release` with the same options.
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

`create-pr` retains PR and ticket-state protocols; `telemetry-review`
retains telemetry interpretation; `triage` retains bounded checkpoint recovery.
Low personal usage alone is not a reason to remove these contracts.
