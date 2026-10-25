# Foreman: multi-ticket projects

Start with [Autopilot](../README.md#try-your-first-ticket) for one ticket. Foreman adds supervised workers and dependency-aware delivery for larger projects.

Foreman is a toolkit and advisor for the coding agent that runs it, not a script it must follow: the `bbs foreman` commands keep the state and the few invariants that must hold (one commit per phase, a human go for `ask` releases, phase order, contract ownership), and the skill plus `bbs foreman next` suggest the path. Two rules are kept hard because they decide the token bill: the coordinator wakes once per settled batch, and each ticket runs start to finish in one worker session.

## Configure Foreman

Foreman requires [Orca](https://www.onorca.dev) for worktrees and worker terminals; the Task/Dispatch lifecycle (launch, questions, reports) is bbs's own (`bbs foreman task`), so no Orca orchestration, mailbox or heartbeat is involved. It selects a worker's agent from `--plan-worker` (basic design), `--implement-worker` (ticket sessions), and `--qa-worker` (phase QA); omitted means this Foreman's own harness. Legacy 2build YAML agent/provider/model/effort preferences are retired and ignored.

2build selects the worker's model and effort from task complexity (`simple`, `normal`, or `hard`) and phase class: `design` for basic design (architecture, prototype, phase plan), `critical` for ticket workers and diagnosis, `normal` for integration, QA, release and cleanup. The policy maps these combinations to `lite`, `flash`, `pro`, or `max` tiers, each with model bindings for the selected agent; for Claude these are `haiku`, `sonnet`, `opus` at `medium` effort, and `opus` at `high` effort; for Codex `gpt-6-luna`, `gpt-6.1-sol`, `gpt-6.1-sol`, `gpt-6-astra`; for OMP the `@smol`, `@slow` and `@plan` roles, with the `normal` role
expressed by omp's own configured default (omp has no `@normal` alias).
Ticket workers implement on `flash`; `max` is reserved for basic design and `hard` tickets, and a `hard` route must name the invariant its own diff changes (`--hard-reason money|auth|security|live-data|concurrency`). Pin the harness with `--plan-worker`, `--implement-worker`, and `--qa-worker` at spawn or adopt; usage does not switch the agent. Explicit phase overrides and valid persisted resume routes take precedence.

Inspect the effective policy or look up one selection from your project directory:

```bash
bbs foreman model --json
bbs foreman model --agent codex --complexity normal --phase-class critical --json
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

See [model routing](../.claude/skills/foreman/references/model-routing.md#model-tiers) for per-agent model/effort bindings and resume behavior.

## Foreman: multi-ticket projects

Invoke the **Foreman skill** in your agent (the examples are skill invocations, not `bbs` CLI commands):

```text
# Claude Code
/bbs:foreman "Rebuild the request flow across web and API"
/bbs:foreman --auto "Rebuild the request flow across web and API"

# Codex
$bbs:foreman "Rebuild the request flow across web and API"
```

Foreman initializes or resumes a parent project, then works in three layers: the **project** (the goal and its basic design), **phases** (production release increments) and **tickets** (one worker session each).

**Requirement and basic design.** Foreman clarifies the requirement itself — outcome, audience, scope, constraints, assumptions — and asks you only the questions that would change the architecture or the release plan, in one batch at kickoff (none under `--auto`). A basic-design worker then produces the architecture, one prototype of the whole product (or a non-UI workflow/interface design), the plan, a ticket manifest, and a **phase plan** in `project.json`: phase 1 is a walking skeleton of the first journey, every phase is safe to release on its own (dark launch, expand/contract migrations), each holds 2–5 tickets sized `M`–`L`, and each names its release mode — `ask` (you give the go), `auto` (the approved design is the go) or `none` (ships with the next phase). You review that once, before any phase starts. Explicit `--auto` delegates this review to Foreman, which checks the evidence and records the approval; it never grants a release go and cannot approve a revised contract that lowers an approved release mode.

**Each phase.** Foreman opens a phase only when the previous phase has landed, creates a phase ticket and its tickets, and launches them in parallel as one batch. **One ticket is one worker session:** a single autopilot Dispatch plans the ticket, gets the plan approved, waits for in-phase prerequisites, adopts their sealed work, implements, reviews and runs QA on one model without handing off between steps, so the prompt cache and the context built while planning carry through to review and repair. Plan approval follows ticket size: `XS` needs none, `S` and `M` plans are self-approved by the worker as Foreman's delegate, and an `L` plan is reviewed and approved by Foreman while the worker waits in the same session. Follow-ups for a ticket — a repair, a prerequisite refresh — return to the same session while it lives. Routing picks one model per ticket from task complexity (`bbs foreman model --phase-class critical`), never a different model per loop step. Tickets are split by context rather than size, because every extra session reloads its context cold; the measured costs and how to re-check them are in [Foreman token cost](foreman-cache-economics.md).

Workers never commit: each passing ticket's uncommitted work is sealed on a hidden ref. When every ticket is sealed, `bbs foreman integrate` composes them into **one commit** for the phase, checked out in its own worktree, and one integration worker runs **cumulative QA** on exactly that commit: every acceptance criterion of this phase and all earlier ones, cross-ticket journeys and regressions, plus a product review when the contract asks. Failures go back to the owning ticket sessions as repairs. On PASS, `bbs foreman land` fast-forwards local base to the tested commit — base history gains one commit per phase. Then the phase is released with `release-prd` (including hypercare): automatically for `auto`, after your go for `ask` (`bbs foreman release --request` publishes it; nothing but a human can resolve it). While a phase waits on your go, the next phase already builds, but it cannot integrate until the earlier one is released. A rolled-back or partial release stops the line until it is fixed forward; a second failure needs your go even for an `auto` phase. `bbs foreman readiness --phase <id> --action dispatch|integrate|land|release` enforces this order, and `bbs foreman next <parent>` reads it back as suggested next steps — advice for Foreman, not a script it must follow.

**Contracts.** Two agreements keep parallel work from breaking itself. The basic design you approve is the SPEC between you and Foreman; its approval pins the exact revision. Where tickets — or two Foremen on one product — build against each other at the same time, the design names the seam as an **interface contract**: repo files (an API doc, a schema, a types file) listed in `.babysit/contracts.yaml`, versioned by their content. Tickets pin the version they build against and are sent back when it changes; a phase that changes a contract runs its consumers' checks in phase QA; and a phase may not change a contract another Foreman owns — it asks that Foreman with a ticket assigned to it. See [contracts](../.claude/skills/references/contracts.md).

Each batch is handed to `bbs foreman batch`, and Foreman ends its turn. The detached watcher — kept alive by launchd or systemd while a batch is open — supervises the workers and wakes it once when the batch settles. Only a question, a worker that exited without reporting, or one still stalled after its nudges wakes it early — and attention arriving within two minutes shares one wake. The wake reads one command, `bbs foreman batch show`, which ends with each project's suggested next steps. Foreman keeps only the goal, the accepted design, the phase plan, the DAG and durable ids in context. A decision only a human can make — a money, auth or irreversible-data plan, a hold, or a release go — parks that one ticket or phase while independent work continues. Simulator and local-ML checks are deferred out of each ticket and run as one consolidated heavy QA per profile.

The last phase's cumulative QA is the project's final QA. Foreman completes the parent only after every phase is released (or delivered, for `none`), cleanup is done and `bbs foreman readiness --action finish` passes.

See [operations](operations.md#watching-a-foreman) for monitoring and recovery.
