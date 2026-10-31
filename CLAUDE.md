# CLAUDE.md

## 1. Think Before Coding

**Don't hide confusion. Surface tradeoffs.**

Before implementing:

- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## 2build

2build is a 2found product for product engineering, with a Claude Code and Codex skill pack for **autonomous** workflows — scheduled runs, background jobs, CI loops, anything where no human is at the keyboard to approve or course-correct. Repository: `2found/2build`. Follow [BRANDING.md](BRANDING.md) for names; `bbs`, `bbs:` and `.babysit` remain compatibility identifiers.

One ticket runs the `builder` autopilot workflow; a multi-ticket project runs
under `team` (formerly `foreman`). A new skill earns its place only by owning a 2build contract,
evidence format or workflow boundary — see [docs/skills.md](docs/skills.md).
A skill that only makes sense when a human asks for it sets
`disable-model-invocation: true`, so it stays a `/bbs:<name>` command without
costing every session a listing line; never set it on a skill a workflow
invokes through the Skill tool.

## Working principles

Skills here should prefer decisions Claude can make and verify alone over decisions that need a human in the loop.

When writing or adapting a babysit skill:
- **Only Team depends on Orca** — keep its transport, lifecycle, and launch
  instructions under `team/`; inject worker adapters at dispatch. Other skills
  and shared preambles consume generic inputs and return artifacts/status.
- **Write only babysit-specific context** — assume a capable model knows ordinary engineering, debugging, planning, and writing practices. Keep commands, schemas, state/ownership contracts, thresholds, product policy, and non-obvious failure constraints. State each rule once in its owning file; link to it from consumers. Remove general tutorials, motivational prose, repeated rationale, and redundant examples instead of moving them into another loaded reference.
- **Prefer decisions over prompts** — default to deciding and logging the decision. Reach for `AskUserQuestion` only when the alternative is a wrong assumption that would land wrong code (see below).
- **Strong self-verification** — run the type-checker, tests, or browser check before declaring done; don't rely on a human to notice regressions.
- **Bounded blast radius** — never force-push, never drop DBs, never send messages without explicit durable authorization (see top-level instructions on reversibility).
- **Observable, not steerable** — emit telemetry (see below) so the human can audit *after the fact* instead of steering in the moment.
- **Fail loud, fail local** — on unresolvable ambiguity, stop with a clear `BLOCKED`/`NEEDS_CONTEXT` status rather than guessing silently.

### Loose skills, strict workflows

Skills and workflows enforce different layers of rules:

- **Workflows enforce strict prerequisites** — they're the orchestration layer.
  When a workflow routes into a step that dispatches a sub-skill, the workflow
  hard-gates on inputs the sub-skill needs (`pointers.requirement`, `pointers.plan`,
  `origin.plan` for sub-tickets, a resolved ticket, current verification evidence).
  See `.claude/skills/autopilot/workflows/*.md` and the workflow frontmatter
  `needs-state:` declarations for the canonical pattern.
- **Skills relax those rules** — a skill can be invoked many ways: composed by
  a workflow (where the gate already ran upstream), by `/bbs:<skill>` from a
  developer terminal where the user has pasted a requirement/plan into context
  instead of writing it to disk, by an orchestrator that supplies docs via
  conversation, or as part of a partial recovery after a crash. A skill that
  refuses to run because `pointers.requirement` is empty cripples every
  non-workflow invocation. Instead, prefer **graceful fallback**: read the doc
  from `pointers.<key>` when present, fall back to conversation context when
  not, and only emit `NEEDS_CONTEXT` when neither source has the input.

The five most common over-strict patterns to avoid in skills (loosen if you
find them — workflows already cover the strict path):
1. **Doc-existence gates** — `pointers.{requirement,plan,design}` missing → don't
   exit 1; read from conversation context, fall back to disk-glob, only
   `NEEDS_CONTEXT` if both are empty AND the skill genuinely cannot proceed.
2. **Ticket-id required** — derive from branch when present, work without it
   when not (skip ticket-state writes silently, log a one-line note).
3. **Branch shape** — `feat/<ticket>_*` should be a hint, not a precondition.
   A skill running on `main` with conversation-supplied context is a valid
   shape.
4. **Git-dirty refusal** — pushed/dirty/clean is the workflow's concern (it
   wraps release gates with the appropriate policy). A skill that does
   read-only or contained work shouldn't refuse a dirty tree.
