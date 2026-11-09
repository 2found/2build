"""Script lint: structure, hook, length, claims, ad-feel, on-screen text and
per-platform copy. Measured on the timeline (estimated or from the take)."""
from __future__ import annotations

import json
import math
import re

from . import craft, timing
from .checks import trend_age, STALE_DAYS
from .common import APPLY_BENEFITS, CAMPAIGN_PLATFORMS, ID_RE, LIB, PLATFORMS, Report, copy_kind, family, hashtags, words, x_weight
from .material import RIGHTS, SHOWABLE, SHOWN, SIGNAL_DAYS, signal_age

RECIPES = json.loads((LIB / "recipes.json").read_text(encoding="utf-8"))["recipes"]

ROLES = {"hook", "context", "problem", "tension", "turn", "demo", "proof", "payoff", "reaction", "beat", "apply", "cta",
         "end"}
FORMATS = {"problem-solution", "demo", "pov", "before-after", "story", "listicle", "myth-buster", "reaction",
           "tutorial", "testimonial", "challenge", "behind-the-scenes", "comparison", "launch", "meme", "skit",
           "explainer"}  # references/formats.md
VOICE_MODES = {"recorded", "clone", "design", "preset", "none"}
# Roles that give the viewer something (a step, a result, a punchline); setup roles do not.
VALUE_ROLES = {"turn", "demo", "proof", "payoff", "reaction", "beat", "apply"}
# The viewer addressed in the apply beat: the point lands on them, not on a character.
YOU = re.compile(r"\b(bạn|you|your)\b", re.I)

# A claim a viewer could hold us to: quantities, prices, superlatives, guarantees, "free".
# A bare ordinal ("Chapter 1", "Part 2") is not a claim; a number with a unit, a
# multiplier, a currency or a percent is.
QTY = (r"[$€£¥₫]\s*\d|\d[\d.,]*\s*(%|x\b|×|k\b|m\b|tr\b|đ\b|usd|vnd|vnđ|\$|lần|times|million|billion|thousand|"
       r"triệu|nghìn|ngàn|tỷ|users?|people|customers?|teams?|downloads?|installs?|reviews?|người|readers?|chapters?|chương|characters?|ký\s*tự|chars?|words?|chữ|"
       r"seconds?|giây|minutes?|phút|hours?|giờ|days?|ngày|languages?|ngôn\s*ngữ|novels?|truyện|stars?|sao)")
CLAIM = re.compile(QTY + r"|%|\b((tốt|nhanh|rẻ|hay|mạnh|dễ|chuẩn|đỉnh)\s*nhất|số\s*1|duy\s*nhất|miễn\s*phí|đảm\s*bảo|"
                   r"cam\s*kết|gấp\s*\w+\s*lần|"
                   r"best|fastest|cheapest|the\s+only|guarantee\w*|free|#1|no\.?\s*1|"
                   r"times\s+(faster|better|cheaper))\b", re.I)
# Selling words that make a native clip read as an ad when they appear before the CTA.
SALES = re.compile(r"\b(tải\s*app|tải\s*ngay|tải\s*ứng\s*dụng|đăng\s*ký\s*ngay|link\s*(ở\s*)?bio|bấm\s*vào|"
                   r"giảm\s*giá|ưu\s*đãi|khuyến\s*mãi|mã\s*giảm|"
                   r"download\s*now|sign\s*up\s*now|link\s*in\s*bio|click\s*the\s*link|discount|promo\s*code|"
                   r"limited\s*time|buy\s*now|use\s*code|subscribe\s*now)\b", re.I)
WEAK_OPENER = re.compile(r"^\s*(xin\s*chào|chào\s*(các|cả)\s*nhà|chào\s*mọi\s*người|hôm\s*nay\s*(mình|tôi|chúng\s*ta)|"
                         r"trong\s*video\s*này|bạn\s*có\s*biết|"
                         r"hey\s*(guys|everyone)|hi\s*(guys|everyone)|hello\s*(guys|everyone)|welcome\s*(back|to)|"
                         r"in\s*this\s*video|today\s*(i'?m|we'?re|i\s*will)|so\s*,?\s*today|did\s*you\s*know)", re.I)
# Spelled numbers match digits, so "top mười" names the search term "top 10".
NUM_WORDS = {w: str(i) for words_ in ("không một hai ba bốn năm sáu bảy tám chín mười", "zero one two three four five six "
             "seven eight nine ten") for i, w in enumerate(words_.split())}
URL = re.compile(r"https?://|\b\w+\.(com|net|org|io|dev|ai|app|vn)\b", re.I)
# Function words carry no content: two lines share meaning when they share content words, not these.
STOP = set("""và là của có không thì mà cho với được này đó những các một cái như khi đã sẽ đang ra vào lại cũng rất
nó họ mình bạn ta tôi chúng người thế vậy nào gì đâu ở trong trên dưới để vì nên hay hoặc nếu rồi chỉ còn từ đến
the a an and or of to in on at is are was were be been it its this that these those for with as by from you your
i we they he she them his her not no do does did so but if then than just very can will would""".split())


def _grams(text: str) -> tuple[set, set]:
    """Content words and the word pairs around them (a Vietnamese word is often two syllables)."""
    w = [NUM_WORDS.get(x, x) for x in re.findall(r"\w+", (text or "").lower())]
    uni = {x for x in w if x not in STOP and len(x) > 1}
    bi = {f"{a} {b}" for a, b in zip(w, w[1:]) if a not in STOP or b not in STOP}
    return uni, bi


def _says(part: str, heard: tuple[set, set]) -> bool:
    """Does the heard text carry this part's content? A heuristic: a paraphrase can miss, so it only WARNs."""
    uni, bi = _grams(part)
    if len(bi) >= 3:
        return len(bi & heard[1]) >= 0.5 * len(bi) or len(uni & heard[0]) >= 0.75 * len(uni)
    return bool(uni) and len(uni & heard[0]) >= 0.6 * len(uni)


def _carries(vo: str, spine: dict) -> bool:
    """A beat gives value when it says part of the use, the takeaway or the payoff, whatever its role label."""
    uni, bi = _grams(vo)
    for k in ("use", "takeaway", "payoff"):
        u2, b2 = _grams(str(spine.get(k) or ""))
        if len(bi & b2) >= 2 or (b2 and len(uni & u2) >= 4):
            return True
    return False


