# AX Markdown delivery

AX means Agent Experience, not a single file-format certification. For hosted
docs, follow the [llms.txt proposal](https://llmstxt.org/) and verify current
framework/server capabilities before implementation.

## Content and discovery

- Maintain one content source for human HTML and clean Markdown. Export text
  without navigation/app shells, preserving headings, examples and useful links.
  Rewrite relative destinations to reachable Markdown or pinned source URLs.
- A directory-style HTML URL `/docs/product/topic/` has a predictable Markdown
  counterpart `/docs/product/topic/index.md`. Retain prior `.md` URLs as identical
  aliases or redirects instead of breaking existing prompts and integrations.
- Publish `/llms.txt` at site root, linking to scoped product indexes. Each index
  is concise Markdown: H1, summary blockquote, H2 groups of links with descriptions.
  Link to actual agent-readable content; don't list unreleased products as usable.
- Advertise Markdown with `rel="alternate" type="text/markdown"` in HTML head
  or HTTP `Link`. Advertise the applicable index with `rel="describedby"` in
  HTML or HTTP `Link`, including on Markdown responses. Choose the index that
  covers that URL; localized paths need discovery within their own scope.

## Serving and framework boundaries

Return HTTP 200 and `Content-Type: text/markdown; charset=utf-8` for Markdown.
An unknown `.md` URL must remain a real 404, with the MIME matching its error body,
rather than a 200 app shell or an HTML error mislabeled Markdown.

Next.js serves generated files from `public/`; `alternates.types` metadata can
advertise Markdown. Static GET Route Handlers can emit files at build time, but
`output: 'export'` cannot inspect runtime request headers or use Next's runtime
header/rewrite layer. Keep serving headers in the static host/CDN for that setup.
See [Next.js static exports](https://nextjs.org/docs/app/guides/static-exports).

`Accept: text/markdown` negotiation is optional. Explicit Markdown URLs work
with static hosting and avoid HTML/Markdown cache variants. If negotiation is
requested, implement it in a supported runtime or edge layer, respect `Accept`
quality values, and verify both variants through the actual cache. `Vary: Accept`
alone is not proof that a CDN varies its cache key. Do not add a Next server merely
to serve documentation files. See [Cloudflare Markdown for Agents](https://developers.cloudflare.com/fundamentals/reference/markdown-for-agents/).

## Evidence

Check exported HTML discovery, index targets, Markdown links/anchors, aliases and
source consistency. Exercise the serving configuration with real HTTP requests:
status, body, MIME and Link headers, plus a missing-file failure. Name any untested
CDN or live-host behavior; local checks do not prove publication.
