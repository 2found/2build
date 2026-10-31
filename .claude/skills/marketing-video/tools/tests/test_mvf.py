"""Unit tests for the mvf library (stdlib only): python3 -m unittest discover -s tools/tests"""
import copy
import importlib.util
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(TOOLS))
os.environ["MVF_TODAY"] = "2026-10-27"

from mvflib import checks, timing  # noqa: E402
from mvflib.common import x_weight  # noqa: E402
from mvflib.export import srt, target_size  # noqa: E402
from mvflib.lint import CLAIM, lint  # noqa: E402
from mvflib.scaffold import caption_groups, placeholders, scaffold  # noqa: E402

spec = importlib.util.spec_from_file_location("tts", TOOLS / "extensions/omnivoice/tts_omnivoice.py")
tts = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tts)

CAMPAIGN = {"slug": "t", "product": "Acme", "goal": "signup", "kpi": "k", "markets": ["US"], "languages": ["en"],
            "platforms": ["tiktok", "x"], "brand": {"names": ["Acme"], "colors": {"accent": "#ff0000"}}}
SEG = {"id": "A1", "platforms": {"tiktok": {"search_terms": ["acme tips"]}}}
REFS = {"facts": {"F1", "F2"}, "segments": {"A1": SEG}, "trends": {}, "angles": {"G1": {"id": "G1", "segment": "A1"}}}


def script(**over):
    s = {
        "id": "s01-test", "angle": "G1", "format": "demo", "language": "en", "value": "learns a trick",
        "voice": {"mode": "none"},
        "beats": [
            {"id": "B01", "role": "hook", "vo": "Your inbox is lying.", "onscreen": "your inbox lies", "visual": "a phone"},
            {"id": "B02", "role": "demo", "vo": "Watch this filter sort it.", "onscreen": "one filter", "visual": "screen"},
            {"id": "B03", "role": "cta", "vo": "Try Acme today.", "onscreen": "acme.app", "visual": "logo"},
        ],
        "platforms": {"tiktok": {"caption": "inbox trick acme tips #a #b #c", "cover_text": "inbox trick"},
                      "x": {"post": "Your inbox is lying.", "cover_text": "inbox trick"}},
    }
    s.update(over)
    return s


def codes(rep, level=None):
    return [i["code"] for i in rep.items if level is None or i["level"] == level]


class XWeight(unittest.TestCase):
    def test_latin_and_cjk(self):
        self.assertEqual(x_weight("hello"), 5)
        self.assertEqual(x_weight("你好"), 4)

    def test_url_is_23(self):
        self.assertEqual(x_weight("go https://example.com/a/very/long/path"), 3 + 23)

    def test_emoji_sequences_weigh_two(self):
        self.assertEqual(x_weight("👍"), 2)
        self.assertEqual(x_weight("👍🏽"), 2)          # skin tone absorbed
        self.assertEqual(x_weight("👨‍👩‍👧"), 2)        # ZWJ family
        self.assertEqual(x_weight("🇻🇳"), 2)           # flag pair

    def test_vietnamese_precomposed_weighs_two(self):
        # twitter-text weights only U+0000–10FF (+ a few punctuation ranges) as 1;
        # Latin Extended Additional (ệ = U+1EC7) is 2, so a VI post fits fewer letters.
        self.assertEqual(x_weight("Viet"), 4)
        self.assertEqual(x_weight("Việt"), 5)


class Claims(unittest.TestCase):
    def test_ordinals_are_not_claims(self):
        for t in ("Chapter 1: Lin Feng", "Part 2", "By chapter fifty"):
            self.assertIsNone(CLAIM.search(t), t)

    def test_quantities_and_superlatives_are(self):
        for t in ("10,000 characters free", "3x faster", "$5", "50%", "the best app", "dịch 300 chương", "rẻ nhất", "miễn phí"):
            self.assertIsNotNone(CLAIM.search(t), t)