def _brand_re(names: list[str]) -> re.Pattern | None:
    names = [n for n in names if n.strip()]
    if not names:
        return None
    return re.compile(r"(?<!\w)(" + "|".join(re.escape(n) for n in sorted(names, key=len, reverse=True)) + r")(?!\w)", re.I)


def lint(script: dict, campaign: dict, refs: dict, manifest: dict | None = None, hook: str | None = None) -> Report:
    """refs: {"facts": set, "trends": {id: trend}, "angles": {id: angle}, "segments": {id: seg},
    "material": {id: item} (the campaign's 00-material pack; absent = not checked),
    "recipes": {name: recipe} (built-in + extensions; absent = recipes.json)}."""
    r = Report()
    sid = script.get("id", "?")
    if script.get("_template"):
        r.fail("S0", sid, "still the template: write the script, then delete the \"_template\" key")
    if not ID_RE["script"].match(str(sid)):
        r.fail("S0", "script.id", f"{sid!r} must look like s01, s01-short-name, or ctl-short-name for a control")
    beats = script.get("beats") or []
    if len(beats) < 3:
        r.fail("S1", sid, "a short needs at least hook → body → payoff/CTA (3 beats)")
        return r
    seen = set()
    for b in beats:
        if not ID_RE["beat"].match(str(b.get("id"))) or b["id"] in seen:
            r.fail("S1", sid, f"beat id {b.get('id')!r} must be unique B01, B02, …")
        seen.add(b.get("id"))
        if b.get("role") not in ROLES:
            r.warn("S1", f"{sid}.{b.get('id')}", f"role {b.get('role')!r} is not one the checks know ({sorted(ROLES)}): "
                   "fine if it is yours, but value, setup and CTA timing are then not measured on it")
    if beats[0].get("role") != "hook":
        r.fail("S1", f"{sid}.{beats[0].get('id')}", "the first beat is the hook (role: hook)")
    if not str(script.get("value", "")).strip():
        r.fail("S2", sid, "value: what the viewer walks away with and can use")
    if script.get("format") not in FORMATS:
        r.warn("S2", sid, f"format {script.get('format')!r} is not a listed one ({sorted(FORMATS)}); a new format is fine, name it plainly")
    voice = script.get("voice") or {}
    if voice.get("mode") not in VOICE_MODES:
        r.fail("S2", sid, f"voice.mode must be one of {sorted(VOICE_MODES)}")
    if voice.get("mode") == "clone" and not voice.get("ref"):
        r.fail("S2", sid, "voice.ref names the reference voice (<campaign>/voices/ or .babysit/marketing-video/voices/)")
    if voice.get("mode") == "design" and not voice.get("instruct"):
        r.fail("S2", sid, "voice.instruct, e.g. 'female, young adult, moderate pitch, american accent'")
    if voice.get("mode") == "preset" and not voice.get("name"):
        r.fail("S2", sid, "voice.name: the engine's stock voice to use")
    angle = refs.get("angles", {}).get(script.get("angle"))
    if script.get("angle") and not angle:
        r.fail("S2", sid, f"angle {script.get('angle')} is not in angles.json")
    for v in script.get("variants") or []:
        if not ID_RE["hook"].match(str(v.get("id"))) or v.get("id") == "H1":
            r.fail("S1", sid, f"variant id {v.get('id')!r}: H2…H9 (H1 is beat B01)")

    variants = [None] + [v.get("id") for v in script.get("variants") or []] if hook is None else [hook]
    brand = _brand_re((campaign.get("brand") or {}).get("names") or [])
    brand_names = (campaign.get("brand") or {}).get("names") or []
    seg = refs.get("segments", {}).get((angle or {}).get("segment")) or {}
    seg_terms = [t for net in (seg.get("platforms") or {}).values() if isinstance(net, dict)
                 for t in net.get("search_terms") or []]
    refs = {**refs, "brand_names": brand_names,
            "known_words": [*(campaign.get("known_words") or []), *brand_names, *seg_terms]}
    platforms = [p for p in (script.get("platforms") or {}) if p in PLATFORMS]
    if not platforms:
        r.fail("P0", sid, f"platforms: give at least one of {' / '.join(CAMPAIGN_PLATFORMS)} with its copy")
    for p in script.get("platforms") or {}:
        if p not in PLATFORMS:
            r.fail("P0", sid, f"unknown platform {p}")

    for hv in variants:
        s = timing.variant(script, hv)
        tl = timing.build(s, manifest)
        tag = f"{sid}[{s['beats'][0]['variant']}]"
        _hook(s, tl, tag, platforms, brand, r)
        ring = _ring(refs, angle)
        if ring in OUTER_RINGS:
            _outsider_hook(s, tag, refs, r)
        else:
            _topic(s, tag, brand, _search_terms(refs, angle, platforms), r)
        _first_value(s, tl, tag, platforms, r)
        _length(tl, tag, platforms, r)
        _tension(s, tag, r)
    tl = timing.build(timing.variant(script, None), manifest)
    _body(script, tl, sid, brand, refs, campaign, r)
    _story(script, tl, sid, angle, refs, r)
    _layers(script, tl, sid, _ring(refs, angle), refs, campaign, r)
    _benefit(script, tl, sid, angle, refs, campaign, r)
    if not str(sid).startswith("ctl-"):  # a control is a rejected script on purpose, often an old one
        _repeat(script, sid, refs, r)
    for p in platforms:
        _copy(script, p, sid, angle, refs, r)
    if "youtube-long" in platforms:
        _chapters(script["platforms"]["youtube-long"] or {}, tl, f"{sid}.youtube-long", r)
    _material(script, sid, refs, r)
    craft.check(script, tl, sid, refs, r, control=str(sid).startswith("ctl-"))
    r.info("T0", sid, f"{tl['timing']} runtime {tl['duration']:.1f}s over {len(beats)} beats")
    _answered(script, r)
    return r


