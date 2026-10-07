# Skill Preamble
Run the bootstrap first and telemetry last. Decisions follow the
[Auto-Decision Framework](auto-decision-framework.md); deliverables follow
[Handoff Contracts](handoff-contracts.md).

## Agent-specific skill references
Use the printed `SKILL_REF` for all babysit invocations, including examples:
Codex `$bbs:<skill>`, OMP/Cursor `/<skill>`, Claude Code/grok `/bbs:<skill>`.

## Resolving shared references
Resolve `../shared/<file>.md` from the skill's filesystem directory.
Do not use `skill://` for sibling references: some harnesses strip `..`.

## Output style — terse by default
Keep machine output terse, human output concise, and downstream artifacts
complete: preserve reasons, constraints and gotchas. Use full explanations
for security, destructive actions or ambiguity. Skill-specific formats win.

<a id="escalation-channels"></a>

## One mode, four escalation channels
Never prompt for taste or cosmetics mid-run. Classify escalation through the
Auto-Decision Framework; look up derivable facts and try recoverable paths.
A second `NEEDS_CONTEXT` in one run means stop and report.

`AGENT_ROLE` (fallback `GT_ROLE`, default `developer`) selects delivery:

| Channel | Action |
|---------|--------|
| `developer` | One `AskUserQuestion`. |
| `dashboard` | Publish an approval record and await its answer. |
| Caller-supplied | Follow the injected escalation adapter. |
| Other roles | Print `NEEDS_CONTEXT` for the orchestrator; never prompt locally. |

An authenticated caller assignment overrides a stale role/spawned shell echo.
Only delivery changes; analysis, artifacts and decisions stay the same.

### `AGENT_ROLE=dashboard`
```bash
bbs ticket approval publish --kind plan --note "<one question>"
bbs ticket approval await
```
Await without a hard timeout: elapsed time is not approval. `await` polls
every 10s, returns the outcome on stdout and the human's note on stderr;
`--reminder-min` defaults to 30. Handle outcomes:

- `approved`: continue.
- `dropped`: stop the ticket and report.
- `redirected`: rework from the required note and publish a new checkpoint.
- `stale`: reviewed `project-plan` artifacts changed; re-publish for review.

Publish once per checkpoint. Re-publishing an unchanged pending record is
idempotent and does not reset its clock.

### `NEEDS_CONTEXT` shape
```text
STATUS: NEEDS_CONTEXT
REASON: <missing fact or conflicting interpretations>
ATTEMPTED: <what you checked>
RECOMMENDATION: <specific question or next action>
```

## Native task list
Multi-step work MUST mirror the driving artifact into the native task list:
Claude Code `TaskCreate`/`TaskUpdate`, Codex `update_plan`, OMP `todo`.
If unavailable, checkpoint milestones on disk. Mark work in progress when
started, complete after verification. On cold resume, rebuild from disk.

## Preamble (run first)
```sh
bbs skill enter --name <skill>
```
Use the skill's frontmatter `name:`. Keep the returned `SESSION_ID` for the
exit call. The command loads config/identity, checks updates, refreshes
sessions, initializes ticket state, recovers context and logs skill start.
A v2 snapshot supplies facts, never release permission. `--json` retains the
telemetry-only API for callers that own bootstrap.

If `bbs` is missing from PATH, try its installed absolute path (usually
`~/.local/bin/bbs`, or the plugin's `bbs` at the plugin root). If absent or
too old, report `BBS_DEGRADED` with `bbs setup` (`go run ./cmd/bbs setup`
from a checkout) / `brew upgrade lohi-ai/babysit/bbs` guidance and continue
the skill where possible.

### Interpreting the state echo
- `INVOKER`: escalation channel above.
- `PROACTIVE=false`: run only requested skills; silently skip auto-invocation.
- `TELEMETRY=off`: skip all telemetry; otherwise local only.
- `SPAWNED=true`: skip welcome text and optional summaries.

### Ticket consistency — the four-layer invariant
1. Resolve identity on startup/resume: `BABYSIT_TICKET` → manifest cwd-match →
   branch regex. Never infer it from conversation; trunk tickets are valid.
2. Compare `checkpoint.json.ticket` with the resolved ticket. Mismatch →
   `BLOCKED`, naming both IDs and recommending identity correction or
   `bbs autopilot clear <ticket>`; do not resume mismatched state.
3. `bbs autopilot` records step boundaries in `timeline.jsonl`.
4. `bbs ticket get status` is authoritative for ticket existence/status.

No ticket is valid: skip ticket-state writes with a one-line note and use
conversation requirements/plans. Never invent an ID. Branch shape and git-flow
prerequisites belong to workflows.

### Handling update-check output
- `UPGRADE_AVAILABLE <old> <new>`: mention `bbs update` once; continue without updating.
- `JUST_UPGRADED <from> <to>`: start the response with:
  “babysit upgraded v<from> → v<to>. Restart your coding agent to pick up the new skills.”

## Telemetry (run last)
```sh
bbs skill exit --invocation <SESSION_ID> --outcome <outcome>
```
Run after success, error or abort. Outcomes: `success`, `error`, `abort`,
`unknown`. Exit correlates the event, computes duration and removes only its
invocation marker. Session records remain available for recovery.

## Completion Status Protocol
End with one status block. Verdicts are defined in [Handoff Contracts](handoff-contracts.md).
```text
STATUS: DONE | DONE_WITH_CONCERNS | BLOCKED | NEEDS_CONTEXT
VERDICT: <skill-specific verdict>
SUMMARY: <1–2 sentences>
```
- `DONE`: completed with evidence.
- `DONE_WITH_CONCERNS`: completed with nonblocking concerns.
- `BLOCKED`: cannot proceed: tool/access failure, three failed attempts or security uncertainty.
- `NEEDS_CONTEXT`: requires human information, or scope exceeds what you can self-verify.

Non-happy statuses add `REASON`, `ATTEMPTED`, `RECOMMENDATION`.
Hook-enforced: QA `FAIL` → `BLOCKED`; unresolved material review findings →
`BLOCKED`; minor review residuals → `DONE_WITH_CONCERNS`.
