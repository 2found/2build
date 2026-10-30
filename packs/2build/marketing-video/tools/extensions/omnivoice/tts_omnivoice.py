#!/usr/bin/env python3
"""Voice a marketing script locally with OmniVoice, then hear it back with Whisper.

Runs in its own Python environment for torch, OmniVoice and Whisper — by default an
ephemeral `uv run --with …` env built by extension.py, or $MVF_OMNIVOICE_PYTHON —
through the heavy prefix when one is configured. Its text frontend is the sibling
tts_text.py. `mvf voice --engine omnivoice` and `mvf voice-design` call it; run it by
hand only to debug.

  take   --job cues.json --out <take-dir>          one WAV per cue + manifest.json
  design --job design.json --out <voices-dir>      N candidate reference voices
  rehear --job say.json --out <take-dir>           re-listen to a take: new words + ASR verdicts, same audio

Each cue is cleaned and chunked by tts_text.py (chunk_plan),
synthesised chunk by chunk, transcribed with word timestamps, and compared with what it
should say. A cue that dropped or added two or more words is re-voiced with a
new seed, up to --retries times; the attempt with the fewest problems is kept and
any remaining problem is written to the manifest, never hidden.

Display words map to spoken words one to many (a brand respelling, a number said
as words), so captions keep the script's spelling with the voice's timing. Vietnamese
written forms (numbers, dates, times, money, percent, ranges) are read by
vi_numbers.py, on the script side and on what Whisper heard.
"""
from __future__ import annotations

import argparse
import difflib
import hashlib
import json
import re
import sys
import time
import unicodedata
import wave
from pathlib import Path

HERE = Path(__file__).resolve().parent
TOOLS = HERE.parents[1]
sys.path[:0] = [str(HERE), str(TOOLS)]
import tts_text  # noqa: E402  (stdlib-only text frontend, sibling file)
from mvflib.common import fold, say_map, words  # noqa: E402  (stdlib-only, safe in this env)
from vi_numbers import SPAN as VI_SPAN, normalize_vi  # noqa: E402

SETTINGS = dict(num_step=32, guidance_scale=2.5, position_temperature=3.0, class_temperature=0.0)
LANG_NAMES = {"vi": "vietnamese", "en": "english", "ja": "japanese", "zh": "chinese", "ko": "korean",
              "fr": "french", "de": "german", "es": "spanish", "pt": "portuguese", "id": "indonesian", "th": "thai"}
PERCENT = {"vi": "phần trăm", "en": "percent"}
SIL_DB = -40.0


def sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


# ---------- text: display → spoken ----------

def _expand_word(w: str, lang: str, say: dict) -> str:
    key = {k.lower(): v for k, v in say.items()}
    if w.lower() in key:
        return key[w.lower()]
    m = re.fullmatch(r"(\d+)(%?)", w)
    if m and lang == "vi":
        return normalize_vi(w)  # the worker's reader; num2words says "hai mươi bốn" for 24
    if m:
        from num2words import num2words
        n = num2words(int(m.group(1)), lang=lang).replace("-", " ")
        return n + (" " + PERCENT.get(lang, "percent") if m.group(2) else "")
    return w


def spoken_text(display: str, lang: str, say: dict) -> tuple[str, list[int]]:
    """The text to synthesise, and how many spoken tokens each display word became.
    A say key or a Vietnamese written form may span several display words
    ("2novel.app", "28/10/2026", "49.000đ"): its spoken tokens are shared out so every
    display word still gets at least one."""
    counts: list[int] = []
    multi = sorted((k for k in say if len(words(k)) > 1), key=len, reverse=True)
    pat = "|".join([re.escape(k) for k in multi] + ([VI_SPAN] if lang == "vi" else []) + [r"[\w'’%]+"])

    def sub(m):
        tok = m.group(0)
        n = len(words(tok))
        out = next((v for k, v in say.items() if k.lower() == tok.lower()), None)
        if out is None and lang == "vi" and re.fullmatch(VI_SPAN, tok, flags=re.I):
            out = normalize_vi(tok)
            if re.search(r"ngày\s*$", m.string[max(0, m.start() - 8):m.start()], flags=re.I):
                out = re.sub(r"^ngày ", "", out)  # "ngày 28/10/2026" must not say "ngày" twice
            if tok[:1].isupper():
                out = out[:1].upper() + out[1:]
        if out is None:
            out = _expand_word(tok, lang, say)
        k = len(fold(out))
        counts.extend([max(1, k - (n - 1))] + [1] * (n - 1))
        return out
    text = re.sub(pat, sub, unicodedata.normalize("NFC", display), flags=re.I)
    return text, counts


