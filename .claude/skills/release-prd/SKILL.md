---
name: release-prd
description: Release a repository or selected services to production, from version and changelog through main sync, GitHub release, package distribution, deployment and hypercare. Supports --local when CI quota is exhausted; use --dry-run for a read-only release plan.
---
# release-prd

Own the release through verified distribution, production behavior and local
consumer updates. Builder owns this delivery skill; autopilot's QA handoff
does not invoke it or grant release permission.

Follow [preamble](../references/preamble.md) for bootstrap/telemetry and the
[Auto-Decision Framework](../references/auto-decision-framework.md) for decisions.
Shared refs are filesystem paths beside this skill's directory, so read them by path, not as `skill://`.
For `--dry-run`, skip the mutating bootstrap and telemetry; read context directly.

## Invocation and scope

Options are agent instructions, not a `bbs release-prd` CLI command:

- `release-prd [project/service] [version|patch|minor|major]`: execute the
  requested release using the repo's normal pipeline and delivery policy.
- `--local`: run all checks, builds and distribution from this machine;
  bypass automatic CI. Read [local mode](references/local-mode.md) before any
  remote mutation. GitHub/registry APIs and production rollout remain remote.
- `--dry-run`: inspect local files and remote state, report resolved stages,
  commands, versions and gaps. No edits, commits, pushes, tags, workflow state
  changes, build/test hooks, publication, deployment or installs.
- `--prepare`: perform local version/changelog edits and checks; stop before
  remote mutations. Combine with `--local` to prepare that delivery path.
- `--resume <tag>`: inspect the recorded release and real external state;
  continue incomplete stages for the same version, SHA and mode.

A request to execute a production release authorizes its selected targets and
their required delivery steps; do not repeatedly ask for confirmation. Creating
this skill/profile, a dry run, a preparation request, or having credentials
does not authorize executing a release. Honor explicit bounds and repo policy;
escalate only unresolved scope, permission or irreversible-data decisions.
Do not expand a service release to siblings or upgrade unrelated local tools.

## Resolve the project contract

Read applicable `AGENTS.md`/`CLAUDE.md`, the profile explicitly linked there,
otherwise `.babysit/release.md` or `RELEASE.md`, git policy, deploy runbook,
version sources and actual release workflows/scripts. Resolve remote from git;
never infer a submodule's release identity from its parent's remote.

Use [profile contract](references/project-profile.md) when a profile is missing
or incomplete. An inline section in `AGENTS.md` is equally valid. Derive facts
from code/CI first; do not require a new config file to release a known project.
Missing facts block only their dependent actions. Every stage needs an owner
(`agent` or a named workflow), concrete verification, or `N/A` with a reason.
In local mode the agent owns every required stage.

### Discover CI and assign execution

Scan the target repo's `.github/workflows/` YAML files at the candidate and
remote base, plus referenced local reusable workflows, composite actions and
release/build scripts. Follow job `needs`, conditions, matrix targets, working
directories, environment requirements, artifact handoffs and lifecycle hooks.
Inspect referenced remote action/workflow definitions when their release work
has no known local equivalent. Do not assume a shell `run` step is portable to
this machine or that the profile's workflow list is still complete.

Persist an execution map in the release record before delivery:
`stage | workflow/job or script | normal executor | local command + cwd |
dependencies | success evidence`. Derive this from the profile **and** actual
CI; report conflicts. In normal mode, trigger and wait only for stages owned
by CI, while the agent executes its assigned stages. Match the exact workflow,
source SHA and intended run; wait for its required jobs to finish successfully,
then verify published outputs. Poll with progress updates; failed, cancelled,
quota-blocked or missing required runs block dependent stages.

`--local` overrides every CI executor with its mapped local equivalent and
preserves dependencies, checks, platforms and artifact verification. No CI
watch/wait or CI artifact dependency remains. Inspecting run state to detect
a competing release is allowed; it is not a delivery gate. A missing local
equivalent is a blocker, never a skipped job or silent fallback to CI.

Resolve branch, selected components, version/tag policy, changelog baseline,
checks, artifacts/registries, deploy order, rollback target, hypercare window,
failure thresholds and local CLI/consumer updates. If version was not given,
use the repo rule; absent one, recommend the smallest semver bump supported by
the API/behavior changes and explain it. Do not overwrite an already prepared
version or use a prerelease as stable. Preserve independently versioned packages.

## Release record and recovery

Before mutating, write a release record under
`<git-common-dir>/bbs-release/<tag>.md` (resolve with `git rev-parse
--git-common-dir`), or the caller's existing evidence destination. It is local
git metadata, never a tracked secret-bearing report. Record scope, profile,
mode, old/new version, baseline, source SHA, artifact hashes, permissions,
workflow restoration state, rollback identity, timestamps and stage results.
Use `pending / running / verified / failed / N/A`; record intent before an
external action and evidence after it. Never store credentials or raw secrets.

