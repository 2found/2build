"""Material pack, sources and the M* lint (stdlib only, no network)."""
import os
import sys
import tempfile
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(TOOLS))
os.environ["MVF_TODAY"] = "2026-10-27"

from mvflib import material  # noqa: E402
from mvflib.lint import lint  # noqa: E402
from mvflib.sources import run  # noqa: E402
from test_mvf import CAMPAIGN, REFS, codes, script  # noqa: E402


def item(iid, **over):
    kind = iid.split(":")[1]
    it = {"id": iid, "kind": kind, "title": iid, "body": "", "signals": {}, "rights": "own", "links": [],
          "source": {"name": "test", "fetched_at": "2026-10-27"}}
    it.update(over)
    return it


REVIEW = item("store:review:r1", signals={"stars": 5, "helpful": 13}, body="Saved me an hour every morning.")
STILL = item("web:asset:repost-1", rights="unknown", source={"name": "web", "url": "https://e.x/a.jpg", "fetched_at": "2026-10-20"})
PROMO = item("web:asset:partner-poster", rights="official-promo", credit="Partner Co",
             source={"name": "web", "url": "https://e.x/p.jpg", "fetched_at": "2026-10-20"})
OLD = item("app:metric:users", signals={"users": 4809}, source={"name": "app.stats", "fetched_at": "2026-08-01"})
PACK = {i["id"]: i for i in (REVIEW, STILL, PROMO, OLD)}


def with_material(beat_material, **over):
    s = script(**over)
    s["beats"][1]["material"] = beat_material
    return s


class Items(unittest.TestCase):
    def test_valid_item(self):
        self.assertEqual(material.validate(REVIEW).fails, 0)

    def test_kind_must_match_id(self):
        self.assertIn("MAT", codes(material.validate({**REVIEW, "kind": "product"}), "FAIL"))

    def test_outside_asset_needs_url_and_promo_needs_credit(self):
        self.assertIn("MAT", codes(material.validate({**STILL, "source": {"name": "web", "fetched_at": "2026-10-20"}}), "FAIL"))
        self.assertIn("MAT", codes(material.validate({**PROMO, "credit": ""}), "FAIL"))

    def test_regather_keeps_richer_fields(self):
        rich = item("app:product:x", body="description", signals={"users30d": 5}, meta={"maker": "A"},
                    source={"name": "app.stats", "fetched_at": "2026-10-27"})
        thin = item("app:product:x", meta={"icon": "u"}, links=["store:review:r1"],
                    source={"name": "store.reviews", "fetched_at": "2026-10-27"})
        c = material.combine(rich, thin)
        self.assertEqual(c["body"], "description")
        self.assertEqual(c["signals"], {"users30d": 5})
        self.assertEqual(c["meta"], {"maker": "A", "icon": "u"})
        self.assertEqual(c["source"]["name"], "app.stats")

    def test_merge_rejects_invalid_and_counts(self):
        with tempfile.TemporaryDirectory() as d:
            added, updated, rep = material.merge(Path(d), [REVIEW, {"id": "bad"}], {"source": "t"})
            self.assertEqual((added, updated, rep.fails > 0), (1, 0, True))
            added, updated, _ = material.merge(Path(d), [REVIEW], {"source": "t"})
            self.assertEqual((added, updated), (0, 1))


class MaterialLint(unittest.TestCase):
    def refs(self, pack=PACK):
        return {**REFS, "material": pack}

    def test_pack_but_nothing_cited_fails(self):
        self.assertIn("M0", codes(lint(script(), CAMPAIGN, self.refs()), "FAIL"))

    def test_override_is_info(self):
        rep = lint(script(material_override="product demo, facts carry it"), CAMPAIGN, self.refs())
        self.assertNotIn("M0", codes(rep, "FAIL"))
        self.assertIn("M0", codes(rep, "INFO"))

    def test_no_pack_only_warns_and_old_callers_are_untouched(self):
        self.assertIn("M0", codes(lint(script(), CAMPAIGN, self.refs({})), "WARN"))
        self.assertNotIn("M0", codes(lint(script(), CAMPAIGN, REFS)))

    def test_unknown_id_fails(self):
        self.assertIn("M1", codes(lint(with_material(["store:review:nope"]), CAMPAIGN, self.refs()), "FAIL"))

    def test_recipe_needs_a_measured_review(self):
        ok = lint(with_material([REVIEW["id"]], recipe="social-proof"), CAMPAIGN, self.refs())
        self.assertNotIn("M3", codes(ok, "FAIL"))
        bad = lint(with_material([PROMO["id"]], recipe="social-proof"), CAMPAIGN, self.refs())
        self.assertIn("M3", codes(bad, "FAIL"))
        self.assertIn("M3", codes(lint(with_material([REVIEW["id"]], recipe="nope"), CAMPAIGN, self.refs()), "FAIL"))

    def test_unknown_rights_cannot_be_shown_promo_can(self):
        self.assertIn("M4", codes(lint(with_material([STILL["id"]]), CAMPAIGN, self.refs()), "FAIL"))
        rep = lint(with_material([PROMO["id"]]), CAMPAIGN, self.refs())
        self.assertNotIn("M4", codes(rep, "FAIL"))
        self.assertIn("M4", codes(rep, "INFO"))

    def test_a_free_licence_does_not_clear_a_real_persons_picture(self):
        portrait = {**STILL, "id": "web:asset:wd/Q1/portrait", "rights": "licensed", "credit": "x / CC BY 2.5",
                    "meta": {"role": "portrait", "licence": "CC BY 2.5"}}
        rep = lint(with_material([portrait["id"]]), CAMPAIGN, self.refs({**PACK, portrait["id"]: portrait}))
        self.assertIn("M4", codes(rep, "FAIL"))

    def test_claim_stands_on_a_fresh_signal_not_a_stale_one(self):
        s = with_material([REVIEW["id"]])
        s["beats"][1]["vo"] = "13 people found this review helpful."
        self.assertNotIn("C1", codes(lint(s, CAMPAIGN, self.refs()), "FAIL"))
        s["beats"][1]["material"] = [OLD["id"]]
        self.assertIn("M2", codes(lint(s, CAMPAIGN, self.refs()), "FAIL"))


class ManualSource(unittest.TestCase):
    def test_params_are_checked_and_file_read(self):
        with self.assertRaises(SystemExit):
            run("manual", {})
        with self.assertRaises(SystemExit):
            run("manual", {"file": "x", "bogus": "1"})
        with tempfile.TemporaryDirectory() as d:
            f = Path(d) / "items.json"
            f.write_text(__import__("json").dumps({"items": [REVIEW]}))
            self.assertEqual(run("manual", {"file": str(f)}, Path(d)), [REVIEW])


if __name__ == "__main__":
    unittest.main()
