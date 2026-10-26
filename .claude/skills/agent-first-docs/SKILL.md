---
name: agent-first-docs
description: Write a product's docs, quick start or marketing page around one copyable coding-agent prompt, and serve docs as agent-readable Markdown (llms.txt, .md URLs). Use for onboarding docs, quick starts, product pages, heroes and CTAs.
disable-model-invocation: true
---
# agent-first-docs
The 2found convention for how a product introduces itself: a reader — human or
agent — gets from "what is this" to a working first result by pasting **one
prompt** into a coding agent. Derive the requirement from supplied artifacts or
conversation; no ticket gate. Shared refs are filesystem paths beside this
skill's directory, so read them by path, not as `skill://`.

## The one prompt
- Keep it in one maintained source per language; the docs quick start and the
  marketing CTA both render it from there. Never write a second prompt.
- It names the task, the public Markdown docs URL, the optional source repo and
  the result the agent must verify. No placeholders; no mandatory plugin install
  unless the product needs one.
- It opens the quick start, before prerequisites or install commands, after a
  one-line invitation.

## Surfaces
| Surface | Leads with | Keeps out |
|---------|-----------|-----------|
| Marketing page | What the visitor achieves; one primary CTA (the prompt as a concrete activation task) and a secondary docs link | Manifest fields, command grammar, full procedures — they live in docs |
| Docs / quick start | The prompt, then setup paths, then focused references | Marketing claims |

## Rules
- Make setup actionable from where the user is: for each real barrier (account,
  billing, VM, credentials) say what the user supplies, what the agent does, the
  path when the resource is missing, and the completion check. Credentials travel
  as a private file or secret-store reference, never values in chat or argv.
- Never invent speed/savings claims, prices, customers, integrations, guarantees
  or release/license status. Keep material limitations in fit guidance or FAQ.
- Verify every command against the product's current contract and provider docs;
  examples are not authorization to create live resources.
- Hosted docs follow [AX Markdown delivery](references/ax-markdown.md). Preserve
  existing URLs when improving paths; verify the CTA's real target and language.
- Reuse the host's components, tokens and copy controls; keep translations and
  metadata aligned.
- Verify the built result through the project's browser workflow: text fit on
  desktop and mobile, plus one adverse case (unknown `.md` URL, clipboard denied,
  broken CTA target). Leave changes reviewable; publishing is the caller's.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: DOCUMENTED
SUMMARY: <surfaces changed, prompt source, AX endpoints, verification, limits>
NEXT: <remaining action or none>
```
