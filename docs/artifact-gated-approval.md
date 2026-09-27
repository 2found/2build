# Babysit hooks

Babysit keeps three runtime hooks: a release check, session tracking, and an
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
`bin/setup-skills` (POSIX) or `bin/setup-skills.ps1` (PowerShell) from a
checkout, or the documented Homebrew install. The manifest commands invoke
`bbs` by name, so it must be on PATH (setup-skills links it into
`~/.local/bin`).

| Agent | Wiring | Payload / decision |
| --- | --- | --- |
| Claude Code | Plugin auto-discovers `hooks/hooks.json` | snake_case input; native deny/ask JSON |
| Codex | Plugin auto-discovers `hooks/hooks.json` | snake_case input; native deny/ask JSON |
| Grok Build | Plugin loads `hooks/hooks.json` | camelCase input; native deny JSON |
| OMP | Load `hooks/omp.ts` as an extension | `tool_call` / `tool_result` / `session_start` / `session_stop`; native block result |

The command manifest calls `bbs hooks <name>` directly — no shell syntax, so
the same manifest works under POSIX shells, PowerShell, and cmd. The
`bin/hooks/*` scripts remain as thin `exec bbs hooks <name>` shims for
installations that still invoke them by path.

For OMP, skills configuration alone does **not** activate these hooks:

```sh
omp --extension "/absolute/path/to/babysit/hooks/omp.ts"
```

For persistent discovery, put a symlink to that file in
`~/.omp/agent/extensions/babysit.ts`. The adapter resolves its real file
location, so the symlink doesn't break script lookup. Avoid loading it twice.
Restart the agent after updating its installed plugin/extension; editing this
checkout does not update an existing marketplace cache.

## Release behavior

Only recognized push / PR-create / PR-merge shell commands pay the cost of
ticket resolution. Other commands return silently. This is a workflow check,
not a shell security sandbox: aliases, scripts, dynamically constructed
commands, and tools outside the host's hook coverage can bypass classification.
Run releases through Babysit's workflows; `bbs ticket land` independently
checks its persisted verdicts.

- No ticket: no objection.
- Ticket identity conflict or unavailable companion binary: deny with a reason.
- Push: a blocked review denies; a missing review requests the review.
- PR creation / merge: check both review and QA, including the QA evidence body.
  Contradictory evidence denies; missing or thin evidence requests the check.
- No objection means **exit 0 with no output**. Never emit `allow` (which
  could override the host's own permission checks) or `defer` (which can
  suspend Claude Code's headless execution).

Claude Code and Codex can present their native `ask` decision. Grok and OMP
return a denial/block with the missing check's reason, so an unattended agent
can perform the check and retry. No custom prompt or automatic approval is
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
`session_stop` extension event. The hook reads `ORCA_TERMINAL_HANDLE`, then
runs `orca orchestration check --terminal <that handle> --peek --json` using
the resolved Orca executable. It never derives worker authority from a ticket,
parses the transcript, consumes messages, acknowledges a Delivery, or sends a
report on the worker's behalf.

An active `dispatchId` blocks stopping with instructions to persist the handoff
and use the injected `worker_done` command with the actual success/failure
outcome. Orca settles the Dispatch on an accepted terminal report; the next
check then allows stopping. The worker does not wait for Foreman to reply.
Foreman still verifies the persisted evidence. An earlier Dispatch's report
cannot satisfy a new assignment, and a rejected report leaves the gate closed.
Explicit `consumer_fenced` or `dispatch_inactive` responses allow stopping:
that worker no longer owns the assignment and must not send another report.

Sessions without an Orca handle skip the check; a valid response with no active
Dispatch also passes (including ordinary terminals and coordinators). Inside
Orca, an unavailable runtime or malformed response blocks because ownership
cannot be verified. The probe has a four-second deadline within the ten-second
hook timeout and never launches the app. `stop_hook_active` does not bypass
verification; every continuation checks the live binding again.

This requires a current Orca runtime that returns the worker `dispatchId` in
`check --peek`, an installed/trusted hook, and `bbs` on the worker's PATH.
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
go test ./internal/cmd ./internal/orca -run 'TestWorkerReportGate|TestPendingWorkerDispatch'
bash tests/test_autopilot_v2_readiness.sh
bash tests/test_hook_session_writer.sh
python3 tests/test_hooks_portability.py
bun test tests/test_hooks_omp.test.ts
```

The Go tests cover the gate's deny/ask/pass matrix (including the enforced v2
readiness path) and the session writer; the portability suite executes the
shipped command manifest against real ticket state; the OMP tests exercise
the adapter contract. These tests make no model requests and never execute
the proposed release command.

Contracts checked against [Claude Code hooks](https://code.claude.com/docs/en/hooks),
[Codex hooks](https://developers.openai.com/codex/hooks),
[Grok Build hooks](https://docs.x.ai/build/features/hooks), and
[OMP extensions](https://github.com/can1357/oh-my-pi/blob/main/docs/extensions.md).
