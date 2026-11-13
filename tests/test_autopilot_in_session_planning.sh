#!/usr/bin/env bash
# autopilot judges both gates in the session it was launched in and hands
# only its named subagent steps to a subagent. Task/phase routing selects the
# lead's launches; Autopilot selects no model beyond design-ui's pinned one.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SKILL="$ROOT/.claude/skills/autopilot/SKILL.md"
TEAM_ASSIGNMENT="$ROOT/.claude/skills/autopilot/references/team-assignment.md"
REF="$ROOT/.claude/skills/team/references/model-routing.md"
WORKFLOWS="$ROOT/.claude/skills/autopilot/workflows"

require() {
  local needle=$1 file=$2
  grep -Fq -- "$needle" "$file" || {
    echo "missing contract '$needle' in $file" >&2
    exit 1
  }
}

forbid() {
  local needle=$1 file=$2
  if grep -Fq -- "$needle" "$file"; then
    echo "stale contract '$needle' still present in $file" >&2
    exit 1
  fi
}

# --- autopilot: subagent steps; both gates stay in-session ------------------
require '### Subagent steps (`design-ui`, `plan-draft`, `investigate`, `perf-audit`, `implement`)' "$SKILL"
require '**`implement`: one subagent per plan milestone**' "$SKILL"
require 'coded in this session. An `implement` subagent never invokes `design-ui`:' "$SKILL"
require 'first milestone the checkpoint does not record as accepted' "$SKILL"
require 'run in this session that needs `design-ui` dispatches it the same way' "$SKILL"
require 'is called with the session'"'"'s own model alias' "$SKILL"
require 'SKILL.md § Subagent steps applies in' "$TEAM_ASSIGNMENT"
require 'plan one subagent per milestone (SKILL.md § Subagent steps)' "$WORKFLOWS/builder.md"
require '**`design-ui` always runs in a subagent**' "$SKILL"
require 'Claude Code: `model: "opus"`,' "$SKILL"
require '`effort: "high"`' "$SKILL"
require '`Make it really good, use lots of tokens and iterate till you'"'"'re proud of it.`' "$SKILL"
require 'never from inside a `plan-draft` subagent' "$SKILL"
require '**`plan-draft`, `investigate` and `perf-audit`: decide per run.**' "$SKILL"
require 'There is no `--planner` flag and no per-step model' "$SKILL"
require '### Current-session gates (`review-pr`, `qa`)' "$SKILL"
require 'Both gates are judged in this session, on the session'"'"'s model' "$SKILL"
require 'Judge QA here; execute it in one cheaper subagent.' "$SKILL"
require 'Executor mode' "$SKILL"
require '## Executor mode' "$ROOT/.claude/skills/qa/SKILL.md"
require 'Edit no code, run no git' "$ROOT/.claude/skills/qa/SKILL.md"
require 'gate is ever dispatched to a child, a second session, or an external process' "$SKILL"
require 'bbs ticket verdict-status --skill plan-draft' "$SKILL"
require 'bbs ticket ensure --no-branch' "$SKILL"
require 'BABYSIT_TICKET="$TICKET"' "$SKILL"

# --- autopilot: the delegation surface is gone ------------------------------
forbid '--planner <model>' "$SKILL"
forbid '--planner-effort' "$SKILL"
forbid 'planner_model' "$SKILL"
forbid 'planner_effort' "$SKILL"
forbid 'OMP launch rule' "$SKILL"
forbid 'automatic QA subagent' "$SKILL"
forbid '../references/model-routing.md' "$SKILL"
require 'Autopilot does not select a model tier or reroute the session' "$SKILL"
require 'The lead owns ticket decomposition, task classification and model selection before' "$TEAM_ASSIGNMENT"
require '### Team ticket assignment' "$SKILL"
require 'independent: build against the accepted contracts' "$TEAM_ASSIGNMENT"
require 'Never implement on an unapproved plan record' "$TEAM_ASSIGNMENT"
require '**Plan approval** — every plan' "$TEAM_ASSIGNMENT"
forbid 'bbs team await' "$SKILL"
require 'set-pointer deferred_heavy' "$TEAM_ASSIGNMENT"
require 'Never commit, push or open a PR' "$TEAM_ASSIGNMENT"
forbid 'may explicitly assign only' "$SKILL"
require 'report the mismatch to the lead before work' "$TEAM_ASSIGNMENT"
require 'human chooses the replacement session; Autopilot does not select another model' "$SKILL"
forbid 'model to restart the run on' "$SKILL"
forbid 'Native subagents need no new pane' "$SKILL"

