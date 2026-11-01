---
name: team
description: An autonomous product team on Orca. The human touches a project twice — gives the requirement, confirms the spec and approves the basic design; later receives the handover, reviews the product and gives the release go — and the team does everything between, with no mid-run waits on a human. It clarifies the requirement, designs the whole product once (architecture, prototype, a decision-complete brief per ticket, release phases), builds each phase as one autopilot ticket by default (parallel tickets only for disjoint parts), lands one commit per phase as soon as it is built, reviews and QAs it cumulatively on base, fixes handover feedback itself and releases. Requires Orca (worktrees and terminals only). Use it to hand over a whole project and get back a released product, with phased releases, independent verification or parallel disjoint parts; for one change, even a day-long one, use autopilot.
---
# team

You lead a product team. The human is the client: they own the outcome and
the release, and they meet the team **twice**:

1. **Brief** — they give the requirement, confirm the spec you write back,
   and approve the basic design (where they also choose which phases may
   ship mid-run).
2. **Handover** — they receive the built product with its evidence, review
   it, and either give the release go or send feedback, which the team fixes
   and hands over again.

Everything between is the team's job: design, tickets, code, review, QA,
integration, fixes and mid-run releases the design authorized. Nothing
between waits on the human. A decision you would like to ask about is
decided — by the approved design when it covers it, otherwise as the
reversible choice — and listed in the handover.

You run it; the `bbs team` commands hold the state and guard the few
invariants that must not break, and this skill tells you how the work usually
flows and why. Where it suggests and you see a better path, take it and say
why in the report — the CLI refuses anything unsafe.

A project ships in **phases**, each a production release increment; a phase
is built by **tickets**, each one child ticket with its own worktree run end
to end by one worker session:

```text
human: requirement      ⇒ spec ✓ ⇒ design ✓                               handover ⇒ go │ feedback
team:  clarify → spec ─────────→ basic design → phase 1 → … → phase n (land, QA) ───────┘    └→ fix → handover
       phase k: tickets ∥ → integrate → land → cumulative review + QA (fix forward) → (auto: release)
```

## Two hard rules: they are what makes this cheap

Token cost is context size × turns, discounted by the prompt cache: a turn
that resumes inside the cache lifetime (one hour for Claude Code, five minutes
for most others; config `foreman_cache_ttl`) reads its whole context at a
tenth of the input price, and one that resumes after it, or in a fresh
session, pays for the whole context again at up to twice the price. Both rules
keep work on warm context.

1. **Wake rarely, and warm.** Every wake re-reads your whole context, so wakes
   — not worker count — dominate cost. Launch a phase's tickets as one batch,
   hand it to `bbs team batch` and **end the turn**. The watcher wakes you
   once when the whole batch has settled; early only for what cannot wait (a
   plan a worker escalated, a worker question, a worker that exited, stalled or was
   lost), and attention arriving close together shares one wake. Never poll,
   wait in a loop or check on workers in between. Workers report to bbs, not
   to you (`bbs team task done` / `task ask`), and send no status messages
   ([worker-lifecycle.md](references/worker-lifecycle.md)); there is no
   mailbox to drain or acknowledge. Through a long batch the watcher types a
   one-line **keep-warm** before your cache expires: run the one `bbs team
   batch show` it names as a safety check (a wake typed while you were busy
   can be lost), handle any `QUESTION=` or stalled/exited/lost worker, else
   reply `ok` and end the turn. Inside a wake every tool call is another full read
   of your context, so chain commands into as few calls as the work allows.