def _merge_letters(toks: list[str]) -> tuple[list[str], list[int]]:
    """Spelled-out letters ("m t l") and the acronym ASR writes ("mtl") compare as
    one token: merge runs of single letters. Returns merged tokens and, for each
    input token, the index of its merged token."""
    out, idx, run = [], [], False
    for t in toks:
        letter = len(t) == 1 and t.isalpha()
        if letter and run:
            out[-1] += t
        else:
            out.append(t)
        run = letter
        idx.append(len(out) - 1)
    return out, idx


_SPOKEN_SYMBOLS = {"dot", "chấm", "cham"}  # "cham": "chấm" once fold() has dropped the tones
_DIGIT_WORDS = dict(zip("0123456789", "zero one two three four five six seven eight nine".split()))
_DIGIT_WORDS_VI = dict(zip("0123456789", "khong mot hai ba bon nam sau bay tam chin".split()))
_HOMOPHONES = {"to": "two", "too": "two"}  # "2novel" is said "to novel"


def _same_letters(spoken: list[str], heard: list[str]) -> bool:
    """True when two token runs differ only in spacing, a spoken "dot"/"chấm" or a digit
    written for its word, in English or Vietnamese: "to novel dot app" vs "2novel app",
    "lo hi hai cham com" vs "lohi2 com"."""
    def squash(toks, digits, homophones):
        s = "".join(homophones.get(t, t) for t in toks if t not in _SPOKEN_SYMBOLS)
        return "".join(digits.get(c, c) for c in s)
    return bool(spoken) and any(squash(spoken, d, h) == squash(heard, d, h)
                                for d in (_DIGIT_WORDS, _DIGIT_WORDS_VI) for h in (_HOMOPHONES, {}))


# Every language is cleaned by tts_text.clean_text, with two exceptions outside
# Vietnamese: written forms are not read in Vietnamese ("15:30" is not "mười lăm giờ ba
# mươi"; spoken_text has already said the numbers), and an apostrophe inside a word
# ("I'm", "don't") survives, carried through the punctuation whitelist as U+02BC, which is
# a letter.
_IN_WORD_APOSTROPHE = re.compile(r"(?<=\w)['’](?=\w)")
_APOSTROPHE_LETTER = "\u02bc"


def _cleaned(fn, texts: list[str], lang: str) -> list:
    """Run a tts_text function that cleans its input, with the language exceptions above."""
    if lang == "vi":
        return fn(texts)
    read = tts_text._READ_WRITTEN_FORMS
    tts_text._READ_WRITTEN_FORMS = lambda t: t
    try:
        return fn([_IN_WORD_APOSTROPHE.sub(_APOSTROPHE_LETTER, t) for t in texts])
    finally:
        tts_text._READ_WRITTEN_FORMS = read


def _apostrophes(text: str) -> str:
    return text.replace(_APOSTROPHE_LETTER, "'")


def clean_for_tts(text: str, lang: str) -> str:
    return _apostrophes(_cleaned(lambda ts: tts_text.clean_text(ts[0]), [text], lang))


def pack_paragraphs(paras: list[str], lang: str) -> list[str]:
    """Synthesis units: engine.pack_paragraphs_for_synthesis, which cleans each paragraph,
    joins one under MIN_SYNTH_CHARS to the next (a short last one to the previous) and ends
    the seam in "." so it is still voiced as a sentence break."""
    units = _cleaned(tts_text.pack_paragraphs_for_synthesis, paras, lang)
    return [_apostrophes(u["text"]) for u in units]


MAX_CHUNK_WORDS = 20
MIN_CHUNK_WORDS = 4


