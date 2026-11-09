"""Script craft advice (mvflib/craft.py): each code fires on the shape it names and stays quiet otherwise."""
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mvflib import flow, timing  # noqa: E402
from mvflib.lint import lint  # noqa: E402
from test_mvf import CAMPAIGN, REFS, codes, script  # noqa: E402


def beats4(**b4):
    """hook, demo, payoff, then a fourth beat (default a CTA)."""
    return [
        {"id": "B01", "role": "hook", "vo": "Your inbox is lying.", "onscreen": "your inbox lies", "visual": "a phone"},
        {"id": "B02", "role": "demo", "vo": "Watch this filter sort it.", "onscreen": "one filter", "visual": "screen"},
        {"id": "B03", "role": "payoff", "vo": "One filter shows the mail it hides.", "onscreen": "see it all",
         "visual": "inbox"},
        {"id": "B04", "role": "cta", "vo": "Try Acme today.", "onscreen": "acme.app", "visual": "logo", **b4},
    ]


def where(rep, code):
    return [i["where"] for i in rep.items if i["code"] == code and i["level"] == "WARN"]


class Ending(unittest.TestCase):
    def test_new_content_after_payoff(self):
        bs = beats4(role="proof", vo="Also, it sorts by sender.")
        self.assertEqual(where(lint(script(beats=bs), CAMPAIGN, REFS), "E1"), ["s01-test.B04"])

    def test_cta_after_payoff_is_fine(self):
        self.assertNotIn("E1", codes(lint(script(beats=beats4()), CAMPAIGN, REFS)))

    def test_payoff_needs_room_before_the_cta(self):
        bs = beats4()
        self.assertIn("E2", codes(lint(script(beats=bs), CAMPAIGN, REFS), "WARN"))
        bs[2]["hold"] = 1.5
        self.assertNotIn("E2", codes(lint(script(beats=bs), CAMPAIGN, REFS), "WARN"))

    def test_last_line_needs_two_seconds(self):
        bs = beats4()[:3]
        self.assertIn("E2", codes(lint(script(beats=bs), CAMPAIGN, REFS), "WARN"))
        bs[2]["hold"] = 1.2
        self.assertNotIn("E2", codes(lint(script(beats=bs), CAMPAIGN, REFS), "WARN"))


class KeyMoments(unittest.TestCase):
    def test_count_and_hold(self):
        self.assertIn("K1", codes(lint(script(beats=beats4()), CAMPAIGN, REFS), "WARN"))
        bs = beats4()
        bs[1]["key_moment"] = True
        bs[2].update(key_moment=True, hold=1.6)
        rep = lint(script(beats=bs), CAMPAIGN, REFS)
        self.assertNotIn("K1", codes(rep, "WARN"))
        self.assertEqual(where(rep, "K2"), ["s01-test.B02"])

    def test_timeline_carries_key(self):
        bs = beats4()
        bs[2]["key_moment"] = True
        tl = timing.build(script(beats=bs))
        self.assertEqual([b["key"] for b in tl["beats"]], [False, False, True, False])

    def test_control_is_not_asked(self):
        rep = lint(script(id="ctl-plain", beats=beats4()), CAMPAIGN, REFS)
        for c in ("K1", "TH0", "TN1"):
            self.assertNotIn(c, codes(rep, "WARN"))


class Thesis(unittest.TestCase):
    def spine(self, **kw):
        return {**script()["spine"], **kw}

    def test_needs_two_wordings(self):
        self.assertIn("TH0", codes(lint(script(), CAMPAIGN, REFS), "WARN"))
        s = script(spine=self.spine(thesis=["One filter shows the hidden mail.", "Your inbox hides mail; one filter shows it."]))
        self.assertNotIn("TH0", codes(lint(s, CAMPAIGN, REFS), "WARN"))

    def test_owner_pick_must_be_said(self):
        s = script(beats=beats4())
        pick = {"thesis": {"s01-test": "one filter shows the mail it hides"}}
        self.assertNotIn("TH1", codes(lint(s, CAMPAIGN, {**REFS, **pick}), "WARN"))
        pick = {"thesis": {"s01-test": "clients stop waiting on replies forever"}}
        self.assertIn("TH1", codes(lint(s, CAMPAIGN, {**REFS, **pick}), "WARN"))

    def test_approve_stores_the_pick_and_sheet_lists_them(self):
        with tempfile.TemporaryDirectory() as d:
            cd = Path(d)
            (cd / "05-scripts").mkdir()
            (cd / "campaign.json").write_text(json.dumps(CAMPAIGN))
            s = script(spine=self.spine(thesis=["Wording one.", "Wording two."]))
            (cd / "05-scripts" / "s01-test.json").write_text(json.dumps(s))
            self.assertIn("2. Wording two.", flow.sheet(cd, "s01-test"))
            rec = flow.approve(cd, "s01-test", "approved", "", None, 2)
            self.assertEqual(rec["thesis"], "Wording two.")
            self.assertEqual(flow.thesis_picks(cd), {"s01-test": "Wording two."})
            with self.assertRaises(SystemExit):
                flow.approve(cd, "s01-test", "approved", "", None, 3)


