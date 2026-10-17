# Kickoff: requirement, basic design, phase plan

Read once when a project starts or its design reopens.

## Human control points

The run is autonomous between these points; nothing else waits on a human.

| Point | Who decides | Mechanism |
|---|---|---|
| Kickoff questions that change architecture or the phase plan | human, once, batched | step 1; none under `--auto` except a User Challenge |
| Basic design + phase/release plan | **human**; `--auto` delegates to Foreman | `project-plan` approval (step 2) |
| Production release of an `ask` phase | **human only**, under every posture | `bbs foreman release --request`; self-resolve refuses it |
| A failed release twice, or a stopped line | human | the CLI demands a new release go |
| Money/auth/irreversible-data plans, `bbs foreman hold` | human | the ticket parks; the rest continues |

An `auto` phase releases on the approved design: approving the design with
`release: auto` phases *is* the durable authorization for those releases.

## 0. Initialize

Read [runtime](runtime.md) for the exact bootstrap and cold-resume
protocol. Load the live Orca orchestration guide, adopt the current Foreman
session, initialize or recover the parent ticket, claim it, and bind its Orca
Run. Use the supported `bbs ticket ensure` / `bbs ticket init` commands; there
is no top-level `bbs init`. Persist identifiers and checkpoints as work advances.

Git has one flow and no policy to resolve ([git flow](../references/git-flow.md)):
workers never commit, Foreman lands **one commit per phase** on local base,
and only a release pushes. Committing a single ticket is an unfinished job,
never a delivery. Find the release profile release-prd would use (`RELEASE.md`, `.babysit/release.md` or the one
linked from `AGENTS.md`/`CLAUDE.md`); without one every phase is `release:
none`. Rebuild the task list from durable state. On resume, continue the first
unmet gate — `bbs foreman next "$PARENT"` names it (`snapshot --json` has the
detail) — without replacing live workers or replaying completed work.

## 1. Clarify the requirement

Foreman does this itself; it is the goal it keeps. Write the parent
`requirement.md`: outcome, audience, scope and non-goals, constraints
(deadline, platforms, data, compliance), assumptions, and open questions. Read
the codebase and docs enough to answer what they can. Classify each remaining
question with the Auto-Decision Framework: answer Mechanical and Taste
questions as recorded assumptions; ask the human only what would change the
architecture or the phase plan, in **one batch**, now — never mid-run. Under
`--auto` ask nothing but a User Challenge; record the rest as assumptions the
design review sees.

## 2. Basic design and phase plan

Read [project contract](project-contract.md),
[execution evidence](execution.md) and
[phases](phases.md). Dispatch one basic-design worker on the
**design** route in the parent checkout to produce the architecture
(`design.md`), one prototype of the whole product (`prototype.html`, N/A with
interfaces and examples for non-UI work), `plan.md`, `manifest.md` and
`project.json` with its `phases` — sized, ordered and release-safe per
[the phase plan rules](phases.md#the-phase-plan), and tickets per
[topology](topology.md#decompose-and-prepare-topology).

Where tickets or Foremen will build against each other in parallel, the
design names those seams as **interface contracts** — repo files indexed in
`.babysit/contracts.yaml`, delivered first ([contracts](../references/contracts.md)).
The basic design itself is the SPEC between the human and Foreman.

Present it for **human review before any phase ticket or production
dispatch**. Only explicit `--auto` delegates this review; persist that choice
across resumes. Under `--auto`, Foreman reviews the same artifacts itself and
records the approval with evidence. A material scope/design change returns
here; ticket plans are reviewed against the accepted design.