class Lint(unittest.TestCase):
    def test_clean_script_has_no_fail(self):
        r = lint(script(), CAMPAIGN, REFS)
        self.assertEqual(codes(r, "FAIL"), [])

    def test_claim_without_fact_fails(self):
        s = script()
        s["beats"][1]["vo"] = "It is 3x faster."
        self.assertIn("C1", codes(lint(s, CAMPAIGN, REFS), "FAIL"))
        s["beats"][1]["facts"] = ["F1"]
        self.assertNotIn("C1", codes(lint(s, CAMPAIGN, REFS), "FAIL"))

    def test_brand_name_digit_is_not_a_claim(self):
        c = copy.deepcopy(CAMPAIGN)
        c["brand"]["names"] = ["2novel"]
        s = script()
        s["beats"][1]["vo"] = "Open 2novel and paste it."
        self.assertNotIn("C1", codes(lint(s, c, REFS)))

    def test_claim_in_copy_needs_facts(self):
        s = script()
        s["platforms"]["x"]["post"] = "First 10,000 characters free"
        self.assertIn("C3", codes(lint(s, CAMPAIGN, REFS), "FAIL"))
        s["platforms"]["x"]["facts"] = ["F2"]
        self.assertNotIn("C3", codes(lint(s, CAMPAIGN, REFS), "FAIL"))

    def test_x_needs_onscreen_hook(self):
        s = script()
        s["beats"][0]["onscreen"] = ""
        self.assertIn("H4", codes(lint(s, CAMPAIGN, REFS), "FAIL"))

    def test_long_hook_fails(self):
        s = script()
        s["beats"][0]["vo"] = "So today I want to tell you about a thing that happened to my inbox last week."
        r = lint(s, CAMPAIGN, REFS)
        self.assertIn("H1", codes(r, "FAIL"))
        self.assertIn("H2", codes(r))  # weak opener "So today"

    def test_hook_variant_is_linted(self):
        s = script(variants=[{"id": "H2", "vo": "Hey guys, welcome back to my channel today.", "onscreen": "x", "visual": "y"}])
        self.assertIn("H2", codes(lint(s, CAMPAIGN, REFS)))

    def test_x_post_over_280_fails(self):
        s = script()
        s["platforms"]["x"]["post"] = "a" * 281
        self.assertIn("P1", codes(lint(s, CAMPAIGN, REFS), "FAIL"))

    def test_template_fails(self):
        self.assertIn("S0", codes(lint(script(_template=True), CAMPAIGN, REFS), "FAIL"))

    def test_sales_talk_before_cta_warns(self):
        s = script()
        s["beats"][1]["vo"] = "Download now with promo code."
        self.assertIn("C2", codes(lint(s, CAMPAIGN, REFS)))

    def test_late_first_value_fails(self):
        s = script()
        setup = "Let me explain how my week went before we get to the inbox part of this story."
        s["beats"].insert(1, {"id": "B09", "role": "context", "visual": "desk", "vo": setup})
        self.assertIn("R1", codes(lint(s, CAMPAIGN, REFS), "WARN"))  # value at about 7 s
        s["beats"][1]["vo"] = setup + " It was a long one, honestly."
        self.assertIn("R1", codes(lint(s, CAMPAIGN, REFS), "FAIL"))

    def test_script_without_value_beat_fails(self):
        s = script()
        s["beats"][1]["role"] = "problem"
        self.assertIn("R1", codes(lint(s, CAMPAIGN, REFS), "FAIL"))

    def test_brand_in_hook_text_warns(self):
        s = script()
        s["beats"][0]["onscreen"] = "Acme sorts your inbox"
        self.assertIn("H3", codes(lint(s, CAMPAIGN, REFS), "WARN"))

    def test_product_must_be_seen_in_first_half(self):
        s = script()
        self.assertIn("B2", codes(lint(s, CAMPAIGN, REFS), "WARN"))
        s["beats"][1]["visual"] = "the real Acme filter screen"
        self.assertNotIn("B2", codes(lint(s, CAMPAIGN, REFS)))


