"""Story lint (spine, loops, recital, hook tension) and the topic-research gate."""
import copy
import sys
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(TOOLS))
sys.path.insert(0, str(Path(__file__).resolve().parent))

from mvflib import research  # noqa: E402
from mvflib.lint import lint  # noqa: E402
from test_mvf import CAMPAIGN, REFS, codes, script  # noqa: E402


def long_script():
    s = script()
    s["beats"] = [
        {"id": "B01", "role": "hook", "vo": "Why does your inbox hide mail?", "onscreen": "it hides mail", "visual": "phone"},
        {"id": "B02", "role": "context", "vo": "Mail comes in from many senders all day.", "visual": "a"},
        {"id": "B03", "role": "beat", "vo": "Some of it is sorted into tabs by rules.", "visual": "b"},
        {"id": "B04", "role": "beat", "vo": "The tabs are set by default for everyone.", "visual": "c"},
        {"id": "B05", "role": "beat", "vo": "Most people never open the other tabs.", "visual": "d"},
        {"id": "B06", "role": "payoff", "vo": "One filter puts it all back in one place.", "visual": "e"},
        {"id": "B07", "role": "cta", "vo": "Try Acme today.", "onscreen": "acme.app", "visual": "logo"},
    ]
    return s


class Story(unittest.TestCase):
    def test_missing_spine_fails(self):
        s = script()
        del s["spine"]
        self.assertIn("N1", codes(lint(s, CAMPAIGN, REFS), "WARN"))
        self.assertIn("W1", codes(lint(s, CAMPAIGN, REFS), "FAIL"))  # the use is the standard; the shape is advice

    def test_gap_with_no_open_question_warns(self):
        s = long_script()
        s["spine"]["loops"] = [{"id": "L1", "q": "why", "open": "B01", "close": "B02"}]
        self.assertIn("N3", codes(lint(s, CAMPAIGN, REFS), "WARN"))

    def test_loop_held_to_payoff_passes_but_recital_warns(self):
        s = long_script()
        s["spine"]["loops"] = [{"id": "L1", "q": "why", "open": "B01", "close": "B06"}]
        r = lint(s, CAMPAIGN, REFS)
        self.assertNotIn("N3", codes(r, "FAIL"))
        self.assertIn("N5", codes(r, "WARN"))    # B02-B05 state facts and move no question

    def test_question_opened_late_warns(self):
        s = long_script()
        s["spine"]["loops"] = [{"id": "L1", "q": "why", "open": "B03", "close": "B06"}]
        self.assertIn("N2", codes(lint(s, CAMPAIGN, REFS), "WARN"))

    def test_bare_label_hook_warns(self):
        s = script()
        s["beats"][0].update(vo="Inbox filters for founders.", onscreen="inbox filters")
        self.assertIn("H7", codes(lint(s, CAMPAIGN, REFS), "WARN"))
        s["beats"][0].update(vo="Why your inbox filters fail.")
        self.assertNotIn("H7", codes(lint(s, CAMPAIGN, REFS), "WARN"))

    def test_angle_idea_must_exist_in_research(self):
        refs = copy.deepcopy(REFS)
        refs["angles"]["G1"]["idea"] = "I9"
        refs["ideas"] = {"I1": {"id": "I1", "pick": True}}
        self.assertIn("N1", codes(lint(script(), CAMPAIGN, refs), "FAIL"))


def good_research():
    srcs = [{"id": f"R{i}", "kind": k, "url": f"https://{d}/p{i}", "fetched_at": "2026-11-06",
             "says": "fans argue about who the real master of the town is"}
            for i, (k, d) in enumerate([("reddit", "reddit.com"), ("forum", "voz.vn"), ("wiki", "fandom.com")] * 3, 1)]
    nugs = [{"id": f"N{i}", "type": "scene", "text": f"detail {i}", "sources": ["R1"]} for i in range(1, 9)]
    ideas = [{"id": f"I{i}", "premise": f"premise number {i} in a sentence", "question": "why give the gift back?",
              "payoff": "because a gift that binds is a debt", "benefit": "lesson", "nuggets": ["N1"],
              "inside": "the boy hands back the jade the immortal gave him",
              "use_when": "a friend asks for a gift back", "pattern": "P1", "new": "no top video says the debt part",
              "pick": i == 1} for i in range(1, 11)]
    tops = [{"id": f"V{i}", "url": f"https://youtube.com/watch?v={i}", "views": 300000 - i, "scope": scope,
             "subject": "being looked down on at work", "says": "do not argue; let results answer",
             "moves": "pain → two usual reactions → question → an old sage → the advice", "transcript": f"t/{i}.txt"}
            for i, scope in enumerate(["benefit", "benefit", "topic", "topic", "topic"], 1)]
    return {"topic": "Kiếm Lai", "sources": srcs, "top_videos": tops,
            "patterns": [{"id": "P1", "what": "a pain of daily life answered by an old authority", "videos": ["V1", "V2"]},
                         {"id": "P2", "what": "a famous line given its setup", "videos": ["V3"]}],
            "competitors": [{"url": "https://tiktok.com/v/1", "views": 1000, "hook": "h", "gap": "g"}] * 3,
            "covered": ["the plot summary"], "questions": [{"q": "who", "sources": ["R2"]}] * 5,
            "nuggets": nugs, "ideas": ideas}


