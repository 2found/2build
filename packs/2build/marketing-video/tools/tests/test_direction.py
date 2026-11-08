"""Direction: text effects on frame 0, their placement, and the on-screen line check (D11)."""
import json
import sys
import math
import tempfile
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(TOOLS))

from mvflib import direction as dirn  # noqa: E402
from mvflib import rig  # noqa: E402
from mvflib.scaffold import geometry  # noqa: E402

GEO = geometry("9x16", ["tiktok"])
ART = {"P1": {"path": "assets/art/P1.png", "approved": True}}


def timeline():
    words = lambda t0, ws: [{"w": w, "s": t0 + 0.4 * i, "e": t0 + 0.4 * i + 0.35} for i, w in enumerate(ws)]
    return {"duration": 4.0, "beats": [{"id": "B01", "start": 0.0, "end": 2.0, "words": words(0.0, ["ông", "đập", "vỡ", "nó"])},
                      {"id": "B02", "start": 2.0, "end": 4.0, "words": words(2.0, ["đời", "ông", "bị", "mua"])}]}


def campaign(tmp: Path, d: dict, b02_captions=True) -> Path:
    beats = [{"id": "B01", "role": "hook", "vo": "ông đập vỡ nó", "onscreen": "đập *vỡ*"},
             {"id": "B02", "role": "turn", "vo": "đời ông bị mua", "onscreen": "đời ông *bị mua*"}]
    if not b02_captions:
        beats[1]["captions"] = False
    (tmp / "05-scripts").mkdir()
    (tmp / "05-scripts/s01.json").write_text(json.dumps({"id": "s01", "beats": beats}, ensure_ascii=False))
    dirn.path(tmp, "s01").parent.mkdir(parents=True)
    dirn.path(tmp, "s01").write_text(json.dumps({"storyline": "x", **d}, ensure_ascii=False))
    return tmp


def shot():
    return {"shots": [{"art": "P1"}]}


def codes(rep, code):
    return [x for x in rep.items if x["code"] == code and x["level"] != "INFO"]


class Direction(unittest.TestCase):
    def check(self, d, **kw):
        with tempfile.TemporaryDirectory() as t:
            cd = campaign(Path(t), d, **kw)
            return dirn.check(cd, "s01", timeline(), GEO, ART, {})

    def test_shown_text_is_on_screen_at_frame_zero(self):
        d = {"beats": {"B01": {**shot(), "fx": [{"type": "slam", "text": "đập *vỡ*", "shown": True, "at": "start",
                                                 "y": 0.45, "sfx": False}]}, "B02": shot()}}
        p, rep = self.check(d)
        f = p["beats"]["B01"]["spec"]["fx"][0]
        self.assertTrue(f["shown"])
        self.assertEqual(f["at"], 0.0)
        self.assertIn("mvk-shown", dirn._fx_html(f))
        self.assertFalse(codes(rep, "D11"), rep.items)

    def test_hook_line_not_shown_at_frame_zero_warns(self):
        _, rep = self.check({"beats": {"B01": {**shot(), "fx": [{"type": "slam", "text": "đập *vỡ*", "at": "start"}]},
                                       "B02": shot()}})
        self.assertTrue(codes(rep, "D11"), rep.items)
        _, rep = self.check({"beats": {"B01": shot(), "B02": shot()}})
        self.assertTrue(codes(rep, "D11"), rep.items)

    def test_captioned_body_line_needs_no_second_text_layer(self):
        hook = {**shot(), "fx": [{"type": "slam", "text": "đập *vỡ*", "shown": True, "at": "start", "y": 0.45}]}
        _, rep = self.check({"beats": {"B01": hook, "B02": shot()}})
        self.assertFalse(codes(rep, "D11"), rep.items)
        _, rep = self.check({"beats": {"B01": hook, "B02": shot()}}, b02_captions=False)
        self.assertTrue(codes(rep, "D11"), rep.items)

    def test_hot_markup_takes_no_width(self):
        # "Trần Bình An" fits one line at 104px; counting the asterisks wrapped it into the caption band
        d = {"beats": {"B01": {**shot(), "fx": [{"type": "slam", "text": "*Trần Bình An*", "shown": True, "at": "start",
                                                 "size": 104, "y": 0.45}]}, "B02": shot()}}
        p, rep = self.check(d)
        self.assertFalse(codes(rep, "D9"), rep.items)
        self.assertLess(p["beats"]["B01"]["spec"]["fx"][0]["box"]["h"], 140)

    def test_chip_takes_text_or_title(self):
        for key in ("text", "title"):
            html = dirn.overlay_html({"chip": {key: "Kiếm Lai", "sub": "chương 17"}}, GEO, 10.0, 3.0)
            self.assertIn("Kiếm Lai", html, key)

class Geometry(unittest.TestCase):
    def test_a_16x9_platform_does_not_shrink_the_vertical_safe_area(self):
        self.assertEqual(geometry("9x16", ["tiktok", "youtube-long"])["cap_top"], geometry("9x16", ["tiktok"])["cap_top"])


