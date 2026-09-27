# Ordered work items

These are execution slices of the L-sized plan, not dispatched tickets. A must land in Orca before BBS makes discovery mandatory. No cross-repository changes are authorized by this planning pass.

## A — Complete the Orca public discovery contract

Expose a versioned, read-only, destination-host-scoped response containing agent IDs, enabled/runnable status, effective default and supported launch overrides. Name commands only after they exist in Orca's live guide/schema; do not implement guessed commands in BBS.

Expose quota snapshots with provider/account-pool identity (opaque, non-secret), observation time, freshness/status, supported windows and reset times, plus explicit mappings to launchable agents where known. Reuse Orca's collectors. Extend public remote access or document a supported host-local read path; never substitute coordinator quotas for remote workers. Account/agent mapping may be unknown. Do not promise quota coverage for every harness.

Acceptance: schema fixtures for enabled versus installed versus running, no default, unavailable default, shared pools, unsupported mapping, stale/error/null quota, multiple windows and local/remote host mismatch. Live read-only smoke on local host and a configured remote if remote dispatch is supported. Freeze the minimal payload consumed by BBS and the capability/freshness rules in its adapter tests. This contract is the implementation prerequisite, not a claim that those fields already exist.

## B — Consume Orca facts in Foreman

Read discovery at startup/resume and before new dispatches, using the existing Orca CLI resolution/preflight in `internal/orca`. Keep this a thin boundary and keep policy in Foreman's existing routing contract. Avoid adding a second scheduler or persistent preference store.

Selection: explicit per-run intent → compatible pinned route/session → Orca effective default for a new route. Validate enabled/runnable status on the selected host. Apply existing complexity/phase model policy and capability checks; validate effective launch receipts. A native/default model of unknown identity cannot prove a required tier.

Quota admission: only a fresh, correctly mapped, authoritative exhausted window defers new work. Persist why, source time and reset/recheck time in the existing run evidence; re-read before retry. For multiple applicable windows respect every exhausted window. An unavailable/stale/error/unsupported/unmapped quota is unknown: record it and continue under existing concurrency/resource limits, treating actual launch rate-limit errors as wait/recheck signals. Never claim unknown is unlimited or available. Do not switch accounts, choose a cheaper tier or mutate live workers to evade a limit. Reuse the existing resume/watch mechanism rather than keeping a new polling daemon.

Persist route intent and observed host/agent/model plus sanitized quota evidence alongside existing Dispatch handoffs. An Orca default change affects new unpinned selections; exact-session recovery continues with recorded identity. Keep native resume compatibility where Orca lacks it; remove only preference resolution. Missing required discovery/default capability blocks new selection with an actionable upgrade/configuration message while read-only BBS and standalone skills remain usable.

Acceptance: adapter fixtures exercise malformed/older responses and bounded failures; Foreman tests cover default changes, explicit overrides, exhausted→reset, unknown usage, remote mismatch, shared pools, unsupported models and exact-session resume. No network/provider calls in unit tests. Read-only live smoke verifies the supported discovery contract; controlled integration verifies receipts before rollout.

## C — Remove duplicate configuration and finish migration

Retire `worker_{agent,provider,model,effort}` and `foreman_{agent,provider,model,effort}` global preferences and their `BABYSIT_WORKER_*`, `BABYSIT_FOREMAN_*`, shared agent/provider/model/effort fallback selectors from launch resolution. Preserve unrelated environment variables. Existing YAML keys remain untouched on disk but are ignored with one actionable diagnostic; new writes to those keys fail clearly. Do not migrate secrets or silently copy preferences into Orca. Preserve historical pinned records and per-phase pointers as execution evidence.

Replace configuration-oriented `bbs agent resolve` behavior/callers with Orca-backed reads or explicit retirement guidance; retain `bbs agent detect` and skill-syntax helpers. Remove `worker-command` if no remaining caller needs it, otherwise keep only a narrow explicit launch/resume adapter without global defaults. Explicit agent/model/effort dispatch requests remain; provider selection is native configuration unless Orca exposes a supported explicit contract. Legacy provider launch evidence stays readable for exact recovery.

Remove dashboard settings API/types/form and default-config examples. Keep `#/settings` as the static ownership notice in `design.md`, so old links remain useful. Old mutation calls must fail with an explicit retirement response and write nothing. Update onboarding, operations docs, help, setup-project ownership guidance and skill routing references. Preserve the user's current model-routing edits and its phase table.

Acceptance: legacy config is not mutated, retired keys cannot change new launches, native pinned recovery still works, a stale dashboard cannot save preferences, setup emits no agent/provider settings, and standalone skill telemetry detects its current harness without requiring Orca.

## Verification commands and browser checks

```sh
go test ./internal/agent ./internal/config ./internal/orca ./internal/foreman ./internal/cmd
bash tests/test_foreman_skill.sh
npm --prefix web run build
```

Browser: Settings and first-run flow at desktop/mobile widths in both themes; confirm no agent/provider/model/effort inputs or Save action; ownership notice works with Orca offline and in a read-only snapshot; no legacy settings fetch. Existing project creation/ticket navigation still works. The old settings API returns retirement guidance without writing config.
