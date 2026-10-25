"""Exercise the shipped launch commands and native agent payloads, offline.

The hooks are compiled into the bbs binary (`bbs hooks <name>`), so the
manifest commands carry no shell syntax at all — the portability claim this
suite guards. Decisions are driven by real ticket state (evidence recorded by
`bbs autopilot verification` under BABYSIT_PROJECT_HOME), not stubs: the gate resolves identity in-process now,
so a fake bbs shim could never intercept it.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


def build_bbs(target: Path):
    result = subprocess.run(
        ["go", "build", "-o", str(target), "./cmd/bbs"],
        cwd=ROOT, capture_output=True, text=True, timeout=300,
    )
    if result.returncode != 0:
        raise RuntimeError(f"go build failed: {result.stderr}")


def git(repo: Path, *args: str):
    subprocess.run(["git", *args], cwd=repo, check=True,
                   capture_output=True, text=True)


class Hooks(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cls_tmp = tempfile.TemporaryDirectory(prefix="bbs hooks build ")
        cls.addClassCleanup(cls.cls_tmp.cleanup)
        cls.bbs = Path(cls.cls_tmp.name) / "bbs"
        build_bbs(cls.bbs)

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="bbs hooks ")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.home = self.root / "home"
        self.home.mkdir()
        self.state = self.root / "state"
        self.project = self.state / "projects" / "testslug"
        self.ticket_home = self.project / "tickets" / "bs-test"
        (self.ticket_home / "verdicts").mkdir(parents=True)
        # The gate's readiness probe needs a git repo at the payload workdir.
        self.repo = self.root / "repo"
        self.repo.mkdir()
        git(self.repo, "init", "-q", "-b", "main")
        git(self.repo, "config", "user.email", "t@t")
        git(self.repo, "config", "user.name", "t")
        (self.repo / "f").write_text("x")
        git(self.repo, "add", "f")
        git(self.repo, "commit", "-qm", "init")
        self.env = {
            "HOME": str(self.home),
            "PATH": f"{self.bbs.parent}{os.pathsep}{os.environ['PATH']}",
            "BABYSIT_HOME": str(self.state),
            "BABYSIT_PROJECT_HOME": str(self.project),
            "BABYSIT_TICKET": "bs-test",
        }
        for k in ("BBS_TICKET", "CODEX_SESSION_ID", "CODEX_THREAD_ID",
                  "GROK_SESSION_ID", "GROK_HOOK_EVENT", "CLAUDE_CODE_SESSION_ID"):
            self.env.pop(k, None)
        self.hooks = json.loads((ROOT / "hooks/hooks.json").read_text())["hooks"]

    # ── helpers ──────────────────────────────────────────────────
    def verdict(self, skill: str, body: str):
        (self.ticket_home / "verdicts" / f"{skill}.md").write_text(body)

    def clear_verdicts(self):
        for f in (self.ticket_home / "verdicts").glob("*.md"):
            f.unlink()

    def run_hook(self, event="PreToolUse", payload=None, env=None):
        command = self.hooks[event][0]["hooks"][0]["command"]
        return subprocess.run(
            ["/bin/sh", "-c", command],
            input=json.dumps(payload or {
                "tool_input": {"command": "git push origin HEAD"},
                "cwd": str(self.repo),
            }),
            text=True, capture_output=True, env=env or self.env, timeout=15,
        )

    def decision(self, command="git push origin HEAD", **changes):
        payload = {"tool_input": {"command": command, "workdir": str(self.repo)},
                   "cwd": str(self.repo)}
        result = self.run_hook(payload=payload, env={**self.env, **changes})
        self.assertEqual(result.returncode, 0, result.stderr)
        if not result.stdout.strip():
            return "pass"
        return json.loads(result.stdout)["hookSpecificOutput"]["permissionDecision"]

    # ── manifest shape ───────────────────────────────────────────
    def test_commands_are_bbs_hooks_with_no_shell_syntax(self):
        # The whole point of the compiled seam: the manifest command must be a
        # bare `bbs hooks <name>` — no $VARS, conditionals, or quoting that a
        # non-POSIX shell could misread.
        self.assertEqual(set(self.hooks), {"PreToolUse", "PostToolUse", "SessionStart", "Stop"})
        commands = {
            "PreToolUse": "bbs hooks pre-tool-gate",
            "PostToolUse": "bbs hooks session-writer",
            "SessionStart": "bbs hooks session-writer",
            "Stop": "bbs hooks worker-report-gate",
        }
        for event, want in commands.items():
            got = self.hooks[event][0]["hooks"][0]["command"]
            self.assertEqual(got, want)
            self.assertNotRegex(got, r"[$`;&|<>]")

    def test_single_gate_and_no_audit_hooks(self):
        self.assertEqual(len(self.hooks["PreToolUse"][0]["hooks"]), 1)

    # ── release decisions ────────────────────────────────────────
    def record_gate(self, gate: str):
        def bbs(*args):
            out = subprocess.run([str(self.bbs), *args], cwd=self.repo, env=self.env,
                                 capture_output=True, text=True, timeout=30)
            self.assertEqual(out.returncode, 0, out.stderr)
            return out.stdout
        attempt = json.loads(bbs("autopilot", "verification", "begin", "--gate", gate,
                                 "--owner", "test", "--handle", "test",
                                 "--transport", "test", "--harness", "test"))["data"]["id"]
        log = self.root / f"{gate}.log"
        log.write_text("ok\n")
        result = self.root / f"{gate}.json"
        result.write_text(json.dumps({
            "checks": [{"argv": ["true"], "cwd": str(self.repo), "exit_code": 0,
                        "log_path": str(log)}],
            "unresolved_findings": [], "limitations": []}))
        bbs("autopilot", "verification", "record", "--attempt", attempt, "--file", str(result))

    def test_release_decisions(self):
        # Verdict prose never opens the gate: without recorded evidence every
        # hard stage denies, whatever the verdict files say.
        for review, qa, command in [
            ("none", "none", "git push origin HEAD"),
            ("DONE", "none", "git push origin HEAD"),
            ("DONE", "DONE", "gh pr create --fill"),
            ("DONE", "DONE", "gh pr merge 12"),
        ]:
            with self.subTest(review=review, qa=qa, command=command):
                self.clear_verdicts()
                if review != "none":
                    self.verdict("review-pr", f"STATUS: {review}\n")
                if qa != "none":
                    self.verdict("qa", f"STATUS: {qa}\nVERDICT: PASS\n")
                self.assertEqual(self.decision(command), "deny")

        # Evidence recorded through the producer opens each action it covers.
        subprocess.run([str(self.bbs), "ticket", "init"], cwd=self.repo, env=self.env,
                       capture_output=True, check=True)
        (self.ticket_home / "requirement.md").write_text("requirement\n")
        (self.ticket_home / "plan.md").write_text("plan\n")
        self.record_gate("review-pr")
        self.assertEqual(self.decision("git push origin HEAD"), "pass")
        self.assertEqual(self.decision("gh pr create --fill"), "deny")
        self.record_gate("qa")
        self.assertEqual(self.decision("gh pr create --fill"), "pass")
        self.assertEqual(self.decision("gh pr merge 12"), "pass")

    def test_no_ticket_passes(self):
        env = {**self.env, "BABYSIT_TICKET": ""}
        result = self.run_hook(env=env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "")

    def test_grok_camel_case_and_deny_contract(self):
        self.verdict("review-pr", "STATUS: BLOCKED\n")
        payload = {"hookEventName": "PreToolUse",
                   "toolInput": {"command": "git push", "workdir": str(self.repo)}}
        result = self.run_hook(payload=payload)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["decision"], "deny")

    def test_tool_workdir_is_used(self):
        result = self.run_hook(payload={"cwd": str(self.repo), "tool_input": {
            "cmd": "git push", "workdir": str(self.root / "does not exist")}})
        self.assertEqual(json.loads(result.stdout)["hookSpecificOutput"]["permissionDecision"], "deny")

    # ── session writer ───────────────────────────────────────────
    def test_session_payloads(self):
        for agent, prefix in (("claude", "cc"), ("codex", "cx"), ("grok", "grok"), ("omp", "omp")):
            with self.subTest(agent=agent):
                result = self.run_hook("SessionStart", payload={
                    "session_id": f"s-{agent}", "agent": agent, "cwd": str(self.repo)})
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertTrue((self.state / "sessions" / f"{prefix}-s-{agent}.yaml").exists())

    def test_session_id_rejects_traversal(self):
        for sid in ("../outside", "a/b", ".."):
            self.run_hook("SessionStart", payload={"session_id": sid, "cwd": str(self.repo)})
        self.assertFalse((self.state / "outside.yaml").exists())
        sessions = self.state / "sessions"
        if sessions.exists():
            for f in sessions.iterdir():
                self.assertTrue(f.name.startswith(".session.") or "/" not in f.name)

    def test_stop_gate_checks_current_foreman_dispatch(self):
        task_dir = self.state / "tasks" / "task-w"
        task_dir.mkdir(parents=True)
        env = {**self.env, "ORCA_TERMINAL_HANDLE": "term-worker"}
        for active in (False, True):
            task = {"id": "task-w", "foreman": "fm-a", "ticket": "bs-a",
                    "created_at": "2026-01-01T00:00:00Z", "dispatches": [{
                        "id": "task-w.d1", "status": "dispatched" if active else "completed",
                        "terminal": "term-worker", "launched_at": "2026-01-01T00:00:00Z"}]}
            (task_dir / "task.json").write_text(json.dumps(task))
            for continued in (False, True):
                result = self.run_hook("Stop", payload={"stop_hook_active": continued}, env=env)
                self.assertEqual(result.returncode, 0, result.stderr)
                if active:
                    decision = json.loads(result.stdout)
                    self.assertEqual(decision["decision"], "block")
                    self.assertIn("task-w.d1", decision["reason"])
                    self.assertIn("bbs foreman task done", decision["reason"])
                else:
                    self.assertEqual(result.stdout, "")

    def test_stop_gate_skips_sessions_outside_orca(self):
        result = self.run_hook("Stop", payload={"stop_hook_active": True})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, "")


if __name__ == "__main__":
    unittest.main()
