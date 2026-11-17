---
name: ui-audit
description: Audit a running app's UI/UX against the shared UI bar, from the user's side. Whole jobs end to end (blockers fixed and back included), the whole product side by side (duplicates, contradictions, drift), backend ground truth, complexity, text, speed and data shapes; every verdict measured and evidenced, reported for the owner with fixes ranked cut-first. Also builds the flow inventory and re-audits after a fix. Use before planning UI improvements, when the UI feels hard to use, slow or inconsistent, when users get stuck, or to audit a live app safely.
disable-model-invocation: true
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# ui-audit
Make it really good, use lots of tokens and iterate till you're proud of it.

Follow [preamble](../../shared/preamble.md) and the
[Auto-Decision Framework](../../shared/auto-decision-framework.md). The
standard is the [UI bar](../../shared/ui-bar.md): it owns the rules and
numbers, findings cite its row ids. Evaluate and report only; fixes go to
`autopilot`. Browser driving: [`browse`](../browse/SKILL.md). A live system
or a founder's real data: [live-safety](references/live-safety.md) is
binding.

This file says what a good audit proves and how it is scored. How you gather
the evidence (tools, scripts, order, subagents, extra checks) is yours: pick
whatever reaches the truth fastest.

## Why it exists
The agent that built the UI judged one ticket, one screen at a time; an agent
driving a UI to finish a task stops seeing it. Every screen can pass alone
while the product fails the person using it. The audit restores the user's
view and puts numbers on it.

## Principles
1. **The job is the unit.** Judge journeys from trigger to done, as the
   persona lives them. A journey that meets a blocker (balance too low,
   signed out, missing key, quota) is walked through the fix and back to the
   original job's end: donate and top-up each passing alone proves nothing
   when "donate → top up in Profile → start again" is what the user gets.
2. **Side by side.** Duplicated facts, contradicting numbers, one job spread
   over four tabs, five loading styles: these appear only with screens
   compared. A product audit always compares, in source and on screen, and
   lays each screen's states beside each other too (loading, empty, error,
   filled, expanded, submitting): what did not change must not move.
3. **The backend is the truth.** Know what the system knows (counts, pending
   work, failures, every state and gate code) before judging the UI. Hidden
   work, false statuses and contradictions outrank anything cosmetic.
4. **Measured, not opined.** Every verdict rests on a count, a measurement or
   a screenshot you looked at. In the DOM is not visible. Ratings of
   difficulty or clarity need a rater who has not seen your findings; your
   own eye is one rater.
5. **Persona knowledge only.** Whatever you used that the persona would not
   have (a URL, the source, the API, a guess, a retry) marks where the UI
   failed. Log it as a workaround.
6. **Real conditions.** The build and engine users run, production-shaped
   data (longest list, null, 80-char name, large number), the narrowest
   width, the loading, empty, error and slow states, and at least one
   non-happy path per journey before any PASS.
7. **Subtract first.** Products grow by addition. Every region must serve a
   job; the first fix proposed is what to cut, then merge, reuse, rework,
   and add only for a job the product cannot do.
8. **Fix the cause where it lives.** UI, backend, content or design
   authority; never propose a UI patch over a backend cause. A drift class
   seen twice is a missing rule: propose the sentence for the design
   authority and the mechanical check that would have failed it.

## Scope
`product` (default): every primary screen compared, every inventory flow
verdicted, the top flows by freq × risk walked in depth. `flow <id>`: one
journey in depth plus the screens it touches beside a sibling each.
`inventory`: build or refresh the flow inventory. `re-audit <report>`: see
below. No runnable target: source evidence only, runtime lenses
`not measured`, `DONE_WITH_CONCERNS` naming what would make it runnable.

## Flow inventory
One file per product beside its design authority (e.g. `docs/ux/flows.md`),
shared with `design-ui`. Rows are the persona's recurring jobs, not the menu:

| id | flow | persona | trigger → done when | freq | risk | ideal steps | blockers → fix flow | last audit |

`ideal steps` is the fewest deliberate actions a perfect UI needs (the
step-ratio baseline). `blockers → fix flow` comes from the backend's gate and
error codes. A job the product cannot do is a row marked `missing`; a
primary screen that serves no row is a cut candidate.

## What is measured
A lens is `pass`, `fail` or `not measured` (with why); it fails when any of
its rows fails. Never "partly", and `not measured` is never a pass. Numbers
are the UI bar's unless the design authority sets its own.

