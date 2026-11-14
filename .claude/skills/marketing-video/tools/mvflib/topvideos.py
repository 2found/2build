"""What the most-viewed videos say. A hook copied from a top video without its content repeats the form and misses
why people watched: the problem it was about, the answer it gave, the order it said things in. This pulls the top
results for a search (logged out), keeps each one's metadata and the first minutes of its auto-subs as plain text in
03-research/top/<id>.txt, and prints them most-viewed first, so the writer reads them before any idea exists.

What mvf cannot do is handed to the agent as `AGENT:` lines and a row's `agent` list: a TikTok search needs a real
browser (the browse skill), TikTok comments are not in yt-dlp, and a video with no subtitles is read from its frames.
Speech recognition is not a fallback: these edits are mostly music, and Whisper invents speech over music."""
from __future__ import annotations

import json
import re
import subprocess
import tempfile
import unicodedata
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

from .common import dump, load, today
from .trendref import frames as frames_of, ytdlp

TOP = "03-research/top"


def fold(text: str) -> str:
    """Lower case without accents, so "Cổ Chân Nhân" matches a title written "co chan nhan"."""
    t = unicodedata.normalize("NFD", (text or "").lower().replace("đ", "d"))
    return "".join(c for c in t if not unicodedata.combining(c))


def on_topic(title: str, query: str) -> bool:
    """A search returns whatever is popular near the words: a title that shares none of them is off topic."""
    want = {w for w in re.findall(r"\w+", fold(query)) if len(w) > 1}
    return not want or bool(want & set(re.findall(r"\w+", fold(title))))


def search(query: str, n: int, max_min: float) -> list[dict]:
    r = ytdlp(["--no-warnings", "--flat-playlist", "-J", f"ytsearch{max(n * 4, 20)}:{query}"])
    if r.returncode:
        raise SystemExit(f"yt-dlp search failed: {r.stderr.strip()[-400:]}")
    hits = [e for e in json.loads(r.stdout).get("entries") or [] if e.get("view_count")]
    keep = [e for e in hits if on_topic(e.get("title") or "", query)
            and not (max_min and (e.get("duration") or 0) > max_min * 60)]
    if len(keep) < len(hits):
        print(f"skipped {len(hits) - len(keep)} results off topic (no search word in the title) or longer than "
              f"{max_min:g} min; pass --url to read one anyway")
    return sorted(keep, key=lambda e: -e["view_count"])[:n]


def tiktok_search(query: str, scope: str) -> str:
    """The agent's steps for a TikTok search, as checked logged out on 2026-11-14 (references/03-trends.md).
    TikTok blocks a loaded search or tag URL, so the steps type into the search box like a person."""
    words = re.findall(r"\w+", fold(query))
    tag = "#" + "".join(words) if len(words) <= 3 else "<the title's hashtag, no spaces or accents>"
    js = ("JSON.stringify([...new Map([...document.querySelectorAll(\"a[href*=\\\"/video/\\\"]\")].map(a=>{let c=a;"
          "while(c.parentElement&&new Set([...c.parentElement.querySelectorAll(\"a[href*=\\\"/video/\\\"]\")]"
          ".map(x=>x.href)).size<2)c=c.parentElement;return [a.href,a.href+\" \"+c.innerText.replace(/\\s+/g,\" \")"
          ".slice(0,200)]})).values()])")
    return "\n".join([
        "AGENT: mvf cannot search TikTok (it needs a real browser). With the browse skill, logged out:",
        "AGENT:   open https://www.tiktok.com/ (the home page; never load a /search or /tag URL, TikTok blocks it), "
        f"snapshot -i, click the Search box, keyboard type {query!r}, press Enter",
        f"AGENT:   then the hashtag {tag}: click it in a caption or the Hashtags tab, or type it in the search box",
        "AGENT:   on each list, collect each post's link and card text (likes, caption, creator) at once: the page swaps "
        f"its results a few seconds after it loads. agent-browser eval '{js}'",
        f"AGENT:   then run: mvf top-videos --url URL... --scope {scope}  (views and duration come from yt-dlp; "
        "it downloads each video, so run a long list in the background)",
    ])


