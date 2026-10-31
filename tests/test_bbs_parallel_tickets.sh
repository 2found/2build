#!/usr/bin/env bash
# tests/test_bbs_parallel_tickets.sh — several tickets in flight at once.
# Foreman cuts a batch of worktrees against one primary checkout and their
# workers share one surface lease, so "two runs hit the same thing at the same
# moment" is the designed usage rather than an edge case; this suite covers
# the interactions no single-ticket test can see.
#
# Scenarios:
#   ensure-race-distinct-tickets    N concurrent ensure → N distinct ids/worktrees
#   ensure-serializes-worktree-add  the cut waits on the add-lock rather than
#                                   running `git worktree add` alongside it
#   force-release-vs-acquire        release --force racing acquire → at most one
#                                   owner, never a lease dir without an owner file

set -u
unset ORCA_TERMINAL_HANDLE  # never let a test cut worktrees in the real Orca app
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BBS_TICKET_BIN="$SCRIPT_DIR/bbs"

PASS=0; FAIL=0; FAIL_NAMES=()
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mok\033[0m  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); FAIL_NAMES+=("$1"); printf '  \033[0;31mFAIL\033[0m  %s\n' "$1"; [ $# -gt 1 ] && printf '        %s\n' "$2"; }

# Bare origin + clone, main pushed.
build_repo() {
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
}

# build_tickets <tmpdir> <n> — n worktree tickets, each with one distinct
# uncommitted file f<i>.txt. Fills TKS/WTS.
build_tickets() {
  local t="$1" n="$2" i out
  TKS=(); WTS=()
  cd "$t/repo"
  for i in $(seq 1 "$n"); do
    out="$("$BBS_TICKET_BIN" ticket ensure --mode=worktree --slug-hint "tick-$i" --type feat 2>/dev/null)" || return 1
    TKS+=("$(printf '%s\n' "$out" | sed -n 's|^TICKET=||p')")
    WTS+=("$(printf '%s\n' "$out" | sed -n 's|^WORKTREE=||p')")
  done
  for i in $(seq 1 "$n"); do
    echo "work $i" > "${WTS[$((i-1))]}/f$i.txt" || return 1
  done
}


# ── ensure-race-distinct-tickets ──────────────────────────────────────
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"; export HOME="$T/home"; mkdir -p "$HOME"
  export AGENT_ROLE=mayor
  build_repo "$T"; cd "$T/repo"

  # 16 and not 6: `git worktree add` is not concurrency-safe, and at 6 workers
  # the build without the add-lock still passed most runs (measured 0/25). At 16
  # it fails ~30% of trials, which is a signal a repeated CI run will surface.
  # The deterministic half of this coverage is ensure-serializes-worktree-add.
  rm -f "$T/go"
  for i in $(seq 1 16); do
    ( while [ ! -f "$T/go" ]; do :; done      # start barrier: collide, don't queue
      "$BBS_TICKET_BIN" ticket ensure --mode=worktree --slug-hint "para-$i" --type feat >"$T/e$i" 2>&1 ) &
  done
  : > "$T/go"; wait

  n_ok=$(grep -l '^TICKET=' "$T"/e* 2>/dev/null | wc -l | tr -d ' ')
  [ "$n_ok" -eq 16 ] || { echo "only $n_ok/16 ensures produced a ticket: $(cat "$T"/e*)"; exit 1; }
  uniq_t=$(cat "$T"/e* | sed -n 's|^TICKET=||p' | sort -u | wc -l | tr -d ' ')
  [ "$uniq_t" -eq 16 ] || { echo "ticket ids collided: $(cat "$T"/e* | sed -n 's|^TICKET=||p' | sort | uniq -d)"; exit 1; }
  uniq_w=$(cat "$T"/e* | sed -n 's|^WORKTREE=||p' | sort -u | wc -l | tr -d ' ')
  [ "$uniq_w" -eq 16 ] || { echo "worktree paths collided"; exit 1; }
  while read -r w; do [ -d "$w" ] || { echo "worktree missing: $w"; exit 1; }; done \
    < <(cat "$T"/e* | sed -n 's|^WORKTREE=||p')
  [ "$(git branch --show-current)" = "main" ] || { echo "primary moved off main"; exit 1; }
) && ok "ensure-race-distinct-tickets" || fail "ensure-race-distinct-tickets"
rm -rf "$T"

