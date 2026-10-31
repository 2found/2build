# Foreman token cost and the prompt cache

What a Foreman project costs in tokens, where the cost comes from, what was
changed on 2026-10-25 to cut it, and how to check whether those changes worked.
The baseline is reproducible: `scripts/foreman-cache-report.py` reads the
local Claude Code transcripts (`~/.claude/projects`) and prints the same
metrics for any time window.

## How the cost works

Claude Code caches the prompt prefix. A request that resumes within the cache
lifetime **reads** its whole context at 0.1× the input price; one that resumes
after the lifetime has run out, or starts in a new session, **writes** it again
at 1.25× (5-minute cache) or 2× (1-hour cache; Claude Code here writes 1-hour
entries). So the cost of a turn is roughly 0.1 × its context size, and a cold
start costs about twenty warm turns.

All figures below are **input-equivalent tokens**:
`read × 0.1 + 5m write × 1.25 + 1h write × 2 + uncached input`. Output tokens
are listed separately.

## Baseline (sessions started before 2026-10-25 12:00 UTC)

Measured from 136 worker sessions and 5 Foreman sessions, mostly from the
Orca-orchestration era, before `bbs foreman task` existed. The raw report is
in [`data/foreman-cache-baseline-2026-10-25.json`](data/foreman-cache-baseline-2026-10-25.json).

### Workers (ticket sessions)

| Metric | Baseline |
|---|---|
| Worker sessions / ticket worktrees | 136 / 66 |
| Sessions per worktree, median / mean | 1.5 / 2.06 |
| Worktrees that needed an extra session | **33 of 66** (70 extra sessions) |
| Total cost | 476.9M (reads 358.5M = 75%, 1h writes 118.4M) |
| Cache hit ratio (read ÷ read+write) | **98.4%** |
| Waste from expired cache (cold turns not caused by compaction) | **2.0%** |
| Turns / cost per turn | 21,891 / ~22k (roughly flat whatever the session length; auto-compaction at ~266k drops the context to ~80k) |
| Context at the first request, median | 58k |
| Cost of the first 25 turns (ramp-up), median | 0.5M |
| Cost per session / per worktree, median | 3.2M / **6.2M** |
| Subagents (mostly review-pr angle agents) | 455, 115.6M, **62% cache writes**, median 1.3M per worktree |

From reading the sessions (not computed by the script):

- An extra session costs about **2M** in all: its ramp-up, its own review
  subagents and QA. Almost every extra session was a refresh, repair or
  conflict fix against a sibling ticket in the same phase that **edited the same
  files or changed an interface the ticket consumed**. For example, two
  tickets both deleted the same service files, and a web ticket had to refresh
  after its API sibling changed its 409 codes.
- Workers read the whole parent `design.md` (~17k) and the whole
  `plan.md` + manifest (~17k) at the start. Those ~35k tokens stay in the
  context and are re-read on every later turn.
- About 35% of worker cost is exploration (search, reads through the shell,
  `Read`).

### Foreman

| Metric | Baseline |
|---|---|
| Sessions / cost | 5 / 87.6M |
| Wakes started by the watcher (skill prompt) | 104 wakes, 25.8M, median 7 turns and 176k per wake, **22 cold (21%)** |
| Wakes from Orca orchestration messages (the old era; gone since `bbs foreman task`) | 331 wakes, 27.8M, median 52k |
| Other wakes (the human, task notifications) | 80 wakes, 33.9M, median 200k, 15 cold |
| Keep-warms | 0 (did not exist yet) |

A cold wake costs 0.2–0.45M. Cold wakes followed gaps of more than 60 minutes,
and the watcher's status prompt defaulted to 3600 s, just past the 1-hour
cache lifetime, so every one of those prompts landed cold.

### What the baseline says

The cache hit rate was already near its ceiling. The real costs were:

1. **Context × turns**: what a worker loads first, it pays for again on
   every turn.
2. **Cold sessions caused by ticket boundaries**: about 2M per extra session,
   needed by half of all tickets.
3. **Cold starts of subagents**: the review angle agents.
4. **Cold Foreman wakes** after gaps longer than the cache lifetime.

## Changes made on 2026-10-25

