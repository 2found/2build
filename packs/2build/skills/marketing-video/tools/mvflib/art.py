"""Art: generated pictures a video stands on next to the real screens — plates,
backgrounds, insets, stickers, thumbnail plates and character references.

One jobs file per campaign, `07-video/art/art.json`; drafts are never overwritten
(`draft/<ID>-s<seed>.png` + a `.json` receipt), and approving one fits it to the size
it is SHOWN at (`<ID>.png` + `<ID>.json`). The gate is that display size over the
drawn size. A draft is drawn no larger than it is shown (`"draw": "WxH"` on a job
overrides that), up to what the engine draws on this machine. Scaffold copies approved art to every project's
`assets/art/`; a beat cites what it shows in `art: [ID]`.

  {"style": "appended to every prompt", "engine": "qwen",
   "jobs": [{"id": "K0", "use": "reference", "prompt": "…", "canvas": "1x1"},
            {"id": "P01", "use": "background", "prompt": "…", "ref": "K0", "look": "illustration"},
            {"id": "S1", "use": "sticker", "display": "420x420", "transparent": true, "prompt": "…"},
            {"id": "C1", "use": "sticker", "display": "500x500", "transparent": true, "image": "brand/founder.jpg"}]}

A job with `image` starts from a picture the campaign already has (a product photo, the founder, a real
cover) instead of a prompt: `draw` cuts it out with RMBG-1.4 when transparent (else takes it as is), and
approve fits it like any draft. Nothing is generated, so it never counts toward the AI label.
"""
from __future__ import annotations

import datetime as dt
import functools
import hashlib
import json
import math
import os
import re
import struct
import subprocess
from pathlib import Path

from . import extensions
from .common import PLATFORMS, SKILL, STYLES, Report, campaign_style, dump, heavy_prefix, load

ART = Path("07-video/art")
ID = re.compile(r"^[A-Za-z][A-Za-z0-9_-]{0,23}$")
# use -> (default canvas, PASS up to this upscale, WARN up to this, above FAILs). None = never shown.
USES = {
    "plate": ("9x16", 1.5, 2.0),        # the full picture of a scene
    "background": ("9x16", 3.0, 3.6),   # under text or UI, dimmed or blurred: softness is fine
    "inset": ("1x1", 1.5, 2.0),         # a card or panel shown smaller than the frame (needs display)
    "sticker": ("1x1", 1.5, 2.0),       # a cutout over the scene (needs display)
    "thumbnail": ("16x9", 1.5, 2.0),    # the plate of a custom thumbnail or cover
    "reference": ("1x1", None, None),   # a character or style sheet other jobs --ref; never on screen
}
LOOKS = {"illustration", "photoreal", "texture"}
# Code draws the picture; a generated full-frame picture is the exception (an establishing beat or two).
FULL_FRAME, MAX_FULL_FRAME = {"plate", "background"}, 2
# Where a recurring character's look comes from (a mascot, a game hero, a book's adaptation). Viewers who know that
# look carry it in their head; a character drawn otherwise reads as "not them". `none` = no adaptation: the design is ours (a reference sheet keeps it).
CAST_SOURCES = {"animation", "comic", "game", "official-art", "live-action", "none"}
PIXEL_REFS = {"animation", "comic", "game", "official-art"}  # a live-action still is an actor's face: text only
# A learned upscaler redraws edges instead of blurring them, so a plate survives a larger
# enlargement: the gate's PASS/WARN limits are multiplied by this when it ran.
IMAGE_GEN = Path(os.environ.get("IMAGE_GEN_HOME") or Path.home() / ".local/share/image-gen")
ESRGAN = IMAGE_GEN / "upscalers/RealESRGAN_x4plus_anime_6B.pth"
SD_CLI = IMAGE_GEN / "sd-build/bin/sd-cli"
UPSCALERS = {"esrgan": 2.0, "blend": 1.6, "lanczos": 1.0}
BLEND = 0.65   # esrgan share in blend: keeps the redrawn edges, gives back some of the painted texture
MAX_REF = 2 << 20
REF_SIDE = 512
# Things a picture model must never be asked for: the real product is shown, never drawn.
UI = re.compile(r"\b(user\s+interface|\bui\b|dashboard|app\s+screen|screen\s*shot|screenshot|website|web\s*page|"
                r"interface\s+of|giao\s+diện|ảnh\s+chụp\s+màn\s+hình)", re.I)
TEXT = re.compile(r"[\"“][^\"”]{2,}[\"”]|\b(text|words?|caption|title|lettering|sign)\s+(that\s+)?(says|reads|reading|saying)\b|"
                  r"\bchữ\b", re.I)
LOGO = re.compile(r"\blogo\b", re.I)


def art_dir(cd: Path) -> Path:
    return cd / ART


