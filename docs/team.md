# Team: an autonomous product team

Team is a product team you hire for a whole project. You meet it twice:

1. **Brief.** You give the requirement. The team reads your codebase, writes the **spec** back to you — journeys, numbered acceptance criteria, non-goals, the assumptions it took — with every open question in one batch. You confirm it. The team then designs the whole product once: architecture, a clickable prototype, a decision-complete brief for every ticket and a release-phase plan. You approve that design, and while doing so you choose whether any phase should go to production before the end (`auto`); by default nothing ships until you say so.
2. **Handover.** When every phase is built, reviewed, QA'd and landed on your local base, the team hands it over: a recap page with what you can now do, how to see it running, evidence for each acceptance criterion, the decisions it took for you and how to reverse them (from its decision trail, `bbs team log`, with a **Check first** list of any decision whose evidence does not resolve or is not verified), known limits, and what the release will deploy. You review the product, not the code, and answer the release request: **approve** and the team releases it with hypercare; **send feedback** and it fixes everything within the spec on its own and hands over again — a request beyond the spec comes back to you as a design change; **decline** and nothing ships.

Nothing between those two meetings waits on you. Questions that come up mid-run are decided by the approved spec and design, or as the reversible option, and listed in the handover. Money, auth and irreversible-data changes were approved when you approved the design that names them; the team lead reviews those ticket plans at high effort on your behalf. Only three things can reach you mid-run: a hold you placed (`bbs team hold`), a release that failed twice, and a blocker that survived three attempts or needs something only you hold, such as a credential — and each parks only the work it blocks.