def sentences(text: str, lang: str) -> list[str]:
    """Generation chunks. Vietnamese goes through tts_text's splitter, exactly as the
    worker voices a chapter. That splitter packs short sentences up to 280 chars and
    rewrites '?' as ','; one long chunk in voice-design mode comes out as noise (2026-10-27:
    a 10.8 s chunk was garbage, each sentence alone was clean). Elsewhere: one sentence per
    chunk, a long sentence cut at commas into <= MAX_CHUNK_WORDS words, and — the worker's
    MIN_SYNTH_CHARS idea at sentence scale — a fragment under MIN_CHUNK_WORDS ("Wow.",
    "Okay.") joins the next sentence (the last one joins the previous) so the model never
    voices a one-word input, while the chunk stays within MAX_CHUNK_WORDS."""
    if lang == "vi":
        return tts_text.split_into_sentences(text)
    out = []
    for sent in re.split(r"(?<=[.!?])\s+", text.strip()):
        if not sent:
            continue
        if len(sent.split()) <= MAX_CHUNK_WORDS:
            out.append(sent)
            continue
        cur = ""
        for part in re.split(r"(?<=,)\s+", sent):
            if cur and len((cur + " " + part).split()) > MAX_CHUNK_WORDS:
                out.append(cur)
                cur = part
            else:
                cur = f"{cur} {part}".strip()
        if cur:
            out.append(cur)

    def fits(a: str, b: str) -> bool:
        return len(a.split()) + len(b.split()) <= MAX_CHUNK_WORDS

    i = 0
    while i < len(out) - 1:
        if len(out[i].split()) < MIN_CHUNK_WORDS and fits(out[i], out[i + 1]):
            out[i] = f"{out[i]} {out.pop(i + 1)}"
        else:
            i += 1
    if len(out) >= 2 and len(out[-1].split()) < MIN_CHUNK_WORDS and fits(out[-2], out[-1]):
        last = out.pop()
        out[-1] = f"{out[-1]} {last}"
    return out


def chunk_plan(spoken: str, lang: str) -> list[list[str]]:
    """Generation chunks grouped by paragraph, as a long-form TTS worker voices a chapter
    (worker._split_paragraphs → engine.synthesize_chapter): each line of the cue is a
    paragraph, cleaned and packed into synthesis units (pack_paragraphs), and each unit is
    cut into sentence chunks. Say() puts SENTENCE_GAP_MS between the chunks of one unit
    and PARAGRAPH_GAP_MS between units."""
    paras = tts_text.split_paragraphs(spoken)
    return [s for s in (sentences(u, lang) for u in pack_paragraphs(paras, lang)) if s]


# ---------- audio helpers ----------

def wav_info(path: Path):
    import numpy as np
    with wave.open(str(path)) as w:
        sr, ch, sw = w.getframerate(), w.getnchannels(), w.getsampwidth()
        x = np.frombuffer(w.readframes(w.getnframes()), dtype={2: np.int16, 4: np.int32}[sw]).astype(float)
    x = x.reshape(-1, ch).mean(1) / (2 ** (8 * sw - 1))
    hop = int(sr * 0.01)
    db = np.array([20 * np.log10(np.sqrt(np.mean(x[i:i + hop] ** 2)) + 1e-9) for i in range(0, max(1, len(x) - hop), hop)])
    act = np.where(db > SIL_DB)[0]
    dur = len(x) / sr
    if not len(act):
        return dur, (0.0, dur)
    return dur, (max(0.0, act[0] * 0.01 - 0.03), min(dur, (act[-1] + 1) * 0.01 + 0.05))


# ---------- ASR and alignment ----------

class Ears:
    def __init__(self, model: str):
        import os
        import warnings
        os.environ.setdefault("HF_HUB_OFFLINE", "1")
        warnings.filterwarnings("ignore")
        from transformers import pipeline
        from transformers.utils import logging as hf_logging
        hf_logging.set_verbosity_error()
        self.asr = pipeline("automatic-speech-recognition", model=model, device="cpu")

    def hear(self, wav: Path, lang: str) -> dict:
        r = self.asr(str(wav), generate_kwargs={"language": LANG_NAMES.get(lang, lang), "task": "transcribe"},
                     return_timestamps="word")
        ws = []
        for c in r.get("chunks", []):
            s, e = c["timestamp"]
            if s is None:
                continue
            w = c["text"].strip()
            # Whisper writes "1300", "50%", "15:30": read them the way the script was voiced
            ws.append({"w": normalize_vi(w) if lang == "vi" else w, "s": float(s), "e": float(e if e is not None else s + 0.2)})
        return {"heard": r["text"].strip(), "words": ws}


