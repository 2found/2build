# Audit checklists
Run the ones that apply to the target. Each item is a finding when it fails.
Row ids (J1, O1, C2 …) are the [UI bar](../references/ui-bar.md)'s; it owns
the rules and numbers, this file owns how to check them.

## Ground truth first
Before the walk, list the facts the backend knows, from the CLI, DB, API or
logs the owner already trusts (read-only): counts, pending work, failed runs,
spend vs cap, items waiting on a human. Table: `| fact | source | seen in UI? where |`.
In the same pass list each object type and every state the backend can put it
in (enums, status columns, error codes); that is the lifecycle table's input.
- Hidden work: a fact with no card, badge or row anywhere is a top finding.
- Contradiction: one number differing between screens (header badge vs card
  count, "0 / 250k" while another screen shows spend, a stale snapshot shown as
  live). Label snapshot age or show "not measured", never a measured-looking 0.
- False status: "done" on a run that failed or spent tokens and filed nothing;
  "running soon" on a stopped item; "almost out" at 136%.
- Cleared state: a resolved item must leave (repeat error rows, badges).
- Unnamed cause: for each distinct error or status code in the data, the UI
  names the cause in the user's words and the exit it offers can work. "No
  reason recorded" over a recorded reason, or a retry that must fail again,
  is a top finding.
- Phantom rows: one backend thing listed twice (a local placeholder beside the
  real record, a stale copy). The UI shows each thing once.

## Map (the screens side by side)
Shots under `shots/map/`, one table per file, `map/<name>.md`; the report
quotes only the failing rows. Count and measure; a cell you could not fill
says why. The tag says what can answer a table: `source` tables need no
running app, `runtime` tables need the driver.

**Screens** (`screens.md`; source + runtime).
`| screen | reached from | shown when | flow ids it serves | primary action | regions |`.
Enumerate from the UI (navigation, tabs, sheets, command palette, deep links)
and from the route or registry source when available. A screen the source
has and the UI cannot reach, or the reverse, is a finding. A screen that
appears only under a condition (role, plan, item count) is listed with it
and audited on both sides. A screen or region with no flow id is a cut
candidate (J1).

**Ownership** (`ownership.md`; source + runtime). Rows: every fact (number,
list, status, chart) and action (create, approve, retry, configure) the
screens show. Columns: the screens. Cell: `owner` · `summary → link` · `copy`
· `copy + controls` · `–`, each with the label it carries there. The owner
is the screen the design authority names, else the screen of the flow whose
job it is; no clear owner is itself a finding for the design authority. The
owner's component mounted again on another screen is still a copy. More than
one non-summary cell in a row is an overlap finding (O1); values that differ
along a row are a contradiction, severity ≥ 3, provable only at runtime.
Screen or tab pairs sharing most of their rows are merge candidates (O2);
differing labels or icons along a row fail O3.

**Lifecycle** (`lifecycle.md`; source + runtime). Rows: object type × state from the ground-truth pass. Columns:
where the state is shown, the exit offered there. An empty cell is a J3
finding, and so is an object the user can create but not edit, stop or
delete. This is how "there is no way to finish it" is found without luck.

**States** (`states.md`; runtime). Rows: screens. Columns: loading · empty · error · partial failure
· slow (> 1 s). Force each one: `network route <url> --abort` or
`set offline on` for errors, an empty fixture or filter for empty, a video of
a cold open (`record start` / `stop`) or a delaying proxy for loading. Where
a reload drops the session, cold means the first visit to the screen in a
fresh session reached through the UI. Cell:
the idiom seen (component name from source, else a description) and whether
content jumped when data landed. A column with more than one idiom, a blank
or stuck screen, an error drawn as "nothing here", or a full-page spinner
where sibling screens use a skeleton is a C2 finding. A state that cannot be
forced is `not reached`, with why. Intercepting a response in the audit's
own browser (abort, delay, mock body) changes nothing on the server and is
allowed on a read-only target; a state only a write can create needs the
snapshot copy or stays `not reached`.

**Geometry** (`geometry.md`; runtime). Per screen, the computed value of each role: page gutter,
header height, title size and weight, section gap, card padding, row height,
control height, radius ([helpers](helpers.md) § geo). One row per role; more
than one value, or a value off the project's scale, is a C3 finding that
names the outlier screens.

**Complexity and text** (`complexity.md`; runtime). Per screen, first viewport, at the common width and
~390px (or the narrowest supported): primary-styled controls, interactive
controls, regions, tabs, visible row actions, words, text lines, rows over 2
lines, helper paragraphs, repeated facts, and where the work surface starts
as a share of the viewport height. Words are whitespace-separated tokens of
the main content area's rendered text (`innerText`) and lines its non-empty
rendered lines, whole screen. Product-wide: navigation entries, greatest
depth, distinct domain terms. Each row ends in pass/fail for S1, S3, S4 and
T1–T3, names the regions no flow uses (J1), and flags any screen at more
than twice the product median: outliers are where users give up.

**Speed** (`speed.md`; runtime). Per primary screen, cold and warm, with production-shaped volume:
`vitals --json` (LCP, CLS, INP), time to usable content, the request log of
one open (`network requests`: duplicates, unbounded lists, results never
shown) and what back-navigation does. Per main action: time to first
feedback and to done. With React, `react renders start` / `stop` around one
interaction shows render storms. Compare with P1–P5. Time spent in the
backend goes to `perf-audit` with the numbers; the finding here is what the
user waits through and sees meanwhile.

