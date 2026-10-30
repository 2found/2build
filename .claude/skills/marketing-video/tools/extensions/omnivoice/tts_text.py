r"""OmniVoice text frontend for the omnivoice voice extension: paragraphs, cleaning,
paragraph packing and sentence chunking, with the gap and size constants they use.
Stdlib only (plus `regex` when present).

It uses the `regex` module for `\p{…}` classes when installed; bare python3 (mvf's
tests) compiles the same patterns with stdlib `re`, each `\p{L}` / `\p{M}` / `\p{N}`
spelled out as the code-point ranges of that category.
"""
from __future__ import annotations

import re
import sys
import unicodedata

from vi_numbers import normalize_vi

try:
    import regex
except ImportError:
    class _StdlibRegex:
        """The slice of `regex` engine.py uses, over stdlib `re`."""
        IGNORECASE = re.IGNORECASE
        escape = staticmethod(re.escape)
        _classes: dict[str, str] = {}

        @classmethod
        def _ranges(cls, cat: str) -> str:
            if cat not in cls._classes:
                out, start, prev = [], None, None
                for cp in range(sys.maxunicode + 1):
                    if unicodedata.category(chr(cp))[0] == cat:
                        if start is None:
                            start = cp
                        prev = cp
                    elif start is not None:
                        out.append(re.escape(chr(start)) + ("-" + re.escape(chr(prev)) if prev > start else ""))
                        start = None
                if start is not None:
                    out.append(re.escape(chr(start)) + ("-" + re.escape(chr(prev)) if prev > start else ""))
                cls._classes[cat] = "".join(out)
            return cls._classes[cat]

        @classmethod
        def compile(cls, pattern: str, flags: int = 0):
            return re.compile(re.sub(r"\\p\{([LMN])\}", lambda m: cls._ranges(m.group(1)), pattern), flags)

    regex = _StdlibRegex

SAMPLE_RATE = 24_000
PARAGRAPH_GAP_MS = 300
SENTENCE_GAP_MS = 150
MAX_SENTENCE_CHARS = 280
# Below this, OmniVoice prosody/voice quality degrades audibly — the model
# lacks context to lock onto a consistent timbre for short inputs.
# Chunks below this floor are merged forward into the next sentence/paragraph.
MIN_SYNTH_CHARS = 80


def split_paragraphs(content: str) -> list[str]:
    return [
        p.strip()
        for p in content.strip().split("\n")
        if p.strip() and any(c.isalnum() for c in p)
    ]


# The only punctuation OmniVoice hears from us. Its own frontend splits on
# `.,;:!?。，；：！？` (omnivoice/utils/text.py SPLIT_PUNCTUATION) and its duration
# estimator weights a punctuation char at 0.5 "pause capability" against 1.0 for
# a Latin letter (utils/duration.py) — punctuation is what buys silence, so a
# mark we keep is a mark we want heard. Everything else is noise the model has
# to guess at, and quotes are actively harmful: OmniVoice opens dialogue
# intonation at one and closes it at the next, but a quoted passage almost never
# fits inside a single MAX_SENTENCE_CHARS chunk, so each generate() call used to
# see an unbalanced quote and reset timbre mid-line. `[...]` is worse — that is
# OmniVoice's non-verbal-tag / CMU-pronunciation syntax, so a bracket left in a
# source chapter can be swallowed as a control token instead of spoken.
#
# `!` and `?` are kept on purpose: they are sentence terminators in OmniVoice's
# own SPLIT_PUNCTUATION, and dropping them flattens every question in the novel
# to a statement. Narrow this to ".," to go comma-and-dot only.
KEPT_PUNCTUATION = ".,!?"

