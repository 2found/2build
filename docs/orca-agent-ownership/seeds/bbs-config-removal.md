# Seed: bbs-config-removal (work item C)

Remove duplicate agent configuration and finish migration. Source spec:
docs/orca-agent-ownership/work-items.md §C and plan.md. Parent: bs-mn46ozj8.
Blocked by: foreman-orca-consumption (B).

- Retire `worker_{agent,provider,model,effort}` and
  `foreman_{agent,provider,model,effort}` global preferences plus
  `BABYSIT_WORKER_*`, `BABYSIT_FOREMAN_*` and shared agent/provider/model/effort
  env fallbacks from launch resolution. Preserve unrelated env vars. Existing
  YAML keys stay on disk but are ignored with one actionable diagnostic; new
  writes to those keys fail clearly. No secret migration, no silent copy into
  Orca. Keep historical pinned records and per-phase pointers as evidence.
- Replace configuration-oriented `bbs agent resolve` behavior/callers with
  Orca-backed reads or explicit retirement guidance; keep `bbs agent detect`
  and skill-syntax helpers. Remove `worker-command` if no caller needs it, else
  keep a narrow explicit launch/resume adapter without global defaults.
  Explicit per-dispatch agent/model/effort requests remain; provider selection
  is native config unless Orca exposes a supported explicit contract. Legacy
  provider launch evidence stays readable for exact recovery.
- Remove dashboard settings API/types/form and default-config examples. Keep
  `#/settings` as the static ownership notice per design.md so old links work.
  Old mutation calls fail with an explicit retirement response, writing
  nothing. Update onboarding, operations docs, help, setup-project guidance and
  skill routing references. Preserve the user's model-routing edits and phase
  table.
- Acceptance: legacy config unmutated; retired keys cannot change new launches;
  native pinned recovery works; stale dashboard cannot save preferences; setup
  emits no agent/provider settings; standalone skill telemetry detects its
  harness without Orca. Browser: Settings + first-run at desktop/mobile both
  themes — no agent inputs or Save, notice works with Orca offline, no legacy
  fetch; old settings API returns retirement guidance without writing.
