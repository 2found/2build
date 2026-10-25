# Project execution contract

Read at entry/cold resume with project-contract.md. The CLI owns mechanical
validation; Foreman owns product judgment and the Orca lifecycle. These commands
read existing ticket/approval/DAG state and never create branches or dispatch.

## Accepted scope and DAG admission

Before publishing the parent project-plan approval, write `project.json` through
`bbs foreman contract "$PARENT" --file <draft.json>`. The file is part of that
approval's fingerprint. Stable seed keys exist before runtime ticket IDs:

```json
{
  "version": 1,
  "audience": "Small teams managing shared work",
  "outcome": "Create a task, assign it, and recover it after reload",
  "non_goals": ["Billing"],
  "seeds": ["tasks", "assignments"],
  "criteria": [
    {"id":"AC1","text":"Tasks survive reload","owner":"tasks","checks":["behavior","failure"]},
    {"id":"AC2","text":"Assign from a narrow screen","owner":"assignments","checks":["behavior","responsive"]}
  ],
  "first_journey": ["AC1"],
  "product_review": true,
  "phases": [
    {"id":"p1","title":"Walking skeleton","seeds":["tasks"],"release":"ask"},
    {"id":"p2","title":"Assignment","seeds":["assignments"],"release":"auto"}
  ]
}
```

