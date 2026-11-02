---
name: release-prd
description: Former name of the release skill; kept so /bbs:release-prd and existing release profiles keep working.
disable-model-invocation: true
---
# release-prd

Renamed to `release`. Read [../release/SKILL.md](../release/SKILL.md) and follow
it with the arguments you were given (`--local`, `--dry-run`, `--prepare`,
`--resume <tag>` are unchanged). Its references, including the
[project profile contract](../release/references/project-profile.md), now live
under `../release/`. `../release/` links are filesystem paths beside this
skill's directory, so read them by path, not as `skill://`.
