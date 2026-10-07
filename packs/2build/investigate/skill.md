---
name: investigate
description: Debug a failure before fixing it. Use when the user asks why something is broken, wants root cause analysis, or reports an error, regression, flaky test, crash, or unexpected behavior.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# investigate
Root cause first, fix second: reproduce or collect the failing evidence,
name the root cause in one sentence before editing, confirm it by toggling
(revert the suspect change, remove the trigger input, or isolate it), apply
the smallest fix, then re-run the reproducer plus one nearby regression
check. Check the pothole map first — the git-archaeology recipe in
`../shared/finding-unknowns.md`: a prior fix commit in the failing area
often names this same root cause. Shared refs (`../shared/*.md`) are
filesystem paths beside this skill's directory, so read them by path, not as
`skill://`. Competing theories: list them, test the
cheapest — never guess silently. No symptom-papering (broad retries,
catches, sleeps, guards) unless the root cause demands it. Preserve
unrelated user changes. If the failure depends on external state you cannot
access, stop with the exact missing evidence.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
ROOT_CAUSE: <one sentence>
EVIDENCE: <what confirmed the cause>
FIX: <what changed, or "none">
VERIFICATION: <commands/checks run>
```
