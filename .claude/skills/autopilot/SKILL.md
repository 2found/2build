---
name: autopilot
description: "Deliver one ticket end-to-end from a requirement or accepted plan to a verified, releasable change left uncommitted in its checkout. Own implementation, review fixes, QA, and evidence across resumes; the default for any size of change, including a day-long one. In a folder with no project yet, start one from an idea (or none): requirement, prototype, scaffolded services, harness and a feature backlog. Use team only for phased releases, independent phase verification or parallel work on disjoint parts."
---
# autopilot
A **goal proxy**: the skill owns init — durable ticket state, requirement,
plan — then works the ticket in this session until the completion condition
holds, with persisted `review-pr`/`qa` verdicts as the terminal condition. Gates
are proportional: under Team a ticket runs focused checks and leaves
`review-pr` and `qa` to its phase (step 3 of Team ticket assignment); a
gate that is not needed is named with `bbs team task done --no-gate
"<why>"`. Record the gates you own, but do not invent work to satisfy the ones it defers. Keep
state on disk; safe to re-enter until a terminal status prints.
**Autopilot is an assistant, not an orchestrator.** It works on the checkout
it was started in and never manages its own git topology: no branch cutting,
no worktrees, no commit, no push, no PR. Its work stays **uncommitted** in the
checkout: a gate records what it checked and never goes stale because code
changed ([verification.md](references/verification.md)), so nothing needs a
commit to be proven, and the checkpoint carries crash-resume. Who commits is
decided above it — the human after a direct run, or `team`, which takes
each ticket's tree at its `task done` and lands **one commit per phase**
([git flow](../references/git-flow.md)). The lead decomposes the project,
creates the worktrees, starts one autopilot assistant per ticket, and
integrates, verifies and lands each phase. There is no parent/orchestrate exception inside
autopilot.
## Delivery contract
For production work, own the ticket through release readiness: implementation,
code review, fixes, regression checks, and QA are agent work, not a checklist
to hand back to the human. Keep the existing plan checkpoint and explicit
`--stop-after` boundaries; after execution starts, do not stop for routine
review findings, cosmetic choices, or locally repairable test failures.
Composed skills' Taste decisions are summarized in the handoff, not separate
approval requests. For a judgment about needing human input, use
`../semantic-decision/SKILL.md` kind `human-review` with the current checkpoint,
accepted direction, evidence and missing input. Follow the shared human-review
contract: repair local gaps, escalate genuine User Challenges, and honor all
explicit stop/approval boundaries before considering a model recommendation.

`DONE` means every acceptance criterion has current evidence, no material
finding remains, and the verified change can proceed to the authorized
release step without more implementation or verification. It does not mean
published or deployed. `DONE_WITH_CONCERNS` is only for nonblocking residuals;
a missing required check or unmet criterion is `BLOCKED`/`NEEDS_CONTEXT`, not
releasable. A report-only project workflow keeps its report outcome and must
not claim release readiness.

Record acceptance criteria and their check/evidence mapping in the existing
plan and handoff. Derive checks from the requirement and repository release/CI
commands, including relevant failure paths; add build, packaging, migration,
compatibility, or rollout checks only where the change needs them. A named
fallback must prove the same required behavior; a unit check cannot stand in
for a required browser journey merely because the browser is unavailable.

## Scope

Autopilot advances **one evidence-bounded ticket** through one workflow. It
never recursively invokes itself or turns a follow-up it noticed into accepted
scope: it names the follow-up and its evidence in the handoff, and the human
or a Team lead decides whether to open a ticket for it.
## Harness and terminal portability
- Follow [the preamble](../references/preamble.md) and
  [Auto-Decision Framework](../references/auto-decision-framework.md) —
  shared refs (`../references/*.md`) are filesystem paths beside this skill's
  directory, so read them by path, not as `skill://`.
  Resolve the active harness from session metadata and exposed tools; use
  the preamble's `AGENT` / `SKILL_REF`, not the terminal brand or an installed
  CLI as evidence of which harness is running.
