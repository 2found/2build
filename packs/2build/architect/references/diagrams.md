# Diagrams
Mermaid inside the Markdown documents is the source of truth: it renders on
GitHub and in most editors, and it diffs.

## The set and where each lives
| diagram | shows | current home | next home |
|---|---|---|---|
| Context | people, the system as one box, external services | system document, Components | — |
| Containers | deployable units, stores, caches, queues, edge; edges say protocol, sync or async, what flows | system document, Components (replaces its diagram) | next-gen document |
| Deployment | hosts, regions, managed services and sizes | the deployment runbook the system document links | next-gen document |
| Sequence | the hottest or riskiest path through every hop, with cache decisions | the owner of that path (service document or caching doc) | next-gen document, only when the path changes |

A home that does not exist yet: create the system document from the
[skeleton](../setup-project/references/project-docs.md#skeleton-for-a-missing-document);
for the others, keep the diagram in the next-gen document under "Current" and
name the missing owner as a finding.

## Shape
- Owner-facing diagrams (context, the next-gen containers) keep to about 12
  nodes; engineer-facing ones to about 15. Split by boundary instead of
  cramming.
- `flowchart LR` up to 8 nodes, `flowchart TB` above; `sequenceDiagram` for
  paths. Re-render as TB or split when the image is wider than 3:1.
- Every node has at least one edge; a node with none belongs in the caption.
- Stores as cylinders `db[("Postgres 16")]`, people as stadiums
  `user(["Reader"])`, hosts and trust boundaries as `subgraph`s titled with
  host and size (`subgraph vm["GCE e2-standard-2"]`).
- Quote every label with punctuation, parentheses or a colon: unquoted
  `[(Redis) - new]` is a parse error.
- Edge labels say what flows: `-->|"REST, JSON"|`, `-.->|"async job"|`.
- Next-gen marks change in the **label first** (survives greyscale and any
  renderer), then in style, attached with `class`:
  ```text
  redis[("Redis (new)")]
  classDef new stroke-dasharray: 5 5
  classDef changed stroke-width: 3px
  classDef removed stroke-dasharray: 2 2, opacity: 0.5
  class redis new
  ```
- Under every diagram, one or two plain sentences: what it shows and what to
  notice, for a reader who does not read diagrams.

## Render check
Render every diagram and look at each image before reporting; a diagram that
does not parse, or whose labels overlap, is not done.
```sh
npx -y @mermaid-js/mermaid-cli@12.0.0 -i <doc>.md -o <out-dir>/<doc>.md -e png -s 2
```
Writes `<doc>-<n>.png` per Mermaid block to the out dir (scratch or report
dir, never the repo) and fails on the first parse error with its line. As
root or in CI, Chromium needs `-p <file>` with
`{"args": ["--no-sandbox"]}`. The first run downloads a headless browser;
without network report `NOT RENDERED` and the reason as a concern. Commit
rendered images only when the project already keeps them.
