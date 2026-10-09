# Project pointers and workspace registration

## Project context links
`AGENTS.md` is the project's navigation map. Before adding tooling pointers,
link its project-specific sources of truth with a short note on when to read:

- **Architecture — required:** component/service boundaries, dependencies,
  data flow and engineering constraints. Use the repo's actual architecture
  document(s), whether named `ARCHITECT*.md`, `ARCHITECTURE*.md` or a maintained
  section of another document. In a multi-service repo, map each service to the
  relevant document instead of presenting an undifferentiated list.
- **Design — when applicable:** UI/UX rules, design system, tokens and shared
  components for projects with a visual/user-facing surface. Link the existing
  design authority and say to read it before visual/UI changes.
- **Deployment — when applicable:** environments, deploy/release procedure,
  prerequisites and rollback for deployable services or sites. Link the owning
  runbook and say to read it before deployment work. A library with no deployed
  service does not need an invented deployment document.

Use real relative Markdown links and existing filenames/anchors; verify the
linked content covers the claimed purpose. Reuse existing service lists or
sections rather than adding a second index. If the harness uses `CLAUDE.md` as
its entrypoint, keep the same map reachable there without duplicating policy.
Missing architecture guidance, or applicable design/deployment guidance, is a
named documentation gap: report the missing owner/content and next step. Do not
insert dangling links, create empty templates or invent operational details to
make setup look complete. Drafting the missing documents is separate scoped work.

## 2build pointers
Add one concise pointer section in `AGENTS.md` or `CLAUDE.md`. Adapt skill
invocations to the active harness using the preamble's `SKILL_REF`; the template
below uses Claude Code syntax. Omit the browser bullet for CLI/library repos
and point at their actual check commands if no `qa.yaml` exists:
```md
## 2build

This repo is configured for 2build autonomous runs.

- Git policy: `.babysit/git-flow.yaml`
- QA harness: `.babysit/qa.yaml`
- Browser: for any UI check — open a URL, click a flow, read console errors, screenshot — invoke `/bbs:browse` (or `/bbs:qa` for a full loop). These drive a real Chromium via `agent-browser`; there is no separate browser *tool* to look for, and `WebFetch` is not a substitute. One-time: `npm install -g agent-browser cloakbrowser`.
- Default run: `/goal "STATUS: DONE or STATUS: BLOCKED appears" /bbs:autopilot "<task>"`

QA must prove the local target or name the blocker, and must include at least one non-happy-path case before PASS.
```
Reuse existing 2build/Babysit pointers. Edit only those bullets; a section named
`## 2build` may also contain unrelated project instructions, so do not replace
it wholesale. Do not install browser tooling during setup.
When related repos exist or the user provides them, also add or update this
section:
```md
## Related Repos

Use these repos for investigation and planning when a task crosses FE/BE,
API contracts, generated types, or shared schemas. Local paths are machine
specific: they live in the workspace mapping in `~/.babysit/config.yaml`
(`bbs config workspace show`), which is the authority. `$RELATED_*_REPO` in
`.babysit/.env` is a fallback for repos outside a workspace.

- Backend API: role `be`
- Frontend app: role `fe`
- Shared package: role `shared`
```
Include only repos that apply. If a `## Related Repos` section already exists,
replace only that section. Do not commit absolute local paths to `AGENTS.md` or
`CLAUDE.md`.
When workspace registration is in scope, reuse the existing workspace name
and roles; the values below are examples, not defaults. Register the named repos:
```bash
bbs config workspace add-repo <workspace> --git-url <this-origin> --path <this-repo> --role fe --repo-type polyrepo
bbs config workspace add-repo <workspace> --git-url <related-origin> --path <related-repo> --role be
```
`add-repo` creates the workspace when needed and writes only
`~/.babysit/config.yaml`. Do not create or commit `.babysit/config.yaml`.
On a repo that is not joining a workspace, seed `.babysit/.env` instead, after
ensuring it is gitignored:
```bash
# .babysit/.env  (gitignored) — fallback when there is no workspace entry
RELATED_BACKEND_REPO=../api
RELATED_FRONTEND_REPO=../web
RELATED_SHARED_REPO=../shared
```
Don't seed both for the same role. Do not fail setup when a related repo path
is absent — record where the path is expected to come from and leave it unset.
