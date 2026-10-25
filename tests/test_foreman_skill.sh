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
  'Autonomous Orca orchestrator' 'release phases for one human review' \
  'production release increment'

has_all "one-git-flow" \
  '../references/git-flow.md' 'Committing a single ticket is an unfinished job' 'launch a phase' \
  'task complexity, model tier'

has_all "bbs-owned-worker-lifecycle" \
  'worktrees and terminals only' 'bbs foreman task create' 'task launch' \
  'agent-launch' 'task abandon' 'durable bbs records'

has_all "foreman-owns-topology" \
  'bbs ticket ensure --mode=worktree' 'bbs foreman seal' 'bbs foreman integrate' \
  'bbs foreman land' 'apply' 'in dependency order'

has_all "finish-cleans-worker-workspace" \
  'every Dispatch of the' 'kept_worktrees' \
  'terminal close --worktree path:<worktreePath> --all' 'never substitute' \
  'the snapshot ref keeps that work'

has_all "bounded-worker-pool" \
  'FOREMAN_MAX_WORKERS' 'bbs foreman resource status' 'min(16, max(2, CPUs - 2))' \
  'positive integer' 'one writer per child worktree'

has_all "global-weighted-admission" \
  'machine-global weighted' 'bbs foreman resource reserve' 'parallel_global_units' 'ADMISSION' \
  'never expires' 'resource backpressure'


has_all "ticket-slice-sizing" \
  'independent, testable unit' \
  'Never split' 'coherent change across siblings' \
  'sub-ticket needing its own worktree' 'returns' \
  'one worker' 'up to `L`' 'prefer fewer, larger tickets' \
  'split inside a worker' 'Phase 1 is a walking skeleton' '2–5 tickets per phase'

has_all "autopilot-worker-execution-envelope" \
  '`AGENT_ROLE=orca`' 'already spawned' \
  'skips any developer `/goal`' 'task launch' \
  'exactly one `task done`'

if grep -q 'An open Foreman Dispatch' "$A" \
   && grep -q "use the spec's worker lifecycle instead" "$A" \
   && grep -q 'developer `/goal` handoff' "$A"; then
  ok "autopilot-accepts-foreman-dispatch-envelope"
else
  fail "autopilot-accepts-foreman-dispatch-envelope"
fi

has_all "ticket-runs-whole-loop-in-one-session" \
  'One ticket is one worker session' 'select its route once' \
  'every follow-up' 'without stopping between steps' 'Adopt prerequisites' \
  'CONFLICT=' 'no separate' 'approval self-resolve'

has_all "ticket-session-ownership" \
  '### Ticket session ownership' 'fresh' 'session' \
  'foreman_keep_settled_workers' 'shut' \
  '`repair`' '`refresh`' '`redirect`' '`deliver`' \
  '### Reuse a ticket session' '--terminal "$TICKET_TERMINAL"' '`transport-matched`' \
  'Never reuse one ticket' 'holds no lease'

has_all "plan-approval-by-ticket-size" \
  '### Plan approval by ticket size' 'Ticket plan checkpoints run inside the ticket session' \
  'the worker self-approves as' 'Foreman reviews and approves' 'Foreman approves' \
  'Exit 4 (rubric)' 'Exit 3 (floor) or 5 (hold/grant)' 'parks the' \
  'Never implement on an unapproved plan record' 'after two redirects'

has_all "whole-phase-one-wake" \
  'one wake per phase' 'Dependent tickets therefore launch' \
  'bbs foreman await --prereqs' 'AWAIT=not-ready' 'Foreman stays light' \
  'dependent never displaces runnable work' '`messages:<n>` (an `L` plan review'

has_all "watcher-self-heals-lost-workers" \
  '`lost:<task>`' '`bbs foreman task abandon` it and close its terminal' \
  'follows a retried Task to its new Dispatch' 'never babysit it'

if grep -q -- '--unblocks\|settled:<task>' "$F"; then
  fail "no-per-wave-wake"
else
  ok "no-per-wave-wake"
fi

has_all "fresh-eyes-only-after-retry-budget" \
  'after its retry budget' 'Fresh read-only' 'diagnosis worker' \
  'a context' 'that has not already failed three times'

has_all "lifecycle-aware-dispatch" \
  'pointers.workflow' 'default to `builder` only' 'LIFECYCLE' 'TRIGGER' 'evidence-only'

has_all "phase-qa-gate" \
  '## Phase integration QA' 'bbs foreman integrate' 'one integration worker session' \
  'every criterion of' 'cross-ticket transitions' 'that exact commit'

has_all "durable-resume-and-finish" \
  'bbs foreman task list "$FOREMAN_ID"' 'Terminal handles are routing metadata' \
  'bbs ticket readiness --action review --json' 'bbs foreman snapshot'

