import { test } from "node:test";
import assert from "node:assert/strict";
import { downloadManifest, downloadsMarkdown } from "./release-assets.mjs";
test("download manifest binds checksums and URLs to the exact version and source", () => {
  const result = downloadManifest("2found/2agent", "0.1.0", "a".repeat(40), [{name: "soot_0.1.0_linux_amd64.tar.gz", bytes: Buffer.from("fixture")}]);
  assert.equal(result.artifacts[0].url, "https://github.com/2found/2agent/releases/download/v0.1.0/soot_0.1.0_linux_amd64.tar.gz");
  assert.equal(result.artifacts[0].sha256.length, 64);
  assert.equal(result.artifacts[0].size, 7);
  assert.equal(result.docs, "https://github.com/2found/2agent/tree/v0.1.0");
  for (const name of ["../private.env", "bad name", "file\n"])
    assert.throws(() => downloadManifest("2found/2agent", "0.1.0", "a".repeat(40), [{name, bytes: Buffer.from("x")}]), /artifact/);
  assert.throws(() => downloadManifest("2found/other", "0.1.0", "a".repeat(40), []), /identity/);
  assert.throws(() => downloadManifest("2found/2ai", "0.1.0", "bad", []), /identity/);
});
test("release notes link the website guides for products documented on 2found.dev", () => {
  const build = downloadsMarkdown(downloadManifest("2found/2build", "1.0.0", "a".repeat(40), [{name: "bbs_1.0.0_linux_amd64.tar.gz", bytes: Buffer.from("x")}]));
  assert.match(build, /\[Documentation for this version\]\(https:\/\/github\.com\/2found\/2build\/tree\/v1\.0\.0\) · \[Guides on 2found\.dev\]\(https:\/\/2found\.dev\/docs\/2build\/\) · \[Release history\]/);
  assert.match(build, /\| \[bbs_1\.0\.0_linux_amd64\.tar\.gz\]\(https:\/\/github\.com\/2found\/2build\/releases\/download\/v1\.0\.0\/bbs_1\.0\.0_linux_amd64\.tar\.gz\) \| 1 \| `[0-9a-f]{64}` \|/);
  const agent = downloadsMarkdown(downloadManifest("2found/2agent", "0.1.0", "a".repeat(40), []));
  assert.doesNotMatch(agent, /2found\.dev/);
});
