---
name: product-marketing-page
description: Write or revise product introduction pages around customer value and a clear activation path. Use for product heroes, benefits, CTAs and agent-assisted quick starts, rather than full technical manuals.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install lohi-ai/babysit/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# product-marketing-page
Grower-owned product communication. Follow the
[Auto-Decision Framework](../shared/auto-decision-framework.md) for choices;
use the user's requested direction and existing brand/design guidance.
Shared refs are filesystem paths beside this skill's directory, so read them by
path, not as `skill://`.

- Lead with what the product helps the visitor achieve. Explain enough mechanism
  to support each benefit; move manifest fields, command grammar and complete
  procedures into docs. Don't invent speed/savings claims, availability guarantees,
  customer evidence, prices, integrations or release/license status.
- Give the primary CTA one concrete activation task and a secondary docs path.
  For agent-assisted onboarding, the quick start is a brief invitation followed
  by **one prompt from the docs' maintained source**, then concise setup paths.
  Don't create a second prompt or imply the website itself performs a deployment.
- Explain real setup barriers, including starting without required resources,
  accounts or credentials. Identify the user's consequential choices and the
  work the agent handles; private credentials travel by file/reference, not chat.
- Docs destinations follow [AX Markdown delivery](../agent-first-docs/references/ax-markdown.md):
  predictable Markdown URLs, root/scoped llms.txt and machine-readable discovery.
  Verify the CTA's actual target and language instead of guessing localized slugs.
- Reuse host components, tokens, layout and copy controls. Keep translations and
  metadata aligned. Check text fit on desktop/mobile and an adverse copy/navigation
  case through the project's browser workflow. Preserve material limitations in
  fit guidance or FAQ. Leave source changes reviewable; release is caller-owned.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: REWRITTEN
SUMMARY: <positioning, activation/docs path, verification, limits>
NEXT: <remaining action or none>
```