def load_jobs(cd: Path) -> dict:
    """art.json with its `preset` (styles.json) filled under it: the file's own style lines win."""
    p = art_dir(cd) / "art.json"
    d = load(p) if p.is_file() else {"jobs": []}
    if "preset" not in d and campaign_style(cd):
        d["preset"] = campaign_style(cd)
    pre = (STYLES.get(d.get("preset")) or {}).get("art") or {}
    for k in ("style", "cutout_style", "sticker"):
        if pre.get(k) and not str(d.get(k) or "").strip():
            d[k] = pre[k]
    return d


def is_cutout(job: dict) -> bool:
    """Drawn to sit over another picture: a sticker, or any job asking for transparency."""
    return bool(job.get("transparent")) or (job.get("use") == "sticker")


def jobs_by_id(cd: Path) -> dict:
    return {j.get("id"): j for j in load_jobs(cd).get("jobs") or []}


# ---------- engines ----------

def image_gen_dir() -> Path:
    """The image-gen skill ships beside this one (2build); $MVF_IMAGE_GEN points at another generate-image."""
    env = os.environ.get("MVF_IMAGE_GEN")
    return Path(env).expanduser().parent if env else SKILL.parent / "image-gen" / "scripts"


def qwen_cli() -> Path:
    return image_gen_dir() / "generate-image"


def remove_bg_cli() -> Path:
    return Path(os.environ.get("MVF_REMOVE_BG") or image_gen_dir() / "remove-bg")


@functools.lru_cache(maxsize=None)
def qwen_limits(cli: str) -> dict:
    """The sizes generate-image draws on this machine (`--limits`); the small defaults when it cannot say."""
    try:
        r = subprocess.run([cli, "--limits"], capture_output=True, text=True, timeout=30)
        lim = json.loads(r.stdout.strip().splitlines()[-1])
        return {"max_pixels": int(lim["max_pixels"]), "sides": tuple(lim["sides"]), "step": int(lim["step"])}
    except (OSError, subprocess.TimeoutExpired, ValueError, KeyError, IndexError):
        return {}


def engines(cd: Path | None) -> dict:
    qwen = extensions.Image("local Qwen Image 2.1 Turbo (image-gen skill): free, about 7 min a megapixel on an M1 Pro "
                            "(2 min for 384x672, 15 for a full 1088x1920 frame), needs an idle Mac; "
                            "transparent drafts come cut out (model alpha, else RMBG-1.4)",
                            command=[str(qwen_cli())], **qwen_limits(str(qwen_cli())))
    return {"qwen": qwen, **extensions.load(cd).images}


def engine_for(cd: Path, want: str | None) -> tuple[str, extensions.Image]:
    name = want or load_jobs(cd).get("engine") or "qwen"
    reg = engines(cd)
    if name not in reg:
        raise SystemExit(f"unknown image engine {name!r}; engines: {', '.join(sorted(reg))} (`mvf extensions`)")
    return name, reg[name]


def canvas_ratio(canvas: str) -> float:
    c = PLATFORMS["canvases"].get(canvas)
    if not c:
        raise SystemExit(f"canvas {canvas!r} not in {sorted(PLATFORMS['canvases'])}")
    return c["width"] / c["height"]


def draw_size(ratio: float, eng: extensions.Image, shown: tuple[int, int] | None = None) -> tuple[int, int]:
    """The size to draw at this aspect (within 3 %), in the engine's step and pixel budget: the smallest that
    covers `shown` without enlarging, else the largest the engine draws."""
    lo, hi = eng.sides
    fits = [(w, h) for w in range(lo, hi + 1, eng.step) for h in range(lo, hi + 1, eng.step)
            if w * h <= eng.max_pixels and abs(w / h - ratio) / ratio <= 0.03]
    if not fits:
        raise SystemExit(f"the engine cannot draw aspect {ratio:.3f} inside {eng.sides} px")
    covers = [s for s in fits if shown and s[0] >= shown[0] and s[1] >= shown[1]]
    return min(covers, key=lambda s: s[0] * s[1]) if covers else max(fits, key=lambda s: s[0] * s[1])


def job_draw_size(job: dict, eng: extensions.Image) -> tuple[int, int]:
    """A job's own `draw` size when it names one (the agent's trade of time for sharpness), else `draw_size`."""
    if job.get("draw"):
        try:
            w, h = (int(v) for v in str(job["draw"]).lower().split("x"))
        except ValueError:
            raise SystemExit(f"{job.get('id')}: draw {job['draw']!r} is not WxH")
        lo, hi = eng.sides
        if not all(lo <= v <= hi and v % eng.step == 0 for v in (w, h)) or w * h > eng.max_pixels:
            raise SystemExit(f"{job.get('id')}: draw {w}x{h} is outside what the engine draws here (multiples of {eng.step}, "
                             f"sides {lo}-{hi}, at most {eng.max_pixels} px)")
        return w, h
    use = USES[job.get("use") or "plate"]
    shown = display_size(job)
    if shown:   # a use that passes soft (a background at 3x) is drawn that much smaller: time, not sharpness, is the cost
        k = use[1] / 1.5
        want = (math.ceil(shown[0] / k), math.ceil(shown[1] / k))
    else:
        want = REF_SIDE, REF_SIDE   # a reference is never shown, and --ref takes at most MAX_REF bytes
    return draw_size(canvas_ratio(job.get("canvas") or use[0]), eng, want)


