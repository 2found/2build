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
| Models, subagents | Sonnet 19% (then set by `CLAUDE_CODE_SUBAGENT_MODEL=sonnet`, removed 2026-11-25) |

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
| Batch verifiers (several findings per verifier) | ~2% | done 2026-11-25 (below): one vote per finding is kept, inside one verifier |
| Shorter skill text | 1–2% | not worth the risk to tuned prompts, which are sent word for word |
| Light tier for correctness angles | part of 7% | correctness is where findings come from; judge by findings, not tokens |
| Hide the nested `2build:*` relisting | 6.5k chars when touching `2build/` | no setting found that disables a nested project skill root; a per-name `skillOverrides` on 24 names would also hide `bbs:*` |

## Changes made on 2026-11-25

| # | Change | Where | Targets |
|---|---|---|---|
| 8 | **`implement` always in a `flash`-tier subagent** (Claude Code `sonnet` at high effort, Codex/OMP from `bbs team model`), every ticket size and UI/UX too. One subagent is continued across milestones and retries (`SendMessage`) instead of a cold one each time; review/QA fixes stay in the session | `autopilot/SKILL.md` § Subagent steps | Opus main-thread output and cache reads while coding |
| 9 | **review-pr runs three subagents** from medium up: a correctness finder, a cleanup finder (each works its angles over one warm context) and one verifier for the whole deduplicated list, which also runs the xhigh/max sweep. Every subagent prompt opens with the same diff brief so the verifier reads it from cache. Was 8–10 finders plus one verifier per candidate (up to 48) | `review-pr/SKILL.md` `F0n`, Phase 2/3 | subagent cache writes (62% of subagent spend in foreman-cache-economics.md) |
| 10 | Model passed explicitly to every pinned subagent; `CLAUDE_CODE_SUBAGENT_MODEL` and `subagentPromptCacheTtl: "1h"` no longer seeded by `setup-project` (it removes the old seeded values) and removed from the user settings | `setup-project/SKILL.md`, `~/.claude/settings.json` | silent model drift on inherited subagents; 2× subagent cache writes |
| 11 | **Design then plan in two subagents**: `design-ui`'s spec carries a `Code map` (files it read that the build touches or reuses) and `plan-draft` starts from it in a fresh subagent; the design subagent is continued only for a light design | `autopilot/SKILL.md`, `design-ui/SKILL.md`, `plan-draft/SKILL.md` | design context re-read on every planning turn; re-surveying code |
| 12 | review-pr writes the diff to a file and passes its path; a diff pasted into a subagent prompt is output at 5× | `review-pr/SKILL.md` `F0n` | output tokens |
| 13 | Team lead: no background tools (each completion is a wake), one-read reply to unexpected wakes, a batch's launch commands in one script, kickoff codebase read and `integrate` conflicts in a foreground subagent | `team/SKILL.md`, `kickoff.md`, `worker-routing.md`, `git-flow.md` | lead turns and context (the lead cost more than all workers in the 2026-11-13 re-check) |
| 14 | Turn and tool-output rule in the preamble (chain calls, logs to file + `tail`, screenshots only for layout); milestone checks read `--stat` plus named hunks; release waits with `gh run watch` | `preamble.md`, `autopilot/SKILL.md`, `release/SKILL.md`, `browse/SKILL.md`, `implement/SKILL.md` | turns and retained output |
| 15 | **semantic-decision is calling-LLM only**: the Cloudflare/Clef provider, its config keys (now ignored), credentials and project limits are removed; a decision is judged in-session and recorded with one `bbs semantic-decision --llm-answers` call, the request/answers template inline in the skill | `internal/decision`, `internal/cmd/semantic_decision.go`, `semantic-decision/SKILL.md` | 3–5 turns per decision (~344 decisions in the log, all `llm`) |
| 16 | qa prototype conformance compares text first (scoped snapshot diff, box `eval`); images viewed only for differences, 80-character and narrowest-width states | `qa/SKILL.md` | images re-read every executor turn |
| 17 | `simple` ticket workers route to `flash` (`simple`×`critical`); a phase diff ≤ ~200 lines off the sensitive paths is reviewed at `low` | `internal/config/foreman_models.go`, `team/references/phase-qa.md` | ticket-worker model price; small phase reviews |
| 18 | Team references split by reader: phase-QA material moved to `phase-qa.md`, OMP launch to `worker-routing-omp.md`, duplicated rules given one owner; the lead's entry reading set fell from 68.6 KB to 59.1 KB | `team/references/` | lead context at entry and after each compaction |
| 19 | `plan-draft` on `flash` for `S`, or up to `M` with a design `Code map`, off the floor paths; `implement` at medium effort on `XS`/`S`; each milestone's acceptance reads hunks of files the plan does not name (silent scope drift fails it) | `autopilot/SKILL.md` | plan model price, thinking output |
| 20 | **File size budget** (800 lines per hand-written source file, or the entrypoint's own): new responsibilities in new files, an over-budget file never grows, big files read by range. Stated in the 2build pointer every session loads; checked at each milestone and by review-pr's File size angle; native ESLint/SwiftLint ratchet seeded by setup-project; oversized files reported by harness-audit | `references/file-size.md`, `implement`, `plan-draft`, `autopilot`, `review-pr`, `setup-project`, `harness-audit` | whole-file reads of 2–5k-line files (~25–60k tokens each, re-read every turn) |
| 21 | review-pr **Plan conformance** angle (`--plan`, in the cleanup finder; no new subagent) | `review-pr/SKILL.md` `F1n`, autopilot gate | quality: silent departures from the plan's Approach and contracts |
