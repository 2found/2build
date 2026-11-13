"""YouTube platforms (lint, copy files, export routing) and generated art (sizes, gate,
prompt checks, draw with an extension engine, approve, disclosure). Stdlib only; the
draw/approve tests need ffmpeg and skip without it."""
import copy
import json
import os
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest
from unittest import mock
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(TOOLS))
os.environ["MVF_TODAY"] = "2026-10-27"

from mvflib import art, extensions  # noqa: E402

os.environ["MVF_UPSCALER"] = "lanczos"   # the learned upscaler is a heavy local model; its own test opts in
from mvflib.export import copy_files  # noqa: E402
from mvflib.lint import lint  # noqa: E402
from test_extensions import campaign, mvf, repo  # noqa: E402
from test_mvf import CAMPAIGN, REFS, codes, script  # noqa: E402

FFMPEG = shutil.which("ffmpeg") and shutil.which("ffprobe")
YT = {"title": "Your inbox is lying to you", "description": "One filter sorts it. acme tips\n\n#inbox #email",
      "tags": ["inbox", "email"]}


def yt_script(**over):
    s = script()
    s["platforms"]["youtube"] = copy.deepcopy(YT)
    s["platforms"].update(over)
    return s


class YouTubeLint(unittest.TestCase):
    def setUp(self):
        self.c = {**CAMPAIGN, "platforms": ["tiktok", "x", "youtube"]}

    def test_clean_shorts_copy_passes(self):
        self.assertEqual([i for i in lint(yt_script(), self.c, REFS).items if i["code"].startswith("P")
                          and i["level"] == "FAIL"], [])

    def test_title_required_and_hard_limit(self):
        s = yt_script()
        s["platforms"]["youtube"]["title"] = ""
        self.assertIn("P1", codes(lint(s, self.c, REFS), "FAIL"))
        s["platforms"]["youtube"]["title"] = "x" * 101
        self.assertIn("P1", codes(lint(s, self.c, REFS), "FAIL"))
        s["platforms"]["youtube"]["title"] = "inbox " * 13  # 78 chars: legal, but cut in the feed
        self.assertIn("P1", codes(lint(s, self.c, REFS), "WARN"))

    def test_hashtag_limits(self):
        s = yt_script()
        s["platforms"]["youtube"]["description"] = "acme tips " + " ".join(f"#t{i}" for i in range(5))
        self.assertIn("P2", codes(lint(s, self.c, REFS), "WARN"))
        s["platforms"]["youtube"]["description"] = "acme tips " + " ".join(f"#t{i}" for i in range(61))
        self.assertIn("P2", codes(lint(s, self.c, REFS), "FAIL"))

    def test_long_form_chapters(self):
        c = {**self.c, "platforms": ["youtube-long"]}
        s = script()
        s["platforms"] = {"youtube-long": {**YT, "canvas": "16x9", "thumbnail_text": "inbox lies",
                                           "description": "acme tips\n0:00 Intro\n0:15 Filter\n0:30 Try it"}}
        self.assertNotIn("P5", codes(lint(s, c, REFS), "FAIL"))
        s["platforms"]["youtube-long"]["description"] = "acme tips\n0:03 Intro\n0:20 Filter\n0:40 Try it"
        self.assertIn("P5", codes(lint(s, c, REFS), "FAIL"))  # must start at 0:00
        s["platforms"]["youtube-long"]["description"] = "acme tips\n0:00 Intro\n0:15 Filter"
        self.assertIn("P5", codes(lint(s, c, REFS), "FAIL"))  # fewer than 3

    def test_copy_files_per_platform(self):
        s = yt_script()
        self.assertEqual(set(copy_files(s, "youtube")), {"title.txt", "description.txt", "tags.txt"})
        self.assertEqual(copy_files(s, "youtube")["tags.txt"], "inbox, email\n")
        self.assertEqual(set(copy_files(s, "tiktok")), {"caption.txt"})
        self.assertEqual(set(copy_files(s, "x")), {"post.txt"})

    def test_beat_art_must_exist_and_be_shown(self):
        s = script()
        s["beats"][1]["art"] = ["P1"]
        refs = {**REFS, "art": {"K0": {"id": "K0", "use": "reference"}}}
        self.assertIn("A1", codes(lint(s, CAMPAIGN, refs), "FAIL"))
        s["beats"][1]["art"] = ["K0"]
        self.assertIn("A1", codes(lint(s, CAMPAIGN, refs), "FAIL"))
        refs["art"]["P1"] = {"id": "P1", "use": "background"}
        s["beats"][1]["art"] = ["P1"]
        self.assertNotIn("A1", codes(lint(s, CAMPAIGN, refs)))


