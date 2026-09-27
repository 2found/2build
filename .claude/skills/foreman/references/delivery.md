## Eager per-ticket finish

A child whose finish prerequisites pass finishes when its worker report is handled.
PR/review-ready tickets and settled workers need not wait for final project
QA. Local lands wait for the last per-ticket surface mutation and pre-land
integration gate, because those operations can reset base.

An evidence-only child is eligible when its workflow verdict, acceptance
evidence, lifecycle signal, and clean worktree are current. Archive its
artifacts, run `BABYSIT_TICKET="$CHILD" bbs ticket set-status done`, release and
close its worker surfaces, and remove the verified-clean non-primary worktree;
keep the branch.
If any code changed, use the code-bearing path instead.

A code-bearing child is eligible when all of these hold:

- current `review-pr` + `qa` verdicts are DONE or DONE_WITH_CONCERNS;
- `bbs ticket readiness --action <land|pr|review> --json` allows that exact
  action — under `review` it still guards a dirty tree, an active attempt,
  and stale evidence, even though Foreman performs no merge or remote write;
- every prerequisite child has itself finished (landed, PRed, or — under
  `review` — gates passed); dependency order is preserved, never reordered;
- under `land`, every required code-bearing child has settled per-ticket
  review/QA, no worker can still compose the primary, and any Pre-land
  integration QA Task passed. `surface compose` resets local base to
  `origin/<base>` and would discard an early merge. Final Integration QA
  depends on these lands, so it must not be a prerequisite of the land handler.

Dispatch `land`/`pr` handlers to normal-phase delivery workers, one child at a
time in dependency order. Name the policy, exact child/head, destination, lease
and required receipt; verify it before cleanup or the next handler. `review`
status and worker release stay with Foreman:

- `land` — `bbs ticket land <child>` from the primary checkout. Revert any
   scratch composition first (`bbs ticket surface revert`); `land` BLOCKs on
   a nonempty `bbs-serving` marker. It merges locally and never pushes. After
   the finish receipt is persisted and the landed head is verified on base,
   run `BABYSIT_TICKET="$CHILD" bbs ticket set-status done`.
- `pr` — invoke the real `create-pr` skill for that child as soon as its
   gates pass; a PR does not mutate base, so final integration QA does not
   hold PR creation, but it still gates project completion. Read back the
   child's `pointers.pr` and `in_review` status persisted
   by `create-pr`; do not repeat those writes. It becomes `done`
   only after the PR is observed merged.
- `review` — no merge is authorized. Run Orca worktree close-out and keep the
   clean branch and Git worktree for human inspection. Run
   `BABYSIT_TICKET="$CHILD" bbs ticket set-status in_review`.

Archive settled output, `worker-release` the worker, release its resource lease,
then close the ticket's Orca surfaces. `worker-release` closes only the agent
terminal its Dispatch owns. Before bulk close, require every recorded Dispatch
settled and `orca orchestration worker-list --run <run_id> --terminal-state active
--include-remote --json` to show no live/unverifiable worker at that path; release
any reclaimable row first. Close only surfaces owned by the exact worktree:

```bash
orca terminal close --worktree path:<worktreePath> --all --json
orca tab list --worktree path:<worktreePath> --json        # then per row:
orca tab close --page <browserPageId> --json
orca emulator list --worktree path:<worktreePath> --json   # then per row:
orca emulator kill --emulator <id> --json
orca terminal list --worktree path:<worktreePath> --json   # verify: zero rows
orca worktree set --worktree path:<worktreePath> \
  --workspace-status <in-review|completed> --json          # in-review under
                                                          #   review/pr, completed under land
orca automations list --json                               # land/pr only: rows whose
                                                          #   runContext.path matches
orca automations edit <id> --disabled --json               #   disable, keep history
bbs ticket worktree-remove <worktreePath>                 # land/pr only; last —
                                                          # retries transient NTFS
                                                          # open-handle failures
```

Bulk terminal close is mandatory even under `review`. Under `land`/`pr`, remove
only the verified-clean non-primary Git worktree using `bbs ticket worktree-remove`
(git worktree remove with bounded NTFS open-handle retries); keep the branch.
Failures/holds retain a recoverable checkout. `selector_not_found` means Orca
tracks nothing there. Never substitute `orca worktree rm`: it deletes the branch
as well; a stale Orca card after safe Git worktree removal is expected.

## Failure and resume

Failure routing — never blind-retry an unchanged state or schedule retry ticks:

| Reported condition | Next action |
|---|---|
| Surface lease held | Queue until release/resource notification or bounded recovery |
| Stale evidence / `ready:false` | Re-run affected gates before delivery |
| Merge conflict | Supervised repair Dispatch merges `origin/<base>` (never local base) into the child; await settlement |
| Other land BLOCK (dirty/off-base/scratch) | Report blocker; wait for the primary state to change |
| A later composition discarded a land | Re-land after surface work settles; recreate a removed worktree from its branch first |
| `create-pr` failed | Recover actual handler state, retry once if recoverable; otherwise block with evidence |

On resume, recognize a finish receipt before evaluating worktree-bound
readiness. Persist each successful handler's action, verified branch/head and
dependency SHAs, gate evidence paths, and PR URL or landed revision in the
child handoff before removing its worktree. A `pointers.pr` link or
`git merge-base --is-ancestor` result is a recovery lead, not proof of current
acceptance. Verify the receipt still matches current scope and revisions; for
`pr`, read the PR's state and head (an open or merged PR's head must match the
verified revision; later unverified commits and closed-unmerged PRs are not a
successful finish). For `land`, verify the
recorded head remains in base. Reuse valid evidence without recreating a
worktree just to run readiness; missing/stale proof requires reconstruction
from the recorded branch and re-verification. For a changed PR head, first
fetch and inspect that actual head; do not re-verify an obsolete local branch
and call the remote change covered. Preserve any divergent local work and
dispatch reconciliation without force-pushing. Recover a lost receipt from
actual handler state and existing gate evidence, never by assuming success.
