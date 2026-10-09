---
name: create-pr
description: Prepare and create a pull request from the current branch. Use when the user requests a PR or a workflow authorizes the PR handoff.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# create-pr
Create a reviewable PR without merging it.
## Flow
1. Inspect git status, branch, commits, and remote. Resolve policy with `eval "$(bbs autopilot git-flow)"` — `$BBS_BASE_BRANCH`, `$BBS_MODE`, `$BBS_LAND`, `$BBS_PUSH`. Honor the mode and the landing policy — see below. Load repo env: `eval "$(bbs secrets load | sed -E '/^export CLOUDFLARE_(ACCOUNT_ID|API_TOKEN)=/d')"`; if `GH_ACCOUNT` is set, `gh auth switch -u "$GH_ACCOUNT"` before any push or `gh pr` call (multi-account machines fail with "Repository not found" on the wrong account).
2. Read requirement, plan, implementation handoff, and verification evidence when present. Carry them into the PR body as a short reviewer explainer: context and intent, where new code meets existing behavior, deviations from the plan (implement handoff's `## Deviations`), QA evidence. Highlight any consequential interpretation, system impact or unresolved assumption the reviewer must judge. Include prototype/QA links when relevant; no mandatory quiz or repeated plan.
3. Resolve mechanical version or changelog requirements only when the repo requires them.
4. With a resolved ticket, if `origin/<base_branch>` has moved, run
   `bbs ticket refresh` first. Without a ticket, follow the repo's base-sync
   policy. Refreshing changes the tested state: rerun affected checks before
   claiming current verification. Commit only remaining intended changes.
   If `push: false`, stop with `BLOCKED` naming the policy before pushing.
   For a resolved ticket, inspect `bbs ticket readiness --action pr --json`
   and require `data.ready=true` for the final head. Then push the source
   branch and open the PR against `base_branch`. When a ticket resolves,
   persist `bbs ticket set-pointer pr <url>` and `bbs ticket set-status in_review`.
   `board --pr` and `fix-pr` use the pointer; status becomes `done` only after the PR is observed merged.
5. Return the PR URL, title, summary, tests, and concerns. Cross-repo tickets: a sibling repo's change needs its own create-pr run there; list the sibling repo + branch in the summary instead of fanning out.
## Git-flow policy
`land: none` (the `pet` profile) means this repo does not do PRs — the push
*is* the release. Stop with `BLOCKED` before anything else:

```
STATUS: BLOCKED
SUMMARY: this repo is a pet project (profile: pet, land: none) — work lands on
$BBS_BASE_BRANCH directly, there is no PR step. This invocation did not push
or release anything.
NEXT: follow the configured direct-delivery workflow; change policy only if requested.
```

The PR is always cut from the **ticket branch** and targets `$BBS_BASE_BRANCH`.

- **trunk** (the default — babysit cut nothing) — the branch the human is on *is* the PR source: push it and open the PR. Two stops: if that branch is `$BBS_BASE_BRANCH` there is nothing to PR (`BLOCKED`, naming `git switch -c <branch>` as the fix), and if it is a shared branch carrying other tickets' work, say so in the PR body rather than pretending the diff is one ticket's.
- **branch** — the current branch is the ticket branch; push it and open the PR.
- **worktree** — the ticket branch lives in a worktree and the base checkout carries throwaway `surface compose` integration merges. Run from the worktree (`bbs ticket resolve` gives the path); never push the base checkout, or those merges leak into the PR.
## Compose PR (multiple tickets, one PR)
When the human reviewed a composed surface (worktree mode, `bbs ticket surface compose <t1> <t2> …`) and wants the set to land together: cut `compose/<date>` from `origin/<base_branch>`, `git merge --no-edit` each ticket branch in (a conflict → `BLOCKED` naming the pair; resolve on the ticket branch, not the compose branch), push it, and open one PR whose body lists every ticket with its evidence per step 2. Never push the base checkout itself — the compose branch reproduces the same merges on a PR-able branch.
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
