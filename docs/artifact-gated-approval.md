# 2build hooks

2build keeps three runtime hooks: a release check, session tracking, and an
Orca worker report check before stopping.
The repository's Git pre-commit hook remains separate.

| Hook | When | Purpose |
| --- | --- | --- |
| `pre-tool-gate` | Before shell tools | Check ticket review/QA artifacts before push, PR creation, or PR merge |
| `session-writer` | Session start and after shell tools | Refresh dashboard session identity, throttled to once per minute |
| `worker-report-gate` | `Stop` / OMP `session_stop` | Keep a worker running while its current Orca Dispatch is still active |

## Installation and agent contracts

Install the companion `bbs` CLI — the hooks are compiled into it as
`bbs hooks pre-tool-gate`, `bbs hooks session-writer`, and
`bbs hooks worker-report-gate`, so neither bash nor
jq is required on any OS. Plugins ship no compiled `bbs`; use
`go run ./cmd/bbs setup` from a checkout, or the documented Homebrew install.
The manifest commands invoke `bbs` by name, so it must be on PATH (`bbs setup`
links it into `~/.local/bin`).

| Agent | Wiring | Payload / decision |
| --- | --- | --- |
| Claude Code | Plugin auto-discovers `hooks/hooks.json` | snake_case input; native deny JSON |
| Codex | Plugin auto-discovers `hooks/hooks.json` | snake_case input; native deny JSON |
| Grok Build | Plugin loads `hooks/hooks.json` | camelCase input; native deny JSON |
| OMP | Plugin loads `hooks/omp.ts` via `package.json` `omp.extensions` | `tool_call` / `tool_result` / `session_start` / `session_stop`; native block result |

The command manifest calls `bbs hooks <name>` directly — no shell syntax, so
the same manifest works under POSIX shells, PowerShell, and cmd. Hooks are
compiled subcommands; there are no per-hook scripts, so older callers that
invoked a `bin/hooks/<name>` path must switch to `bbs hooks <name>`.

For OMP, `bbs install omp` (or `omp plugin install bbs@babysit`) loads both
skills and the extension. Restart the agent after installing or upgrading;
`/reload-plugins` refreshes skills but not extension modules. Avoid loading
`hooks/omp.ts` a second time via `--extension` or `~/.omp/agent/extensions`.

## Release behavior

Only recognized push / PR-create / PR-merge shell commands pay the cost of
ticket resolution. Other commands return silently. This is a workflow check,
not a shell security sandbox: aliases, scripts, dynamically constructed
commands, and tools outside the host's hook coverage can bypass classification.
Run releases through 2build's workflows; `bbs foreman land` independently
checks phase QA evidence.

- No ticket: no objection.
- Ticket identity conflict, unavailable companion binary or a readiness error
  (a checkpoint from before `schema_version: 2` is refused with
  `CHECKPOINT_V1` naming `bbs autopilot checkpoint --refresh`): deny with the
  reason.
- Push needs current `review-pr` evidence; PR creation and merge need
  `review-pr` and `qa`. The gate is `bbs ticket readiness`, which reads only
  evidence `bbs autopilot verification record` accepted for the current tree;
  a missing, failed or stale gate denies naming it. Verdict files are never
  read.
- No objection means **exit 0 with no output**. Never emit `allow` (which
  could override the host's own permission checks) or `defer` (which can
  suspend Claude Code's headless execution).

Every host receives a denial/block with the missing gate's reason, so an
unattended agent can run the gate and retry. No custom prompt or automatic approval is
introduced. OMP also blocks process failures, timeouts, and malformed
decision responses. Host-native timeout/error handling otherwise applies;
this is not a universal fail-closed boundary.

The gate uses the payload's working directory (`tool_input.workdir` when
provided, otherwise `cwd`). Shell-internal directory changes and `git -C`
aren't parsed; invoke release tools from the target repository.

## Session tracking

Both snake_case and Grok's camelCase session IDs are supported. Files use
`cc-`, `cx-`, `grok-`, or `omp-` prefixes under
`${BABYSIT_HOME:-$HOME/.babysit}/sessions`. Codex is identified by its
session/thread environment or turn payload; OMP supplies its identity explicitly.
Session IDs containing path separators are rejected. Tracking is advisory;
an unwritable state directory never blocks tool execution.

## Worker report before stop

Claude Code and Codex use the manifest's `Stop` hook; OMP uses its awaited
`session_stop` extension event. The hook reads `ORCA_TERMINAL_HANDLE` and
looks it up in the bbs task store (`$BABYSIT_HOME/tasks`): it needs no Orca
call. It never derives worker authority from a ticket, parses the transcript,
or reports on the worker's behalf.

An open Foreman Dispatch launched into this exact terminal blocks stopping
with the `bbs foreman task done <task> --dispatch <id> --outcome …` command to
run after persisting the handoff; a pending question blocks with its
`task ask --resume <id>` command instead. The settled Dispatch then allows
stopping. The worker does not wait for Foreman to reply. Foreman still verifies
the persisted evidence. An earlier Dispatch's report cannot satisfy a new
assignment, and a Dispatch Foreman abandoned no longer blocks: that worker no
longer owns the assignment.

Sessions without an Orca handle, and terminals no open Dispatch was launched
into (ordinary terminals, coordinators), pass. `stop_hook_active` does not
bypass the check; every continuation reads the store again.

This requires an installed/trusted hook and `bbs` on the worker's PATH.
Host stop-continuation limits, user interrupts, process crashes, and forced
termination remain outside the guard. Keep Foreman's reconciliation backup.
Other agents need an equivalent blocking stop event before claiming coverage.

## Removed audits

- `verify-skill-output`: the Skill tool loads instructions before the model
  writes its verdict; inspecting its output does not validate the final verdict.
- `clean-handoff-check`: a turn ending with working-tree changes is normal for
  directly invoked skills, so the warning incorrectly encouraged commits/stashes.
- `qa-evidence-audit`: duplicated the evidence check at the release boundary.

Their scripts and registrations were removed. Existing telemetry rows remain
available for historical analysis; skill telemetry and persisted verdicts remain.
The gate has one registration instead of five host-specific `if` filters.

## Verification

```sh
go test ./internal/cmd -run 'TestGate|TestRunPreToolGate|TestSessionWriter|TestClassifyGateStage'
go test ./internal/cmd ./internal/foreman -run 'TestWorkerReportGate|TestTaskDispatchLifecycle'
bash tests/test_autopilot_v2_readiness.sh
bash tests/test_hook_session_writer.sh
python3 tests/test_hooks_portability.py
bun test tests/test_hooks_omp.test.ts
```

The Go tests cover the gate's deny/pass decisions over readiness and the
session writer; the portability suite executes the
shipped command manifest against real ticket state; the OMP tests exercise
the adapter contract. These tests make no model requests and never execute
the proposed release command.

Contracts checked against [Claude Code hooks](https://code.claude.com/docs/en/hooks),
[Codex hooks](https://developers.openai.com/codex/hooks),
[Grok Build hooks](https://docs.x.ai/build/features/hooks), and
[OMP extensions](https://github.com/can1357/oh-my-pi/blob/main/docs/extensions.md).
