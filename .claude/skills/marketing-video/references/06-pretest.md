# Stage 6 - Pre-test

Output: `06-pretest/<round>/decision.json`, plus the evidence beside it. A
pre-test picks one of the 3 candidate scripts and one hook, and filters the
obvious: a hook that reads as an ad, a benefit nobody can name, a claim nobody
believes. It does not forecast views.

## Bar
An honest test establishes, for the winner:
- **It holds.** Strangers keep watching past the hook and do not swipe at the
  same dead second.
- **It explains.** Each reader's `retell` says what the video told them, and
  it matches the script's `spine.takeaway`. A retell that is only the hook's
  question, a slogan, a line echoed without its meaning or "it depends" fails
  the script, as does a stranger listing in `unclear` a term the answer
  needed. Fix the body, not the hook.
- **It gives.** Readers can name what they would do, decide or think in their
  own life (for an `answer`: the answer they now hold), and it matches the
  script's `spine.use`. A reader who writes
  "nothing", or a fact about the story or the product, fails the script
  however long they watched.
- **It travels.** Readers would save it or send it to someone, and can say to
  whom.
- **It beats something real.** In a feed with real videos of known views and a
  script already rejected (the control), the candidate ranks above the control.
  If the control wins or ties, the panel cannot tell, and its other answers
  mean nothing.
- **The owner reads it.** On a campaign's first video, the owner reads the
  winning script before any voice or art is made: `mvf sheet <sid>` writes
  the page they read (picture, text and voice per second, the panel's table,
  the retell and the expert flags), and `mvf approve <sid> --verdict
  approved|rejected|delegated [--note]` records their word. `mvf voice` waits
  for it. A rejection carries the reason, which the rewrite starts from.

`decision.json` records the true method:
- `simulated-panel`, a persona-simulation tool such as Micro Fish, with its `run` id;
- `human-panel`, real people from the segment, with n;
- `agent-panel`, fresh-context subagents;
- `skipped`, with a `skipped_reason`.

Never present yourself, or agents reading your own brief, as a human panel.

`mvf check pretest` checks the latest decision; older rounds are history. It
FAILs when:
- the method is not one of those four, or a skipped round has no reason;
- a round names no `winner` or `evidence`, or a simulated panel names no `run`;
- a simulated or human panel's winner has no `retell`, or a feed answer about
  the winner has none;
- a drafted decision still has its `_todo`.

It WARNs when an agent-panel round has no feed, and when `changes` is missing.

## Micro Fish
The default evaluator. Micro Fish runs a simulated society on the user's own Soot: a crowd of
personas from the campaign's segments watches the stimuli, an expert panel
looks for what blocks the video, and an analyst tallies the result. It is a
`simulated-panel`: the result is directional and never a forecast of views.

**First use (once per machine).** `mvf doctor` lists what is missing, and
`mvf judge status` shows the Soot providers that are signed in.
- **Only the user can sign in.** If no provider is signed in, ask them to run
  `! soot login <provider>` themselves. Never ask for a key, and never paste
  or write one.
- **Only the user picks the model.** Ask which provider and model the judge
  should use, then run `mvf judge setup --provider P --model M`. This writes
  mvf's own AI config, holding only that provider entry (no secrets), records
  the choice in `~/.babysit/marketing-video/config.json`, and creates the
  Micro Fish workspace.

Later runs reuse the choice. Set it again only when the user asks.

**Judge the scripts.**
1. Run `mvf judge scripts <sid>… --control ctl-<name> [--bench <id>]
   [--size quick|standard|deep] [--round rN]`. That is at most 4 stimuli: the 3
   candidates and a control (a script written to lose, `ctl-` prefix, not
   lint-gated). Each stimulus is what a viewer gets, second by second. The
   brief asks every persona to retell the video ("Video nói là…"), to tag
   what was unclear, and to say what they would use and who they would send
   it to. A quick run takes 10-30 minutes.
2. It writes `06-pretest/<round>/microfish/`:
   - `answer.md`, the report;
   - `tally.json` and `judge.json`, the per-script numbers and whether the
     lead is clear and beats the control;
   - `retell.md`, the intended retell beside the quoted ones;
   - `interview.md`, the two leaders interviewed in the same conversation.
   It also drafts `decision.json`, or writes `decision.suggested.json` when a
   decision already exists.
