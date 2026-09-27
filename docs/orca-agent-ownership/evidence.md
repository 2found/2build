# Orca agent ownership — survey

Planning baseline: repository HEAD `7155004`, Orca runtime `1.4.215`, inspected 2026-09-27. `.claude/skills/references/model-routing.md` already has user changes; this plan does not edit it. No ticket resolves on this checkout, so no ticket pointers were written. Scope L: cross-cutting CLI/API removal and more than ten affected files.

## Verified public surfaces

| Read | Observed contract | Boundary |
| --- | --- | --- |
| `orca status --json` | Runtime readiness, version and capabilities | Runtime available locally during survey |
| `orca agent-context --json` | Machine-readable public command registry | No advertised configured/enabled/default-agent discovery command; `agent-context` describes commands, not available harnesses |
| `orca terminal list --json` | `agentIdentity`, execution host, live handle and worktree | Running instances, not a configured-agent inventory; structured workers may not have terminals |
| `orca orchestration worker-list --json` | Registry documents supervised worker resource accounting and remote inclusion | Process/resource state is distinct from task status and quota |
| `orca account list --json` | `result.rateLimits`, provider entries with status, timestamp, error and nullable windows; populated weekly windows include `usedPercent`, `windowMinutes`, `resetsAt` | Actual response includes quota information absent from command help; only managed Claude/Codex accounts are listed, but quota entries cover more providers |
| `orca account list` command schema | Rejects `--environment` and `--pairing-code` | Local account quota cannot establish a remote worker's budget |
| `orca repo show --repo <selector> --json` | Repo identity and hook settings | Surveyed response does not expose default agent |
| `orca orchestration worker-start` command schema | Explicit `--agent`; selected agents support model/effort, with effort requiring model | No provider flag; model/effort forwarding is not universal |

The account response includes non-provider metadata inside `rateLimits`; consume documented provider records, not every map value as a quota object. Credentials, account identifiers and raw terminal output are deliberately excluded from these artifacts.

## Ownership recommendation

Orca owns enabled/available harness discovery, host/default selection, launcher capabilities and native account/quota collection. Native agents still own authentication and provider configuration. BBS owns task/phase policy, admission decisions, QA leases and durable evidence. Keep a thin read/launch boundary, not a duplicate mutable registry or credential scraper.

“Active agent” needs three separate facts: enabled in Orca, runnable on the destination host, and currently running. None proves the other two. Quotas belong to provider/account pools, potentially shared by multiple harnesses and workers; an agent label is not a quota key. The observed account payload does not prove complete mapping for OMP or custom providers.

## Existing code and archaeology

- `internal/agent/settings.go`: independent field precedence flags → role env → shared env → machine config; automatic fallback selects detected/installed CLI. Removing the form alone leaves this authority intact.
- `internal/agent/agent.go`, `detect.go`: launch/resume adapters, skill invocation syntax and session detection coexist. Keep detection and skill syntax for skills running outside Orca.
- `internal/cmd/foreman.go`, `internal/foreman/foreman.go`: records pin agent/provider/model/effort and conversation identity; watchdog recovery must honor that record.
- `internal/cmd/config.go`: generic key setter accepts any valid key. Explicitly reject retired setting keys with migration guidance rather than accepting ineffective settings silently.
- `internal/cmd/dashboard_agent_settings.go`, `web/src/views/Settings.tsx`, `web/src/lib/api.ts`: eight-field mutation API and form. Retire the mutation endpoint with a clear response for old browser bundles; the replacement notice has no Orca dependency.
- `web/src/components/FirstRunChecklist.tsx`: currently directs users to choose an agent/provider in BBS Settings.
- `.claude/skills/setup-project/SKILL.md`: currently does not ask for provider selection. Keep that behavior, add the ownership rule, and preserve workspace registration, git policy and QA setup.
- `.claude/skills/references/model-routing.md`, `.claude/skills/foreman/references/worker-routing.md`: keep phase/tier policy, OMP role semantics, effective-setting checks and per-phase evidence while replacing BBS config resolution.
- Commit `2f96450` introduced settings across agent resolution, dashboard/API, config, docs and Foreman recovery. Tests `TestForemanRecoveryPinsLaunchPreferences`, `TestResumeUsesThePinnedAgentNotCurrentConfig`, `TestAdoptedForemanRecoveryPinsEachAgent` and `TestSkillRuntimeUsesCurrentAgentNotWorkerSelection` expose the main regression boundaries.

## Planning verification

- Completed: inspected local public CLI guide/schema and live status, accounts, terminals and repo responses; surveyed affected code and introducing commit; inventoried dashboard components/tokens and accessibility rules.
- Completed: prototype rendered using real dashboard components at desktop/light and 390px/mobile/dark; no horizontal overflow or browser errors. Browser and Vite dev server were closed after checking. See `design.md`.
- Implementation verification is specified in `work-items.md`; production changes and their tests have not been performed.
