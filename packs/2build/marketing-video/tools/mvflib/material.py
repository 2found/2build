"""Material: the data a campaign's scripts and videos stand on, from any source.

Every source (a hand-researched file, or an extension reading a product API,
a store, a ranking) yields the same Item envelope; `mvf gather` validates and
merges them into one frozen pack per campaign, `00-material/index.json`. Scripts
cite item ids the way they cite fact ids, so a review, a metric, a ranking or a
screenshot all reach the script by one path and are checked by one lint.

Item:
  id        "<ns>:<kind>:<key>"  e.g. appstore:review:12345
  kind      one of KINDS (+ kinds extensions add)
  title     short label
  body      the text itself (review, quote, description) — what a card shows
  signals   {name: number} measured popularity (stars, likes, users30d, rank…)
  source    {name, url?, fetched_at}  where and when it was read
  rights    one of RIGHTS — whether a video may show/quote it
  links     [item id] — product ↔ feature ↔ review ↔ asset
  credit    who to credit on screen/caption (required for official-promo)
  meta      anything else the source returns (image url, store, version…)
"""
from __future__ import annotations

import re
from pathlib import Path

from . import extensions
from .common import Report, dump, load, parse_date, today

KINDS = {
    "product": "a product or plan, with its stats",
    "feature": "one thing the product does, as users meet it",
    "quote": "a line someone said or wrote that the video can carry",
    "review": "a user's review or rating (store, marketplace, survey), with its signals",
    "post": "a community/social post about the product",
    "ranking": "an ordered list (store chart, leaderboard, award); members in links",
    "metric": "a measured number about the product or its users",
    "person": "a real person (founder, customer, creator) — never drawn by AI",
    "asset": "a picture or clip that may appear in the video",
    "trend": "a format/sound/meme observed on a platform",
}
# Kinds whose rights decide whether a video may show them (lint M4).
SHOWN = {"asset", "person", "post", "review"}
# Kinds that measure what people like; a pack with none of them is opinion.
MEASURED = {"quote", "review", "ranking", "metric", "post"}
RIGHTS = {
    "own": "our own content or catalogue: free to show",
    "official-promo": "official promotional material (poster, still, trailer): short, credited, with our commentary",
    "licensed": "we hold a licence: note it in meta.licence",
    "public-domain": "no rights remain",
    "unknown": "not cleared: may inform the script, may not be shown",
}
# Show only what is ours, licensed, public domain or official promotional material
# used briefly with credit. "unknown" never ships.
SHOWABLE = {"own", "official-promo", "licensed", "public-domain"}
SIGNAL_DAYS = 30  # a popularity number older than this is not a claim we can make today
ID = re.compile(r"^[a-z0-9-]+:[a-z]+:\S+$")
DIR, INDEX = "00-material", "00-material/index.json"


def kinds(campaign: Path | None = None) -> dict:
    return {**KINDS, **extensions.load(campaign).kinds}


def validate(item: dict, where: str = "item", known: dict | None = None) -> Report:
    known = known or KINDS
    r = Report()
    iid = item.get("id")
    if not isinstance(iid, str) or not ID.match(iid):
        r.fail("MAT", where, f"id {iid!r} must look like <ns>:<kind>:<key>")
    elif iid.split(":")[1] != item.get("kind"):
        r.fail("MAT", where, f"id {iid} names kind {iid.split(':')[1]!r} but kind is {item.get('kind')!r}")
    if item.get("kind") not in known:
        r.fail("MAT", where, f"kind {item.get('kind')!r} not in {sorted(known)}")
    if not str(item.get("title", "")).strip():
        r.fail("MAT", where, "title is empty")
    if item.get("rights") not in RIGHTS:
        r.fail("MAT", where, f"rights {item.get('rights')!r} not in {sorted(RIGHTS)}")
    if item.get("rights") == "official-promo" and not str(item.get("credit", "")).strip():
        r.fail("MAT", where, "official-promo needs credit (studio/platform shown on screen and in the caption)")
    licence = str((item.get("meta") or {}).get("licence", ""))
    if item.get("rights") == "licensed" and "BY" in licence.upper().replace("-", " ").split() and \
            not str(item.get("credit", "")).strip():
        r.fail("MAT", where, f"licence {licence} requires attribution: set credit")
    src = item.get("source") or {}
    if not str(src.get("name", "")).strip():
        r.fail("MAT", where, "source.name is empty")
    if not parse_date(src.get("fetched_at")):
        r.fail("MAT", where, "source.fetched_at must be an ISO date")
    if item.get("kind") in {"person", "asset", "trend", "ranking"} and \
            not str(src.get("url", "")).startswith(("http://", "https://")) and item.get("rights") != "own":
        r.fail("MAT", where, f"a {item.get('kind')} from outside needs source.url")
    for k, v in (item.get("signals") or {}).items():
        if not isinstance(v, (int, float)) or isinstance(v, bool):
            r.fail("MAT", where, f"signal {k} must be a number, got {v!r}")
    if not isinstance(item.get("links", []), list):
        r.fail("MAT", where, "links must be a list of item ids")
    return r


