# Audit checklists
Run the ones that apply to the target. Each item is a finding when it fails.

## Ground truth first
Before the walk, list the facts the backend knows, from the CLI, DB, API or
logs the owner already trusts (read-only): counts, pending work, failed runs,
spend vs cap, items waiting on a human. Table: `| fact | source | seen in UI? where |`.
- Hidden work: a fact with no card, badge or row anywhere is a top finding.
- Contradiction: one number differing between screens (header badge vs card
  count, "0 / 250k" while another screen shows spend, a stale snapshot shown as
  live). Label snapshot age or show "not measured", never a measured-looking 0.
- False status: "done" on a run that failed or spent tokens and filed nothing;
  "running soon" on a stopped item; "almost out" at 136%.
- Cleared state: a resolved item must leave (repeat error rows, badges).

## Dead ends
- A banner or message naming a button, card or page that does not exist here.
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
(error boundary, retry, reload) · long names (80 chars) · narrow width ·
stale state after another action completes.

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
Route the finding to that owner; do not patch the UI over a backend cause.
Check by reproducing with the data (DB row, API response, console error).

## Prototype conformance (after fixes)
For each prototype state id, capture prototype and real UI side by side at the
same viewport, plus the non-happy states, a long-name case and narrow width.
Per-state verdict: match / fixed / mismatch. Never edit the prototype to match
the build. Show the side-by-side index; tests and builds are not proof.
