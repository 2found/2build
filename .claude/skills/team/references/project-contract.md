# Team project contract

Read with the team skill at entry and cold resume. The parent is the durable
review and delivery unit; a closed worker/worktree is never proof of delivery.

## Project design checkpoint

Before phase tickets, worktrees, or production dispatch:

1. The lead has written the clarified `requirement.md` (SKILL.md step 1).
   Dispatch the basic-design worker on the design route in the parent
   checkout to run `plan-draft` and, for user-facing work, `design-ui`,
   producing:

   - `design.md`: architecture — components, data model and migrations,
     interfaces between tickets, risks, rollout and rollback per phase — then
     one **ticket brief** per seed under a `## <seed key>` heading (below).
     Its `## Alternatives considered` section first lists 3–6 criteria taken
     from the spec (acceptance criteria, constraints, named risks), then names
     1–2 structurally different shapes for the one-way decisions —
     architecture, data model and migrations, ticket interfaces, phase plan —
     and why each lost against those criteria. "None viable" with the reason
     is an answer; the human reads this section at design review.
   - `prototype.html`: one prototype of the whole product across all phases,
     with transitions and empty/error states, each screen and state under an
     element id the owning brief links to. Non-UI designs use interfaces,
     examples and a workflow with visual prototype N/A.
   - `plan.md`: the phase plan — what each phase ships and why it is safe to
     release alone — and acceptance criteria.
   - `manifest.md`: one row per ticket — phase, stable seed key, scope, size,
     in-phase dependencies, owned files, shared contracts, acceptance
     ownership — per topology.md and [phases](phases.md#the-phase-plan).
     Runtime ids/status belong in relations/report, so progress does not
     rewrite the approved design.

   **Supplied by the human** — a prototype or design given with the request
   or linked from `AGENTS.md` — is accepted input, not a draft: link and
   reuse it as-is, and produce only what it leaves missing (screens, states,
   architecture, briefs, phase plan). Change it only where it contradicts the
   spec, and list each change for the design review.

   The basic design decides once, for every ticket, what tickets would
   otherwise each re-decide. A **ticket brief** is decision-complete for its
   seed: owned files and modules; the exact shape of every interface it
   provides or consumes, with the providing seed; its data and schema
   changes; the prototype ids of the screens and states it owns; its
   acceptance IDs; and the few choices it may make locally. The bar: an
   `XS`/`S` ticket builds from its brief alone, and an `M`/`L` ticket plans
   only what its brief leaves open. Before publishing, the lead checks the
   bar and returns gaps to the design worker's session: a seed without a
   brief, an interface without one owner and an exact shape, a user-facing
   seed without prototype ids, an architecture, data-model or contract
   choice left to a ticket, a one-way decision with no alternative weighed, or a phase split into several tickets without a
   reason [split by context](topology.md#split-by-context) accepts.

   Planning/review may precede approval; phase tickets and production edits
   may not. Write `project.json`, with its `phases`, through `bbs team
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
   the lead reviews the artifacts/prototype itself — confirming the general
   plan is its job — and fills the five-line rubric with evidence. Use
   `../semantic-decision/SKILL.md` kind `human-review` to judge that evidence
   against the accepted direction and delegated scope (shared human-review
   contract in `../references/semantic-decision.md`). Only `proceed` advances
   to self-resolution; send `revise` back to the planning worker's session,
   route `needs-human`, and record `blocked`. The lead publishes
   `project-plan`, then uses
   `approval self-resolve --team "$FOREMAN_ID" --rubric-file <path>` and logs
   telemetry. runtime.md owns flag adoption; recorded `auto: true` survives resume,
   omitted flags and restart. Old records default to human review. Holds/grants,
   non-delegable decisions and release modes still apply; no fake human verdicts.
   `--auto` never grants a release: the handover still waits for a human go,
   and a revised contract that lowers a seed's release gate — moving it from
   the handover into an `auto` release, directly or through a `none` phase
   that now ships with one — goes to human review even under `--auto`.
5. **Ticket plan checkpoints run inside the ticket session**, so planning flows
   into implementation without a worker switch; no plan waits on the lead by
   size:
   - `XS`/`S`: no plan and no record (an `S` touching money, auth or
     irreversible data plans as `M`).
   - Money, auth or irreversible data in an `M`/`L` plan is the decision
     floor. When the human approved the project design, that approval covers
     it: `self-resolve` clears the floor for a plan whose parent design a
     human approved, and the plan goes to you (`task ask`), never the human —
     review it at `high` effort against the brief. Under a design you
     approved yourself (`--auto`), the floor still reaches a human.
   - `M`/`L` — **self-approve**: the worker judges its plan against the
     accepted parent artifacts with `human-review`, writes the five-line
     rubric with named evidence, runs `approval publish --kind plan`, and
     resolves it with `approval self-resolve --team "$FOREMAN_ID"
     --rubric-file <path>` as this lead's delegate. Exit 4 (rubric) is
     repaired in session, at most twice. Exit 3 (floor) or 5 (hold/grant), a
     `needs-human` or `blocked` judgment, a third rubric refusal, or a
     material deviation from accepted scope or a shared contract becomes a
     `bbs team task ask` to the lead, and the session waits.

   the lead answers a plan ask from the ticket's `plan.md`/`design.md`, the
   parent goal, its acceptance IDs and the manifest's shared contracts. Approve
   with `BABYSIT_TICKET=<ticket> bbs ticket approval self-resolve --team
   "$FOREMAN_ID" --rubric-file <path>` (the lead's own rubric) and answer
   `approved`; or `approval resolve --action redirect --note <changes>` and
   answer with those exact changes — the worker revises and asks again, and
   after two redirects the lead approves with named conditions. A hold/grant
   refusal or a floor under a self-approved design parks the ticket: the
   worker reports `NEEDS_CONTEXT`, the ticket parks, the lead relays the
   pending approval through the preamble's channel, lists it under
   `Remaining`, and resumes it with a `redirect` on a fresh session once it
   resolves; unaffected work continues. A decision that reopens accepted
   scope is not taken mid-run: the ticket builds what the design says and
   the lead lists the change for the handover, where the human can ask for
   it as a change request.

   Never implement on an unapproved plan record. The lead audits each ticket's
   approval record and rubric at light verification. Keep these `kind=plan`
   reviews distinct from `project-plan`. Re-read `approval status` before
   dispatch and finish. On resume preserve live work, but gather
   missing/stale approval before new production dispatch.

This is the human's brief touchpoint; do not add routine final Taste
confirmation, and do not reopen it mid-run for anything the design covers.
Under `--auto`, the lead resolves Taste decisions; route unresolved User
Challenges through the preamble.

## Durable project report

At bounded reconciliation, delivery/gate transitions and completion, atomically
replace parent `report.md` via a sibling temporary file and set `pointers.report`.
`bbs team report <parent>` reads this saved snapshot after worktrees close.
Empty waits do not rewrite it. Events update affected rows only, retaining other
observation times; full reconciliation refreshes the whole snapshot. Link real
artifacts/receipts and include every required child (UNKNOWN when unreachable):

```markdown
# <Project title> — <parent>
Observed at: <UTC time> | Lead: <id> | Base: <branch>
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
bbs Tasks, gate evidence and handler receipts, never terminal closure or a
`task done` report alone. Cancelled scope still needs an accepted contract change.

| Delivery | Required evidence, including final acceptance PASS |
|---|---|
| `LANDED` | Every phase commit on local base (`bbs team snapshot` delivery rows) |
| `RELEASED` | `LANDED`, and every `ask`/`auto` phase's release record passed |
| `PENDING` / `UNKNOWN` | Missing gate / unreachable evidence with observation time |

Evidence-only projects report artifact acceptance/delivery explicitly. Until the
final gate passes, delivery stays PENDING even if individual handlers succeeded.
On a status request, reconcile before showing the snapshot. The report alone
never unlocks a gate on resume.

## Phase integration QA

Every code-bearing phase ends in one integration QA on its landed phase
commit ([phases](phases.md#2-integrate-the-phase)), proving every criterion of that
phase and all earlier ones — fresh for this phase's criteria and whatever its
diff touches, carried for the rest; the last phase's run is the project's final QA and
passes before the lead's `done` heartbeat. Per-ticket QA never substitutes for
it. Wholly evidence-only projects may record N/A with acceptance evidence.
The lead owns the evidence checks; one integration worker session on the normal
route reviews the phase diff and tests through the `qa` skill, and fixes small
findings in their owning ticket's worktree
([phases](phases.md#4-cumulative-qa-and-product-review)). The lead never tests
inline.

1. `bbs team integrate "$PARENT" --phase <k>` builds the phase commit and
   prints its detached integration worktree. A conflict names the ticket and
   files: `repair` that ticket, then integrate again. `bbs team land
   "$PARENT" --phase <k>` puts it on local base right away.
2. Begin evidence with `"phase"` (execution.md), then run the `qa` skill in
   that worktree with parent identity, the phase commit, the approved artifact
   revision and the ticket list, in its Team integration mode: no commits,
   edits of the integration worktree or topology changes. The runtime must serve that exact tree;
   probe the changed behavior first. Run the cumulative journeys,
   cross-ticket transitions and adjacent regressions. A shared resource (port, database,
   device) is taken with `bbs ticket surface acquire` and released on every
   outcome.
3. Persist `integration-qa.md` with commands, results, runtime identity,
   evidence links and the tested commit; set `pointers.integration_qa` and the
   parent `qa` verdict. A worktree that is no longer clean at the commit means
   STALE and a rerun, not PASS.
4. On FAIL/STALE, the session fixes what it can with `seal --fixup` and
   re-checks; `repair` the owning tickets for the rest, reseal, integrate and
   land the fix commit, and re-check what changed. PASS unlocks the release
   and removes the phase's worktrees ([delivery](delivery.md)).

## Executable completion

Use the before/after evidence attempts and `bbs team complete` protocol in
[execution.md](execution.md). Markdown reports remain readable context; the shared
CLI/dashboard evaluator is the finish gate. Never infer readiness from command
exit success alone: inspect `data.ready` and its reasons.
