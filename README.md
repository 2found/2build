# babysit

English | [Tiếng Việt](README.vi.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

**Give it a goal. It plans, builds, reviews, and verifies while you are away.**

Babysit is an agent skill pack plus a companion Go CLI. Use Autopilot for one serial ticket; use Foreman when a project needs dependent tickets and supervised workers. Babysit skills follow five product-building archetypes—Prototyper, Builder, Sweeper, Grower, Maintainer—chosen by the work, not the file type. See [the archetypes](.claude/skills/references/archetypes.md).

## Install

Install the CLI and the agent plugin separately:

```bash
brew install lohi-ai/babysit/bbs

# Claude Code
claude plugin marketplace add lohi-ai/babysit
claude plugin install bbs@babysit

# Codex CLI
codex plugin marketplace add lohi-ai/babysit
codex plugin add bbs@babysit
```

Restart your agent after installing. The plugin provides skills; `bbs` is a separate required CLI used by skills and local hooks. It is not bundled in the plugin. See [installation details](docs/install.md), including Linux packages.

To work on Babysit itself, clone the repository and run `go run ./cmd/bbs setup --full`; it builds `bbs` and links the checkout's skills. `bbs update` refreshes the CLI and installed plugins.

## Configure Foreman

Foreman requires [Orca](https://www.onorca.dev) with orchestration enabled. Agents and defaults are configured in Orca. Former Babysit worker/foreman configuration keys are retired and ignored; new launches use explicit per-dispatch agent/model/effort choices or Orca's configured defaults.

Foreman routes workers automatically from task complexity and phase: planning, design, and review use the critical route; implementation, QA, and delivery use the normal route. Explicit per-dispatch agent/model/effort choices take precedence; otherwise Foreman uses the configured Orca defaults.

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

Skills are the agent-facing workflows; `bbs` is the companion CLI they use. `/bbs:foreman` runs the project coordinator; `bbs foreman` manages durable Foreman records, contracts, and reports. `/bbs:autopilot` runs the one-ticket workflow; `bbs autopilot` exposes its checkpoint and state helpers. `bbs ticket` owns ticket identity, DAG relations, evidence, test surfaces, delivery, and cleanup.

Useful CLI entry points:

```bash
bbs dashboard
bbs foreman report <parent-ticket>
bbs ticket dag <parent-ticket>
bbs autopilot explain
```

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