def reading_frames(url: str, out: Path, length: float | None) -> list[str]:
    """The whole video as up to 4 sheets of 3x3 frames, large enough to read on-screen text, in order."""
    frames_of(url, out)  # downloads video.mp4 once and cuts the hook sheet
    for old in out.glob("read-*.jpg"):
        old.unlink()
    fps = min(0.5, 36 / length) if length else 0.5
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(out / "video.mp4"), "-vf",
                    f"fps={fps:.4f},scale=480:-2,tile=3x3:padding=4", "-frames:v", "4", str(out / "read-%d.jpg")],
                   check=True)
    return [str(out / "hook-sheet.jpg"), *map(str, sorted(out.glob("read-*.jpg")))]


def agent_todo(row: dict, url: str, platform: str, frames: list[str] | None) -> list[str]:
    """What only the agent can still do for one video."""
    todo = []
    if not row["transcript"]:
        seen = f"look at {', '.join(frames)} (on-screen text, pictures), " if frames else ""
        todo.append(f"no subtitles: {seen}watch {url} and write what it shows and says in {TOP}/{row['id']}.txt; "
                    f"research.json top_videos[].transcript names that file")
    if platform == "tiktok":
        todo.append("caption and comments: yt-dlp reads no TikTok comments; with the browse skill open the video "
                    "by clicking it from the search results or hashtag list you reached through the search box "
                    "(a loaded video, search or tag URL is blocked) and read "
                    "[data-e2e=browse-video-desc] and [data-e2e=comment-level-1]")
    return todo


def _ts(x: str) -> float:
    h, m, s = x.split(":")
    return int(h) * 3600 + int(m) * 60 + float(s)


def vtt_text(vtt: str, seconds: float) -> str:
    """Auto-sub VTT to plain text up to `seconds`: each cue's new words once, in order."""
    out, last = [], ""
    for m in re.finditer(r"(\d\d:\d\d:\d\d\.\d+) --> [^\n]*\n(.*?)(?:\n\n|\Z)", vtt, re.S):
        if seconds and _ts(m.group(1)) > seconds:
            break
        line = re.sub(r"<[^>]+>", "", m.group(2).strip().split("\n")[-1]).strip()
        if line and line != last:
            out.append(line[len(last):].strip() if line.startswith(last) else line)
            last = line
    return re.sub(r"\s+", " ", " ".join(out)).strip()


def transcribe(url: str, seconds: float, langs: list[str]) -> tuple[dict, str, str | None]:
    """Metadata and subtitles in the campaign's languages; English only if those give nothing (a video is often
    in the topic's own language). One language set per request: a blocked English track (HTTP 429) must not cost
    the Vietnamese one. A failed subtitle download still keeps the metadata."""
    meta, errs = None, []
    for want in dict.fromkeys([tuple(dict.fromkeys(langs)), ("en",)]):
        with tempfile.TemporaryDirectory() as d:  # downloaded data stays in its own empty dir
            r = ytdlp(["-i", "--skip-download", "--write-auto-subs", "--write-subs",
                       "--sub-langs", ",".join(x for lang in want for x in (lang, f"{lang}-orig")),
                       "--sub-format", "vtt", "--sleep-subtitles", "1", "--retries", "3",
                       "--write-info-json", "-o", f"{d}/v", url], 180)
            info = Path(d, "v.info.json")
            if not info.is_file():
                raise RuntimeError(f"yt-dlp could not read {url}: {r.stderr.strip()[-300:]}")
            meta = meta or json.loads(info.read_text(encoding="utf-8"))
            subs = {f.name.split(".")[1].split("-")[0]: f for f in sorted(Path(d).glob("v.*.vtt"))}
            pick = next((lang for lang in want if lang in subs), None)
            text = vtt_text(subs[pick].read_text(encoding="utf-8"), seconds) if pick else ""
            if text:
                return meta, text, pick
            if not (meta.get("subtitles") or meta.get("automatic_captions")):
                break  # the video has no tracks at all (TikTok): another language set would fetch it again for nothing
            # with -i a failed subtitle is only a WARNING line: keep it, or "blocked" reads as "has none"
            errs += [ln for ln in r.stderr.splitlines() if ln.startswith("ERROR") or "nable to download" in ln]
    if errs:
        meta["_sub_error"] = errs[0][:200]
    return meta, "", None


