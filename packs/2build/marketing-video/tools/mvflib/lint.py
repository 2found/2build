"""Script lint: structure, hook, length, claims, ad-feel, on-screen text and
per-platform copy. Measured on the timeline (estimated or from the take)."""
from __future__ import annotations

import json
import re

from . import timing
from .checks import trend_age, STALE_DAYS
from .common import ID_RE, LIB, PLATFORMS, Report, hashtags, words, x_weight
from .material import RIGHTS, SHOWABLE, SHOWN, SIGNAL_DAYS, signal_age

RECIPES = json.loads((LIB / "recipes.json").read_text(encoding="utf-8"))["recipes"]

ROLES = {"hook", "context", "problem", "tension", "turn", "demo", "proof", "payoff", "reaction", "beat", "cta", "end"}
FORMATS = {"problem-solution", "demo", "pov", "before-after", "story", "listicle", "myth-buster", "reaction",
           "tutorial", "testimonial", "challenge", "behind-the-scenes", "comparison", "launch", "meme", "skit"}
VOICE_MODES = {"recorded", "clone", "design", "preset", "none"}
# Roles that give the viewer something (a step, a result, a punchline); setup roles do not.
VALUE_ROLES = {"turn", "demo", "proof", "payoff", "reaction", "beat"}

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
URL = re.compile(r"https?://|\b\w+\.(com|net|org|io|dev|ai|app|vn)\b", re.I)


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
        r.fail("S0", "script.id", f"{sid!r} must look like s01 or s01-short-name")
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
            r.fail("S1", f"{sid}.{b.get('id')}", f"role {b.get('role')!r} not in {sorted(ROLES)}")
    if beats[0].get("role") != "hook":
        r.fail("S1", f"{sid}.{beats[0].get('id')}", "the first beat is the hook (role: hook)")
    if not str(script.get("value", "")).strip():
        r.fail("S2", sid, "value: what the viewer walks away with (a laugh, a feeling, a thing learned, a result)")
    if script.get("format") not in FORMATS:
        r.fail("S2", sid, f"format {script.get('format')!r} not in {sorted(FORMATS)}")
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
    refs = {**refs, "brand_names": (campaign.get("brand") or {}).get("names") or []}
    platforms = [p for p in (script.get("platforms") or {}) if p in PLATFORMS]
    if not platforms:
        r.fail("P0", sid, "platforms: give at least one of tiktok / x with its copy")
    for p in script.get("platforms") or {}:
        if p not in PLATFORMS:
            r.fail("P0", sid, f"unknown platform {p}")

    for hv in variants:
        s = timing.variant(script, hv)
        tl = timing.build(s, manifest)
        tag = f"{sid}[{s['beats'][0]['variant']}]"
        _hook(s, tl, tag, platforms, brand, r)
        _first_value(s, tl, tag, platforms, r)
        _length(tl, tag, platforms, r)
    tl = timing.build(timing.variant(script, None), manifest)
    _body(script, tl, sid, brand, refs, campaign, r)
    for p in platforms:
        _copy(script, p, sid, angle, refs, r)
    _material(script, sid, refs, r)
    r.info("T0", sid, f"{tl['timing']} runtime {tl['duration']:.1f}s over {len(beats)} beats")
    return r


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
    if not on and "x" in platforms:
        r.fail("H4", tag, "X autoplays muted: the hook needs on-screen text")
    elif not on:
        r.warn("H4", tag, "no on-screen hook text: many TikTok viewers watch muted at first")
    elif len(words(on)) > 9:
        r.warn("H4", tag, f"hook text is {len(words(on))} words; ≤ 7 reads in one glance")
    if not str(b0.get("visual", "")).strip():
        r.fail("H5", tag, "frame 0 needs a visual: the thumbnail is the first frame (a face, the result, motion)")
    elif re.search(r"\blogo\b|title card|black|màn\s*đen", b0["visual"], re.I):
        r.warn("H5", tag, "frame 0 shows a logo/title/black: open on the subject in motion")


