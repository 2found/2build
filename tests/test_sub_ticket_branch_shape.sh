#!/usr/bin/env bash
# Regression guard for the assistant/orchestrator topology boundary.

set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
AUTOPILOT="$ROOT/.claude/skills/autopilot/SKILL.md"
BUILDER="$ROOT/.claude/skills/autopilot/workflows/builder.md"
TEAM_DIR="$ROOT/.claude/skills/team"

PASS=0; FAIL=0; FAIL_NAMES=()
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mok\033[0m  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); FAIL_NAMES+=("$1"); printf '  \033[0;31mFAIL\033[0m  %s\n' "$1"; }

if ! grep -q 'CHILD_BRANCH=' "$AUTOPILOT" "$BUILDER" \
   && ! grep -q 'orchestrate mode' "$AUTOPILOT" "$BUILDER"; then
  ok "autopilot-has-no-child-topology"
else
  fail "autopilot-has-no-child-topology"
fi

if grep -q 'origin.type=sub_ticket' "$BUILDER" \
   && grep -q 'worktree Team prepared' "$BUILDER"; then
  ok "builder-consumes-prepared-child-worktree"
else
  fail "builder-consumes-prepared-child-worktree"
fi

if grep -rq 'bbs ticket ensure --mode=worktree' "$TEAM_DIR" \
   && grep -rq 'bbs team land' "$TEAM_DIR"; then
  ok "team-owns-worktree-lifecycle"
else
  fail "team-owns-worktree-lifecycle"
fi

if grep -q 'parent carrying `manifest.md`' "$AUTOPILOT" \
   && grep -q 'hand it to `team`' "$AUTOPILOT" \
   && grep -q 'never dispatch or merge its children here' "$AUTOPILOT"; then
  ok "decomposed-parent-routes-to-team"
else
  fail "decomposed-parent-routes-to-team"
fi

echo
echo "PASS: $PASS  FAIL: $FAIL"
if [ "$FAIL" -gt 0 ]; then
  printf '  - %s\n' "${FAIL_NAMES[@]}"
  exit 1
fi
