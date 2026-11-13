# Release phases

Read at entry and cold resume with project-contract.md and execution.md. A
project ships as ordered **phases**; each phase is a production release
increment made of **tickets**, and each ticket is one autopilot worker session.
The CLI enforces the order below — `bbs team readiness` is the gate, never
this prose or the lead's memory.

| Term | Is | Owner |
|---|---|---|
| Project | the parent ticket, its accepted contract and basic design | The lead |
| Phase | a release increment: a phase ticket (direct child of the parent) whose children are that phase's tickets | The lead |
| Ticket | one worktree, one worker session, plan → build → review → QA; work left uncommitted | its ticket worker |
| Phase commit | the phase's finished tickets composed into **one commit** on local base (plus a fix commit per QA fix round) | The lead (`bbs team integrate` / `land`) |

## The phase plan

Basic design (project-contract.md) writes the phase plan into `project.json`:

```json
"phases": [
  {"id":"p1","title":"Walking skeleton: create and reload a task","seeds":["tasks"],"release":"none"},
  {"id":"p2","title":"Assignment","seeds":["assignments","notify"],"release":"ask"}
]
```

The CLI rejects a plan where a seed sits in no phase or in two, and requires
every `first_journey` criterion to be owned by phase 1. Design rules the
reviewer checks:

- **Phase 1 is a walking skeleton**: the thinnest end-to-end slice of the first
  journey, released to production. Later phases widen it.
- **Every phase is safe to release alone** with later phases unbuilt: unfinished
  surfaces stay dark (flag, unlinked route, admin-only); migrations are
  expand/contract across phases, never a destructive change in the phase that
  still needs the old shape; no rewrite of a large table in a deploy-time
  migration.
