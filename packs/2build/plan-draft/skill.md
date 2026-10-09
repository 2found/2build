---
name: plan-draft
description: Draft a technical plan before implementation. Use when the user asks for a plan, architecture, ticket breakdown, or wants to turn a requirement into plan.md without coding yet.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# plan-draft
Serve two readers with one concise plan:
- Human: catch misunderstandings and correct the direction before code —
  expected behavior, consequential interpretations/choices, system effects,
  scope expansion and uncertain assumptions, including those within the request.
- `implement`: preserve the agreed intent through durable decisions, boundaries,
  invariants, code/spec pointers and acceptance checks, without needing the
  planning conversation. Leave coding details open; new evidence can require
  revisiting a decision through the existing deviation/escalation contract.

The shared anchor is observable acceptance: both readers must mean the same
thing by correct and complete. Commands alone are not acceptance criteria.

Survey deeply; write only what serves those purposes. Task-level file lists
and coding steps belong to `implement`; cross-ticket dependencies, contract
ownership and rollout order belong in the plan.

Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`.

## Flow
1. Read the requirement and trace the affected flow through callers, services,
   modules, data stores and consumers, including relevant neighboring repos
   available in the workspace. Use architecture docs, code and recent similar
   commits; read `../shared/finding-unknowns.md` for git archaeology.
   Bound the survey by the change's reach, not just the edited directory. Record
   unavailable system context as an unknown, not an invented dependency.
   Derive implied cases: shared state/routes, permissions, empty/error states,
   concurrency, existing-data migration and query access paths at production
   volume. An aggregate/filter the indexes cannot bound needs a plan-level
   index/denormalization/cache decision. Actively look for evidence that
   challenges the requirement's assumptions, not just answers to listed questions.
   Cite material discoveries and derived scope/risks in **Unknowns**: what was
   learned, why it matters, and resolved/open status. Resolve derivable choices
   through `../shared/auto-decision-framework.md` into **Approach**, retaining
   a short discovery note when it changes the human's understanding even if
   already resolved; do not repeat the decision detail.
   Shared refs are filesystem paths beside this skill, not `skill://` resources.
2. Survey existing patterns before proposing new ones. For UI/frontend work,
   inspect components (`bbs design components`), tokens (`bbs design tokens`)
   and the nearest similar flow; for backend work, routes, data access and
   errors. When adding or reshaping a user-facing surface, inspect the design
   spec at `pointers.design` and its prototype for coverage of the planned flows
   and key states. Invoke `design-ui` to supply missing/outdated coverage before
   finalizing the plan. The spec and reviewable prototype are plan inputs and
   the baseline for checking the delivered UI; link them in **Design** with the
   key flows/states to compare. Do not drop them to meet the review budget.
3. Use `../semantic-decision/skill.md` with kind `task-size` and the canonical
   `../shared/ticket-size-rubric.md` to classify XS/S/M/L. With a ticket,
   persist via `bbs ticket set-pointer ticket_size <size>`; otherwise report
   size inline. Size controls planning depth, not a quota to fill.
4. Write **Approach** around the before/after system behavior: where logic and
   data ownership live, affected service/module relationships, changed API,
   event or schema contracts and their callers/consumers. Name reused patterns;
   justify a new one briefly. Include compatibility, deployment order and
   rollback constraints when the change requires them. A local change may say
   why it leaves the enclosing contract intact; do not invent system-wide work.
   Lead with consequential interpretations and decisions, with a short
   reason/tradeoff; expose meaningful ambiguity even within the requested scope.
   Explicitly flag major effects and work beyond the literal request: distinguish a
   necessary supporting change from an optional expansion, with its reason
   and acceptance status. Optional proposals are not implementation scope;
   route unresolved scope changes through the Auto-Decision Framework.
   Preserve invariants and give `implement` concrete code/spec anchors without
   prescribing every edit. If an unverified assumption could overturn the
   approach, do a bounded check or name it as a blocker before dependent work.
   Pick the visual that makes those decisions reviewable (below).
5. For L work or an explicit breakdown request, use semantic-decision kind
   `orchestration` with the proposed dependency boundaries to decide whether
   to split. If split, produce the ticket DAG and manifest below; splitting
   proposes work, it does not create tickets/worktrees or dispatch workers.
   Foreman owns orchestration.
6. Before handoff, check coverage and review cost (below), then re-check size.
   If ≥40% of in-scope items were deferred to follow-up tickets, downgrade one
   tier using the hook in `../shared/ticket-size-rubric.md`. Required child
   tickets in this plan are decomposition, not deferred scope.

