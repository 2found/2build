# babysit

English | [Tiếng Việt](README.vi.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

**Give it a goal. It plans, builds, reviews, and verifies while you are away.**

Babysit is an agent skill pack plus a companion Go CLI. Use Autopilot for one serial ticket; use Foreman when a project needs dependent tickets and supervised workers. Babysit skills follow five product-building archetypes—Prototyper, Builder, Sweeper, Grower, Maintainer—chosen by the work, not the file type. See [the archetypes](.claude/skills/references/archetypes.md).

## Install

Install the CLI, then let Babysit detect and configure your harnesses:

```bash
brew install lohi-ai/babysit/bbs
bbs install
```

Restart your agent after installing. `bbs install` supports Claude Code, Codex and Antigravity; Claude Code and Codex need their CLI on PATH. To install for one harness only, use `bbs install claude`, `bbs install codex`, or `bbs install antigravity`.

All three OSes are covered: Homebrew on macOS and Linux, or the per-arch tarball from the latest release on Linux. Windows publishes no binary — run inside WSL or Git-Bash, or build `go run ./cmd/bbs setup` from a checkout. See [installation details](docs/install.md) for the full platform matrix.

To work on Babysit itself, clone the repository and run `go run ./cmd/bbs setup --full`; it builds `bbs` and prints local plugin registration commands. `bbs update` refreshes the CLI and installed plugins.

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

See [model routing](.claude/skills/foreman/references/model-routing.md#model-tiers) for per-agent model/effort bindings and resume behavior.

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

## Autopilot: one ticket

Invoke the **Autopilot skill** for a single ticket:

```text
# Claude Code skill, in the session you opened
/bbs:autopilot "Add a dark-mode toggle"
```

Standalone Autopilot plans, implements, reviews, and QAs in the session you started. It does not choose a new model or switch models mid-run; choose the model before starting. In Codex, invoke the skill as `$bbs:autopilot`. It works without Orca and never opens a background worker session.

## Skills and CLI

Skills are the agent-facing workflows; `bbs` is the companion CLI they use. `/bbs:foreman` runs the project coordinator; `bbs foreman` manages model policy lookups, durable Foreman records, contracts, and reports. `/bbs:autopilot` runs the one-ticket workflow; `bbs autopilot` exposes its checkpoint and state helpers. `bbs ticket` owns ticket identity, DAG relations, evidence, test surfaces, delivery, and cleanup.

Useful CLI entry points:

```bash
bbs dashboard
bbs foreman report <parent-ticket>
bbs ticket dag <parent-ticket>
bbs autopilot snapshot --json
bbs autopilot recover --json
```

`snapshot` reads canonical ticket state and gate evidence; `recover` adds bounded artifact excerpts for resuming work.

Run `bbs <subcommand> --help` for usage. More detail: [companion CLI](docs/companion-cli.md), [profiles](docs/profiles.md), and [operations](docs/operations.md).

## Repository layout

- `bbs` — the multicall binary (gitignored build output; `go build -o bbs ./cmd/bbs`). Hooks are compiled subcommands, `bbs hooks <name>`.
- `.claude/skills/` — agent skills and workflow references.
- `internal/` — Go CLI and services.
- `web/` — dashboard SPA, embedded in release builds.
- `tests/`, `docs/` — verification suites and user documentation.

## Telemetry

Skill usage is recorded locally as JSONL under `~/.babysit/analytics/`; telemetry is the primary feedback channel for unattended runs.

## License

MIT.