class Checks(unittest.TestCase):
    def test_angle_needs_share_reason(self):
        g = {"id": "G1", "segment": "A1", "insight": "i", "promise": "p", "why_watch": "w", "format": "demo",
             "facts": ["F1"]}
        a = {"angles": [g], "plan": [{"script": "s01-x", "angle": "G1", "why": "y"}]}
        self.assertIn("why_share is empty", str(checks.check_angles(a, {"F1"}, {"A1"}, set(), ["tiktok"]).items))
        g["why_share"] = "sent to the friend who reads at 3 am"
        self.assertNotIn("why_share", str(checks.check_angles(a, {"F1"}, {"A1"}, set(), ["tiktok"]).items))

    def test_inference_only_priority1_fails(self):
        seg = {"segments": [{"id": "A1", "priority": 1, "name": "n", "who": "w", "pains": ["p"], "jobs": ["j"],
                             "evidence": [{"type": "inference", "ref": "my guess"}]}]}
        self.assertTrue(any(i["code"] == "AUD" and "inference" in i["msg"] and i["level"] == "FAIL"
                            for i in checks.check_segments(seg, ["tiktok"]).items))

    def test_url_source_needs_quote(self):
        f = {"one_liner": "x", "facts": [{"id": "F1", "kind": "feature", "claim": "c", "checked": "2026-10-27",
                                         "source": {"type": "url", "ref": "https://a.b"}}]}
        self.assertIn("SRC", codes(checks.check_facts(f), "FAIL"))

    def test_old_rising_trend_in_use_fails(self):
        t = {"trends": [{"id": "T1", "platform": "tiktok", "kind": "format", "stage": "rising", "decision": "use",
                         "url": "https://x", "metric": "m", "observed_at": "2026-09-01",
                         "fit": {"segments": ["A1"], "how": "h"}}]}
        self.assertIn("TRD", codes(checks.check_trends(t, ["tiktok"], {"A1"}), "FAIL"))

    def test_stale_price_warns(self):
        f = {"one_liner": "x", "facts": [{"id": "F1", "kind": "price", "claim": "c", "checked": "2026-08-01",
                                         "source": {"type": "user", "ref": "owner"}}]}
        self.assertTrue(any("re-read" in i["msg"] for i in checks.check_facts(f).items))


MANIFEST = {"cues": [
    {"id": "B01", "duration_s": 1.6, "speech": [0.1, 1.5], "words": [{"w": "Your", "s": 0.1, "e": 0.3}, {"w": "inbox", "s": 0.3, "e": 0.7},
                                                                     {"w": "is", "s": 0.7, "e": 0.8}, {"w": "lying", "s": 0.8, "e": 1.5}]},
    {"id": "B02", "duration_s": 2.0, "speech": [0.05, 1.9], "words": [{"w": w, "s": 0.05 + i * 0.37, "e": 0.4 + i * 0.37} for i, w in
                                                                      enumerate(["Watch", "this", "filter", "sort", "it"])]},
    {"id": "B03", "duration_s": 1.2, "speech": [0.1, 1.1], "words": [{"w": w, "s": 0.1 + i * 0.33, "e": 0.4 + i * 0.33} for i, w in
                                                                     enumerate(["Try", "Acme", "today"])]},
]}


