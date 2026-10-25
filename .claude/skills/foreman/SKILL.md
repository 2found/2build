---
name: foreman
description: Autonomous Orca orchestrator for large projects — an advisor and toolkit for the coding agent running it. Clarifies the requirement, has a worker produce the basic design (architecture, prototype, plan) split into release phases for one human review, then runs each phase as parallel autopilot tickets — one worker session per ticket, plan through QA — integrates and QAs the phase cumulatively, and releases it to production before the next phase. Wakes only when a batch settles or a decision needs it. Requires Orca (worktrees and terminals only); use autopilot directly for one serial ticket.
---
# foreman

You run the project; Foreman is your toolkit and advisor. The `bbs foreman`
commands hold the state and guard the few invariants that must not break;
this skill tells you how the work usually flows and why. Where it suggests and
you see a better path, take it and say why in the report — the CLI refuses
anything unsafe.

A project ships in **phases**, each a production release increment; a phase is
built by **tickets**, each one child ticket with its own worktree run end to
end by one worker session:

```text
requirement → basic design ⇒ human review → phase 1: tickets ∥ → integrate → cumulative QA → land → release
                                            phase 2: tickets ∥ → …
```

## Two hard rules: they are what makes this cheap

1. **Wake rarely.** Every wake re-reads your whole context, so wakes — not
   worker count — dominate cost. Launch a phase's tickets as one batch, hand it
   to `bbs foreman batch` and **end the turn**. The watcher wakes you once when
   the whole batch has settled; early only for what cannot wait (an `L` plan
   review, a worker question, a worker that exited, stalled or was lost), and
   attention arriving close together shares one wake. Never poll, wait in a
   loop or check on workers in between. Workers report to bbs, not to you
   (`bbs foreman task done` / `task ask`), and send no status messages
   ([worker-lifecycle.md](references/worker-lifecycle.md)); there is no
   mailbox to drain or acknowledge.
2. **One ticket is one worker session.** A single autopilot Dispatch runs the
   ticket's whole loop — plan, plan approval, await and adopt prerequisites,
   implement, review, QA — on one model without stopping between steps.
   Questions and plan reviews go to the live session while it works. Never
   split a ticket's steps across agents: a fresh worker re-reads requirement,
   plan, code and history on a cold prompt cache. Prefer fewer, larger tickets
   (`M`–`L`) for the same reason. The session ends when it reports done: the
   watcher closes its terminal as soon as the Dispatch settles (an idle agent
   costs memory and, after about 54 minutes, tokens in auto-compaction), so a
   follow-up (repair, refresh, redirect) for a finished ticket goes to a
   **fresh** session in the same worktree.

**Foreman stays light.** Your context holds the goal, the accepted design, the
phase plan, the DAG and durable ids — never worker transcripts, diffs or test
logs. Judge work from readiness JSON, verdicts and handoff paths.

## Invariants the CLI keeps (you do not need to re-check them)

- Workers never commit; Foreman lands **one commit per phase** on local base,
  and only a release pushes ([git flow](../references/git-flow.md)).
  Committing a single ticket is an unfinished job, never a delivery.
- Nothing dispatches before the basic design is approved (by the human, or by
  you under an explicit `--auto`).
- An `ask` phase's production release needs a **human** go under every
  posture; a failed release stops the line.
- Phases integrate and release in order; a phase that changes a contract
  another Foreman owns does not integrate.

## Tools

| Need | Command |
|---|---|
| What now? | `bbs foreman next "$PARENT"` — suggested steps with commands, or who holds the ball |
| What settled? | `bbs foreman batch show "$FOREMAN_ID"` — batch, attention, and every owned project's next steps in one read |
| Hand over a batch | `bbs foreman batch "$FOREMAN_ID" --tasks <task,...>`, then end the turn |
| Keep a passing ticket's work | `bbs foreman seal <ticket>...` |
| Build / test / ship a phase | `bbs foreman integrate`, `evidence`, `land`, `release` `"$PARENT" --phase <id>` |
| Why was I refused? | `bbs foreman readiness "$PARENT" --action <dispatch\|integrate\|land\|release\|finish> [--phase <id>] --json` |
| Full state | `bbs foreman snapshot "$PARENT" --json`, `bbs ticket dag "$PARENT" --mermaid` |
| Seams between parallel work | `bbs contract list\|status\|check` ([contracts](../references/contracts.md)) |
| Route and launch a worker | `bbs foreman model`, `route`, `task create`, `task launch` ([worker routing](references/worker-routing.md)) |
| Answer or stop a worker | `bbs foreman task answer <task> --id <q> --body …`, `task abandon <task> --reason …`, `task show\|list` |

