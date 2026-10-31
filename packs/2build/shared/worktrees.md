# Worktrees

Read this when a run is inside a ticket worktree — a lead batch (the lead
creates it, then starts autopilot there) or a human who ran
`bbs ticket ensure --mode=worktree` to isolate one ticket. The flow itself is
[git-flow.md](git-flow.md).

`bbs ticket ensure --mode=worktree` cuts `<ticket>_<slug>` from local base,
prints `WORKTREE=<path>`, and `manifest.yaml` records it — cd there, and every
later step runs there; never derive the path yourself. Run from an Orca
terminal on a repo Orca tracks, it is created through `orca worktree create`,
so it follows Orca's workspace rule (`<workspaceDir>/<repo>/<name>`) and shows
as an Orca workspace under the caller's; a plain git worktree elsewhere is an
external worktree Orca hides by default. Otherwise (no Orca, or Orca refuses)
it is `<repo>/.babysit/worktrees/<ticket>_<slug>/`, excluded locally so it
never shows up as untracked work. Phase worktrees (`bbs team integrate`)
follow the same rule.

## Rules inside a ticket worktree

- Work stays **uncommitted**. Gates bind the tree's content; the lead seals it.
- Install and run what the ticket needs in its own worktree. Use the ports,
  databases and secrets `qa.yaml` names; never edit the primary checkout.
- Need another ticket's work? `bbs team await` adopts sealed prerequisites;
  `bbs ticket adopt <ticket>` adopts on demand. Resolve every `CONFLICT=`
  file, then rerun the affected checks.

## Surface lease — one QA session on a shared resource

When tickets share something only one can use at a time (a dev database, a
fixed port, a device), the surface lease serializes them:

1. `bbs ticket surface acquire` — BLOCKs naming the holder while another
   ticket holds it.
2. Run QA; `prepare:` / `revert:` from `qa.yaml` run here when set.
3. `bbs ticket surface release`.

`bbs ticket surface status` shows the holder. Reentrant for the owner; past its
ttl (60 min default, `--ttl-min`) a crashed holder's lease is stale and the
next `acquire` takes it, loudly. One lease per repo. Solo runs never see it.

## Status

`bbs ticket board` lists tickets, verdicts and the lease holder.
