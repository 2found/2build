## Ticket close-out and phase landing

A ticket is never delivered alone. Its worker leaves the work uncommitted;
the lead seals it (`bbs team seal`), and it reaches base only inside its
phase's single commit ([phases](phases.md)). **A ticket that was committed,
pushed or PR'd on its own is unfinished work**, not a shortcut: treat it as a
`repair` (the session discards nothing; the next seal snapshots the tree).

A ticket is sealable when:

- current `review-pr` + `qa` verdicts are DONE or DONE_WITH_CONCERNS, except
  the gates its size leaves to phase review (`seal` records that deferral);
- `bbs ticket readiness --action review --json` reports `data.ready: true`
  (it checks content-bound evidence, an active attempt, and staleness);
- every prerequisite it adopted is itself sealed at that snapshot.

Gates are proportional, not a ritual: the lead judges. A simple or docs-only
ticket, or a `repair` whose handoff already shows the tests, build and review
done on the final tree, may be sealed without re-stamping the gates:

```bash
bbs team seal "$CHILD" --waive review-pr,qa --reason "docs only; checked in handoff"
```

`--waive` accepts `review-pr`, `qa` or both and covers only a gate that is
**missing or stale**. It never overrides a BLOCKED/failed verdict, a limited
result, unresolved material findings, a manifest or snapshot change. The
waiver (gates, state, reason, time) is stored in the seal and shown as a note
in `bbs team snapshot` and `readiness`: a concern to report, never a
blocker. Prefer a real gate when it is cheap; a waiver is the exception.

An evidence-only child is sealable when its workflow verdict, acceptance
evidence are current and its worktree holds no code
change; seal its artifacts (`bbs team seal "$CHILD" --file`). If any code
changed, use the code-bearing path instead.

A ticket's worker is shut down when it reports done; a phase QA failure or a
phase that does not compose comes back as a `repair` on a fresh session in the
ticket worktree, pointed at the ticket's handoffs.

### When the phase is landed and verified

`bbs team land "$PARENT" --phase "$K"` only fast-forwards base; the worktrees
stay for phase QA and its fixes. The evidence record that makes the landed
phase pass QA removes the phase's integration worktree and every ticket
worktree whose content still equals its sealed snapshot
(the snapshot ref keeps that work). It first closes every Orca terminal in those worktrees
(`closed_terminals`) — the ticket sessions and phase QA worker were kept only
for repairs — never its caller's:

```bash
bbs team evidence ... --json   # cleanup: removed_worktrees / kept_worktrees / closed_terminals
BABYSIT_TICKET="$CHILD" bbs ticket set-status done   # each ticket of the phase
```

A `kept_worktrees` entry holds edits made after its seal: report it; never
delete it. Remove one that is only held open by a stale handle with
`bbs ticket worktree-remove <path>` (bounded NTFS open-handle retries).

Archive settled output, release the worker's resource lease, then close the
ticket's Orca surfaces. Before bulk close, require every Dispatch of the
ticket's Tasks settled (`bbs team task list "$FOREMAN_ID"`; `task abandon` a
stuck one first) and `orca terminal list --worktree path:<worktreePath> --json`
to show no agent you did not launch. Close only surfaces owned by the exact worktree:

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
| `land`: base moved since `integrate` | `bbs team integrate` again, then land (nothing was tested yet) |
| `land`: local changes in a checkout of base overlap the phase | Report the files and the checkout; its owner commits or moves them. Never stash, reset or overwrite them |
| cleanup: worktree kept | Report it; keep the work |

On resume, read `bbs team snapshot` first: a phase whose commit is on base
is landed whatever the worktrees look like; a sealed ticket's work lives on
`refs/bbs/tickets/<id>` even after its worktree is gone. Never assume success
from a log line — the snapshot is the record.

## Hand over

The handover is the human's second touchpoint. They review the **product**,
not the code: everything they need to judge it without having watched the
run, in one place.

1. **Make it reviewable.** The last phase has landed and its cumulative QA
   passed on that exact commit. Make sure the human can use it: the running
   preview the QA used (URL or the one command that starts it, test
   credentials by reference to where they live — never the secret itself),
   or, for non-UI work, the commands that exercise it.
