# Auto-Decision Framework
Classify → act → report. Auto-deciding replaces human judgment, not analysis.

| Tier | When | Action |
|------|------|--------|
| Mechanical | One clearly right answer. | Decide silently; no log. |
| Taste | Multiple reasonable choices. | Apply the principles; report in the run summary. |
| User Challenge | A guess could land incorrect work or change the user's direction. | Never auto-decide; emit `NEEDS_CONTEXT` via the [preamble's channel](preamble.md#one-mode-four-escalation-channels). |

## The 6 Decision Principles
1. **Correctness:** don't guess when the result could be wrong.
2. **Bounded scope:** auto-approve expansion only within the task/direct importers,
   touching fewer than 5 files with no new infrastructure; escalate wider work.
3. **Pragmatic:** spend at most five seconds on equivalent fixes.
4. **DRY:** search before adding code.
5. **Explicit:** prefer the obvious, small fix.
6. **Verified action:** act and self-verify; report nonblocking concerns at completion.

Tiebreakers: design P1+P4; implementation P5+P3; quality P1+P2;
frontend P1+P3; ops/migration/deploy P2+P6.

## User Challenges
Escalate materially different requirement readings, unauthorized irreversible
or high-impact actions, missing non-derivable config/credentials, security
choices with no clear safe option, or a proposed change to the user's direction.
Keep the original direction as the default and supply:
```text
WHAT YOU SAID: <original direction>
WHAT WE RECOMMEND: <change or clarification>
WHY: <reason and principle>
WHAT WE MIGHT BE MISSING: <blind spots>
IF WE'RE WRONG, THE COST IS: <downside>
OPTIONS: <2–4 labeled, mutually exclusive options>
```
Prefix security/feasibility risks with `⚠️ FLAGGED AS RISK, NOT PREFERENCE:`.

## The Final Gate (`INVOKER=developer` only)
After completing work, present Taste decisions together: summary, each choice
and principle, viable alternative and impact, decision count.
Use one `AskUserQuestion`: approve, override, ask about a choice, or redo.
No Taste decisions → skip the gate. Six or more → flag “High ambiguity run —
review carefully”. Other invokers name Taste decisions in their status
`SUMMARY`; unresolved User Challenges still require `NEEDS_CONTEXT`.
