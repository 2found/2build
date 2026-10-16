---
name: create-pr
description: Prepare and create a pull request from the current branch. Use when a human requests a PR; 2build workflows never open one.
---
# create-pr
Create a reviewable PR without merging it.
## Flow
1. Inspect git status, branch, commits, and remote. The PR targets the base branch (`BBS_BASE_BRANCH` → `bbs config get base_branch` → `origin/HEAD` → `main`). The current branch is the PR source; if it *is* the base there is nothing to PR (`BLOCKED`, naming `git switch -c <branch>` as the fix), and if it carries other work, say so in the PR body. Load repo env: `eval "$(bbs secrets load | sed -E '/^export CLOUDFLARE_(ACCOUNT_ID|API_TOKEN)=/d')"`; if `GH_ACCOUNT` is set, `gh auth switch -u "$GH_ACCOUNT"` before any push or `gh pr` call (multi-account machines fail with "Repository not found" on the wrong account).
2. Read requirement, plan, implementation handoff, and verification evidence when present. Carry them into the PR body as a short reviewer explainer: context and intent, where new code meets existing behavior, deviations from the plan (implement handoff's `## Deviations`), QA evidence. Highlight any consequential interpretation, system impact or unresolved assumption the reviewer must judge. Include prototype/QA links when relevant; no mandatory quiz or repeated plan.
3. Resolve mechanical version or changelog requirements only when the repo requires them.
4. If `origin/<base_branch>` has moved, sync it the way the human asks;
   syncing changes the tested state: rerun affected checks before claiming
   current verification. Commit only remaining intended changes.
   For a resolved ticket, inspect `bbs ticket readiness --action pr --json`
   and require `data.ready=true` for the final head. Then push the source
   branch and open the PR against `base_branch`. When a ticket resolves,
   persist `bbs ticket set-pointer pr <url>` and `bbs ticket set-status in_review`.
   `board --pr` and `fix-pr` use the pointer; status becomes `done` only after the PR is observed merged.
5. Return the PR URL, title, summary, tests, and concerns. Cross-repo tickets: a sibling repo's change needs its own create-pr run there; list the sibling repo + branch in the summary instead of fanning out.
## Rules
- Never force-push.
- Do not include unrelated working-tree changes.
- Do not claim checks passed unless they ran.
- Do not merge; landing and deployment are outside this skill.
- If authentication or remote configuration is missing, emit exact setup guidance.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: PR_CREATED
PR: <url>
SUMMARY: <title + checks>
NEXT: human review
```
