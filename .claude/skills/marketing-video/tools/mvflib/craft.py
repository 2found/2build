"""Script craft advice: the ending, key moments, thesis wordings, the turn's shape, AI-tell words, length,
tempo and devices. Every code here is a WARN or INFO a writer follows or answers in warn_notes."""
from __future__ import annotations

import re

from .common import Report, words

ENDING_HOLD = 2.0     # seconds after the last line of the ending when nothing follows
CTA_HOLD = 1.5        # seconds after the payoff line before the CTA voice starts
KEY_HOLD = 1.5        # a key line holds its picture this long after the last word
KEY_RANGE = (2, 4)
TEMPOS = {"fast", "breath", "hold"}

# A dash glues two clauses the way generated copy does; a numeric range (3–5, 2010-2020) is fine.
DASH = re.compile(r"—|–|--")
# Stock phrasings readers have learned to hear as generated or as an ad. Vietnamese and English.
TELLS = [
    (r"\bkhông\s+(phải|chỉ)(\s+là)?\b[^.?!\n]{1,60}?\bmà\s+(là|còn)\b", "\"không phải X mà là Y\""),
    (r"\bhãy\s+tưởng\s+tượng\b", "\"hãy tưởng tượng\""),
    (r"\bhành\s+trình\b", "\"hành trình\""),
    (r"\bkhám\s+phá\s+(ngay|thêm)\b", "\"khám phá ngay\""),
    (r"\bđừng\s+bỏ\s+lỡ\b", "\"đừng bỏ lỡ\""),
    (r"\b(siêu\s+phẩm|kiệt\s+tác|đỉnh\s+của\s+chóp|không\s+thể\s+tin\s+nổi|chưa\s+từng\s+có|bùng\s+nổ|"
     r"thay\s+đổi\s+hoàn\s+toàn|vô\s+cùng\s+(tuyệt|đặc\s+biệt))\b", "inflated word"),
    (r"\bnot\s+(just|only)\b[^.?!\n]{1,60}?\b(but|it'?s)\b", "\"not just X, but Y\""),
    (r"\bimagine\s+(a|if|that|being)\b", "\"imagine…\""),
    (r"\b(journey|delve|unlock|unleash|elevate|seamless\w*|game[- ]changer|revolutioni[sz]e\w*|tapestry|"
     r"testament\s+to|in\s+today'?s\s+(fast[- ]paced\s+)?world)\b", "stock AI word"),
    (r"\b(don'?t\s+miss|discover\s+now|you\s+won'?t\s+believe)\b", "sales line"),
    (r"!\s*!|![^!\n]{1,40}!", "stacked \"!\""),
]
TELLS = [(re.compile(p, re.I), name) for p, name in TELLS]
# Three abstract nouns in a row ("sự kiên trì, niềm tin và lòng dũng cảm"; "passion, dedication and excellence").
ABSTRACT_VI = r"(sự|niềm|nỗi|lòng|tình|tinh\s+thần)\s+\w+(\s+\w+)?"
ABSTRACT_EN = r"\w+(tion|sion|ness|ity|ment|ence|ance)"
TRIAD = [re.compile(rf"\b{a}\s*,\s*{a}\s*,?\s*(và|hay|and|or)\s+{a}\b", re.I) for a in (ABSTRACT_VI, ABSTRACT_EN)]
# A number or a comparison is a relation the picture can show (a counter, dots, a ladder, bars).
_VI_N = r"(hai|ba|bốn|năm|sáu|bảy|tám|chín|mười|mươi|lăm|chục|trăm|nghìn|ngàn|vạn|triệu|tỷ)"
_VI_UNIT = (r"(người|lần|năm|tháng|ngày|tuần|giờ|phút|giây|tuổi|phần|cái|con|cuốn|quyển|chương|tầng|cấp|bậc|"
            r"cảnh|kiếp|đời|kiếm|thanh|chiêu|bước|đồng|chữ|câu|trận|vòng|nơi|cách|loại|kiểu|điều|thứ|mét|cân|ký)")