class Timing(unittest.TestCase):
    def test_measured_slots_are_contiguous_and_cover_frame0(self):
        tl = timing.build(timing.variant(script(), None), MANIFEST)
        self.assertEqual(tl["timing"], "measured")
        self.assertEqual(tl["beats"][0]["start"], 0.0)
        for a, b in zip(tl["beats"], tl["beats"][1:]):
            self.assertAlmostEqual(a["end"], b["start"], places=3)
        self.assertAlmostEqual(tl["beats"][0]["vo_end"] - tl["beats"][0]["vo_start"], 1.4, places=3)
        self.assertEqual(tl["beats"][-1]["end"], tl["duration"])

    def test_words_are_absolute_and_trimmed(self):
        tl = timing.build(timing.variant(script(), None), MANIFEST)
        b0 = tl["beats"][0]
        self.assertAlmostEqual(b0["words"][0]["s"], timing.LEAD_IN, places=3)  # speech[0] trimmed off

    def test_asr_word_before_trimmed_speech_stays_in_slot(self):
        m = copy.deepcopy(MANIFEST)
        m["cues"][2]["speech"] = [0.17, 1.1]
        m["cues"][2]["words"][0]["s"] = 0.0  # Whisper starts the first word at 0
        tl = timing.build(timing.variant(script(), None), m)
        b = tl["beats"][2]
        self.assertGreaterEqual(b["words"][0]["s"], b["vo_start"])

    def test_variant_replaces_beat0(self):
        s = script(variants=[{"id": "H2", "vo": "New hook here.", "onscreen": "new", "visual": "v"}])
        v = timing.variant(s, "H2")
        self.assertEqual(v["beats"][0]["vo"], "New hook here.")
        self.assertEqual(timing.cue_id(v["beats"][0]), "H2")
        self.assertEqual(s["beats"][0]["vo"], "Your inbox is lying.")  # original untouched

    def test_mixed_timing_is_reported(self):
        m = {"cues": MANIFEST["cues"][:2]}
        self.assertEqual(timing.build(timing.variant(script(), None), m)["timing"], "mixed")


class Captions(unittest.TestCase):
    def test_a_short_group_never_overlaps_the_next(self):
        m = copy.deepcopy(MANIFEST)  # "Watch" alone for 0.1 s, then the rest: the 0.3 s floor must not overrun
        m["cues"][1]["words"] = [{"w": "Watch", "s": 0.05, "e": 0.15}] + [
            {"w": w, "s": 0.2 + i * 0.2, "e": 0.35 + i * 0.2} for i, w in enumerate(["this", "filter", "sort", "it"])]
        s = script()
        s["beats"][1]["vo"] = "Watch. this filter sort it"
        g = caption_groups(timing.build(timing.variant(s, None), m), s)
        for a, b in zip(g, g[1:]):
            self.assertLessEqual(a["e"], b["s"])

    def test_words_sharing_a_start_still_get_a_readable_group(self):
        # ASR lumps a fast number ("ba mươi tư" all at 0.84): the group between them had a
        # zero-width window, so its SRT cue was 00:00:00,540 --> 00:00:00,540 and it never painted.
        m = copy.deepcopy(MANIFEST)
        m["cues"][1]["words"] = [{"w": "Watch", "s": 0.05, "e": 0.5},
                                 {"w": "twenty", "s": 0.5, "e": 0.5},
                                 {"w": "four", "s": 0.5, "e": 1.2}]
        s = script()
        s["beats"][1]["vo"] = "Watch twenty four"
        g = caption_groups(timing.build(timing.variant(s, None), m), s)
        for a, b in zip(g, g[1:]):
            self.assertLessEqual(a["e"], b["s"])
        self.assertTrue(all(x["e"] - x["s"] >= 0.12 - 1e-9 for x in g), g)
        cue = srt(g)
        self.assertNotIn("00:00:00,500 --> 00:00:00,500", cue)
        self.assertIn("twenty four", cue)

    def test_groups_keep_punctuation_and_cap_three_words(self):
        tl = timing.build(timing.variant(script(), None), MANIFEST)
        g = caption_groups(tl, script())
        texts = [" ".join(t["text"] for t in x["tokens"]) for x in g]
        self.assertEqual(texts[:2], ["Your inbox", "is lying."])  # balanced, no orphan
        self.assertTrue(all(len(x["tokens"]) <= 3 for x in g))
        for a, b in zip(g, g[1:]):
            self.assertLessEqual(a["e"], b["s"])  # one group on screen at a time

    def test_chunks_are_balanced_and_never_end_weak(self):
        from mvflib.scaffold import _chunk
        def split(t):
            return [" ".join(x["text"] for x in g) for g in _chunk([{"text": w} for w in t.split()], 3)]
        self.assertEqual(split("his sister shows up."), ["his sister", "shows up."])
        self.assertEqual(split("The first ten thousand characters are free."),
                         ["The first", "ten thousand", "characters are free."])
        self.assertEqual(split("one two"), ["one two"])

    def test_srt_format(self):
        out = srt([{"s": 0.05, "e": 1.234, "tokens": [{"text": "Hi"}, {"text": "there"}]}])
        self.assertIn("00:00:00,050 --> 00:00:01,234\nHi there", out)