def display_size(job: dict) -> tuple[int, int] | None:
    """The pixels the picture fills on screen: `display`, else the job canvas at delivery size."""
    use = USES.get(job.get("use") or "plate")
    if not use or use[1] is None:
        return None
    if job.get("display"):
        w, h = (int(v) for v in str(job["display"]).lower().split("x"))
        return w, h
    c = PLATFORMS["canvases"][job.get("canvas") or use[0]]
    return c["width"], c["height"]


def upscale(drawn: tuple[int, int], shown: tuple[int, int]) -> float:
    """Cover-fit factor: how much the drawn picture is enlarged to fill what is shown."""
    return round(max(shown[0] / drawn[0], shown[1] / drawn[1]), 2)


def verdict(job: dict, factor: float, upscaler: str = "lanczos") -> str:
    _, ok, warn = USES[job.get("use") or "plate"]
    k = UPSCALERS.get(upscaler, 1.0)
    return "PASS" if factor <= ok * k else "WARN" if factor <= warn * k else "FAIL"


def esrgan_ready() -> bool:
    return ESRGAN.is_file() and SD_CLI.is_file()


def pick_upscaler(job: dict, want: str | None) -> str:
    """CLI > job > MVF_UPSCALER > by look. esrgan's weights are an anime/illustration model: a
    photoreal job defaults to lanczos and must ask for it."""
    u = (want or job.get("upscaler") or os.environ.get("MVF_UPSCALER")
         or ("esrgan" if job.get("look", "illustration") == "illustration" else "lanczos"))
    if u not in UPSCALERS:
        raise SystemExit(f"upscaler {u!r}: one of {sorted(UPSCALERS)}")
    if u != "lanczos" and not esrgan_ready():
        return "lanczos"
    return u


def _esrgan(cd: Path, src: Path, out: Path) -> None:
    r = subprocess.run([*heavy_prefix(cd), str(SD_CLI), "-M", "upscale", "--upscale-model", str(ESRGAN), "-i", str(src),
                        "-o", str(out)], capture_output=True, text=True)
    if r.returncode or not out.is_file():
        raise SystemExit(f"esrgan upscale failed: {(r.stderr or r.stdout).strip()[-600:]}")


def image_size(path: Path) -> tuple[int, int]:
    """(width, height) of a PNG or JPEG, without Pillow."""
    b = Path(path).read_bytes()
    if b[:8] == b"\x89PNG\r\n\x1a\n":
        return struct.unpack(">II", b[16:24])
    i = 2
    while i < len(b) - 9:
        if b[i] != 0xFF:
            i += 1
            continue
        marker, seg = b[i + 1], struct.unpack(">H", b[i + 2:i + 4])[0]
        if marker in (0xC0, 0xC1, 0xC2):
            h, w = struct.unpack(">HH", b[i + 5:i + 9])
            return w, h
        i += 2 + seg
    raise SystemExit(f"{path}: not a PNG or JPEG")


# ---------- checks ----------

