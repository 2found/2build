---
name: social-content
description: Create short-form product video scripts for TikTok, Reels, or Shorts. Use for hooks, scripts, captions, production notes, or a small content calendar.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install lohi-ai/babysit/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# social-content
Write ready-to-shoot short-form scripts — hooks, script beats, on-screen
text, caption, CTA, production notes — grounded in `product-marketing.md`,
README, or supplied context (label inferred positioning when no source
exists). Keep scripts shootable with realistic props and screen recordings.
Never invent customer proof, metrics, or testimonials; never post, schedule,
or call social APIs; never touch product source. Save to a local markdown
artifact only when useful.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: SCRIPTS
SUMMARY: <scripts/angles produced>
NEXT: shoot, revise, or none
```
