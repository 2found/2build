"""Download the pinned weights from HuggingFace and verify size + SHA-256.

  fetch.py [--with-vision] [--no-cutout] [--check]

A file lands in $IMAGE_GEN_HOME/models/<as> only after it matches models.json;
an existing file is verified, never overwritten. --check uses no network.
"""
import hashlib
import json
import os
import sys
import urllib.request
from pathlib import Path

spec = json.loads((Path(__file__).resolve().parent / "models.json").read_text())
models = Path(os.environ["IMAGE_GEN_HOME"]) / "models"
args = set(sys.argv[1:])
if args - {"--with-vision", "--no-cutout", "--check"}:
    sys.exit(f"fetch: unknown arguments {sorted(args - {'--with-vision', '--no-cutout', '--check'})}")
want = dict(spec["qwen"]["weights"])
if "--with-vision" in args:
    want["vision"] = spec["qwen"]["vision"]
if "--no-cutout" not in args:
    want["rmbg"] = spec["rmbg"]["weights"]


def sha256(p: Path) -> str:
    h = hashlib.sha256()
    with p.open("rb") as f:
        for block in iter(lambda: f.read(4 << 20), b""):
            h.update(block)
    return h.hexdigest()


models.mkdir(parents=True, exist_ok=True)
for name, w in want.items():
    dest = models / w["as"]
    if dest.is_file():
        if dest.stat().st_size != w["bytes"] or sha256(dest) != w["sha256"]:
            sys.exit(f"fetch: {dest} does not match the pinned {name}; delete it and rerun setup")
    elif "--check" in args:
        sys.exit(f"fetch: missing {name} ({dest}); run setup")
    else:
        url = f"https://huggingface.co/{w['repository']}/resolve/{w['revision']}/{w['file']}"
        print(f"downloading {name}: {w['repository']}@{w['revision'][:8]} {w['file']} ({w['bytes'] / 1e9:.2f} GB)", flush=True)
        part = dest.with_name(dest.name + ".part")
        h, total = hashlib.sha256(), 0
        try:
            with urllib.request.urlopen(url, timeout=120) as src, part.open("wb") as out:
                for block in iter(lambda: src.read(4 << 20), b""):
                    total += len(block)
                    if total > w["bytes"]:
                        raise ValueError("larger than pinned")
                    h.update(block)
                    out.write(block)
            if total != w["bytes"] or h.hexdigest() != w["sha256"]:
                raise ValueError("size or SHA-256 differs from models.json")
            os.link(part, dest)  # never overwrite
        except (OSError, ValueError) as e:
            sys.exit(f"fetch: {name} failed: {e}")
        finally:
            part.unlink(missing_ok=True)
    print(f"verified {name}: {w['sha256'][:16]}…", flush=True)
