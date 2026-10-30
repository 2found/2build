# Stage 6 - Pre-test (optional, recorded)

Output: `06-pretest/<round>/brief.md` + `decision.json`. A pre-test filters the
obvious (a hook that reads as an ad, a joke nobody gets, a claim nobody
believes). It does not forecast views.

`mvf pretest-brief <sid>… [--round r1]` writes the brief (every hook variant
verbatim with timestamps, on-screen text, picture, platform copy, the segments)
and a `decision.json` stub. Pick one method and record it:
1. **Simulated panel**, if the user has one (for example the Micro Fish skill
   from the Soot pack, or any persona-simulation tool). The brief carries every
   hook variant verbatim with timestamps, the on-screen text, the picture and the
   platform copy, plus the segments as researched; do not summarise it. Record
   the tool's run id in `run`. Never ask for or write a model key in chat or files.
2. **Honest small human panel**: 3-5 people from the target segment (the user
   finds them) read or watch the hooks cold and answer: where would you swipe,
   what would you comment, does it read like an ad, is anything unbelievable.
   Save each answer verbatim to `06-pretest/<round>/panel/<person>.md`. Label
   `"method": "human-panel"` with n.
3. **Skipped**, with a stated reason (`"skipped_reason"`). Fine for a first
   low-stakes post; say it is untested.

Do not fake a panel with yourself. Persona subagents answering your own brief are
a weak signal at best; if used, label `"method": "agent-panel"` and say so.

```json
{"method": "simulated-panel|human-panel|agent-panel|skipped", "run": "id", "winner": "s01/H2",
 "evidence": "what separated the hooks (n=...)", "objections": [""],
 "changes": ["B04: cut 'documentary' -> 'nature show'"], "untested": ["rewritten H3"],
 "decided_at": "YYYY-MM-DD", "skipped_reason": ""}
```
A hook variant rewritten after the round is a new hook: list it in `untested`.
The winner is produced first; losing hooks can still ship as A/B creatives.

Gate (`mvf check pretest`): `decision.json` exists with an honest method; a non-skipped round names a
winner and evidence.