class ExplainBlocks(unittest.TestCase):
    """callout, tap, steps, compare, bars, quote, lower: placed, anchored to words, given their sounds."""
    def plan(self, fx):
        hook = {**shot(), "fx": [{"type": "slam", "text": "đập *vỡ*", "shown": True, "at": "start", "y": 0.45, "sfx": False}]}
        with tempfile.TemporaryDirectory() as t:
            cd = campaign(Path(t), {"beats": {"B01": hook, "B02": {**shot(), "fx": fx}}})
            return dirn.check(cd, "s01", timeline(), GEO, ART, {})

    def test_steps_land_on_their_words_with_one_pop_each(self):
        p, rep = self.plan([{"type": "steps", "items": ["đời", "ông", "*bị mua*"], "at": "word:đời",
                             "ats": ["word:đời", "word:ông", "word:bị"]}])
        self.assertFalse([x for x in rep.items if x["level"] == "FAIL"], rep.items)
        f = p["beats"]["B02"]["spec"]["fx"][0]
        self.assertEqual(f["times"], [0.0, 0.4, 0.8])
        self.assertEqual(dirn._fx_html(f).count('class="mvk-step"'), 3)
        pops = [x for x in p["sfx"] if x["kind"] == "pop"]
        self.assertEqual(len(pops), 3)

    def test_compare_needs_both_sides(self):
        _, rep = self.plan([{"type": "compare", "left": ["a"], "heads": ["A", "B"]}])
        self.assertTrue([x for x in rep.items if x["code"] == "D8" and x["level"] == "FAIL"])
        p, rep = self.plan([{"type": "compare", "left": ["nhanh", "rẻ"], "right": ["chắc", "*bền*"], "heads": ["Võ phu", "Luyện khí"]}])
        html = dirn._fx_html(p["beats"]["B02"]["spec"]["fx"][0])
        self.assertIn("mvk-cmp-l", html)
        self.assertIn("<h4>Luyện khí</h4>", html)

    def test_bars_scale_to_the_largest_value(self):
        p, _ = self.plan([{"type": "bars", "items": [{"label": "A", "value": 50}, {"label": "B", "value": 100}], "suffix": "%"}])
        html = dirn._fx_html(p["beats"]["B02"]["spec"]["fx"][0])
        self.assertIn("--w:50.0%", html)
        self.assertIn("100%", html)

    def test_callout_stays_in_frame_and_keeps_its_label_out_of_captions(self):
        p, rep = self.plan([{"type": "callout", "rect": [0.8, 0.1, 0.4, 0.2]}])
        self.assertTrue([x for x in rep.items if x["code"] == "D8" and x["level"] == "WARN"])
        x, _, ww, _ = p["beats"]["B02"]["spec"]["fx"][0]["rect"]
        self.assertLessEqual(x + ww, 1080, "a box past the edge is clamped into the frame, not refused")
        p, rep = self.plan([{"type": "callout", "rect": [0.1, 0.2, 0.5, 0.15], "text": "nút này", "spot": True}])
        f = p["beats"]["B02"]["spec"]["fx"][0]
        self.assertIn("mvk-callout-spot", dirn._fx_html(f))
        self.assertFalse([x for x in rep.items if x["code"] == "D9" and x["level"] == "FAIL"])
        p, rep = self.plan([{"type": "callout", "rect": [0.1, 0.55, 0.5, 0.08], "text": "nhãn"}])
        f = p["beats"]["B02"]["spec"]["fx"][0]
        self.assertLess(f["label"]["top"], f["rect"][1], "no room below: the label goes above the box")
        self.assertFalse([x for x in rep.items if x["code"] == "D9" and x["level"] == "FAIL"])
        _, rep = self.plan([{"type": "callout", "rect": [0.1, 0.66, 0.5, 0.06]}])
        self.assertTrue([x for x in rep.items if x["code"] == "D9" and x["level"] == "WARN"], "a box under the captions warns")

    def test_punctuation_after_a_hot_run_stays_on_its_word(self):
        self.assertEqual(dirn._title_words("hết *một quyển*."),
                         ['<span class="mvk-w">hết</span>', '<span class="mvk-w hot">một</span>',
                          '<span class="mvk-w hot">quyển.</span>'])

    def test_position_may_differ_per_canvas(self):
        p, rep = self.plan([{"type": "tap", "x": {"9x16": 0.5, "16x9": 0.3}, "y": 0.4}])
        self.assertEqual(p["beats"]["B02"]["spec"]["fx"][0]["rect"][0], 540)
        _, rep = self.plan([{"type": "tap", "x": {"16x9": 0.3}, "y": 0.4}])
        self.assertTrue([x for x in rep.items if x["code"] == "D8" and x["level"] == "FAIL"])

    def test_tap_quote_lower_render(self):
        p, rep = self.plan([{"type": "tap", "x": 0.5, "y": 0.4}, {"type": "quote", "text": "hay *thật*", "sub": "— độc giả"},
                            {"type": "lower", "text": "Ninh Diêu", "sub": "kiếm tiên", "y": 0.12}])
        fx = p["beats"]["B02"]["spec"]["fx"]
        self.assertEqual([f["type"] for f in fx], ["tap", "quote", "lower"])
        self.assertIn("mvk-tap-ring", dirn._fx_html(fx[0]))
        self.assertIn("mvk-q-mark", dirn._fx_html(fx[1]))
        self.assertIn("mvk-lower-sub", dirn._fx_html(fx[2]))
        self.assertIn("click", [x["kind"] for x in p["sfx"]])


if __name__ == "__main__":
    unittest.main()