def _answered(script: dict, r: Report):
    """A WARN is advice to follow or answer in writing. script.warn_notes holds the answers, keyed by the code
    ("V2") or by the code and where it fired ("U1 s01[H2]"); an answered WARN stays visible as INFO."""
    notes = script.get("warn_notes") or {}
    if not isinstance(notes, dict):
        r.warn("S2", script.get("id", "?"), "warn_notes is {\"CODE\" or \"CODE where\": \"why this advice is not followed\"}")
        return
    for it in r.items:
        if it["level"] != "WARN":
            continue
        why = notes.get(f"{it['code']} {it['where']}") or notes.get(it["code"])
        if str(why or "").strip():
            it["level"], it["msg"] = "INFO", f"answered: {str(why).strip()} (was: {it['msg']})"


def _hook(s, tl, tag, platforms, brand, r: Report):
    b0, t0 = s["beats"][0], tl["beats"][0]
    vo, on = (b0.get("vo") or "").strip(), (b0.get("onscreen") or "").strip()
    if not vo and not on:
        r.fail("H0", tag, "the hook has neither voice nor on-screen text")
    limit = min(PLATFORMS[p]["recommend"]["hook_s"] for p in platforms) if platforms else 2.5
    if vo:
        end = t0["vo_end"]
        if end > limit + 1.0:
            r.fail("H1", tag, f"hook line ends at {end:.1f}s ({t0['timing']}); land it by {limit}s — cut words, never speed up")
        elif end > limit:
            r.warn("H1", tag, f"hook line ends at {end:.1f}s ({t0['timing']}); aim for ≤ {limit}s")
        if WEAK_OPENER.search(vo):
            r.warn("H2", tag, f"weak opener {WEAK_OPENER.search(vo).group(0)!r}: start on the picture, the stake or the result")
    if brand and brand.search(f"{vo} {on}"):
        r.warn("H3", tag, "the brand is in the hook: viewers swipe ads; open on the viewer's problem or the result "
               "(showing the product in the picture is fine)")
    muted = [PLATFORMS[p]["label"] for p in platforms if PLATFORMS[p].get("sound_default") == "off"]
    if not on and muted:
        r.fail("H4", tag, f"{', '.join(muted)} autoplay muted: the hook needs on-screen text")
    elif not on:
        r.warn("H4", tag, "no on-screen hook text: many TikTok viewers watch muted at first")
    elif len(words(on)) > 9:
        r.warn("H4", tag, f"hook text is {len(words(on))} words; ≤ 7 reads in one glance")
    if not str(b0.get("visual", "")).strip():
        r.fail("H5", tag, "frame 0 needs a visual: the thumbnail is the first frame (a face, the result, motion)")
    elif re.search(r"\blogo\b|title card|\bblack\s+(screen|frame|card|background)\b|^\W*black\b|màn\s*đen", b0["visual"], re.I):
        r.warn("H5", tag, "frame 0 shows a logo/title/black: open on the subject in motion")


# A hook that only names the topic gives no reason to stay: it needs a question, a number,
# a contradiction or a stake.
TENSION = re.compile(r"\?|\d|\b(vì\s*sao|tại\s*sao|sao\s*lại|nhưng|mà\s*lại|không\s*ai|chưa\s*ai|ai\s*ngờ|"
                     r"hóa\s*ra|thật\s*ra|sự\s*thật|sai|chết|mất|đừng|duy\s*nhất|bí\s*mật|lần\s*cuối|không\s*phải|chẳng\s*phải|"
                     r"hay\s*là|nên\s*hay|bỏ\s*hay|"
                     r"why|but|never|nobody|no\s*one|wrong|only|secret|actually|turns?\s*out|dies?|lost?|stop|not|isn'?t|"
                     r"don'?t|or\s*not|should\s*you)\b", re.I)
SETUP_ROLES = {"beat", "context"}


def _tension(s, tag, r: Report):
    b0 = s["beats"][0]
    text = f"{b0.get('vo') or ''} {b0.get('onscreen') or ''}"
    if text.strip() and not TENSION.search(text) and not any(w in NUM_WORDS for w in words(text.lower())):
        r.warn("H7", tag, "the hook names a topic but gives no reason to stay: a question, a stake, a contradiction, "
               "a number or a scene that makes a stranger need what comes next. Keep it specific; a generic "
               "question is weaker than a specific claim")


OUTER_RINGS = {"broad", "genre"}
SENT_END = re.compile(r"[.!?…:;\"“”'‘’()\[\]—–\-·•|=/→+]\s*$")  # on-screen lists split on · | = → too


def _ring(refs: dict, angle: dict | None) -> str | None:
    return (refs.get("segments", {}).get((angle or {}).get("segment")) or {}).get("ring")


def _names(text: str) -> list[str]:
    """Runs of capitalised words: the names a stranger has not met yet. One capital at a sentence start is just
    the sentence; a run of two or more there ("Quỷ Bí Chi Chủ: …") is still a name."""
    runs, last, start = [], -2, 0
    for line in re.split(r"\n+", text or ""):
        for m in re.finditer(r"\b\w+", line):
            w = m.group(0)
            if not (w[0].isupper() and not w.isupper()):
                last = -2
                continue
            if last == m.start() - 1 and runs:  # a run of capitals is one name
                runs[-1][0].append(w)
            else:
                runs.append([[w], not m.start() or bool(SENT_END.search(line[:m.start()]))])
            last = m.end()
        last = -2
    out = []
    for ws, at_start in runs:
        if at_start and ws[0].lower() in STOP:  # "Trong Cổ Chân Nhân": the sentence's first word, then the name
            ws = ws[1:]
        elif at_start and len(ws) == 1:
            continue
        if ws:
            out.append(" ".join(ws))
    return out


def _plain(x: str) -> str:
    return f" {' '.join(re.findall(r'\w+', x.lower()))} "


def _lore(text: str, refs: dict) -> list[str]:
    """The words a stranger may not know: research.lore_terms and names, minus what the audience already knows
    (campaign.known_words, the segment's search terms, the brand)."""
    low = _plain(text)
    known = [_plain(k) for k in refs.get("known_words") or [] if str(k).strip()]
    hits = [x for x in refs.get("lore_terms") or [] if _plain(x) in low and _plain(x) not in known]
    return hits + [n for n in _names(text) if not any(n.lower() in h.lower() for h in hits)
                   and not any(_plain(n) in k for k in known)]