def align(display: str, counts: list[int], spoken: str, heard: dict, speech: tuple[float, float]) -> tuple[list, list, list]:
    """Display words with times, plus dropped/added runs (>= 2 tokens)."""
    sp_raw = fold(spoken)
    sp, sp_map = _merge_letters(sp_raw)
    h_raw, h_times_raw = [], []
    for w in heard["words"]:
        for t in fold(w["w"]):
            h_raw.append(t)
            h_times_raw.append((w["s"], w["e"]))
    htoks, h_map = _merge_letters(h_raw)
    htimes = [None] * len(htoks)
    for i, j in enumerate(h_map):
        a, b = h_times_raw[i]
        htimes[j] = (a, b) if htimes[j] is None else (htimes[j][0], b)
    times: list = [None] * len(sp)
    dropped, added = [], []
    for op, i1, i2, j1, j2 in difflib.SequenceMatcher(None, sp, htoks, autojunk=False).get_opcodes():
        if op == "equal":
            for k in range(i2 - i1):
                times[i1 + k] = htimes[j1 + k]
        elif op == "replace" and (i2 - i1) == (j2 - j1):  # misheard tones/spellings: same length, keep times
            for k in range(i2 - i1):
                times[i1 + k] = htimes[j1 + k]
        elif op == "replace" and _same_letters(sp[i1:i2], htoks[j1:j2]):
            # Spacing only: "two novel dot app" heard as "twonovel.app". Spread the heard span.
            a, b = htimes[j1][0], htimes[j2 - 1][1]
            step = (b - a) / (i2 - i1)
            for k in range(i2 - i1):
                times[i1 + k] = (a + step * k, a + step * (k + 1))
        else:
            if (i2 - i1) - (j2 - j1) >= 2 or (op == "delete" and i2 - i1 >= 2):
                dropped.append(" ".join(sp[i1:i2]))
            if (j2 - j1) - (i2 - i1) >= 2 or (op == "insert" and j2 - j1 >= 2):
                added.append(" ".join(htoks[j1:j2]))
    # Interpolate unmatched tokens between known neighbours (or the speech span).
    known = [(i, t) for i, t in enumerate(times) if t]
    n = len(sp)
    for i in range(n):
        if times[i]:
            continue
        prev = max((k for k in known if k[0] < i), default=None, key=lambda k: k[0])
        nxt = min((k for k in known if k[0] > i), default=None, key=lambda k: k[0])
        a_i, a_t = (prev[0], prev[1][1]) if prev else (-1, speech[0])
        b_i, b_t = (nxt[0], nxt[1][0]) if nxt else (n, speech[1])
        step = (b_t - a_t) / max(1, b_i - a_i)
        times[i] = (a_t + step * (i - a_i - 1) + 0.0, a_t + step * (i - a_i))
    times = [times[j] for j in sp_map]  # back to one entry per spoken token
    # Display words take the span of their spoken tokens.
    out, k = [], 0
    for w, c in zip(words(display), counts):
        span = times[k:k + c] or [(speech[1], speech[1])]
        out.append({"w": w, "s": round(span[0][0], 3), "e": round(max(span[-1][1], span[0][0] + 0.05), 3)})
        k += c
    # Monotonic, inside the speech span.
    last = speech[0]
    for w in out:
        w["s"] = max(w["s"], last)
        w["e"] = max(w["e"], w["s"] + 0.05)
        last = w["s"]
    return out, dropped, added


# ---------- synthesis ----------

