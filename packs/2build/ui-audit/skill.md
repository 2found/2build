---
name: ui-audit
description: Audit a running app's UI/UX against the shared UI bar. Maps the whole product side by side (features duplicated across screens, jobs that cannot finish, complexity, text density, component and loading/spacing drift, UI speed, backend ground truth) and walks real user journeys end to end (PASS/FRICTION/BLOCKED); measured and evidenced, reported for the owner with fixes ranked cut-first. Also builds the flow inventory and re-audits after a fix. Use before planning UI improvements, when the UI feels hard to use, slow or inconsistent, when users get stuck, or to audit a live app safely.
disable-model-invocation: true
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# ui-audit
Make it really good, use lots of tokens and iterate till you're proud of it.

Follow [preamble](../shared/preamble.md) and the
[Auto-Decision Framework](../shared/auto-decision-framework.md). Shared
refs are filesystem paths beside this skill's directory, so read them by path, not as `skill://`.
The standard is the [UI bar](../shared/ui-bar.md); findings cite its row
ids. Evaluate and report only; fixes go to `autopilot` (`plan-draft` /
`design-ui` / `implement`). Browser setup, session and cleanup: the
[`browse`](../browse/skill.md) skill (`agent-browser`); do not duplicate it.
Detail lives in `references/`: [checklists](references/checklists.md) ·
[live-safety](references/live-safety.md) · [helpers](references/helpers.md) ·
[report-template](references/report-template.md).

## Agent blind spots (the reason this skill exists)
An agent driving a UI to finish a task stops seeing it, and the agent that
built it judged one ticket at a time. Hard rules:
- **A screen is not the product.** Every screen can pass alone while the
  product fails: one number on three screens, five loading styles, a job
  spread over four tabs. These show only with the screens laid side by side,
  so the Map (*Procedure §3*) is never skipped for a product audit.
- **Built is not needed.** The product grew by addition. Ask of every screen
  and region which job uses it; the first proposal is always what to remove.
- **Judge visibility from pixels.** Look at every screenshot you take: Map
  shots on contact sheets, full size for anything you rate, every severity
  ≥ 3 finding and every before/after pair. The
  accessibility tree/DOM proves existence, not visibility: present-in-tree but
  0-sized, clipped, covered or off-screen is a finding, never a click target.
  Walk and read text from snapshot text (cheap, exact quotes); screenshots are
  the evidence for layout, contrast and anything you rate.
- **The UI is not the truth.** Gather the backend facts first (counts, pending
  work, failures) and check the UI shows each one. Hidden work and numbers
  that contradict each other outrank every cosmetic finding.
- **A screen is not a journey.** The bar for every screen: does it show the
  next step, and does the journey reach its end?
- **Fixtures flatter.** Three tidy rows hide the 60-row list, the null
  measure that blanks a tab and the raw `11360000`. Audit the data shapes
  production holds (D1–D3).
- **Persona knowledge only.** Anything the persona would not know that you
  used — source, CLI, logs, URLs, API, tree search, element index, a guess —
  is a `workaround` row. Every retry (misclick, lost focus, input that did
  not land) is logged before retrying.
- **Success is not a verdict, and neither is your own eye.** Scores come from
  counts, measurements and raters who have not seen your findings
  (*Procedure §7*).

## Scope
- `product` (default when no flow is named): the Map over every primary
  screen, a journey verdict for every inventory flow, and the full step log
  for the top three by freq × risk. No inventory yet → build it first.
- `flow <id>`: one journey in depth; the Map lenses run on the screens it
  touches plus one sibling screen each, for comparison.
- `inventory`: build or refresh the flow inventory only.
- `re-audit <report>`: *Re-audit* below.

No runnable target → the Map tables tagged `source` only, every other lens
`not measured`, and `DONE_WITH_CONCERNS` naming what would make it runnable.

## Inputs
Target = the build users actually run, on the engine they run it in: a
WebKit shell (Tauri, WKWebView, Safari) is audited in WebKit, not in
`agent-browser`'s Chromium. A number only another engine can take (LCP, CLS,
INP outside Chromium) is taken there, labelled cross-engine in the header,
and counts as measured. Record version/commit and whether the tree is dirty.
If the tree changes under you mid-audit (another agent, hot reload), keep
going and never re-run a pass for it: note the change in the header, and a
symptom seen once around it that does not reproduce is a `question`, not a
finding. Personas from
the brief, `AGENTS.md`, design authority or product docs; none → derive from
the stated audience and say so. Driver: `browse` for web, the caller's GUI
driver for desktop; real input only, no state-changing `eval`/API calls
inside the flow. Fixture records are named `audit-…` and listed in the report;
never touch data not designated for the audit. Auditing production or a
founder's live data: follow [live-safety](references/live-safety.md) (read-only
default, mutation allowlist, secrets, isolated session, snapshot copy for any
mutation); an origin/auth gate is reported `blocked here`, never bypassed.

