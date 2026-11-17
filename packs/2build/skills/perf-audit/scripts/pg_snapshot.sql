-- perf-audit: read-only Postgres snapshot (PostgreSQL 13+, psql 10+).
--   psql "$READ_ONLY_DB_URL" -X -q -P pager=off -f pg_snapshot.sql
-- Runs in one READ ONLY transaction with short timeouts and rolls back. Each
-- section sits behind a savepoint, so a missing extension or privilege skips
-- that section only. Prints numbers, object names and normalized statement
-- text from pg_stat_statements (constants are usually replaced by $n, but not
-- always: utility statements and statements mentioning password, secret or
-- token are filtered out, and the rest is still read before it is shared).
-- No session query text, users or client addresses.
\set ON_ERROR_STOP off
BEGIN READ ONLY;
SET LOCAL statement_timeout = '10s';
SET LOCAL lock_timeout = '1s';

\echo '== server'
SAVEPOINT s;
SELECT current_setting('server_version') AS version,
       date_trunc('second', now() - pg_postmaster_start_time()) AS uptime,
       pg_size_pretty(pg_database_size(current_database())) AS db_size,
       (SELECT stats_reset FROM pg_stat_database WHERE datname = current_database()) AS db_stats_reset,
       current_setting('max_connections') AS max_connections,
       current_setting('shared_buffers') AS shared_buffers;
ROLLBACK TO SAVEPOINT s;

\echo '== buffer cache hit (OLTP budget >= 99 %)'
SAVEPOINT s;
SELECT round(100.0 * blks_hit / nullif(blks_hit + blks_read, 0), 2) AS cache_hit_pct,
       xact_commit, xact_rollback, deadlocks, temp_files,
       pg_size_pretty(temp_bytes) AS temp_bytes
FROM pg_stat_database WHERE datname = current_database();
ROLLBACK TO SAVEPOINT s;

\echo '== top statements by total time (pg_stat_statements; window = since its last reset)'
SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements') AS has_pgss \gset
\if :has_pgss
SAVEPOINT s;
SELECT round((total_exec_time / 1000)::numeric, 1) AS total_s,
       calls,
       round((total_exec_time / nullif(calls, 0))::numeric, 2) AS mean_ms,
       round((rows::numeric / nullif(calls, 0)), 1) AS rows_per_call,
       round(100.0 * shared_blks_hit / nullif(shared_blks_hit + shared_blks_read, 0), 1) AS hit_pct,
       left(regexp_replace(query, '\s+', ' ', 'g'), 160) AS statement
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
  AND query ~* '^\s*(with|select|insert|update|delete|merge|copy)\y'
  AND query !~* '(password|passwd|secret|token|api[_-]?key)'
ORDER BY total_exec_time DESC
LIMIT 15;
ROLLBACK TO SAVEPOINT s;
\else
\echo 'pg_stat_statements is not installed: per-statement cost is unknown. Enabling it (shared_preload_libraries, or the managed database flag, then CREATE EXTENSION) is an owner-approved change, not an audit step.'
\endif

\echo '== largest tables: size, rows, dead rows, scans'
SAVEPOINT s;
SELECT relname AS table_name,
       pg_size_pretty(pg_total_relation_size(relid)) AS total_size,
       n_live_tup AS live_rows,
       round(100.0 * n_dead_tup / nullif(n_live_tup + n_dead_tup, 0), 1) AS dead_pct,
       seq_scan, idx_scan,
       CASE WHEN seq_scan > 0 THEN seq_tup_read / seq_scan END AS rows_per_seq_scan,
       greatest(last_autovacuum, last_vacuum)::date AS last_vacuum
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(relid) DESC
LIMIT 15;
ROLLBACK TO SAVEPOINT s;

\echo '== sequential-scan suspects (>= 10k rows, more seq than index scans)'
SAVEPOINT s;
SELECT relname AS table_name, n_live_tup AS live_rows, seq_scan, idx_scan,
       seq_tup_read / nullif(seq_scan, 0) AS rows_per_seq_scan
FROM pg_stat_user_tables
WHERE n_live_tup >= 10000 AND seq_scan > coalesce(idx_scan, 0)
ORDER BY seq_tup_read DESC
LIMIT 15;
ROLLBACK TO SAVEPOINT s;

\echo '== unused-index candidates (0 scans since stats reset, > 1 MB, not unique)'
\echo '   Not proof: counts are per server (a replica may use it), may cover less than a full business cycle (monthly jobs), and an index backing a foreign key may matter for deletes.'
SAVEPOINT s;
SELECT s.relname AS table_name, s.indexrelname AS index_name,
       pg_size_pretty(pg_relation_size(s.indexrelid)) AS index_size
FROM pg_stat_user_indexes s
JOIN pg_index i ON i.indexrelid = s.indexrelid
WHERE s.idx_scan = 0 AND NOT i.indisunique
  AND pg_relation_size(s.indexrelid) > 1024 * 1024
ORDER BY pg_relation_size(s.indexrelid) DESC
LIMIT 15;
ROLLBACK TO SAVEPOINT s;

\echo '== connections by state and wait (no query text)'
SAVEPOINT s;
SELECT coalesce(state, backend_type) AS state,
       coalesce(wait_event_type, '-') AS wait_type,
       count(*) AS sessions,
       max(date_trunc('second', now() - xact_start)) AS oldest_xact,
       count(*) FILTER (WHERE state = 'active' AND now() - query_start > interval '5 seconds') AS active_over_5s
FROM pg_stat_activity
WHERE datname = current_database()
GROUP BY 1, 2
ORDER BY sessions DESC;
ROLLBACK TO SAVEPOINT s;

\echo '== lock waits'
SAVEPOINT s;
SELECT locktype, mode, count(*) AS waiting
FROM pg_locks WHERE NOT granted
GROUP BY 1, 2 ORDER BY waiting DESC;
ROLLBACK TO SAVEPOINT s;

ROLLBACK;
