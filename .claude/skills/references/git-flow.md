# Git flow

There is one flow and no configuration. The only setting is the base branch:
`BBS_BASE_BRANCH` → `bbs config get base_branch` → `origin/HEAD` → `main`.
A one-shot `BBS_BASE_BRANCH=production` is how a hotfix targets another base.

## Who owns git

| Role | Does | Never |
|---|---|---|
| **Autopilot** (a worker) | edits files in the checkout it was started in and leaves the work **uncommitted** | commits, branches, stashes, resets, pushes, opens a PR |
| **The lead** | cuts ticket worktrees, seals each ticket, builds **one commit per phase**, lands it on local base, then QAs it there | commits a single ticket's work; releases a phase QA did not pass on |
| **release-prd** | pushes the landed base and deploys | runs before phase QA passed on the landed base |
| **A human** | owns everything else: PRs, promotion branches, merges done by hand | — |

A ticket's commit is not a deliverable. **Committing a single ticket is an
unfinished job**: the work counts only when its phase commit is on base.

## How work moves

1. `bbs ticket ensure --mode=worktree` cuts the ticket's worktree
   ([where](worktrees.md)) from local base (scratch branch `<type>/<id>_<slug>`; it never gains a
   commit). `--mode=trunk` (the default) rides the current checkout.
2. The worker plans, builds, reviews and QAs there. Verification evidence is
   bound to the **content** of the ticket's own change (`tree_digest`: paths that
   differ from the ticket's start, minus files already dirty then), so a commit
   is never needed to prove anything, and an edit to those paths after a gate
   makes it stale; unrelated edits in a shared checkout do not.
3. A dependent ticket runs `bbs team await`: when its prerequisites are
   sealed, their work is **adopted** into its worktree as uncommitted changes
   (three-way merged; `CONFLICT=` names files to resolve). `bbs ticket adopt
   <ticket>` does the same on demand.
4. `bbs team seal` snapshots the worktree on a hidden ref
   `refs/bbs/tickets/<id>` — plumbing, never on a branch.
5. `bbs team integrate "$PARENT" --phase k` composes the phase's snapshots
   into **one commit** on the current tip of local base, checked out
   (detached) in the phase worktree `<parent>_phase-<k>` (its JSON names the
   path). Base having moved since the tickets started is normal: it is a
   three-way merge, nothing more.
6. **Conflicts are resolved in place.** When tickets conflict with each
   other or with base, `integrate` lays the merge out in the phase worktree
   with conflict markers (`CONFLICT=<ticket|base>: <file>`, `RESOLVE_IN=`)
   and exits non-zero. Whoever ran it — the lead for a mechanical conflict
   (generated files: regenerate; imports, lists, both-sides additions), the
   phase integration worker otherwise — resolves there, keeping both sides'
   intent, runs the focused checks, and runs `integrate` again, which commits
   the resolution (the commit body names the resolved files). A reseal in
   between starts the layout over. A cold `repair` session is only for a
   conflict that needs real rework of one ticket (base replaced the layer it
   builds on); then one repair carries all of it.
7. `bbs team land "$PARENT" --phase k` fast-forwards local base to it right
   away — no QA first, so parallel work builds on it at once. If base moved
   since `integrate`, land composes again on the new tip itself
   (`REINTEGRATED=`); a conflict there is laid out as in step 6.
   **A dirty base checkout** is normal too. Local edits that do not touch the
   phase's files ride along. Edits that do stop the land before anything
   moves, listing every uncommitted file (`LOCAL_CHANGES=<checkout>: …`):
   ask the human once, through the preamble's channel, to allow committing
   them on base; with the go, `land --commit-local "<message>"` commits that
   checkout's changes, composes the phase on top and lands. Park only this
   land while waiting; other work continues. Nothing is ever stashed or reset.
8. Phase QA runs in the phase worktree, on exactly that landed commit. A
   failure is fixed forward: reseal the owning ticket, integrate (the fix
   delta, conflicts with newer base laid out as in step 6), land, re-check
   what changed. Passing QA removes the phase's worktrees.
9. `release-prd` pushes and deploys base, only once phase QA passed. A
   mid-run release over work outside the project (commits no phase made,
   uncommitted changes in the base checkout) waits for a human go or is
   skipped at its deadline (`bbs team release --request`, `--skip`).

Base history gains one commit per phase, plus one per QA fix round.
