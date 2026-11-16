"""The library: what one video leaves for the next, kept apart from the campaign folders so those can be deleted.
It lives in <repo>/.babysit/marketing-video/library/ (gitignored; $MVF_LIBRARY moves it):

  history.jsonl          every angle and script this owner has had, one line each: lint RP1, `mvf overlap`, `mvf library`
  products/<product>/    the product pack: campaign defaults, 01-product (facts, screens, captures), 02-audience,
                         03-trends, brand
  subjects/<subject>/    00-material on one subject (a novel, a feature), merged by item id
  formats/<name>/        saved series formats (`mvf format`)
  code/<campaign>/<sid>/ a video's own production code (07-video/<sid>/src/), for the next one to copy from
  saved.json             which campaign was saved when

`mvf export` (final) and `mvf library save` save a campaign into it, `mvf approve` records its scripts, `mvf init`
starts from it, and `mvf library check <dir>` says whether a campaigns folder can be deleted."""
from __future__ import annotations

import datetime
import json
import os
import re
import shutil
import subprocess
import sys
import unicodedata
from pathlib import Path

from .common import dump, load, local_dir, today

HISTORY = "history.jsonl"
SAVED = "saved.json"
DEFAULTS = ("brand", "markets", "languages", "platforms", "voice_engine", "known_words")  # campaign.json keys a product keeps
SNAPSHOT = ("01-product/facts.json", "02-audience", "03-trends")  # fact ids repeat across campaigns: the newest copy wins
UNION = ("01-product/screens", "01-product/captures", "brand")  # files add up; items.json merges by id
MERGED = {"items.json", "index.json"}
MEDIA = {".mp4", ".mov", ".webm", ".mp3", ".wav", ".m4a", ".png", ".jpg", ".jpeg", ".webp", ".gif"}


def root(start) -> Path:
    if os.environ.get("MVF_LIBRARY"):
        return Path(os.environ["MVF_LIBRARY"])
    p = Path(start).resolve()
    if (p / "campaign.json").is_file():
        p = p.parent
    return local_dir(p) / "library"


def slug(name) -> str:
    folded = unicodedata.normalize("NFKD", str(name or "").lower().replace("đ", "d"))
    s = re.sub(r"[^a-z0-9]+", "-", "".join(ch for ch in folded if not unicodedata.combining(ch))).strip("-")
    if not s:
        raise SystemExit(f"{name!r}: not a usable name")
    return s


def _json(p: Path):
    try:
        return load(p)
    except (ValueError, OSError):
        return None


def _template(p: Path) -> bool:
    """A stage still holding the template is nothing to keep."""
    fs = [p] if p.is_file() else sorted(p.glob("*.json")) if p.is_dir() else []
    return any(isinstance(d := _json(f), dict) and d.get("_template") for f in fs)


def _merge(src: Path, dst: Path) -> None:
    """Items keyed by id, as a list or as {"items": {id: item}}: the newer copy of an id wins, the rest stay."""
    new, old = _json(src), _json(dst)
    if isinstance(new, list) and isinstance(old, list):
        ids = {x.get("id") for x in new if isinstance(x, dict)}
        dump(dst, [x for x in old if not (isinstance(x, dict) and x.get("id") in ids)] + new)
    elif isinstance(new, dict) and isinstance(old, dict) and isinstance(new.get("items"), dict) \
            and isinstance(old.get("items"), dict):
        dump(dst, {**old, **new, "items": {**old["items"], **new["items"]}})
    else:
        shutil.copy2(src, dst)


def _put(src: Path, dst: Path) -> None:
    dst.parent.mkdir(parents=True, exist_ok=True)
    if dst.exists():
        if os.path.samefile(src, dst):
            return
        if dst.name in MERGED:
            return _merge(src, dst)
        dst.unlink()
    if src.suffix.lower() in MEDIA:  # a copy-on-write clone costs no disk until one side changes
        flag = "-c" if sys.platform == "darwin" else "--reflink=auto"
        if subprocess.run(["cp", flag, "-p", str(src), str(dst)], capture_output=True).returncode == 0:
            return
    shutil.copy2(src, dst)


def _tree(src: Path, dst: Path, replace: bool = False) -> int:
    if not src.exists():
        return 0
    if src.is_file():
        if dst.exists() and replace:
            dst.unlink()
        _put(src, dst)
        return 1
    if replace and dst.is_dir():
        shutil.rmtree(dst)
    n = 0
    for f in sorted(src.rglob("*")):
        if f.is_file() and not f.name.startswith("."):
            _put(f, dst / f.relative_to(src))
            n += 1
    return n


