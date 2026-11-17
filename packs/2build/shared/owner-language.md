# Owner language
Applies to everything a person reads at a stop: a question, a handoff, a
report's owner section, a final summary. The reader owns the product and has
never opened a terminal for anything but pasting your command. Machine output
(the status block, tables for other agents, raw evidence) keeps its format and
comes after the owner part, never instead of it.

## Shape of a stop
1. **What happened**, in one sentence, in the user's language: the outcome,
   not the activity ("The sign-up page works and is ready for you to try",
   not "implement and QA completed").
2. **Where to look**: one URL or file, and how to open it.
3. **What to do next**: exactly one copy-paste command (the preamble's
   `SKILL_REF` form) or exactly one question. Nothing to do → say so.

Anything else the owner must know (a cost, a risk, something left out) is one
short line each, most important first. Detail goes below a visible divider or
in a linked file.

## Questions
- One at a time. Two to four choices, the recommended one first with its
  reason, each with its consequence in money, time or risk.
- "Decide for me" is always a valid answer: take the recommended choice and
  record it as a Taste decision
  ([Auto-Decision Framework](auto-decision-framework.md)).
- Never ask what a file, the code or a sensible default answers.

## Words
- A role before a name, and the name glossed once: "the database copy that
  serves reads (read replica)", "the service in front of the site that keeps
  copies of pages (Cloudflare, a CDN)".
- No git, CI, framework or statistics vocabulary without that gloss. Say
  "saved as one checkpoint in the project's history", not "committed".
- Internal identifiers stay out of the owner part: ticket ids, finding ids,
  rung and stage numbers, status and verdict vocabulary, file paths the owner
  will not open.
- A number is what a person feels or pays, next to what it is compared with:
  "the page appears after 3.1 seconds on a phone; under 2.5 feels fast",
  "about $12 a month". Technical measures (percentiles, CPU share, query
  counts) belong to the technical part.
- Lead with the verdict. Good, bad or unknown comes before the evidence.

## Test before sending
A stranger who reads only the owner part can say what happened, whether it is
good or bad, what it costs, and what to do next. If they would have to ask what
a word means, replace the word.
