# Contracts: the seams parallel work builds against

Two kinds of agreement keep parallel work from breaking itself. Neither is a
new document format; both are files the work already has.

| Between | Agreement | Where it lives | Who changes it |
|---|---|---|---|
| human ↔ Foreman | **the SPEC**: the basic design — `requirement.md`, `design.md`, `prototype.html`, `plan.md`, `project.json` | the parent ticket | Foreman; the human's `project-plan` approval pins its exact revision, so any edit needs a new review |
| ticket ↔ ticket, Foreman ↔ Foreman | **an interface contract**: an API doc, schema, types file, event list, CLI usage | repo files named in `.babysit/contracts.yaml` | its owner — a Foreman's phase, or anyone when unowned |

## When to declare one

Only where two pieces of work meet and one could break the other: a backend
and the UI that calls it built in parallel, two Foremen on one product, a
schema several tickets read. A seam inside one ticket is not a contract. Name
it in the basic design (`design.md` → "Contracts"), not later.

```yaml
# .babysit/contracts.yaml — committed with the code
api.search:
  owner: fm-catalog              # optional: only this Foreman's phases may change it
  about: novel search API        # optional
  files: [docs/api/search.yaml, api/src/search/types.ts]   # "dir/" = every file under it
  checks: ["bun test api/test/search.contract.test.ts"]   # the consumers' tests
```

A contract's **version is the digest of its files**: editing them publishes a
new one, git is the history, and there is nothing else to keep in sync.

## Commands

| Command | Use |
|---|---|
| `bbs contract list` | contracts in this checkout with version and owner |
| `bbs contract pin <id>...` | a ticket records the version it builds against |
| `bbs contract status` | `current`, `behind` (base has a newer one; read it), `changed` or `gone` (adapt, re-verify, re-pin) |
| `bbs contract check [<id>...]` | run the consumers' checks here; exit 1 on a failure |

## How the work uses it

- **Design**: put the contract's files in the first phase that needs them, as
  their own small ticket or the provider ticket's first step. Consumers build
  against the file — mocks and fixtures from it — not against the provider's
  code, so both sides run in parallel.
- **Ticket (consumer)**: `bbs contract pin <id>` while planning. If the
  contract later changes in the worktree, the ticket's readiness reports
  `contract:<id>:changed` and its gates are stale until it adapts and re-pins;
  Foreman sends it a `refresh`.
- **Ticket (provider)**: change the contract files first, then the code;
  run `bbs contract check <id>` before reporting.
- **Phase**: `bbs foreman integrate` records the contracts the phase commit
  changes (`contracts_changed`, with their checks). Phase QA runs them —
  `bbs contract check <id>` in the integration worktree — beside the criteria.
- **Two Foremen**: `integrate` refuses a phase that changes a contract another
  Foreman owns. Ask the owner instead: write a ticket with the change you
  need and `BABYSIT_TICKET=<it> bbs ticket assign <owner-foreman>`; it appears
  in `bbs foreman inbox <owner>`. Build against the version on base until the
  owner lands the new one.

Contract checks prove the seam, not the product: phase QA still runs the
journeys end to end.
