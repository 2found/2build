"""Brand font files and a campaign frame rate reach the scaffolded project."""
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from mvflib import scaffold  # noqa: E402


class BrandFonts(unittest.TestCase):
    def test_font_with_file_keeps_family_name_and_gets_a_face(self):
        b = scaffold._brand({"brand": {"fonts": {"caption": {"family": "Be Vietnam Pro", "file": "fonts/BVP.ttf", "weight": 800},
                                                 "body": "Inter"}}})
        self.assertEqual(b["fonts"]["caption"], "Be Vietnam Pro")
        self.assertEqual(b["fonts"]["body"], "Inter")
        face = scaffold.font_faces({}, b)
        self.assertIn('font-family: "Be Vietnam Pro"', face)
        self.assertIn('url("assets/fonts/BVP.ttf")', face)
        self.assertIn("font-weight: 800", face)

    def test_plain_brand_has_no_font_files(self):
        self.assertEqual(scaffold._brand({})["font_files"], [])


if __name__ == "__main__":
    unittest.main()