class Layers(unittest.TestCase):
    """2.5D layers: cut-outs over a plate, parallax by depth, moves/hits on words, and the art they need."""
    ART = {**ART, "HERO": {"path": "assets/art/HERO.png", "approved": True, "cutout": True, "shown_h": 1200},
           "FLAT": {"path": "assets/art/FLAT.png", "approved": True, "cutout": False, "shown_h": 1920}}

    def plan(self, layers, cam="push:1.2"):
        d = {"beats": {"B01": {"shots": [{"art": "P1", "cam": cam, "layers": layers}]}}}
        p, rep = dirn.plan(d, timeline(), GEO, self.ART, {})
        return p, rep

    def layer(self, p):
        return p["beats"]["B01"]["shots_resolved"][0]["layers"]

    def test_a_nearer_layer_travels_further_than_the_plate(self):
        p, rep = self.plan([{"art": "HERO", "z": 0.5}])
        sh = p["beats"]["B01"]["shots_resolved"][0]
        L = self.layer(p)[0]
        self.assertEqual(codes(rep, "D12"), [])
        self.assertAlmostEqual(L["cam"]["to"]["s"] - 1, (sh["cam"]["to"]["s"] - 1) * 1.5, places=3)
        self.assertEqual(self.plan([{"art": "HERO", "z": 0}])[0]["beats"]["B01"]["shots_resolved"][0]["layers"][0]["cam"]["to"],
                         {**sh["cam"]["to"]})

    def test_moves_chain_from_where_the_last_one_left_and_count_as_something_new(self):
        p, rep = self.plan([{"art": "HERO", "x": 0.5, "y": 0.9,
                             "moves": [{"at": "word:đập", "x": 0.3}, {"at": "word:nó", "y": 0.8, "s": 1.2}]}])
        m0, m1 = self.layer(p)[0]["moves"]
        W, H = GEO["W"], GEO["H"]
        self.assertEqual(m0["to"]["x"], round(-0.2 * W, 1))
        self.assertEqual(m1["from"], m0["to"])
        self.assertEqual((m1["to"]["x"], m1["to"]["y"], m1["to"]["s"]), (m0["to"]["x"], round(-0.1 * H, 1), 1.2))
        self.assertIn(round(m1["at"], 3), p["events"])

    def test_a_move_overlapping_the_last_one_waits_for_it(self):
        p, rep = self.plan([{"art": "HERO", "moves": [{"at": "word:đập", "x": 0.3, "dur": 0.6}, {"at": "word:vỡ", "x": 0.6}]}])
        m0, m1 = self.layer(p)[0]["moves"]
        self.assertAlmostEqual(m1["at"], m0["at"] + 0.6, places=3)
        self.assertTrue(codes(rep, "D12"))

    def test_hits_recoil_away_from_where_they_come_from(self):
        p, _ = self.plan([{"art": "HERO", "hits": [{"at": "word:đập", "from": "left"}, {"at": "word:nó", "from": "right", "sfx": "impact"}]}])
        h0, h1 = self.layer(p)[0]["hits"]
        self.assertGreater(h0["dx"], 0)
        self.assertLess(h1["dx"], 0)
        self.assertTrue(any(s["kind"] == "impact" for s in p["sfx"]))
        self.assertFalse(any(s["src"].endswith("hit0") for s in p["sfx"]))  # silent unless asked

    def test_bad_layers_fail_and_say_what_to_do(self):
        _, rep = self.plan([{"art": "NOPE"}])
        self.assertIn("nothing drawn", codes(rep, "D12")[0]["msg"])
        _, rep = self.plan([{"art": "FLAT"}])
        self.assertIn("transparent: true", codes(rep, "D12")[0]["msg"])
        _, rep = self.plan([{"art": "HERO", "enter": "teleport", "idle": "dance"}])
        self.assertEqual(len(codes(rep, "D12")), 2)
        _, rep = self.plan([{"art": "HERO", "hits": [{"at": "word:mua"}]}])  # B02's word: outside the shot
        self.assertTrue(codes(rep, "D3") or codes(rep, "D12"))

    def test_a_layer_shown_bigger_than_approved_warns(self):
        _, rep = self.plan([{"art": "HERO", "h": 0.8}])  # 1536 px tall, approved for 1200
        self.assertTrue(codes(rep, "D13"))
        _, rep = self.plan([{"art": "HERO", "h": 0.6}])
        self.assertFalse(codes(rep, "D13"))

    def test_scene_html_nests_one_wrapper_per_motion_and_starts_entrances_off(self):
        p, _ = self.plan([{"art": "HERO", "enter": "slide-left", "idle": "breathe", "shadow": True, "flip": True},
                          {"art": "HERO", "enter": "fade"}])
        html = dirn.scene_html("B01", p["beats"]["B01"], GEO)
        for c in ("mvk-lyc", "mvk-ly", "mvk-lym", "mvk-lyh", "mvk-lyi", "mvk-lysh", "scaleX(-1)", 'src="assets/art/HERO.png"'):
            self.assertIn(c, html)
        self.assertIn(f"transform:translate({0.6 * GEO['W']:g}px,0)", html)  # off frame before it slides in
        self.assertIn('class="mvk-ly" style="opacity:0;', html)                # a fade starts unseen
        p, _ = self.plan([{"art": "HERO", "anchor": "center", "moves": [{"at": "word:đập", "rot": 200}]}])
        self.assertIn('class="mvk-ly mvk-lyc-center"', dirn.scene_html("B01", p["beats"]["B01"], GEO))  # spins in place
        self.assertLess(html.index("mvk-kb"), html.index("B01-s0-L0"))          # over the plate, under the punch-ins

    def test_a_face_under_the_captions_warns_until_the_beat_moves_them(self):
        bottom = (GEO["cap_top"] + 100) / GEO["H"] + 0.5  # a 0.5-tall layer whose head sits in the band
        _, rep = self.plan([{"art": "HERO", "y": bottom, "h": 0.5}])
        self.assertTrue(codes(rep, "D14"))
        d = {"beats": {"B01": {"captions_y": 0.12, "shots": [{"art": "P1", "layers": [{"art": "HERO", "y": bottom, "h": 0.5}]}]}}}
        _, rep = dirn.plan(d, timeline(), GEO, self.ART, {})
        self.assertFalse(codes(rep, "D14"))
        _, rep = self.plan([{"art": "HERO", "y": bottom, "h": 0.1}])  # a prop, not a face
        self.assertFalse(codes(rep, "D14"))

    def test_captions_take_the_beats_own_top(self):
        from mvflib.scaffold import captions_html
        groups = [{"s": 0.1, "e": 0.9, "tokens": [{"text": "ông", "s": 0.1, "e": 0.4, "key": False}]},
                  {"s": 2.1, "e": 2.9, "tokens": [{"text": "đời", "s": 2.1, "e": 2.4, "key": False}]}]
        brand = {"colors": {"accent": "#f00", "ink": "#000", "paper": "#fff", "fg": "#fff", "bg": "#000"}, "fonts": {"caption": "Inter", "display": "Inter", "body": "Inter"}}
        groups[1]["s"] = 1.96  # shown just before the beat starts, as caption_groups does
        out = captions_html(groups, GEO, brand, 4.0, None, None, [(2.0, 4.0, 300)])
        self.assertIn('<div id="cap1" class="cap" style="top:300px">', out)
        self.assertIn('<div id="cap0" class="cap">', out)

    def test_raw_tweens_pass_through_so_the_agent_is_not_capped_by_the_presets(self):
        p, rep = self.plan([{"art": "HERO", "tweens": [
            {"at": "word:vỡ", "part": "img", "from": {"filter": "blur(0px)"}, "to": {"filter": "blur(6px)", "skewX": 8}, "dur": 0.3},
            {"at": "word:đập", "part": "idle", "to": {"rotation": 3, "yoyo": True, "repeat": -1}, "dur": 0.2}]}])
        t0, t1 = self.layer(p)[0]["tweens"]
        self.assertEqual(codes(rep, "D12"), [])
        self.assertEqual(t0["to"], {"filter": "blur(6px)", "skewX": 8})
        self.assertGreater(t1["to"]["repeat"], 0)  # never repeat: -1 in a rendered frame
        _, rep = self.plan([{"art": "HERO", "tweens": [{"at": "word:đập", "part": "elbow", "to": {}}]}])
        self.assertTrue(codes(rep, "D12"))


