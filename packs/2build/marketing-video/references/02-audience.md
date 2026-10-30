# Stage 2 - Target audience

Output: `02-audience/segments.json`. A segment shares a **pain in their own
words** and a **place where they scroll**. 1-3 segments, one `priority: 1`.
Everything later (angle, hook, search terms, pre-test personas) is written for it.

```json
{ "segments": [{ "id": "A1", "priority": 1, "name": "", "who": "concrete situation and what they already do",
  "pains": ["in their words"], "jobs": ["what they are trying to get done"], "objections": ["what they will comment"],
  "platforms": {"tiktok": {"search_terms": ["", "", ""], "habits": [""]}, "x": {"habits": [""]}},
  "evidence": [{"type": "url|file|user|measured|inference", "ref": "", "quote": ""}] }],
  "anti_segments": ["who will see it and must not be targeted"] }
```
- `who` is concrete ("indie devs shipping a SaaS alone past 20 customers"), not
  "developers 18-34". The hook is usually a pain line.
- `search_terms`: what they type into TikTok search; the caption's first line uses them.
- Evidence: forum and Reddit threads (use a search result that quotes the post
  if the site blocks fetchers), app-store and competitor reviews, TikTok
  comments read logged out, your own support tickets/analytics, the owner's
  knowledge of users (`user`). `inference` is allowed but a priority-1 segment
  may not rest on inference alone.

Gate (`mvf check audience`): every segment has at least one real voice; priority-1 has 3 search terms,
2 objections and 1 anti-segment; its pains read as something they would say.
