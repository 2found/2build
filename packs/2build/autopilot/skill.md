---
name: autopilot
description: "Deliver one ticket end-to-end from a requirement or accepted plan to a verified, releasable change left uncommitted in its checkout. Own implementation, review fixes, QA, and evidence across resumes; the default for any size of change, including a day-long one. Use team only for phased releases, independent phase verification or parallel work on disjoint parts."
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and stops.

# autopilot
A **goal proxy**: the skill owns init — durable ticket state, requirement,
plan — and the harness's work loop owns execution (`/goal` where supported),
with persisted `review-pr`/`qa` verdicts as the terminal condition. Gates
are proportional: under Team a ticket runs focused checks and leaves
`review-pr` and `qa` to its phase (step 5 of Team ticket assignment); a
follow-up already verified on its final tree may be sealed by
`team` with `bbs team seal --waive` and a reason. Record the gates you
own, but do not invent work to satisfy the ones it defers. Keep
state on disk; safe to re-enter until a terminal status prints.
**Autopilot is an assistant, not an orchestrator.** It works on the checkout
it was started in and never manages its own git topology: no branch cutting,
no worktrees, no commit, no push, no PR. Its work stays **uncommitted** in the
checkout: gates bind the content of the ticket's own change (`tree_digest`,
[verification.md](references/verification.md)), so nothing needs a
commit to be proven, and the checkpoint carries crash-resume. Who commits is
decided above it — the human after a direct run, or `team`, which seals each
ticket and lands **one commit per phase**
([git flow](../shared/git-flow.md)). The lead decomposes the project,
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
`../semantic-decision/skill.md` kind `human-review` with the current checkpoint,
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
- Follow [the preamble](../shared/preamble.md) and
  [Auto-Decision Framework](../shared/auto-decision-framework.md) —
  shared refs (`../shared/*.md`) are filesystem paths beside this skill's
  directory, so read them by path, not as `skill://`.
  Resolve the active harness from session metadata and exposed tools; use
  the preamble's `AGENT` / `SKILL_REF`, not the terminal brand or an installed
  CLI as evidence of which harness is running.
- Claude Code, Codex, and OMP use the same workflow and disk gates, on the
  same rule: every step runs in this session. Adapt skill loading and task
  tracking to the tools actually exposed. No Skill tool → read the resolved
  skill's `SKILL.md` in full and follow its references, preamble/telemetry,
  and output contract.
  No TaskCreate → use the harness's task/plan tool; if absent, checkpoint
  milestones on disk. Never invent a tool or claim a hook fired.
- Plain terminal, cmux, and Orca are presentation/transport choices, not
  model selectors. Use terminal-specific transport only when requested
  or already required by the invoking workflow; absence is not a gate.
