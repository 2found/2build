"""A channel: the folder a series of campaigns lives in, <repo>/marketing/channels/<channel>/.

  AGENTS.md, README.md     rules for agents (CLAUDE.md links to AGENTS.md) and the overview for people
  docs/                    bible, production workflow, decisions (Proposed until Decided)
  pack/campaign.json       the product pack's source: `mvf init` copies it into the library before every campaign
  pack/series/<name>.json  an overlay `mvf init --series <name>` merges into the new campaign.json
  campaigns/<slug>/        one campaign per episode
  brand/, metrics/         logo and character sheets; videos.csv and weekly.md

What every episode shares lives at channel level; what one video needs lives in its campaign."""
from __future__ import annotations

import json
import os
from pathlib import Path

from .common import TOOLS, dump, load, project_root
from . import library

TEMPLATES = TOOLS / "templates/channel"
PACK = "pack/campaign.json"
SERIES = "pack/series"
ABOUT = ("The channel's product pack for mvf: `mvf init --channel {channel}` copies it into the library "
         "(products/{product}/campaign.json) before every new campaign, so edit brand, colours, fonts and "
         "pronunciations here, never in the library. brand.logo is relative to the channel folder.")


def path(name: str) -> Path:
    return project_root() / "marketing/channels" / library.slug(name)


def init(chd: Path, product: str) -> list[str]:
    """Write whatever of the skeleton is missing; an existing file is never touched. Returns what was written."""
    made = []
    subs = {"{{CHANNEL}}": chd.name, "{{PRODUCT}}": product}
    for src in sorted(TEMPLATES.rglob("*")):
        dst = chd / src.relative_to(TEMPLATES)
        if not src.is_file() or os.path.lexists(dst):
            continue
        text = src.read_text(encoding="utf-8")
        for k, v in subs.items():
            text = text.replace(k, v)
        dst.parent.mkdir(parents=True, exist_ok=True)
        dst.write_text(text, encoding="utf-8")
        made.append(dst.relative_to(chd).as_posix())
    if not os.path.lexists(chd / "CLAUDE.md"):
        try:
            (chd / "CLAUDE.md").symlink_to("AGENTS.md")
        except OSError:  # no symlinks here: an import line does the same for Claude Code
            (chd / "CLAUDE.md").write_text("@AGENTS.md\n", encoding="utf-8")
        made.append("CLAUDE.md")
    for d in ("campaigns", "brand"):
        if not (chd / d).is_dir():
            (chd / d).mkdir(parents=True)
            (chd / d / ".gitkeep").touch()
            made.append(f"{d}/")
    if not (chd / PACK).is_file():
        dump(chd / PACK, {"_about": ABOUT.format(channel=chd.name, product=library.slug(product)),
                          **current_pack(chd, product)})
        made.append(PACK)
    return made


def _defaults(d) -> dict:
    return {k: d[k] for k in library.DEFAULTS if k in d} if isinstance(d, dict) else {}


def template_pack(product: str) -> dict:
    text = (TOOLS / "templates/campaign/campaign.json").read_text(encoding="utf-8")
    return _defaults(json.loads(text.replace("{{PRODUCT}}", product).replace("{{SLUG}}", "")))


def library_pack(chd: Path, product: str) -> dict:
    return _defaults(library._json(library.root(chd) / "products" / library.slug(product) / "campaign.json"))


def current_pack(chd: Path, product: str) -> dict:
    """The library's product pack when it has one, else the campaign template's defaults."""
    return library_pack(chd, product) or template_pack(product)


def sync_pack(chd: Path, product: str) -> list[str]:
    """Copy the channel's pack/campaign.json (without `_about`) over the library's product pack, with the logo it
    names. A pack still equal to the template is no source: it is refilled from the library when the library has a
    pack, so a first campaign's saved brand reaches the second. Returns notes for the owner."""
    src = chd / PACK
    if not src.is_file():
        return []
    try:
        pack = load(src)
    except ValueError as e:
        raise SystemExit(f"{src}: not JSON ({e})")
    about = pack.pop("_about", None)
    old = library_pack(chd, product)
    if pack == template_pack(product):
        if not old or old == pack:
            return []
        dump(src, {**({"_about": about} if about else {}), **old})
        return [f"{src} was still the template: filled from the library's product pack"]
    pd = library.root(chd) / "products" / library.slug(product)
    notes = []
    changed = sorted(k for k in old if old.get(k) != pack.get(k))
    if changed:
        notes.append(f"the library's product pack differed on {', '.join(changed)}: {src} replaced it "
                     f"(put a change you want kept into the channel's pack)")
    dump(pd / "campaign.json", pack)
    logo = (pack.get("brand") or {}).get("logo")
    if isinstance(logo, str) and logo and not Path(logo).is_absolute() and (chd / logo).is_file():
        library._put(chd / logo, pd / "brand" / Path(logo).name)
    return notes


def series(chd: Path) -> list[str]:
    d = chd / SERIES
    return sorted(p.stem for p in d.glob("*.json") if not p.name.startswith(("_", "."))) if d.is_dir() else []


def overlay(chd: Path, name: str) -> dict:
    from .formats import plain
    p = chd / SERIES / f"{plain(name, 'series name')}.json"
    if not p.is_file():
        raise SystemExit(f"no series {name!r} in {chd / SERIES} (have: {', '.join(series(chd)) or 'none'})")
    try:
        o = load(p)
    except ValueError as e:
        raise SystemExit(f"{p}: not JSON ({e})")
    o.pop("_about", None)
    return {**o, "series": o.get("series") or name}


def merge(base: dict, over: dict) -> dict:
    """Lists are unioned in order (the campaign's items first), objects merged, other values replaced."""
    out = dict(base)
    for k, v in over.items():
        b = out.get(k)
        if isinstance(v, list) and isinstance(b, list):
            out[k] = b + [x for x in v if x not in b]
        elif isinstance(v, dict) and isinstance(b, dict):
            out[k] = merge(b, v)
        else:
            out[k] = v
    return out
