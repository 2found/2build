# Foreman project contract

Read with the Foreman skill at entry and cold resume. The parent is the durable
review and delivery unit; a closed worker/worktree is never proof of delivery.

## Project design checkpoint

Before child creation, worktrees, or production dispatch:

1. Dispatch a planning/design worker on the critical phase route in the parent
   checkout to run `plan-draft` and, for user-facing work, `design-ui`, producing:

   - `requirement.md`, `plan.md`, `design.md` and `prototype.html`: all ticket
     scopes, transitions, empty/error states and acceptance criteria. Non-UI
     designs use interfaces, examples and a workflow with visual prototype N/A.
   - `manifest.md`: stable seed keys, scopes, dependencies, shared contracts,
     acceptance ownership and integration checks. Runtime ids/status belong in
     relations/report, so progress does not rewrite the approved design.

   Parent planning/review may precede approval; child creation and production
   edits may not. Write `project.json` through `bbs foreman contract` (execution.md).
2. Publish the artifact-fingerprinted approval:

   ```bash
   BABYSIT_TICKET="$PARENT" bbs ticket approval publish --kind project-plan \
     --note "Review the project plan, design/prototype, and proposed tickets"
   ```

   It covers requirement, plan, design, prototype, manifest and project contract
   through their pointers. Changed/missing artifacts make `approval status`
   stale; refresh and republish. Never overwrite artifacts during pending human
   review. `redirected` requires rework/republish; `dropped` stops the project.
3. Default: present artifact links and proposed tickets for **human review** through
   the preamble's invocation channel; persist the answer with `approval resolve`.
   An Orca coordinator relays this checkpoint, not its own Taste answer.
   Silence is not approval. Only current `approved` unlocks children.
4. **`--auto` delegates the human design reviews** within authorized scope.
   Dispatch a critical-phase design reviewer to inspect the artifacts/prototype
   and fill the five-line rubric with evidence. Foreman verifies it, publishes
   `project-plan`, then uses
   `approval self-resolve --foreman "$FOREMAN_ID" --rubric-file <path>` and logs
   telemetry. runtime.md owns flag adoption; recorded `auto: true` survives resume,
   omitted flags and restart. Old records default to human review. Holds/grants,
   non-delegable decisions and finish policy still apply; no fake human verdicts.
5. Review child plans with critical-phase workers against accepted parent artifacts;
   Foreman applies their evidenced rubrics autonomously. Keep these `kind=plan`
   reviews distinct from `project-plan`. Material scope/design changes pause
   affected production and reopen the parent checkpoint; unaffected work continues.
   Re-read `approval status` before dispatch and finish. On resume preserve live
   work, but gather missing/stale approval before new production dispatch.

This is Foreman's human design checkpoint; do not add routine final Taste
confirmation. Under `--auto`, log Taste decisions; route unresolved User
Challenges through the preamble.

## Durable project report

At bounded reconciliation, delivery/gate transitions and completion, atomically
replace parent `report.md` via a sibling temporary file and set `pointers.report`.
`bbs foreman report <parent>` reads this saved snapshot after worktrees close.
Empty waits do not rewrite it. Events update affected rows only, retaining other
observation times; full reconciliation refreshes the whole snapshot. Link real
artifacts/receipts and include every required child (UNKNOWN when unreachable):

```markdown
# <Project title> — <parent>
Observed at: <UTC time> | Foreman: <id> | Run: <id> | Finish: <policy>
Project review: <human/auto, current approval revision, artifact links>
Execution: <verified/required tickets; running/queued/blocked counts>
Delivery: <PENDING / REVIEW_READY / PR_READY / LANDED_LOCAL / MERGED_REMOTE / UNKNOWN>
Integration QA: <PENDING / PASS / FAIL / STALE / N/A reason> — <branch>@<SHA>, <evidence>

| Ticket / work | Worker / phase | Branch / verified head | Review / QA | PR / merge | Cleanup / blocker |
|---|---|---|---|---|---|
| <id + meaningful title> | <Dispatch + current step> | <branch + SHA> | <evidence links> | <URL/state or land receipt> | <worktree/terminal state; blocker> |

Remaining: <what still has to happen, who owns it, next action>
```

Execution, delivery and cleanup are separate axes. Derive rows from ticket/DAG,
Orca, gate evidence and handler receipts, never terminal closure or `worker_done`
alone. Cancelled scope still needs an accepted contract change.

| Delivery | Required evidence, including final acceptance PASS |
|---|---|
| `REVIEW_READY` | Retained clean branches/worktrees |
| `PR_READY` | Verified PR heads, at least one open; report merged/total |
| `LANDED_LOCAL` | Verified heads on local base; state remote delivery separately |
| `MERGED_REMOTE` | Observed remote merges, not a local ancestor test |
| `PENDING` / `UNKNOWN` | Missing gate / unreachable evidence with observation time |