class Research(unittest.TestCase):
    def test_complete_research_passes(self):
        self.assertEqual(codes(research.check(good_research()), "FAIL"), [])

    def test_a_nugget_may_stand_on_a_product_fact(self):
        d = good_research()
        d["nuggets"][0]["sources"] = ["F1"]
        self.assertIn("RS4", codes(research.check(d), "FAIL"))
        self.assertNotIn("RS4", codes(research.check(d, facts={"F1"}), "FAIL"))

    def test_filler_ideas_are_named_and_a_filler_pick_fails(self):
        d = good_research()
        d["ideas"][5]["payoff"] = "x"
        self.assertIn("RS5", codes(research.check(d), "WARN"))
        self.assertNotIn("RS5", codes(research.check(d), "FAIL"))
        d["ideas"][0]["payoff"] = "?"
        self.assertIn("RS5", codes(research.check(d), "FAIL"))

    def test_one_pick_per_script(self):
        d = good_research()
        for x in d["ideas"][:3]:
            x["pick"] = True
        self.assertNotIn("RS6", codes(research.check(d, scripts=3), "WARN"))
        self.assertIn("RS6", codes(research.check(d, scripts=2), "WARN"))

    def test_no_community_voice_warns_and_no_sources_fails(self):
        d = good_research()
        for s in d["sources"]:
            s["kind"] = "wiki"
        self.assertIn("RS1", codes(research.check(d), "WARN"))
        self.assertNotIn("RS1", codes(research.check(d), "FAIL"))
        d["sources"] = []
        self.assertIn("RS1", codes(research.check(d), "FAIL"))

    def test_a_pick_rides_the_pattern_the_audience_watched(self):
        d = good_research()
        self.assertNotIn("RS6", codes(research.check(d), "WARN"))
        d["ideas"][0]["pattern"] = "P2"
        self.assertNotIn("RS6", codes(research.check(d), "WARN"))  # P2 is topic-scope: P1's benefit views are no rival
        d["patterns"].append({"id": "P3", "what": "a power question with a surprising answer", "videos": ["V4"]})
        d["top_videos"][2]["views"] = 40000  # P2's only video, against P3's 299,996 in the same scope
        self.assertIn("RS6", codes(research.check(d), "WARN"))

    def test_a_pick_needs_something_from_inside_the_subject(self):
        d = good_research()
        d["ideas"][0]["inside"] = ""
        self.assertIn("RS5", codes(research.check(d), "FAIL"))
        d = good_research()
        for n in d["nuggets"]:
            n["type"] = "number"  # 1770 chapters, 2.5 hours, 6 names: the packaging
        self.assertIn("RS6", codes(research.check(d), "WARN"))

    def test_a_pick_must_be_usable(self):
        d = good_research()
        d["ideas"][0]["use_when"] = ""
        self.assertIn("RS5", codes(research.check(d), "FAIL"))
        d = good_research()
        d["ideas"][0]["benefit"] = "story"
        self.assertIn("RS5", codes(research.check(d), "FAIL"))

    def test_top_videos_are_read_not_listed(self):
        d = good_research()
        del d["top_videos"][0]["transcript"]
        self.assertIn("RS8", codes(research.check(d), "FAIL"))
        d = good_research()
        for v in d["top_videos"]:
            v["scope"] = "topic"
        self.assertIn("RS8", codes(research.check(d), "WARN"))
        d = good_research()
        d["top_videos"] = d["top_videos"][:2]
        self.assertIn("RS8", codes(research.check(d), "WARN"))
        d["top_videos"] = []
        self.assertIn("RS8", codes(research.check(d), "FAIL"))
        d["top_videos_note"] = "no video in this niche has subtitles"
        self.assertNotIn("RS8", codes(research.check(d), "FAIL"))

    def test_pick_off_the_campaign_benefit_warns(self):
        self.assertIn("RS6", codes(research.check(good_research(), "decide"), "WARN"))
        self.assertNotIn("RS6", codes(research.check(good_research(), "lesson"), "WARN"))

    def test_few_competitors_warn(self):
        d = good_research()
        d["competitors"] = []
        self.assertIn("RS2", codes(research.check(d), "WARN"))
        self.assertNotIn("RS2", codes(research.check(d), "FAIL"))


if __name__ == "__main__":
    unittest.main()
