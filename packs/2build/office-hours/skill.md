---
name: office-hours
description: Stress-test an idea before building. Use for startup/product judgment, builder brainstorming, narrowing a wedge, shaping a requirement, or deciding whether an idea is worth implementing.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# office-hours
Help the user think before code — pressure-test the idea (user, pain, current
workaround, narrow wedge, proof, next step) and write a short artifact the
next skill can use: problem, audience, scope, non-goals, risks, recommended
next action. Ask only for context that changes the decision; otherwise infer
and label assumptions. One sharp recommendation, not a menu. If the idea is
not ready, say what evidence would change that. No product code; never invent
customer proof, positioning, revenue, or analytics. Build-ready work routes
to `plan-draft` or `autopilot`.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: DESIGNED | NOT_READY
SUMMARY: <decision + reasoning>
NEXT: plan-draft, gather evidence, or none
```
