#!/usr/bin/env python3
"""Build the `2build` soot pack from .claude/skills/.

Every babysit skill becomes one capability (capability name == skill dir name,
skill name == 2build_<skill>). `foreman` is excluded — its runtime depends on
Orca transport and does not port to a soot pack. Output layout:

    <out>/2build/            git-addable, self-contained pack folder
        pack.json
        README.md
        build-meta.json      version/commit provenance (release builds only)
        <skill>/skill.md     one per capability
        <skill>/…            aux files copied verbatim (references/, workflows/, data/)
        shared/              .claude/skills/references/ copied verbatim

Install into a soot deployment:
    soot add <babysit-repo> --path packs/2build
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
        body = skill_file.read_bytes()
        if len(body) > SOOT_SKILL_LIMIT:
            failures.append(
                f"{name}: SKILL.md is {len(body)} bytes, over the {SOOT_SKILL_LIMIT}-byte soot skill limit")
            continue
        text = body.decode()
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
        rows.append((name, len(body), description))

    shared = SKILLS / "references"
    if shared.is_dir():
        dst = out / "shared"
        dst.mkdir(parents=True, exist_ok=True)
        for path in sorted(shared.rglob("*")):
            if path.is_file():
                target = dst / path.relative_to(shared)
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(path, target)

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
        "Babysit skill pack for Soot — part of the 2found ecosystem. Each",
        "capability is one babysit skill; select them in the Soot definition.",
        "`foreman` is excluded on purpose: it needs the Orca worker runtime.",
        "",
        "| Capability | Bytes | When to use |",
        "| --- | --- | --- |",
    ]
    lines += [f"| `{n}` | {b} | {d} |" for n, b, d in rows]
    lines += [
        "",
        "Shared babysit references live under `shared/`; skill files reference",
        "them by relative path where the original skill did. Auxiliary skill",
        "assets (references/, workflows/, data/) ship under each capability dir.",
        "",
        "Install: `soot add <repo> --path packs/2build`, then choose either the",
        "default recipe (`\"packs\": [\"2build\"]`, selects every capability) or",
        "direct selectors (`\"use\": [\"2build/implement\", …]`).",
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
