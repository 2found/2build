"""A custom thumbnail / cover still: one authored HyperFrames frame (an approved art
plate, the thumbnail line in the brand type, the real product), snapshotted and
encoded to the platform's limits. YouTube long-form needs one (1280x720+, ≤ 2 MB);
a Shorts or TikTok cover is the video's own frame 0 and does not.

`07-video/<sid>/thumb-<canvas>/index.html` is written once and then yours to edit;
re-running re-renders it."""
from __future__ import annotations

import html
import shutil
import subprocess
from pathlib import Path

from . import art
from .common import HYPERFRAMES, PLATFORMS, Report, heavy_prefix, load
from .scaffold import _brand

MAX_BYTES = 2_000_000  # YouTube: 2 MB on mobile uploads (answer/72431)


def thumb_text(script: dict, canvas: str) -> str:
    p = script.get("platforms") or {}
    if canvas == "16x9":
        return (p.get("youtube-long") or {}).get("thumbnail_text", "")
    return next((c.get("cover_text") for c in p.values() if c and c.get("cover_text")), "")


def pick_art(cd: Path, canvas: str, want: str | None) -> str | None:
    if want:
        if not art.approved(cd, want):
            raise SystemExit(f"art {want} is not approved (mvf art approve {want})")
        return want
    for jid, job in art.jobs_by_id(cd).items():
        if job.get("use") == "thumbnail" and (job.get("canvas") or "16x9") == canvas and art.approved(cd, jid):
            return jid
    return None


def page(W: int, H: int, brand: dict, text: str, plate: str | None, logo: str | None) -> str:
    c, f = brand["colors"], brand["fonts"]
    big = 150 if W > H else 132
    img = f'<img id="plate" src="{plate}" alt="">' if plate else ""
    lg = f'<img id="logo" src="{logo}" alt="">' if logo else ""
    return f"""<!doctype html>
<!-- mvf thumbnail: written once, now yours. One still at t=0.5 s. Words here, never in the picture;
     ≤ 5 words, readable at 320 px wide; the product or a face carries it, never a logo card. -->
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width={W}, height={H}" />
    <script src="https://cdn.jsdelivr.net/npm/gsap@3.14.2/dist/gsap.min.js"></script>
    <style>
      * {{ margin: 0; padding: 0; box-sizing: border-box; }}
      html, body {{ width: {W}px; height: {H}px; overflow: hidden; background: {c['bg']}; }}
      #root {{ position: relative; width: 100%; height: 100%; }}
      #plate {{ position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }}
      #shade {{ position: absolute; inset: 0; background: linear-gradient({'90deg' if W > H else '0deg'}, {c['bg']}ee 0%, {c['bg']}99 45%, transparent 75%); }}
      #line {{ position: absolute; {'left: 80px; top: 50%; transform: translateY(-50%); width: 58%;' if W > H else 'left: 70px; right: 70px; top: 300px;'}
               font: 800 {big}px/1.0 '{f['display']}', system-ui, sans-serif; color: {c['fg']}; letter-spacing: -0.02em; }}
      #line em {{ font-style: normal; color: {c['accent']}; }}
      #logo {{ position: absolute; right: 56px; bottom: 48px; height: 72px; }}
    </style>
  </head>
  <body>
    <div id="root" data-composition-id="main" data-start="0" data-duration="1" data-width="{W}" data-height="{H}">
      {img}
      <div id="shade"></div>
      <div id="line">{html.escape(text) or 'THUMBNAIL LINE'}</div>
      {lg}
    </div>
    <script>
      window.__timelines = window.__timelines || {{}};
      window.__timelines["main"] = gsap.timeline({{ paused: true }});
    </script>
  </body>
</html>
"""


def qa(path: Path, canvas: str) -> Report:
    r = Report()
    w, h = art.image_size(path)
    c = PLATFORMS["canvases"][canvas]
    if abs(w / h - c["width"] / c["height"]) > 0.01:
        r.fail("T1", path.name, f"{w}x{h} is not {canvas}")
    if min(w, h) < 640 or (canvas == "16x9" and w < 1280):
        r.fail("T1", path.name, f"{w}x{h}: YouTube wants ≥ 1280x720 (640 minimum)")
    if path.stat().st_size > MAX_BYTES:
        r.fail("T2", path.name, f"{path.stat().st_size} bytes > 2 MB")
    mean = subprocess.run(["ffmpeg", "-hide_banner", "-i", str(path), "-vf", "signalstats,metadata=print:key=lavfi.signalstats.YAVG",
                           "-f", "null", "-"], capture_output=True, text=True).stderr
    y = [float(x.split("=")[1]) for x in mean.splitlines() if "YAVG=" in x]
    if y and y[0] < 12:
        r.fail("T3", path.name, f"mean luma {y[0]:.0f}: the still is black (did the page load?)")
    return r


def thumbnail(cd: Path, sid: str, canvas: str, art_id: str | None, text: str | None) -> tuple[Path, Report]:
    script = load(cd / "05-scripts" / f"{sid}.json")
    c = load(cd / "campaign.json")
    brand = _brand(c)
    cv = PLATFORMS["canvases"][canvas]
    W, H = cv["width"], cv["height"]
    proj = cd / "07-video" / sid / f"thumb-{canvas}"
    (proj / "assets").mkdir(parents=True, exist_ok=True)
    jid = pick_art(cd, canvas, art_id)
    plate = None
    if jid:
        (proj / "assets/art").mkdir(exist_ok=True)
        shutil.copy2(cd / art.ART / f"{jid}.png", proj / "assets/art" / f"{jid}.png")
        plate = f"assets/art/{jid}.png"
    logo = None
    if (cd / "brand").is_dir():
        shutil.copytree(cd / "brand", proj / "assets/brand", dirs_exist_ok=True)
        logo = f"assets/brand/{Path(brand['logo']).name}" if brand.get("logo") else None
    idx = proj / "index.html"
    if not idx.exists():
        idx.write_text(page(W, H, brand, text or thumb_text(script, canvas), plate, logo), encoding="utf-8")
    out_dir = proj / "renders"
    shutil.rmtree(out_dir, ignore_errors=True)
    log = proj / "snapshot.log"
    with open(log, "w") as fh:
        r = subprocess.run([*heavy_prefix(cd), "npx", "--yes", f"hyperframes@{HYPERFRAMES}", "snapshot", ".", "--at", "0.5",
                            "--no-end", "--describe", "false", "-o", "renders"], cwd=proj, stdout=fh, stderr=subprocess.STDOUT)
    pngs = sorted(out_dir.rglob("*.png")) if out_dir.is_dir() else []
    if r.returncode or not pngs:
        raise SystemExit(f"snapshot failed (see {log}):\n{log.read_text()[-1500:]}")
    out = cd / "07-video" / sid / f"thumbnail-{canvas}.jpg"
    for q in (2, 4, 6, 9):  # the best JPEG quality that fits 2 MB
        subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(pngs[0]), "-vf", f"scale={W}:{H}",
                        "-q:v", str(q), str(out)], check=True)
        if out.stat().st_size <= MAX_BYTES:
            break
    return out, qa(out, canvas)