- **One ticket per phase by default**, up to `L` on the size rubric: one long
  session is cheaper and holds its context better than several cold ones. A
  phase splits only where [split by context](topology.md#split-by-context)
  allows it, and the plan names each split's reason. Tickets in one phase have disjoint file
  ownership and interface-first contracts named in the manifest, and prefer
  no dependency on each other. A `blocked_by` edge inside a phase is
  allowed: the dependent launches once its prerequisite's `task done`
  recorded its tree, and builds what it needs (a binary, a bundle) from that
  tree (`refs/bbs/tickets/<id>`) read-only, never editing it.
- **`release`** per phase follows the human's two touchpoints. The **last
  phase is `ask`**: its release go is the handover, the human's review of the
  whole product. Every earlier phase is `none` — landed, QA'd cumulatively and
  shipped with the next release — unless the human marked it `auto` at design
  approval, which makes that approval its go and ships it mid-run. Propose
  `auto` only where an early production release earns something (real users,
  a deadline, de-risking a migration) and say so in the phase plan; the
  contract refuses `ask` on any phase but the last. A repository with no
  release profile uses `none` throughout and the project ends when its last
  phase lands.
- **Merge a `none` phase into the next unless it earns its own boundary.** Every phase pays for its own integrate, land, review, QA and evidence (in measured runs a phase verification session cost about twice a ticket session), and a `none` phase ships nothing on its own. Keep a `none` phase separate only when the next phase's tickets build in parallel on its landed code, or when a migration needs a landed expand step before its contract step; the phase plan names which reason applies. Any other `none` phase joins the phase after it.

A one-release project is one phase. A new contract always declares `phases`;
unphased contracts only resume.

## Phase loop

For phase `k`, in order. `bbs team next` suggests the step; every verb
re-checks its own gate, and a refusal names what is missing.

**Pipeline one phase deep:** while an `auto` phase `k` releases, open phase
`k+1` (readiness allows it once `k` has landed), so a release never idles
the workers. Phase `k+1` cannot integrate until `k` is verified (and
released, unless `none`).
A wake handles every phase that is ready — open, integrate, land, release —
before handing over the next batch.

**Land first, verify after.** A built phase is integrated and landed on
local base in the same wake, then QA'd on that landed commit. Parallel work
sees it at once instead of drifting from it, and a phase never waits on QA to
land, so base cannot move under a tested commit. Local base is not
pushed — the push is the release — and release requires phase QA passed, so
nothing untested ships. A QA failure is fixed forward: a fix commit on base.

### 1. Open the phase

```bash
bbs team readiness "$PARENT" --action dispatch --phase "$K" --json
```

It requires the phase ticket bound, phase `k-1` **landed** (its commit is on
local base for the new tickets to start from) and every phase before `k-1`
resolved. So phase `k` builds while phase `k-1` runs phase QA or waits on its
release, never further ahead.

Create the phase ticket once (`ensure` resolves an existing identity first, so
clear it):

```bash
OUT="$(env -u BABYSIT_TICKET bbs ticket ensure --no-branch \
  --from-input "$PHASE_TITLE" --reason foreman-phase)"      # parse TICKET=
BABYSIT_TICKET="$PHASE_TICKET" bbs ticket init --parent "$PARENT"
BABYSIT_TICKET="$PARENT" bbs ticket add-child "$PHASE_TICKET"
bbs team bind "$PARENT" --phase "$K" --child "$PHASE_TICKET"
```

Then create the phase's tickets per topology.md with one difference: `init
--parent "$PHASE_TICKET"` and `add-child` on the phase ticket. `bbs ticket
ensure --mode=worktree` cuts from local base, which already holds phase `k-1`
before its release pushes it. Bind each seed on the project: `bbs team bind "$PARENT"
--seed <key> --child "$TICKET"`. Launch the phase's tickets as one batch
(SKILL.md); end the turn.

### 2. Integrate the phase

When every ticket of the phase is built (its succeeded `task done` recorded its work):

```bash
bbs team readiness "$PARENT" --action integrate --phase "$K" --json
bbs team integrate "$PARENT" --phase "$K" [--message "<subject>"]
```

Readiness requires every earlier phase resolved (landed, verified and, unless
`none`, released), so a release never ships another phase's unverified code. `integrate` composes the tickets' recorded snapshots,
in dependency order, into **one commit** whose parent is local base (default
message `<phase title> (phase <id>)`, body listing its tickets), and checks it
out detached in the phase worktree `<parent>_phase-<k>`
([location](../references/worktrees.md)). Re-running with the same subject
and base reuses the commit. Its JSON names each repository's `commit` and
`worktree`. Land it right away (§3).

A phase that does not compose — tickets against each other or against a
base that moved since they started — is laid out in the phase worktree with
markers; resolve it there and integrate again
([git flow](../references/git-flow.md) steps 5–6 own the rules). The
resolution needs no review of its own: it is part of the phase diff §4
reviews.

### 3. Land

```bash
bbs team land "$PARENT" --phase "$K"
```

Requires the phase built and integrated — no evidence yet. It fast-forwards
local base to exactly the phase commit; the phase and ticket worktrees stay
for QA and fixes. A base that moved since `integrate` is composed again by
land itself; a dirty base checkout that overlaps the phase needs the human's
permission to commit it (`land --commit-local`) — [git flow](../references/git-flow.md)
step 7.

### 4. Cumulative QA and product review

Dispatch one **phase integration worker** (normal route, `standard` profile)
on `--qa-worker` (default: this lead's harness) in the integration worktree,
which holds the landed phase commit.
It never edits the integration worktree. In that session it begins
evidence with the phase named (execution.md) — the CLI derives the surfaces
from the phase commit:

```json
{"kind":"integration","producer":"<integration dispatch id>","phase":"p2"}
```

A phase that is one ticket skips this worker: its phase session verifies
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
ticket PASS counts — then submits the evidence. The rest of phases 1..k
stands on the earlier phases' passing evidence, with no carry step:
re-running an untouched earlier journey every phase grows QA with the
project and proves nothing new. When the contract
requires product review, the **same session** then begins a `kind: product`
attempt on the same running phase commit and judges it as the product
evaluator (product-review.md): it built no ticket, so it is independent of the
work, and one session pays one cold start and one runtime setup instead of
two. `--begin` rejects a phase that is not landed or a worktree that is not
at the phase commit; once begun, a commit landed or an edit made while the
check runs never rejects its results, and recorded evidence stands on every
later commit of the phase. Passing evidence removes the phase's worktrees
(delivery.md).

**QA fixes what it finds; QA done is the phase done.** The integration
session holds the whole phase in context, and a cold `repair` session per
ticket measured 66–93 minutes and 2–7M tokens each, so a failure never goes
back through the lead. **Fix in one batch, not one finding at a time:**
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

That passing record is the phase done; nothing goes back to the lead for a
`repair`. Only a fix outside the approved scope or design ends the session
`BLOCKED` with the evidence. After three failed phase QA attempts `bbs team
next` answers `blocked`: park the phase and escalate; independent work
continues.

### Phase session: a one-ticket phase verifies itself

A phase of one ticket needs no second session: the ticket session already
holds the phase's whole context, and a cold integration session would load it
again. Create its Task with `--kind phase` (route and spec as a ticket's,
plus this section by link); one Dispatch then runs:

1. the ticket loop through its focused checks, then `bbs team task built
   <task> --dispatch <id>` — the same readiness gate and work snapshot a
   ticket's `task done` applies, without settling the Dispatch;
2. `bbs team integrate` and `land` of its phase (`task built` prints both
   commands);
3. §4 on the landed commit from the integration worktree: evidence
   `--begin`, `review-pr` on the phase diff (its finders are fresh
   subagents), `qa` through its executor subagent, and — when the contract
   requires it — product review in a **fresh subagent** that is given only
   the spec, the criteria, the prototype and the running app, never this
   session's plan or diff: the subagent is the independence a separate
   evaluator gave;
4. the batched fix round of §4 in its own ticket worktree (an open phase
   Dispatch does not hide its edits from `FIXUP=`), `integrate` + `land`,
   one re-test;
5. for an `auto` phase, the release (§5) in the same session;
6. `bbs team task done`, which refuses `succeeded` until the phase is
   delivered, released or skipped (`GATE="phase-verified: …"`).

While the Dispatch is open, `bbs team next` answers `wait` for that phase,
to this session as well, so the lead never dispatches an integration worker
beside it; once the phase is delivered only an `auto` release still waits on
the session. A phase
session that ends without success after `task built` leaves the phase built
or landed: the normal loop resumes and the lead dispatches the integration
worker. A phase of several tickets keeps the integration worker above:
`task create --kind phase` is refused on it.

### Review effort

Review spends effort on logic, not on merge mechanics:

| Change | Review |
|---|---|
| A whole phase diff, once | `medium` (the cap — never `high`) |
| A fix delta or a repair | none — re-run the focused checks and re-test the criteria it touches |
| A conflict resolution that changed logic | `low`, on that resolution only |
| A mechanical resolution (regenerated files, lockfiles, imports, both-sides list entries) with build and focused tests passing | none — the resolved files are named in the commit; log it |

The phase is reviewed once: a passing `<phase>/review` on any commit of the
phase stands after a fix-forward commit.

### 5. Release

Readiness `release` requires the phase landed and its phase QA passed,
earlier phases resolved and a release mode other than `none`:

```bash
bbs team readiness "$PARENT" --action release --phase "$K" --json
```

- **`ask`** — the last phase: this is the **handover**
  ([delivery](delivery.md#hand-over)). Render the recap, then `bbs team
  release "$PARENT" --phase "$K" --request --note "<recap path, what ships,
  rollback>"` writes `release-request.json` on the phase ticket and publishes
  a `release` approval over it. Relay it through the preamble's channel and
  list it under `Remaining`. Only a human resolves it; `approval
  self-resolve` refuses kind `release` under every posture and grant, and so
  must you. End the turn; the answer is picked up at the next wake (the
  watcher's periodic status prompt), never by polling. A redirect is
  feedback: `bbs team next` turns it into a fix round.
- **`auto`** — the approved project design is the durable go, on a clean
  base. When base also carries **work outside the project** — commits its
  upstream lacks that no phase made (other sessions, `land --commit-local`)
  or uncommitted changes in the base checkout — readiness lists it
  (`outside` in the snapshot) and the release needs a human go, because a
  release would ship or build from it. `bbs team release "$PARENT" --phase
  "$K" --request` publishes a confirmation naming that work, with a deadline
  (`--wait`, default `30m`). Ask without blocking: say it in your turn — the
  question, what the outside work is, *release* or *skip (recommended)*, and
  the deadline — and keep working; never `AskUserQuestion` here, it would
  hold you past the deadline. The human answers in the dashboard or tells
  you; record their answer with `BABYSIT_TICKET=<phase ticket> bbs ticket
  approval resolve --action approve|drop --note "<their words>"`. A go
  releases (the CHANGELOG names the outside commits it ships). A drop, or no
  answer by the deadline (`bbs team next` names it at the first wake after),
  is a skip: `bbs team release "$PARENT" --phase "$K" --skip --reason
  "<why>"`. A skipped phase resolves like `none` — the next phase integrates
  on it and it ships with a later release, whose QA covers it. The last
  phase is never skipped, and a failed release is fixed, not skipped.

With a go, `release` runs for the repository's release profile from local
base, including hypercare: it pushes base and deploys. For an `auto` phase the
integration assignment continues into it in the same session once phase QA
passed and `readiness --action release` is ready — one wake per phase.
Otherwise dispatch a **release worker** (normal route). Then record the verdict:

```bash
bbs team release "$PARENT" --phase "$K" --file receipt.json
```

```json
{"verdict":"RELEASED","tag":"v1.4.0",
 "record":"/abs/<git-common-dir>/bbs-release/v1.4.0.md",
 "repos":[{"dir":"/abs/primary/repo","sha":"<released source SHA>"}]}
```

The CLI checks the record exists and that each released SHA contains every
phase commit of phases 1..k. `ROLLED_BACK` or `PARTIAL` **stops the line**: no
later phase opens, integrates or releases until this phase releases. The
release session fixes forward itself, the way phase QA does (§4), and
releases again — no `repair` through the lead. An `auto` phase retries once under the design's go; an `ask` phase needs
a fresh `--request` go for every attempt, and from the second failure every
phase does, so a human sees a repeatedly failing release.

`none` phases are resolved once landed and verified; their code ships in the
next release, whose QA already covers it cumulatively.

## Finish

The last phase's integration QA is the project's final QA: with it every
criterion has passing evidence — on this commit, or on the earlier phase
commit that last touched it — and the verified, landed base is what the team hands over
([delivery](delivery.md#hand-over)). `readiness --action finish` reports each unresolved phase with its
state (`pending`, `building`, `built`, `landed` (QA pending), `delivered`
(landed and verified), `released`, `release_failed`); only when all are resolved does `bbs team complete`
write the receipt.