class ArtSizes(unittest.TestCase):
    def test_qwen_draw_sizes(self):
        q = extensions.Image("q", command=["x"])
        self.assertEqual(art.draw_size(1080 / 1920, q), (384, 672))
        self.assertEqual(art.draw_size(1920 / 1080, q), (672, 384))
        self.assertEqual(art.draw_size(1.0, q), (512, 512))

    def test_a_job_is_drawn_at_the_size_it_is_shown_within_what_the_machine_draws(self):
        big = extensions.Image("q", command=["x"], max_pixels=1088 * 1920, sides=(256, 1920))
        self.assertEqual(art.job_draw_size({"use": "plate"}, big), (1088, 1920))
        self.assertEqual(art.job_draw_size({"use": "background"}, big), (544, 960))   # passes soft: half the frame
        self.assertEqual(art.job_draw_size({"use": "sticker", "display": "420x420"}, big), (448, 448))
        self.assertEqual(art.job_draw_size({"use": "reference"}, big), (512, 512))
        self.assertEqual(art.job_draw_size({"use": "plate"}, extensions.Image("q", command=["x"])), (384, 672))
        self.assertEqual(art.job_draw_size({"use": "background", "draw": "768x1344"}, big), (768, 1344))  # the agent's call
        with self.assertRaises(SystemExit):
            art.job_draw_size({"id": "P", "use": "plate", "draw": "1088x1920"}, extensions.Image("q", command=["x"]))

    def test_qwen_limits_come_from_generate_image_and_fall_back(self):
        art.qwen_limits.cache_clear()
        self.assertEqual(art.qwen_limits("/no/such/generate-image"), {})
        ok = mock.Mock(stdout='{"max_pixels": 2088960, "sides": [256, 1920], "step": 32, "memory_gib": 16}\n')
        with mock.patch.object(art.subprocess, "run", return_value=ok):
            self.assertEqual(art.qwen_limits("gi"), {"max_pixels": 2088960, "sides": (256, 1920), "step": 32})
        art.qwen_limits.cache_clear()

    def test_gate_is_measured_at_display_size(self):
        plate = {"use": "plate"}
        f = art.upscale((384, 672), art.display_size(plate))
        self.assertEqual(f, 2.86)
        self.assertEqual(art.verdict(plate, f), "FAIL")
        self.assertEqual(art.verdict({"use": "background"}, f), "PASS")
        inset = {"use": "inset", "display": "600x600"}
        self.assertEqual(art.verdict(inset, art.upscale((512, 512), art.display_size(inset))), "PASS")
        self.assertIsNone(art.display_size({"use": "reference"}))


class Upscaler(unittest.TestCase):
    def test_esrgan_widens_the_gate(self):
        job = {"use": "plate"}
        self.assertEqual(art.verdict(job, 2.8), "FAIL")
        self.assertEqual(art.verdict(job, 2.8, "esrgan"), "PASS")
        self.assertEqual(art.verdict(job, 4.5, "esrgan"), "FAIL")

    def test_photoreal_defaults_to_lanczos_and_missing_model_falls_back(self):
        env = os.environ.pop("MVF_UPSCALER")
        try:
            self.assertEqual(art.pick_upscaler({"look": "photoreal"}, None), "lanczos")
            ready, art.esrgan_ready = art.esrgan_ready, lambda: False
            try:
                self.assertEqual(art.pick_upscaler({}, "esrgan"), "lanczos")
            finally:
                art.esrgan_ready = ready
        finally:
            os.environ["MVF_UPSCALER"] = env


