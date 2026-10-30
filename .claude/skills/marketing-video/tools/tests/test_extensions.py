"""Extensions (load order, contracts), the built-in files voice engine, score and master.
Stdlib only; the audio tests need ffmpeg and skip without it."""
import json
import os
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

TOOLS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(TOOLS))
os.environ["MVF_TODAY"] = "2026-10-27"

from mvflib import extensions, material  # noqa: E402
from mvflib.lint import lint  # noqa: E402
from test_mvf import CAMPAIGN, REFS, codes, script  # noqa: E402

FFMPEG = shutil.which("ffmpeg") and shutil.which("ffprobe")
ENV = {**os.environ, "PYTHONDONTWRITEBYTECODE": "1", "MVF_EXTENSIONS": "", "MVF_HEAVY": ""}


def mvf(cwd: Path, *args, ok=True, env=None) -> subprocess.CompletedProcess:
    r = subprocess.run([sys.executable, str(TOOLS / "mvf.py"), *args], cwd=cwd, capture_output=True, text=True,
                       env={**ENV, **(env or {})})
    if ok and r.returncode:
        raise AssertionError(f"mvf {' '.join(args)} failed:\n{r.stdout}\n{r.stderr}")
    return r


def repo() -> tuple[tempfile.TemporaryDirectory, Path]:
    d = tempfile.TemporaryDirectory()
    root = Path(d.name)
    (root / ".git").mkdir()
    return d, root


def campaign(root: Path, voice=None, **script_over) -> Path:
    mvf(root, "init", "demo", "--product", "Acme")
    cd = root / "marketing/campaigns/demo"
    s = script(voice=voice or {"mode": "recorded"}, **script_over)
    (cd / "05-scripts/s01-test.json").write_text(json.dumps(s))
    return cd


def tone(path: Path, lead=0.3, speech=1.0, tail=0.3) -> None:
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i",
                    f"anullsrc=r=48000:cl=mono:d={lead}", "-f", "lavfi", "-i", f"sine=f=220:r=48000:d={speech}",
                    "-f", "lavfi", "-i", f"anullsrc=r=48000:cl=mono:d={tail}", "-filter_complex",
                    "[0][1][2]concat=n=3:v=0:a=1", str(path)], check=True)


class Layout(unittest.TestCase):
    def test_init_gitignores_local_state_and_extensions_init_ignores_everything(self):
        d, root = repo()
        with d:
            cd = campaign(root)
            gi = (cd / ".gitignore").read_text()
            for line in ("extensions/", "voices/", "08-delivery/", "07-video/*/takes/", "07-video/*/recordings/"):
                self.assertIn(line, gi)
            mvf(root, "extensions", "init")
            local = root / ".babysit/marketing-video"
            self.assertEqual((local / ".gitignore").read_text().splitlines()[-1], "*")
            self.assertTrue((local / "extensions").is_dir() and (local / "voices").is_dir())
            self.assertIn("omnivoice", mvf(cd, "extensions").stdout)


