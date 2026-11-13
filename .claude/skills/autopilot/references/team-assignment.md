# Team ticket assignment

Loaded by autopilot when the session holds an open Team Dispatch. Links
resolve against the autopilot skill directory.

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
   undecided, plans and approves as `M`. Other tickets of the phase are
   independent: build against the accepted contracts, never their code. For each interface contract the ticket builds against
   (`.babysit/contracts.yaml`), `bbs contract pin <id>`; a ticket that
   provides one changes its files first and runs `bbs contract check <id>`
   before reporting ([contracts](../references/contracts.md)).

   The ticket brief (`origin.design_doc`) and the parent prototype it links
   are the accepted design. Before building UI, open every state link in the
   spec's `## Prototype` section and build from the rendered states, not
   from the brief's prose; a link that does not load, or a state marked
   "not in" the prototype, is a missing input for `bbs team task ask`, never
   a guess. An `M`/`L` plan records only what the brief left
   open and any deviation, without restating it. Ticket-level `design-ui`
   only extends the parent prototype with a state the brief missed, linking
   the parent id; never a new layout — its subagent assignment carries that
   limit and the parent id. Changing a brief's decision —
   interface shape, data model, scope or a prototype flow — or needing a
   screen the prototype lacks is a material deviation for the lead (step 2).
2. **Plan approval** — every plan (`M`, `L`, or an `S` planned as `M`) is
   self-approved; nothing waits on the lead by size. Judge the plan with
   `../semantic-decision/SKILL.md` kind `human-review` against the accepted
   parent artifacts, write the five-line rubric with named evidence and run
   `bbs ticket approval publish --kind plan`, then resolve it yourself
   with `bbs ticket approval self-resolve --team <assigned id>
   --rubric-file <path>`; exit 4 means repair the plan or rubric here, at most
   twice. Exit 3/5 (the money/auth/irreversible-data floor), a third refusal, a `needs-human`/`blocked`
   judgment, or a material deviation from accepted scope or a shared
   contract: `bbs team task ask <task> --body` the lead with the artifact
   paths and wait in this session (exit 4 is a timeout: `ask --resume
   <question-id>`).
   `approved` continues; a redirect is revised here and asked again;
   `await` means a human holds it: run `bbs ticket approval await`, re-run
   it while pending, continue on `approved` (revise on a redirect), and end
   the Dispatch with `NEEDS_CONTEXT` only if it is still pending after two
   hours.
   Never implement on an unapproved plan record.
3. **Build** with focused checks only — typecheck, lint and the unit tests
   of the touched code, fixed until green — and report as soon as they pass:
   the phase commit is reviewed once and QA'd cumulatively right after
   integration, so a ticket never runs `review-pr` or `qa` on its own tree,
   whatever its size. The one exception: when a changed path hits the
   money/auth/irreversible-data floor or names privacy, tenant, erasure or
   retention, run `review-pr <assigned effort>
   --fix` on this ticket's diff before reporting (its `qa` still waits for
   the phase). `task done` applies this itself; a
   deferred report reads `GATE="deferred to phase review: …"`. A failed
   verdict or open finding of a gate you did run is never deferred. A check
   the assignment defers to a heavy profile is skipped and recorded as
   `DEFERRED_HEAVY=<profile>: <checks>` in the handoff and with
   `bbs ticket set-pointer deferred_heavy "<profile>: <checks>"`, leaving the
   `qa` verdict open.
4. **Report.** Require `bbs ticket readiness --action review --json` (`push`
   when heavy checks are deferred) for the gates your size owns, persist the checkpoint, handoff and gate
   evidence per `references/verification.md`, and report exactly once with
   `bbs team task done`; it re-checks readiness and refuses success with
   `GATE=not-ready` and the missing reasons, which this session fixes and
   then reports again — a later session would have to reload everything you
   hold. A `COMPOSE_WARNING=<sibling>: <files>` line is advisory: the lead
   resolves that conflict when the phase composes.
   Record workflow completion only when the gates your size owns and
   readiness are current. Never commit, push or open a PR.

A follow-up carries a new Dispatch and names its kind and trigger evidence:
`repair` picks up a session that ended without success and fixes the named
failure, `redirect` resumes from step 2 with the lead's answer — each then
re-runs the focused checks the change touches. A consolidated heavy-QA
assignment lists several tickets and writes each one's `qa` verdict with
`BABYSIT_TICKET=<ticket>` on that ticket's assigned head; a failure it fixes
itself in that ticket's worktree and re-checks.

The lead owns ticket decomposition, task classification and model selection before
launch. Honor its assigned model; record the supplied routing metadata and
observed launch settings in the handoff. If launch evidence conflicts with
the assignment, report the mismatch to the lead before work; never select a
replacement model or start a worker to fix it.
Keep each assigned skill in this session on its launched model; never delegate
the whole ticket, dispatch sibling Tasks or switch models. The review skill's
internal analysis fan-out and the QA executor subagent remain allowed under
SKILL.md § Current-session gates, and SKILL.md § Subagent steps applies in
full: `design-ui` on its pinned model, the others on this session's model.
Work beyond the ticket — another ticket's files or contracts, scope one
coherent plan cannot hold, a scope change — is a handoff to the lead with a
proposed split or change request, never absorbed silently. This explicit envelope takes
precedence over the workflow files' mode selection and the design-approval question;
ordinary direct autopilot invocations retain the full current-session loop.
