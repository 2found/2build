# npm distribution

Read package scripts, `files`, exports, `publishConfig`, registry and workspace
ownership. Use the repo's package manager for install/build/version/lockfile
work. Do not run `npm version` with implicit commit/tag creation when the
release workflow owns tagging. Reject publishing an internal/private package.

After checks/build, inspect `npm pack --dry-run --json`; it can run lifecycle
hooks, so it is not part of the skill's read-only `--dry-run`. Create the actual
tarball with the repo's intended packing command, inspect its file list and
hash it. With the build complete, `npm pack --ignore-scripts` avoids rebuilding
different bytes. Publish the exact inspected tarball with explicit settings:

```sh
npm publish "./$RELEASE_TARBALL" --registry "$RELEASE_REGISTRY" --access public --tag latest --ignore-scripts
npm view "$RELEASE_PACKAGE@$RELEASE_VERSION" version dist.integrity dist.tarball --json --registry "$RELEASE_REGISTRY"
npm view "$RELEASE_PACKAGE" dist-tags --json --registry "$RELEASE_REGISTRY"
```

`public`/`latest` are example choices, not defaults for private packages or
prereleases. Prefer the repo's trusted-publishing path in normal mode; local
publishing needs local authentication/2FA. Never print tokens or persist an OTP
in the release record. Do not downgrade authentication policy.

An existing version is not automatically success: compare its integrity with
the intended tarball and release source. An explicit registry not-found result
means absent; auth/network failure does not. npm versions cannot be overwritten.
A mismatch stops completion and needs a new version.

Download the published tarball, compare integrity, and install/import it in a
temporary consumer (or run its CLI version/help and a failing input). For an
authorized local CLI update, install the exact released version using its
original manager; inspect PATH candidates and prove the active executable's
version. Updating an app dependency/lockfile is a new source change; include
it only in selected consumer scope and verify before its deployment.

Command semantics: [npm publish](https://docs.npmjs.com/cli/v11/commands/npm-publish/),
[npm pack](https://docs.npmjs.com/cli/v11/commands/npm-pack/).
