# Report template
`<out>/report.md` has two parts with a divider between them. The owner reads
only the first; the agent or engineer who fixes things reads the second.
Delete sections that do not apply.

## Part 1 — for the owner
Written last, from the finished technical part, in the language the owner
writes in, to [owner language](../shared/owner-language.md). It opens
the file: nothing technical comes before it. One screen, about 40 lines.

Allowed in this part: journeys named by what a person does ("opening a
chapter", not a route), seconds, money per month, time to do, plain risk.
Not allowed: routes, file names, SQL, percentiles, query counts, finding ids,
rung numbers, colour statuses, `assumed (…)` tags, vendor names without their
role. Each of those has a home in part 2.

- A percentile becomes people: "for most visits" (p50–p75), "for the slowest
  1 in 20 visits" (p95), "at worst" (p99).
- A budget becomes a feeling: "under 2.5 seconds feels fast".
- Status words: Fast · Slow · Very slow · Not measured (green · yellow · red ·
  unmeasured).
- A cause is one or two sentences a non-engineer could repeat, naming what the
  system does that it does not need to do: "to show one chapter the site asks
  the database 40 separate questions; one would do".
- Recommendations are numbered 1, 2, 3 in the order to do them; number n is
  fix `F<n>` in part 2.
- An assumption is a plain sentence under "I assumed": what was assumed and
  what changes if it is wrong.

```markdown
# Is <product> fast enough? — <feature | the whole product> — <date>

**Short answer:** <Good | Needs attention | Problem | Not measured>. <One
sentence: what a user notices, and whether fixing it costs money.>

## What people feel today
| When someone… | It takes | Feels fast under | Verdict |
|---|---|---|---|
| <opens a chapter on a phone> | <3.1 s for most visits> | <2.5 s> | <Slow> |

## Why
<The main cause, then at most two more, each one or two plain sentences.>

## What I recommend, in order
1. **<What changes for the user>**: <from 3.1 s to about 1.5 s>.
   Cost: <free | about $X a month>. Risk: <low, undone in minutes | cannot be
   undone>. Work: <about half a day for the agent>.
   To start it, paste: `/bbs:autopilot "<one-line requirement> — acceptance in <report path>#f1"`
2. …

## Not worth doing now
<Ideas you may hear, and why not, one line each: "a bigger server: the server
is idle 80 % of the time, the wait is in the database questions above".>

## Decisions only you can make
<Only money, downtime or something that cannot be undone. Each: the question,
my recommendation, what happens if you say nothing.>

## I assumed
<Business facts inferred, one line each, with what changes if wrong.>

## What I could not measure
<What is unknown, what access or time it takes to find out, and that it only
reads.>

Measured <date> from <real visitors over N days | a test on this machine>.
Everything below this line is detail for whoever does the fixing.

---
```

## Part 2 — technical detail
About 150 lines at most. Numbers in tables, causes in one sentence, no dumps:
link the raw outputs saved beside the report (aggregates only). Every number
carries its date and source here.

```markdown
## How this was measured
Revision running vs default branch · evidence level (live | lab | static) ·
windows (UTC, requests per full hour) · vantage point · sources · limits
(what could not be seen, why) · doc-vs-code drift found

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

## Assumptions
| fact | `assumed (<source>, <date>)` | consequence if wrong |

## What works (keep when fixing)

## Known-findings ledger
| id | claim | status fixed / still / regressed / not re-tested | note |
Statuses carry over from the previous report; new ids only for new findings.

## Cleanup
Sessions and tunnels closed · local servers and load tools stopped · temp
copies and secrets removed (or kept at <path>)
```

A `not-measured` report replaces Findings and Fix queue with the plan below;
its part 1 says so in the short answer and lists the same rows in plain words
under "What I could not measure".
```markdown
## Measurement plan
| path | signal to read | command or place | access needed | time | touches production |
```

Verify mode replaces Findings and Fix queue with the table below; its part 1
keeps "What people feel" with a before and an after column and says per
recommendation whether it worked.
```markdown
## Before → after
| id | path | metric | before (date, level) | after (date, level) | change | beyond noise? | verdict |
```