NUMBER = re.compile(rf"\d|\b(mười|trăm|nghìn|ngàn|vạn|triệu|tỷ)\b|\b{_VI_N}\s+({_VI_N}|{_VI_UNIT})\b|"
                    r"\b(two|three|four|five|six|seven|eight|nine|ten|hundred|thousand|million|billion)\b", re.I)
COMPARE = re.compile(r"\b(hơn(?!\s+nữa)|gấp|so\s+với|bằng\s+(cả|nửa)|than(?!\s+(thở|ôi|vãn|khóc|phiền|đá|củi))|"
                     r"versus|vs\.?|compared|twice|half\s+of)\b", re.I)
TURN_SHAPE = re.compile(r"\b(trước|before)\b.*\b(sau|after|giờ|now)\b", re.I | re.S)


def check(script: dict, tl: dict, sid: str, refs: dict, r: Report, control: bool = False):
    beats = script["beats"]
    sp = script.get("spine") or {}
    _ending(beats, tl, sid, r)
    if not control:
        _keys(beats, tl, sid, r)
        _thesis(script, sp, sid, refs, r)
        _turn(sp, sid, r)
    _tells(script, sid, r)
    _length(script, tl, sid, r)
    _tempo(beats, sid, r)
    _devices(beats, sid, r)


def _ending(beats, tl, sid, r: Report):
    """The payoff is the last idea, and it keeps its own seconds after the line."""
    pay = [i for i, b in enumerate(beats) if b.get("role") == "payoff"]
    if pay:
        p = pay[-1]
        for b in beats[p + 1:]:
            if b.get("role") not in {"cta", "end"} and (b.get("vo") or "").strip():
                r.warn("E1", f"{sid}.{b['id']}", f"new content after the payoff ({beats[p]['id']}): the ending is the "
                       "last idea the viewer gets; move this before the payoff or cut it")
    spoken = [i for i, t in enumerate(tl["beats"]) if t.get("vo_end") is not None]
    if not spoken:
        return
    last = pay[-1] if pay and pay[-1] in spoken else spoken[-1]
    end = tl["beats"][last]["vo_end"]
    nxt = next((tl["beats"][i]["vo_start"] for i in spoken if i > last), None)
    room, need = ((nxt - end, CTA_HOLD) if nxt is not None else (tl["duration"] - end, ENDING_HOLD))
    if room < need - 1e-6:
        what = "the next line starts" if nxt is not None else "the video ends"
        r.warn("E2", f"{sid}.{beats[last]['id']}", f"{what} {room:.1f}s after the ending's last word: give it ≥ "
               f"{need:.1f}s to land (`\"hold\": {need:.1f}` seconds on {beats[last]['id']}, with the picture and sound still moving)")


def _keys(beats, tl, sid, r: Report):
    keys = [(b, t) for b, t in zip(beats, tl["beats"]) if b.get("key_moment")]
    lo, hi = KEY_RANGE
    if not lo <= len(keys) <= hi:
        r.warn("K1", sid, f"{len(keys)} beats marked key_moment: mark the {lo}–{hi} lines a viewer must not miss "
               "(the stake, the turn, the payoff); each gets its own hold, picture and sound moment")
    for b, t in keys:
        if t.get("vo_end") is not None and t["end"] - t["vo_end"] < KEY_HOLD - 1e-6:
            r.warn("K2", f"{sid}.{b['id']}", f"key line holds {t['end'] - t['vo_end']:.1f}s after its last word: "
                   f"give it ≥ {KEY_HOLD:.1f}s (`\"hold\": {KEY_HOLD:.1f}` seconds on the beat; `tempo` does not add time) so it lands before the next line")


