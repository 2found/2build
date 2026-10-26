---
workflow: builder
version: 1
description: Turn one ticket's idea, requirement, or accepted plan into a production-grade, QA-verified change on the current checkout. The default path for serial product work and foreman-dispatched child tickets; multi-ticket parent orchestration belongs to foreman.
needs-state:
  requirement_md: optional
---
# builder
The default autopilot workflow. One workflow, several modes — pick the mode from durable state each time you resume, never
from conversation memory.
### mode selection
Re-read ticket state, checkpoint, and git status; choose the **first** match:

| Mode | Condition | What it does |
|------|-----------|--------------|
| **child** | `origin_type = sub_ticket` | One Foreman ticket: plan, implement, review and QA only this child's scope; hand off to the parent. |
| **implement** | `plan.md` exists, no manifest | Implement the accepted plan. |
| **build** | `requirement.md` exists, `plan.md` absent | Plan first, then implement. |
| **verify** | none of the above, but the checkout holds existing changes (uncommitted, or commits off base) | QA-only pass on existing work. |
A ticket with `manifest.md` is a multi-ticket project, not a builder mode:
stop with `BLOCKED` and hand it to `foreman`. If no mode matches and there is
no ticket/requirement, stop with `NEEDS_CONTEXT`.
## run
> produces: verdict:builder + qa:checked
1. Ensure ticket and checkpoint exist; record the mode in the checkpoint.
   Read these facts from `bbs autopilot snapshot --json`. Work happens on the current checkout —
   autopilot never cuts a branch or worktree (a foreman that dispatched you
   already placed you in one; work in place). Record missing QA configuration as a handoff warning and point
   at `$bbs:setup-project` when it matters.
2. **build and child modes (skip when init already seeded `plan.md`; a child
   plans its own ticket within the accepted parent plan, XS skips):** run
   `plan-draft` (user-facing work routes through `design-ui`, so the plan
   carries the UI spec + prototype). Write `plan.md` unless the task is XS.
   Don't stop for plan/prototype review — the human reviews once, at the
   final handoff — unless `--stop-after=plan` was passed. A Foreman ticket
   approves its plan by ticket size here instead (SKILL.md § Foreman ticket
   assignment).
3. **build / implement / child modes:** run `implement` against the
   requirement, plan, and (child mode) only the child scope. `implement`
   leaves its output uncommitted, and so does this workflow: gates bind the
   tree's content, and committing belongs to the human or Foreman.
4. Record each gate around its actual execution with autopilot
   references/verification.md; markdown verdicts never satisfy readiness.
   Run `review-pr --fix` in the current autopilot session via SKILL.md's
   **Current-session gates (`review-pr`, `qa`)** policy (applies fixes to
   the working tree), bracketed by
   `bbs autopilot verification begin --gate review-pr` and `record` — the
   push/PR hook reads that evidence when a human pushes or opens a PR.
   Verify mode may reuse review only if its accepted evidence covers the
   current tree; otherwise run it too.
5. Run `qa` under the same policy (judged in this session, its execution in
   the QA executor subagent), after review fixes are integrated, against
   the requirement's acceptance criteria, the
   `**Verify:**` line, and the implement handoff — not just the diff. The
   `qa` skill tests this checkout: inside a ticket worktree it runs what it
   needs there and takes the surface lease for a shared resource (see
   `../shared/worktrees.md`). No runnable target → record the blocker
   and run the strongest fallback (`browse` for UI, else a narrow local
   check). Bracket it with `bbs autopilot verification begin --gate qa` and
   `record`.
6. Apply autopilot's **Repair until the final change passes** loop: a QA fix
   returns to review, and both gates must cover the final tree.
   Reconcile every acceptance criterion with executed evidence and require
   `bbs ticket readiness --action review --json` to report ready. Autopilot
   stops here, work uncommitted: never commit, push, or open a PR — the human
   commits (or runs `create-pr`); under a foreman, Foreman seals the ticket
   and lands it with its phase.
7. Write a handoff: mode, checkout, changed files, deviations from the plan
   (the implement handoff's `## Deviations`), prototype path when `design-ui`
   produced one, QA evidence, concerns, next action, and any follow-up worth
   its own ticket with the evidence for it. Child mode targets
   the parent foreman run. Cross-repo: list every touched repo with its
   branch. Write it so a non-technical owner can act: lead with what was
   built and where to see it (URL), and give the next action as a copy-paste
   command. Confirm first: no debug leftovers, nothing outside the ticket's
   scope in the diff, checkpoint current.
### Foreman-dispatched children
`origin.type=sub_ticket` scopes child mode; the current checkout is already the
worktree foreman prepared. Never derive or check out a branch in this
workflow. The child handoff targets its parent and leaves sealing, phase
integration, phase QA and landing to foreman.
### Cross-repo tasks (related repos)
`setup-project` records siblings: meaning in `AGENTS.md` § Related Repos,
machine-local paths in the workspace registry (`bbs config workspace show`), which
is the authority; `RELATED_*_REPO` in `.babysit/.env` is the fallback for
repos outside a workspace. **Prefer the
current repo** — cross into a sibling only for the slice that genuinely
cannot be done here, and do the minimum there. Steps 4–5 are repo-relative:
review and QA each repo's change against *its own* base, once per repo touched.
1. Resolve the path (`bbs config workspace show`, else `grep '^RELATED_'
   .babysit/.env`). Unset path, or a sibling path that is not a git
   repository → don't guess: `NEEDS_CONTEXT`
   naming the repo and its slice of the requirement.
2. Work the sibling inside one subshell so its identity never leaks back
   into the parent repo (autopilot never cuts, even in a sibling):
   ```bash
   (
     cd "$sibling"
     ENSURE_OUT=$(bbs ticket ensure --no-branch)
     export BABYSIT_TICKET=$(printf '%s\n' "$ENSURE_OUT" | sed -n 's/^TICKET=//p')
     # … all sibling-side implement / bbs ticket / qa calls run here …
   )
   ```
   Then link both sides:
   `bbs ticket set-sibling --role <fe|be|shared> --repo <name> --ticket <id>`.
3. Run `qa` in each repo you touched; the `qa` skill owns that repo's
   surface protocol. Record that repo's gate on the sibling ticket
   (`BABYSIT_TICKET=<sibling> bbs autopilot verification begin --gate qa`, then
   `record`).
4. The handoff lists every touched repo with its changed files.
**Stop conditions**

- `NEEDS_CONTEXT`: missing requirement, missing credentials, or a human-only
  decision (no plan to implement and none can be drafted safely).
- `BLOCKED`: QA/verification fails and cannot be fixed locally, merge
  conflict, or no changes produced.
**Final status**
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: BUILT
SUMMARY: <mode, checkout, files, QA evidence>
NEXT: commit it or /bbs:create-pr — under a foreman, nothing: Foreman lands it with its phase
```