def study(cd: Path, query: list[str], urls: list[str], n: int, seconds: float, scope: str,
          max_min: float = 30, platform: str = "youtube") -> list[dict]:
    langs = (load(cd / "campaign.json").get("languages") or ["vi"]) if (cd / "campaign.json").is_file() else ["vi"]
    q = " ".join(query).strip()
    if not q and not urls:
        raise SystemExit("give search words (as the audience types them) or --url")
    if q and platform == "tiktok":
        print(tiktok_search(q, scope))
        if not urls:
            return []
        q = ""
    targets = [e.get("url") or f"https://www.youtube.com/watch?v={e['id']}" for e in search(q, n, max_min)] if q else []
    out_dir = cd / TOP
    out_dir.mkdir(parents=True, exist_ok=True)

    def read(u: str) -> tuple[dict | None, str]:
        """One video's row, and what to say about it; a blocked video (HTTP 429, removed, private) costs no other."""
        try:
            meta, text, sub_lang = transcribe(u, seconds, langs)
        except RuntimeError as e:
            return None, f"SKIP {e}"
        vid = re.sub(r"[^\w-]", "", str(meta.get("id") or "x"))
        txt = out_dir / f"{vid}.txt"
        if text or not (txt.is_file() and txt.read_text(encoding="utf-8").strip()):  # keep what the agent wrote
            txt.write_text(text + "\n", encoding="utf-8")
        host = "tiktok" if "tiktok" in str(meta.get("extractor_key") or meta.get("extractor") or "").lower() else "youtube"
        row = {"id": vid, "url": meta.get("webpage_url") or u, "scope": scope, "query": q or None,
               "title": meta.get("title"),
               "caption": (meta.get("description") or "").strip()[:1000] or None, "views": meta.get("view_count"), "duration_s": meta.get("duration"),
               "creator": meta.get("uploader") or meta.get("channel"), "observed_at": today().isoformat(),
               "transcript": f"{TOP}/{vid}.txt" if text else None, "sub_lang": sub_lang,
               "transcript_s": seconds, "sub_error": meta.get("_sub_error"), "platform": host}
        frames, note = None, ""
        if not text and not row["sub_error"]:
            try:  # pictures the agent can read when nothing was said in subtitles
                frames = [str(Path(p).relative_to(cd)) for p in reading_frames(row["url"], out_dir / vid,
                                                                                 row["duration_s"])]
            except (SystemExit, subprocess.CalledProcessError) as e:
                note = f"WARN no frames for {row['url']}: {str(e)[-200:]}"
        row["frames"], row["agent"] = frames, agent_todo(row, row["url"], host, frames)
        dump(out_dir / f"{vid}.json", row)
        return row, note

    todo = list(dict.fromkeys(targets + list(urls)))  # one read per video: two would write one file
    # TikTok videos are downloaded and cut, so read them side by side; YouTube is read one at a time (HTTP 429)
    workers = 1 if any("youtu" in u for u in todo) else min(4, len(todo) or 1)
    print(f"reading {len(todo)} video(s){f', {workers} at a time' if workers > 1 else ''}", flush=True)
    rows, failed = [], []
    with ThreadPoolExecutor(workers) as pool:
        for u, (row, note) in zip(todo, pool.map(read, todo)):
            if note:
                print(note, flush=True)
            if row is None:
                failed.append(u)
                continue
            rows.append(row)
            if row["sub_error"]:
                failed.append(row["url"])
    rows.sort(key=lambda x: -(x["views"] or 0))
    for x in rows:
        note = x["transcript"] or (f"SUBTITLES BLOCKED, retry later: {x['sub_error']}" if x["sub_error"]
                                   else "NO SUBTITLES")
        print(f"{x['views'] or '?':>10} views  {x['duration_s'] or '?':>5}s  {x['title']}\n           {note}")
        for t in x["agent"]:
            print(f"           AGENT: {t}")
    if failed:
        print(f"\n{len(failed)} could not be read; retry later with --url {' '.join(failed)}")
    print(f"\nread every transcript, then add each video to research.json top_videos with what it is about (subject), "
          f"what it tells the viewer (says) and the order it says it in (moves); then write the patterns they share "
          f"(references/03b-research.md#what-the-top-videos-say)")
    return rows