def _rows(lib: Path) -> list[dict]:
    p = lib / HISTORY
    out = []
    for line in p.read_text(encoding="utf-8").splitlines() if p.is_file() else []:
        try:
            r = json.loads(line)
        except ValueError:
            continue
        if isinstance(r, dict):
            out.append(r)
    return out


def _day(p: Path) -> str:
    """When this was last written: newest-first lists stay true for campaigns saved long after."""
    return datetime.date.fromtimestamp(p.stat().st_mtime).isoformat()


def history(start, exclude: str | None = None) -> list[dict]:
    return [r for r in _rows(root(start)) if r.get("campaign") != exclude]


def record(cd: Path) -> int:
    """Replace this campaign's lines in history.jsonl with its angles and scripts as they are now."""
    c = _json(cd / "campaign.json") or {}
    base = {"campaign": cd.name, "product": c.get("product")}
    rows = []
    ap = cd / "04-strategy/angles.json"
    angles = (_json(ap) or {}).get("angles") if not _template(ap) else None
    for g in angles if isinstance(angles, list) else []:
        if isinstance(g, dict) and g.get("id"):
            rows.append({**base, "at": _day(ap), "kind": "angle", "id": g["id"], "promise": g.get("promise"),
                         "format": g.get("format")})
    for sp in sorted((cd / "05-scripts").glob("s*.json")):
        s = _json(sp)
        if not isinstance(s, dict) or s.get("_template"):
            continue
        beats = [b for b in s.get("beats") or [] if isinstance(b, dict)]
        rows.append({**base, "at": _day(sp), "kind": "script", "id": s.get("id") or sp.stem,
                     "made": (cd / "08-delivery" / sp.stem).is_dir(),
                     "payoff": (s.get("spine") or {}).get("payoff") or s.get("value"),
                     "vo": " ".join(str(b.get("vo") or "") for b in beats)})
    lib = root(cd)
    keep = [r for r in _rows(lib) if r.get("campaign") != cd.name]
    lib.mkdir(parents=True, exist_ok=True)
    tmp = lib / (HISTORY + ".tmp")
    tmp.write_text("".join(json.dumps(r, ensure_ascii=False) + "\n" for r in keep + rows), encoding="utf-8")
    tmp.replace(lib / HISTORY)
    return len(rows)


def save(cd: Path, subject: str | None = None) -> dict:
    """Keep what a later video can use: the product pack, the subject's material, the history, legacy formats."""
    c = _json(cd / "campaign.json") or {}
    lib = root(cd)
    out = {}
    if c.get("product"):
        pd = lib / "products" / slug(c["product"])
        if not c.get("_template"):
            dump(pd / "campaign.json", {**(_json(pd / "campaign.json") or {}), **{k: c[k] for k in DEFAULTS if k in c}})
        for rel in SNAPSHOT:
            if not _template(cd / rel):
                out[rel] = _tree(cd / rel, pd / rel, replace=True)
        for rel in UNION:
            out[rel] = _tree(cd / rel, pd / rel)
    sub = slug(subject or c.get("subject") or cd.name)
    out["00-material"] = _tree(cd / "00-material", lib / "subjects" / sub / "00-material")
    out["history"] = record(cd)
    for src in sorted((cd / "07-video").glob("*/src")):
        out["code"] = out.get("code", 0) + _tree(src, lib / "code" / cd.name / src.parent.name, replace=True)
    legacy = cd.parent / "_formats"
    for f in sorted(legacy.glob("*/format.json")) if legacy.is_dir() else []:
        if not (lib / "formats" / f.parent.name).exists():
            out["formats"] = out.get("formats", 0) + _tree(f.parent, lib / "formats" / f.parent.name)
    saved = _json(lib / SAVED) or {}
    saved[cd.name] = {"at": today().isoformat(), "product": c.get("product"), "subject": sub}
    dump(lib / SAVED, saved)
    return out


def seed(cd: Path, product: str, subject: str | None) -> list[str]:
    """A new campaign starts from the product pack and the subject's material, when the library has them."""
    lib = root(cd)
    done = []
    pd = lib / "products" / slug(product)
    if pd.is_dir():
        d = _json(pd / "campaign.json") or {}
        if d:
            c = load(cd / "campaign.json")
            dump(cd / "campaign.json", {**c, **{k: v for k, v in d.items() if k in DEFAULTS}})
        for rel in SNAPSHOT + UNION:
            if (pd / rel).exists():
                _tree(pd / rel, cd / rel, replace=True)
                done.append(rel)
        last = max(((v.get("at", ""), k) for k, v in (_json(lib / SAVED) or {}).items()
                    if isinstance(v, dict) and v.get("product") == product), default=None)
        done[:0] = [f"product {product}" + (f" (last saved from {last[1]} on {last[0]})" if last else "")]
    if subject:
        md = lib / "subjects" / slug(subject) / "00-material"
        if md.is_dir():
            _tree(md, cd / "00-material", replace=True)
            done.append(f"00-material of {slug(subject)}")
    return done


