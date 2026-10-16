# Foreman: multi-ticket projects

Start with [Autopilot](../README.md#try-your-first-ticket) for one ticket. Foreman adds supervised workers and dependency-aware delivery for larger projects.

## Configure Foreman

Foreman requires [Orca](https://www.onorca.dev) with orchestration enabled. It selects a worker's agent from an explicit choice, a compatible phase pin, or Orca's configured default on the destination host. Legacy 2build YAML agent/provider/model/effort preferences are retired and ignored.

2build selects the worker's model and effort from task complexity (`simple`, `normal`, or `hard`) and phase class. Planning, design, and review are `critical` phases; implementation, QA, and delivery are `normal` phases. The policy maps these combinations to `lite`, `flash`, `pro`, or `max` tiers, each with model bindings for the selected agent; for Claude these are `haiku`, `sonnet`, `opus`, and `opus` at `max` effort. Simple tasks in normal phases get `lite`. Explicit phase overrides and valid persisted resume routes take precedence.

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

Foreman initializes or resumes a parent project and binds its Orca Run, then works in three layers: the **project** (the goal and its basic design), **phases** (production release increments) and **tickets** (one worker session each).

**Requirement and basic design.** Foreman clarifies the requirement itself — outcome, audience, scope, constraints, assumptions — and asks you only the questions that would change the architecture or the release plan, in one batch at kickoff (none under `--auto`). A basic-design worker then produces the architecture, one prototype of the whole product (or a non-UI workflow/interface design), the plan, a ticket manifest, and a **phase plan** in `project.json`: phase 1 is a walking skeleton of the first journey, every phase is safe to release on its own (dark launch, expand/contract migrations), each holds 2–5 tickets sized `M`–`L`, and each names its release mode — `ask` (you give the go), `auto` (the approved design is the go) or `none` (ships with the next phase). You review that once, before any phase starts. Explicit `--auto` delegates this review to Foreman, which checks the evidence and records the approval; it never grants a release go and cannot approve a revised contract that lowers an approved release mode.

**Each phase.** Foreman opens a phase only when the previous phase has landed, creates a phase ticket and its tickets, and launches them in parallel as one batch. **One ticket is one worker session:** a single autopilot Dispatch plans the ticket, gets the plan approved, waits for in-phase prerequisites, adopts their sealed work, implements, reviews and runs QA on one model without handing off between steps, so the prompt cache and the context built while planning carry through to review and repair. Plan approval follows ticket size: `XS` needs none, `S` and `M` plans are self-approved by the worker as Foreman's delegate, and an `L` plan is reviewed and approved by Foreman while the worker waits in the same session. Follow-ups for a ticket — a repair, a prerequisite refresh — return to the same session while it lives. Routing picks one model per ticket from task complexity (`bbs foreman model --phase-class critical`), never a different model per loop step.

Workers never commit: each passing ticket's uncommitted work is sealed on a hidden ref. When every ticket is sealed, `bbs foreman integrate` composes them into **one commit** for the phase, checked out in its own worktree, and one integration worker runs **cumulative QA** on exactly that commit: every acceptance criterion of this phase and all earlier ones, cross-ticket journeys and regressions, plus a product review when the contract asks. Failures go back to the owning ticket sessions as repairs. On PASS, `bbs foreman land` fast-forwards local base to the tested commit — base history gains one commit per phase. Then the phase is released with `release-prd` (including hypercare): automatically for `auto`, after your go for `ask` (`bbs foreman release --request` publishes it; nothing but a human can resolve it). While a phase waits on your go, the next phase already builds, but it cannot integrate until the earlier one is released. A rolled-back or partial release stops the line until it is fixed forward; a second failure needs your go even for an `auto` phase. `bbs foreman readiness --phase <id> --action dispatch|integrate|land|release` enforces this order.

Each batch is handed to `bbs foreman batch`, and Foreman ends its turn. The detached watcher — kept alive by launchd or systemd while a batch is open — supervises the workers, compacts the Foreman session at the batch boundary, and wakes it once when the batch settles. Only a question, a worker that exited without reporting, or one still stalled after its nudges wakes it early. Foreman keeps only the goal, the accepted design, the phase plan, the DAG and durable ids in context. A decision only a human can make — a money, auth or irreversible-data plan, a hold, or a release go — parks that one ticket or phase while independent work continues. Simulator and local-ML checks are deferred out of each ticket and run as one consolidated heavy QA per profile.

The last phase's cumulative QA is the project's final QA. Foreman completes the parent only after every phase is released (or delivered, for `none`), cleanup is done and `bbs foreman readiness --action finish` passes.

See [operations](operations.md#watching-a-foreman) for monitoring and recovery.