**Drift** (`drift.md`; source, read-only): what a screenshot cannot show.
`| job | idioms with call-site counts | off-idiom sites file:line | enforced by a check? yes/no/partly |`.
A call site is one rendered element (a JSX or template tag), not an import or
a line; an idiom is a component identity, not a prop variant. A source-only
finding is severity ≤ 2 unless the code shows a wrong state reaching the user.
- For each job in C1, and any other job two screens both do, list every
  component and piece of ad-hoc markup that serves it and count call sites:
  the shared component, bare primitives (a raw skeleton, spinner or loader
  icon), literal loading/empty/error copy. More than one idiom per job is a
  finding carrying the counts and, as file:line, the sites that are not the
  component the design authority names. When it names none, or two, that is
  the finding (owner: design authority): list every idiom and propose the one.
- Local look-alikes (C4): a component in a feature folder doing a shared
  component's job, a wrapper that restyles one, selectors into a library's
  internals.
- Off-scale values (C3, C4): arbitrary-value utilities, literal colours,
  sizes and radii, one-off spacing. A project with no stated scale for type
  or spacing is a design-authority finding: report the de facto values with
  their counts as the scale to adopt.
- Authority vs code: for each rule the design authority states that names a
  component, a number or a ban, count the violations and check whether a
  lint, build step or test that fails the build enforces it. Broken and
  unenforced → propose the rule (C5). A job that drifts because the authority
  never named its component or value is a finding owned by the design
  authority, with the sentence to add. So is an authority that states a rule
  twice, contradicts itself or keeps history inline: agents skim it and each
  screen follows a different reading.

## Dead ends
- A banner, message or error naming a button, section, card or page this
  view does not render (check every fix's new copy for it too).
- A button whose action differs from its label (an "acknowledge" that wakes a
  model, "Retry" against an exhausted budget with no warning).
- A path, link or deep link rendered as plain text.
- Raw id, enum, cron, env name, JSON or a tool dump shown to the user; a
  language other than the product's (model answer, prompt shown as a user bubble).
- A step that tells the user to ask someone else when the user is the only one.
- Same label for actions with different cost or risk (free tool run vs model run).
- Recovery that does not recover: "Retry" after a crash, a reload that drops the
  connection.
- Empty-state copy that points at a control the screen lacks.

## Discoverability of user needs
For each recurring need ("run this now", "see this week's numbers", "fix the
missing key"): where would the user look first? Count clicks from there. Check:
- The need is reachable from the place the user thinks it lives, not only from
  a list they would have to know.
- Search / command palette finds it by the on-screen title and by the id.
- Long lists have search or filter; titles are not truncated into duplicates.
- A missing prerequisite links to the exact field that fixes it.

## Non-happy paths (cover every reachable one)
Missing credential or key · over budget (single owner, team, company; reset
time shown) · remote or unreachable dependency · empty data · failed background
run (is it surfaced, can it be retried, does retry cost?) · crash and recovery
(error boundary, retry, reload) · long names (80 chars) · null or missing
measure · large raw number · the longest real list · narrow width · stale
state after another action completes.

## Information density (T1–T4)
Users came to do a task, not to read. The counts come from the Map's
complexity-and-text table; each hit is a finding (usually sev 2; sev 3 when
text pushes the work surface below the fold). Also count: titles over ~6
words, a "Select all" / "Select none" text pair where one tri-state checkbox
with an n/total count does the job.

Fix, in order of preference: cut the text; merge it into the row's one meta
line; turn a count/fact into a muted 16px icon + number whose full sentence is
its tooltip and accessible name (focusable); move the explanation (what a group
does, what it costs, what the icons mean) into ONE ⓘ popover beside the group
title, or a guide link; split a crowded screen into regions so position
carries what a sentence was explaining. Give status dots and row icons one
shared leading column so they align (misaligned leading icons are a real
defect).

Icon facts must pass: accessible name present (aria-label = the full
sentence); tooltip opens on keyboard focus; target >= 44px on narrow screens.
Hover tooltips do not open on touch, so on mobile the ⓘ popover/legend carries
the meaning. Check no horizontal scroll at 390px. Re-audit with before/after
screenshots at both widths and the line/word counts.

## Agent path
The product is also run by agents (`../references/agent-operable.md`). For each
action and each "needs attention" fact the walk used, check read-only:
- The action exists outside the UI (API or CLI) behind the same permission. A
  UI-only operator action is a finding (sev 2; sev 3 when it is a recurring job).
- The fact is served by a read endpoint, with the same number the screen shows.
- An API error is a status code plus `code` and recovery, not an HTML page or a
  `200`.
- Public facts the UI shows (prices, plans, limits) match `/llms.txt` and its
  pages.
Owner layer is backend or content; never propose a UI patch for these.

## Embedded pages and iframes
- Frame is sized to its content or the pane (no fixed min-height box with
  blank space; wheel and keyboard both scroll).
- Links inside open the intended target; `app://`-style links render as buttons.
- Dev or alternate origin gates: record "blocked here", do not bypass.
- Drive with the re-resolving helper in `helpers.md`.

## Cause layer
Decide where each symptom originates before assigning a fix:
- UI: rendering, wiring, copy, missing control.
- Backend/runtime: state written before the thing it guards exists (lost
  item), a refused job never resumed, a spend not attributed, a tool null
  field, a restart that locks work.
- Content/pack/config: language, prompts, descriptions, tool classification.
- Design authority: the rule is missing, ambiguous or unenforced, so each
  screen answered it differently. The fix is the rule and its mechanical
  check, then the screens.
Route the finding to that owner; do not patch the UI over a backend cause.
Check by reproducing with the data (DB row, API response, console error).

## Prototype conformance (after fixes)
For each prototype state id, capture prototype and real UI side by side at the
same viewport, plus the non-happy states, a long-name case and narrow width.
Per-state verdict: match / fixed / mismatch. Never edit the prototype to match
the build. Show the side-by-side index; tests and builds are not proof.
