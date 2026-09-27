# Plan
**Goal (L):** Make Orca the authority for configured agents, defaults and usage; remove BBS's persistent worker/foreman agent/provider/model/effort settings.
**Out of scope:** Provider authentication, new quota collectors, billing, changing a live worker, deleting standalone harness detection, or changing the existing phase/tier policy.
**Approach:** Foreman reads destination-host Orca discovery and quota snapshots at startup/resume and before new dispatches; BBS retains phase selection, resource leases, durable route evidence and recovery.
Keep explicit per-run agent/model/effort requests as dispatch intent; use Orca's default for new unpinned selections, verify effective launch receipts, and preserve pinned sessions on resume.
Known exhaustion defers a new dispatch to reset/recheck; stale, unmapped or unavailable quotas remain unknown. Never downgrade a required tier or switch accounts automatically.
Remove the eight persistent settings and their environment fallback from CLI/default config, dashboard form/API and setup guidance; preserve unrelated config and read legacy pinned records without reactivating global preferences.
**Sequence:** [A: Orca contract → B: Foreman consumption → C: BBS removal](work-items.md); A is an external prerequisite, all three are in scope for completion.
**Unknowns:**
- *Derived — discovery:* Orca 1.4.215 has live terminal identity and account rate limits, but no advertised configured/enabled/default-agent read; A must supply and verify a public host-scoped contract ([evidence](evidence.md)).
- *Derived — quota identity:* `account list` is local-only and keyed by provider, not Dispatch; shared pools, remote hosts, OMP bindings and freshness require explicit provenance before quotas can gate work ([evidence](evidence.md)).
- *Derived — recovery:* `internal/cmd/foreman.go` still constructs native resume commands; retain the minimal compatibility adapter until Orca can resume the exact pinned session, never substitute today's default (`internal/foreman/foreman.go`).
- *Derived — reach:* removal includes onboarding, generic `config set`, env resolution and phase pointers; preserve standalone skill detection and the pre-existing edits in `.claude/skills/references/model-routing.md` ([work items](work-items.md)).
**Verify:** `go test ./internal/agent ./internal/config ./internal/orca ./internal/foreman ./internal/cmd`; `bash tests/test_foreman_skill.sh`; `npm --prefix web run build`; fixture/live checks and browser states in [work items](work-items.md).
**Design:** [design.md](design.md), isolated [prototype](../../web/prototype/orca-agent-settings/index.html); no production changes in this planning pass.
