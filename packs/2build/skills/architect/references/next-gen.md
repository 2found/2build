# Next-gen document
`docs/architecture-next.md` unless the project already owns its roadmap
elsewhere. The system document links it from Sources ("next horizon and
scaling roadmap → docs/architecture-next.md") and holds none of its content.
Aim for ten minutes of reading: tables over prose, one sentence per decision.
Unknown facts are `TODO(2build):` lines; every assumed number is tagged
`assumed (<source>, <date>)`.

It is **stale** when older than 30 days, older than the last shipped stage,
or when its horizon's product sources changed; a stale document is refreshed
before anyone builds from it.

```markdown
# <Product> — Next-generation architecture
Horizon: <date or milestone> (source: <PRD / milestone / tickets / owner / baseline ×2 and ×5>) ·
current: <system document link> · measured load: <perf report link or "unmeasured"> · updated <date>

## For the owner
- **Today:** <the system in one sentence; how much growth it takes before users feel it>
- **What breaks first, and when:** <the part, at what growth, roughly when>
- **Unsafe now, whatever the growth:** <missing safety-floor items, or "nothing">
- **The plan:** <steps in order, each one line in plain words with what starts it>
- **Do now:** <cheap preparations and critical fixes>
- **Your decisions:**
  | question | default if you say nothing | cost if the default is wrong | decide by |

## Drivers
| driver (PRD item or growth) | load shape | multiplier at horizon | source or assumption |

## Headroom: today → horizon
| component | peak today | limit | headroom | at horizon | breaks? | evidence (date) |

## Stage inventory
| track | stage | status in place / partial / absent | evidence |

## Findings
| id | severity | finding | evidence | smallest fix | stage |

## Next-gen design
<containers diagram, changes marked> <plain caption>
<deployment diagram, changes marked> <plain caption>
**Stays the same:** <components and contracts deliberately unchanged>

## Decisions
| # | decision | options considered | why | monthly cost | reversal |

## Roadmap
| stage | trigger | prerequisites | change | watch for | cost | rollback | verify | status |
Status: `seam ready` · `waiting for trigger` · `triggered` · `in progress` · `done <date>`.

## Seams to keep now
| seam | for stage | check | status |

## Not yet
| idea | why not now | trigger that would change it |

## Verify next
| fact this design rests on | why it could not be read | command or person that settles it |

## Open risks
```

The owner section uses plain words only: no stage ids, no acronyms without a
gloss, no component jargon where a role works ("the database copy that
serves reads" before "read replica").

When a stage ships: update the current owners (diagram and facts), set its
status to `done <date>`, re-project the headroom table; a finished row is
history and can then be deleted.
