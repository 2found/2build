# Kickoff: the brief touchpoint

Read once when a project starts or its design reopens. This is the first of
the human's two touchpoints: they give the requirement, confirm the spec and
approve the basic design. Spend their attention here, once, so the run needs
none of it.

## The human's two touchpoints

| Touchpoint | The human | Mechanism |
|---|---|---|
| **1. Brief** | gives the requirement, answers one batch of questions, confirms the spec | step 1; the preamble's escalation channel |
| | approves the basic design and the release plan — which phases may ship mid-run (`auto`) | `project-plan` approval (step 2); `--auto` delegates it to you |
| **2. Handover** | reviews the built product, then gives the release go or feedback | the last phase's `release` approval ([delivery](delivery.md#hand-over)); human only, under every posture |

Nothing else waits on them; the few mid-run exceptions are listed in the
skill (*No mid-run questions*), each parking only what it blocks. Money, auth and irreversible-data work is not an exception: the
human approved it when they approved the design that names it, and you review
those ticket plans at `high` effort on their behalf.

## 0. Initialize

Read [runtime](runtime.md) for the exact bootstrap and cold-resume
protocol. Confirm Orca is reachable, adopt the current lead session,
initialize or recover the parent ticket, and claim it. Use the supported `bbs ticket ensure` / `bbs ticket init` commands; there
is no top-level `bbs init`. Persist identifiers and checkpoints as work advances.

Git has one flow and no policy to resolve ([git flow](../references/git-flow.md)):
workers never commit, the lead lands **one commit per phase** on local base,
and only a release pushes. Committing a single ticket is an unfinished job,
never a delivery. Find the release profile release-prd would use (`RELEASE.md`, `.babysit/release.md` or the one
linked from `AGENTS.md`/`CLAUDE.md`); without one every phase is `release:
none`. Rebuild the task list from durable state. On resume, continue the first
unmet gate — `bbs team next "$PARENT"` names it (`snapshot --json` has the
detail) — without replacing live workers or replaying completed work.

## 1. Clarify the requirement

The lead does this itself; it is the goal it keeps. Write the parent
`requirement.md`: outcome, audience, scope and non-goals, constraints
(deadline, platforms, data, compliance), assumptions, and open questions. Read
the codebase and docs enough to answer what they can. Classify each remaining
question with the Auto-Decision Framework: answer Mechanical and Taste
questions as recorded assumptions; ask the human only what would change the
architecture, the scope or the phase plan.

Then write the **spec** back in `requirement.md`, in the human's terms: who
it is for, the journeys they will be able to complete, numbered acceptance
criteria a reviewer can check by using the product, non-goals, constraints,
and the assumptions you took. Send it with the remaining questions as **one
batch** through the preamble's channel and wait: this is the cheapest place
in the project to be wrong, and the last place you ask. Fold the answers in,
note `Confirmed: <who>, <UTC time>` at the top, and move on; a spec the human
corrects goes round once more, not mid-run. Under `--auto` ask nothing but a
User Challenge; the spec and its assumptions go to the design review instead.

The spec is also the yardstick for touchpoint 2: handover feedback inside it
is a fix the team owes, feedback beyond it is a change request.

## 2. Basic design and phase plan

Read [project contract](project-contract.md),
[execution evidence](execution.md) and
[phases](phases.md). Dispatch one basic-design worker on the
**design** route in the parent checkout, using `--plan-worker`
(default: this lead's harness), to produce the architecture
(`design.md`), one prototype of the whole product (`prototype.html`, N/A with
interfaces and examples for non-UI work), `plan.md`, `manifest.md` and
`project.json` with its `phases`, with one decision-complete ticket brief
per seed ([project contract](project-contract.md#project-design-checkpoint);
a prototype or design the human supplied is reused, only its gaps are made)
— sized, ordered and release-safe per
[the phase plan rules](phases.md#the-phase-plan), and tickets per
[topology](topology.md#decompose-and-prepare-topology). When what ships has
public surfaces beyond the product itself — a docs or marketing site, SDK/CLI
reference, examples, e2e or fixture scripts, store listings — the last phase
carries a **launch** ticket that brings them in line with what ships, so the
handover does not leave "docs not synced" to the human.

Where tickets or teams will build against each other in parallel, the
design names those seams as **interface contracts** — repo files indexed in
`.babysit/contracts.yaml`, delivered first ([contracts](../references/contracts.md)).
The confirmed spec and the approved basic design together are the SPEC
between the human and the team.

Present it for **human review before any phase ticket or production
dispatch**, as one reviewable package: the prototype first, then the phase
plan in product terms (what the human can use after each phase, which phases
ship to production on their own and which wait for the handover), then the
alternatives weighed for the one-way decisions, then the architecture and
briefs for whoever wants them. Ask them to mark any phase
they want released mid-run as `auto`; the default is one release, at the
handover ([phases](phases.md#the-phase-plan)).

Only explicit `--auto` delegates this review; persist that choice across
resumes. Under `--auto`, the lead reviews the same artifacts itself and
records the approval with evidence. A material scope/design change returns
here; ticket plans are reviewed against the accepted design.

The basic design is where design effort pays off: one worker, warm, decides
architecture, contracts and UI for every ticket, and the human reviews it
once. Spend depth there (its `design-ui` iterates the whole prototype until it
holds), so the tickets only build.