- `/goal` instructions below apply only where that command is supported — and
  support is verified for the harness actually running, never inferred from
  the brand or its absence: all three supported harnesses expose it today
  (Claude Code as `/goal`; `codex features list` for Codex's `goals` feature;
  `omp config get goal.enabled` for OMP's built-in goal mode). Assume support
  unless that harness's own probe or command list says otherwise: dropping the
  handoff silently drops the design checkpoint, which is the worse error. Only
  on a harness that genuinely lacks it, after init continue the same work loop
  in this session, honoring `--stop-after` and any invoker-held approval
  checkpoint. At such a checkpoint report the artifacts and a native skill
  resume instruction, not an unsupported `/goal` command. A received `/goal`
  block on such a harness expresses the completion condition; it does not
  establish a Stop hook.
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
   them ([finding-unknowns](../shared/finding-unknowns.md)).
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
   project workflow (`.claude/workflows/<name>.md`). `NEEDS_CONTEXT` only
   when there is no ticket, requirement, plan, manifest, or branch work at
   all.
4. Seed the plan when the routed mode needs one (build or child mode, size
   above XS) or `--stop-after=plan` explicitly requests it:
   run `plan-draft` in this session, per the planning policy below — `plan.md` on
   disk is what a crashed loop recovers from. User-facing work routes through
   `design-ui` inside `plan-draft`; make sure that ran, so the spec and
   prototype exist *before* the `/goal` handoff — design is reviewed before
   implementation, not discovered after it. Stop here on `--stop-after=plan`.
   Size relaxes *only this step*: an XS change still gets the step-2 ticket
   and the verdict gates — there is no inline path.
5. Hand the work to the harness's work loop (below). Init never executes
   workflow steps; under `--auto` or without `/goal`, transition directly to
   execution.

### Planning runs in this session
`plan-draft` — including the `design-ui` it invokes when the work adds or
reshapes a user-facing surface — executes in the current autopilot session, on
the session's model. Never dispatch it to a native child, a second session, or
an external process: a routed planner is a second model, a second usage bill,
and a second context to reconcile, selected by a run that can see neither its
cost nor its state. There is no `--planner` flag and no per-step model
selector. The planning model is a **launch** decision: the human chooses the
session for a direct run; the lead selects the model before opening a ticket worker.
Autopilot does not select a model tier or reroute the session. The launched model
executes every assigned step in this session.

The step owns one writer, not one child: load and execute the real `plan-draft`
skill here, in this context, with the same bounded assignment a dispatched
child would have received — the resolved `SKILL.md` path; ticket id and
absolute repo/ticket paths; requirement and checkpoint paths; permission to
write the plan, design spec, prototype artifacts, pointers, `plan-draft`
verdict, and handoff — and no implementation, git, push, or close-out
authority.

Then inspect the artifacts, confirm they belong to this attempt, and read
`bbs ticket verdict-status --skill plan-draft`. Missing or inadequate output is
not a plan: re-run it in-session once, naming the gap. If the plan is
inadequate because this session's model cannot carry it, that is a launch
problem rather than a dispatch problem — stop with `NEEDS_CONTEXT` naming the
observed capability or artifact gap and durable resume pointers. The lead or the
human chooses the replacement session; Autopilot does not select another model.
Never silently replace an accepted plan. Record the session model, artifact
paths, and result in the checkpoint/handoff.
## The work loop (`/goal`)
### Autonomous entry (`--auto`)
`--auto` means nobody will paste a `/goal` block or answer a prompt: not a
human, and not an orchestrator woken just to type one. A Team ticket
assignment, `SPAWNED=true` and an open Team Dispatch imply it.
Record it once with `BABYSIT_TICKET="$TICKET" bbs ticket set-pointer
autopilot_auto true` so re-entry and resume keep it. Under it:

1. Never print the paste handoff or stop to wait for one.
2. If this agent can set its own goal — a goal/objective tool in
   **its own tool list** (for example Codex's when its `goals` feature is on, or OMP's
   goal mode) — set it with the completion condition from the template below
   and keep working in the same turn. A slash command only a user can type
   (Claude Code's `/goal`) is not such a tool; never ask anyone to type it.
3. Otherwise enter the work loop directly in this session and hold yourself
   to the same condition: do not end the turn until it holds or a terminal
   `DONE`/`NEEDS_CONTEXT`/`BLOCKED` status block is printed. Compaction or a
   crash resumes from the checkpoint like any cold re-entry.
4. Init's design checkpoint is judged in-session: run
   `../semantic-decision/skill.md` kind `human-review` on the plan/prototype
   and continue on `proceed`; repair `revise` once in place. `--stop-after`,
   invoker-held approvals (a lead plan approval) and the decision floors
   still stop the run; a genuine User Challenge ends it with a
   `NEEDS_CONTEXT` block through the invocation channel rather than a wait.

### Developer handoff
On a harness that supports `/goal`, `/goal <condition>` arms a Stop hook
that blocks stopping until the condition holds. Without `--auto`, autopilot
cannot arm it itself — after init, print the handoff and stop (`developer`:
the human copy-pastes it; orchestrators that still spawn with a prompt put
the block in the spawn prompt).
Without `/goal` support, skip this paste/stop protocol and continue execution
as specified above; `developer` alone never requires a `/goal` handoff.
If `SPAWNED` is already true, you are that process: follow Autonomous entry.
An open Team Dispatch (the launch prompt names it; `bbs team task show`
confirms it) is equivalent evidence even when a legacy state echo says
`SPAWNED=false`: honor the Task spec's effective `AGENT_ROLE=orca`, execute in
this turn, and use the spec's worker lifecycle instead of producing a
developer `/goal` handoff.
For a `developer` handoff **on a harness supporting `/goal`**, the handoff
**is the whole final message and must be the very last thing on screen** —
nothing after it. The template and mandatory copy-paste rules below apply
only to that supported handoff, not to direct execution or a native resume
instruction on other harnesses. Init may have run `plan-draft` /
`design-ui`, which print their own reports; do **not** let those be the last
thing the human sees. Condense any step-skill report into the pointer lines
below and drop the rest, so the copy-paste line is what the eye lands on. Lead
with a fixed **Review-before-you-paste** preamble carrying the pointers init
produced — one line each, omit a line only when that artifact does not exist —
then the `/goal` block, fenced on its own so it is one-click copyable:
```
Ready for <ticket>. Before you paste, review what will be built:
  plan:      <plan.md path>
  prototype: <prototype path>
Redirect the design now if it's wrong — otherwise you're one paste from done.

👉 Copy the block below and paste it into <the current agent> to build it:

/goal <ticket> is done: acceptance criteria verified on the uncommitted tree,
current review-pr and qa gates passed, no unresolved material findings,
release readiness checked, evidence and handoff persisted —
or a NEEDS_CONTEXT / BLOCKED status block printed verbatim.
Work it: <SKILL_REF>autopilot <workflow> <ticket>
```
The preamble is mandatory whenever `plan-draft`/`design-ui` produced those
artifacts — it is the design checkpoint, not decoration; keep it in plain
words and never assume the human knows git or babysit internals.
The preamble prints the current agent and `SKILL_REF`; substitute both values
in this template and never print the angle-bracket placeholders. The
`👉 Copy … paste it into <agent>` line is mandatory in every supported
`developer` `/goal` handoff — a non-technical user must never guess that the block
is a thing they paste. Orchestrators (non-`developer`) skip the preamble and
the copy-paste line and put only the `/goal` block in the spawn prompt.
Inside the loop (re-entry, orchestrator, `SPAWNED=true`, `--auto`), skip the
handoff and work. Cold session: recover from checkpoint, ticket files, workflow file,
git state first; warm session: keep going with what's in context. Treat the
workflow file as mode router + gate list, not a script — pick the mode from
durable state, honor the gates, and do everything between them the way a
direct session would. Checkpoint at milestones (plan seeded, implementation
verified, findings fixed, QA verdict), not per step. Mirror those milestones
into the harness's native task list at loop entry — rebuilt from checkpoint
+ `plan.md` on cold re-entry — and close each as its gate passes; step skills
add their finer tasks to the same list. The task list is the visible progress
view, disk stays the brain. End every pass with the status block below.

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
local check). Debug: `investigate`. Committing and closing out is the human's
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

An open Team Dispatch (`bbs team task show <task> --spec`) assigns either one
**project ticket** — one ticket of a release phase, in its own worktree — or a
**follow-up** on an existing ticket. Either way the whole assignment runs in this
session on its launched model: everything it loads stays warm in the prompt
cache for every later step, while a new session would load it all again. A
follow-up normally arrives in a fresh session and recovers from the ticket's
handoffs, not by re-exploring the code.

A ticket assignment runs the ticket's whole loop without stopping between
steps and ends when the ticket is ready for its phase's integration:

1. **Plan.** Run `plan-draft` scoped by the ticket seed/requirement and the
   accepted parent plan and design, which constrain it — read the sections the
   assignment names for this ticket, not the whole parent documents, since
   every later turn re-reads what you load first; reuse a current ticket
   plan on resume. An `XS` or `S` ticket skips this step and step 2: run
   `bbs ticket set-pointer ticket_size <assigned size>` (the phase-review
   deferral reads it, and an unset size counts as `M`), pin its contracts as
   below, and build straight from the ticket brief. An `S` seed that
   touches money, auth or irreversible data, or that its brief leaves
   undecided, plans and approves as `M`. Plan against the
   prerequisites' accepted contracts and their `plan.md`; their code may not
   exist yet. For each interface contract the ticket builds against
   (`.babysit/contracts.yaml`), `bbs contract pin <id>`; a ticket that
   provides one changes its files first and runs `bbs contract check <id>`
   before reporting ([contracts](../shared/contracts.md)).

   The ticket brief (`origin.design_doc`) and the parent prototype it links
   are the accepted design: an `M`/`L` plan records only what the brief left
   open and any deviation, without restating it. Ticket-level `design-ui`
   only extends the parent prototype with a state the brief missed, linking
   the parent id; never a new layout. Changing a brief's decision —
   interface shape, data model, scope or a prototype flow — or needing a
   screen the prototype lacks is a material deviation for the lead (step 2).
2. **Plan approval** — every plan (`M`, `L`, or an `S` planned as `M`) is
   self-approved; nothing waits on the lead by size. Judge the plan with
   `../semantic-decision/skill.md` kind `human-review` against the accepted
   parent artifacts, write the five-line rubric with named evidence and run
   `bbs ticket approval publish --kind plan`, then resolve it yourself
   with `bbs ticket approval self-resolve --team <assigned id>
   --rubric-file <path>`; exit 4 means repair the plan or rubric here, at most
   twice. Exit 3/5 (the money/auth/irreversible-data floor), a third refusal, a `needs-human`/`blocked`
   judgment, or a material deviation from accepted scope or a shared
   contract: `bbs team task ask <task> --body` the lead with the artifact
   paths and wait in this session (exit 4 is a timeout: `ask --resume
   <question-id>`).
   `approved` continues; a redirect is revised here and asked again; a park
   answer ends the Dispatch with `NEEDS_CONTEXT`.
   Never implement on an unapproved plan record.
3. **Await prerequisites.** With prerequisites assigned, run
   `bbs team await --prereqs <ticket>:<task>,...` in the foreground and
   repeat it on `AWAIT=timeout`. `AWAIT=not-ready` ends the Dispatch: report
   `BLOCKED` naming the prerequisite.
4. **Adopt prerequisites.** On `AWAIT=ready`, `await` has already adopted
   each prerequisite's sealed snapshot into this worktree as uncommitted
   changes (`SNAPSHOT=` lines). `AWAIT=conflict` lists `CONFLICT=` files:
   resolve their markers against the accepted shared interfaces. Record the
   adopted snapshots in the handoff; review this ticket's own changes, not the
   adopted ones. This is the only integration a ticket session performs —
   never fetch, merge, rebase, stash or reset. Then reconcile the plan with
   the adopted code: fix details in place; a scope, acceptance or
   shared-contract change is a material deviation and goes back through
   step 2.
5. **Build** with focused checks only — typecheck, lint and the unit tests
   of the touched code, fixed until green — and report as soon as they pass:
   the phase commit is reviewed once and QA'd cumulatively right after
   integration, so a ticket never runs `review-pr` or `qa` on its own tree,
   whatever its size. The one exception: when a changed path hits the
   money/auth/irreversible-data floor, run `review-pr <assigned effort>
   --fix` on this ticket's diff before reporting (its `qa` still waits for
   the phase). `task done`, `seal` and `await` apply this themselves; a
   deferred report reads `GATE="deferred to phase review: …"`. A failed
   verdict or open finding of a gate you did run is never deferred. A check
   the assignment defers to a heavy profile is skipped and recorded as
   `DEFERRED_HEAVY=<profile>: <checks>` in the handoff and with
   `bbs ticket set-pointer deferred_heavy "<profile>: <checks>"`, leaving the
   `qa` verdict open.