def _lore_beat(b: dict, refs: dict) -> list[str]:
    """vo and on-screen text are separate lines: each starts its own sentence."""
    out = _lore(b.get("vo") or "", refs)
    return out + [w for w in _lore(b.get("onscreen") or "", refs) if w.lower() not in {x.lower() for x in out}]


def _outsider_hook(s, tag, refs, r: Report):
    """A hook for people who do not know the topic is in their words: an everyday situation, not the title,
    the hero's name or a world term. The title and search terms go in the caption and the end."""
    b0 = s["beats"][0]
    hits = _lore_beat(b0, refs)
    if hits:
        r.warn("U1", tag, f"the hook uses {sorted(set(hits))}: this script is for viewers who do not know the topic, and a "
               "word they do not know reads as 'not for me'. Say the situation in everyday words; "
               "the name or the term can come later. A word this audience already knows (a place, the title they "
               "search for) goes in campaign.known_words")


def _layers(script, tl, sid, ring, refs, campaign, r: Report):
    """One story, read at several depths: what a stranger gets, what a genre fan gets, what a fan gets."""
    ly = script.get("layers")
    if not isinstance(ly, dict) or not any(str(v).strip() for v in ly.values()):
        r.warn("N6", f"{sid}.layers", "layers: what a stranger (broad), a fan of the genre (genre) and a fan of this "
               "very topic (core) each get from the video (references/05-script.md#several-audiences)")
    elif campaign.get("goal") in {"awareness", "follow", "launch"} and not any(
            str(ly.get(k, "")).strip() for k in OUTER_RINGS):
        r.warn("N6", f"{sid}.layers", "only the core layer is filled: a reach goal needs something for viewers who do "
               "not know the topic yet")
    if ring not in OUTER_RINGS:
        return
    half = tl["duration"] / 2
    seen = []
    for b, t in zip(script.get("beats") or [], tl["beats"]):
        if t["start"] >= half:
            break
        for w in _lore_beat(b, refs):
            if w.lower() not in {x.lower() for x in seen}:
                seen.append(w)
    if len(seen) > 2:
        r.warn("U2", sid, f"{len(seen)} new words before half-way ({', '.join(seen[:6])}): a stranger holds one new "
               "word at a time. Give each word the answer needs its plain meaning when it is first heard, and cut "
               "the rest; never keep a word and drop its meaning")


def _benefit(script, tl, sid, angle, refs, campaign, r: Report):
    """What the viewer can use afterwards, said about their life. A story, a twist or a debate holds a viewer; the
    benefit is what they leave with (references/05-script.md#the-benefit)."""
    sp = script.get("spine") or {}
    use = str(sp.get("use", "")).strip()
    if not use:
        r.fail("W1", f"{sid}.spine", "use is empty: what the viewer can do, decide or think afterwards in their own "
               "life, one sentence (for an answer: the answer they can now repeat). Retelling the story is not a use")
    _retell(script, sid, sp, r)
    kind = ((angle or {}).get("benefit") or {}).get("type") or campaign.get("benefit")
    # An answer is about the topic, so it names the topic's things; only advice must survive the name deletion.
    for k in ("use", "takeaway") if kind != "answer" else ():
        hits = _lore(str(sp.get(k) or ""), refs)
        if hits:
            r.warn("W2", f"{sid}.spine.{k}", f"names {sorted(set(hits))}: a takeaway that needs insider words is a fact "
                   "about the topic. Delete them and it must still help someone who never meets the topic")
    if kind not in APPLY_BENEFITS:
        return
    beats = script.get("beats") or []
    at = [(b, t) for b, t in zip(beats, tl["beats"]) if b.get("role") == "apply"]
    if not at:
        r.warn("W3", sid, f"no beat is marked apply: where does this {kind} video hand the viewer the use, in their "
               "own situation? Mark that beat so its timing is measured")
        return
    b, t = at[0]
    if t["start"] > 0.8 * tl["duration"]:
        r.warn("W3", f"{sid}.{b['id']}", f"the first apply beat starts at {t['start']:.1f}s of {tl['duration']:.1f}s: "
               "most viewers are gone by then; give the use by two thirds, then prove or deepen it")
    if script.get("language", "vi")[:2] in {"vi", "en"} and not any(YOU.search(x.get("vo") or "") for x, _ in at):
        r.warn("W4", f"{sid}.{b['id']}", "the apply beat never speaks to the viewer (\"bạn\" / \"you\"): the point "
               "may still be about the subject, not about them")


REPEAT_FAIL, REPEAT_WARN = 0.5, 0.3  # share of this script's word triples heard in one earlier script


def _triples(text: str) -> set:
    w = re.findall(r"\w+", (text or "").lower())
    return {tuple(w[i:i + 3]) for i in range(len(w) - 2)}


def _repeat(script, sid, refs, r: Report):
    """The owner has already shipped what an earlier campaign said. Unrelated scripts share under a fifth of their
    word triples; a remake shares over half."""
    mine = _triples(" ".join(b.get("vo") or "" for b in script.get("beats") or []))
    if not mine:
        return
    for where, vo in refs.get("past") or []:
        share = len(mine & _triples(vo)) / len(mine)
        if share >= REPEAT_FAIL:
            r.fail("RP1", sid, f"{share:.0%} of its lines are already in {where}: that video was made; "
                   "find what it never said")
        elif share >= REPEAT_WARN:
            r.warn("RP1", sid, f"{share:.0%} of its lines are already in {where}: say what this one adds")


def _retell(script, sid, sp, r: Report):
    """The retell is what a stranger says after one hearing, so the vo has to say it. Word overlap is only a
    proxy for meaning: a paraphrase can trip it, and a script can pass it and still be unclear to a listener."""
    take = str(sp.get("takeaway") or "").strip()
    if not take:
        return
    heard = _grams(" ".join(b.get("vo") or "" for b in script.get("beats") or []))
    parts = [x.strip() for x in re.split(r"[.;:!?]+|\s[—–]\s", take) if len(_grams(x)[0]) >= 2]
    missing = [x for x in parts if not _says(x, heard)]
    if missing:
        r.warn("W5", f"{sid}.spine.takeaway", f"the vo may not say this part of the retell: {missing[:3]}. A stranger "
               "only retells what they heard; say it in the vo (if the vo says it in other words, answer this in "
               "warn_notes)")


