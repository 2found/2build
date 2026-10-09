# 2build

**Give your Soot a feature to build.**

`autopilot` plans the work, writes code, reviews it, tests it and
fixes issues. For UI work, it creates a prototype first. You can
follow progress without reminding the agent to do each step.

The pack contains 22 skills for product engineering and growth.
The companion `bbs` CLI keeps progress and evidence on disk.

A 2found product. [Soot](https://trysoot.com), powered by 2agent, is
**agent as config**: add config to your source code. Add an AI teammate.

**[Add the pack](#quick-start) · [Explore 2build](https://github.com/2found/2build)**

## Prerequisites

You need Soot, an existing deployment, and the `bbs` CLI on the host
where skills run. A skill pack supplies instructions; your Soot also
needs the tools and repository access required by the task.
The Git installation below also requires Git on `PATH`.

Install `bbs` on macOS or Linux:

```sh
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs --version
```

Without Homebrew, use a verified
[release archive](https://github.com/2found/2build/blob/main/docs/install.md#release-archives-macos-or-linux).
On Windows, run the Linux CLI inside WSL. A plugin-only install does not
include the binary. Without `bbs`, packed skills report `BBS_DEGRADED`
and stop; `semantic-decision` can use its calling LLM fallback, and
`design-ui` can continue design without ticket-state writes.

## Quick start

From your existing Soot project, add the pack:

```sh
soot add https://github.com/2found/2build --path packs/2build
```

Start with Autopilot in the Soot definition for a complete one-ticket
workflow:

```json
{"use": ["2build/autopilot"]}
```

Run `soot check --show` to inspect the resolved configuration before
giving the Soot a task. This validates configuration; it does not run
the task or prove the resulting code works.

## Choose the work

| Work | Start with |
| --- | --- |
| Validate an idea | [prototype](prototype/skill.md), [recon](recon/skill.md) |
| Finish a ticket | [autopilot](autopilot/skill.md), [implement](implement/skill.md), [qa](qa/skill.md) |
| Simplify code | [sweep](sweep/skill.md) |
| Improve marketing | [product-marketing-page](product-marketing-page/skill.md) |
| Diagnose or harden | [investigate](investigate/skill.md), [maintain](maintain/skill.md) |

`foreman` is not included: its project coordinator requires the Orca
worker runtime. See the [full product](https://github.com/2found/2build)
for that workflow.

## Full capability inventory

<details>
<summary>Browse all 22 skills and their packaged sizes</summary>

| Capability | Bytes | When to use |
| --- | --- | --- |
| `agent-first-docs` | 2928 | Write product documentation and quick starts that begin with one coding-agent prompt, then human and agent references. Use for agent-assisted onboarding and discoverable Markdown documentation. |
| `analytics-review` | 3379 | Maintainer pass over babysit telemetry. Use to turn ~/.babysit/analytics (skill-usage.jsonl, decisions.jsonl) into a short ticket-ready report — which skills fire and fail, whether plan-draft habitually over-sizes, where runs go BLOCKED. |
| `autopilot` | 31140 | Deliver one ticket end-to-end from a requirement or accepted plan to a releasable, locally committed change. Own implementation, review fixes, QA, and evidence across resumes; use foreman for multi-ticket projects. |
| `browse` | 8470 | Use the browser for focused web-app checks: open a URL, inspect state, click through a flow, capture screenshots, read console errors, or verify a frontend fix. Prefer this over a full QA workflow. |
| `create-pr` | 4892 | Prepare and create a pull request from the current branch. Use when the user requests a PR or a workflow authorizes the PR handoff. |
| `design-ui` | 3388 | Design and iterate a reviewable UI using the project's design authority and existing screens. Use for UI design requests or a workflow's design/prototype handoff; a direct build request can use the working UI as its prototype. |
| `fix-pr` | 3954 | Address unresolved review comments on an open pull request — fix on the PR head branch, reply in-thread, resolve threads, push. Use after a human or bot review leaves comments on a PR. |
| `harness-audit` | 8651 | Audit AGENTS.md, CLAUDE.md and linked project instructions against repository code, scripts, CI and agent configuration. Use to find stale commands, broken references, conflicting rules or missing verification paths; setup-project owns initial 2build configuration. |
| `implement` | 6125 | Implement a scoped code change from the user's request, an accepted plan.md, or ticket context. Use for feature work, bug fixes, endpoints, UI changes, integrations, and contained refactors. |
| `investigate` | 1709 | Debug a failure before fixing it. Use when the user asks why something is broken, wants root cause analysis, or reports an error, regression, flaky test, crash, or unexpected behavior. |
| `maintain` | 2347 | Keep a mature system secure, reliable, and efficient at scale. Use for security and dependency audits, reliability hardening, db/query performance (schema, indexes, partitioning, caching, batching, async processing), and architecture reviews under change or scale pressure. |
| `plan-draft` | 11648 | Draft a technical plan before implementation. Use when the user asks for a plan, architecture, ticket breakdown, or wants to turn a requirement into plan.md without coding yet. |
| `product-marketing-page` | 2601 | Write or revise product introduction pages around customer value and a clear activation path. Use for product heroes, benefits, CTAs and agent-assisted quick starts, rather than full technical manuals. |
| `prototype` | 2278 | Build a fast, throwaway spike to validate one risky technical or product idea before committing to production work. Use to test feasibility, churn a rough proof, or de-risk an assumption — not to ship, and not for UI look-and-feel questions (that is design-ui). |
| `qa` | 18396 | Systematically test a web application, fix issues caused by the current change, and re-verify. Use for full QA loops, critical user flows, release checks, or test-and-fix requests. |
| `recon` | 1231 | Evaluate an external repository, library, or tool against the current project. Use for adoption decisions, architecture comparisons, or requests to explore and borrow from another project. |
| `review-pr` | 18118 | Review code before it lands — the current branch, working diff, or a GitHub pull request. Use when asked to review a PR/diff/change, run a code review, do a pre-merge or pre-commit check, hunt for bugs, or gate a change before landing. Surfaces correctness bugs, removed behavior, cross-file breakage, security, performance, and cleanup, then verifies each candidate before reporting. Effort levels low|medium|high|xhigh|max (default medium); --fix applies findings to the working tree, --comment posts inline PR comments. Claude Code /code-review pipeline with babysit semantic-decision integration. |
| `semantic-decision` | 2513 | Make an explicit, structured judgment from evidence and bounded choices. Use for task sizing, skill routing, orchestration need, test impact, review findings, recovery, decision tiers, or whether human/deeper review is warranted. Defaults to the current LLM; configured Cloudflare Clef/Clef-flash can replace the provider. |
| `setup-project` | 6674 | Initialize or update a repo's 2build configuration, QA target and project pointers. Use for onboarding or requested configuration changes; use harness-audit to inspect existing AGENTS.md, CLAUDE.md and related harness files. |
| `sweep` | 1749 | Simplify and shrink working code without changing behavior. Use to remove dead code, unship unused features, cut complexity, tidy UI, or optimize a measured hot path. |
| `test` | 7358 | Create meaningful regression coverage, audit weak or redundant tests, and optimize test lanes using measured cost and change impact. Use for test creation, suite audits, failing-test investigation, or CI test optimization; browser journey QA stays with qa/browse. |
| `triage` | 4387 | Tier-1 triage for a stalled or BLOCKED autonomous run. Use when a worker returned BLOCKED/NEEDS_CONTEXT or a ticket's checkpoint stopped advancing — classify recoverable vs needs-human, post a structured handoff, optionally resume from the checkpoint. |

</details>

## Pack layout

Shared 2build references live under `shared/`; skill files reference
them by relative path where the original skill did. Auxiliary skill
assets (references/, workflows/, data/) ship under each capability dir.
The CLI prerequisite is inlined at the top of every `skill.md`.

Direct `use` selectors load the skills you choose. The default recipe,
`"packs": ["2build"]`, selects every capability in `pack.json`.

This README is generated from `scripts/build-2build-pack.py` in the
2build repository. Edit that source and regenerate the pack to update it.
