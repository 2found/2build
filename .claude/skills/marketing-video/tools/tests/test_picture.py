"""Measured picture/sound checks on a render (mvflib/picture.py), the cold viewer (mvflib/viewer.py), the review
score, the listen plan and the ≤1 MP review sheets. Clips are synthetic and tiny; skipped without ffmpeg."""
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mvflib import picture, viewer  # noqa: E402
from mvflib.common import SHEET_PX, Report, tile_width  # noqa: E402
from mvflib.export import listen_plan  # noqa: E402

FF = shutil.which("ffmpeg") and shutil.which("ffprobe")
D = 8
TL = {"duration": D, "beats": [{"id": "B01", "start": 0.0, "end": 1.0, "role": "hook"},
                               {"id": "B02", "start": 1.0, "end": 4.5, "role": "body"},
                               {"id": "B03", "start": 4.5, "end": 7.0, "role": "payoff"},
                               {"id": "B04", "start": 7.0, "end": 8.0, "role": "cta"}]}


def codes(rep, level="WARN"):
    return sorted({i["code"] for i in rep.items if i["level"] == level})


@unittest.skipUnless(FF, "needs ffmpeg")
class Picture(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        S = cls.d = Path(cls.tmp.name)
        png = S / "still.png"
        ff = ["ffmpeg", "-hide_banner", "-loglevel", "error", "-y"]
        subprocess.run([*ff, "-f", "lavfi", "-i", "testsrc2=s=180x320:r=1:d=1", "-frames:v", "1", str(png)], check=True)
        still = ["-loop", "1", "-framerate", "30", "-t", str(D), "-i", str(png)]
        tone = ["-f", "lavfi", "-i", f"sine=f=300:d={D}:sample_rate=48000"]

        def mk(name, src, vf=None, af=None, extra=()):
            cmd = [*ff, *src, *tone, *extra, "-shortest"]
            if vf:
                cmd += ["-vf", vf]
            if af:
                cmd += ["-af", af]
            subprocess.run([*cmd, "-pix_fmt", "yuv420p", "-c:v", "libx264", str(S / f"{name}.mp4")], check=True)

        mk("still", still)
        mk("pop", still, vf="drawbox=x=0:y=0:w=iw:h=ih:color=white:t=fill:enable='eq(n,95)'")
        mk("push", still, vf="scale=360:640,zoompan=z='1+0.0015*on':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=1:s=180x320:fps=30")
        mk("moving", ["-f", "lavfi", "-i", f"testsrc2=s=180x320:r=30:d={D}"])
        mk("empty", ["-f", "lavfi", "-i", f"color=c=0x2b3050:s=180x320:r=30:d={D}"],
           vf="drawbox=x=70:y=200:w=40:h=60:color=white:t=fill")
        mk("loud", ["-f", "lavfi", "-i", f"testsrc2=s=180x320:r=30:d={D}"], af="volume='if(between(t,1.2,4.3),4,1)':eval=frame")

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def check(self, name):
        return picture.check(self.d / f"{name}.mp4", TL)

    def test_a_still_open_and_beats_where_nothing_acts(self):
        got = self.check("still")
        self.assertIn("PX1", codes(got))
        self.assertIn("PX2", codes(got))
        self.assertIn("PX0", codes(got, "INFO"))

    def test_a_moving_picture_is_clean(self):
        got = codes(self.check("moving"))
        for c in ("PX1", "PX2", "PX4", "PX5"):
            self.assertNotIn(c, got)

    def test_a_camera_push_alone_is_not_action(self):
        got = codes(self.check("push"))
        self.assertIn("PX2", got)
        self.assertNotIn("PX4", got)

    def test_one_flash_frame(self):
        pops = [i for i in self.check("pop").items if i["code"] == "PX4"]
        self.assertEqual(len(pops), 1)
        self.assertIn("3.17s", pops[0]["msg"])

    def test_the_same_picture_far_apart(self):
        self.assertTrue([i for i in self.check("still").items if i["code"] == "PX3"])

    def test_a_beat_louder_than_the_rest(self):
        px5 = [i for i in self.check("loud").items if i["code"] == "PX5"]
        self.assertEqual({i["where"] for i in px5}, {"loud.mp4.B02"})
        self.assertTrue([i for i in px5 if "against the other beats" in i["msg"]])

    def test_a_small_subject_in_an_empty_frame(self):
        self.assertIn("PX6", codes(self.check("empty")))
        self.assertNotIn("PX6", codes(self.check("moving")))

    def test_an_unreadable_file_is_a_warn(self):
        (self.d / "junk.mp4").write_text("not a video")
        self.assertEqual(codes(self.check("junk")), ["PX0"])


def project(root: Path, master: Path | None) -> tuple[Path, Path]:
    cd = root / "camp"
    proj = cd / "07-video/s01/9x16"
    (proj / "renders").mkdir(parents=True)
    (cd / "05-scripts").mkdir()
    (cd / "campaign.json").write_text(json.dumps({"product": "Acme", "goal": "awareness"}))
    (cd / "05-scripts/s01.json").write_text(json.dumps({"id": "s01", "value": "why the third fold holds",
                                                        "platforms": {"tiktok": {}}, "beats": []}))
    (proj / "mvf.json").write_text(json.dumps({"script": "05-scripts/s01.json", "canvas": "9x16", "hook": "h1"}))
    tl = {**TL, "beats": [{**b, "vo_start": b["start"] + 0.1, "vo_end": b["end"] - 0.2,
                           "words": [{"w": f"w{b['id']}"}]} for b in TL["beats"]]}
    (proj / "timeline.json").write_text(json.dumps(tl))
    if master:
        shutil.copy2(master, proj / "renders/master.mp4")
    return cd, proj


class Viewer(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.cd, self.proj = project(Path(self.tmp.name), None)
        (self.proj / "review").mkdir()

    def tearDown(self):
        self.tmp.cleanup()

    def run_check(self, text=None, issues=None, score=None):
        if text is not None:
            (self.proj / "review/viewer-v1.md").write_text(text)
        if issues is not None or score is not None:
            rv = {"issues": issues or []}
            if score is not None:
                rv["score"] = score
            (self.proj / "review.json").write_text(json.dumps(rv))
        rep = Report()
        viewer.check(self.proj, rep)
        viewer.score_check(self.proj, rep)
        return rep

    def test_no_viewer_or_no_verdict(self):
        self.assertIn("VW1", codes(self.run_check()))
        self.assertIn("VW1", codes(self.run_check("notes, but the helper ran out of budget")))
        self.assertIn("VW1", codes(self.run_check("Verdict: UNVERIFIABLE\nRed items: none\nRetell: ?")))

    def test_red_items_must_reach_review_json(self):
        text = "notes\nVerdict: FIX\nRed items: 3s caption over the face; 6.5s frozen hand\nRetell: a potter breaks a cup"
        rep = self.run_check(text, issues=[{"sev": "P0", "at": 3.2, "what": "caption", "status": "fixed"}], score=7)
        vw2 = [i["msg"] for i in rep.items if i["code"] == "VW2"]
        self.assertEqual(len(vw2), 1)
        self.assertIn("6.5s frozen hand", vw2[0])
        self.assertNotIn("caption over", vw2[0])
        rep = self.run_check(text, issues=[{"at": 3.0}, {"at": 6.0}], score=7)
        self.assertEqual(codes(rep), [])
        self.assertIn("VW0", codes(rep, "INFO"))

    def test_a_red_item_over_a_span_is_carried_by_an_issue_inside_it(self):
        text = "Verdict: FIX\nRed items: 27-35s table text unreadable; 40–42s blur\nRetell: r"
        self.assertEqual(codes(self.run_check(text, issues=[{"at": 31.0}, {"at": 41.0}], score=7)), [])
        vw2 = [i["msg"] for i in self.run_check(text, issues=[{"at": 31.0}], score=7).items if i["code"] == "VW2"]
        self.assertEqual(len(vw2), 1)
        self.assertIn("40.0s blur", vw2[0])

    def test_the_latest_viewer_wins(self):
        (self.proj / "review/viewer-v1.md").write_text("Verdict: FIX\nRed items: 2s x\nRetell: r")
        (self.proj / "review/viewer-v2.md").write_text("Verdict: APPROVE\nRed items: none\nRetell: r")
        self.assertEqual(codes(self.run_check(score=8)), [])

    def test_an_echoed_template_line_is_not_the_verdict(self):
        text = ("I was asked to end with\nVerdict: APPROVE | FIX | UNVERIFIABLE\nRed items: <second>s <what>\n\nnotes\n"
                "Verdict: FIX\nRed items: 6.5s frozen hand\nRetell: r")
        rep = self.run_check(text, issues=[], score=7)
        self.assertIn("VW2", codes(rep))
        self.assertIn("6.5s frozen hand", " ".join(i["msg"] for i in rep.items if i["code"] == "VW2"))

    def test_red_items_count_on_an_approve_too(self):
        rep = self.run_check("Verdict: APPROVE\nRed items: 12s text over face\nRetell: r", issues=[], score=8)
        self.assertIn("VW2", codes(rep))

    def test_an_answer_to_an_older_brief_is_stale(self):
        for n in (1, 2):
            (self.proj / f"review/viewer-brief-v{n}.md").write_text("brief")
        rep = self.run_check("Verdict: APPROVE\nRed items: none\nRetell: r", score=8)
        self.assertIn("viewer-brief-v2.md", " ".join(i["msg"] for i in rep.items if i["code"] == "VW1"))
        (self.proj / "review/viewer-v2.md").write_text("Verdict: APPROVE\nRed items: none\nRetell: r")
        self.assertEqual(codes(self.run_check(score=8)), [])

    def test_score_on_a_fixed_scale(self):
        ok = "Verdict: APPROVE\nRed items: none\nRetell: r"
        self.assertIn("RV2", codes(self.run_check(ok, issues=[])))
        self.assertIn("RV2", codes(self.run_check(ok, score="great")))
        self.assertIn("RV2", codes(self.run_check(ok, score=6), "INFO"))
        self.assertEqual(codes(self.run_check(ok, score=8)), [])

    def test_read_parses_the_three_lines(self):
        p = self.proj / "review/viewer-v1.md"
        p.write_text("Verdict: fix\nRed items: 1.5s a; no second\nRetell: two lines")
        self.assertEqual(viewer.read(p), {"verdict": "FIX", "retell": "two lines", "score": None,
                                          "red": [{"at": 1.5, "what": "a"}, {"at": None, "what": "no second"}]})
        p.write_text("Verdict: FIX\nRed items: none\nScore: 5\nRetell: r")
        self.assertEqual(viewer.read(p)["score"], 5.0)

    def test_the_makers_score_is_not_above_the_strangers(self):
        seen = "Verdict: FIX\nRed items: none\nScore: 5\nRetell: r"
        self.assertIn("RV2", codes(self.run_check(seen, score=7)))
        self.assertEqual(codes(self.run_check(seen, score=5)), [])


@unittest.skipUnless(FF, "needs ffmpeg")
class ViewerBrief(unittest.TestCase):
    def test_brief_shows_the_cut_and_hides_the_plan(self):
        with tempfile.TemporaryDirectory() as t:
            clip = Path(t) / "c.mp4"
            subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", f"testsrc2=s=180x320:r=30:d={D}",
                            "-pix_fmt", "yuv420p", str(clip)], check=True)
            cd, proj = project(Path(t), clip)
            out = viewer.brief(cd, proj)
            text = out.read_text()
            self.assertEqual(out.name, "viewer-brief-v1.md")
            for x in ("why the third fold holds", "awareness", "review/viewer-v1-strip-1.jpg", "wB02",
                      "another video", "15 tool calls", "Verdict: APPROVE | FIX | UNVERIFIABLE",
                      "review/viewer-v1-open.jpg", "small in an empty frame", "Score: <1-10>", "no reference videos", "weakest five seconds"):
                self.assertIn(x, text)
            self.assertTrue((proj / "review/viewer-v1-open.jpg").is_file())
            top = cd / "03-research/top"
            (top / "abc").mkdir(parents=True)
            shutil.copy2(clip, top / "abc/video.mp4")
            (top / "abc.json").write_text(json.dumps({"views": 120000}))
            text = viewer.brief(cd, proj).read_text()
            self.assertIn("120,000 views", text)
            self.assertIn("review/viewer-v2-refs.jpg", text)
            self.assertTrue((proj / "review/viewer-v2-refs.jpg").is_file())
            for x in ("05-scripts", "direction", "plan.md"):
                self.assertNotIn(f"`{x}", text)
            sheet = proj / "review/viewer-v1-strip-1.jpg"
            r = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "stream=width,height", "-of", "csv=p=0", str(sheet)],
                               capture_output=True, text=True, check=True)
            w, h = map(int, r.stdout.strip().split(","))
            self.assertLessEqual(w * h, SHEET_PX)
            self.assertEqual(viewer.brief(cd, proj).name, "viewer-brief-v3.md")

    def test_a_key_frame_is_laid_beside_the_most_viewed_references(self):
        with tempfile.TemporaryDirectory() as t:
            clip = Path(t) / "c.mp4"
            subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", f"testsrc2=s=180x320:r=30:d={D}",
                            "-pix_fmt", "yuv420p", str(clip)], check=True)
            cd, proj = project(Path(t), clip)
            frame = Path(t) / "f.png"
            subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", str(clip), "-frames:v", "1", str(frame)], check=True)

            def width(p):
                r = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "stream=width", "-of", "csv=p=0", str(p)],
                                   capture_output=True, text=True, check=True)
                return int(r.stdout.strip())
            out, notes = viewer.beside(frame, cd, proj / viewer.KEYFRAME)
            self.assertEqual(notes, [])  # no research video: the frame stands alone
            self.assertLess(width(out), viewer.KEY_W)
            top = cd / "03-research/top"
            (top / "abc").mkdir(parents=True)
            shutil.copy2(clip, top / "abc/video.mp4")
            (top / "abc.json").write_text(json.dumps({"views": 120000}))
            out, notes = viewer.beside(frame, cd, proj / viewer.KEYFRAME)
            self.assertEqual(notes, ["120,000 views"])
            self.assertEqual(width(out), viewer.KEY_W)
            self.assertEqual([p.name for p in (proj / "review").iterdir()], ["keyframe.jpg"])

    def test_no_render_is_a_clear_stop(self):
        with tempfile.TemporaryDirectory() as t:
            cd, proj = project(Path(t), None)
            with self.assertRaises(SystemExit) as e:
                viewer.brief(cd, proj)
            self.assertIn("render the cut first", str(e.exception))


