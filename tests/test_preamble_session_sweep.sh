#!/usr/bin/env bash
# tests/test_preamble_session_sweep.sh — the preamble's session sweep must be
# portable: no `find` (Windows find.exe shadows it on PATH — audit
# bs-b3m7rnkw #9), stale files (>120 min) removed, fresh files counted.
set -u

REPO="$(cd "$(dirname "$0")/.." && pwd)"
PREAMBLE="$REPO/.claude/skills/references/preamble.md"
[ -f "$PREAMBLE" ] || { echo "FAIL: missing $PREAMBLE" >&2; exit 1; }

PASS=0; FAIL=0; FAIL_NAMES=()
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mok\033[0m  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); FAIL_NAMES+=("$1"); printf '  \033[0;31mFAIL\033[0m  %s\n' "$1"; [ $# -gt 1 ] && printf '        %s\n' "$2"; }

T="$(mktemp -d)"
trap 'rm -rf "$T"' EXIT

# Run the native bootstrap; it needs no find/stat/jq subprocesses.
printf '\"%s\" skill enter --name test\n' "$REPO/bbs" > "$T/preamble.sh"

SHELLS=(bash)
command -v zsh >/dev/null 2>&1 && SHELLS+=(zsh)

for SH in "${SHELLS[@]}"; do
  H="$T/home-$SH"; mkdir -p "$H/.babysit/sessions"
  # One stale file (fixed old mtime — always >120 min), one fresh.
  echo stale > "$H/.babysit/sessions/cc-old.yaml"
  touch -t 200001010000.00 "$H/.babysit/sessions/cc-old.yaml"
  echo fresh > "$H/.babysit/sessions/cc-new.yaml"

  out="$(env -i HOME="$H" PATH="$PATH" "$SH" -c ". '$T/preamble.sh'" 2>/dev/null)"

  if [ ! -f "$H/.babysit/sessions/cc-old.yaml" ]; then
    ok "sweep-removes-stale-$SH"
  else
    fail "sweep-removes-stale-$SH" "stale file survived"
  fi
  if [ -f "$H/.babysit/sessions/cc-new.yaml" ]; then
    ok "sweep-keeps-fresh-$SH"
  else
    fail "sweep-keeps-fresh-$SH" "fresh file removed"
  fi
  n="$(printf '%s\n' "$out" | sed -n 's/^SESSIONS_ACTIVE: //p')"
  if [ -n "$n" ] && [ "$n" -ge 1 ] 2>/dev/null; then
    ok "sessions-counted-$SH"
  else
    fail "sessions-counted-$SH" "SESSIONS_ACTIVE='$n' (output: $(printf '%s' "$out" | head -5 | tr '\n' '|'))"
  fi
done

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || { printf 'failed: %s\n' "${FAIL_NAMES[*]}"; exit 1; }