On resume, the record is a pointer, not proof: recheck remote tag SHA, release
draft/public state, published artifact integrity and deployed revision before
skipping work. A timeout is not proof publication failed. Query before retrying;
retry a transient read at most three times, and retry a mutation only after
confirming its outcome. Do not move published tags or replace published assets;
a different artifact/source needs a new version. Report partial success rather
than undoing a package publication or declaring the whole release complete.

## Flow

1. **Inspect and reconcile.** Inventory dirty/staged files, branch, local
   commits, remote main, tags, authentication and concurrent release work.
   Fetch the configured remote/base and tags; inspect ahead/behind/divergence.
   Follow repo merge/rebase policy without force pushes. Preserve unrelated
   user changes; use a clean release checkout when the build context would
   include them, without creating a new branch contrary to repo policy. Do not
   stash/reset another person's work. Include only requested changes.

2. **Version and changelog.** Update the authoritative version and its required
   mirrors, lockfile metadata and generated release files together. Derive
   changelog from actual changes since the previous release of this component
   (diff + commits, not just commit titles). On the first release, state the
   baseline rather than guessing a previous tag. Describe user-visible changes,
   breaking changes, migration/compatibility notes and upgrade steps. Reuse
   this content for GitHub notes; exclude secrets/private operational details.

3. **Verify the candidate.** Run the profile's required checks and meaningful
   QA, including a non-happy-path case. Library/CLI QA can be a failing input
   or isolated install/import; UI QA uses `browse`/`qa` when available. In local
   mode run CI-equivalent checks locally, including all distributed targets;
   unavailable required checks block release rather than becoming a waiver.
   Commit intended changes according to repo policy, then fetch/reconcile main
   again before pushing. A changed candidate invalidates affected verification
   and artifacts. Record the final release SHA; remote main must contain it
   before release. For a PR policy, require the authorized landing path and
   check the resulting merge SHA; never silently bypass branch protection.

4. **Sync remote main and release on GitHub.** In normal mode, let the named
   pipeline own tagging/distribution it already implements; follow the run for
   this SHA (`gh run list --commit <sha> --workflow <file>`), not just the last
   green run. CI quota exhaustion is a blocker unless `--local` was requested;
   offer that concrete recovery path. For agent-owned releases, push main,
   create/push the exact configured tag at the verified SHA and create the
   release with `--verify-tag --notes-file`. Verify the remote tag resolves to
   that SHA. Stage a draft when artifacts/distribution gates must precede
   public release. Respect the project's publication order; a pipeline that
   publishes npm before making its GitHub draft public is valid. Before any
   push/tag/release event in local mode, apply the workflow guard.

5. **Distribute.** Publish only selected packages/binaries using the profile's
   registry, access, channel and credentials. Build from the verified source;
   inspect the actual packed artifact and publish those exact bytes. Read
   [npm distribution](references/npm.md) for npm targets. Verify download,
   exact version, integrity, expected assets and channel/formula pointers;
   a green publish command alone is insufficient. If CI owns this, verify its
   actual outputs. Complete/publish the GitHub draft after its required gates.

6. **Deploy.** Capture the previous active revision/image first. Deploy only
   selected apps in dependency order from the same source/artifacts; record
   image digests and production origin. Never substitute an ambiguous `latest`
   image. Follow the current runbook for migrations and traffic changes;
   image rollback does not roll back schema. In local mode build locally and
   push the resulting image, then call the rollout tool directly or a proven
   `--skip-build` wrapper with the exact tag. A wrapper invoking Cloud Build
   is not a local build. No deployed app means `N/A`, not an invented deployment.

7. **Post-deploy and hypercare.** Verify public health/readiness plus changed
   user behavior and a non-happy-path case against production. Use safe test
   accounts; do not create charges or messages without scope authorization.
   Compare errors, latency, restarts and relevant queues/jobs to the pre-deploy
   baseline for the configured window; record timestamped observations at
   start and end plus intermediate samples. No observable traffic/metrics
   means a named evidence gap, not a clean bill of health. Apply the configured
   stop/rollback rule when breached, verify recovery, and stop later dependent
   deploys. Otherwise update the selected local CLI/consumer from the released
   artifact, inspect PATH collisions and run version + smoke checks. Pin the
   version, or prove the upgrade channel resolves to this release. Verify any
   website/download catalog the release is expected to update. Restore local
   mode's workflow states on success, failure or interruption; missing
   restoration blocks completion. Finish only after required post-release work.

## Output

```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: RELEASED | PREPARED | DRY_RUN | PARTIAL | ROLLED_BACK
RELEASE: <repo + component + version/tag + source SHA + mode>
STAGES: <version/changelog; main; GitHub; distribution; deploy; hypercare; local update>
EVIDENCE: <record path + release URL + artifact/deployed identity + checks>
WORKFLOWS: <unchanged or restored IDs; outstanding restoration if any>
NEXT: <none or exact incomplete action>
```

`RELEASED` requires every applicable stage verified. `PREPARED` and `DRY_RUN`
mean their requested scope completed, never a production release. Required
publication/deploy/observation/update failures are `BLOCKED` + `PARTIAL`;
`DONE_WITH_CONCERNS` is for nonblocking concerns only.
