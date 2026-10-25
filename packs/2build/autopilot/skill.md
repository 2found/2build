---
name: autopilot
description: "Deliver one ticket end-to-end from a requirement or accepted plan to a verified, releasable change left uncommitted in its checkout. Own implementation, review fixes, QA, and evidence across resumes; use foreman for multi-ticket projects."
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
are proportional: a simple or docs-only ticket, or a follow-up already
verified on its final tree, may be sealed by `foreman` with
`bbs foreman seal --waive` and a reason, so record them when you can (a
follow-up should end by recording both on the final tree) but do not invent
work to satisfy them. Keep
state on disk; safe to re-enter until a terminal status prints.
**Autopilot is an assistant, not an orchestrator.** It works on the checkout
it was started in and never manages its own git topology: no branch cutting,
no worktrees, no commit, no push, no PR. Its work stays **uncommitted** in the
checkout: gates bind the tree's content (`tree_digest`), so nothing needs a
commit to be proven, and the checkpoint carries crash-resume. Who commits is
decided above it — the human after a direct run, or `foreman`, which seals each
ticket and lands **one commit per phase**
([git flow](../shared/git-flow.md)). Foreman decomposes the project,
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
releasable. Explicit prototype, recommendation-only, or audit-only workflows
keep their learning/report outcome and must not claim release readiness.

Record acceptance criteria and their check/evidence mapping in the existing
plan and handoff. Derive checks from the requirement and repository release/CI
commands, including relevant failure paths; add build, packaging, migration,
compatibility, or rollout checks only where the change needs them. A named
fallback must prove the same required behavior; a unit check cannot stand in
for a required browser journey merely because the browser is unavailable.

## Lifecycle loop

Autopilot advances **one evidence-bounded ticket through one archetype**. It
never recursively invokes itself or turns a suggested next stage into accepted
scope. A foreman may chain lifecycle tickets in a DAG; a serial run leaves the
next stage in its handoff.

The product loop is `prototyper -> builder -> grower -> maintainer`, with
`sweeper` inserted only when a measured simplification opportunity exists.
Promotion is evidence-gated:

- `prototyper -> builder`: the risky assumption was validated and the handoff
  names the production requirement; throwaway spike code is not promoted.
- `builder -> grower`: the feature is released or has an authorized experiment
  venue, and a target metric plus instrumentation are available.
- `builder|grower|maintainer -> sweeper`: current behavior is characterized and
  specific removable weight or a measured hot path was found.
- `grower -> maintainer`: a shipped experiment exposed a reliability, security,
  cost, dependency, or scale signal.
- `maintainer -> grower|builder`: analysis found a product experiment or a
  structural change; maintain does not smuggle that larger scope into its fix.

Every workflow handoff ends with `LIFECYCLE: <stage> -> <next|stop>` and
`TRIGGER: <observed evidence or missing signal>`. `stop` is a successful answer
when the next stage would require fabricated validation, unavailable production
data, or scope the ticket did not authorize. Under a foreman, those two lines
are durable routing evidence: create or unblock the next child only when the
trigger is satisfied.
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
   already a ticket worktree (a foreman put you here), that is fine: work in
   place. When seeding `requirement.md`
   from free text, list open decisions explicitly instead of papering over
   them ([finding-unknowns](../shared/finding-unknowns.md)).
   Before edits, record the starting HEAD, integration base, and pre-existing
   working changes in the checkpoint/handoff. Preserve that baseline on resume
   so the ticket's own changes stay distinguishable from what was already
   there.
   Stop here on `--stop-after=requirement`.