class Presets(unittest.TestCase):
    """styles.json fills defaults only: the direction's own look and captions win, an unknown preset FAILs."""

    def test_a_look_preset_sits_over_the_brand_and_under_the_direction(self):
        brand = {"colors": {"bg": "#000000"}, "colors_set": {"bg": True}}
        lk = dirn.look_of({"look": {"preset": "ink"}}, brand)
        self.assertEqual(lk["colors"]["ground"], "#f4efe3")
        lk = dirn.look_of({"look": {"preset": "ink", "colors": {"ground": "#ffffff"}}}, brand)
        self.assertEqual((lk["colors"]["ground"], lk["colors"]["ink"]), ("#ffffff", "#1d1b18"))
        self.assertNotIn("preset", lk)
        self.assertEqual(dirn.captions_of({"look": {"preset": "ink"}})["style"], "box")
        self.assertEqual(dirn.captions_of({"look": {"preset": "ink"}, "captions": {"style": "shadow"}})["style"], "shadow")

    def test_an_unknown_preset_fails(self):
        with tempfile.TemporaryDirectory() as t:
            cd = campaign(Path(t), {"look": {"preset": "pixar"}, "beats": {"B01": shot(), "B02": shot()}})
            _, rep = dirn.check(cd, "s01", timeline(), GEO, ART, {})
            self.assertIn("direction.look.preset", [i["where"] for i in rep.items if i["code"] == "D0"])


class Rig(unittest.TestCase):
    """The stick rig: a figure drawn in code whose joints the timeline tweens."""

    def plan(self, layer, d=None, cam="hold"):
        d = {**(d or {}), "beats": {"B01": {"shots": [{"color": "ground", "cam": cam, "layers": [layer]}]}}}
        return dirn.plan(d, timeline(), GEO, Layers.ART, {})

    def L(self, p):
        return p["beats"]["B01"]["shots_resolved"][0]["layers"][0]

    def test_poses_keep_the_feet_on_the_ground(self):
        self.assertAlmostEqual(rig.pose("stand")["drop"], 0.0, delta=0.3)
        self.assertGreater(rig.pose("crouch")["drop"], 5)
        self.assertGreater(rig.pose("sit")["drop"], 15)
        p = rig.pose({"armR": 90}, rig.pose("sit"))  # a joint dict changes only what it names
        self.assertEqual((p["armR"], p["legL"]), (90.0, rig.POSES["sit"]["legL"]))
        with self.assertRaises(rig.RigError):
            rig.pose({"tail": 30})

    def test_a_rig_layer_needs_no_art_and_resolves_its_timeline(self):
        p, rep = self.plan({"rig": "stick", "pose": "stand", "face": "calm", "talk": True,
                            "poses": [{"at": "word:nó", "pose": "wave", "face": "happy"}, {"at": "wend:3", "set": {"head": 12}}],
                            "walk": [{"at": "start", "until": "word:vỡ"}],
                            "emotes": [{"at": "word:vỡ", "type": "!"}, {"at": "word:nó", "text": "?!"}]})
        self.assertEqual([i for i in rep.items if i["level"] == "FAIL"], [])
        R = self.L(p)["rig"]
        walk = [t for t in R["tweens"] if t["ease"] == "sine.inOut"]
        self.assertGreater(len(walk), 1)
        back = R["tweens"][len(walk)]
        self.assertEqual(back["to"], R["pose0"])  # a walk ends in the pose it started from
        self.assertEqual(R["tweens"][-1]["to"]["head"], 12.0)
        self.assertEqual(R["facesets"], [{"at": R["tweens"][-2]["at"], "face": "happy"}])
        self.assertEqual(len(R["flaps"]), 4)  # one open mouth per spoken word
        self.assertTrue(all(a < b for a, b in R["flaps"]))
        self.assertEqual(len(R["emotes"]), 2)

    def test_bad_rig_entries_fail_and_point_at_the_raw_form(self):
        for bad in ({"rig": "stick", "pose": "moonwalk"}, {"rig": "stick", "face": "smirk"},
                    {"rig": "stick", "emotes": [{"at": "start", "type": "heart"}]}, {"rig": "bone"}):
            _, rep = self.plan(bad)
            self.assertTrue(codes(rep, "D12"), bad)
        _, rep = self.plan({"rig": {"type": "stick", "faces": {"smirk": ["<circle r='1'/>", "<path d='M0 0'/>"]}}, "face": "smirk"})
        self.assertEqual(codes(rep, "D12"), [])

    def test_the_rig_draws_joints_as_variables_and_the_preset_inks_it(self):
        p, _ = self.plan({"rig": {"type": "stick", "view": "side"}, "pose": "sit", "flip": True,
                          "emotes": [{"at": "start", "svg": "<path d='M0 0 L1 1'/>"}]}, d={"look": {"preset": "ink"}})
        L = self.L(p)
        out = dirn._layer_html(L, GEO["W"], GEO["H"])
        self.assertIn("--legL:85", out)
        self.assertIn("rotate(calc(var(--armR) * -1deg))", out)
        self.assertIn("feTurbulence", out)  # the preset's line boil
        self.assertIn("#1d1b18", out)
        self.assertIn("scaleX(-1)", out)
        js = dirn._spec_for_js({"fx": [], "shots": [{"layers": [L]}]})
        self.assertNotIn("svg", js["shots"][0]["layers"][0]["rig"]["emotes"][0])

    def test_raw_tweens_reach_any_joint_or_selector(self):
        p, rep = self.plan({"rig": "stick", "tweens": [
            {"at": "start", "part": "rig", "to": {"--elbowR": 80, "yoyo": True, "repeat": -1}, "dur": 0.3},
            {"at": "word:vỡ", "sel": ".rig-f-calm", "to": {"opacity": 0.5}, "dur": 0.2}]})
        self.assertEqual(codes(rep, "D12"), [])
        t0, t1 = self.L(p)["tweens"]
        self.assertGreater(t0["to"]["repeat"], 0)
        self.assertEqual(t1["sel"], ".rig-f-calm")


