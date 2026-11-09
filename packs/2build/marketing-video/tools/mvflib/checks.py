"""Validators for the research stages: brief, product facts, audience, trends,
angles and pre-test decisions. Each returns a Report; FAIL blocks the stage."""
from __future__ import annotations

from pathlib import Path

from .common import BENEFITS, CAMPAIGN_PLATFORMS, ID_RE, PLATFORMS, Report, family, load, parse_date, today

GOALS = {"awareness", "follow", "signup", "install", "purchase", "retention", "launch"}
SOURCE_TYPES = {"url", "file", "user", "measured"}
EVIDENCE_TYPES = SOURCE_TYPES | {"inference"}
TREND_KINDS = {"hashtag", "sound", "format", "meme", "topic", "creator", "event", "search"}
TREND_STAGES = {"rising", "peak", "fading", "evergreen"}
TREND_DECISIONS = {"use", "adapt", "watch", "avoid"}

STALE_DAYS, DEAD_DAYS, PRICE_DAYS = 14, 30, 30
PRETEST_METHODS = {"simulated-panel", "human-panel", "agent-panel", "skipped"}
# Reach rings: what a viewer already knows. broad = has never heard of the topic, genre = likes the kind of
# thing (the genre, the category) but not this one, core = already knows it. A reach goal needs a ring past core.
RINGS = {"broad", "genre", "core"}
REACH_GOALS = {"awareness", "follow", "launch"}


def _ids(items, kind, where, r: Report) -> set:
    seen = set()
    for i, it in enumerate(items):
        iid = it.get("id") if isinstance(it, dict) else None
        if not isinstance(iid, str) or not ID_RE[kind].match(iid):
            r.fail("ID", f"{where}[{i}]", f"id {iid!r} must match {ID_RE[kind].pattern}")
        elif iid in seen:
            r.fail("ID", f"{where}[{i}]", f"duplicate id {iid}")
        else:
            seen.add(iid)
    return seen


def _source(src, where, r: Report, types=SOURCE_TYPES) -> None:
    if not isinstance(src, dict):
        r.fail("SRC", where, "needs a source {type, ref, quote}")
        return
    t = src.get("type")
    if t not in types:
        r.fail("SRC", where, f"source type {t!r} not in {sorted(types)}")
    if not str(src.get("ref", "")).strip():
        r.fail("SRC", where, "source ref is empty (a URL, a repo path, or who said it)")
    if t in {"url", "file"} and not str(src.get("quote", "")).strip():
        r.fail("SRC", where, "url/file sources carry the verbatim quote the claim rests on")
    if t == "url" and not str(src.get("ref", "")).startswith(("http://", "https://")):
        r.fail("SRC", where, "url source ref must start with http(s)://")


def template_left(d: dict, where: str, r: Report) -> bool:
    """Files written by `mvf init` carry _template until someone fills them."""
    if isinstance(d, dict) and d.get("_template"):
        r.fail("TPL", where, "still the template: fill it in, then delete the \"_template\" key")
        return True
    return False


def check_campaign(c: dict) -> Report:
    r = Report()
    template_left(c, "campaign", r)
    for k in ("slug", "product", "goal", "platforms", "languages", "markets"):
        if not c.get(k):
            r.fail("BRF", "campaign", f"{k} is required")
    if c.get("goal") and c["goal"] not in GOALS:
        r.fail("BRF", "campaign.goal", f"{c['goal']!r} not in {sorted(GOALS)}")
    bad = set(c.get("platforms") or []) - set(CAMPAIGN_PLATFORMS)
    if bad:
        r.fail("BRF", "campaign.platforms", f"unknown {sorted(bad)}; this flow targets {list(CAMPAIGN_PLATFORMS)}")
    if not (c.get("brand") or {}).get("names"):
        r.fail("BRF", "campaign.brand.names", "list every spelling of the brand (used by the ad-feel lint)")
    if not str(c.get("kpi", "")).strip():
        r.warn("BRF", "campaign.kpi", "no KPI: name the number this campaign should move")
    b = c.get("benefit")
    if not b:
        r.info("BRF", "campaign.benefit", f"no campaign benefit: each angle picks its own ({sorted(BENEFITS)}); set one "
               "only when the owner asks for it (references/04-strategy.md#benefit)")
    elif b not in BENEFITS:
        r.fail("BRF", "campaign.benefit", f"{b!r} not in {sorted(BENEFITS)}")
    return r