6. **Report.** Require `bbs ticket readiness --action review --json` (`push`
   when heavy checks are deferred) for the gates your size owns, persist the checkpoint, handoff and gate
   evidence per `references/verification.md`, and report exactly once with
   `bbs team task done`; it re-checks readiness and refuses success with
   `GATE=not-ready` and the missing reasons, or with `GATE=not-composing` and
   a `CONFLICT_WITH=<sibling>: <files>` line when a phase sibling already
   reported work this tree conflicts with (`bbs ticket adopt <sibling>`,
   resolve, review the resolution delta), which this session fixes and then
   reports again — a later session would have to reload everything you hold.
   Record workflow completion only when the gates your size owns and
   readiness are current. Never commit, push or open a PR.

A follow-up carries a new Dispatch and names its kind and trigger evidence:
`repair` fixes the named findings or failures, `refresh` re-runs steps 3–4
for the named prerequisites (adopting them — never a rebase, which moves the
base and reopens the whole review) (or re-reads a changed contract, adapts and
re-pins it), `redirect` resumes from step 2 with the lead's
answer — each then re-runs every gate the change invalidates. A `repair`
from phase integration QA names the failing criterion and the tested phase
commit; reproduce it in this ticket's worktree. A `repair` because the phase
does not compose names the colliding ticket: `bbs ticket adopt <ticket>`,
resolve every `CONFLICT=` file, rerun the affected gates. A consolidated heavy-QA assignment lists
several tickets and writes each one's `qa` verdict with
`BABYSIT_TICKET=<ticket>` on that ticket's assigned head, without code fixes.

