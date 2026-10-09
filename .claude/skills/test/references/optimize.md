# Measurement and affected lanes

Start with the actual package scripts and CI entry points. Distinguish unit,
DB/integration, browser, build/static analysis and release checks. Verify each
suite is collected somewhere, including suites excluded by a default config.
Do not invent CI where none exists or claim a cloud bill reduction from local
wall time alone.

Record baseline revision/environment, command, prerequisites, file/case counts,
failures/skips, elapsed time, and (if available) per-suite duration, CPU/worker
minutes, setup, retries and cache state. Fix invalid baselines before comparing
speed. Use comparable before/after runs; repeat noisy measurements only as
needed. Parallel work can lower latency while increasing billed worker time.

## Lane contract
Each lane needs:
- behavior and production boundaries it protects;
- runnable command, working directory and prerequisites;
- changed paths AND callers/contracts that select it;
- what is deliberately outside its claim;
- when the full suite, including this lane, runs.

Start conservatively with service-level lanes if dependency evidence cannot
justify finer selection. A shared auth, schema, pricing contract, generated
client, lockfile, runner or root config change selects every affected consumer.
For example, a broker webhook change also reaches its API consumer and reader
UI. A filename-only nearest-directory filter is not a dependency graph.

For automatic selection, define the diff base explicitly (PR merge base vs.
push range), include old AND new paths for renames/deletions, and include staged,
unstaged and untracked work locally. Unknown base/path/dependency, selector
failure or missing coverage mapping falls back to broad checks or a visible
block; never a successful empty run. Docs-only exclusion needs evidence that
no generated output, packaging or source-contract test consumes those docs.

Target policy: routine pushes/PRs run affected lanes plus mandatory checks;
stable releases run the full suite. Run broad checks periodically to catch
mapping drift. When CI optimization is requested, implement and validate that
selection within the authorized scope; a diagram alone does not optimize CI.
Honor existing authorization without asking for it again. If repository rules
require full checks on every push and changing that rule is outside the request,
keep it and report the constraint; affected lanes can still shorten local feedback.

## Changes worth measuring
Remove proven redundancy; replace source-shape tests with behavior tests;
isolate process-wide mocks; reuse safe setup; separate live-service tests from
hermetic checks with explicit prerequisites; shard only when fixture ownership
and resource budgets support concurrency. Missing DB/browser/config must fail
an explicitly selected integration lane instead of silently skipping it.

Validate a selector with representative single-service, shared-contract,
rename/delete, unknown-path and no-match cases. Force a selected test to fail
in a disposable fixture and check the runner propagates nonzero status. Check
collection parity for a full run: every previously meaningful test must still
run in its assigned lane, including special browser configs.