class Turn(unittest.TestCase):
    def test_before_after_shape(self):
        sp = script()["spine"]
        self.assertIn("TN1", codes(lint(script(), CAMPAIGN, REFS), "WARN"))
        for turn in ("Trước: tưởng chăm là được / Sau: phải sinh ra đã có",
                     {"before": "inbox is honest", "after": "it hides mail", "kind": "reveal"}):
            self.assertNotIn("TN1", codes(lint(script(spine={**sp, "turn": turn}), CAMPAIGN, REFS), "WARN"))
        self.assertIn("TN1", codes(lint(script(spine={**sp, "turn": {"before": "x"}}), CAMPAIGN, REFS), "WARN"))


class Tells(unittest.TestCase):
    def lint_vo(self, vo, k="vo"):
        bs = beats4()
        bs[1][k] = vo
        return lint(script(beats=bs), CAMPAIGN, REFS)

    def test_dash_but_not_a_range(self):
        self.assertEqual(where(self.lint_vo("Watch this — it sorts."), "WT0"), ["s01-test.B02.vo"])
        self.assertEqual(where(self.lint_vo("Rest -- then go.", "onscreen"), "WT0"), ["s01-test.B02.onscreen"])
        self.assertEqual(where(self.lint_vo("It takes 3–5 days, 2010-2020."), "WT0"), [])

    def test_stock_phrases(self):
        for vo in ("Không phải chăm chỉ, mà là sinh ra đã có.", "Hãy tưởng tượng bạn có một thanh kiếm.",
                   "Hành trình của kiếm tu.", "Đừng bỏ lỡ chương này.", "Not just a filter, but a habit.",
                   "Unlock your inbox.", "Wow! Amazing!", "Sự kiên trì, niềm tin và lòng dũng cảm.",
                   "Passion, dedication and excellence."):
            self.assertIn("WT1", codes(self.lint_vo(vo), "WARN"), vo)
        for vo in ("Kiếm tu nghèo vì tiền đổ hết vào kiếm.", "Watch this filter sort it.", "Không ai làm được."):
            self.assertNotIn("WT1", codes(self.lint_vo(vo), "WARN"), vo)

    def test_variants_are_checked(self):
        s = script(beats=beats4(), variants=[{"id": "H2", "vo": "Hãy tưởng tượng inbox biết nói.", "onscreen": "x"}])
        self.assertIn("s01-test.H2.vo", where(lint(s, CAMPAIGN, REFS), "WT1"))


class LengthTempoDevices(unittest.TestCase):
    def test_word_count_info(self):
        self.assertIn("WC", codes(lint(script(), CAMPAIGN, REFS), "INFO"))

    def test_three_alike_tempos(self):
        bs = beats4()
        for b in bs[:3]:
            b["tempo"] = "fast"
        self.assertIn("TP1", codes(lint(script(beats=bs), CAMPAIGN, REFS), "WARN"))
        bs[1]["tempo"] = "breath"
        self.assertNotIn("TP1", codes(lint(script(beats=bs), CAMPAIGN, REFS), "WARN"))
        bs[1]["tempo"] = "slow"
        self.assertIn("TP1", codes(lint(script(beats=bs), CAMPAIGN, REFS), "WARN"))

    def test_number_needs_a_device(self):
        bs = beats4()
        bs[1]["vo"] = "Three filters sort it twice as fast."
        self.assertEqual(where(lint(script(beats=bs), CAMPAIGN, REFS), "DV1"), ["s01-test.B02"])
        bs[1]["device"] = "counter"
        self.assertEqual(where(lint(script(beats=bs), CAMPAIGN, REFS), "DV1"), [])

    def test_everyday_vietnamese_words_are_not_numbers(self):
        from mvflib import craft
        for vo in ("Năm ngoái ba tôi kể lại chuyện đó.", "Hơn nữa, ta đi tiếp.", "Than ôi, kiếm gãy.", "Một người bạn cũ."):
            self.assertFalse(craft.needs_device({"vo": vo, "role": "body"}), vo)
        for vo in ("Ba năm luyện một chiêu.", "Một trong ba mươi người qua.", "Hắn mạnh hơn ta.", "Kiếm 30 năm."):
            self.assertTrue(craft.needs_device({"vo": vo, "role": "body"}), vo)

    def test_answered_in_warn_notes(self):
        rep = lint(script(warn_notes={"TH0": "one wording is the brief's own line"}), CAMPAIGN, REFS)
        self.assertNotIn("TH0", codes(rep, "WARN"))
        self.assertIn("TH0", codes(rep, "INFO"))


if __name__ == "__main__":
    unittest.main()
