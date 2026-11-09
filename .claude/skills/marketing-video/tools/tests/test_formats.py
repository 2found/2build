"""Saved formats (mvflib/formats.py): save a series' look by name, list it, start a new campaign from it."""
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(TOOLS))

from mvflib import direction, formats  # noqa: E402


def mvf(*args, cwd):
    return subprocess.run([sys.executable, str(TOOLS / "mvf.py"), *args], cwd=cwd, capture_output=True, text=True)


class Formats(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name) / "campaigns"
        r = mvf("init", "ep1", "--product", "Acme", "--root", str(self.root), cwd=self.tmp.name)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.cd = self.root / "ep1"
        c = json.loads((self.cd / "campaign.json").read_text())
        (self.cd / "campaign.json").write_text(json.dumps({**c, "style": "stickman"}))
        (self.cd / "07-video/s01").mkdir(parents=True)
        (self.cd / "07-video/s01/direction.json").write_text(json.dumps({
            "look": {"preset": "stickman", "accent": "#c33"}, "captions": {"style": "comic", "words": 3},
            "sound": {"ambience": [{"kind": "wind", "db": -34}], "cues": [{"at": "B01@word:x", "kind": "hit"}]},
            "beats": {"B01": {"shots": []}}}))
        (self.cd / "07-video/rigs.json").write_text(json.dumps({"hero": {"kind": "stick"}}))
        (self.cd / "05-scripts/s01.json").write_text(json.dumps({"id": "s01", "voice": {"mode": "clone", "ref": "thay"}}))
        (self.cd / "voices/thay.wav").write_bytes(b"RIFF")
        (self.cd / "voices/thay.txt").write_text("lời mẫu")

    def tearDown(self):
        self.tmp.cleanup()

    def test_save_list_and_start_from_it(self):
        r = mvf("format", "save", "kiem-tu", "--from", "s01", cwd=self.cd)
        self.assertEqual(r.returncode, 0, r.stderr)
        saved = json.loads((self.root / "_formats/kiem-tu/format.json").read_text())
        self.assertEqual(saved["from"], "ep1/s01")
        self.assertEqual(saved["sound"], {"ambience": [{"kind": "wind", "db": -34}]})  # cues are one script's
        self.assertEqual(saved["voice"]["ref"], "thay")
        r = mvf("format", "ls", cwd=self.cd)
        self.assertIn("kiem-tu: from ep1/s01", r.stdout)
        self.assertIn("voice thay, rigs.json", r.stdout)

        r = mvf("init", "ep2", "--product", "Acme", "--root", str(self.root), "--format", "kiem-tu", cwd=self.tmp.name)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn('voice.ref = "thay"', r.stdout)
        cd2 = self.root / "ep2"
        self.assertEqual(json.loads((cd2 / "campaign.json").read_text())["style"], "stickman")
        self.assertTrue((cd2 / "07-video/rigs.json").is_file())
        self.assertTrue((cd2 / "voices/thay.txt").is_file())
        (cd2 / "07-video/s01").mkdir()
        (cd2 / "07-video/s01/direction.json").write_text(json.dumps({"look": {"accent": "#00f"}, "beats": {}}))
        d = direction.load_direction(cd2, "s01")
        self.assertEqual(d["look"], {"preset": "stickman", "accent": "#00f"})  # the episode's own key wins
        self.assertEqual(d["captions"], {"style": "comic", "words": 3})
        self.assertEqual(d["sound"]["ambience"][0]["kind"], "wind")
        self.assertNotIn("kiem-tu", [x.name for x in self.root.iterdir() if not x.name.startswith("_")])

    def test_refusals(self):
        self.assertNotEqual(mvf("format", "save", "x", "--from", "s09", cwd=self.cd).returncode, 0)
        self.assertEqual(mvf("format", "save", "x", "--from", "s01", cwd=self.cd).returncode, 0)
        r = mvf("format", "save", "x", "--from", "s01", cwd=self.cd)
        self.assertIn("--force", r.stderr)
        self.assertEqual(mvf("format", "save", "x", "--from", "s01", "--force", cwd=self.cd).returncode, 0)
        r = mvf("init", "ep3", "--product", "Acme", "--root", str(self.root), "--format", "nope", cwd=self.tmp.name)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("have: x", r.stderr)
        self.assertFalse((self.root / "ep3").exists())  # a bad format name leaves no half-made campaign
        self.assertNotEqual(mvf("format", "save", "../evil", "--from", "s01", cwd=self.cd).returncode, 0)
        (self.root / "x2/format.json").parent.mkdir()
        (self.root / "x2/format.json").write_text("{}")  # outside _formats/: init must not reach it
        r = mvf("init", "ep4", "--product", "Acme", "--root", str(self.root), "--format", "../x2", cwd=self.tmp.name)
        self.assertNotEqual(r.returncode, 0)
        self.assertFalse((self.root / "ep4").exists())
        keep = self.root / "_formats/hand"
        keep.mkdir()
        (keep / "notes.md").write_text("mine")  # a folder with no format.json is still not ours to delete
        self.assertIn("--force", mvf("format", "save", "hand", "--from", "s01", cwd=self.cd).stderr)
        self.assertTrue((keep / "notes.md").is_file())
        s = json.loads((self.cd / "05-scripts/s01.json").read_text())
        (self.cd / "05-scripts/s01.json").write_text(json.dumps({**s, "voice": {"mode": "clone", "ref": "../../x"}}))
        self.assertIn("voice.ref", mvf("format", "save", "y", "--from", "s01", cwd=self.cd).stderr)

    def test_a_broken_applied_format_is_ignored(self):
        (self.cd / formats.APPLIED).write_text("{")
        self.assertEqual(direction.load_direction(self.cd, "s01")["captions"]["style"], "comic")


if __name__ == "__main__":
    unittest.main()