# Marks that are a pause under another name — folded onto a kept mark rather
# than dropped, so the clause boundary survives the whitelist below. The
# fullwidth forms arrive straight from Chinese-source chapters; deleting them
# would silently erase the sentence structure of a whole paragraph.
#
# Each fold emits a trailing space because the fullwidth marks are self-spacing
# — `nói。Nàng` folded to `nói.Nàng` is a boundary `_SENTENCE_BOUNDARY` cannot
# see, so the paragraph would never split into sentences. `_WS` collapses the
# doubled space that the halfwidth marks then leave behind.
_PUNCT_FOLD = str.maketrans({
    "\u3002": ". ", "\uff0e": ". ", "\uff01": "! ", "\uff1f": "? ",
    "\uff0c": ", ", "\u3001": ", ", "\uff1b": ", ", "\uff1a": ", ",
    ";": ", ", ":": ", ",
})

# Anything that is not a letter, mark, number or whitespace becomes a space
# (never nothing — deleting would glue `chữ"đạo"` into `chữđạo`).
#
# This is the `regex` module, not stdlib `re`, for the Unicode property classes:
# `\p{L}` is every letter in every script, `\p{M}` every combining mark, `\p{N}`
# every numeral. Spelling the same thing with `re` means `\w`, which is wrong at
# both ends — it counts `_` as a word character, and it does NOT match category
# Mn, so every Vietnamese tone mark in decomposed text would be stripped.
_NON_SPEECH = regex.compile(
    r"[^\p{L}\p{M}\p{N}\s" + regex.escape(KEPT_PUNCTUATION) + r"]"
)

# Words the model cannot pronounce as written. `haiz` is Vietnamese web slang
# for a sigh (from 唉); a syllable-final `z` does not exist in Vietnamese
# phonotactics, so OmniVoice guesses at it. Elongations (`haizz`, `haizzz`) are
# the same word. The `z` is required — a bare `hai` is the number two, which
# appears 14 times in the test fixtures alone.
_PRONUNCIATION = (
    (regex.compile(r"\bhaiz+\b", regex.IGNORECASE), "hai"),
)

_LEAD_DASH = regex.compile(
    r"^\s*['\"\u201c\u201d\u2018\u2019«»„““”‘’「」『』]*\s*[-\u2013\u2014]\s*"
)
_MULTI_BANG = re.compile(r"([!?])\1+")
_MULTI_DOT = re.compile(r"\.{2,}")
_WS = re.compile(r"\s+")
# Dropping a mark can strand the punctuation behind it after a space; reattach
# it so the model reads a normal clause. Also cleans up the `A — B` → `A , B`
# that the em-dash rewrite below leaves whenever the dash had spaces around it.
_SPACE_BEFORE_PUNCT = re.compile(r"\s+([" + re.escape(KEPT_PUNCTUATION) + r"])")
# Ensure punctuation directly touching a following letter has a space so sentence
# splitters (_SENTENCE_BOUNDARY) and TTS tokenizers recognize the boundary
# (e.g. `Hít...Ôi...` → `Hít... Ôi...`, `câu một.Câu hai` → `câu một. Câu hai`, `chữ,chữ` → `chữ, chữ`).
_SPACE_AFTER_PUNCT = regex.compile(r"([.,!?]+)(?=[\p{L}])")
# Written forms read as words (numbers, dates, times, money, percent, ranges,
# `thứ N`, abbreviations) — see vi_numbers.py. It runs before the punctuation
# fold, which would otherwise turn `15:30` into `15, 30` and drop `%` and `/`.
# A module attribute so scripts/generate_normal_voice_tts_golden.py can switch
# it off: the Edge clients keep digits as written.
_READ_WRITTEN_FORMS = normalize_vi
# Vietnamese thousands separator: `2.000` is one number, not `2` + fake
# sentence boundary + `000`. Only the clients' pipeline (golden script, with
# `_READ_WRITTEN_FORMS` off) still sees digits here; `\d{1,3}` on the first
# group keeps `1234.000` (ambiguous) and `1.2.3` (version) untouched;
# `(?!\w)` keeps `1.0000` untouched. `,` is NOT folded — in Vietnamese it is
# the decimal mark.
_THOUSANDS_DOT = re.compile(r"(?<!\w)\d{1,3}(?:\.\d{3})+(?!\w)")
_MD_EMPH = re.compile(r"\*+")
_SENTENCE_BOUNDARY = re.compile(r'([.!?]+["\']?)\s+')
_SOFT_BREAK_RE = re.compile(r"[,;:]\s+|[—–]\s+")


