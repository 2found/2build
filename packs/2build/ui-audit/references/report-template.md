# Report template
`<out>/report.md`, bounded to about 120 lines. Quote, do not paste dumps; link
shots. Delete sections that do not apply. Numbers in rows, prose in findings.

```markdown
# <product> UI audit — <date>
Build/commit · driver · mode (persona flow | journeys) · target (live | copy)
· fixtures · mutations made (allowlist) · limits (what could not be seen, why)

## Frame
Persona, goal, done-when, ideal path. (Journey mode: the journey list.)

## Ground truth
| fact | source | seen in UI? where |

## Journeys
| # | journey | verdict PASS/FRICTION/BLOCKED | exact clicks | what was missing (next step absent?) |

## Scorecard (persona-flow mode)
Every metric from SKILL.md: value vs pass, grade.

## Step log (persona-flow mode)
| # | expectation | observed | PURE (you/2nd) | wait s | result | shot |

## Findings (ranked by user effort or stall, most blocking first)
| id | sev | screen | what breaks / needs a human | evidence (shot path or quote) | proposed fix | owner layer UI/backend/content | file:line or unknown |
Use `second-look` / `workaround` / `question` tags where they apply.

## Known-findings ledger
| id | claim | status confirmed / partial / not reproduced / not tested | note |
Statuses carry over from the previous report; add rows for new ids only.
Re-audit: fixed / still / regressed with a new shot.

## What works (keep when fixing)

## User-need UX proposal
One section per need that is hard to find or hard to finish ("run this now",
"fix the missing key"):
- Problem: where it lives today, clicks from where the user looks, what the
  label hides (cost, risk).
- Proposal: numbered, each a concrete control in a named place (row action,
  command-palette entry with title and id, filter chips, label by cost,
  disabled-with-reason, receipt toast with the real result).
- Owner and size.

## Fix themes
Findings grouped by shared cause, impact / effort. Cause layer named.

## Cleanup
Session closed · PIDs stopped · secrets deleted · copies kept/removed.
```
