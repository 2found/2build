# Optional DESIGN.md token format
Use only when the project wants `bbs design tokens` to read its design doc.
The command reads YAML frontmatter; existing prose-only docs remain valid
project authorities. Pass `--design <path>` for a non-root document.

```yaml
---
schema: babysit-design/v1
project: <project name>
tokens:
  colors:
    primary: <existing value or CSS variable>
  typography:
    body: { family: <project font> }
  spacing: { base: <project spacing unit> }
  layout: { radius: <project radius> }
---
```
Include only fields useful to this project, with values traceable to its source
styles or chosen new design. Reference the owning token/component files rather
than maintaining an exhaustive duplicate inventory. No prescribed prose sections.