def check(cd: Path, persons: list[str] | None = None) -> Report:
    """The jobs file: ids, uses, sizes, refs, and prompts that ask for what a video may never show."""
    r = Report()
    d = load_jobs(cd)
    jobs = d.get("jobs") or []
    cast = _check_cast(cd, d, r)
    ids = set()
    for j in jobs:
        jid = j.get("id")
        where = f"art.{jid}"
        for c in j.get("cast") or []:
            if c not in cast:
                r.fail("A10", where, f"cast {c!r} is not in art.json cast")
        if not isinstance(jid, str) or not ID.match(jid) or jid in ids:
            r.fail("A0", where, "id must be unique, a letter then letters/digits/_- (≤ 24)")
        ids.add(jid)
        use = j.get("use") or "plate"
        if use not in USES:
            r.fail("A0", where, f"use {use!r} not in {sorted(USES)}")
            continue
        if j.get("look", "illustration") not in LOOKS:
            r.warn("A0", where, f"look {j.get('look')!r} is not one mvf knows ({', '.join(sorted(LOOKS))}): it is passed "
                                f"through and upscaled with lanczos")
        if (j.get("canvas") or USES[use][0]) not in PLATFORMS["canvases"]:
            r.fail("A0", where, f"canvas {j.get('canvas')!r} not in {sorted(PLATFORMS['canvases'])}")
        if use in {"inset", "sticker"} and not re.match(r"^\d+x\d+$", str(j.get("display", ""))):
            r.fail("A0", where, f"a {use} needs display: \"WxH\", the pixels it fills on screen")
        if j.get("image"):
            if not (cd / str(j["image"])).is_file():
                r.fail("A1", where, f"image {j['image']!r} is not a file in the campaign")
            if j.get("prompt"):
                r.warn("A1", where, "has both image and prompt: an image job is cut out, not drawn; the prompt is ignored")
            if use == "sticker" and not j.get("transparent"):
                r.warn("A5", where, "a sticker is cut out: set transparent: true (RMBG-1.4 removes the background)")
            continue
        prompt = str(j.get("prompt", ""))
        if not prompt.strip():
            r.fail("A1", where, "prompt is empty (or name an existing picture in image)")
        if UI.search(prompt):
            r.fail("A2", where, f"{UI.search(prompt).group(0)!r}: never draw a product UI — composite the real screen "
                   "(01-product/screens); reword if this is not a UI")
        for name in persons or []:
            if name and re.search(r"(?<!\w)" + re.escape(name) + r"(?!\w)", prompt, re.I):
                r.fail("A3", where, f"names the real person {name!r}: no AI likeness of a real person, never by name")
        if TEXT.search(prompt):
            r.warn("A4", where, "asks for text in the picture: the model misspells it (diacritics most); put words in the "
                   "scene's HTML and prompt 'no text'")
        if LOGO.search(prompt):
            r.warn("A4", where, "a drawn logo is a fake logo: place the real one from brand/ in the scene")
        if use == "sticker" and not j.get("transparent") and d.get("sticker") != "multiply":
            r.warn("A5", where, "a sticker is cut out: set transparent: true (the model's own alpha), or draw it as line "
                   "art on white and show it with blend: multiply (preset stickman)")
        if is_cutout(j) and "style" not in j and not str(d.get("cutout_style", "")).strip() and str(d.get("style", "")).strip():
            r.warn("A9", where, "a cut-out takes the shared style, which describes a whole picture: set cutout_style (or "
                   "the job's own style: the same look and light, a flat plain background, no scenery), or a preset")
        ref = j.get("ref")
        if ref and ref not in {x.get("id") for x in jobs} and not (cd / str(ref)).is_file():
            r.fail("A6", where, f"ref {ref!r} is neither a job id nor a file in the campaign")
    rp = cd / "03-research/research.json"
    seen = [x.get("title") for x in (load(rp).get("adaptations") or [])] if rp.is_file() else []
    if seen and not cast and any(not j.get("image") for j in jobs):
        r.warn("A10", "art.cast", f"the audience already knows a look from {seen}: describe each recurring character's "
               "look from it in art.json cast, and list them on the jobs that draw them (cast: [id])")
    if d.get("preset") and d["preset"] != "none" and d["preset"] not in STYLES:
        r.fail("A0", "art.preset", f"preset {d['preset']!r} not in {sorted(STYLES)} (or drop it and write style yourself)")
    if jobs and not str(d.get("style", "")).strip():
        r.info("A7", "art", "no style: one shared style line keeps every plate in one look")
    full = [j.get("id") for j in jobs if (j.get("use") or "plate") in FULL_FRAME and not j.get("image")]
    if len(full) > MAX_FULL_FRAME:
        r.warn("A12", "art", f"{len(full)} generated full-frame pictures ({', '.join(map(str, full))}): scenes built on them "
               "play as a slideshow. Draw the place in code and keep art to textures, far layers, portraits and props the "
               "code moves, or answer in warn_notes why this video needs them")
    return r


# ---------- draw ----------

def drafts(cd: Path, jid: str) -> list[Path]:
    d = art_dir(cd) / "draft"
    return sorted(d.glob(f"{jid}-s*.png"), key=lambda p: p.stat().st_mtime) if d.is_dir() else []


def approved(cd: Path, jid: str) -> dict | None:
    rec = art_dir(cd) / f"{jid}.json"
    return load(rec) if rec.is_file() and (art_dir(cd) / f"{jid}.png").is_file() else None


def ref_path(cd: Path, ref: str) -> Path:
    """A job id gives the draft its approved picture came from (small, as drawn), else its newest draft."""
    a = approved(cd, ref)
    if a and (cd / a["from"]).is_file():
        return cd / a["from"]
    ds = drafts(cd, ref)
    if ds:
        return ds[-1]
    p = cd / ref
    if p.is_file():
        return p
    raise SystemExit(f"ref {ref}: nothing drawn yet (draw {ref} first)")


