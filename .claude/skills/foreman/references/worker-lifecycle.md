# Foreman worker transport

Foreman owns this adapter. Include these instructions in every worker Task
spec alongside the live injected runtime contract; member skills do not load
this reference. The outer worker applies the lifecycle once around its assigned
skill, including on resume or failure.

Declare `SPAWNED=true` and `AGENT_ROLE=orca` in the assignment. An authenticated
current Dispatch overrides a stale developer/unspawned shell echo. Supply the
blocking escalation channel and phase/stop boundary before invoking the skill.

## Worker lifecycle
Read `~/.claude/skills/orchestration/SKILL.md` and its version-matched guide.
The live injected Orca preamble is authoritative for executable, handle,
capability, Task ID and Dispatch ID; never reconstruct authority from env/tickets.

- Use injected `orca orchestration ask`; no `AskUserQuestion` in a worker.
  Timeout leaves the question pending: `--resume <message_id>`, not a new ask.
  Bus/coordinator failure returns the shared preamble's structured NEEDS_CONTEXT.
- Follow the lifecycle for the whole Dispatch, not each nested skill. Read
  follow-ups with `orca orchestration check` at checkpoints and before
  completion; follow the injected heartbeat cadence.
- Persist handoff/verdicts, then send exactly one `worker_done` via injected
  `orca orchestration send`, with both lifecycle IDs and a three-sentence
  did/found/left summary. Use `--outcome succeeded` for DONE/DONE_WITH_CONCERNS,
  `--outcome failed` when ending as BLOCKED/NEEDS_CONTEXT. A pending ask is
  not a terminal report. `--files-modified` / `--report-path` name real artifacts.
- After delivery, end the dispatched turn and idle. Ordinary sessions without
  an injected Dispatch emit no lifecycle messages. The coordinator reads
  `bbs ticket verdict-status`; the message does not substitute for a verdict.
  Send errors/rejections are reporting failures: retain artifacts and use
  Orca's recovery contract, never guess a replacement Dispatch.

The `worker-report-gate` Stop hook checks the current assignment. A block
requires resolving the lifecycle failure; the hook neither reports for the
worker nor waits for a Foreman reply. A continuation is not proof of delivery.