def check_facts(f: dict) -> Report:
    r = Report()
    template_left(f, "facts", r)
    if not str(f.get("one_liner", "")).strip():
        r.fail("PRD", "facts.one_liner", "write what the product is in one line, in the viewer's words")
    facts = f.get("facts") or []
    _ids(facts, "fact", "facts", r)
    if len(facts) < 3:
        r.warn("PRD", "facts", f"only {len(facts)} facts; a script needs a promise, a proof and a limit")
    kinds = {x.get("kind") for x in facts}
    if "proof" not in kinds:
        r.warn("PRD", "facts", "no `proof` fact (a real result, number, quote or screen): demos and claims will be empty")
    if "limit" not in kinds:
        r.warn("PRD", "facts", "no `limit` fact: knowing what the product does NOT do keeps scripts honest")
    for x in facts:
        where = f"facts.{x.get('id')}"
        if not str(x.get("claim", "")).strip():
            r.fail("PRD", where, "claim is empty")
        _source(x.get("source"), where, r)
        d = parse_date(x.get("checked"))
        if not d:
            r.fail("PRD", where, "checked must be an ISO date (the day the source was read)")
        elif x.get("kind") in {"price", "number"} and (today() - d).days > PRICE_DAYS:
            r.warn("PRD", where, f"{x['kind']} checked {d} (> {PRICE_DAYS} days): re-read the source before publishing")
    return r


def check_segments(s: dict, platforms: list[str], goal: str | None = None) -> Report:
    r = Report()
    template_left(s, "segments", r)
    segs = s.get("segments") or []
    if not segs:
        r.fail("AUD", "segments", "no segments")
    _ids(segs, "segment", "segments", r)
    for x in segs:
        where = f"segments.{x.get('id')}"
        for k in ("name", "who"):
            if not str(x.get(k, "")).strip():
                r.fail("AUD", where, f"{k} is empty")
        for k in ("pains", "jobs"):
            if not x.get(k):
                r.fail("AUD", where, f"{k} needs at least one entry, in the segment's own words")
        if not x.get("objections"):
            r.warn("AUD", where, "no objections: the script cannot pre-empt what this segment will say in the comments")
        if not isinstance(x.get("priority"), int):
            r.fail("AUD", where, "priority must be an integer (1 = primary)")
        if x.get("ring") not in RINGS:
            r.warn("AUD2", where, f"ring {x.get('ring')!r} not in {sorted(RINGS)}: say what this segment already knows "
                   "(broad = never heard of it, genre = likes the kind, core = knows it)")
        ev = x.get("evidence") or []
        if not ev:
            r.fail("AUD", where, "no evidence: cite a review, a comment, a support ticket, a metric or the user's words")
        for i, e in enumerate(ev):
            _source(e, f"{where}.evidence[{i}]", r, EVIDENCE_TYPES)
        if ev and all(e.get("type") == "inference" for e in ev):
            (r.fail if x.get("priority") == 1 else r.warn)(
                "AUD", where, "evidence is inference only: this segment is a hypothesis; find one real voice for it")
        for p in dict.fromkeys(family(p) for p in platforms):
            pl = (x.get("platforms") or {}).get(p)
            if not pl:
                r.warn("AUD", where, f"no {p} habits: how and when this segment uses {PLATFORMS[p]['label']}")
            elif p in {"tiktok", "youtube"} and not pl.get("search_terms"):
                r.warn("AUD", where, f"no {p}.search_terms: {PLATFORMS[p]['label']} is a search engine for this audience; "
                       "captions and titles use these")
    if segs and not any(x.get("priority") == 1 for x in segs):
        r.fail("AUD", "segments", "no priority-1 segment")
    if goal in REACH_GOALS and segs and all(x.get("ring") == "core" for x in segs):
        r.fail("AUD2", "segments", f"goal {goal} and every segment is core: a video only people who already know the topic "
               "understand reaches only them. Add a broad or genre segment (references/02-audience.md#reach-rings)")
    if not s.get("anti_segments"):
        r.info("AUD", "segments", "no anti_segments: who sees the video but must not be targeted (and will still comment)")
    return r


def trend_age(t: dict) -> int | None:
    d = parse_date(t.get("observed_at"))
    return None if d is None else (today() - d).days


