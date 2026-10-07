---
name: agent-first-docs
description: Write product documentation and quick starts that begin with one coding-agent prompt, then human and agent references. Use for agent-assisted onboarding and discoverable Markdown documentation.
---
# agent-first-docs
Builder-owned documentation delivery. Follow the
[Auto-Decision Framework](../references/auto-decision-framework.md) for choices;
derive requirements from supplied artifacts or conversation, without a ticket gate.
Shared refs are filesystem paths beside this skill's directory, so read them by
path, not as `skill://`.

- Landing docs and quick starts open with a brief invitation and **one copyable
  prompt**, before prerequisites or install commands. Deep references stay
  focused on their subject. The prompt names the task, public Markdown/docs
  URL, optional source repo and the result the agent should verify. Avoid
  placeholders and mandatory plugin installation unless the product needs it.
- Make setup actionable from the user's current position. For each real barrier,
  explain what the user supplies, what the agent handles, the missing-resource
  path and the completion check. Deployment onboarding offers an existing VM
  alongside cloud provisioning, including account/billing/login setup; identify
  resources and provider costs before applying. For credentials, use a private
  file or secret-store reference and share its path, never values in chat/argv.
- Verify commands against the maintained product contract and current provider
  docs. Examples are instructions, not authorization to create live resources.
  State what was actually verified and any remaining blocker.
- Derive the docs and marketing quick-start prompt from one source per language.
  Keep imported snapshots separate from local onboarding; never fabricate a
  revision or bypass integrity checks to publish uncommitted upstream content.
- For hosted docs, read [AX Markdown delivery](references/ax-markdown.md).
  Publish clean Markdown, predictable URLs, discovery indexes and machine-readable
  links. Keep source examples intact; preserve existing URLs when improving paths.
- Verify built artifacts and HTTP behavior, including an adverse case such as
  an unknown Markdown URL or clipboard denial. UI checks use the host project's
  browser workflow. Leave source changes reviewable; release is caller-owned.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: DOCUMENTED
SUMMARY: <content, AX endpoints/discovery, verification, limits>
NEXT: <remaining action or none>
```
