---
name: recon
description: Evaluate an external repository, library, or tool against the current project. Use for adoption decisions, architecture comparisons, or requests to explore and borrow from another project.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# recon
Evaluate the external target against the local project's concrete problem —
primary sources only (source code, official docs, releases, license) — and
recommend `copy`, `refactor`, `rebuild`, or `pass`, accounting for
maintenance, security, dependency, and migration cost. Separate attractive
ideas from code worth importing; preserve attribution and license
obligations. Implement only when the user requested adoption and the
recommendation is clear.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT
VERDICT: STEAL(<approach>) | PASS
SUMMARY: <fit, costs, recommendation>
NEXT: <implementation action or none>
```