The lead owns ticket decomposition, task classification and model selection before
launch. Honor its assigned model; record the supplied routing metadata and
observed launch settings in the handoff. If launch evidence conflicts with
the assignment, report the mismatch to the lead before work; never select a
replacement model or start a worker to fix it.
Keep each assigned skill in this session on its launched model; never delegate
the whole ticket, dispatch sibling Tasks or switch models. The review skill's
internal analysis fan-out and the QA executor subagent remain allowed under
Current-session gates below.
Work beyond the ticket — another ticket's files or contracts, scope one
coherent plan cannot hold, a scope change — is a handoff to the lead with a
proposed split or change request, never absorbed silently. This explicit envelope takes
precedence over the workflow files' mode selection and `/goal` handoff;
ordinary direct autopilot invocations retain the full current-session loop.

### Current-session gates (`review-pr`, `qa`)
Applies to every workflow's `review-pr` and `qa` steps, without an opt-in
flag. Both gates are judged in this session, on the session's model: no autopilot
step is ever dispatched to a child, a second session, or an external process.
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
   that revision, untracked files included. Adopted prerequisite work is
   context, not this ticket's diff.
   **Re-review the fix delta, not the whole ticket.** When each review gate
   begins, snapshot its tree without touching the index:
   `i=$(mktemp -u); GIT_INDEX_FILE=$i git read-tree HEAD && GIT_INDEX_FILE=$i git add -A && GIT_INDEX_FILE=$i git write-tree; rm -f "$i"`.
   Once that review's report is persisted, store the printed tree with
   `bbs ticket set-pointer review_tree <tree>`. A later review on the same
   base, requirement and plan — after `--fix`, QA fixes, or a repair Dispatch —
   targets `git diff <review_tree> <current snapshot>` (written to a patch file
   passed as the review target), at the resolved effort,
   with the previous report as context: finders read callers and enclosing
   code beyond the hunk, and every earlier finding the fixes claim to resolve
   is checked. Its report names the `review_tree` it builds on. A
   follow-up Dispatch reviews its delta at `low` effort, raised to the
   assignment's effort only when the delta touches money, auth or
   irreversible-data paths. Adopted work that touches no path in the
   reviewed diff needs no review and no QA re-run: run the focused checks it
   affects, and `task done` / `seal` keep the verdicts (`GATE="carried: …"`).
   Adopted work that does touch the reviewed diff is reviewed as a delta.
   Review the complete ticket diff again only when there is no
   `review_tree`, when the base moved, the requirement or plan changed, or
   when the delta is most of the ticket diff.
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
   Subsequent edits invalidate affected gates; run them again before finish.
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
   reuse a recorded decision only while its evidence is current. Fix in-scope
   defects autonomously; use
   `investigate` when the cause is unclear. Record evidence for refuted or
   nonblocking findings. Required behavior is never deferred just to get a
   green verdict; unrelated improvements stay outside the ticket.