class Encode(unittest.TestCase):
    def test_x_api_box(self):
        self.assertEqual(target_size((1080, 1920), {"width": 720, "height": 1280}, {"max_long_side": 1280, "max_short_side": 1024}), (720, 1280))
        self.assertEqual(target_size((1080, 1080), {"width": 720, "height": 1280}, {"max_long_side": 1280, "max_short_side": 1024}), (1024, 1024))
        self.assertEqual(target_size((1080, 1080), {"width": 1080, "height": 1920}, {}), (1080, 1080))  # never upscale


class Speech(unittest.TestCase):
    SAY = {"2novel": "two novel", "2novel.app": "two novel dot app", "MTL": "M T L"}

    def test_multiword_say_key(self):
        text, counts = tts.spoken_text("Go to 2novel.app now", "en", self.SAY)
        self.assertEqual(text, "Go to two novel dot app now")
        self.assertEqual(sum(counts), len(tts.fold(text)))
        self.assertEqual(len(counts), 5)  # Go to 2novel app now

    def test_acronym_aligns_with_asr(self):
        sp, c = tts.spoken_text("Reading MTL be like", "en", self.SAY)
        heard = {"words": [{"w": "Reading", "s": 0.1, "e": 0.4}, {"w": "MTL", "s": 0.45, "e": 0.9},
                           {"w": "be", "s": 0.95, "e": 1.0}, {"w": "like", "s": 1.05, "e": 1.3}]}
        ws, dropped, added = tts.align("Reading MTL be like", c, sp, heard, (0.1, 1.3))
        self.assertEqual((dropped, added), ([], []))
        self.assertEqual(ws[1], {"w": "MTL", "s": 0.45, "e": 0.9})

    def test_url_heard_without_spaces_is_not_dropped(self):
        text = "Paste at 2novel.app. First"
        sp, c = tts.spoken_text(text, "en", self.SAY)
        for w in ("twonovel.app,", "2novel.app,"):
            heard = {"words": [{"w": "Paste", "s": 0.1, "e": 0.4}, {"w": "at", "s": 0.45, "e": 0.55},
                               {"w": w, "s": 0.6, "e": 1.5}, {"w": "First", "s": 1.7, "e": 2.0}]}
            ws, dropped, added = tts.align(text, c, sp, heard, (0.1, 2.0))
            self.assertEqual((dropped, added), ([], []), w)
            self.assertEqual((ws[2]["w"], ws[2]["s"], ws[3]["w"], ws[3]["e"]), ("2novel", 0.6, "app", 1.5))

    def test_english_is_voiced_sentence_by_sentence(self):
        s = "Okay, so it got dropped. I'm reading the raws tonight, and honestly? It's better."
        self.assertEqual(tts.sentences(s, "en"), ["Okay, so it got dropped.",
                                                  "I'm reading the raws tonight, and honestly? It's better."])
        long = ", ".join(["one two three four five six seven"] * 4) + "."
        self.assertTrue(all(len(c.split()) <= tts.MAX_CHUNK_WORDS for c in tts.sentences(long, "en")))
        self.assertEqual(" ".join(tts.sentences(long, "en")), long)

    def test_cue_lines_are_paragraphs_like_the_worker(self):
        # a paragraph under MIN_SYNTH_CHARS joins the next one; a one-word sentence joins its neighbour
        plan = tts.chunk_plan("Okay. It got dropped.\n\n—\nI'm reading the raws tonight!", "en")
        self.assertEqual(plan, [["Okay. It got dropped.", "I'm reading the raws tonight!"]])
        self.assertEqual(tts.chunk_plan(" \n…\n", "en"), [])
        long = "This first line is long enough to stand on its own as a paragraph, so it keeps its gap."
        plan = tts.chunk_plan(f"{long}\nShort line,\nAnd the last one is here", "en")
        self.assertEqual(plan, [[long, "Short line. And the last one is here"]])
        self.assertEqual(tts.chunk_plan(f"{long}\n{long}", "en"), [[long], [long]])

    def test_short_fragments_never_exceed_the_chunk_cap(self):
        s = "Wow. " + " ".join(["word"] * 20) + "."
        self.assertEqual(tts.sentences(s, "en")[0], "Wow.")

    def test_non_vietnamese_cleaning_follows_the_worker(self):
        clean = tts.clean_for_tts
        self.assertEqual(clean("— “Don’t go!!!” she said…", "en"), "Don't go! she said...")
        self.assertEqual(clean("Wait...What? Fine:yes; **now**", "en"), "Wait... What? Fine, yes, now")
        self.assertEqual(clean("Doors open 15:30, rock'n'roll", "en"), "Doors open 15, 30, rock'n'roll")
        self.assertEqual(clean("好的。走吧！", "zh"), "好的. 走吧!")
        self.assertEqual(tts.sentences(clean("好的。走吧！真的吗？", "zh"), "zh"), ["好的. 走吧! 真的吗?"])
        # Vietnamese: written forms read, quotes and dashes gone
        self.assertEqual(clean("— “Giá 49.000đ lúc 15:30!!”", "vi"), "Giá bốn mươi chín nghìn đồng lúc mười lăm giờ ba mươi!")

    def test_dropped_words_are_reported(self):
        sp, c = tts.spoken_text("one two three four five", "en", {})
        heard = {"words": [{"w": "one", "s": 0, "e": 0.2}, {"w": "five", "s": 1.0, "e": 1.2}]}
        _, dropped, _ = tts.align("one two three four five", c, sp, heard, (0, 1.2))
        self.assertEqual(dropped, ["two three four"])


