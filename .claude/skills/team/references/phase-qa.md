# Phase QA, phase sessions and evidence

Read by the phase integration (QA) worker, a handover-fix worker and a
`--kind phase` ticket session; the lead's Task spec for those links this file.
The lead does not need it at entry: it reads [phases](phases.md) for the loop
and [execution](execution.md) for scope and readiness.

## Cumulative QA and product review

The **phase integration worker** (normal route, `standard` profile, on
`--qa-worker`) works in the integration worktree, which holds the landed phase
commit, and never edits it. It begins evidence with the phase named
([evidence](#evidence)); the CLI derives the surfaces from the phase commit. A phase that is one ticket has no such worker: its phase session verifies
it ([below](#phase-session-a-one-ticket-phase-verifies-itself)).

The session then **reviews the phase diff once**: `review-pr --fix` is not used here —
run `review-pr` on `git diff <base_head> <phase commit>` at the effort in
[review effort](#review-effort) and record it as
the check `{"criterion":"<phase id>","kind":"review"}` whose log is the
review report. Readiness requires a passing one whenever a ticket of the phase
deferred `review-pr` (every ticket off the floor). It then runs the `qa` skill's
Team integration mode — through the skill, whose browser and test execution
runs in its QA executor subagent while this session only judges, never as
browser or test work on the session's own thread — against **every criterion of this phase**, **every
prototype state** in its spec's `## Prototype` section (qa § prototype
conformance — a phase with an unchecked state does not pass), the
cross-ticket journeys, adjacent regressions and each earlier phase's
criterion whose files or flows this phase's diff touches — not a union of
ticket PASS counts — then submits the evidence. The runtime must serve that
exact tree: probe the changed behavior first. A shared resource (port,
database, device) is taken with `bbs ticket surface acquire` and released on
every outcome. The rest of phases 1..k
stands on the earlier phases' passing evidence, with no carry step:
re-running an untouched earlier journey every phase grows QA with the
project and proves nothing new. When the contract
requires product review, the **same session** then begins a `kind: product`
attempt on the same running phase commit and judges it as the product
evaluator ([product-review.md](product-review.md)): it built no ticket, so it is independent of the
work, and one session pays one cold start and one runtime setup instead of
two. `--begin` rejects a phase that is not landed or a worktree that is not
at the phase commit; once begun, a commit landed or an edit made while the
check runs never rejects its results, and recorded evidence stands on every
later commit of the phase. Passing evidence removes the phase's worktrees
([delivery](delivery.md#when-the-phase-is-landed-and-verified)).

**QA fixes what it finds; QA done is the phase done.** The session
fixes every failure itself, forward on base: it holds the whole phase in
context, and a cold `repair` session per ticket measured 66–93 minutes and
2–7M tokens each, so a failure never goes back through the lead. **Fix in one batch, not one finding at a time:**
finish review, QA and product review first, collect every failing criterion
and material finding from all three, then fix them all in one round — each
fix-and-land round is another integrate, land, runtime restart and re-test.
For that one batch the session itself:

1. edits the **owning ticket's worktree** (`bbs team snapshot` lists it; its
   Dispatch must be settled), never the integration worktree, and runs the
   focused checks of what changed;
2. merges once: `bbs team integrate "$PARENT" --phase "$K"` snapshots each
   ticket whose worktree holds such a fix (`FIXUP=<ticket>`) and composes
   them as one fix delta on the landed phase; then `bbs team land` it;
3. re-tests every criterion it fixed in one normal `--begin` attempt on the
   fix commit. The fix delta is never reviewed ([review effort](#review-effort)),
   and everything that already passed stands. A criterion that still fails
   starts the next batch.

That passing record is the phase done (persist `integration-qa.md` with the
commands, results, runtime identity, evidence links and tested commit; set
`pointers.integration_qa` and the parent `qa` verdict); nothing goes back to
the lead for a `repair`. Only a fix outside the approved scope or design ends the session
`BLOCKED` with the evidence. After three failed phase QA attempts `bbs team
next` answers `blocked`: park the phase and escalate; independent work
continues.

## Phase session: a one-ticket phase verifies itself

A phase of one ticket needs no second session: the ticket session already
holds the phase's whole context, and a cold integration session would load it
again. Create its Task with `--kind phase` (route and spec as a ticket's,
plus this file by link); one Dispatch then runs:

1. the ticket loop through its focused checks, then `bbs team task built
   <task> --dispatch <id>` — the same readiness gate and work snapshot a
   ticket's `task done` applies, without settling the Dispatch;
2. `bbs team integrate` and `land` of its phase (`task built` prints both
   commands);
3. [Cumulative QA](#cumulative-qa-and-product-review) on the landed commit from the integration worktree: evidence
   `--begin`, `review-pr` on the phase diff (its finders are fresh
   subagents), `qa` through its executor subagent, and — when the contract
   requires it — product review in a **fresh subagent** that is given only
   the spec, the criteria, the prototype and the running app, never this
   session's plan or diff: the subagent is the independence a separate
   evaluator gave;
4. the batched fix round above in its own ticket worktree (an open phase
   Dispatch does not hide its edits from `FIXUP=`), `integrate` + `land`,
   one re-test;
5. for an `auto` phase, the release ([phases §5](phases.md#5-release)) in the same session;
6. `bbs team task done`, which refuses `succeeded` until the phase is
   delivered, released or skipped (`GATE="phase-verified: …"`).

## Review effort

Review spends effort on logic, not on merge mechanics:

| Change | Review |
|---|---|
| A whole phase diff, once, ≤ ~200 changed lines and no money, auth, irreversible-data, privacy or tenant path | `low` |
| A whole phase diff, once, anything larger or touching such a path | `medium` (the cap — never `high`) |
| A fix delta or a repair | none — re-run the focused checks and re-test the criteria it touches |
| A conflict resolution that changed logic | `low`, on that resolution only |
| A mechanical resolution (regenerated files, lockfiles, imports, both-sides list entries) with build and focused tests passing | none — the resolved files are named in the commit; log it |

The phase is reviewed once: a passing `<phase>/review` on any commit of the
phase stands after a fix-forward commit.

## Evidence

The phase commit is built and landed first (`bbs team integrate`, `land`,
[phases](phases.md#2-integrate-the-phase)). In its integration worktree,
before testing, create a verification attempt:

```sh
bbs team evidence "$PARENT" --begin --file /absolute/check-spec.json
```

```json
{"kind":"integration","producer":"actual-qa-dispatch-id","phase":"p2"}
```

The CLI captures the phase subject (the contract plus every ticket of phases
1..k) and derives the surfaces from the phase commit; it refuses a phase that
is not ready, or an integration worktree that is not clean at that commit.
Store the returned attempt ID. Tests run in that worktree, against that exact
commit; earlier phases' commits must already be on base. After tests, run:

```sh
bbs team evidence "$PARENT" --attempt <id> --file /absolute/results.json
```

```json
{
  "checks":[{"criterion":"AC1","kind":"behavior","command":["actual","command"],"exit_code":0,"log":"/absolute/real-check.log"}],
  "findings":[]
}
```

Record every required criterion/check pair and real exit code. Also record the phase review as `{"criterion":"<phase id>","kind":"review",...}`
with the review report as its log ([above](#cumulative-qa-and-product-review));
a passing one stands on every later commit of the phase, and a fix delta is
never reviewed ([review effort](#review-effort)). The producer
archives logs and hashes them; changed inputs reject results. Failures remain
durable; repairs get a fresh attempt. A preview requires `preview` HTTP(S) URL,
`runtime` full tested SHA, and a passing `runtime` check whose probe log contains
that SHA exactly. The probe must query the running build; echoing an expected
SHA proves nothing. The dashboard labels the observation time, never current
server availability.

When the contract requires product review, the integration session runs it
next as the read-only product evaluator ([product-review.md](product-review.md));
a separate evaluator Task is the exception named there. Its identity must
differ from child gate producers — in a phase session, which built the
ticket, the evaluator is a fresh subagent ([phase session](#phase-session-a-one-ticket-phase-verifies-itself)). Final evidence and every
criterion check use `kind: product`; `kind: journey` is only for an explicitly
required preliminary check. Material findings require repair and fresh evidence;
minor findings stay in the handoff.

## Progress reports

The `bbs team progress` report shape is in [worker lifecycle](worker-lifecycle.md#progress-reports).
