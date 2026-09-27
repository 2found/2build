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
  "product_review": true
}
```

Use relevant check names, including accessibility, persistence, packaging or
operational checks where required. Set `product_review: true` for user-facing
products. Optional `deadline` is an RFC3339 work budget: expiry blocks new
dispatch; it cannot convert an incomplete project to success. A changed budget
or scope revisits the project approval. Usage stays unknown without provider
observations; do not invent cost or token totals.

Allocate children with the existing topology protocol, then bind each accepted
seed with `bbs foreman bind "$PARENT" --seed <key> --child <ticket>`. One code
repository per child keeps each evidence subject unambiguous; use dependent
children across repos. Every required leaf must have one accepted seed. A
cancelled child leaves unmet scope until an explicit contract revision removes
or replaces it.

After parent approval, admit ready tickets according to the DAG and resource
bounds. `first_journey` identifies acceptance coverage, not a separate admission
gate before other independent tickets. Shared interfaces and edit ownership are
fixed in the accepted plan; dependency edges enforce necessary ordering.

At each full reconciliation use `bbs foreman snapshot "$PARENT" --json`. It returns
scope coverage, child seals, current evidence, blockers, delivery and observed
time. Before new production dispatch use
`bbs foreman readiness "$PARENT" --action dispatch --json`; inspect each child's
dependencies through the existing DAG as well. Before closing use `--action
finish`. Both return `ready` and specific reasons; a zero exit code only means
the read succeeded. Reads never reconcile or mutate ticket status.

## Worker packet and verification

Use the worker packet in SKILL.md and the Orca envelope in topology.md.
Autopilot honors its **Foreman phase assignments** contract on the assigned checkout.

New managed code children use `bbs autopilot checkpoint --ticket "$TICKET"
--workflow builder --step run --status in_progress --contract-version 2` (substitute
their actual workflow). Existing v1 tickets require that explicit migration and
new gate runs; never wrap legacy PASS prose as typed evidence. Autopilot's
[verification producer](../autopilot/references/verification.md) captures
before/after subjects and archives logs. Seal each passing child **before** its
finish handler or worktree removal: `bbs foreman seal "$TICKET"`.

A seal preserves verified source revisions, policies, gate attempts and log
digests. Readiness still detects changed branches, gate inputs, dependencies and
policy after cleanup. Keep source branches as the existing finish policy requires.
Repairing a sealed child needs fresh gates and a new seal, then fresh integration.

Evidence-only projects explicitly approve `artifacts_only: true`. Their children
keep the workflow verdict and durable outputs under the ticket directory. Seal
with `bbs foreman seal "$TICKET" --file <artifacts.json>`, containing
`{"paths":["<absolute ticket output>"]}`. Production commits cannot be delivered
through this path. Final acceptance checks still cover each criterion; no Git
surface is required, and delivery reads `ARTIFACTS_READY`.

## Integrated checks and independent product review

Prepare the real final surface using project-contract.md, after finish handlers.
Before testing, create a verification attempt:

```sh
bbs foreman evidence "$PARENT" --begin --file /absolute/check-spec.json
```

```json
{
  "kind":"integration",
  "producer":"actual-qa-dispatch-id",
  "surfaces":[{
    "dir":"/absolute/primary/repo",
    "ref":"qa/parent-id",
    "base_ref":"main",
    "restore_ref":"main",
    "restore_head":"<recorded original full SHA>"
  }]
}
```

The CLI captures current heads and project subject. Under `land`, ref, base_ref
and restoration are the delivered base; under `pr`, base_ref is `origin/<base>`.
Under `review`/`pr`, preserve the `qa/<parent>` ref and the original checkout
recorded before preparation. Store the returned attempt ID. Tests must run while
the exact clean surface is checked out. After tests, **before restoration**, run:

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

When the contract requires product review, dispatch a **fresh read-only product
evaluator** on the normal phase route using [product-review.md](product-review.md).
Its identity must differ from child gate producers. Final evidence and every
criterion check use `kind: product`; `kind: journey` is only for an explicitly
required preliminary check. Material findings require repair and fresh evidence;
minor findings stay in the handoff.

After final QA/restoration, release settled workers and leases, persist the
report, then run `bbs foreman complete "$PARENT" --foreman "$FOREMAN_ID"`.
It writes the completion receipt only when all assigned parents are verified;
retained review/open PR parents remain `in_review`. Status edits or a raw done
heartbeat cannot replace this receipt.

## Progress, waits and repair

Write `bbs foreman progress <ticket> --file <report.json>` at a meaningful
milestone or wait transition. Use the actual run/Dispatch/attempt IDs:

```json
{"producer":"worker-id","phase":"qa","summary":"Task survives reload",
 "run_id":"run-id","dispatch_id":"dispatch-id","attempt_id":"attempt-id",
 "evidence":"/absolute/durable/check.log"}
```

Only changed evidence advances `progress_at`; summary rewrites and extra commits
do not. Waits add `wait_kind` (`test`, `dependency`, `approval`, `resource`),
`wait_reason` and `wait_until` at most 15 minutes ahead. Refresh from actual
ongoing waits, never a timer. Expiry is not failure or a reason to inspect a
worker; Orca owns runtime state and runtime.md defines reconciliation triggers.

On confirmed repeated semantic failure, dispatch planning/review diagnosis on
the critical phase route with failure evidence, then repairs on the normal route.
Preserve failure counts and work; never restart a live writer, expire its lease,
blindly resend assignments or silently upgrade its model. Missing intent/authority
escalates one User Challenge; unaffected work continues. Record telemetry and
recovery evidence before retrying.
