import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

export function downloadManifest(repository, version, commit, files) {
  if (version !== version?.trim() || !/^2found\/(2ai|2agent|2build|2server)$/.test(repository) || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version) || !/^[a-f0-9]{40}$/.test(commit)) throw new Error("Invalid release identity");
  const tag = `v${version}`, base = `https://github.com/${repository}`;
  const artifacts = files.map(({name, bytes}) => {
    if (name !== name.trim() || !/^[A-Za-z0-9_.-]+$/.test(name) || !bytes.length) throw new Error("Invalid release artifact");
    return {name, size: bytes.length, sha256: createHash("sha256").update(bytes).digest("hex"), url: `${base}/releases/download/${tag}/${name}`};
  });
  if (new Set(artifacts.map(a => a.name)).size !== artifacts.length) throw new Error("Duplicate release artifact");
  return {schemaVersion: 1, repository, version, tag, commit, docs: `${base}/tree/${tag}`, artifacts};
}

const websiteDocs = {"2found/2build": "https://2found.dev/docs/2build/", "2found/2server": "https://2found.dev/docs/2server/"};

export function downloadsMarkdown({repository, version, commit, docs, artifacts}) {
  const links = [`[Documentation for this version](${docs})`, ...(websiteDocs[repository] ? [`[Guides on 2found.dev](${websiteDocs[repository]})`] : []), `[Release history](https://github.com/${repository}/releases)`];
  return `# ${repository.split("/")[1]} ${version}\n\nSource commit: \`${commit}\`.\n\n${links.join(" · ")}\n\n| Download | Bytes | SHA-256 |\n| --- | ---: | --- |\n${artifacts.map(a => `| [${a.name}](${a.url}) | ${a.size} | \`${a.sha256}\` |`).join("\n")}\n`;
}

function main() {
  const [version, directory] = process.argv.slice(2), repository = process.env.GITHUB_REPOSITORY;
  if (!directory) throw new Error("Usage: release-assets.mjs <version> <artifact-directory>");
  const git = (...args) => execFileSync("git", args, {encoding: "utf8"}).trim();
  const commit = git("rev-parse", "HEAD");
  downloadManifest(repository, version, commit, []);
  if (git("rev-parse", `v${version}^{commit}`) !== commit) throw new Error("Build checkout differs from release tag");
  mkdirSync(directory, {recursive: true});
  const docs = git("ls-files").split("\n").filter(p => p === "VERSION" || p === "package.json" || /^README(?:\.[^.]+)?\.md$/.test(p) || p === "AGENTS.md" || p === "CLAUDE.md" || p === "BRANDING.md" || p === "CHANGELOG.md" || p === "CHANGELOG-ARCHIVE.md" || p.startsWith("docs/") && p.endsWith(".md") || /\/README(?:\.[^.]+)?\.md$/.test(p));
  const docsName = `${repository.split("/")[1]}-docs_${version}.tar.gz`;
  execFileSync("git", ["archive", "--format=tar.gz", `--output=${resolve(directory, docsName)}`, "HEAD", ...docs]);
  const files = readdirSync(directory).filter(name => /\.(?:tar\.gz|tgz|zip|exe|dmg|sha256)$/.test(name) && statSync(join(directory, name)).isFile())
    .sort().map(name => ({name, bytes: readFileSync(join(directory, name))}));
  if (!files.length || (repository !== "2found/2ai" && files.length < 2)) throw new Error("Missing distribution artifacts");
  const manifest = downloadManifest(repository, version, commit, files);
  const downloads = downloadsMarkdown(manifest);
  writeFileSync(join(directory, "release.json"), JSON.stringify(manifest, null, 2) + "\n");
  writeFileSync(join(directory, "DOWNLOADS.md"), downloads);
  const checksumFiles = [...files, ...["release.json", "DOWNLOADS.md"].map(name => ({name, bytes: readFileSync(join(directory, name))}))];
  writeFileSync(join(directory, "checksums.txt"), checksumFiles.map(f => `${createHash("sha256").update(f.bytes).digest("hex")}  ${f.name}\n`).join(""));
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try { main(); } catch (error) { console.error(error.message); process.exitCode = 1; }
}
