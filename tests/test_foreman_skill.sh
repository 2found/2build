#!/usr/bin/env bash
# Contract checks for the Orca-native autonomous foreman.

set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
F="$ROOT/.claude/skills/foreman/SKILL.md"
A="$ROOT/.claude/skills/autopilot/SKILL.md"
C="$ROOT/.claude/skills/create-pr/SKILL.md"

# References are loaded at their decision boundaries after the entry skill was
# reduced. These smoke checks cover the assembled contract; Go tests exercise
# readiness, evidence freshness, completion and watcher behavior.
ENTRY="$F"
F="$(mktemp)"
trap 'rm -f "$F"' EXIT
cat "$ENTRY" "$ROOT"/.claude/skills/foreman/references/*.md > "$F"
PASS=0; FAIL=0; FAIL_NAMES=()
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mok\033[0m  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); FAIL_NAMES+=("$1"); printf '  \033[0;31mFAIL\033[0m  %s\n' "$1"; }

has_all() {
  local name="$1"; shift
  local needle
  for needle in "$@"; do
    grep -q -- "$needle" "$F" || { fail "$name"; return; }
  done
  ok "$name"
}

has_all "autonomous-project-scope" \
  'Autonomous Orca orchestrator' 'multiple tickets or dependent features' \
  'Foreman owns the ticket DAG'

has_all "repository-autonomy-profiles" \
  'bbs autopilot git-flow' 'profile sets review/QA breadth' 'admitted ready wave' \
  'task complexity, model tier'

has_all "live-orchestration-contract" \
  '~/.claude/skills/orchestration/SKILL.md' 'skills get orchestration' \
  'worker-start' 'check --wait' 'worker-release' 'request recovery'

has_all "foreman-owns-topology" \
  'bbs ticket ensure --mode=worktree' 'git worktree list' 'git worktree remove' 'apply' \
  'in dependency order'

has_all "finish-cleans-worker-workspace" \
  'terminal its Dispatch owns' 'verified-clean non-primary Git' \
  'terminal close --worktree path:<worktreePath> --all' 'mandatory even under `review`' \
  'Git worktree for human inspection' 'keep the branch'

has_all "bounded-worker-pool" \
  'bbs config get parallel_max_workers' 'MAX_WORKERS=4' \
  'positive integer' 'one writer per child worktree'

has_all "global-weighted-admission" \
  'machine-global weighted' 'bbs foreman resource reserve' 'parallel_global_units' 'ADMISSION' \
  'never expires' 'resource backpressure'


has_all "child-slice-sizing" \
  'independent, testable, releasable units' \
  'never split one coherent change across siblings' \
  'sub-ticket needing its own worktree returns to Foreman' \
  'explicit phases inside its own ticket'

has_all "autopilot-worker-execution-envelope" \
  '`AGENT_ROLE=orca`' 'already spawned' \
  'skips any developer `/goal`' 'worker-start --agent' \
  'exactly one `worker_done`'

if grep -q 'authenticated, current Orca Dispatch preamble' "$A" \
   && grep -q 'use the injected lifecycle instead' "$A" \
   && grep -q 'developer `/goal` handoff' "$A"; then
  ok "autopilot-accepts-orca-dispatch-envelope"
else
  fail "autopilot-accepts-orca-dispatch-envelope"
fi

has_all "agent-independent-design-gate" \
  'Keep separate phase Dispatches' '--stop-after=plan' 'approved' 'approval self-resolve'

has_all "lifecycle-aware-dispatch" \
  'pointers.workflow' 'default to `builder` only' 'LIFECYCLE' 'TRIGGER' 'evidence-only'

has_all "project-qa-gate" \
  'Integration QA Task' 'bbs ticket surface compose' 'surface lease' 'read-only QA worker' \
  'parent scope and cross-ticket flows'

has_all "durable-resume-and-finish" \
  'pointers.orca_run' 'Terminal handles are routing metadata' \
  'bbs ticket readiness --action <review|land|pr> --json' 'BBS_FINISH=review | land | pr'

has_all "ensure-before-init-ordering" \
  'ensure --mode=worktree' '--from-input-file "$SEED_PATH"' \
  'never pre-create' 'fast-path no-op' 'origin-type sub_ticket' \
  'from inside its own worktree'

has_all "revert-before-land" \
  'surface revert' 'bbs-serving' 'BLOCKs' 'dependency order'

has_all "native-task-list-init" \
  'Initialize the native task list' 'parent, children' \
  'DAG; rebuild it from ticket + Orca state'

P="$ROOT/.claude/skills/references/preamble.md"
if grep -q 'MUST mirror' "$P" \
   && grep -q 'update_plan' "$P" \
   && grep -q 'TaskCreate' "$P" \
   && grep -q '`todo`' "$P"; then
  ok "preamble-task-list-per-harness"
else
  fail "preamble-task-list-per-harness"
fi

has_all "single-writer-multi-foreman" \
  '--foreman-id <id>' 'bbs ticket claim' 'hard fence' 'Different parents may' \
  'one mutating Foreman'

has_all "direct-skill-invocation-is-primary" \
  'Direct skill invocation' 'default entrypoint' 'bbs foreman adopt' 'its actual agent' \
  '--agent <current-agent>' 'not a prerequisite' 'Re-adopt the current session'

has_all "live-change-intake" \
  'bbs foreman inbox' 'change-request' 'a settled ticket or silently replace a live assignment' \
  'prior Integration QA'

has_all "dag-emission-contract" \
  'bbs ticket dag "$PARENT" --mermaid' 'topology is built/changed' 'never re-type the edges' \
  'never writes ticket state' 'user requests status'

has_all "goal-compaction-and-days" \
  'persistent goal facility' 'Compaction is a cold-resume boundary' 'bbs foreman ensure <id>' \
  'never resume an ambiguous'

has_all "event-driven-reconciliation" \
  'block on Orca `check --wait`' 'Do not actively check terminals' \
  'An empty timeout only re-arms the wait' 'Do not' 'start a polling timer' 'reconciliation' \
  'return to the blocking wait'
has_all "eager-per-ticket-finish" \
  '## Eager per-ticket finish' 'settled workers need not wait' \
  'bbs ticket land <child>' 'create-pr' 'dependency order' \
  'Final Integration QA' 'resets local base' \
  'pointers.pr' 'merge-base --is-ancestor' \
  'Supervised repair Dispatch' 'never blind-retry' \
  'under `land`' 'final integration QA does not' '`review`'

has_all "parent-design-gates-child-topology" \
  'Before step 2, pass' 'Project design checkpoint' \
  'Append `--auto` only when explicitly requested' 'recorded `auto`' 'accepted parent design' \
  'all'

has_all "final-qa-after-finish-before-done" \
  'exact delivered base' 'retained QA/wave' 'final project QA' 'Only then run' 'No missing gate' \
  'before the Foreman `done` heartbeat'

PROJECT_CONTRACT="$ROOT/.claude/skills/foreman/references/project-contract.md"
if python3 - "$PROJECT_CONTRACT" "$ROOT/.claude/skills/qa/SKILL.md" <<'PY'
from pathlib import Path
import sys
contract, qa = (Path(p).read_text() for p in sys.argv[1:])
for required in (
    'approval publish --kind project-plan',
    'Only current `approved` unlocks children',
    '`--auto` delegates the human design reviews',
    'auto: true', 'approval self-resolve',
    'bbs foreman report <parent>', 'atomically\nreplace parent `report.md`',
    'PR_READY', 'LANDED_LOCAL', 'MERGED_REMOTE', 'UNKNOWN',
    'Never move `<base>`', 'No `-B`, force update or deletion',
    'Do not use\n   `surface revert` as restoration',
    'Changed inputs mean STALE',
):
    assert required in contract, required
assert 'Foreman final integration mode' in qa
assert "coordinator's lease" in qa
assert 'This explicit mode takes precedence' in qa
PY
then
  ok "project-approval-report-and-final-surface-contract"
else
  fail "project-approval-report-and-final-surface-contract"
fi

has_all "ticket-status-closeout" \
  'bbs ticket set-status done' 'bbs ticket set-status in_review' \
  'only after the PR is observed merged' 'parent becomes `done`' 'remains `in_review`'

if grep -q 'bbs ticket set-status in_review' "$C" \
   && grep -q 'status becomes `done` only after the PR is observed merged' "$C"; then
  ok "create-pr-persists-review-status"
else
  fail "create-pr-persists-review-status"
fi


has_all "status-wake-full-snapshot" \
  'For a status request' 'show every project Task and supervised worker' \
  'resource use and the DAG' 'IN_PROGRESS'

has_all "terminal-done-heartbeat" \
  'bbs foreman complete "$PARENT" --foreman "$FOREMAN_ID"' \
  'A `done` heartbeat alone is not completion' \
  'watcher closes only the adopted Foreman terminal' \
  'blocked required ticket or unfinished cleanup'

if ! grep -q 'bbs foreman mailbox' "$F" "$ROOT/.claude/skills/references/preamble.md" \
   && ! grep -q 'sleep 20' "$F" \
   && ! grep -q 'Copy the block below' "$F"; then
  ok "no-pane-or-mailbox-coordination"
else
  fail "no-pane-or-mailbox-coordination"
fi

has_all "orca-delivery-and-settlement" \
  'Process every message in a Delivery before acknowledging its' 'exact id' \
  'accepted lifecycle settlement' 'reports do not complete work'

PREAMBLE="$ROOT/.claude/skills/foreman/references/worker-lifecycle.md"
if grep -q 'live injected Orca preamble is authoritative' "$PREAMBLE" \
   && grep -q 'orca orchestration ask' "$PREAMBLE" \
   && grep -q 'orca orchestration check' "$PREAMBLE" \
   && grep -q 'orca orchestration send' "$PREAMBLE" \
   && grep -q 'whole Dispatch, not each nested skill' "$PREAMBLE" \
   && grep -q 'both lifecycle IDs' "$PREAMBLE" \
   && grep -q -- '--outcome failed' "$PREAMBLE" \
   && ! grep -q 'MAILBOX=off' "$PREAMBLE"; then
  ok "worker-uses-injected-orca-lifecycle"
else
  fail "worker-uses-injected-orca-lifecycle"
fi

if grep -q '"skills": "./.claude/skills"' "$ROOT/.codex-plugin/plugin.json"; then
  ok "codex-plugin-exposes-claude-skills"
else
  fail "codex-plugin-exposes-claude-skills"
fi

has_all "no-coordinator-polling-timer" \
  'external missed-event/restart backup' 'Foreman never schedules its own status timer' \
  'or polls on empty waits' 'adopt/spawn start it automatically'

if grep -q 'bbs agent resolve --role worker' "$ROOT/.claude/skills/foreman/references/worker-routing.md"; then
  fail "worker-routing-avoids-coordinator-agent-resolution"
else
  ok "worker-routing-avoids-coordinator-agent-resolution"
fi

REF="$ROOT/.claude/skills/foreman/references/model-routing.md"
if grep -q 'model-routing.md' "$F" \
   && ! grep -q 'model-routing.md' "$A" \
   && grep -q '## Phase routing' "$REF" \
   && grep -q 'bbs foreman model' "$REF" \
   && grep -q '~/.babysit/settings.json' "$REF" \
   && ! grep -q 'gpt-6.1-sol' "$REF"; then
  ok "canonical-model-policy-cli-owned"
else
  fail "canonical-model-policy-cli-owned"
fi

echo
echo "PASS: $PASS  FAIL: $FAIL"
if [ "$FAIL" -gt 0 ]; then
  printf '  - %s\n' "${FAIL_NAMES[@]}"
  exit 1
fi
