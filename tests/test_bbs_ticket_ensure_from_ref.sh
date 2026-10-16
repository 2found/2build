#!/usr/bin/env bash
# tests/test_bbs_ticket_ensure_from_ref.sh — where `ensure --mode=worktree`
# cuts a ticket from. A Foreman phase lands on local base before its release
# pushes, so the next phase's tickets start from local base by default;
# `--from-ref` names another commit.
#
# Scenarios:
#   default-cuts-local-base    the locally landed commit is in the worktree
#   from-ref-cuts-named-commit --from-ref main~1 leaves it out
#   bad-ref-refused            a ref that is not a commit exits 2, cuts nothing
#   manifest-canonical-absolute the manifest names the checkout it was cut from,
#                              absolutely, so `bbs foreman seal` can resolve it

set -u
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BBS="$SCRIPT_DIR/bbs"

PASS=0; FAIL=0; FAIL_NAMES=()
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mok\033[0m  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); FAIL_NAMES+=("$1"); printf '  \033[0;31mFAIL\033[0m  %s\n' "$1"; [ $# -gt 1 ] && printf '        %s\n' "$2"; }

T="$(mktemp -d)"
trap 'rm -rf "$T"' EXIT
export HOME="$T/home" BABYSIT_HOME="$T/home/.babysit" AGENT_ROLE=mayor
mkdir -p "$HOME"
git init -q --bare "$T/remote.git"
git init -q "$T/repo"
cd "$T/repo"
git -c user.email=t@t -c user.name=t commit --allow-empty -q -m init
git branch -M main
git remote add origin "$T/remote.git"
git push -q origin main
echo phase1 > landed.txt
git add landed.txt
git -c user.email=t@t -c user.name=t commit -q -m "phase 1 landed locally"

worktree_of() { printf '%s\n' "$1" | sed -n 's|^WORKTREE=||p'; }

out="$("$BBS" ticket ensure --mode=worktree --slug-hint plain --type feat 2>/dev/null)"
wt="$(worktree_of "$out")"
if [ -n "$wt" ] && [ -e "$wt/landed.txt" ]; then ok default-cuts-local-base
else fail default-cuts-local-base "worktree=$wt out=$out"; fi

manifest="$(sed -n 's|^TICKET_HOME=||p' <<<"$out")/manifest.yaml"
canonical="$(sed -n 's|^ *canonical: ||p' "$manifest")"
if [ "$canonical" = "$(git rev-parse --show-toplevel)" ]; then ok manifest-canonical-absolute
else fail manifest-canonical-absolute "canonical=$canonical manifest=$manifest"; fi

out="$("$BBS" ticket ensure --mode=worktree --slug-hint older --type feat --from-ref main~1 2>/dev/null)"
wt="$(worktree_of "$out")"
if [ -n "$wt" ] && [ ! -e "$wt/landed.txt" ]; then ok from-ref-cuts-named-commit
else fail from-ref-cuts-named-commit "worktree=$wt out=$out"; fi

before="$(git worktree list | wc -l) $(find "$BABYSIT_HOME" | wc -l)"
"$BBS" ticket ensure --mode=worktree --slug-hint bad --type feat --from-ref no-such-ref >/dev/null 2>&1
rc=$?
after="$(git worktree list | wc -l) $(find "$BABYSIT_HOME" | wc -l)"
if [ "$rc" = 2 ] && [ "$before" = "$after" ]; then ok bad-ref-refused
else fail bad-ref-refused "rc=$rc worktrees/state $before -> $after"; fi

echo
echo "  $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || { printf '  failed: %s\n' "${FAIL_NAMES[*]}"; exit 1; }
