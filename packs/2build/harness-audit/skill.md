---
name: harness-audit
description: Audit AGENTS.md, CLAUDE.md and linked project instructions against repository code, scripts, CI and agent configuration. Use to find stale commands, broken references, conflicting rules or missing verification paths; setup-project owns initial 2build configuration.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# harness-audit
Check whether an agent can discover the right constraints, run the project and
verify a change from the instructions actually available to it. Findings need
repository evidence, not a preferred documentation template.

Follow [preamble](../shared/preamble.md) for bootstrap and telemetry, and
[Auto-Decision Framework](../shared/auto-decision-framework.md) for decisions.
Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`.

## Scope and authority
Audit is read-only by default, apart from normal skill telemetry and a report
when requested. An explicit request to fix or optimize the harness authorizes
scoped edits; do not turn “check AGENTS.md” into setup, policy changes or a
rewrite of every linked doc. No ticket or 2build configuration is required.
Do not create branches, commit, push, deploy, install tools, rotate credentials
or change global settings as part of an audit.

Start at the requested repo/service or file. Discover root and relevant nested
`AGENTS.md` / `CLAUDE.md`, imports/symlinks and referenced architecture, design,
run/test/deploy docs. Follow links needed to establish an operational claim;
exclude dependency/vendor/generated trees unless referenced. Do not traverse
every sibling repo or entire monorepo for a service-scoped request.

Record which instructions apply to each checked service, including inherited
and local rules. Use the actual harness's loading/override rules when known;
do not assume Codex, Claude Code and other hosts load the same files. If that
is unknown, report the assumption. Missing one filename is not itself a defect
when another entrypoint correctly serves the intended harness.

## Evidence checks
Inspect the following only where relevant or present:

| Surface | Compare against | Report when |
| --- | --- | --- |
| Instruction entrypoints and linked docs | Actual paths, link anchors, imports/symlink targets and service ownership | Broken links, circular imports, unreachable required guidance or instructions applying to the wrong service |
| Run/build/test commands | Package scripts, lockfiles, Makefile targets, compose, runner config and CI; resolve working directory and prerequisites | Command missing, wrong runtime/port/cwd, suites excluded from documented checks, required infrastructure omitted |
| Architecture and operational rules | Representative implementation and current configuration at the named path | Guidance contradicts reality, duplicates have drifted, ownership or exceptions are ambiguous |
| `.babysit/git-flow.yaml` | [git-flow](../shared/git-flow.md), actual refs and effective `bbs autopilot git-flow` output | Nonexistent base, conflicting release policy or unexplained override; intentional overrides are valid |
| `.babysit/qa.yaml` and local overrides | Effective local environment, service commands, credential variable names and meaningful flows | Wrong/missing target, hosted-only QA, no failure/empty/validation case, override changes target or credential source unexpectedly |
| Ignore rules and examples | Tracked filenames, `.gitignore`, `.env.example`, configuration references | Secret-bearing local files tracked or not ignored, committed credential literals, names pointing at the wrong environment |
| Related repo/workspace pointers | Only relevant entries in `~/.babysit/config.yaml`, named local paths and `RELATED_*_REPO` fallback | Referenced repo unavailable, roles disagree, machine paths committed, project-local `.babysit/config.yaml` mistaken for the registry |

Read [git-flow](../shared/git-flow.md) only for 2build git-policy checks.
Missing `.babysit` files or workspace registration in a repo that does not use
those features is not a finding. For CLI/library repos, validate their actual
check path instead of requiring a browser URL. An unavailable submodule or
machine-local repo is an evidence gap, not proof its documentation is wrong.

For instruction quality, keep project-specific constraints and failure lessons
that change decisions. Flag generic filler, repeated policy, stale examples or
large always-loaded detail only when they obscure a real task or drift from an
owner. Recommend one owning file plus precise pointers; do not delete rules
merely to hit a line/token budget. If implementation violates a deliberate
invariant, report that conflict rather than rewriting the invariant to match
broken code. Separate factual drift from policy decisions that need an owner.

## Validate without executing the instructions blindly
Check file existence, anchors and script/target definitions first. Parse YAML
with an available parser; CLI output is additional evidence, not a complete
schema validator. Inspect these read-only resolvers when 2build is configured:
```sh
bbs autopilot git-flow
bbs secrets qa default-env
bbs secrets qa probe --env <local-name>
```
Choose the actual local name from config; a simple top-level target is `local`.
Do not `eval` resolver output, source `.env`, or run commands copied from docs
just to audit them. Inspect tracked/ignored status without dumping secrets.
Capture and redact command output before reporting if URLs or legacy configs
contain credentials. Record file/key and risk, never secret values.

Resolve target/cwd/command consistency statically by default. A safe read-only
probe of an already running local service can strengthen evidence; report it
separately from configured intent. Execute application checks or start services
only when runtime verification is requested, after inspecting side effects and
prerequisites. Missing tools/services become `NOT RUN` with a reason. Never run
migration, rollback, seed, deployment or destructive prepare hooks to “verify”
documentation. A health response proves reachability, not auth or user flows;
use `browse` / `qa` for requested UI execution.

## Findings, optional fixes and output
For each actionable finding give severity, file:line, the conflicting evidence,
impact on a concrete agent task, and the smallest correction. Prioritize unsafe
commands/secret exposure, instructions that prevent execution or verification,
then drift and maintainability. Separate confirmed defects, suggestions and
unverified claims. Do not invent a numerical harness score.

If fixes are authorized, patch the owning file, preserve unrelated user edits
and existing policy, then recheck affected links/config/commands. Reuse
[setup-project](../setup-project/skill.md) when the requested fix is missing
2build configuration. Do not silently switch profile, release model, credential
names or command semantics as a documentation cleanup.

Return the report directly unless the caller requests a file or supplies a
handoff destination. Include inspected scope, prioritized findings, checks
performed, changes made and gaps. Findings do not prevent completing an audit;
inability to inspect its primary target does.
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: CLEAN | FINDINGS | INCOMPLETE
SCOPE: <entrypoints/services and linked files actually inspected>
VERIFY: <static checks, optional runtime evidence, NOT RUN and why>
NEXT: <smallest correction, missing evidence, or none>
```
`CLEAN` covers only the inspected scope and requires no unresolved material
verification gaps. `INCOMPLETE` means required evidence is missing; it is never
an application QA PASS. Report known findings even when the audit is incomplete.
