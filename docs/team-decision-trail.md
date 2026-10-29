# Team decision trail and handover audit

Status: partly implemented in 1.115.0. Owner: `team` (lead + `bbs team`).

**What shipped, and what changed from this proposal:**

- Part 1 ships as `bbs team log` → `<parent>/decisions.jsonl`, with the row
  schema below. Rows hold judgments only: seals, waivers, integrations,
  landings and releases are already in project state, so integrate/land rows
  are not logged and the "landed phase has an integrate row" check is
  dropped. `--supersedes` must name an existing row at write time. The lead
  is the only writer (workers report through their handoff), so no
  concurrency test is needed. The skill contract is
  `team/references/delivery.md` § Decision trail.
- Part 2 layer 1 ships inside `bbs team recap`, which is what the release
  request points at (open question 1): evidence that does not resolve, and
  any result other than `verified`, is listed under **Check first**. No
  `audit.json`; the recap re-checks on every render.
- Part 2 layer 2 (a reviewer from another model family) is **deferred**:
  `bbs agent list` has no family concept, and
  [quality-and-design-controls.md](quality-and-design-controls.md) defers
  cross-family judging too.

The rest of this document is the original proposal.

Two additions to the unattended team run. Both serve one reader: the human
who opens `recap.html` after the run, with no watcher and no way to ask
mid-run.

1. **Decision trail** — a per-project, append-only log of the decisions the
   team took for the human, each with evidence and a result.
2. **Handover audit** — before the handover is rendered, a check of that trail:
   deterministic evidence checks, then a judgment pass by a reviewer from a
   different model family. Its findings appear in an **Attention** section of
   `recap.html`.

Neither gates completion or release. The recap is "for the human, not a gate"
(`projectComplete`, `internal/cmd/foreman_project_evidence.go`), and the
audit follows the same rule.

## Problem

- The only decision log is `~/.babysit/analytics/decisions.jsonl`, written by
  `learnings.Append` (`internal/learnings/learnings.go`). It is machine-wide,
  not per project, and it records ticket-level events only: design-checkpoint
  self-resolves (`logSelfResolvedApproval`), size resizes, semantic-decision
  rows. It has no evidence pointer and no result, and it is gated by
  `telemetry: off`. It is telemetry, not an audit trail.
- A lead that runs overnight leaves no per-project record of "I chose X over Y
  because Z, and here is how it was checked". The human reconstructs it from
  git log and transcripts.
- Nothing checks the lead's own claims before the human reads them. A row
  citing a commit that does not exist, or a verification that was never run,
  reaches the human unflagged.

Concept source: the **show-me-your-work** and overnight-audit ideas in
pstack (MIT, Copyright (c) 2026 Lauren Tan). This spec re-expresses the ideas in
bbs terms; no pstack text is copied, so no notice travels with the code.

## Non-goals

- Replacing `decisions.jsonl`. The telemetry row stays as it is, and the new
  trail is written in addition.
- Logging every action. Only decision points, checkpoints and reversals are
  logged.
- A reviewer that re-does the work or re-runs QA. The reviewer reads the trail
  and the evidence it points to.
- Blocking merge, release or completion on a flag.

## Part 1 — Decision trail

### Location