2. After fixes, rerun affected checks. Any code change invalidates both
   gates: QA fixes return to review, and review fixes return to QA. Finish
   with review and QA covering the same final tree, base, requirement, and
   plan.
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
   gate evidence `bbs autopilot verification record` accepted for the current
   tree. Readiness does not grade a report, so also verify coverage and
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
started by other steps. A QA executor that returns without a `CLEANUP:` line
(crash, timeout) leaves its named browser session and target to you. Do not leave a server or browser running for review:
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
  push/PR hook and `bbs team seal` check. They are proportional: the lead may
  seal a simple or already-verified ticket with `--waive` and a reason, so
  re-record them after any change when you can, but a missing gate is a
  concern to report, not a reason to stall. `review-pr` only reports findings,
  so recording its gate is yours: unresolved material findings go in
  `unresolved_findings`, and `set-verdict` refuses both gate names.
- Git scope is exactly: `git init` plus its empty initial commit on an
  unborn repo, and reading history.
  Never commit, push, stash, reset, rebase or open a PR, and never cut a
  branch for itself — the work stays uncommitted for the human (`create-pr`)
  or the dispatching Team lead (one commit per phase). Step skills edit the
  working tree and never commit either.
- `INVOKER=developer`: lead every stop — handoff, `NEEDS_CONTEXT`, final
  status — with one plain-language sentence saying what happened and the
  exact next command to paste; a non-technical user must be able to keep
  the build moving without knowing git.
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
VERDICT: PLANNED | BUILT | FIXED | HANDOFF
SUMMARY: <acceptance results, final revision, review/QA evidence, release readiness>
CLEANUP: <owned processes stopped and verified; pre-existing/shared resources retained>
NEXT: commit it, or /bbs:create-pr for the verified change — under a team lead,
nothing: the lead seals the ticket and lands its phase
```
