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

Foreman initializes or resumes a parent project and binds its Orca Run. A planning/design worker first creates one general plan, prototype (or a non-UI workflow/interface design), and stable ticket manifest covering scope and dependencies. By default, you review those artifacts and proposed tickets before child tickets, worktrees, or production dispatch. Explicit `--auto` delegates that review to Foreman itself, which checks the evidence and records the approval; it does not bypass safety holds or later QA. This general plan is the one point where a human is expected.

After approval, the project is split into **phases** — vertical slices up to `L` size, each its own child ticket, branch and worktree — joined by explicit dependency edges. Phases are deliberately few and large: every boundary costs a cold session and a full review/QA cycle. **One phase is one worker session:** a single autopilot Dispatch plans the phase, gets the plan approved, waits for its prerequisite phases, merges them, implements, reviews and runs QA on one model without handing off between steps, so the prompt cache and the context built while planning carry through to review and repair. Plan approval follows phase size: `XS` needs none, `S` and `M` plans are self-approved by the worker as Foreman's delegate, and an `L` plan is reviewed and approved by Foreman while the worker waits in the same session. Later follow-ups for a phase — a repair, a prerequisite refresh, PR delivery — return to the same session while it lives; a fresh worker resumes from disk only when it is gone. Routing picks one model per phase from task complexity (`bbs foreman model --phase-class critical`), never a different model per loop step.

Foreman launches the whole phase DAG as one batch of up to `min(16, max(2, CPUs - 2))` workers (`parallel_max_workers`): dependent phases start with their prerequisites, plan while those build, and wait inside their own session with `bbs foreman await` until each prerequisite's Dispatch settles and passes readiness. It then hands the batch to `bbs foreman batch` and ends its turn. The detached watcher supervises the workers — nudging one that sits idle with its Dispatch open — compacts the Foreman session at the batch boundary, and wakes it once, by Orca message and pane prompt, when the whole batch settles. Only a question (an `L` plan review or a worker's mid-run decision), a worker that exited without reporting, or one still stalled after its nudges wakes it early. Foreman keeps only the goal, the accepted plan, the DAG and durable ids in context; it does not wait, poll terminals, read worker transcripts or start retry timers. A decision only a human can make — a money, auth or irreversible-data plan, or a hold — parks that one phase with its session retained while the rest of the DAG continues. Simulator and local-ML checks are deferred out of each phase and run as one consolidated heavy QA per profile once light work has drained, so heavy work never displaces parallel phases. Foreman checks each phase's plan approval, revisions, and review/QA verdicts before advancing. Once a phase passes its gates, its configured finish policy (`review`, `land`, or `pr`) controls delivery. Foreman verifies the receipt, releases settled workers and leases, closes owned Orca surfaces, and removes only eligible clean worktrees while retaining branches.

Project finish is worker-led too: audit and authorized delivery/cleanup workers settle first, then a read-only final QA worker checks the exact delivered base or retained QA composition. Foreman completes the parent only after final evidence, cleanup, and readiness pass. Interacting tickets also receive pre-land integration QA; it does not replace final project QA.

See [operations](operations.md#watching-a-foreman) for monitoring and recovery.
