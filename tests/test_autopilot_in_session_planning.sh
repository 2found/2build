#!/usr/bin/env bash
# autopilot runs every step in the session it was launched in — planning
# included. No planner subagent, no per-harness QA child, no routed model: the
# session model executes its assigned phases. This suite pins that contract,
# and keeps configurable model routing honest for the one skill that still
# reads it (foreman).
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SKILL="$ROOT/.claude/skills/autopilot/SKILL.md"
REF="$ROOT/.claude/skills/references/model-routing.md"
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
forbid 'Native subagents need no new pane' "$SKILL"

# --- workflows carry no "automatic QA subagent path" ------------------------
for f in "$WORKFLOWS"/*.md; do
  forbid 'automatic QA subagent' "$f"
  forbid 'automatic QA path' "$f"
done

# --- model routing belongs to foreman, which launches phase workers ----------
require '`foreman` routes each supervised' "$REF"
require '`autopilot` deliberately does not route models' "$REF"
require 'launches Plan, Build, Review and QA as phase-scoped supervised sessions' "$REF"
forbid '`autopilot` routes its planner' "$REF"
require 'bbs agent resolve --role worker --json' "$REF"
require 'worker_provider' "$REF"
require 'worker_model' "$REF"
require 'worker_effort' "$REF"
require 'An empty provider/model/effort means native default' "$REF"
require '## Phase routing' "$REF"
require '| Code review and review diagnosis | **strong** |' "$REF"
require '| Implementation and code repairs | **normal** |' "$REF"
require '| Per-ticket QA, integration QA and product acceptance checks | **normal** |' "$REF"
require 'launch flag' "$REF"
require 'came back short' "$REF"
require '| `simple` | an obvious local docs/config edit' "$REF"
require '| `critical` / `hard` | security, auth, money' "$REF"
forbid 'gpt-5.6-sol' "$REF"
forbid 'grok-4.6' "$REF"
forbid '## Ladders' "$REF"

# --- no README still advertises the removed flags ---------------------------
for f in "$ROOT"/README.md "$ROOT"/README.zh.md "$ROOT"/README.ja.md "$ROOT"/README.ko.md "$ROOT"/README.vi.md; do
  forbid '--planner' "$f"
done

echo "autopilot in-session planning contract: ok"
