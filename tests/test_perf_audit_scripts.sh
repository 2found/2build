#!/usr/bin/env bash
# tests/test_perf_audit_scripts.sh — perf-audit's log aggregator turns request
# logs into route templates without leaking raw ids, reads the log shapes the
# skill points it at (Docker `--timestamps` prefix, json-file records, plain
# JSON with a time field), and reports the two production signatures the skill
# relies on: burst seconds (many unrelated routes slow at once = blocked event
# loop) and a 429 retry loop. The SQL snapshot must stay read-only.
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/.claude/skills/perf-audit/scripts/log_paths.py"
SQL="$ROOT/.claude/skills/perf-audit/scripts/pg_snapshot.sql"
command -v python3 >/dev/null 2>&1 || { echo "SKIP: python3 not installed" >&2; exit 0; }

TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
fail=0
check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }

python3 - "$TMP/app.log" <<'PY'
import json, sys
out = open(sys.argv[1], "w")
def line(ts, path, ms, status=200, method="GET", style="prefix", user="user-secret-42"):
    rec = {"severity": "INFO", "message": "request", "method": method,
           "path": path, "status": status, "durationMs": ms}
    if user:
        rec["userId"] = user
    if style == "prefix":
        out.write(f"{ts} {json.dumps(rec)}\n")
    elif style == "jsonfile":
        out.write(json.dumps({"log": json.dumps(rec) + "\n", "stream": "stdout", "time": ts}) + "\n")
    else:
        rec["time"] = ts
        out.write(json.dumps(rec) + "\n")
