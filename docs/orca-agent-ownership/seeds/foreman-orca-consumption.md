# Seed: foreman-orca-consumption (work item B)

Consume Orca facts in Foreman. Source spec: docs/orca-agent-ownership/work-items.md §B
and docs/orca-agent-ownership/plan.md. Parent: bs-mn46ozj8.

- Read agent discovery + quota snapshots at startup/resume and before new
  dispatches via the existing Orca CLI resolution/preflight in `internal/orca`.
  Thin read boundary; policy stays in Foreman's routing contract. No second
  scheduler, no persistent preference store.
- Selection precedence: explicit per-run intent → compatible pinned
  route/session → Orca effective default for a new route. Validate
  enabled/runnable on the destination host. Keep complexity/phase model policy
  and capability checks; verify effective launch receipts. A native default of
  unknown identity cannot prove a required tier.
- Quota admission: only a fresh, correctly mapped, authoritative exhausted
  window defers new work — persist why, source time, reset/recheck time in run
  evidence; re-read before retry; respect every exhausted window.
  Unavailable/stale/error/unsupported/unmapped quota = unknown: record and
  continue under existing limits; treat launch rate-limit errors as
  wait/recheck signals. Never claim unknown is unlimited. Never switch accounts,
  downgrade tiers, or mutate live workers. Reuse resume/watch, no new poller.
- Persist route intent + observed host/agent/model + sanitized quota evidence in
  Dispatch handoffs. Orca default changes affect only new unpinned selections;
  exact-session recovery keeps recorded identity. Keep native resume
  compatibility where Orca lacks it. Missing required discovery/default
  capability blocks new unpinned selection with an actionable upgrade/config
  message while read-only BBS and standalone skills stay usable.
- Acceptance: adapter fixtures for malformed/older responses and bounded
  failures; Foreman tests for default changes, explicit overrides,
  exhausted→reset, unknown usage, remote mismatch, shared pools, unsupported
  models, exact-session resume. No network/provider calls in unit tests.
- Do not implement guessed Orca commands; consume only what the installed Orca
  exposes and mark everything else unavailable. Do not remove the persistent
  config settings in this ticket — that is bbs-config-removal (work item C).
