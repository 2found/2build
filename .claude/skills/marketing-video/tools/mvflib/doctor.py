"""`mvf doctor`: is this machine ready to make a video? One line per backbone (HyperFrames edits, OmniVoice
speaks, Qwen Image + RMBG draw, Micro Fish judges), each with the fix. The agent fixes what it can; what only
the user can do (sign in to an AI provider, pick the evaluator model) is marked NEEDS."""
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
from pathlib import Path

from . import judge
from .common import HYPERFRAMES, config, local_dir


def _ver(cmd: list[str]) -> str | None:
    try:
        r = subprocess.run(cmd, capture_output=True, text=True, timeout=20)
    except (OSError, subprocess.TimeoutExpired):
        return None
    return (r.stdout or r.stderr).strip().splitlines()[0] if r.returncode == 0 and (r.stdout or r.stderr).strip() else None


def _hyperframes_cached() -> bool:
    for pj in (Path.home() / ".npm/_npx").glob("*/node_modules/hyperframes/package.json"):
        try:
            if json.loads(pj.read_text()).get("version") == HYPERFRAMES:
                return True
        except (OSError, json.JSONDecodeError):
            continue
    return False


def checks(cd: Path | None = None, quick: bool = False) -> list[dict]:
    out = []

    def add(area, state, what, fix=""):
        out.append({"area": area, "state": state, "what": what, "fix": fix})

    # ---- edit: HyperFrames (node + npx + ffmpeg) ----
    for tool in ("ffmpeg", "ffprobe"):
        v = _ver([tool, "-version"]) if shutil.which(tool) else None
        add("edit", "OK" if v else "MISSING", f"{tool} {v.split(' ')[2] if v and len(v.split(' ')) > 2 else ''}".strip(),
            "" if v else "brew install ffmpeg")
    nv = _ver(["node", "--version"]) if shutil.which("node") else None
    major = int(re.sub(r"[^0-9].*", "", nv.lstrip("v")) or 0) if nv else 0
    add("edit", "OK" if major >= 22 else "MISSING", f"node {nv or 'not found'} (HyperFrames needs 22+)",
        "" if major >= 22 else "install Node.js 22 (brew install node@22, or nvm install 22)")
    if shutil.which("npx"):
        cached = _hyperframes_cached()
        add("edit", "OK" if cached else "WARN", f"hyperframes@{HYPERFRAMES} " + ("cached" if cached else "not cached yet"),
            "" if cached else f"the first scaffold check or render downloads it (npx --yes hyperframes@{HYPERFRAMES} --version)")
    else:
        add("edit", "MISSING", "npx", "comes with Node.js")

    # ---- voice: OmniVoice (+ Whisper word timing) ----
    py = os.environ.get("MVF_OMNIVOICE_PYTHON", "").strip()
    if py:
        ok = Path(py).is_file() and _ver([py, "-c", "import omnivoice, whisper; print('ok')"]) == "ok"
        add("voice", "OK" if ok else "MISSING", f"OmniVoice via $MVF_OMNIVOICE_PYTHON ({py})",
            "" if ok else "that Python lacks omnivoice/whisper: unset MVF_OMNIVOICE_PYTHON to use the uv environment")
    elif shutil.which("uv"):
        add("voice", "OK", "OmniVoice via uv (first take downloads ~2 GB of wheels + weights, then cached)")
    else:
        add("voice", "MISSING", "uv (runs OmniVoice in its own environment)", "brew install uv")
    refs = sorted({p.stem for d in ((cd / "voices") if cd else None, local_dir(cd) / "voices") if d and d.is_dir()
                   for p in d.glob("*.wav") if p.with_suffix(".txt").is_file()})
    add("voice", "OK" if refs else "INFO", "reference voices: " + (", ".join(refs) if refs else "none yet"),
        "" if refs else "design one once per campaign: mvf voice-design --name N --instruct … --sample …, then voice-pick")

    # ---- image: Qwen Image + RMBG (image-gen skill) ----
    from .art import image_gen_dir
    ig = image_gen_dir()
    if not (ig / "generate-image").is_file():
        add("image", "MISSING", f"image-gen skill ({ig})", "install the image-gen skill beside this one, or set MVF_IMAGE_GEN")
    elif quick:
        add("image", "INFO", "image-gen installed (weights not verified: --quick)")
    else:
        r = subprocess.run([str(ig / "setup"), "--check"], capture_output=True, text=True)
        ready = r.returncode == 0
        add("image", "OK" if ready else "MISSING", "Qwen Image 2.1 Turbo + RMBG-1.4 weights " + ("verified" if ready else "not set up"),
            "" if ready else f"{ig / 'setup'} (downloads the weights once; no account, no cost)")
    lic = config(cd).get("rmbg_license")
    add("image", "OK" if lic == "bria-agreement" else "WARN",
        "RMBG-1.4 cutouts: " + ({"bria-agreement": "commercial use covered by a Bria agreement",
                                 "non-commercial": "non-commercial use declared"}.get(lic) or "licence not declared"),
        "" if lic == "bria-agreement" else "RMBG-1.4 is non-commercial; marketing a product needs a Bria agreement. Record "
        "it once: ~/.babysit/marketing-video/config.json {\"rmbg_license\": \"bria-agreement\"|\"non-commercial\"}")

    # ---- judge: Micro Fish on Soot ----
    st = judge.status()
    sv = (_ver([st["soot"], "version"]) or "").split(" (")[0] if st["soot"] else "not found"
    add("judge", "OK" if st["soot"] else "MISSING", f"Soot {sv.removeprefix('soot ')}",
        "" if st["soot"] else "install Soot (https://github.com/2found/2agent)")
    add("judge", "OK" if st["micro_fish"] else "MISSING", f"micro-fish skill {st['micro_fish'] or 'not found'}")
    if st["signed_in"]:
        add("judge", "OK", "Soot providers signed in: " + ", ".join(st["signed_in"]))
    ch = st["choice"]
    if ch:
        add("judge", "OK", f"evaluator model {ch['provider']}/{ch['model']} (chosen {ch.get('set_at')})")
    add("judge", "OK" if st["workspace"] else ("INFO" if not ch else "MISSING"),
        f"Micro Fish workspace {judge.home() / 'sim'} " + ("ready" if st["workspace"] else "not created"))
    seen = {x["what"] for x in out}
    for what, msg, fix in st["problems"]:
        state = "NEEDS" if what in {"provider", "model"} else "MISSING"
        if msg not in seen:
            add("judge", state, msg, fix)

    # ---- research (optional) ----
    v = _ver(["yt-dlp", "--version"]) if shutil.which("yt-dlp") else None
    add("research", "OK" if v else "WARN", f"yt-dlp {v or 'not found'} (top-videos, trend-video)",
        "" if v else "brew install yt-dlp")
    return out


def run(cd: Path | None, as_json: bool, quick: bool) -> int:
    xs = checks(cd, quick)
    blocking = [x for x in xs if x["state"] in {"MISSING", "NEEDS"}]
    if as_json:
        print(json.dumps({"ready": not blocking, "checks": xs}, ensure_ascii=False, indent=1))
    else:
        for x in xs:
            print(f"{x['state']:7} {x['area']:8} {x['what']}" + (f"\n{'':17}→ {x['fix']}" if x["fix"] else ""))
        needs = [x for x in blocking if x["state"] == "NEEDS"]
        print("READY" if not blocking else
              f"NOT_READY: {len(blocking)} to fix" + (f" ({len(needs)} only the user can do: ask them)" if needs else ""))
    return 0 if not blocking else 1