def _story(script, tl, sid, angle, refs, r: Report):
    """The spine: one question the viewer stays to answer, loops that keep a question open
    from the hook to the payoff, and no run of beats that only lists facts."""
    sp = script.get("spine") or {}
    for k in ("question", "stakes", "turn", "payoff", "takeaway"):
        if not str(sp.get(k, "")).strip():
            r.warn("N1", f"{sid}.spine", f"{k} is empty: say what keeps the viewer watching and what they get at the end, "
                   "whatever shape the video takes")
    cap = 45 if str(script.get("language", ""))[:2] == "vi" else 30  # a Vietnamese word is often two syllables
    if len(words(sp.get("takeaway") or "")) > cap:
        r.warn("N1", f"{sid}.spine", f"takeaway over {cap} words: the retell a stranger gives is a sentence or two")
    ideas = refs.get("ideas")
    if ideas is not None and angle and angle.get("idea") and angle["idea"] not in ideas:
        r.fail("N1", sid, f"angle idea {angle['idea']} is not in 03-research/research.json")
    beats = script["beats"]
    pos = {b["id"]: i for i, b in enumerate(beats)}
    loops = sp.get("loops") or []
    if not loops:
        r.warn("N2", f"{sid}.spine", "no loops: name what the viewer is waiting for and where it lands, so the "
               "pull can be measured; or say how this video holds attention instead")
        return
    spans, events = [], []
    for lp in loops:
        w = f"{sid}.spine.{lp.get('id', '?')}"
        o, c = pos.get(lp.get("open")), pos.get(lp.get("close"))
        if o is None or c is None or c <= o:
            r.warn("N2", w, "open and close name existing beats, open before close")
            continue
        spans.append((tl["beats"][o]["start"], tl["beats"][c]["start"]))
        events += [tl["beats"][o]["start"], tl["beats"][c]["start"]]
        lp["_o"], lp["_c"] = o, c
    if not spans:
        return
    first = min(lp.get("_o", 99) for lp in loops)
    if first > 1:
        r.warn("N2", sid, "nothing is open by B02: what makes the viewer stay past the hook?")
    last_close = max(s1 for _, s1 in spans)
    if last_close < 0.6 * tl["duration"]:
        r.warn("N2", sid, f"the last open question closes at {last_close:.1f}s of {tl['duration']:.1f}s: after that "
               "the viewer has nothing left to wait for. Keep one question open to the end (the proof, the condition, "
               "the next step), never by holding back the answer the hook promised")
    # A question the narrator asks holds the viewer until the next beat answers it; a beat that lands part of
    # the answer moves the viewer on. Both count, so nobody pads the vo with rhetorical questions for the check.
    for i, b in enumerate(beats[1:-1], 1):
        if "?" in (b.get("vo") or ""):
            spans.append((tl["beats"][i]["start"], tl["beats"][i + 1]["start"]))
            events.append(tl["beats"][i]["start"])
        if b.get("role") in VALUE_ROLES or _carries(b.get("vo") or "", sp):
            events.append(tl["beats"][i]["start"])
    t, gaps = tl["beats"][0]["end"], []
    for a0, a1 in sorted(spans):
        if a0 > t + 3.0:
            gaps.append((t, a0))
        t = max(t, a1)
    # a question stays open until the payoff (or the last body beat when there is none)
    body = [i for i, b in enumerate(beats) if b.get("role") not in {"cta", "end"}]
    pay = [i for i in body if beats[i].get("role") == "payoff"]
    target = tl["beats"][(pay or body)[-1]]["start"]
    if target > t + 3.0:
        gaps.append((t, target))
    for g0, g1 in gaps:
        r.warn("N3", sid, f"{g0:.1f}–{g1:.1f}s: no question is open; that is where viewers swipe; open the next one "
               "before you close the last")
    ev = sorted(set(events) | {0.0, last_close})
    for e0, e1 in zip(ev, ev[1:]):
        if e1 - e0 > 12:
            r.warn("N4", sid, f"{e0:.1f}–{e1:.1f}s: {e1 - e0:.0f}s with no question opened or answered and nothing "
                   "learned; re-hook, or name the scene the viewer is waiting through as a loop")
    touched = {lp.get("_o") for lp in loops} | {lp.get("_c") for lp in loops}
    run = []
    for i, b in enumerate(beats + [{"role": "end"}]):
        if b.get("role") in SETUP_ROLES and i not in touched:
            run.append(b.get("id"))
            continue
        if len(run) >= 3:
            r.warn("N5", sid, f"{run[0]}–{run[-1]}: {len(run)} beats in a row that state facts and move no question: "
                   "this reads like a wiki; turn one into a stake, a surprise or a scene")
        run = []


def _first_value(s, tl, tag, platforms, r: Report):
    limit = min(PLATFORMS[p]["recommend"]["first_value_s"] for p in platforms) if platforms else 6.0
    spine = s.get("spine") or {}
    first = next(((b, t) for b, t in zip(s["beats"][1:], tl["beats"][1:])
                  if b.get("role") in VALUE_ROLES or (b.get("role") not in {"cta", "end"}
                                                      and _carries(b.get("vo") or "", spine))), None)
    if not first:
        r.warn("R1", tag, "no beat says part of spine.use / takeaway / payoff and none has a value role "
               f"({sorted(VALUE_ROLES)}): the first-value timing is not measured")
        return
    b, t = first
    if t["start"] > limit + 2.0:
        r.fail("R1", tag, f"first value ({b['id']} {b.get('role')}) starts at {t['start']:.1f}s ({t['timing']}): "
               f"most viewers are gone; cut setup so it lands by {limit}s")
    elif t["start"] > limit:
        r.warn("R1", tag, f"first value ({b['id']} {b.get('role')}) starts at {t['start']:.1f}s ({t['timing']}); "
               f"aim for ≤ {limit}s")