5. **Git protocol in the skill** — git belongs to Team
   ([git flow](.claude/skills/references/git-flow.md)): autopilot and every
   skill it runs edit the current checkout and leave the work uncommitted;
   Team owns worktrees, seals, the one commit per phase, and landing. The
   one sanctioned exception is `qa`, which takes the surface lease
   (`bbs ticket surface acquire/release`) itself when a shared resource needs
   it — that is test-surface management, not topology. Other skills only edit
   the working tree. A skill invoked directly (e.g. via `/goal`) operates on
   the current checkout as-is and verifies the surface it tests actually
   serves the change, instead of enforcing how it got there.

Rule of thumb: when adding a gate to a skill, ask "does the workflow that
calls this skill already enforce this?" If yes, the skill check is dead
weight that breaks legitimate non-workflow invocations. Push the gate up to
the workflow and let the skill assume it ran.

### Decisions run through the Auto-Decision Framework

The *core* of babysit is `.claude/skills/references/auto-decision-framework.md`.
It classifies every decision as **Mechanical**, **Taste**, or **User
Challenge**, gives 6 principles for auto-answering, and routes User Challenges
to the right surface for the invocation mode. Every skill in the pack references
it — the Mechanical/Taste classifier and the "replaces judgment, not analysis"
rule are the core of how babysit decides unattended.

When adding or editing a skill, don't write new ad-hoc prompts. Classify the
decision point and let the framework route it.

### Escalation channels — design skills accordingly

Every skill always runs autonomously: decisions go through the Auto-Decision
Framework, and skills *never* prompt mid-flight for taste, style, or cosmetic
choices. The only thing that varies between runs is the **delivery channel**
for `NEEDS_CONTEXT`, picked by `AGENT_ROLE` (or legacy `GT_ROLE`):

- `AGENT_ROLE=developer` (default, no env var) — render `NEEDS_CONTEXT` as a
  single `AskUserQuestion`. The human is at the Claude Code terminal.
- `AGENT_ROLE=dashboard` — publish the question as a durable approval record
  (`bbs ticket approval publish`) and block on `approval await`. The human
  answers in the web dashboard, where the artifacts are readable; nobody is
  watching this terminal.
- Caller-supplied channel — the outer invocation provides its escalation
  adapter. Team injects its worker transport contract; member skills do not
  depend on that runtime.
- `AGENT_ROLE=mayor|general|scanner|...` — emit the structured `NEEDS_CONTEXT`
  block. An orchestrator (babysit-office, gastown, cron) relays via its own
  channel. `AskUserQuestion` here would hang the run.

When you write a skill, don't branch the *behavior* by mode — branch only the
*delivery surface* for User Challenges and the final taste-decision gate. The
analysis, artifacts, and decision logic are identical either way.

