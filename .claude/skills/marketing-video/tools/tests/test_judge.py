"""The Micro Fish evaluator against a fake Soot and a fake micro-fish.sh: first-use model choice, the AI config
mvf owns, the brief's stimuli, the tally read back into a decision draft, and the gates that read it."""
import json
import os
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(TOOLS))
os.environ["MVF_TODAY"] = "2026-10-27"

from mvflib import checks, judge  # noqa: E402
from mvflib.common import Report  # noqa: E402
from test_extensions import mvf, repo  # noqa: E402
from test_mvf import script  # noqa: E402

FAKE_SOOT = """#!/usr/bin/env python3
import json, os, sys
a = sys.argv[1:]
if a[:1] == ["version"]:
    print("soot 9.9.9 (fake)")
elif a[:2] == ["providers", "--json"]:
    print(json.dumps([{"id": "devin", "credential": os.environ.get("FAKE_DEVIN", "available")},
                      {"id": "claude-code", "credential": "missing"}]))
elif a[:1] == ["check"]:
    print("ok workspace"); print("  agent sim-moderator model=" + os.environ.get("FAKE_MODEL", "?"))
else:
    sys.exit(2)
"""

# The tally is built from the key mvf wrote, so the test decides which script the crowd prefers.
FAKE_MF = """#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
home = Path(os.environ["MICRO_FISH_HOME"])
log = home.parent / "mf.log"
log.parent.mkdir(parents=True, exist_ok=True)
with log.open("a") as f:
    f.write(" ".join(sys.argv[1:]) + " port=" + os.environ["MICRO_FISH_PORT"] + " ai=" + os.environ["MICRO_FISH_AI_CONFIG"] + "\\n")
cmd = sys.argv[1]
if cmd == "setup":
    (home / "sim").mkdir(parents=True, exist_ok=True); (home / "sim/deployment.json").write_text("{}")
    print("workspace created")
elif cmd == "ask":
    runs = home / "runs"; runs.mkdir(parents=True, exist_ok=True)
    n = len(list(runs.glob("*.md")))
    (runs / f"{n}.md").write_text("# Report\\n" + Path(sys.argv[2]).read_text()[:80])
    (runs / f"{n}.progress").write_text("progress")
    print(f"state=completed conversation={sys.argv[3] if len(sys.argv) > 3 else 'conv-1'} answer={runs / f'{n}.md'}")
    print(f"progress={runs / f'{n}.progress'}")
elif cmd == "tally":
    key = json.loads(Path(os.environ["FAKE_KEY"]).read_text())["stimuli"]
    pref = json.loads(os.environ["FAKE_PREF"])
    rnd = {lab: {"n": 24, "attention_mean": 3.5, "watch_through_mean": pref[m["id"]], "clarity_top2": 60,
                 "intent_top2": pref[m["id"]] / 2, "share_rate": 0.2, "sentiment_mean": 0.1} for lab, m in key.items()}
    lead = max(key, key=lambda l: pref[key[l]["id"]])
    print(json.dumps({"sim": sys.argv[2], "complete": True, "by_round": {"0": rnd, "1": rnd},
                      "preference": {"leader": lead, "clear": True,
                                     "shares": {l: {"pct": pref[m["id"]]} for l, m in key.items()}},
                      "objections": {l: {"last": [["ad-feel", 3], ["unclear-term", 2]]} for l in key},
                      "quotes": {l: {"positive": [{"persona": "p1", "quote": "It says one filter shows hidden mail"}],
                                     "negative": []} for l in key},
                      "experts": {"blocking": [{"severity": "P1", "stimulus": lead, "issue": "CTA too early"}]},
                      "warnings": [], "caveat": "simulated"}))
"""


def write_exec(p: Path, text: str) -> Path:
    p.write_text(textwrap.dedent(text))
    p.chmod(0o755)
    return p


