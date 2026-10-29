# 2build

**Product engineering.**

2build supports the work of building a product:

```text
plan → implement → review → test → QA → release
```

It provides engineering workflows, skills and a companion CLI. Coding agents
can carry out those workflows; the product is defined by the engineering work
and its verified outcomes.

**A 2found product.** See the ecosystem branding in
[`kiem-lai/BRANDING.md`](https://github.com/reallongnguyen/novel-reader/blob/main/BRANDING.md).

| Surface | Name |
| --- | --- |
| Product | `2build` |
| Repository | `2found/2build` |
| CLI | `bbs` |
| Skill/plugin namespace | `bbs` |
| Soot pack | `2build` |
| One-ticket workflow | **Autopilot** (`/bbs:autopilot`) |
| Project workflow | **Team** (`/bbs:team`, `bbs team`) |

Write `2build` in lowercase. Use `https://github.com/2found/2build` for repository,
installation and release links. No separate product domain or persona is defined.

The previous product name was **babysit**. Existing installations keep the
`babysit` marketplace ID (`bbs@babysit`), `.babysit` state/config directories,
`BABYSIT_*` environment variables and existing API field names for compatibility.
These are technical identifiers; use **2build** in product copy and display names.

**Team** is an autonomous product team: the human meets it at the **brief**
(requirement, spec, basic design) and the **handover** (product review,
release go), and it does everything between. Call its orchestrating session
the **lead** (or team lead) and its sessions **workers**; never "foreman",
"manager" or "outsourcing". Team was previously named **Foreman**:
`/bbs:foreman`, `bbs foreman`, `--foreman <id>`, `BABYSIT_FOREMAN`,
`FOREMAN_*` variables, the `foreman` settings key, `~/.babysit/foremen/`,
`foreman_*` config keys and the `foreman` JSON fields stay as compatibility
identifiers. Use **Team** in product copy and display names.
