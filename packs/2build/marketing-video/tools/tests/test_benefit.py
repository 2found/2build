"""Benefit: what the viewer can use afterwards (campaign, angle, lint W1-W4) and the top-video transcripts."""
import sys
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(TOOLS))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from mvflib import checks  # noqa: E402
from mvflib.lint import lint  # noqa: E402
import io  # noqa: E402
import tempfile  # noqa: E402
from contextlib import redirect_stdout  # noqa: E402
from unittest import mock  # noqa: E402

from mvflib import topvideos  # noqa: E402
from mvflib.topvideos import agent_todo, vtt_text  # noqa: E402
from test_mvf import CAMPAIGN, REFS, codes, script  # noqa: E402

LESSON = {**CAMPAIGN, "benefit": "lesson"}


def apply_script(vo="Next time you lend money, set the date first.", **over):
    s = script(**over)
    s["beats"].insert(2, {"id": "B03", "role": "apply", "vo": vo, "onscreen": "date first", "visual": "a calendar"})
    s["beats"][3]["id"] = "B04"
    return s


class Campaign(unittest.TestCase):
    def test_benefit_is_asked_for_and_checked(self):
        base = {"slug": "s", "product": "p", "goal": "awareness", "platforms": ["tiktok"], "languages": ["vi"],
                "markets": ["VN"], "brand": {"names": ["p"]}, "kpi": "views"}
        self.assertIn("BRF", codes(checks.check_campaign(base), "WARN"))
        self.assertIn("BRF", codes(checks.check_campaign({**base, "benefit": "story"}), "FAIL"))
        self.assertNotIn("BRF", codes(checks.check_campaign({**base, "benefit": "lesson"})))


class Angle(unittest.TestCase):
    def angles(self, benefit):
        g = {"id": "G1", "segment": "A1", "insight": "i", "promise": "p", "why_watch": "w", "why_share": "s",
             "format": "story", "facts": ["F1"], "benefit": benefit}
        return checks.check_angles({"angles": [g]}, {"F1"}, {"A1"}, set(), ["tiktok"], None, "lesson", ["Trần Bình An"])

    def test_angle_names_a_usable_benefit(self):
        self.assertIn("STR3", codes(self.angles(None), "FAIL"))
        ok = {"type": "lesson", "takeaway": "a gift given is theirs; asking it back costs the friend",
              "use_when": "you regret something you lent a friend"}
        self.assertNotIn("STR3", codes(self.angles(ok)))
        self.assertIn("STR3", codes(self.angles({**ok, "takeaway": "Trần Bình An never takes back a gift"}), "WARN"))
        self.assertIn("STR3", codes(self.angles({**ok, "type": "decide"}), "WARN"))


class Lint(unittest.TestCase):
    def test_use_is_required(self):
        s = script()
        del s["spine"]["use"]
        self.assertIn("W1", codes(lint(s, CAMPAIGN, REFS), "FAIL"))

    def test_lesson_needs_an_apply_beat_spoken_to_the_viewer(self):
        self.assertIn("W3", codes(lint(script(), LESSON, REFS), "WARN"))  # advice: the shape is the writer's
        rep = lint(apply_script(), LESSON, REFS)
        self.assertNotIn("W3", codes(rep, "FAIL"))
        self.assertNotIn("W4", codes(rep))
        self.assertIn("W4", codes(lint(apply_script("He set the date first."), LESSON, REFS), "WARN"))

    def test_takeaway_about_characters_warns(self):
        s = script()
        s["spine"]["use"] = "be like Trần Bình An"
        self.assertIn("W2", codes(lint(s, CAMPAIGN, {**REFS, "lore_terms": ["Trần Bình An"]}), "WARN"))

    def test_an_answer_may_name_the_topic(self):
        s = script()
        s["spine"]["use"] = "a level-3 võ phu beats any luyện khí sĩ in the lower five"
        refs = {**REFS, "lore_terms": ["võ phu", "luyện khí sĩ"]}
        self.assertNotIn("W2", codes(lint(s, {**CAMPAIGN, "benefit": "answer"}, refs)))
        self.assertIn("W2", codes(lint(s, LESSON, refs), "WARN"))


class Transcript(unittest.TestCase):
    def test_rolling_auto_subs_become_plain_text_once(self):
        vtt = ("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nTrong cuộc đời\n\n"
               "00:00:02.000 --> 00:00:04.000\nTrong cuộc đời\nTrong cuộc đời này có một nghịch lý\n\n"
               "00:03:00.000 --> 00:03:02.000\nquá giờ\n")
        self.assertEqual(vtt_text(vtt, 120), "Trong cuộc đời này có một nghịch lý")

    def test_tiktok_search_is_handed_to_the_agent(self):
        """mvf cannot search TikTok: it prints the browse steps and runs no search."""
        out = io.StringIO()
        with tempfile.TemporaryDirectory() as d, mock.patch.object(topvideos, "search") as search, redirect_stdout(out):
            rows = topvideos.study(Path(d), ["kiếm", "lai"], [], 6, 300, "topic", platform="tiktok")
        self.assertEqual(rows, [])
        search.assert_not_called()
        self.assertIn("https://www.tiktok.com/tag/kiemlai", out.getvalue())
        self.assertIn("mvf top-videos --url URL... --scope topic", out.getvalue())
        self.assertIn("<the title's hashtag", topvideos.tiktok_search("quỷ bí chi chủ nên đọc hay xem phim", "benefit"))

    def test_a_rerun_keeps_the_transcript_the_agent_wrote(self):
        def meta(u):
            vid = u.rsplit("/", 1)[-1]
            return {"id": vid, "webpage_url": u, "extractor_key": "TikTok", "view_count": int(vid), "duration": 30}, "", None
        urls = [f"https://www.tiktok.com/@a/video/{i}" for i in (7, 8)]
        with tempfile.TemporaryDirectory() as d, redirect_stdout(io.StringIO()), \
                mock.patch.object(topvideos, "transcribe", side_effect=lambda u, *_: meta(u)), \
                mock.patch.object(topvideos, "reading_frames", return_value=[]):
            txt = Path(d, topvideos.TOP, "7.txt")
            txt.parent.mkdir(parents=True)
            txt.write_text("what the agent read off the frames\n", encoding="utf-8")
            rows = topvideos.study(Path(d), [], urls + urls[:1], 6, 300, "topic")
            self.assertEqual(txt.read_text(encoding="utf-8"), "what the agent read off the frames\n")
        self.assertEqual([r["id"] for r in rows], ["8", "7"])

    def test_agent_gets_what_mvf_cannot_read(self):
        row = {"id": "1", "transcript": None}
        todo = agent_todo(row, "https://www.tiktok.com/@a/video/1", "tiktok", ["top/1/read-1.jpg"])
        self.assertEqual(len(todo), 2)
        self.assertIn("top/1/read-1.jpg", todo[0])
        self.assertIn("03-research/top/1.txt", todo[0])
        self.assertIn("comment-level-1", todo[1])
        self.assertEqual(agent_todo({"transcript": "top/x.txt"}, "https://youtu.be/x", "youtube", None), [])


if __name__ == "__main__":
    unittest.main()
