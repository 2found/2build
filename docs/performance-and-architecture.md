# Performance and architecture

Two skills answer "is the system fast enough, and what should it become?"
before anyone buys capacity:

| Skill | Answers | Writes |
|---|---|---|
| `/bbs:perf-audit` | What do users feel, which proven cause is behind it, what to fix first? | a report in `.babysit/perf-audit/<date>-<scope>/` (or the ticket's `evidence/`), and `docs/PERFORMANCE.md` |
| `/bbs:architect` | What does the system really look like today, and what is the next generation, in which order? | the current architecture docs with diagrams, and `docs/architecture-next.md` |

Neither skill edits product code, configuration, infrastructure or data.
Fixes ship through `/bbs:autopilot` with the audit's numbers as acceptance;
staged architecture work goes to `/bbs:autopilot` or `/bbs:team`.

## Run a performance audit

```text
/bbs:perf-audit                          # the whole system: every hot path and shared resource
/bbs:perf-audit audit "<journey|page|route|job>"
/bbs:perf-audit verify <report path>     # re-measure after a fix
```

The audit maps the path from the user's entry through edge, web, API, caches,
database, queues and external calls, ranks paths by weight × total time,
gives each a budget, then diagnoses the red and yellow ones. A cause counts
only when it is proven: a local toggle that removes it and re-measures, a
plan at production volume, or a counted fact. An unproven cause stays a
suspect and is never ranked as a fix.

### What it needs

| Evidence | Source | Without it |
|---|---|---|
| `live` | production logs, database statistics, host metrics, CDN headers, field page data | the report is a measurement plan, health `not-measured` |
| `lab` | a local or reserved environment at production shape | causes stay suspects |
| `static` | code and query plans | always available; never a status or a gain |

Production access must be a route that a repo doc or the owner names as
read-only. A credential that can write (an app database URL, an admin role,
an SSH user that can restart services) is not read-only: the skill asks once
for access, naming what it reads and for how long, and finishes with lab and
static evidence when nobody can answer. Record the route in the project's
`docs/PERFORMANCE.md` Signals table so later audits find it.

On production it never load-tests, restarts, redeploys, changes config or
flags, purges caches, writes or enables extensions. Database reads go through
the bundled `pg_snapshot.sql`, and `EXPLAIN` runs inside
`BEGIN READ ONLY … ROLLBACK`. Logs are aggregated by route with the bundled
`log_paths.py`, which replaces ids, emails and token-like strings; raw log
lines, ids, emails, IPs and SQL parameters never reach the report.

### Budgets

The project's `docs/PERFORMANCE.md` wins; otherwise these defaults apply.

| Class | Budget |
|---|---|
| Page, real mobile users at p75 | LCP ≤ 2.5 s · INP ≤ 200 ms · CLS ≤ 0.1 · TTFB ≤ 0.8 s |
| Interactive API read / write, server p95 | ≤ 300 ms / ≤ 1 s |
| Errors per path | 5xx and timeouts ≤ 0.1 % |
| Host at peak | CPU pressure `some avg60` ≤ 20 % · memory ≤ 80 % · disk ≤ 80 % |
| Database | buffer cache hit ≥ 99 % · connections ≤ 80 % of max |

A path is `green` within budget, `yellow` up to 1.5× or 20 % worse than the
last audit, `red` beyond 1.5×, over its error budget or on a saturated shared
resource, and `unmeasured` without a live or lab number.

### Reading the report

The first section is written for an owner who is not an engineer: health,
what users feel (each a journey, a time, its target, date and source), the
main cause in plain words, what to do first, and what needs a decision. Each
proposal states money per month, downtime, risk, whether it can be undone and
who approves. Business facts the skill could not read are written as
`assumed (<source>, <date>)` with the cost of being wrong, instead of being
asked mid-run.

Fixes are ordered on a ladder, lowest rung first: code, query and index,
data, flow, cache on what already exists, then infrastructure and
architecture. Each proven cause goes on the lowest rung that removes it. Data
changes (purges, retention) are always owner-approved proposals marked
irreversible. Caching what an index fixes, adding capacity for unattributed
cost and splitting a service for performance alone are never proposed.

Each fix theme ends with a ready command:

```text
/bbs:autopilot "<fix theme> — acceptance in .babysit/perf-audit/<date>-<scope>/report.md#f1"
```

After the fix ships, `verify` re-measures the same paths with the same
method and comparable traffic and marks each finding `fixed`, `still` or
`regressed`. A change counts when the after median lies outside the before
range and differs by at least 10 %. A lab verify of undeployed code says so;
production confirmation is a later `verify`.

A verify judges each finding by its own acceptance criteria, not by whether
the patch merged. On 2novel (2026-11-09) the audit found that a bulk cancel
refunded every job at once on the request pool and froze the API for about
four minutes. Two hours after the fix shipped, `verify` returned
`VERIFIED(fixed=1 still=2 regressed=0)`:

| Finding | Before → after | Verdict |
|---|---|---|
| F1 bulk cancel drains the request pool | freeze ~4 min → ~15 s; cancel errors 2/25 → 3/5 | `still`: the root cause is gone (lab tests pass), but live acceptance failed |
| F2 logs lack route, workspace and SQLSTATE | all three present, no SQL params | `fixed` |
| F3 library aggregates every chapter per poll | customers ~20 ms, internal workspace 0.89 s | `still`, deferred by design |

The F2 logs also exposed what the freeze had hidden: each failed cancel was a
`40P01` deadlock between two updates fired by the cancel itself. That became
a new rung-1 finding with its own acceptance. A one-window verify says that
its numbers are provisional until a second comparable window is measured.

### The performance profile

`docs/PERFORMANCE.md` carries what one audit learns to the next: the read-only
signal sources and commands, budgets, hot paths with their last status, known
traps and the audit log. `AGENTS.md` links it so every agent reads it before
performance work.

## Design the next generation

```text
/bbs:architect            # current, then next
/bbs:architect current    # verify the as-is architecture and its diagrams
/bbs:architect next       # horizon, next-gen design and roadmap
```

`current` reads code, deploy config and read-only running state, corrects the
project's architecture docs where a fact is verified wrong, renders the
diagrams and audits safety gaps, headroom, single points of failure,
coupling, scaling cliffs and cost. Where docs and code disagree, the code
wins and the drift becomes a finding.

`next` takes a horizon from the PRD, roadmap or tickets (else ×2 and ×5 the
current peak, labelled), projects the headroom table onto it and designs the
smallest change set that clears the first limits. The roadmap orders stages
(edge, right-size, database, shared cache, jobs, instances, replicas,
partitions, services), and each stage waits for a measured trigger:

```text
<signal> <op> <threshold> in <n> of <m> windows
host CPU PSI some avg60 > 20 % in 2 of 3 weekly peak windows
```

A trigger whose signal is not collected is added to `docs/PERFORMANCE.md` as
`TODO(2build):`. Seams (database location from the environment, idempotent
side effects, no shared state in process memory) are kept now; the component
they enable is added when its trigger fires. A "Not yet" list records what
was considered and the trigger that would change the answer.

A load-driven stage needs a perf-audit report, or equal evidence, that the
waste is already removed. Run `/bbs:perf-audit` first when load is
unmeasured.

## Verdicts

```text
VERDICT: AUDITED(<good|attention|problem|not-measured>, red=<n> yellow=<n> green=<n> unmeasured=<n>)
VERDICT: VERIFIED(fixed=<n> still=<n> regressed=<n>)
VERDICT: ARCHITECTED(<current|next|both>, findings=<n>, stages=<n>)
```

`not-measured`, or a critical path left unmeasured, ends
`DONE_WITH_CONCERNS`. Both skills run unattended; the only question either
may raise is missing read-only production access.
