# Architecture and release documents

Setup leaves every repo with a reachable architecture owner per service and a
release owner. When the facts are not known yet, it writes the structure with
placeholders instead of skipping the file or inventing content.

## Placeholders
A placeholder is one line, greppable across repos:
```md
TODO(2build): <missing fact> — <where it will come from or who decides>
```
Use it for a fact the repository cannot answer yet (no deploy target chosen, no
version policy). Never use it for a fact that a file, script or CI config
already answers; read and record that instead. A placeholder is an open gap:
consumers (`plan-draft`, `release-prd`, `harness-audit`) treat it as unresolved,
never as permission or a default. `N/A — <reason>` is a decided answer, not a
placeholder.

## Discover the existing owners
Before creating anything, search the repo (excluding dependency, vendor and
generated trees) for architecture documents in every service, not only at the
root: `ARCHITECT*.md`, `ARCHITECTURE*.md`, `docs/architecture*.md`, nested
`AGENTS.md`/`CLAUDE.md` that link one, and maintained sections of a README.
Starters often ship one per service (e.g. `api/docs/ARCHITECTURE-API.md`,
`web/docs/ARCHITECT-WEB.md`). Reuse what exists; create only what is missing.
Release owners: the profile linked from the entrypoint, otherwise
`.babysit/release.md`, `RELEASE.md` or a maintained inline section.

## Which architecture documents
| Repo shape | Owners |
| --- | --- |
| Single service / library / CLI | One architecture document for the repo |
| Several services or apps | One per service (internals) **plus** one system document (what sits between them) |
| Polyrepo member | Its own document; cross-repo contracts in the system owner named by the workspace, linked, not copied |

### System document — what sits between services
Keep it about boundaries, so it stays short and stays true:

1. **Sources** — a "need → read" table pointing at each service document,
   database/schema doc, content/data pipeline, deployment runbook, release
   profile and design system that exist.
2. **Fixed constraints** — stack, infrastructure, domains, identity provider.
3. **Components** — one diagram and a one-line role per top-level directory.
4. **Boundaries and contracts** — who calls whom, over what, who owns data and
   auth, which service alone touches the database.
5. **Invariants** — numbered, testable rules every change must keep
   (e.g. "the paper never contains answers"; "public pages never read cookies").
6. **Key decisions** — the decision and the one-line reason, for choices that
   are costly to reverse.
7. **Open risks** — only unresolved items.

Keep out of it, and link the owner instead: service internals (layout, layering,
error format), table/column detail, deploy commands and env/secret values,
content-file naming conventions, phase roadmaps and plans, resolved
history/changelog of decisions, dated measurements, credentials and account IDs.
Those drift fastest and already have owners.

### Service document — inside one service
Layout and module boundaries, layering/dependency rules, how to add a route/
module/page, configuration and env ownership, data access and external services,
error/logging conventions, and where its verification lives. Product decisions
that span services belong to the system document; link up, don't restate.

### Skeleton for a missing document
Create it at the conventional path for the repo (`docs/architecture.md` for the
system; `<service>/docs/ARCHITECTURE.md` for a service when no convention
exists), fill every heading the repository already answers, and leave one
`TODO(2build):` line under each heading it does not:
```md
# <Name> — Architecture

## Sources
| Need | Read |
| --- | --- |
| <service> internals | [<path>](<path>) |

## Fixed constraints
TODO(2build): hosting target — decided at first deploy

## Components
...
```
Use the headings above for the document's role. Do not add prose to make it
look complete.

## Release profile
Fields are [release-prd's profile contract](../release-prd/references/project-profile.md).
For a releasable app, CLI, library or pack with no owner, create `RELEASE.md`
with one row per field: the observed fact (version files, scripts, CI,
runbooks), `N/A — <reason>`, or `TODO(2build):`. A greenfield repo that will be
released gets the same file, mostly placeholders. Separate independently
released components; distinguish the normal pipeline from the local path.
Only a repo with no release surface at all (fixture, notes, internal-only tool
never distributed) records `Release: N/A — <reason>` inline in the entrypoint
instead of a file.

## Audit the owners
Run on every setup, for existing and newly created files, and report results in
`VERIFY`:

- **Reachability** — each service's architecture document and the release owner
  are linked from the root entrypoint with when to read them; nested entrypoints
  link up to the system document.
- **Coverage** — every service directory has an owner; the system document
  links every service document that exists.
- **Scope** — the system document holds the sections above and none of the
  excluded content; duplicated detail (schema, deploy env, conventions) that
  another owner holds is a finding with the owner to move it to.
- **Truth** — paths, commands, ports, versions and deploy facts match the
  repository; a document claiming "no deploy target" while `deploy/` exists is
  drift.
- **Gaps** — list every `TODO(2build):` left, per file.

Setup fixes links, adds missing owners and missing headings with placeholders,
and corrects factual drift it has verified. Trimming or moving existing content
between owners is reported as a finding with the target owner, and done only
when the user asks for it.
