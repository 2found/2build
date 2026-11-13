"""Sound: SFX rendered from sfx.json cues, procedural beds, and the voice-over-music check."""
import json
import sys
import tempfile
import unittest
from pathlib import Path

import numpy as np

TOOLS = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(TOOLS))

from mvflib import sound, synth  # noqa: E402


def project(tmp: Path, cues, vo=True) -> Path:
    p = tmp / "proj"
    (p / "assets/vo").mkdir(parents=True)
    beats = [{"id": "B01", "cue": "B01" if vo else None, "vo_start": 0.5 if vo else None, "vo_end": 1.5 if vo else None,
              "vo_trim": 0.0}]
    (p / "timeline.json").write_text(json.dumps({"duration": 3.0, "beats": beats}))
    (p / "sfx.json").write_text(json.dumps({"cues": cues}))
    if vo:
        t = np.arange(int(1.0 * synth.SR)) / synth.SR
        sound.write(p / "assets/vo/B01.wav", 0.5 * np.sin(2 * np.pi * 220 * t))
    return p


class Synth(unittest.TestCase):
    def test_every_one_shot_renders_finite_and_peaks(self):
        for k, f in synth.ONE_SHOTS.items():
            x = synth.norm(f(3))
            self.assertTrue(np.isfinite(x).all(), k)
            self.assertAlmostEqual(float(np.max(np.abs(x))), 1.0, places=3, msg=k)

    def test_moods_render_to_length(self):
        for m in synth.MOODS:
            x = synth.mood_bed(m, 4.0)
            self.assertEqual(len(x), 4 * synth.SR, m)
            self.assertTrue(np.isfinite(x).all(), m)


class Render(unittest.TestCase):
    def test_cues_land_at_their_level_and_unknown_kinds_are_reported(self):
        with tempfile.TemporaryDirectory() as d:
            p = project(Path(d), [{"t": 2.0, "kind": "thump", "db": -12}, {"t": 0.1, "kind": "nope", "db": -20}], vo=False)
            rep = sound.render_sfx(p)
            self.assertEqual(rep["unknown"], ["nope"])
            x = sound.decode(p / "assets/music/sfx.wav")
            self.assertLess(np.max(np.abs(x[: int(1.9 * synth.SR)])), 1e-3)          # silent before the cue
            self.assertAlmostEqual(20 * np.log10(np.max(np.abs(x))), -12, delta=0.5)  # peak = the cue's dB

    def test_mixcheck_flags_a_hit_under_speech(self):
        with tempfile.TemporaryDirectory() as d:
            p = project(Path(d), [{"t": 0.9, "kind": "boom", "db": 0}])
            sound.render_sfx(p)
            rep = sound.mixcheck(p)
            self.assertEqual(rep["under"], ["B01"])
            (p / "sfx.json").write_text(json.dumps({"cues": [{"t": 2.2, "kind": "boom", "db": -3}]}))
            sound.render_sfx(p)
            self.assertEqual(sound.mixcheck(p)["under"], [])

    def test_product_audio_plays_in_the_pauses_and_never_under_speech(self):
        with tempfile.TemporaryDirectory() as d:
            p = project(Path(d), [])
            t = np.arange(int(4.0 * synth.SR)) / synth.SR
            sound.write(Path(d) / "player.wav", 0.1 * np.sin(2 * np.pi * 330 * t))
            (p / sound.PRODUCT).write_text(json.dumps({"clip": "player.wav", "in": 0.5}))
            rep = sound.product_track(p, Path(d))
            self.assertEqual(rep["windows"], [[0.0, 0.5], [1.5, 3.0]])
            x = sound.decode(p / "assets/music/product.wav")
            sr = synth.SR
            self.assertLess(np.max(np.abs(x[int(0.7 * sr):int(1.3 * sr)])), 1e-3)   # silent under the line
            self.assertAlmostEqual(sound._db(x[int(1.8 * sr):int(2.8 * sr)]), sound._db(0.5 * np.sin(2 * np.pi * 220 * t)), delta=1.0)
            mc = sound.mixcheck(p)
            self.assertEqual(mc["under"], [])
            self.assertEqual(mc["product_first_s"], 0.0)


if __name__ == "__main__":
    unittest.main()
