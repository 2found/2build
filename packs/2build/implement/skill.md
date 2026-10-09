---
name: implement
description: Implement a scoped code change from the user's request, an accepted plan.md, or ticket context. Use for feature work, bug fixes, endpoints, UI changes, integrations, and contained refactors.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# implement
Build the smallest correct change; this file only sets the babysit-specific
guardrails.
- Read the request, plan, linked specs/manifest and nearby code before editing.
  Preserve the plan's accepted scope, contracts, invariants and acceptance
  checks; optional proposals are not work to implement. Derive files, task
  order and coding steps from the code within those boundaries. If the work
  collapses to ≤3 trivial doc/comment-only edits, downgrade `ticket_size` one
  tier using the downgrade hook in `../shared/ticket-size-rubric.md` (it writes the
  audit-log line). Shared refs (`../shared/*.md`) are filesystem paths
  beside this skill's directory, so read them by path, not as `skill://`.
- Read the plan's findings and unknowns as evidence to carry forward. Check
  relevant open items before dependent work; retain each material item's outcome
  and evidence in the handoff (confirmed, corrected, or still open with impact
  and next check). Do not re-investigate resolved findings without conflicting
  evidence. New discoveries follow `../shared/finding-unknowns.md`; preserve
  them durably, and record changed assumptions/decisions in `## Deviations`.
  An unresolved acceptance blocker prevents a completion claim.
- The plan file is the Claude Code plan: derive the native task list
  (TaskCreate) from `plan.md` — you own task order, one task per verifiable
  unit — and keep it live: in_progress when started, completed only after its
  check passes. A deviation updates the task list and `## Deviations`; never
  track the work in an ad-hoc list beside the plan.
- Reuse before writing: before creating any util, helper, or component, check
  the reuse notes in the plan's **Approach**, then grep shared/lib/util dirs and the
  nearest similar feature. A new shared util or abstraction is a plan
  decision, not an ad-hoc call.
- UI: read the design doc linked from AGENTS.md/CLAUDE.md at its declared
  path and inspect the actual component/token source. Reuse the project's
  components, tokens, spacing and interaction patterns. Read the linked design
  spec and inspect its prototype (from the plan or `pointers.design`); the accepted flows, states and appearance are the
  baseline. Build to it. No new one-off component, color, font size, or layout when the
  project has one. New user-facing surface with no design spec
  (`pointers.design` empty, nothing in conversation) → invoke the `design-ui`
  skill via the Skill tool first; never improvise a layout the human first
  sees after implementation.
- For changed UI, compare the rendered result and key flows/states against
  that prototype using `browse` or `qa` as appropriate; retain screenshots/check
  evidence and explain mismatches in the handoff. If comparison cannot run,
  name the missing verification. Do not claim visual conformance from a build
  alone or rewrite the accepted prototype to conceal implementation drift.
- API surfaces follow best practice by default, even when the plan is silent
  — an unpaginated list endpoint is a bug, not a simplification.
- Same at the DB layer: every new or changed query needs a bounded access
  path — check the declared indexes; cost should grow with rows returned,
  not table size. A query the schema can't serve is a bug — add the index,
  or log a denormalization/caching need as a deviation.
- When an edge case forces a deviation within the accepted direction: pick
  the smallest reversible change preserving the plan's intent, log it (see
  Ticket Mode), and keep going. If evidence invalidates a material design
  decision, invariant or scope boundary, record it and route through
  `../shared/auto-decision-framework.md` before dependent work; reversibility
  alone does not authorize a new direction. Never silently absorb a deviation.
- Never branch, commit, or push — leave the change in the working tree.
  Skills are infra-isolated; git belongs to the invoking workflow
  (autopilot) or the human.
- When planning verification, read [test](../test/skill.md) and use its
  `create` workflow scoped to this change: identify changed behavior, plausible
  failures, existing evidence, the lowest credible layer and whether another
  test is needed. Follow its failure-investigation rules; never weaken or skip
  a test to obtain green. It owns affected-case selection through
  `semantic-decision`; retain mandatory repo checks and regression reproducers.
  Whole-suite `audit` and `optimize` are not routine implement steps.
- Verify with the narrowest meaningful command (tests, typecheck, lint,
  build, or browser check) and summarize changed files, verification, and
  remaining risk. In the handoff, identify the tested code state (including
  uncommitted changes), commands, environment/prerequisites, selected lanes
  and results so QA can assess freshness and reuse valid automated evidence.
## Ticket Mode
When running inside babysit, read `requirement.md`, `plan.md`, and the checkpoint if present. Write concise handoff notes for what changed and how it was verified, plus a `## Deviations` section when any occurred — one entry each:
```
- **<short title>** — Plan said: <expectation> · Found: <reality — cite file/symbol> · Chose: <option> — because <one line>
```
`qa` seeds test cases from this section and the final handoff surfaces it.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: BUILT | FIXED | CHANGED
SUMMARY: <files changed + verification>
NEXT: <human next action or "none">
```
