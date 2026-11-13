"""Draw one image with Qwen Image 2.1 Turbo (Q4_K_M) through the pinned sd-cli, on Metal.

  generate-image "<prompt>" -o <new.png|.webp|.jpg> [--size WxH] [--seed N] [--ref ref.png] [--transparent]

  generate-image --limits     the sizes this machine draws, as JSON

One run at a time (a non-blocking lock: a second run is refused, not queued),
bounded pixels, a hard timeout that kills the native process group. The last
stdout line is a JSON receipt.

Weights are read from disk a segment at a time, so a run peaks near 3.5 GiB at
any size and quantization (measured 384x672 to 1088x1920, Q4 to Q8, M1 Pro
16 GB). Time is what a larger picture costs.

--transparent asks the model for alpha. Qwen's alpha is usually near-opaque, so
when under 5 % of the picture is clear the render is cut out with RMBG-1.4
(installed by setup unless --no-cutout); the receipt's `cutout` says which.
"""
import argparse
import fcntl
import hashlib
import json
import os
import resource
import signal
import subprocess
import sys
import tempfile
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))  # -I drops the script dir
SPEC = json.loads((HERE / "models.json").read_text())
Q = SPEC["qwen"]
HOME = Path(os.environ["IMAGE_GEN_HOME"])
MODELS = HOME / "models"
SD_CLI = HOME / "sd-build/bin/sd-cli"
SIDES, STEP = (256, 1920), 32
FULL_PIXELS = 1088 * 1920   # a whole 1080x1920 frame; measured on 16 GB
SMALL_PIXELS = 512 * 512    # under 16 GB nothing larger was measured
VRAM_GIB, RAM_GIB = 3, 2
TIMEOUT_S = 3000
MIN_CLEAR = 0.05


def fail(msg: str) -> None:
    sys.exit(f"generate-image: {msg}")


def limits() -> dict:
    """What this machine draws, from its memory. Callers (mvf art) size their jobs from it."""
    gib = os.sysconf("SC_PHYS_PAGES") * os.sysconf("SC_PAGE_SIZE") / 2**30
    return {"max_pixels": FULL_PIXELS if gib >= 15.5 else SMALL_PIXELS, "sides": list(SIDES), "step": STEP,
            "memory_gib": round(gib), "model": f"{Q['model']} {Q['quantization']}"}


def weight(key: str, w: dict) -> Path:
    p = MODELS / w["as"]
    if not p.is_file() or p.stat().st_size != w["bytes"]:
        fail(f"{key} weights missing or not the pinned file ({p}); run setup" + (" --with-vision" if key == "vision" else ""))
    return p


def clear_share(im) -> float:
    if "A" not in im.getbands():
        return 0.0
    hist = im.getchannel("A").histogram()
    return sum(hist[:128]) / (im.width * im.height)