# --- workflows carry no "automatic QA subagent path" ------------------------
for f in "$WORKFLOWS"/*.md; do
  forbid 'automatic QA subagent' "$f"
  forbid 'automatic QA path' "$f"
done

# --- team selects one route per ticket before opening its worker -------
require '`autopilot` does not select models' "$REF"
require 'The lead splits each release phase into tickets and selects one route per ticket' "$REF"
forbid '`autopilot` routes its planner' "$REF"
require 'bbs agent detect --json' "$REF"
require 'bbs agent list --json' "$REF"
require 'New worker routes select the explicit agent first' "$REF"
require 'BBS global agent/provider/model/effort preferences' "$REF"
require 'bytes remain unchanged' "$REF"
require '`bbs config set` rejects new writes' "$REF"
require '`bbs agent resolve`' "$REF"
require '## Assignment routing' "$REF"
require 'verifies the launched agent' "$REF"
require 'came back short' "$REF"
require 'an obvious local docs/config edit' "$REF"
require 'money, auth, security, live-data or concurrency' "$REF"
require 'complexity `critical` to `hard`' "$REF"
require 'explicit ticket-specific user selection first, then a valid persisted' "$REF"
require 'config changes alone do not change it' "$REF"
require 'Legacy strong/normal routes lacking the' "$REF"
require 'An unmapped agent needs an explicit ticket route' "$REF"
require 'unknown binding needs `NEEDS_CONTEXT`' "$REF"
require '`BLOCKED` before dispatch' "$REF"
require 'Never replace a live writer' "$REF"
require 'Do not invent an OMP effort' "$REF"
require 'Standalone Autopilot runs all steps in the human-opened session' "$REF"
require 'load this routing table, recommend a tier, or change models between steps' "$REF"


# --- routing policy is CLI-owned: assert live defaults, plus the doc's
# phase-class table which stays in markdown ------------------------------
BBS_BIN_DIR="$(mktemp -d)"
POLICY_DIR="$(mktemp -d)"
trap 'rm -rf "$BBS_BIN_DIR" "$POLICY_DIR"' EXIT
( cd "$ROOT" && go build -o "$BBS_BIN_DIR/bbs" ./cmd/bbs ) || { echo "FAIL: go build" >&2; exit 1; }

require 'bbs team model' "$REF"
require 'The CLI owns complexity-to-tier routing' "$REF"

"$BBS_BIN_DIR/bbs" team model --dir "$POLICY_DIR" --json > "$BBS_BIN_DIR/policy.json"
python3 - "$REF" "$BBS_BIN_DIR/policy.json" <<'PY'
import json, sys
from pathlib import Path

policy = json.loads(Path(sys.argv[2]).read_text())


assert policy["routing"] == {
    "simple": {"normal": "flash", "critical": "pro",   "design": "pro"},
    "normal": {"normal": "pro",   "critical": "pro",   "design": "pro"},
    "hard":   {"normal": "pro",   "critical": "max",   "design": "max"},
}, policy["routing"]
assert policy["tiers"] == {
    "lite":  {"codex": {"model": "gpt-6-luna", "effort": "high"},
              "claude": {"model": "haiku"},
              "omp": {"model": "@smol"}},
    "flash": {"codex": {"model": "gpt-6.1-sol", "effort": "high"},
              "claude": {"model": "sonnet", "effort": "high"},
              "omp": {"model": ""}},
    "pro":   {"codex": {"model": "gpt-6.1-sol", "effort": "high"},
              "claude": {"model": "opus", "effort": "medium"},
              "omp": {"model": "@slow"}},
    "max":   {"codex": {"model": "gpt-6-astra", "effort": "high"},
              "claude": {"model": "opus", "effort": "high"},
              "omp": {"model": "@plan"}},
}, policy["tiers"]
assert "offload" not in policy, policy
print("foreman model policy defaults: ok")

text = Path(sys.argv[1]).read_text()
rows = [[cell.strip().strip('`') for cell in line.strip('|').split('|')]
        for line in text.splitlines() if line.startswith('|')]
phases = {r[0]: r[1] for r in rows if len(r) == 2 and r[1] in ('critical', 'normal', 'design')}
assert phases == {
    'Basic design: architecture, prototype, release-phase plan, design review': 'design',
    "Ticket worker: one session for the ticket's whole loop and every follow-up on it": 'critical',
    'Mechanical re-verification of recorded checks on unchanged code': 'normal',
    'Phase integration: QA of the phase commit, cumulative criteria, then land': 'normal',
    'Consolidated heavy QA and product acceptance checks': 'normal',
    'Phase release: `release` and its hypercare': 'normal',
    'Handover fix: every in-scope feedback item, integrate, land, re-test': 'normal',
    'Finish audits, restoration and cleanup in a fresh session': 'normal',
}, phases
PY

# --- gate cost contracts: delta re-review and tiered review subagents --------
VERIFY="$ROOT/.claude/skills/autopilot/references/verification.md"
REVIEW="$ROOT/.claude/skills/review-pr/SKILL.md"
require '**Review once.** One `review-pr --fix` pass is the review gate.' "$SKILL"
require 'One `review-pr --fix` pass is the review gate.' "$VERIFY"
require 'The file takes exactly these three keys' "$VERIFY"
require 'returns its ID as `data.id`' "$VERIFY"
require '## 5. Subagent model routing (medium and above)' "$REVIEW"
require 'Phase 2 verifier and the sweep finder — on the mid tier.' "$REVIEW"
require '**Effort is capped at `medium`** unless the human' "$REVIEW"
require 'route above this session'"'"'s model' "$REVIEW"
require 'When the subagent tool has no model selector' "$REVIEW"

# --- no README still advertises the removed flags ---------------------------
for f in "$ROOT"/README.md "$ROOT"/README.zh.md "$ROOT"/README.ja.md "$ROOT"/README.ko.md "$ROOT"/README.vi.md; do
  forbid '--planner' "$f"
done

echo "autopilot in-session planning contract: ok"
