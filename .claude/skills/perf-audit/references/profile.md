# Performance profile
The project's durable answer to "how do we see production, what is hot, what
is fast enough". One owner per repo (or per product in a workspace): an
existing linked section of the deployment/ops runbook, else
`docs/PERFORMANCE.md`. Link it from `AGENTS.md` with "read before performance
work". The first audit creates it; later audits update it.

Unknown facts are `TODO(2build):` lines per the
[placeholder rule](../setup-project/references/project-docs.md#placeholders);
a placeholder is an open gap, never a default. No secret values: name the
environment variable or credential and the access route only.

```markdown
# <Product> — Performance profile
Reports: `.babysit/perf-audit/` · host runbook: <link or N/A> · caching contract: <link or N/A>

## Signals
| layer | source | read-only command or place | fields | retention | access |
| app requests | <container/service log> | `<bounded cmd>` | method, route or path, status, duration (unit), user id present? | <days seen> | <read-only route> |
| database | <engine, managed?> | `psql "$<VAR>" …` | pg_stat_statements on? | — | read-only role |
| edge | <CDN> | <analytics / headers> | cache status | | |
| host | <VM/runtime> | <runbook> | PSI, memory, disk | | |
| page (real users) | RUM / CrUX / none | | p75 vitals | 28 d | |

## Budgets
| path or class | metric | budget | source (owner decision + date, or default) |

## Hot paths
| id | path (route template, page or job) | journey | weight | calls/day | p95 | budget | status | last audit |
Weight: `critical` (money, sign-in, the core loop) · `core` · `secondary`.

## Known traps
- <date> <lesson that changes where to look first>

## Audit log
- <date> <scope> <health> → <report path>
```

Rules:
- Hot-path numbers are the latest audit's, dated; history lives in reports.
- Owner decisions (weights, budgets, downtime, spend) record who decided and
  when; an inferred one reads `assumed (<source>, <date>)` until confirmed.
- A known trap is a reusable lesson ("a recreated Redis silently disabled the
  cache in old images: check client list and hit counters"), not a finding.
