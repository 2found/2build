# Token cost inside a session: skills, subagents and background context

Where the tokens of an autopilot or review-pr run go once a session is running,
what every session pays before it starts work, what was changed on 2026-10-25,
and how to check the changes. The session and prompt-cache side (cold sessions,
ticket boundaries, Team wakes) is in
[foreman-cache-economics.md](foreman-cache-economics.md); this document covers
what that one does not.

Every figure comes from `/bbs:token-audit`, whose script
(`.claude/skills/token-audit/scripts/token_audit.py`) reads local Claude Code,
Codex and OMP session logs:

```bash
python3 .claude/skills/token-audit/scripts/token_audit.py usage --since 2026-10-15 --project babysit-worktrees --harness claude
python3 .claude/skills/token-audit/scripts/token_audit.py context --repo <repo>
```

Figures are **weighted tokens**: input 1, 5-minute cache write 1.25, 1-hour
cache write 2, cache read 0.1, output 5. Because output is included, the
totals are higher than the input-equivalent totals in
foreman-cache-economics.md (681.6M here against 476.9M there for the same
136 sessions). Shares are comparable within one report; they are not prices.

## Baseline: worker sessions, 2026-10-15 to 2026-10-25

136 Claude Code worker sessions (`babysit-worktrees` projects), 681.6M weighted.

| Metric | Value |
|---|---|
| Main thread / subagents | **81%** / 19% |
| Main thread by component | cache read 53%, 1h cache write 17%, output 11% (of the total) |
| Subagents by component | 1h cache write **10%**, cache read 6%, output 2% |
| Context at the first request, median | 57.8k |
| Peak context, median | 265k |
| Turns per session, median | 152 |
| Compactions | 148 |
| Models, main thread | Opus 50%, Sonnet 31% |
| Models, subagents | Sonnet 19% (already set by `CLAUDE_CODE_SUBAGENT_MODEL=sonnet`) |

Main thread by active skill:

| Skill | Share |
|---|---|
| qa | 20% |
| review-pr | 18% |
| autopilot | 18% |
| no skill yet (Team phase integration and setup) | 11% |
| plan-draft | 6% |
| browse | 5% |

Subagents: review-pr's correctness angles A, B and C cost 7% between them
(355k–459k per run); the cleanup, altitude and conventions finders cost about
2.5%; the verifiers about 4%.

Friction found in the same window:

| Signal | Count | Cause |
|---|---|---|
| `bbs autopilot verification begin/record` rejected | ~230 | subject changed after `review-pr --fix` (44), "already owns" an open attempt (42), empty log (25), missing `--attempt` (22), wrong gate name (18), cwd outside the checkout (11), extra results keys (5) |
| `--help` lookups | 644 | agents guessing flags the skill did not state |
| Orca heartbeats | 584 | removed in v1.107 |
| Reviews repeated on the same ticket with the full fan-out | 22 of 68 | no rule for what a second review should target; one ticket was reviewed 6× on its whole `web/` diff |

### What the baseline says

1. **The main thread is the cost.** Cache reads of a ~165k context, over
   ~150 turns, are most of every run. A cheaper subagent model can only touch
   the 19%. Trimming skill text saves 1–2%.
2. **The one-hour subagent cache TTL cost ~4%.** `subagentPromptCacheTtl: "1h"`
   made each short-lived subagent write its prefix at 2× instead of 1.25×, and
   most subagents never live long enough to read it back.
3. **Friction costs turns, and turns cost context.** Every rejected verification
   call and `--help` lookup is a turn that re-reads the whole context.
4. **Repeated reviews were full reviews.** About 7.5% of cost went to review
   runs that re-read a whole ticket diff to check a small fix.

## Background context: what every session loads

Measured with `token_audit.py context` in the kiem-lai checkout (the
2build-consuming repo) on 2026-10-25.

| Item | Before | After |
|---|---|---|
| Claude Code skill listing (user, project, plugins) | ≈26.3k chars ≈ 7.5k tokens | ≈10.4k chars ≈ 3k tokens |
| claude.ai synced skills (`anthropic-skills:*`), almost all unused | 10.0k chars | 0.1k (`skillOverrides`) |
| User skills (`~/.claude/skills`), 10 of 14 unused in 30 days | 5.9k chars | 3.2k (5 removed, 4 `name-only`) |
| Project ops/content skills | 4.0k chars | 0.4k (`disable-model-invocation: true` on 17 of 19) |
| `2build/.claude/skills` relisted as `2build:*` when work touches files under `2build/` | +6.5k chars | unchanged (see below) |
| Stale skill-root copies without `SKILL.md` | 2 | 0 |
| Auto-memory `MEMORY.md` | 25.1 KB / 174 lines (over the 24.4 KB read limit) | 16.0 KB / 137 lines |

