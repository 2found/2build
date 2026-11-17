"""Angle freshness (mvflib/fresh.py): prediction and overlap briefs, OV1/OV2/SW1."""
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mvflib import fresh  # noqa: E402


def angle(i, **kw):
    return {"id": i, "promise": f"promise {i}", "format": "explainer", "hooks": ["h1", "h2"], **kw}


class Fresh(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        root = Path(self.tmp.name)
        self.cd = root / "new"
        (self.cd / "04-strategy").mkdir(parents=True)
        (self.cd / "campaign.json").write_text(json.dumps({"product": "Acme", "goal": "awareness", "past": ["a talk"],
                                                            "format": "answer", "platforms": ["tiktok"]}))
        old = root / "old"
        (old / "04-strategy").mkdir(parents=True)
        (old / "05-scripts").mkdir()
        (old / "campaign.json").write_text(json.dumps({"product": "Acme"}))
        (old / "04-strategy/angles.json").write_text(json.dumps({"angles": [angle("G1", promise="the old promise")]}))
        (old / "05-scripts/s01.json").write_text(json.dumps({"id": "s01", "spine": {"payoff": "the old payoff"}}))
        (old / "05-scripts/broken.json").write_text("{")
        (self.cd / "04-strategy/angles.json").write_text(json.dumps({"angles": [angle("G1"), angle("G2")]}))

    def tearDown(self):
        self.tmp.cleanup()

    def codes(self, angles):
        return [(i["code"], i["where"]) for i in fresh.check(self.cd, angles).items]

    def test_predict_brief_has_only_the_brief(self):
        b = fresh.predict_brief(json.loads((self.cd / "campaign.json").read_text()))
        self.assertIn("P1–P5", b)
        self.assertIn(fresh.PREDICT, b)
        self.assertNotIn("a talk", b)  # past videos belong to the overlap check, not the prediction

    def test_a_delivered_script_is_marked_made(self):
        (self.cd.parent / "old/08-delivery/s01").mkdir(parents=True)
        self.assertIn("old/s01 (made): the old payoff", fresh.past_videos(self.cd))

    def test_predict_brief_names_the_subject_from_the_notes(self):
        b = fresh.predict_brief({"product": "Acme", "constraints": ["owner brief: the novel Foo, audio plug"]})
        self.assertIn("the novel Foo", b)

    def test_overlap_brief_lists_past_videos(self):
        b = fresh.overlap_brief(self.cd)
        for x in ("old/G1 (angle): the old promise", "old/s01 (script only, never made): the old payoff", "(this owner) a talk", "no prediction yet"):
            self.assertIn(x, b)
        (self.cd / fresh.PREDICT).write_text("P1. the obvious one")
        self.assertIn("P1. the obvious one", fresh.overlap_brief(self.cd))

    def test_missing_overlap_and_swap(self):
        got = self.codes([angle("G1")])
        self.assertIn(("OV1", "angles"), got)
        self.assertIn(("SW1", "angles.G1"), got)
        self.assertNotIn("SW1", [c for c, _ in self.codes([angle("G1", swap="with any app it still works")])])

    def test_overlap_verdicts(self):
        (self.cd / fresh.OVERLAP).write_text(json.dumps({"G1": {"verdict": "OVERLAP", "with": ["P1"], "what": "same"},
                                                         "G2": {"verdict": "clear"}}))
        got = self.codes([angle("G1", swap="x"), angle("G2", swap="x"), angle("G3", swap="x")])
        self.assertEqual(got, [("OV2", "angles.G1"), ("OV1", "angles.G3")])
        got = self.codes([angle("G1", swap="x", overlap_note="our take answers it with the novel's own lines")])
        self.assertEqual(got, [])

    def test_a_sibling_with_odd_json_is_skipped(self):
        odd = self.cd.parent / "odd"
        (odd / "04-strategy").mkdir(parents=True)
        (odd / "05-scripts").mkdir()
        (odd / "campaign.json").write_text("[1]")
        (odd / "04-strategy/angles.json").write_text(json.dumps({"angles": ["G1", angle("G9", promise="kept")]}))
        (odd / "05-scripts/s01.json").write_text("[]")
        past = fresh.past_videos(self.cd)
        self.assertTrue(any(p.startswith("odd/G9 (angle): kept") for p in past), past)

    def test_malformed_overlap_is_a_warn_not_a_crash(self):
        for bad in ("{\"G1\": [1,", "[1, 2]"):
            (self.cd / fresh.OVERLAP).write_text(bad)
            got = self.codes([angle("G1", swap="x")])
            self.assertEqual(got, [("OV1", fresh.OVERLAP)])


if __name__ == "__main__":
    unittest.main()
