"""Reach: audience rings, the reach score, outsider hooks (U1/U2), layers (N6) and the comparative feed."""
import json
import sys
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(TOOLS))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from mvflib import checks, pretest, research  # noqa: E402
from mvflib.lint import _names, lint  # noqa: E402
from test_mvf import CAMPAIGN, REFS, codes, script  # noqa: E402
from test_story import good_research  # noqa: E402

AWARE = {**CAMPAIGN, "goal": "awareness"}


def seg(ring, **kw):
    return {"id": "A1", "priority": 1, "ring": ring, "name": "n", "who": "w", "pains": ["p"], "jobs": ["j"],
            "objections": ["o"], "evidence": [{"type": "user", "ref": "owner"}],
            "platforms": {"tiktok": {"search_terms": ["kiếm lai"]}}, **kw}


def refs(ring, lore=()):
    return {**REFS, "segments": {"A1": seg(ring)}, "lore_terms": list(lore)}


class Rings(unittest.TestCase):
    def test_reach_goal_needs_a_ring_past_core(self):
        self.assertIn("AUD2", codes(checks.check_segments({"segments": [seg("core")]}, ["tiktok"], "awareness"), "FAIL"))
        self.assertNotIn("AUD2", codes(checks.check_segments({"segments": [seg("core")]}, ["tiktok"], "signup"), "FAIL"))
        both = {"segments": [seg("core"), {**seg("broad"), "id": "A2", "priority": 2}]}
        self.assertNotIn("AUD2", codes(checks.check_segments(both, ["tiktok"], "awareness")))

    def test_missing_ring_warns(self):
        s = seg("core")
        del s["ring"]
        self.assertIn("AUD2", codes(checks.check_segments({"segments": [s]}, ["tiktok"]), "WARN"))


class OutsiderHook(unittest.TestCase):
    def test_names_skip_sentence_starts_and_group_runs(self):
        # one capital at a sentence start is the sentence; a run of capitals there is still a name
        self.assertEqual(_names("Đứa bé ấy là Trần Bình An. Kiếm Lai, chương 25."), ["Trần Bình An", "Kiếm Lai"])
        self.assertEqual(_names("Giết. Cả nhà đi. Dùng nó"), [])
        self.assertEqual(_names("Quỷ Bí Chi Chủ: xem phim hay đọc truyện?"), ["Quỷ Bí Chi Chủ"])

    def test_known_words_and_separate_fields_are_not_lore(self):
        s = script()
        s["beats"][0].update(vo="Truyện này bị gỡ ở Trung Quốc?", onscreen="Gỡ vì sao")
        self.assertNotIn("U1", codes(lint(s, {**AWARE, "known_words": ["Trung Quốc"]}, refs("broad", [])), "WARN"))
        self.assertIn("U1", codes(lint(s, AWARE, refs("broad", [])), "WARN"))

    def test_broad_hook_with_lore_warns_and_skips_h6(self):
        s = script()
        s["beats"][0].update(vo="Bản mệnh sứ của Trần Bình An bị vỡ.", onscreen="bản mệnh sứ")
        rep = lint(s, AWARE, refs("broad", ["bản mệnh sứ"]))
        self.assertIn("U1", codes(rep, "WARN"))
        self.assertNotIn("H6", codes(rep))
        s["beats"][0].update(vo="Bạn cho thằng em hàng xóm một con cá.", onscreen="cho rồi có đòi lại?")
        self.assertNotIn("U1", codes(lint(s, AWARE, refs("broad", ["bản mệnh sứ"]))))

    def test_core_hook_still_needs_search_terms(self):
        s = script()
        s["beats"][0].update(vo="Bạn cho thằng em hàng xóm một con cá.", onscreen="cho rồi có đòi lại?")
        rep = lint(s, AWARE, refs("core"))
        self.assertIn("H6", codes(rep, "WARN"))
        self.assertNotIn("U1", codes(rep))

    def test_lore_piling_up_before_half_way_warns(self):
        s = script()
        s["beats"][1].update(vo="Ở Ly Châu, Tề Tĩnh Xuân giữ bản mệnh sứ.")
        self.assertIn("U2", codes(lint(s, AWARE, refs("broad", ["bản mệnh sứ"])), "WARN"))


class Layers(unittest.TestCase):
    def test_layers_are_asked_for_and_need_an_outer_ring_for_reach(self):
        self.assertIn("N6", codes(lint(script(), AWARE, refs("broad")), "WARN"))
        self.assertIn("N6", codes(lint(script(layers={"core": "chương 25"}), AWARE, refs("broad")), "WARN"))
        ok = script(layers={"broad": "a choice to argue", "core": "chương 25"})
        self.assertNotIn("N6", codes(lint(ok, AWARE, refs("broad"))))


class Feed(unittest.TestCase):
    def test_feed_hides_ids_and_shuffles_per_seed(self):
        bench = pretest.bench_clip({"id": "recap", "views": 328796, "url": "u", "caption": "c",
                                    "rows": [{"s": "0-3", "voice": "thanh niên này"}]})
        clips = [pretest.script_clip(script(), "H1", None, "candidate"),
                 pretest.script_clip(script(id="s09-old"), "H1", None, "control"), bench]
        text, key = pretest.feed(clips, 1)
        self.assertNotIn("s09-old", text)
        self.assertNotIn("328796", text)
        self.assertEqual(sorted(v["kind"] for v in key.values()), ["bench", "candidate", "control"])
        orders = {json.dumps([key[k]["id"] for k in sorted(key)]) for key in (pretest.feed(clips, i)[1] for i in range(6))}
        self.assertGreater(len(orders), 1)
        self.assertIn('"retell"', text)  # readers retell what each clip told them: echoing lines is not understanding
        self.assertIn('"unclear"', text)

    def test_winner_without_a_retell_fails_the_pretest(self):
        import tempfile
        with tempfile.TemporaryDirectory() as d:
            r1 = Path(d) / "r1"
            (r1 / "feed").mkdir(parents=True)
            dec = r1 / "decision.json"
            dec.write_text(json.dumps({"method": "agent-panel", "winner": "s01-x/H1", "evidence": "n=1", "changes": []}))
            (r1 / "feed/feed-1.key.json").write_text(json.dumps({"A": {"id": "s01-x/H1"}, "B": {"id": "bench:y"}}))
            ans = r1 / "feed/feed-1.broad.json"
            ans.write_text(json.dumps({"videos": {"A": {"swipe_at": None}, "B": {}}, "rank": ["A", "B"]}))
            self.assertIn("PRE3", codes(checks.check_pretest(dec), "FAIL"))
            ans.write_text(json.dumps({"videos": {"A": {"retell": ""}}, "rank": ["A"]}))
            rep = checks.check_pretest(dec)
            self.assertNotIn("PRE3", codes(rep, "FAIL"))
            self.assertIn("PRE3", codes(rep, "WARN"))
            ans.write_text(json.dumps({"videos": {"A": {"retell": "a level-3 warrior beats the lower five"}}}))
            self.assertNotIn("PRE3", codes(checks.check_pretest(dec)))


if __name__ == "__main__":
    unittest.main()
