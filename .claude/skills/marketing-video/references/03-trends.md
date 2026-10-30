# Stage 3 - Trend research

Output: `03-trends/trends.json` plus reference frames in `03-trends/refs/`.
A trend is **observed, with a URL, a number and a date**. Formats (POV over a
real screen, "X be like", word-by-word captions) outlive hashtags and shape the
script; sounds and hashtags are short-lived and risky.

```json
{ "trends": [{ "id": "T1", "platform": "tiktok|x|cross", "kind": "format|meme|topic|hashtag|sound|creator|event|search",
  "stage": "rising|peak|fading|evergreen", "url": "", "metric": "412k views, ER 6.1%", "observed_at": "YYYY-MM-DD",
  "decision": "use|adapt|watch|avoid", "fit": {"segments": ["A1"], "how": "exactly how a script uses it"},
  "risk": {"license": ""} }] }
```
Freshness: a rising or peak trend older than 14 days is re-observed; older than
30 days with use/adapt is dropped. Per platform: at least 3 trends including one `format`.

## How to observe
- TikTok search logged out (`tiktok.com/search?q=<segment search term>`): top
  videos for the segment's terms; read views on cards, open the top ones.
- Per reference video record views, likes, comments, shares, sound, posted date;
  use any available downloader/metadata tool, or read the page. Cut the first
  3 s into frames (ffmpeg, 4 fps) when you can, to see what is on screen at 0.0,
  0.5 and 1 s: text? face? product? That is the hook structure to adapt.
- X search and Explore need a login: do not log in. Record X behaviour as
  `cross` formats plus patterns seen through web search or embeds, and say so.
- A third-party roundup is a lead, not a trend.
- Use a browser automation tool if you have one; never log in, like, follow or comment.

Engagement rate (engagement / views) beats views: a competitor ad with high
views and ER under 2% is paid reach, so `avoid` the format. A licensed pop
track is `avoid` for a business account (use the platform's commercial music
library at upload). Copying a video is not adapting a format.

Gate: per platform you can name 2-3 formats to use, the one to avoid and why,
each with a number and date; every use/adapt has `fit.how`.
