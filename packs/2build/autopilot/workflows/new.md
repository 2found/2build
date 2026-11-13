---
workflow: new
version: 1
description: Turn an idea, or none yet, into a runnable repository autopilot can build features in — requirement, approved prototype, minimal architecture, services scaffolded from starters, local infrastructure, deploy and release documents, the 2build harness and a feature backlog. Ends with one skeleton commit; creates no cloud resource and deploys nothing.
needs-state:
  requirement_md: optional
---
# new
Autopilot's project-start workflow. It runs when the checkout holds no
project (SKILL.md init step 3) or the invocation names it
(`/bbs:autopilot new "<idea>"`). The person at the keyboard may never have
used a terminal: every question and stop follows
[owner language](../shared/owner-language.md).

The whole run is one ticket. It needs no `plan.md` (init step 4 does not
apply: the steps below are the plan) and its design approval is the product
checkpoint in § prototype. After the interview that shapes the idea, the
human decides at exactly two points: that checkpoint, and before anything
that costs money or creates an account — which this workflow never does, so
the second belongs to the first `release`. Taste decisions are reported in
the handoff, never asked.

`architect`, `setup-project` and `harness-audit` are commands a human types;
they are not in the model's skill listing. Run each by reading its `SKILL.md`
by path in full and following it (SKILL.md § Harness and terminal
portability, the full-file fallback): `../architect/skill.md`,
`../setup-project/skill.md`, `../harness-audit/skill.md`. Like every
reference here, these resolve from the autopilot skill's directory, not from
this file's.

Checkpoint each step as it completes, under its heading's name:
`bbs autopilot checkpoint --ticket "$TICKET" --workflow new --step <step>
--status done_step`.

### resume and partial repos
Pick the step from disk, never from conversation memory: a step whose product
already exists and is still true is skipped, so a crashed run resumes and a
repo that already has a requirement, a prototype or a service only gets what
is missing. Never scaffold over an existing service directory.

| Product on disk | Step it satisfies |
|---|---|
| `docs/REQUIREMENT.md` with requirement IDs | idea |
| `prototype/` and `design_approved` on the ticket | prototype |
| `docs/architecture.md` | architecture |
| a service directory with `.babysit/starter.lock.yaml` or its own manifest | scaffold, for that service |
| a start command in `AGENTS.md` and a deploy config | infrastructure |
| root `.babysit/qa.yaml` and an `AGENTS.md` project map | harness |
| `docs/BACKLOG.md` and the skeleton commit | backlog |

`--stop-after=requirement` stops after § idea; `--stop-after=plan` after
§ architecture, with the pointer lines of SKILL.md § Design approval.

## idea
> produces: docs/REQUIREMENT.md + ticket requirement.md
1. **An idea was given** → restate it in one sentence and go to 2.
   **None** → ask at most three questions about who the person is, what they
   do all day and what annoys them or people they know; then offer three
   product ideas, each with who it is for, the problem, the smallest version
   worth using and why this person can build and sell it. They pick or edit
   one. With no channel that can answer (`--auto`, an orchestrator) and no
   idea, stop with `NEEDS_CONTEXT`.
2. **Interview** until these are settled: who uses it, the one problem, where
   they use it (phone browser, desktop browser), the smallest set of things
   it must do, what it will not do yet, the main screens, whether people sign
   in, whether it takes money, what it must remember. Ask one question at a
   time with choices; point out what is vague or contradicts an earlier
   answer; stop asking when the fields are settled or the person says
   enough, and decide the rest as recorded defaults. With no channel that can
   answer, skip the interview and record every open field as an open decision
   with its default ([finding-unknowns](../shared/finding-unknowns.md)).
   A product the starters cannot carry (a native mobile or desktop app) is
   said here, with the web-first version offered as the default.
3. Write `docs/REQUIREMENT.md`: user, problem, platform, requirements with
   IDs (`R1`, `R2`, …) each with how to tell it works, the subset marked
   **first release**, out of scope, main screens, open decisions with their
   defaults. The ticket's `requirement.md`
   (`bbs ticket path requirement --write`; init seeds none when no idea was
   typed) names the idea, points at that file and carries this workflow's
   exit criteria (§ backlog step 3).

## prototype
> produces: prototype/ + design spec + design_approved
1. Run `design-ui` (SKILL.md § Subagent steps) for the first-release screens
   in `docs/REQUIREMENT.md`, into `prototype/`, with one empty or error
   state. A product with no screen (an API only) skips this step's design
   and keeps its checkpoint.
2. **Product checkpoint**, one question through the invocation's channel:
   the product in three lines (who, the problem, what the first release
   does), what is left out, then "Is this the product you want?" with
   approve or say what to change. Open the prototype for the person first
   (the system's open command, or a local address they can click); never
   hand them a path to find. Changes → revise the requirement and the prototype, ask
   again. Approved → `bbs ticket set-pointer design_approved true`. Under
   `--auto` it is judged in session (SKILL.md § Autonomous entry step 4).

## architecture
> produces: docs/architecture.md
Run `architect new` on `docs/REQUIREMENT.md`. This is not a checkpoint: the
owner cannot review an architecture, so its owner summary goes into the final
handoff and only a decision that is the owner's (a paid third-party service
the first release cannot work without, where data must live) is raised, as
one question with a default.

