#!/usr/bin/env bash
# autopilot runs every step in the session it was launched in — planning
# included. Task/phase routing selects Foreman's launches; Autopilot executes
# in the human-opened or Foreman-opened session without model selection.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SKILL="$ROOT/.claude/skills/autopilot/SKILL.md"
REF="$ROOT/.claude/skills/foreman/references/model-routing.md"
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

# --- autopilot: planning and both gates are in-session ----------------------
require '### Planning runs in this session' "$SKILL"
require 'executes in the current autopilot session, on' "$SKILL"
require 'There is no `--planner` flag and no per-step model' "$SKILL"
require '### Current-session gates (`review-pr`, `qa`)' "$SKILL"
require 'Both gates run in this session, on the session'"'"'s model' "$SKILL"
require 'Run QA in this session too.' "$SKILL"
require 'step is ever dispatched to a child, a second session, or an external process' "$SKILL"
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
require 'Foreman owns task classification, phase splitting and model selection before' "$SKILL"
require 'report the mismatch to Foreman before work' "$SKILL"
require 'human chooses the replacement session; Autopilot does not select another model' "$SKILL"
forbid 'model to restart the run on' "$SKILL"
forbid 'Native subagents need no new pane' "$SKILL"

# --- workflows carry no "automatic QA subagent path" ------------------------
for f in "$WORKFLOWS"/*.md; do
  forbid 'automatic QA subagent' "$f"
  forbid 'automatic QA path' "$f"
done

# --- foreman selects the route before opening each phase worker ------------
require '`autopilot` does not select models' "$REF"
require 'Foreman splits work into phase assignments' "$REF"
forbid '`autopilot` routes its planner' "$REF"
require 'bbs agent detect --json' "$REF"
require 'bbs agent list --json' "$REF"
require 'New worker routes select the explicit agent first' "$REF"
require 'BBS global agent/provider/model/effort preferences' "$REF"
require 'bytes remain unchanged' "$REF"
require '`bbs config set` rejects new writes' "$REF"
require '`bbs agent resolve`' "$REF"
require '## Phase routing' "$REF"
require 'launch.effective' "$REF"
require 'came back short' "$REF"
require 'an obvious local docs/config edit' "$REF"
require 'security, auth, money' "$REF"
require 'complexity `critical` to `hard`' "$REF"
require 'explicit phase-specific user selection first, then a valid persisted' "$REF"
require 'config changes alone do not change it' "$REF"
require 'Legacy strong/normal routes lacking the' "$REF"
require 'An unmapped agent needs an explicit phase route' "$REF"
require 'unknown binding needs `NEEDS_CONTEXT`' "$REF"
require '`BLOCKED` before dispatch' "$REF"
require 'Never replace a live writer' "$REF"
require 'Do not invent an OMP effort' "$REF"
require 'Standalone Autopilot runs all steps in the human-opened session' "$REF"
require 'load this routing table, recommend a tier, or change models between phases' "$REF"


# --- routing policy is CLI-owned: assert live defaults, plus the doc's
# phase-class table which stays in markdown ------------------------------
BBS_BIN_DIR="$(mktemp -d)"
POLICY_DIR="$(mktemp -d)"
trap 'rm -rf "$BBS_BIN_DIR" "$POLICY_DIR"' EXIT
( cd "$ROOT" && go build -o "$BBS_BIN_DIR/bbs" ./cmd/bbs ) || { echo "FAIL: go build" >&2; exit 1; }

require 'bbs foreman model' "$REF"
require 'The CLI owns complexity-to-tier routing' "$REF"

"$BBS_BIN_DIR/bbs" foreman model --dir "$POLICY_DIR" --json > "$BBS_BIN_DIR/policy.json"
python3 - "$REF" "$BBS_BIN_DIR/policy.json" <<'PY'
import json, sys
from pathlib import Path

policy = json.loads(Path(sys.argv[2]).read_text())


assert policy["routing"] == {
    "simple": {"normal": "flash", "critical": "flash"},
    "normal": {"normal": "flash", "critical": "pro"},
    "hard":   {"normal": "pro",   "critical": "max"},
}, policy["routing"]
assert policy["tiers"] == {
    "flash": {"codex": {"model": "gpt-6-luna", "effort": "high"},
              "claude": {"model": "opus", "effort": "high"},
              "omp": {"model": "@normal"}},
    "pro":   {"codex": {"model": "gpt-5.6-sol", "effort": "high"},
              "claude": {"model": "opus", "effort": "high"},
              "omp": {"model": "@slow"}},
    "max":   {"codex": {"model": "gpt-6-astra", "effort": "high"},
              "claude": {"model": "opus", "effort": "high"},
              "omp": {"model": "@plan"}},
}, policy["tiers"]
print("foreman model policy defaults: ok")

text = Path(sys.argv[1]).read_text()
rows = [[cell.strip().strip('`') for cell in line.strip('|').split('|')]
        for line in text.splitlines() if line.startswith('|')]
phases = {r[0]: r[1] for r in rows if len(r) == 2 and r[1] in ('critical', 'normal')}
assert phases == {
    'Parent/child planning, decomposition, design, design feedback': 'critical',
    'Code review and review diagnosis': 'critical',
    'Mechanical re-verification of recorded checks on unchanged code': 'normal',
    'Implementation and code repairs': 'normal',
    'Per-ticket QA, integration QA and product acceptance checks': 'normal',
    'Finish audits, merges, composition, authorized delivery, restoration and cleanup': 'normal',
}, phases
PY

# --- no README still advertises the removed flags ---------------------------
for f in "$ROOT"/README.md "$ROOT"/README.zh.md "$ROOT"/README.ja.md "$ROOT"/README.ko.md "$ROOT"/README.vi.md; do
  forbid '--planner' "$f"
done

echo "autopilot in-session planning contract: ok"
