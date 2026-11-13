# Report template
`<out>/report.md`, about 160 lines; full Map tables stay in `map/`. Quote, do
not paste dumps; link shots. Delete sections that do not apply. Numbers in
rows, prose only in the owner summary and findings.

```markdown
# <product> UI audit — <date>
Build/commit · driver · scope (product | flow <id>) · target (live | copy |
read-only proxy) · fixtures · mutations made (allowlist) · limits (what could
not be seen or measured, why)

## For the owner
One grade for the product, then one plain sentence per lens, in the user's
words, with its worst number ("The same five numbers sit on Overview, each product page and
Results"; "Opening Monitoring takes 3.4 s and the page jumps twice").
Then the worst problems, at most seven, most blocking first:
- **<what the user has to do today>** → <what it should be>. ![](shots/…)
No ids, file paths or metric names here; a reader who stops after this
section knows what is wrong and what gets cut first.

## Frame
Persona, goal, done-when, ideal path; the flows in scope.

## Lenses
| lens | verdict pass/fail/not measured | worst number vs bar | findings |
Flows · Overlap · Simple · Text · System · Speed · Data · Truth · Access.

## Ground truth
| fact | source | seen in UI? where |

## Journeys
| # | flow id | verdict PASS/FRICTION/BLOCKED | actions / ideal | screen switches | exact clicks | what was missing (next step absent?) |

## Map (failing rows only; full tables in `map/`)
Overlap: | fact or action | owner | also on (copy / copy + controls) | values agree? |
Lifecycle: | object · state | shown where | exit | gap |
States: | state | idioms seen (screens each) | layout jump | not reached |
Geometry: | role | values (screens each) | project scale |
Complexity and text: | screen | primary actions | controls | tabs | words / lines (desktop · narrow) | work surface starts at | vs median | S1 S3 S4 T1–T3 pass/fail |
Speed: | screen or action | cold · warm | CLS | requests (duplicates) | bar |
Drift: | job | idioms with call-site counts | off-idiom sites file:line | enforced by a check? yes/no/partly |

## Step log (walked flows)
| # | expectation | observed | PURE (you/2nd) | wait s | result | shot |
PURE score/color, completion, recovery and jargon per flow.

## Findings (ranked by user effort or stall, most blocking first)
| id | sev | bar row | screens · call sites | what breaks / needs a human | evidence (shot, count or quote) | proposed fix (cut / merge / reuse / rework / add) | owner layer UI/backend/content/design authority (root first) | file:line or unknown |
Use `second-look` / `cold-read` / `workaround` / `question` tags where they apply.

## Known-findings ledger
| id | claim | status confirmed / partial / not reproduced / not tested | note |
Statuses carry over from the previous report; add rows for new ids only.
Re-audit: fixed / still / regressed, each with a before/after pair at the same
screen, viewport and data, and the measured number beside each shot.

## What works (keep when fixing)

## Fix themes (cut → merge → reuse → rework → add)
One block per theme, each a ready `autopilot` requirement:
- Problem in the user's words; the findings it closes.
- Change: what is removed, merged into which owner, or swapped for which
  shared component. For a `missing` job only: the concrete control in a named
  place (row action, command-palette entry with title and id, filter chips,
  label by cost, disabled-with-reason, receipt with the real result).
- Acceptance: bar row + the number measured here → the number to reach.
- Owner layer and size.
Then, when the System lens failed:
- Rules for the design authority (the sentence to add, where).
- Rules for the mechanical UI check (what it matches, the planted case that
  must fail).

## Cleanup
Session closed · PIDs stopped · secrets deleted · copies kept/removed.
```
