# Workspaces — the multi-repo registry

A **workspace** is a named list of repos that make up one product. It answers
which repos a foreman owns and where those repos live on this machine.

```bash
bbs config workspace add-repo acme --git-url git@github.com:acme/web.git --path ~/src/web --role fe --repo-type polyrepo
bbs config workspace add-repo acme --git-url git@github.com:acme/api.git --path ~/src/api --role be
bbs config workspace list
bbs config workspace show                 # membership of the current repo
```

Workspace registrations and machine-local babysit settings live in one file:
`~/.babysit/config.yaml`.

```yaml
workspaces:
  acme:
    version: 1
    repos:
      - git_url: git@github.com:acme/web.git
        path: /Users/me/src/web
        role: fe
        repo_type: polyrepo
        harness_version: 1.60.0
      - git_url: git@github.com:acme/api.git
        path: /Users/me/src/api
        role: be
        harness_version: 1.60.0
```

There is no `<repo>/.babysit/config.yaml` and no
`~/.babysit/workspaces/<name>.yaml`. Workspace membership is resolved by a
registered local path or git URL. A checkout matching multiple entries is an
error rather than an arbitrary choice.

Local paths are machine-specific, so the unified file is not committed. Agent
enablement and defaults are configured in Orca, not the babysit config.
Repository policy and QA remain committed separately in
`.babysit/git-flow.yaml` and `.babysit/qa.yaml`; secrets remain in the ignored
`.babysit/.env`.

## Three things are called "workspace"

| Meaning | What it is | How it is named in output |
|---|---|---|
| Orca terminal | one visible worker and agent session | **Orca terminal**, never bare |
| worktree pool | `<repo>/.babysit/worktrees/<ticket>_<slug>` | **worktree** |
| registry workspace | a named set of repos in `config.yaml` | **workspace acme** |

`foreman` gains no extra field for the registry. Its `ProjectDir` is matched to
the unified registry. `foreman.Record.WorkspaceDir`, `WorkspaceRef`, and
`WorkspaceTitle` refer to the Orca terminal.

## Repository metadata

`harness_version`, `name`, `description`, and `repo_type` belong to each repo
entry. `add-repo` records the running harness version. A missing
`harness_version` is valid and silent; a known older version is shown by
`bbs config workspace show`.

`repo_type` changes behavior: `monorepo` disables sibling-repo fan-out because
its related code is already inside the same checkout. `polyrepo` and an unset
value keep fan-out enabled.

## Sibling paths

`bbs ticket serve` resolves a related role from:

1. the current repo's matched workspace entry — authoritative
2. `RELATED_*_REPO` in `<repo>/.babysit/.env` — fallback

When both sources name different paths, babysit blocks and prints both values.
When they agree, the workspace path wins without a warning. A repo absent from
the workspace registry continues to use `.babysit/.env`.

## Tests

Tests that touch workspaces call `workspace.TestHome(t)`, which redirects
`BABYSIT_STATE_DIR` so the unified config cannot touch the developer's real
`~/.babysit/config.yaml`.
