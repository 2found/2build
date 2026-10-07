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
    for skill in ("autopilot", "fix-pr", "implement", "plan-draft", "qa", "review-pr", "triage", "semantic-decision"):
        text = (pack / skill / "skill.md").read_text()
        for target in re.findall(r"\.\./(?:semantic-decision|shared)/[\w.-]+\.md", text):
            assert (pack / skill / target).is_file(), (skill, target)
    contract = (pack / "shared/semantic-decision.md").read_text()
    assert (pack / "shared" / re.search(r"\[semantic-decision skill\]\(([^)]+)\)", contract)[1]).is_file()
    semantic = (pack / "semantic-decision/skill.md").read_text()
    assert "uses the calling LLM fallback" in semantic
    assert "skill reports `BBS_DEGRADED` and stops" not in semantic
    assert "skill reports `BBS_DEGRADED` and stops" in (pack / "qa/skill.md").read_text()

# Execute each consumer's actual load expression with a dummy project exporter.
# Provider values from the project must be filtered, while QA variables and
# pre-existing user provider values remain available.
for skill in ("qa", "fix-pr", "browse", "create-pr"):
    text = (root / ".claude/skills" / skill / "SKILL.md").read_text()
    expression = re.search(r'eval "\$\((bbs secrets load.*?)\)"', text)[1]
    script = """bbs() { printf '%s\\n' 'export CLOUDFLARE_ACCOUNT_ID=project-account' 'export CLOUDFLARE_API_TOKEN=project-token' 'export QA_PASS=project-login'; }
""" + 'eval "$(' + expression + ')"\n' + """test "$QA_PASS" = project-login
test "${CLOUDFLARE_ACCOUNT_ID-}" = "${EXPECTED_ACCOUNT-}"
test "${CLOUDFLARE_API_TOKEN-}" = "${EXPECTED_TOKEN-}"
"""
    environment = os.environ.copy()
    for key in ("CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_API_TOKEN", "EXPECTED_ACCOUNT", "EXPECTED_TOKEN"):
        environment.pop(key, None)
    subprocess.run(["bash", "-eu", "-c", script], env=environment, check=True)
    environment.update(CLOUDFLARE_ACCOUNT_ID="user-account", CLOUDFLARE_API_TOKEN="user-token",
                       EXPECTED_ACCOUNT="user-account", EXPECTED_TOKEN="user-token")
    subprocess.run(["bash", "-eu", "-c", script], env=environment, check=True)
print("OK: packed semantic reads/fallback and consumer credential boundaries")
PY