3. Read `retell.md` and `interview.md`. Write `retell` with the quoted lines,
   correct `winner` if the quotes and the numbers disagree, and list
   `changes`. Delete `_todo`.
4. Apply the changes to the script before the owner reads it. A change that
   was not tested goes in `untested`.

A stopped run resumes with `mvf judge resume --round rN`. When the control leads or
ties, the panel could not tell the scripts apart: rewrite from the research,
not the hook.

**Judge the cut.** `mvf judge video <proj>` (its round defaults to `v1`) gives the cut the same panel. One
stimulus is built from the timeline and what review saw. It finds where
people leave, whether the hook carries muted, and whether they can still
retell it. Export WARNs (JV1) when a delivered cut was never judged.

Without Soot, use the feed and cold read below as an `agent-panel`, and say so.

## Direction
- **Agent readers are generous and judge entertainment.** Handed one script and
  a persona, they pass almost anything: two scripts with no swipes were rejected
  by the owner as boring or as giving nothing. Make them choose: rank the
  candidates in a feed between real videos and the control. Ask what they would
  use, save and send, and rank by "the one you would most want to save or send
  first".
- **Readers must be cold:** new readers each round, given only what a viewer
  gets (picture, on-screen text, voice, caption), never the spine or the pitch.
- **Cover the rings:** a stranger to the topic, a fan of the kind of thing and
  an impatient scroller, so a fan-only script shows itself.
- **Benchmarks have known views**, from the research's top videos, at least one
  from the benefit's own audience. Pick ones whose body holds, not only their
  opening: if every reader swipes a benchmark at its greeting, it tested the
  hook alone, and any candidate "beats something real" by default.
- **Hearing is not reading.** Readers get text they can re-read; a viewer hears
  each line once. Tell them so, and trust `retell` and `unclear` over "I
  watched to the end".
- **Read the signal.** When every hook loses readers at the same beat, fix the
  body; when one hook holds, the hook is the variable. "Reads like an ad"
  counts only when a reader names the beat.
- **Stop after about 3 rounds.** If the use is still unnamed, go back to the
  benefit and the research, not to the hook. Record what is left in
  `objections`; a hook rewritten after a round goes in `untested`.
- **Set the winner's `hook`** and produce only that script
  ([07](07-production.md)).

## Cold read
`mvf coldread <sid> [--hooks H1 H2] [--round rN]` writes
`06-pretest/<round>/coldread/<sid>-<hook>.md`: only what a stranger gets,
second by second. One way to run it is to give each file to a few fresh
subagents, one per ring plus one impatient viewer, each with a one-line
persona. Save each JSON answer beside the file as `<sid>-<hook>.<reader>.json`.
Each answer has `swipe_at`, `retell`, `unclear`, `learned_or_felt`, `use`,
`use_when`, `save`, `send_to`, `comment` and `reads_like_an_ad`.

## Feed
1. Transcribe 2-3 real videos into `06-pretest/bench/<id>.json`:
   `{id, url, views, caption, rows: [{s: "0-2.5", picture, onscreen, voice}]}`.
   Use the first 30-45 s, from the auto-subs or a viewing;
   `03-research/top/` holds transcripts already.
2. Run `mvf feed <sid>… --control <rejected sid> --round rN [--readers 4] [--seed 1]`.
   It writes one shuffled `feed-N.md` per reader, with the clips labelled
   A, B, C… and nothing else, and a `feed-N.key.json` that only you read.
3. Give each feed to one fresh reader. Save the answer as
   `feed-N.<reader>.json`: per clip `swipe_at`, `why`, `retell`, `unclear`,
   `learned_or_felt`, `use`, `save` and `send_to`, plus a `rank`.

`mvf pretest-brief <sid>… [--round rN]` writes a verbatim brief and a
`decision.json` stub for a simulated or human panel. Keep model keys out of
chat and files.

```json
{"method": "simulated-panel|human-panel|agent-panel|skipped", "run": "id", "winner": "s02/H2",
 "evidence": "what separated the candidates (n=...)", "feed": {"ranks": {"reader": ["s02/H1", "bench:x"]}},
 "objections": [""], "changes": [""], "untested": [""], "decided_at": "YYYY-MM-DD", "skipped_reason": ""}
```
