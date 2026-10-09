#!/usr/bin/env python3
"""Build the `2build` soot pack from .claude/skills/.

Every 2build skill becomes one capability (capability name == skill dir name,
skill name == 2build_<skill>). `foreman` is excluded — its runtime depends on
Orca transport and does not port to a soot pack. Output layout:

    <out>/2build/            git-addable, self-contained pack folder
        pack.json
        README.md
        build-meta.json      version/commit provenance (release builds only)
        <skill>/skill.md     one per capability
        <skill>/…            aux files (references/, workflows/, data/)
        shared/              .claude/skills/references/ (Markdown paths translated)

Install into a soot deployment:
    soot add <2build-repo> --path packs/2build
    # then pick capabilities in the Soot definition:
    #   "packs": ["2build"]            → default recipe (pack.use)
    #   "use": ["2build/implement", …] → direct selectors

CI / release usage:
    python3 scripts/build-2build-pack.py --check-tree          # CI drift check
    python3 scripts/build-2build-pack.py --version 1.94.1 \
        --commit "$GITHUB_SHA" --tarball dist/2build-pack_1.94.1.tar.gz
The tarball is reproducible: fixed order, uid/gid 0, mtime = SOURCE_DATE_EPOCH
(or 0). Version resolution: --version flag → $BABYSIT_VERSION → VERSION file;
"" means a local dev build (build-meta.json is then omitted so the checked-in
pack tree stays byte-stable across rebuilds).
"""
import argparse
import filecmp
import gzip
import json
import os
import re
import shutil
import sys
import tarfile
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SKILLS = ROOT / ".claude" / "skills"
DEFAULT_OUT = ROOT / "packs" / "2build"
EXCLUDE = {"foreman", "references"}
SOOT_SKILL_LIMIT = 64 << 10  # matches internal/config/config.go maxPromptBytes

# Soot inlines a skill body verbatim into the agent prompt and never resolves
# the `../references/` links the source skills carry, so the bbs-CLI
# prerequisite has to sit in the body itself to be read. Injected into every
# packed skill.md (after the frontmatter) and repeated in the README.
PREREQUISITE = (
    "> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.\n"
    "> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release\n"
    "> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);\n"
    "> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the\n"
    "> skill reports `BBS_DEGRADED` and stops.\n"
    "\n"
)

FRONTMATTER = re.compile(r"\A---\n(.*?)\n---\n", re.DOTALL)


def frontmatter_field(text, key):
    m = FRONTMATTER.match(text)
    if not m:
        return ""
    for line in m.group(1).splitlines():
        k, _, v = line.partition(":")
        if k.strip() == key:
            return v.strip().strip('"')
    return ""


def with_prerequisite(text):
    """Insert the bbs-CLI prerequisite after the frontmatter block."""
    m = FRONTMATTER.match(text)
    at = m.end() if m else 0
    prerequisite = PREREQUISITE
    if frontmatter_field(text, "name") == "semantic-decision":
        prerequisite = prerequisite.replace(
            "skill reports `BBS_DEGRADED` and stops.",
            "skill reports `BBS_DEGRADED` and uses the calling LLM fallback;\n"
            "> retain the request, choice and cited reason in the handoff.")
    if frontmatter_field(text, "name") == "design-ui":
        prerequisite = prerequisite.replace(
            "skill reports `BBS_DEGRADED` and stops.",
            "skill reports `BBS_DEGRADED` and continues design; ticket-state writes\n"
            "> remain unavailable until the CLI is installed.")
        prerequisite += (
            "> **Optional Soot design data.** If using `bbs design suggest` or\n"
            "> `ux-check`, pass `--data <packs_dir>/2build/design-ui/data`, resolving\n"
            "> `packs_dir` from the deployment file. These tables are optional aids,\n"
            "> not prerequisites for designing or verifying the UI.\n\n"
        )
    return text[:at] + "\n" + prerequisite + text[at:]


def pack_markdown(text):
    """Translate source skill-directory paths to the pack's filesystem layout."""
    text = text.replace("../references/", "../shared/")
    return re.sub(r"(\.\./[A-Za-z0-9_-]+/)SKILL\.md\b", r"\1skill.md", text)


def copy_skill(dst_dir, src_dir):
    dst_dir.mkdir(parents=True, exist_ok=True)
    for path in sorted(src_dir.rglob("*")):
        if not path.is_file():
            continue
        rel = path.relative_to(src_dir)
        rel = Path("skill.md") if rel == Path("SKILL.md") else rel
        target = dst_dir / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(path, target)
        if target.suffix == ".md":
            target.write_text(pack_markdown(target.read_text()))
        if rel == Path("skill.md"):
            target.write_text(with_prerequisite(target.read_text()))