class CharacterRig(unittest.TestCase):
    """A rig that plays a cast character: a body, drawn parts and colours, shared through 07-video/rigs.json."""

    AN = {"type": "stick", "cast": "HERO", "body": "doodle", "skin": "#f4dcc8", "shirt": "#e7e3d8",
          "parts": [{"on": "head", "back": True, "svg": "<path class='hair-back' d='M -11 0 Q 0 -16 11 0 Z'/>"},
                    {"on": "head", "view": "side", "svg": "<path class='hair-side' d='M 0 -10 L 10 -4'/>"},
                    {"on": "torso", "svg": "<path class='collar' d='M -4 -20 L 0 -14 L 4 -20'/>"}]}

    def plan(self, layer, d=None):
        d = {**(d or {}), "beats": {"B01": {"shots": [{"color": "ground", "layers": [layer]}]}}}
        return dirn.plan(d, timeline(), GEO, Layers.ART, {})

    def html(self, p):
        return dirn._layer_html(p["beats"]["B01"]["shots_resolved"][0]["layers"][0], GEO["W"], GEO["H"])

    def test_the_doodle_body_is_big_headed_and_its_height_is_the_figure(self):
        dm = rig.body({"body": "doodle"})
        self.assertEqual(dm["fig_h"], 15 + 15 + 22 + 1 + 2 * 17)
        self.assertGreater(dm["head_r"] * 2 / dm["fig_h"], 0.35)  # the head is over a third of the figure
        self.assertEqual(rig.body({"body": "doodle", "dims": {"head_r": 14}})["fig_h"], 15 + 15 + 22 + 1 + 28)

    def test_the_doodle_preset_gives_the_body_and_parts_ride_their_bone_in_their_view(self):
        p, rep = self.plan({"rig": self.AN, "pose": "stand", "face": "laugh"}, d={"look": {"preset": "stickman"}})
        self.assertEqual(codes(rep, "D12"), [])
        out = self.html(p)
        self.assertIn('fill="#e7e3d8"', out)  # the shirt colour
        self.assertLess(out.index("hair-back"), out.index('<rect x="-10"'))  # behind the head's skin
        self.assertIn("collar", out)
        self.assertNotIn("hair-side", out)  # a side-view part stays out of the front view
        p, _ = self.plan({"rig": {**self.AN, "view": "side"}, "pose": "sit"}, d={"look": {"preset": "stickman"}})
        self.assertIn("hair-side", self.html(p))

    def test_a_bad_part_fails(self):
        for part in ({"on": "tail", "svg": "<path/>"}, {"on": "head"}, {"on": "head", "svg": "<path/>", "view": "top"}):
            _, rep = self.plan({"rig": {"type": "stick", "parts": [part]}})
            self.assertTrue(codes(rep, "D12"), part)

    def test_a_built_in_emote_keeps_a_stick_size_beside_a_big_head(self):
        p, _ = self.plan({"rig": {"type": "stick", "body": "doodle"}, "emotes": [{"at": "start", "type": "?"}]})
        out = self.html(p)
        self.assertIn(f"scale({math.sqrt(17 / 9):.3f})", out)  # not the head's 17/9
        self.assertEqual(self.L(p)["rig"]["emotes"][0]["anchor"], [13.5, -26])
        p, _ = self.plan({"rig": "stick", "emotes": [{"at": "start", "type": "!", "dx": 4, "dy": -6}]})
        self.assertEqual(self.L(p)["rig"]["emotes"][0]["anchor"], [17.0, -31.0])

    def L(self, p):
        return p["beats"]["B01"]["shots_resolved"][0]["layers"][0]

    def test_rigs_json_is_the_character_sheet_and_use_overrides_it(self):
        with tempfile.TemporaryDirectory() as t:
            cd = campaign(Path(t), {"beats": {"B01": {"shots": [{"color": "ground", "layers": [
                {"rig": {"use": "AN", "shirt": "#ff0000"}}, {"rig": "KID"}, {"rig": {"use": "NOBODY"}}]}]}, "B02": shot()}})
            (cd / "07-video/rigs.json").write_text(json.dumps({"rigs": {
                "AN": self.AN, "KID": {"type": "stick", "cast": "KID"}}}))
            (cd / "07-video/art").mkdir(parents=True, exist_ok=True)
            (cd / "07-video/art/art.json").write_text(json.dumps({"cast": {"HERO": {"prompt": "x"}}}))
            p, rep = dirn.check(cd, "s01", timeline(), GEO, ART, {})
        rigs = [L.get("rig") for L in p["beats"]["B01"]["shots_resolved"][0]["layers"]]
        self.assertEqual((rigs[0]["shirt"], rigs[0]["skin"]), ("#ff0000", "#f4dcc8"))
        self.assertIsNotNone(rigs[1])
        self.assertIn("direction.B01.s0.layers2.rig", [i["where"] for i in codes(rep, "D12")])
        d15 = [i["where"] for i in codes(rep, "D15")]
        self.assertEqual(d15.count("rigs.KID"), 2)  # not in art.json cast, and no drawn parts
        self.assertNotIn("rigs.AN", d15)