## Review budget and format
Aim for roughly 20–30 readable rendered lines in the main plan; a large
architecture or multi-ticket plan may use around 50 when needed. A small fix
can be much shorter. Around 100–250 words is a useful reference for an ordinary
plan, including table text and diagram labels, not a hard cap for larger work.
Judge diagrams by their visible footprint, not Mermaid source lines. Size
increases survey depth, not automatically document length. These are review
budgets, not quotas: never cut a material decision merely to meet the target.

Spend that budget on the decision, system impact, consequential tradeoff,
material risk/blocker and proof of success. Cut repeated requirements, code
inventories, generic risks and implementation recipes. Use a diagram instead
of describing its edges again. Link evidence and detailed specs for drill-down;
keep decisions and blockers inline. For large projects, keep the dependency
shape in the plan and per-ticket detail in the manifest. If it still cannot fit,
identify the independent decisions/phases and summarize each briefly with a
link to its detail. If a material decision still needs more room, briefly say
why; do not hide consequential choices. Never meet the budget by packing
unreadable sentences or padding a small task to the target.

Use these labels for small plans; expand into brief sections only when helpful.
Omit optional sections with no content; never manufacture unknowns to fill a quota.
```markdown
# Plan
**Goal:** <observable outcome>
**Out of scope:** <boundary>
**Approach:** <chosen design, contracts/invariants, why; code/spec anchors>
**Impact:** <major system effects or work beyond the request, why needed, included vs proposed; omit when immaterial>
<architecture/flow diagram when useful>
**Tickets:** <decomposed only — DAG, integration gate and manifest link>
**Unknowns:** <material discovery or open question → evidence, implication, resolved/open; next check for open items>
**Verify:** <exact commands/checks + expected behavior, including a relevant failure/regression case>
**Design:** <user-facing work — design.md + prototype links, key flows/states for final comparison>
```
For interacting services/tickets, **Verify** includes the end-to-end integration
outcome; independent unit checks alone do not prove the system change.

## Visuals and decomposition
Choose by the decision being reviewed, independently of ticket size:
- Architecture, ownership or data-flow change: include a compact Mermaid
  component/flow diagram showing affected services/modules, stores and labeled
  contract/data-flow edges. Distinguish existing and changed relationships.
- Ordering, retries, races or lifecycle is the main risk: prefer a sequence or
  state diagram that exposes that behavior.
- Multi-ticket delivery: include a Mermaid DAG with stable seed keys, meaningful
  titles and prerequisite → dependent edges. Show independent work and where
  integration gates join it; explain non-obvious dependencies.

A simple local change needs no diagram. Usually one diagram suffices; include
both architecture and ticket DAG when they answer different material questions.
Diagrams replace repetitive prose, not contract definitions. Keep them to the
affected system slice and boundary neighbors, not an inventory of the platform.
Use plain Markdown/Mermaid; no external renderer or diagram skill is required.

For decomposition, write `manifest.md` beside `plan.md` (not the identity file
`manifest.yaml`). Use one row per proposed ticket:
`Seed key | Outcome/scope | Depends on | Owned contract/boundary | Acceptance check`.
Dependencies reference seed keys, not fabricated runtime ticket ids. Add shared
contracts, acceptance ownership and integration checks; keep the plan's DAG
consistent with these rows. Prefer children delivering bounded, independently
verifiable behavior; split by technical layer only when dependency/ownership
boundaries justify it. Distinguish true prerequisites from shared-file conflicts;
assign ownership or sequencing for conflicts instead of claiming false
parallelism. Runtime ids/status are Foreman's concern. For large breakdowns,
show a phase DAG in the plan and the complete ticket DAG in the manifest,
with explicit phase-to-ticket mapping.

## Handoff check
Check both readers: can the human spot a wrong interpretation and correct a
consequential choice, including system effects or scope expansion, without
opening code? Can `implement` identify what is included, what must remain true,
where to start and the evidence of completion without guessing design intent?
Both must share the same observable acceptance criteria. Preserve material
findings that corrected assumptions and give open unknowns a next check, marking
blockers. For user-facing work, confirm the prototype is available and covers
those criteria. Keep unresolved proposals distinct from accepted scope.
For decomposition, also check that every required outcome has an owner, all
DAG references exist, no dependency cycle remains and integration is covered.
If a gap would change the design or split, do a targeted follow-up survey and
repair it; do not loop on wording. Preserve the existing output/status contract.

## Native plan mode
Developer session already in plan mode → present the finished draft through
`ExitPlanMode` (native approval is the "plan accepted" checkpoint) and do the
writes — `plan.md`, `manifest.md` when decomposed, `set-pointer` — after approval,
since plan mode blocks them. Unattended runs never enter plan mode; artifact
creation does not bypass the caller's plan/project approval checkpoint.

## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: PLANNED(<XS|S|M|L>) | DECOMPOSED(<N>)
PLAN: <path or inline summary>
NEXT: implement, Foreman for decomposition, or resolve named blockers
```
