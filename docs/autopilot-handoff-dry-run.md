# Autopilot handoff dry run

2026-09-29. Result: repaired instruction contracts pass static checks and real
CLI artifact round trips in isolated temporary repositories. This is a no-model
dry run, not a live autonomous run or product QA. No push, PR, landing, or model
launch was performed.

Root cause: member skills described results without consistently publishing the
artifacts their consumers read; workflow examples also omitted required report
bodies, and standalone scope assumed ticket/remote state.

## Producer → consumer

| Producer | Durable output | Consumer / verified boundary |
|---|---|---|
| design-ui | Canonical design file and design pointer; prototype location | plan-draft accepts disk or conversation spec; design path/pointer round trip passes |
| plan-draft | Plan file, plan pointer, STATUS-bearing verdict | Autopilot can read verdict status; implement can read plan after resume |
| implement | Sequenced change brief: SUMMARY, FILES, APPROACH, BLAST_RADIUS, deviations and checks | QA reads the latest implement handoff; documented command round trip passes |
| review-pr | Full report with STATUS, FINDINGS, FIXED, RISK_AREAS | QA reads full review; findings-only JSON is correctly rejected as a verdict |
| qa | Full report with sources, rubric, final evidence and cleanup | QA evidence audit accepts the fixture and detects stale freshness; blocked status fails readiness |
| v2 verification producer | Typed attempt/receipt and compact verdict projection | Full review survives separately in reviews; missing and stale typed evidence fail readiness |

## Repaired gaps

- Replaced the invalid positional add-handoff example with actual CLI flags.
- Made plan/design persistence and implement handoff publication explicit.
- Added review's missing status-bearing output and full report persistence.
- Preserved full review/QA bodies separately from v2 verdict projections, which
  omit findings, risk areas, rubric and cleanup. Consumers read both surfaces.
- Added report bodies to workflow verdict writes: a bare set-verdict call really
  overwrites an existing verdict with a placeholder. Those writes are v1-only;
  v2 uses its verification producer.
- Removed nonexistent security-review/phased-build dependencies from the touched
  QA/reason instructions; corrected the stale QA section reference and sibling
  ticket environment variable. Unattended prototyper no longer calls the
  developer-only office-hours path. Maintainer retains current-checkout ownership.
- Matched browse/investigate/setup-project output verdicts to the shared table.

## Standalone contracts

| Skill | Supported inputs without autopilot | Remaining real requirements |
|---|---|---|
| implement | Request or conversation plan/design; dirty main; no ticket | Enough intent to implement the change; required design can invoke design-ui |
| qa | Supplied behavior and local/URL target; scratch evidence; no ticket/plan/remote | Runnable client/target and credentials for required coverage; cleanup still applies |
| review-pr | Explicit PR/range/files or local diff; no ticket/plan/remote | Medium+ requires agent fan-out capability; explicit low works without agents |

The standalone fixtures use dirty main with no origin, plan or resolved ticket,
read changed and untracked files, and verify no ticket was created. Skill-text
assertions check the fallback contracts; they do not prove model compliance.
URL-only QA, browser behavior and live medium+ review fan-out were not executed.
An unspecified committed target with no usable base still needs clarification.

## Verification

- `python3 -m pytest tests/test_skill_handoff_contracts.py -q`: 10 passed.
- `bash tests/test_archetype_workflows.sh`: 47 checks passed.
- `bash tests/test_foreman_skill.sh`: 41 checks passed.
- `bash tests/test_autopilot_readiness_gate.sh`: 4 checks passed.
- `bash tests/test_autopilot_v2_readiness.sh`: missing denies, current passes,
  stale denies.
- Skill-reference and in-session planning checks passed; git diff whitespace
  check passed. Existing workflow assertions were updated for compact wording,
  retaining their original behavioral requirements.

Legacy readiness reads statuses, not complete coverage. The QA instructions now
explicitly run qa-evidence after persistence; autopilot must still reconcile
required coverage and unresolved findings. Typed evidence also depends on honest
reporting of executed checks; these fixtures are labeled as contract tests.

## Context footprint after handoff repairs

Snapshot before the subsequent Foreman transport isolation described below.

UTF-8 bytes against HEAD, counting each file once; not measured input tokens:

| File set | Before | After | Reduction |
|---|---:|---:|---:|
| 24 skills + preamble + decision framework + finding-unknowns | 164,877 | 96,384 | 41.5% |
| Foreman skill + preamble + decision framework | 33,636 | 21,605 | 35.8% |
| Autopilot skill + preamble + decision framework | 54,829 | 29,596 | 46.0% |

Foreman's core plus all local references is 80,893 → 68,862 bytes (14.9%);
autopilot's is 58,952 → 34,202 (42.0%). References load by phase, so inventory
size does not establish per-invocation cost. Token collection remains the
separate plumbing gap documented in token-usage-investigation.md.

## Foreman-only transport boundary

Follow-up: moved the worker lifecycle adapter and model-routing policy under
foreman/references. Foreman includes the adapter in every worker assignment;
autopilot and shared preambles accept a generic caller envelope. Setup and shared
git-flow instructions no longer carry runtime-specific ownership details. The
old shared model-routing path is a short compatibility link.

The boundary regression scans all non-Foreman skill Markdown for Orca,
worker_done and orchestration-skill dependencies, and checks Foreman still
injects its lifecycle. All 11 handoff/boundary tests pass, as do reference and
in-session planning checks. Foreman's suite reports 40 passed and one unrelated
failure: the current workspace has deleted skills/foreman/SKILL.md, which its
Codex wrapper check still expects. That existing deletion was not changed.
