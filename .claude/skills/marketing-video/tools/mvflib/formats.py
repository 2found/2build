"""A saved format: one series' look kept by name, so the next episode starts where the last one ended instead of
being re-directed from a blank page. `mvf format save <name> --from <sid>` keeps the direction's look, captions
and ambience, the campaign style, the rigs and props sheets and the script's voice; `mvf init --format <name>`
starts a campaign with them. Formats live beside the campaigns in `_formats/<name>/`."""
from __future__ import annotations

import datetime
import shutil
from pathlib import Path

from .common import campaign_style, dump, load

DIR = "_formats"
FILE = "format.json"
SHEETS = ("rigs.json", "props.json")
APPLIED = "07-video/format.json"  # a direction takes these as defaults under its own keys


def root(cd_or_campaigns: Path) -> Path:
    return Path(cd_or_campaigns) / DIR


def plain(name: str, what: str = "format name") -> str:
    """A name that stays one folder (or file stem) deep: no path separators, no `..`, no leading `_`/`.`."""
    if not name or name.startswith(("_", ".")) or any(c in name for c in "/\\") or ".." in name:
        raise SystemExit(f"{what} {name!r}: a plain slug")
    return name


def save(cd: Path, name: str, sid: str, force: bool = False) -> Path:
    plain(name)
    dp = cd / "07-video" / sid / "direction.json"
    if not dp.is_file():
        raise SystemExit(f"no {dp}: direct the episode first, then save its format")
    out = root(cd.parent) / name
    if out.exists() and not force:
        raise SystemExit(f"format {name} exists ({out}); pass --force to replace it")
    d = load(dp)
    sp = cd / "05-scripts" / f"{sid}.json"
    voice = (load(sp).get("voice") or {}) if sp.is_file() else {}
    snd = {k: v for k, v in ((d.get("sound") or {}).items()) if k == "ambience"}  # cues belong to one script's words
    fmt = {"name": name, "from": f"{cd.name}/{sid}", "saved": datetime.date.today().isoformat(),
           "style": campaign_style(cd), "look": d.get("look") or {}, "captions": d.get("captions") or {},
           "sound": snd, "voice": voice}
    if out.exists():
        shutil.rmtree(out)
    out.mkdir(parents=True)
    for s in SHEETS:
        if (cd / "07-video" / s).is_file():
            shutil.copy2(cd / "07-video" / s, out / s)
    ref = voice.get("ref")
    if ref:
        plain(str(ref), "voice.ref")
    if ref and (cd / "voices" / f"{ref}.wav").is_file():  # the campaign's own voice; a machine voice is shared already
        (out / "voices").mkdir()
        for ext in (".wav", ".txt"):
            if (cd / "voices" / f"{ref}{ext}").is_file():
                shutil.copy2(cd / "voices" / f"{ref}{ext}", out / "voices" / f"{ref}{ext}")
    dump(out / FILE, fmt)
    return out


def listing(campaigns: Path) -> list[dict]:
    r = root(campaigns)
    rows = []
    for p in sorted(r.glob(f"*/{FILE}")) if r.is_dir() else []:
        try:
            f = load(p)
        except (ValueError, OSError):
            rows.append({"name": p.parent.name, "broken": True})
            continue
        rows.append({"name": p.parent.name, "from": f.get("from"), "saved": f.get("saved"), "style": f.get("style"),
                     "voice": (f.get("voice") or {}).get("ref"),
                     "sheets": [s for s in SHEETS if (p.parent / s).is_file()]})
    return rows


def apply(cd: Path, name: str) -> dict:
    """Copy a saved format into a fresh campaign: style into campaign.json, sheets into 07-video/, its voice
    into voices/, and the look/captions/ambience into 07-video/format.json for every direction to start from."""
    src = root(cd.parent) / plain(name)
    if not (src / FILE).is_file():
        have = ", ".join(r["name"] for r in listing(cd.parent)) or "none"
        raise SystemExit(f"no format {name!r} in {root(cd.parent)} (have: {have})")
    f = load(src / FILE)
    (cd / "07-video").mkdir(parents=True, exist_ok=True)
    for s in SHEETS:
        if (src / s).is_file():
            shutil.copy2(src / s, cd / "07-video" / s)
    if (src / "voices").is_dir():
        (cd / "voices").mkdir(exist_ok=True)
        for v in (src / "voices").iterdir():
            shutil.copy2(v, cd / "voices" / v.name)
    if f.get("style"):
        c = load(cd / "campaign.json")
        dump(cd / "campaign.json", {**c, "style": f["style"]})
    dump(cd / APPLIED, {k: f.get(k) or {} for k in ("look", "captions", "sound")} | {"format": name, "voice": f.get("voice") or {}})
    return f


def defaults(cd: Path) -> dict:
    p = Path(cd) / APPLIED
    try:
        return load(p) if p.is_file() else {}
    except (ValueError, OSError):
        return {}


def under(d: dict, fmt: dict) -> dict:
    """The direction over the format: its own look/captions keys win, its own ambience replaces the format's."""
    if not fmt or not isinstance(d, dict):
        return d
    d = dict(d)
    for k in ("look", "captions"):
        if fmt.get(k):
            d[k] = {**fmt[k], **(d.get(k) or {})}
    amb = (fmt.get("sound") or {}).get("ambience")
    if amb and "ambience" not in (d.get("sound") or {}):
        d["sound"] = {**(d.get("sound") or {}), "ambience": amb}
    return d
