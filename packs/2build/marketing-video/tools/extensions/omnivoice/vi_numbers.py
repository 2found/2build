"""Vietnamese written forms → the words a speaker says: numbers, dates, times, currency,
percent, ranges, `thứ N`, a few abbreviations. Stdlib only.

A digit glued to a letter (`2novel`, `abc17`) is not a number, and `1.2.3` is read
"một chấm hai chấm ba".
"""
from __future__ import annotations

import re

_DIGIT_WORD = {
    "0": "không", "1": "một", "2": "hai", "3": "ba", "4": "bốn",
    "5": "năm", "6": "sáu", "7": "bảy", "8": "tám", "9": "chín",
}
_UNIT_SINGLE = ("", "mươi", "trăm")
_UNIT_TRIPLE = ("", "nghìn", "triệu", "tỷ", "nghìn tỷ", "triệu tỷ", "tỷ tỷ")


def _sandhi(text: str) -> str:
    """The alternations Vietnamese writes but does not speak."""
    return (text.replace("mười năm", "mười lăm").replace("mươi năm", "mươi lăm")
            .replace("mươi bốn", "mươi tư").replace("mươi một", "mươi mốt").replace("linh bốn", "linh tư"))


def digits(run: str) -> str:
    """Symbol by symbol: `2024` -> `hai không hai bốn`."""
    return " ".join(_DIGIT_WORD.get(ch, ch) for ch in run if not ch.isspace())


def _chunks(number: str) -> list[str]:
    chunks = [number[i:i + 3] for i in range(len(number) - 3, -1, -3)][::-1]
    if len(number) % 3:
        chunks = [number[: len(number) % 3]] + chunks
    return chunks


def _chunk(chunk: str, scale_index: int) -> str:
    """One 3-digit chunk plus its scale word. An all-zero chunk is silent."""
    if chunk == "000":
        return ""
    result = ""
    position = len(chunk) - 1
    while position >= 0:
        digit = chunk[position]
        if position == len(chunk) - 1 and digit == "0" and len(chunk) > 1:
            pass                    # trailing zero: "hai mươi", not "hai mươi không"
        elif position == len(chunk) - 2 and digit in ("1", "0"):
            if position == 0 and digit == "0":
                pass
            elif digit == "1":
                units = chunk[position + 1]
                result = "mười" if units == "0" else f"mười {_DIGIT_WORD[units]}"
            else:
                units = chunk[position + 1]
                result = "" if units == "0" else f"linh {_DIGIT_WORD[units]}"
        else:
            result = " ".join(part for part in
                              (_DIGIT_WORD[digit], _UNIT_SINGLE[len(chunk) - position - 1], result) if part)
        position -= 1
    return " ".join(part for part in (result, _UNIT_TRIPLE[scale_index]) if part)


def cardinal(n: int) -> str:
    """`2026` -> `hai nghìn không trăm hai mươi sáu` (also how a year is read)."""
    number = str(int(n))
    chunks = _chunks(number)
    if len(chunks) > len(_UNIT_TRIPLE):
        return digits(number)
    parts = [_chunk(c, len(chunks) - i - 1) for i, c in enumerate(chunks)]
    return _sandhi(" ".join(p for p in parts if p)) or _DIGIT_WORD["0"]


def number(token: str) -> str:
    """`.` groups thousands, `,` marks the decimal; a run over 8 digits is an identifier."""
    if "," in token:
        whole, _, fraction = token.partition(",")
        return _decimal(whole, fraction)
    if "." in token:
        parts = token.split(".")
        if all(len(p) == 3 for p in parts[1:]):
            return cardinal(int("".join(parts)))
        if len(parts) > 2:  # a version or an outline number: `1.2.3` is "một chấm hai chấm ba"
            return " chấm ".join(cardinal(int(p)) if p else "" for p in parts).strip()
        whole, _, fraction = token.partition(".")
        return _decimal(whole, fraction)
    if len(token) > 8:
        return digits(token)
    return cardinal(int(token))


def _decimal(whole: str, fraction: str) -> str:
    return f"{cardinal(int(whole.replace('.', '') or 0))} phẩy {digits(fraction)}"


