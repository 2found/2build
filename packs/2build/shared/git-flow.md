# Git flow

There is one flow and no configuration. The only setting is the base branch:
`BBS_BASE_BRANCH` → `bbs config get base_branch` → `origin/HEAD` → `main`.
A one-shot `BBS_BASE_BRANCH=production` is how a hotfix targets another base.

## Who owns git

| Role | Does | Never |
|---|---|---|
| **Autopilot** (a worker) | edits files in the checkout it was started in and leaves the work **uncommitted** | commits, branches, stashes, resets, pushes, opens a PR |
| **The lead** | cuts ticket worktrees, seals each ticket, builds **one commit per phase**, QAs it, lands it on local base | commits a single ticket's work; lands anything QA did not pass on |
| **release-prd** | pushes the landed base and deploys | runs before a phase landed |
| **A human** | owns everything else: PRs, promotion branches, merges done by hand | — |

A ticket's commit is not a deliverable. **Committing a single ticket is an
unfinished job**: the work counts only when its phase commit is on base.

## How work moves

1. `bbs ticket ensure --mode=worktree` cuts `<repo>/.babysit/worktrees/<ticket>_<slug>/`
   from local base (scratch branch `<type>/<id>_<slug>`; it never gains a
   commit). `--mode=trunk` (the default) rides the current checkout.
2. The worker plans, builds, reviews and QAs there. Verification evidence is
   bound to the **content** of the tree (`tree_digest`), so a commit is never
   needed to prove anything, and any edit after a gate makes it stale.
3. A dependent ticket runs `bbs team await`: when its prerequisites are
   sealed, their work is **adopted** into its worktree as uncommitted changes
   (three-way merged; `CONFLICT=` names files to resolve). `bbs ticket adopt
   <ticket>` does the same on demand.
4. `bbs team seal` snapshots the worktree on a hidden ref
   `refs/bbs/tickets/<id>` — plumbing, never on a branch.
5. `bbs team integrate "$PARENT" --phase k` composes the phase's snapshots
   into **one commit** on local base, checked out (detached) in
   `<repo>/.babysit/worktrees/<parent>_phase-<k>`. A conflict names the
   ticket and files; the later ticket adopts the earlier one, resolves and
   reseals.
6. Phase QA runs in that worktree, on exactly that commit.
7. `bbs team land "$PARENT" --phase k` fast-forwards local base to it and
   removes the phase's worktrees. Local edits in a checkout of base ride
   along; one that touches a file the phase changes stops the land with the
   file named — nothing is ever stashed or reset. If base moved since the
   build, land refuses: integrate again and re-run phase QA.
8. `release-prd` pushes and deploys base.

Base history gains exactly one commit per phase.