`tickets/<parent>/decisions.jsonl` (the team's `st.Home()`). One file per
project, under the project's own directory, so it is committed or archived
with the project if the human chooses.

### Row schema

One JSON object per line, under 4 KiB.

| field | required | meaning |
|---|---|---|
| `id` | yes | first 8 hex of sha256(`ts` + `ticket` + `decision`). Stable reference for `supersedes` |
| `ts` | yes | UTC ISO-8601, `learnings.Timestamp()` |
| `phase` | yes | project phase id, or `design` / `handover` / `start` |
| `ticket` | no | ticket id, empty for project-level decisions |
| `actor` | yes | `lead` or the ticket's worker session id |
| `decision` | yes | what was chosen or done, one line, plain words |
| `why` | yes | the reason, one line. Names the rule or evidence it rested on |
| `evidence` | yes | a resolvable pointer: commit SHA, `path:line`, PR URL, artifact path, or a rubric row. Never prose |
| `result` | yes | the state the decision reached: `verified`, `failed`, `open`, `inconclusive`, `reverted`, or a short predicate note |
| `supersedes` | no | `id` of the row this row replaces |

Append-only. A wrong row is corrected by a new row with `supersedes`, never
edited or deleted.

### Writer

- `bbs team log --parent ID --phase P --decision D --why W --evidence E --result R`
  appends one row. `actor` and `ticket` come from `BABYSIT_TICKET` and the
  session, the same identity ladder as `bbs ticket resolve`.
- The command rejects an empty `evidence` or `result`. It does not check that
  the evidence resolves; the audit does that.
- Concurrency: a file opened with `O_APPEND` and a single write under 4 KiB
  is not interleaved on POSIX. Parallel tickets therefore append without a
  lock. Rows over 4 KiB are rejected, not truncated.
- A resumed lead (new session on the same project) first appends a `start`
  row. Its `evidence` names the new session, and its `decision` says which
  rows it did not write. This is the one place a cold start is recorded.

### What gets a row

Write a row where the team takes a decision for the human or reverses one:

- Design-checkpoint self-resolve (replaces the telemetry-only write; the
  telemetry line is still emitted).
- Plan size and approval for a ticket, and any resize.
- Phase integrate and land, with the commit SHA as evidence.
- A seal waiver, a fixup, or a gate `--waive`, with the reason.
- A hold, grant or revoke seen by the lead.
- A worker replaced, or a pivot or revert, with its trigger.
- A blocker that survived its attempts, and what was done instead.
- The release verdict the human gives, copied from the release request.

Do not log: routine commands, passing checks that are part of a sealed
ticket, or the reasoning inside a worker that never changed the plan.

### Where it is shown

The recap gets a **Decisions** section, rendered from this file, placed after
the handover report and before phases. Each row shows decision, why, evidence
(linked where it is a URL or a commit), and result. Superseded rows are
hidden and counted in a footnote. The section is omitted when the file is
absent.

## Part 2 — Handover audit

### When it runs

After the last phase is landed and before the handover recap is rendered for
the release request. Concretely: inside the same path that
`bbs team complete` and `bbs team recap` use, before `writeProjectRecap` is
called. The exact hook point is to be confirmed in `foreman_project.go` and
`foreman_project_evidence.go` during implementation. The audit runs once per
handover; a later `bbs team recap` re-renders the page from the stored
`audit.json` and does not re-run the reviewer.

### Layer 1 — deterministic checks (Go, no model)

For each row in `decisions.jsonl`:

- `evidence` looks like a commit SHA: `git cat-file -e <sha>^{commit}` in the
  canonical repo succeeds.
- `evidence` looks like `path` or `path:line`: the path exists at the landed
  head, and the line is within the file.
- `evidence` is a PR URL: recorded, not fetched.
- `supersedes` names an `id` that exists and is earlier.
- Each phase that is `landed` has a `decision` row with `result: verified`
  from integrate.

Each failure becomes a flag with `layer: "evidence"`. No model is needed, and
the check runs even when no other family is installed.

### Layer 2 — judgment (a different model family)

The lead cannot review its own trail. The reviewer must come from a different
model family from the lead, as declared in `bbs` agent config. Selection:

1. Take the installed agents from `bbs agent list --json`.
2. Keep those whose family differs from the lead's.
3. Use the first one in config order. If none, skip this layer.

Input (read-only): `decisions.jsonl`, the handover report, the phase
commit list and each phase's `completion.json`. Transcripts are not read in
v1.

Output: the reviewer returns JSON validated against this shape. Anything else
is treated as a failed run.

```json
{
  "reviewer": "claude-x / codex-y",
  "flags": [
    {
      "severity": "high | medium | low",
      "rows": ["3f2a9c10"],
      "kind": "unsupported-claim | unverified-step | risky-choice | gap",
      "summary": "one line, plain words",
      "evidence": "path, SHA or row id the flag rests on"
    }
  ]
}
```

Prompt scope: find what the human should look at first. It does not re-review
code and it does not propose fixes.

### Result file

`tickets/<parent>/audit.json`:

```json
{
  "version": 1,
  "at": "<iso>",
  "status": "ran | not-run | failed",
  "reason": "no other family installed | reviewer error | …",
  "reviewer": "<name or null>",
  "flags": []
}
```

- `ran` with an empty `flags` array means the reviewer raised nothing. The
  recap says so and names the reviewer. An empty list without a reviewer is
  not allowed.
- `not-run` and `failed` are shown in the recap as "not reviewed", with the
  reason. They are never rendered as clean.

### Non-blocking rule

The audit never changes the outcome of `complete`, `release --request` or a
phase's readiness. A crash, timeout (default 10 min) or invalid output writes
`status: failed` and the run continues. The lead does not fix flags on its
own; fixing is the human's choice, or a feedback loop the human starts.

## Part 3 — Attention section in the recap

Placed after **Open blockers** and before **Handover**.

- Flags sorted by severity, then by first referenced row.
- Each flag: severity badge, kind, summary, and links to its rows and evidence.
- With `status: ran` and no flags: "No flags from <reviewer>".
- With `status: not-run` or `failed`: "Not reviewed: <reason>".
- The section is omitted only when `audit.json` is absent, which means the
  audit was never invoked. The recap then says so, rather than staying silent.

## Files expected to change

- `internal/cmd/team_log.go` (new): `bbs team log` writer and validation.
- `internal/cmd/team_audit.go` (new): layer 1 and layer 2, `audit.json`.
- `internal/cmd/foreman_recap.go`: Decisions and Attention sections.
- `internal/cmd/foreman_project.go`, `foreman_project_evidence.go`: hook the
  audit before the recap render; add the `log` and `audit` subcommands to the
  `team` dispatch.
- `internal/cmd/approval_selfresolve.go`: write a trail row next to the
  telemetry row in `logSelfResolvedApproval`.
- `.claude/skills/team/SKILL.md` and `references/delivery.md`: the lead writes
  rows at the listed points and runs the audit before the handover.
- `docs/team.md`: one sentence in the handover description.

No change to the telemetry schema or to `skill-usage.jsonl`.

## Tests

- Writer rejects empty `evidence` or `result`, and rows of 4 KiB or more.
- Concurrency: 20 goroutines × 50 appends to one file. Every line parses and
  none is lost.
- `supersedes` hides the superseded row in the recap and keeps it in the file.
- Layer 1: a real commit SHA resolves, a fake SHA does not, a missing path
  does not, and a line beyond the file's length does not.
- Reviewer selection: a same-family agent is never chosen; with none
  available, status is `not-run`, not `ran`.
- Reviewer returns invalid JSON: status `failed`, and `complete` still succeeds.
- Recap: renders `ran` with flags, `ran` with none, `not-run`, and absent
  `audit.json`, each distinctly.
- Non-blocking: `complete` and `release --request` succeed with every
  audit status.

## Acceptance

- A project that completed without any approval self-resolve still has a
  `decisions.jsonl` containing at least its `start` row and its phase
  integrate rows.
- An injected row with a fake commit SHA appears in the Attention section as
  an `evidence` flag, and the project still completes.
- With only the lead's family installed, the recap says "Not reviewed: no other
  family installed" rather than showing no flags.

## Open questions

1. **Hook point.** Confirm that the release request reads the recap rendered
   by `complete`, or that it renders its own. The audit must run before either.
2. **Reviewer cost.** One reviewer call per handover, with a prompt size
   capped at the trail plus the handover report. Confirm the budget is
   acceptable against the team's token economics note
   (`docs/token-economics.md`).
3. **Transcripts.** v1 does not read them. pstack's audit also checks the log
   against the session transcript. Adding that means reading a path the
   runtime owns, so it waits for a separate decision.
4. **Worker rows.** Workers do not write to the trail in v1; the lead
   records what the worker's handoff reports. Confirm this is enough.