def _first_value(s, tl, tag, platforms, r: Report):
    limit = min(PLATFORMS[p]["recommend"]["first_value_s"] for p in platforms) if platforms else 6.0
    first = next(((b, t) for b, t in zip(s["beats"][1:], tl["beats"][1:]) if b.get("role") in VALUE_ROLES), None)
    if not first:
        r.fail("R1", tag, f"no beat gives value (roles {sorted(VALUE_ROLES)}): the viewer leaves with nothing")
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
        claim = CLAIM.search(brand.sub("", text) if brand else text)  # "2Novel" is a name, not a number
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
            r.fail("V1", where, "visual is empty: say what the viewer sees (the real screen, the person, the object)")
        dur = t["end"] - t["start"]
        if dur > 4.0:
            r.warn("V2", where, f"{dur:.1f}s on one beat: plan a visible change at least every 3.5s (cut, push, reveal)")
        on = words(b.get("onscreen") or "")
        if on and len(on) > 3.0 * dur:
            r.warn("V3", where, f"{len(on)} on-screen words in {dur:.1f}s: unreadable; ≤ 3 words/s")
        for tid in b.get("trends") or []:
            _trend_ref(tid, trends, where, r)
    if len(brand_beats) > 1:
        r.warn("B1", sid, f"brand spoken in {len(brand_beats)} body beats ({', '.join(brand_beats)}): ad-feel; "
               "let the product appear on screen, say its name once")
    half = tl["duration"] / 2
    if brand and not any(brand.search(f"{b.get('visual') or ''} {b.get('onscreen') or ''}")
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
    if p == "tiktok":
        cap = c.get("caption", "")
        if not cap.strip():
            r.fail("P1", where, "caption is empty")
        if len(cap) > hard["caption_chars"]:
            r.fail("P1", where, f"caption {len(cap)} chars > {hard['caption_chars']}")
        n = len(hashtags(cap))
        lo, hi = rec["hashtags"]
        if not lo <= n <= hi:
            r.warn("P2", where, f"{n} hashtags; {lo}–{hi} specific ones beat a wall of #fyp")
        if URL.search(cap):
            r.warn("P2", where, "links in a TikTok caption are not clickable: say where to go, or use the bio")
        seg = refs.get("segments", {}).get((angle or {}).get("segment"))
        terms = [t.lower() for t in ((seg or {}).get("platforms") or {}).get("tiktok", {}).get("search_terms", [])]
        if terms and not any(t in cap.lower() for t in terms):
            r.warn("P3", where, f"caption has none of the segment's search terms {terms[:4]}: TikTok search reads it")
        sound = str(c.get("sound", ""))
        if sound.startswith("trend:"):
            _trend_ref(sound.split(":", 1)[1], refs.get("trends", {}), where, r)
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
    copy = " ".join([c.get("caption", ""), c.get("post", ""), *(c.get("thread") or []), c.get("cover_text", "")])
    brand = _brand_re(refs.get("brand_names") or [])
    claim = CLAIM.search(brand.sub("", copy) if brand else copy)
    if claim and not c.get("facts") and not _signals(c, refs, where, r):
        r.fail("C3", where, f"claim {claim.group(0)!r} in the post copy needs facts: [F…] or material: […] on platforms.{p}")
    for fid in c.get("facts") or []:
        if fid not in refs.get("facts", set()):
            r.fail("C3", where, f"unknown fact {fid}")
    cover = c.get("cover_text", "")
    if not cover.strip():
        r.warn("P4", where, "cover_text: the profile-grid cover line (≤ 6 words)")
    elif len(words(cover)) > 6:
        r.warn("P4", where, f"cover_text is {len(words(cover))} words; ≤ 6 reads on a grid thumbnail")


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
        r.fail("M3", sid, f"recipe {rec!r} not in {sorted(recipes)}")
        return
    for need in recipes[rec]["needs"]:
        kinds = set(need["kind"].split("|"))
        have = [i for i in cited.values() if i.get("kind") in kinds and (not need.get("signal") or i.get("signals"))]
        if len(have) < need.get("min", 1):
            what = f"{need.get('min', 1)} {need['kind']}" + (" with a measured signal" if need.get("signal") else "")
            if override and need.get("signal"):
                r.info("M3", sid, f"recipe {rec} wants {what}: overridden — {override}")
            else:
                r.fail("M3", sid, f"recipe {rec} needs {what}; cites {len(have)}")
