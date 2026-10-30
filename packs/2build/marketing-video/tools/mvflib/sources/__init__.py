"""Source registry: every way data reaches a campaign's material pack.

Built in: `manual` (a JSON file of Items researched by hand). Everything else —
a product's analytics API, an app-store review export, a ranking site — is an
extension (mvflib/extensions.py) exporting SOURCES = {"ns.name": Source(...)}.
A Source fetches with a flat dict of string params and returns Items (see
mvflib/material.py). It reads only: no source writes to a product, logs in to a
platform or spends money.
"""
from __future__ import annotations

from pathlib import Path

from .. import extensions
from ..extensions import Source  # noqa: F401  (extensions import it from here or from mvflib.extensions)


def registry(campaign: Path | None = None) -> dict[str, Source]:
    from . import manual
    return {**manual.SOURCES, **extensions.load(campaign).sources}


def run(name: str, params: dict, campaign: Path | None = None) -> list[dict]:
    reg = registry(campaign)
    if name not in reg:
        raise SystemExit(f"unknown source {name!r}; `mvf sources` lists {sorted(reg)}")
    src = reg[name]
    missing = [p[1:] for p in src.params if p.startswith("*") and p[1:] not in params]
    unknown = [k for k in params if k not in {p.lstrip("*") for p in src.params}]
    if missing or unknown:
        raise SystemExit(f"{name}: " + "; ".join(
            ([f"missing {', '.join(missing)}"] if missing else []) + ([f"unknown {', '.join(unknown)}"] if unknown else []))
            + f" — params: {', '.join(src.params) or 'none'}")
    return src.fetch(params)