class VietnameseReading(unittest.TestCase):
    """extensions/omnivoice/vi_numbers.py: Vietnamese written forms read as words."""

    def test_parity_with_loa_frontend(self):
        from vi_numbers import cardinal, normalize_vi, number  # on sys.path via the omnivoice worker
        for n, said in [(0, "không"), (15, "mười lăm"), (21, "hai mươi mốt"), (24, "hai mươi tư"),
                        (104, "một trăm linh tư"), (110, "một trăm mười"), (1005, "một nghìn không trăm linh năm"),
                        (2026, "hai nghìn không trăm hai mươi sáu"), (1_250_000, "một triệu hai trăm năm mươi nghìn"),
                        (2_000_000_000, "hai tỷ")]:
            self.assertEqual(cardinal(n), said, n)
        for tok, said in [("12,5", "mười hai phẩy năm"), ("1.250.000", "một triệu hai trăm năm mươi nghìn"),
                          ("3.14", "ba phẩy một bốn"), ("0987654321", "không chín tám bảy sáu năm bốn ba hai một")]:
            self.assertEqual(number(tok), said, tok)
        self.assertEqual(normalize_vi("Ngày 23/8/2024, 15h30."),
                         "Ngày hai mươi ba tháng tám năm hai nghìn không trăm hai mươi tư, mười lăm giờ ba mươi.")
        self.assertEqual(normalize_vi("Giá 1.250.000đồng."), "Giá một triệu hai trăm năm mươi nghìn đồng.")
        self.assertEqual(normalize_vi("Chương thứ 104."), "Chương thứ một trăm linh tư.")
        self.assertNotIn("giờ", normalize_vi("tỉ lệ 3:1"))
        plain = "Hứa Thật An tỉnh lại một cách yếu ớt."
        self.assertEqual(normalize_vi(plain), plain)

    def test_script_forms_are_read_and_keep_one_count_per_display_word(self):
        say = {"Acme": "Ác Mi", "acme.vn": "ác mi chấm vi en"}
        for display, said in [
            ("Kiếm Lai có 1.300 chương.", "Kiếm Lai có một nghìn ba trăm chương."),
            ("Giảm 50% chỉ 49.000đ.", "Giảm năm mươi phần trăm chỉ bốn mươi chín nghìn đồng."),
            ("Họp lúc 15:30 ngày 28/10/2026.",
             "Họp lúc mười lăm giờ ba mươi ngày hai mươi tám tháng mười năm hai nghìn không trăm hai mươi sáu."),
            ("Đọc 3-5 chương, 2,5 giờ, 24 tập.", "Đọc ba đến năm chương, hai phẩy năm giờ, hai mươi tư tập."),
            ("Thứ 4 dùng Acme, vào acme.vn nhé.", "Thứ tư dùng Ác Mi, vào ác mi chấm vi en nhé."),
        ]:
            text, counts = tts.spoken_text(display, "vi", say)
            self.assertEqual(text, said, display)
            self.assertEqual(len(counts), len(tts.words(display)), display)
            self.assertEqual(sum(counts), len(tts.fold(text)), display)

    def test_digit_inside_a_word_is_not_a_number(self):
        text, counts = tts.spoken_text("Mở 2novel nghe", "vi", {})
        self.assertEqual(text, "Mở 2novel nghe")
        self.assertEqual(counts, [1, 1, 1])

    def test_whisper_digits_match_the_spoken_number(self):
        display = "Cả 1300 chương anh tóm"
        sp, c = tts.spoken_text(display, "vi", {})
        heard = {"words": [{"w": "Cả", "s": 0.1, "e": 0.3}, {"w": tts.normalize_vi("1300"), "s": 0.35, "e": 1.2},
                           {"w": "chương", "s": 1.25, "e": 1.5}, {"w": "anh", "s": 1.55, "e": 1.7},
                           {"w": "tóm", "s": 1.75, "e": 2.0}]}
        ws, dropped, added = tts.align(display, c, sp, heard, (0.1, 2.0))
        self.assertEqual((dropped, added), ([], []))
        self.assertEqual((ws[1]["w"], ws[1]["s"], ws[1]["e"]), ("1300", 0.35, 1.2))


