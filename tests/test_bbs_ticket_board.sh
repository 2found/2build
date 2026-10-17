#!/usr/bin/env bash
# tests/test_bbs_ticket_board.sh — coverage for `bbs ticket` § board.
#
# board = read-only aggregated view: every ticket joined with its verdicts,
# branch, session, PR pointer, and siblings, plus a surface-lease footer.
# Zero mutation — running board must never change any state.
#
# Scenarios:
#   board-rows-and-verdicts   both tickets listed; A's qa verdict DONE, B none;
#                             branch column from manifest; footer FREE
#   board-lease               lease held by A → footer shows owner; released →
#                             FREE
#   board-status-filter       done ticket hidden by default, shown with --all
#   board-sibling-unresolved  sibling with unset RELATED_* env → "path
#                             unresolved" sub-row, main row intact, rc 0
#   board-base-drift          BASE line: matches origin after push; ahead after
#                             a phase lands on local base, with the "awaiting a
#                             release push" sub-line; "no origin ref" when the
#                             repo has no remote

set -u
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BBS_TICKET_BIN="$SCRIPT_DIR/bbs"

PASS=0; FAIL=0; FAIL_NAMES=()
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mok\033[0m  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); FAIL_NAMES+=("$1"); printf '  \033[0;31mFAIL\033[0m  %s\n' "$1"; [ $# -gt 1 ] && printf '        %s\n' "$2"; }

# Fixture: bare origin + clone, then two worktree tickets A and B, each with
# one uncommitted file. Sets globals
# WT_A/WT_B (worktree paths) and TK_A/TK_B (ticket ids). Runs in $1/repo.
build_two_tickets() {
  local t="$1"
  git init -q --bare "$t/remote.git"
  git init -q "$t/repo"
  (
    cd "$t/repo"
    git -c user.email=t@t -c user.name=t commit --allow-empty -q -m init
    git branch -M main
    git remote add origin "$t/remote.git"
    git push -q origin main
  )
  cd "$t/repo"
  local out
  out="$("$BBS_TICKET_BIN" ticket ensure --mode=worktree --slug-hint tick-a --type feat 2>/dev/null)" || return 1
  TK_A="$(printf '%s\n' "$out" | sed -n 's|^TICKET=||p')"
  WT_A="$(printf '%s\n' "$out" | sed -n 's|^WORKTREE=||p')"
  out="$("$BBS_TICKET_BIN" ticket ensure --mode=worktree --slug-hint tick-b --type feat 2>/dev/null)" || return 1
  TK_B="$(printf '%s\n' "$out" | sed -n 's|^TICKET=||p')"
  WT_B="$(printf '%s\n' "$out" | sed -n 's|^WORKTREE=||p')"
  [ -n "$WT_A" ] && [ -n "$WT_B" ] || return 1
  echo "work a" > "$WT_A/a.txt"
  echo "work b" > "$WT_B/b.txt"
}

# ── board-rows-and-verdicts ───────────────────────────────────────────
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"
  export HOME="$T/home"; mkdir -p "$HOME"
  export AGENT_ROLE=mayor
  build_two_tickets "$T" || { echo "fixture failed"; exit 1; }
  # verdicts/qa.md is the verification producer's projection; write it as one.
  th="$(BABYSIT_TICKET="$TK_A" "$BBS_TICKET_BIN" ticket path home --read)" \
    || { echo "ticket home failed"; exit 1; }
  mkdir -p "$th/verdicts" && printf 'STATUS: DONE\n' > "$th/verdicts/qa.md"

  out="$("$BBS_TICKET_BIN" ticket board 2>"$T/err")"; rc=$?
  [ "$rc" -eq 0 ] || { echo "board failed rc=$rc: $(cat "$T/err")"; exit 1; }
  row_a="$(printf '%s\n' "$out" | grep "^$TK_A ")" || { echo "A row missing: $out"; exit 1; }
  printf '%s\n' "$out" | grep -q "^$TK_B " || { echo "B row missing: $out"; exit 1; }
  printf '%s' "$row_a" | grep -q "DONE" || { echo "A's qa verdict not shown: $row_a"; exit 1; }
  printf '%s' "$row_a" | grep -q "feat/${TK_A}_tick-a" || { echo "A's branch not shown: $row_a"; exit 1; }
  # PUSHED renders the manifest bool through Python's repr — "False", not "false".
  [ "$(printf '%s' "$row_a" | awk '{print $5}')" = "False" ] \
    || { echo "A's PUSHED column should be False: $row_a"; exit 1; }
  printf '%s\n' "$out" | grep -q "^QA-LEASE: FREE$" || { echo "expected free lease: $out"; exit 1; }
  printf '%s\n' "$out" | grep -q "^SERVING:" && { echo "board still reports a served composition: $out"; exit 1; }
  true
) && ok "board-rows-and-verdicts" || fail "board-rows-and-verdicts"
rm -rf "$T"

# ── board-lease ───────────────────────────────────────────────────────
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"
  export HOME="$T/home"; mkdir -p "$HOME"
  export AGENT_ROLE=mayor
  build_two_tickets "$T" || { echo "fixture failed"; exit 1; }

  "$BBS_TICKET_BIN" ticket surface acquire --ticket "$TK_A" >/dev/null 2>&1 || { echo "acquire failed"; exit 1; }
  out="$("$BBS_TICKET_BIN" ticket board)"
  printf '%s\n' "$out" | grep -q "^QA-LEASE: $TK_A " || { echo "lease owner not shown: $out"; exit 1; }
  BABYSIT_TICKET="$TK_A" "$BBS_TICKET_BIN" ticket surface release >/dev/null 2>&1
  "$BBS_TICKET_BIN" ticket board | grep -q "^QA-LEASE: FREE$" \
    || { echo "expected a free lease after release"; exit 1; }
) && ok "board-lease" || fail "board-lease"
rm -rf "$T"

