---
name: perf-audit
description: Audit one feature's or the whole system's performance for an owner who is not an engineer: what users feel, proven causes from production signals, fixes ranked easiest and safest first. Use when something feels slow, before buying capacity, or to verify a fix.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# perf-audit
Measure, attribute, then propose. Most "we need bigger servers" is avoidable
work in code or queries; capacity is justified only for measured real work.
This skill evaluates and reports. It never edits product code, config,
infrastructure or data; fixes ship through `autopilot` with this audit's
numbers as their acceptance, and `verify` re-measures them. A cause that is a
bug rather than a cost (wrong results, crashes, a broken deploy) goes to
`investigate`.

Follow [preamble](../shared/preamble.md) and the
[Auto-Decision Framework](../shared/auto-decision-framework.md). Shared
refs are filesystem paths beside this skill's directory, so read them by path, not as `skill://`.
Detail: [measure](references/measure.md) (access, sources, where time goes,
budgets, noise) · [ladder](references/ladder.md) (fix order, safety per rung)
· [profile](references/profile.md) (the project's performance profile) ·
[report](references/report.md) (template).

## Modes
- `audit <feature>` — one journey, page, route or job, end to end.
- `audit` with no target, or `system` — every hot path and shared resource.
- `verify <report>` — re-measure an earlier audit's paths after a fix.

## Writing for the owner
The reader decides with money and risk, not stack traces.
- Never block on a question. Infer business facts (which journeys earn money
  or keep users, complaints, budget, acceptable downtime) from product docs,
  analytics and code; write each as `assumed (<source>, <date>)` and list it
  under "Needs your decision" with its consequence if wrong. Budgets come
  from the profile or the defaults, never from a question.
- Every number carries its date and source, and reads as what a user feels
  ("the chapter page appears after 3.1 s on a phone; the target is 2.5 s").
  Gloss every vendor term once ("Cloudflare, the CDN in front of the site").
- Every proposal states money per month, downtime, risk, reversibility and
  who must approve. Technical detail sits below the owner summary.

## Production is read-only
The rules in [measure](references/measure.md) § Access and safety apply to
every command. In short: only an access route a repo doc or the owner names
as read-only; nothing that writes, restarts, purges or loads production; logs
aggregated, never quoted. The only files this skill writes are its output
dir, the [profile](references/profile.md), and one link line in `AGENTS.md`
when the profile is new.

The one `NEEDS_CONTEXT` this skill may raise is missing read-only production
access. Raise it once, through the
[preamble's channel](../shared/preamble.md#one-mode-four-escalation-channels),
naming exactly what access is needed, for how long and that it only reads;
when the channel cannot wait, finish with lab and static evidence and put the
request in `NEXT`.

## Flow
Output dir: `evidence/perf-audit/<date>-<scope>/` in the ticket when one
resolves, else `.babysit/perf-audit/<date>-<scope>/`. Mirror the steps into
the task list.

1. **Map.** Read `AGENTS.md`, the architecture, caching and deployment docs
   it links, the profile and the previous report (carry its known-findings
   ledger). Feature: trace the path from the user's entry through edge, web,
   API, caches, database, queues and external calls, including requests the
   browser fires after load (polling, prefetch, retries); name the files,
   routes, queries and each cache layer with its owner; write the feature's
   route templates to `<out>/routes.txt` (input to `log_paths.py --routes`);
   draw the path as a sequence diagram. System: components, what runs where,
   sizes, managed services. Where a doc and the code disagree, the code wins:
   map what runs, and record the drift as a finding citing both. No
   architecture doc → map from code and deploy config and recommend
   `architect`. Compare the running revision with the default branch: an
   undeployed fix is a finding, not a bottleneck.
2. **What matters.** Rank paths by weight × total time (calls × mean), from
   production signals first ([measure](references/measure.md) § Sources),
   weight from product docs and the profile. Collect pain: over-budget
   latency, errors and timeouts, saturation, retry loops, regressions around
   a deploy, complaints.
3. **Budget.** Each hot path gets the profile's budget, else the default.
   `green` within budget · `yellow` up to 1.5×, or ≥ 20 % worse than the last
   audit · `red` beyond 1.5×, errors over budget, or a saturated shared
   resource · `unmeasured` without a live or lab number.
4. **Diagnose** red and yellow paths, heaviest first: split the time
   ([measure](references/measure.md) § Where time goes), name the cause in one
   sentence, classify it waste or real work, and prove it (a local toggle
   that removes the suspect and re-measures, a plan at production volume, or
   a counted fact such as queries per request). Check earlier fixes in the
   same area first ([git archaeology](../shared/finding-unknowns.md)). An
   unproven cause is a `suspect`, never a ranked fix.
5. **Rank** each proven cause on the lowest [ladder](references/ladder.md)
   rung that removes it, with estimated gain (ms × calls, CPU share, money),
   confidence, rollout risk and undo. Drop fixes within noise or under 5 % of
   their path. Infra and architecture rungs go to `architect` with the
   headroom evidence.
6. **Report** ([template](references/report.md)) and update the profile. Each
   fix theme is a ready `autopilot` requirement with its acceptance command.

With no live or lab number, the deliverable is a **measurement plan**: the
static findings as suspects, and for each hot path the exact signal to read,
the access it needs and how long it takes. Health is `not-measured`.

## Verify mode
Re-measure the report's paths with the same method, level, window type and
comparable traffic; before → after per path, judged by
[measure](references/measure.md) § Noise. Each finding becomes `fixed`,
`still` or `regressed`, and the rest of the inventory is re-checked for
collateral regressions. A lab verify of undeployed code says so; production
confirmation is a later `verify` after release.

## Output
Health: `good` every measured hot path green · `attention` any yellow ·
`problem` any red · `not-measured` no live or lab number (owner words: Good,
Needs attention, Problem, Not measured).
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: AUDITED(<good|attention|problem|not-measured>, red=<n> yellow=<n> green=<n> unmeasured=<n>) | VERIFIED(fixed=<n> still=<n> regressed=<n>)
REPORT: <report path; profile path>
SUMMARY: <scope, evidence level live|lab|static, worst path vs budget, its cause>
NEXT: <copy-paste /bbs:autopilot "<fix theme>" in ladder order; /bbs:architect; the first measurement or the access request; owner decisions>
```
`not-measured`, or unmeasured critical paths, is `DONE_WITH_CONCERNS`. Not
being able to map the target at all is `BLOCKED`.
