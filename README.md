# 2build

English | [Tiếng Việt](README.vi.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

**Give your coding agent a goal. Come back to a reviewed, tested change.**

2build supports product engineering, from planning and implementation through review, testing, QA and release. Its open-source skill pack works with Claude Code, Codex and Antigravity, with a companion CLI that saves progress and verification evidence. Start with **Autopilot**: one ticket, from requirement to a local commit, in the agent session you already use.

A 2found product. Previously named babysit; the `bbs` CLI, `bbs:` skills and existing `.babysit` state remain compatible. See [branding and naming](BRANDING.md).

<a id="install"></a>

## Install with one prompt

Paste this into your coding agent with terminal access:

```text
Install 2build for the coding agent I am using. Follow
https://raw.githubusercontent.com/2found/2build/main/docs/install.md. Detect my OS and
current agent, reuse a working bbs installation or install the CLI, then install the
skill pack for this agent only. Verify bbs --version and the installed plugin; report
any missing prerequisite instead of claiming success. Tell me whether I need to restart
and give me the exact Autopilot invocation for my agent to run my first small task.
```

You need a supported coding agent and its existing model access. Claude Code and Codex also need their CLI on PATH. 2build has no separate model account to configure; your agent's normal usage charges apply. **Orca is only required for Foreman.** See [installation and troubleshooting](docs/install.md).

<details>
<summary>Prefer terminal commands? Homebrew on macOS or Linux</summary>

```bash
brew tap 2found/2build https://github.com/2found/2build
brew install 2found/2build/bbs
bbs install
```

`bbs install` configures all detected supported agents. Use `bbs install claude`, `bbs install codex`, or `bbs install antigravity` to select one. Restart that agent after installation. Without Homebrew, use a [release archive](docs/install.md#release-archives-macos-or-linux); on Windows, use WSL.

</details>

## Why 2build?

A prompt can describe how to build something. 2build adds the workflow and durable state needed to carry a goal through review and verification, even after a session restarts.

| What you need | What 2build adds |
|---------------|-------------------|
| Finish a task without directing every step | Autopilot carries one ticket through planning, implementation, review fixes and QA. |
| Pick up after a crash or context reset | Requirements, plans, checkpoints and handoffs live on disk. Resume from that evidence. |
| Know whether the result works | Review and QA verdicts are persisted; completion requires current checks and no unresolved material findings. |
| Keep control of delivery | Standalone Autopilot commits locally. You review the evidence before pushing or opening a PR. |

Use it when a task needs a verified handoff or you want to leave a run working while you are away. A quick edit may need only your coding agent. 2build still needs a usable project test environment; missing access or required checks are reported as `NEEDS_CONTEXT` or `BLOCKED`.

<a id="autopilot-one-ticket"></a>

## Try your first ticket

1. Restart your agent, open a Git repo and choose a small bug or feature with a clear success check. Autopilot works and commits on your current checkout; create your preferred branch first if you want isolation.
2. Invoke the **skill in agent chat**, replacing the example with your task:

   | Agent | Example |
   |-------|---------|
   | Claude Code | `/bbs:autopilot "Fix the empty search result state and add a regression test"` |
   | Codex | `$bbs:autopilot "Fix the empty search result state and add a regression test"` |
   | Antigravity | Ask it to use the installed `autopilot` skill for your task. |

3. When Autopilot returns a plan and a `/goal` block, review the plan, then paste that block into the same agent to start the build. On an agent without goal mode, it continues in the same session.
4. Expect a local commit, review and QA evidence, and a handoff naming what changed and what was checked. A blocked run names the gap. After a restart, give Autopilot the ticket ID from the handoff to resume.

No Orca, new worker session or project configuration is needed for this first ticket. You choose the session's model before starting. [Inspect progress and recover a session](docs/companion-cli.md).

## Pick the work, not the job title

Autopilot routes a task to one of five product-team archetypes. You can also name the workflow or invoke a skill directly.

| Archetype | Use it to |
|-----------|-----------|
| Prototyper | Validate a risky idea before investing in production code. |
| Builder | Deliver a feature or bug fix through review and QA. |
| Sweeper | Remove weight or improve a measured hot path while preserving behavior. |
| Grower | Improve copy, conversion or a measurable growth experiment. |
| Maintainer | Diagnose failures and harden reliability, security or dependencies. |

Browse the [skill index](docs/skills.md) for individual skills and workflow examples.

<a id="configure-foreman"></a>
<a id="foreman-multi-ticket-projects"></a>

## Larger projects: Foreman

For dependent tickets and supervised workers, [Foreman](docs/foreman.md) uses [Orca](https://www.onorca.dev) to plan a project, isolate child tickets in worktrees, dispatch ready work and run project-wide QA. It presents the parent plan/design for review before production dispatch; explicit `--auto` delegates that review. The configured finish policy controls delivery.

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

## Semantic decisions

`semantic-decision` makes task sizing, complexity, routing, test impact and review
judgments an explicit reusable skill step. Its default provider is the current
LLM; no configuration or extra model process is needed. To use Cloudflare:

```bash
bbs config set semantic_decision_provider cloudflare
bbs config set semantic_decision_model clef-flash  # or clef
```

Set `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_API_TOKEN` in the environment or
`~/.babysit/.env`. Only user configuration selects the provider; project policy
can restrict external inference. Cloudflare errors or uncertain answers fall
back to the current LLM. See the [skill](.claude/skills/semantic-decision/SKILL.md)
and [contract](.claude/skills/references/semantic-decision.md). See
[settings without a dashboard control](docs/operations.md#settings-without-a-dashboard-control)
for config paths, defaults, project limits and human-review behavior.

For agents, the [Markdown documentation index](llms.txt) links directly to installation, skill and runtime contracts.

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
