---
name: test
description: Create meaningful regression coverage, audit weak or redundant tests, and optimize test lanes using measured cost and change impact. Use for test creation, suite audits, failing-test investigation, or CI test optimization; browser journey QA stays with qa/browse.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# test
Prove behavior at the cheapest layer that can actually observe the failure.
A smaller suite is useful only if it retains its ability to catch regressions.

Modes: `create [scope]`, `audit [scope]`, `optimize [scope]`. With no mode,
infer it from the request; ordinary implementation uses `create` scoped to the
change, not a whole-project audit.

Follow [preamble](../shared/preamble.md) for bootstrap, telemetry and
standalone/ticket handling, and the
[Auto-Decision Framework](../shared/auto-decision-framework.md) for decisions.
Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`.

## Before adding a test
Read the changed behavior, its callers and existing tests. Answer briefly:
1. What observable behavior changes, and what requirement defines the answer?
2. What plausible input, state, failure or timing would break it?
3. Which existing tests already distinguish correct from broken behavior?
4. What is the lowest credible layer that observes that failure?
5. Is another test necessary, or does existing evidence suffice?

Use [semantic-decision](../semantic-decision/skill.md) kind `testcase` when
choosing affected existing cases; supply the behavior/caller map and evidence.
Preserve mandatory repository checks and regression reproducers regardless of
that choice. No ticket, provider or orchestration setup is needed to reason
about tests; use the calling LLM fallback if the decision CLI is unavailable.

Pure rules often need unit tests; SQL predicates/locks need real disposable DB
coverage; HTTP authorization needs the actual middleware/handler boundary;
React effects need a mounted hook/component; browser APIs or user journeys may
need `browse`/`qa`. A mock of the mechanism under test cannot prove it works.

## create
- Scout the package scripts, runner configuration, fixtures and existing
  coverage before choosing a new framework or dependency.
- Run relevant existing tests first. Add only missing behavior, boundaries and
  realistic failures. A docs-only or already-covered change may need no new test.
- Import and exercise production code. Keep expected results independent of
  the calculation being checked. Do not transcribe the implementation into a
  test-only model, mock the subject itself, or assert merely that a mock returns
  what the fixture supplied. A model is useful only when compared to production.
- Tie each new assertion to a named regression. For a bug fix, demonstrate
  failure before the fix when feasible; a bounded mutation in a disposable copy
  is another option. Verify the intended assertion failed, not setup/imports.
  Never mutate a user's dirty source to manufacture evidence.
- Stub external transport, not the contract being tested. Restore global
  state, clocks and resources. If tests pass alone but fail together, investigate
  runner isolation and fixture ownership before touching expectations.
- Run the affected lane plus required checks. Report what those tests cannot
  prove (e.g. DOM emulation is not browser playback or layout).

## When a test fails
Reproduce with the exact command and capture the failure. Classify it from
evidence: product regression, invalid fixture/expectation, isolation leak,
missing infrastructure, or an existing defect. Check alone and with neighbors
when order matters. Repair the responsible layer and rerun it.

Never weaken assertions, bless snapshots, catch errors into passing defaults,
comment tests out, add `.skip`/`.todo`, broaden excludes, use `|| true`, or raise
retries/timeouts merely to obtain green. An intentional behavior change can
require a new expectation, but cite the requirement and retain the invariant
it replaces. Skipping/quarantining a test requires explicit user authorization;
record the reason and restoration condition. Existing skips remain visible in
the report and never count as passed coverage. Missing required infra is
`NOT RUN`/`BLOCKED`, not a passing lane. Existing failures are investigated and
reported; calling one "unrelated" does not discharge it or authorize disabling it.

## audit
Inventory collected files/cases, scripts and CI/gates, then inspect candidates
in production context. A regex hit or high assertion count is only a lead.
Look for copied implementations, self-derived oracles, empty fixtures,
conditional assertions that never execute, swallowed failures, mocks that
ignore predicates, source-string checks claimed as runtime coverage, duplicate
cases, stale skips and tests absent from all documented runners.

For each finding record: file/test, claimed behavior, concrete defect it fails
to catch, evidence, and keep/rewrite/merge/delete with rationale. Architectural
source guards and fixed protocol constants can be legitimate contracts; don't
delete them just because they read source or assert literals.

Audit is read-only unless the request also authorizes cleanup/fixes. When
changing the suite, map every removed behavior to a retained/replacement test,
or explain why it was never meaningful. Replace weak coverage of a real risk
before removing it. Fewer tests and a green run alone prove neither coverage
nor correctness. Do not delete a failing test as a substitute for investigating.

## optimize
Read [measurement and lanes](references/optimize.md). Measure first; do not
assume the biggest file, most assertions or most cases dominate cost. Preserve
full-suite stable-release checks. For routine pushes/PRs, select affected lanes
from the behavior/caller map plus mandatory checks; do not default to the whole
suite when a validated impact map suffices. Never call a faster run with
missing coverage an optimization.

## Output
Return a concise behavior → failure → existing/new evidence → lane map. Include
exact commands and results, collection/pass/fail/skip/not-run counts when
available, prerequisites, and unresolved risks. Audit findings and optimization
measurements must separate observed facts from estimates. Publish the report
through the caller's handoff if a ticket exists; otherwise return it directly.
Do not create branches, commit or push. CI lane changes stay within the
requested optimization scope; deployment and release authority stay with the
caller.

```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: TESTED | AUDITED | OPTIMIZED | FAIL
SUMMARY: <behavior covered, meaningful removals, checks and remaining gaps>
NEXT: <remaining action or none>
```
`TESTED` means the selected checks ran and passed, not that unrun lanes passed.
An audit can finish with findings; unresolved failures prevent a test/release
PASS. Claim `OPTIMIZED` only with comparable measurements and retained coverage.
