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
| Phase commit | the phase's sealed tickets composed into **one commit** on local base | The lead (`bbs team integrate` / `land`) |

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
  ownership and interface-first contracts named in the manifest; a dependency
  inside a phase is an explicit `blocked_by` edge the dependent awaits in
  session, which adopts the prerequisite's sealed work. A dependency on an
  earlier phase needs no edge: the ticket is cut from a base that already
  contains it.
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

A one-release project is one phase. A new contract always declares `phases`;
unphased contracts only resume.

## Phase loop

For phase `k`, in order. `bbs team next` suggests the step; every verb
re-checks its own gate, and a refusal names what is missing.

**Pipeline one phase deep:** while an `auto` phase `k` releases, open phase
`k+1` (readiness allows it once `k` has landed), so a release never idles
the workers. Phase `k+1` cannot integrate until `k` is released.
A wake handles every phase that is ready — open, integrate, land, release —
before handing over the next batch. Phase QA always tests that exact commit
`integrate` built, in its integration worktree.

### 1. Open the phase

```bash
bbs team readiness "$PARENT" --action dispatch --phase "$K" --json
```

It requires the phase ticket bound, phase `k-1` **landed** (its commit is on
local base for the new tickets to start from) and every phase before `k-1`
release-resolved. So phase `k` builds while phase `k-1` waits on its
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

When every ticket of the phase is sealed:

```bash
bbs team readiness "$PARENT" --action integrate --phase "$K" --json
bbs team integrate "$PARENT" --phase "$K" [--message "<subject>"]
```

Readiness requires every earlier phase release-resolved, so a release never
ships another phase's code. `integrate` composes the tickets' sealed snapshots,
in dependency order, into **one commit** whose parent is local base (default
message `<phase title> (phase <id>)`, body listing its tickets), and checks it
out detached in `<repo>/.babysit/worktrees/<parent>_phase-<k>`. Base itself
does not move yet. Re-running with the same subject and base reuses the commit,
so a retry never invalidates QA already recorded on it. Its JSON names each
repository's `commit` and `worktree`.

A phase that does not compose is refused with each ticket and file:
`repair` the later ticket — it runs `bbs ticket adopt <earlier ticket>`,
resolves the conflict, re-runs its gates and is resealed — then integrate
again. Never resolve a conflict outside a ticket session.

### 3. Cumulative QA and product review

Dispatch one **phase integration worker** (normal route, `standard` profile)
on `--qa-worker` (default: this lead's harness) in the integration worktree.
It never edits the integration worktree. In that session it begins
evidence with the phase named (execution.md) — the CLI derives the surfaces
from the phase commit:

```json
{"kind":"integration","producer":"<integration dispatch id>","phase":"p2"}
```

then **reviews the phase diff once**: `review-pr --fix` is not used here —
run `review-pr medium` on `git diff <base_head> <phase commit>` (`high` when
any changed path touches money, auth or irreversible data) and record it as
the check `{"criterion":"<phase id>","kind":"review"}` whose log is the
review report. Readiness requires a passing one whenever a ticket of the phase
deferred `review-pr` by size or took a fixup. It then runs the `qa` skill's
Team integration mode against **every criterion of phases 1..k**, the
cross-ticket journeys and adjacent regressions — not a union of ticket PASS
counts — then submits the evidence. When the contract
requires product review, the **same session** then begins a `kind: product`
attempt on the same running phase commit and judges it as the product
evaluator (product-review.md): it built no ticket, so it is independent of the
work, and one session pays one cold start and one runtime setup instead of
two. The CLI rejects evidence for a phase that is not ready, a worktree that
is not clean at the phase commit, or a stale subject.

**Fix small findings in the same session.** The integration session holds
the whole phase in context; a cold `repair` session per ticket measured 66–93
minutes and 2–7M tokens each, mostly re-reviewing what was already reviewed.
For each failing criterion or material finding — from review, QA or product
review — whose fix is local (one ticket's files, no change to scope, a shared
contract, or a money/auth/irreversible-data path):

1. edit the **owning ticket's worktree** (`bbs team snapshot` lists it; its
   Dispatch must be settled), never the integration worktree, and run the
   focused checks of what changed;