- Claude Code, Codex, and OMP use the same workflow and disk gates, on the
  same rule: every step runs in this session, except the steps § Subagent
  steps hands to a subagent. Adapt skill loading and task
  tracking to the tools actually exposed. No Skill tool → read the resolved
  skill's `SKILL.md` in full and follow its references, preamble/telemetry,
  and output contract.
  No TaskCreate → use the harness's task/plan tool; if absent, checkpoint
  milestones on disk. Never invent a tool or claim a hook fired.
- Plain terminal, cmux, and Orca are presentation/transport choices, not
  model selectors. Use terminal-specific transport only when requested
  or already required by the invoking workflow; absence is not a gate.
- Autopilot never asks anyone to paste or type `/goal`. A run started under
  one — the human typed `/goal … autopilot …`, or a spawn prompt carries the
  block — takes it as the completion condition and works.
## Init (every fresh invocation)
1. Resolve the invocation: inline requirement, ticket id, named workflow, or
   resume. A checkpoint with work in flight means loop re-entry (below), not
   init.
2. Not a git repo yet → `git init -b main` plus an empty initial commit
   (`git commit --allow-empty -m init`), so the ticket has a base.
   Then ensure a ticket dir with `requirement.md` and `checkpoint.json`:
   ```bash
   ENSURE_OUT=$(bbs ticket ensure --no-branch)
   TICKET=$(printf '%s\n' "$ENSURE_OUT" | sed -n 's/^TICKET=//p')
   ```
   Parse, never eval — `TICKET_HOME` is an unquoted path. Then carry the id
   explicitly: an `export` here does not survive into the next tool call on
   harnesses with per-call shells, so every later `bbs ticket` /
   `bbs autopilot` invocation runs as `BABYSIT_TICKET="$TICKET" <cmd>` (or
   exports it inside the same shell invocation). `--no-branch` keeps the
   ticket on the checkout it was started in — autopilot never cuts a branch
   for itself and never diverts to a worktree. If the current checkout is
   already a ticket worktree (Team put you here), that is fine: work in
   place. When seeding `requirement.md`
   from free text, list open decisions explicitly instead of papering over
   them ([finding-unknowns](../references/finding-unknowns.md)).
   A requirement an agent drafted (a Soot draft, another agent's ticket) is
   confirmed by a person once before step 4, `--auto` included, with its
   after-release criteria rewritten and its effort estimate as the plan's
   size bound: follow
   [agent-drafted requirements](../references/agent-drafted-requirements.md).
   Before edits, record the starting HEAD, integration base, and pre-existing
   working changes in the checkpoint/handoff. Preserve that baseline on resume
   so the ticket's own changes stay distinguishable from what was already
   there.
   Stop here on `--stop-after=requirement`.
3. A parent a team lead owns (`bbs ticket get assignee` names a team) is
   the lead's: stop init and hand it back; never dispatch or merge its
   children here. Everything else stays in autopilot, whatever its size or
   length: several requirements, or a `manifest.md` nobody's team owns, run
   as this one ticket, their parts as ordered milestones of one plan in this
   session. Never stop, block, defer or ask the user to switch to `team`
   because the work is long or large. When phased releases or parallel work
   on disjoint parts would genuinely help, add at most one `Alternative:`
   line to the final handoff naming `team` — information, not a
   recommendation to stop. Run `builder`, unless the invocation names a
   workflow (`new`, or a project's `.claude/workflows/<name>.md`) or the
   checkout holds no project: no service code and no package manifest, at
   most notes, documents or a prototype. That checkout runs
   [`new`](workflows/new.md), the inline text as its idea, or none; a
   checkout that already holds a project never routes there on its own.
   `NEEDS_CONTEXT` only when a project exists and there is no ticket,
   requirement, plan, manifest, or branch work at all. A ticket whose `bbs ticket get assigned_to.role` is set and is not
   `swe` (human, marketing, ops work) is not agent work: stop with
   `NEEDS_CONTEXT` naming the role and the fix (`bbs ticket assign-to swe`).
   A `kind` of `milestone` or `phase` is a container: work its `swe`
   children (`bbs ticket find --milestone|--phase <id> --role swe --ready`),
   not the container itself.
4. Seed the plan when the routed mode needs one (build or child mode, size
   above XS) or `--stop-after=plan` explicitly requests it:
   run `plan-draft` per § Subagent steps — `plan.md` on disk is what a
   crashed loop recovers from. Work that adds or reshapes a user-facing
   surface runs `design-ui` first, so the spec and prototype exist before the
   plan and *before* the design approval — design is reviewed before
   implementation, not discovered after it. Stop here on `--stop-after=plan`.
   Size relaxes *only this step*: an XS change still gets the step-2 ticket
   and the verdict gates — there is no inline path.
5. Enter the work loop (below): directly under `--auto`, otherwise after
   § Design approval. Init never executes workflow steps.

### Subagent steps (`design-ui`, `plan-draft`, `investigate`, `perf-audit`, `implement`)
Most of these steps' tokens are exploration or code reading this session never
needs again, so they run in **one foreground subagent at a time**, never
alongside another editing step or a gate. This session keeps the checkpoint,
the judgment of what comes back, and every gate; a subagent never decides
readiness.

- **`design-ui` always runs in a subagent** — init step 4, builder step 2, a
  Team ticket's prototype extension. Claude Code: `model: "opus"`,
  `effort: "high"`; elsewhere the harness's top advertised model at high
  effort, or inherited when the subagent tool has no selector (record which).
  Its prompt carries this line verbatim:
  `Make it really good, use lots of tokens and iterate till you're proud of it.`
  Dispatch it from this session
  before `plan-draft`, never from inside a `plan-draft` subagent: nested
  delegation is rejected on some harnesses. A `plan-draft` or `implement`
  run in this session that needs `design-ui` dispatches it the same way,
  never inline. If it reports S2 unchecked for want of a subagent tool,
  dispatch that cold read from this session, to a fresh subagent that sees
  only the screenshots.
- **`plan-draft`, `investigate` and `perf-audit`: decide per run.** Default
  to a subagent on the session's model (inherited, no effort override) when
  the survey, failure or measurement spans more than a few files or signals;
  run in session when the needed
  context is already warm, or briefing would cost more than the work. Log
  the choice in the checkpoint. A `plan-draft` subagent plans from the spec
  at `pointers.design` and does not invoke `design-ui`: missing UI coverage
  comes back as a gap, and this session dispatches `design-ui` and re-plans.
  An `investigate` subagent runs in the repair loop with nothing else
  editing; it may apply its fix in ticket scope, and this session re-runs
  the reproducer before accepting it. A `perf-audit` subagent measures and
  writes its report; it changes no code — a fix it ranks is `implement` work.
- **`implement`: one subagent per plan milestone** when the plan is `M`/`L`
  with two or more milestones (a manifest's rows count), in plan order, on
  the session's model (inherited). Each gets its milestone, the earlier
  milestones' handoffs, and the plan's acceptance checks for it; it codes
  the milestone, runs those checks, and writes the implement handoff
  (`## Deviations` included). Before the next milestone this session reads
  that handoff and the diff and re-runs the checks; a failure goes to one
  more subagent for that milestone with the evidence, then to the repair
  loop. Record each accepted milestone in the checkpoint with its handoff
  filename. `XS`/`S` tickets, single-milestone plans and every review/QA fix are
  coded in this session. An `implement` subagent never invokes `design-ui`:
  a user-facing surface without a spec comes back as a gap, and this
  session dispatches `design-ui` first. A browser it opens uses a session
  name this session entered in the cleanup ledger, and it returns a
  `CLEANUP:` line.

Each assignment carries the resolved `SKILL.md` path; ticket id and absolute
repo/ticket paths; requirement, checkpoint, plan and design pointers; what it
may write (`design-ui`: spec, prototype, `pointers.design`; `plan-draft`: plan,
pointers, `plan-draft` verdict, handoff; `investigate`: the fix; `perf-audit`:
its report; `implement`: its milestone's code and handoff) — and no git,
push, ticket status, gate, close-out or autopilot authority. No subagent tool
→ run the step here and record why.

There is no `--planner` flag and no per-step model flag. The session model is
a **launch** decision: the human chooses it for a direct run; the lead selects
it before opening a ticket worker.
Autopilot does not select a model tier or reroute the session; `design-ui`'s
pinned subagent is the one fixed exception. On Claude Code an inherited
subagent is called with the session's own model alias: an omitted `model`
resolves to `CLAUDE_CODE_SUBAGENT_MODEL` when the project sets it
(`setup-project` seeds `sonnet`), not to the session's model.

When `plan-draft` returns, from a subagent or this session, inspect the
artifacts, confirm they belong to this attempt, and read
`bbs ticket verdict-status --skill plan-draft`. Missing or inadequate output is
not a plan: run it once more, naming the gap. If it is still inadequate
because the model cannot carry it, that is a launch problem — stop with
`NEEDS_CONTEXT` naming the observed capability or artifact gap and durable
resume pointers. The lead or the
human chooses the replacement session; Autopilot does not select another model.
Never silently replace an accepted plan.

Record each step's mode (subagent or session), model, effort, artifact paths
and result in the checkpoint/handoff. A cold resume continues at the
first milestone the checkpoint does not record as accepted: a handoff already
on disk for it is checked as above, not re-dispatched, and a subagent sent to
a milestone with code but no handoff is told the tree holds a partial attempt.
## The work loop
### Autonomous entry (`--auto`)
`--auto` means nobody will answer a prompt: not a human, and not an
orchestrator woken just to approve a design. A Team ticket
assignment, `SPAWNED=true` and an open Team Dispatch imply it.
Record it once with `BABYSIT_TICKET="$TICKET" bbs ticket set-pointer
autopilot_auto true` so re-entry and resume keep it. Under it:

1. Never ask for design approval or stop to wait for one.
2. If this agent can set its own goal — a goal/objective tool in
   **its own tool list** (for example Codex's when its `goals` feature is on, or OMP's
   goal mode) — set it with the completion condition below
   and keep working in the same turn. A slash command only a user can type
   (Claude Code's `/goal`) is not such a tool; never ask anyone to type it.
3. Otherwise enter the work loop directly in this session and hold yourself
   to the same condition: do not end the turn until it holds or a terminal
   `DONE`/`NEEDS_CONTEXT`/`BLOCKED` status block is printed. Compaction or a
   crash resumes from the checkpoint like any cold re-entry.
4. Init's design checkpoint is judged in-session: run
   `../semantic-decision/SKILL.md` kind `human-review` on the plan/prototype
   and continue on `proceed`; repair `revise` once in place. `--stop-after`,
   invoker-held approvals (a lead plan approval) and the decision floors
   still stop the run; a genuine User Challenge ends it with a
   `NEEDS_CONTEXT` block through the invocation channel rather than a wait.

### Design approval
Without `--auto`, a run whose init produced a plan or prototype asks once
before building — a question, never a block to paste — through the
invocation's escalation channel
([preamble](../references/preamble.md)): `developer` one `AskUserQuestion`,
`dashboard` a `--kind plan` approval record. The question is the design
checkpoint and the last thing on screen. Init may have run `plan-draft` /
`design-ui`, which print their own reports: condense them into the pointer
lines below, one per artifact that exists, in plain words and the user's
language; never assume the human knows git or babysit internals.
```
<ticket> is ready to build. Review what will be built:
  plan:      <plan.md path>
  design:    <design spec path>
  prototype: <prototype path and URL>
Approve to build it now, or say what to change.
```
- **Approved** → record it (`BABYSIT_TICKET="$TICKET" bbs ticket set-pointer
  design_approved true`, so a resume does not ask again) and enter the work
  loop in the same turn, as Autonomous entry steps 2–3.
- **Changes requested** → revise the design or plan from the note
  (§ Subagent steps) and ask again.
- **Nothing to review** — an `XS` change, a plan accepted before this
  invocation, verify mode — → no question; enter the work loop.

`--stop-after=plan` stops with the same pointer lines and a native resume
instruction instead of asking.

If `SPAWNED` is already true, you are an orchestrator's process: follow
Autonomous entry.
An open Team Dispatch (the launch prompt names it; `bbs team task show`
confirms it) is equivalent evidence even when a legacy state echo says
`SPAWNED=false`: honor the Task spec's effective `AGENT_ROLE=orca`, execute in
this turn, and use the spec's worker lifecycle instead of asking for design
approval.

**Completion condition** — what a goal tool is set with and what every run
holds itself to:
```
<ticket> is done: acceptance criteria verified on the uncommitted tree,
current review-pr and qa gates passed, no unresolved material findings,
release readiness checked, evidence and handoff persisted —
or a NEEDS_CONTEXT / BLOCKED status block printed verbatim.
```

Inside the loop (re-entry, orchestrator, `SPAWNED=true`, `--auto`), skip the
question and work. Cold session: recover from checkpoint, ticket files, workflow file,
git state first; warm session: keep going with what's in context. Treat the
workflow file as mode router + gate list, not a script — pick the mode from
durable state, honor the gates, and do everything between them the way a
direct session would. Checkpoint at milestones (plan seeded, implementation
verified, findings fixed, QA verdict), not per step. Mirror those milestones
into the harness's native task list at loop entry — rebuilt from checkpoint
+ `plan.md` on cold re-entry — and close each as its gate passes; step skills
add their finer tasks to the same list. The task list is the visible progress
view, disk stays the brain. End every pass with the status block below.
Outside a Team dispatch, mark the rung others plan by: `bbs ticket
set-status in_progress` at loop entry, `in_review` with the `DONE`
handoff (the human or `create-pr` closes it) and `blocked` with a terminal
`BLOCKED`, so a caller that only reads the ticket (Soot's company loop) sees
a run that stopped. Under a Team dispatch leave
status alone — the lead owns its children's lifecycle, and a stale
`in_progress` would read as a live worker and stop a follow-up dispatch.

When available, read `bbs autopilot snapshot --json` once at loop entry and
accept it only when its envelope is `schema_version: 2` and `ok: true`. Use its
identity, policy, artifact digests, gates, and obligations as the common
read-only packet; read every listed artifact that is required for the chosen
action. An unavailable packet falls back to reading the ticket files directly.
Never treat an `ok: true` snapshot or a projected gate as readiness or finish
permission: AP-03's readiness evaluator is the authorization boundary.
## Step Skills
"Run `<skill>`" means load and execute that skill through the active harness's
skill mechanism (or the full-file fallback above), never approximate its job
inline. Use `SKILL_REF` in prompts: Claude Code `/bbs:<name>`, Codex
`$bbs:<name>`, OMP `/<name>`. Use the installed skill name for tool calls.
Planning: `plan-draft`. Coding: `implement`. Landing review: `review-pr`.
QA: `qa` (no runnable target → record the fallback, use `browse` or a narrow
local check). Debug: `investigate`. Performance: `perf-audit` — a
requirement citing its report takes that report as the baseline and
`verify <report>` as acceptance evidence; `audit` runs only when none exists.
Committing and closing out is the human's
(`create-pr`) or the lead's phase landing — never autopilot's.
### Managed project evidence

Read [verification.md](references/verification.md) before the review/QA loop.
Its producer captures each gate's subject before checks and archives results
afterwards; it is the only writer of gate evidence and of `verdicts/review-pr.md`
and `verdicts/qa.md`. Under Team, keep parent acceptance IDs, dependency
revisions, and write scope in the durable plan/handoff; report milestones and
bounded waits through `bbs team progress`. These are evidence
responsibilities, not topology or sibling orchestration.

### Team ticket assignment

An open Team Dispatch (`bbs team task show <task> --spec`) assigns one project
ticket or a follow-up on one. Read
[team-assignment.md](references/team-assignment.md) before any other step: it
replaces this file's loop for that session (plan, approval, build and focused
checks, one `task done`; review and QA belong to the phase). This explicit
envelope takes precedence over the workflow files' mode selection and the
design-approval question; ordinary direct autopilot invocations never load it and retain the
full current-session loop.

### Current-session gates (`review-pr`, `qa`)
Applies to every workflow's `review-pr` and `qa` steps, without an opt-in
flag. Both gates are judged in this session, on the session's model: neither
gate is ever dispatched to a child, a second session, or an external process.
The one delegation is QA's mechanical execution (item 2), which hands
back evidence, never a verdict.
1. **Run review in the current session.** Always load and execute the real
   `review-pr --fix` skill in the current autopilot session, on the session's
   model. Never dispatch the whole review skill to a native child or external
   process, even when a smaller or different model is available. This
   execution boundary is load-bearing: `review-pr` at medium and higher effort
   uses the Task tool for its own independent finder and verifier agents, and
   making it a child creates nested delegation that some harnesses reject.
   The review skill's own Task fan-out is allowed and required. Honor its
   resolved effort; if the current session cannot provide the required
   fan-out, record `BLOCKED` rather than delegating the whole skill or silently
   lowering review effort.
2. **Judge QA here; execute it in one cheaper subagent.** After the review
   pass and its fixes, load the real `qa` skill here and own its judgment:
   load context, choose the code-level checks and write the case matrix
   (`qa` steps 1 and 3–4, the check commands listed in the matrix rather than
   run), triage failures and fix them (step 6), and grade and persist the
   verdict. Driving the browser, running the matrix and its checks, and
   screenshots and logs (`qa` steps 2, 3's runs, 5, 7–8) cost most of a QA
   run's tokens and need no judgment, so they go to **one foreground
   executor subagent** in `qa` **Executor mode**, on the harness's
   mid-tier model: Claude Code `model: "sonnet"`; elsewhere the advertised
   model one tier below the session's, or inherited when the subagent tool
   has no model selector. Its assignment carries the `qa` skill path, the
   ticket id and absolute repo/ticket paths, the matrix path, the target
   URL/env, the run number, the cases to run, and the browser session name you
   chose and entered in the cleanup ledger. It edits no code, runs no git and
   writes no gate or verdict; it returns only the Executor-mode block, and you
   read its report file only for the failures you triage. After a fix (and
   the review it re-opens), run a new executor on the failed and affected
   cases plus the final primary-journey pass. Record the executor's model,
   runs and report paths in the checkpoint. Run the whole QA in this session
   only when the harness has no subagent tool or the executor cannot reach
   the target, and record why. If the session lacks what a check needs (no
   browser, no runnable target), the skill's named fallback applies and the
   limitation is recorded; a missing capability is never a fabricated PASS.
3. **Run one mutating gate at a time:** finish the current-session
   `review-pr --fix` pass and its fixes before starting `qa`. Never run
   either gate concurrently with implementation or with the other gate. The
   session retains ticket/checkpoint ownership, and a gate run
   never invokes autopilot or recursively delegates another gate.
4. **Scope each gate from the ticket, not from memory:** the resolved skill
   path; ticket id and absolute ticket/repo paths; requirement, plan and
   relevant handoff paths; acceptance criteria, available check commands, QA
   URL/surface; permitted edits and no git/close-out authority. Read those
   artifacts rather than replaying the implementation narrative, so the gate
   judges the revision on disk, not what you intended to write. Require the
   actual skill, evidence paths, changed files, unresolved findings, and its
   status/verdict body. Require real runtime QA, including a relevant
   error/empty/validation/responsive case, or the skill's named fallback.
   Lacking required browser/runtime access, return that limitation, not a
   fabricated PASS.
   Record the ticket's starting revision before implementation and pass the
   complete ticket diff as the first review's target: the working tree against
   that revision, untracked files included.
   **Review once.** One `review-pr --fix` pass is the review gate. Its fixes,
   later QA fixes and repairs are verified by the checks they record, never
   by another review round; a code change never reopens a gate
   ([verification.md](references/verification.md)).
5. **Accept evidence, not completion text.** Record the session model,
   reviewed revision, report, fixes, and evidence paths in the
   checkpoint/handoff. A gate owns the checkout while it runs — no editing the
   same files until its report is in. Inspect each report and unresolved
   finding; persist each full report with
   `BABYSIT_TICKET="$TICKET" bbs ticket set-review --skill <review-pr|qa> --body-file <path>` unless
   the skill already persisted it, and record the gate with
   `bbs autopilot verification record` ([verification.md](references/verification.md)).
   Require evidence from this attempt and the current change, not an old
   `DONE` left on disk. A crash, missing report, or inadequate check is not a
   pass: fix and re-run the gate, or record `BLOCKED`. Never overwrite a failed
   gate with a parent-authored success without fixing and re-verifying.
   On cold resume recover any recorded gate attempt and artifacts; do not
   start a duplicate run while one is still in flight.
6. **Require cleanup evidence from QA.** Its verdict is incomplete without a
   `CLEANUP:` line confirming that its exact browser session and every
   QA-owned auxiliary process exited, while identifying any pre-existing or
   coordinator-owned resource intentionally retained. Do not infer cleanup
   from `STATUS: DONE` or from a browser screenshot.
### Repair until the final change passes
Apply this loop to every production workflow before its final handoff:

1. Triage every review/QA finding using semantic-decision kind `review-finding`
   with a reproduction you ran (a test, script or CLI call against the real
   code; name it when it cannot run), cited guards/callers and the review
   verdict ladder;
   reuse a recorded decision. Fix in-scope
   defects autonomously; use
   `investigate` when the cause is unclear (§ Subagent steps). Record evidence for refuted or
   nonblocking findings. Required behavior is never deferred just to get a
   green verdict; unrelated improvements stay outside the ticket.
2. After fixes, rerun affected checks. A fix never re-runs review: the
   review's own `--fix` edits complete it, and QA fixes are proven by QA's
   checks. A later edit never sends either gate back: nothing stales a
   recorded gate.
3. Persist failures, attempted fixes, and their results in the checkpoint or
   handoff. Retry with a changed hypothesis. Before the second attempt at a
   blocker, record in the checkpoint `premise: the fix assumes <X>` — the
   fact the failed fix and the next one both rest on — and observe X directly
   on the real artifact, recording `holds`, `fails` or `inconclusive`.
   `fails` stops fixing: hand the premise and its evidence to `investigate`.
   `inconclusive` continues but is reported as unconfirmed. A resume reads
   the recorded premise instead of re-checking it. Three failed attempts at
   the same blocker end in `BLOCKED` with reproduction and exact missing capability or
   decision. Never reset that count on resume, or count a live check/wait as a
   failure. Keep completing independent in-scope work before escalating.
4. Reconcile the acceptance/evidence mapping, refresh the checkpoint, and read both verdict bodies. Run
   `BABYSIT_TICKET="$TICKET" bbs ticket readiness --action review --json`.
   Inspect `ok` and `data.ready`, not just exit 0; any false/error result is
   unfinished. Markdown verdicts never satisfy readiness: it reads only the
   gate evidence `bbs autopilot verification record` accepted. Readiness does not grade a report, so also verify coverage and
   unresolved findings from the actual reports.
   If a sub-skill reports `DONE_WITH_CONCERNS` for blocked required coverage,
   retain its report and record the gate with the unmet check under
   `limitations` (the producer records it `BLOCKED`); a permissive fallback
   status cannot make production work releasable.
   Never rewrite old evidence to make it appear current.

Leave one release handoff: acceptance results, final revision, review and QA
evidence, nonblocking residuals, and the exact remaining release action. The
human should not need to repeat review or QA to discover whether it is ready.
### Process cleanup gate
Autopilot owns the lifecycle of every external process its steps start. Keep an
ownership ledger in the checkpoint/handoff as processes are launched: exact
browser session and namespace, supervised server/watcher/recorder/proxy
handles, simulator UDIDs and drivers. Record the relevant pre-existing state
first; a shared dev server, human browser, or Team-owned leased surface is
not yours to stop.

On every terminal path — success, `NEEDS_CONTEXT`, `BLOCKED`, or a failed step
that can still run cleanup — close owned resources and wait for exit before
printing the status block. `qa`/`browse` must perform their own scoped
finalizers; autopilot reads their `CLEANUP:` evidence and then closes anything
started by other steps. A QA executor or `implement` subagent that returns
without a `CLEANUP:` line (crash, timeout) leaves its named browser session
and target to you. Do not leave a server or browser running for review:
durable screenshots/logs are the handoff. Never use broad
`pkill`/`killall`, `agent-browser close --all`, or simulator-wide shutdown for
routine cleanup; preserve resources whose ownership is uncertain.

Verify the owned browser session is absent, supervised processes exited, and
the simulator state returned to its recorded baseline. Retry a scoped close
once. If any owned process remains, persist its exact resource/PID and cleanup
attempt, then return `BLOCKED`; neither release readiness nor a passing QA
verdict overrides this gate. A crash may prevent the finalizer, so browser
steps also set the bounded idle timeout required by `browse`, but that backstop
does not replace normal cleanup.

## Rules
- Disk state must always be enough for a cold session to resume — but disk is
  the backup, not the brain; in a live session use everything already learned.
- Full reasoning depth at every step; requirement and plan are single-pass on
  wording, not on thinking. `review-pr` and `qa` are the gates — the
  evidence `bbs autopilot verification record` accepts for them is what the
  push/PR hook and `bbs team task done` check. They are proportional: a
  gate that is not needed is named with `task done --no-gate "<why>"`, and a
  missing gate is a concern to report, not a reason to stall. `review-pr` only reports findings,
  so recording its gate is yours: unresolved material findings go in
  `unresolved_findings`, and `set-verdict` refuses both gate names.
- Git scope is exactly: `git init` plus its empty initial commit on an
  unborn repo, the `new` workflow's one skeleton commit on a repository it
  started from empty, and reading history.
  Otherwise never commit, and never push, stash, reset, rebase or open a PR, and never cut a
  branch for itself — the work stays uncommitted for the human (`create-pr`)
  or the dispatching Team lead (one commit per phase). Step skills edit the
  working tree and never commit either.
- `INVOKER=developer`: every stop — a question, a handoff, `NEEDS_CONTEXT`,
  the final status — follows [owner language](../references/owner-language.md):
  what happened, where to look, the exact next command to paste. A
  non-technical user must be able to keep the build moving without knowing
  git.
- Never force-push, drop data, or send external messages.
- Always run QA before final handoff and record its gate with
  `BABYSIT_TICKET="$TICKET" bbs autopilot verification begin --gate qa` and `record`
  (real checks, or the blocker under `limitations`). "Implemented but not QA'd" is
  incomplete; happy-path-only QA is incomplete — include at least one
  validation/error/empty/responsive case.
- Leave a clean handoff: no debug leftovers in the diff, checkpoint current.
  After edits that follow the step's checkpoint, run
  `BABYSIT_TICKET="$TICKET" bbs autopilot checkpoint --refresh` or the Stop-time audit flags it stale.
- Keep the final handoff short: checkout, files changed, QA evidence, next
  human action. A truly human-only decision → `NEEDS_CONTEXT` naming the
  exact missing input.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: PLANNED | BUILT | FIXED | HANDOFF | PROJECT_READY
SUMMARY: <acceptance results, final revision, review/QA evidence, release readiness>
CLEANUP: <owned processes stopped and verified; pre-existing/shared resources retained>
NEXT: commit it, or /bbs:create-pr for the verified change — under a team lead,
nothing: your task done records the work and the lead lands its phase
```