Evidence-only projects report artifact acceptance/delivery explicitly. Until the
final gate passes, delivery stays PENDING even if individual handlers succeeded.
On a status request, reconcile before showing the snapshot. The report alone
never unlocks a gate on resume.

## Pre-land integration QA

When tickets interact, add a Pre-land integration QA Task before landing them.
Acquire the parent surface lease and dispatch an integration worker on the
normal phase route to prepare the covered branches with `bbs ticket surface compose`,
then a read-only QA worker to test that composition. Record covered revisions
and acceptance evidence; changes invalidate the result. Revert only this known
scratch composition before landing. This check never replaces final project QA.
Per-ticket QA owns its own surface lifecycle; serialize all shared-surface work
through leases and treat contention as queued work.

## Final integration QA

Every code-bearing project has a final Integration QA Task. It runs **after
the selected finish handlers and before the Foreman `done` heartbeat**, even
when child tickets are independent. A pre-land integration check is additional
evidence, not a substitute. Wholly evidence-only projects may record N/A with
acceptance evidence. Use a read-only QA worker on the normal phase route; fixes
go to owning children. Foreman owns lease acquisition and evidence checks;
dispatch an integration worker on the normal phase route for surface preparation,
merges/composition and restoration
in steps 2–3 and 6. Wait for its settlement and verify the resulting refs before
QA or lease release. Never prepare or test the integrated surface inline.

1. Reconcile the complete required child set and current receipt/PR heads.
   Acquire the parent surface lease in every participating primary checkout;
   refresh its TTL during long tests and serialize with other projects.
   Record the original branch/HEAD and the current scratch marker before
   changing a surface. Require a clean checkout and no in-progress Git op.
2. **`finish: land`**: after the last per-ticket QA/scratch composition,
   revert only the known scratch composition before landing. Do not discard
   unrelated retained local lands; reconcile them or block that reset. Land
   required children in dependency order, verify their receipt heads remain
   on `<base>`, then test that actual `<base>` HEAD in the primary checkout.
   Never run `surface compose`, `surface revert`, `serve`, or a reset after
   those lands as preparation/cleanup for final QA. Keep the retained base.
3. **`finish: pr` or `review`**: fetch and record the intended base revision
   (`origin/<base>` for PRs; the configured local base for local review).
   Create a retained `qa/<parent>` branch with `git switch -c` from that
   exact revision in the leased primary checkout. Merge the verified child
   SHAs in dependency order with ordinary Git merges; for PRs, verify/fetch
   the actual current PR heads, not stale local branch tips. A missing head
   or failed fetch is BLOCKED, never permission to test an older revision.
   Record branch/base/child SHAs immediately on the parent in
   `integration-qa.md`. Never move `<base>` to build this composition.
   If the QA branch already exists, reuse it only when its recorded manifest
   and head exactly match; otherwise retain it and create a new uniquely
   suffixed `qa/<parent>-<attempt>` branch. No `-B`, force update or deletion.
   On conflict, abort only the merge this attempt started, preserve the QA
   branch, and dispatch conflict repair to an owning child before rebuilding.
4. Dispatch the QA worker on that **already prepared primary surface** with
   parent identity, branch/HEAD, approved artifact revision, and the full
   child/base manifest. Explicitly require the `qa` skill's Foreman final
   integration mode: no composing, resetting, topology changes, code fixes,
   or nested surface release. Foreman owns the lease. Runtime must serve
   that exact tree; probe the changed behavior after preparing/restarting it.
   Run the parent acceptance journeys including cross-ticket transitions and
   adjacent regressions, not just a union of child PASS counts.
5. Persist `integration-qa.md` with commands, results, runtime identity,
   evidence links and all tested revisions; set `pointers.integration_qa`.
   Persist the parent `qa` verdict too. Check branch/HEAD, clean tree, child
   heads/PR heads, base revision and approval revision again after the tests.
   Changed inputs mean STALE and a rerun, not PASS. Under `land`, compare the
   tested final base HEAD; under `pr`/`review`, compare the source base too.
6. Always finish environment cleanup while still holding the lease. Under
   `pr`/`review`, switch back to the recorded original branch only when safe;
   preserve its HEAD, the QA branch, and the prior scratch marker. Do not use
   `surface revert` as restoration. Under `land`, leave the tested base HEAD
   intact. Release leases on every terminal outcome; a restoration failure
   is a blocker reported with the recoverable checkout, never forced away.
7. On FAIL/STALE, dispatch child repairs and rerun their review/QA and finish
   handlers before rebuilding final QA. If a child's QA reset displaced
   retained lands, re-land and verify every required head before testing base
   again. Only current PASS (or justified evidence-only N/A), successful
   handlers and cleanup permit the final report and `done` heartbeat.

## Executable completion

Use the before/after evidence attempts and `bbs foreman complete` protocol in
[execution.md](execution.md). Markdown reports remain readable context; the shared
CLI/dashboard evaluator is the finish gate. Never infer readiness from command
exit success alone: inspect `data.ready` and its reasons.
