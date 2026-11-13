"""The Micro Fish evaluator against a fake Soot and a fake micro-fish.sh: first-use model choice, the AI config
mvf owns, the brief mvf writes, the tally read back into a decision draft, and the gates that read it."""
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
    print("ok workspace"); print("  agent sim-analyst model=" + os.environ.get("FAKE_MODEL", "?"))
else:
    sys.exit(2)
"""

# The tally is built from the key mvf wrote, so the test decides which script the audience finishes.
FAKE_MF = """#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
home = Path(os.environ["MICRO_FISH_HOME"])
log = home.parent / "mf.log"
log.parent.mkdir(parents=True, exist_ok=True)
with log.open("a") as f:
    f.write(" ".join(sys.argv[1:]) + " port=" + os.environ["MICRO_FISH_PORT"] + " ai=" + os.environ["MICRO_FISH_AI_CONFIG"] + "\\n")
cmd = sys.argv[1]
rate = lambda pct, n=24: {"k": round(pct * n / 100), "pct": pct, "ci95": [max(0, pct - 15), min(100, pct + 15)]}
if cmd == "setup":
    (home / "society").mkdir(parents=True, exist_ok=True); (home / "society/deployment.json").write_text("{}")
    print("workspace created")
elif cmd == "run":
    brief = json.loads(Path(sys.argv[2]).read_text())
    print("round 1/3: 24 awake, 24 completed", file=sys.stderr)
    if os.environ.get("FAKE_STOPPED"):
        print(json.dumps({"sim": brief["sim"], "state": "stopped", "phase": "round 2", "reason": "most sessions failed"}))
        sys.exit(2)
    runs = home / "runs"; runs.mkdir(parents=True, exist_ok=True)
    (runs / "report.md").write_text("# Report\\n" + brief["goal"])
    print(json.dumps({"sim": brief["sim"], "state": "completed", "sessions": 60, "failed": 0, "report": str(runs / "report.md")}))
