---
name: ui-audit
description: Evaluate a running app's UI/UX by using it as a named persona through one real work flow — look at every screen before acting, log every friction and workaround, score the flow, and hand back evidence-backed findings ranked by severity with fix directions. Use before planning UI improvements, to inventory an app's work flows, or to re-audit a flow after a fix.
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
This skill **evaluates and reports**. It changes no product code; a caller
(or the user) turns the report into `plan-draft` / `design-ui` / `implement`.

## Why this skill exists
An agent that drives a UI to get a task done stops seeing it: it reads the
accessibility tree, clicks by element index, retries silently, reads source to
find a button, and reports the task "done". A human watching sees a popup with
0px height, English jargon, a dead end and four buttons that do one thing.
Every rule below exists to break that **operator blindness**:

1. **Pixels before the tree.** The accessibility tree / DOM proves an element
   *exists*, not that a person can see it. Judge only from screenshots you
   actually open and look at. An element present in the tree but absent,
   clipped, covered or 0-sized on screen is a finding, not a click target.
2. **Persona knowledge only.** Act with what the persona knows: visible
   labels, position, common app conventions. Every time you use knowledge the
   persona lacks (source code, CLI, logs, URLs, API, element search, a guess
   that happened to work) write a **workaround** row. Workarounds are findings.
3. **Doing ≠ judging.** Completing the task is not evidence the UX is good.
   The flow's verdict comes from the friction log, not from success.
4. **Every retry is a finding.** Misclick, focus lost, input not landing,
   double submit, waiting without feedback — log it before trying again.
5. **Look twice.** After your own pass, a fresh reviewer that has not seen your
   findings looks at the same screenshots (see *Second look*). What it finds
   that you missed shows where you were blind; fold it in.

## Inputs
- **Target**: the build real users run (installed desktop app, deployed or
  local web URL). Record the version/commit; a stale build voids the audit.
- **Personas**: from the brief, `AGENTS.md`, design authority or product docs.
  Each has role, tech comfort, vocabulary, what they want, what scares them.
  No persona anywhere → derive one from the product's stated audience and
  say so in the report; do not audit as "a developer".
- **Flow**: one entry from the flow inventory. No inventory yet → build it
  first (*Flow inventory*), then audit its top-priority flow.
- **Driver**: `browse` (agent-browser) for web; for native/webview desktop
  apps, the caller's GUI driver. Real pointer and keyboard input only; no
  scripted `click()`, no `eval` that changes state, no direct API calls
  inside the audited flow (seeding fixtures *before* the flow is fine and is
  noted in the report).

Never touch production data the user did not designate for the audit. Use
throwaway records with an obvious name (`audit-…`) and list them in the
report so they can be removed.

## Flow inventory
Write it once per product and keep it current; it is the audit backlog.
Location: the project's docs beside its design authority (e.g.
`docs/ux/flows.md`), else the ticket dir. One row per flow, **from the
persona's job, not the app's menu**:

| id | flow | persona | trigger → done when | entry | freq | risk | ideal steps | last audit |
|----|------|---------|---------------------|-------|------|------|-------------|------------|

- *trigger → done when*: the real-world reason and the observable finish
  ("a post draft is waiting for my approval"), never "opened screen X".
- *ideal steps*: the fewest deliberate actions a perfect UI would need;
  becomes the baseline for the step ratio.
- Order by `freq × risk`; first-run setup and anything touching money, data
  loss or publishing ranks high. Mark flows the product cannot do at all —
  a missing capability is a finding of its own.
Find flows by reading the product brief/docs, walking every navigation
entry, and asking for each persona "what do they come here to get done each
week?" Cross-check that every primary screen belongs to at least one flow;
an orphan screen is itself worth a note.

## Audit procedure (one flow)
Work in `<out>/` = ticket dir or `.babysit/ui-audit/<YYYY-MM-DD>-<flow-id>/`
(or a caller-supplied path). Screenshots go in `<out>/shots/NN-<what>.png`.

### 0. Frame (before touching the app)
Write in the report: persona, goal, done-when, where the persona would
expect to start, and the ideal path. This is the expectation every
observation is compared against.

### 1. Walk the happy path — per step: EXPECT → LOOK → FIND → ACT → FEEL
- **EXPECT** one line: what the persona wants now and what they expect to see.
- **LOOK**: screenshot, open it, then write *before judging*: where the eye
  lands first, what the screen asks you to do, the primary action, every
  visible word the persona would not understand. Run the
  [lenses](lenses.md). Crop/zoom dense regions; read small print.
- **FIND** the next control the way the persona would. Needed scrolling,
  hovering, guessing, tree search or a second try → friction row.
- **ACT** with real input. Note the wait.
- **FEEL**: screenshot after. Did something visibly respond within ~1 s? Is it
  obvious what happened and what comes next? Did anything jump, vanish,
  reset, or lose focus? Does the result match EXPECT?