def check_trends(t: dict, platforms: list[str], segment_ids: set) -> Report:
    r = Report()
    template_left(t, "trends", r)
    trends = t.get("trends") or []
    _ids(trends, "trend", "trends", r)
    for x in trends:
        where = f"trends.{x.get('id')}"
        if x.get("platform") not in {*map(family, platforms), "cross"}:
            r.fail("TRD", where, f"platform {x.get('platform')!r} not in campaign platforms (or 'cross')")
        if x.get("kind") not in TREND_KINDS:
            r.fail("TRD", where, f"kind {x.get('kind')!r} not in {sorted(TREND_KINDS)}")
        if x.get("stage") not in TREND_STAGES:
            r.fail("TRD", where, f"stage {x.get('stage')!r} not in {sorted(TREND_STAGES)}")
        if x.get("decision") not in TREND_DECISIONS:
            r.fail("TRD", where, f"decision {x.get('decision')!r} not in {sorted(TREND_DECISIONS)}")
        if not str(x.get("url", "")).startswith(("http://", "https://")):
            r.fail("TRD", where, "url is required: the page where the trend was observed")
        if x.get("method", "observed") not in {"observed", "inference"}:
            r.fail("TRD", where, "method: observed (you saw it, with its url and number) or inference (a guess)")
        if not x.get("metric") and x.get("method") != "inference":
            r.warn("TRD", where, "no metric (rank, views, posts, growth): a trend without a number is an opinion")
        age = trend_age(x)
        if age is None:
            r.fail("TRD", where, "observed_at must be an ISO date")
        elif x.get("stage") in {"rising", "peak"}:
            if age > DEAD_DAYS and x.get("decision") in {"use", "adapt"}:
                r.fail("TRD", where, f"{x['stage']} trend observed {age} days ago: re-observe it or drop it")
            elif age > STALE_DAYS:
                r.warn("TRD", where, f"{x['stage']} trend observed {age} days ago: re-check before production")
        fit = x.get("fit") or {}
        for sid in fit.get("segments") or []:
            if sid not in segment_ids:
                r.fail("TRD", where, f"fit.segments names unknown segment {sid}")
        if x.get("decision") in {"use", "adapt"}:
            if not str(fit.get("how", "")).strip():
                r.fail("TRD", where, "fit.how: say exactly how a script uses it")
            if not fit.get("segments"):
                r.fail("TRD", where, "fit.segments: which segment this trend reaches")
        if x.get("kind") == "sound" and x.get("platform") == "tiktok" and x.get("decision") in {"use", "adapt"}:
            lic = str((x.get("risk") or {}).get("license", "")).lower()
            if lic not in {"cml", "original", "licensed"}:
                r.warn("TRD", where, "a business account may only use Commercial Music Library sounds: set risk.license "
                       "to cml / original / licensed after checking")
    for p in dict.fromkeys(family(p) for p in platforms):
        mine = [x for x in trends if x.get("platform") in {p, "cross"}]
        if len(mine) < 3:
            r.warn("TRD", "trends", f"{len(mine)} trends cover {PLATFORMS[p]['label']}; observe at least 3 (formats, topics, sounds)")
        if mine and not any(x.get("kind") == "format" for x in mine):
            r.warn("TRD", "trends", f"no `format` trend for {PLATFORMS[p]['label']}: formats outlive hashtags and shape the script")
        if mine and all(x.get("method") == "inference" for x in mine):
            r.warn("TRD", "trends", f"every {PLATFORMS[p]['label']} row is inference: nothing was observed there; say "
                   "what blocked it in observed_with and treat these as guesses")
    refs = t.get("references") or []
    if not refs:
        r.warn("TRD", "references", "no reference videos: run `mvf trend-video` on 3+ top posts in the niche")
    return r


def check_angles(a: dict, fact_ids: set, segment_ids: set, trend_ids: set, platforms: list[str],
                 ideas: dict | None = None, benefit: str | None = None, lore: list | None = None,
                 goal: str | None = None) -> Report:
    r = Report()
    template_left(a, "angles", r)
    angles = a.get("angles") or []
    if not angles:
        r.fail("STR", "angles", "no angles")
    _ids(angles, "angle", "angles", r)
    for x in angles:
        where = f"angles.{x.get('id')}"
        if x.get("segment") not in segment_ids:
            r.fail("STR", where, f"segment {x.get('segment')!r} is not in segments.json")
        for k in ("insight", "promise", "why_watch", "why_share", "format"):
            if not str(x.get(k, "")).strip():
                r.fail("STR", where, f"{k} is empty")
        _benefit(x.get("benefit"), where, benefit, lore or [], r)
        if goal == "awareness" and (x.get("benefit") or {}).get("type") == "decide":
            r.warn("STR3", where, "a `decide` angle on an awareness campaign: if the decision is about the product "
                   "itself (start it, which version, where), only viewers already weighing it care, and a stranger "
                   "gets nothing to retell; give the stranger something from inside it and let the decision follow")
        if not x.get("facts"):
            r.fail("STR", where, "facts: the promise must stand on at least one fact")
        for fid in x.get("facts") or []:
            if fid not in fact_ids:
                r.fail("STR", where, f"unknown fact {fid}")
        for tid in x.get("trends") or []:
            if tid not in trend_ids:
                r.fail("STR", where, f"unknown trend {tid}")
        for p in x.get("platforms") or []:
            if p not in platforms:
                r.fail("STR", where, f"platform {p} is not a campaign platform")
        if ideas is not None:
            idea = ideas.get(x.get("idea"))
            if not idea:
                r.fail("STR", where, f"idea {x.get('idea')!r}: an angle builds on a picked idea from 03-research")
            elif not idea.get("pick"):
                r.warn("STR", where, f"idea {idea['id']} is not picked in research.json")
    if len({x.get("format") for x in angles}) < min(2, len(angles)):
        r.warn("STR", "angles", "every angle uses the same format: test different formats, not different words")
    return r