class Loading(unittest.TestCase):
    EXT = textwrap.dedent("""
        from mvflib.extensions import Source, Voice
        SOURCES = {{"acme.reviews": Source("{tag} reviews", ["review"], lambda p: [])}}
        KINDS = {{"changelog": "a release note"}}
        RECIPES = {{"launch-note": {{"about": "x", "needs": [{{"kind": "changelog", "min": 1}}]}}}}
        VOICES = {{"omnivoice": Voice("{tag} shadow", run=lambda *a: None)}}
    """)

    def test_campaign_beats_local_beats_builtin(self):
        d, root = repo()
        with d:
            cd = root / "c"
            (cd / "extensions").mkdir(parents=True)
            (cd / "campaign.json").write_text("{}")
            (cd / "extensions/mine.py").write_text(self.EXT.format(tag="campaign"))
            local = extensions.init_local(cd) / "extensions"
            (local / "pkg").mkdir()
            (local / "pkg/extension.py").write_text(self.EXT.format(tag="local"))
            ext = extensions.load(cd)
            self.assertEqual(ext.sources["acme.reviews"].about, "campaign reviews")
            self.assertEqual(ext.voices["omnivoice"].about, "campaign shadow")
            self.assertIn("changelog", material.kinds(cd))
            self.assertIn("launch-note", ext.recipes)
            shadowed = {(k, n) for k, n, _ in ext.shadowed}
            self.assertIn(("VOICES", "omnivoice"), shadowed)  # local and the shipped omnivoice lost
            self.assertIn(("SOURCES", "acme.reviews"), shadowed)

    def test_a_voice_needs_exactly_one_of_command_or_run(self):
        d, root = repo()
        with d:
            (root / "campaign.json").write_text("{}")
            (root / "extensions").mkdir()
            (root / "extensions/bad.py").write_text("from mvflib.extensions import Voice\n"
                                                     "VOICES = {'x': Voice('x', command=['a'], run=print)}\n")
            with self.assertRaises(SystemExit):
                extensions.load(root)

    def test_extension_recipe_and_kind_reach_lint_and_material(self):
        note = {"id": "acme:changelog:v2", "kind": "changelog", "title": "v2", "rights": "own",
                "source": {"name": "t", "fetched_at": "2026-10-27"}}
        self.assertEqual(material.validate(note, known={**material.KINDS, "changelog": "x"}).fails, 0)
        self.assertGreater(material.validate(note).fails, 0)
        s = script(recipe="launch-note")
        s["beats"][1]["material"] = [note["id"]]
        recipes = {"launch-note": {"needs": [{"kind": "changelog", "min": 1}]}}
        rep = lint(s, CAMPAIGN, {**REFS, "material": {note["id"]: note}, "recipes": recipes})
        self.assertNotIn("M3", codes(rep, "FAIL"))
        self.assertIn("M3", codes(lint(s, CAMPAIGN, {**REFS, "material": {note["id"]: note}}), "FAIL"))


FAKE_ENGINE = textwrap.dedent("""
    import json, sys, wave
    from pathlib import Path
    mode, job, out = sys.argv[1], json.loads(Path(sys.argv[3]).read_text()), Path(sys.argv[5])
    extra = sys.argv[6:]
    out.mkdir(parents=True, exist_ok=True)
    cues = []
    for c in job["cues"]:
        with wave.open(str(out / f"{c['id']}.wav"), "wb") as w:
            w.setnchannels(1); w.setsampwidth(2); w.setframerate(16000); w.writeframes(b"\\0\\0" * 16000)
        cues.append({"id": c["id"], "text": c["text"], "path": f"{c['id']}.wav", "duration_s": 1.0,
                     "speech": [0.1, 0.9], "words": [], "asr": {"heard": c["text"], "dropped": [], "added": []}})
    json.dump({"engine": "fake", "language": job["language"], "voice": job["voice"], "extra": extra, "cues": cues},
              open(out / "manifest.json", "w"))
""")


class CommandEngine(unittest.TestCase):
    def test_job_contract_opts_heavy_prefix_and_reuse(self):
        d, root = repo()
        with d:
            cd = campaign(root, voice={"mode": "preset", "name": "nova", "engine": "fake"})
            (cd / "extensions/fake").mkdir()  # an extension with helper files is a folder
            (cd / "extensions/fake/fake_engine.py").write_text(FAKE_ENGINE)
            (cd / "extensions/fake/extension.py").write_text(
                "import sys\nfrom pathlib import Path\nfrom mvflib.extensions import Voice\n"
                "VOICES = {'fake': Voice('test', command=[sys.executable, str(Path(__file__).with_name('fake_engine.py'))])}\n")
            r = mvf(cd, "voice", "s01-test", "--opt", "style=calm", env={"MVF_HEAVY": "env"})
            self.assertIn("3 line(s) to voice with fake", r.stdout)
            m = json.loads((cd / "07-video/s01-test/takes/t01/manifest.json").read_text())
            self.assertEqual(m["voice"], {"mode": "preset", "speed": 1.0, "name": "nova"})
            self.assertEqual(m["extra"], ["--style", "calm"])
            self.assertEqual(m["mvf_voice"]["engine"], "fake")
            self.assertIn("already voices every line", mvf(cd, "voice", "s01-test").stdout)

    def test_clone_without_engine_or_voice_names_the_way(self):
        d, root = repo()
        with d:
            cd = campaign(root, voice={"mode": "clone", "ref": "me"})
            r = mvf(cd, "voice", "s01-test", ok=False)
            self.assertNotEqual(r.returncode, 0)
            self.assertIn("needs an engine", r.stderr)
            r = mvf(cd, "voice", "s01-test", "--engine", "omnivoice", ok=False)
            self.assertIn("reference voice 'me' not found", r.stderr)
            self.assertIn(".babysit/marketing-video/voices", r.stderr)