_ABBREV = {"tp": "thành phố", "gs": "giáo sư", "ts": "tiến sĩ", "nxb": "nhà xuất bản", "tr": "trang"}
_ABBREV_RE = re.compile(r"\b(tp|gs|ts|nxb|tr)\.", re.IGNORECASE)
_THU_RE = re.compile(r"\bthứ\s+(\d+)\b", re.IGNORECASE)
_DATE_RE = re.compile(r"\b(\d{1,2})[/.-](\d{1,2})[/.-](\d{4})\b")
_TIME_RE = re.compile(r"\b(?P<h_hour>\d{1,2})[hg](?P<h_minute>\d{1,2})?\b"
                      r"|\b(?P<c_hour>[01]?\d|2[0-3]):(?P<c_minute>[0-5]\d)\b")
_CURRENCY_RE = re.compile(r"(\d[\d.,]*)\s*(?:vnđ|vnd|đồng|đ|₫)\b", re.IGNORECASE)
_DOLLAR_RE = re.compile(r"\$\s*(\d(?:[\d.,]*\d)?)")
_PERCENT_RE = re.compile(r"(\d[\d.,]*)\s*%")
_RANGE_RE = re.compile(r"\b(\d+)\s*-\s*(\d+)\b")
# A digit glued to a letter is part of a name, not a number: `lamlamyu17`, `2novel`.
# (loa/frontend.py reads those digits; it sees them as separate spans already.)
_NUMBER_RE = re.compile(r"(?<!\w)(?:\d[\d.,]*\d|\d)(?!\w)")
_UNIT_ONLY_RE = re.compile(r"(?<!\w)(?:vnđ|vnd|đ|₫)(?!\w)", re.IGNORECASE)
_ORDINALS = {1: "nhất", 2: "hai", 3: "ba", 4: "tư"}


def _date(m: re.Match) -> str:
    day, month, year = m.groups()
    before = m.string[max(0, m.start() - 8):m.start()].lower()
    lead = "" if re.search(r"ngày\s*$", before) else "ngày "
    return f"{lead}{cardinal(int(day))} tháng {cardinal(int(month))} năm {cardinal(int(year))}"


def _time(m: re.Match) -> str:
    hour = m.group("h_hour") or m.group("c_hour")
    minute = m.group("h_minute") or m.group("c_minute")
    return f"{cardinal(int(hour))} giờ" + (f" {cardinal(int(minute))}" if minute else "")


def normalize_vi(text: str) -> str:
    """Every written form in `text` read out; prose with none comes back unchanged."""
    text = _ABBREV_RE.sub(lambda m: _ABBREV[m.group(1).lower()], text)
    text = _THU_RE.sub(lambda m: f"thứ {_ORDINALS.get(int(m.group(1)), cardinal(int(m.group(1))))}", text)
    text = _DATE_RE.sub(_date, text)
    text = _TIME_RE.sub(_time, text)
    text = _CURRENCY_RE.sub(lambda m: f"{number(m.group(1))} đồng", text)
    text = _DOLLAR_RE.sub(lambda m: f"{number(m.group(1))} đô la", text)
    text = _PERCENT_RE.sub(lambda m: f"{number(m.group(1))} phần trăm", text)
    text = _RANGE_RE.sub(lambda m: f"{number(m.group(1))} đến {number(m.group(2))}", text)
    text = _UNIT_ONLY_RE.sub("đồng", text)
    return _NUMBER_RE.sub(lambda m: number(m.group(0)), text)


# One written form as a single span, so a caller can map it onto the display words it
# covers (`28/10/2026` is three display words, `49.000đ` two). Longest forms first; the
# whole span stands alone, so the `2` of `2novel` is not a number.
SPAN = (r"(?<!\w)(?:"
        r"\d{1,2}[/.-]\d{1,2}[/.-]\d{4}"                          # date
        r"|(?:[01]?\d|2[0-3]):[0-5]\d"                            # clock
        r"|\d{1,2}[hg]\d{0,2}"                                    # 15h30, 9g45, 15h
        r"|\d[\d.,]*\s*(?:vnđ|vnd|đồng|đ|₫)"                      # currency
        r"|\$\s*\d(?:[\d.,]*\d)?"                                 # dollars
        r"|\d[\d.,]*\s*%"                                         # percent
        r"|\d+\s*-\s*\d+"                                         # range
        r"|\d[\d.,]*\d|\d"                                        # number
        r"|thứ\s+\d+"                                             # ordinal day / rank
        r"|(?:tp|gs|ts|nxb|tr)\."                                 # abbreviation
        r")(?!\w)")