has_all "ensure-before-init-ordering" \
  'ensure --mode=worktree' '--from-input-file "$SEED_PATH"' \
  'never pre-create' 'fast-path no-op' 'origin-type sub_ticket' \
  'from inside' 'landed on' 'env -u BABYSIT_TICKET bbs ticket ensure --no-branch'

has_all "land-never-stashes" \
  'Never stash, reset or overwrite them' 'integrate` again' 'base moved since the build'

has_all "native-task-list-init" \
  'native task list from parent, children' \
  'DAG; rebuild it from' 'ticket + task state'

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

has_all "batched-wake-reconciliation" \
  'run `bbs foreman batch' 'then \*\*end the turn\*\*' 'Do not wait, actively check' \
  'bbs foreman batch show' '`BATCH_STATE=settled`' '`stalled:<task>`' \
  'start a polling timer' 'cold-resume boundary' 'hand it to `bbs foreman batch` and end the turn'

has_all "heavy-checks-deferred-and-merged" \
  'consolidated heavy QA' 'DEFERRED_HEAVY=' 'only when no light ticket is ready' \
  'never displaces light work' 'one simulator boot' 'BABYSIT_TICKET=<ticket>'
has_all "ticket-closeout-inside-phase" \
  '## Ticket close-out and phase landing' 'unfinished work' \
  'bbs foreman land "$PARENT" --phase "$K"' 'every prerequisite it adopted' \
  'never blind-retry' 'phase that does not compose'

has_all "basic-design-gates-phase-topology" \
  'Before any phase opens, pass' 'Project design checkpoint' \
  'Append `--auto` only when explicitly requested' 'recorded `auto`' 'accepted design' \
  'one prototype of the whole product' 'in \*\*one batch\*\*, now'

has_all "final-qa-is-last-phase-qa" \
  "last phase's cumulative QA is the project's final QA" 'Only then run' \
  'unreleased phase, missing gate' 'before the Foreman `done` heartbeat'

has_all "phase-release-control" \
  '## Human control points' 'human only\*\*, under every posture' \
  'bbs foreman release "$PARENT" --phase "$K" --request' \
  'bbs foreman release "$PARENT" --phase "$K" --file receipt.json' \
  'refuses kind `release`' '\*\*stops the line\*\*' 'Pipeline one phase deep' \
  'cannot integrate until `k` is released' '`--auto` never grants a release' \
  'lowers a phase' 'bbs foreman readiness "$PARENT" --action release --phase "$K" --json'

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
    '`LANDED`', '`RELEASED`', 'UNKNOWN',
    'bbs foreman integrate', 'bbs foreman land',
    'STALE and a rerun',
):
    assert required in contract, required
assert 'Foreman integration mode' in qa
assert 'tests that exact commit read-only' in qa
assert 'Do not edit code, commit' in qa
PY
then
  ok "project-approval-report-and-final-surface-contract"
else
  fail "project-approval-report-and-final-surface-contract"
fi

has_all "ticket-status-closeout" \
  'bbs ticket set-status done' 'parent becomes `done`' 'every phase landed'

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
  'blocked ticket or unfinished cleanup'

if ! grep -q 'bbs foreman mailbox' "$F" "$ROOT/.claude/skills/references/preamble.md" \
   && ! grep -q 'sleep 20' "$F" \
   && ! grep -q 'Copy the block below' "$F"; then
  ok "no-pane-or-mailbox-coordination"
else
  fail "no-pane-or-mailbox-coordination"
fi

has_all "bbs-report-and-settlement" \
  'refused and completes nothing' 'worker exited without reporting' \
  'the report does not substitute for a verdict'

PREAMBLE="$ROOT/.claude/skills/foreman/references/worker-lifecycle.md"
if grep -q 'never to an Orca mailbox' "$PREAMBLE" \
   && grep -q 'bbs foreman task ask' "$PREAMBLE" \
   && grep -q 'bbs foreman task done' "$PREAMBLE" \
   && grep -q -- '--resume <id>' "$PREAMBLE" \
   && grep -q 'whole Dispatch, not each nested skill' "$PREAMBLE" \
   && grep -q -- '--outcome succeeded|failed' "$PREAMBLE" \
   && grep -q 'Send no status messages' "$PREAMBLE" \
   && ! grep -q 'orca orchestration' "$PREAMBLE" \
   && ! grep -q 'worker_done' "$PREAMBLE"; then
  ok "worker-uses-bbs-task-lifecycle"
else
  fail "worker-uses-bbs-task-lifecycle"
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
   && grep -q '## Assignment routing' "$REF" \
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