class Voice:
    def __init__(self):
        import torch
        from omnivoice import OmniVoice
        self.torch = torch
        device = "mps" if torch.backends.mps.is_available() else "cpu"
        t0 = time.time()
        self.model = OmniVoice.from_pretrained("k2-fsa/OmniVoice", device_map=device,
                                               dtype=torch.float16 if device != "cpu" else torch.float32,
                                               attn_implementation="eager")
        print(f"OmniVoice loaded on {device} in {time.time() - t0:.0f}s", flush=True)
        self.prompt = None

    def clone(self, wav: Path, text: str):
        self.prompt = self.model.create_voice_clone_prompt(ref_audio=str(wav), ref_text=text, preprocess_prompt=True)

    def use(self, wav: Path | None, text: str | None):
        """Switch the clone prompt; prompts are kept, so lines alternating two voices build each once."""
        cache = self.__dict__.setdefault("_prompts", {})
        if wav is None:
            self.prompt = cache.get(None)
            return
        key = str(wav)
        if key not in cache:
            self.clone(wav, text)
            cache[key] = self.prompt
        self.prompt = cache[key]

    def say(self, plan: list[list[str]], lang: str, speed: float, seed: int, instruct: str | None = None):
        """Voice a chunk_plan: chunks of a paragraph SENTENCE_GAP_MS apart, paragraphs PARAGRAPH_GAP_MS."""
        from tts_text import PARAGRAPH_GAP_MS, SAMPLE_RATE, SENTENCE_GAP_MS
        torch = self.torch
        torch.manual_seed(seed)
        parts = []
        gaps = {ms: torch.zeros(int(SAMPLE_RATE * ms / 1000)) for ms in (SENTENCE_GAP_MS, PARAGRAPH_GAP_MS)}
        for unit in plan:
            for i, chunk in enumerate(unit):
                kw = dict(text=chunk, language=lang, speed=speed, **SETTINGS)
                if self.prompt is not None:
                    kw["voice_clone_prompt"] = self.prompt
                elif instruct:
                    kw["instruct"] = instruct
                audio = torch.cat(self.model.generate(**kw), dim=-1)
                if audio.dim() == 2:
                    audio = audio.squeeze(0)
                if parts:
                    parts.append(gaps[SENTENCE_GAP_MS if i else PARAGRAPH_GAP_MS])
                parts.append(audio.float().cpu())
        return torch.cat(parts), SAMPLE_RATE

    def save(self, path: Path, audio, sr: int):
        import torchaudio
        torchaudio.save(str(path), audio.unsqueeze(0), sr)


def voice_cue(v: Voice, ears: Ears, cue: dict, lang: str, speed: float, say: dict, out: Path, retries: int,
              instruct: str | None = None) -> dict:
    spoken, counts = spoken_text(cue["text"], lang, say)
    plan = chunk_plan(spoken, lang)
    if not plan:
        raise SystemExit(f"{cue['id']}: nothing left to say after cleaning")
    tts_text = "\n".join(" ".join(unit) for unit in plan)
    best = None
    base = int(hashlib.sha1(cue["id"].encode()).hexdigest()[:6], 16)
    for attempt in range(retries + 1):
        audio, sr = v.say(plan, lang, speed, seed=base + attempt, instruct=instruct)
        wav = out / f"{cue['id']}.try{attempt}.wav"
        v.save(wav, audio, sr)
        dur, speech = wav_info(wav)
        heard = ears.hear(wav, lang)
        ws, dropped, added = align(cue["text"], counts, spoken, heard, speech)
        score = len(" ".join(dropped).split()) + len(" ".join(added).split())
        rec = {"wav": wav, "dur": dur, "speech": speech, "heard": heard["heard"], "words": ws,
               "dropped": dropped, "added": added, "score": score, "attempt": attempt}
        print(f"  {cue['id']} try {attempt}: {dur:.2f}s" + (f"  dropped={dropped} added={added}" if score else "  ok"), flush=True)
        if best is None or score < best["score"]:
            best = rec
        if score == 0:
            break
    final = out / f"{cue['id']}.wav"
    best["wav"].replace(final)
    for p in out.glob(f"{cue['id']}.try*.wav"):
        p.unlink()
    return {"id": cue["id"], "text": cue["text"], "spoken": tts_text, "path": final.name, "sha256": sha(final),
            "duration_s": round(best["dur"], 3), "speech": [round(best["speech"][0], 3), round(best["speech"][1], 3)],
            "words": best["words"], "asr": {"heard": best["heard"], "dropped": best["dropped"], "added": best["added"],
                                            "attempts": best["attempt"] + 1}}


