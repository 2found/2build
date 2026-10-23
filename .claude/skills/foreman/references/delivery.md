## Ticket close-out and phase landing

A ticket is never delivered alone. Its worker leaves the work uncommitted;
Foreman seals it (`bbs foreman seal`), and it reaches base only inside its
phase's single commit ([phases](phases.md)). **A ticket that was committed,
pushed or PR'd on its own is unfinished work**, not a shortcut: treat it as a
`repair` (the session discards nothing; the next seal snapshots the tree).

A ticket is sealable when:

- current `review-pr` + `qa` verdicts are DONE or DONE_WITH_CONCERNS;
- `bbs ticket readiness --action review --json` reports `data.ready: true`
  (it checks content-bound evidence, an active attempt, and staleness);
- every prerequisite it adopted is itself sealed at that snapshot.

An evidence-only child is sealable when its workflow verdict, acceptance
evidence and lifecycle signal are current and its worktree holds no code
change; seal its artifacts (`bbs foreman seal "$CHILD" --file`). If any code
changed, use the code-bearing path instead.

Keep the ticket's worker session until its phase lands: a phase QA failure or
a phase that does not compose comes back to that session as a `repair`.

### When the phase lands

`bbs foreman land "$PARENT" --phase "$K"` fast-forwards base and removes the
phase's integration worktree and every ticket worktree whose content still
equals its sealed snapshot (the snapshot ref keeps that work). It first closes
every Orca terminal in those worktrees (`closed_terminals`) — the ticket
sessions and phase QA worker were kept only for repairs — never its caller's:

```bash
bbs foreman land "$PARENT" --phase "$K" --json   # removed_worktrees / kept_worktrees / closed_terminals
BABYSIT_TICKET="$CHILD" bbs ticket set-status done   # each ticket of the phase
```

A `kept_worktrees` entry holds edits made after its seal: report it; never
delete it. Remove one that is only held open by a stale handle with
`bbs ticket worktree-remove <path>` (bounded NTFS open-handle retries).

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
  --workspace-status completed --json
orca automations list --json                               # rows whose
                                                          #   runContext.path matches
orca automations edit <id> --disabled --json               #   disable, keep history
```

`selector_not_found` means Orca tracks nothing there. Never substitute
`orca worktree rm`; a stale Orca card after the Git worktree is removed is
expected.

## Failure and resume

Failure routing — never blind-retry an unchanged state or schedule retry ticks:

| Reported condition | Next action |
|---|---|
| Surface lease held | Queue until release/resource notification or bounded recovery |
| Stale evidence / `ready:false` | Re-run affected gates, then reseal |
| `integrate`: phase does not compose | `repair` on the later ticket named: `bbs ticket adopt <earlier>`, resolve, re-run its gates, reseal; then integrate again |
| `land`: base moved since the build | `bbs foreman integrate` again, re-run phase QA on the new commit, land |
| `land`: local changes in a checkout of base overlap the phase | Report the files and the checkout; its owner commits or moves them. Never stash, reset or overwrite them |
| `land`: worktree kept | Report it; keep the work |

On resume, read `bbs foreman snapshot` first: a phase whose commit is on base
is landed whatever the worktrees look like; a sealed ticket's work lives on
`refs/bbs/tickets/<id>` even after its worktree is gone. Never assume success
from a log line — the snapshot is the record.

## Finish the project

1. The last phase's cumulative QA is the project's final QA; nothing is
   re-tested by a separate final pass unless a later repair invalidated it.
2. `bbs foreman complete` itself checks every phase, gate and cleanup, so no
   audit worker is needed by default. Dispatch a read-only finish-audit
   worker on the **normal** route only for what the CLI cannot see — scope
   cancelled mid-run (did it go through an accepted contract change?), kept
   worktrees, a release whose hypercare was cut short.
3. Release settled workers and leases, persist the parent report, and require
   `bbs foreman readiness "$PARENT" --action finish --json` to report ready.
   Only then run `bbs foreman complete "$PARENT" --foreman "$FOREMAN_ID"`. No
   unreleased phase, missing gate, blocked ticket or unfinished cleanup can
   become DONE.

Report the result per phase — phase commit, release tag and verdict, QA
evidence — and remaining human action. Use `STATUS: DONE | IN_PROGRESS |
NEEDS_CONTEXT | BLOCKED`, link the parent `report.md`, and end with `Next:`
(a pending release go, kept worktrees, parked approvals, or none).
Distinguish landed-on-local-base from released (pushed and deployed).
Normal worker activity stays in Orca; report state changes, requests and
terminal evidence.
