#!/usr/bin/env bash
# Contract checks for the Orca-native autonomous team.

set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
F="$ROOT/.claude/skills/team/SKILL.md"
A="$ROOT/.claude/skills/autopilot/SKILL.md"
C="$ROOT/.claude/skills/create-pr/SKILL.md"

# References are loaded at their decision boundaries after the entry skill was
# reduced. These smoke checks cover the assembled contract; Go tests exercise
# readiness, evidence freshness, completion and watcher behavior.
ENTRY="$F"
F="$(mktemp)"
trap 'rm -f "$F"' EXIT
cat "$ENTRY" "$ROOT"/.claude/skills/team/references/*.md > "$F"
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
  'An autonomous product team' 'The human touches a project twice' \
  'production release increment'

has_all "one-git-flow" \
  '../references/git-flow.md' 'Committing a single ticket is an unfinished job' 'launch a phase' \
  'task complexity, model tier'

has_all "bbs-owned-worker-lifecycle" \
  'worktrees and terminals only' 'bbs team task create' 'task launch' \
  'agent-launch' 'task abandon' 'durable bbs records'

has_all "foreman-owns-topology" \
  'bbs ticket ensure --mode=worktree' 'bbs team seal' 'bbs team integrate' \
  'bbs team land' 'apply' 'in dependency order'

has_all "finish-cleans-worker-workspace" \
  'every Dispatch of the' 'kept_worktrees' \
  'terminal close --worktree path:<worktreePath> --all' 'never substitute' \
  'the snapshot ref keeps that work'

has_all "bounded-worker-pool" \
  'FOREMAN_MAX_WORKERS' 'bbs team resource status' 'min(16, max(2, CPUs - 2))' \
  'positive integer' 'one writer per child worktree'

has_all "global-weighted-admission" \
  'machine-global weighted' 'bbs team resource reserve' 'parallel_global_units' 'ADMISSION' \
  'never expires' 'resource backpressure'


has_all "ticket-slice-sizing" \
  'independent, testable unit' \
  'Never' 'split one module across siblings' \
  'sub-ticket needing its own worktree' 'returns' \
  'one worker' 'up to `L`' 'split inside a worker' 'Phase 1 is a walking skeleton' \
  '## Split by context' 'Same context → one ticket' 'Disjoint contexts → separate tickets' \
  'Order alone is not a seam' 'One ticket per phase by default' \
  'Split by context, not by size' 'only where the invariant owns its own files'

has_all "autopilot-worker-execution-envelope" \
  '`AGENT_ROLE=orca`' 'already spawned' \
  'skips any developer `/goal`' 'task launch' \
  'exactly one `task done`'

if grep -q 'An open Team Dispatch' "$A" \
   && grep -q "use the spec's worker lifecycle instead" "$A" \
   && grep -q 'developer `/goal` handoff' "$A"; then
  ok "autopilot-accepts-foreman-dispatch-envelope"
else
  fail "autopilot-accepts-foreman-dispatch-envelope"
fi

has_all "ticket-runs-whole-loop-in-one-session" \
  'One context, one session' 'select its route once' \
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
  'the worker self-approves as' 'the lead reviews and approves' 'the lead approves' \
  'Exit 4 (rubric)' 'Exit 3 (floor) or 5 (hold/grant)' 'parks the' \
  'Never implement on an unapproved plan record' 'after two redirects'

has_all "ticket-session-cache-economics" \
  'Wake rarely, and warm' 'keep-warm' 'reply `ok`' 'chain commands' \
  'ticket|repair|refresh|redirect`' 'GATE=not-ready' 'carrying every gap' \
  'merge its repair and refresh triggers into one spec' 'manifest rows it owns, by path and heading'

has_all "whole-phase-one-wake" \
  'one wake per phase' 'Dependent tickets therefore launch' \
  'bbs team await --prereqs' 'AWAIT=not-ready' 'Stay light' \
  'dependent never displaces runnable work' '`messages:<n>` (an `L` plan review'

has_all "watcher-self-heals-lost-workers" \
  '`lost:<task>`' '`bbs team task abandon` it and close its terminal' \
  'follows a retried Task to its new Dispatch' 'never babysit it'

if grep -q -- '--unblocks\|settled:<task>' "$F"; then
  fail "no-per-wave-wake"
else
  ok "no-per-wave-wake"
fi

has_all "fresh-eyes-only-after-retry-budget" \
  'after its retry budget' 'Fresh read-only' 'diagnosis worker' \
  'a context' 'that has not already failed three times'

has_all "workflow-aware-dispatch" \
  'pointers.workflow' '`builder` unless the project defines its own' 'evidence-only'

has_all "phase-qa-gate" \
  '## Phase integration QA' 'bbs team integrate' 'one integration worker session' \
  'every criterion of' 'cross-ticket transitions' 'that exact commit'

has_all "durable-resume-and-finish" \
  'bbs team task list "$FOREMAN_ID"' 'Terminal handles are routing metadata' \
  'bbs ticket readiness --action review --json' 'bbs team snapshot'

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
  '--team-id <id>' 'bbs ticket claim' 'hard fence' 'Different parents may' \
  'one mutating lead'

has_all "direct-skill-invocation-is-primary" \
  'Direct skill invocation' 'default entrypoint' 'bbs team adopt' 'its actual agent' \
  '--agent <current-agent>' 'not a prerequisite' 'Re-adopt the current session'

has_all "live-change-intake" \
  'bbs team inbox' 'change-request' 'a settled ticket or silently replace a live assignment' \
  'prior Integration QA'

has_all "dag-emission-contract" \
  'bbs ticket dag "$PARENT" --mermaid' 'topology is built/changed' 'never re-type the edges' \
  'never writes ticket state' 'user requests status'

has_all "goal-compaction-and-days" \
  'persistent goal facility' 'Compaction is a cold-resume boundary' 'bbs team ensure <id>' \
  'never resume an ambiguous'

has_all "batched-wake-reconciliation" \
  'run `bbs team batch' 'then \*\*end the turn\*\*' 'Do not wait, actively check' \
  'bbs team batch show' '`BATCH_STATE=settled`' '`stalled:<task>`' \
  'start a polling timer' 'cold-resume boundary' 'hand it to `bbs team batch` and end the turn'

has_all "heavy-checks-deferred-and-merged" \
  'consolidated heavy QA' 'DEFERRED_HEAVY=' 'only when no light ticket is ready' \
  'never displaces light work' 'one simulator boot' 'BABYSIT_TICKET=<ticket>'
has_all "ticket-closeout-inside-phase" \
  '## Ticket close-out and phase landing' 'unfinished work' \
  'bbs team land "$PARENT" --phase "$K"' 'every prerequisite it adopted' \
  'never blind-retry' 'phase that does not compose'

has_all "basic-design-gates-phase-topology" \
  'Before any phase opens, pass' 'Project design checkpoint' \
  'Append `--auto` only when explicitly requested' 'recorded `auto`' 'accepted design' \
  'one prototype of the whole product' 'the last place you ask'

has_all "final-qa-is-last-phase-qa" \
  "last phase's cumulative QA is the project's final QA" 'Only then run' \
  'unreleased phase, missing gate' "before the lead's \`done\` heartbeat"

has_all "phase-release-control" \
  "## The human's two touchpoints" 'human only, under every posture' \
  'release "$PARENT" --phase "$K" --request' \
  'bbs team release "$PARENT" --phase "$K" --file receipt.json' \
  'refuses kind `release`' '\*\*stops the line\*\*' 'Pipeline one phase deep' \
  'cannot integrate until `k` is released' '`--auto` never grants a release' \
  "lowers a seed's release gate" 'bbs team readiness "$PARENT" --action release --phase "$K" --json'

has_all "two-human-touchpoints" \
  'No mid-run questions' 'write the \*\*spec\*\* back' 'the last place you ask' \
  'contract refuses `ask` on any phase but the last' '## Hand over' \
  'Decisions taken for you' 'bbs team recap "$PARENT"' 'Run one fix round alone' \
  'the brief touchpoint' 'review it at `high` effort against the brief'

has_all "decision-trail-and-alternatives" \
  'bbs team log "$PARENT" --phase' 'written from that trail, not memory' \
  '--supersedes <id>' '\*\*Check first\*\*' '`--phase start` row' \
  '## Alternatives considered' 'a one-way decision with no alternative weighed'

PROJECT_CONTRACT="$ROOT/.claude/skills/team/references/project-contract.md"
if python3 - "$PROJECT_CONTRACT" "$ROOT/.claude/skills/qa/SKILL.md" <<'PY'
from pathlib import Path
import sys
contract, qa = (Path(p).read_text() for p in sys.argv[1:])
for required in (
    'approval publish --kind project-plan',
    'Only current `approved` unlocks children',
    '`--auto` delegates the human design reviews',
    'auto: true', 'approval self-resolve',
    'bbs team report <parent>', 'atomically\nreplace parent `report.md`',
    '`LANDED`', '`RELEASED`', 'UNKNOWN',
    'bbs team integrate', 'bbs team land',
    'STALE and a rerun',
):
    assert required in contract, required
assert 'Team integration mode' in qa
assert 'tests that exact commit read-only' in qa
assert 'integration worktree, commit or switch revisions' in qa
assert 'seal --fixup' in qa
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
  'bbs team complete "$PARENT" --team "$FOREMAN_ID"' \
  'A `done` heartbeat alone is not completion' \
  'leaves its terminal open and idle' 'recap.html' \
  'blocked ticket or unfinished cleanup'

if ! grep -q 'bbs team mailbox' "$F" "$ROOT/.claude/skills/references/preamble.md" \
   && ! grep -q 'sleep 20' "$F" \
   && ! grep -q 'Copy the block below' "$F"; then
  ok "no-pane-or-mailbox-coordination"
else
  fail "no-pane-or-mailbox-coordination"
fi

has_all "bbs-report-and-settlement" \
  'refused and completes nothing' 'worker exited without reporting' \
  'the report does not substitute for a verdict'

PREAMBLE="$ROOT/.claude/skills/team/references/worker-lifecycle.md"
if grep -q 'never to an Orca mailbox' "$PREAMBLE" \
   && grep -q 'bbs team task ask' "$PREAMBLE" \
   && grep -q 'bbs team task done' "$PREAMBLE" \
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
  'external missed-event/restart backup' 'The lead never schedules its own status timer' \
  'or polls on empty waits' 'adopt/spawn start it automatically'

if grep -q 'bbs agent resolve --role worker' "$ROOT/.claude/skills/team/references/worker-routing.md"; then
  fail "worker-routing-avoids-coordinator-agent-resolution"
else
  ok "worker-routing-avoids-coordinator-agent-resolution"
fi

REF="$ROOT/.claude/skills/team/references/model-routing.md"
if grep -q 'model-routing.md' "$F" \
   && ! grep -q 'model-routing.md' "$A" \
   && grep -q '## Assignment routing' "$REF" \
   && grep -q 'bbs team model' "$REF" \
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