`phases` is the release plan ([phases](phases.md#the-phase-plan)); the CLI
rejects an uncovered or doubly-assigned seed and a `first_journey` criterion
outside phase 1.

Use relevant check names, including accessibility, persistence, packaging or
operational checks where required. Set `product_review: true` for user-facing
products. Optional `deadline` is an RFC3339 work budget: expiry blocks new
dispatch; it cannot convert an incomplete project to success. A changed budget
or scope revisits the project approval. Usage stays unknown without provider
observations; do not invent cost or token totals.

Allocate each phase's ticket and children when the phase opens (phases.md,
topology.md), then bind each accepted seed with `bbs foreman bind "$PARENT" --seed <key> --child <ticket>`. One code
repository per child keeps each evidence subject unambiguous; use dependent
children across repos. Every required leaf must have one accepted seed. A
cancelled child leaves unmet scope until an explicit contract revision removes
or replaces it.

After parent approval, open phases as readiness allows and admit each open
phase's tickets as SKILL.md's batch rule orders them — ready tickets first,
then dependents that wait in session with `bbs foreman await` — within
resource bounds. Shared interfaces and edit ownership are
fixed in the accepted plan; dependency edges enforce necessary ordering.

At each full reconciliation use `bbs foreman snapshot "$PARENT" --json`. It returns
scope coverage, child seals, current evidence, blockers, delivery, each
phase's state, releases and problems, and observed time. Gate each phase step
with `bbs foreman readiness "$PARENT" --action dispatch|integrate|land|release
--phase <id> --json`; inspect each child's dependencies through the existing
DAG as well. Before closing use `--action finish`. Each returns `ready`,
specific reasons and, with `--phase`, the phase object; a zero exit code only
means the read succeeded. Reads never reconcile or mutate ticket status.

## Worker packet and verification

Use the worker packet in SKILL.md and the Orca envelope in topology.md.
Autopilot honors its **Foreman ticket assignment** contract on the assigned checkout.

Every ticket is created with a `schema_version: 2` checkpoint; an older one is
refused by readiness, gates and seal until migrated as the producer reference
says, and its gates then run again. Autopilot's
[verification producer](../autopilot/references/verification.md) captures
before/after subjects and archives logs. Seal each passing child as soon as it
passes: `bbs foreman seal "$TICKET"`.

A seal snapshots the worker's uncommitted tree on `refs/bbs/tickets/<id>` and
preserves gate attempts and log digests. An edit in the worktree after the
seal invalidates it until the ticket is re-verified and resealed; once its
phase landed and the worktree is gone, the snapshot is the record.
Repairing a sealed child needs fresh gates and a new seal, then a fresh phase
commit.

Evidence-only projects explicitly approve `artifacts_only: true`. Their children
keep the workflow verdict and durable outputs under the ticket directory. Seal
with `bbs foreman seal "$TICKET" --file <artifacts.json>`, containing
`{"paths":["<absolute ticket output>"]}`. Code changes cannot be delivered
through this path. Final acceptance checks still cover each criterion; there is
no phase commit to integrate or land, and delivery reads `ARTIFACTS_READY`.

## Integrated checks and independent product review

Build the phase commit first (`bbs foreman integrate`, [phases](phases.md#2-integrate-the-phase)).
In its integration worktree, before testing, create a verification attempt:

```sh
bbs foreman evidence "$PARENT" --begin --file /absolute/check-spec.json
```

```json
{"kind":"integration","producer":"actual-qa-dispatch-id","phase":"p2"}
```

The CLI captures the phase subject (the contract plus every ticket of phases
1..k) and derives the surfaces from the phase commit; it refuses a phase that
is not ready, or an integration worktree that is not clean at that commit.
Store the returned attempt ID. Tests run in that worktree, against that exact
commit; earlier phases' commits must already be on base. After tests, run:

```sh
bbs foreman evidence "$PARENT" --attempt <id> --file /absolute/results.json
```

```json
{
  "checks":[{"criterion":"AC1","kind":"behavior","command":["actual","command"],"exit_code":0,"log":"/absolute/real-check.log"}],
  "findings":[]
}
```

Record every required criterion/check pair and real exit code. The producer
archives logs and hashes them; changed inputs reject results. Failures remain
durable; repairs get a fresh attempt. A preview requires `preview` HTTP(S) URL,
`runtime` full tested SHA, and a passing `runtime` check whose probe log contains
that SHA exactly. The probe must query the running build; echoing an expected
SHA proves nothing. The dashboard labels the observation time, never current
server availability.

Release receipts and requests are phases.md's: `bbs foreman release --request`
and `--file` write `release-request.json` on the phase ticket and
`project-releases/<id>.json` on the parent, and the snapshot reads them back.

When the contract requires product review, dispatch a **fresh read-only product
evaluator** on the normal route using [product-review.md](product-review.md).
Its identity must differ from child gate producers. Final evidence and every
criterion check use `kind: product`; `kind: journey` is only for an explicitly
required preliminary check. Material findings require repair and fresh evidence;
minor findings stay in the handoff.

After the last phase is released (or landed, for `none`), release settled
workers and leases, persist the report, then run `bbs foreman complete "$PARENT" --foreman "$FOREMAN_ID"`.
It writes the completion receipt only when all assigned parents are verified. Status edits or a raw done
heartbeat cannot replace this receipt.

## Progress, waits and repair

Write `bbs foreman progress <ticket> --file <report.json>` at a meaningful
milestone or wait transition. Use the actual Task/Dispatch/attempt IDs
(`run_id` carries the bbs Task id):

```json
{"producer":"worker-id","phase":"qa","summary":"Task survives reload",
 "run_id":"run-id","dispatch_id":"dispatch-id","attempt_id":"attempt-id",
 "evidence":"/absolute/durable/check.log"}
```

`phase` names the ticket's loop step (`plan`, `approve`, `await`, `adopt`,
`build`, `review`, `qa`, `repair`), not a release phase. An `approve` wait on Foreman is `wait_kind: approval`; an
`await` on prerequisites is `wait_kind: dependency`.
Only changed evidence advances `progress_at`; summary rewrites and extra edits
do not. Waits add `wait_kind` (`test`, `dependency`, `approval`, `resource`),
`wait_reason` and `wait_until` at most 15 minutes ahead. Refresh from actual
ongoing waits, never a timer. Expiry is not failure or a reason to inspect a
worker; Orca owns runtime state and runtime.md defines reconciliation triggers.

A ticket session repairs its own failures first: autopilot's repair loop, then
Foreman's `repair` follow-ups into a fresh session in the same worktree. When the ticket reports the
same blocker `BLOCKED` after its retry budget, dispatch a fresh read-only
diagnosis worker on the critical route with the failure evidence — a context
that has not already failed three times is the point — then send its findings
to the ticket session as a `repair` (a fresh worker on the ticket route if the
session is gone). Preserve failure counts and work; never restart a live
writer, expire its lease, blindly resend assignments or silently upgrade its
model. Missing intent/authority
escalates one User Challenge; unaffected work continues. Record telemetry and
recovery evidence before retrying.
