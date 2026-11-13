---
name: design-ui
description: Design and iterate a reviewable UI from the user's jobs, using the project's design authority and existing screens, to the shared UI bar (jobs that finish, one owner per feature, simple, scannable, one component system, fast). Use for UI design requests or a workflow's design/prototype handoff; a direct build request can use the working UI as its prototype.
---
# design-ui
Make it really good, use lots of tokens and iterate till you're proud of it.

Own project continuity and a reviewable design handoff. Visual direction comes
from the brief and the design authority, not a preset style. Usability is not
taste: the design meets the [UI bar](../references/ui-bar.md), whose row ids
(J1, O1, C2 …) are cited below.

Follow [preamble](../references/preamble.md) and the
[Auto-Decision Framework](../references/auto-decision-framework.md).
Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`.

## Project continuity
Read the brief (conversation or ticket), `AGENTS.md` and its linked architecture
and design docs. The declared design doc is authoritative wherever it lives.
Inspect actual components, tokens and the nearest screen; extending a page
means reviewing it in context and inheriting its local patterns. Read the
flow inventory beside the design authority (e.g. `docs/ux/flows.md`) and the
latest `ui-audit` report when one exists (ticket dir or `.babysit/ui-audit/`):
its open findings on the screens you touch are part of the brief.

Preserve the project's stack, library, content and asset constraints. For a new
product, derive a direction from the audience and purpose; missing brand rules
allow design judgment. Do not invent business facts or rebrand an existing UI.

Document reusable project decisions only when needed, in the existing design
authority or a concise `DESIGN.md` linked from `AGENTS.md`. No mandatory schema
or exhaustive inventory.

## Fit the product before drawing
A brief asks for an addition, and a product designed one addition at a time
ends with the same number on three screens and one job spread over four.
Settle these before any layout:
- **Jobs.** Write each job the change serves as trigger → done with its ideal
  steps, as rows of the flow inventory (create the file with these rows when
  it is missing). Design the path first, then the screens it needs (J1, J2).
- **Owner.** For each fact and action in the brief, find the screen that
  already owns it or its nearest relative, and extend that. A new screen, tab
  or navigation entry needs a job no existing screen can own, and names what
  it replaces (O1, O2).
- **Subtract.** List what the change makes redundant (a duplicate summary, a
  tab, a setting, a step) and remove it in the same design. "Nothing to
  remove" is stated with its reason.
- **Lifecycle.** Every state the backend can put the object in has a place
  and an exit (J3); read the states from the code, not the brief.
- **System.** Name the shared component for each job on the screen: page
  shell, list row, form footer, sheet, loading, empty, error (C1). No fit →
  propose a shared component or variant as a decision; never a local one-off
  (C4).

When the design authority is silent on something this depends on (who owns a
fact, which component is the loading state, the page gutter), decide it and
record it there in a line or two. That is what stops the next change
drifting.

## Make and iterate
Deliver a preview the user can open. A design-only phase keeps it isolated
from production navigation/state; a direct build request uses the requested
app, without a duplicate throwaway UI. Show existing-page changes in context.

Each changed screen is shown loaded, loading, empty, failed and partly failed,
built from the named shared components (C2), with production-shaped data: the
longest real list, a missing value, an 80-character name, a large number
(D1–D3). Never lorem or three tidy rows.

Use `browse` or the caller's browser tooling: render, inspect screenshots at
the common and the narrowest supported width, fix what falls short, and
recheck. On the rendered preview:
- Walk each job, counting actions and screen switches against its ideal
  steps (J2, J4), plus one relevant failure.
- Cold read (S2): a fresh subagent sees only the screenshot of each new or
  reshaped screen and answers "what is this for?" and "what would you do
  first?". No subagent tool → report S2 as unchecked.
- Count per changed screen: primary actions, tabs, row actions, rows over 2
  lines, helper paragraphs, repeated facts (S1, S3, S4, T1–T3). Prefer
  layout, position and icons to sentences.
- Lay it beside its two nearest sibling screens: header, gutter, rows and
  states match (C1–C3).
- Name what the screen fetches on open and what it shows before data lands
  (P1, P4).
- The other color scheme and `agent-browser a11y` on each screen.

`ui-audit`'s [checklists](../ui-audit/references/checklists.md) say how each
row is measured on the built UI. Honor requested build/verification commands.
A build is not visual evidence; report blocked inspection. Keep previews local
unless publishing is requested.

## Handoff
Record decisions the UI cannot convey, deliberate departures from project
rules, unresolved facts, links to the preview/verification evidence, and the
UX contract `implement` builds to and `qa` / `ui-audit` check the build
against:
```text
Jobs: <flow id: trigger → done · ideal steps · steps in this design>
Owner: <fact or action → owner screen> · removed or replaced: <what, or the reason for nothing>
Components: <job → shared component> · new: <name, why none fits, where recorded>
States: <screen → loading / empty / error / partial components>
Bar: <rows checked on the preview with their count or shot · rows unmet and why>
```
With a ticket, resolve `bbs ticket path design --write`, write the note there,
then `bbs ticket set-pointer design <path>`. Otherwise an inline note suffices.

The caller owns review checkpoints; neither add approval rounds nor skip held
ones. Preserve the reviewed UI as the implementation baseline for comparison.
Do not commit, push or deploy as part of this skill.

```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: DESIGNED
PROTOTYPE: <path/URL and how to open; or why unavailable>
VERIFY: <rendered views, walked jobs with step counts, bar rows checked and gaps>
SUMMARY: <what was removed or merged, project decisions and design note location>
NEXT: <caller's next step or unresolved prerequisite>
```
A text-only spec is not a completed visual design.