def main():
    lim = limits()
    if sys.argv[1:] == ["--limits"]:
        print(json.dumps(lim))
        return
    from PIL import Image

    ap = argparse.ArgumentParser(prog="generate-image")
    ap.add_argument("prompt")
    ap.add_argument("-o", "--output", required=True)
    ap.add_argument("--size", default="512x512", help="WxH: multiples of 32, each side 256-1920, within --limits' max_pixels")
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--ref", help="PNG/JPEG reference <= 2 MiB (needs setup --with-vision)")
    ap.add_argument("--transparent", action="store_true", help="a cutout: model alpha, else RMBG-1.4")
    a = ap.parse_args()

    if not a.prompt.strip() or len(a.prompt) > 4000:
        fail("prompt must be 1-4000 characters")
    try:
        w, h = (int(v) for v in a.size.lower().split("x"))
    except ValueError:
        fail(f"--size {a.size!r} is not WxH")
    if not all(SIDES[0] <= v <= SIDES[1] and v % STEP == 0 for v in (w, h)) or w * h > lim["max_pixels"]:
        fail(f"--size {w}x{h}: each side a multiple of {STEP} in {SIDES[0]}-{SIDES[1]}, at most {lim['max_pixels']} pixels "
             f"on this {lim['memory_gib']} GB machine")
    if not 0 <= a.seed <= 2**31 - 1:
        fail("--seed out of range")
    out = Path(a.output).expanduser().resolve()
    if out.suffix.lower() not in (".png", ".webp", ".jpg", ".jpeg"):
        fail("output must be .png, .webp or .jpg")
    if out.exists():
        fail(f"{out} exists; choose a new path")
    if a.transparent and out.suffix.lower() in (".jpg", ".jpeg"):
        fail("--transparent needs .png or .webp (alpha)")
    ref = Path(a.ref).expanduser().resolve() if a.ref else None
    if ref and (ref.suffix.lower() not in (".png", ".jpg", ".jpeg") or not ref.is_file() or ref.stat().st_size > 2 << 20):
        fail("--ref must be an existing PNG/JPEG of at most 2 MiB")
    if not os.access(SD_CLI, os.X_OK):
        fail(f"no sd-cli at {SD_CLI}; run setup")
    paths = {k: weight(k, v) for k, v in Q["weights"].items()}
    if ref:
        paths["vision"] = weight("vision", Q["vision"])

    prompt = a.prompt
    if a.transparent:
        prompt = f"This is an RGBA image with transparency. {prompt} The image has alpha channel and the background is transparent."
    argv = [str(SD_CLI), "--diffusion-model", str(paths["diffusion"]), "--llm", str(paths["llm"]), "--vae", str(paths["vae"]),
            "--backend", "diffusion=metal,te=metal,vae=metal", "--params-backend", "disk", "--disable-prefetch",
            "--max-vram", f"metal={VRAM_GIB},cpu={RAM_GIB}", "--fa", "--vae-tiling", "--vae-tile-size", "512x512",
            "--vae-tile-overlap", "0.5", "--model-args", "qwen_image_2_1_prefix_cache_type=" + ("q8_0" if ref else "f16"),
            "--cfg-scale", str(Q["cfg"]), "--sampling-method", "euler", "--steps", str(Q["steps"]),
            "--sigmas", ",".join(map(str, Q["sample_sigmas"])), "--width", str(w), "--height", str(h),
            "--seed", str(a.seed), "-p", prompt]
    if ref:
        argv += ["--llm_vision", str(paths["vision"]), "-r", str(ref)]

    lock = open(HOME / "inference.lock", "a")
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        fail("busy: another generation holds the lock; retry after it finishes")
    with tempfile.TemporaryDirectory(prefix="image-gen-") as tmp:
        raw = Path(tmp) / "image.png"
        log = Path(tmp) / "sd-cli.log"
        t0 = time.monotonic()
        with log.open("wb") as fh:
            proc = subprocess.Popen([*argv, "-o", str(raw)], cwd=tmp, stdin=subprocess.DEVNULL, stdout=fh, stderr=fh,
                                    start_new_session=True)
            try:
                rc = proc.wait(timeout=TIMEOUT_S)
            except BaseException:  # timeout or Ctrl-C: never leave a 4 GiB model running
                os.killpg(proc.pid, signal.SIGKILL)
                proc.wait()
                fail(f"stopped after {round(time.monotonic() - t0)} s; no image written")
        seconds = round(time.monotonic() - t0, 1)
        if rc or not raw.is_file():
            tail = log.read_text(errors="replace")[-1200:]
            fail(f"sd-cli exited {rc} (memory or runtime?):\n{tail}")
        with Image.open(raw) as im:
            im.load()
            img = im.copy()
        if img.size != (w, h):
            fail(f"sd-cli returned {img.size}, not {w}x{h}")
        receipt = {"cutout": None}
        if not a.transparent:
            img = img.convert("RGB")  # the RGBA VAE's stray alpha is not a cutout
        elif clear_share(img) >= MIN_CLEAR:
            receipt["cutout"] = "model-alpha"
        else:
            import remove_bg
            if remove_bg.installed():
                img, _, stats = remove_bg.cut(img)
                receipt["cutout"] = "rmbg-1.4"
                receipt["rmbg"] = stats
            else:
                receipt["cutout"] = "none"
                print("generate-image: the model's alpha is opaque and RMBG-1.4 is not installed: no cutout", file=sys.stderr)
        out.parent.mkdir(parents=True, exist_ok=True)
        staged = Path(tempfile.mkstemp(prefix=".image-gen-", suffix=out.suffix, dir=out.parent)[1])
        try:
            if out.suffix.lower() == ".png":
                img.save(staged, format="PNG")
            else:
                (img.convert("RGB") if out.suffix.lower() in (".jpg", ".jpeg") else img).save(staged, quality=92)
            os.link(staged, out)  # never overwrite
        except FileExistsError:
            fail(f"{out} appeared meanwhile; choose a new path")
        finally:
            staged.unlink(missing_ok=True)
    print(json.dumps({"output": str(out), "prompt": a.prompt, "width": w, "height": h, "seed": a.seed, "seconds": seconds,
                      "model": f"{Q['model']} {Q['quantization']}", "steps": Q["steps"], "ref": str(ref) if ref else None,
                      "transparent": a.transparent, **receipt,
                      "peak_rss_mib": round(resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss / 2**20),
                      "sha256": hashlib.sha256(out.read_bytes()).hexdigest()}, ensure_ascii=False))


if __name__ == "__main__":
    main()
