"""Channels (mvflib/channel.py): a campaign lives in <repo>/marketing/channels/<channel>/campaigns/<slug>/."""
import json
import os
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mvflib import channel  # noqa: E402
from test_extensions import mvf, repo  # noqa: E402

SKELETON = ("AGENTS.md", "README.md", "docs/01-bible.md", "docs/02-production-workflow.md", "docs/03-decisions.md",
            "pack/campaign.json", "pack/series/_example.json", "metrics/videos.csv", "metrics/weekly.md",
            "campaigns/.gitkeep", "brand/.gitkeep")


def write(p: Path, data) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(json.dumps(data, ensure_ascii=False) if not isinstance(data, str) else data)


def read(p: Path):
    return json.loads(p.read_text())


class Channel(unittest.TestCase):
    def setUp(self):
        self.d, self.root = repo()
        self.lib = self.root / ".babysit/marketing-video/library/products/acme"

    def tearDown(self):
        self.d.cleanup()

    def test_a_campaign_opens_in_the_products_channel_with_its_skeleton(self):
        mvf(self.root, "init", "ep1", "--product", "Acme")
        chd = self.root / "marketing/channels/acme"
        self.assertTrue((chd / "campaigns/ep1/campaign.json").is_file())
        for rel in SKELETON:
            self.assertTrue((chd / rel).is_file(), rel)
            self.assertNotIn("{{", (chd / rel).read_text(), rel)
        self.assertTrue((chd / "CLAUDE.md").is_symlink())
        self.assertEqual(os.readlink(chd / "CLAUDE.md"), "AGENTS.md")
        self.assertIn("--channel acme", (chd / "AGENTS.md").read_text())
        self.assertTrue((chd / "metrics/videos.csv").read_text().startswith("posted,series,slug,"))
        pack = read(chd / "pack/campaign.json")
        self.assertIn("_about", pack)
        self.assertEqual(pack["brand"]["names"], ["Acme"])
        self.assertNotIn("_template", pack)
        self.assertFalse((self.root / "marketing/campaigns").exists())

    def test_channel_flag_names_the_folder(self):
        mvf(self.root, "init", "ep1", "--product", "Acme", "--channel", "Hoả Long")
        self.assertTrue((self.root / "marketing/channels/hoa-long/campaigns/ep1/campaign.json").is_file())
        self.assertFalse((self.root / "marketing/channels/acme").exists())

    def test_root_keeps_the_old_layout(self):
        mvf(self.root, "init", "ep1", "--product", "Acme", "--root", str(self.root / "marketing/campaigns"))
        self.assertTrue((self.root / "marketing/campaigns/ep1/campaign.json").is_file())
        self.assertFalse((self.root / "marketing/channels").exists())
        self.assertFalse((self.lib / "campaign.json").exists())  # no channel pack was synced
        r = mvf(self.root, "init", "ep2", "--product", "Acme", "--root", "x", "--channel", "c", ok=False)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("--root", r.stderr)

    def test_the_skeleton_never_overwrites_a_file(self):
        chd = self.root / "marketing/channels/acme"
        write(chd / "AGENTS.md", "mine")
        write(chd / "CLAUDE.md", "my own claude file")
        write(chd / "docs/01-bible.md", "my bible")
        write(chd / "pack/campaign.json", {"markets": ["VN"]})
        r = mvf(self.root, "channel", "init", "acme", "--product", "Acme")
        self.assertNotIn("AGENTS.md,", r.stdout)
        self.assertEqual((chd / "AGENTS.md").read_text(), "mine")
        self.assertEqual((chd / "CLAUDE.md").read_text(), "my own claude file")
        self.assertEqual((chd / "docs/01-bible.md").read_text(), "my bible")
        self.assertEqual(read(chd / "pack/campaign.json"), {"markets": ["VN"]})
        self.assertTrue((chd / "docs/03-decisions.md").is_file())
        self.assertFalse((chd / "campaigns/ep1").exists())  # channel init opens no campaign
        self.assertIn("nothing written", mvf(self.root, "channel", "init", "acme", "--product", "Acme").stdout)

    def test_the_channel_pack_is_the_product_packs_source(self):
        chd = self.root / "marketing/channels/acme"
        write(self.lib / "campaign.json", {"markets": ["US"], "known_words": ["old"]})
        write(chd / "pack/campaign.json", {"_about": "edit here", "markets": ["VN"], "languages": ["vi"],
                                           "known_words": ["Acme"], "brand": {"names": ["Acme"],
                                                                              "logo": "brand/logo.png"}})
        (chd / "brand").mkdir(parents=True)
        (chd / "brand/logo.png").write_bytes(b"png")
        r = mvf(self.root, "init", "ep1", "--product", "Acme")
        self.assertIn("differed on known_words, markets", r.stdout)
        lib = read(self.lib / "campaign.json")
        self.assertNotIn("_about", lib)
        self.assertEqual(lib["markets"], ["VN"])
        self.assertEqual((self.lib / "brand/logo.png").read_bytes(), b"png")
        c = read(chd / "campaigns/ep1/campaign.json")
        self.assertEqual((c["markets"], c["languages"], c["known_words"]), (["VN"], ["vi"], ["Acme"]))
        self.assertTrue((chd / "campaigns/ep1/brand/logo.png").is_file())
        self.assertEqual(read(chd / "pack/campaign.json")["_about"], "edit here")  # the source is never rewritten

    def test_a_missing_pack_is_written_from_the_library(self):
        write(self.lib / "campaign.json", {"markets": ["JP"], "languages": ["ja"]})
        mvf(self.root, "init", "ep1", "--product", "Acme")
        pack = read(self.root / "marketing/channels/acme/pack/campaign.json")
        self.assertEqual((pack["markets"], pack["languages"]), (["JP"], ["ja"]))
        self.assertIn("_about", pack)

    def test_series_overlay_is_merged_into_the_campaign(self):
        chd = self.root / "marketing/channels/acme"
        write(chd / "pack/series/kids.json", {
            "_about": "overlay", "known_words": ["T. rex"], "goal": "follow",
            "constraints": ["no competitor names", "owner: family audience"], "brand": {"say": {"T. rex": "tee rex"}}})
        mvf(self.root, "init", "ep1", "--product", "Acme", "--series", "kids")
        c = read(chd / "campaigns/ep1/campaign.json")
        self.assertEqual(c["series"], "kids")
        self.assertEqual(c["goal"], "follow")
        self.assertEqual(c["constraints"], ["no competitor names", "no prices on screen unless a fact is checked "
                                            "this week", "owner: family audience"])
        self.assertEqual(c["known_words"], ["T. rex"])
        self.assertEqual(c["brand"]["say"], {"T. rex": "tee rex"})
        self.assertEqual(c["brand"]["names"], ["Acme"])
        self.assertNotEqual(c.get("_about"), "overlay")  # the campaign keeps its own note
        self.assertTrue(c.get("_template"))  # the brief is still the owner's

        for bad in ("nope", "_example"):
            r = mvf(self.root, "init", "ep2", "--product", "Acme", "--series", bad, ok=False)
            self.assertNotEqual(r.returncode, 0)
            self.assertFalse((chd / "campaigns/ep2").exists())
        self.assertIn("have: kids", mvf(self.root, "init", "ep2", "--product", "Acme", "--series", "nope",
                                        ok=False).stderr)

    def test_merge_unions_lists_and_replaces_values(self):
        out = channel.merge({"a": [1, 2], "b": {"x": 1, "y": [1]}, "c": "old"},
                            {"a": [2, 3], "b": {"y": [2], "z": 3}, "c": "new", "d": 4})
        self.assertEqual(out, {"a": [1, 2, 3], "b": {"x": 1, "y": [1, 2], "z": 3}, "c": "new", "d": 4})


if __name__ == "__main__":
    unittest.main()
