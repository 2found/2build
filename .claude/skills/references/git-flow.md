# Git flow

There is one flow and no configuration. The only setting is the base branch:
`BBS_BASE_BRANCH` → `bbs config get base_branch` → `origin/HEAD` → `main`.
A one-shot `BBS_BASE_BRANCH=production` is how a hotfix targets another base.

## Who owns git

| Role | Does | Never |
|---|---|---|
| **Autopilot** (a worker) | edits files in the checkout it was started in and leaves the work **uncommitted**; its `new` workflow makes one skeleton commit on a repository it started from empty | commits anything else, branches, stashes, resets, pushes, opens a PR |
| **The lead** | cuts ticket worktrees, builds **one commit per phase**, lands it on local base, then QAs it there | commits a single ticket's work; releases a phase QA did not pass on |
| **release** | pushes the landed base and deploys | runs before phase QA passed on the landed base |
| **A human** | owns everything else: PRs, promotion branches, merges done by hand | — |

A ticket's commit is not a deliverable. **Committing a single ticket is an
unfinished job**: the work counts only when its phase commit is on base.

## How work moves

1. `bbs ticket ensure --mode=worktree` cuts the ticket's worktree
   ([where](worktrees.md)) from local base (scratch branch `<type>/<id>_<slug>`; it never gains a
   commit). `--mode=trunk` (the default) rides the current checkout.
2. The worker plans, builds, reviews and QAs there. Verification evidence
   records what was checked, so a commit is never needed to prove anything;
   nothing stales a recorded gate.
3. Tickets of one phase are independent and never take each other's work;
   what one needs from another lands first, in an earlier phase.
4. The worker's succeeded `bbs team task done` snapshots the worktree on a
   hidden ref `refs/bbs/tickets/<id>` — plumbing, never on a branch; there
   is no seal step, and `integrate` snapshots any edit made after it.
5. `bbs team integrate "$PARENT" --phase k` composes the phase's snapshots
   into **one commit** on the current tip of local base, checked out
   (detached) in the phase worktree `<parent>_phase-<k>` (its JSON names the
   path). Base having moved since the tickets started is normal: it is a
   three-way merge, nothing more.
6. **Conflicts are resolved in place.** When tickets conflict with each
   other or with base, `integrate` lays the merge out in the phase worktree
   with conflict markers (`CONFLICT=<ticket|base>: <file>`, `RESOLVE_IN=`)
   and exits non-zero. Whoever ran it — the lead for the phase, the phase QA
   session for its own fix — resolves there, keeping both sides' intent
   (generated files: regenerate), runs the focused checks, and runs
   `integrate` again, which commits the resolution (the commit body names
   the resolved files). A ticket whose work changes in between starts the
   layout over. A conflict never goes back to a ticket; one that needs a
   change beyond the approved design parks the phase `BLOCKED`.
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
   Land also installs a `pre-push` hook (unless the repository has its
   own): a push of base carrying a phase whose QA has not passed is
   refused, so a human push never publishes unverified work.
8. Phase QA runs in the phase worktree, on exactly that landed commit. The
   QA session fixes a failure forward itself: it edits the owning ticket's
   worktree, `integrate` snapshots that ticket again and composes the fix delta
   (conflicts with newer base laid out as in step 6), `land`, re-check what
   changed. Passing QA removes the phase's worktrees.
9. `release` pushes and deploys base, only once phase QA passed. A
   mid-run release over work outside the project (commits no phase made,
   uncommitted changes in the base checkout) waits for a human go or is
   skipped at its deadline (`bbs team release --request`, `--skip`).

Base history gains one commit per phase, plus one per QA fix round.
