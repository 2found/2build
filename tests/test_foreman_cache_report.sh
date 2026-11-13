#!/usr/bin/env bash
# tests/test_foreman_cache_report.sh — the Team cost report finds workers by
# their launch prompt wherever Orca put the worktree, groups them by the Task
# kind their spec names, and tells the lead's wakes apart (watcher, keep-warm,
# background notification, human). Fixtures live in a throwaway directory.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/scripts/foreman-cache-report.py"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
P="$TMP/projects"
N=0
tick() { N=$((N + 1)); T="$(printf '2026-11-07T15:%02d:00Z' "$N")"; }

fail=0
expect() { # $1=name $2=want $3=got
  if [ "$2" = "$3" ]; then echo "ok $1"; else echo "FAIL $1: want '$2', got '$3'"; fail=1; fi
}

req() { # $1=id $2=cache read tokens
  tick
  printf '{"type":"assistant","timestamp":"%s","requestId":"%s","message":{"id":"%s","usage":{"input_tokens":0,"cache_read_input_tokens":%s,"cache_creation_input_tokens":0,"output_tokens":0},"content":[]}}\n' "$T" "$1" "$1" "$2"
}
prompt() { # $1=text
  tick
  printf '{"type":"user","timestamp":"%s","message":{"content":"%s"}}\n' "$T" "$1"
}
spec() { # $1=kind
  tick
  printf '{"type":"user","timestamp":"%s","message":{"content":[{"type":"tool_result","tool_use_id":"x","content":"SPAWNED=true · AGENT_ROLE=orca · task t1 (kind %s) · ticket bs-1"}]}}\n' "$T" "$1"
}

# A ticket worker in an Orca workspace, a phase session launched in the
# canonical repo, and a lead session in the same repo directory.
mkdir -p "$P/-Users-x-orca-workspaces-app-bs-aaaa-p1" "$P/-Users-x-repo"
{ prompt "You are a bbs Team worker for Task t1, Dispatch t1.d1. Read your assignment"; spec ticket; req w1 10000000; } \
  > "$P/-Users-x-orca-workspaces-app-bs-aaaa-p1/s1.jsonl"
{ prompt "You are a bbs Team worker for Task t2, Dispatch t2.d1. Read your assignment"; spec phase; req w2 30000000; } \
  > "$P/-Users-x-repo/s2.jsonl"
{
  prompt "<command-message>bbs:team</command-message>"; req l1 1000000
  prompt "bbs team watch (team t): batch b settled"; req l2 2000000
  prompt "Prompt-cache keep-warm from bbs team watch: batch b is still running"; req l3 1000000
  prompt "<task-notification> <task-id>z</task-id>"; req l4 3000000
  prompt "xong chưa?"; req l5 4000000
} > "$P/-Users-x-repo/lead.jsonl"
# Not a Team session at all.
mkdir -p "$P/-Users-x-other"
{ prompt "fix the build"; req o1 5000000; } > "$P/-Users-x-other/o.jsonl"

OUT="$(python3 "$SCRIPT" --projects "$P" --since 2026-11-01 --json)"
get() { printf '%s' "$OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print($1)"; }

expect "workers found by launch prompt" 2 "$(get 'd["workers"]["sessions"]')"
expect "ticket kind" 1 "$(get 'd["workers"]["by_kind"]["ticket"]["sessions"]')"
expect "phase kind cost" 3000000 "$(get 'd["workers"]["by_kind"]["phase"]["cost"]')"
expect "one lead, workers excluded" 1 "$(get 'd["foreman"]["sessions"]')"
expect "watcher wakes (skill load + batch prompt)" 2 "$(get 'd["foreman"]["by_source"]["watcher"]["wakes"]')"
expect "keep-warms" 1 "$(get 'd["foreman"]["keep_warms"]')"
expect "notification cost" 300000 "$(get 'd["foreman"]["by_source"]["notification"]["cost"]')"
expect "human cost" 400000 "$(get 'd["foreman"]["by_source"]["human"]["cost"]')"

python3 "$SCRIPT" --projects "$P" --since 2026-11-01 | grep -q "kind phase" && echo "ok human report lists kinds" || { echo "FAIL human report lists kinds"; fail=1; }

exit $fail
