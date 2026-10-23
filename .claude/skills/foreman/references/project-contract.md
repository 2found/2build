# Foreman project contract

Read with the Foreman skill at entry and cold resume. The parent is the durable
review and delivery unit; a closed worker/worktree is never proof of delivery.

## Project design checkpoint

Before phase tickets, worktrees, or production dispatch:

1. Foreman has written the clarified `requirement.md` (SKILL.md step 1).
   Dispatch the basic-design worker on the design route in the parent
   checkout to run `plan-draft` and, for user-facing work, `design-ui`,
   producing:

   - `design.md`: architecture — components, data model and migrations,
     interfaces between tickets, risks, rollout and rollback per phase.
   - `prototype.html`: one prototype of the whole product across all phases,
     with transitions and empty/error states. Non-UI designs use interfaces,
     examples and a workflow with visual prototype N/A.
   - `plan.md`: the phase plan — what each phase ships and why it is safe to
     release alone — and acceptance criteria.
   - `manifest.md`: one row per ticket — phase, stable seed key, scope, size,
     in-phase dependencies, owned files, shared contracts, acceptance
     ownership — per topology.md and [phases](phases.md#the-phase-plan).
     Runtime ids/status belong in relations/report, so progress does not
     rewrite the approved design.

   Planning/review may precede approval; phase tickets and production edits
   may not. Write `project.json`, with its `phases`, through `bbs foreman
   contract` (execution.md).
2. Publish the artifact-fingerprinted approval:

   ```bash
   BABYSIT_TICKET="$PARENT" bbs ticket approval publish --kind project-plan \
     --note "Review the basic design, prototype, and phase/release plan"
   ```

   It covers requirement, plan, design, prototype, manifest and project contract
   through their pointers. Changed/missing artifacts make `approval status`
   stale; refresh and republish. Never overwrite artifacts during pending human
   review. `redirected` requires rework/republish; `dropped` stops the project.
3. Default: present artifact links, the phases with their release modes and
   the proposed tickets for **human review** through
   the preamble's invocation channel; persist the answer with `approval resolve`.
   An Orca coordinator relays this checkpoint, not its own Taste answer.
   Silence is not approval. Only current `approved` unlocks children.
4. **`--auto` delegates the human design reviews** within authorized scope.
   Foreman reviews the artifacts/prototype itself — confirming the general
   plan is its job — and fills the five-line rubric with evidence. Use
   `../semantic-decision/SKILL.md` kind `human-review` to judge that evidence
   against the accepted direction and delegated scope (shared human-review
   contract in `../references/semantic-decision.md`). Only `proceed` advances
   to self-resolution; send `revise` back to the planning worker's session,
   route `needs-human`, and record `blocked`. Foreman publishes
   `project-plan`, then uses
   `approval self-resolve --foreman "$FOREMAN_ID" --rubric-file <path>` and logs
   telemetry. runtime.md owns flag adoption; recorded `auto: true` survives resume,
   omitted flags and restart. Old records default to human review. Holds/grants,
   non-delegable decisions and release modes still apply; no fake human verdicts.
   `--auto` never grants a release: an `ask` phase still waits for a human go,
   and a revised contract that lowers a phase's release mode (`ask` to `auto`
   or `none`) goes to human review even under `--auto`.
