"""Lint reads what the script says, not only its labels: first value from content, the retell in the vo,
planned shots, spoken questions, answered WARNs."""
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from mvflib.lint import lint  # noqa: E402
from test_mvf import CAMPAIGN, REFS, codes, script  # noqa: E402


def long_script(**over):
    beats = [{"id": "B01", "role": "hook", "vo": "Your inbox is lying.", "onscreen": "your inbox lies", "visual": "a phone"}]
    beats += [{"id": f"B0{i}", "role": "context", "vo": "Some setup words that fill a little time here.",
               "onscreen": "", "visual": "x"} for i in range(2, 5)]
    beats += [{"id": "B05", "role": "context", "vo": "One filter shows the mail your inbox hides.", "onscreen": "",
               "visual": "screen"},
              {"id": "B06", "role": "cta", "vo": "Try Acme.", "onscreen": "", "visual": "end"}]
    return script(beats=beats, **over)


class FirstValueFromContent(unittest.TestCase):
    def test_a_context_beat_that_says_the_takeaway_is_value(self):
        s = long_script()
        r1 = [i for i in lint(s, CAMPAIGN, REFS).items if i["code"] == "R1"]
        self.assertTrue(r1 and "B05" in r1[0]["msg"], r1)

    def test_no_value_anywhere_is_not_measured(self):
        s = long_script()
        s["beats"][4]["vo"] = "More setup words."
        self.assertIn("R1", codes(lint(s, CAMPAIGN, REFS), "WARN"))


class RetellInVo(unittest.TestCase):
    def test_takeaway_missing_from_vo_warns(self):
        s = script()
        s["spine"]["takeaway"] = "Archive old newsletters weekly. Unsubscribe from promotions monthly."
        self.assertIn("W5", codes(lint(s, CAMPAIGN, REFS), "WARN"))

    def test_takeaway_said_in_vo_passes(self):
        s = script()
        s["beats"][1]["vo"] = "One filter shows the mail your inbox hides."
        self.assertNotIn("W5", codes(lint(s, CAMPAIGN, REFS)))


class Shots(unittest.TestCase):
    def test_planned_shots_clear_v2(self):
        s = script()
        s["beats"][1]["vo"] = " ".join(["word"] * 25)
        self.assertIn("V2", codes(lint(s, CAMPAIGN, REFS), "WARN"))
        s["beats"][1]["shots"] = ["wide", "push", "cut", "close", "reveal"]
        self.assertNotIn("V2", codes(lint(s, CAMPAIGN, REFS), "WARN"))


class Answered(unittest.TestCase):
    def test_warn_notes_turn_a_warn_into_answered_info(self):
        s = script()
        s["beats"][1]["vo"] = " ".join(["word"] * 25)
        s["warn_notes"] = {"V2": "one continuous screen take; the cursor moves"}
        rep = lint(s, CAMPAIGN, REFS)
        self.assertNotIn("V2", codes(rep, "WARN"))
        self.assertTrue(any(i["code"] == "V2" and i["msg"].startswith("answered:") for i in rep.items))


class SpokenQuestions(unittest.TestCase):
    def test_a_spoken_question_counts_as_a_reason_to_stay(self):
        s = long_script()
        s["spine"]["loops"] = [{"id": "L1", "q": "q", "open": "B01", "close": "B02"}]
        for b in s["beats"][1:5]:
            b["vo"] = " ".join(["word"] * 14)
        before = codes(lint(s, CAMPAIGN, REFS), "WARN").count("N4")
        s["beats"][2]["vo"] = " ".join(["word"] * 13) + " why?"
        self.assertLessEqual(codes(lint(s, CAMPAIGN, REFS), "WARN").count("N4"), before)


if __name__ == "__main__":
    unittest.main()