class Planes(unittest.TestCase):
    """Full-frame planes, blends, swaps and depth limits."""

    def plan(self, layers, d=None, cam="push:1.2"):
        d = {**(d or {}), "beats": {"B01": {"shots": [{"art": "P1", "cam": cam, "layers": layers}]}}}
        return dirn.plan(d, timeline(), GEO, Layers.ART, {})

    def Ls(self, p):
        return p["beats"]["B01"]["shots_resolved"][0]["layers"]

    def test_a_fill_plane_is_overscanned_past_its_parallax(self):
        p, rep = self.plan([{"art": "HERO", "anchor": "fill", "z": 0.6}], cam="pan-left")
        L = self.Ls(p)[0]
        for e in (L["cam"]["from"], L["cam"]["to"]):
            self.assertGreaterEqual((1 + 2 * L["ov"]) * e["s"] / 2, 0.5 + abs(e["x"]))
        self.assertIn("mvk-lyfill", dirn._layer_html(L, GEO["W"], GEO["H"]))
        _, rep = self.plan([{"art": "HERO", "z": -1.5}])
        self.assertTrue(codes(rep, "D12"))

    def test_opaque_line_art_is_fine_once_it_multiplies(self):
        _, rep = self.plan([{"art": "FLAT"}])
        self.assertTrue(codes(rep, "D12"))
        p, rep = self.plan([{"art": "FLAT", "blend": "multiply"}])
        self.assertEqual(codes(rep, "D12"), [])
        self.assertIn("mix-blend-mode:multiply", dirn._layer_html(self.Ls(p)[0], GEO["W"], GEO["H"]))
        with tempfile.TemporaryDirectory():
            pass
        dd = {"look": {"preset": "stickman"}}
        p, rep = dirn.plan({**dd, "beats": {"B01": {"shots": [{"art": "P1", "layers": [{"art": "FLAT"}]}]}}},
                           timeline(), GEO, Layers.ART, {})
        self.assertEqual(codes(rep, "D12"), [])  # the stickman preset multiplies its line art by default
        self.assertEqual(self.Ls(p)[0]["blend"], "multiply")
        _, rep = self.plan([{"art": "HERO", "blend": "glow"}])
        self.assertTrue(codes(rep, "D12"))

    def test_a_swap_changes_the_drawing_on_its_word(self):
        art = {**Layers.ART, "HERO2": {"path": "assets/art/HERO2.png", "approved": True, "cutout": True}}
        d = {"beats": {"B01": {"shots": [{"art": "P1", "layers": [{"art": "HERO", "swaps": [{"at": "word:vỡ", "art": "HERO2"}]}]}]}}}
        p, rep = dirn.plan(d, timeline(), GEO, art, {})
        L = self.Ls(p)[0]
        self.assertEqual(codes(rep, "D12"), [])
        self.assertEqual((L["srcs"], L["swaps"][0]["k"]), (["assets/art/HERO.png", "assets/art/HERO2.png"], 1))
        self.assertIn(round(L["swaps"][0]["at"], 3), p["events"])
        self.assertEqual(dirn._layer_html(L, GEO["W"], GEO["H"]).count("mvk-lyart"), 2)


class Speedlines(unittest.TestCase):
    def fx(self, **f):
        d = {"beats": {"B01": {"shots": [{"art": "P1"}], "fx": [{"type": "speedlines", "at": "word:vỡ", **f}]}}}
        p, rep = dirn.plan(d, timeline(), GEO, ART, {})
        return p["beats"]["B01"]["spec"]["fx"], rep

    def test_seeded_lines_flicker_through_their_variants(self):
        (a,), rep = self.fx(kind="radial", seed=3)
        (b,), _ = self.fx(kind="radial", seed=3)
        self.assertEqual(rep.items and [i for i in rep.items if i["level"] == "FAIL"], [])
        self.assertEqual(len(a["polys"]), 3)
        self.assertEqual(a["polys"], b["polys"])  # the same frame on every render
        self.assertEqual(a["color"], "var(--mv-on-plate)")
        (c,), _ = self.fx(kind="linear", angle=20, color="#000")
        self.assertIn('class="mvk-speed"', dirn._fx_html(c))
        self.assertEqual(c["color"], "#000")
        _, rep = self.fx(kind="spiral")
        self.assertTrue(codes(rep, "D8"))