The listing is part of the cached prefix, so it is re-read on every turn:
4.5k tokens less is about 2–3% of the main-thread cost of a 150-turn run, in
every repo that loads these roots.

## Changes made on 2026-10-25

| # | Change | Where | Targets |
|---|---|---|---|
| 1 | Removed `subagentPromptCacheTtl` (back to the 5-minute default) | user `~/.claude/settings.json` | 1h subagent writes, ~4% |
| 2 | **Tiered review subagents**: cleanup finders (Reuse, Simplification, Efficiency, Conventions) on the light tier; correctness angles, Altitude, verifiers and sweep on the mid tier; never above the session's model; inherit when the harness has no model selector (Codex, OMP); a routed model that is unavailable reruns on the inherited one, so no finder or vote is lost | `review-pr/SKILL.md` §5 | subagent share |
| 3 | **Delta re-review**: each review gate snapshots its tree with a temporary index (`GIT_INDEX_FILE`, the real index is untouched) and stores it as `review_tree`. A later review on the same base, requirement and plan targets `git diff <review_tree> <now>` with the previous report as context. It falls back to the whole ticket diff when the base, requirement or plan changed, or when the delta is most of the diff | `autopilot/SKILL.md` § Current-session gates | repeated full reviews, ~7.5% |
| 4 | **Verification playbook**: exact gate names, `data.id`, one open attempt per ticket, the three-key results file, `--fix` failing the attempt as expected, "already owns" recovery | `autopilot/references/verification.md` | rejected calls, ~2% |
| 5 | **`/bbs:token-audit`** skill and script, so the next audit is one command and needs no re-derivation | `.claude/skills/token-audit/`, `tests/test_token_audit.sh` | measurement |
| 6 | Effort `medium` for the main model | user `~/.claude/settings.json` | output and thinking |
| 7 | Background context trimmed (table above): ops skills manual-only, unused skills removed (copies in `~/Backups/agent-skills-2026-10-25/`), synced and `artifact-*` skills off by `skillOverrides` | kiem-lai `.claude/skills/*/SKILL.md`, user settings | per-turn context |

Tests: `tests/test_autopilot_review_delta.sh` drives the delta-review protocol
against the real `bbs` binary; `tests/test_autopilot_in_session_planning.sh`
pins the review-pr §5 and autopilot contracts; `tests/test_token_audit.sh`
checks the script against synthetic Claude Code, Codex and OMP logs.

## Checked and not done

| Lever | Estimate | Why not |
|---|---|---|
| Auto-compact window 300k → 200k | ~10% of main thread | 2.4× more compactions; it trades recall for cost, and forced compaction was removed on purpose. User decision. |
| Batch verifiers (several findings per verifier) | ~2% | breaks the tuned one-finding, one-vote design |
| Shorter skill text | 1–2% | not worth the risk to tuned prompts, which are sent word for word |
| Light tier for correctness angles | part of 7% | correctness is where findings come from; judge by findings, not tokens |
| Hide the nested `2build:*` relisting | 6.5k chars when touching `2build/` | no setting found that disables a nested project skill root; a per-name `skillOverrides` on 24 names would also hide `bbs:*` |

## Re-check

After a week of runs on the new skills:

```bash
python3 .claude/skills/token-audit/scripts/token_audit.py usage --since <date> --project babysit-worktrees --harness claude
```

| Metric | Baseline | Expect | Change it tests |
|---|---|---|---|
| `sub:write_1h` share | 10% | ~0; 5-minute writes appear instead at a lower weight | 1 |
| Subagent share | 19% | lower; cleanup finders on the light model in `by model` | 2 |
| review-pr share of the main thread | 18% | lower; second reviews name a `review_tree` | 3 |
| Errored `bbs autopilot verification` calls | ~230 | a handful | 4 |
| `--help` lookups per session | 4.7 | lower | 4 |
| First-turn context, median | 57.8k | lower by the trimmed listing (~4k) | 7 |
| Output share of the main thread | 11% | lower | 6 |

Watch for the cost of each lever as well: findings a light-tier finder missed
(compare review reports before and after), delta reviews that missed a
regression outside the fix, and ops skills nobody remembers to call by
`/name`.
