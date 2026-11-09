# Stage 4 - Strategy (angles)

Output: `04-strategy/angles.json`, 2-3 angles. An angle is one benefit, for one
segment, made believable by facts. It is built on a picked idea from
[research](03b-research.md) and given a reason to share.

## Bar
`mvf check strategy` FAILs when:
- there are no angles, or ids repeat;
- an angle's `segment`, `facts`, `trends` or `platforms` point at something
  that does not exist in the earlier stages;
- `insight`, `promise`, `why_watch`, `why_share` or `format` is empty;
- an angle stands on no fact;
- research exists and the angle's `idea` is not in it (an idea that exists but
  is not picked only WARNs: pick it in research, one picked idea per script);
- the `benefit` is missing or malformed ([Benefit](#benefit)).

It WARNs when every angle has the same format, when the benefit differs from
the campaign's, when the takeaway needs insider words, and on freshness
([Is it fresh?](#is-it-fresh)): no overlap verdict (OV1), an `OVERLAP` angle
with no `overlap_note` (OV2), an empty `swap` (SW1).

## Is it fresh?
An angle the viewer has already seen loses them at the hook, however good the
benefit. Two fresh helpers answer it, each with only its brief, so neither has
seen the angles you are proud of.
1. `mvf predict` writes `04-strategy/predict-brief.md` with the brief's words
   only. The helper lists the five videos anyone in the niche would make first
   (P1–P5) into `04-strategy/predict.md`.
2. `mvf overlap` writes `04-strategy/overlap-brief.md` with the angles, that
   list and this owner's past videos (sibling campaigns' angles and payoffs,
   plus `campaign.past`). The helper writes `04-strategy/overlap.json`, one
   verdict per angle: `CLEAR` (nothing makes this point this way), `SHARES`
   (same topic or format, another point) or `OVERLAP` (the same point the same
   way).
- An `OVERLAP` is changed (another angle or format) or kept with an
  `overlap_note` saying why it is still worth making (the owner asked for the
  sequel; this one answers what that one left open).
- Freshness is about the angle and the format. A known benefit is fine; the
  benefit is the owner's call.
- **The swap test.** In `swap`, put the closest competitor in place of the
  product and write what is left. If the video still works unchanged, the angle
  is about the category; move the product to where the value happens.

## Benefit
Every angle says what the viewer can use:
`benefit: {type, takeaway, use_when}`.
- `type` is this angle's benefit: lesson, decide, answer, match or how-to.
  Angles may differ. If the owner named a campaign `benefit`, the angles
  follow it. On an awareness campaign, a `decide` about the product itself
  (start it, which version, where) only serves viewers already weighing it,
  and the check WARNs on it.
- `takeaway` is the principle, answer or skill in plain words.
- `use_when` is the moment in the viewer's own life it is for. Examples: "when
  your invoices pile up at month end", "when you pick a gift for a gamer", "when
  a colleague takes credit for your work".

The **name-deletion test** checks a lesson, a decision, a match or a how-to.
Strike every product name, character, place and piece of jargon from the
promise and the takeaway. If nothing usable is left, the angle is a recap or a
feature tour, so pick another idea. An `answer` keeps the topic's words: its
takeaway is the answer itself, stated so a viewer can repeat it to whoever
asked, never a general moral drawn from it.

## Direction
- **The insight is something the segment already knows** and rarely hears said:
  true, specific, often a little funny or painful.
- **`why_watch` must not be "to learn about the product".** That answer means
  it is an ad.
- **`why_share` is the reach.** A video nobody forwards stops at its first push.
  Name the trigger and the line someone would send it with. The triggers are
  identity ("this is so you"), utility (saved for later), debate (people
  comment to disagree) and emotion (a laugh or a gasp to pass on). Debate and
  emotion carry a video only on top of a benefit. A debate angle still settles
  the question: each side's evidence, then the verdict or the exact condition;
  people argue with an answer, not with a shrug. "It is a good product" is not
  a reason to share.
- **Test different ideas, not synonyms.** Angles differ in benefit, segment or
  format: a demo against a skit, a tutorial against a testimonial, a comparison
  against a story. Each angle becomes one of the 3 candidate scripts; the
  pre-test picks one ([06](06-pretest.md)).
- **Say which top-video pattern the angle rides on** (from research), or why
  the audience wants something none of the top videos gave.
- **The product is the tool of the benefit, not the topic.** It is seen where
  the value happens. Brand recall comes from seeing it work, and from a
  recurring format or series that earns the follow.
- **Write the hooks from the payoff backwards.** The strongest moment, result
  or detail of the body usually makes the best opening. A hook names the topic
  in the segment's own words and makes them need the next second. A bare label
  ("5 features of X") tells them the topic and gives no reason to stay. Promise
  only what the body delivers: clickbait buys 3 s and loses the watch-through.
- **Fit each platform.** Long-form is a different script on a 16:9 canvas, not
  a stretched short, and the thumbnail and title make its promise.

## Tool
`mvf check strategy`. Keys the tool reads (`tools/templates/campaign/angles.json`):

```json
{"angles": [{"id": "G1", "segment": "A1", "idea": "I1", "insight": "", "promise": "", "why_watch": "",
             "why_share": "", "benefit": {"type": "lesson", "takeaway": "", "use_when": ""},
             "format": "demo", "facts": ["F1"], "trends": ["T1"], "platforms": ["tiktok"],
             "swap": "with <competitor> in its place: what still works", "overlap_note": ""}],
 "plan": [{"script": "s01-name", "angle": "G1", "why": "what this video tests"}]}
```
`hooks` (draft openers) and `plan` are notes for the writer; the check does not
read them.