class Judge(unittest.TestCase):
    def setUp(self):
        self.d, self.root = repo()
        self.home = self.root / "bhome"
        sh = self.root / "soothome"
        sh.mkdir()
        (sh / "config.json").write_text(json.dumps({
            "providers": {"devin": {"type": "devin", "accounts": [{"id": "a-1"}], "key_env": ""},
                          "other": {"type": "openai", "key_env": "OTHER_KEY"}},
            "model": {"provider": "devin", "name": "swe-2"}, "tiers": {"flash": {"provider": "devin", "name": "swe-2"}}}))
        self.env = {"BABYSIT_HOME": str(self.home), "SOOT_HOME": str(sh),
                    "MVF_SOOT": str(write_exec(self.root / "soot", FAKE_SOOT)),
                    "MVF_MICRO_FISH": str(write_exec(self.root / "micro-fish.sh", FAKE_MF))}
        mvf(self.root, "init", "demo", "--product", "Acme")
        self.cd = self.root / "marketing/campaigns/demo"
        for sid in ("s01-a", "s02-b", "s00-old"):
            (self.cd / "05-scripts" / f"{sid}.json").write_text(json.dumps(script(id=sid)))

    def tearDown(self):
        self.d.cleanup()

    def run_mvf(self, *args, ok=True, **env):
        return mvf(self.cd, *args, ok=ok, env={**self.env, **env})

    def setup_judge(self):
        return self.run_mvf("judge", "setup", "--provider", "devin", "--model", "swe-2", FAKE_MODEL="devin/swe-2")

    def test_first_use_needs_a_model_choice_from_the_user(self):
        r = self.run_mvf("judge", "status", ok=False)
        self.assertEqual(r.returncode, judge.NEEDS_CONTEXT)
        self.assertIn("NEEDS model", r.stdout)
        self.assertIn("devin/swe-2", r.stdout)  # the user's Soot default is offered as a hint, not applied
        self.assertFalse((self.home / "marketing-video/soot-ai.json").exists())

    def test_signed_out_provider_asks_the_user_to_log_in(self):
        r = self.run_mvf("judge", "setup", "--provider", "devin", "--model", "swe-2", ok=False, FAKE_DEVIN="missing")
        self.assertEqual(r.returncode, judge.NEEDS_CONTEXT)
        self.assertIn("soot login devin", r.stdout)
        r = self.run_mvf("judge", "setup", "--provider", "devin", ok=False)
        self.assertIn("go together", r.stderr)

    def test_setup_writes_its_own_config_with_only_the_chosen_provider(self):
        r = self.setup_judge()
        self.assertIn("devin/swe-2", r.stdout)
        ai = json.loads((self.home / "marketing-video/soot-ai.json").read_text())
        self.assertEqual(list(ai["providers"]), ["devin"])
        self.assertEqual(ai["tiers"]["flash"], {"provider": "devin", "name": "swe-2"})
        self.assertEqual(ai["tiers"]["lite"], ai["model"])
        cfg = json.loads((self.home / "marketing-video/config.json").read_text())
        self.assertEqual(cfg["judge"]["model"], "swe-2")
        log = (self.home / "marketing-video/mf.log").read_text()
        self.assertIn("setup", log)
        self.assertIn("port=7811", log)  # never the user's own Micro Fish server
        self.assertEqual(self.run_mvf("judge", "status").returncode, 0)

    def test_dry_brief_dictates_ids_and_keeps_every_voice_line(self):
        self.setup_judge()
        self.run_mvf("judge", "scripts", "s01-a", "s02-b", "--control", "s00-old", "--dry")
        out = self.cd / "06-pretest/r1/microfish"
        text = (out / "brief.txt").read_text()
        key = json.loads((out / "key.json").read_text())
        self.assertEqual(sorted(key["stimuli"]), ["a", "b", "c"])
        self.assertEqual(sorted(v["kind"] for v in key["stimuli"].values()), ["candidate", "candidate", "control"])
        self.assertIn("sim id `demo-r1`", text)
        self.assertIn('VOICE: "Your inbox is lying."', text)
        self.assertNotIn("s00-old", text)  # the panel never sees which one is the control
        self.assertNotIn("learns a trick", text)  # nor the writer's intent

    def test_long_visuals_are_shortened_before_any_voice(self):
        rows = [(i, i + 1, "a very long picture description " * 20, "", (f"line {i} " * 8).strip()) for i in range(12)]
        text = judge.stimulus(rows, 12.0, "measured", "cap", "tiktok", False)
        self.assertLessEqual(len(text), judge.STIMULUS_CHARS)
        for i in range(12):
            self.assertIn(f'"{(f"line {i} " * 8).strip()}"', text)

    def test_a_brief_over_sots_byte_limit_shortens_pictures_not_voice(self):
        # Vietnamese letters are 2-3 bytes: four 3000-character stimuli passed the character cap and Soot refused it
        build = lambda chars: "ữ" * (chars * 2) + " GIỌNG: kept"
        text = judge.fit(build)
        self.assertLessEqual(len(text.encode("utf-8")), judge.MESSAGE_BYTES)
        self.assertTrue(text.endswith("kept"))
        with self.assertRaises(SystemExit):
            judge.fit(lambda chars: "ữ" * judge.MESSAGE_BYTES)

    def test_vietnamese_campaign_gets_a_vietnamese_brief(self):
        self.setup_judge()
        c = json.loads((self.cd / "campaign.json").read_text())
        c["languages"] = ["vi"]
        (self.cd / "campaign.json").write_text(json.dumps(c))
        self.run_mvf("judge", "scripts", "s01-a", "--dry")
        self.assertIn("GIỌNG", (self.cd / "06-pretest/r1/microfish/brief.txt").read_text())

    def run_scripts(self, pref):
        self.setup_judge()
        key = self.cd / "06-pretest/r1/microfish/key.json"
        return self.run_mvf("judge", "scripts", "s01-a", "s02-b", "--control", "s00-old",
                            FAKE_KEY=str(key), FAKE_PREF=json.dumps(pref))

    def test_run_reads_the_tally_into_a_decision_draft_the_gate_holds(self):
        r = self.run_scripts({"s01-a/H1": 50, "s02-b/H1": 30, "s00-old/H1": 20})
        out = self.cd / "06-pretest/r1/microfish"
        j = json.loads((out / "judge.json").read_text())
        self.assertEqual(j["winner"], "s01-a/H1")
        self.assertTrue(j["beats_control"])
        self.assertTrue((out / "interview.md").is_file())  # the leaders were interviewed in the same conversation
        self.assertIn("ask", (self.home / "marketing-video/mf.log").read_text().splitlines()[-2] + " conv-1")
        self.assertIn("one filter shows hidden mail", (out / "retell.md").read_text())
        dec = json.loads((self.cd / "06-pretest/r1/decision.json").read_text())
        self.assertEqual(dec["method"], "simulated-panel")
        self.assertIn("micro-fish:demo-r1", dec["run"])
        self.assertIn("ad-feel", dec["evidence"])
        self.assertTrue(all("CTA too early" in o or o.startswith("crowd:") for o in dec["objections"]))  # winner's only
        rep = checks.check_pretest(self.cd / "06-pretest/r1/decision.json")
        fails = [i["code"] for i in rep.items if i["level"] == "FAIL"]
        self.assertIn("PRE3", fails)  # no retell written yet
        self.assertIn("PRE", fails)   # the draft is unread
        dec.pop("_todo")
        dec["retell"] = "p1: 'It says one filter shows hidden mail' (matches the takeaway)"
        (self.cd / "06-pretest/r1/decision.json").write_text(json.dumps(dec))
        rep = checks.check_pretest(self.cd / "06-pretest/r1/decision.json")
        self.assertEqual([i for i in rep.items if i["level"] == "FAIL"], [])
        self.assertIn("decision draft", r.stdout)

    def test_a_candidate_the_analyst_calls_the_control_is_named(self):
        key = {"a": {"id": "ctl-x/H1", "kind": "control"}, "b": {"id": "s02/H1", "kind": "candidate"},
               "d": {"id": "s03/H1", "kind": "candidate"}}
        answer = "Nói thẳng: đối chứng d xem hết ngang b.\n| 3 | d (đối chứng, đã loại) |\na thua cả đối chứng."
        self.assertEqual(judge.mislabelled_controls(answer, key), ["d"])
        self.assertEqual(judge.mislabelled_controls("a (control) loses to b", key), [])

    def test_a_control_that_wins_means_no_winner(self):
        self.run_scripts({"s01-a/H1": 30, "s02-b/H1": 20, "s00-old/H1": 50})
        j = json.loads((self.cd / "06-pretest/r1/microfish/judge.json").read_text())
        self.assertIsNone(j["winner"])
        self.assertIn("cannot tell", j["why"])

    def test_existing_decision_is_never_overwritten(self):
        (self.cd / "06-pretest/r1").mkdir(parents=True)
        (self.cd / "06-pretest/r1/decision.json").write_text(json.dumps({"method": "human-panel", "winner": "x"}))
        self.run_scripts({"s01-a/H1": 50, "s02-b/H1": 30, "s00-old/H1": 20})
        self.assertEqual(json.loads((self.cd / "06-pretest/r1/decision.json").read_text())["winner"], "x")
        self.assertTrue((self.cd / "06-pretest/r1/microfish/decision.suggested.json").is_file())

    def test_the_cut_is_judged_with_what_review_saw(self):
        self.setup_judge()
        mvf(self.cd, "scaffold", "s01-a.json", "--take", "none")
        proj = self.cd / "07-video/s01-a/9x16"
        (proj / "review.json").write_text(json.dumps({"seen": [{"at": 0.2, "what": "a red inbox badge reading 99"}],
                                                      "issues": []}))
        key = proj / "judge/v1/key.json"
        self.run_mvf("judge", "video", "07-video/s01-a/9x16", FAKE_KEY=str(key), FAKE_PREF=json.dumps({"s01-a/H1": 70}))
        out = proj / "judge/v1"
        self.assertIn("a red inbox badge reading 99", (out / "brief.txt").read_text())  # the cut, not only the plan
        j = json.loads((out / "judge.json").read_text())
        self.assertEqual((j["complete"], j["summary"]["watch_through"]), (True, 70))
        self.assertIn("one filter shows hidden mail", (out / "retell.md").read_text())
        rep = Report()
        judge.check_video_judge(proj, rep)
        self.assertNotIn("WARN", [i["level"] for i in rep.items])

    def test_more_than_four_videos_is_refused(self):
        self.setup_judge()
        for sid in ("s03-c", "s04-d"):
            (self.cd / "05-scripts" / f"{sid}.json").write_text(json.dumps(script(id=sid)))
        r = self.run_mvf("judge", "scripts", "s01-a", "s02-b", "s03-c", "s04-d", "--control", "s00-old", "--dry", ok=False)
        self.assertIn("at most 4", r.stderr)


class VideoJudgeGate(unittest.TestCase):
    def test_unjudged_cut_warns_and_expert_flags_are_reported(self):
        with tempfile.TemporaryDirectory() as d:
            proj = Path(d)
            rep = Report()
            judge.check_video_judge(proj, rep)
            self.assertEqual([i["code"] for i in rep.items if i["level"] == "WARN"], ["JV1"])
            (proj / "judge/v1").mkdir(parents=True)
            (proj / "judge/v1/judge.json").write_text(json.dumps({"summary": {"expert_blocking": ["P1: CTA too early"]}}))
            rep = Report()
            judge.check_video_judge(proj, rep)
            self.assertEqual([i["level"] for i in rep.items], ["INFO"])


if __name__ == "__main__":
    unittest.main()
