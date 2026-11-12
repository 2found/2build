# Requirements drafted by an agent

Applies when the requirement was written by an agent for a person who has not
read it: a Soot `company-loop` draft, a ticket another agent filed, a
generated opportunity. The file says so (`Status: draft written by …`, a
`Source:` line naming a tool) or the invocation does. A requirement the
invoking human wrote or pasted is not one, and none of this applies to it.

Such a draft proves someone approved an *idea*, not this spec. Three rules,
the same in `autopilot` and `team`.

## 1. A person confirms the spec once, `--auto` included

Write the spec back into `requirement.md` in the requester's terms — the
journeys, numbered acceptance criteria, non-goals, constraints and the
assumptions you took — and get one confirmation before any plan, design or
code. `--auto` delegates the design and plan reviews that follow; it never
delegates this one, and `approval self-resolve` refuses kind `spec`.

| Run | Channel |
|---|---|
| `developer`, no `--auto` | the preamble's one `AskUserQuestion`, with the spec path |
| `--auto`, or any other role | a durable record, because nobody watches the terminal (below) |

```bash
BABYSIT_TICKET="$TICKET" bbs ticket approval publish --kind spec \
  --note "Confirm the spec: <requirement.md path>. <open questions, one line each>"
BABYSIT_TICKET="$TICKET" bbs ticket approval await
```

`approved`: note `Confirmed: <who>, <UTC time>` at the top of the spec and
continue. `redirected`: fold the note in and publish once more. `dropped`:
stop and report. Put every open question in the same note; this is the only
time the requester is asked.

## 2. A criterion that can only be seen after release is not a QA criterion

"`first_payers` rises by 10 within 30 days" cannot pass or fail before the
handoff, so QA would either skip it or fake it. Move each such line to a
`## Measured after release` section naming the metric, the expected change,
the window and who measures it. In its place, the acceptance criteria carry
what can be checked now:

- the behaviour that is meant to move the metric, as a journey a reviewer
  completes by using the product;
- that the named metric is readable where the requester measures it. When
  the product does not record it, say so as an open question in the
  confirmation; never build analytics the draft did not ask for.

## 3. The requester's effort estimate is the appetite

A draft's estimate (`est_effort`, "effort S", a day count) is how much the
idea is worth to the requester, not a forecast to correct. Map it to the
[size rubric](ticket-size-rubric.md) and plan inside it: cut scope to the
thinnest version that meets the criteria before growing the size.

When the smallest plan that meets the criteria is still more than one size
above the estimate, that is the requester's call. Before confirmation it is
one of the questions in rule 1. Found later (at basic design or `plan-draft`)
it is a User Challenge: publish it the same way, naming the two options —
the cut that fits, and the full size with its cost — and build neither until
it is answered. A draft with no estimate has no appetite; size it normally.