def write_tarball(pack_dir, out_path):
    """Deterministic tar.gz: sorted entries, uid/gid 0, fixed mtime."""
    mtime = int(os.environ.get("SOURCE_DATE_EPOCH", "0"))
    out_path.parent.mkdir(parents=True, exist_ok=True)

    def fix(info):
        info.uid = info.gid = 0
        info.uname = info.gname = ""
        info.mtime = mtime
        info.mode = 0o755 if info.isdir() else (info.mode & 0o777 or 0o644)
        return info

    with open(out_path, "wb") as raw, gzip.GzipFile(
            filename="", mode="wb", fileobj=raw, mtime=mtime) as gz:
        with tarfile.open(fileobj=gz, mode="w") as archive:
            archive.add(pack_dir, arcname=pack_dir.name, filter=fix, recursive=False)
            for path in sorted(pack_dir.rglob("*")):
                archive.add(path, arcname=str(Path(pack_dir.name) / path.relative_to(pack_dir)),
                            filter=fix, recursive=False)


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--out", type=Path, default=DEFAULT_OUT,
                    help="pack output dir (default: packs/2build)")
    ap.add_argument("--version", default=None,
                    help="bare version (e.g. 1.94.1); default: $BABYSIT_VERSION, then VERSION file")
    ap.add_argument("--commit", default="",
                    help="source commit recorded in build-meta.json")
    ap.add_argument("--tarball", type=Path, default=None,
                    help="also write a deterministic tar.gz of the pack at this path")
    ap.add_argument("--check-tree", action="store_true",
                    help="rebuild into a temp dir and diff against --out; exit 1 on drift (CI freshness check)")
    args = ap.parse_args()
    if args.check_tree:
        with tempfile.TemporaryDirectory() as tmp:
            fresh = Path(tmp) / "2build"
            rc = build(fresh, version=resolve_version(None), commit="")
            if rc:
                return rc
            diff = _walk_diff(filecmp.dircmp(args.out, fresh), args.out, fresh)
            if diff:
                for line in diff:
                    print(f"FAIL {line}", file=sys.stderr)
                print("regenerate with: python3 scripts/build-2build-pack.py", file=sys.stderr)
                return 1
            print("pack tree is current")
            return 0

    rc = build(args.out, version=resolve_version(args.version), commit=args.commit)
    if rc:
        return rc
    if args.tarball:
        write_tarball(args.out, args.tarball)
    return 0


def _walk_diff(cmp_result, left, right):
    out = []
    for name in sorted(cmp_result.left_only):
        out.append(f"only in {left}: {name}")
    for name in sorted(cmp_result.right_only):
        out.append(f"missing from {left}: {name}")
    for name in sorted(cmp_result.diff_files):
        out.append(f"differs: {left / name}")
    for name, sub in cmp_result.subdirs.items():
        out += _walk_diff(filecmp.dircmp(left / name, right / name), left / name, right / name)
    return out


def resolve_version(flag_value):
    if flag_value is not None:
        return flag_value.lstrip("v")
    env = os.environ.get("BABYSIT_VERSION")
    if env:
        return env.lstrip("v")
    vf = ROOT / "VERSION"
    if vf.is_file():
        return vf.read_text().strip().lstrip("v")
    return ""


