#!/usr/bin/env python3
"""Structural release checks for the Codex plugin's canonical skill package."""

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CANONICAL = ROOT / ".claude" / "skills"


def skill_names(root: Path) -> set[str]:
    return {
        path.parent.name
        for path in root.glob("*/SKILL.md")
        if not path.parent.name.startswith(".")
    }


def test_manifest_ships_every_canonical_skill() -> None:
    manifest = json.loads((ROOT / ".codex-plugin" / "plugin.json").read_text())
    source = ROOT / manifest["skills"]
    assert source.resolve() == CANONICAL.resolve()
    assert skill_names(source), "A plugin without packaged skills cannot release"


def test_packaged_skills_are_real_files_with_frontmatter() -> None:
    for name in skill_names(CANONICAL):
        skill = CANONICAL / name / "SKILL.md"
        assert not skill.is_symlink(), f"Codex strips symlinked skill files: {name}"
        assert skill.read_text().startswith("---\n"), f"Missing skill frontmatter: {name}"


def test_manifests_match_release_version() -> None:
    version = (ROOT / "VERSION").read_text().strip()
    codex = json.loads((ROOT / ".codex-plugin" / "plugin.json").read_text())
    claude = json.loads((ROOT / ".claude-plugin" / "marketplace.json").read_text())
    assert codex["name"] == "bbs"
    assert codex["skills"] == "./.claude/skills"
    assert codex["version"] == version
    assert claude["metadata"]["version"] == version
    assert claude["plugins"][0]["version"] == version


if __name__ == "__main__":
    test_manifest_ships_every_canonical_skill()
    test_packaged_skills_are_real_files_with_frontmatter()
    test_manifests_match_release_version()
    print("PASS Codex packaged skills and both plugin versions")
