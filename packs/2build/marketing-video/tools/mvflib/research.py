"""Stage 3.5 topic research: what the audience already argues about, what the top
videos on the topic already said, the specific details worth a stranger's 30 s,
and the idea bank scored before one is picked. A script without this is written
from the writer's memory, and reads like it."""
from __future__ import annotations

from urllib.parse import urlparse

from .common import BENEFITS, Report, parse_date

PATH = "03-research/research.json"
COMMUNITY = {"forum", "reddit", "comments", "review", "group", "qa"}
SOURCE_KINDS = COMMUNITY | {"wiki", "news", "official", "book", "video", "interview", "article", "data"}
NUGGET_TYPES = {"fact", "scene", "quote", "number", "controversy", "theory", "behind-the-scenes", "connection", "mistake"}
# nugget types that come from inside the subject (what happens in it, what is said in it, what it means); a pick
# standing only on numbers and facts about it is about the packaging: length, price, where to get it
INSIDE = {"scene", "quote", "theory", "connection", "mistake", "controversy"}
STRONGER = 4  # a pick rides a pattern whose best video has at least 1/STRONGER of the strongest same-scope pattern's
MIN = {"sources": 8, "domains": 3, "community": 2, "competitors": 3, "questions": 5, "nuggets": 8, "ideas": 6,
       "top_videos": 5, "benefit_videos": 2, "patterns": 2}
FILLER = {"x", "?", "-", "...", "…", "tbd", "todo", "n/a", "na", "none", "same", "idem"}


def _filler(v) -> bool:
    """A field written to pass the count, not to say something."""
    t = str(v or "").strip().lower()
    return len(t) < 8 or t in FILLER


def _domain(url: str) -> str:
    h = urlparse(url).netloc.lower()
    return h[4:] if h.startswith("www.") else h