def _check_cast(cd: Path, d: dict, r: Report) -> dict:
    """Each recurring character's look, taken from the adaptation the audience saw when there is one."""
    cast = d.get("cast") or {}
    for cid, c in cast.items():
        where = f"art.cast.{cid}"
        src = c.get("source") or {}
        kind = src.get("kind")
        if kind not in CAST_SOURCES:
            r.fail("A10", where, f"source.kind {kind!r} not in {sorted(CAST_SOURCES)}: where viewers met this "
                   "character's look (the donghua, the drama, the comic), or none")
            continue
        if len(str(c.get("design", "")).strip()) < 20:
            r.fail("A10", where, "design: the look as the adaptation shows it (face shape, hair, eyes, costume, colours, "
                   "props), written so every prompt that draws this character carries it")
        if kind != "none" and not (str(src.get("title", "")).strip() and str(src.get("url", "")).startswith("http")):
            r.fail("A10", where, "source.title and source.url: the adaptation and the page the design was read from")
        img = src.get("image")
        if img and not (cd / img).is_file():
            r.fail("A10", where, f"source.image {img} is not a file in the campaign")
        if kind == "live-action":
            if img:
                r.fail("A11", where, "a still of a live-action character is an actor's face: never pass it to the "
                       "image model; keep the costume, hair and colours in design, and drop source.image")
            r.info("A11", where, "live-action: the design takes the costume, hair, colours and props, never the actor's "
                   "face or name")
        elif kind in PIXEL_REFS and not img:
            r.warn("A10", where, f"no source.image: put one clear still of {cid} from {src.get('title') or 'the adaptation'}"
                   " in 07-video/art/cast/ so the drawings match what viewers remember (a reference, never shown)")
    return cast


def cast_ref(cd: Path, job: dict, cast: dict) -> str | None:
    """A job's reference pixels: its own ref, else the first cast member's adaptation still."""
    if job.get("ref"):
        return job["ref"]
    for c in job.get("cast") or []:
        src = (cast.get(c) or {}).get("source") or {}
        if src.get("kind") in PIXEL_REFS and src.get("image"):
            return src["image"]
    return None


def full_prompt(job: dict, style: str, cast: dict | None = None, cutout_style: str = "") -> str:
    p = str(job["prompt"]).strip().rstrip(".")
    looks = [str((cast or {}).get(c, {}).get("design", "")).strip().rstrip(".") for c in job.get("cast") or []]
    looks = [x for x in looks if x]
    if looks:
        p += ". Character look: " + "; ".join(looks)
    if "style" in job:
        style = job["style"]
    elif cutout_style and is_cutout(job):
        style = cutout_style  # the shared style paints a whole scene; a cut-out wants its subject alone
    style = style or ""
    return f"{p}. {style.strip()}" if style.strip() else p


def draw(cd: Path, ids: list[str], want_engine: str | None, seed: int | None, spend: bool, persons: list[str]) -> Report:
    rep = check(cd, persons)
    bad = {i["where"].split(".", 1)[1] for i in rep.items if i["level"] == "FAIL"}
    d = load_jobs(cd)
    jobs = {j["id"]: j for j in d.get("jobs") or [] if j.get("id")}
    todo = ids or [j for j in jobs if not drafts(cd, j)]
    # references first: the jobs that --ref them need their pixels
    todo = sorted(todo, key=lambda j: (jobs.get(j, {}).get("use") != "reference", todo.index(j)))
    for jid in todo:
        if jid not in jobs:
            raise SystemExit(f"no art job {jid} in {art_dir(cd) / 'art.json'}")
        if jid in bad:
            rep.fail("A9", f"art.{jid}", "not drawn: fix the FAILs above first")
            continue
        job = jobs[jid]
        if job.get("image"):
            _take(cd, job, rep)
            continue
        name, eng = engine_for(cd, want_engine or job.get("engine"))
        if eng.paid and not spend:
            raise SystemExit(f"engine {name} costs money: run with --spend only when the user said yes to this spend")
        missing = [e for e in eng.needs if not os.environ.get(e)]
        if missing:
            raise SystemExit(f"engine {name} needs {', '.join(missing)} in the environment")
        if job.get("transparent") and not eng.transparent:
            raise SystemExit(f"engine {name} cannot draw transparency ({jid})")
        s = int(seed if seed is not None else job.get("seed", 1))
        out = art_dir(cd) / "draft" / f"{jid}-s{s}.png"
        if out.exists():
            rep.info("A9", f"art.{jid}", f"{out.name} exists; a redraw is a new seed (--seed)")
            continue
        out.parent.mkdir(parents=True, exist_ok=True)
        w, h = job_draw_size(job, eng)
        ref = None
        want = cast_ref(cd, job, d.get("cast") or {})
        if want and not eng.refs:
            if job.get("ref"):
                raise SystemExit(f"engine {name} cannot take a reference ({jid})")
            rep.warn("A10", f"art.{jid}", f"engine {name} takes no reference image: the cast design goes in as text only")
            want = None
        if want:
            ref = ref_path(cd, want)
            if ref.stat().st_size > MAX_REF:
                raise SystemExit(f"ref {ref} is over 2 MiB: approve a smaller draft or pass a JPEG")
        req = {"prompt": full_prompt(job, d.get("style", ""), d.get("cast"), d.get("cutout_style", "")), "size": f"{w}x{h}", "seed": s,
               "ref": str(ref) if ref else None, "transparent": bool(job.get("transparent"))}
        print(f"drawing {jid} ({req['size']}, seed {s}) with {name}…", flush=True)
        t0 = dt.datetime.now()
        receipt = _run(cd, eng, req, out)
        if not out.is_file():
            raise SystemExit(f"engine {name} wrote no {out}")
        size = image_size(out)
        rec = {"id": jid, "engine": name, **req, "drawn": f"{size[0]}x{size[1]}",
               "seconds": receipt.get("seconds") or round((dt.datetime.now() - t0).total_seconds(), 1),
               "cost_usd": receipt.get("cost_usd", 0.0 if not eng.paid else None),
               "sha256": hashlib.sha256(out.read_bytes()).hexdigest(), "at": t0.isoformat(timespec="seconds"),
               "cutout": receipt.get("cutout")}
        dump(out.with_suffix(".json"), rec)
        shown = display_size(job)
        if shown:
            f = upscale(size, shown)
            v = verdict(job, f, pick_upscaler(job, None))  # the gate itself is `approve`; a draft is only a candidate
            (rep.info if v == "PASS" else rep.warn)(
                "A8", f"art.{jid}", f"{out.name}: {size[0]}x{size[1]} shown at {shown[0]}x{shown[1]} = {f}x upscale "
                f"({v} for a {job.get('use') or 'plate'}" + ("; approve refuses it" if v == "FAIL" else "")
                + "); look at it before approving")
        else:
            rep.info("A8", f"art.{jid}", f"{out.name}: {size[0]}x{size[1]} reference; look at it before others ref it")
    return rep