class ArtChecks(unittest.TestCase):
    def jobs(self, cd, *jobs, style="flat ink illustration, no text"):
        (cd / art.ART).mkdir(parents=True, exist_ok=True)
        (cd / art.ART / "art.json").write_text(json.dumps({"style": style, "jobs": list(jobs)}))

    def test_prompt_rules(self):
        with tempfile.TemporaryDirectory() as d:
            cd = Path(d)
            self.jobs(cd, {"id": "P1", "use": "background", "prompt": "a dashboard of the Acme app"},
                      {"id": "P2", "use": "plate", "prompt": "Jane Doe smiling at a desk"},
                      {"id": "P3", "use": "plate", "prompt": "a shop sign that says \"OPEN\""},
                      {"id": "S1", "use": "sticker", "prompt": "a red envelope"},
                      {"id": "P4", "use": "plate", "prompt": "a calm desk at dawn", "ref": "K9"})
            r = art.check(cd, ["Jane Doe"])
            fails = {(i["code"], i["where"]) for i in r.items if i["level"] == "FAIL"}
            self.assertIn(("A2", "art.P1"), fails)
            self.assertIn(("A3", "art.P2"), fails)
            self.assertIn(("A0", "art.S1"), fails)  # sticker without display
            self.assertIn(("A6", "art.P4"), fails)  # unknown ref
            warns = {(i["code"], i["where"]) for i in r.items if i["level"] == "WARN"}
            self.assertIn(("A4", "art.P3"), warns)
            self.assertIn(("A5", "art.S1"), warns)

    def test_a_cut_out_wants_its_own_style(self):
        with tempfile.TemporaryDirectory() as d:
            cd = Path(d)
            self.jobs(cd, {"id": "S1", "use": "sticker", "display": "400x400", "transparent": True, "prompt": "a pebble"},
                      {"id": "S2", "use": "sticker", "display": "400x400", "transparent": True, "prompt": "a pebble",
                       "style": "same look, flat grey background"}, style="a village street at dusk")
            warns = {i["where"] for i in art.check(cd).items if i["code"] == "A9"}
            self.assertEqual(warns, {"art.S1"})

    def test_a_preset_writes_both_styles_and_its_sticker_mode(self):
        with tempfile.TemporaryDirectory() as d:
            cd = Path(d)
            (cd / art.ART).mkdir(parents=True)
            (cd / art.ART / "art.json").write_text(json.dumps({"preset": "stickman", "jobs": [
                {"id": "P1", "use": "plate", "prompt": "an empty doorstep"},
                {"id": "S1", "use": "sticker", "display": "400x400", "prompt": "a pebble"}]}))
            r = art.check(cd)
            self.assertEqual([i["code"] for i in r.items if i["code"] in {"A5", "A9", "A0"}], [])
            dd = art.load_jobs(cd)
            plate, stk = dd["jobs"]
            cut = art.STYLES["stickman"]["art"]["cutout_style"]
            self.assertIn(cut, art.full_prompt(stk, dd["style"], None, dd["cutout_style"]))
            self.assertNotIn(cut, art.full_prompt(plate, dd["style"], None, dd["cutout_style"]))
            own = {**stk, "style": "my own line"}
            self.assertNotIn(cut, art.full_prompt(own, dd["style"], None, dd["cutout_style"]))  # the job's own style wins
            (cd / art.ART / "art.json").write_text(json.dumps({"preset": "ghibli", "style": "x", "jobs": []}))
            self.assertEqual([i["code"] for i in art.check(cd).items if i["level"] == "FAIL"], ["A0"])

    def test_unicode_ui_word_and_clean_job(self):
        with tempfile.TemporaryDirectory() as d:
            cd = Path(d)
            self.jobs(cd, {"id": "P1", "use": "background", "prompt": "giao diện ứng dụng đọc truyện"},
                      {"id": "P2", "use": "background", "prompt": "rain on a night window, warm lamp"})
            r = art.check(cd)
            self.assertEqual([i["where"] for i in r.items if i["level"] == "FAIL"], ["art.P1"])

    def test_more_than_two_generated_full_frames_is_a_slideshow(self):
        with tempfile.TemporaryDirectory() as d:
            cd = Path(d)
            (cd / "cover.png").write_bytes(b"x")
            two = [{"id": "P1", "use": "plate", "prompt": "a mountain gate at dawn"},
                   {"prompt": "mist over a lake", "id": "P2"},  # no use: a plate
                   {"id": "C1", "use": "plate", "image": "cover.png"},  # the campaign's own picture is not generated
                   {"id": "S1", "use": "sticker", "display": "400x400", "transparent": True, "prompt": "a sword"}]
            self.jobs(cd, *two)
            self.assertNotIn("A12", [i["code"] for i in art.check(cd).items])
            self.jobs(cd, *two, {"id": "B1", "use": "background", "prompt": "a night sky"})
            self.assertEqual([(i["code"], i["level"]) for i in art.check(cd).items if i["code"] == "A12"], [("A12", "WARN")])


