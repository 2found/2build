#!/usr/bin/env bash
# tests/test_autopilot_readiness_gate.sh — policy/readiness consumer boundary.
#
# The v2 packet makes policy observation read-only. An unconfigured repository
# uses the resolver's pet defaults; a workflow must not manufacture a startup
# git-flow file while it is executing. Snapshot must be non-mutating and the
# builder must consume it without writing policy.

set -u
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BBS_AUTOPILOT="$SCRIPT_DIR/bbs"
BUILDER_MD="$SCRIPT_DIR/.claude/skills/autopilot/workflows/builder.md"
[ -x "$BBS_AUTOPILOT" ] || { echo "FAIL: $BBS_AUTOPILOT not executable" >&2; exit 1; }
[ -f "$BUILDER_MD" ]    || { echo "FAIL: $BUILDER_MD missing"          >&2; exit 1; }

PASS=0
FAIL=0
FAIL_NAMES=()
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mok\033[0m  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); FAIL_NAMES+=("$1"); printf '  \033[0;31mFAIL\033[0m  %s\n' "$1"; [ $# -gt 1 ] && printf '        %s\n' "$2"; }

# ── v2-snapshot-is-read-only ────────────────────────────────────────
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"
  unset BABYSIT_TICKET BBS_TICKET
  git init -q "$T/repo"; cd "$T/repo"
  git -c user.email=t@t -c user.name=t commit --allow-empty -q -m init
  out="$("$BBS_AUTOPILOT" autopilot snapshot --json)"
  printf '%s' "$out" | jq -e '.schema_version == 2 and .ok == true and .data.ticket == null' >/dev/null \
    || { echo "snapshot did not return a valid no-ticket envelope"; exit 1; }
  [ ! -e .babysit ] || { echo "snapshot created ticket state"; exit 1; }
) && ok "v2-snapshot-is-read-only" || fail "v2-snapshot-is-read-only"
rm -rf "$T"

# ── builder-uses-read-only-policy ───────────────────────────────────
(
  grep -q 'snapshot --json' "$BUILDER_MD" || { echo "builder does not capability-check snapshot"; exit 1; }
  grep -q 'never create or rewrite' "$BUILDER_MD" || { echo "builder can still write policy"; exit 1; }
  grep -q '\$bbs:setup-project' "$BUILDER_MD" || { echo "no setup-project recommendation"; exit 1; }
) && ok "builder-uses-read-only-policy" || fail "builder-uses-read-only-policy"

echo
if [ "$FAIL" -eq 0 ]; then
  printf '\033[0;32mPASS\033[0m  %d checks\n' "$PASS"
  exit 0
else
  printf '\033[0;31mFAIL\033[0m  %d/%d checks failed: %s\n' "$FAIL" "$((PASS + FAIL))" "${FAIL_NAMES[*]}"
  exit 1
fi