def _take(cd: Path, job: dict, rep: Report) -> None:
    """An image job's draft: the campaign's own picture, cut out with RMBG-1.4 when transparent."""
    jid, src = job["id"], cd / job["image"]
    out = art_dir(cd) / "draft" / f"{jid}-s0.png"
    if out.exists():
        rep.info("A9", f"art.{jid}", f"{out.name} exists (an image job has one draft; delete it to cut again)")
        return
    out.parent.mkdir(parents=True, exist_ok=True)
    t0 = dt.datetime.now()
    if job.get("transparent"):
        cmd = [*heavy_prefix(cd), str(remove_bg_cli()), str(src), "-o", str(out)]
        print(f"cutting out {jid} ({job['image']}) with RMBG-1.4…", flush=True)
        try:
            p = subprocess.run(cmd, capture_output=True, text=True)
        except FileNotFoundError:
            raise SystemExit(f"remove-bg not found ({remove_bg_cli()}): install the image-gen skill and run its setup")
        if p.returncode or not out.is_file():
            raise SystemExit(f"remove-bg failed on {src}: {(p.stderr or p.stdout).strip()[-600:]}")
        cut = "rmbg-1.4"
    else:
        subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(src), str(out)], check=True)
        cut = None
    size = image_size(out)
    dump(out.with_suffix(".json"), {"id": jid, "engine": "image", "image": job["image"], "drawn": f"{size[0]}x{size[1]}",
                                    "cutout": cut, "cost_usd": 0.0, "seconds": round((dt.datetime.now() - t0).total_seconds(), 1),
                                    "sha256": hashlib.sha256(out.read_bytes()).hexdigest(), "at": t0.isoformat(timespec="seconds")})
    if cut:
        clear = clear_share(out)
        (rep.info if MIN_CLEAR <= clear <= 0.97 else rep.warn)(
            "A9", f"art.{jid}", f"{out.name}: {clear:.0%} cleared" + ("" if MIN_CLEAR <= clear <= 0.97 else
            ": RMBG found no clear subject; use a photo with one subject on a plain background") + "; look at its edges")
    else:
        rep.info("A9", f"art.{jid}", f"{out.name}: {size[0]}x{size[1]} taken as is")


def _run(cd: Path, eng: extensions.Image, req: dict, out: Path) -> dict:
    if eng.run:
        return eng.run(req, out) or {}
    cmd = [*(heavy_prefix(cd) if eng.heavy else []), *eng.command, req["prompt"], "-o", str(out),
           "--size", req["size"], "--seed", str(req["seed"])]
    if req["ref"]:
        cmd += ["--ref", req["ref"]]
    if req["transparent"]:
        cmd.append("--transparent")
    try:
        p = subprocess.run(cmd, capture_output=True, text=True)
    except FileNotFoundError as e:
        raise SystemExit(f"image engine not found: {e.filename} (qwen: the image-gen skill and its setup, or $MVF_IMAGE_GEN)")
    if p.returncode:
        raise SystemExit(f"image engine failed ({p.returncode}): {(p.stderr or p.stdout).strip()[-800:]}")
    for line in reversed(p.stdout.strip().splitlines()):
        try:
            return json.loads(line)
        except json.JSONDecodeError:
            continue
    return {}


# ---------- approve ----------