def build(out, version, commit):
    capabilities = {}
    rows = []
    failures = []
    for skill_dir in sorted(p for p in SKILLS.iterdir() if p.is_dir()):
        name = skill_dir.name
        if name in EXCLUDE:
            continue
        skill_file = skill_dir / "SKILL.md"
        if not skill_file.is_file():
            failures.append(f"{name}: missing SKILL.md")
            continue
        text = skill_file.read_text()
        packed = len(with_prerequisite(pack_markdown(text)).encode())
        if packed > SOOT_SKILL_LIMIT:
            failures.append(
                f"{name}: skill.md is {packed} bytes, over the {SOOT_SKILL_LIMIT}-byte soot skill limit")
            continue
        description = frontmatter_field(text, "description")
        if not description:
            failures.append(f"{name}: no description in SKILL.md frontmatter")
            continue
        copy_skill(out / name, skill_dir)
        capabilities[name] = {
            "skills": [{
                "name": f"2build_{name}",
                "file": f"{name}/skill.md",
                "description": description,
            }]
        }
        rows.append((name, packed, description))

    shared = SKILLS / "references"
    if shared.is_dir():
        dst = out / "shared"
        dst.mkdir(parents=True, exist_ok=True)
        for path in sorted(shared.rglob("*")):
            if path.is_file():
                target = dst / path.relative_to(shared)
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(path, target)
                if target.suffix == ".md":
                    target.write_text(pack_markdown(target.read_text()))

    manifest = {
        "api_version": "soot/v1",
        "id": "2build",
        "use": sorted(capabilities),
        "capabilities": capabilities,
    }
    data = json.dumps(manifest, indent=2, ensure_ascii=False) + "\n"
    if len(data.encode()) > 64 << 10:
        failures.append(f"pack.json would be {len(data.encode())} bytes, over the 64 KiB manifest limit")
    else:
        (out / "pack.json").write_text(data)

    if failures:
        for f in failures:
            print(f"FAIL {f}", file=sys.stderr)
        shutil.rmtree(out, ignore_errors=True)
        return 1

    if version:
        meta = {"pack": "2build", "version": version, "skills": len(capabilities)}
        if commit:
            meta["source_commit"] = commit
        (out / "build-meta.json").write_text(json.dumps(meta, indent=2) + "\n")

    lines = [
        "# 2build",
        "",
        "**Give your Soot a feature to build.**",
        "",
        "`autopilot` plans the work, writes code, reviews it, tests it and",
        "fixes issues. For UI work, it creates a prototype first. You can",
        "follow progress without reminding the agent to do each step.",
        "",
        f"The pack contains {len(capabilities)} skills for product engineering and growth.",
        "The companion `bbs` CLI keeps progress and evidence on disk.",
        "",
        "A 2found product. [Soot](https://trysoot.com), powered by 2agent, is",
        "**agent as config**: add config to your source code. Add an AI teammate.",
        "",
        "**[Add the pack](#quick-start) · [Explore 2build](https://github.com/2found/2build)**",
        "",
        "## Prerequisites",
        "",
        "You need Soot, an existing deployment, and the `bbs` CLI on the host",
        "where skills run. A skill pack supplies instructions; your Soot also",
        "needs the tools and repository access required by the task.",
        "The Git installation below also requires Git on `PATH`.",
        "",
        "Install `bbs` on macOS or Linux:",
        "",
        "```sh",
        "brew tap 2found/2build https://github.com/2found/2build",
        "brew install 2found/2build/bbs",
        "bbs --version",
        "```",
        "",
        "Without Homebrew, use a verified",
        "[release archive](https://github.com/2found/2build/blob/main/docs/install.md#release-archives-macos-or-linux).",
        "On Windows, run the Linux CLI inside WSL. A plugin-only install does not",
        "include the binary. Without `bbs`, packed skills report `BBS_DEGRADED`",
        "and stop; `semantic-decision` can use its calling LLM fallback, and",
        "`design-ui` can continue design without ticket-state writes.",
        "",
        "## Quick start",
        "",
        "From your existing Soot project, add the pack:",
        "",
        "```sh",
        "soot add https://github.com/2found/2build --path packs/2build",
        "```",
        "",
        "Start with Autopilot in the Soot definition for a complete one-ticket",
        "workflow:",
        "",
        "```json",
        '{"use": ["2build/autopilot"]}',
        "```",
        "",
        "Run `soot check --show` to inspect the resolved configuration before",
        "giving the Soot a task. This validates configuration; it does not run",
        "the task or prove the resulting code works.",
        "",
        "## Choose the work",
        "",
        "| Work | Start with |",
        "| --- | --- |",
        "| Validate an idea | [prototype](prototype/skill.md), [recon](recon/skill.md) |",
        "| Finish a ticket | [autopilot](autopilot/skill.md), [implement](implement/skill.md), [qa](qa/skill.md) |",
        "| Simplify code | [sweep](sweep/skill.md) |",
        "| Improve marketing | [product-marketing-page](product-marketing-page/skill.md) |",
        "| Diagnose or harden | [investigate](investigate/skill.md), [maintain](maintain/skill.md) |",
        "",
        "`foreman` is not included: its project coordinator requires the Orca",
        "worker runtime. See the [full product](https://github.com/2found/2build)",
        "for that workflow.",
        "",
        "## Full capability inventory",
        "",
        "<details>",
        f"<summary>Browse all {len(capabilities)} skills and their packaged sizes</summary>",
        "",
        "| Capability | Bytes | When to use |",
        "| --- | --- | --- |",
    ]
    lines += [f"| `{n}` | {b} | {d} |" for n, b, d in rows]
    lines += [
        "",
        "</details>",
        "",
        "## Pack layout",
        "",
        "Shared 2build references live under `shared/`; skill files reference",
        "them by relative path where the original skill did. Auxiliary skill",
        "assets (references/, workflows/, data/) ship under each capability dir.",
        "The CLI prerequisite is inlined at the top of every `skill.md`.",
        "",
        "Direct `use` selectors load the skills you choose. The default recipe,",
        "`\"packs\": [\"2build\"]`, selects every capability in `pack.json`.",
        "",
        "This README is generated from `scripts/build-2build-pack.py` in the",
        "2build repository. Edit that source and regenerate the pack to update it.",
    ]
    (out / "README.md").write_text("\n".join(lines) + "\n")

    print(json.dumps({
        "pack": str(out),
        "version": version or "dev",
        "capabilities": len(capabilities),
        "manifest_bytes": len(data.encode()),
    }))
    return 0


if __name__ == "__main__":
    sys.exit(main())
