"""The library (mvflib/library.py): what one video leaves for the next, so the campaign folders can be deleted."""
import json
import os
import shutil
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import mvf as mvf_tool  # noqa: E402
from mvflib import fresh, library  # noqa: E402
from test_extensions import mvf, repo  # noqa: E402

os.environ["MVF_TODAY"] = "2026-10-27"


def write(p: Path, data) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(json.dumps(data, ensure_ascii=False) if not isinstance(data, str) else data)


class Library(unittest.TestCase):
    def setUp(self):
        self.d, self.root = repo()
        self.camps = self.root / "marketing/channels/acme"
        mvf(self.root, "init", "ep1", "--product", "Acme", "--subject", "Kiếm Lai")
        self.cd = self.camps / "ep1"
        c = json.loads((self.cd / "campaign.json").read_text())
        c.pop("_template")
        write(self.cd / "campaign.json", {**c, "markets": ["VN"], "languages": ["vi"], "known_words": ["Acme"]})
        write(self.cd / "01-product/facts.json", {"facts": [{"id": "F1", "claim": "free", "checked": "2026-10-20"}]})
        write(self.cd / "01-product/captures/items.json", [{"id": "acme:asset:a"}])
        (self.cd / "01-product/captures/a.mp4").write_bytes(b"video-a")
        write(self.cd / "02-audience/segments.json", {"segments": [{"id": "A1"}]})
        (self.cd / "brand/logo.png").write_bytes(b"png")
        write(self.cd / "00-material/index.json", {"items": {"acme:passage:1": {"id": "acme:passage:1"}}})
        write(self.cd / "04-strategy/angles.json", {"angles": [{"id": "G1", "promise": "the old promise"}]})
        write(self.cd / "05-scripts/s01.json", {"id": "s01", "spine": {"payoff": "the old payoff"},
                                                "beats": [{"id": "B01", "vo": "một câu nói đã nói rồi trong video cũ"}]})
        (self.cd / "08-delivery/s01").mkdir(parents=True)
        (self.cd / "07-video/renders").mkdir(parents=True)
        (self.cd / "07-video/renders/master.mp4").write_bytes(b"x" * 1000)
        write(self.cd / "07-video/s01/src/build.py", "def scene(): pass\n")

    def tearDown(self):
        self.d.cleanup()

    def test_the_next_campaign_starts_from_what_the_last_one_saved(self):
        self.assertIn("saved ep1", mvf(self.cd, "library", "save").stdout)
        lib = library.root(self.cd)
        self.assertEqual(lib, self.root.resolve() / ".babysit/marketing-video/library")
        self.assertTrue((lib / "products/acme/01-product/captures/a.mp4").is_file())
        self.assertTrue((lib / "subjects/kiem-lai/00-material/index.json").is_file())
        self.assertFalse(list(lib.rglob("master.mp4")))  # renders are not kept
        self.assertTrue((lib / "code/ep1/s01/build.py").is_file())

        r = mvf(self.root, "init", "ep2", "--product", "Acme", "--subject", "kiem-lai")
        self.assertIn("product Acme (last saved from ep1", r.stdout)
        cd2 = self.camps / "ep2"
        c2 = json.loads((cd2 / "campaign.json").read_text())
        self.assertEqual((c2["markets"], c2["subject"]), (["VN"], "kiem-lai"))
        self.assertTrue(c2.get("_template"))  # the brief is still the owner's to fill
        self.assertEqual(json.loads((cd2 / "01-product/facts.json").read_text())["facts"][0]["claim"], "free")
        self.assertEqual((cd2 / "01-product/captures/a.mp4").read_bytes(), b"video-a")
        self.assertIn("acme:passage:1", json.loads((cd2 / "00-material/index.json").read_text())["items"])
        (cd2 / "01-product/captures/a.mp4").write_bytes(b"edited")  # a copy, not the library's file
        self.assertEqual((lib / "products/acme/01-product/captures/a.mp4").read_bytes(), b"video-a")

        # a later save merges captures and material by id; facts are the newest campaign's
        write(cd2 / "01-product/captures/items.json", [{"id": "acme:asset:b"}])
        write(cd2 / "00-material/index.json", {"items": {"acme:passage:2": {"id": "acme:passage:2"}}})
        write(cd2 / "01-product/facts.json", {"facts": [{"id": "F1", "claim": "now paid", "checked": "2026-10-27"}]})
        library.save(cd2)
        caps = json.loads((lib / "products/acme/01-product/captures/items.json").read_text())
        self.assertEqual([x["id"] for x in caps], ["acme:asset:a", "acme:asset:b"])
        items = json.loads((lib / "subjects/kiem-lai/00-material/index.json").read_text())["items"]
        self.assertEqual(sorted(items), ["acme:passage:1", "acme:passage:2"])
        facts = json.loads((lib / "products/acme/01-product/facts.json").read_text())["facts"]
        self.assertEqual(facts[0]["claim"], "now paid")

    def test_history_outlives_the_campaign_folder(self):
        mvf(self.root, "init", "ep2", "--product", "Acme")
        cd2 = self.camps / "ep2"
        write(cd2 / "05-scripts/s01.json", {"id": "s01", "beats": [
            {"id": "B01", "vo": "một câu nói đã nói rồi trong video cũ"}]})
        mvf(self.cd, "library", "save")
        r = mvf(self.root, "library", "check", str(self.camps), ok=False)
        self.assertIn("ep2: never saved", r.stdout)
        self.assertIn("NOT SAFE", r.stdout)
        mvf(cd2, "library", "save")
        self.assertIn("SAFE: nothing", mvf(self.root, "library", "check", str(self.camps)).stdout)

        shutil.rmtree(self.cd)
        # lint RP1 still hears the deleted campaign's lines
        self.assertIn(("ep1/s01", "một câu nói đã nói rồi trong video cũ"), mvf_tool.past_scripts(cd2))
        past = fresh.past_videos(cd2)
        self.assertIn("ep1/G1 (angle): the old promise []", past)
        self.assertIn("ep1/s01 (made): the old payoff", past)
        out = mvf(cd2, "library").stdout
        self.assertIn("product acme: 1 facts (checked 2026-10-20 … 2026-10-20), 1 captures", out)
        self.assertIn("- ep1/s01 (made): the old payoff", out)
        self.assertIn("production code in", out)
        self.assertNotIn("ep2/s01", out)  # not this campaign's own

    def test_a_resave_keeps_the_subject_it_was_saved_under(self):
        c = json.loads((self.cd / "campaign.json").read_text())
        c.pop("subject")  # made before --subject
        write(self.cd / "campaign.json", c)
        mvf(self.cd, "library", "save", "--subject", "Kiếm Lai")
        library.save(self.cd)  # the final export saves again without --subject
        lib = library.root(self.cd)
        self.assertEqual(json.loads((lib / "saved.json").read_text())["ep1"]["subject"], "kiem-lai")
        self.assertFalse((lib / "subjects/ep1").exists())

    def test_a_template_never_overwrites_a_kept_pack(self):
        mvf(self.cd, "library", "save")
        mvf(self.root, "init", "ep3", "--product", "Acme")
        cd3 = self.camps / "ep3"
        c = json.loads((cd3 / "campaign.json").read_text())
        write(cd3 / "campaign.json", {**c, "markets": ["US"]})  # still the template brief
        write(cd3 / "01-product/facts.json", {"_template": True, "facts": [{"id": "F1", "claim": ""}]})
        library.save(cd3)  # nothing of a template replaces ep1's pack
        lib = library.root(self.cd)
        self.assertEqual(json.loads((lib / "products/acme/campaign.json").read_text())["markets"], ["VN"])
        facts = json.loads((lib / "products/acme/01-product/facts.json").read_text())["facts"]
        self.assertEqual(facts[0]["claim"], "free")


if __name__ == "__main__":
    unittest.main()