def approve(cd: Path, jid: str, draft: str | None, focus: tuple[float, float], force: bool,
            upscaler: str | None = None) -> dict:
    jobs = jobs_by_id(cd)
    if jid not in jobs:
        raise SystemExit(f"no art job {jid}")
    job = jobs[jid]
    src = (cd / draft if not Path(draft).is_absolute() else Path(draft)) if draft else (drafts(cd, jid) or [None])[-1]
    if not src or not Path(src).is_file():
        raise SystemExit(f"{jid}: no draft to approve (mvf art draw {jid})")
    src = Path(src).resolve()
    out = art_dir(cd) / f"{jid}.png"
    drawn = image_size(src)
    shown = display_size(job)
    receipt = load(src.with_suffix(".json")) if src.with_suffix(".json").is_file() else {}
    rec = {"id": jid, "from": str(src.relative_to(cd.resolve()) if src.is_relative_to(cd.resolve()) else src),
           "use": job.get("use") or "plate",
           "look": job.get("look", "illustration"), "drawn": f"{drawn[0]}x{drawn[1]}",
           "generated": not job.get("image"), "cutout": receipt.get("cutout")}
    if shown is None:  # a reference keeps its native pixels
        out.write_bytes(src.read_bytes())
        dump(out.with_suffix(".json"), {**rec, "verdict": "REF"})
        return {**rec, "verdict": "REF"}
    f = upscale(drawn, shown)
    up = pick_upscaler(job, upscaler) if f > 1.05 else "lanczos"
    v = verdict(job, f, up)
    if v == "FAIL" and not force:
        raise SystemExit(f"{jid}: {drawn[0]}x{drawn[1]} shown at {shown[0]}x{shown[1]} is a {f}x upscale: too soft for a "
                         f"{rec['use']}. Use it as a background (dimmed/blurred), show it smaller (display), draw it on "
                         "a bigger engine, or --force after looking at it at full size")
    W, H = shown
    tmp = []
    if up != "lanczos" and not job.get("transparent"):
        big = src.parent / f".{src.stem}.esrgan.png"
        _esrgan(cd, src, big)
        tmp.append(big)
        if up == "blend":
            soft = src.parent / f".{src.stem}.lanczos.png"
            bw, bh = image_size(big)
            r = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(src), "-i", str(big),
                                "-filter_complex", f"[0]scale={bw}:{bh}:flags=lanczos[a];[a][1]blend=all_expr="
                                f"'A*{1 - BLEND:.2f}+B*{BLEND:.2f}'", str(soft)], capture_output=True, text=True)
            if r.returncode:
                raise SystemExit(f"blend {jid}: {r.stderr.strip()[-400:]}")
            tmp.append(soft)
            big = soft
        src, drawn = big, image_size(big)
        rec["upscaler"] = up
    else:
        rec["upscaler"] = "lanczos"
    k = max(W / drawn[0], H / drawn[1])
    sw, sh = max(W, math.ceil(drawn[0] * k)), max(H, math.ceil(drawn[1] * k))
    fx, fy = focus
    vf = [f"scale={sw}:{sh}:flags=lanczos", f"crop={W}:{H}:{round((sw - W) * fx)}:{round((sh - H) * fy)}"]
    alpha = bool(job.get("transparent"))
    if not alpha:
        if rec["upscaler"] == "lanczos":
            vf.append("unsharp=5:5:0.6:5:5:0.0")
    elif clear_share(src) < MIN_CLEAR:
        # An engine that returned an opaque "transparent" draft (qwen cuts its own with RMBG-1.4; another engine
        # may not): clear the flat background the prompt asked for, with no matting model.
        keyed = src.parent / f".{src.stem}.keyed.png"
        rec["keyed"] = "#" + cutout(src, keyed)
        src = keyed
    r = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(src), "-vf", ",".join(vf),
                        "-pix_fmt", "rgba" if alpha else "rgb24", str(out)], capture_output=True, text=True)
    if r.returncode:
        raise SystemExit(f"approve {jid}: ffmpeg failed: {r.stderr.strip()[-600:]}")
    if rec.get("keyed"):
        src.unlink()
    for t in tmp:
        t.unlink(missing_ok=True)
    if alpha:
        rec["clear"] = round(clear_share(out), 3)
        if rec["clear"] < MIN_CLEAR:
            out.unlink()
            raise SystemExit(f"{jid}: no usable cutout ({rec['clear']:.0%} clear): redraw on a plainer, flat background "
                             "colour far from the subject's colours (--seed), or drop transparent")
    rec.update(shown=f"{W}x{H}", upscale=f, verdict=v, forced=v == "FAIL", focus=list(focus))
    dump(out.with_suffix(".json"), rec)
    return rec


MIN_CLEAR = 0.05  # a cutout with under 5 % clear pixels is an opaque picture


def _raw(path: Path, fmt: str) -> tuple[bytes, int, int]:
    w, h = image_size(path)
    r = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-i", str(path), "-f", "rawvideo", "-pix_fmt", fmt, "-"],
                       capture_output=True, check=True)
    return r.stdout, w, h


def clear_share(path: Path) -> float:
    """Share of pixels at least half transparent."""
    b, w, h = _raw(path, "rgba")
    return sum(1 for i in range(3, len(b), 4) if b[i] < 128) / (w * h)