5. **Ticket plan checkpoints run inside the ticket session**, so planning flows
   into implementation without a worker switch. The larger of the ticket's
   manifest size and its plan's `ticket_size` picks the approver, so a plan
   that grows to `L` goes to Foreman and none shrinks out of its review:
   - `XS`: no plan and no record.
   - `S`/`M` — **self-approve**: the worker judges its plan against the
     accepted parent artifacts with `human-review`, writes the five-line
     rubric with named evidence, runs `approval publish --kind plan`, and
     resolves it with `approval self-resolve --foreman "$FOREMAN_ID"
     --rubric-file <path>` as this Foreman's delegate. Exit 4 (rubric) is
     repaired in session, at most twice. Exit 3 (floor) or 5 (hold/grant), a
     `needs-human` or `blocked` judgment, a third rubric refusal, or a
     material deviation from accepted scope or a shared contract becomes an
     Orca `ask` to Foreman, and the session waits.
   - `L` — **Foreman approves**: the worker writes the rubric, publishes the
     plan record and asks Foreman with the artifact paths; the session waits.

   Foreman answers a plan ask from the ticket's `plan.md`/`design.md`, the
   parent goal, its acceptance IDs and the manifest's shared contracts. Approve
   with `BABYSIT_TICKET=<ticket> bbs ticket approval self-resolve --foreman
   "$FOREMAN_ID" --rubric-file <path>` (Foreman's own rubric) and answer
   `approved`; or `approval resolve --action redirect --note <changes>` and
   answer with those exact changes — the worker revises and asks again, and
   after two redirects Foreman approves with named conditions or parks the
   ticket. A floor/hold/grant refusal that Foreman's authority cannot clear,
   or a decision that reopens accepted scope, parks the ticket: the worker
   reports `NEEDS_CONTEXT`, the ticket parks, Foreman relays the pending
   approval through the preamble's channel, lists it under `Remaining`, and
   resumes it with a `redirect` on a fresh session once it resolves. Material
   scope/design changes reopen the parent checkpoint for the affected tickets
   only; unaffected work continues.

   Never implement on an unapproved plan record. Foreman audits each ticket's
   approval record and rubric at light verification. Keep these `kind=plan`
   reviews distinct from `project-plan`. Re-read `approval status` before
   dispatch and finish. On resume preserve live work, but gather
   missing/stale approval before new production dispatch.

This is Foreman's human design checkpoint; do not add routine final Taste
confirmation. Under `--auto`, Foreman resolves Taste decisions; route
unresolved User Challenges through the preamble.

## Durable project report

At bounded reconciliation, delivery/gate transitions and completion, atomically
replace parent `report.md` via a sibling temporary file and set `pointers.report`.
`bbs foreman report <parent>` reads this saved snapshot after worktrees close.
Empty waits do not rewrite it. Events update affected rows only, retaining other
observation times; full reconciliation refreshes the whole snapshot. Link real
artifacts/receipts and include every required child (UNKNOWN when unreachable):

```markdown
# <Project title> — <parent>
Observed at: <UTC time> | Foreman: <id> | Run: <id> | Base: <branch>
Project review: <human/auto, current approval revision, artifact links>
Execution: <verified/required tickets; running/queued/blocked counts>
Delivery: <PENDING / LANDED / RELEASED / UNKNOWN>

| Phase | Release mode | State | Cumulative QA | Release |
|---|---|---|---|---|
| <id + title, phase ticket> | <ask/auto/none> | <pending…released / release_failed> | <PASS/FAIL/STALE> — phase commit <SHA>, <evidence> | <tag + verdict + record, or pending go> |

| Ticket / work | Phase / worker step | Sealed snapshot | Review / QA | Landed in | Cleanup / blocker |
|---|---|---|---|---|---|
| <id + meaningful title> | <phase; Dispatch + current loop step> | <refs/bbs/tickets/id @ SHA> | <evidence links> | <phase commit SHA or pending> | <worktree/terminal state; blocker> |

Remaining: <what still has to happen, who owns it, next action>
```

Execution, delivery and cleanup are separate axes. Derive rows from ticket/DAG,
Orca, gate evidence and handler receipts, never terminal closure or `worker_done`
alone. Cancelled scope still needs an accepted contract change.

| Delivery | Required evidence, including final acceptance PASS |
|---|---|
| `LANDED` | Every phase commit on local base (`bbs foreman snapshot` delivery rows) |
| `RELEASED` | `LANDED`, and every `ask`/`auto` phase's release record passed |
| `PENDING` / `UNKNOWN` | Missing gate / unreachable evidence with observation time |

Evidence-only projects report artifact acceptance/delivery explicitly. Until the
final gate passes, delivery stays PENDING even if individual handlers succeeded.
On a status request, reconcile before showing the snapshot. The report alone
never unlocks a gate on resume.

## Phase integration QA

Every code-bearing phase ends in one integration QA on its phase commit
([phases](phases.md#2-integrate-the-phase)), testing every criterion of that
phase and all earlier ones; the last phase's run is the project's final QA and
passes before the Foreman `done` heartbeat. Per-ticket QA never substitutes for
it. Wholly evidence-only projects may record N/A with acceptance evidence.
Foreman owns the evidence checks; one integration worker session on the normal
route tests through the `qa` skill — never fixing code. Foreman never tests
inline.

1. `bbs foreman integrate "$PARENT" --phase <k>` builds the phase commit and
   prints its detached integration worktree. A conflict names the ticket and
   files: `repair` that ticket, then integrate again.
2. Begin evidence with `"phase"` (execution.md), then run the `qa` skill in
   that worktree with parent identity, the phase commit, the approved artifact
   revision and the ticket list, in its Foreman integration mode: no commits,
   code fixes or topology changes. The runtime must serve that exact tree;
   probe the changed behavior first. Run the cumulative journeys,
   cross-ticket transitions and adjacent regressions. A shared resource (port, database,
   device) is taken with `bbs ticket surface acquire` and released on every
   outcome.
3. Persist `integration-qa.md` with commands, results, runtime identity,
   evidence links and the tested commit; set `pointers.integration_qa` and the
   parent `qa` verdict. A worktree that is no longer clean at the commit means
   STALE and a rerun, not PASS.
4. On FAIL/STALE, `repair` the owning tickets, reseal, integrate again and
   rerun. On PASS, `bbs foreman land "$PARENT" --phase <k>` moves base to
   exactly the tested commit ([delivery](delivery.md)).

## Executable completion

Use the before/after evidence attempts and `bbs foreman complete` protocol in
[execution.md](execution.md). Markdown reports remain readable context; the shared
CLI/dashboard evaluator is the finish gate. Never infer readiness from command
exit success alone: inspect `data.ready` and its reasons.
