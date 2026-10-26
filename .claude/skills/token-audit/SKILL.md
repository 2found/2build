---
name: token-audit
description: Audit where agent sessions spend tokens and what every session loads before work starts (skill listing, CLAUDE.md/AGENTS.md, memory), then rank savings that keep quality. Use when runs feel expensive, before tuning skills, models, effort or compaction, or to check background context bloat.
disable-model-invocation: true
---
# token-audit
Measure first, then propose. The usual guess (cheaper subagent models, shorter
skill text) is often the smallest lever; the report must show the real split.
Read-only: it never edits settings, skills or memory in the same run unless the
request explicitly asks to apply the proposals.

Follow [preamble](../references/preamble.md) for bootstrap and telemetry, and
[Auto-Decision Framework](../references/auto-decision-framework.md) for decisions.
Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`.

## Measure
`scripts/token_audit.py` sits beside this file; run it with `python3`. It reads
only local session logs and prints counts, paths and command keys, never
message text.

```sh
python3 <skill-dir>/scripts/token_audit.py usage --days 14 [--project <slug-substring>] [--harness auto|claude|codex|omp]
python3 <skill-dir>/scripts/token_audit.py context --repo <repo> --days 30
```

- `usage`: weighted tokens (input-token equivalents; see the script header)
  split by main thread vs subagents, token component (input, cache write
  5m/1h, cache read, output), active skill, model, subagent label, tool-result
  carry, and friction (errored calls, `--help` lookups, repeated commands and
  skill runs). Scope Foreman workers with `--project babysit-worktrees`.
- `context`: every skill root the harness scans, each skill's listed chars and
  uses in the window, `disable-model-invocation` / `skillOverrides` state,
  duplicate and stale roots, and the instruction/memory files with their limits.
  Skills bundled inside the harness binary have no files; to see the full
  listing after a change, ask a fresh non-interactive session on the smallest
  model to print its skill names, and diff that against the expected set.

Harness coverage: Claude Code logs carry cache writes/reads and subagents;
Codex logs carry cached input but no subagents; OMP logs carry cache
read/write and skill prompts. If the active harness writes none of these logs,
return `NEEDS_CONTEXT` naming the log location needed; do not estimate from
memory.

## Diagnose
Each signal maps to a lever. Apply the lever only when its signal is large
enough to matter (≥2% of weighted, or one background item ≥1k tokens per session).

| Signal | Lever | Quality risk |
|---|---|---|
| Cache reads dominate main thread | Fewer turns (remove friction below) or smaller context per turn | none for friction; context cuts need evidence |
| `sub:write_1h` visible | Unset `subagentPromptCacheTtl`: short subagents pay 2× per write instead of 1.25× | none |
| Errored calls, `--help` lookups, repeated commands | Fix the doc that made the agent guess (exact flags, schemas, error playbook) | none |
| One skill's phase share high | That skill's loop: repeat runs, full re-reviews instead of delta, polling | check the skill's gates still hold |
| Subagent fan-out share high | Route cleanup finders to a lighter tier, keep correctness on mid; never drop a vote | measured on findings, not tokens |
| Large tool-result carry | Narrow reads (`rg`, line ranges, `--json` + `jq`); avoid catting whole files/logs | none |
| Many compactions or high peak context | Lower auto-compact window only with the user: it trades cost for recall | user decision |
| Listed skills unused in the window | Project ops skills → `disable-model-invocation: true` (still `/name`); unused third-party → remove; host-provided → `skillOverrides` `off`/`name-only`/`user-invocable-only` | a skill the model must pick up by itself stops auto-triggering |
| Duplicate roots (`dup`) | A nested `.claude/skills` (submodule) relists the same skills when work touches it | none |
| Stale dir in a skill root | Remove the copy (scanned, never listed) | none |
| MEMORY.md near its read limit, or a large CLAUDE.md | Index one line per entry; move detail to topic files; drop history | keep every live rule |

## Propose
Rank by estimated saving divided by quality risk. Each item gives the lever,
evidence (shares or counts from the report), estimated saving as a share of
weighted tokens, the quality risk, and the owning file or setting. Measured
shares only; a saving under 1% is listed as "minor" or omitted. Name what you
checked and rejected so the next audit does not redo it.

When asked to apply: change only the named files and settings, then compare a
later window against this report. A saving claim needs a before/after window.

## Rules
- Local data only: never send transcript contents to an external service, and
  never paste message text into the report.
- Removing a skill is reversible only if a copy exists: move it aside (or
  reinstall from its recorded source) rather than deleting.
- Global settings and other people's skills change only on explicit request.

## Output
```text
STATUS: DONE | NEEDS_CONTEXT | BLOCKED
VERDICT: AUDITED
WINDOW: <since> · <harnesses> · <sessions> sessions · <weighted>
SPLIT: main <x>% (read <a>% · write <b>% · output <c>%) · subagents <y>%
BACKGROUND: listing ≈<n> tokens · instructions <bytes> · memory <bytes>/<lines>
PROPOSALS:
1. <lever> — saves ≈<s>% — risk <none|low|user decision> — <evidence> — <file/setting>
...
REJECTED: <lever — why>
NEXT: <apply item 1, or "no action needed">
```
