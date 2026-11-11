# Agent-operable product

After launch a 2found product is run by agents as well as people: a Soot
(`2found/2agent`) answers its users, watches it and acts on it. This is the
bar that lets one do so. It applies when a change adds or reshapes an action
someone performs on the product, a status it reports, a public fact (price,
plan, limit) or an API. Anything else skips it. Build only the rows the change
touches; a row it touches and does not meet is a named gap in the plan and
handoff, never a silent one.

| The product provides | Bar |
|---|---|
| A non-UI path for every action | What an operator or support person can do in the UI, an agent can do through an API or CLI behind the same permission checks. A UI-only admin action is a gap. |
| Reads apart from writes | Read endpoints have no side effects, so an agent can be granted read-only. Each write names the one resource it changes. |
| Writes safe to retry | Idempotent on a key derived from the effect (resource, revision, target state), not on a fresh UUID. The same key with a different body is `409`. A stale revision is `409`, never a silent overwrite. |
| A preview of every write | A dry run that validates exactly as the real call does, changes nothing, and returns one human-readable summary of what will happen, or the reason it cannot. |
| Risky writes that can wait for a person | Publishing, paying, bulk sends, deletes: the input alone carries what an approver must see (ids, revision, the content itself) and is repeatable, with no signed or expiring values in it. |
| Errors a machine can act on | A stable `code`, a message, and the recovery (which field, which next call). Invalid input is `400` with that body; never a `200` carrying an error, never an HTML error page on an API route. |
| A scoped agent identity | Its own credential, least privilege, bound to tenant and user. Acting for a user means verifying a short-lived signed assertion, never trusting a header. The audit trail names the agent. |
| State readable without the UI | A health endpoint, and a read path for what needs attention (pending, failed, due, over budget): the same facts the dashboard shows, from the same source. |
| Facts published once, for machines | `/llms.txt` plus Markdown pages generated from the data the product runs on, one topic per page ([AX Markdown delivery](../agent-first-docs/references/ax-markdown.md)). A retyped price is a second copy. |
| Events with stable ids | What is worth reacting to (new order, failed job, user message) leaves the product with an id, so a duplicate delivery is harmless. |
| A way to test without harm | Test accounts and seed data an eval can run against; no flow that only works on production. |

## How Soot consumes it

The Soot contract is the authority and still changes; read its current docs
before building against it, and treat these as pointers.

- **Tools** (`docs/authoring-packs.md`, `docs/runtime-packs.md`): a product
  service implements `POST <service_url>/v1/tool` with a bearer token; JSON
  out, 128 KiB at most, 15 s, no redirects, no retries. A tool is `read` by
  default and declares `write`; `preflight: true` calls the same route with
  `hook: "tool.preflight"` (`400 invalid_input` refuses, `200` is the
  preview); `approval: "operator"` holds the call for a person.
- **Conversations** (`docs/product-integration.md`): the product backend holds
  the Soot key, asserts its own authenticated user, and sends a stable request
  id per message. A lost response is reconciled by reading that id, never by
  sending again.
- **Knowledge** (`docs/knowledge.md`): Soot mirrors `/llms.txt` one level
  deep, same host, 40 pages at most, and cites the page it read.
- **Evals** (`docs/evals.md`): write tools run as preflight by default, so a
  write with no preflight cannot be evaluated.
