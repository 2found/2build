# Scaling roadmap
Three tracks. The **safety floor** does not wait for load; the **load
ladder** waits for its triggers; **availability** follows the owner's target.
Inventory every stage of all three as `in place`, `partial` or `absent`, with
the evidence (file, config, console) for each.

## Triggers
Written as `<signal> <op> <threshold> in <n> of <m> windows`, read at peak
after waste is removed. Default: 2 of 3 weekly peak windows. Signals and
budgets come from the [performance profile](../perf-audit/references/profile.md)
and [perf-audit budgets](../perf-audit/references/measure.md#budgets); a
trigger whose signal nobody collects names that missing signal as a finding
and is appended to the profile's Signals table as `TODO(2build):`.

Default thresholds: host CPU pressure `some avg60` > 20 % · memory peak > 80 %
· database CPU > 70 % · database connections > 80 % of max · cache hit < 95 %
on a cached path · origin share of viewer-identical `GET`s > 30 % · job
oldest-age growing · p95 over budget on a hot path after its fixes. Managed
databases report CPU, memory, connections and storage in their console or
monitoring API; read them there, not from the app.

**Triggers decide, the order breaks ties.** Skip a stage with evidence it does
not apply (all responses personalized → edge gains little); take one early
when its trigger fires early; never take one whose trigger has not fired,
except its seam.

## Safety floor
Take now, whatever the load; an absent item is a `critical` finding.
| item | in place when |
|---|---|
| Recoverable data | automated backups, point-in-time recovery, and a restore tested within 90 days, with its duration |
| Database not reachable from the internet | private network or an authorized proxy; app role without DDL or superuser |
| Alerts reach a person | site down, 5xx rate, disk > 80 %, certificate expiry, failed backup |
| Observability floor | request log with route template, status and duration; database statement statistics; host CPU pressure and memory |
| Secrets out of the repo | env or secret manager, rotation possible without a code change |

## Load ladder
Cheapest to add and to undo first, outermost first: a request the edge
answers never reaches the cache, and a query the cache answers never reaches
the database.

| stage | trigger (default) | needs first (seams) | watch for | cost · reversal |
|---|---|---|---|---|
| **Remove waste** | always, before any other stage | — | — | config or code · easy |
| **Edge cache** (a CDN such as Cloudflare) | viewer-identical `GET`s reach origin; origin serves static or media bytes; far users see high TTFB; abusive traffic in origin logs | cacheable responses read no identity; per-viewer state loads on the client; purge on write | one viewer's content cached for everyone if identity slips in | config · easy |
| **Isolate neighbours** | another product or job on the same host or database takes a measured share of the peak | per-product attribution (containers, database roles) | moving the neighbour, not the hot product, is often cheaper | tens per month · easy |
| **Right-size** | attributed real work keeps a host or database over its threshold | waste removed (a bigger box hides a retry loop until traffic grows into it) | resize window | tens per month · easy |
| **Own database**: off the app host, not shared, pooled | app and database compete for the same cores or memory; connections > 80 % of max | database URL from config; online-safe migrations | every query pays network latency, so N+1 hurts more; transaction pooling breaks prepared statements, advisory locks, `LISTEN` | tens per month · hard once data moves |
| **Shared cache service** (Redis, Valkey) | hot repeated reads remain after the edge, across several instances or a working set too large for process memory | one owner per key family; event invalidation; falls through when down | stampede on cold start; silent loss after the service is recreated (check client list and hit ratio, not "up") | tens per month · easy |
| **Background jobs** (a Postgres-backed queue first, `SKIP LOCKED`) | side effects or third-party calls push a request over budget; heavy jobs (render, TTS, import) steal CPU from requests | idempotent handlers; retries with backoff; visible status; a dead-letter path | failures move out of sight unless alerted | config to tens · easy |
| **Workers on their own host** | jobs still take a measured share of request-host CPU | the queue above | the cheap version of a service split: take it first | tens per month · easy |
| **More app instances** behind a load balancer | one instance at its ceiling and right-size no longer cheaper; zero-downtime or host-failure tolerance required | no shared state in process memory; uploads in object storage; scheduled jobs take a lock; shared rate limits | per-process caches diverge; cron runs twice; mixed versions during deploy | tens per month · easy |
| **Read replicas**, reporting off the primary | primary CPU over threshold from reads after the stages above, queries already efficient | separate read and write handles; lag-tolerant reads named | read-your-own-write; money, auth and entitlement reads stay on the primary | hundreds per month · easy |
| **Partition big tables** by time or owner | a table's size makes vacuum, index builds, migrations or retention slow, and hot queries touch a slice | the partition key in hot queries | queries without the key scan every partition; unique keys include it | config · hard |
| **Specialized store** (search, analytics, vector) | a workload the primary serves badly even indexed | one source of truth, a sync path (outbox or change capture), a rebuild procedure | the copies drift; a second system to operate | hundreds per month · easy (the source stays) |
| **Split a service, add a broker** | a domain needs isolation jobs cannot give (scaling, failure or security profile), its own team or deploy cadence; several services consume the same events | the module owns its tables and is called through an interface; versioned contracts; an outbox | transactions become sagas; tracing, deploys and on-call multiply. Never for performance alone | an operating model · very hard |
| **Shard** | writes or size exceed the largest primary you will pay for, after replicas, partitions and a functional split | the shard key on every row and in every hot query; ids not tied to one sequence | cross-shard joins; every migration × N; rebalancing. Prefer a managed distributed database | an operating model · practically none |

A broker starts as a managed queue; a log-based broker (Kafka-like) only for
event streams with several independent consumers or replay.

## Availability
Driven by the owner's target, written as downtime they accept ("at most one
hour a month"); no target → `assumed: best effort` under the owner's decisions.

| step | for a target of | in place when |
|---|---|---|
| Health checks and automatic restart | any | the orchestrator restarts a hung process |
| Zero-downtime deploys | under a few hours a month | blue/green or rolling, migrations online-safe |
| Off-site backups | any | a copy outside the provider account or region |
| Second app instance, database high availability | under an hour a month | failover tested, its duration known |
| Several zones | under minutes a month | instances and database across zones |
| Several regions | a stated residency or disaster requirement only | never for latency a CDN solves |

## Seams to keep now
Cheap today; they make a later stage a configuration change. A missing seam
the horizon needs is a finding.

| seam | for | check |
|---|---|---|
| Cacheable responses read no identity; per-viewer data loads on the client | edge cache | no cookie or auth read in cacheable server routes |
| Every write that changes a cached artifact emits a purge or delete | edge cache, shared cache | write paths call one invalidation helper |
| Database, cache and storage locations come from the environment | right-size, own database, more instances | no hard-coded hosts, sockets or local paths |
| Migrations are online-safe on big tables | own database, partition | no table rewrite in deploy-time migrations |
| Side effects sit behind one function per effect, idempotent | background jobs | an idempotency key per job or effect |
| No shared state in process memory; files in object storage; jobs take a lock | more instances | sessions, uploads, cron inventory |
| Separate read and write handles (both may point at the primary today) | read replicas | one place decides where a read goes |
| Each module owns its tables; others call it, never join into it | split a service | table-writer map from the code |
| Big tables carry the owner or tenant key; ids not tied to one sequence | partition, shard | key present in hot queries |