## scaffold
> produces: service directories with verified starter locks
1. For each service the architecture names, from the repo root:
   `bbs bootstrap <dir> --template <id> --json` (`web`, `api`; lowercase
   kebab-case; the directory must not exist). It installs and checks for
   several minutes with no output: say so before starting, and give the
   command that long. Bootstrap publishes the project
   only when the template's own checks pass; `"verified": false` is a
   failure to repair, and `--no-verify` is never used. A part outside the
   starters is generated with the generator the architecture named.
2. A runtime the template needs and the machine lacks (`node`, `pnpm`,
   `bun`) is installed with the machine's package manager or the runtime's
   official installer and named in the handoff. An install that fails or
   needs a password is `NEEDS_CONTEXT` with the one command to run.
3. Several services: wire them together — distinct local ports, the web
   app's API address pointing at the API's, each in that service's
   `.env.example`.
4. A starter's `.github/workflows/` runs only from the repository root: move
   each service's check there as `<service>-check.yml` with its
   `working-directory` and a path filter.
5. Carry the approved look into the web service's `DESIGN.md` and token
   file, and put the product's name on the starter's home page, so the first
   thing the owner opens is recognisably theirs. Screens are not ported
   here: `prototype/` stays as the reference and each screen is built by its
   backlog item.

## infrastructure
> produces: one local start command + deploy config with placeholders
1. **Local**: whatever the architecture needs to run on this machine (the
   store, seed data) behind one start command recorded in `AGENTS.md`.
   Secret values are generated into local files the root `.gitignore`
   covers and never printed; `.env.example` carries names only.
2. **Production is prepared, never provisioned.** Write the deploy config for
   the target the architecture chose (default: 2server on one small VM,
   following <https://2found.dev/docs/2server/index.md>), with host, domain
   and credentials as `TODO(2build): … — decided at first release`. No cloud
   account, VM, DNS record, purchase, remote repository or deploy happens in
   this workflow; the first `release` does that with the owner's yes.

## harness
> produces: verdict:setup-project + qa:checked
1. Run `setup-project`: root `AGENTS.md` project map, the system and
   release documents, and the root `.babysit/qa.yaml`, which is the one runs
   read: it covers every service, built from what each starter shipped. A
   harness that loads only `CLAUDE.md` gets a root one importing `AGENTS.md`.
2. Run `harness-audit` scoped to this repo, with fixes authorized: repair
   every finding it proves.
3. Gates, each recorded per [verification](references/verification.md):
   `review-pr --fix` on the files this workflow wrote by hand (wiring, CI,
   config, documents — not starter output, which its release verified),
   passed by path: the tree is untracked, so there is no diff to read. Then
   `qa` on the local stack: every service starts with the one command, the
   home page renders, the API answers its health check, and one not-found or
   error case behaves. Apply SKILL.md § Repair until the final change passes
   and require `bbs ticket readiness --action review --json` to report ready.

## backlog
> produces: docs/BACKLOG.md + skeleton commit + handoff
1. Write `docs/BACKLOG.md` and link it from `AGENTS.md`: every requirement,
   first release first, ordered so each item builds on the ones above and is
   one autopilot ticket. The first item is the smallest journey a user can
   finish end to end. Each item:
   ```markdown
   - [ ] **<what the user can do after it>** (R1, R3)
     Paste: `/bbs:autopilot "<one-line requirement> — R1, R3 in docs/REQUIREMENT.md, screens in prototype/<file>"`
   ```
   in the preamble's `SKILL_REF` form.
2. **Skeleton commit**, the one commit autopilot ever makes (SKILL.md
   § Rules): only when this run started from a repository with no project
   files, after readiness reports ready and `git status` shows no secret or
   ignored local file staged — `git add -A && git commit -m "chore: project
   skeleton"`. Never push, add a remote or tag. A repository that already
   held a project keeps the work uncommitted like any other ticket.
3. Exit criteria: the product checkpoint is approved; every service starts
   with the one command; both gates are recorded and readiness is ready;
   every requirement is in the backlog; each `TODO(2build):` left is listed.
4. Handoff, to [owner language](../shared/owner-language.md): what now
   exists and the one command and address that open it; the architect's
   owner summary (what it is made of, what it will cost per month once
   live, what was left out); decisions taken on the owner's behalf with
   their defaults; then the first backlog item's line as the next command.

**Stop conditions**

- `NEEDS_CONTEXT`: no idea and nobody to ask, a runtime install that needs
  the person, or a decision only the owner can make that has no safe default.
- `BLOCKED`: a starter that will not verify, a local stack that will not
  start, or a gate that fails after the repair loop.

**Final status**
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: PROJECT_READY
SUMMARY: <services, local address, checks run, TODO(2build) count, backlog items>
CLEANUP: <owned processes stopped and verified; pre-existing/shared resources retained>
NEXT: <the first docs/BACKLOG.md Paste line>
```
