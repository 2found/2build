# Foreman contracts — design note

Status: **shipped** (2026-10-17). User guide:
[contracts](../../.claude/skills/references/contracts.md).

## Goal

Let Foreman run more generally with fewer special cases, and make room for two
Foremen (or a Foreman and a human) to work in parallel without breaking each
other — without making the model carry more rules.

## Decisions

1. **The CLI suggests; the model judges.** `bbs foreman next <parent>` turns
   the project snapshot into suggested steps with commands. It adds no rule:
   every suggestion comes from the state and gates the verbs already enforce
   (`projectPhaseReasons`, seal readiness), so the skill can say "ask `next`"
   instead of spelling out the order, and a Foreman that sees a better path
   takes it — the verbs still refuse anything unsafe.
2. **A contract is files the repo already has.** `.babysit/contracts.yaml`
   maps an id to `files` (plus optional `owner`, `about`, `checks`). Its
   version is the digest of those files. No states, no messages, no registry:
   editing the files is publishing, git is the history, review is the PR/phase
   QA that already exists.
3. **Three enforcement points, all existing seams.**
   - ticket readiness: a pinned contract that changed in the worktree adds
     `contract:<id>:changed|gone` → the ticket's gates are stale;
   - `bbs foreman integrate`: a phase that changes a contract another Foreman
     owns is refused; the changed contracts and their checks are recorded on
     the phase candidate for phase QA;
   - phase QA runs `bbs contract check`.
4. **SPEC = the basic design.** The human↔Foreman agreement already exists:
   requirement, design, prototype, plan and `project.json`, with the
   `project-plan` approval pinned to their revision. Only the vocabulary is new.
5. **Foreman↔Foreman exchange reuses tickets.** A request to another Foreman
   is a ticket assigned to it (`bbs ticket assign`), read from its
   `bbs foreman inbox`. Ownership of the seam is the contract's `owner`.

## Left out on purpose

| Idea | Why not now |
|---|---|
| Contract lifecycle (proposed → accepted → fulfilled), a message log, a registry repo | Every state is something the model must track and the CLI must reconcile. Content digest + git + tickets covered every case we had. |
| One generic node type for project/phase/ticket; gates as a YAML table | A large refactor of working code for no new capability; `next` already gives the model one place to look. Revisit when a third node kind appears. |
| Write scopes (path globs per Foreman) | Contract ownership covers the seams that matter; base is still serialized by `land`, which refuses a moved base. |
| Breaking vs additive versions | Consumers' checks decide compatibility mechanically; a label would be a guess. |

## Prior art

- Central coordination contains error amplification better than independent
  agents; most multi-agent failures are unclear roles and handoffs and missing
  verification — so one owner per seam and checks at the boundary
  ([amux](https://amux.io/guides/ai-agent-orchestration-2026/),
  [rework](https://resources.rework.com/de/tools/ai-agents/best-multi-agent-frameworks-2026)).
- Commit the shared interface before parallel implementers start
  ([morphllm](https://www.morphllm.com/claude-orchestrator),
  [Augment](https://www.augmentcode.com/guides/api-contract-testing-agent-authored-specs)).
- Consumer-driven contract tests prove a seam, not a workflow — end-to-end QA
  stays ([aqua-cloud](https://aqua-cloud.io/contract-testing-benefits-best-practices/),
  [Gravitee](https://www.gravitee.io/corpus/gen-2464/quality-assurance-journal/contract-testing-for-ai-agent-apis-and-mcp-tool-schemas.html)).