def corner_color(path: Path, n: int = 8) -> str:
    """Mean colour of the four n x n corners: the flat background a cutout prompt asked for."""
    b, w, h = _raw(path, "rgb24")
    acc, k = [0, 0, 0], 0
    for y0, x0 in ((0, 0), (0, w - n), (h - n, 0), (h - n, w - n)):
        for y in range(y0, y0 + n):
            for x in range(x0, x0 + n):
                i = (y * w + x) * 3
                acc = [acc[c] + b[i + c] for c in range(3)]
                k += 1
    return "".join(f"{round(v / k):02x}" for v in acc)


def cutout(src: Path, out: Path, lo: int = 40, hi: int = 90) -> str:
    """Clear the background connected to the border whose colour is near the corners' (flood fill, so a
    subject's own patches of that colour survive), with an alpha ramp from distance lo to hi; returns the key."""
    b, w, h = _raw(src, "rgb24")
    key = corner_color(src)
    kr, kg, kb = (int(key[i:i + 2], 16) for i in (0, 2, 4))
    dist = [max(abs(b[i] - kr), abs(b[i + 1] - kg), abs(b[i + 2] - kb)) for i in range(0, len(b), 3)]
    bg = bytearray(w * h)
    stack = [i for i in range(w)] + [(h - 1) * w + i for i in range(w)] + [y * w for y in range(h)] + [y * w + w - 1 for y in range(h)]
    while stack:
        i = stack.pop()
        if bg[i] or dist[i] >= hi:
            continue
        bg[i] = 1
        x, y = i % w, i // w
        stack += [j for j, ok in ((i - 1, x > 0), (i + 1, x < w - 1), (i - w, y > 0), (i + w, y < h - 1)) if ok]
    rgba = bytearray(w * h * 4)
    green = kg > kr + 30 and kg > kb + 30
    for i in range(w * h):
        r, g, bl = b[3 * i], b[3 * i + 1], b[3 * i + 2]
        a = 255 if not bg[i] else max(0, min(255, round((dist[i] - lo) * 255 / (hi - lo))))
        if bg[i] and green:
            g = min(g, max(r, bl))  # despill the fringe
        rgba[4 * i:4 * i + 4] = bytes((r, g, bl, a))
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "rawvideo", "-pix_fmt", "rgba", "-s",
                    f"{w}x{h}", "-i", "-", str(out)], input=bytes(rgba), check=True)
    return key


# ---------- status and disclosure ----------

def status(cd: Path) -> list[dict]:
    out = []
    for jid, job in jobs_by_id(cd).items():
        a = approved(cd, jid)
        ds = drafts(cd, jid)
        out.append({"id": jid, "use": job.get("use") or "plate", "look": job.get("look", "illustration"),
                    "drafts": len(ds), "approved": bool(a), "verdict": (a or {}).get("verdict"),
                    "upscale": (a or {}).get("upscale")})
    return out


def used_in(proj: Path) -> set[str]:
    """Art ids a scaffolded project's scenes actually load (assets/art/<ID>.png)."""
    ids = set()
    for p in (proj / "compositions").glob("*.html"):
        ids |= set(re.findall(r"assets/art/([A-Za-z][A-Za-z0-9_-]{0,23})\.png", p.read_text(encoding="utf-8")))
    return ids


def rmbg_shown(cd: Path, proj: Path, script: dict) -> list[str]:
    """Approved art on screen whose background RMBG-1.4 removed (a cut image job, or a qwen draft it cut)."""
    ids = used_in(proj) | {a for b in script.get("beats") or [] for a in b.get("art") or []}
    return sorted(i for i in ids if (approved(cd, i) or {}).get("cutout") == "rmbg-1.4")


def disclosure(cd: Path, proj: Path, script: dict) -> dict:
    """Whether the upload must carry the platform's AI label, from what is on screen and who speaks.

    TikTok and YouTube require a label for REALISTIC generated or altered pictures, sound or video;
    non-realistic art (illustration, texture) and production help (thumbnails, captions) are exempt
    on YouTube. A synthetic narrator voice is realistic audio: label it on TikTok."""
    jobs = jobs_by_id(cd)
    shown = sorted(i for i in used_in(proj) | {a for b in script.get("beats") or [] for a in b.get("art") or []}
                   if i in jobs and not jobs[i].get("image"))  # an image job is the campaign's own picture, cut out
    real = [i for i in shown if jobs[i].get("look") == "photoreal"]
    voice = (script.get("voice") or {}).get("mode")
    synthetic_voice = voice in {"clone", "design", "preset"}
    why = [f"photoreal generated art {', '.join(real)}"] if real else []
    if synthetic_voice:
        why.append(f"synthetic voice ({voice})")
    # Meta asks for disclosure of photorealistic video and realistic-sounding audio made or altered digitally.
    meta = "ON" if (real or synthetic_voice) else "off"
    return {"art": shown, "photoreal": real, "synthetic_voice": synthetic_voice,
            "tiktok": "ON" if (real or synthetic_voice) else ("recommended" if shown else "off"),
            "youtube": "ON" if real else "off", "instagram": meta, "facebook": meta, "why": why}
