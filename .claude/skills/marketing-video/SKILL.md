---
name: marketing-video
description: Plan and produce marketing videos for your own product on TikTok, YouTube (Shorts and long-form), Instagram and Facebook Reels, X and LinkedIn, for any product and any kind of video, from research to a script strangers can retell, art, voice, edit and per-platform files; you post them.
disable-model-invocation: true
---
# marketing-video
A founder or indie dev with no marketing team gets a platform-ready video, made
by an agent. This is a manual stopgap until 2found's marketing product (2market)
ships. Follow the [preamble](../references/preamble.md) for bootstrap and
telemetry. Shared refs are filesystem paths beside this skill's directory, so
read them by path, not as `skill://`.

It works for any product: an app, a SaaS, a game, a book, a physical product,
a service, a course or a creator. It also works for any kind of video: a demo,
a tutorial, a story, an explainer, a testimonial, a comparison, a list, a skit
or a long-form essay.

**This skill sets the standards and the direction; the craft is yours.** It says
what a video must achieve and what must never be broken, and passes on lessons
from past campaigns. It does not prescribe a structure, a shot list, a look or
an edit. Invent those for the product, the audience and the platform, and use
whatever tool does it best.

**You own every step; `mvf` assists.** Its presets, recipes, effects and rig are
shortcuts, and each has a raw form beside it: raw HTML or SVG plates and effects,
raw GSAP tweens, hand-authored scenes and captions, extra CSS. Use those, or
another tool entirely, whenever the shortcut is not what the video needs. A
check FAILs only on a standard, or on something that would break the render,
the platform or the timing. Taste is WARN advice that you answer in writing.