def _benefit(b, where: str, wanted: str | None, lore: list, r: Report) -> None:
    """The angle says what the viewer can use afterwards, in their life, without the topic's names."""
    if not isinstance(b, dict):
        r.fail("STR3", where, "benefit: {type, takeaway, use_when}: what the viewer can use after the video "
               "(references/04-strategy.md#benefit)")
        return
    if b.get("type") not in BENEFITS:
        r.fail("STR3", where, f"benefit.type {b.get('type')!r} not in {sorted(BENEFITS)}")
    elif wanted and b["type"] != wanted:
        r.warn("STR3", where, f"benefit.type {b['type']!r}, but the owner asked for {wanted!r}")
    for k, what in (("takeaway", "the sentence the viewer keeps, about their life"),
                    ("use_when", "the situation in the viewer's own life where they use it")):
        if len(str(b.get(k, "")).strip()) < 10:
            r.fail("STR3", where, f"benefit.{k}: {what}")
    low = f" {str(b.get('takeaway', '')).lower()} "
    hits = [w for w in lore if f" {w.lower()} " in low]
    if hits and b.get("type") != "answer":  # an answer names the topic's things; advice must survive without them
        r.warn("STR3", where, f"benefit.takeaway names {hits}: delete the topic's names and the takeaway must still "
               "help someone who never meets the topic")


def check_pretest(path: Path) -> Report:
    r = Report()
    d = load(path)
    where = f"pretest.{path.parent.name}"
    m = d.get("method")
    if m not in PRETEST_METHODS:
        r.fail("PRE", where, f"method must be one of {sorted(PRETEST_METHODS)}")
    if m == "skipped":
        if not str(d.get("skipped_reason", "")).strip():
            r.fail("PRE", where, "skipped needs skipped_reason")
        return r
    if not d.get("winner"):
        r.fail("PRE", where, "winner: the hook/variant id production uses")
    if not str(d.get("evidence", "")).strip():
        r.fail("PRE", where, "evidence: the numbers or panel flags behind the pick")
    if m == "agent-panel" and not (path.parent / "feed").is_dir():
        r.warn("PRE2", where, "agent readers judging one script alone pass almost anything (a script the user called "
               "boring had no swipes); rank it in a feed against real videos and a rejected control (mvf feed)")
    if m == "simulated-panel" and not d.get("run"):
        r.fail("PRE", where, "a simulated-panel decision names the run id the evidence comes from")
    if "changes" not in d:
        r.warn("PRE", where, "changes: list what the pre-test changed in the script (empty list if nothing)")
    if m in {"simulated-panel", "human-panel"} and d.get("winner") and not str(d.get("retell", "")).strip():
        r.fail("PRE3", where, "retell: quote what the panel retold about the winner (mvf judge writes retell.md and "
               "interview.md); a pick nobody could retell was not tested for understanding")
    if d.get("_todo"):
        r.fail("PRE", where, f"the drafted decision is unread: {d['_todo']}")
    _retells(path.parent / "feed", str(d.get("winner") or ""), where, r)
    return r


def _retells(feed: Path, winner: str, where: str, r: Report) -> None:
    """The winner was tested for understanding: every feed reader retold what it said. A reader who read the
    script as text can echo its lines and rank it first without having understood it."""
    if not winner or not feed.is_dir():
        return
    missing, empty = [], []
    for kp in sorted(feed.glob("feed-*.key.json")):
        n = kp.name[:-len(".key.json")]
        lab = next((k for k, v in load(kp).items() if (v or {}).get("id") == winner), None)
        if not lab:
            continue
        for ap in sorted(feed.glob(f"{n}.*.json")):
            if ap.name.endswith(".key.json"):
                continue
            v = ((load(ap).get("videos") or {}).get(lab)) or {}
            if "retell" not in v:
                missing.append(ap.name)
            elif not str(v["retell"]).strip():
                empty.append(ap.name)
    if missing:
        r.fail("PRE3", where, f"no retell of {winner} in {', '.join(missing)}: understanding was not tested. Re-run the "
               "feed (mvf feed asks for retell and unclear) with fresh readers")
    if empty:
        r.warn("PRE3", where, f"{', '.join(empty)}: the reader could not say what {winner} told them; fix the body "
               "before the owner reads it")
