#!/usr/bin/env python3
"""Build the `2build` soot pack from .claude/skills/.

Every 2build skill becomes one pack skill, `skills/<skill>/SKILL.md` (Soot
pack v2: the folder name is the skill name, used as is). Rename shims (`foreman`, `release-prd`, `analytics-review`) are excluded, and so is `team` — its runtime depends on
Orca transport and does not port to a soot pack. Output layout:

    <out>/2build/                    git-addable, self-contained pack folder
        pack.json                    {name, description, keywords}; skills are found by folder
        README.md
        build-meta.json              version/commit provenance (release builds only)
        skills/<skill>/SKILL.md      one per skill
        skills/<skill>/…             aux files (references/, workflows/, data/)
        shared/                      .claude/skills/references/

Every directory under skills/ is a skill, so the shared references sit beside
skills/ and relative paths in Markdown are rewritten to the packed layout.

Install into a soot deployment:
    soot add <2build-repo> --path packs/2build
    # then select the pack in the Soot definition:
    #   "packs": ["2build"]            → every skill under skills/

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
import posixpath
import re
import shutil
import sys
import tarfile
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SKILLS = ROOT / ".claude" / "skills"
DEFAULT_OUT = ROOT / "packs" / "2build"
EXCLUDE = {"team", "foreman", "release-prd", "analytics-review", "references"}
SOOT_SKILL_LIMIT = 64 << 10  # matches internal/config/config.go maxPromptBytes

# Soot inlines a skill body verbatim into the agent prompt and never resolves
# the `../references/` links the source skills carry, so the bbs-CLI
# prerequisite has to sit in the body itself to be read. Injected into every
# packed SKILL.md (after the frontmatter) and repeated in the README.
PREREQUISITE = (
    "> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.\n"
    "> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release\n"
    "> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);\n"
    "> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the\n"
    "> skill reports `BBS_DEGRADED` and stops.\n"
    "\n"
)

# Prose-only skills that never shell out to `bbs`: the prerequisite would tell
# them to stop when the CLI is missing.
NO_BBS = {"humanizer-vi"}

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
    if frontmatter_field(text, "name") in NO_BBS:
        return text
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
    return text[:at] + "\n" + prerequisite + text[at:]


RELATIVE_PATH = re.compile(r"(?:\.\./)+[A-Za-z0-9_.-][A-Za-z0-9_./-]*|(?:\.\./)+")


def packed_location(rel):
    """Map a path relative to .claude/skills to its path inside the pack."""
    parts = rel.split("/", 1)
    if parts[0] == "references":
        return "/".join(["shared"] + parts[1:])
    return "skills/" + rel


def pack_markdown(text, src_rel, out_rel):
    """Rewrite `../`-relative paths so they resolve from the packed file.

    src_rel and out_rel are the file's paths relative to .claude/skills and to
    the pack root; targets outside .claude/skills are left as written."""
    src_dir = posixpath.dirname(src_rel)
    out_dir = posixpath.dirname(out_rel)

    # Auxiliary files (a skill's references/, workflows/) often write paths
    # relative to the skill folder; use that reading when the file-relative
    # one names nothing.
    skill_dir = src_rel.split("/", 1)[0] if "/" in src_rel else ""

    def fix(m):
        link = m.group(0)
        resolved = posixpath.normpath(posixpath.join(src_dir, link))
        if not (SKILLS / resolved).exists() and skill_dir and skill_dir != src_dir:
            alt = posixpath.normpath(posixpath.join(skill_dir, link))
            if (SKILLS / alt).exists():
                resolved = alt
        if resolved == ".." or resolved.startswith("../"):
            return link
        target = packed_location(resolved) if resolved != "." else ""
        new = posixpath.relpath(target or ".", out_dir or ".")
        return new + "/" if link.endswith("/") and not new.endswith("/") else new

    return RELATIVE_PATH.sub(fix, text)


def copy_tree(src_dir, dst_dir, out):
    """Copy src_dir to dst_dir, rewriting relative paths in Markdown files."""
    for path in sorted(src_dir.rglob("*")):
        if not path.is_file():
            continue
        target = dst_dir / path.relative_to(src_dir)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(path, target)
        if target.suffix == ".md":
            text = pack_markdown(target.read_text(),
                                 path.relative_to(SKILLS).as_posix(),
                                 target.relative_to(out).as_posix())
            if target.name == "SKILL.md" and target.parent.parent.name == "skills":
                text = with_prerequisite(text)
            target.write_text(text)


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
    skills = []
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
        packed = len(with_prerequisite(text).encode())
        if packed > SOOT_SKILL_LIMIT:
            failures.append(
                f"{name}: SKILL.md is {packed} bytes, over the {SOOT_SKILL_LIMIT}-byte soot skill limit")
            continue
        description = frontmatter_field(text, "description")
        if not description:
            failures.append(f"{name}: no description in SKILL.md frontmatter")
            continue
        if frontmatter_field(text, "name") != name:
            failures.append(f"{name}: frontmatter name must equal the folder name")
            continue
        copy_tree(skill_dir, out / "skills" / name, out)
        skills.append(name)
        rows.append((name, packed, description))

    shared = SKILLS / "references"
    if shared.is_dir():
        copy_tree(shared, out / "shared", out)

    manifest = {
        "name": "2build",
        "description": {
            "vi": "Quy trình làm sản phẩm của 2build: lên kế hoạch, viết code, review, kiểm thử và phát hành từng việc một. Các skill gọi CLI bbs.",
            "en": "Product engineering workflows from 2build: plan, build, review, test and ship one ticket at a time. Skills shell out to the bbs CLI.",
        },
        "keywords": {
            "vi": ["lập trình", "kiểm thử", "phát hành"],
            "en": ["engineering", "coding", "qa", "release"],
        },
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
        meta = {"pack": "2build", "version": version, "skills": len(skills)}
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
        f"The pack contains {len(skills)} skills for product engineering and growth.",
        "The companion `bbs` CLI keeps progress and evidence on disk.",
        "",
        "A 2found product. With [Soot](https://trysoot.com), powered by 2agent,",
        "you give recurring work to a small team of Soots that runs on schedule",
        "and asks you only when a person is needed.",
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
        "Select the pack in the Soot definition; start with Autopilot for a complete",
        "one-ticket workflow:",
        "",
        "```json",
        '{"packs": ["2build"]}',
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
        "| Evaluate external code | [recon](skills/recon/SKILL.md) |",
        "| Finish a ticket | [autopilot](skills/autopilot/SKILL.md), [implement](skills/implement/SKILL.md), [qa](skills/qa/SKILL.md) |",
        "| Docs or marketing page | [agent-first-docs](skills/agent-first-docs/SKILL.md) |",
        "| Diagnose a failure | [investigate](skills/investigate/SKILL.md) |",
        "",
        "`foreman` is not included: its project coordinator requires the Orca",
        "worker runtime. See the [full product](https://github.com/2found/2build)",
        "for that workflow.",
        "",
        "## Full skill inventory",
        "",
        "<details>",
        f"<summary>Browse all {len(skills)} skills and their packaged sizes</summary>",
        "",
        "| Skill | Bytes | When to use |",
        "| --- | --- | --- |",
    ]
    lines += [f"| `{n}` | {b} | {d} |" for n, b, d in rows]
    lines += [
        "",
        "</details>",
        "",
        "## Pack layout",
        "",
        "Each skill is `skills/<name>/SKILL.md`; shared 2build references live",
        "under `shared/`, and relative paths in Markdown are rewritten to this",
        "layout. Auxiliary skill assets (references/, workflows/, data/) ship",
        "under each skill dir.",
        "The CLI prerequisite is inlined at the top of every `SKILL.md`.",
        "",
        "A Soot that selects the pack (`\"packs\": [\"2build\"]`) reads every",
        "skill under `skills/`; the pack has no tools.",
        "",
        "This README is generated from `scripts/build-2build-pack.py` in the",
        "2build repository. Edit that source and regenerate the pack to update it.",
    ]
    (out / "README.md").write_text("\n".join(lines) + "\n")

    print(json.dumps({
        "pack": str(out),
        "version": version or "dev",
        "skills": len(skills),
        "manifest_bytes": len(data.encode()),
    }))
    return 0


if __name__ == "__main__":
    sys.exit(main())
