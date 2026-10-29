# Quality and design controls

Status: decided 2026-10-29; partly implemented in 1.115.0.

| # | Control | Outcome |
|---|---|---|
| 1 | Interrogate with runs | **Moved.** Not in `review-pr`'s verify phase, which is recall-biased (PLAUSIBLE findings are kept, so a run only changes a label) and already 18% of tokens. Instead autopilot's repair triage requires a reproduction that was run, or named when it cannot run. |
| 2 | Arena-lite basic design | **Reduced.** Several candidates plus a judge would double the one warm design session. The basic-design `design.md` instead opens with `## Alternatives considered`: criteria from the spec, then 1–2 structurally different shapes for each one-way decision and why each lost. The lead's brief bar rejects a one-way decision with no alternative weighed. |
| 3 | Evidence ladder and three-state verdict | **Not adopted.** `qa` already blocks on missing credentials and grades code-read-only evidence D; `review-pr` has its own ladder. A third vocabulary would restate rules owned elsewhere. |
| 4 | Attack the premise | **Adopted** in autopilot's repair loop, as prose: premise recorded and observed before the second attempt; `fails` hands to `investigate`. |

The rest of this document is the original proposal. Four controls, all inspired by pstack (MIT, Copyright (c) 2026
Lauren Tan). Concepts are re-expressed in bbs terms; no pstack text is copied.

1. **Interrogate with runs** — `review-pr` correctness findings must be proven by execution.
2. **Arena-lite basic design** — `team` compares candidate designs before the one human design checkpoint.
3. **Evidence ladder and three-state verdict** — one vocabulary for `review-pr` and `qa`.
4. **Attack the premise** — a repair loop tests its assumption before the last attempt.

Shared rules are defined once, in section 3. Sections 1, 2 and 4 link to it.

**Deliberate deferral.** Judging by a model from a different family is out of scope
for now. Every judge below runs in the same family as the author. Section 2 names the
bias this leaves and the mitigation it uses. Revisit when a second family is installed
and a judge from it can be dispatched from bbs.

---

## 1. Interrogate with runs (`review-pr`)

### Problem

The verify pass in `review-pr` (`.claude/skills/review-pr/SKILL.md`, "Verify phase")
has one verifier read the diff and the candidate, then return CONFIRMED / PLAUSIBLE /
REFUTED. CONFIRMED means "can name the inputs that trigger it and quote the line".
That is a reading, not a proof. A plausible-sounding race or off-by-one can be
CONFIRMED with no run behind it.

### Change

A correctness finding is CONFIRMED only when the verifier has **run** a repro and the
output shows the wrong behavior.

- **Repro.** The verifier writes the smallest command that exercises the candidate
  (a test case, a script that imports the changed function, or a CLI call) and runs it.
  The finding keeps the exact command and the output excerpt (≤ 20 lines).
- **Where it runs.** Only in the local worktree that holds the reviewed diff, under the
  same trust as the repo's own tests. Repros never write to that tree. They use a
  temp dir or `git worktree` for any file they create.
- **PR target without a local checkout.** No execution. Such candidates stay PLAUSIBLE
  and name the repro that would confirm them. Running code from a PR we have not
  checked out locally is not allowed.
- **Budget.** One repro per candidate, 120 s timeout. A repro that times out or
  cannot build is PLAUSIBLE with the reason, not CONFIRMED.
- **Scope.** Correctness angle only. Cleanup, altitude and conventions findings keep
  their current verdicts and do not need a run.
- **REFUTED** keeps its current meaning, and now also accepts a run that shows the
  behavior does not occur.

Output: the existing finding fields stay. The repro goes into `failure_scenario`
as `repro: <command> → <output excerpt>`. The `ReportFindings` schema is unchanged,
so no host change is needed.

### Acceptance

- A seeded fixture with an off-by-one in a changed function: the finding is CONFIRMED
  and its output shows the wrong value.
- The same finding on a PR target with no checkout: PLAUSIBLE, with the repro named.
- A cleanup finding: no repro, no change in verdict behavior.
- A candidate whose repro passes: REFUTED, with the run output as its reason.

---

## 2. Arena-lite basic design (`team`)

### Problem

The basic design (`team/references/kickoff.md` step 2, `project-contract.md`) is one
worker producing `design.md`, `plan.md`, `manifest.md` and `project.json`. The human
approves that one design. Nothing compares it with an alternative shape. A wrong
architecture is found after several phases are built.

### Scope

Applies once per project, in step 2, to the one-way decisions: architecture, data model
and migrations, interfaces between tickets, and the phase plan. It does **not** apply to
per-ticket plans. A second arena on an approved design is wasted work.

