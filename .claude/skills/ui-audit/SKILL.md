---
name: ui-audit
description: Evaluate a running app's UI/UX by using it as a named persona through one real work flow, scored with PURE step ratings, Nielsen severities and measured thresholds; also builds the app's flow inventory. Use before planning UI improvements or to re-audit a flow after a fix.
---
# ui-audit
Make it really good, use lots of tokens and iterate till you're proud of it.

Follow [preamble](../references/preamble.md) and the
[Auto-Decision Framework](../references/auto-decision-framework.md). Shared
refs are filesystem paths beside this skill's directory, so read them by path, not as `skill://`.
Evaluate and report only; fixes go to `plan-draft` / `design-ui` / `implement`.

## Agent blind spots (the reason this skill exists)
An agent driving a UI to finish a task stops seeing it. Hard rules:
- **Judge from pixels.** Open every screenshot you take. The accessibility
  tree/DOM proves existence, not visibility: present-in-tree but 0-sized,
  clipped, covered or off-screen is a finding, never a click target. Use the
  tree only to *measure* (frames, roles, names).
- **Persona knowledge only.** Anything the persona would not know that you
  used — source, CLI, logs, URLs, API, tree search, element index, a guess —
  is a `workaround` row. Every retry (misclick, lost focus, input that did
  not land) is logged before retrying.
- **Success is not a verdict.** The score comes from the step ratings below.
- **Second evaluator.** PURE needs ≥ 2 independent raters: a fresh subagent
  rates the same screenshots without seeing your ratings (*Procedure §4*).

## Inputs
Target = the build users actually run (record version/commit). Personas from
the brief, `AGENTS.md`, design authority or product docs; none → derive from
the stated audience and say so. Driver: `browse` for web, the caller's GUI
driver for desktop; real input only, no state-changing `eval`/API calls
inside the flow. Fixture records are named `audit-…` and listed in the report;
never touch data not designated for the audit.

## Flow inventory
One file per product beside its design authority (e.g. `docs/ux/flows.md`).
Rows come from the persona's recurring jobs, not the menu:

| id | flow | persona | trigger → done when | entry | freq | risk | ideal steps | last audit |

`ideal steps` = fewest deliberate actions a perfect UI needs (the step-ratio
baseline). Order by freq × risk. A job the product cannot do is a row marked
`missing`. Every primary screen should belong to ≥ 1 flow.

## Procedure
Output dir: ticket dir, else `.babysit/ui-audit/<date>-<flow-id>/`;
screenshots `shots/NN-<what>.png`.
1. **Frame** before opening the app: persona, goal, done-when, expected entry,
   ideal path.
2. **Happy path**, one row per deliberate action: expectation → screenshot →
   what the screen asks of the persona → act → screenshot → result. Rate the
   step (PURE) and record waits.
3. **Non-happy passes** (all that are reachable): empty/invalid/duplicate
   input; abandon mid-way with unsaved changes and re-enter; dependency down
   or slow; first-run empty state; narrowest supported width; other color
   scheme; keyboard-only main action.
4. **Second rater**: give a fresh subagent the persona, goal, ordered
   screenshots with one-line action captions and the criteria table; not your
   ratings or findings. Reconcile step ratings (note reason per disagreement);
   merge its new problems tagged `second-look`.
5. **Verify** each finding: screenshot + region, quoted text or repro; re-open
   the shot for every severity ≥ 3. Unevidenced → `question`. With source
   access, name the owning shared component/rule so the fix targets the cause.

## Criteria and thresholds
| metric | how measured | pass |
|--------|--------------|------|
| PURE step rating | 1 easy/known pattern · 2 some cognitive load · 3 hard, some target users fail here | no step = 3 |
| PURE task score | sum of step ratings; color = worst step | green/yellow |
| Step ratio | actual deliberate actions ÷ ideal steps | ≤ 1.5 |
| Depth | max layers/screens from the flow's start surface | ≤ 2 |
| Response | waits > 1 s without visible feedback; > 10 s without progress | 0 / 0 |
| Completion | unaided · with workaround (n) · failed | unaided |
| Recovery | each non-happy pass: unaided · with help · not | all unaided |
| Jargon | distinct on-screen terms the persona can't decode (ids, paths, English in a localized UI, template leaks) | 0 |
| WCAG 2.2 AA | text contrast ≥ 4.5:1 (large/UI ≥ 3:1, SC 1.4.3/1.4.11), targets ≥ 24×24 CSS px (2.5.8), focus visible (2.4.7) — measured from pixels/frames, not eyeballed | 0 failures |

Finding severity (Nielsen 0–4, judged on frequency × impact × persistence):
4 catastrophe (blocks, loses data, misleads into harm) · 3 major (needs a
workaround or causes real confusion) · 2 minor (hesitation, inconsistency) ·
1 cosmetic. **Grade**: A = all metrics pass, max severity ≤ 1 · B = max 2 ·
C = any 3 or a failed metric · F = any 4 or a PURE red task.

## Report (`<out>/report.md`)
Header: build, date, driver, persona, fixtures. Then: Frame · Scorecard (every
metric above, value vs pass, grade) · Step log `| # | expectation | observed |
PURE (you/2nd) | wait s | result | shot |` · Findings sorted by severity
(`F<n> [sev] claim — where (shot) — quoted evidence — why it hurts the
persona — cause file:line|unknown — fix direction — tags`) · What works (keep
when fixing) · Questions · Fix themes (findings grouped by shared cause,
impact ÷ effort). Update the flow's `last audit` cell.

## Re-audit
Same flow, passes and metrics on the new build; link the previous report and
mark each old finding `fixed` / `still` / `regressed` with a new shot. Tests
or a build never confirm a UX fix — only re-audit screenshots do.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: AUDITED(<grade>, PURE <score>/<color>, sev4=<n> sev3=<n> sev2=<n> sev1=<n>) | INVENTORIED(<n>)
REPORT: <report path; inventory path>
SUMMARY: <persona, flow, failed metrics, top findings, second-look count>
NEXT: <fix themes for plan-draft/design-ui/implement, or next flow>
```
An audit without screenshots you looked at is `BLOCKED`, not `DONE`.

Sources: PURE — Rohrer et al., CHI 2016 (measuringu.com/pure); Nielsen
severity ratings and response-time limits (nngroup.com); WCAG 2.2. Step
ratio and depth thresholds are 2build defaults; a project's design authority
may set its own.
