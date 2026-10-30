"""Extensions: data sources, voice engines, material kinds and recipes that live outside the skill.

An extension is a Python file in an extensions dir — `<dir>/<name>.py` or
`<dir>/<name>/extension.py` (a folder for an extension with helper files). It may export:

  SOURCES = {"ns.name": Source(...)}   ways data reaches the material pack (`mvf gather`)
  VOICES  = {"name": Voice(...)}       voice engines (`mvf voice --engine name`)
  KINDS   = {"kind": "what it is"}     extra material kinds
  RECIPES = {"name": {...}}            extra recipes (same shape as recipes.json)

Dirs are searched in this order; the first definition of a name wins:

  1. <campaign>/extensions/                       this campaign only
  2. <repo>/.babysit/marketing-video/extensions/  this machine's project extensions
  3. $MVF_EXTENSIONS (os.pathsep-separated)       anything else
  4. <skill>/tools/extensions/                    shipped with the skill (omnivoice)

Campaign extensions and .babysit/marketing-video/ are gitignored (`mvf init`,
`mvf extensions init`): they hold private endpoints, local model paths and voices.
An extension reads only — it never posts, logs in to a platform or spends money —
and takes secrets from environment variables it names in `needs`, never from files.
"""
from __future__ import annotations

import hashlib
import importlib.util
import os
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable

from .common import TOOLS, local_dir

VOICE_MODES = ("take", "design", "rehear")


@dataclass
class Source:
    about: str
    kinds: list[str]
    fetch: Callable[[dict], list[dict]]
    params: dict[str, str] = field(default_factory=dict)  # name → help; "*name" = required
    needs: list[str] = field(default_factory=list)  # env vars (values are never printed)


@dataclass
class Voice:
    """A voice engine. `modes` ⊆ take | design | rehear (see references/07-production.md).

    command: an external program run as `[*command, mode, "--job", job.json, "--out", dir, *opts]`
             (through the heavy prefix), for engines that need their own environment (torch…).
    run:     an in-process callable run(mode, job: dict, out: Path, opts: dict) for light engines.
    Exactly one of the two. `opts` come from `mvf voice --opt key=value`; a command gets `--key value`.
    """
    about: str
    modes: tuple = ("take",)
    command: list[str] | None = None
    run: Callable[[str, dict, Path, dict], None] | None = None
    cwd: str | None = None
    needs: list[str] = field(default_factory=list)
    opts: dict[str, str] = field(default_factory=dict)  # option → help, shown by `mvf extensions`


def dirs(campaign: Path | None = None) -> list[Path]:
    out = []
    if campaign:
        out.append(Path(campaign) / "extensions")
    out.append(local_dir(campaign) / "extensions")
    out += [Path(p).expanduser() for p in os.environ.get("MVF_EXTENSIONS", "").split(os.pathsep) if p.strip()]
    out.append(TOOLS / "extensions")
    return out


def _files(d: Path) -> list[Path]:
    if not d.is_dir():
        return []
    return sorted(p for p in d.glob("*.py") if not p.name.startswith("_")) + sorted(d.glob("*/extension.py"))


def _import(path: Path):
    name = "mvf_ext_" + hashlib.sha1(str(path.resolve()).encode()).hexdigest()[:10]
    if name in sys.modules:
        return sys.modules[name]
    spec = importlib.util.spec_from_file_location(name, path)
    mod = importlib.util.module_from_spec(spec)
    sys.modules[name] = mod
    try:
        spec.loader.exec_module(mod)
    except Exception as e:  # one broken extension must not take the tool down
        del sys.modules[name]
        raise SystemExit(f"extension {path} failed to load: {type(e).__name__}: {e}")
    return mod


@dataclass
class Loaded:
    sources: dict = field(default_factory=dict)
    voices: dict = field(default_factory=dict)
    kinds: dict = field(default_factory=dict)
    recipes: dict = field(default_factory=dict)
    origin: dict = field(default_factory=dict)  # (kind, name) → file
    shadowed: list = field(default_factory=list)  # (kind, name, file) a nearer dir already defined
    files: list = field(default_factory=list)


_CACHE: dict = {}


def load(campaign: Path | None = None) -> Loaded:
    key = str(Path(campaign).resolve()) if campaign else str(Path.cwd().resolve())
    if key in _CACHE:
        return _CACHE[key]
    out = Loaded()
    for d in dirs(campaign):
        for f in _files(d):
            mod = _import(f)
            out.files.append(f)
            for attr, table in (("SOURCES", out.sources), ("VOICES", out.voices), ("KINDS", out.kinds), ("RECIPES", out.recipes)):
                for name, val in (getattr(mod, attr, None) or {}).items():
                    if name in table:
                        out.shadowed.append((attr, name, f))
                        continue
                    if attr == "VOICES":
                        _check_voice(name, val, f)
                    table[name] = val
                    out.origin[(attr, name)] = f
    _CACHE[key] = out
    return out


def _check_voice(name: str, v, f: Path) -> None:
    if not isinstance(v, Voice):
        raise SystemExit(f"{f}: VOICES[{name!r}] must be a mvflib.extensions.Voice")
    if (v.command is None) == (v.run is None):
        raise SystemExit(f"{f}: voice {name!r} needs exactly one of command= or run=")
    bad = set(v.modes) - set(VOICE_MODES)
    if bad:
        raise SystemExit(f"{f}: voice {name!r} has unknown modes {sorted(bad)} (allowed {VOICE_MODES})")


GITIGNORE = "# Machine-local marketing-video state: extensions, reference voices, config. Never committed.\n*\n"


def init_local(start: Path | None = None) -> Path:
    d = local_dir(start)
    for sub in ("extensions", "voices"):
        (d / sub).mkdir(parents=True, exist_ok=True)
    gi = d / ".gitignore"
    if not gi.exists():
        gi.write_text(GITIGNORE, encoding="utf-8")
    return d