@unittest.skipUnless(FFMPEG, "ffmpeg not installed")
class FilesEngine(unittest.TestCase):
    def test_missing_recordings_are_listed_then_voiced_and_measured(self):
        d, root = repo()
        with d:
            cd = campaign(root)
            r = mvf(cd, "voice", "s01-test", ok=False)
            self.assertIn("record these lines", r.stderr)
            self.assertIn("B02: Watch this filter sort it.", r.stderr)
            rec = cd / "07-video/s01-test/recordings"
            rec.mkdir(parents=True)
            for cue in ("B01", "B02", "B03"):
                tone(rec / f"{cue}.wav")
            mvf(cd, "voice", "s01-test")
            m = json.loads((cd / "07-video/s01-test/takes/t01/manifest.json").read_text())
            c = m["cues"][1]
            self.assertAlmostEqual(c["speech"][0], 0.3, delta=0.08)
            self.assertAlmostEqual(c["speech"][1], 1.3, delta=0.08)
            self.assertEqual([w["w"] for w in c["words"]], ["Watch", "this", "filter", "sort", "it"])
            self.assertEqual(c["words"][0]["s"], c["speech"][0])
            self.assertEqual(m["mvf_voice"]["engine"], "files")
            self.assertIn("(take t01)", mvf(cd, "lint", "s01-test", ok=False).stdout)


@unittest.skipUnless(FFMPEG, "ffmpeg not installed")
class ScoreAndMaster(unittest.TestCase):
    def test_bed_is_levelled_ducked_and_master_hits_minus_14(self):
        from mvflib.export import loudness, master_audio
        from mvflib.mix import duck_expr, score
        self.assertEqual(duck_expr([], 0.4), "1")
        self.assertIn("max(", duck_expr([(0.5, 1.0), (2.0, 3.0)], 0.4))
        with tempfile.TemporaryDirectory() as d:
            proj = Path(d)
            (proj / "assets/vo").mkdir(parents=True)
            tone(proj / "assets/vo/B01.wav", 0.0, 1.5, 0.0)
            (proj / "timeline.json").write_text(json.dumps({"duration": 4.0, "beats": [
                {"id": "B01", "cue": "B01", "vo_start": 0.5, "vo_end": 2.0, "start": 0, "end": 4.0}]}))
            bed = proj / "bed-src.wav"
            subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
                            "sine=f=440:r=48000:d=1.5", str(bed)], check=True)
            rep = score(proj, bed)
            out = proj / "assets/music/bed.wav"
            self.assertTrue(out.is_file())
            info = json.loads(subprocess.run(["ffprobe", "-v", "error", "-show_format", "-of", "json", str(out)],
                                             capture_output=True, text=True).stdout)
            self.assertAlmostEqual(float(info["format"]["duration"]), 4.0, delta=0.05)  # looped past 1.5 s
            self.assertLess(loudness(out)[0], rep["voice_lufs"] - 6)
            self.assertEqual(score(proj, None)["bed"], None)
            self.assertFalse(out.exists())
            vid = proj / "raw.mp4"
            subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=gray:s=64x64:d=3",
                            "-f", "lavfi", "-i", "sine=f=330:r=48000:d=3,volume=-20dB", "-shortest", "-c:v", "libx264",
                            "-c:a", "aac", str(vid)], check=True)
            lufs, peak = loudness(master_audio(vid, proj / "master.mp4"))
            self.assertAlmostEqual(lufs, -14.0, delta=1.0)
            self.assertLess(peak, -0.5)


if __name__ == "__main__":
    unittest.main()