| # | Change | Where | Targets |
|---|---|---|---|
| 1 | **Split by context, not by size.** Work on one module, one set of files, or an interface still being shaped is one ticket and one long session, even when it ends up `L`. Split only where modules are disjoint behind an interface frozen in the design. Being ordered (`blocked_by`) is not a reason to split. A phase that changes one module is one ticket (the old rule was "2–5 tickets per phase"). A hard invariant gets its own ticket only when it owns its own files. | `team/references/topology.md` § Split by context, `phases.md`, `model-routing.md`, `plan-draft/SKILL.md`, `team/SKILL.md` rule 2 | extra sessions per worktree |
| 2 | **`task done` gate.** Tasks created with `--kind ticket\|repair\|refresh\|redirect` cannot report `succeeded` until the ticket passes readiness (review, or light when heavy checks are deferred); the refusal prints `GATE=not-ready` and the reasons. `--no-gate "<why>"` skips it, and an unreadable readiness check is recorded as `unverified` instead of blocking. The verdict is stored as `Dispatch.Gate`. | `internal/cmd/foreman_task.go`, `internal/foreman/task.go` | cold sessions launched only to re-record gates |
| 3 | **One follow-up per ticket per wake**: several triggers for the same ticket go into one spec. | `team/SKILL.md`, `tickets.md` | extra sessions |
| 4 | **Scoped briefs**: a worker gets its ticket's slice of the design, plan and manifest (path and heading), not the whole documents. | `tickets.md`, `autopilot/SKILL.md` | base context, cost per turn |
| 5 | **`batch show` includes each settled member's ticket, kind, gate, summary and report**, so Foreman needs no `task show` per member. Foreman is told to chain commands into few tool calls. | `internal/cmd/foreman_batch.go`, `team/SKILL.md` | turns per wake |
| 6 | **`foreman_status_interval` default 3600 → 3300**, so the status prompt arrives while the cache is still warm. | `internal/config/config.go` | cold watcher wakes |
| 7 | **`foreman_cache_ttl`** (default from the agent: claude 3600, others 300). `await` and `task ask` block for at most the lifetime minus 60 s (still capped at 540 s). During a batch, the watcher sends a one-line keep-warm (plain text, not the skill prompt) once the batch has gone the lifetime minus 10 minutes without prompting Foreman, at most 8 per batch, and only when the lifetime is at least 30 minutes. | `internal/cmd/foreman_cache.go`, `foreman_batch.go`, `foreman_watch.go` | cold Foreman wakes after a long batch |

## Re-check

Run this after a few Foreman projects have gone through the new flow. The
cutoff must be after the 2build build containing these changes was installed.

```bash
python3 scripts/foreman-cache-report.py --since <install date>
python3 scripts/foreman-cache-report.py --since <install date> --json > /tmp/after.json
```

Compare with the baseline above, normalised per worktree (projects differ in
size). Expected if the changes work:

| Metric | Baseline | Expect | Change it tests |
|---|---|---|---|
| Worktrees with an extra session | 33/66 (50%) | well under 50% | 1, 2, 3 |
| Sessions per worktree, mean | 2.06 | toward 1.3 | 1, 2, 3 |
| Median cost per worktree | 6.2M | higher is fine if the worktree count drops; judge **total cost per project** | 1 |
| Context at the first request, median | 58k | lower | 4 |
| `task done` gate refusals | 0 | > 0 means the gate caught a gap while the worker was warm | 2 |
| Watcher wakes landing cold | 21% | under 10% | 6, 7 |
| Keep-warms that landed cold | — | ~0, each costing ≈ 0.1 × Foreman context | 7 |
| Turns per watcher wake, median | 7 | lower | 5 |
| Cache hit ratio / expiry waste | 98.4% / 2.0% | unchanged or better (not the lever) | — |

What to look for in the result:

- **Tickets got too big.** Sessions that compact many times, `L` plans that
  stall, or worse review quality are the price of affinity. If they show up,
  the split rule needs a size ceiling, not a return to "2–5 tickets".
- **The gate is abused.** Look at `--no-gate` reasons in Task records
  (`$BABYSIT_HOME/tasks/*/task.json`, `gate: "skipped: …"`) and at
  `unverified` verdicts.
- **Keep-warms are noise.** If Foreman runs other work during batches, its
  pane is never idle and keep-warms cost without saving anything; check
  `keep_warms` against `watcher` cold wakes.
- **Agents other than Claude** leave no transcripts here; this report only
  covers Claude Code sessions.

## Re-check 2026-10-27 and the follow-up loop

`--since 2026-10-25T12:00:00Z` (Claude sessions only; 2 Foreman sessions, 6
worktrees — a small sample):

| Metric | Baseline | 2026-10-27 |
|---|---|---|
| Watcher wakes landing cold | 21% | **0%** (all wakes 6.5%) |
| Foreman cost | 87.6M / 5 sessions | 5.3M / 2 sessions |
| Context at the first request, median | 58k | 51k |
| Sessions per worktree, mean | 2.06 | **2.83** (worse) |
| Worktrees with an extra session | 50% | **5 of 6** |
| `task done` gate refusals | 0 | 15 |
| Subagents | — | 25% of cost, 54% cache writes |

The Foreman side worked; the worker side did not. Task records
(`$BABYSIT_HOME/tasks/*/task.json`) show where the wall time went. One phase
of three parallel client tickets coded in 33–41 minutes each, then spent
about ten hours in follow-ups: two refreshes of ~72 minutes each that adopted
work touching none of the ticket's files and still re-ran a full 8-angle
review and QA; five repairs because all three tickets edited the same api seam
files (`requests/index.ts`, `messages/routes.ts`); four to six phase QA and
product-review sessions, each finding fixed by its own cold repair. Another
project's first phase ran product review four times and phase QA twice.