def _length(tl, tag, platforms, r: Report):
    d = tl["duration"]
    for p in platforms:
        hard, rec = PLATFORMS[p]["hard"], PLATFORMS[p]["recommend"]
        label = PLATFORMS[p]["label"]
        if d > hard["max_s"] or d < hard["min_s"]:
            r.fail("L1", tag, f"{d:.1f}s is outside {label}'s {hard['min_s']}–{hard['max_s']}s limit")
        elif rec.get("reach_max_s") and d > rec["reach_max_s"]:
            r.warn("L1", tag, f"{d:.1f}s: {label} recommends only videos up to {rec['reach_max_s']}s to people who do "
                   "not follow you")
        elif d > rec["max_s"] or d < rec["min_s"]:
            r.warn("L1", tag, f"{d:.1f}s is outside {label}'s {rec['min_s']}–{rec['max_s']}s sweet spot; "
                   "keep it only if every beat earns its time")


def _body(script, tl, sid, brand, refs, campaign, r: Report):
    beats = script["beats"]
    facts, trends = refs.get("facts", set()), refs.get("trends", {})
    cta_idx = [i for i, b in enumerate(beats) if b.get("role") == "cta"]
    if not cta_idx:
        (r.info if campaign.get("goal") in {"awareness", "follow"} else r.warn)(
            "S3", sid, "no CTA beat: say the one thing to do next (follow, search, open, sign up)")
    elif len(cta_idx) > 1:
        r.warn("S3", sid, f"{len(cta_idx)} CTA beats: one ask, once, at the end")
    else:
        at = tl["beats"][cta_idx[0]]["start"] / max(tl["duration"], 0.01)
        if at < 0.7:
            r.warn("S3", sid, f"CTA starts at {at:.0%} of the runtime; earn it first (≥ 70%)")
    brand_beats = []
    for b, t in zip(beats, tl["beats"]):
        where = f"{sid}.{b['id']}"
        text = " ".join(str(b.get(k) or "") for k in ("vo", "onscreen"))
        claim = CLAIM.search(brand.sub("", text) if brand else text)  # "Acme2" is a name, not a number
        if claim and not b.get("facts") and not _signals(b, refs, where, r):
            r.fail("C1", where, f"claim {claim.group(0)!r} needs a fact id (facts: [F…]) or a measured item (material: […])")
        for fid in b.get("facts") or []:
            if fid not in facts:
                r.fail("C1", where, f"unknown fact {fid}")
        if b.get("role") not in {"cta", "end"} and SALES.search(text):
            r.warn("C2", where, f"sales talk {SALES.search(text).group(0)!r} before the CTA reads as an ad")
        if brand and b.get("role") not in {"cta", "end", "hook"} and brand.search(b.get("vo") or ""):
            brand_beats.append(b["id"])
        if not str(b.get("visual", "")).strip():
            r.warn("V1", where, "visual is empty: say what the viewer sees (the real screen, the person, the object)")
        dur = t["end"] - t["start"]
        shots = max(1, len(b.get("shots") or []))
        if dur / shots > 4.0:
            r.warn("V2", where, f"{dur:.1f}s on {shots} shot(s): plan a visible change about every 3.5s (cut, push, "
                   "reveal). List the planned changes in the beat's `shots`; never split a sentence to pass this")
        on = words(b.get("onscreen") or "")
        if on and len(on) > 3.0 * dur:
            r.warn("V3", where, f"{len(on)} on-screen words in {dur:.1f}s: unreadable; ≤ 3 words/s")
        for tid in b.get("trends") or []:
            _trend_ref(tid, trends, where, r)
        for aid in b.get("art") or []:
            job = (refs.get("art") or {}).get(aid)
            if not job:
                r.fail("A1", where, f"unknown art {aid} (07-video/art/art.json)")
            elif job.get("use") == "reference":
                r.fail("A1", where, f"art {aid} is a reference sheet: it is never shown")
    if len(brand_beats) > 1:
        r.warn("B1", sid, f"brand spoken in {len(brand_beats)} body beats ({', '.join(brand_beats)}): ad-feel; "
               "let the product appear on screen, say its name once")
    half = tl["duration"] / 2
    recipe = (refs.get("recipes") or RECIPES).get(script.get("recipe") or "") or {}
    if brand and recipe.get("brand") == "end":
        pass  # a story's product belongs at the CTA: a mid-story product card reads as an ad (cold reads, 2026-11)
    elif brand and not any(brand.search(f"{b.get('visual') or ''} {b.get('onscreen') or ''}")
                         for b, t in zip(beats, tl["beats"]) if t["start"] < half):
        r.warn("B2", sid, "the product is not seen in the first half: most viewers leave before the CTA; show it "
               "where the value happens and name it in that beat's visual")


def _trend_ref(tid, trends, where, r: Report):
    t = trends.get(tid)
    if not t:
        r.fail("TR", where, f"unknown trend {tid}")
        return
    if t.get("decision") == "avoid":
        r.fail("TR", where, f"trend {tid} is marked avoid")
    age = trend_age(t)
    if age is not None and age > STALE_DAYS and t.get("stage") in {"rising", "peak"}:
        r.warn("TR", where, f"trend {tid} observed {age} days ago: re-check it is still live")


