# Stage 5 - Script

Output: 3 candidate scripts (`budget.scripts`), `05-scripts/<sid>.json`
(`mvf new-script <sid> --angle G1`).
Each carries the video, its hook variants and the platform copy (caption, X
post and thread, YouTube title, description and tags, cover or thumbnail text).
The candidates come from different angles or framings, not three polishes of
one draft. The pre-test ([06](06-pretest.md)) picks one script and one hook,
and only that one is produced.

## Bar
`mvf lint <sid>` measures against the latest voice take, or against an estimate
it labels as such. It FAILs when:
- **Benefit:** `value` or `spine.use` is empty ([The benefit](#the-benefit)).
- **Hook:** the hook has no voice and no on-screen text, or the spoken hook
  ends more than 1 s past the platform's limit (2.5 s on TikTok and Shorts,
  2.0 s on X, 5 s on long-form). Cut words to fix it; never speed the voice up.
  It also FAILs when there is no on-screen hook text while X is a platform
  (X autoplays muted), or when frame 0 has no `visual`.
- **Value:** the first beat that gives value starts more than 2 s after the
  platform's first-value time (6 s for a short, 15 s for long-form). A beat
  gives value when its role is a value role (turn, demo, proof, payoff,
  reaction, beat, apply) or when its `vo` says part of `spine.use`,
  `takeaway` or `payoff`, whatever its label. Relabelling a beat is never the
  fix; moving the answer earlier is.
- **Length:** the runtime is outside the platform's hard limits.
- **Claims:** a number with a unit, a price, "free", a superlative or a
  guarantee in a beat or in the copy cites no fact or measured material, or
  cites one that does not exist.
- **Material and rights:** cited material is unknown, stale or not showable,
  or a real person is pictured without the rights. No material is cited while
  a pack exists, unless `material_override` says why.
- **Art:** an `art` id is unknown, or is a reference sheet.
- **Copy:** a caption, post, title, description, tags, hashtags or chapters
  break the platform's hard limits.
- **Structure:** fewer than 3 beats, repeated ids, a first beat that is not
  `role: hook`, variant ids outside H2…H9, an incomplete voice mode, or a
  reference to an angle, idea, trend or platform that does not exist.

Everything else WARNs as advice: spine shape and loops, roles, recipes, the
apply beat, layers and insider words, brand placement, CTA timing, hook wording,
search terms, pacing, and whether the `vo` says the retell (W5, a word-overlap
proxy). Follow it, or answer it in `warn_notes`: an answered WARN shows as INFO
with your reason. Never bend the script to silence a WARN that is wrong for it
(a rhetorical question added for a loop check, a sentence split for a pacing
check, a role relabelled); answer it instead.

## The benefit
A viewer stays, saves and sends a video for what it does for *them*. The story,
demo or joke is the proof; the benefit is the product. In one campaign, two
scripts passed every agent panel and the owner rejected both: one was a fan
recap, and one was a good story that "gave the listener nothing".
- The angle's `benefit` (lesson, decide, answer, match or how-to) is what the
  script delivers ([04](04-strategy.md#benefit)). The use rides on something
  from inside the subject that a stranger would retell. A script whose body is
  the product's packaging (length, chapter counts, where to get it) has
  nothing to retell, however useful the decision is.
- `spine.use` is the sentence a viewer could say afterwards, in plain words.
  For a lesson, a decision, a match or a how-to it is about their own life:
  "next time the build breaks I check the cache first", "I'll start this
  series at book 2", "I'll put the chopping board under a damp towel".
- **For an `answer` the use is the answer itself**, said so the viewer can
  repeat it to whoever asked: "a level-3 warrior beats any mage below level
  6", "the fee is waived after 12 months", "the gap is the sleeve, not the
  case". It names the topic's own things, because the question did. Do not
  bend it into a life lesson: an answer turned into an aphorism ("it depends
  when you ask") is no longer the answer anyone asked for.
- **The name-deletion test** (lesson, decide, match, how-to): delete every
  name, title and piece of jargon from the script. If the use disappears with
  them, there was none. An `answer` takes the retell test below instead.
- Hand the use over before most viewers have left, not in the last line.
  Speak it to the viewer's situation, not the character's or the product's.

## Understood in one hearing
The bar no check can measure, and the one the owner reads for first (the
standards in SKILL.md).
- **The retell.** Before any pre-test, write in `spine.takeaway` the one or
  two sentences a stranger should say after one viewing. Then read only the
  `vo` lines, once, at speed: if they do not say that retell, in that order,
  with nothing missing, the script is not done. Lint W5 flags a part of the
  takeaway whose words the `vo` never uses; it cannot hear meaning, so the
  read-through is still yours. The pre-test asks readers for
  the same retell ([06](06-pretest.md#bar)).
- **Answer what the hook asked, in the form it asked.** "How many levels of A
  equal one of B?" wants a mapping; "which is better?" wants a verdict or the
  condition and each result; "why?" wants the cause. A turn or a counterpoint
  deepens the answer; it never replaces it. Two sides each "half right" is an
  answer only when the video says exactly which half, when.
- **Define, don't dodge.** Each term the answer needs gets its plain meaning
  the first time it is heard, in the same breath ("the first five levels,
  roughly the beginner tier"). A term it does not need is cut. Never keep the
  word and drop the meaning.
- **Every step the answer stands on is in the video.** If the payoff needs
  the viewer to know how the ladder is counted, the video counts it. That
  step is the explanation, not setup; budget the seconds for it.
- **Explain, don't relay.** Sources prove a line; they do not replace
  explaining it. A chain of "the book says… someone replies… the other
  answers…" hands over quotes without their meaning. Paraphrase a dense
  source line into plain speech, cite it, and quote verbatim only a line that
  is clear on its own at speaking pace.
- **Length follows the content.** A short is as long as the explanation needs
  and no longer: cut repetition, asides and qualifiers the answer does not
  use, never a step. When the explanation cannot fit, narrow the question;
  do not thin the answer.
- The owner reads the first script of a campaign before any voice or art.
  Agent panels judge entertainment; the owner judges benefit.

## Several audiences
Most people the feed shows a video to have never heard of the product or the
topic ([rings](02-audience.md)). A reach script works for them first:
- **Use their words and their situation** (money, time, family, work, a gift,
  being stuck) before any name, title, feature name or jargon. A word a
  stranger does not know reads as "not for me". Names can be the reveal.
- **Give each depth something.** The stranger gets the benefit. A fan of the
  kind of thing gets a familiar trope turned over. A fan of this very product
  gets the exact detail, number or line they will comment on. `layers` records
  what each one gets.
- **Keep findability.** The caption, the title and the end carry the search
  terms.
- **Say who is asking.** The post copy names the brand before the video asks
  anything (AD1), and the platform's branded-content label goes on. A viewer
  who learns at the CTA that a "tip" was an ad reads it as seeding.
- **Sound like someone telling a friend.** A solemn or salesy voice from the
  first line reads as a lecture or an ad.

## Direction
The shape is yours: a demo, a tutorial, a story, a testimonial, a comparison, a
list, a skit or an essay ([formats](formats.md) names what each kind gives the
viewer and what usually breaks it). What holds across all of them:
- **One question answered, not a topic covered.** The opening makes the viewer
  need something, and the video delivers it. Every beat raises the stakes,
  moves toward the answer or gives value; cut the rest.
- **Write for the viewer who leaves at 3 s, at 6 s and at half-way**: each
  leaves with something. No setup between the hook and the first value; the
  first piece of the explanation is value, not setup.
- **One idea per video.** Ten features are ten videos, or one shown well.
- **Specific beats generic:** the real number, the exact line, the real screen,
  one scene with a place and an action.
- **Spoken, not written.** Read it aloud at speed: short sentences, key word
  last. On screen goes the punch, and it tells the video muted.
- **The payoff keeps the hook's promise.** End on it, not on a goodbye. A
  question back to the viewer ("which side are you on?") rides after the
  answer, never instead of it.
- **Brand in the picture:** seen where the value happens, named at most once,
  one soft CTA. For a story about a book, a film or a game, the product is
  where the viewer continues it.
- **Write several hooks**, each a different way in. The clearest becomes H1.
  Variants replace beat B01 only, so the pre-test isolates the hook.

What cold readers punished, in general terms:
- the world, the rules or the feature list before a person and what they want;
- polishing the same draft, when the fix was in the source (back to research);
- a vivid detail that does not touch the question;
- the tease: the hook asks, the body hints, the end asks the viewer back. The
  owner could not tell what such a script was about after many rounds that
  every check and panel passed;
- a term kept and its meaning cut, so the line needs the source to make sense;
- vague verbs ("did something shady"), where the reader wanted the act;
- a name or title used as the introduction;
- unsourced lines. Fans and users check.

## Craft
Lint WARNs on each of these (answer one in `warn_notes` when it is wrong for
the script). Control scripts (`ctl-…`) skip the key, thesis and turn checks.
- **Write the ending first.** Decide the payoff line and the picture it lands
  on before the hook. Everything the ending needs is set up in the first third,
  so the last beat pays off instead of explaining. Nothing new comes after the
  payoff except a CTA or an end card (E1). The payoff keeps its own seconds
  after the last word: 2 s when the video ends there, 1.5 s when a CTA follows
  (E2); give the last beat `"hold": <seconds>` (a `tempo` of `hold` adds no time).
- **Key moments.** Mark the 2–4 lines a viewer should remember with
  `key_moment: true` (K1). Each holds at least 1.5 s after its last word (K2),
  so the line can land. Direction gives each one its own sound moment: one hit
  at the line's end and the bed ducked a further 6 dB over the hold, unless a
  cue already sits there or the shot sets `key_sfx: false`.
  ([07-craft](07-craft.md#key-moments)). `keys` is something else: caption
  word highlights.
- **Thesis wordings.** `spine.thesis` holds 2–3 wordings of the payoff line,
  each with a different verb or stance (TH0). `mvf sheet` shows them, the owner
  picks one with `mvf approve <sid> --thesis N`, and the picked wording must be
  said in a payoff or turn `vo` (TH1). Wording is the owner's taste; the agent
  offers, it does not pick.
- **The turn has a before and an after.** `spine.turn` says what the viewer
  believed and what they believe now: "Trước: … / Sau: …", "Before … after …",
  or `{"before": "", "after": "", "kind": ""}` (TN1). Kinds that work:
  reversal (the reason flips), reveal (a hidden fact), transformation (the
  person changes), threshold (a line is crossed), mirror (the hook comes back
  meaning more), contrast (two things side by side).
- **Chain of questions** (an informing short): each beat answers the last
  beat's question and raises the next one, so no stretch passes without an open
  question (N3, N4 time the gaps). The next question is held in the picture or
  in a word, then answered in the very next beat. A question carried to the end
  and answered vaguely is a tease, which the owner rejects.
- **Two passes.** First pass: every step of the explanation, in order. Second
  pass: read it aloud at the voice's pace and cut filler inside lines (a
  qualifier, a repeated subject, "actually"), never a step. Lint prints the
  word count and words per second at the voice's rate (WC).
- **Tells.** Readers hear certain shapes as generated copy or as an ad, and
  stop trusting the line: a dash joining clauses (WT0; a numeric range is
  fine), "không phải X mà là Y" / "not just X, but Y", "hãy tưởng tượng" /
  "imagine…", "hành trình" / "journey", "khám phá ngay", "đừng bỏ lỡ",
  inflated words (siêu phẩm, kiệt tác, game-changer, unlock), three abstract
  nouns in a row, stacked "!" (WT1). Lint reads every `vo`, `onscreen` and
  hook variant. Say the plain thing instead.
- **Vietnamese scripts go through [humanizer-vi](../humanizer-vi/SKILL.md)**
  in embedded mode before lint and the pre-test, and again after any rewrite
  the pre-test asks for: `vo`, `onscreen`, hook variants and platform copy.
  It catches what lint cannot hear (calques of English idioms, "việc/sự/một
  cách", bookish words in a spoken line). It changes wording only: every
  fact, number, name and quoted line stays, and a beat keeps its step.
- **Tempo.** An optional beat `tempo` (`fast`, `breath`, `hold`) says how the
  beat should feel; three alike in a row reads as one long beat (TP1).
- **Devices.** A line with a number or a comparison names a relation the
  picture can show. Give the beat a `device` (counter, dots, ladder, bars,
  compare, steps, html), or `"device": "none"` when the picture already shows
  it (DV1). Direction checks the device is on screen.

## Tool
`mvf new-script <sid>` writes the template (`tools/templates/campaign/script.json`);
`mvf lint <sid> [--hook H2] [--take latest|none|tNN]` runs the checks. When the
evidence says a candidate should be replaced, pick the new idea in research and
unpick the old one, point an angle and its `plan` row at the new idea, and run
`mvf new-script <new-sid> --angle G…`. Then delete the old script and its sheet,
so only live candidates remain. The keys the tool reads:

```json
{"id": "s01-name", "angle": "G1", "format": "demo", "recipe": "product-demo", "language": "en",
 "value": "what the viewer walks away with", "hook": "H1",
 "layers": {"broad": "", "genre": "", "core": ""},
 "spine": {"question": "", "stakes": "", "turn": "", "payoff": "", "takeaway": "", "use": "",
           "thesis": ["wording 1", "wording 2"], "loops": [{"id": "L1", "q": "", "open": "B01", "close": "B04"}]},
 "voice": {"mode": "recorded|clone|design|preset|none", "ref": "", "instruct": "", "name": "", "engine": "", "speed": 1.0},
 "beats": [{"id": "B01", "role": "hook", "vo": "", "onscreen": "", "visual": "", "shots": [], "facts": [],
            "material": [], "art": [], "keys": [], "voice": "", "key_moment": false, "tempo": "", "device": ""}],
 "variants": [{"id": "H2", "vo": "", "onscreen": "", "visual": ""}],
 "material_override": "",
 "warn_notes": {"V2": "why this advice does not fit", "U1 s01[H2]": "a note for one place only"},
 "platforms": {"tiktok": {"canvas": "9x16", "caption": "", "sound": "original|trend:T1", "cover_text": ""},
               "x": {"post": "", "thread": [], "cover_text": "", "facts": [], "premium": false},
               "youtube": {"title": "", "description": "", "tags": [], "cover_text": ""},
               "youtube-long": {"canvas": "16x9", "title": "", "description": "", "tags": [], "thumbnail_text": ""}}}
```
- `hook` is the variant the video is made with: only it is voiced and produced.
- `shots` lists the visible changes planned inside a beat (a cut, a push, a
  reveal); the pacing check counts them, so a long spoken sentence stays one
  beat.
- `warn_notes` answers WARNs, keyed by code (`"V2"`) or by code and place as
  lint prints it (`"U1 s01[H2]"`).
- Words this audience already knows (a country, the title they search for)
  go in `campaign.known_words`, so the stranger checks (U1, U2) skip them; the
  segment's search terms and the brand are skipped already.
- The lint knows the roles hook, context, problem, tension, turn, demo, proof,
  apply, payoff, reaction, beat, cta and end. It uses them to time the first
  value, the apply beat and the CTA; another role only WARNs.
- `recipe` names the material a script stands on (`tools/mvflib/recipes.json`,
  plus extensions).
- A beat's `voice` speaks that line in another voice.