Changes (v1.111.0):

| # | Change | Where |
|---|---|---|
| 8 | **Compose check at `task done`.** A ticket whose tree conflicts with a phase sibling that already reported is refused `GATE=not-composing` with `CONFLICT_WITH=<sibling>: <files>`; the warm worker adopts and resolves it. Seam files (composition roots, registrations, module `index`, catalogs, generated types) get one owner per phase in the manifest. | `internal/cmd/foreman_compose_gate.go`, `topology.md` |
| 9 | **Carried gates.** When every path changed since `review_tree` came verbatim from an adopted snapshot and none is in the ticket's own diff, `task done` records `GATE="carried: …"` and `seal` waives the stale verdicts with that reason. No review or QA re-run for a disjoint refresh. | same file, `foreman_project.go`, `autopilot/SKILL.md` |
| 10 | **One session for phase QA and product review**, all findings grouped into one repair per owning ticket, and the re-check carries both attempts' passed checks. | `phases.md`, `product-review.md`, `execution.md` |
| 11 | **Follow-up review at `low` effort on the delta**, unless it touches money, auth or irreversible-data paths. | `autopilot/SKILL.md`, `tickets.md` |

Re-check with the same report after a few projects: expect sessions per
worktree back under 2, refresh Dispatches under 20 minutes, at most one
product-review attempt per phase, and `GATE="carried: …"` / `not-composing`
appearing in task records. Projects run on `omp` leave no transcripts, so for
those judge by Dispatch durations in the task records.

## Phase cost by gate, 2026-10-27 (4 hours, soot-autoupdate p2)

Worker spend in the window was 34.2M (plus 0.9M Foreman), 93% of it one
three-ticket phase. Attributing each turn to the last gate skill invoked
(review includes the `--fix` work after it):

| Step | Cost | Wall time |
|---|---|---|
| Tail of the 3 ticket sessions (review/QA) + 1 ruling repair | 5.7M | tickets ran 66–121 min |
| Phase QA + product review (two parallel dispatches) | 9.3M | 55 min |
| Three `repair` sessions after phase QA | 14.8M | 66–93 min each |
| A `refresh` after the repairs (still running) | 2.1M | — |

`review-pr` and its subagents were about 48% of the spend; every repair
re-ran `review-pr medium --fix` with 7–12 finder subagents (2–4.6M each)
although v1.111.0 asked for a low-effort delta review. Per-ticket review and
QA did not prevent the phase findings, and the cold repair cycle after phase
QA cost more than the phase QA itself.

Changes (v1.113.0):

| # | Change | Where |
|---|---|---|
| 12 | **Tickets defer gates to the phase.** Every ticket leaves `review-pr` and `qa` to its phase; a change whose paths hit the money/auth/irreversible-data floor keeps its own `review-pr`. `task done`, `await` (`GATE="deferred to phase review: …"`) and `seal` apply it. | `internal/cmd/foreman_defer_gate.go`, `autopilot/SKILL.md`, `tickets.md` |
| 13 | **One review of the phase diff.** When any ticket deferred review or took a fixup, phase readiness needs a passing `<phase>/review` check in the integration evidence; it never carries — a rebuild after land is a fix delta, reviewed at `low`. | `foreman_project_phases.go`, `foreman_project_evidence.go`, `phases.md` |
| 14 | **Integration session fixes small findings** in the owning ticket's worktree and reseals it with `bbs foreman seal <ticket> --fixup "<finding>"` (refused while a Dispatch owns the ticket), then integrates again and re-checks only what changed. Only findings it cannot fix locally become `repair` Dispatches. | `foreman_project.go`, `phases.md`, `qa/SKILL.md` |

| 15 | **Land first, verify after.** A sealed phase is integrated and landed at once; phase QA runs on the landed commit and a failure lands as one fix commit (composed against the landed phase). Base never moves under a tested commit, so there is no re-integration or re-QA for drift; release still waits on phase QA. | `foreman_project_phases.go`, `foreman_project_integrate.go`, `phases.md` |

Re-check: per-ticket sessions without review/QA for `XS`–`S`, at most one
`repair` Dispatch per phase, and phase wall time from first dispatch to land.

## Measurement notes

- A **worker session** is a main transcript in a project directory whose name
  contains `-babysit-worktrees-` (one directory per ticket worktree). Its
  `subagents/` transcripts are counted as its subagents.
- A **Foreman session** is a main transcript with a user prompt invoking
  `/bbs:foreman`. Each non-harness user prompt starts one wake, tagged
  `watcher`, `keep_warm`, `orchestration` or `other`. The baseline includes one
  development session and one probe session, which inflate `other`.
- A **cold** request has a context over 20k with more than half of it written
  to the cache. Expiry waste counts only cold requests not explained by
  compaction.
- Requests are deduplicated by request id. A session belongs to a window by
  the time of its first request.
