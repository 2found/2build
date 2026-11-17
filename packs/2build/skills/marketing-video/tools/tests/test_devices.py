"""Devices, cut rhythm, anticipation and key moments in direction (dots, ladder, DV1, V5, V6, antic, key cue/duck)."""
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from mvflib import direction as dirn  # noqa: E402
from mvflib import mix  # noqa: E402
from test_direction import ART, GEO, Layers, campaign, shot, timeline  # noqa: E402


def fails(rep, code):
    return [x for x in rep.items if x["code"] == code and x["level"] == "FAIL"]


def warns(rep, code):
    return [x for x in rep.items if x["code"] == code and x["level"] == "WARN"]


def plan_fx(fx):
    return dirn.plan({"beats": {"B01": shot(), "B02": {**shot(), "fx": fx}}}, timeline(), GEO, ART, {})


class Dots(unittest.TestCase):
    def test_grid_lights_lit_of_total(self):
        p, rep = plan_fx([{"type": "dots", "total": 100, "lit": 3, "text": "*3* in 100", "sub": "1 dot = 100 people"}])
        self.assertFalse(fails(rep, "D8"), rep.items)
        f = p["beats"]["B02"]["spec"]["fx"][0]
        html = dirn._fx_html(f)
        self.assertEqual(html.count("<i"), 100)
        self.assertEqual(html.count('class="lit"'), 3)
        self.assertIn("1 dot = 100 people", html)
        self.assertIn("ticks", [x["kind"] for x in p["sfx"]])

    def test_bad_params_fail_with_a_reason(self):
        for bad in ({"total": 5000, "lit": 1}, {"total": 10, "lit": 11}, {"total": "many"}):
            _, rep = plan_fx([{"type": "dots", **bad}])
            self.assertTrue(fails(rep, "D8"), bad)


class Ladder(unittest.TestCase):
    def test_two_columns_with_a_marked_rung(self):
        p, rep = plan_fx([{"type": "ladder", "left": [f"L{i}" for i in range(15)], "right": [f"V{i}" for i in range(11)],
                           "heads": ["Luyện khí sĩ", "Võ phu"], "mark": ["r", 3]}])
        self.assertFalse(fails(rep, "D8"), rep.items)
        html = dirn._fx_html(p["beats"]["B02"]["spec"]["fx"][0])
        self.assertEqual(html.count("<p"), 26)
        self.assertIn('<p class="hot">V3</p>', html)
        self.assertIn("<h4>Võ phu</h4>", html)

    def test_a_tall_ladder_shrinks_to_end_above_the_captions(self):
        p, rep = plan_fx([{"type": "ladder", "left": [f"L{i}" for i in range(16)], "right": ["a", "b"]}])
        self.assertFalse(fails(rep, "D9"), rep.items)
        b = p["beats"]["B02"]["spec"]["fx"][0]["box"]
        self.assertLessEqual(b["top"] + b["h"], GEO["cap_top"] - 12)
        self.assertLess(b["size"], dirn.TEXT_SIZE["ladder"])

    def test_needs_both_sides_and_a_real_mark(self):
        _, rep = plan_fx([{"type": "ladder", "left": ["a"]}])
        self.assertTrue(fails(rep, "D8"))
        _, rep = plan_fx([{"type": "ladder", "left": ["a"], "right": ["b"], "mark": ["r", 4]}])
        self.assertTrue(fails(rep, "D8"))


class DeviceShown(unittest.TestCase):
    def check(self, fx, device=None):
        with tempfile.TemporaryDirectory() as t:
            cd = campaign(Path(t), {"beats": {"B01": shot(), "B02": {**shot(), "fx": fx}}})
            s = json.loads((cd / "05-scripts/s01.json").read_text())
            s["beats"][1]["vo"] = "đời ông bị mua ba lần"
            if device:
                s["beats"][1]["device"] = device
            (cd / "05-scripts/s01.json").write_text(json.dumps(s, ensure_ascii=False))
            return dirn.check(cd, "s01", timeline(), GEO, ART, {})[1]

    def test_number_line_without_a_device_effect(self):
        self.assertEqual([x["where"] for x in warns(self.check([]), "DV1")], ["direction.B02"])
        self.assertFalse(warns(self.check([{"type": "counter", "to": 3}]), "DV1"))
        self.assertFalse(warns(self.check([], device="none"), "DV1"))


