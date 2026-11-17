"""Cast: characters drawn from the adaptation viewers saw (A10/A11), the design in every prompt, the still as ref."""
import json
import sys
import tempfile
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(TOOLS))

from mvflib import art  # noqa: E402

DESIGN = "slim boy, tanned, black topknot with a straw cord, grey patched robe, straw sandals, calm dark eyes"


def codes(rep, level=None):
    return [i["code"] for i in rep.items if level is None or i["level"] == level]


class Cast(unittest.TestCase):
    def run_check(self, cast, jobs=None, adaptations=None, files=()):
        with tempfile.TemporaryDirectory() as t:
            cd = Path(t)
            (cd / art.ART).mkdir(parents=True)
            for f in files:
                (cd / f).parent.mkdir(parents=True, exist_ok=True)
                (cd / f).write_bytes(b"x")
            jobs = jobs or [{"id": "Q01", "use": "plate", "prompt": "a boy in a lane", "cast": list(cast)}]
            (cd / art.ART / "art.json").write_text(json.dumps({"style": "ink", "cast": cast, "jobs": jobs}))
            if adaptations is not None:
                (cd / "03-research").mkdir()
                (cd / "03-research/research.json").write_text(json.dumps({"adaptations": adaptations}))
            return art.check(cd)

    def test_animated_design_with_a_still_passes(self):
        src = {"kind": "animation", "title": "Kiếm Lai (donghua)", "url": "https://example.com/a",
               "image": "07-video/art/cast/an.jpg"}
        self.assertNotIn("A10", codes(self.run_check({"an": {"design": DESIGN, "source": src}},
                                                     files=["07-video/art/cast/an.jpg"])))
        del src["image"]
        self.assertIn("A10", codes(self.run_check({"an": {"design": DESIGN, "source": src}}), "WARN"))

    def test_live_action_still_never_goes_to_the_model(self):
        src = {"kind": "live-action", "title": "a drama", "url": "https://example.com/d", "image": "still.jpg"}
        self.assertIn("A11", codes(self.run_check({"an": {"design": DESIGN, "source": src}}, files=["still.jpg"]),
                                   "FAIL"))

    def test_unknown_cast_and_missing_design_fail(self):
        rep = self.run_check({"an": {"design": "boy", "source": {"kind": "none"}}},
                             jobs=[{"id": "Q01", "prompt": "a boy", "cast": ["an", "girl"]}])
        self.assertEqual(codes(rep, "FAIL").count("A10"), 2)

    def test_known_adaptation_without_cast_warns(self):
        rep = self.run_check({}, jobs=[{"id": "Q01", "prompt": "a boy"}], adaptations=[{"title": "donghua", "kind": "animation"}])
        self.assertIn("A10", codes(rep, "WARN"))

    def test_design_goes_into_the_prompt_and_the_still_is_the_ref(self):
        cast = {"an": {"design": DESIGN, "source": {"kind": "animation", "image": "cast/an.jpg"}}}
        job = {"id": "Q01", "prompt": "a boy in a lane", "cast": ["an"]}
        self.assertIn("Character look: " + DESIGN, art.full_prompt(job, "ink", cast))
        self.assertEqual(art.cast_ref(Path("."), job, cast), "cast/an.jpg")
        self.assertEqual(art.cast_ref(Path("."), {**job, "ref": "K0"}, cast), "K0")
        live = {"an": {"design": DESIGN, "source": {"kind": "live-action", "image": "x.jpg"}}}
        self.assertIsNone(art.cast_ref(Path("."), job, live))


if __name__ == "__main__":
    unittest.main()
