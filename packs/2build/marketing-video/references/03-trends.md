# Stage 3 - Trends

Output: `03-trends/trends.json`, with reference frames in `03-trends/refs/`.

## Bar
A trend is observed, not assumed. It records the platform, a kind, a stage, a
decision, the URL where it was seen and the date it was seen. A trend you will
`use` or `adapt` also says how a script uses it (`fit.how`) and which segment
it reaches. A rising or peak trend older than 30 days that you will use is
re-observed or dropped.

`mvf check trends` FAILs on these. It WARNs when:
- a trend has no metric;
- a trend is more than 14 days old;
- a platform has fewer than 3 trends or no `format`;
- every row for a platform is `method: inference` (nothing was observed there);
- a TikTok sound is not cleared for a business account;
- there are no reference videos.

## Direction
- **Formats outlive hashtags.** A format (a POV over a real screen, "X be
  like", word-by-word captions) shapes the script. Sounds and hashtags are
  short-lived and risky.
- **Engagement rate beats views.** High views with an engagement rate under 2%
  is usually paid reach, so `avoid` that format.
- **A licensed pop track is `avoid`** for a business account. Add the sound in
  the app at upload, from the commercial library.
- Adapt a format; never copy a video. A third-party roundup is a lead, not a
  trend.
- **Observe logged out.** X search needs a login, so record X behaviour as
  `cross` patterns seen through web search or embeds, and say so. Never like,
  follow or comment.
- For a long-form reference, the title and the thumbnail carry the hook.
- **Read TikTok through the `browse` skill** (`../browse/skill.md`: headed
  agent-browser on cloakbrowser, logged out, its session export and cleanup).
  Checked 2026-11-08:
  - These pages list `/video/` links with their caption and creator:
    - `/search?q=<words>` (the Top tab; results drift off topic);
    - `/tag/<tag>`;
    - `/explore`.
  - These pages fail:
    - `/search/video?q=` errors;
    - a video URL loaded directly errors too.
  - To open a video, click it from a list. The page then shows:
    - `[data-e2e=browse-video-desc]`;
    - `browse-like-count`;
    - `browse-comment-count`;
    - the comments, in `comment-level-1`.
  - The grid shows no views. Collect the URLs and run `mvf trend-video URL…`
    or `mvf top-videos --url URL…`. `yt-dlp` reads one TikTok video's views,
    likes and duration. It finds no subtitles or comments; `mvf top-videos`
    prints `AGENT:` lines for those.
    `mvf top-videos --platform tiktok "QUERY"` prints these steps.
- **When a platform stays closed** after that, say so instead of filling
  rows. Write what you could not see in `observed_with`, and mark a row you
  did not see as `method: inference`.

## Tool
- `trends.json` holds `trends: [{id: "T1", platform: <campaign platform>|cross, method: observed (the default)|inference, kind: format|meme|topic|hashtag|sound|creator|event|search, stage: rising|peak|fading|evergreen, decision: use|adapt|watch|avoid, url, metric, observed_at, fit: {segments, how}, risk: {license}}]` and `references`.
- `mvf trend-video URL… [--frames] [--note TEXT]` reads a post's public
  metadata (views, engagement, sound, duration) into `references`. With
  `--frames` it also cuts the first seconds into a hook sheet. It needs
  `yt-dlp`.
