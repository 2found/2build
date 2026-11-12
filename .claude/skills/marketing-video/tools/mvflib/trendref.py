"""Reference videos for the trend stage: public metadata of a TikTok or X post via
yt-dlp (no login, no download unless --frames), engagement normalised by views,
and optionally the first seconds as frames so the hook can be studied, not guessed."""
from __future__ import annotations

import json
import subprocess
from pathlib import Path

from .common import dump, load, tile_width, today


def ytdlp(args: list[str], limit: float = 120) -> subprocess.CompletedProcess:
    """yt-dlp, given up on after `limit` seconds: a throttled site holds the connection open and never answers."""
    try:
        return subprocess.run(["yt-dlp", *args], capture_output=True, text=True, timeout=limit)
    except subprocess.TimeoutExpired:
        return subprocess.CompletedProcess(args, 124, "", f"no answer after {limit:g} s")


def meta(url: str) -> dict:
    r = ytdlp(["-J", "--no-warnings", "--skip-download", url])
    if r.returncode:
        raise SystemExit(f"yt-dlp could not read {url}: {r.stderr.strip()[-400:]}")
    d = json.loads(r.stdout)
    views = d.get("view_count") or 0  # yt-dlp leaves a count out (or 0) when the page hid it: unknown, not zero
    if not views:
        print(f"WARN {url}: no view count in the public page this time; re-run later before trusting it")
    eng = sum(d.get(k) or 0 for k in ("like_count", "comment_count", "repost_count"))
    host = "tiktok" if "tiktok" in (d.get("extractor") or "").lower() else "x" if d.get("extractor", "").lower() in {"twitter", "x"} else d.get("extractor")
    return {
        "url": d.get("webpage_url") or url, "platform": host, "id": d.get("id"),
        "creator": d.get("uploader") or d.get("channel") or d.get("uploader_id"),
        "followers": d.get("channel_follower_count"),
        "posted": _date(d.get("upload_date")), "duration_s": d.get("duration"),
        "text": (d.get("description") or d.get("title") or "").strip()[:1000],
        "sound": " — ".join(x for x in (d.get("track"), d.get("artist")) if x) or None,
        "metrics": {k: d.get(k) for k in ("view_count", "like_count", "comment_count", "repost_count") if d.get(k)},
        "engagement_rate": round(eng / views, 4) if views else None,
        "observed_at": today().isoformat(),
    }


def _date(s):
    return f"{s[:4]}-{s[4:6]}-{s[6:]}" if s and len(s) == 8 else s


def frames(url: str, out: Path, seconds: float = 3.0) -> list[str]:
    """Download once into its own dir, then the first `seconds` as 4 fps frames + a sheet."""
    out.mkdir(parents=True, exist_ok=True)
    vid = out / "video.mp4"
    if not vid.exists():
        r = ytdlp(["-q", "--no-warnings", "-f", "mp4/best", "-o", str(vid), url], 600)
        if r.returncode or not vid.exists():
            raise SystemExit(f"yt-dlp download failed for {url}: {r.stderr.strip()[-400:]}")
    pr = subprocess.run(["ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of",
                         "csv=p=0", str(vid)], capture_output=True, text=True)
    try:
        W, H = (int(x) for x in pr.stdout.strip().split(",")[:2])
    except ValueError:
        W, H = 9, 16
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-t", str(seconds), "-i", str(vid),
                    "-vf", f"fps=4,scale={tile_width(W, H, 6, 2, 270)}:-2,tile=6x2:padding=4", "-frames:v", "1",
                    str(out / "hook-sheet.jpg")], check=True)
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(vid),
                    "-vf", f"fps=1/2,scale={tile_width(W, H, 8, 4, 200)}:-2,tile=8x4:padding=4", "-frames:v", "1",
                    str(out / "contact.jpg")], check=True)
    return [str(out / "hook-sheet.jpg"), str(out / "contact.jpg")]


def add(campaign_dir: Path, urls: list[str], with_frames: bool, note: str | None) -> list[dict]:
    path = campaign_dir / "03-trends/trends.json"
    data = load(path) if path.is_file() else {"trends": [], "references": []}
    refs = data.setdefault("references", [])
    added = []
    for u in urls:
        m = meta(u)
        if note:
            m["note"] = note
        if with_frames:
            m["frames"] = [str(Path(p).relative_to(campaign_dir)) for p in
                           frames(u, campaign_dir / "03-trends/refs" / f"{m['platform']}-{m['id']}")]
        old = next((r for r in refs if r.get("url") == m["url"]), None)
        if old and not m["metrics"].get("view_count") and (old.get("metrics") or {}).get("view_count"):
            m["metrics"], m["engagement_rate"] = old["metrics"], old.get("engagement_rate")
            m["note"] = f"{m.get('note') or ''} counts from {old.get('observed_at')}: the page hid them on {m['observed_at']}".strip()
        refs[:] = [r for r in refs if r.get("url") != m["url"]]
        refs.append(m)
        added.append(m)
        er = f"{m['engagement_rate'] * 100:.1f}%" if m["engagement_rate"] is not None else "n/a"
        print(f"{m['platform']} {m['creator']}: {m['metrics'].get('view_count', '?')} views, ER {er}, "
              f"{m['duration_s']}s, sound={m['sound']}")
    dump(path, data)
    return added