3. A parent carrying `manifest.md` is a multi-ticket project: stop init and
   hand it to `foreman`; never dispatch or merge its children here. Otherwise
   pick the archetype workflow ([archetypes](../shared/archetypes.md)):
   named one wins; else use `../semantic-decision/skill.md` kind
   `skill-route` with the requirement and archetype mandates to route by shape;
   ambiguous or ordinary production work → `builder`. Several *independent* requirements in one
   invocation → don't batch here: autopilot runs one ticket end-to-end;
   parallel dispatch belongs to the `foreman` skill (one visible worker per
   ticket, each running autopilot). `NEEDS_CONTEXT` only when there is no ticket,
   requirement, plan, manifest, or branch work at all *and* no archetype was
   named — a named archetype is direction enough to proceed.
4. Seed the plan when the routed mode needs one (build or child mode, size
   above XS) or `--stop-after=plan` explicitly requests it, regardless of archetype:
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
session for a direct run; Foreman selects the model before opening a ticket worker.
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
observed capability or artifact gap and durable resume pointers. Foreman or the
human chooses the replacement session; Autopilot does not select another model.
Never silently replace an accepted plan. Record the session model, artifact
paths, and result in the checkpoint/handoff.
## The work loop (`/goal`)
### Autonomous entry (`--auto`)
`--auto` means nobody will paste a `/goal` block or answer a prompt: not a
human, and not an orchestrator woken just to type one. A Foreman ticket
assignment, `SPAWNED=true` and an open Foreman Dispatch imply it.
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
   invoker-held approvals (a Foreman plan approval) and the decision floors
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
An open Foreman Dispatch (the launch prompt names it; `bbs foreman task show`
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
(`create-pr`) or foreman's phase landing — never autopilot's.
### Managed project evidence

Read [verification.md](references/verification.md) before the review/QA loop.
Its producer captures each gate's subject before checks and archives results
afterwards; it is the only writer of gate evidence and of `verdicts/review-pr.md`
and `verdicts/qa.md`. Under Foreman, keep parent acceptance IDs, dependency
revisions, and write scope in the durable plan/handoff; report milestones and
bounded waits through `bbs foreman progress`. These are evidence
responsibilities, not topology or sibling orchestration.

### Foreman ticket assignment

An open Foreman Dispatch (`bbs foreman task show <task> --spec`) assigns either one
**project ticket** — one ticket of a release phase, in its own worktree — or a
**follow-up** on a ticket this session already ran. Either way the whole assignment runs in this
session on its launched model; that continuity is why Foreman sends a ticket's
follow-ups back here instead of to a fresh worker.

A ticket assignment runs the ticket's whole loop without stopping between
steps and ends when the ticket is ready for its phase's integration:

1. **Plan.** Run `plan-draft` scoped by the ticket seed/requirement and the
   accepted parent plan and design, which constrain it; reuse a current ticket
   plan on resume; an XS ticket skips this step and step 2. Plan against the
   prerequisites' accepted contracts and their `plan.md`; their code may not
   exist yet. For each interface contract the ticket builds against
   (`.babysit/contracts.yaml`), `bbs contract pin <id>`; a ticket that
   provides one changes its files first and runs `bbs contract check <id>`
   before reporting ([contracts](../shared/contracts.md)).
2. **Plan approval, by ticket size** — the larger of the assigned size and
   the plan's `ticket_size`. Judge the plan with
   `../semantic-decision/skill.md` kind `human-review` against the accepted
   parent artifacts, write the five-line rubric with named evidence and run
   `bbs ticket approval publish --kind plan`. `S`/`M`: resolve it yourself
   with `bbs ticket approval self-resolve --foreman <assigned id>
   --rubric-file <path>`; exit 4 means repair the plan or rubric here, at most
   twice. `L`, or exit 3/5, a third refusal, a `needs-human`/`blocked`
   judgment, or a material deviation from accepted scope or a shared
   contract: `bbs foreman task ask <task> --body` Foreman with the artifact
   paths and wait in this session (exit 4 is a timeout: `ask --resume
   <question-id>`).
   `approved` continues; a redirect is revised here and asked again; a park
   answer ends the Dispatch with `NEEDS_CONTEXT`.
   Never implement on an unapproved plan record.
3. **Await prerequisites.** With prerequisites assigned, run
   `bbs foreman await --prereqs <ticket>:<task>,...` in the foreground and
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
5. **Build**, then **Review** with `review-pr <effort> --fix` at the
   assignment's effort (`medium`, or `low` when Foreman marked the ticket
   simple), then **QA**, all under the Current-session gates and the repair
   loop below: QA fixes return to review exactly as in a direct run. A check
   the assignment defers to a heavy profile is skipped and recorded as
   `DEFERRED_HEAVY=<profile>: <checks>` in the handoff and with
   `bbs ticket set-pointer deferred_heavy "<profile>: <checks>"`, leaving the
   `qa` verdict open.