def _respell(text: str) -> str:
    """Rewrite words OmniVoice mispronounces, keeping sentence-initial case."""
    for pattern, replacement in _PRONUNCIATION:
        text = pattern.sub(
            lambda m: replacement.capitalize()
            if m.group()[:1].isupper()
            else replacement,
            text,
        )
    return text


def clean_text(text: str) -> str:
    """Reduce text to letters, digits and `KEPT_PUNCTUATION` for synthesis.

    Everything the model does not speak is dropped — quotes, brackets, symbols,
    emoji — so a line of dialogue keeps one continuous prosody across the chunks
    it gets split into. Pause-bearing marks (`;` `:` and the fullwidth CJK
    family) are folded onto a kept mark first so their clause boundary survives.

    Order matters: `_LEAD_DASH` runs before the em-dash rewrite so an opening
    dialogue dash is removed rather than turned into a leading comma, written
    forms are read before the punctuation fold (`15:30`, `50%`, `28/10/2026`
    need their marks) and so before the whitelist, so a number is words rather
    than a 3.5×-weighted character the duration estimator over-budgets, and
    `_respell` runs after it so a word is matched against clean letters.
    """
    # `_NON_SPEECH` keeps combining marks either way, but composing first keeps
    # `len()` honest: a decomposed `ế` is two code points, and every chunking
    # decision downstream (MAX_SENTENCE_CHARS, MIN_SYNTH_CHARS) counts chars.
    text = unicodedata.normalize("NFC", text)
    text = text.replace("\u2026", "...")
    text = text.replace("\xa0", " ")
    text = _LEAD_DASH.sub("", text)
    text = _READ_WRITTEN_FORMS(text)
    text = text.replace("\u2014", ", ").replace("\u2013", ", ")
    text = _MD_EMPH.sub("", text)
    text = text.translate(_PUNCT_FOLD)
    text = _MULTI_DOT.sub("...", text)
    text = _MULTI_BANG.sub(r"\1", text)
    text = _THOUSANDS_DOT.sub(lambda m: m.group(0).replace(".", ""), text)
    text = _SPACE_AFTER_PUNCT.sub(r"\1 ", text)
    text = _NON_SPEECH.sub(" ", text)
    text = _respell(text)
    text = _WS.sub(" ", text)
    text = _SPACE_BEFORE_PUNCT.sub(r"\1", text)
    return text.strip()


def _sentence_parts(text: str) -> list[str]:
    """Split at sentence punctuation, preserving optional closing quotes."""
    parts: list[str] = []
    start = 0
    for match in _SENTENCE_BOUNDARY.finditer(text):
        end = match.end(1)
        part = text[start:end].strip()
        if part:
            parts.append(part)
        start = match.end()
    tail = text[start:].strip()
    if tail:
        parts.append(tail)
    return parts


def _soften_terminal_punctuation(text: str) -> str:
    if text.endswith(('"', "'")) and len(text) >= 2 and text[-2] in "?!":
        return text[:-2] + "," + text[-1]
    if text and text[-1] in "?!":
        return text[:-1] + ","
    return text