def _copy(script, p, sid, angle, refs, r: Report):
    c = script["platforms"][p] or {}
    where = f"{sid}.{p}"
    hard, rec = PLATFORMS[p]["hard"], PLATFORMS[p]["recommend"]
    canvas = c.get("canvas", PLATFORMS[p]["canvas"])
    if canvas not in PLATFORMS["canvases"]:
        r.fail("P1", where, f"canvas {canvas!r} not in {sorted(PLATFORMS['canvases'])}")
    kind = copy_kind(p)
    if kind == "caption":
        _caption(c, p, where, _search_terms(refs, angle, [p]), refs, r)
    elif kind == "youtube":
        _youtube(c, p, where, _search_terms(refs, angle, [p]), r)
    else:
        post = c.get("post", "")
        if not post.strip():
            r.fail("P1", where, "post text is empty: on X the post sells the click into the video")
        w = x_weight(post)
        if w > hard["post_weight"] and not c.get("premium"):
            r.fail("P1", where, f"post weighs {w} > {hard['post_weight']} (set premium: true only for a Premium account)")
        n = len(hashtags(post))
        lo, hi = rec["hashtags"]
        if n > hi:
            r.warn("P2", where, f"{n} hashtags on X; ≤ {hi} — X reads hashtag walls as spam")
        for i, t in enumerate(c.get("thread") or []):
            if x_weight(t) > hard["post_weight"] and not c.get("premium"):
                r.fail("P1", f"{where}.thread[{i}]", f"reply weighs {x_weight(t)} > {hard['post_weight']}")
    copy = " ".join([c.get("caption", ""), c.get("post", ""), *(c.get("thread") or []), c.get("cover_text", ""),
                     c.get("title", ""), c.get("description", ""), c.get("thumbnail_text", "")])
    brand = _brand_re(refs.get("brand_names") or [])
    claim = CLAIM.search(brand.sub("", copy) if brand else copy)
    if claim and not c.get("facts") and not _signals(c, refs, where, r):
        r.fail("C3", where, f"claim {claim.group(0)!r} in the post copy needs facts: [F…] or material: […] on platforms.{p}")
    if brand and not sid.startswith("ctl") and not brand.search(copy):
        r.warn("AD1", where, f"the post copy never names {refs['brand_names'][0]}: a post that sells says who made it "
               "before it asks anything (open the caption with it, and turn on the platform's branded-content label); "
               "a viewer who finds out later reads it as seeding")
    for fid in c.get("facts") or []:
        if fid not in refs.get("facts", set()):
            r.fail("C3", where, f"unknown fact {fid}")
    cover = c.get("cover_text", "")
    if p == "youtube-long":
        pass  # the custom thumbnail is its cover (thumbnail_text above)
    elif not cover.strip():
        r.warn("P4", where, "cover_text: the profile-grid cover line (≤ 6 words)")
    elif len(words(cover)) > 6:
        r.warn("P4", where, f"cover_text is {len(words(cover))} words; ≤ 6 reads on a grid thumbnail")


def _caption(c: dict, p: str, where: str, terms: list[str], refs: dict, r: Report):
    """One caption under the video (TikTok, Instagram, Facebook, LinkedIn): its limits come from platforms.json."""
    P = PLATFORMS[p]
    hard, rec, label = P["hard"], P["recommend"], P["label"]
    cap = c.get("caption", "")
    if not cap.strip():
        r.fail("P1", where, "caption is empty")
    if len(cap) > hard["caption_chars"]:
        r.fail("P1", where, f"caption {len(cap)} chars > {hard['caption_chars']}")
    elif rec.get("caption_chars") and len(cap) > rec["caption_chars"]:
        r.warn("P1", where, f"caption {len(cap)} chars: {label} shows about the first {rec['caption_chars']} before "
               "'more'; put the promise and the search words first")
    n = len(hashtags(cap))
    lo, hi = rec["hashtags"]
    if hard.get("hashtags_max") is not None and n > hard["hashtags_max"]:
        r.fail("P2", where, f"{n} hashtags: {label} allows at most {hard['hashtags_max']}")
    elif not lo <= n <= hi:
        r.warn("P2", where, f"{n} hashtags; {lo}–{hi} specific ones beat a wall of generic tags")
    if URL.search(cap) and not P.get("links_clickable"):
        r.warn("P2", where, f"links in a {label} caption are not clickable: say where to go, or use the bio")
    if terms and not any(t in cap.lower() for t in terms):
        r.warn("P3", where, f"caption has none of the segment's search terms {terms[:4]}: {label} search reads it")
    sound = str(c.get("sound", ""))
    if sound.startswith("trend:"):
        _trend_ref(sound.split(":", 1)[1], refs.get("trends", {}), where, r)


def _search_terms(refs: dict, angle: dict | None, platforms: list[str]) -> list[str]:
    """The segment's search words for the first of these networks that has any, then TikTok's and YouTube's."""
    for net in [*(family(p) for p in platforms), "tiktok", "youtube"]:
        t = _terms(refs, angle, net)
        if t:
            return t
    return []


def _tokens(text: str) -> set[str]:
    return {NUM_WORDS.get(w, w) for w in re.findall(r"\w+", text.lower())}


def _topic(s, tag, brand, terms, r: Report):
    """Every hook names the topic in the segment's own search words: a viewer who hears only the hook must know
    what the video is about. A term counts when >= 60 % of its words (brand words dropped) are in the hook."""
    b0 = s["beats"][0]
    have = _tokens(f"{b0.get('vo') or ''} {b0.get('onscreen') or ''}")
    for t in terms:
        need = _tokens(brand.sub(" ", t) if brand else t)
        if need and len(need & have) >= math.ceil(0.6 * len(need)):
            return
    if terms:
        r.warn("H6", tag, f"the hook names none of the segment's search terms {terms[:3]}: a stranger hearing only it "
               "cannot tell what the video is about. Open on the topic in their words (\"Top 10 …\", the title, the "
               "event); a metaphor or insider reference only pays off for people who already know")


def _terms(refs: dict, angle: dict | None, network: str) -> list[str]:
    seg = refs.get("segments", {}).get((angle or {}).get("segment"))
    return [t.lower() for t in ((seg or {}).get("platforms") or {}).get(network, {}).get("search_terms", [])]


CHAPTER = re.compile(r"^\s*(\d{1,2}:)?\d{1,2}:\d{2}\b", re.M)