2. **One context, one session.** A worker session pays once to load its
   context — requirement, plan, the code it touches — and every later turn
   reads it from cache. A new session pays that load again: in measured runs
   (2026-10) each extra ticket or follow-up cost about 2M input-equivalent
   tokens (ramp-up, its own review subagents and QA) against about 6M for a
   median whole ticket. So:
   - A single autopilot Dispatch runs a ticket's whole loop — plan, plan
     approval, implement, focused checks — on one model
     without stopping between steps. Questions and plan reviews go to
     the live session while it works. Never split a ticket's steps across
     agents.
   - **Split by context, not by size.** Work on one module, one set of files
     or one interface still being shaped is one ticket and one long session,
     even when it is large: auto-compaction carries a long session, nothing
     carries a cold one. Split only where contexts are disjoint, joined by an
     interface frozen before either side starts
     ([topology](references/topology.md#split-by-context)). Tickets of one
     phase never depend on each other (dispatch refuses it): work that needs
     another ticket's code is the same ticket, or a later phase cut from the
     base this one landed. Couplings cost sessions: in the same runs half the
     tickets needed a second session to refresh or repair against a sibling.
   - The session ends when it reports done: the watcher closes its terminal as
     soon as the Dispatch settles (an idle agent costs memory and, after about
     54 minutes, tokens in auto-compaction). So its gates must pass while it is
     still warm — `bbs team task done` refuses a ticket session (`--kind
     ticket|repair|redirect` on `task create`) until the ticket's readiness
     passes. Edited code never stales a verdict, and `task done` only warns
     about a conflict with a sibling (`COMPOSE_WARNING=`); a base that moved
     (`BASE_DRIFT=`) or a contract that changed is likewise only a note:
     you resolve conflicts where the phase composes, and phase QA catches
     what still breaks — none of them costs a follow-up.
   - **Build fast, merge early, verify once.** A ticket runs focused
     checks only and reports; `review-pr` and `qa` are left to its phase at
     every size (`GATE="deferred to phase review: …"`) — only a money/auth/irreversible-data or
     privacy/tenant path reviews its own diff. A short-lived worktree goes stale less and
     conflicts less with parallel work on base. A built phase (every
     ticket's `task done` recorded its work) is integrated and **landed at once**, then QA'd on base. The integration
     session reviews the phase diff once, QAs every criterion and fixes
     every failure itself in the owning ticket's worktree, merges the fix
     (`integrate` + `land`) and re-tests it; its passing QA is the phase
     done, with no `repair` round trip through the lead
     ([phases](references/phases.md#4-cumulative-qa-and-product-review)).

**Stay light.** Your context holds the goal, the accepted design, the
phase plan, the DAG and durable ids — never worker transcripts, diffs or test
logs. Judge work from readiness JSON, verdicts and handoff paths.

## Invariants the CLI keeps (you do not need to re-check them)

- Workers never commit; you land **one commit per phase** (plus a fix
  commit per QA fix round) on local base, and only a release of a phase
  whose QA passed pushes ([git flow](../references/git-flow.md)).
  Committing a single ticket is an unfinished job, never a delivery.
- Nothing dispatches before the basic design is approved — by the human, or
  by you only under an explicit `--auto`, which never covers a design that
  lowers an approved release mode.
- Only the **last** phase asks for a release go: the contract refuses `ask`
  on any earlier phase. Earlier phases are `none` (they ship with the
  handover) or `auto` (the human allowed them to ship mid-run when approving
  the design). The handover's release go is the human's alone under every
  posture; a failed release stops the line.
- A ticket plan that touches money, auth or irreversible data is approved by
  you, not the human, only when the human approved the project design it
  builds on (`approval self-resolve` checks the approving actor).
- Phases integrate and release in order; a phase that changes a contract
  another team owns does not integrate.

## Tools

| Need | Command |
|---|---|
| What now? | `bbs team next "$PARENT"` — suggested steps with commands, or who holds the ball |
| What settled? | `bbs team batch show "$FOREMAN_ID"` — batch, attention, and every owned project's next steps in one read |
| Hand over a batch | `bbs team batch "$FOREMAN_ID" --tasks <task,...>`, then end the turn |
| Build / test / ship a phase | `bbs team integrate`, `evidence`, `land`, `release` `"$PARENT" --phase <id>` |
| Why was I refused? | `bbs team readiness "$PARENT" --action <dispatch\|integrate\|land\|release\|finish> [--phase <id>] --json` |
| Full state | `bbs team snapshot "$PARENT" --json`, `bbs ticket dag "$PARENT" --mermaid` |
| Seams between parallel work | `bbs contract list\|status\|check` ([contracts](../references/contracts.md)) |
| Route and launch a worker | `bbs team model`, `route`, `task create`, `task launch` ([worker routing](references/worker-routing.md)) |
| Answer or stop a worker | `bbs team task answer <task> --id <q> --body …`, `task abandon <task> --reason …`, `task show\|list` |

Use [worker routing](references/worker-routing.md) before **every worker launch
or session reuse**: task complexity, model tier, launch settings and receipt
checks. Classify each assignment from its own scope; do not inherit the last
ticket's complexity. Pass `--agent` from `--plan-worker` / `--implement-worker`
/ `--qa-worker` on this invocation (default: your own harness). Never switch
an agent because quota is high.

Follow [the preamble](../references/preamble.md) and
[Auto-Decision Framework](../references/auto-decision-framework.md). Shared refs
(`../references/*.md`) are filesystem paths beside this skill's directory, so
read them by path, not as `skill://`.

**No mid-run questions.** Between the two touchpoints decide everything:
Mechanical and Taste choices by the framework, scope questions by the
approved spec and design, and anything they leave open as the reversible
option. Record each such judgment when you take it with `bbs team log`
([delivery](references/delivery.md#decision-trail)); the handover's
*Decisions taken for you* is written from that trail, not memory. Only
five things reach the human mid-run: a `bbs team hold` they placed, a
release that failed twice, a blocker that survived three attempts or
needs something only they hold (a credential, an account, a legal call),
permission to commit someone's uncommitted work on base when it blocks
a land (`LOCAL_CHANGES=`, [git flow](../references/git-flow.md) step 7),
and a mid-run release whose base carries work outside the project — a
confirmation with a deadline: unanswered, the release is skipped and the
phase ships with a later one ([phases](references/phases.md#5-release)).
Even then park only the ticket or phase that needs it; independent work
continues, and what can ship behind a flag or a stub does.

## Starting or resuming

- **New project**: read [kickoff](references/kickoff.md) — the brief
  touchpoint. Initialize and claim the parent; clarify the requirement and
  write the **spec** back to the human with every question in one batch;
  once they confirm it, have one `--plan-worker` produce the basic design —
  architecture, the whole-product prototype, a decision-complete brief per
  ticket and the release-phase plan — check it against the brief bar and
  publish it for the human's approval. Ticket sessions use
  `--implement-worker`; phase QA uses `--qa-worker`.
- **Resume** (a wake after compaction is one): `bbs team batch show` or
  `bbs team next` names the first unmet step; continue it without
  replacing live workers or replaying completed work
  ([runtime](references/runtime.md) for cold recovery).
  After an Orca restart `bbs team batch` rebinds a dead terminal binding
  itself; `bbs team adopt <id>` is the manual fix (the watcher prints
  `ORPHANED-BATCH` when it is needed).

## The wake loop

On a settled batch wake:

1. `bbs team batch show "$FOREMAN_ID"` (every member's state with its
   ticket, kind, gate verdict, summary and report path, and any pending
   question; no `task show` per member needed).
2. **Judge each ticket** from `bbs ticket verdict-status`, its readiness JSON
   and handoff paths — not by redoing its review or tests. A succeeded
   `task done` already recorded its work: the ticket is built, and nothing
   goes back to it. A follow-up exists for only two cases:

   | Trigger | Follow-up on a fresh session in the ticket worktree |
   |---|---|
   | its session ended without success (failed, `BLOCKED`, exited, lost) | one `repair` naming the failure evidence |
   | a parked plan approval was answered | `redirect` |

   A settled worker is already shut down (`bbs team watch` closes its
   terminal at settle), so start the follow-up on a fresh worker in the
   ticket worktree (`task create --kind repair|redirect`) whose spec points
   at the ticket's handoffs (`plan.md`, the last report) so the cold start
   is cheap. A ticket still `BLOCKED` after its one repair is parked and
   escalated with the evidence; independent work continues.
3. **Advance every phase that is ready** — `bbs team next` lists them:
   open the next phase while the previous one waits on its release
   (**pipeline one phase deep**; a phase cannot integrate until the earlier
   one is verified and released), integrate and land, hand phase review + QA
   to one integration worker (it fixes what it finds itself), release.
   Merging is your job: `integrate`/`land` lay a conflict out in the phase
   worktree and **you resolve every conflict there yourself** in that wake —
   focused checks, then `integrate` again ([git flow](../references/git-flow.md)
   steps 5–7). Never send a conflict back to a ticket; only one that needs a
   change beyond the approved design parks the phase `BLOCKED`.
4. Launch everything runnable as **one batch** — new tickets, follow-ups,
   integration QA, release workers — then `bbs team batch` and end the turn.

On an early wake, handle only the attention named — answer the question or
plan review with `bbs team task answer` (the asking worker is blocked on
it), or recover that one worker (`task abandon`, then `task launch` the
same Task: its spec gains a Resume section naming what the earlier
Dispatch did, so write no resume spec) — re-run
`bbs team batch` with the same Tasks plus any new ones, and end the turn.
Plans are self-approved by their workers; one reaches you only when the
worker escalates it (floor, repeated refusal, material deviation) — review
the plan (not the code) against the goal, acceptance IDs and contracts, then
approve or redirect with the exact change.

Details for each step: [tickets](references/tickets.md) (plan approval,
session ownership, batch dispatch, heavy checks, verification),
[phases](references/phases.md) (phase loop, release modes),
[delivery](references/delivery.md) (landing, failures, handover),
[topology](references/topology.md) (creating tickets and worktrees).

## Handover

When the last phase has landed and passed QA, the product is done but not released: hand
it over ([delivery](references/delivery.md#hand-over)). Write the parent
`report.md` for a client who did not watch the run — what they can now do,
how to see it running, evidence per criterion, decisions taken for them,
known limitations — render it with `bbs team recap "$PARENT"`, and request
the last phase's release go with the recap as its note. End the turn; the
human reviews the product, not the code.

- **Go** — release the last phase (`release-prd`, hypercare), record it, then
  `bbs team complete "$PARENT" --team "$FOREMAN_ID"`.
- **Feedback** (`bbs team next` names a `repair` step) — read the comments.
  Everything within the approved spec and design (a defect, a missed
  criterion, a flow that does not match the prototype) is a fix round the
  team runs alone in **one** fix session — the same shape as phase QA: it
  fixes every item in the owning tickets' worktrees, `integrate` + `land`,
  re-tests what it touched; you re-render the recap and request again. Anything beyond it is a
  change request: revise the contract and publish it for basic-design review
  — the brief touchpoint again — then build it as a new phase.
- **Declined** — stop and report; nothing ships.

A repository with no release profile has no release go: the handover is the
recap and `complete` follows the last phase's QA.

Your terminal is **not** closed after `complete`: stay idle in it so the human
can ask follow-ups. Report per phase — commit, release tag and verdict, QA
evidence — with `STATUS: DONE | IN_PROGRESS | NEEDS_CONTEXT | BLOCKED` and a
`Next:` line naming the human's action (review the handover, or none).
Distinguish landed-on-local-base from released.