ENGINE = textwrap.dedent("""
    import subprocess
    from mvflib.extensions import Image

    def run(req, out):
        w, h = req["size"].split("x")
        open(str(out) + ".prompt", "w").write(req["prompt"] + "|" + str(req["ref"]))
        subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i",
                        f"testsrc2=s={w}x{h}:d=1", "-frames:v", "1", str(out)], check=True)
        return {"seconds": 0.1}

    IMAGES = {"fake": Image("test pattern", run=run, heavy=False),
              "dear": Image("costs money", run=run, paid=True, heavy=False)}
""")


@unittest.skipUnless(FFMPEG, "needs ffmpeg")
class ArtFlow(unittest.TestCase):
    def test_draw_approve_scaffold_disclose(self):
        d, root = repo()
        with d:
            cd = campaign(root, voice={"mode": "none"})
            (cd / "extensions").mkdir(exist_ok=True)
            (cd / "extensions/fake.py").write_text(ENGINE)
            (cd / art.ART).mkdir(parents=True, exist_ok=True)
            (cd / art.ART / "art.json").write_text(json.dumps({"style": "flat ink, no text", "engine": "fake", "jobs": [
                {"id": "K0", "use": "reference", "prompt": "a fox character sheet"},
                {"id": "P1", "use": "background", "prompt": "a fox reading at night", "ref": "K0"},
                {"id": "P2", "use": "plate", "prompt": "a photo of a desk", "look": "photoreal"}]}))
            out = mvf(cd, "art", "draw").stdout
            self.assertIn("K0-s1.png", out)
            drafts = cd / art.ART / "draft"
            self.assertEqual(art.image_size(drafts / "P1-s1.png"), (384, 672))
            sent = (drafts / "P1-s1.png.prompt").read_text()
            self.assertIn("flat ink, no text", sent)       # style appended
            self.assertIn("K0-s1.png", sent)               # reference resolved to K0's draft
            self.assertEqual(json.loads((drafts / "P1-s1.json").read_text())["engine"], "fake")
            self.assertIn("exists", mvf(cd, "art", "draw", "P1").stdout)  # never overwritten

            r = mvf(cd, "art", "draw", "P1", "--engine", "dear", "--seed", "2", ok=False)
            self.assertNotEqual(r.returncode, 0)
            self.assertIn("--spend", r.stderr)

            self.assertIn("PASS P1", mvf(cd, "art", "approve", "P1").stdout)
            self.assertEqual(art.image_size(cd / art.ART / "P1.png"), (1080, 1920))
            r = mvf(cd, "art", "approve", "P2", ok=False)  # a 2.86x full-frame plate
            self.assertIn("too soft", r.stderr)
            self.assertIn("FORCED", mvf(cd, "art", "approve", "P2", "--force").stdout)

            s = json.loads((cd / "05-scripts/s01-test.json").read_text())
            s["beats"][1]["art"] = ["P2"]
            (cd / "05-scripts/s01-test.json").write_text(json.dumps(s))
            mvf(cd, "scaffold", "s01-test.json", "--take", "none")
            proj = cd / "07-video/s01-test/9x16"
            self.assertTrue((proj / "assets/art/P1.png").is_file())
            self.assertFalse((proj / "assets/art/K0.png").is_file())  # a reference is never approved-for-screen
            ai = art.disclosure(cd, proj, s)
            self.assertEqual((ai["photoreal"], ai["youtube"], ai["tiktok"]), (["P2"], "ON", "ON"))
            s["beats"][1]["art"] = ["P1"]
            ai = art.disclosure(cd, proj, s)
            self.assertEqual((ai["youtube"], ai["tiktok"]), ("off", "recommended"))
            self.assertIn("jobs 3 · drafted 3 · approved 2", mvf(cd, "status").stdout)


FAKE_RMBG = """#!/usr/bin/env python3
import subprocess, sys
src, out = sys.argv[1], sys.argv[sys.argv.index("-o") + 1]
# a centred opaque square on a cleared ground, like a matte of one subject
subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", src, "-vf",
                "format=rgba,geq=r='r(X,Y)':g='g(X,Y)':b='b(X,Y)':a='if(between(X,W/4,3*W/4)*between(Y,H/4,3*H/4),255,0)'",
                "-frames:v", "1", out], check=True)
print('{"model": "rmbg-1.4"}')
"""