2. **Write the handover** at the top of parent `report.md`, for a client who
   did not watch the run:

   ```markdown
   ## Handover
   **What you can do now** — the journeys of the spec, in product terms.
   **See it** — preview URL or start command; where test accounts live.
   **Evidence** — per acceptance criterion: PASS and its evidence link.
   **Decisions taken for you** — each with the reason and how to reverse it.
   **Limits and follow-ups** — known gaps, waived gates, carried evidence,
     anything parked; what is out of scope by the spec.
   **What ships** — the release go deploys <services>; rollback is <how>.
   ```

   Write *Decisions taken for you* from the [decision trail](#decision-trail).
   Then `bbs team recap "$PARENT"` renders it with the phases, commits,
   coverage, tickets and the trail into `<parent>/recap.html` (`RECAP=`).
   Its **Check first** section lists trail rows whose evidence does not
   resolve or whose result is not `verified`: fix a wrong row with a
   superseding one, and name what stays open under *Limits and follow-ups* —
   never hand over a recap that flags a row you can correct.
3. **Ask for the go**: `bbs team release "$PARENT" --phase "$LAST" --request
   --note "Handover: <recap path>"`, relay it through the preamble's channel,
   and end the turn. Earlier `auto` phases are already in production; say so
   in the handover.

The human answers on the release approval:

- **Approve** — the go. Dispatch the release (`release-prd` with hypercare),
  record the verdict with `bbs team release --file`, then finish below.
- **Redirect** with comments — feedback. `bbs team next` names a `repair`
  step; read the comments (`BABYSIT_TICKET=<last phase> bbs ticket approval
  comments`) and sort each item against the confirmed spec and approved
  design:
  - **in scope** — a defect, a criterion that does not hold, a flow that
    differs from the prototype, a decision taken for them that they reverse:
    the team owes the fix. Run one fix round alone: one `repair` per owning
    ticket carrying all its items, reseal, integrate, land, phase review +
    QA (carry what the fixes cannot touch), update the handover with what
    changed, re-render the recap and request the go again. No questions in
    between — an ambiguous item is fixed the way the spec reads, and the
    handover says how.
  - **beyond scope** — a new journey, a changed criterion, a different
    product decision: a change request. Revise `requirement.md` and the
    contract with a new last phase for it (the previous last phase becomes
    `none`), and publish it as a `project-plan` review — the brief touchpoint
    again. In-scope items of the same feedback are fixed meanwhile.
- **Drop** — the human declined the release. Stop, report what was built and
  where it is, and leave base as it is.

A handover with no release profile ends at the recap: the project completes
when the last phase is landed and verified, and feedback opens a new project or a contract
change.

## Finish the project

1. The last phase's cumulative QA is the project's final QA; nothing is
   re-tested by a separate final pass unless a later repair invalidated it.
2. `bbs team complete` itself checks every phase, gate and cleanup, so no
   audit worker is needed by default. Dispatch a read-only finish-audit
   worker on the **normal** route only for what the CLI cannot see — scope
   cancelled mid-run (did it go through an accepted contract change?), kept
   worktrees, a release whose hypercare was cut short.
3. Release settled workers and leases, persist the parent report, and require
   `bbs team readiness "$PARENT" --action finish --json` to report ready.
   Only then run `bbs team complete "$PARENT" --team "$FOREMAN_ID"`. No
   unreleased phase, missing gate, blocked ticket or unfinished cleanup can
   become DONE.
4. **Recap.** Update the `## Handover` section with the release outcome
   (tag, hypercare result). `complete` re-renders `<parent>/recap.html` from
   that report plus durable state and names it in the receipt's `recap`.
   Link it in your final report. The watcher leaves your terminal open and
   idle; do not retire yourself.

Report the result per phase — phase commit, release tag and verdict, QA
evidence — and remaining human action. Use `STATUS: DONE | IN_PROGRESS |
NEEDS_CONTEXT | BLOCKED`, link the parent `recap.html` and `report.md`, and end with `Next:`
(the handover awaiting review, kept worktrees, parked approvals, or none).
Distinguish landed-on-local-base from released (pushed and deployed).
Normal worker activity stays in Orca; report state changes, requests and
terminal evidence.

## Decision trail

`<parent>/decisions.jsonl` is the project's append-only record of the
judgments the team took for the human. Project state already records seals,
waivers, integrations, landings and releases; log what it cannot reconstruct:

```bash
bbs team log "$PARENT" --phase <id|design|handover|start> [--ticket <id>] \
  --decision "<what was chosen>" --why "<rule or evidence it rests on>" \
  --evidence <commit SHA | path[:line] | URL | rubric row> \
  --result <verified|failed|open|inconclusive|reverted>
```

- **Log**: an assumption or Taste choice the spec left open; a project-plan
  approval you resolved under `--auto` and each `L` or money/auth/
  irreversible-data plan you approved; a resize; the reason behind a waiver,
  a carry or a fixup; a worker replaced, a pivot or revert and its trigger; a
  blocker that survived its attempts and what was done instead; the human's
  handover answer. A worker's `M` self-approval reaches you in its handoff —
  log it if it decided something the brief left open.
- **Skip**: routine commands, passing checks, a worker's reasoning that never
  changed the plan.
- **Resume**: a resumed lead first logs a `--phase start` row naming its
  session and any decisions of the previous session it could not recover.
- **Correct**: never edit the file. Log a new row with `--supersedes <id>`;
  the recap hides the superseded row and counts it.

Evidence is a pointer, never prose; relative paths resolve in the parent
ticket directory, then each project repository. The recap checks each
pointer offline: an unknown commit, a missing path or a line past the end of
the file is flagged under **Check first**, as is any result other than
`verified`.