Skip it when the requirement has one viable shape. The lead writes the reason into the
decision trail ([team-decision-trail.md](team-decision-trail.md), `phase: design`, `result: skipped`).

### Steps

1. **Rubric first.** Before any candidate exists, the lead writes
   `design/rubric.md`: 3–6 criteria derived from `requirement.md` (acceptance criteria,
   non-goals, constraints, and the risks named in the spec). Each criterion says how a
   judge checks it from the documents. Interface depth is one criterion when the design
   has public interfaces: prefer the shape that hides more behind a smaller surface.
2. **Candidates.** Two basic-design workers by default, three if the requirement is
   marked high-risk. Each writes `design/candidates/<n>/design.md` with a `## Shape`
   section first. That section says in one or two sentences how this candidate is
   structured, and what it does differently from every other candidate.
   - Each candidate must be structurally different. If two `## Shape` sections describe
     the same structure, the lead sends one candidate back once. If it is still the same,
     the run continues with the candidates it has, and `selection.md` records the
     collapse.
   - Candidate workers do not write `plan.md`, `manifest.md`, `project.json` or
     `prototype.html`. Those come from the winning candidate after selection.
3. **Blind judge.** One fresh session, not the author of any candidate, gets the rubric
   and the candidate documents with labels `A`, `B`, `C`. Author names, run ids and
   file paths that show the author are removed. The judge scores each criterion per
   candidate, cites the section that supports the score (evidence ladder, section 3),
   and names the pick.
4. **Synthesis.** The pick becomes `design.md`. The lead may graft a named part from
   another candidate. Each graft is listed in `selection.md` with its source candidate.
   Nothing is grafted silently.
5. **Record.** `design/selection.md` holds the rubric, the scores, the pick, every
   graft, and why each dropped candidate lost. The lead writes one decision-trail row
   for the pick, with `evidence` pointing at `selection.md`.
6. Continue the existing step 2 unchanged: manifest, prototype, plan, and the human
   design checkpoint. The human reads `selection.md` in that review.

### Known bias

The judge is the same family as the candidates, so it may favor its own style. The
mitigations are: a rubric written before the candidates, blind labels, and a judge that
must cite sections. These reduce the bias but do not remove it. `selection.md` must say
that the judge is single-family. The human's design review is the only check on the
pick.

### Acceptance

- A fixture requirement with two obvious shapes: two candidates are written, the judge
  picks one, and `selection.md` has scores with cited sections.
- A fixture where both candidates produce the same `## Shape`: one resend, then the run
  continues and `selection.md` records the collapse.
- A requirement marked single-shape: no candidates, and a `skipped` row with the reason.
- The judge's input contains no author or path that identifies a candidate's author.

---

## 3. Evidence ladder and three-state verdict (shared)

Defined here once. `review-pr`, `qa` and the team recap use these terms.

### Evidence ladder

Each claim that a change is safe, or that a finding is real, is placed on the lowest
rung that actually supports it:

| Level | Meaning | Example |
|---|---|---|
| 1 | Asserted | "this is safe" |
| 2 | Pointed at | `file:line` or library source quoted |
| 3 | Argued | the bad case was walked step by step and does not reach |
| 4 | Ran | a command or test that calls the real code showed the behavior |
| 5 | Reproduced in the running app | the surface was driven and observed |

Rules:
- A `review-pr` finding is CONFIRMED only at level 4 or 5 (section 1).
- Level 1 is never reported as evidence. Level 2 and 3 are reported as PLAUSIBLE or
  as a "cleared" claim with its level stated.
- `review-pr` keeps two lists: confirmed findings, and "cleared" checks, each with its
  level. A cleared check at level 1 is dropped, not reported.

### Three-state verdict

`VERIFIED`, `NOT_VERIFIED`, `INCONCLUSIVE`.

- `VERIFIED`: the check ran and the observed result matches the expectation.
- `NOT_VERIFIED`: the check ran and the result is wrong. Evidence is attached.
- `INCONCLUSIVE`: the check could not produce a result: missing credentials, a
  tool that failed, a flaky environment, an unreachable service. **INCONCLUSIVE is not
  a pass.** It never becomes PASS by itself.

A gate passes only when every criterion it covers is `VERIFIED`, or when a
criterion is `INCONCLUSIVE` and the owning gate records it as nonblocking with a
reason. The reason goes into the verdict.

### Where this applies

- `review-pr`: finding verdicts (CONFIRMED / PLAUSIBLE / REFUTED, section 1) are kept
  as they are. Each finding also states its evidence level.
