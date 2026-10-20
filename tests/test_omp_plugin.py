#!/usr/bin/env python3
"""Structural checks for the OMP plugin package."""

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CANONICAL = ROOT / ".claude" / "skills"


def test_package_manifest() -> None:
    manifest = json.loads((ROOT / "package.json").read_text())
    version = (ROOT / "VERSION").read_text().strip()
    assert manifest["name"] == "@2found/bbs"
    assert manifest["version"] == version
    assert manifest["type"] == "module"
    assert manifest["omp"]["extensions"] == ["./hooks/omp.ts"]
    assert (ROOT / "hooks" / "omp.ts").is_file()


def test_skills_symlink_exposes_canonical_pack() -> None:
    skills = ROOT / "skills"
    assert skills.is_symlink(), "OMP discovers skills/ at the plugin root"
    assert skills.resolve() == CANONICAL.resolve()
    assert (skills / "autopilot" / "SKILL.md").is_file()
    assert (skills / "references" / "preamble.md").is_file()


if __name__ == "__main__":
    test_package_manifest()
    test_skills_symlink_exposes_canonical_pack()
    print("PASS OMP plugin package and skills symlink")