class Scaffold(unittest.TestCase):
    def test_rescaffold_keeps_authored_scene_and_moves_slots(self):
        with tempfile.TemporaryDirectory() as d:
            cd = Path(d)
            (cd / "05-scripts").mkdir()
            (cd / "campaign.json").write_text(json.dumps(CAMPAIGN))
            sp = cd / "05-scripts/s01-test.json"
            sp.write_text(json.dumps(script()))
            proj = scaffold(cd, sp, "9x16", "H1", None, ["tiktok", "x"])
            self.assertEqual(placeholders(proj), ["B01", "B02", "B03"])
            (proj / "compositions/B02.html").write_text("<template><div id='root'>authored</div></template>")
            s = script()
            s["beats"][0]["vo"] = "Your inbox is lying to you every single morning."
            sp.write_text(json.dumps(s))
            proj2 = scaffold(cd, sp, "9x16", "H1", None, ["tiktok", "x"])
            self.assertEqual(proj, proj2)
            self.assertIn("authored", (proj / "compositions/B02.html").read_text())
            self.assertEqual(placeholders(proj), ["B01", "B03"])
            tl = json.loads((proj / "timeline.json").read_text())
            self.assertGreater(tl["beats"][1]["start"], 2.0)  # B02 moved later with the longer hook
            idx = (proj / "index.html").read_text()
            self.assertIn(f'data-start="{tl["beats"][1]["start"]}"', idx)
            self.assertIn("--mv-accent: #ff0000", idx)

    def test_hook_copies_follow_source_until_edited(self):
        with tempfile.TemporaryDirectory() as d:
            cd = Path(d)
            (cd / "05-scripts").mkdir()
            (cd / "campaign.json").write_text(json.dumps(CAMPAIGN))
            sp = cd / "05-scripts/s01-test.json"
            sp.write_text(json.dumps(script(variants=[{"id": "H2", "vo": "New hook here.", "onscreen": "n", "visual": "v"}])))
            h1 = scaffold(cd, sp, "9x16", "H1", None, ["tiktok"])
            h2 = scaffold(cd, sp, "9x16", "H2", None, ["tiktok"], from_dir=h1)
            (h1 / "compositions/B01.html").write_text("<template>h1 hook</template>")
            (h1 / "compositions/B02.html").write_text("<template>v1</template>")
            scaffold(cd, sp, "9x16", "H2", None, ["tiktok"], from_dir=h1)
            self.assertIn("v1", (h2 / "compositions/B02.html").read_text())          # placeholder → authored
            self.assertNotIn("h1 hook", (h2 / "compositions/B01.html").read_text())  # beat 0 is the variant's own
            scaffold(cd, sp, "9x16", "H2", None, ["tiktok"])                         # `mvf score` re-scaffolds without a source
            (h1 / "compositions/B02.html").write_text("<template>v2</template>")
            scaffold(cd, sp, "9x16", "H2", None, ["tiktok"], from_dir=h1)
            self.assertIn("v2", (h2 / "compositions/B02.html").read_text())          # untouched copy still follows
            (h2 / "compositions/B02.html").write_text("<template>local</template>")
            (h1 / "compositions/B02.html").write_text("<template>v3</template>")
            scaffold(cd, sp, "9x16", "H2", None, ["tiktok"], from_dir=h1)
            self.assertIn("local", (h2 / "compositions/B02.html").read_text())       # local edit wins



