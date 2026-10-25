#!/usr/bin/env bash
# tests/test_token_audit.sh — token-audit's script attributes weighted tokens
# from Claude Code, Codex and OMP session logs, and inventories background
# context (listed skills, manual-only/override state, stale roots, memory size)
# without printing message text. Fixtures live in a throwaway HOME.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/.claude/skills/token-audit/scripts/token_audit.py"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
export HOME="$TMP/home"
TODAY="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
SECRET="do-not-print-this-message-text"

fail=0
expect() { # $1=name $2=want $3=got
  if [ "$2" = "$3" ]; then echo "ok $1"; else echo "FAIL $1: want '$2', got '$3'"; fail=1; fi
}

# --- Claude Code: one main turn per skill phase, one subagent, one errored call
P="$HOME/.claude/projects/-repo"
mkdir -p "$P/s1/subagents"
cat > "$P/s1.jsonl" <<EOF
{"type":"user","timestamp":"$TODAY","message":{"content":"$SECRET"}}
{"type":"assistant","timestamp":"$TODAY","message":{"id":"m1","model":"opus","usage":{"input_tokens":100,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"output_tokens":0},"content":[{"type":"tool_use","id":"t1","name":"Skill","input":{"skill":"bbs:review-pr"}}]}}
{"type":"assistant","timestamp":"$TODAY","message":{"id":"m2","model":"opus","usage":{"input_tokens":0,"cache_read_input_tokens":1000,"cache_creation_input_tokens":0,"output_tokens":20},"content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"bbs ticket get --help"}}]}}
{"type":"user","timestamp":"$TODAY","message":{"content":[{"type":"tool_result","tool_use_id":"t2","is_error":true,"content":"$SECRET"}]}}
EOF
cat > "$P/s1/subagents/agent-a.jsonl" <<EOF
{"type":"assistant","timestamp":"$TODAY","message":{"id":"x1","model":"haiku","usage":{"input_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":100,"cache_creation":{"ephemeral_1h_input_tokens":100},"output_tokens":0},"content":[]}}
EOF
echo '{"description":"Review angle A line scan","agentType":"Explore"}' > "$P/s1/subagents/agent-a.meta.json"

# --- Codex: cached input is a cache read, the rest plain input
C="$HOME/.codex/sessions/2026/10/25"; mkdir -p "$C"
cat > "$C/rollout-1.jsonl" <<EOF
{"timestamp":"$TODAY","type":"session_meta","payload":{"cwd":"/repo"}}
{"timestamp":"$TODAY","type":"turn_context","payload":{"model":"gpt-x"}}
{"timestamp":"$TODAY","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"\$bbs:autopilot $SECRET"}]}}
{"timestamp":"$TODAY","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":300,"cached_input_tokens":200,"output_tokens":10}}}}
EOF

# --- OMP: skill prompt sets the phase, cacheWrite counts as a 5m write
O="$HOME/.omp/agent/sessions/-repo"; mkdir -p "$O"
cat > "$O/s.jsonl" <<EOF
{"type":"custom_message","timestamp":"$TODAY","customType":"skill-prompt","content":"[IMPORTANT: User invoked the \"qa\" skill; follow its instructions.] $SECRET"}
{"type":"message","timestamp":"$TODAY","message":{"role":"assistant","provider":"p","model":"m","content":[],"usage":{"input":40,"output":0,"cacheRead":0,"cacheWrite":80,"totalTokens":120}}}
EOF

U="$(python3 "$SCRIPT" usage --days 2 --json)" || { echo "FAIL usage exit"; exit 1; }
# claude main 100 + 0.1*1000 + 5*20 = 300; sub 2.0*100 = 200; codex 100 + 0.1*200 + 5*10 = 170; omp 40 + 1.25*80 = 140
expect "weighted total across harnesses" 810 "$(jq .weighted_total <<<"$U")"
expect "sessions from three harnesses" 3 "$(jq .sessions <<<"$U")"
expect "subagent share" 0.247 "$(jq .sub_share <<<"$U")"
expect "1h cache writes attributed to subagents" 0.247 "$(jq '.components["sub:write_1h"]' <<<"$U")"
expect "phase from Claude Skill tool" 0.37 "$(jq -r '.by_phase[] | select(.[0]=="review-pr") | .[1]' <<<"$U")"
expect "phase from Codex \$skill mention" 0.21 "$(jq -r '.by_phase[] | select(.[0]=="autopilot") | .[1]' <<<"$U")"
expect "phase from OMP skill prompt" 0.173 "$(jq -r '.by_phase[] | select(.[0]=="qa") | .[1]' <<<"$U")"
expect "subagent label" "Review angle A line scan" "$(jq -r '.subagents[0][0]' <<<"$U")"
expect "--help lookup counted" 1 "$(jq .friction.help_lookups <<<"$U")"
expect "errored call keyed by command" '["bash:bbs ticket get",1]' "$(jq -c '.friction.errored_calls[0]' <<<"$U")"
expect "no message text in usage output" 0 "$(grep -c "$SECRET" <<<"$U")"

# --- context: listing, manual-only, override, stale root, memory limits
REPO="$TMP/repo"; mkdir -p "$REPO/.claude/skills/ops" "$REPO/.claude/skills/dev" "$HOME/.claude/skills/stale-copy"
printf -- '---\nname: ops\ndescription: Operate things.\ndisable-model-invocation: true\n---\n' > "$REPO/.claude/skills/ops/SKILL.md"
printf -- '---\nname: dev\ndescription: >-\n  Develop\n  things.\n---\n' > "$REPO/.claude/skills/dev/SKILL.md"
mkdir -p "$HOME/.claude/skills/hidden"
printf -- '---\nname: hidden\ndescription: Hidden by override.\n---\n' > "$HOME/.claude/skills/hidden/SKILL.md"
echo '{"skillOverrides":{"hidden":"off"}}' > "$HOME/.claude/settings.json"
MEM="$HOME/.claude/projects/$(sed 's/[^A-Za-z0-9]/-/g' <<<"$REPO")/memory"; mkdir -p "$MEM"
for i in $(seq 1 190); do echo "- [m$i](m$i.md) — hook"; done > "$MEM/MEMORY.md"

X="$(python3 "$SCRIPT" context --repo "$REPO" --days 2 --json)" || { echo "FAIL context exit"; exit 1; }
expect "manual-only skill is not listed" 0 "$(jq '.skills[] | select(.name=="ops") | .listed_chars' <<<"$X")"
expect "folded description parsed" 22 "$(jq '.skills[] | select(.name=="dev") | .listed_chars' <<<"$X")"
expect "override off is not listed" off "$(jq -r '.skills[] | select(.name=="hidden") | .override' <<<"$X")"
expect "listing total" 22 "$(jq .claude_listing_chars <<<"$X")"
expect "stale skill-root dir reported" "$HOME/.claude/skills/stale-copy" "$(jq -r '.stale_dirs[0][1]' <<<"$X")"
expect "memory index measured" 191 "$(jq '.instruction_files[] | select(.file|startswith("auto-memory")) | .lines' <<<"$X")"
python3 "$SCRIPT" context --repo "$REPO" --days 2 | grep -q 'NEAR LIMIT' \
  && echo "ok memory near-limit warning" || { echo "FAIL memory near-limit warning"; fail=1; }

exit "$fail"
