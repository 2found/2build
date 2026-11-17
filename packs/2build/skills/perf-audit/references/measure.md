# Measuring

## Evidence levels
Every number carries its level; levels never mix in one before/after.

| level | what | proves |
|---|---|---|
| `live` | production signals: real users, data and traffic | what users feel; what is hot |
| `lab` | local or reserved environment at production shape | cause and fix effect, not user experience |
| `static` | code reading, plans without data | where to look; never a status or a gain |

## Access and safety
**Finding access**, first hit wins: the [profile](references/profile.md)'s Signals table
→ project skills and runbooks for logs, hosts and databases → `AGENTS.md`
deploy section → deploy scripts and compose files (they name hosts, services
and log drivers). Record what you found in the profile. Resolve container
names at run time (`docker ps --filter label=…` or `--filter name=…`):
blue/green deploys flip them.

**Allowed on production** only through a route a repo doc or the owner names
as read-only. A credential that can write (an app database URL, an admin
role, an SSH user that can restart services) is not a read-only route: stop
and raise the access `NEEDS_CONTEXT` instead of "being careful" with it.
- Logs: bounded pulls only (`--since`, `--tail`); one batched script per host,
  since every SSH/IAP round trip costs seconds. Logs vanish when a container
  is recreated, so note the retention you actually saw.
- Database: `scripts/pg_snapshot.sql`; plain `EXPLAIN` inside
  `BEGIN READ ONLY; … ROLLBACK;` with a `statement_timeout`. No
  `EXPLAIN ANALYZE` on the primary: it executes the statement.
- HTTP: a few sequential `curl` or Lighthouse runs against public pages.
- Times in UTC; each window recorded with its traffic level.

**Never on production**, not even "to measure": load tests, restarts,
redeploys, config or flag changes, cache purges, kills, writes, enabling
extensions.

**Lab**: a local or audit-reserved environment. `EXPLAIN (ANALYZE, BUFFERS)`
runs on a sanitized snapshot or on a seed shaped from production statistics
(row counts from `pg_stat_user_tables`, distributions from `pg_stats`),
never on a raw copy of production data. Load tools are pinned and never
point at a production origin: the repo's own tool, else
`npx -y autocannon@8.0.0`, `oha` or `k6`.

**Never in the transcript or report**: raw log lines, user ids, emails, IPs,
query strings, headers, tokens, connection strings. Aggregate by route
template (counts, percentiles, status classes, shares).

## Sources
| layer | live source | lab source |
|---|---|---|
| Page, real users | RUM if installed; CrUX field data (PageSpeed Insights API, p75 over 28 days) for a public origin with traffic | Lighthouse mobile preset, median of ≥ 5 runs |
| Edge / CDN | CDN analytics by path and cache status; `curl -sI` for `cache-control`, `age` and the CDN's cache-status header | repeated `curl -w '%{time_starttransfer}\n'` |
| Origin render (pages) | web server request log by route (time to render the document) | local production build |
| API | request log by route template (`scripts/log_paths.py`); traces when OpenTelemetry or an APM is on | load tool above; runtime CPU profiler |
| Database | `scripts/pg_snapshot.sql`; MySQL: `performance_schema` statement digests | `EXPLAIN (ANALYZE, BUFFERS)` per § Access and safety |
| Host | the project's host runbook (CPU pressure, steal, memory peak, disk, CPU per container) | — |
| Jobs / queues | queue depth, oldest-job age, duration by type | the job on production-sized input |

A page needs both an edge signal (how many requests the CDN answered) and an
origin-render signal (how long the misses took); one without the other cannot
say whether to fix the render or the cache. The identified share from
`log_paths.py` (requests carrying a user id) says how much traffic a shared
cache can serve at all.

**Vantage point.** Timings taken from the agent's machine include its network
distance. Record where they were taken; when the best observed time from
there already exceeds 70 % of the budget, compare against the server-side
time instead and say so.