@unittest.skipUnless(FFMPEG, "needs ffmpeg")
class ImageJob(unittest.TestCase):
    """A picture the campaign already has, cut out with RMBG: drawn by nobody, so no AI label; licence flagged."""
    def test_cut_approve_disclose_and_licence(self):
        d, root = repo()
        with d:
            cd = campaign(root, voice={"mode": "none"})
            (cd / "brand").mkdir(exist_ok=True)
            subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=s=800x800:d=1",
                            "-frames:v", "1", str(cd / "brand/founder.jpg")], check=True)
            fake = root / "remove-bg"
            fake.write_text(FAKE_RMBG)
            fake.chmod(0o755)
            (cd / art.ART).mkdir(parents=True, exist_ok=True)
            (cd / art.ART / "art.json").write_text(json.dumps({"jobs": [
                {"id": "C1", "use": "sticker", "display": "500x500", "transparent": True, "image": "brand/founder.jpg"},
                {"id": "C2", "use": "inset", "display": "400x400", "image": "brand/missing.jpg"}]}))
            r = mvf(cd, "art", "check", ok=False)
            self.assertIn("art.C2", r.stdout)                         # a missing picture FAILs
            self.assertNotIn("prompt is empty", r.stdout)             # no prompt needed
            env = {"MVF_REMOVE_BG": str(fake)}
            out = mvf(cd, "art", "draw", "C1", env=env, ok=False).stdout  # exits 1: C2's FAIL
            self.assertIn("75% cleared", out)
            self.assertNotIn("A10", out)                              # a real picture needs no cast design
            rec = json.loads((cd / art.ART / "draft/C1-s0.json").read_text())
            self.assertEqual((rec["engine"], rec["cutout"]), ("image", "rmbg-1.4"))
            mvf(cd, "art", "approve", "C1")
            a = art.approved(cd, "C1")
            self.assertEqual((a["generated"], a["cutout"]), (False, "rmbg-1.4"))
            s = json.loads((cd / "05-scripts/s01-test.json").read_text())
            s["beats"][1]["art"] = ["C1"]
            proj = cd / "07-video/s01-test/9x16"
            proj.mkdir(parents=True, exist_ok=True)
            self.assertEqual(art.disclosure(cd, proj, s)["art"], [])  # not generated: nothing to label
            self.assertEqual(art.rmbg_shown(cd, proj, s), ["C1"])     # but the matte's licence applies


@unittest.skipUnless(FFMPEG, "needs ffmpeg")
class Cutout(unittest.TestCase):
    def test_flood_fill_keeps_the_subjects_own_key_colour(self):
        with tempfile.TemporaryDirectory() as d:
            src, out = Path(d) / "s.png", Path(d) / "o.png"
            # an opaque 'transparent' render: green ground, a white subject with a green patch inside it
            subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=0x22aa22:s=64x64",
                            "-vf", "drawbox=x=16:y=16:w=32:h=32:c=white:t=fill,drawbox=x=28:y=28:w=8:h=8:c=0x22aa22:t=fill,format=rgba",
                            "-frames:v", "1", str(src)], check=True)
            self.assertLess(art.clear_share(src), art.MIN_CLEAR)
            self.assertTrue(art.cutout(src, out).startswith("22aa"))  # yuv round trip
            b, w, _ = art._raw(out, "rgba")
            alpha = lambda x, y: b[(y * w + x) * 4 + 3]
            self.assertEqual(alpha(2, 2), 0)       # ground cleared
            self.assertEqual(alpha(20, 20), 255)   # subject kept
            self.assertEqual(alpha(31, 31), 255)   # the subject's own green survives (not connected to the border)
            self.assertAlmostEqual(art.clear_share(out), 1 - 32 * 32 / 64 / 64, places=2)


@unittest.skipUnless(FFMPEG, "needs ffmpeg")
class ExportRouting(unittest.TestCase):
    def test_project_platforms_follow_canvas(self):
        d, root = repo()
        with d:
            cd = campaign(root, voice={"mode": "none"})
            s = yt_script(**{"youtube-long": {**YT, "canvas": "16x9"}})
            (cd / "05-scripts/s01-test.json").write_text(json.dumps(s))
            out = mvf(cd, "scaffold", "s01-test.json", "--take", "none").stdout
            self.assertNotIn("copied", out)  # a 16x9 scene is never laid into a 9x16 project
            sys.path.insert(0, str(TOOLS))
            import mvf as cli
            v = cli.project_platforms(cd, cd / "07-video/s01-test/9x16")
            h = cli.project_platforms(cd, cd / "07-video/s01-test/16x9")
            self.assertEqual(sorted(v), ["tiktok", "x", "youtube"])
            self.assertEqual(h, ["youtube-long"])


if __name__ == "__main__":
    unittest.main()
