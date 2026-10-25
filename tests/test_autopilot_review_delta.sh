#!/usr/bin/env bash
# tests/test_autopilot_review_delta.sh — the review-gate protocol documented in
# autopilot/references/verification.md and SKILL.md § Current-session gates
# works against the real producer: `data.id`, the "already owns" recovery,
# strict results keys, `--fix` failing the attempt, and a delta re-review
# (tree snapshot + review_tree pointer) recording a passing gate.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

BBS="$TMP/bbs"
( cd "$ROOT" && go build -o "$BBS" ./cmd/bbs ) || { echo "FAIL: go build" >&2; exit 1; }

export HOME="$TMP/home" BABYSIT_HOME="$TMP/state" BABYSIT_PROJECT_HOME="$TMP/state/projects/repo"
export BABYSIT_TICKET=delta-ticket
mkdir -p "$HOME" "$BABYSIT_PROJECT_HOME"
unset BBS_TICKET CODEX_SESSION_ID CODEX_THREAD_ID GROK_SESSION_ID GROK_HOOK_EVENT CLAUDE_CODE_SESSION_ID

REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO" || exit 1
git init -qb main; git config user.email t@t; git config user.name t
echo x > f; git add f; git commit -qm init

TICKET_HOME="$BABYSIT_PROJECT_HOME/tickets/delta-ticket"
mkdir -p "$TICKET_HOME"
echo requirement > "$TICKET_HOME/requirement.md"
echo plan > "$TICKET_HOME/plan.md"
"$BBS" ticket init >/dev/null 2>&1 || true
"$BBS" autopilot checkpoint --ticket delta-ticket --workflow builder \
  --step run --status in_progress >/dev/null || { echo "FAIL: checkpoint"; exit 1; }

fail=0
expect() { # $1=name $2=want $3=got
  if [ "$2" = "$3" ]; then echo "ok $1"; else echo "FAIL $1: want '$2', got '$3'"; fail=1; fi
}
# The snapshot recipe from SKILL.md, verbatim.
snap() { i=$(mktemp -u); GIT_INDEX_FILE=$i git read-tree HEAD && GIT_INDEX_FILE=$i git add -A && GIT_INDEX_FILE=$i git write-tree; rm -f "$i"; }
begin() {
  "$BBS" autopilot verification begin --gate review-pr --owner w1 --handle s1 \
    --transport claude-code --harness claude-code
}

echo change >> f; echo new > untracked.txt
T1=$(snap)
expect "snapshot leaves the index alone" "" "$(git diff --cached --name-only)"

A1=$(begin | jq -r .data.id)
case "$A1" in review-pr-*) echo "ok begin returns data.id" ;; *) echo "FAIL begin: $A1"; fail=1 ;; esac
expect "second begin names the open attempt" \
  "active attempt $A1 already owns this ticket" "$(begin | jq -r .error.message)"
REV=$("$BBS" autopilot attempt show --id "$A1" | jq -r .data.attempt.revision)
expect "own open attempt can be failed" failed "$("$BBS" autopilot attempt update --id "$A1" \
  --expect-revision "$REV" --json '{"state":"failed","failure":{"reason":"test"}}' | jq -r .data.state)"

A2=$(begin | jq -r .data.id)
expect "checkpoint keeps the open attempt" "$A2" "$(jq -r .active_attempt_id "$TICKET_HOME/checkpoint.json")"
echo "typecheck ok" > "$TMP/check.log"
jq -n --arg cwd "$REPO" --arg log "$TMP/check.log" \
  '{checks:[{argv:["true"],cwd:$cwd,exit_code:0,log_path:$log}],unresolved_findings:[],limitations:[]}' > "$TMP/results.json"
jq '. + {evidence:"analysis"}' "$TMP/results.json" > "$TMP/extra.json"
case "$("$BBS" autopilot verification record --attempt "$A2" --file "$TMP/extra.json" | jq -r .error.message)" in
  *'unknown field "evidence"'*) echo "ok results file rejects extra keys" ;;
  *) echo "FAIL extra key accepted"; fail=1 ;;
esac

echo fix >> f # what review-pr --fix does to the tree
expect "--fix fails the attempt" "verification subject changed; begin a new gate on the repaired tree" \
  "$("$BBS" autopilot verification record --attempt "$A2" --file "$TMP/results.json" | jq -r .error.message)"

"$BBS" ticket set-pointer review_tree "$T1"
expect "review_tree pointer persists" "$T1" "$("$BBS" ticket get pointers.review_tree)"
T2=$(snap)
expect "delta holds only the fix" "f" "$(git diff --name-only "$T1" "$T2")"

A3=$(begin | jq -r .data.id)
expect "delta gate records" true "$("$BBS" autopilot verification record --attempt "$A3" --file "$TMP/results.json" | jq -r .ok)"
expect "delta gate passes review readiness" pass \
  "$("$BBS" ticket readiness --action review --json | jq -r '.data.gates[] | select(.name=="review-pr") | .state')"

exit "$fail"