def run_take(job: dict, out: Path, retries: int, asr_model: str) -> None:
    lang, voice, say = job["language"], job["voice"], say_map(job.get("say"))
    speed = float(voice.get("speed", 1.0))
    if out.exists() and any(out.iterdir()):
        raise SystemExit(f"{out} is not empty: voice into a new take dir")
    out.mkdir(parents=True, exist_ok=True)
    ref = None
    if voice["mode"] == "clone":
        ref = Path(voice["ref_wav"])
        ref_text = Path(voice["ref_txt"]).read_text(encoding="utf-8").strip()
    v = Voice()
    if ref:
        v.clone(ref, ref_text)
    v.__dict__.setdefault("_prompts", {})[None] = v.prompt
    ears = Ears(asr_model)
    cues = []
    for c in job["cues"]:
        # a cue with its own reference (a second voice) speaks as a clone of it; the rest keep the script voice
        own = c.get("ref_wav")
        v.use(Path(own) if own else None, Path(c["ref_txt"]).read_text(encoding="utf-8").strip() if own else None)
        rec = voice_cue(v, ears, c, lang, speed, say, out, retries, instruct=None if own else voice.get("instruct"))
        if c.get("voice"):
            rec["voice"] = c["voice"]
            rec["voice_reference"] = {"wav_sha256": sha(Path(own))}
        cues.append(rec)
    bad = [c["id"] for c in cues if c["asr"]["dropped"] or c["asr"]["added"]]
    manifest = {"engine": "local-omnivoice", "language": lang, "voice": voice, "settings": {**SETTINGS, "speed": speed},
                "asr_model": asr_model, "voice_reference": {"wav_sha256": sha(ref), "transcript": ref_text} if ref else None,
                "cues": cues, "asr_problems": bad, "qa": "PENDING LISTEN"}
    (out / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"VOICED {len(cues)} cues → {out}" + (f"; ASR problems left in {bad}" if bad else "; every cue says its line"))


def run_rehear(job: dict, take: Path, asr_model: str) -> None:
    """Re-listen to an existing take (after an alignment fix): same audio, new words and ASR verdicts."""
    mp = take / "manifest.json"
    manifest = json.loads(mp.read_text(encoding="utf-8"))
    lang, say, ears = manifest["language"], say_map(job.get("say")), Ears(asr_model)
    for c in manifest["cues"]:
        spoken, counts = spoken_text(c["text"], lang, say)
        wav = take / c["path"]
        dur, speech = wav_info(wav)
        heard = ears.hear(wav, lang)
        ws, dropped, added = align(c["text"], counts, spoken, heard, speech)
        c.update(words=ws, asr={**c["asr"], "heard": heard["heard"], "dropped": dropped, "added": added})
        print(f"  {c['id']}: " + (f"dropped={dropped} added={added}" if dropped or added else "ok"), flush=True)
    manifest["asr_problems"] = [c["id"] for c in manifest["cues"] if c["asr"]["dropped"] or c["asr"]["added"]]
    mp.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"REHEARD {len(manifest['cues'])} cues in {take}; ASR problems: {manifest['asr_problems'] or 'none'}")


def run_design(job: dict, out: Path, asr_model: str) -> None:
    """Candidates for a designed voice; the picked one becomes a clone reference."""
    out.mkdir(parents=True, exist_ok=True)
    v, ears = Voice(), Ears(asr_model)
    lang, text = job["language"], job["sample"]
    # the sample is read like a cue: "1.300", "15:30" said as words, compared with what was said
    spoken, counts = spoken_text(text, lang, say_map(job.get("say")))
    plan = chunk_plan(spoken, lang)
    rows = []
    for k in range(job.get("n", 3)):
        name = f"{job['name']}.cand{k + 1}"
        audio, sr = v.say(plan, lang, 1.0, seed=1000 + k, instruct=job["instruct"])
        wav = out / f"{name}.wav"
        v.save(wav, audio, sr)
        (out / f"{name}.txt").write_text(text + "\n", encoding="utf-8")
        dur, speech = wav_info(wav)
        heard = ears.hear(wav, lang)
        _, dropped, added = align(text, counts, spoken, heard, speech)
        rows.append({"name": name, "duration_s": round(dur, 2), "heard": heard["heard"], "dropped": dropped, "added": added})
        print(f"  {name}: {dur:.1f}s" + (f" dropped={dropped} added={added}" if dropped or added else " ok"), flush=True)
    (out / f"{job['name']}.candidates.json").write_text(json.dumps({"instruct": job["instruct"], "language": lang,
                                                                    "sample": text, "candidates": rows}, ensure_ascii=False, indent=2) + "\n")
    print(f"DESIGNED {len(rows)} candidates → {out}")


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("mode", choices=["take", "design", "rehear"])
    ap.add_argument("--job", type=Path, required=True)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--retries", type=int, default=2)
    ap.add_argument("--asr-model", default="openai/whisper-small")
    a = ap.parse_args()
    job = json.loads(a.job.read_text(encoding="utf-8"))
    if a.mode == "take":
        run_take(job, a.out.resolve(), a.retries, a.asr_model)
    elif a.mode == "rehear":
        run_rehear(job, a.out.resolve(), a.asr_model)
    else:
        run_design(job, a.out.resolve(), a.asr_model)


if __name__ == "__main__":
    main()
