---
workflow: sweeper
version: 1
description: Behavior-preserving simplification of an existing branch or area — establish a green baseline, shrink, re-verify.
needs-state:
  branch_pushed: optional
---
# sweeper
The Sweeper archetype (see `../shared/archetypes.md`). Use when working code
should get smaller or faster without changing behavior.
## run
> produces: verdict:sweeper + qa:checked
1. Read the target area, branch, and git diff. Run the existing tests /
   typecheck / build and record the green baseline.
2. Run `sweep`: apply focused, behavior-preserving cleanups (dead code,
   duplication, unused features, measured hot paths).
3. Run `review-pr --fix` in the current autopilot session via SKILL.md's
   Current-session gates (`review-pr`, `qa`) policy (applies fixes to the
   working tree), bracketed by
   `bbs autopilot verification begin --gate review-pr` and `record`.
4. Run `qa` in the same session (or the strongest equivalent fallback) and
   confirm behavior matches the recorded baseline, bracketed by
   `bbs autopilot verification begin --gate qa` and `record`.
5. Apply autopilot's **Repair until the final change passes** loop. Both gates
   must cover the same final tree, and
   `bbs ticket readiness --action review --json` must report `data.ready=true`.
6. Leave the result uncommitted. Autopilot never commits, pushes, or opens a
   PR — that is the human's (`create-pr`) or Foreman's phase landing.
7. Write a handoff: what shrank, net line delta, and baseline-still-green proof.
   When a signal warrants, name the forward lifecycle edge after `create-pr`:
   surface shipped and measurable → `grower`; scale/security/cost signal
   observed while sweeping → `maintainer`.
**Stop conditions**

- `NEEDS_CONTEXT`: no safety net exists and behavior cannot be characterized.
- `BLOCKED`: the baseline is red before changes, or a cleanup changes behavior
  and cannot be made behavior-preserving.
**Final status**
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: SWEPT
SUMMARY: <what shrank + line delta + QA evidence>
LIFECYCLE: sweeper -> grower | maintainer | stop
TRIGGER: <measurable product surface, operational signal, or no next-stage evidence>
NEXT: human review, then /bbs:create-pr
```