def _split_long_sentence(text: str, max_chars: int = MAX_SENTENCE_CHARS) -> list[str]:
    """Split overlong sentence-like text without leaving a tiny tail."""
    chunks: list[str] = []
    remaining = text.strip()
    while len(remaining) > max_chars:
        # If remaining text fits in two chunks, split near the middle at a soft
        # break or whitespace so both halves remain balanced and >= MIN_SYNTH_CHARS.
        if len(remaining) <= 2 * max_chars:
            half = len(remaining) // 2
            cut = 0
            best_dist = float("inf")
            for match in _SOFT_BREAK_RE.finditer(remaining):
                if MIN_SYNTH_CHARS <= match.end() <= len(remaining) - MIN_SYNTH_CHARS:
                    dist = abs(match.end() - half)
                    if dist < best_dist:
                        best_dist = dist
                        cut = match.end()
            if cut == 0:
                for i in range(max(MIN_SYNTH_CHARS, half - 40), min(len(remaining) - MIN_SYNTH_CHARS, half + 40)):
                    if remaining[i] == " ":
                        dist = abs(i + 1 - half)
                        if dist < best_dist:
                            best_dist = dist
                            cut = i + 1
            if cut > 0:
                chunk = remaining[:cut].strip()
                if chunk:
                    chunks.append(chunk)
                remaining = remaining[cut:].strip()
                continue

        cut_limit = max_chars
        tail_len_at_limit = len(remaining) - max_chars
        if 0 < tail_len_at_limit < MIN_SYNTH_CHARS:
            cut_limit = max(MIN_SYNTH_CHARS, len(remaining) - MIN_SYNTH_CHARS)

        window = remaining[:cut_limit]
        cut = 0

        for match in _SOFT_BREAK_RE.finditer(window):
            if match.end() >= MIN_SYNTH_CHARS:
                cut = match.end()

        if cut == 0:
            space = window.rfind(" ")
            if space >= MIN_SYNTH_CHARS:
                cut = space + 1

        if cut == 0:
            cut = cut_limit

        chunk = remaining[:cut].strip()
        if chunk:
            chunks.append(chunk)
        remaining = remaining[cut:].strip()

    if remaining:
        chunks.append(remaining)
    return chunks

def split_into_sentences(text: str, max_chars: int = MAX_SENTENCE_CHARS) -> list[str]:
    """Split text into sentence-level chunks for better TTS quality.

    Only breaks at sentence-ending punctuation — `.`, `!`, `?` — never at
    `;`, `:` or inside quoted dialogue. Short sentences are then packed
    together up to `max_chars` so we don't synthesize one-word chunks.
    Long sentences (> max_chars) are split at soft punctuation or whitespace,
    with the cut point pulled earlier when needed so the final tail is not a
    tiny TTS input.

    Within each packed chunk, a short sub-sentence (< MIN_SYNTH_CHARS) that
    is followed by another sub-sentence has its terminal `?`/`!` softened
    to `,` — mirrors `_ensure_sentence_terminator` at paragraph seams. This
    prevents OmniVoice from doing a prosody reset on a tiny interjection
    like "Ồ?" mid-call (the model otherwise renders it with full sentence-
    final intonation and "restarts" timbre on the next fragment with no
    context). Softening only happens between fragments that share a chunk;
    a short fragment that ends a chunk keeps its original terminator.

    After greedy packing, any sub-floor group is absorbed into a neighbor.
    The greedy packer can leave a short tail (e.g. parts `[195, 30]` →
    chunks `[195]` + `[30]`) — that 30-char chunk hits the same prosody
    instability MIN_SYNTH_CHARS exists to avoid. Controlled overshoot of
    `max_chars` is preferred over a sub-floor chunk: OmniVoice degrades
    far more on tiny inputs than on slightly-over-cap ones.
    """
    # Treat `.`, `!`, `?` and `...` as sentence boundaries, keeping the
    # punctuation attached to the preceding sentence. The optional closing
    # quote in `_SENTENCE_BOUNDARY` is defensive only — `clean_text` has
    # already stripped quotes off anything that reaches synthesis.
    parts = [
        chunk
        for part in _sentence_parts(text)
        for chunk in _split_long_sentence(part, max_chars)
    ]

    # Group parts into chunks up to max_chars.
    groups: list[list[str]] = []
    current: list[str] = []
    current_len = 0
    for part in parts:
        if current and current_len + len(part) + 1 > max_chars:
            groups.append(current)
            current = [part]
            current_len = len(part)
        else:
            current.append(part)
            current_len += len(part) + (1 if current_len else 0)
    if current:
        groups.append(current)

    # Absorb any sub-floor group into a neighbor: forward first into the next
    # sentence so speech flows naturally, and backward for the tail.
    def _glen(g: list[str]) -> int:
        return sum(len(p) for p in g) + max(0, len(g) - 1)

    i = 0
    while i < len(groups) - 1:
        if _glen(groups[i]) < MIN_SYNTH_CHARS:
            groups[i + 1] = groups[i] + groups[i + 1]
            del groups[i]
        else:
            i += 1
    if len(groups) >= 2 and _glen(groups[-1]) < MIN_SYNTH_CHARS:
        groups[-2].extend(groups.pop())
    # Soften short non-final sub-sentences inside each chunk.
    chunks: list[str] = []
    for group in groups:
        softened: list[str] = []
        for i, part in enumerate(group):
            if i < len(group) - 1 and len(part) < MIN_SYNTH_CHARS:
                part = _soften_terminal_punctuation(part)
            softened.append(part)
        chunks.append(" ".join(softened))

    final_chunks: list[str] = []
    for chunk in chunks:
        if len(chunk) > max_chars:
            final_chunks.extend(_split_long_sentence(chunk, max_chars))
        else:
            final_chunks.append(chunk)
    return final_chunks


