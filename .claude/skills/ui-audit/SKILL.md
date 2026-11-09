---
name: ui-audit
description: Audit a running app's UI/UX by walking real user or founder journeys end to end (PASS/FRICTION/BLOCKED), checking every screen against backend ground truth, dead ends, discoverability and non-happy paths; scored with PURE step ratings, Nielsen severities and measured thresholds. Also builds the flow inventory and re-audits after a fix. Use before planning UI improvements, when users report getting stuck, or to audit a live app safely.
disable-model-invocation: true
---
# ui-audit
Make it really good, use lots of tokens and iterate till you're proud of it.

Follow [preamble](../references/preamble.md) and the
[Auto-Decision Framework](../references/auto-decision-framework.md). Shared
refs are filesystem paths beside this skill's directory, so read them by path, not as `skill://`.
Evaluate and report only; fixes go to `plan-draft` / `design-ui` / `implement`.
Browser setup, session and cleanup: the [`browse`](../browse/SKILL.md) skill
(`agent-browser`); do not duplicate it. Detail lives in `references/`:
[checklists](references/checklists.md) · [live-safety](references/live-safety.md)
· [helpers](references/helpers.md) · [report-template](references/report-template.md).

## Agent blind spots (the reason this skill exists)
An agent driving a UI to finish a task stops seeing it. Hard rules:
- **Judge visibility from pixels.** Open every screenshot you take. The
  accessibility tree/DOM proves existence, not visibility: present-in-tree but
  0-sized, clipped, covered or off-screen is a finding, never a click target.
  Walk and read text from snapshot text (cheap, exact quotes); screenshots are
  the evidence for layout, contrast and anything you rate.
- **The UI is not the truth.** Gather the backend facts first (counts, pending
  work, failures) and check the UI shows each one. Hidden work and numbers
  that contradict each other outrank every cosmetic finding.
- **A screen is not a journey.** The bar for every screen: does it show the
  next step, and does the journey reach its end?
- **Persona knowledge only.** Anything the persona would not know that you
  used — source, CLI, logs, URLs, API, tree search, element index, a guess —
  is a `workaround` row. Every retry (misclick, lost focus, input that did
  not land) is logged before retrying.
- **Success is not a verdict.** The score comes from the step ratings below.
- **Second evaluator.** PURE needs ≥ 2 independent raters: a fresh subagent
  rates the same screenshots without seeing your ratings (*Procedure §6*).

## Inputs
Target = the build users actually run (record version/commit). Personas from
the brief, `AGENTS.md`, design authority or product docs; none → derive from
the stated audience and say so. Driver: `browse` for web, the caller's GUI
driver for desktop; real input only, no state-changing `eval`/API calls
inside the flow. Fixture records are named `audit-…` and listed in the report;
never touch data not designated for the audit. Auditing production or a
founder's live data: follow [live-safety](references/live-safety.md) (read-only
default, mutation allowlist, secrets, isolated session, snapshot copy for any
mutation); an origin/auth gate is reported `blocked here`, never bypassed.

## Flow inventory
One file per product beside its design authority (e.g. `docs/ux/flows.md`).
Rows come from the persona's recurring jobs, not the menu (for a whole
product: the 8-15 journeys the owner lives in, e.g. morning "what needs me",
run a flow now, handle a failure, fix a missing credential):

| id | flow | persona | trigger → done when | entry | freq | risk | ideal steps | last audit |

`ideal steps` = fewest deliberate actions a perfect UI needs (the step-ratio
baseline). Order by freq × risk. A job the product cannot do is a row marked
`missing`. Every primary screen should belong to ≥ 1 flow.

## Procedure
Output dir: ticket dir, else `.babysit/ui-audit/<date>-<flow-id>/`;
screenshots `shots/NN-<what>.png`.
1. **Frame** before opening the app: persona, goal, done-when, expected entry,
   ideal path. Read the previous report and carry its known-findings ledger.
2. **Ground truth** (read-only): list backend facts and mark where the UI shows
   each ([checklists](references/checklists.md) § Ground truth).
3. **Happy path**, one row per deliberate action: expectation → screenshot →
   what the screen asks of the persona → act → screenshot → result. Rate the
   step (PURE), record waits and the exact clicks. Journey verdict: PASS
   (reaches its end) · FRICTION (reaches it with detours or missing next step) ·
   BLOCKED (cannot finish, or not testable here, say why).
4. **Sweeps**: dead ends, discoverability of user needs, iframes/embedded
   pages ([checklists](references/checklists.md)). Count clicks from where the
   user thinks the feature lives; try search/command palette.
5. **Non-happy passes** (all that are reachable; at least one before any
   PASS): missing credential, over budget, unreachable dependency, empty data,
   failed background run, crash and retry; invalid/duplicate input; abandon
   mid-way and re-enter; first-run empty state; narrowest width; other color
   scheme; keyboard-only main action.
6. **Second rater**: give a fresh subagent the persona, goal, ordered
   screenshots with one-line action captions and the criteria table; not your
   ratings or findings. Reconcile step ratings (note reason per disagreement);
   merge its new problems tagged `second-look`.
7. **Verify** each finding: screenshot + region, quoted text or repro; re-open
   the shot for every severity ≥ 3. Unevidenced → `question`. Name the owner
   layer (UI / backend / content) and, with source access, file:line; reproduce
   the cause with the data (row, response, console error). Route a backend or
   content cause to its owner; never propose a UI patch over it. Rank by user
   effort or stall, not by how easy it is to fix.

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
Use [report-template](references/report-template.md): header, frame, ground
truth, journey table, scorecard, step log, findings (id, severity, evidence,
fix, owner layer, file:line), known-findings ledger (confirmed / partial / not
reproduced / not tested), what works, **user-need UX proposal**, fix themes.
Bound it to about 120 lines. Update the flow's `last audit` cell.

## Re-audit
Same flow, passes and metrics on the new build; link the previous report and
mark each old finding `fixed` / `still` / `regressed` with a new shot. Tests
or a build never confirm a UX fix — only re-audit screenshots do. When a
prototype exists, add a side-by-side prototype-vs-real screenshot per state,
non-happy states included ([checklists](references/checklists.md) § Prototype
conformance). Reconcile against the ledger so settled items are not re-reported.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: AUDITED(<grade>, PURE <score>/<color>, sev4=<n> sev3=<n> sev2=<n> sev1=<n>) | INVENTORIED(<n>)
REPORT: <report path; inventory path>
SUMMARY: <persona, flow, failed metrics, top findings, second-look count>
NEXT: <fix themes for plan-draft/design-ui/implement, or next flow>
CLEANUP: <session closed, PIDs stopped, secrets deleted; mutations made>
```
An audit without screenshots you looked at is `BLOCKED`, not `DONE`; so is one
with a browser or secret file of yours left behind.

Sources: PURE — Rohrer et al., CHI 2016 (measuringu.com/pure); Nielsen
severity ratings and response-time limits (nngroup.com); WCAG 2.2. Step
ratio and depth thresholds are 2build defaults; a project's design authority
may set its own.
