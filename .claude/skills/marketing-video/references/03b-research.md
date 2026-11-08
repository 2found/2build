# Stage 3.5 - Topic research and the idea bank

Output: `03-research/research.json`, with transcripts in `03-research/top/`.
A script written from memory repeats what every other video already said.
This stage finds what a stranger would stay for, and what they would take
away, before any angle exists.

## Bar
- **Sources were read.** Every source has an http(s) URL, `says` (what the
  page actually told you) and the date it was read.
- **The top videos were read, not just listed.** Each one in `top_videos` has
  its URL, its measured views, its `subject` (the viewer's problem it is
  about), what it `says` (the answer it gives) and a transcript. If none can
  be read, `top_videos_note` says why.
- **Questions and nuggets are sourced.** Each question is in the audience's
  words, and each nugget is a specific, checkable detail, both with sources
  that exist.
- **The ideas the scripts stand on are picked, and each stands on nuggets.**
  Pick one idea per candidate script (up to `budget.scripts`); the angles in
  [04](04-strategy.md) are built on them. Each pick names its `premise`, its
  `benefit` (lesson, decide, answer, match or how-to) and `use_when`, the
  moment in the viewer's own life it is for, in real sentences: a picked
  idea with filler fields (`"x"`, `"?"`, a few words) FAILs.

`mvf check research` FAILs on these. It WARNs on thin research, on a pick that
scores low, on more picks than scripts, on filler ideas, and on a pick that is
off the campaign benefit or off every pattern. Thin research means fewer than
8 sources from 3 sites with 2 community voices, 3 competitors, 5 questions, 8
nuggets, about twice as many ideas as scripts (at least 6), or 5 top videos (2
of them in the benefit scope). The counts are a floor for comparing, not a
quota: when the obvious ideas are found early, say so rather than pad the
bank.

## Direction
Search in every language the topic lives in, and read the community as well as
the official pages. Never log in to read more.

### What the top videos say
Hooks are half of a top video. The other half is what it gives the viewer, and
a writer who reads only the first 3 s misses it. Read the transcripts in two
scopes:
- **topic**: the most-viewed videos about this product or subject;
- **benefit**: the most-viewed videos that give this audience the same benefit,
  often about something else entirely.

Then write the `patterns` they share: the problem, the answer, the order.

The benefit audience is usually far bigger than the topic's fans. Kiếm Lai is
one example. Recaps of the novel drew a few hundred thousand views. Life-advice
videos on "how to handle people who look down on you" drew more, by using a
historical or fictional figure as the authority while the viewer stayed the
subject. A budgeting app's best reference may be a money-advice creator, not
another app demo. A campaign that reads only its own topic writes a fan recap.

### Reach
Write each premise once in the words of someone who has never heard of the
topic: no title, no hero's or product name, no jargon. If it still makes them
need the answer, it reaches. If it only works once the world or the product is
explained, it is an insider video, and its `reach` score says so. List the
insider words in `lore_terms`; the lint flags them in hooks and early beats.

### Ideas
- **Benefit first.** Score `use`: something the viewer can do or think the
  next time it happens to them; for an `answer`, how fully the evidence
  settles the question as the audience asked it. A good story, an explainer or a ranking with
  nothing to keep is how a video holds attention, not what it gives. Two
  story scripts that every agent panel passed were rejected for exactly this.
- **Shapes that tend to work:**
  - a principle for a moment in the viewer's life, with the subject as the
    proof;
  - an answer to a question the audience already argues about, with
    evidence;
  - help choosing (who it is for, who should skip it);
  - a misread corrected;
  - one specific detail nobody retells.
- **Shapes that tend to bore:**
  - a system or feature explained in order, like a wiki: names and parts
    with no question pulling them (explaining is fine; a list without a
    reason is not);
  - a top-N list with a line per item;
  - a backstory that needs the world's rules before anyone cares;
  - the product's feature as the topic.
- **Existing depictions.** If the subject already has a look viewers carry in
  their heads (an animation, a drama, a comic, a game, a mascot), list it in
  `adaptations`; generated art follows it.

## Tool
- `mvf top-videos "QUERY" [--platform youtube|tiktok] [--n 6] [--seconds 300] [--max-min 30] [--scope topic|benefit]`
  searches YouTube logged out. It drops results whose title shares no search
  word and results longer than `--max-min` (compilations, livestreams), and
  keeps the first `--seconds` of subtitles (0 = all) in the campaign's
  languages, else English. A video that cannot be read (HTTP 429, removed) is
  skipped and listed for a retry; the rest still land.
  `mvf top-videos --url URL… --scope benefit` reads videos you found by hand.
  Either way it writes `top/<id>.txt` and `top/<id>.json`, most-viewed first.
  It does what it can and prints the rest as `AGENT:` lines, which are also
  kept in the row's `agent` list. Do them before `mvf check research`:
  - `--platform tiktok "QUERY"` prints the `browse` steps: open `/search?q=`
    and `/tag/<tag>`, and collect each post's link and card text at once
    (the search page swaps its results a few seconds after loading). Then
    pass the links with `--url`. mvf cannot search TikTok itself. It
    downloads each TikTok video and reads 4 at a time, so run a long list in
    the background.
  - When a video has no subtitles (TikTok never has them), mvf cuts
    `top/<id>/read-*.jpg`. These are the whole video in order, large enough to
    read the on-screen text. Write what it shows and says into `top/<id>.txt`,
    which `top_videos[].transcript` names; a rerun keeps what you wrote.
    Speech recognition is not used, because it invents speech over music.
  - TikTok comments need `browse`: open the video from its list and read them.
    Keep the ones you use with `mvf gather manual`, as kind `post` (id
    `tiktok:post:<key>`) and rights `unknown`.
  - If TikTok will not open, say so in `top_videos_note`.
- `mvf trend-video URL… [--frames]` reads a competitor's metadata and its hook
  frames.
- `research.json` keys:
  - `topic`
  - `sources: [{id, kind, url, title, lang, fetched_at, says, signal}]`. `kind`
    is a community one (forum, reddit, comments, review, group, qa) or wiki,
    news, official, book, video, interview, article or data.
  - `competitors: [{url, views, posted, hook, what, top_comments, gap}]`,
    `competitors_note`, `covered`
  - `top_videos: [{url, views, scope, title, subject, says, moves, transcript}]`,
    `top_videos_note`, `patterns: [{id, what, videos}]`
  - `adaptations: [{title, kind, url, year}]` (`[]` when there are none),
    `lore_terms`
  - `questions: [{q, sources, heat}]`
  - `nuggets: [{id, type, text, sources, where}]`. `sources` are source ids or
    01-product fact ids. `type` is fact, scene, quote,
    number, controversy, theory, behind-the-scenes, connection or mistake.
  - `ideas: [{id, premise, question, payoff, benefit, use_when, pattern, nuggets, new, scores, pick}]`
    with `scores` 1-5 for curiosity, use, emotion, share, novelty, proof and
    reach. A strong pick totals 26 or more with nothing under 3 and `use` of
    at least 4.
- Angles (04) build on a picked idea: `idea: "I3"`.