6. **Report.** Require `bbs ticket readiness --action review --json` (`push`
   when heavy checks are deferred), persist the checkpoint, handoff and gate
   evidence per `references/verification.md`, and report exactly once with
   `bbs foreman task done`. Record workflow completion only when review, QA and
   readiness are all current. Never commit, push or open a PR.

A follow-up carries a new Dispatch and names its kind and trigger evidence:
`repair` fixes the named findings or failures, `refresh` re-runs steps 3–4
for the named prerequisites (or re-reads a changed contract, adapts and
re-pins it), `redirect` resumes from step 2 with Foreman's
answer — each then re-runs every gate the change invalidates. A `repair`
from phase integration QA names the failing criterion and the tested phase
commit; reproduce it in this ticket's worktree. A `repair` because the phase
does not compose names the colliding ticket: `bbs ticket adopt <ticket>`,
resolve every `CONFLICT=` file, rerun the affected gates. A consolidated heavy-QA assignment lists
several tickets and writes each one's `qa` verdict with
`BABYSIT_TICKET=<ticket>` on that ticket's assigned head, without code fixes.

Foreman owns ticket decomposition, task classification and model selection before
launch. Honor its assigned model; record the supplied routing metadata and
observed launch settings in the handoff. If launch evidence conflicts with
the assignment, report the mismatch to Foreman before work; never select a
replacement model or start a worker to fix it.
Keep each assigned skill in this session on its launched model; never delegate
the whole ticket, dispatch sibling Tasks or switch models. The review skill's
internal analysis fan-out and the QA executor subagent remain allowed under
Current-session gates below.
Work beyond the ticket — another ticket's files or contracts, scope one
coherent plan cannot hold, a scope change — is a handoff to Foreman with a
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
   complete ticket diff as the review target: the working tree against that
   revision, untracked files included. Adopted prerequisite work is context,
   not this ticket's diff.
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
   with reproduction, cited guards/callers and the review verdict ladder;
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
   handoff. Retry with a changed hypothesis; three failed attempts at the same
   blocker end in `BLOCKED` with reproduction and exact missing capability or
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
first; a shared dev server, human browser, or Foreman-owned leased surface is
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
  push/PR hook and `foreman seal` check. They are proportional: Foreman may
  seal a simple or already-verified ticket with `--waive` and a reason, so
  re-record them after any change when you can, but a missing gate is a
  concern to report, not a reason to stall. `review-pr` only reports findings,
  so recording its gate is yours: unresolved material findings go in
  `unresolved_findings`, and `set-verdict` refuses both gate names.
- Git scope is exactly: `git init` plus its empty initial commit on an
  unborn repo, and reading history.
  Never commit, push, stash, reset, rebase or open a PR, and never cut a
  branch for itself — the work stays uncommitted for the human (`create-pr`)
  or the dispatching foreman (one commit per phase). Step skills edit the
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
LIFECYCLE: <current stage> -> <next stage | stop>
TRIGGER: <observed promotion evidence or missing signal>
CLEANUP: <owned processes stopped and verified; pre-existing/shared resources retained>
NEXT: commit it, or /bbs:create-pr for the verified change — under a foreman,
nothing: Foreman seals the ticket and lands its phase
```