## Flow inventory
One file per product beside its design authority (e.g. `docs/ux/flows.md`),
shared with `design-ui`, which adds the rows a change serves. Rows come from
the persona's recurring jobs, not the menu (for a whole product: the 8-15
journeys the owner lives in, e.g. morning "what needs me", run a flow now,
handle a failure, fix a missing credential):

| id | flow | persona | trigger → done when | entry | freq | risk | ideal steps | last audit |

`ideal steps` = fewest deliberate actions a perfect UI needs (the step-ratio
baseline). Order by freq × risk. A job the product cannot do is a row marked
`missing`. Every primary screen should belong to ≥ 1 flow.

## Procedure
Output dir: ticket dir, else `.babysit/ui-audit/<date>-<scope>/`; screenshots
`shots/NN-<what>.png`, Map tables `map/<table>.md`. The Map's source sweep,
the cold reads and the second rater are independent of the walk: run them in
subagents when the harness has them. Turns are the cost: crawl the Map with
one script per pass that writes raw JSON and shots to disk and prints one
line per screen, build the tables from those files, and never read a dump
back into context.
1. **Frame** before opening the app: persona, goal, done-when, expected entry,
   ideal path. Read the design authority, the previous report (carry its
   known-findings ledger) and any design note's UX contract for the screens
   in scope. A carried finding is re-measured in this run; no Map table is
   filled from an earlier report.
2. **Ground truth** (read-only): list backend facts and mark where the UI shows
   each, and list each object's states for the lifecycle table
   ([checklists](references/checklists.md) § Ground truth).
3. **Map**: shoot every primary screen (navigation entry, tab, sheet) loaded
   with production-shaped data at the common and the narrowest width, then
   build the tables in [checklists](references/checklists.md) § Map: screens,
   ownership, lifecycle, states, geometry, complexity, speed, drift. Counted
   and measured, never eyeballed.
4. **Walk** the happy path, one row per deliberate action: expectation →
   screenshot → what the screen asks of the persona → act → screenshot →
   result. Rate the step (PURE), record waits, screen switches, anything
   retyped or carried in the head, and the exact clicks. Journey verdict: PASS
   (reaches its end) · FRICTION (reaches it with detours or missing next step) ·
   BLOCKED (cannot finish, or not testable here, say why).
5. **Sweeps**: dead ends, discoverability of user needs, iframes/embedded
   pages, agent path ([checklists](references/checklists.md)). Count clicks
   from where the user thinks the feature lives; try search/command palette.
6. **Non-happy passes** (all that are reachable; at least one before any
   PASS): missing credential, over budget, unreachable dependency, empty data,
   failed background run, crash and retry; invalid/duplicate input; abandon
   mid-way and re-enter; first-run empty state; narrowest width; other color
   scheme; keyboard-only main action.
7. **Independent raters**, each a fresh subagent that never sees your ratings
   or findings. *Cold read* (S2): one screenshot per primary screen and the
   two questions "what is this screen for?" and "what would you do first?";
   a mismatch with the intended purpose or first action is a finding.
   *Second rater* (PURE needs ≥ 2): the persona, goal, ordered screenshots
   with one-line action captions and the criteria table; report both
   ratings with the reason per disagreement, score with the worse one, send
   a gap of 2 to a third fresh rater, and merge its new problems tagged
   `second-look`. No subagent tool → say so; S2 and PURE are `single-rater`.
8. **Verify** each finding: screenshot + region, quoted text, count or repro;
   re-open the shot for every severity ≥ 3. Unevidenced → `question`. Name the
   bar row, the owner layer (UI / backend / content / design authority; root
   cause first when two) and,
   with source access, file:line; reproduce the cause with the data (row,
   response, console error). Route a backend or content cause to its owner;
   never propose a UI patch over it. Rank by user effort or stall, not by how
   easy it is to fix.

## Criteria and thresholds
One verdict per lens, `pass` · `fail` · `not measured`, never "partly" and
never its own letter grade: a lens fails when any of its bar rows fails, on
source or runtime evidence alike, and lists the rows it could not measure.
Every number recorded in a Map table is judged against its row; a count
without a verdict is not a measurement. Numbers
are the [UI bar](../shared/ui-bar.md)'s unless the design authority sets
its own.

