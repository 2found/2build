#!/usr/bin/env bash
set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
python3 - "$ROOT" <<'PY'
import importlib.util
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

root = Path(sys.argv[1])
spec = importlib.util.spec_from_file_location("pack_builder", root / "scripts/build-2build-pack.py")
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)

with tempfile.TemporaryDirectory() as temp:
    pack = Path(temp) / "2build"
    assert builder.build(pack, version="", commit="") == 0
    for skill in ("autopilot", "implement", "plan-draft", "qa", "review-pr", "triage", "semantic-decision"):
        text = (pack / "skills" / skill / "SKILL.md").read_text()
        for target in re.findall(r"(?:\.\./)+(?:semantic-decision|shared)/[\w.-]+\.md", text):
            assert (pack / "skills" / skill / target).is_file(), (skill, target)
    contract = (pack / "shared/semantic-decision.md").read_text()
    assert (pack / "shared" / re.search(r"\[semantic-decision skill\]\(([^)]+)\)", contract)[1]).is_file()
    semantic = (pack / "skills/semantic-decision/SKILL.md").read_text()
    assert "--llm-answers" in semantic
    assert "cloudflare" not in semantic.lower() and "clef" not in contract.lower()
    assert "skill reports `BBS_DEGRADED` and stops" not in semantic
    assert "skill reports `BBS_DEGRADED` and stops" in (pack / "skills/qa/SKILL.md").read_text()

print("OK: packed semantic reads")
PY