elif cmd == "tally":
    key = json.loads(Path(os.environ["FAKE_KEY"]).read_text())["stimuli"]
    pref = json.loads(os.environ["FAKE_PREF"])
    lead = max(key, key=lambda l: pref[key[l]["id"]])
    subj = {lab: {"assigned": 24, "reached": 24, "full": rate(5), "depth": {"n": 24, "pct": pref[m["id"]], "ci95": [pref[m["id"]] - 8, pref[m["id"]] + 8]}, "stopped": rate(min(100, pref[m["id"]] + 20)),
                  "like": rate(10), "share": rate(0), "comment": rate(4), "rank_in_feed": 3, "of": 17,
                  "comments": [{"who": "Lan", "text": "hay"}],
                  "interview": {"answered": 20, "clarity": {"top2": rate(60, 20)}, "intent": {"top2": rate(pref[m["id"]] / 2, 20)},
                                "objections": {"tags": [["ad-feel", 3], ["unclear-term", 2]]},
                                "retell": {"answers": [{"who": "Lan", "attention": "full", "text": "It says one filter shows hidden mail"}]}}}
            for lab, m in key.items()}
    print(json.dumps({"sim": sys.argv[2], "complete": True, "subjects": subj,
                      "comparison": {"metric": "depth", "leader": lead, "clear": True} if len(key) > 1 else None,
                      "feed_rank": [{"rank": 1, "post": "c1"}],
                      "experts": {"blocking": [{"severity": "P1", "subject": lead, "issue": "CTA too early", "fix": "move it"}]},
                      "warnings": ["a: reached 24 personas"], "caveat": "simulated"}))
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
        self.language("vi")  # Micro Fish ships a Vietnamese persona library

    def language(self, lang):
        c = json.loads((self.cd / "campaign.json").read_text())
        c["languages"] = [lang]
        (self.cd / "campaign.json").write_text(json.dumps(c))

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

    def test_dry_brief_gives_each_video_one_arm_and_keeps_every_voice_line(self):
        self.setup_judge()
        self.run_mvf("judge", "scripts", "s01-a", "s02-b", "--control", "s00-old", "--dry")
        out = self.cd / "06-pretest/r1/microfish"
        text = (out / "brief.json").read_text()
        b = json.loads(text)
        key = json.loads((out / "key.json").read_text())
        self.assertEqual(sorted(key["stimuli"]), ["a", "b", "c"])
        self.assertEqual(sorted(v["kind"] for v in key["stimuli"].values()), ["candidate", "candidate", "control"])
        self.assertEqual((b["sim"], b["platform"], b["language"]), ("demo-r1", "TikTok", "vi"))
        self.assertEqual([x["id"] for x in b["subjects"]], ["a", "b", "c"])
        self.assertEqual({x["account"] for x in b["subjects"]}, {"Acme"})  # nothing but the video tells the arms apart
        self.assertEqual(b["audience"], {"age": [18, 45], "n": 72, "cap": True})  # quick: 24 personas per video
        self.assertIn('GIỌNG: \\"Your inbox is lying.\\"', text)
        self.assertEqual([q["id"] for q in b["questions"]][:3], ["retell", "clarity", "intent"])
        self.assertEqual(len(b["experts"]), 4)
        for hidden in ("s00-old", "control", "đối chứng", "learns a trick"):  # the key and the writer's intent stay out
            self.assertNotIn(hidden, text)

    def test_long_visuals_are_shortened_before_any_voice(self):
        rows = [(i, i + 1, "a very long picture description " * 20, "", (f"line {i} " * 8).strip()) for i in range(12)]
        text = judge.stimulus(rows, 12.0, "measured", "cap", "tiktok", False, chars=3000)
        self.assertLessEqual(len(text), 3000)
        for i in range(12):
            self.assertIn(f'"{(f"line {i} " * 8).strip()}"', text)

    def test_a_three_minute_cut_is_one_subject_with_every_voice_line(self):
        # a 3-minute Vietnamese cut measured 5268 characters: Micro Fish 1 refused it, then split it into parts
        rows = [(i * 4.0, i * 4.0 + 4, "một hình " * 6, "", (f"câu {i} " * 22).strip()) for i in range(45)]
        text = judge.stimulus(rows, 180.0, "measured", "cap", "tiktok", True)
        self.assertLessEqual(len(text), judge.STIMULUS_CHARS)
        for i in range(45):
            self.assertEqual(text.count(f'"{(f"câu {i} " * 22).strip()}"'), 1)
        with self.assertRaises(SystemExit):
            judge.stimulus(rows * 4, 720.0, "measured", "", "tiktok", True)

    def test_the_campaign_names_its_audience_and_another_language_needs_its_own_library(self):
        self.setup_judge()
        (self.cd / "02-audience").mkdir(exist_ok=True)
        (self.cd / "02-audience/segments.json").write_text(json.dumps({"segments": [], "panel": {"age": [18, 30], "zone": "Đô Thị"}}))
        self.run_mvf("judge", "scripts", "s01-a", "--dry")
        b = json.loads((self.cd / "06-pretest/r1/microfish/brief.json").read_text())
        self.assertEqual(b["audience"], {"age": [18, 30], "zone": "Đô Thị", "n": 24, "cap": True})
        self.language("en")
        r = self.run_mvf("judge", "scripts", "s01-a", "--dry", ok=False)
        self.assertIn("judge.library", r.stderr)
        feed = self.root / "feed.json"
        feed.write_text(json.dumps([{"account": "cats", "content": "A cat video"}]))
        cfg = self.home / "marketing-video/config.json"
        c = json.loads(cfg.read_text())
        c["judge"].update({"library": "/lib/en.jsonl", "context": str(feed)})
        cfg.write_text(json.dumps(c))
        self.run_mvf("judge", "scripts", "s01-a", "--dry")
        b = json.loads((self.cd / "06-pretest/r1/microfish/brief.json").read_text())
        self.assertEqual((b["language"], b["audience"]["library"], b["context"][0]["account"]), ("en", "/lib/en.jsonl", "cats"))
        self.assertIn("VOICE", b["subjects"][0]["content"])

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
        self.assertEqual((j["stimuli"]["a"]["n"], j["stimuli"]["a"]["rank_in_feed"], j["stimuli"]["a"]["interviewed"]), (24, "3/17", 20))
        self.assertIn("# Report", (out / "answer.md").read_text())
        log = (self.home / "marketing-video/mf.log").read_text().splitlines()
        self.assertTrue(any(ln.startswith("run ") for ln in log) and log[-1].startswith("tally demo-r1"), log)
        self.assertIn("[full] Lan: It says one filter shows hidden mail", (out / "retell.md").read_text())
        dec = json.loads((self.cd / "06-pretest/r1/decision.json").read_text())
        self.assertEqual(dec["method"], "simulated-panel")
        self.assertIn("micro-fish:demo-r1", dec["run"])
        self.assertIn("ad-feel", dec["evidence"])
        self.assertTrue(all("CTA too early" in o or o.startswith("audience:") for o in dec["objections"]))  # winner's only
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

    def test_a_run_the_provider_stopped_keeps_its_record_and_resumes(self):
        self.setup_judge()
        key = self.cd / "06-pretest/r1/microfish/key.json"
        pref = json.dumps({"s01-a/H1": 50, "s00-old/H1": 20})
        r = self.run_mvf("judge", "scripts", "s01-a", "--control", "s00-old", ok=False, FAKE_STOPPED="1", FAKE_KEY=str(key), FAKE_PREF=pref)
        self.assertIn("Micro Fish stopped: most sessions failed", r.stderr)
        self.assertIn("mvf judge resume", r.stderr)
        out = self.cd / "06-pretest/r1/microfish"
        self.assertFalse((out / "judge.json").exists())  # nothing is decided on half a feed
        self.assertIn("round 1/3", (out / "progress.log").read_text())
        self.run_mvf("judge", "resume", FAKE_KEY=str(key), FAKE_PREF=pref)
        self.assertEqual(json.loads((out / "judge.json").read_text())["winner"], "s01-a/H1")

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
        b = json.loads((out / "brief.json").read_text())
        self.assertIn("a red inbox badge reading 99", b["subjects"][0]["content"])  # the cut, not only the plan
        self.assertEqual((len(b["subjects"]), b["audience"]["n"], b["experts"][0]["id"]), (1, 24, "editor"))
        j = json.loads((out / "judge.json").read_text())
        self.assertEqual((j["complete"], j["summary"]["watch_pct"], j["summary"]["stopped"]), (True, 70, 90))
        s = j["summary"]  # few finish a short video: the cut is read on watch time over length, likes and comments
        self.assertEqual((s["finished"], s["like_rate"], s["comment_rate"]), (5, 10, 4))
        self.assertAlmostEqual(s["avg_watch_s"], round(0.7 * s["duration_s"], 1))
        self.assertEqual(b["metric"], "depth")
        self.assertEqual(j["summary"]["expert_blocking"], ["P1: CTA too early → move it"])
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
