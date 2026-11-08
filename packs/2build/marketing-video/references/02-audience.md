# Stage 2 - Audience

Output: `02-audience/segments.json`. Write 1-3 segments, with one at
`priority: 1`. The angles, hooks, search terms and pre-test readers that come
later are all written for these segments.

## Bar
- Every segment has a name, a `who`, `pains` and `jobs` in its own words, an
  integer priority, and evidence: a review, a comment, a support ticket, a
  metric or the user's words.
- The priority-1 segment stands on at least one real voice, not on inference
  alone.
- A reach goal (awareness, follow or launch) has at least one segment past
  `core`.

`mvf check audience` FAILs on any of these. It WARNs when a segment has no
ring, no objections, no platform habits, or no search terms for TikTok or
YouTube.

## Direction
- **Segment by need, not by demographics.** Write "people looked down on at
  work", "solo founders past 20 customers" or "parents choosing a first
  tablet", not "18-34". A need has its own searches and its own most-viewed
  videos ([03b](03b-research.md#what-the-top-videos-say)), and it is what the
  campaign benefit answers. A segment with no need has no reason to stay.
- **Pains in their words.** The hook is usually a pain line. Write the search
  terms the segment really types. A broad segment does not type the product's
  name.
- **Objections** are the comments the segment will write. A script that
  answers one early earns trust.
- **Anti-segments** are people who will see the video but are not the target.
  Expect their comments anyway.

### Reach rings
The feed shows a video first to people who have never heard of the topic. A
video that only insiders understand stays with the insiders.

| Ring | Knows | Comes in through |
| --- | --- | --- |
| `broad` | nothing about the topic | an everyday situation: money, work, family, being cheated or underestimated, a choice to argue about, a laugh |
| `genre` | the kind of thing (the category, the genre), not this one | a trope or a habit of the category, turned over |
| `core` | the product or the topic itself | a detail they missed, said exactly |

The broad ring leaves at the first insider word or name it does not know.
Write for it in plain words, and keep the jargon for the inner rings.

## Tool
`segments.json` keys:
- `segments: [{id: "A1", priority, ring: broad|genre|core, knows, name, who, pains, jobs, objections, platforms: {tiktok|x|youtube: {habits, search_terms}}, evidence: [{type: url|file|user|measured|inference, ref, quote}]}]`
- `anti_segments`

The template also offers `triggers` and `language` for your own use. The checks
do not read them.