# ── ensure-serializes-worktree-add ────────────────────────────────────
# The probabilistic scenario above only fails ~30% of the time, so pin the
# mechanism too: hold the add-lock and the cut must wait for it rather than
# running a second `git worktree add` alongside whoever holds it.
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"; export HOME="$T/home"; mkdir -p "$HOME"
  export AGENT_ROLE=mayor
  build_repo "$T"; cd "$T/repo"

  mkdir "$T/repo/.git/bbs-worktree-add.lock"      # stand in for an add in flight
  "$BBS_TICKET_BIN" ticket ensure --mode=worktree --slug-hint held --type feat >"$T/out" 2>&1 &
  pid=$!
  sleep 2
  kill -0 "$pid" 2>/dev/null || { echo "ensure finished while the add-lock was held"; exit 1; }
  n=$(find "$T/repo/.babysit/worktrees" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')
  [ "$n" -eq 0 ] || { echo "worktree was cut while the add-lock was held ($n dirs)"; exit 1; }

  rmdir "$T/repo/.git/bbs-worktree-add.lock"
  wait "$pid" || { echo "ensure failed after the lock was released: $(cat "$T/out")"; exit 1; }
  grep -q '^TICKET=' "$T/out" || { echo "no ticket emitted: $(cat "$T/out")"; exit 1; }
  w="$(sed -n 's|^WORKTREE=||p' "$T/out")"
  [ -d "$w" ] || { echo "worktree missing after release: $w"; exit 1; }
  [ -d "$T/repo/.git/bbs-worktree-add.lock" ] && { echo "add-lock left behind"; exit 1; }
  :
) && ok "ensure-serializes-worktree-add" || fail "ensure-serializes-worktree-add"
rm -rf "$T"

# ── force-release-vs-acquire ──────────────────────────────────────────
# `release --force` is the "that run is dead" escape hatch; a racer acquiring
# at the same moment must not end up co-owner or leave a half-built lease.
T="$(mktemp -d)"
(
  export PATH="$SCRIPT_DIR:$PATH"; export HOME="$T/home"; mkdir -p "$HOME"
  export AGENT_ROLE=mayor
  build_repo "$T"; build_tickets "$T" 3 || { echo "fixture failed"; exit 1; }

  for trial in 1 2 3 4 5 6 7 8; do
    rm -rf "$T/repo/.git/bbs-qa-lease"
    (cd "${WTS[0]}" && "$BBS_TICKET_BIN" ticket surface acquire >/dev/null 2>&1) || { echo "seed acquire failed"; exit 1; }
    rm -f "$T/go"
    ( while [ ! -f "$T/go" ]; do :; done
      cd "$T/repo" && "$BBS_TICKET_BIN" ticket surface release --force >"$T/fr" 2>&1 ) &
    for i in 2 3; do
      ( while [ ! -f "$T/go" ]; do :; done
        cd "${WTS[$((i-1))]}" && "$BBS_TICKET_BIN" ticket surface acquire >"$T/a$i" 2>&1 ) &
    done
    : > "$T/go"; wait

    won=$(grep -l 'ACQUIRED=1' "$T"/a[23] 2>/dev/null | wc -l | tr -d ' ')
    [ "$won" -le 1 ] || { echo "trial $trial: $won owners after a forced release: $(cat "$T"/a[23])"; exit 1; }
    if [ -d "$T/repo/.git/bbs-qa-lease" ]; then
      [ -f "$T/repo/.git/bbs-qa-lease/owner" ] || { echo "trial $trial: lease dir left with no owner file"; exit 1; }
    fi
    if [ "$won" -eq 1 ]; then
      claimed=$(grep -l 'ACQUIRED=1' "$T"/a[23] | head -1)
      want=$(sed -n 's|^OWNER=||p' "$claimed" | head -1)
      got=$(cd "$T/repo" && "$BBS_TICKET_BIN" ticket surface status 2>/dev/null | sed -n 's|^OWNER=||p')
      [ -n "$got" ] && [ "$want" = "$got" ] \
        || { echo "trial $trial: winner '$want' but the lease on disk says '$got'"; exit 1; }
    fi
  done
) && ok "force-release-vs-acquire" || fail "force-release-vs-acquire"
rm -rf "$T"

echo
if [ "$FAIL" -eq 0 ]; then
  printf '\033[0;32mPASS\033[0m %d scenario(s)\n' "$PASS"
else
  printf '\033[0;31mFAIL\033[0m %d of %d: %s\n' "$FAIL" "$((PASS + FAIL))" "${FAIL_NAMES[*]}"
  exit 1
fi
