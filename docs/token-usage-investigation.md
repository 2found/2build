# Token usage investigation

Investigated 2026-09-29 against babysit `22f7df4` and locally installed OMP
`17.4.0`. Scope: verify the reported observability gap and identify the repair
boundary. No runtime changes were made.

Root cause: babysit has storage for supplied token counts, but no production
adapter that collects harness usage and attributes it to skill invocations or
project runs; project readers and analytics instructions also lack aggregation.

## Evidence

| Boundary | Finding |
|---|---|
| `internal/cmd/skill_runtime.go:96` | `skillExit` records counts only when both token flags are supplied. Otherwise usage remains unavailable. |
| `hooks/omp.ts` | Registers gates and session heartbeats, but no message-usage observer and no `skill enter`/`skill exit` calls. |
| `.claude/skills/references/preamble.md:296,346` | Bash writes legacy start/end JSON directly; neither path calls the runtime CLI or includes usage. |
| `.claude/skills/references/preamble.ps1:43,221` | PowerShell also writes legacy lifecycle rows directly. |
| `internal/cmd/foreman_progress.go:123` | Project events always construct unavailable usage. |
| `internal/cmd/foreman_project.go:38` | Project snapshots independently construct unavailable usage; changing the event writer alone cannot populate project totals. |
| `.claude/skills/analytics-review/SKILL.md:14` | Aggregates lifecycle duration/outcomes and decisions, with no token aggregation or usage-coverage instructions. |
| `web/src/views/SkillEvents.tsx:28` | Can display supplied token totals; the missing observation is upstream of this UI. |

The original runtime landed in `7c4413d` with a CLI and tests, without migrating
the preamble or adding a harness producer. The later OMP hook port (`620acb0`)
added gate/session integration, not usage collection. Repository searches found
`skill enter`/`skill exit` consumers in documentation and tests, not production
hook or preamble paths.

An isolated current-source CLI reproduction toggled only the supplied counts:

| `bbs skill exit` input | Result |
|---|---|
| No token flags | `available:false`, all token fields `null` |
| `--input-tokens 12 --output-tokens 7` | `available:true`, total `19` |

This confirms that the sink works; it does not establish live collection.
`TestSkillRuntimeAcceptsOnlyObservedUsage` itself only marshals a manually
populated struct, so it cannot catch missing adapter wiring.

Local all-history telemetry inspection, without emitting message contents:

| File | Rows | Usage absent | Usage unavailable | Usage observed |
|---|---:|---:|---:|---:|
| `skill-usage.jsonl` | 1,960 | 1,896 | 64 | 0 |
| `project-events.jsonl` | 19,028 | 0 | 19,028 | 0 |

The skill file included 52 foreman and 199 autopilot rows, all without observed
usage. No malformed rows were found. These are local record counts, not unique
production runs; test/manual provenance was not classified. Lifecycle telemetry
is flowing, so “analytics has nothing to aggregate” applies to tokens, not all
analytics.

## Available observation source

Installed OMP exposes `message_end` with the completed message in
`src/extensibility/extensions/types.ts:759`, and emits a detached snapshot in
`src/session/agent-session.ts:3584`. Assistant messages carry provider, model,
and usage (`pi-ai/src/types.ts:899`). The public upstream
[extension contract](https://github.com/can1357/oh-my-pi/blob/main/packages/coding-agent/src/extensibility/extensions/types.ts)
also exposes this notification. The integration is missing; OMP is not inherently
unable to supply usage.

OMP usage distinguishes input, output, cache reads, and cache writes. Its
OpenAI accounting computes total across those categories
(`pi-ai/src/providers/openai-shared.ts:448`). Passing only OMP's input/output
fields to the existing CLI would therefore omit cached tokens from that total.
Provider initialization can also contain zero counters before usage arrives;
presence of a usage object alone is not proof of an observed zero-token call.
Currency estimates require separate provenance and must not be presented as
invoiced cost. Other harnesses' collection contracts were not investigated here.

## Context footprint

Measured UTF-8 file bytes, counting each file once:

| File set | Foreman | Autopilot |
|---|---:|---:|
| `SKILL.md` + shared preamble + decision framework | 33,636 | 54,829 |
| Above + all skill-local `references/*.md` | 80,893 | 58,952 |

At the rough bytes/4 heuristic these are 8.4k/13.7k and 20.2k/14.7k tokens,
respectively. They are not tokenizer measurements, request usage, or bills.
Foreman reads references by phase, so its full reference inventory is not a
proven per-invocation bootstrap load. The counts exclude the live Orca guide,
workflow, project artifacts, repository instructions, conversation, and composed
skills. Reloads and cache accounting require request-level observations.

Autopilot's launch-cost doctrine is explicit at `SKILL.md:151–161`; it describes
an architectural constraint, not an implemented usage collection mechanism.

## Repair boundary

1. Collect completed assistant usage at the OMP adapter into a local numeric
   ledger, preserving source, provider/model, cache categories, observation
   status, and durable event identity. Do not store message content. Deduplicate
   replay and distinguish failed/aborted responses with missing observations.
2. Bind native session identity to ticket/run/dispatch and skill boundaries.
   The existing runtime sets `session` to its generated invocation ID and lacks
   project identifiers. Migrate both preambles to the common lifecycle path;
   replacing `printf` alone cannot discover usage. Keep a session/run total
   when finer skill attribution is unavailable rather than inventing a split.
3. Derive project totals from unique underlying observations, including workers
   and coordinator. Never sum overlapping autopilot/sub-skill totals or attach
   cumulative totals to every project lifecycle event and then sum those events.
   Report partial coverage when any participating session is unobserved.
4. Teach `analytics-review` to aggregate observed usage and report coverage;
   verify synthetic message → persisted observation → project total → report,
   plus replay, cache, missing-usage, nested-skill and telemetry-off cases.

This requires collection, identity, and aggregation changes; there is no
one-line exit-hook fix that closes the reported project-level gap. Context
reduction is a separate follow-up and should use measured load sets.

Verification: `go test ./internal/cmd -run 'TestSkill' -count=1` passed;
`bun test tests/test_hooks_omp.test.ts` passed (9 tests, 27 assertions); a binary
built into a temporary directory passed the flag-toggle reproduction above.
Temporary binary and isolated telemetry were removed. Existing setup-script
changes were left untouched.
