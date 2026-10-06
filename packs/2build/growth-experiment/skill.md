---
name: growth-experiment
description: Propose, rank, and optionally scaffold a measurable product growth experiment. Use for A/B tests, activation, retention, acquisition, funnel, or ICE-ranking requests.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install lohi-ai/babysit/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# growth-experiment
Turn a growth question into 5-10 experiments ranked by ICE (impact,
confidence, ease) against the real funnel — when the product runs, walk the
target flow live (`browse`), never a guessed one. Recommend one winner with
the assumption that most constrains confidence. Never invent funnel numbers,
traffic sources, or activation data. Scaffold only when asked: the smallest
flagged, reversible variant with exposure and conversion tracking, behind an
existing flagging pattern when available, measurable in one event path.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: RANKED | SCAFFOLDED
SUMMARY: <winner + metric + verification if implemented>
NEXT: approve, implement, or create-pr
```
