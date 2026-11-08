"""Cut an image out of its background with the pinned RMBG-1.4 ONNX model (CPU, ~1 s).

  remove-bg <input> -o <new-output.png> [--mask <new-matte.png>]

The model sees a 1024x1024 resize; its matte is scaled back to the source size
and becomes the alpha of a new RGBA PNG. A source that already has alpha keeps
its transparent pixels transparent. The source is never touched and an existing
output is never overwritten. Licence: bria-rmbg-1.4 (see SKILL.md).
"""
import argparse
import json
import os
import resource
import sys
import time
from pathlib import Path

SPEC = json.loads((Path(__file__).resolve().parent / "models.json").read_text())["rmbg"]
WEIGHTS = Path(os.environ["IMAGE_GEN_HOME"]) / "models" / SPEC["weights"]["as"]


def installed() -> bool:
    return WEIGHTS.is_file() and WEIGHTS.stat().st_size == SPEC["weights"]["bytes"]


def cut(image):
    """(RGBA cutout, matte, stats) for a PIL image."""
    import numpy as np
    import onnxruntime as ort
    from PIL import Image

    rgb = image.convert("RGB")
    x = np.asarray(rgb.resize((1024, 1024), Image.BILINEAR), dtype=np.float32) / 255.0
    x = ((x - np.array(SPEC["mean"], np.float32)) / np.array(SPEC["std"], np.float32)).transpose(2, 0, 1)[None]
    opts = ort.SessionOptions()
    opts.log_severity_level = 3
    t0 = time.monotonic()
    session = ort.InferenceSession(str(WEIGHTS), opts, providers=["CPUExecutionProvider"])
    y = session.run(None, {session.get_inputs()[0].name: x})[0][0, 0].astype(np.float32)
    y = (y - y.min()) / max(float(y.max() - y.min()), 1e-6)
    matte = Image.fromarray((y * 255).round().astype(np.uint8)).resize(rgb.size, Image.BILINEAR)
    if "A" in image.getbands():
        matte = Image.fromarray(np.minimum(np.asarray(matte), np.asarray(image.getchannel("A"))))
    out = rgb.copy()
    out.putalpha(matte)
    a = np.asarray(matte)
    return out, matte, {"model": "rmbg-1.4", "seconds": round(time.monotonic() - t0, 2),
                        "transparent_pct": round(float((a < 8).mean()) * 100, 1),
                        "opaque_pct": round(float((a > 247).mean()) * 100, 1)}


def main():
    from PIL import Image

    p = argparse.ArgumentParser(prog="remove-bg")
    p.add_argument("input")
    p.add_argument("-o", "--output", required=True, help="new .png path")
    p.add_argument("--mask", help="also write the grayscale matte to this new path")
    a = p.parse_args()
    out = Path(a.output).expanduser().resolve()
    mask = Path(a.mask).expanduser().resolve() if a.mask else None
    if out.suffix.lower() != ".png":
        sys.exit("remove-bg: output must be .png (it carries alpha)")
    for path in filter(None, (out, mask)):
        if path.exists():
            sys.exit(f"remove-bg: {path} exists; choose a new path")
    if not installed():
        sys.exit("remove-bg: RMBG-1.4 not installed; run setup (without --no-cutout)")
    with Image.open(Path(a.input).expanduser()) as im:
        im.load()
        result, matte, stats = cut(im)
    out.parent.mkdir(parents=True, exist_ok=True)
    result.save(out)
    if mask:
        mask.parent.mkdir(parents=True, exist_ok=True)
        matte.save(mask)
    print(json.dumps({"output": str(out), "mask": str(mask) if mask else None, "width": result.width,
                      "height": result.height, **stats,
                      "peak_rss_mib": round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 2**20)}))


if __name__ == "__main__":
    main()
