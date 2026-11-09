# Report template
`<out>/report.md`, about 150 lines at most. Numbers in tables, causes in one
sentence, no dumps: link the raw outputs saved beside the report (aggregates
only). Delete sections that do not apply.

```markdown
# <product> performance — <feature | system> — <date>
Revision running vs default branch · evidence level (live | lab | static) ·
windows (UTC, requests per full hour) · vantage point · sources · limits
(what could not be seen, why) · doc-vs-code drift found

## For the owner
**Health:** Good | Needs attention | Problem | Not measured — <one sentence why>
- **What users feel:** <2–4 bullets, each a journey, a time, its target, date and source>
- **Why:** <the main cause in plain words>
- **Do first (cheap, safe):** <fix themes, expected result in user terms>
- **Needs your decision:** <item — cost per month / downtime / risk — recommendation>;
  every `assumed (<source>, <date>)` fact with what changes if it is wrong
- **What I need to measure** (Not measured only): <access or signal — how long — touches production? (read-only)>

## Path map
<Mermaid sequence of the audited path, or the system's component map,
marking where the time goes>

## Hot paths
| id | path | weight | calls/day | p50 / p95 / p99 | budget | status | level |

## Findings (ladder order)
| id | rung | path | cause (one sentence) | proof | waste/real | gain (est.) | confidence | rollout risk | undo |

## Fix queue
### F1 — <theme>
- Problem (user terms):
- Cause and proof:
- Fix direction (rung):
- Acceptance: <path, metric, budget, baseline, exact measure command, level>
- Regression guard:
- Rollout:
- Run: `/bbs:autopilot "<one-line requirement> — acceptance in <report path>#f1"`

## Headroom (system scope, or when a fix reaches rung 6–7)
| component | peak load | limit | headroom | saturates first? | evidence |

## Not worth it
| idea | why not (gain within noise, cold path, risk > gain) |

## Suspects (unproven)
| suspect | evidence so far | next check |

## What works (keep when fixing)

## Known-findings ledger
| id | claim | status fixed / still / regressed / not re-tested | note |
Statuses carry over from the previous report; new ids only for new findings.

## Cleanup
Sessions and tunnels closed · local servers and load tools stopped · temp
copies and secrets removed (or kept at <path>)
```

A `not-measured` report replaces Findings and Fix queue with:
```markdown
## Measurement plan
| path | signal to read | command or place | access needed | time | touches production |
```

Verify mode replaces Findings and Fix queue with:
```markdown
## Before → after
| id | path | metric | before (date, level) | after (date, level) | change | beyond noise? | verdict |
```