def signal_age(item: dict) -> int | None:
    d = parse_date((item.get("source") or {}).get("fetched_at"))
    return (today() - d).days if d else None


def load_pack(cd: Path) -> dict:
    p = cd / INDEX
    return load(p) if p.is_file() else {"items": {}, "runs": []}


def combine(old: dict, new: dict) -> dict:
    """A re-read wins field by field, but a thinner read (a product seen from a
    review, no description or stats) never erases what a richer one gathered."""
    out = {**old, **new}
    out["body"] = new.get("body") or old.get("body", "")
    for k in ("signals", "meta"):
        out[k] = {**(old.get(k) or {}), **(new.get(k) or {})}
    out["links"] = list(dict.fromkeys([*(old.get("links") or []), *(new.get("links") or [])]))
    if old.get("signals") and not new.get("signals"):
        out["source"] = old.get("source")  # the date a claim is measured from stays the measuring read's
    return out


def merge(cd: Path, items: list[dict], run: dict) -> tuple[int, int, Report]:
    """Validate and merge into the pack (see combine)."""
    pack = load_pack(cd)
    known = kinds(cd)
    rep, added, updated = Report(), 0, 0
    good = []
    for i, it in enumerate(items):
        r = validate(it, it.get("id") or f"{run['source']}[{i}]", known)
        rep.extend(r)
        if not r.fails:
            good.append(it)
    for it in good:
        if it["id"] in pack["items"]:
            updated += 1
            it = combine(pack["items"][it["id"]], it)
        else:
            added += 1
        pack["items"][it["id"]] = it
    pack["runs"].append({**run, "items": len(good), "rejected": len(items) - len(good)})
    dump(cd / INDEX, pack)
    return added, updated, rep


def check_pack(pack: dict, known: dict | None = None) -> Report:
    r = Report()
    items = pack.get("items") or {}
    if not items:
        r.fail("MAT", INDEX, "empty: run `mvf gather <source> …` (see `mvf sources`)")
    for iid, it in items.items():
        r.extend(validate(it, iid, known))
        for ln in it.get("links") or []:
            if ln not in items:
                r.info("MAT", iid, f"links to {ln}, not gathered")
    seen = {it.get("kind") for it in items.values()}
    if items and not seen & MEASURED:
        r.warn("MAT", INDEX, f"no {'/'.join(sorted(MEASURED))}: nothing measured says what people like yet")
    return r


def rank(items: list[dict], kind: str | None = None, q: str | None = None) -> list[dict]:
    """Filter, then strongest first: the biggest signal relative to its kind's best."""
    xs = [i for i in items if (not kind or i.get("kind") == kind)]
    if q:
        ql = q.lower()
        xs = [i for i in xs if ql in (i.get("title", "") + " " + i.get("body", "") + " " + i["id"]).lower()]

    def score(i):
        s = i.get("signals") or {}
        if "rank" in s and len(s) == 1:
            return 1.0 / max(s["rank"], 1)
        return max([v for k, v in s.items() if k != "rank"] or [0])
    return sorted(xs, key=lambda i: (i.get("kind"), -score(i), i["id"]))


def card(item: dict, pack_items: dict | None = None, width: int = 400) -> str:
    """A few lines an agent can read instead of the source."""
    sig = " ".join(f"{k}={v:g}" if isinstance(v, float) else f"{k}={v}" for k, v in (item.get("signals") or {}).items())
    src = item.get("source") or {}
    age = signal_age(item)
    lines = [f"{item['id']}  [{item['kind']}] {item.get('title', '')}",
             f"  rights={item.get('rights')}" + (f" credit={item['credit']}" if item.get("credit") else "") +
             (f"  {sig}" if sig else "") + f"  ({src.get('name')}, {src.get('fetched_at')}"
             + (f", {age}d old" if age else "") + ")"]
    body = " ".join(str(item.get("body") or "").split())
    if body:
        lines.append("  " + (body[:width] + "…" if len(body) > width else body))
    if src.get("url"):
        lines.append(f"  {src['url']}")
    for ln in item.get("links") or []:
        t = (pack_items or {}).get(ln)
        lines.append(f"  → {ln}" + (f"  {t.get('title')}" if t else "  (not gathered)"))
    return "\n".join(lines)
