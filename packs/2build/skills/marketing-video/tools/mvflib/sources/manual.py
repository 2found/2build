"""Hand research into the pack: a JSON file of Items (a list, or {"items": [...]}).

This is how research reaches a script before a connector (an extension) exists:
reviews copied from a store page, a ranking read off a site, a metric from a
dashboard, a screenshot you own. Every item still needs source.url + fetched_at, and rights is
the researcher's honest call ("unknown" when unsure: it informs, never ships).
"""
from __future__ import annotations

import json
from pathlib import Path

from . import Source


def _fetch(p: dict) -> list[dict]:
    path = Path(p["file"]).expanduser()
    if not path.is_file():
        raise SystemExit(f"manual: no file {path}")
    d = json.loads(path.read_text(encoding="utf-8"))
    items = d.get("items") if isinstance(d, dict) else d
    if not isinstance(items, list):
        raise SystemExit("manual: expected a list of items or {\"items\": [...]}")
    return items


SOURCES = {
    "manual": Source("Items researched by hand (reviews, rankings, metrics, screenshots) from a JSON file",
                     ["any"], _fetch, {"*file": "path to a JSON list of Items"}),
}