**The operational rules — when to escalate, the `NEEDS_CONTEXT` format the
orchestrator expects, the `INVOKER` values — live in
[.claude/skills/references/preamble.md § Escalation channels](.claude/skills/references/preamble.md#escalation-channels).**
That file is loaded at skill-invocation time regardless of which repo the skill
runs in. *This* `CLAUDE.md` is only in context when someone is working inside
the babysit repo itself; non-`developer` invocations from babysit-office or
gastown won't see it. Keep runtime rules in preamble.md; keep authoring
guidance here.

`autopilot` is the skill that makes the rest of babysit usable unattended. Core units plan, implement, review, and QA; domain skills stay directly invocable. `autopilot` is a **goal proxy**: init owns durable state (ticket, requirement, plan) on the current checkout — it never manages git topology — then Claude Code's `/goal` owns the work loop — inside it the model works free-form with full context, the workflow file is a mode router + gate list rather than a script, and the persisted `review-pr`/`qa` verdicts are the terminal condition.


When to reach for it (and how to think about it when editing it):

- **The composition problem it solves is context, not control flow.** Chaining `plan-draft` → `implement` ad-hoc can lose the plan and handoff state if the session crashes or gets compacted. `autopilot` runs the workflow in one session, and checkpoints all state to disk (`checkpoint.json`, `plan.md`, `requirement.md`, `handoffs/`) after each step. A fresh session after a crash re-reads the workflow + checkpoint and picks up at the next step — no conversation memory required.
- **Prefer `/bbs:autopilot <workflow> <ticket>` over hand-chaining skills** whenever one ticket needs >1 heavy skill, or when the user's ask could be "build this whole thing." Inline free-text (`/bbs:autopilot <one-line requirement>`) routes to the `builder` workflow, which creates the ticket, seeds `requirement.md`, and picks its own one-ticket mode (child / implement / build / verify) from ticket state. A decomposed parent routes to `team`; autopilot never orchestrates its children.
- **Workflows are markdown, not code.** `.claude/skills/autopilot/workflows/*.md` (builtins) and `.claude/workflows/*.md` (per-project). Steps are `## <step-id>` headings with optional `> needs:` / `> produces:` directives. Workflow frontmatter declares `needs-state:` prerequisites so autopilot's Assign phase can route deterministically. Adding a new workflow is a file, not a refactor. See [authoring.md](.claude/skills/autopilot/references/authoring.md).
- **When editing autopilot or its workflows**, disk state must always be *sufficient*: on cold start or resume, re-read the workflow file and checkpoint, re-derive `$TICKET` through the identity ladder (`BABYSIT_TICKET` → manifest cwd-match → branch regex). That's the crash-survival contract — anything that makes resume *require* in-context state is a regression. But sufficiency is not amnesia: within a continuous session the model keeps using what it already learned; a rule that forces a healthy session to behave like a cold one caps run quality at the worst case.

### Human checkpoints shape where workflows stop

Workflows are split along the four points where a human actually adds value:

1. **Requirement accepted** → `requirement.md` on the ticket. Autopilot drafts it in Flow steps 1–2 and stops at `--stop-after=requirement` if requested; requirement drafting is part of autopilot, not a separate skill.
2. **Plan accepted** → `plan.md` on the ticket. Owner: autopilot init via `plan-draft`; user-facing work routes through `design-ui` inside it, so the plan carries the UI spec + prototype **before** the `/goal` handoff — the handoff is where design is reviewed ahead of implementation (builder build mode covers the case init didn't seed it); stops at `--stop-after=plan` if requested. **Team (the product-team skill, formerly Foreman) meets the human twice. Brief: it clarifies the requirement, writes the spec back with every question in one batch, and — once the human confirms it — presents one basic design (architecture, whole-product prototype, a decision-complete brief per ticket, and a release-phase plan in `project.json`) for human approval before any phase ticket; `--auto` explicitly delegates that approval and persists across resumes (but never a release go, and never a contract that moves a seed out from behind the handover go). Handover: the last phase is the only `ask` phase — the CLI rejects `ask` earlier — and its release request carries the recap; earlier phases are `none` unless the human marked them `auto` at design approval. Nothing between waits on the human: ticket plans are approved inside the ticket session (`XS`/`S` none, `M`/`L` self-approved by the worker as the lead's delegate via `approval self-resolve --team <id>`, escalating to the lead only on the floor or a material deviation), and a money/auth/irreversible-data plan under a human-approved design is approved by the lead at high effort, because the human approved it with the design.** A team lead writes the same approval-record verdict the dashboard writes, via `bbs ticket approval self-resolve`, after filling the same rubric with named evidence and logging it to `decisions.jsonl` — no explicit grant required, so an overnight or cron-driven batch does not stall on the first worker that adds a component. Opt back into human-held with `bbs team hold <id>` (a `hold:` block on `~/.babysit/foremen/<id>.yaml`, naming who held and when); release with `hold release` (effective at the next checkpoint — nothing is rolled back). Optional bounds only: `bbs team grant <id>` (`--hours`, `--max`, `--tickets`; `--unbounded` must be typed) narrows default autonomy; `grant revoke` returns to unbounded default autonomy and does **not** force human-held. Two things no posture reaches: money, auth and irreversible-data plans under a design the lead approved for itself escalate to a human, and a rubric that can't be filled ends in `BLOCKED` with the gaps named rather than an approval or a wait.
3. **QA ready** → work implemented, reviewed, checked with `qa` or a named fallback, and left uncommitted in the checkout. Owner: `builder` (implement / build / child / verify modes) — the default end-to-end stop. For a multi-ticket project, `team` owns this checkpoint project-wide, one release phase at a time: one supervised autopilot session per ticket (a bbs Task/Dispatch in an Orca terminal on its own worktree) running plan → approval → await + adopt prerequisites (`bbs team await`) → implement → review → QA in a single Dispatch, a phase's tickets launched in parallel as one batch so the lead wakes once per phase, that ticket's follow-ups on a fresh session in the same worktree, a shared resource serialized on `bbs ticket surface acquire`, each passing ticket sealed (`bbs team seal`), then **one commit per phase** (`bbs team integrate`) that `bbs team land` fast-forwards local base to at once, cumulative QA of every criterion so far on exactly that landed commit (a failure fixed forward as a fix commit; release waits on it), then the **handover**: the last phase's release request (`bbs team release --request`, never self-resolvable) carries the recap; the human approves (release via `release-prd`), or sends feedback, which the team fixes within the spec and hands over again (beyond the spec it is a contract change back at the design review); `auto` phases released mid-run on the approved design; a failed release stops the line. `bbs team readiness --phase` enforces that order. A durable parent report separates execution, delivery and cleanup; `bbs team complete` renders it with project state into `recap.html` for the human, and the lead's terminal stays open afterwards. Verdicts and readiness are always verified from disk.
4. **PR ready** → a human reviews the QA handoff and, if the team wants a PR, invokes `create-pr`. No workflow crosses this checkpoint: autopilot never commits, and Team's only commits are the phase commit (and its QA fix commits) it lands on local base; nothing is pushed before phase QA passes and the release. A single ticket committed, pushed or PR'd on its own is unfinished work, not a delivery.

When adding or editing a workflow, be explicit about which checkpoint it stops at, and make sure the final step's handoff comment ends with a `Next:` line pointing at the human's next action (read + accept plan, review QA evidence, run `create-pr`, etc.). A workflow that crosses a checkpoint without stopping should say so in its frontmatter description (see `builder.md`).

Pick the workflow whose stop-point matches the checkpoint you want to own. `/bbs:autopilot <workflow> <ticket>` runs the whole thing end-to-end in one session; if it crashes mid-workflow, re-dispatching resumes from the last checkpoint on disk.

## Skill inventory by `INVOKER` compatibility

When composing an autonomy workflow (scheduled run, orchestrator pipeline, CI
loop), pick only from the **`INVOKER`-agnostic** column. The
**`developer`-only** skills hang when run without a human — use them from a
terminal or wrap them in an orchestrator that can relay `NEEDS_CONTEXT`.

| Compatibility | Skills |
|---------------|--------|
| **`INVOKER`-agnostic** (safe to chain unattended) | `agent-first-docs`, `analytics-review`, `autopilot`, `browse`, `create-pr`, `design-ui`, `harness-audit`, `implement`, `investigate`, `phased-build`, `plan-draft`, `qa`, `recon`, `release-prd`, `review-pr`, `semantic-decision`, `setup-project`, `team`, `test`, `token-audit`, `triage`, `ui-audit` |
| **`developer`-only** (require a human at the keyboard) | None; unresolved setup policy uses the caller's `NEEDS_CONTEXT` channel. |

Team reaches its parent human design checkpoint by default; unattended callers
must explicitly pass `--auto` to delegate routine project design reviews. Child
reviews, artifact requirements, holds, safety floors and release modes still apply.

Rules of thumb when wiring a workflow:
- Treat "`developer`-only" as a hard stop for non-`developer` orchestrators.
  If you need that capability unattended, add a code path that emits
  `NEEDS_CONTEXT` instead of calling `AskUserQuestion`.
- Before adding a new skill to the `INVOKER`-agnostic column, confirm it
  follows the single-mode pattern in
  [preamble.md](.claude/skills/references/preamble.md) and routes decisions
  through the Auto-Decision Framework.

## Layout

```
babysit/
├── bbs              # the multicall binary (gitignored; `go build -o bbs ./cmd/bbs`)
│                    #   bbs secrets   .babysit/.env loader + env resolve/is-set/prompt + qa.yaml
│                    #   bbs config    ~/.babysit/config.yaml, + workspace (multi-repo registry)
│                    #                 and repo (<repo>/.babysit/config.yaml)
│                    #   bbs autopilot checkpoint + timeline runner behind the autopilot skill
│                    #   bbs ticket    ticket identity (the big subcommand); `ticket env` derives
│                    #                 slug / branch / ticket from git remote + branch
│                    #   bbs upgrade (+ upgrade check), dashboard, team (alias foreman), …
│                    #   bbs hooks <name>   release gate, session writer, repo pre-commit
│                    #                 (compiled subcommands, not files)
│                    #   bbs setup     installer — builds bbs + links it into ~/.local/bin/
├── hooks/             # command-hook manifest + OMP extension adapter (see docs/artifact-gated-approval.md)
├── tests/             # shell + python suites for bins, workflows, and autopilot integration
│                      #   (tests/fixtures/lib/ — shared shell library, lock.sh)
├── docs/              # roadmap, identity, workspaces, operations, artifact-gated-approval
├── web/               # dashboard SPA (Vite/React) over ~/.babysit state; release
│                      #   builds stage it into internal/webui/dist for //go:embed
│                      #   (scripts/build-webui.sh), so brew-only installs can serve it
└── .claude/
    └── skills/        # see "Skill inventory by invocation mode" above
```

Skills are namespaced with the `bbs:` prefix when installed globally (e.g. `bbs:implement`).

## Workspaces — three meanings, one registry

"Workspace" names three different things here: an **Orca terminal** (a visible
worker's tab), the **worktree pool** (`<ticket>_<slug>` checkouts — in Orca's workspace dir
when cut from an Orca terminal, else `.babysit/worktrees/`),
and the **registry workspace** (`bbs config workspace` — a named list of repos that
form one product). When you write output or docs, meaning 3 always carries its
name ("workspace acme"), meaning 1 always carries the `Orca` prefix, and
meaning 2 is called a worktree. See [docs/workspaces.md](docs/workspaces.md)
for the registry's files, the `RELATED_*_REPO` fallback rule, and the
`BABYSIT_HOME` test guard.

## Ticket identity

A ticket is identified by three signals, in priority order:

1. **`BABYSIT_TICKET`** env var — set by autopilot's §0.X Workspace phase or
   by `bbs ticket session attach <id>`. Always wins.
2. **`tickets/<ticket>/manifest.yaml`** — durable identity anchor; one row
   per repo with `name / branch / canonical / worktree / base / pushed`.
3. **Branch regex** `^(feat|fix|chore|bug|refactor|hotfix)/<ticket>_<slug>` —
   the legacy fallback. Pre-`manifest.yaml` ticket dirs still resolve here.

`bbs ticket resolve` walks the ladder; conflicts (e.g. `BABYSIT_TICKET=A` on
a `feat/B_…` branch) exit 2 with a 3-line BLOCK. There is exactly one
identity codepath. Schema lives in [docs/identity.md](docs/identity.md).

Sessions persist at `~/.babysit/sessions/<id>.yaml` via the
`bbs hooks session-writer` plugin hook (SessionStart + PostToolUse(Bash),
ticket derived from the cwd's worktree dir or branch) — the preamble's
session-writer block is best-effort only, since skills aren't guaranteed to
execute it. `bbs ticket session list / attach / end` manage them; `attach`
echoes `export BABYSIT_TICKET=…` so a fresh shell can recover identity after
a crash.

## Install

```
go run ./cmd/bbs setup --full       # builds bbs → ~/.local/bin/, prints Claude Code, Codex and OMP plugin commands
go run ./cmd/bbs setup --uninstall
```

## Releasing — version bumps

Use [.claude/skills/release-prd/SKILL.md](.claude/skills/release-prd/SKILL.md)
with [RELEASE.md](RELEASE.md) for the complete delivery and post-release contract.
`--local` runs checks/build/distribution on the operator's machine without CI
runtime; its workflow guard restores original workflow states afterward.
Autopilot still stops at its existing QA handoff and does not invoke release.

When bumping the version (any change to `VERSION`), **always update the plugin
manifests in the same commit: `.claude-plugin/marketplace.json` for Claude Code,
`.codex-plugin/plugin.json` for Codex, and `package.json` for OMP**. The plugin
loaders use those files to detect upgrades — a stale version can leave users on
old skills.

Five fields must stay in sync across plugin ecosystems:

| File | Field | Plugin |
|------|-------|--------|
| `VERSION` | bare version string, e.g. `1.4.2` | all |
| `.claude-plugin/marketplace.json` | `metadata.version` | Claude Code |
| `.claude-plugin/marketplace.json` | `plugins[0].version` | Claude Code |
| `.codex-plugin/plugin.json` | `version` | Codex |
| `package.json` | `version` | OMP |

Quick check: `grep -r "version" .claude-plugin/ .codex-plugin/ package.json VERSION` — all five should show the same value. CI enforces this too: `.github/workflows/release.yml` fails the run when they disagree, rather than shipping a plugin that misreports its own version.

**Bumping VERSION on `main` is the release.** `release.yml` validates all five
version fields, calls the full test workflow, then tags the verified source.
Dependent distribution jobs build the four CLI archives (with the dashboard
embedded), the 2build pack and versioned docs/download metadata, commit the
formula's checksums, and publish the complete draft last. Ordinary pushes with
an already tagged version do not release. A manual `v*` tag must match `VERSION`;
workflow dispatch with an existing tag resumes an unpublished release.

The version is never derived from commit messages. Tagging and distribution
stay in one dependency graph: `GITHUB_TOKEN`-created tags do not start another
workflow. Published assets/tags are immutable; fixes need a new version.
`DOWNLOADS.md`, `release.json` and `checksums.txt` are assets of each release and
its body links the matching docs. 2build ships a CLI with an embedded web
dashboard, not a separate native desktop app.

## Telemetry

Logged events land under `~/.babysit/analytics/skill-usage.jsonl`. Because babysit runs unattended, telemetry is the *primary* feedback channel — treat it as load-bearing, not decoration.
