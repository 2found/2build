# Local delivery without CI runtime

`--local` changes the executor, not the release quality bar. Run checks,
cross-platform builds, artifact packaging and uploads from the operator's
machine. No Actions dispatch/rerun/watch dependency, CI artifact download,
Cloud Build submission or other remote build runner. GitHub and package APIs,
image registries and production rollout are still required remote operations.
Missing local tools, credentials or required capabilities are named blockers.

## Prevent automatic pipelines from racing the local release

A push of main, tag or formula commit, and a published release can each trigger
workflows. Skipping by commit message is not a sufficient general guard: not
all event types honor skip directives. Local builds cannot substitute for
required branch-protection checks without an existing authorized landing
policy. Do not change branch protection to make this mode work.

Read workflows at remote main as well as the candidate; inspect relevant
`push`, `create`, `release`, `workflow_run` and reusable/dispatch paths. Include
all automatic workflows that the selected release events would trigger, not
just the publisher. Resolve their current IDs/states via the GitHub API:

```sh
gh api --paginate "repos/$RELEASE_REPO/actions/workflows"
```

The profile specifies whether temporary workflow suspension is permitted for
local delivery. An explicit `--local` execution follows that declared policy;
if policy is absent, resolve this before the first workflow mutation, after
preparing the concrete release. Do not disable Actions globally or rewrite
workflow YAML. Check active/queued release runs and other release activity;
if they conflict, stop before suspension instead of cancelling others' work.

1. Persist each affected workflow ID, path, original state and restore command
   in the release record **before** changing it. Report the suspension scope
   in a progress update. Keep originally disabled workflows disabled.
2. Disable only affected, originally active workflows:

   ```sh
   gh workflow disable "$RELEASE_WORKFLOW_ID" --repo "$RELEASE_REPO"
   gh api "repos/$RELEASE_REPO/actions/workflows/$RELEASE_WORKFLOW_ID" --jq .state
   ```

   Verify `disabled_manually` before sending any triggering event. If one
   disable fails, restore the successful changes and stop before push.
3. Keep the guard until all main/tag/formula pushes and GitHub release events
   finish. Do not dispatch pipelines or use CI as fallback after local failure.
   Unexpected runs require inspection; do not claim zero CI usage or cancel
   unrelated runs.
4. Restore every workflow changed by this invocation to active immediately
   after the last triggering event, before hypercare/local installs where
   possible; restore on **failure** too. Query its state to verify:

   ```sh
   gh workflow enable "$RELEASE_WORKFLOW_ID" --repo "$RELEASE_REPO"
   ```

   Use a cleanup/finally path when scripting the release. On interruption,
   restore first on resume before continuing other work, then reacquire the
   guard if more release events remain. Preserve the saved original states;
   do not overwrite them with the currently disabled state. A restore failure
   is `BLOCKED`, naming the exact outstanding command; never hide it behind
   an otherwise successful publish. If another operator changed the workflow
   meanwhile, reconcile ownership rather than enabling it blindly.

GitHub documents [disable](https://cli.github.com/manual/gh_workflow_disable),
[enable](https://cli.github.com/manual/gh_workflow_enable) and
[skip directive limits](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/skip-workflow-runs).

## Publish from the exact local candidate

Map every required CI job to a local command, including cross-compilation,
generated packs, embedded dashboards, checksums, docs/download catalogs and
package-manager metadata. Use repo helpers that implement these outputs;
do not invoke a script labeled CI-only or forge `GITHUB_OUTPUT` to bypass it.

Inspect lifecycle hooks first. If build hooks change tracked inputs (for
example `go mod tidy`), reconcile and verify them before freezing the SHA/tag;
do not build different sources under an existing release identity. Upload
explicit expected files, never an unreviewed wildcard over a stale `dist/`.
Verify published bytes against local hashes before public completion.

Formula/download metadata may need a later main commit with real asset hashes.
Keep it separate from the immutable source tag, guard its push too, and verify
main still carries this version before updating pointers. Preserve a newer
concurrent version rather than downgrading it. This metadata commit does not
justify moving the release tag or rebuilding its artifacts.
