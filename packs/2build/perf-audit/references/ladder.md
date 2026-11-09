# The optimization ladder
Lower rungs are cheaper, smaller in blast radius and easier to undo. Fix each
proven cause on the **lowest rung that removes it**; climb only with evidence
that the rung below cannot reach the budget (the query already uses the right
index and returns only what it needs, yet runs too often → cache).

| rung | removes | typical fixes | blast radius · undo |
|---|---|---|---|
| 1 Code | avoidable work per request | N+1 → one query; independent awaits in parallel; over-fetching trimmed; sync CPU work off the request path; client waterfalls, polling and retry loops; bundle, image and payload size | one module · revert |
| 2 Query & index | database work per statement | sargable predicates, `LIMIT`, keyset pagination, only needed columns; add, fix or drop an index; refresh statistics | one table · drop the index |
| 3 Data | work caused by data volume | purge or archive by a retention rule; fix bloat; move large blobs out of hot tables; maintained counters instead of `COUNT(*)` | data · **irreversible** without a backup |
| 4 Flow | work the feature asks for | side effects to a background job; batch; precompute; paginate or lazy-load; UX that needs fewer calls | product behaviour · needs a plan |
| 5 Cache on what exists | repeated identical work | cache headers on the CDN already in front; keys in the cache service already running; in-process memo for immutable data | staleness, a new failure mode · turn off |
| 6–7 Infra, architecture | real work beyond one resource; limits of the shape | the [architect roadmap](../architect/references/roadmap.md)'s stages, in its order (edge, right-size, database, shared cache, jobs, instances, replicas, partitions, services) | money and operations · `architect` designs |

Never: cache what an index fixes, add capacity for unattributed cost, or split
a service for performance alone. The project's own rules (`AGENTS.md`,
caching and migration docs) override anything here.

## Safety per rung
Each fix theme carries its rung's notes into its autopilot requirement.

1. **Code**: behaviour and API contracts stay identical. Guard: a test that
   pins the path's query or call count, or a benchmark with its budget.
2. **Query & index**: show the planner uses the index at production volume.
   On a big table (the project's threshold; default > 1 M rows or > 1 GB):
   `CREATE INDEX CONCURRENTLY` outside a transaction (check the migration tool
   allows it), the migration opens with a `lock_timeout`, and no deploy-time
   migration rewrites the table. A failed concurrent build leaves an
   `INVALID` index that still taxes writes: the migration checks and drops
   it. On write-heavy tables state the write cost. Dropping an index with
   `idx_scan = 0` is a candidate, not a fact: counts are per server (a replica
   may use it), the stats window must cover a full business cycle, an index
   behind a foreign key matters for deletes; drop with
   `DROP INDEX CONCURRENTLY`.
3. **Data**: always an owner-approved proposal flagged **irreversible**, never
   an audit action. Options cheapest first, riskiest labelled: what is
   removed (rows, bytes, age), the retention rule that keeps it clean, the
   backup or export taken first, batched execution with pauses. Bloat:
   `VACUUM` or `REINDEX CONCURRENTLY`, never `VACUUM FULL` on a live table.
4. **Flow**: state the user-visible change. Background jobs are idempotent,
   retried with backoff, with visible status; precomputed data names its
   freshness rule.
5. **Cache**: cache the outermost artifact identical for every viewer first
   (edge document → API response → query result → in-process memo). One
   owner per layer and per key; invalidation is an event on write, a TTL only
   the safety net. Never cache personalized, money, auth or real-time state
   at a shared layer. Hot keys get single flight or stale-while-revalidate;
   the hit ratio is measured after rollout.
6. **Infra / architecture**: the audit supplies per-component peak
   utilization, limit, headroom and what saturates first, with waste removed;
   `architect` designs the stage.

## Rollout notes every fix carries
Deploy order (migration before or after code), rollback path, cold-start
effects (an emptied cache, an index build's duration), and the metric and
window that will prove it in production.