def _size(p: Path) -> int:
    return sum(f.stat().st_size for f in p.rglob("*") if f.is_file() and not f.is_symlink()) if p.is_dir() else 0


def summary(start, cd: Path | None = None, limit: int = 40) -> str:
    lib = root(start)
    c = (_json(cd / "campaign.json") or {}) if cd else {}
    lines = [f"library {lib}"]
    for pd in sorted((lib / "products").glob("*")) if (lib / "products").is_dir() else []:
        facts = (_json(pd / "01-product/facts.json") or {}).get("facts") or []
        dates = sorted(str(f.get("checked")) for f in facts if isinstance(f, dict) and f.get("checked"))
        caps = _json(pd / "01-product/captures/items.json") or []
        lines.append(f"product {pd.name}: {len(facts)} facts" + (f" (checked {dates[0]} … {dates[-1]})" if dates else "")
                     + f", {len(caps) if isinstance(caps, list) else 0} captures, segments "
                     + ("yes" if (pd / "02-audience").is_dir() else "no") + ", trends "
                     + ("yes" if (pd / "03-trends").is_dir() else "no"))
    subs = []
    for sd in sorted((lib / "subjects").glob("*")) if (lib / "subjects").is_dir() else []:
        items = (_json(sd / "00-material/index.json") or {}).get("items") or {}
        subs.append(f"{sd.name} ({len(items)})")
    if subs:
        lines.append("subjects (material items): " + ", ".join(subs))
    fm = sorted(p.parent.name for p in (lib / "formats").glob("*/format.json")) if (lib / "formats").is_dir() else []
    if fm:
        lines.append("formats: " + ", ".join(fm))
    code = sorted(p.relative_to(lib / "code").as_posix() for p in (lib / "code").glob("*/*") if p.is_dir()) \
        if (lib / "code").is_dir() else []
    if code:
        lines.append(f"production code in {lib / 'code'}: " + ", ".join(code[-12:]))
    rows = [r for r in _rows(lib) if not cd or r.get("campaign") != cd.name]
    mine = c.get("product")
    rows.sort(key=lambda r: str(r.get("at") or ""), reverse=True)
    rows.sort(key=lambda r: bool(mine) and r.get("product") != mine)  # stable: this product first, newest first
    made = [r for r in rows if r.get("kind") == "script"]
    if made:
        lines.append(f"what this owner already has ({len(made)} scripts; newest first, made = delivered):")
        for r in made[:limit]:
            pay = " ".join(str(r.get("payoff") or "").split())
            lines.append(f"- {r['campaign']}/{r['id']} ({'made' if r.get('made') else 'script only'}): {pay[:160]}")
    return "\n".join(lines)


def check(campaigns: Path) -> tuple[bool, list[str]]:
    """Can this folder be deleted? Every campaign saved, every script in history, every legacy format kept."""
    lib = root(campaigns)
    saved = _json(lib / SAVED) or {}
    have = {(r.get("campaign"), r.get("id")) for r in _rows(lib) if r.get("kind") == "script"}
    out, ok, delivered = [], True, 0
    for cd in sorted(p for p in Path(campaigns).iterdir() if (p / "campaign.json").is_file()):
        miss = []
        if cd.name not in saved:
            miss.append("never saved")
        for sp in sorted((cd / "05-scripts").glob("s*.json")):
            s = _json(sp)
            if isinstance(s, dict) and not s.get("_template") and (cd.name, s.get("id") or sp.stem) not in have:
                miss.append(f"script {sp.stem} not in history")
        delivered += _size(cd / "08-delivery")
        if miss:
            ok = False
            out.append(f"{cd.name}: {', '.join(miss[:4])}" + (" …" if len(miss) > 4 else "")
                       + f" → mvf --campaign {cd} library save")
    legacy = Path(campaigns) / "_formats"
    for f in sorted(legacy.glob("*/format.json")) if legacy.is_dir() else []:
        if not (lib / "formats" / f.parent.name / "format.json").is_file():
            ok = False
            out.append(f"format {f.parent.name} only in {legacy} → `mvf library save` in any campaign here")
    out.append(("SAFE: nothing a later video reads lives only in " if ok else "NOT SAFE to delete ") + str(campaigns)
               + f". Finished videos (08-delivery, {delivered / 1e9:.1f} GB) are not kept: copy what you still post.")
    return ok, out