### Scripts
`scripts/log_paths.py` turns JSON request logs into a per-template table
(percentiles over non-failed requests, error shares, identified share, total
time) plus traffic level, burst seconds and retry signals; it prints
templates and numbers only. For a feature, pass the routes from Map:
```sh
<bounded log command> | python3 <skill-dir>/scripts/log_paths.py --routes <out>/routes.txt --only
<bounded log command> | python3 <skill-dir>/scripts/log_paths.py --json > <out>/paths.json   # system scope
python3 <skill-dir>/scripts/log_paths.py --help   # line shapes, fields, units
```
Static routes sharing a shape with a parameter route (`/novel/search` beside
`/novel/<slug>`) need `--routes`. Logs without a route template make every
audit guess: logging the framework's matched route (Hono `c.req.routePath`,
Express `req.route.path`) beside the path is a rung-1 fix worth proposing.

`scripts/pg_snapshot.sql` runs in one `READ ONLY` transaction with timeouts
and rolls back:
```sh
psql "$<READ_ONLY_DB_URL_VAR>" -X -q -P pager=off -f <skill-dir>/scripts/pg_snapshot.sql
```
Statement text from `pg_stat_statements` is normalized but can still hold a
literal: read it before quoting it.

## Where time goes
Request wall time hides its cause: a slow query, a wait for a pool
connection and a blocked event loop all show as the same `duration`.

| segment | looks like | confirm with |
|---|---|---|
| Client | server time fine, page still late: bundle, images, request waterfalls, render work | Lighthouse trace, network waterfall, bytes per page |
| Edge | viewer-identical `GET` reaching origin (miss/dynamic/bypass); far users with high TTFB | response headers, CDN analytics by cache status |
| Origin queue | many unrelated routes slow in the **same second**, health checks included: blocked event loop or CPU starvation. DB-backed routes slow while the database is idle and the pool is full: pool wait | burst seconds, loop-lag or pool metrics, CPU pressure, lab at peak concurrency |
| App CPU | CPU seconds per request (container CPU s/s ÷ requests/s) in tens of ms on a simple route | CPU profile of a lab run |
| Database | statement time × statements per request; N+1 (count grows with result size); sequential scans on big tables; lock waits | statement stats, query count per request in lab, plans at production row counts (a plan on a tiny dev database proves nothing) |
| External | third-party calls and synchronous side effects (email, render, upload) on the request path | spans, timing around the call |
| Payload | large responses, over-fetching, missing compression | response size, `content-encoding` |

Waste is removable without changing what users get: repeat work, retry
loops, a missing index, a cache that silently switched off, probes and
polling, duplicates. Real work grows with traffic, and only real work
justifies capacity.

## Budgets
Used when the profile sets none. Web vitals are Google's "good" thresholds at
p75 of real mobile users; the rest are 2build defaults.

| class | metric | budget |
|---|---|---|
| Page (field) | LCP · INP · CLS · TTFB | ≤ 2.5 s · ≤ 200 ms · ≤ 0.1 · ≤ 0.8 s |
| Interactive API read | server p95 | ≤ 300 ms |
| Interactive API write | server p95 | ≤ 1 s |
| Anything a user waits for | wall time | > 1 s shows progress; > 10 s becomes async with status |
| Errors | 5xx and timeouts per path | ≤ 0.1 %; 0 timeouts on critical paths |
| Host | CPU pressure `some avg60` at peak · memory peak · disk | ≤ 20 % · ≤ 80 % · ≤ 80 % |
| Process | pool waits · event-loop lag p99 | ≈ 0 · ≤ 100 ms |
| Database | buffer cache hit (OLTP) · connections used | ≥ 99 % · ≤ 80 % of max |
| Job / queue | duration · oldest job age | within the job's SLA · not growing across windows |

## Noise
- Live: at least two windows at a representative time, each with its
  requests per full hour; compare windows of similar traffic only.
- Lab: warm up, then ≥ 5 runs, same machine, data, concurrency and cache state
  (cold or warm, stated) before and after; report median and min–max.
- A change counts when the after median lies outside the before min–max and
  differs by at least 10 %.
- Under ~100 requests report max instead of p99; under ~20 the values.
