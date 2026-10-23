# Release phases

Read at entry and cold resume with project-contract.md and execution.md. A
project ships as ordered **phases**; each phase is a production release
increment made of **tickets**, and each ticket is one autopilot worker session.
The CLI enforces the order below — `bbs foreman readiness` is the gate, never
this prose or Foreman's memory.

| Term | Is | Owner |
|---|---|---|
| Project | the parent ticket, its accepted contract and basic design | Foreman |
| Phase | a release increment: a phase ticket (direct child of the parent) whose children are that phase's tickets | Foreman |
| Ticket | one worktree, one worker session, plan → build → review → QA; work left uncommitted | its ticket worker |
| Phase commit | the phase's sealed tickets composed into **one commit** on local base | Foreman (`bbs foreman integrate` / `land`) |

## The phase plan

Basic design (project-contract.md) writes the phase plan into `project.json`:

```json
"phases": [
  {"id":"p1","title":"Walking skeleton: create and reload a task","seeds":["tasks"],"release":"ask"},
  {"id":"p2","title":"Assignment","seeds":["assignments","notify"],"release":"auto"}
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
- **2–5 tickets per phase, each `M` or `L`** on the size rubric, so one phase
  keeps several workers busy for hours. Tickets in one phase have disjoint file
  ownership and interface-first contracts named in the manifest; a dependency
  inside a phase is an explicit `blocked_by` edge the dependent awaits in
  session, which adopts the prerequisite's sealed work. A dependency on an
  earlier phase needs no edge: the ticket is cut from a base that already
  contains it.
- **`release`** per phase: `ask` (a human gives the go), `auto` (the approved
  design is the go), `none` (an internal step that ships with the next phase).
  Default phase 1 and any phase with a migration, auth, money or a public
  contract change to `ask`. A repository with no release profile uses `none`
  throughout and the project ends when its last phase lands.

A one-release project is one phase. A new contract always declares `phases`;
unphased contracts only resume.

## Phase loop

For phase `k`, in order. `bbs foreman next` suggests the step; every verb
re-checks its own gate, and a refusal names what is missing.

**Pipeline one phase deep:** while phase `k` waits on its release, open phase
`k+1` (readiness allows it once `k` has landed), so a human release decision
never idles the workers. Phase `k+1` cannot integrate until `k` is released.
A wake handles every phase that is ready — open, integrate, land, release —
before handing over the next batch. Phase QA always tests that exact commit
`integrate` built, in its integration worktree.

### 1. Open the phase

```bash
bbs foreman readiness "$PARENT" --action dispatch --phase "$K" --json
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
bbs foreman bind "$PARENT" --phase "$K" --child "$PHASE_TICKET"
```

Then create the phase's tickets per topology.md with one difference: `init
--parent "$PHASE_TICKET"` and `add-child` on the phase ticket. `bbs ticket
ensure --mode=worktree` cuts from local base, which already holds phase `k-1`
before its release pushes it. Bind each seed on the project: `bbs foreman bind "$PARENT"
--seed <key> --child "$TICKET"`. Launch the phase's tickets as one batch
(SKILL.md); end the turn.

### 2. Integrate the phase

When every ticket of the phase is sealed:

```bash
bbs foreman readiness "$PARENT" --action integrate --phase "$K" --json
bbs foreman integrate "$PARENT" --phase "$K" [--message "<subject>"]
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
on `--qa-worker` (default: this Foreman's harness) in the integration worktree. It never edits code. In that session it begins
evidence with the phase named (execution.md) — the CLI derives the surfaces
from the phase commit:

```json
{"kind":"integration","producer":"<integration dispatch id>","phase":"p2"}
```

and runs the `qa` skill's Foreman integration mode against **every criterion
of phases 1..k**, the cross-ticket journeys and adjacent regressions — not a
union of ticket PASS counts — then submits the evidence. When the contract
requires product review, a fresh read-only evaluator adds `kind: product`
evidence for the same phase (product-review.md). The CLI rejects evidence for a
phase that is not ready, a worktree that is not clean at the phase commit, or
a stale subject.

A small repair after phase QA does not restart the phase's QA. After the
repair's ticket is resealed and the phase is integrated again, carry the checks
that already passed on the earlier phase commit and re-test only the criteria
the repair touched:

```bash
bbs foreman evidence "$PARENT" --phase "$K" --carry --from "$INT_ATTEMPT,$PRODUCT_ATTEMPT" \
  --reason "repair touched only X" [--except AC-2.1,AC-2.2]
```

`--carry` writes current-subject evidence from the passed checks of the named
attempts of the same phase (never failed ones, never another phase's), minus
`--except`, with provenance, and readiness shows it as carried. Run a normal
`--begin` attempt for the excepted criteria; fresh and carried checks together
satisfy the cumulative check. Carrying is Foreman's judgment: if the repair
could affect a criterion, except it. A repair that touches shared code gets a
full fresh attempt.

QA failure → `repair` follow-ups on the owning ticket sessions, then reseal,
integrate again (a new commit) and run a fresh attempt. After three failed
attempts on the same blocker, park the phase and escalate; independent work
continues.

### 4. Land

```bash
bbs foreman readiness "$PARENT" --action land --phase "$K" --json
bbs foreman land "$PARENT" --phase "$K"
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
bbs foreman readiness "$PARENT" --action release --phase "$K" --json
```

- **`ask`** — `bbs foreman release "$PARENT" --phase "$K" --request --note
  "<what ships, QA evidence, rollback>"` writes `release-request.json` on the
  phase ticket and publishes a `release` approval over it. Relay it through
  the preamble's channel and list it under `Remaining`. Only a human resolves
  it; `approval self-resolve` refuses kind `release` under every posture and
  grant, and so must you. While it waits, phase `k+1` builds (step 1). End
  the turn; the go is picked up at the next wake — the open batch settling,
  or the watcher's periodic status prompt when nothing is running — never by
  polling the approval.
- **`auto`** — the approved project design is the durable go.

With a go, `release-prd` runs for the repository's release profile from local
base, including hypercare: it pushes base and deploys. For an `auto` phase the
integration assignment continues into it in the same session once the phase
landed and `readiness --action release` is ready — one wake per phase.
Otherwise dispatch a **release worker** (normal route). Then record the verdict:

```bash
bbs foreman release "$PARENT" --phase "$K" --file receipt.json
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
criterion. `readiness --action finish` reports each unresolved phase with its
state (`pending`, `building`, `sealed`, `integrated`, `delivered` (landed),
`release_failed`); only when all are resolved does `bbs foreman complete`
write the receipt.
