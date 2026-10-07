# 2build

2build skill pack for Soot — part of the 2found ecosystem. Each
capability is one 2build skill; select them in the Soot definition.
`foreman` is excluded on purpose: it needs the Orca worker runtime.

## Prerequisites

The `bbs` CLI must be on `PATH` — every skill shells out to it. Install:

```sh
brew install 2found/2build/bbs
```

Homebrew covers macOS and Linux; on Linux you can also download the
per-arch tarball from the latest GitHub release. Windows publishes no
binary — run inside WSL or Git-Bash (the Linux tarball works there), or
build `go run ./cmd/bbs setup` from a checkout. A plugin-only install ships
no compiled binary; without `bbs` the skills report `BBS_DEGRADED` and stop,
except `semantic-decision`, which uses its calling LLM fallback.
The prerequisite is inlined at the top of every `skill.md`, so the
agent reads it on any skill load.

| Capability | Bytes | When to use |
| --- | --- | --- |
| `agent-first-docs` | 2928 | Write product documentation and quick starts that begin with one coding-agent prompt, then human and agent references. Use for agent-assisted onboarding and discoverable Markdown documentation. |
| `analytics-review` | 2989 | Maintainer pass over babysit telemetry. Use to turn ~/.babysit/analytics (skill-usage.jsonl, decisions.jsonl) into a short ticket-ready report — which skills fire and fail, whether plan-draft habitually over-sizes, where runs go BLOCKED. |
| `autopilot` | 31140 | Deliver one ticket end-to-end from a requirement or accepted plan to a releasable, locally committed change. Own implementation, review fixes, QA, and evidence across resumes; use foreman for multi-ticket projects. |
| `browse` | 8470 | Use the browser for focused web-app checks: open a URL, inspect state, click through a flow, capture screenshots, read console errors, or verify a frontend fix. Prefer this over a full QA workflow. |
| `conversion-fix` | 1231 | Audit and improve a marketing or activation surface in source. Use for landing pages, pricing, signup, onboarding, paywalls, conversion friction, or CRO requests. |
| `copy-rewrite` | 1196 | Rewrite product marketing copy in source. Use for headlines, hero text, CTAs, feature copy, positioning clarity, tone, or copy audits. |
| `create-pr` | 4906 | Prepare and create a pull request from the current branch. Use when code is ready to push, the user asks for a PR, or a babysit handoff is ready for human review. |
| `design-ui` | 6621 | Design a feature, page, or component and deliver a reviewable prototype before implementation. Use for UI/UX specs, style/color/typography selection, and early design feedback on frontend work. |
| `fix-pr` | 3572 | Address unresolved review comments on an open pull request — fix in the ticket worktree, reply in-thread, resolve threads, push. Use after a human or bot review leaves comments on a PR. |
| `growth-experiment` | 1325 | Propose, rank, and optionally scaffold a measurable product growth experiment. Use for A/B tests, activation, retention, acquisition, funnel, or ICE-ranking requests. |
| `implement` | 4358 | Implement a scoped code change from the user's request, an accepted plan.md, or ticket context. Use for feature work, bug fixes, endpoints, UI changes, integrations, and contained refactors. |
| `investigate` | 1709 | Debug a failure before fixing it. Use when the user asks why something is broken, wants root cause analysis, or reports an error, regression, flaky test, crash, or unexpected behavior. |
| `maintain` | 2347 | Keep a mature system secure, reliable, and efficient at scale. Use for security and dependency audits, reliability hardening, db/query performance (schema, indexes, partitioning, caching, batching, async processing), and architecture reviews under change or scale pressure. |
| `office-hours` | 1361 | Stress-test an idea before building. Use for startup/product judgment, builder brainstorming, narrowing a wedge, shaping a requirement, or deciding whether an idea is worth implementing. |
| `plan-draft` | 5025 | Draft a technical plan before implementation. Use when the user asks for a plan, architecture, ticket breakdown, or wants to turn a requirement into plan.md without coding yet. |
| `product-marketing-page` | 2601 | Write or revise product introduction pages around customer value and a clear activation path. Use for product heroes, benefits, CTAs and agent-assisted quick starts, rather than full technical manuals. |
| `prototype` | 2278 | Build a fast, throwaway spike to validate one risky technical or product idea before committing to production work. Use to test feasibility, churn a rough proof, or de-risk an assumption — not to ship, and not for UI look-and-feel questions (that is design-ui). |
| `qa` | 15265 | Systematically test a web application, fix issues caused by the current change, and re-verify. Use for full QA loops, critical user flows, release checks, or test-and-fix requests. |
| `reason` | 9972 | Deliberate-reasoning scaffold that lifts a smaller model's planning, solution design, debugging, and QA thinking toward frontier quality. Use before drafting a plan, choosing between designs, diagnosing a hard bug, writing a QA plan, or whenever the first plausible answer might be wrong. Composable — run another skill "with reason" to harden its decision points. |
| `recon` | 1231 | Evaluate an external repository, library, or tool against the current project. Use for adoption decisions, architecture comparisons, or requests to explore and borrow from another project. |
| `review-pr` | 18118 | Review code before it lands — the current branch, working diff, or a GitHub pull request. Use when asked to review a PR/diff/change, run a code review, do a pre-merge or pre-commit check, hunt for bugs, or gate a change before landing. Surfaces correctness bugs, removed behavior, cross-file breakage, security, performance, and cleanup, then verifies each candidate before reporting. Effort levels low|medium|high|xhigh|max (default medium); --fix applies findings to the working tree, --comment posts inline PR comments. Claude Code /code-review pipeline with babysit semantic-decision integration. |
| `semantic-decision` | 2513 | Make an explicit, structured judgment from evidence and bounded choices. Use for task sizing, skill routing, orchestration need, test impact, review findings, recovery, decision tiers, or whether human/deeper review is warranted. Defaults to the current LLM; configured Cloudflare Clef/Clef-flash can replace the provider. |
| `setup-project` | 10866 | Configure the current repo for babysit/autopilot. Use when the user asks to set up a project, initialize babysit config, or make autopilot understand branch and QA defaults. |
| `social-content` | 1212 | Create short-form product video scripts for TikTok, Reels, or Shorts. Use for hooks, scripts, captions, production notes, or a small content calendar. |
| `sweep` | 1749 | Simplify and shrink working code without changing behavior. Use to remove dead code, unship unused features, cut complexity, tidy UI, or optimize a measured hot path. |
| `triage` | 4025 | Tier-1 triage for a stalled or BLOCKED autonomous run. Use when a worker returned BLOCKED/NEEDS_CONTEXT or a ticket's checkpoint stopped advancing — classify recoverable vs needs-human, post a structured handoff, optionally resume from the checkpoint. |

Shared 2build references live under `shared/`; skill files reference
them by relative path where the original skill did. Auxiliary skill
assets (references/, workflows/, data/) ship under each capability dir.

Install: `soot add <repo> --path packs/2build`, then choose either the
default recipe (`"packs": ["2build"]`, selects every capability) or
direct selectors (`"use": ["2build/implement", …]`).