class SetPieces(unittest.TestCase):
    """The props sheet, sheet merges, scoped ids, campaign media inside a drawing, and shot-local anchors."""

    def check(self, layers, rigs=None, props=None, media=None):
        with tempfile.TemporaryDirectory() as t:
            cd = campaign(Path(t), {"beats": {"B01": {"shots": [{"color": "ground", "layers": layers}]}, "B02": shot()}})
            for k, v in (("rigs", rigs), ("props", props)):
                if v:
                    (cd / f"07-video/{k}.json").write_text(json.dumps({k: v}))
            return dirn.check(cd, "s01", timeline(), GEO, ART, media or {})

    def Ls(self, p):
        return p["beats"]["B01"]["shots_resolved"][0]["layers"]

    def test_a_prop_is_drawn_once_and_placed_by_name(self):
        props = {"STONE": {"svg": "<circle class='stone' r='40'/>", "viewBox": "-50 -50 100 100", "anchor": "center", "z": 0}}
        p, rep = self.check([{"prop": "STONE", "x": 0.3, "h": 0.05}, {"prop": "STONE", "z": 0.4}, {"prop": "ROCK"}], props=props)
        a, b = self.Ls(p)
        self.assertEqual((a["x"], a["h"], a["z"], a["anchor"]), (0.3, 0.05, 0.0, "center"))
        self.assertEqual(b["z"], 0.4)  # the layer's own keys win over the sheet's
        self.assertIn("stone", dirn._layer_html(a, GEO["W"], GEO["H"]))
        self.assertEqual([i["where"] for i in codes(rep, "D12")], ["direction.B01.s0.layers2"])

    def test_use_appends_parts_and_merges_faces(self):
        an = {"type": "stick", "parts": [{"on": "torso", "svg": "<path class='collar' d='M0 0'/>"}],
              "faces": {"calm2": ["<circle r='1'/>", "<path d='M0 0'/>"]}}
        p, rep = self.check([{"rig": {"use": "AN", "parts": [{"on": "handR", "svg": "<g class='knife'/>"}],
                                      "faces": {"grin": ["<circle r='1'/>", "<path d='M0 1'/>"]}}, "face": "calm2"}],
                            rigs={"AN": an})
        self.assertEqual(codes(rep, "D12"), [])
        out = dirn._layer_html(self.Ls(p)[0], GEO["W"], GEO["H"])
        self.assertIn("collar", out)
        self.assertIn("knife", out)

    def test_ids_are_scoped_per_layer(self):
        out = rig.scope_ids('<pattern id="dots"/><rect fill="url(#dots)"/><use href="#dots"/>', "B01-s0-L3")
        self.assertEqual(out, '<pattern id="B01-s0-L3-dots"/><rect fill="url(#B01-s0-L3-dots)"/><use href="#B01-s0-L3-dots"/>')

    def test_a_drawing_can_hold_a_campaign_file(self):
        phone = {"svg": '<image href="media:01-product/screens/ch8.png" width="10" height="20"/>', "viewBox": "0 0 10 20"}
        p, rep = self.check([phone], media={"01-product/screens/ch8.png": "assets/media/ch8.png"})
        self.assertEqual(codes(rep, "D12"), [])
        self.assertIn('href="assets/media/ch8.png"', dirn._layer_html(self.Ls(p)[0], GEO["W"], GEO["H"]))
        _, rep = self.check([phone])
        self.assertTrue(codes(rep, "D12"))

    def test_start_is_the_shots_own_and_offsets_chain(self):
        d = {"beats": {"B01": {"shots": [{"color": "ground"}, {"color": "ground", "at": "word:nó",
                                          "layers": [{"svg": "<path/>", "enter": {"type": "fade", "at": "start+0.2-0.1"}}]}]}}}
        p, rep = dirn.plan(d, timeline(), GEO, ART, {})
        s = p["beats"]["B01"]["shots_resolved"][1]
        self.assertEqual([i for i in rep.items if i["level"] == "FAIL"], [])
        self.assertAlmostEqual(s["layers"][0]["enter"]["at"], s["start"] + 0.1)

    def test_an_endtag_goes_below_captions_set_at_the_top(self):
        fx = [{"type": "endtag", "text": "Kiếm Lai", "sub": "chương 8", "at": "word:nó"}]
        for cap_y, below in ((190, True), (None, False)):
            d = {"beats": {"B01": {"shots": [{"color": "ground"}], "fx": fx}}}
            if cap_y:
                d["captions"] = {"y": cap_y}
            p, rep = dirn.plan(d, timeline(), GEO, ART, {})
            box = p["beats"]["B01"]["spec"]["fx"][0]["box"]
            self.assertGreaterEqual(box["top"], 0)
            if below:
                self.assertGreater(box["top"], cap_y + 2 * GEO["cap_size"])
            self.assertEqual(codes(rep, "D9"), [])


