#!/usr/bin/env bash
# The Go preamble needs one bbs binary, not shell code or companion aliases.

set -u
REPO="$(cd "$(dirname "$0")/.." && pwd)"
PREAMBLE="$REPO/.claude/skills/references/preamble.md"
[ -f "$PREAMBLE" ] || { echo "FAIL: missing $PREAMBLE" >&2; exit 1; }
command -v go >/dev/null 2>&1 || { echo "SKIP: go not installed" >&2; exit 0; }

PASS=0; FAIL=0; FAIL_NAMES=()
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mok\033[0m  %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); FAIL_NAMES+=("$1"); printf '  \033[0;31mFAIL\033[0m  %s\n' "$1"; [ $# -gt 1 ] && printf '        %s\n' "$2"; }

T="$(mktemp -d)"
trap 'rm -rf "$T"' EXIT

BIN="$T/bbs"
(cd "$REPO" && go build -o "$BIN" ./cmd/bbs) || { echo "FAIL: go build" >&2; exit 1; }

# A directly invoked binary must reach its own subcommands even off PATH.
SHELLS=(bash)
command -v zsh >/dev/null 2>&1 && SHELLS+=(zsh)
BARE_PATH="/usr/bin:/bin:/usr/sbin:/sbin"
for SH in "${SHELLS[@]}"; do
  H="$T/home-$SH"; mkdir -p "$H"
  out="$(env -i HOME="$H" PATH="$BARE_PATH" "$SH" -c '"$1" skill enter --name test' _ "$BIN" 2>&1)"
  case "$out" in
    *"SKILL: test"*"SLUG: "*"AUTOPILOT_CONTRACT: v2"*) ok "[$SH] bootstrap off PATH" ;;
    *) fail "[$SH] bootstrap off PATH" "$out" ;;
  esac
done

# Native execution also works with no shell or Unix utilities available.
out="$(env -i HOME="$T/home-native" PATH="$T/empty" "$BIN" skill enter --name test 2>&1)"
case "$out" in
  *"SKILL: test"*"SLUG: unknown"*) ok "native bootstrap without shell utilities" ;;
  *) fail "native bootstrap without shell utilities" "$out" ;;
esac

# 6. Nothing readers act on may call a hyphenated alias brew does not ship.
#    Three kinds of hyphenated mention stay legitimate, so they're filtered
#    rather than fixed:
#      - lines *about* the aliases (argv0 / alias / symlink), e.g. docs/install.md
#      - filesystem paths (bin/bbs-ticket, tests/fixtures/bbs-*.reference)
#      - bbs-ticket-test / bbs-ticket-lint, which are separate scripts, not symlinks
#    Excluded outright: CHANGELOG.md and blogs/ (dated records — rewriting them
#    would falsify history).
#    The tail char after the alias must be invocation-shaped (letter, digit, -,
#    / or end-of-line): that still matches `bbs-ticket init` and bare-name calls,
#    while backticked prose mentions like (`bbs-config`, `bbs-env`) in
#    docs/companion-cli.md no longer trip the scan.
ALIAS_RE='(^|[^/[:alnum:]_-])bbs-(ticket|design|secrets|qa-config|autopilot|config|slug|env|update|upgrade|update-check|dashboard)([^[:alnum:]_`-]|$)'
STRAY="$(grep -rnoE "$ALIAS_RE" \
    "$REPO/.claude/skills" "$REPO/docs" "$REPO"/README*.md "$REPO/CLAUDE.md" \
    --include='*.md' 2>/dev/null \
  | grep -vE 'bbs-ticket-(test|lint)' || true)"
# Re-check each hit against its full source line — the -o match alone can't see
# whether the line is discussing the alias or invoking it.
STRAY="$(echo "$STRAY" | while IFS=: read -r file line _; do
  [ -n "${file:-}" ] || continue
  src="$(sed -n "${line}p" "$file")"
  case "$src" in
    *argv*|*alias*|*symlink*|*bin/*) ;;
    *) echo "$file:$line: $src" ;;
  esac
done)"
if [ -z "$STRAY" ]; then
  ok "no hyphenated alias calls left in skills or docs"
else
  fail "no hyphenated alias calls left in skills or docs" "$(echo "$STRAY" | head -3)"
fi

echo ""
if [ "$FAIL" -eq 0 ]; then
  printf '\033[0;32m%d passed\033[0m\n' "$PASS"
  exit 0
fi
printf '\033[0;31m%d failed\033[0m, %d passed: %s\n' "$FAIL" "$PASS" "${FAIL_NAMES[*]}"
exit 1