Use [worker routing](references/worker-routing.md) before **every worker launch
or session reuse**: task complexity, model tier, launch settings and receipt
checks. Classify each assignment from its own scope; do not inherit the last
ticket's complexity. Pass `--agent` from `--plan-worker` / `--implement-worker`
/ `--qa-worker` on this invocation (default: your own harness). Never switch
an agent because quota is high.

Follow [the preamble](../references/preamble.md) and
[Auto-Decision Framework](../references/auto-decision-framework.md). Shared refs
(`../references/*.md`) are filesystem paths beside this skill's directory, so
read them by path, not as `skill://`. Decide Taste choices; escalate missing
authority/context and honor explicit holds. A decision only a human can make
parks the one ticket or phase that needs it while independent work continues.

## Starting or resuming

- **New project**: read [kickoff](references/kickoff.md) — initialize and
  claim the parent, clarify the requirement (questions to the human in one
  batch, now), have one `--plan-worker` produce the basic design and phase
  plan, get it approved. Ticket sessions use `--implement-worker`; phase QA
  uses `--qa-worker`.
- **Resume** (a wake after compaction is one): `bbs foreman batch show` or
  `bbs foreman next` names the first unmet step; continue it without
  replacing live workers or replaying completed work
  ([runtime](references/runtime.md) for cold recovery).
  After an Orca restart `bbs foreman batch` rebinds a dead terminal binding
  itself; `bbs foreman adopt <id>` is the manual fix (the watcher prints
  `ORPHANED-BATCH` when it is needed).

## The wake loop

On a settled batch wake:

1. `bbs foreman batch show "$FOREMAN_ID"` (every member's state, report
   summary path via `bbs foreman task show`, and any pending question).
2. **Judge each ticket** from `bbs ticket verdict-status`, its readiness JSON
   and handoff paths — not by redoing its review or tests. Seal every one that
   passes, in one call. A gap goes back to **that ticket's session** as a
   follow-up:

   | Trigger | Follow-up on a fresh session in the ticket worktree |
   |---|---|
   | verification gap, review/QA finding, phase QA failure, phase does not compose | `repair` naming the exact evidence |
   | a prerequisite was resealed, or now passes after `AWAIT=not-ready`; a pinned contract changed | `refresh` |
   | a parked plan approval was answered | `redirect` |
   | `BLOCKED` after the worker's own retry budget | fresh read-only critical diagnosis worker, then `repair` |

   A settled worker is already shut down (`bbs foreman watch` closes its
   terminal at settle; config `foreman_keep_settled_workers: true` opts out), so
   there is no owner to decide: start the follow-up on a fresh worker in the
   ticket worktree, and make its spec point at the ticket's handoffs
   (`bbs ticket` handoff paths, `plan.md`, the last report) so the cold start
   is cheap.
3. **Advance every phase that is ready** — `bbs foreman next` lists them:
   open the next phase while the previous one waits on its release
   (**pipeline one phase deep**; a phase cannot integrate until the earlier
   one is released), integrate, hand phase QA to one integration worker, land,
   release.
4. Launch everything runnable as **one batch** — follow-ups, new tickets,
   integration QA, release workers — then `bbs foreman batch` and end the turn.

On an early wake, handle only the attention named — answer the question or
plan review with `bbs foreman task answer` (the asking worker is blocked on
it), or recover that one worker (`task abandon`, then relaunch) — re-run
`bbs foreman batch` with the same Tasks plus any new ones, and end the turn.
Plan reviews: `XS` none, `S`/`M` the worker self-approves, `L` you review the
plan (not the code) against the goal, acceptance IDs and contracts, then
approve or redirect with the exact change.

Details for each step: [tickets](references/tickets.md) (plan approval,
session ownership, batch dispatch, heavy checks, verification),
[phases](references/phases.md) (phase loop, release modes),
[delivery](references/delivery.md) (landing, failures, finishing),
[topology](references/topology.md) (creating tickets and worktrees).

## Finishing

The last phase's cumulative QA is the project's final QA. When every phase is
released (or landed, for `none`), release workers and leases, persist the
parent report and run `bbs foreman complete "$PARENT" --foreman "$FOREMAN_ID"`
([delivery](references/delivery.md#finish-the-project)). Report per phase —
commit, release tag and verdict, QA evidence — with `STATUS: DONE |
IN_PROGRESS | NEEDS_CONTEXT | BLOCKED` and a `Next:` line naming any human
action left. Distinguish landed-on-local-base from released.
