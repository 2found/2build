"""The owner's path: `mvf next` names one action and who takes it, `mvf sheet` is the page the owner reads,
`mvf approve` records their word, and voicing waits for it."""
import json
import os
import sys
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(TOOLS))
os.environ["MVF_TODAY"] = "2026-10-27"

from mvflib import flow  # noqa: E402
from mvflib.common import Report  # noqa: E402
from test_extensions import mvf, repo  # noqa: E402
from test_mvf import script  # noqa: E402


class Owner(unittest.TestCase):
    def setUp(self):
        self.d, self.root = repo()
        self.home = self.root / "bhome"
        os.environ["BABYSIT_HOME"] = str(self.home)
        mvf(self.root, "init", "demo", "--product", "Acme")
        self.cd = self.root / "marketing/campaigns/demo"
        c = json.loads((self.cd / "campaign.json").read_text())
        c.pop("_template")
        (self.cd / "campaign.json").write_text(json.dumps(c))
        self.write("s01-test", voice={"mode": "recorded"})

    def tearDown(self):
        os.environ.pop("BABYSIT_HOME", None)
        self.d.cleanup()

    def write(self, sid, **over):
        (self.cd / "05-scripts" / f"{sid}.json").write_text(json.dumps(script(id=sid, **over)))

    def nxt(self):
        return flow.next_action(self.cd, lambda st: Report(), lambda sid: 0)

    def test_voice_waits_for_the_owner(self):
        r = mvf(self.cd, "voice", "s01-test", ok=False)
        self.assertIn("BLOCKED owner", r.stderr)
        self.assertIn("mvf sheet s01-test", r.stderr)
        r = mvf(self.cd, "approve", "s01-test", "--verdict", "rejected", ok=False)
        self.assertIn("reason", r.stderr)
        mvf(self.cd, "approve", "s01-test", "--verdict", "rejected", "--note", "the hook teases, say the answer")
        self.assertIn("say the answer", mvf(self.cd, "voice", "s01-test", ok=False).stderr)
        # an explicit skip passes the gate (it then needs recordings: the files engine lists them)
        r = mvf(self.cd, "voice", "s01-test", "--owner-skip", "test take", ok=False)
        self.assertNotIn("BLOCKED owner", r.stderr + r.stdout)
        mvf(self.cd, "approve", "s01-test", "--verdict", "delegated")
        self.assertNotIn("BLOCKED owner", (lambda r: r.stderr + r.stdout)(mvf(self.cd, "voice", "s01-test", ok=False)))

    def test_an_edit_after_approval_is_flagged(self):
        flow.approve(self.cd, "s01-test", "approved", "", None)
        self.assertIsNone(flow.owner_drift(self.cd, "s01-test"))
        self.write("s01-test", value="something else")
        self.assertIn("changed after", flow.owner_drift(self.cd, "s01-test"))

    def test_the_owner_reads_the_campaigns_first_script_only(self):
        self.write("s02-b")
        self.assertIsNotNone(flow.owner_gate(self.cd, "s02-b", None))
        (self.cd / "08-delivery/s01-test").mkdir(parents=True)  # one video shipped: the owner has read the campaign
        self.assertIsNone(flow.owner_gate(self.cd, "s01-test", None))
        self.assertIsNone(flow.owner_gate(self.cd, "s02-b", None))
        flow.approve(self.cd, "s02-b", "rejected", "off brand", None)  # a rejection still holds
        self.assertIn("off brand", flow.owner_gate(self.cd, "s02-b", None))

    def test_next_walks_scripts_evaluator_pretest_owner_voice(self):
        x = self.nxt()
        self.assertEqual((x["who"], x["run"]), ("agent", "mvf new-script <sid>"))
        self.write("s02-b", voice={"mode": "recorded"})
        self.write("s03-c")
        self.write("ctl-old")
        x = self.nxt()
        self.assertEqual(x["who"], "owner")  # first use: the owner picks the evaluator model
        self.assertIn("evaluator", x["what"])
        (self.home / "marketing-video").mkdir(parents=True)
        (self.home / "marketing-video/config.json").write_text(json.dumps({"judge": {"provider": "p", "model": "m"}}))
        x = self.nxt()
        self.assertEqual(x["run"], "mvf judge scripts s01-test s02-b s03-c --control ctl-old")
        (self.cd / "06-pretest/r1").mkdir(parents=True)
        (self.cd / "06-pretest/r1/decision.json").write_text(json.dumps(
            {"method": "simulated-panel", "winner": "s02-b/H1", "evidence": "pref 50%", "retell": "q", "decided_at": "2026-10-27"}))
        x = self.nxt()
        self.assertEqual((x["who"], x["run"]), ("owner", "mvf sheet s02-b"))
        flow.approve(self.cd, "s02-b", "rejected", "too slow", None)
        self.assertIn("rewrite s02-b", self.nxt()["what"])
        flow.approve(self.cd, "s02-b", "approved", "", None)
        x = self.nxt()
        self.assertEqual((x["who"], x["run"]), ("agent", "mvf voice s02-b"))

    def test_template_brief_comes_first_and_asks_the_owner_only_for_gaps(self):
        c = json.loads((self.cd / "campaign.json").read_text())
        c["_template"] = True
        (self.cd / "campaign.json").write_text(json.dumps(c))
        x = self.nxt()
        self.assertEqual(x["who"], "agent")
        self.assertIn("only for what the request leaves out", x["what"])

    def test_sheet_reads_like_a_page_in_the_campaign_language(self):
        c = json.loads((self.cd / "campaign.json").read_text())
        c["languages"] = ["vi"]
        (self.cd / "campaign.json").write_text(json.dumps(c))
        r = mvf(self.cd, "sheet", "s01-test")
        self.assertIn("| s | Hình | Chữ | Lời |", r.stdout)
        self.assertIn("Your inbox is lying.", r.stdout)
        self.assertIn("one filter shows the mail your inbox hides", r.stdout)
        self.assertIn("mvf approve s01-test --verdict approved", r.stdout)
        self.assertTrue((self.cd / "06-pretest/sheet-s01-test.md").is_file())

    def test_a_control_is_a_valid_script_id_and_is_not_lint_gated(self):
        self.write("ctl-list-names")
        self.assertNotIn("S0", mvf(self.cd, "lint", "ctl-list-names", ok=False).stdout)
        self.assertTrue(flow.is_control("ctl-list-names"))

    def test_a_newer_round_supersedes_an_older_one(self):
        for rnd, d in (("r1", {"method": "simulated-panel", "winner": "s01-test/H1", "evidence": "x", "run": "r",
                               "decided_at": "2026-10-01"}),  # no retell: FAILs while current
                       ("r2", {"method": "simulated-panel", "winner": "s01-test/H1", "evidence": "x", "run": "r",
                               "retell": "q", "changes": [], "decided_at": "2026-10-20"})):
            (self.cd / "06-pretest" / rnd).mkdir(parents=True)
            (self.cd / "06-pretest" / rnd / "decision.json").write_text(json.dumps(d))
        r = mvf(self.cd, "check", "pretest")
        self.assertIn("superseded by r2", r.stdout)

    def test_sheet_shows_the_panel_as_a_table(self):
        (self.cd / "06-pretest/r1/microfish").mkdir(parents=True)
        (self.cd / "06-pretest/r1/decision.json").write_text(json.dumps(
            {"method": "simulated-panel", "winner": "s01-test/H1", "evidence": "e", "retell": "they said X",
             "changes": ["split beat 2"], "run": "r", "decided_at": "2026-10-27"}))
        row = lambda i, k, p: {"id": i, "kind": k, "n": 24, "preference_pct": p, "watch_through": 70, "clarity_top2": 60,
                               "intent_top2": 40, "ad_feel": 2, "expert_blocking": ["P1: beat 2 is dense"] if k == "candidate" else []}
        (self.cd / "06-pretest/r1/microfish/judge.json").write_text(json.dumps(
            {"clear": True, "beats_control": True, "stimuli": {"a": row("s01-test/H1", "candidate", 70), "b": row("ctl-x/H1", "control", 5)}}))
        out = mvf(self.cd, "sheet", "s01-test").stdout
        self.assertIn("| **s01-test** | 70% |", out)
        self.assertIn("ctl-x (control) | 5%", out)
        self.assertIn("- split beat 2", out)
        self.assertIn("- P1: beat 2 is dense", out)

    def test_every_reference_the_tool_names_exists(self):
        import re
        skill = TOOLS.parent
        src = "".join(p.read_text() for p in [TOOLS / "mvf.py", *sorted((TOOLS / "mvflib").glob("*.py"))])
        for f, anchor in set(re.findall(r"references/([0-9a-z-]+\.md)(?:#([a-z0-9-]+))?", src)):
            text = (skill / "references" / f).read_text()  # raises when the file is gone
            if anchor:
                slugs = {re.sub(r"[^a-z0-9 -]", "", h.strip().lower()).replace(" ", "-")
                         for h in re.findall(r"^#+ (.*)$", text, re.M)}
                self.assertIn(anchor, slugs, f"references/{f}#{anchor}")

    def test_next_cli_prints_who(self):
        r = mvf(self.cd, "next", "--json", env={"BABYSIT_HOME": str(self.home)})
        self.assertIn(json.loads(r.stdout)["who"], {"agent", "owner"})


if __name__ == "__main__":
    unittest.main()