def _thesis(script, sp, sid, refs, r: Report):
    from .lint import _grams, _says
    th = [str(x).strip() for x in sp.get("thesis") or [] if str(x).strip()]
    if len(th) < 2:
        r.warn("TH0", f"{sid}.spine", "spine.thesis: write 2–3 wordings of the payoff line that differ in verb or "
               "stance; the owner picks one on the sheet (`mvf approve --thesis N`)")
    pick = (refs.get("thesis") or {}).get(script.get("id"))
    if pick:
        said = " ".join(b.get("vo") or "" for b in script["beats"] if b.get("role") in {"payoff", "turn"})
        if not _says(pick, _grams(said)):
            r.warn("TH1", f"{sid}.spine", f"the owner picked the thesis {pick!r}: the payoff or turn vo does not say it")


def _turn(sp, sid, r: Report):
    t = sp.get("turn")
    if isinstance(t, dict):
        ok = bool(str(t.get("before", "")).strip() and str(t.get("after", "")).strip())
    else:
        ok = bool(TURN_SHAPE.search(str(t or "")))
    if t and not ok:
        r.warn("TN1", f"{sid}.spine", "the turn has no before/after: write `{\"before\": …, \"after\": …, \"kind\": "
               "reversal|reveal|transformation|threshold|mirror|contrast}` or \"Trước: … / Sau: …\", so what changes "
               "in the viewer's head is checkable")


def _lines(script):
    for b in script["beats"]:
        for k in ("vo", "onscreen"):
            yield f"{b.get('id')}.{k}", str(b.get(k) or "")
    for v in script.get("variants") or []:
        for k in ("vo", "onscreen"):
            yield f"{v.get('id')}.{k}", str(v.get(k) or "")


def _tells(script, sid, r: Report):
    for where, text in _lines(script):
        if not text.strip():
            continue
        dashes = [m for m in DASH.finditer(text)
                  if not (re.search(r"\d\s*$", text[:m.start()]) and re.match(r"\s*\d", text[m.end():]))]
        if dashes:
            r.warn("WT0", f"{sid}.{where}", "a dash joins clauses the way generated copy does: use a period, a comma "
                   "or a colon (a number range like 3–5 is fine)")
        found = [name for rx, name in TELLS if rx.search(text)] + (["three abstract nouns"] if any(t.search(text) for t in TRIAD) else [])
        if found:
            r.warn("WT1", f"{sid}.{where}", f"stock phrasing ({', '.join(dict.fromkeys(found))}): say the concrete "
                   "thing instead")


def _length(script, tl, sid, r: Report):
    from .timing import wps
    n = sum(len(words(b.get("vo") or "")) for b in script["beats"])
    if n:
        r.info("WC", sid, f"{n} words, {n / wps(script):.1f}s at the voice's rate, {n / max(tl['duration'], 0.1):.1f} "
               "words/s over the runtime. Read it aloud once at that pace, then cut filler words inside lines, "
               "never a step")


def _tempo(beats, sid, r: Report):
    tags = [(b.get("id"), b.get("tempo")) for b in beats]
    for b in beats:
        if b.get("tempo") is not None and b["tempo"] not in TEMPOS:
            r.warn("TP1", f"{sid}.{b.get('id')}", f"tempo {b['tempo']!r}: one of {sorted(TEMPOS)}")
    for i in range(len(tags) - 2):
        t = tags[i][1]
        if t in TEMPOS and tags[i + 1][1] == t and tags[i + 2][1] == t and (i == 0 or tags[i - 1][1] != t):
            r.warn("TP1", sid, f"{tags[i][0]}–{tags[i + 2][0]}: three '{t}' beats in a row; vary the tempo "
                   "(fast, breath, hold) so the cut has a rhythm")


def needs_device(b: dict) -> bool:
    vo = b.get("vo") or ""
    return b.get("role") not in {"cta", "end"} and bool(NUMBER.search(vo) or COMPARE.search(vo))


def _devices(beats, sid, r: Report):
    for b in beats:
        if needs_device(b) and not b.get("device"):
            r.warn("DV1", f"{sid}.{b.get('id')}", "the vo carries a number or a comparison: name the device that "
                   "shows it (`device`: counter, dots, ladder, bars, compare, steps, or html) or answer why the "
                   "picture alone tells it")