| lens | bar rows | quantity | pass |
|------|----------|----------|------|
| Flows | J1–J5 | per journey: actions ÷ ideal, screen switches beyond ideal, values re-entered or carried in the head, steps redone after a blocker; object states with no screen or no exit | ratio ≤ 1.5; the rest 0; every journey PASS |
| Overlap | O1–O3 | facts and actions with more than one owner screen; values that differ between screens; names per concept | 0 · 0 · 1 |
| Simple | S1–S4 | per view: primary actions, nav entries, tabs, row actions, depth, where the work surface starts; cold-read match | within the bar |
| Text | T1–T5 | rows over 2 lines, helper paragraphs, repeated facts, labels over length, how-to sentences; words in the first viewport vs the product median; must-keep text cut | 0; ≤ 2× median |
| System | C1–C6 | idioms per job (counted in source and on screen), behaviours per kind of action, values per geometry role, idioms per state across screens; px moved by unchanged elements and controls resized on each state change | 1; 0 |
| Speed | P1–P5 | LCP, INP, CLS, duplicate or unbounded requests, back-navigation | within the bar |
| Data | D1–D3 | production-shaped cases | no crash, raw number or flooded list |
| Truth | — | backend facts the UI never shows; statuses the data contradicts | 0 · 0 |
| Access | WCAG 2.2 AA | contrast, target size, visible focus | 0 failures |
| Agent | [agent-operable](../../shared/agent-operable.md) | recurring UI-only actions; shown facts with no read endpoint | 0 |

Per journey:
- **Verdict:** PASS (reaches its end on the ideal path) · FRICTION (reaches
  it with detours, a missing next step, or a blocker that restarts the job)
  · BLOCKED (cannot finish, or not testable here, with why).
- **Recovery** per blocker and non-happy path: resumed (same step, input
  kept, refreshed state shown) · restarted · with help · not. Pass = resumed.
- **Step difficulty** (PURE, Rohrer et al.): 1 known pattern · 2 some load ·
  3 some target users fail, or easy but leads to the wrong result. Two
  independent raters, score the worse; task color = worst step; a 3 fails.

**Severity** (Nielsen, frequency × impact × persistence): 4 blocks, loses
data or misleads into harm · 3 needs a workaround or causes real confusion ·
2 hesitation or inconsistency · 1 cosmetic. A drift or overlap class counts
once, at its worst instance, with its instance count. **Grade:** A = every
lens passes, max severity ≤ 1 · B = max 2 · C = any 3 or a failed lens ·
F = any 4, a PURE red task or a `missing` high-frequency job.

## Evidence
A finding carries its bar row, its proof (screenshot region, quoted text,
count or repro), its owner layer and, with source access, file:line. No proof
→ a `question`, as is a symptom seen once that does not reproduce. Re-open
the shot of every severity ≥ 3. Keep raw captures on disk beside the report
and read back only what a verdict needs: dumps in context are the audit's
main cost.

## Report
`<out>/report.md`, out = the ticket dir, else
`.babysit/ui-audit/<date>-<scope>/`. Its shape is yours; it must hold:
1. **For the owner**, standing alone: the grade, one plain sentence per lens
   with its worst number, and at most seven problems, most blocking first,
   each as what the user has to do today → what it should be, with its
   screenshot. No ids, paths or metric names.
2. Header: build/commit, target, driver, data, mutations made, limits.
3. Lens verdicts with the worst number against the bar; journey verdicts;
   findings ranked by user effort or stall, not by ease of fix.
4. The known-findings ledger, carried from the previous report and
   re-measured in this run.
5. Fix themes ranked the same way, each a ready `autopilot` requirement:
   problem in the user's words, change on the lowest rung that removes it
   (cut → merge → reuse → rework → add), acceptance = bar row + number
   measured here → number to reach.
6. What works (keep when fixing) and cleanup.

Update each walked flow's `last audit` cell.

## Re-audit
Same scope and measurements on the new build. Each old finding is `fixed`,
`still` or `regressed`, shown as a before/after pair at the same screen,
viewport and data with the number beside each. Tests and builds never
confirm a UX fix. With a prototype, add prototype-vs-real per state, failure
states included; never edit the prototype to match the build.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: AUDITED(<grade>, sev4=<n> sev3=<n> sev2=<n> sev1=<n>) | INVENTORIED(<n>)
REPORT: <report path; inventory path>
SUMMARY: <scope, persona, failed lenses with their worst number, journeys PASS/FRICTION/BLOCKED, top findings>
NEXT: <copy-paste /bbs:autopilot "<fix theme>" in rank order, or the next flow>
CLEANUP: <session closed, PIDs stopped, secrets deleted; mutations made>
```
No screenshots you looked at, or a browser or secret file of yours left
behind → `BLOCKED`. A product audit without the side-by-side comparison, or
with a lens `not measured` → at best `DONE_WITH_CONCERNS`.

Sources: PURE — Rohrer et al., CHI 2016; Nielsen severity ratings and
response-time limits (nngroup.com); Core Web Vitals (web.dev/vitals); WCAG
2.2. Other thresholds are 2build defaults in the UI bar.