# ── board-status-filter ───────────────────────────────────────────────
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"
  export HOME="$T/home"; mkdir -p "$HOME"
  export AGENT_ROLE=mayor
  build_two_tickets "$T" || { echo "fixture failed"; exit 1; }
  BABYSIT_TICKET="$TK_A" "$BBS_TICKET_BIN" ticket set-status done >/dev/null 2>&1 \
    || { echo "set-status failed"; exit 1; }

  out="$("$BBS_TICKET_BIN" ticket board)"
  printf '%s\n' "$out" | grep -q "^$TK_A " && { echo "done ticket shown by default: $out"; exit 1; }
  printf '%s\n' "$out" | grep -q "^$TK_B " || { echo "B row missing: $out"; exit 1; }
  "$BBS_TICKET_BIN" ticket board --all | grep -q "^$TK_A " \
    || { echo "done ticket missing under --all"; exit 1; }
) && ok "board-status-filter" || fail "board-status-filter"
rm -rf "$T"

# ── board-sibling-unresolved ──────────────────────────────────────────
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"
  export HOME="$T/home"; mkdir -p "$HOME"
  export AGENT_ROLE=mayor
  build_two_tickets "$T" || { echo "fixture failed"; exit 1; }
  BABYSIT_TICKET="$TK_A" "$BBS_TICKET_BIN" ticket set-sibling \
    --role be --repo ghost-api --ticket bs-ghost000 >/dev/null 2>&1 || { echo "set-sibling failed"; exit 1; }

  out="$("$BBS_TICKET_BIN" ticket board 2>"$T/err")"; rc=$?
  [ "$rc" -eq 0 ] || { echo "board failed rc=$rc: $(cat "$T/err")"; exit 1; }
  printf '%s\n' "$out" | grep -q "^$TK_A " || { echo "main row lost: $out"; exit 1; }
  printf '%s\n' "$out" | grep -q "be:bs-ghost000 (ghost-api) — path unresolved" \
    || { echo "expected unresolved sub-row: $out"; exit 1; }
) && ok "board-sibling-unresolved" || fail "board-sibling-unresolved"
rm -rf "$T"

# ── board-base-drift ──────────────────────────────────────────────────
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"
  export HOME="$T/home"; mkdir -p "$HOME"
  export AGENT_ROLE=mayor
  build_two_tickets "$T" || { echo "fixture failed"; exit 1; }

  # main was pushed by the fixture and nothing has landed on it yet.
  "$BBS_TICKET_BIN" ticket board | grep -q "^BASE: main — matches origin/main" \
    || { echo "expected in-sync BASE line: $("$BBS_TICKET_BIN" ticket board)"; exit 1; }

  # A landed phase sits on local main until its release pushes it.
  echo "phase" > phase.txt && git add phase.txt \
    && git -c user.email=t@t -c user.name=t commit -q -m "phase 1"
  out="$("$BBS_TICKET_BIN" ticket board)"
  printf '%s\n' "$out" | grep -qE "^BASE: main — [1-9][0-9]* ahead / 0 behind origin/main" \
    || { echo "expected ahead BASE line: $out"; exit 1; }
  printf '%s\n' "$out" | grep -q "landed phases awaiting a release push" \
    || { echo "expected the awaiting-release sub-line: $out"; exit 1; }

  # base_branch names a branch that exists on origin but not locally: the line
  # must say so rather than vanishing (the one case that used to print nothing).
  git push -q origin main:develop
  git fetch -q origin
  BBS_BASE_BRANCH=develop "$BBS_TICKET_BIN" ticket board | grep -q "^BASE: develop — no local 'develop' branch" \
    || { echo "expected the missing-local-base line: $(BBS_BASE_BRANCH=develop "$BBS_TICKET_BIN" ticket board)"; exit 1; }

  # A repo with no remote must say so rather than printing a bogus count.
  git init -q "$T/solo"
  (
    cd "$T/solo"
    git -c user.email=t@t -c user.name=t commit --allow-empty -q -m init
    git branch -M main
  )
  cd "$T/solo"
  "$BBS_TICKET_BIN" ticket ensure --slug-hint solo --type feat >/dev/null 2>&1 || { echo "solo ensure failed"; exit 1; }
  "$BBS_TICKET_BIN" ticket board | grep -q "^BASE: main — no origin/main ref" \
    || { echo "expected no-origin BASE line: $("$BBS_TICKET_BIN" ticket board)"; exit 1; }
) && ok "board-base-drift" || fail "board-base-drift"
rm -rf "$T"

echo
if [ "$FAIL" -eq 0 ]; then
  printf '\033[0;32mPASS\033[0m %d scenario(s)\n' "$PASS"
  exit 0
else
  printf '\033[0;31mFAIL\033[0m %d / %d failed:\n' "$FAIL" "$((PASS + FAIL))"
  for n in "${FAIL_NAMES[@]}"; do printf '  - %s\n' "$n"; done
  exit 1
fi