class Rhythm(unittest.TestCase):
    def tl(self, n, each):
        t, beats = 0.0, []
        for i in range(n):
            beats.append({"id": f"B{i + 1:02d}", "start": t, "end": t + each[i], "words": []})
            t += each[i]
        return {"duration": t, "beats": beats}

    def check(self, cams, each):
        tl = self.tl(len(cams), each)
        d = {"beats": {b["id"]: {"shots": [{"art": "P1", "cam": c}]} for b, c in zip(tl["beats"], cams)}}
        p, rep = dirn.plan(d, tl, GEO, ART, {})
        dirn.variety(p, tl, rep)
        return rep

    def test_same_move_on_adjacent_shots(self):
        rep = self.check(["push", "push:1.4", "pull"], [2.0, 3.0, 4.0])
        self.assertEqual([x["where"] for x in warns(rep, "V5")], ["direction.B02-s0"])
        self.assertFalse(warns(self.check(["drift", "drift", "hold"], [2.0, 3.0, 4.0]), "V5"))

    def test_even_shot_lengths(self):
        cams = ["push", "pull", "pan-left", "tilt-up", "push"]
        self.assertTrue(warns(self.check(cams, [2.0, 2.1, 2.0, 1.9, 2.0]), "V6"))
        self.assertFalse(warns(self.check(cams, [1.0, 3.5, 1.2, 4.0, 2.0]), "V6"))
        self.assertFalse(warns(self.check(cams[:4], [2.0, 2.0, 2.0, 2.0]), "V6"))


class Anticipation(unittest.TestCase):
    def moves(self, m):
        p, _ = dirn.plan({"beats": {"B01": {"shots": [{"art": "P1", "layers": [{"art": "HERO", "x": 0.5, "moves": [m]}]}]}}},
                         timeline(), GEO, Layers.ART, {})
        return p["beats"]["B01"]["shots_resolved"][0]["layers"][0]["moves"]

    def test_default_on_a_real_move_with_opt_outs(self):
        self.assertEqual(self.moves({"at": "word:đập", "x": 0.2})[0]["antic"], dirn.ANTIC)
        for m in ({"at": "word:đập", "x": 0.2, "antic": False}, {"at": "word:đập", "x": 0.2, "ease": "none"},
                  {"at": "word:đập", "x": 0.51}, {"at": "word:đập", "x": 0.2, "dur": 0.2}):
            self.assertNotIn("antic", self.moves(m)[0], m)

    def test_rig_pose_change(self):
        layer = {"rig": "stick", "pose": "stand", "poses": [{"at": "word:đập", "pose": "wave", "dur": 0.4},
                                                            {"at": "word:nó", "pose": "stand", "dur": 0.4, "antic": False}]}
        p, _ = dirn.plan({"beats": {"B01": {"shots": [{"color": "ground", "cam": "hold", "layers": [layer]}]}}},
                         timeline(), GEO, Layers.ART, {})
        tws = p["beats"]["B01"]["shots_resolved"][0]["layers"][0]["rig"]["tweens"]
        self.assertEqual([t.get("antic") for t in sorted(tws, key=lambda t: t["at"])], [dirn.ANTIC, None])


class KeyMoments(unittest.TestCase):
    def tl(self):
        tl = timeline()
        tl["beats"][1].update(key=True, vo_start=2.0, vo_end=3.2)
        return tl

    def test_key_line_gets_one_hit_unless_a_cue_is_there(self):
        p, _ = dirn.plan({"beats": {"B01": shot(), "B02": shot()}}, self.tl(), GEO, ART, {})
        self.assertEqual([(c["t"], c["kind"]) for c in p["sfx"] if c["src"] == "B02.key"], [(3.2, "hit")])
        busy = {**shot(), "sfx": [{"at": "word:mua", "kind": "boom"}]}
        p, _ = dirn.plan({"beats": {"B01": shot(), "B02": busy}}, self.tl(), GEO, ART, {})
        self.assertFalse([c for c in p["sfx"] if c["src"] == "B02.key"])
        p, _ = dirn.plan({"beats": {"B01": shot(), "B02": {**shot(), "key_sfx": False}}}, self.tl(), GEO, ART, {})
        self.assertFalse([c for c in p["sfx"] if c["src"] == "B02.key"])

    def test_bed_ducks_over_the_key_hold(self):
        self.assertEqual(mix.key_holds(self.tl()), [(3.2, 4.0)])
        self.assertEqual(mix.key_holds(timeline()), [])


if __name__ == "__main__":
    unittest.main()