def _ensure_sentence_terminator(text: str) -> str:
    """Normalize the tail of `text` so `split_into_sentences` treats the end
    as a sentence boundary. OmniVoice gets a 150ms sentence_gap silence after
    `.!?…`; colon-led dialogue merges would otherwise produce no audible
    pause between merged paragraphs.
    """
    tail = text.rstrip()
    if not tail:
        return text
    if tail[-1] in ".!?…":
        return text
    if tail[-1] in ":;,—–":
        return tail[:-1] + "."
    return tail + "."


def _merge_unit(target: dict, source: dict) -> None:
    target_text = _ensure_sentence_terminator(target["text"])
    target["text"] = f"{target_text} {source['text']}".strip()
    target["para_indices"].extend(source["para_indices"])
    target["para_lengths"].extend(source["para_lengths"])


def pack_paragraphs_for_synthesis(paragraphs: list[str]) -> list[dict]:
    """Group consecutive paragraphs into synthesis units to avoid tiny TTS inputs.

    OmniVoice produces unstable prosody/timbre on very short inputs (e.g.
    one- or two-word paragraphs). Short paragraphs are merged forward into
    the next unit so dialogue and interjections flow into the spoken line
    that follows. If the chapter ends on a short unit, it is merged backward
    into the previous unit. Normal paragraphs keep their own paragraph-gap
    boundary so paragraph-level audio cutting/seek remains predictable.
    At the join, the previous unit's tail is normalized to end with `.` so
    `split_into_sentences` emits a sentence boundary (→ 150ms sentence_gap
    silence in the final audio) at each merged paragraph boundary.

    DOM paragraph indices are preserved in `para_indices` so the manifest
    can still map frontend scroll positions to audio timestamps via
    proportional splitting of the merged unit's duration.
    """
    units: list[dict] = []
    for pi, raw in enumerate(paragraphs):
        text = clean_text(raw)
        if not text:
            continue
        units.append({
            "text": text,
            "para_indices": [pi],
            "para_lengths": [len(text)],
        })

    if not units:
        return []

    # Merge forward any unit < MIN_SYNTH_CHARS into the next unit so short
    # dialogue / interjections flow into the spoken line that follows.
    i = 0
    while i < len(units) - 1:
        if len(units[i]["text"]) < MIN_SYNTH_CHARS:
            _merge_unit(units[i], units.pop(i + 1))
        else:
            i += 1

    # If the last unit in the chapter is still short, merge backward into previous unit.
    if len(units) >= 2 and len(units[-1]["text"]) < MIN_SYNTH_CHARS:
        _merge_unit(units[-2], units.pop())

    return units