2. `bbs team seal <ticket> --fixup "<finding, evidence id>"` — it waives
   the ticket's now-stale gates and marks the next phase commit as needing a
   fresh review;
3. `bbs team integrate "$PARENT" --phase "$K"` again (it re-checks out the
   same worktree), then the re-check below: review only the fixup delta
   (`git diff <old phase commit> <new>`) at `low` and record a fresh
   `<phase>/review` check — a review is never carried — and re-test the
   criteria the fixes touched.

Its report lists every finding it could not fix that way, by owning ticket.
The lead sends each such ticket **one** `repair` carrying all of its items,
launched as one batch: fixing findings one at a time costs a repair session,
a reseal and a phase re-check per finding.

A small repair after phase QA does not restart the phase's QA. After the
repairs' tickets are resealed and the phase is integrated again, one
integration session re-checks it: carry the checks that already passed — from
both the integration and the product attempt — on the earlier phase commit,
and re-test only the criteria the repairs touched:

```bash
bbs team evidence "$PARENT" --phase "$K" --carry --from "$INT_ATTEMPT,$PRODUCT_ATTEMPT" \
  --reason "repair touched only X" [--except AC-2.1,AC-2.2]
```

`--carry` writes current-subject evidence from the passed checks of the named
attempts of the same phase (never failed ones, never another phase's), minus
`--except`, with provenance, and readiness shows it as carried. Run a normal
`--begin` attempt for the excepted criteria; fresh and carried checks together
satisfy the cumulative check. Carrying is the lead's judgment: if the repair
could affect a criterion, except it. A repair that touches shared code gets a
full fresh attempt.

QA failure the integration session cannot fix → `repair` follow-ups on the
owning ticket sessions, then reseal, integrate again (a new commit) and run a
fresh attempt. After three failed
attempts on the same blocker, park the phase and escalate; independent work
continues.

### 4. Land

```bash
bbs team readiness "$PARENT" --action land --phase "$K" --json
bbs team land "$PARENT" --phase "$K"
```

Requires current passing phase evidence on the current phase commit. It
fast-forwards local base to exactly that commit — base history gains one
commit for the phase — then removes the integration worktree and every ticket
worktree whose content still equals its seal (delivery.md). Local edits in a
checkout of base ride along unless they touch a file the phase changes; then
land refuses before anything moves and names the files. If base moved since
the build, land refuses: integrate again and re-run phase QA.

### 5. Release

Readiness `release` requires the phase landed, earlier phases resolved and
a release mode other than `none`:

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
- **`auto`** — the approved project design is the durable go.

With a go, `release-prd` runs for the repository's release profile from local
base, including hypercare: it pushes base and deploys. For an `auto` phase the
integration assignment continues into it in the same session once the phase
landed and `readiness --action release` is ready — one wake per phase.
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
later phase opens, integrates or releases until this phase releases. Fix
forward: `repair` on the owning ticket, reseal, then a new phase commit:
integrate, QA, land, release again. An `auto` phase retries once under the design's go; an `ask` phase needs
a fresh `--request` go for every attempt, and from the second failure every
phase does, so a human sees a repeatedly failing release.

`none` phases are resolved once landed; their code ships in the next
release, whose QA already covers it cumulatively.

## Finish

The last phase's integration QA is the project's final QA: it covers every
criterion, and its landing is what the team hands over
([delivery](delivery.md#hand-over)). `readiness --action finish` reports each unresolved phase with its
state (`pending`, `building`, `sealed`, `integrated`, `delivered` (landed),
`release_failed`); only when all are resolved does `bbs team complete`
write the receipt.
