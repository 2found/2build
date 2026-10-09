---
name: design-ui
description: Design and iterate a reviewable UI using the project's design authority and existing screens. Use for UI design requests or a workflow's design/prototype handoff; a direct build request can use the working UI as its prototype.
---
# design-ui
Make it really good, use lots of tokens and iterate till you're proud of it.

Own project continuity and a reviewable design handoff. Choose the design
approach from the brief, not a preset style or checklist.

Follow [preamble](../references/preamble.md) and the
[Auto-Decision Framework](../references/auto-decision-framework.md).
Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`.

## Project continuity
Read the brief (conversation or ticket), `AGENTS.md` and its linked architecture
and design docs. The declared design doc is authoritative wherever it lives.
Inspect actual components, tokens and the nearest screen; extending a page
means reviewing it in context and inheriting its local patterns.

Preserve the project's stack, library, content and asset constraints. For a new
product, derive a direction from the audience and purpose; missing brand rules
allow design judgment. Do not invent business facts or rebrand an existing UI.

Document reusable project decisions only when needed, in the existing design
authority or a concise `DESIGN.md` linked from `AGENTS.md`. No mandatory schema
or exhaustive inventory.

## Make and iterate
Deliver a preview the user can open. A design-only phase keeps it isolated
from production navigation/state; a direct build request uses the requested
app, without a duplicate throwaway UI. Show existing-page changes in context.

Use `browse` or the caller's browser tooling: render, inspect screenshots at
relevant widths, exercise the main journey and a relevant failure/empty state,
fix what falls short, and recheck. Honor requested build/verification commands.
A build is not visual evidence; report blocked inspection. Keep previews local
unless publishing is requested.

## Handoff
Record only decisions the UI cannot convey, deliberate departures from project
rules, unresolved facts and links to the preview/verification evidence.
With a ticket, resolve `bbs ticket path design --write`, write the note there,
then `bbs ticket set-pointer design <path>`. Otherwise an inline note suffices.

The caller owns review checkpoints; neither add approval rounds nor skip held
ones. Preserve the reviewed UI as the implementation baseline for comparison.
Do not commit, push or deploy as part of this skill.

```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: DESIGNED
PROTOTYPE: <path/URL and how to open; or why unavailable>
VERIFY: <rendered views, exercised flows, checks and gaps>
SUMMARY: <project decisions and design note location>
NEXT: <caller's next step or unresolved prerequisite>
```
A text-only spec is not a completed visual design.