| lens | bar rows | measured by | pass |
|------|----------|-------------|------|
| Flows | J1–J4 | journey verdicts, step ratio, screen switches, lifecycle table | every flow PASS, ratio ≤ 1.5, no state without an exit, no `missing` job |
| Overlap | O1–O3 | ownership table, screen pairs, terms per concept | 0 duplicates, 0 contradictions |
| Simple | S1–S4 | per-screen counts, cold read | counts within the bar, every cold read matches |
| Text | T1–T4 | per-screen counts at both widths | 0 / 0 / 0, nothing over-cut |
| System | C1–C5 | states, geometry and drift tables | 1 idiom per job, 1 value per role |
| Speed | P1–P5 | vitals, request log, frames | every primary screen and main action within the bar |
| Data | D1–D3 | production-shape shots | no crash, raw number or flooded list |
| Truth | — | ground-truth table | 0 hidden facts, 0 false statuses |
| Access | WCAG 2.2 AA | text contrast ≥ 4.5:1 (large/UI ≥ 3:1, SC 1.4.3/1.4.11), targets ≥ 24×24 CSS px (2.5.8), focus visible (2.4.7): `agent-browser a11y` per screen, then pixels/frames for what axe cannot see | 0 failures |

Per walked flow (PURE, Rohrer et al.):

| metric | how measured | pass |
|--------|--------------|------|
| PURE step rating | 1 easy/known pattern · 2 some cognitive load · 3 hard, some target users fail here; a step that is easy to take but leads to the wrong result is a 3 | no step = 3 |
| PURE task score | sum of step ratings; color = worst step | green/yellow |
| Completion | unaided · with workaround (n) · failed | unaided |
| Recovery | each non-happy pass: unaided · with help · not | all unaided |
| Jargon | distinct on-screen terms the persona can't decode (ids, paths, English in a localized UI, template leaks) | 0 |

Finding severity (Nielsen 0–4, judged on frequency × impact × persistence):
4 catastrophe (blocks, loses data, misleads into harm) · 3 major (needs a
workaround or causes real confusion) · 2 minor (hesitation, inconsistency) ·
1 cosmetic. A drift or overlap class counts once at the severity of its worst
instance, with its instance count. **Grade**: A = every lens passes, max
severity ≤ 1 · B = max 2 · C = any 3 or a failed lens · F = any 4, a PURE red
task or a `missing` high-frequency job. A lens that could not be measured is
`not measured` with why, never a pass.

## Report (`<out>/report.md`)
Use [report-template](references/report-template.md). It opens with an owner
summary that stands alone: one plain sentence per lens, then the worst
problems, each as what the user has to do today → what it should be, with its
screenshot. Tables follow; full Map tables stay in `map/`. Update the
flow's `last audit` cell.

Fix themes group findings by shared cause, in this order; a later rung is
proposed only when the earlier ones cannot remove the problem:
1. **Cut** the duplicate, the region no job uses, the text.
2. **Merge** two screens, tabs or steps into the one owner.
3. **Reuse** the shared component in place of the local idiom.
4. **Rework** layout or wording in place.
5. **Add** a screen, control or copy: last, and only for a `missing` job or
   the shared component a Reuse needs and the project lacks.

The rungs order the cure for one problem, not the themes: themes are
ranked by user effort or stall. Every failed lens ends in at least one
theme, and a system-wide drift is a theme of its own (the rule, then the
migration), never "out of scope". Each theme is a ready `autopilot` requirement whose acceptance is the bar row
and the number this audit measured ("O1: overview no longer repeats the five
result tiles; ownership table 0 duplicates"). A drift class with two or more
instances also proposes the rule for the project's mechanical UI check (C5),
and a rule the design authority lacks is written out for it.

## Re-audit
Same scope, passes and measurements on the new build; link the previous report and
mark each old finding `fixed` / `still` / `regressed`. Lead with before/after
pairs: one per finding, same screen, viewport and data, the measured number
beside each. On live data that moves between passes, compare whether the
defect is present (a count of its instances, a ratio), not the totals. Tests or a build never confirm a UX fix; only re-audit
screenshots and numbers do. When a prototype exists, add a side-by-side
prototype-vs-real screenshot per state, non-happy states included
([checklists](references/checklists.md) § Prototype conformance). Reconcile
against the ledger so settled items are not re-reported.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: AUDITED(<grade>, sev4=<n> sev3=<n> sev2=<n> sev1=<n>) | INVENTORIED(<n>)
REPORT: <report path; inventory path>
SUMMARY: <scope, persona, failed lenses with their worst number, flows PASS/FRICTION/BLOCKED, PURE score/color, top findings>
NEXT: <copy-paste /bbs:autopilot "<fix theme>" in cut-first order, or the next flow>
CLEANUP: <session closed, PIDs stopped, secrets deleted; mutations made>
```
An audit without screenshots you looked at is `BLOCKED`, not `DONE`; so is one
with a browser or secret file of yours left behind. A product audit without
the Map, or with a lens `not measured`, is at best `DONE_WITH_CONCERNS`.

Sources: PURE — Rohrer et al., CHI 2016 (measuringu.com/pure); Nielsen
severity ratings and response-time limits (nngroup.com); Core Web Vitals
(web.dev/vitals); WCAG 2.2. Other thresholds are 2build defaults in the UI
bar; a project's design authority may set its own.