- `qa`: each acceptance criterion and each journey is recorded as one of the three
  states. The current verdict vocabulary in `qa/SKILL.md` steps 7–8 (`PASS`, `FAIL`,
  `BLOCKED`) maps onto them: `PASS` requires every covered criterion to be
  `VERIFIED` or recorded as nonblocking. Before implementation, confirm the exact
  current statuses and the structured verdict file that `bbs ticket readiness` reads.
- Team recap: the Attention section (`team-decision-trail.md`, part 3) shows
  `INCONCLUSIVE` items as "not verified", never as clean.

### Acceptance

- A `qa` run where credentials are missing: the affected criteria are `INCONCLUSIVE`,
  the gate does not pass, and `bbs ticket readiness` is not ok.
- A `review-pr` finding supported only by reading (level 2): not CONFIRMED.
- A cleared check at level 1 does not appear in the report.

---

## 4. Attack the premise (repair loop)

### Problem

The repair rule in `autopilot/SKILL.md` (step 3 of the verification section) says to
retry "with a changed hypothesis" and end in BLOCKED after three failed attempts. It
does not say what the hypothesis is. In practice two fixes can share one wrong
assumption, and each failure looks like a new bug.

### Change

For every repair loop (review fixes, QA fixes, and the team's blocker counter):

1. **Name the premise.** Before attempt 2, write one line in the checkpoint:
   `premise: the fix assumes <X>`. X is a fact the fix depends on, such as "this
   function returns sorted output", "the migration has run", or "the header is set
   by the proxy".
2. **Test it directly.** Before attempt 2 is made, run a check that observes X on the
   real artifact (evidence level 4, section 3). Record the result as `premise_result:
   holds | fails | inconclusive`.
3. **Branch on the result.**
   - `fails`: stop fixing. Hand the blocker to `investigate` with the premise and its
     result. The attempt counter does not reset.
   - `holds`: the premise was not the problem. Continue with the planned attempt and
     record that the premise was checked.
   - `inconclusive`: treat as `holds` for the loop, but the record must say that the
     premise was not confirmed. The recap shows it as not verified (section 3).
4. Persist the premise row in the checkpoint, so a cold resume reads it and does not
   redo the check. Resume never resets the counter, as the current rule already says.

"Same blocker" means the same gate and the same failure fingerprint: the gate name plus
the first failing assertion or error line, with run-specific values such as ids and
timestamps removed.

### Acceptance

- A fixture where two fixes assume a function returns sorted output and it does not:
  the premise row is `fails`, the loop stops, and the blocker goes to `investigate`.
- The same fixture where the premise holds: the planned third attempt runs, and the
  row records `holds`.
- A cold resume after attempt 2 reads the premise row and does not run the check again.
- The counter is still 3 at the end, whatever the premise result.

---

## Files expected to change

- `.claude/skills/review-pr/SKILL.md`: verify phase (repro, PR-target rule), output
  (`repro:` in `failure_scenario`), cleared list with levels.
- `.claude/skills/team/references/kickoff.md`, `project-contract.md`: arena-lite step
  between brief and manifest; `design/rubric.md`, `design/candidates/`,
  `design/selection.md`.
- `.claude/skills/references/evidence.md` (new): section 3, linked by the skills above.
- `.claude/skills/qa/SKILL.md`: three-state criteria, mapping to the current verdict.
- `.claude/skills/autopilot/SKILL.md`: premise step in the repair loop.
- `internal/cmd` (readiness and verdict reading): treat `INCONCLUSIVE` as not passing;
  additive fields only.

## Tests

- Section 1: seeded off-by-one reaches CONFIRMED with output; no-checkout PR target
  stays PLAUSIBLE; cleanup finding has no repro.
- Section 2: distinct-shape fixture produces selection with scores; collapsed-shape
  fixture records one resend; single-shape skip writes a row; judge input has no author
  identity.
- Section 3: missing credentials yield INCONCLUSIVE; readiness rejects it; level-2
  finding is not CONFIRMED.
- Section 4: fails-branch stops and hands to investigate; holds-branch continues; cold
  resume reads the premise row; counter stays at 3.

## Open questions

1. **Section 1 cost.** A repro per correctness candidate adds run time to every
   `review-pr` at medium and above. Measure on two real PRs before making it default at
   medium, or keep it for high and above only.
2. **Section 2 cost.** Two candidates plus a judge roughly double the basic-design
   session. Confirm this against `docs/token-economics.md` before enabling by default.
3. **Section 3 statuses.** Confirm the exact current verdict statuses in `qa` and the
   verdict file schema before writing the mapping.
4. **Section 2 red flags.** A checklist of design red flags is useful for candidates,
   but this repo does not have one yet. Write our own list; do not import one without
   reading its license terms.