def _youtube(c: dict, p: str, where: str, terms: list[str], r: Report):
    """Title, description, tags and hashtags for a Short (`youtube`) or a long-form upload (`youtube-long`)."""
    hard, rec = PLATFORMS[p]["hard"], PLATFORMS[p]["recommend"]
    title, desc, tags = c.get("title", ""), c.get("description", ""), c.get("tags") or []
    if not title.strip():
        r.fail("P1", where, "title is empty: on YouTube the title is the search result and the shelf label")
    elif len(title) > hard["title_chars"]:
        r.fail("P1", where, f"title {len(title)} chars > {hard['title_chars']}")
    elif len(title) > rec["title_chars"]:
        r.warn("P1", where, f"title {len(title)} chars: the feed truncates past ~{rec['title_chars']}; front-load the promise")
    if len(desc) > hard["description_chars"]:
        r.fail("P1", where, f"description {len(desc)} chars > {hard['description_chars']}")
    if not isinstance(tags, list) or len(",".join(tags)) > hard["tags_chars"]:
        r.fail("P1", where, f"tags: a list of at most {hard['tags_chars']} characters in total")
    n = len(hashtags(f"{title} {desc}"))
    lo, hi = rec["hashtags"]
    if n > hard["hashtags_max"]:
        r.fail("P2", where, f"{n} hashtags: over {hard['hashtags_max']} YouTube ignores every one")
    elif n > hi:
        r.warn("P2", where, f"{n} hashtags; the first {hi} show by the title, the rest only dilute")
    if terms and not any(t in f"{title} {desc[:200]}".lower() for t in terms):
        r.warn("P3", where, f"title and the first lines of the description carry none of the segment's search terms "
               f"{terms[:4]}: YouTube search and suggested read them")
    if p == "youtube" and URL.search(desc):
        r.warn("P2", where, "links in a Shorts description are not clickable: point to the channel, a pinned comment "
               "or the Shorts related-video link")
    if p == "youtube-long":
        thumb = c.get("thumbnail_text", "")
        if not thumb.strip():
            r.warn("P4", where, "thumbnail_text: the 2-4 words on the custom thumbnail (`mvf thumbnail`)")
        elif len(words(thumb)) > 5:
            r.warn("P4", where, f"thumbnail_text is {len(words(thumb))} words; ≤ 4 reads at search-result size")


def _chapters(c: dict, tl: dict, where: str, r: Report):
    """Chapters make a long upload skimmable and searchable: 0:00 first, at least 3, each at least 10 s."""
    stamps = [m.group(0).strip() for m in CHAPTER.finditer(c.get("description", ""))]
    if not stamps:
        if tl["duration"] >= PLATFORMS["youtube-long"]["recommend"]["chapters_from_s"]:
            r.warn("P5", where, f"{tl['duration']:.0f}s with no chapters: put 0:00 + one timestamp per beat group in the description")
        return
    secs = [sum(int(x) * 60 ** i for i, x in enumerate(reversed(s.split(":")))) for s in stamps]
    if secs[0] != 0 or len(secs) < 3 or any(b - a < 10 for a, b in zip(secs, secs[1:])):
        r.fail("P5", where, f"chapters {stamps}: YouTube shows none unless the first is 0:00, there are ≥ 3 and each is ≥ 10 s")


def _signals(holder: dict, refs: dict, where: str, r: Report) -> bool:
    """Does `holder` (a beat or a platform copy) cite a measured item fresh enough to claim from?"""
    pack = refs.get("material") or {}
    measured = [pack[m] for m in holder.get("material") or [] if m in pack and (pack[m].get("signals") or {})]
    for it in measured:
        age = signal_age(it)
        if age is not None and age > SIGNAL_DAYS:
            r.fail("M2", where, f"{it['id']} was measured {age} days ago: re-gather it before claiming from it")
            return True
    return bool(measured)


def _material(script: dict, sid: str, refs: dict, r: Report):
    """M0 cites the pack, M1 ids exist, M3 recipe covered, M4 only showable material on screen."""
    if "material" not in refs:
        return
    pack = refs["material"] or {}
    holders = [(f"{sid}.{b.get('id')}", b) for b in script.get("beats") or []] + \
              [(f"{sid}.{p}", c) for p, c in (script.get("platforms") or {}).items() if isinstance(c, dict)]
    cited = {}
    for where, h in holders:
        for m in h.get("material") or []:
            if m not in pack:
                r.fail("M1", where, f"unknown material {m} (mvf material ls; gather it first)")
                continue
            cited[m] = pack[m]
            it = pack[m]
            if it.get("rights") not in SHOWABLE and it.get("kind") in SHOWN:
                r.fail("M4", where, f"{m} rights={it.get('rights')}: {RIGHTS.get(it.get('rights'), '')} — "
                       "cite it in research notes, not in a beat")
            elif it.get("rights") == "official-promo":
                r.info("M4", where, f"{m} is official promo: brief, with our commentary, credit "
                       f"'{it.get('credit')}' on screen and in the caption")
            if it.get("kind") == "person":
                r.info("M4", where, f"{m} is a real person: use official pictures only, never an AI likeness")
            if it.get("kind") == "asset" and it.get("rights") != "official-promo" and (
                    (it.get("meta") or {}).get("role") == "portrait" or any(":person:" in ln for ln in it.get("links") or [])):
                # A free licence clears the photographer's copyright, not the person's own image rights.
                r.fail("M4", where, f"{m} pictures a real person with rights={it.get('rights')}: a person appears "
                       "only in official pictures from the rights holder (references/00-brief-material.md)")
    override = str(script.get("material_override") or "").strip()
    if not cited:
        if override:
            r.info("M0", sid, f"no material cited: {override}")
        elif pack:
            r.fail("M0", sid, "cites no material: put the item ids the script stands on in beats[].material "
                   "(or give material_override: <why this script needs none>)")
        else:
            r.warn("M0", sid, "no material pack: `mvf gather` what people actually like before writing")
    recipes = refs.get("recipes") or RECIPES
    rec = script.get("recipe")
    if rec is None:
        if pack:
            r.warn("M3", sid, f"recipe: name what this script is built from ({', '.join(recipes)})")
        return
    if rec not in recipes:
        r.warn("M3", sid, f"recipe {rec!r} is not a known recipe ({sorted(recipes)}): its material is not checked")
        return
    for need in recipes[rec]["needs"]:
        kinds = set(need["kind"].split("|"))
        have = [i for i in cited.values() if i.get("kind") in kinds and (not need.get("signal") or i.get("signals"))]
        if len(have) < need.get("min", 1):
            what = f"{need.get('min', 1)} {need['kind']}" + (" with a measured signal" if need.get("signal") else "")
            if override and need.get("signal"):
                r.info("M3", sid, f"recipe {rec} wants {what}: overridden — {override}")
            else:
                r.warn("M3", sid, f"recipe {rec} usually stands on {what}; cites {len(have)}")
