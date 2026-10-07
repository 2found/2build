# Foreman: multi-ticket projects

Start with [Autopilot](../README.md#try-your-first-ticket) for one ticket. Foreman adds supervised workers and dependency-aware delivery for larger projects.

## Configure Foreman

Foreman requires [Orca](https://www.onorca.dev) with orchestration enabled. It selects a worker's agent from an explicit choice, a compatible phase pin, or Orca's configured default on the destination host. Legacy Babysit YAML agent/provider/model/effort preferences are retired and ignored.

Babysit selects the worker's model and effort from task complexity (`simple`, `normal`, or `hard`) and phase class. Planning, design, and review are `critical` phases; implementation, QA, and delivery are `normal` phases. The policy maps these combinations to `flash`, `pro`, or `max` tiers, each with model bindings for the selected agent. Explicit phase overrides and valid persisted resume routes take precedence.

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

Foreman initializes or resumes a parent project and binds its Orca Run. A planning/design worker first creates one general plan, prototype (or a non-UI workflow/interface design), and stable ticket manifest covering scope and dependencies. By default, you review those artifacts and proposed tickets before child tickets, worktrees, or production dispatch. Explicit `--auto` delegates that review to a separate evidence-checking worker and records the approval; it does not bypass safety holds or later QA.

After approval, accepted seeds become a ticket DAG with explicit dependency edges. Foreman dispatches only ready tickets, within worker/resource limits, and keeps one writer per ticket worktree. Each ticket moves through separate bounded Plan, Implement, Review, and QA worker phases. Routing uses task complexity plus phase or explicit per-dispatch choices.

Foreman waits for worker reports, questions, or escalations through Orca; it does not poll terminals or start retry timers. It checks each phase's artifacts, revisions, and verdicts before advancing. Once a ticket passes its gates, its configured finish policy (`review`, `land`, or `pr`) controls delivery. Foreman verifies the receipt, releases settled workers and leases, closes owned Orca surfaces, and removes only eligible clean worktrees while retaining branches.

Project finish is worker-led too: audit and authorized delivery/cleanup workers settle first, then a read-only final QA worker checks the exact delivered base or retained QA composition. Foreman completes the parent only after final evidence, cleanup, and readiness pass. Interacting tickets also receive pre-land integration QA; it does not replace final project QA.

See [operations](operations.md#watching-a-foreman) for monitoring and recovery.