# 30 distinct word slugs (no digits) → cardinality collapses them to :param; chapter numbers → :n
for i in range(30):
    for c in range(4):
        slug = "slug-number-" + chr(97 + i % 26) + chr(97 + i // 26)
        line(f"2026-11-09T01:{i:02d}:0{c}.123456789Z", f"/novel/{slug}/chapter/{c+1}?ref=home", 40 + i, style=["prefix", "jsonfile", "plain"][c % 3])
# a slow but rare route
for k in range(3):
    line(f"2026-11-09T02:00:1{k}Z", "/novel/search", 2500)
# a burst: 6 unrelated routes, incl. healthz, slow in the same second
for p in ["/healthz", "/a", "/b", "/c", "/d", "/e"]:
    line("2026-11-09T03:00:00Z", p, 3000)
# a 429 retry loop on a probe
for k in range(40):
    line(f"2026-11-09T04:00:{k:02d}Z", "/tts/probe/0f3a9c2be4d14e5f8a7b6c5d4e3f2a1b", 5, status=429)
# failures must not drag the percentiles: 10 fast 200s, 10 slow 500s, anonymous
for k in range(10):
    line(f"2026-11-09T02:30:{k:02d}Z", "/mixed", 10, user=None)
    line(f"2026-11-09T02:31:{k:02d}Z", "/mixed", 5000, status=500, user=None)
# personal data and secrets in paths
line("2026-11-09T02:40:00Z", "/users/reader%40mail.example/profile", 20)
line("2026-11-09T02:40:01Z", "/users/someone@mail.example/profile", 20)
line("2026-11-09T02:40:02Z", "/reset/AbCdEfGhIjKlMnOpQrStUvWxYz", 20)
line("2026-11-09T02:40:03Z", "/auth/eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl", 20)
# noise that must be skipped
out.write("not json at all\n")
out.write(json.dumps({"message": "cron tick"}) + "\n")
PY

OUT="$(python3 "$SCRIPT" --where message=request --slow-ms 1000 < "$TMP/app.log")"
JSON="$(python3 "$SCRIPT" --where message=request --json < "$TMP/app.log")"
row() { python3 -c "import json,sys; d=json.load(sys.stdin); r=[p for p in d[\"paths\"] if p[\"template\"]==sys.argv[1]][0]; sys.exit(not ($2))" "$1" <<<"${3:-$JSON}"; }
summ() { python3 -c "import json,sys; s=json.load(sys.stdin)[\"summary\"]; sys.exit(not ($1))" <<<"${2:-$JSON}"; }
printf '%s\n' '/novel/search' '/novel/:slug/chapter/:n  # chapter reader' > "$TMP/routes.txt"
FEAT="$(python3 "$SCRIPT" --where message=request --routes "$TMP/routes.txt" --only --json < "$TMP/app.log")"

check "slug collapses to :param, chapter to :n" 'grep -q "GET /novel/:param/chapter/:n" <<<"$OUT"'
check "query string dropped" '! grep -q "ref=home" <<<"$OUT"'
check "no raw slug, user id, hash, email or token leaks" '! grep -Eq "slug-number|user-secret|0f3a9c2b|mail\.example|reader|someone|AbCdEf|eyJhbGci" <<<"$OUT$JSON"'
check "emails collapse to :email, long tokens to :token" 'grep -q "/users/:email/profile" <<<"$OUT" && grep -q "/reset/:token" <<<"$OUT" && grep -q "/auth/:token" <<<"$OUT"'
check "hex probe id collapses to :hash" 'grep -q "/tts/probe/:hash" <<<"$OUT"'
check "all three line shapes parsed (120 chapter reads)" 'python3 -c "import json,sys; d=json.load(sys.stdin); r=[p for p in d[\"paths\"] if p[\"template\"]==\"GET /novel/:param/chapter/:n\"][0]; sys.exit(r[\"count\"]!=120)" <<<"$JSON"'
check "noise lines skipped by reason, not fatal" 'summ "s[\"skipped_lines\"]=={\"not_json\":1,\"filtered\":1}"'
check "timestamps read from Docker prefix and json-file" 'python3 -c "import json,sys; d=json.load(sys.stdin); sys.exit(d[\"summary\"][\"window_utc\"]!=[\"2026-11-09T01:00:00Z\",\"2026-11-09T04:00:39Z\"])" <<<"$JSON"'
check "burst second found, health included" 'python3 -c "import json,sys; d=json.load(sys.stdin); s=d[\"summary\"]; sys.exit(not (s[\"burst_seconds\"]==1 and s[\"top_bursts\"][0][\"includes_health\"]))" <<<"$JSON"'
check "429 retry loop flagged" 'grep -q "429 ≥ 5 %.*/tts/probe/:hash" <<<"$OUT"'
check "p99 withheld under 100 samples" 'python3 -c "import json,sys; d=json.load(sys.stdin); r=[p for p in d[\"paths\"] if p[\"template\"]==\"GET /novel/search\"][0]; sys.exit(r[\"p99_ms\"] is not None)" <<<"$JSON"'
check "sorted by total time" 'python3 -c "import json,sys; d=json.load(sys.stdin); t=[p[\"total_s\"] for p in d[\"paths\"]]; sys.exit(t!=sorted(t,reverse=True))" <<<"$JSON"'
check "percentiles over non-failed requests, errors counted apart" 'row "GET /mixed" "r[\"p95_ms\"]==10 and r[\"max_ms\"]==5000 and r[\"5xx_pct\"]==50 and r[\"ok_count\"]==10"'
check "identified share counts user ids without printing them" 'row "GET /mixed" "r[\"identified_pct\"]==0" && row "GET /novel/search" "r[\"identified_pct\"]==100"'
check "requests per hour drops partial edge hours" 'summ "s[\"requests_per_hour\"][\"full_hours\"]==2 and s[\"requests_per_hour\"][\"min\"]==6 and s[\"requests_per_hour\"][\"max\"]==27"'
check "--routes --only keeps the feature, totals the rest" 'python3 -c "import json,sys; d=json.load(sys.stdin); t=sorted(p[\"template\"] for p in d[\"paths\"]); o=d[\"summary\"][\"outside_feature\"]; sys.exit(not (t==[\"GET /novel/:slug/chapter/:n\",\"GET /novel/search\"] and o[\"count\"]==70))" <<<"$FEAT"'
printf '%s\n' '/novel/:slug' '/novel/search' > "$TMP/routes-param-first.txt"
check "--routes: a static route beats a param route listed before it" 'python3 "$SCRIPT" --where message=request --routes "$TMP/routes-param-first.txt" --json < "$TMP/app.log" | python3 -c "import json,sys; t=[p[\"template\"] for p in json.load(sys.stdin)[\"paths\"]]; sys.exit(not (\"GET /novel/search\" in t and \"GET /novel/:slug\" not in t))"'
check "--only without --routes is a usage error" '! python3 "$SCRIPT" --only </dev/null 2>/dev/null'
printf '%s\n' '{"path":"/go","duration":"1m2.5s"}' '{"path":"/go","duration":"750µs"}' '{"path":"/go","duration":"1.5e2"}' '{"path":"/go","duration":"12ms"}' > "$TMP/go.log"
check "Go, µs and exponent durations parse" 'row "/go" "r[\"count\"]==4 and r[\"max_ms\"]==62500 and r[\"total_s\"]==62.7" "$(python3 "$SCRIPT" --json "$TMP/go.log")"'
check "implausible unit warns" 'python3 "$SCRIPT" --duration-unit ns < "$TMP/app.log" | grep -q "WARNING: median duration"'
check "--rule rewrites before templating" 'python3 "$SCRIPT" --rule "^/novel/search=/novel/find" < "$TMP/app.log" | grep -q "/novel/find"'
check "empty input exits non-zero with a hint" '! python3 "$SCRIPT" < /dev/null >"$TMP/empty" && grep -q "path-field" "$TMP/empty"'

check "SQL snapshot opens READ ONLY and rolls back" 'grep -q "^BEGIN READ ONLY;" "$SQL" && tail -n 1 "$SQL" | grep -q "^ROLLBACK;"'
check "SQL snapshot detects pg_stat_statements via pg_extension, filters secrets" 'grep -q "FROM pg_extension WHERE extname = .pg_stat_statements." "$SQL" && grep -q "password|passwd|secret|token" "$SQL"'
check "SQL regexes use the Postgres word boundary \\y, not \\b (backspace)" 'python3 -c "import sys; sys.exit(chr(92)+\"b\" in open(sys.argv[1]).read())" "$SQL"'
check "SQL snapshot never reads session query text" '! grep -Eq "pg_stat_activity[^;]*\\bquery\\b[^_]" "$SQL" && ! grep -Eiq "^\\s*(insert|update|delete|create|drop|alter|truncate|vacuum)\\b" "$SQL"'

exit "$fail"