class Plumbing(unittest.TestCase):
    def test_review_sheets_stay_under_a_megapixel(self):
        for W, H, c, r in ((1080, 1920, 6, 3), (1920, 1080, 6, 3), (1080, 1920, 8, 4), (1080, 1080, 6, 6)):
            w = tile_width(W, H, c, r, cap=10_000)
            self.assertLessEqual((w * c + 8 * c) * (w * H / W * r + 8 * r), SHEET_PX * 1.02, (W, H, c, r))
            self.assertEqual(w % 2, 0)

    def test_listen_plan_names_hook_keys_and_ending(self):
        tl = {**TL, "beats": [dict(b) for b in TL["beats"]]}
        tl["beats"][1].update(key=True, vo_start=1.3)
        self.assertEqual(listen_plan(tl), "PENDING LISTEN: 0.0–1.0s hook; 1.3–4.5s key B02; 4.5–8.0s ending B03")
        self.assertEqual(listen_plan({"beats": []}), "PENDING LISTEN")
        tl["beats"][1].update(vo_start=None)  # a key beat with no line (timing writes None, not a missing key)
        self.assertIn("1.0–4.5s key B02", listen_plan(tl))

    def test_a_zero_frame_rate_is_a_value_error(self):
        from unittest import mock
        out = json.dumps({"streams": [{"width": 2, "height": 2, "r_frame_rate": "0/0"}]})
        with mock.patch.object(picture.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, out, "")):
            with self.assertRaises(ValueError):
                picture._probe(Path("x.mp4"))


if __name__ == "__main__":
    unittest.main()