def check(d: dict, benefit: str | None = None, scripts: int = 3, facts: set | None = None) -> Report:
    """`facts` are the 01-product fact ids: a nugget may stand on a checked fact as well as on a source."""
    r = Report()
    if d.get("_template"):
        r.fail("TPL", "research", "still the template: research the topic, then delete the \"_template\" key")
    if not str(d.get("topic", "")).strip():
        r.fail("RS0", "research.topic", "name the topic as the audience searches it")

    srcs = d.get("sources") or []
    ids = set()
    for i, s in enumerate(srcs):
        w = f"sources[{i}] {s.get('id', '?')}"
        ids.add(s.get("id"))
        if not str(s.get("url", "")).startswith(("http://", "https://")):
            r.fail("RS1", w, "url: the page you read (http/https)")
        if s.get("kind") not in SOURCE_KINDS:
            r.warn("RS1", w, f"kind {s.get('kind')!r} not in {sorted(SOURCE_KINDS)}")
        if len(str(s.get("says", "")).strip()) < 20:
            r.fail("RS1", w, "says: what this page actually told you, in a sentence or a verbatim quote")
        if not parse_date(s.get("fetched_at")):
            r.fail("RS1", w, "fetched_at: YYYY-MM-DD you read it")
    if not srcs:
        r.fail("RS1", "research.sources", "no sources: research the topic and the audience on the web before writing")
    elif len(srcs) < MIN["sources"]:
        r.warn("RS1", "research.sources", f"{len(srcs)} sources; read at least {MIN['sources']} pages before writing")
    doms = {_domain(s.get("url", "")) for s in srcs if s.get("url")}
    if srcs and len(doms) < MIN["domains"]:
        r.warn("RS1", "research.sources", f"{len(doms)} sites; read at least {MIN['domains']} different sites")
    if srcs and sum(1 for s in srcs if s.get("kind") in COMMUNITY) < MIN["community"]:
        r.warn("RS1", "research.sources", f"fewer than {MIN['community']} community sources ({sorted(COMMUNITY)}): "
               "the audience's own words are where the questions and the jokes are")

    comps = d.get("competitors") or []
    for i, c in enumerate(comps):
        w = f"competitors[{i}]"
        if not str(c.get("url", "")).startswith(("http://", "https://")):
            r.fail("RS2", w, "url of the video")
        if not isinstance(c.get("views"), int):
            r.fail("RS2", w, "views: the measured number (int)")
        if not str(c.get("hook", "")).strip():
            r.warn("RS2", w, "hook: its first line, verbatim (what it says and shows in the first 2 s)")
        if not str(c.get("gap", "")).strip():
            r.warn("RS2", w, "gap: what it got wrong, skipped or left the comments asking")
    if len(comps) < MIN["competitors"]:
        msg = f"{len(comps)} competitor videos; watch the top {MIN['competitors']} on this topic first"
        r.warn("RS2", "research.competitors", msg)
    if not d.get("covered"):
        r.warn("RS2", "research.covered", "covered: what the existing videos already said, so ours adds something")

    pats = _top_videos(d, r)
    pat_views = _pattern_views(d)

    qs = d.get("questions") or []
    for i, q in enumerate(qs):
        if not str(q.get("q", "")).strip() or not q.get("sources"):
            r.fail("RS3", f"questions[{i}]", "q (as the audience asks it) and sources")
        for sid in q.get("sources") or []:
            if sid not in ids:
                r.fail("RS3", f"questions[{i}]", f"unknown source {sid}")
    if len(qs) < MIN["questions"]:
        r.warn("RS3", "research.questions", f"{len(qs)} audience questions; find at least {MIN['questions']}")

    nugs = d.get("nuggets") or []
    nids = set()
    for i, n in enumerate(nugs):
        w = f"nuggets[{i}] {n.get('id', '?')}"
        nids.add(n.get("id"))
        if not str(n.get("text", "")).strip():
            r.fail("RS4", w, "text: the specific detail (a name, a number, a line, a scene)")
        if n.get("type") not in NUGGET_TYPES:
            r.warn("RS4", w, f"type {n.get('type')!r} not in {sorted(NUGGET_TYPES)}")
        if not n.get("sources"):
            r.fail("RS4", w, "sources: where it is stated (a page, a chapter, a doc, a timestamp)")
        for sid in n.get("sources") or []:
            if sid not in ids and sid not in (facts or ()):
                r.fail("RS4", w, f"unknown source {sid}: cite a research source id or a 01-product fact id")
    if len(nugs) < MIN["nuggets"]:
        r.warn("RS4", "research.nuggets", f"{len(nugs)} nuggets; collect at least {MIN['nuggets']} specific details")

    ideas = d.get("ideas") or []
    picked, padded = [], []
    for i, x in enumerate(ideas):
        w = f"ideas[{i}] {x.get('id', '?')}"
        for k in ("premise", "question", "payoff", "inside", "new", "use_when"):
            if not str(x.get(k, "")).strip():
                (r.fail if x.get("pick") and k in ("premise", "inside", "use_when") else r.warn)("RS5", w, f"{k} is empty" + (": the situation in the viewer's own life where the idea helps"
                                                     if k == "use_when" else ": the scene, twist, line or rule from inside "
                                                     "the subject that a stranger would tell a friend tomorrow"
                                                     if k == "inside" else ""))
            elif _filler(x.get(k)):
                if x.get("pick"):
                    r.fail("RS5", w, f"{k} {x.get(k)!r} is filler: a picked idea says its {k} in a real sentence")
                elif x.get("id") not in padded:
                    padded.append(x.get("id"))
        if not str(x.get("benefit") or "").strip():
            (r.fail if x.get("pick") else r.warn)("RS5", w, f"benefit is empty: what the viewer leaves with ({sorted(BENEFITS)}, or your own)")
        elif x["benefit"] not in BENEFITS:
            r.warn("RS5", w, f"benefit {x['benefit']!r} is your own (listed: {sorted(BENEFITS)}): fine when it is what the viewer "
                   "leaves with, not only how the video holds them")
        for nid in x.get("nuggets") or []:
            if nid not in nids:
                r.fail("RS5", w, f"unknown nugget {nid}")
        if x.get("pick"):
            picked.append(x)
            if not x.get("nuggets"):
                r.fail("RS6", w, "a picked idea stands on nuggets")
            if benefit and x.get("benefit") != benefit:
                r.warn("RS6", w, f"benefit {x.get('benefit')!r}, but the owner asked for {benefit!r}")
            kinds = {n.get("type") for n in nugs if n.get("id") in set(x.get("nuggets") or [])}
            if kinds and not kinds & INSIDE:
                r.warn("RS6", w, f"stands only on {sorted(kinds)} nuggets: numbers and facts about the subject "
                       "(its length, its price, where to get it) are its packaging, and nobody retells them; build it "
                       f"on something from inside it ({sorted(INSIDE)})")
            mine = pat_views.get(x.get("pattern"))
            if mine:
                rivals = {k: v for k, v in pat_views.items() if v[1] == mine[1]}  # topic vs topic, benefit vs benefit
                top = max(rivals, key=lambda k: rivals[k][0])
                if mine[0] * STRONGER < rivals[top][0]:
                    r.warn("RS6", w, f"rides {x.get('pattern')} (best {mine[1]} video {mine[0]:,} views) while {top} "
                           f"reaches {rivals[top][0]:,}: the audience already voted for {top}; ride it, or keep this "
                           "one knowing it was watched far less")
            if pats and x.get("pattern") not in pats:
                r.warn("RS8", w, f"pattern {x.get('pattern')!r}: name the top-video pattern this idea rides on "
                       f"({sorted(pats)}), or say in `new` why the audience wants something none of them gave")
    ad = d.get("adaptations")
    if ad is None:
        r.info("RS9", "research.adaptations", "adaptations: if the topic already has a look viewers know (an "
               "animation, a drama, a comic, a game, a mascot), list it; drawn characters must match it")
    elif not isinstance(ad, list) or any(not (isinstance(x, dict) and x.get("title") and x.get("kind")) for x in ad):
        r.fail("RS9", "research.adaptations", "a list of {title, kind, url, year}")
    lore = d.get("lore_terms")
    if lore is None:
        r.warn("RS7", "research.lore_terms", "list the insider words a stranger does not know (jargon, product terms, "
               "names, world terms); mvf lint flags them in the hook and the first half")
    elif not isinstance(lore, list) or not all(isinstance(x, str) and x.strip() for x in lore):
        r.fail("RS7", "research.lore_terms", "a list of words")
    want = max(MIN["ideas"], 2 * scripts)
    if padded:
        r.warn("RS5", "research.ideas", f"ideas {', '.join(map(str, padded))} are filler (fields like 'x' or a few "
               "words): a bank padded to the count compares nothing; fewer real ideas beat more empty ones")
    if len(ideas) < want:
        r.warn("RS5", "research.ideas", f"{len(ideas)} ideas; write about {want} real ones (twice the scripts) before "
               "picking, never filler to reach the count")
    if not picked:
        r.fail("RS6", "research.ideas", "no idea has pick: true")
    elif len(picked) > scripts:
        r.warn("RS6", "research.ideas", f"{len(picked)} ideas picked for {scripts} scripts: pick the ideas the "
               "candidate scripts stand on, at most one per script")
    return r