- **LOG** a friction-log row (template below) for every step, including the
  ones that went fine (`ok`), so step counts are real.

### 2. Break it — at least these non-happy passes
- **Wrong/empty input** at the flow's main form (empty submit, too long,
  invalid, duplicate name).
- **Abandon**: cancel, Escape, back, close mid-way with unsaved changes;
  then re-enter — is state kept or lost as the persona would expect?
- **Dependency down or slow** when reachable without harming anything
  (offline service, provider error, no data yet): does the message say what
  happened, whether anything is lost, and the one next step — calmly?
- **First run / empty state** of each screen in the flow, if reachable.
- **Environment**: narrowest supported window/viewport, the other color
  scheme, and keyboard-only for the main action. Screenshot each.

### 3. Sweep the surroundings
On every screen the flow touched, also judge what the persona *passes by*:
stale or contradictory numbers, test/junk records, duplicate names, items
that claim urgency but are not actionable, raw ids. They shape trust even
when off-task.

### 4. Second look (independent)
Spawn one fresh reviewer (subagent) with: the persona, the goal, the ordered
screenshot paths with one-line captions of the action taken, and the lenses
file. Not your findings, not your verdict. Ask for every problem it sees per
screenshot with severity. Merge: new valid items become findings tagged
`second-look`; record how many there were — a high count means redo the
LOOK steps more slowly on the next flow. No subagent capability → re-review
your own screenshots in reverse order after a break from the flow, and say so.

### 5. Verify findings
Each finding needs evidence a stranger can check: screenshot path + region,
exact on-screen text quoted, or a reproduction. Re-open the screenshot for
every S0/S1 to confirm it. Drop anything you cannot evidence; mark
unconfirmed suspicions as `question`, not findings. When the source is
available, find the **owning component or rule** for each confirmed finding
(shared component, copy source, layout primitive) — fix directions target the
shared cause, not the one screen.

## Severity and scoring
| sev | meaning |
|-----|---------|
| S0 blocker | persona cannot finish, loses data, or is misled into a harmful action |
| S1 major | finishes only with a workaround, help, or real confusion; trust-damaging error/copy |
| S2 minor | slows, makes them hesitate or re-read; inconsistency with the rest of the app |
| S3 polish | visual/copy nit a careful user notices |

Flow scorecard (all measured from the friction log, not estimated):
- **Completion**: unaided · with workaround (count) · failed
- **Step ratio**: actual deliberate actions ÷ ideal steps
- **Depth**: max screens/layers away from the starting surface
- **Waits**: count of waits > 1 s without visible progress
- **Words**: count of on-screen terms the persona would not understand
- **Recovery**: for each non-happy pass, recovered unaided / with help / not
- **Ease (1–7)** for the persona, with a one-sentence justification
- **Grade**: A (≤ S3 only) · B (S2s, no S1) · C (any S1) · F (any S0)

## Report (`<out>/report.md`)
```text
# UI audit — <flow id> <flow name>
Build: <version/commit, how installed> · Date · Driver · Persona(s)
Fixtures created: <records + how to remove> | none

## Frame            persona, goal, done-when, expected entry, ideal path
## Scorecard        the metrics above + grade
## Friction log     | # | expect | observed | wait | result(ok/friction/workaround/fail) | shot |
## Findings         sorted S0→S3:
  ### F<n> [S1] <short claim>
  Where: screen › region (shot NN) · Seen: "<quoted text>" / what is visible
  Why it hurts <persona>: …
  Cause: <owning component/rule, file:line when known> | unknown
  Fix direction: <shared-cause change, one or two sentences>
  Tags: lens, second-look?, also-affects <other flows>
## What works        things to keep when fixing (so a fix doesn't regress them)
## Questions         unconfirmed suspicions, missing facts
## Fix themes        findings grouped by shared cause, ordered by impact ÷ effort
```
Update the flow's `last audit` cell (date, grade, report link) in the
inventory.

## Re-audit
After a fix, rerun the same flow and passes against the new build, link the
previous report, and mark every earlier finding `fixed` / `still` /
`regressed` with a new screenshot. New findings get new ids. A fix is not
confirmed by tests or a build — only by the re-audit screenshots.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: AUDITED(<grade>, S0=<n> S1=<n> S2=<n> S3=<n>) | INVENTORIED(<n flows>)
REPORT: <report path; inventory path>
SUMMARY: <persona, flow, the 1–3 findings that matter most, second-look count>
NEXT: <fix themes for plan-draft/design-ui/implement, or next flow to audit>
```
`BLOCKED` when the target cannot be driven at all (name what stops it); an
audit that could only read the tree without screenshots is `BLOCKED`, not
`DONE`.
