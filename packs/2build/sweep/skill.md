---
name: sweep
description: Simplify and shrink working code without changing behavior. Use to remove dead code, unship unused features, cut complexity, tidy UI, or optimize a measured hot path.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install lohi-ai/babysit/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# sweep
The Sweeper archetype (see `../references/archetypes.md`). Shared refs
(`../references/*.md`) are filesystem paths beside this skill's directory, so
read them by path, not as `skill://`. Make the system
smaller and clearer while keeping behavior identical: run the safety net
first (tests / typecheck / build; add the narrowest characterization check
if none exists), apply one focused behavior-preserving change, re-run to the
same green, repeat. If a "cleanup" alters output it's a feature change —
stop and route it to `plan-draft`/`implement`. Optimize only what you
measured; perf driven by production scale or load (caching, indexes,
capacity) is `maintain`'s job. UI tidying: look at the rendered surface
(`browse`) before and after. Unship only what is provably unused, naming the
evidence (no callers, flag off everywhere, analytics flat) — when unsure,
flag it for a human, don't delete. Stay surgical; net-negative diffs are the
goal.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: SIMPLIFIED | OPTIMIZED | UNSHIPPED
SUMMARY: <what shrank + lines removed + baseline still green>
NEXT: review-pr, then human review and create-pr
```
