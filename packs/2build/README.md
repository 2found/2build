# 2build

**Give your Soot a feature to build.**

`autopilot` plans the work, writes code, reviews it, tests it and
fixes issues. For UI work, it creates a prototype first. You can
follow progress without reminding the agent to do each step.

The pack contains 21 skills for product engineering and growth.
The companion `bbs` CLI keeps progress and evidence on disk.

A 2found product. With [Soot](https://trysoot.com), powered by 2agent,
you give recurring work to a small team of Soots that runs on schedule
and asks you only when a person is needed.

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

Select the pack in the Soot definition; start with Autopilot for a complete
one-ticket workflow:

```json
{"packs": ["2build"]}
```

Run `soot check --show` to inspect the resolved configuration before
giving the Soot a task. This validates configuration; it does not run
the task or prove the resulting code works.

## Choose the work

| Work | Start with |
| --- | --- |
| Evaluate external code | [recon](recon/skill.md) |
| Finish a ticket | [autopilot](autopilot/skill.md), [implement](implement/skill.md), [qa](qa/skill.md) |
| Docs or marketing page | [agent-first-docs](agent-first-docs/skill.md) |
| Diagnose a failure | [investigate](investigate/skill.md) |

`foreman` is not included: its project coordinator requires the Orca
worker runtime. See the [full product](https://github.com/2found/2build)
for that workflow.

## Full skill inventory

<details>
<summary>Browse all 21 skills and their packaged sizes</summary>

| Skill | Bytes | When to use |
| --- | --- | --- |
| `agent-first-docs` | 3256 | Write a product's docs, quick start or marketing page around one copyable coding-agent prompt, and serve docs as agent-readable Markdown (llms.txt, .md URLs). Use for onboarding docs, quick starts, product pages, heroes and CTAs. |
| `analytics-review` | 3373 | Review babysit telemetry. Use to turn ~/.babysit/analytics (skill-usage.jsonl, decisions.jsonl) into a short ticket-ready report — which skills fire and fail, whether plan-draft habitually over-sizes, where runs go BLOCKED. |
| `autopilot` | 41321 | Deliver one ticket end-to-end from a requirement or accepted plan to a verified, releasable change left uncommitted in its checkout. Own implementation, review fixes, QA, and evidence across resumes; the default for any size of change, including a day-long one. Use team only for phased releases, independent phase verification or parallel work on disjoint parts. |
| `browse` | 9759 | Use the browser for focused web-app checks: open a URL, inspect state, click through a flow, capture screenshots, read console errors, or verify a frontend fix. Prefer this over a full QA workflow. |
| `create-pr` | 3056 | Prepare and create a pull request from the current branch. Use when a human requests a PR; 2build workflows never open one. |
| `design-ui` | 3388 | Design and iterate a reviewable UI using the project's design authority and existing screens. Use for UI design requests or a workflow's design/prototype handoff; a direct build request can use the working UI as its prototype. |
| `harness-audit` | 10830 | Audit AGENTS.md, CLAUDE.md and linked project instructions against repository code, scripts, CI and agent configuration. Use to find stale commands, broken references, conflicting rules or missing verification paths; setup-project owns initial 2build configuration. |
| `implement` | 6125 | Implement a scoped code change from the user's request, an accepted plan.md, or ticket context. Use for feature work, bug fixes, endpoints, UI changes, integrations, and contained refactors. |
| `investigate` | 1709 | Debug a failure before fixing it. Use when the user asks why something is broken, wants root cause analysis, or reports an error, regression, flaky test, crash, or unexpected behavior. |
| `marketing-video` | 6645 | Plan and produce short marketing videos (TikTok and X first) for your own product, from product facts and audience research to a script with A/B hooks, production and per-platform files; you post them. |
| `plan-draft` | 12393 | Draft a technical plan before implementation. Use when the user asks for a plan, architecture, ticket breakdown, or wants to turn a requirement into plan.md without coding yet. |
| `qa` | 20176 | Systematically test a web application, fix issues caused by the current change, and re-verify. Use for full QA loops, critical user flows, release checks, or test-and-fix requests. |
| `recon` | 1262 | Evaluate an external repository, library, or tool against the current project. Use for adoption decisions, architecture comparisons, or requests to explore and borrow from another project. |
| `release-prd` | 13218 | Release a repository or selected services to production, from version and changelog through main sync, GitHub release, package distribution, deployment and hypercare. Supports --local when CI quota is exhausted; use --dry-run for a read-only release plan. |
| `review-pr` | 19014 | Review code before it lands — the current branch, working diff, or a GitHub pull request. Use when asked to review a PR/diff/change, run a code review, do a pre-merge or pre-commit check, hunt for bugs, or gate a change before landing. Surfaces correctness bugs, removed behavior, cross-file breakage, security, performance, and cleanup, then verifies each candidate before reporting. Effort levels low|medium|high|xhigh|max (default medium); --fix applies findings to the working tree, --comment posts inline PR comments. Claude Code /code-review pipeline with babysit semantic-decision integration. |
| `semantic-decision` | 2513 | Make an explicit, structured judgment from evidence and bounded choices. Use for task sizing, skill routing, orchestration need, test impact, review findings, recovery, decision tiers, or whether human/deeper review is warranted. Defaults to the current LLM; configured Cloudflare Clef/Clef-flash can replace the provider. |
| `setup-project` | 7410 | Initialize or update a repo's 2build configuration, QA target, release profile and project pointers. Use for onboarding or requested configuration changes; use harness-audit to inspect existing AGENTS.md, CLAUDE.md and related harness files. |
| `test` | 7358 | Create meaningful regression coverage, audit weak or redundant tests, and optimize test lanes using measured cost and change impact. Use for test creation, suite audits, failing-test investigation, or CI test optimization; browser journey QA stays with qa/browse. |
| `token-audit` | 5997 | Audit where agent sessions spend tokens and what every session loads before work starts (skill listing, CLAUDE.md/AGENTS.md, memory), then rank savings that keep quality. Use when runs feel expensive, before tuning skills, models, effort or compaction, or to check background context bloat. |
| `triage` | 4510 | Tier-1 triage for a stalled or BLOCKED autonomous run. Use when a worker returned BLOCKED/NEEDS_CONTEXT or a ticket's checkpoint stopped advancing — classify recoverable vs needs-human, post a structured handoff, optionally resume from the checkpoint. |
| `ui-audit` | 6907 | Evaluate a running app's UI/UX by using it as a named persona through one real work flow, scored with PURE step ratings, Nielsen severities and measured thresholds; also builds the app's flow inventory. Use before planning UI improvements or to re-audit a flow after a fix. |

</details>

## Pack layout

Shared 2build references live under `shared/`; skill files reference
them by relative path where the original skill did. Auxiliary skill
assets (references/, workflows/, data/) ship under each skill dir.
The CLI prerequisite is inlined at the top of every `skill.md`.

A Soot that selects the pack (`"packs": ["2build"]`) reads every
skill in `pack.json`; the pack has no tools.

This README is generated from `scripts/build-2build-pack.py` in the
2build repository. Edit that source and regenerate the pack to update it.