**The default look is the stickman comic.** It is set by `"style": "stickman"`
in `campaign.json`: chibi characters drawn in code, a bold ink outline,
hand-lettered labels and boxed captions on light paper
([07 Production](references/07-production.md#layered-art-25d)). Pick another
preset, or `"none"`, when the product or the audience calls for it.

One campaign is one folder, `marketing/campaigns/<slug>/` by default. Each stage
leaves one artifact on disk, so the work survives a crash and can be audited.

## How it runs
The owner touches a campaign three times, and the agent does everything else.
1. **The brief.** What to promote, the goal, and where it goes (platforms,
   language); the viewer benefit only if the owner names one. Ask once, only for what the request leaves out,
   and write every other assumption into `campaign.json`.
2. **The script.** The owner reads the winning script's page (`mvf sheet
   <sid>`) and says yes, no with a reason, or "you decide" (`mvf approve`).
   This happens on the campaign's first video; nothing is voiced before it.
3. **The cut.** The owner watches the delivered files and posts them.

Once the owner approves the script, offer to go on in a new chat: everything
lives on disk and `mvf next` picks up from it, while this chat carries the
whole research in its context. Continue here if the owner prefers.

Between those points the agent loops on **`mvf next`**, which names the one
next action and says whether the agent or the owner takes it. The agent does
the action, runs its check, and asks again. `mvf status` shows the whole
board. A FAIL blocks; a WARN is advice to follow or answer in writing.
An `AGENT:` line is a step mvf cannot do, such as a page that needs a
browser or a video it cannot read. Do it yourself before that stage's check.

### Backbones
| Job | Engine | First use |
| --- | --- | --- |
| Edit and render | HyperFrames (HTML to video, `npx hyperframes`, Node 22+) | the first scaffold downloads the pinned version |
| Voice | OmniVoice (clone or designed voice; Whisper times the words) | uv builds its environment on the first take (about 2 GB, once) |
| Pictures | Qwen Image 2.1 Turbo, and RMBG-1.4 cutouts (the `image-gen` skill) | `image-gen/scripts/setup` downloads the weights once; free and local |
| Judge | Micro Fish on the user's Soot: a simulated crowd and an expert panel | the user signs in to a provider and names the model, then `mvf judge setup` |

Run **`mvf doctor`** before the first video. It checks each backbone and
prints the fix for anything missing. The agent fixes what it can; `NEEDS`
lines are the user's to do:
- **Signing in to an AI provider for Soot.** Ask the user to run
  `! soot login <provider>`. Never ask for, paste or write a key.
- **Naming the judge model on first use.** Ask once, offer the providers that
  `mvf judge status` lists as signed in, then run `mvf judge setup --provider
  P --model M`.

Other engines plug in as extensions, but these four are the defaults the
checks expect.

## Stages
| # | Stage -> artifact | The bar it clears | Reference |
| --- | --- | --- | --- |
| 0 | brief -> `campaign.json`, material -> `00-material/` | goal and KPI, the viewer `benefit` only when the owner names one, brand, platforms, constraints; what people already like or know about the product | [00](references/00-brief-material.md) |
| 1 | product -> `01-product/facts.json` | every claim the video may make, sourced and dated; real screens | [01](references/01-product.md) |
| 2 | audience -> `02-audience/segments.json` | who it is for, by need, from evidence, in their words | [02](references/02-audience.md) |
| 3 | trends -> `03-trends/trends.json` | what the platform rewards this fortnight, observed | [03](references/03-trends.md) |
| 3.5 | research -> `03-research/research.json` | what the audience asks, what the most-viewed videos say, the specific details; one idea picked per candidate script, each built on something from inside the subject and ranked by what the audience already watched | [03b](references/03b-research.md) |
| 4 | strategy -> `04-strategy/angles.json` | 2-3 angles, each a benefit for a segment, standing on facts, checked fresh against the obvious videos and this owner's past ones | [04](references/04-strategy.md) |
| 5 | script -> `05-scripts/<sid>.json` | 3 candidate scripts, each delivering its benefit early and in plain words, complete enough that a stranger can retell it, written from its ending, with 2-4 key moments, hook variants and platform copy; the kind of video and its channels from the [atlas](references/formats.md) | [05](references/05-script.md) |
| 6 | pre-test -> `06-pretest/<round>/decision.json` | Micro Fish (or another honest test) picks one script and one hook, and strangers can retell what it told them; the owner reads the winner before production | [06](references/06-pretest.md) |
| 7 | video -> `07-video/<sid>/` | one video: real product, approved art, measured audio, reviewed by looking, by a cold viewer who never read the plan, and judged by a simulated panel | [07](references/07-production.md), [craft](references/07-craft.md) |
| 8 | delivery -> `08-delivery/<sid>/<canvas>/<platform>/` | the platform's specs, QA, captions, copy, AI labels | [08](references/08-delivery.md) |

Open only the reference for the stage you are in, and only the sections you
need: `rg -n '^## ' references/<file>.md` lists them, and the Bar comes first.
Stages 1-4 are research, so a
guess is written down as `inference`, never as a fact. Go back a stage whenever
the evidence says so. Run only the stages the user needs, and say where you
stopped.

## Standards (FAIL in the checks; breaking them costs people)
- **Something from inside worth retelling, and a use.** A stranger stays for
  something from inside the subject: a scene, a twist, a line, a rule of its
  world, a fact they did not know. They save and send the video for what it
  does for them: something to apply, decide, understand or do. A video needs
  both. A story with nothing to keep fails the second half. A guide about the
  product with nothing from inside it fails the first. Facts about the
  product's packaging (its length, price, size, where to get it)
  are neither, so give them one line at most. Each angle picks its `benefit`;
  the campaign sets one only when the owner asks. Every script says in plain words
  what the viewer can use (`spine.use`). For an `answer` the use *is* the
  answer, in the topic's own words; only a lesson, a decision, a match or a
  how-to is turned into advice about the viewer's life.
- **Understood in one hearing, and complete.** A stranger who hears the video
  once, at speaking pace, can retell what it told them in a sentence or two,
  and that retell is the answer the hook promised, not the question again, a
  slogan or "it depends". Every term the point needs gets its plain meaning
  the first time it is heard; a term it does not need is cut, never kept
  unexplained. A conditional answer names the condition and gives each
  result. A short video that leaves the viewer asking "so what is it?" has
  given nothing, however well it opens. Length follows what the viewer needs
  to follow: cut repetition and asides, never a step of the explanation.
- **Research before writing.** Read what the audience says and what the
  most-viewed videos on the topic, and for the benefit, actually tell their
  viewers, not only how they open. Ideas come from that evidence, not from
  memory, and are ranked by it: the views of the top videos that carry the
  same kind of payload, never the writer's own scores. Never remake what an
  earlier campaign said; `mvf lint` FAILs a script whose lines repeat one.
- **Claims stand on facts.** Every quantity, price, "free", superlative or
  guarantee in a beat or in the copy names a fact id. Re-read price and limits
  before every export (re-check any older than 30 days).
- **Real product on screen.** UI is a captured screenshot or recording, never a
  drawn mock-up. A missing screen is a labelled placeholder and a report line.
- **The first seconds earn the rest.** Most viewers decide within 3 s. The
  spoken hook ends near the platform's limit (2.5 s on TikTok and Shorts,
  2.0 s on X; more than 1 s over fails), X needs on-screen text because it
  autoplays muted, and frame 0 shows something. The first real value lands
  early (about 6 s for a short, 15 s for long-form; more than 2 s late fails),
  and the payoff keeps the hook's promise.
- **Measured, not hand-typed.** Timing, loudness and specs come from the
  audio and the rendered file. Estimates are labelled.
- **Rights.** Show only `own`, `licensed`, `public-domain` or `official-promo`
  material (brief, credited, with your own commentary). `unknown` may inform
  the script but never appears on screen. Never bake a licensed track into the
  file; trend sounds are added in the app.
- **No AI likeness of a real person**, and never prompt an image model with a
  real person's name. Generated art is never the product UI, a logo or words.
  Only approved art is shown. Realistic AI picture or voice gets the
  platform's AI label.
- **Honest testing.** A pre-test records its true method: Micro Fish is a
  simulated panel, never "users", and agent readers are labelled as such. The
  winner beats a control written to lose, and the panel's retell, quoted,
  matches the script's takeaway. The owner reads the first script of a
  campaign before any production.
- **One video at a time.** Write 3 candidate scripts, then pick one script and
  one hook and produce only that one. The machine does not have the resources
  for more.

## Direction (lessons, not rules)
- **Start from what people already like or know** about the product, not from
  what the product wants to say.
- **Speak to people who have never heard of it.** Use their words and their
  situation first; insider names and jargon come later, for the fans. Making
  it reach means explaining a word, not avoiding it: a script that dodges the
  terms but still leans on them ("the lower five", "level three") loses
  strangers and fans at once.
- **Short is not thin.** The checks measure the hook, the timing and the
  jargon; none of them can tell that a script said almost nothing. A script
  revised round after round to pass them shrank to a hook, two quotes and
  "both sides are half right", and the owner could not tell what it was
  about. Before any pre-test, write the retell a stranger should give, and
  check that the body actually says it.
- **Draw what the audience already pictures.** If the product, character or
  world has a known look (an adaptation, a mascot, brand art), generated art
  follows it.
- **Agent readers judge entertainment, not benefit.** Two scripts that every
  agent panel passed were rejected by the owner because they gave the viewer
  nothing to use. Ask readers what they would use, save or send, and make
  them retell what the video told them: a reader who read the script as text
  can echo its lines without having understood them. They are also polite
  about boredom, so ask what they would tell a friend (`tell_a_friend`). If
  most readers say "nothing", the script bores, however useful it is.
- **Brand in the picture, not in the pitch.** Show the product where the value
  happens. Keep its name out of the hook, speak it once at most, and save
  selling words for one soft CTA: most viewers never reach the end, and a
  pitch reads as an ad.
- **Every angle names who a viewer would send it to, and why.**
- Each reference gives its stage's bar first, then the lessons. Treat the
  lessons as evidence to weigh. Where the advice and your judgement differ,
  follow your judgement and note why.

## Tool
`mvf` is a stdlib Python CLI. Run it as
`PYTHONDONTWRITEBYTECODE=1 python3 <this-skill-dir>/tools/mvf.py <cmd>` from
inside a campaign.
- It needs python3 and ffmpeg; `yt-dlp` for video references and transcripts;
  `mvf doctor` checks the rest.
- `mvf init <slug> --product NAME` writes the folder and its `.gitignore`.
  For the next episode of a series, `mvf format save <name> --from <sid>`
  keeps the last one's look, captions, ambience, rigs, props and voice, and
  `mvf init <slug> --product NAME --format <name>` starts from them.
  `mvf next` says what to do, `mvf status` shows the board, and `mvf --help`
  lists every command.
- Pass `--campaign <dir>` (before the command) whenever more than one
  campaign is open or the shell's directory may move, such as parallel
  agents sharing a shell; otherwise `mvf` writes into the campaign that
  contains the current directory.
- The checks (`mvf check <stage>`, `mvf lint`, `mvf qa`) report two levels:
  **FAIL** is a broken standard and blocks the next stage; **WARN** is advice,
  so follow it or write down why not (a script's `warn_notes`). The checks
  count and match words; they cannot tell whether a script is clear. Never
  bend a script to quiet a WARN that is wrong for it: answer it.
- The voice, art, scaffold, direction, sound and export helpers are one way to
  build the video, not the only one. Any route that meets the standards is fine.
- Local engines, data sources and recipes are gitignored extensions
  ([contract](references/07-production.md#extensions)).
- Fresh helpers (`mvf predict`, `mvf overlap`, `mvf coldread`, `mvf viewer`)
  get a brief with only what their role may see, a budget of 15 tool calls,
  and a short reply; the details go to a file.

The cold viewer, the prediction and overlap tests, the writing tells and the
motion numbers adapt ideas from [cinecode](https://github.com/sarpstar/cinecode)
(MIT); no code was copied.

## Boundary
Never log in to, upload to or post on any platform, and never spend money (ads,
boosts, paid APIs) without the user's explicit say. Delivery ends at local
files plus caption and post text; the user posts. Keep secrets out of campaign
files, and read them from env vars only. Observe platforms logged out (pages
that need a real browser, such as TikTok, through `browse`), and do not like,
follow or comment.

## Report
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
CAMPAIGN: <folder> | STAGES: <run / skipped and why>
FILES: <delivered paths with duration, size, loudness, QA verdict>
OPEN: <unverified trends, placeholder screens, untested hooks, what the user must watch and post>
```