class BeatVoice(unittest.TestCase):
    """A beat's own `voice` speaks that line; changing it makes the line stale."""

    def setUp(self):
        spec = importlib.util.spec_from_file_location("mvfcli", TOOLS / "mvf.py")
        self.mvf = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.mvf)

    def test_cue_carries_beat_voice(self):
        s = script()
        s["beats"][1]["voice"] = "pro_reader"
        cues = {c["id"]: c for c in self.mvf.cue_list(s)}
        self.assertEqual(cues["B02"]["voice"], "pro_reader")
        self.assertNotIn("voice", cues["B01"])

    def test_voice_change_is_stale(self):
        s = script()
        s["beats"][1]["voice"] = "pro_reader"
        with tempfile.TemporaryDirectory() as d:
            take = Path(d)
            cues = [{"id": c["id"], "text": c["text"], **({"voice": c["voice"]} if "voice" in c else {})}
                    for c in self.mvf.cue_list(s)]
            (take / "manifest.json").write_text(json.dumps({"mvf_voice": self.mvf.voice_key(s), "cues": cues}))
            ok, stale = self.mvf.take_state(s, take)
            self.assertEqual(stale, set())
            s["beats"][1]["voice"] = "other"
            ok, stale = self.mvf.take_state(s, take)
            self.assertEqual(stale, {"B02"})


if __name__ == "__main__":
    unittest.main()
