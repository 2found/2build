---
name: setup-project
description: Initialize or update a repo's 2build configuration, QA target, release profile and project pointers. Use for onboarding or requested configuration changes; use harness-audit to inspect existing AGENTS.md, CLAUDE.md and related harness files.
---
# setup-project
Create the minimum configuration future runs need. Re-running preserves valid
settings and fills gaps; it is not an implicit profile switch or repo-wide audit.

Follow [preamble](../references/preamble.md) for bootstrap and telemetry, and
[Auto-Decision Framework](../references/auto-decision-framework.md) for decisions.
Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`.

## Discover before writing
Read applicable `AGENTS.md` / `CLAUDE.md`, existing `.babysit` configuration,
remote/default branches, package scripts, lockfiles, Makefile/compose and CI.
Discover the project's architecture authority and, where applicable, design,
deployment and release guidance, including version sources, package publication
and automatic pipelines. Derive service working directories, runtime, local
target and useful checks.
Preserve explicit user choices, intentional overrides, named environments and
credential variable names. Do not copy template ports or commands as facts.

For an inspection-only request, use [harness-audit](../harness-audit/SKILL.md).
Setup owns configuration writes; that skill owns the evidence checklist. Do
not rewrite architecture docs or unrelated instructions during onboarding.

## Configure only missing or requested settings
- Git needs no configuration ([git flow](../references/git-flow.md)). Verify
  the base branch resolves to the branch the repo releases from (`origin/HEAD`
  by default); if not, report the one-line fix (`git remote set-head origin
  <branch>`) instead of writing a policy file. A leftover
  `.babysit/git-flow.yaml` is ignored; mention it, never extend it.
- `.babysit/qa.yaml`: use the smallest supported local configuration. Preserve
  service check maps and named environments when present. Record actual start
  and check commands with their working directories, a health/page target,
  and critical flows including a validation, error or empty-state case.
  A hosted URL cannot substitute for a missing local target. For CLI/library
  projects, document their real check commands; do not fabricate a web app.
- `.gitignore`: ignore `.babysit/.env` and any machine-local
  `.babysit/qa.local.yaml`. Verify they are also untracked; an ignore rule
  does not remove an already tracked secret. Never print secret values.
- `.claude/settings.json` (project, committed), when Claude Code is a harness:
  merge `env.CLAUDE_CODE_SUBAGENT_MODEL: "sonnet"` and
  `subagentPromptCacheTtl: "1h"` if absent, so subagents run on Sonnet and keep
  their prompt cache warm across long tool waits. Keep every other key; an
  existing value is a deliberate choice. Never write `settings.local.json` or
  `~/.claude/`.
- If login is needed, keep credential **names** in QA config and values in
  ignored files or environment variables. Preserve environment-specific names;
  use `QA_USER` / `QA_PASS` only for a new single-target harness. Seed only
  missing placeholders with `bbs secrets seed --repo-root <repo> <names...>`.
  Never replace values or invent accounts. Multiple GitHub accounts may need
  `GH_ACCOUNT`; use a known login, never guess one.
- Make `AGENTS.md` a project map: link architecture guidance, plus design and
  deployment/release guidance when applicable, with service scope and when to read each.
  Follow [project context links](references/project-pointers.md#project-context-links);
  these project characteristics belong alongside the tooling configuration.
- Release profile: for a releasable app, CLI, library or pack, reuse the profile
  linked by the instruction entrypoint, otherwise existing `.babysit/release.md`,
  `RELEASE.md` or a maintained inline section. Fill missing/requested facts using
  [release-prd's profile contract](../release-prd/references/project-profile.md).
  If no equivalent owner exists, create `RELEASE.md` from observed version files,
  scripts, CI and runbooks, then link it from `AGENTS.md`/`CLAUDE.md`. Separate
  independent components/submodules and distinguish normal pipeline ownership
  from the local execution path. Mark non-applicable distribution/deploy/update
  stages N/A with reasons; record unresolved commands, thresholds or authority
  as gaps rather than inventing them. Preserve existing landing/publication
  policy; setup does not grant workflow suspension or release permission.
  A local path that still invokes Actions/Cloud Build is not `--local` support.
  For a repo with no release surface, record N/A without creating an empty file.
- Add one concise 2build pointer section to the existing instruction entrypoint
  (`AGENTS.md` preferred when both exist; create it if neither exists). Read
  [project pointers](references/project-pointers.md) for the section and,
  only when related repos are in scope, workspace registration. Update existing
  sections in place and preserve imports/symlinks. Machine-local workspace
  mappings belong in `~/.babysit/config.yaml`, never `.babysit/config.yaml`.

For a new single-target app, adapt this shape to observed commands:
```yaml
version: 1
url: http://localhost:5173
start: npm run dev
check: npm test
flows: primary journey, validation error, empty state
# credentials:              # only when login is required
#   username_env: QA_USER
#   password_env: QA_PASS
```
Add `prepare` / `revert` only for an established, safe local lifecycle. Do not
seed generic migrate/rollback commands: reversal is not necessarily safe.

## Verify and hand off
Use [harness-audit](../harness-audit/SKILL.md)'s checks scoped to the files and
services configured here; reuse the facts already gathered. Parse YAML and
inspect `bbs secrets qa probe --env <local-name>` without executing its
output; treat deliberate overrides as valid.

Inspect start/prepare commands before running them. Probe the local app when
available; start only the intended local service when safe within this task.
Report a missing service, credentials or infrastructure as unverified, not a
passing QA run. Configuration validation does not prove application journeys;
use `browse` / `qa` when actual UI verification is requested.

```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
CONFIG: <files/keys changed, or already configured; workspace registration if any>
VERIFY: <config checks and local probe results; explicit unverified items>
NEXT: <remaining prerequisite or /bbs:autopilot "<feature>">
```
Do not branch, commit, push, tag, publish, deploy, suspend workflows or upgrade
local tools as part of this skill. Creating a release profile prepares future
delivery; an execution request belongs to `release-prd`.