class StickmanDefault(unittest.TestCase):
    """The comic stickman is the default look (campaign.json style); the chibi rig; the raw hatches beside the presets."""

    def plan(self, beat, d=None):
        d = {**(d or {}), "beats": {"B01": beat}}
        return dirn.plan(d, timeline(), GEO, Layers.ART, {})

    def test_the_campaign_style_fills_a_direction_without_a_preset_and_none_opts_out(self):
        with tempfile.TemporaryDirectory() as t:
            cd = campaign(Path(t), {"beats": {}})
            self.assertNotIn("preset", (dirn.load_direction(cd, "s01").get("look") or {}))  # no style: an older campaign
            (cd / "campaign.json").write_text(json.dumps({"style": "stickman"}))
            self.assertEqual(dirn.load_direction(cd, "s01")["look"]["preset"], "stickman")
            dirn.path(cd, "s01").write_text(json.dumps({"look": {"preset": "none"}, "beats": {}}))
            self.assertEqual(dirn.load_direction(cd, "s01")["look"]["preset"], "none")
            self.assertEqual(dirn.preset_of(dirn.load_direction(cd, "s01")), {})
        lk = dirn.look_of({"look": {"preset": "stickman"}}, {"colors": {}})
        self.assertTrue(lk["paper"])
        self.assertEqual(dirn.captions_of({"look": {"preset": "stickman"}})["style"], "comic")

    def test_the_chibi_body_draws_hair_blush_and_a_red_mouth_and_blinks_on_a_seed(self):
        L = {"rig": {"type": "stick", "body": "chibi", "hair": {"style": "spiky", "color": "#e2445c"},
                     "blush": "#f29aa0"}, "face": "happy"}
        p, rep = self.plan({"shots": [{"color": "ground", "layers": [L]}]})
        self.assertEqual([i for i in rep.items if i["level"] == "FAIL"], [])
        Lr = p["beats"]["B01"]["shots_resolved"][0]["layers"][0]
        out = dirn._layer_html(Lr, GEO["W"], GEO["H"])
        self.assertIn("#e2445c", out)
        laugh = self.plan({"shots": [{"color": "ground", "layers": [{**L, "face": "laugh"}]}]})[0]
        self.assertIn("#b8333d", dirn._layer_html(laugh["beats"]["B01"]["shots_resolved"][0]["layers"][0], GEO["W"], GEO["H"]),
                      "the chibi laugh is an open red mouth")
        self.assertIn("rig-ey", out)  # happy keeps its eyes in .rig-ey: they blink
        self.assertIn("#f29aa0", out)
        blinks = Lr["rig"]["blinks"]
        self.assertTrue(blinks)
        self.assertEqual(blinks, self.plan({"shots": [{"color": "ground", "layers": [L]}]})[0]["beats"]["B01"]
                         ["shots_resolved"][0]["layers"][0]["rig"]["blinks"], "the same layer blinks the same every render")
        _, rep = self.plan({"shots": [{"color": "ground", "layers": [{**L, "rig": {**L["rig"], "hair": "mohawk"}}]}]})
        self.assertTrue([i for i in codes(rep, "D12") if i["where"].endswith(".rig.hair")])

    def test_comic_captions_box_each_word_in_ink(self):
        from mvflib.scaffold import captions_html
        groups = [{"s": 0.1, "e": 0.9, "tokens": [{"text": "ông", "s": 0.1, "e": 0.4, "key": False}]}]
        brand = {"colors": {"accent": "#f00", "ink": "#000", "paper": "#fff", "fg": "#fff", "bg": "#000"},
                 "fonts": {"caption": "Inter", "display": "Inter", "body": "Inter"}}
        lk = dirn.look_of({"look": {"preset": "stickman"}}, {"colors": {}})
        out = captions_html(groups, GEO, brand, 4.0, {"style": "comic", "css": ".cap .w { rotate: -2deg; }"}, lk)
        self.assertIn("solid #1b1b1b", out)
        self.assertIn("color: #1b1b1b", out)
        self.assertIn("rotate: -2deg", out)  # captions.css: the agent's own rules ride after the generated ones

    def test_pointer_draws_a_leader_line_from_its_lettering_to_a_dot(self):
        p, rep = self.plan({"shots": [{"color": "ground"}], "fx": [{"type": "pointer", "text": "*cơ* bắp", "point": [0.6, 0.5],
                                                                    "label": [0.3, 0.3], "at": "word:đập"}]})
        f = p["beats"]["B01"]["spec"]["fx"][0]
        self.assertFalse([i for i in rep.items if i["level"] == "FAIL"], rep.items)
        out = dirn._fx_html(f)
        self.assertIn('pathLength="1"', out)
        self.assertIn(f'cx="{round(0.6 * GEO["W"])}"', out)
        self.assertIn('<span class="hot">cơ</span>', out)
        _, rep = self.plan({"shots": [{"color": "ground"}], "fx": [{"type": "pointer", "text": "x"}]})
        self.assertTrue(codes(rep, "D8"))

    def test_raw_markup_and_tweens_are_first_class(self):
        svg = '<svg viewBox="0 0 10 10"><path d="M0 0 L10 10" pathLength="1"/></svg>'
        p, rep = self.plan({"shots": [{"html": '<div class="sky">mine</div>'}],
                            "fx": [{"type": "html", "html": svg, "style": "left:10px;top:20px;width:300px", "in": "draw"}],
                            "tweens": [{"at": "word:vỡ", "sel": ".sky", "to": {"rotation": 4, "repeat": -1, "yoyo": True}, "dur": 0.2}]})
        self.assertFalse([i for i in rep.items if i["level"] == "FAIL"], rep.items)
        html_out = dirn.scene_html("B01", p["beats"]["B01"], GEO)
        self.assertIn('<div class="sky">mine</div>', html_out)
        self.assertIn(svg, html_out)
        tw = p["beats"]["B01"]["spec"]["tweens"][0]
        self.assertEqual(tw["sel"], ".sky")
        self.assertGreater(tw["to"]["repeat"], 0)
        _, rep = self.plan({"shots": [{"color": "ground"}], "tweens": [{"at": "start", "to": {"x": 1}}]})
        self.assertTrue(codes(rep, "D12"))

    def test_taste_is_a_warning_not_a_wall(self):
        _, rep = self.plan({"shots": [{"color": "ground"}, {"color": "ink", "at": "word:ông+0.1"}]})
        self.assertEqual([i["level"] for i in rep.items if i["code"] == "D4"], ["WARN"])  # a flash cut is allowed
        _, rep = self.plan({"shots": [{"color": "ground", "layers": [{"rig": "stick", "z": -2}]}]})
        self.assertFalse([i for i in rep.items if i["level"] == "FAIL"], rep.items)
