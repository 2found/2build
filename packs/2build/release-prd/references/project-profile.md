# Project release profile

Keep the shared flow in the skill and project facts in the repo. Link the
profile from `AGENTS.md` or `CLAUDE.md`; `.babysit/release.md`, `RELEASE.md`, or
an existing inline release section all work. This is an agent-readable contract,
not a new CLI config schema. Resolve relative paths from the target repo root.
Never copy deploy settings or secrets out of their maintained owners.

| Field | Required decision |
| --- | --- |
| Identity/scope | Git remote, base branch, release units, components and dependency order |
| Authority | Delivery bounds, landing policy, whether local workflow suspension is permitted |
| Version | Authoritative files, synchronized mirrors, semver rule, tag template; first-release baseline |
| Changelog | File and baseline tag policy, public/private release-note boundaries |
| Verification | Commands + cwd, smoke and failure case, required platforms/prerequisites |
| Normal executor | Workflow/script, trigger, stage ownership and output evidence |
| Local executor | Equivalent local checks/build/package; affected automatic workflows and guard policy |
| Execution map | Per stage: workflow/job or script, normal executor, local command + cwd, dependencies and success evidence; derived against actual workflows |
| GitHub | Draft/public order, exact tag/SHA, expected assets/checksums/docs |
| Distribution | Package, registry, access, channel, command + cwd, integrity/install verification, or N/A |
| Deploy | App/environment/origin, command + cwd, exact artifact, migrations/rollback reference, or N/A |
| Hypercare | Window, cadence, metrics/log source, baseline, thresholds and rollback/stop behavior |
| Local follow-up | CLI installation owner, pinned update, PATH/version/smoke check; consumer/catalog scope |

An existing runbook can supply these fields. Independently released components
need their own version/tag and distribution/deploy rows. A submodule is a
separate repo; release it there and update its parent pointer only when parent
delivery is also in scope. An app with no package distribution marks that stage
N/A. A library can mark deployment N/A without skipping install/smoke checks.
An App Store submission names its actual status, never live while review is pending.

## Minimal example to adapt

```markdown
# Release profile

- Scope: one npm package; base main, land via the existing repo policy.
- Version: package.json + lockfile; tag v<version>; CHANGELOG.md from the
  previous published tag. Use the prepared version, otherwise semver from diff.
- Checks (repo root): pnpm install --frozen-lockfile; pnpm check.
- Normal: release.yml owns verify → tag → npm → GitHub public release.
- Local: temporary suspension of affected automatic release/check workflows
  is permitted for an explicit --local release; restore original states.
  Run the same checks locally; npm pack --ignore-scripts after the checked
  build; publish the reviewed tarball, then complete GitHub release.
- Distribution: use package.json publishConfig for registry/access; stable
  latest. Compare downloaded integrity and install/import in isolation.
- Deploy: N/A, library only.
- Hypercare: verify download and stable pointer twice over 2 minutes; failure
  stops completion; a mismatched public package needs a new version.
- Local follow-up: no global CLI; update a consumer only if selected by caller.
```

This example does not grant permission for any real package or workflow.