def _top_videos(d: dict, r: Report) -> set:
    """What the most-viewed videos are about and what they tell the viewer, read from their transcripts."""
    tops = d.get("top_videos") or []
    for i, v in enumerate(tops):
        w = f"top_videos[{i}] {v.get('id', '?')}"
        if not str(v.get("url", "")).startswith(("http://", "https://")):
            r.fail("RS8", w, "url of the video")
        if not isinstance(v.get("views"), int):
            r.fail("RS8", w, "views: the measured number (int)")
        if v.get("scope") not in {"topic", "benefit"}:
            r.warn("RS8", w, "scope: topic (about the subject) or benefit (gives this audience the same kind of benefit)")
        for k, what in (("subject", "the viewer's problem or question it is about, in their words"),
                        ("says", "what it tells the viewer: the answer, the claim, the advice"),
                        ("moves", "the order it says things in")):
            if len(str(v.get(k, "")).strip()) < 10:
                (r.warn if k == "moves" else r.fail)("RS8", w, f"{k}: {what}")
        if not str(v.get("transcript", "")).strip():
            r.fail("RS8", w, "transcript: the file `mvf top-videos` wrote, or what you heard when it has no subtitles; "
                   "a title and a hook are not what it says")
    if not tops:
        (r.warn if str(d.get("top_videos_note", "")).strip() else r.fail)(
            "RS8", "research.top_videos", "no top videos read: read what the most-viewed videos say before any idea "
            "(`mvf top-videos`), or say in top_videos_note why none can be read")
    elif len(tops) < MIN["top_videos"]:
        r.warn("RS8", "research.top_videos", f"{len(tops)} top videos studied; {MIN['top_videos']} give a pattern")
    if tops and sum(1 for v in tops if v.get("scope") == "benefit") < MIN["benefit_videos"]:
        r.warn("RS8", "research.top_videos", f"fewer than {MIN['benefit_videos']} benefit-scope videos: also read the "
               "most-viewed videos that give this audience the benefit you promise, not only the ones about the topic")
    pats = d.get("patterns") or []
    ids = set()
    for i, p in enumerate(pats):
        ids.add(p.get("id"))
        if len(str(p.get("what", "")).strip()) < 10 or not p.get("videos"):
            r.warn("RS8", f"patterns[{i}]", "what (what the top videos share: the problem, the answer, the order) and "
                   "videos (the top_videos ids that show it)")
    if tops and len(pats) < MIN["patterns"]:
        r.warn("RS8", "research.patterns", f"{len(pats)} patterns; write at least {MIN['patterns']}: what the "
               "most-viewed videos are about and how they give it, ranked by views")
    return ids


def _pattern_views(d: dict) -> dict:
    """Each pattern's best measured views and that video's scope, from the top videos it names (by id or url)."""
    views = {}
    for v in d.get("top_videos") or []:
        if isinstance(v.get("views"), int):
            for k in (v.get("id"), v.get("url")):
                if k:
                    views[k] = (v["views"], v.get("scope"))
    out = {}
    for p in d.get("patterns") or []:
        seen = [views[k] for k in p.get("videos") or [] if k in views]
        if p.get("id") and seen:
            out[p["id"]] = max(seen)
    return out


def ideas(d: dict) -> dict:
    return {x["id"]: x for x in d.get("ideas") or [] if x.get("id")}