**When to use it.** Start with [Autopilot](../README.md#try-your-first-ticket): one long session, even a day-long one, is the fastest and cheapest way to deliver one change, and you stay close to it. Hand a project to Team when you want to step away and come back to a finished product: a release split into phases, an independent fresh-session review and QA of every phase, or parallel work on disjoint services or repos. It costs a lead session, a design session and an integration session per phase on top of the work.

The team lead is a coding agent; `bbs team` is its toolkit, not a script. The commands keep the state and the few invariants that must hold — one commit per phase, a human go for the handover release, phase order, contract ownership — and the skill plus `bbs team next` suggest the path. Two rules decide the token bill and stay hard: the lead wakes once per settled batch, and each ticket runs start to finish in one worker session. `bbs foreman` and `/bbs:foreman` still work as aliases.

## Configure Team

Team requires [Orca](https://www.onorca.dev) for worktrees and worker terminals; the Task/Dispatch lifecycle (launch, questions, reports) is bbs's own (`bbs team task`), so no Orca orchestration, mailbox or heartbeat is involved. It selects a worker's agent from `--plan-worker` (basic design), `--implement-worker` (ticket sessions), and `--qa-worker` (phase QA); omitted means the lead session's own harness. Legacy 2build YAML agent/provider/model/effort preferences are retired and ignored.

2build selects the worker's model and effort from task complexity (`simple`, `normal`, or `hard`) and phase class: `design` for basic design (architecture, prototype, phase plan), `critical` for ticket workers and diagnosis, `normal` for integration, QA, release and cleanup. The policy maps these combinations to `lite`, `flash`, `pro`, or `max` tiers, each with model bindings for the selected agent; for Claude these are `haiku`, `sonnet`, `opus` at `medium` effort, and `opus` at `high` effort; for Codex `gpt-6-luna`, `gpt-6.1-sol`, `gpt-6.1-sol`, `gpt-6-astra`; for OMP the `@smol`, `@slow` and `@plan` roles, with the `normal` role
expressed by omp's own configured default (omp has no `@normal` alias).
Ticket workers and phase integration run on `pro`; simple mechanical re-checks run on `flash`, basic design runs on `pro`, and `max` is reserved for `hard` tickets and `hard` designs, and a `hard` route must name the invariant its own diff changes (`--hard-reason money|auth|security|live-data|concurrency`). Pin the harness with `--plan-worker`, `--implement-worker`, and `--qa-worker` at spawn or adopt; usage does not switch the agent. Explicit phase overrides and valid persisted resume routes take precedence.

Inspect the effective policy or look up one selection from your project directory:

```bash
bbs team model --json
bbs team model --agent codex --complexity normal --phase-class critical --json
```

These lookups need no ticket or Orca connection. Add `--dir <repo-or-worktree>` to inspect another project's policy. Override individual fields under `foreman.models` in `~/.babysit/settings.json` or `<repo>/.babysit/settings.json`; repository settings take precedence over global settings, then built-in defaults. For example, route normal phases of hard tasks to the `max` tier:

```json
{
  "foreman": {
    "models": {
      "routing": {
        "hard": { "normal": "max" }
      }
    }
  }
}
```

See [model routing](../.claude/skills/team/references/model-routing.md#model-tiers) for per-agent model/effort bindings and resume behavior.

## Run a project

Invoke the **team skill** in your agent (these are skill invocations, not `bbs` CLI commands):

```text
# Claude Code
/bbs:team "Rebuild the request flow across web and API"
/bbs:team --auto "Rebuild the request flow across web and API"

# Codex
$bbs:team "Rebuild the request flow across web and API"
```

`--auto` delegates the design approval to the lead for runs with nobody at the keyboard; it never gives the release go, and cannot approve a revised design that moves work from your release go into an earlier automatic one.

The team works in three layers: the **project** (the spec and its basic design), **phases** (release increments) and **tickets** (one worker session each).

**Design.** One design session decides architecture, interface contracts and UI for every ticket, and iterates the whole-product prototype until it holds; it also names the structurally different shapes it weighed for the one-way decisions and why each lost, so you review a choice, not just a result. Each ticket gets a brief — what to build, files it owns, the prototype screens it implements, acceptance IDs, what it must not change — so tickets build instead of redesigning. Phase 1 is a walking skeleton of the first journey; every phase is safe to release on its own (dark launch, expand/contract migrations). A phase is one ticket by default; it splits only where the contexts are disjoint, each side is at least `M`, and the plan names the wall-time it saves. The last phase is `ask` — its release go is the handover; earlier phases are `none` (shipped with the handover) unless you mark them `auto`.

**Each phase.** The lead opens a phase once the previous one has landed and launches its tickets as one batch. **One ticket is one worker session:** a single autopilot Dispatch plans (`M`/`L` only — `XS`/`S` build straight from the brief), implements, reviews and QAs on one model, so the context built while planning carries through to review and repair. `M` plans are self-approved by the worker; the lead reviews `L` plans. Workers never commit: each passing ticket's work is sealed on a hidden ref. `bbs team integrate` composes the phase into **one commit** in its own worktree, and one integration session reviews the phase diff, runs **cumulative QA** — every acceptance criterion of this phase and all earlier ones — plus a product review when the contract asks, and fixes small findings itself. `bbs team land` then fast-forwards local base to the tested commit. An `auto` phase releases right after with `release-prd`; a rolled-back release stops the line until it is fixed forward. `bbs team readiness --phase <id>` enforces the order and `bbs team next <parent>` reads it back as suggested steps; it also turns your handover feedback into the fix round.

**Contracts.** Your confirmed spec and approved design are the SPEC between you and the team; the approval pins their exact revision. Where tickets — or two teams on one product — build against each other at the same time, the design names the seam as an **interface contract**: repo files listed in `.babysit/contracts.yaml`, versioned by content. See [contracts](../.claude/skills/references/contracts.md).

**Waking.** Each batch is handed to `bbs team batch`, and the lead ends its turn. A detached watcher — kept alive by launchd or systemd while a batch is open — supervises the workers and wakes the lead once when the batch settles, early only for a worker question, an exited worker or one still stalled after its nudges. Token measurements and how to re-check them are in [team token cost](foreman-cache-economics.md).

See [operations](operations.md#watching-a-team) for monitoring and recovery.
