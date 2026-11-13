"""The evaluator: Micro Fish, a simulated society on the user's local Soot, judges the candidate scripts and
the finished cut. Every persona is its own Soot: it meets the video in a feed among ordinary posts, skips or
watches part of it, reacts, and is interviewed afterwards; an expert panel records findings; mvf reads the
deterministic statistics (the tally) back to draft the decision. Micro Fish is a general mechanism: mvf only
writes its brief (a video as text, the questions, the lenses) and reads its tally.

The user owns Soot and the AI account: they sign in to a provider once (`soot login <provider>`) and name the
model on first use (`mvf judge setup --provider P --model M`). mvf keeps its own AI config and Micro Fish
workspace under ~/.babysit/marketing-video/, so the choice never edits ~/.soot/config.json and never
reaches the user's other Soot workspaces. No key is ever read, written or printed here: the AI config holds
the provider entry exactly as Soot stores it (account ids and the name of a key variable)."""
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
from pathlib import Path

from . import timing
from .common import PLATFORMS, Report, config, dump, load, save_user_config, today, user_dir
from .pretest import SIZES, caption_of

STIMULUS_CHARS = 12000  # Micro Fish refuses a longer subject (about six minutes of Vietnamese voice)
LABELS = "abcd"  # Micro Fish takes up to 4 subjects, one arm of the audience each
AUDIENCE = {"age": [18, 45]}  # who meets a short video, unless 02-audience/segments.json names a `panel` filter
PORT = "7811"  # not the default 7801, so it never collides with the user's own Micro Fish server
NEEDS_CONTEXT = 3  # exit code: the user must decide (sign in, pick a model)


# ---------- where things live ----------

def home() -> Path:
    return user_dir() / "micro-fish"


def ai_config_path() -> Path:
    return user_dir() / "soot-ai.json"


def env() -> dict:
    return {**os.environ, "MICRO_FISH_HOME": str(home()), "MICRO_FISH_AI_CONFIG": str(ai_config_path()),
            "MICRO_FISH_PORT": os.environ.get("MVF_MICRO_FISH_PORT", PORT)}


def micro_fish() -> Path | None:
    """micro-fish.sh from the micro-fish skill: $MVF_MICRO_FISH, then the usual skill dirs."""
    env_p = os.environ.get("MVF_MICRO_FISH", "").strip()
    cands = [Path(env_p).expanduser()] if env_p else []
    h = Path.home()
    cands += [h / d / "skills/micro-fish/micro-fish.sh" for d in (".claude", ".codex", ".agents", ".config/opencode")]
    return next((c for c in cands if c.is_file()), None)


def soot_bin() -> str | None:
    return os.environ.get("MVF_SOOT") or shutil.which("soot")


def choice() -> dict | None:
    j = config().get("judge") or {}
    return j if j.get("provider") and j.get("model") else None


# ---------- Soot state ----------

def providers() -> list[dict]:
    """`soot providers --json`: each configured provider and whether its credential is available (no network)."""
    sb = soot_bin()
    if not sb:
        return []
    r = subprocess.run([sb, "providers", "--json"], capture_output=True, text=True)
    try:
        return json.loads(r.stdout) if r.returncode == 0 else []
    except json.JSONDecodeError:
        return []


def soot_ai_config() -> dict:
    p = Path(os.environ.get("SOOT_HOME") or Path.home() / ".soot") / "config.json"
    try:
        return load(p)
    except (OSError, json.JSONDecodeError):
        return {}


def status() -> dict:
    """What the evaluator needs, each with the fix the user (or the agent) applies."""
    sb, mf, ch = soot_bin(), micro_fish(), choice()
    ps = providers() if sb else []
    ready_ps = [p["id"] for p in ps if p.get("credential") == "available"]
    deployment = home() / "society/deployment.json"
    out = {"soot": sb, "micro_fish": str(mf) if mf else None, "providers": ps, "signed_in": ready_ps,
           "choice": ch, "workspace": deployment.is_file(), "problems": []}
    if not sb:
        out["problems"].append(("soot", "Soot is not installed", "install Soot (https://github.com/2found/2agent) so `soot version` works"))
    if not mf:
        out["problems"].append(("micro-fish", "the micro-fish skill is not installed",
                                "link soot-forge's groups/society-sim/claude-code/micro-fish to ~/.claude/skills/micro-fish, or set MVF_MICRO_FISH"))
    if sb and not ready_ps:
        ids = ", ".join(p["id"] for p in ps) or "none configured"
        out["problems"].append(("provider", f"no Soot AI provider is signed in ({ids})",
                                "the user runs `! soot login <provider>` (or adds an API-key provider in Soot Nest → Settings → AI providers)"))
    if sb and ready_ps and not ch:
        out["problems"].append(("model", "no evaluator model chosen yet (first use)",
                                "ask the user which provider and model Micro Fish should use, then `mvf judge setup --provider P --model M`"))
    if ch and ps and ch["provider"] not in ready_ps:
        out["problems"].append(("provider", f"the chosen provider {ch['provider']} is not signed in",
                                f"the user runs `! soot login {ch['provider']}`, or `mvf judge setup` with another provider"))
    if ch and not deployment.is_file():
        out["problems"].append(("workspace", "the Micro Fish workspace is not created", "`mvf judge setup --provider "
                                f"{ch['provider']} --model {ch['model']}` (downloads the society-sim group once)"))
    return out


def print_status(st: dict) -> None:
    print(f"soot        {st['soot'] or 'MISSING'}")
    print(f"micro-fish  {st['micro_fish'] or 'MISSING'}")
    for p in st["providers"]:
        print(f"provider    {p['id']:<14} {p.get('credential')}")
    ch = st["choice"]
    print(f"model       {ch['provider'] + '/' + ch['model'] + ' (chosen ' + str(ch.get('set_at')) + ')' if ch else 'not chosen'}")
    print(f"workspace   {home() / 'society'} {'ok' if st['workspace'] else 'not created'}")
    cur = (soot_ai_config().get("tiers") or {}).get("flash") or soot_ai_config().get("model") or {}
    if not ch and cur.get("name"):
        print(f"hint        your Soot default is {cur.get('provider')}/{cur.get('name')}; Micro Fish runs one short "
              "session per persona per round, so a fast, cheap flash model with a generous rate limit fits best")
    for what, msg, fix in st["problems"]:
        print(f"NEEDS {what:<10} {msg} → {fix}")
    print("READY" if not st["problems"] else "NOT_READY")


def setup(provider: str, model: str, effort: str | None = None, soot_forge: str | None = None) -> Path:
    """Record the user's choice, write mvf's own AI config, and create the Micro Fish workspace once."""
    st = status()
    if not st["soot"] or not st["micro_fish"]:
        print_status(st)
        raise SystemExit(NEEDS_CONTEXT)
    ps = {p["id"]: p for p in st["providers"]}
    if provider not in ps:
        raise SystemExit(f"Soot has no provider {provider!r} (configured: {', '.join(ps) or 'none'}). The user adds it in "
                         f"Soot Nest → Settings → AI providers, or signs in with `soot login {provider}`")
    if ps[provider].get("credential") != "available":
        print(f"NEEDS provider  {provider} is not signed in → the user runs `! soot login {provider}`")
        raise SystemExit(NEEDS_CONTEXT)
    entry = (soot_ai_config().get("providers") or {}).get(provider)
    if not entry:
        raise SystemExit(f"{provider} is not in ~/.soot/config.json providers; add it in Soot first")
    sel = {"provider": provider, "name": model, **({"effort": effort} if effort else {})}
    dump(ai_config_path(), {"providers": {provider: entry}, "model": sel, "tiers": {"flash": sel, "lite": sel}})
    save_user_config({"judge": {"provider": provider, "model": model, **({"effort": effort} if effort else {}),
                                "set_at": today().isoformat()}})
    mf = str(micro_fish())
    subprocess.run([mf, "stop"], env=env(), capture_output=True, text=True)  # a running server keeps the old model
    if not (home() / "society/deployment.json").is_file():
        args = [mf, "setup"] + (["--from", soot_forge] if soot_forge else [])
        r = subprocess.run(args, env=env(), text=True, capture_output=True)
        print(r.stdout.strip())
        if r.returncode:
            raise SystemExit(f"micro-fish setup failed:\n{r.stderr.strip()}\n(with a soot-forge checkout: "
                             "`mvf judge setup … --soot-forge <dir>`)")
    chk = subprocess.run([st["soot"], "check", "--config", str(home() / "society/deployment.json"), "--secrets", "--show"],
                         env=env(), capture_output=True, text=True)
    first = next((ln for ln in chk.stdout.splitlines() if " model=" in ln), "")
    print((chk.stdout.splitlines() or [""])[0])
    if first:
        print(first.strip())
    if chk.returncode:
        raise SystemExit(f"soot check failed:\n{chk.stdout[-800:]}{chk.stderr[-800:]}")
    print(f"evaluator: Micro Fish on {provider}/{model} · config {ai_config_path()} · workspace {home() / 'society'}")
    return ai_config_path()


def require_ready() -> None:
    st = status()
    if st["problems"]:
        print_status(st)
        raise SystemExit(NEEDS_CONTEXT)


# ---------- subjects: a video as text ----------

def _vi(c: dict) -> bool:
    return (c.get("languages") or ["en"])[0].lower().startswith("vi")


def _rows(beats: list[dict], tl_beats: list[dict]) -> list[tuple]:
    return [(t["start"], t["end"], (b.get("visual") or "").strip(), (b.get("onscreen") or "").strip().replace("\n", " / "),
             (b.get("vo") or "").strip()) for b, t in zip(beats, tl_beats)]


def stimulus(rows: list[tuple], duration: float, timing_kind: str, caption: str, platform: str, vi: bool,
             chars: int = STIMULUS_CHARS) -> str:
    """One video as a viewer meets it, second by second: the text a persona reads in its feed.
    Over the limit, pictures are shortened first; the voice is never cut, because understanding is what is tested."""
    lab = PLATFORMS[platform]["label"]
    head = (f"Video {lab}, {duration:.0f} giây ({'đo từ giọng thật' if timing_kind == 'measured' else 'thời lượng ước tính'}). "
            "Đọc như video đang chạy: HÌNH là thứ thấy, CHỮ là chữ trên màn hình, GIỌNG là lời nghe MỘT lần, đúng tốc độ nói."
            if vi else
            f"{lab} video, {duration:.0f} s ({'measured voice' if timing_kind == 'measured' else 'estimated length'}). Read it as "
            "it plays: PICTURE is what you see, TEXT is on screen, VOICE is heard ONCE at speaking pace.")
    P, T, V = ("HÌNH", "CHỮ", "GIỌNG") if vi else ("PICTURE", "TEXT", "VOICE")
    cap = f"Caption: \"{caption.strip()}\"" if caption.strip() else ""
    for limit in (240, 140, 90, 50, 0):
        lines = [head]
        for a, b, pic, on, vo in rows:
            pic_s = pic if len(pic) <= limit else pic[: max(0, limit - 1)].rstrip() + "…"
            parts = [f"[{a:.1f}–{b:.1f}]"] + ([f"{P}: {pic_s}"] if limit and pic_s else []) + \
                    ([f"{T}: {on}"] if on else []) + ([f"{V}: \"{vo}\""] if vo else [])
            lines.append(" | ".join(parts))
        if cap:
            lines.append(cap)
        text = "\n".join(lines)
        if len(text) <= chars:
            return text
    raise SystemExit(f"this video is {len(text)} characters even without pictures; Micro Fish takes {chars} per "
                     "subject. Judge it in two runs, or shorten it")


def script_stimulus(script: dict, hook: str, manifest: dict | None, platform: str, vi: bool,
                    chars: int = STIMULUS_CHARS) -> tuple[str, dict]:
    sv = timing.variant(script, hook)
    tl = timing.build(sv, manifest)
    return stimulus(_rows(sv["beats"], tl["beats"]), tl["duration"], tl["timing"], caption_of(script), platform, vi,
                    chars), tl


def bench_stimulus(d: dict, platform: str, vi: bool, chars: int = STIMULUS_CHARS) -> str:
    rows = []
    for x in d.get("rows") or []:
        a, _, b = str(x.get("s", "0-0")).partition("-")
        rows.append((float(a), float(b or a), x.get("picture", ""), x.get("onscreen", ""), x.get("voice", "")))
    return stimulus(rows, rows[-1][1] if rows else 0.0, "measured", d.get("caption", ""), platform, vi, chars)


def video_stimulus(proj: Path, script: dict, platform: str, vi: bool, chars: int = STIMULUS_CHARS) -> str:
    """The cut as rendered: the measured timeline, what each beat's scene shows (from the storyboard and the
    direction), and what the reviewer saw on its frames."""
    tl = load(proj / "timeline.json")
    notes = {}
    rv = proj / "review.json"
    if rv.is_file():
        for i in load(rv).get("seen") or []:  # optional: {"at": s, "what": "..."} lines the reviewer wrote
            notes[float(i.get("at", -1))] = i.get("what", "")
    pics = {b["id"]: (b.get("visual") or "").strip() for b in script.get("beats") or []}
    sv = timing.variant(script, (load(proj / "mvf.json") or {}).get("hook"))
    by_id = {b["id"]: b for b in sv["beats"]}
    rows = []
    for t in tl["beats"]:
        b = by_id.get(t["id"], {})
        seen = [v for k, v in notes.items() if t["start"] <= k < t["end"]]
        pic = "; ".join([x for x in [pics.get(t["id"], "")] + seen if x])
        rows.append((t["start"], t["end"], pic, (b.get("onscreen") or "").strip(), (b.get("vo") or "").strip()))
    return stimulus(rows, tl["duration"], tl.get("timing", "measured"), caption_of(script), platform, vi, chars)


# ---------- the brief ----------

def questions(vi: bool) -> list[dict]:
    """What each persona is asked after the feed closes, about the one video it met. `retell` is the test of
    understanding; the tags are how the numbers find ad-feel, no-use and the words nobody followed."""
    if vi:
        return [
            {"id": "retell", "type": "text", "ask": "Mở đầu bằng 'Video nói là…' và kể lại điều video nói như kể cho bạn. Đã lướt qua hoặc không kể được thì nói thẳng."},
            {"id": "clarity", "type": "scale", "ask": "Nghe một lần có hiểu ngay không (1 không hiểu gì, 5 hiểu hết)"},
            {"id": "intent", "type": "scale", "ask": "Tuần này bạn có lưu, gửi, hoặc làm theo điều video nói không (1 chắc chắn không, 5 chắc chắn có; 'nghe hay' không tính)"},
            {"id": "use", "type": "text", "ask": "Video cho bạn dùng được gì trong đời mình (làm, quyết, biết)? Không gì thì nói không"},
            {"id": "objections", "type": "tags", "ask": "Mỗi điều vướng một nhãn: ad-feel nếu thấy như quảng cáo, no-use nếu không dùng được gì, unclear-<từ> cho mỗi chữ hay bước chưa hiểu, hard-to-believe, too-long"},
            {"id": "send", "type": "text", "ask": "Bạn sẽ gửi cho ai, kèm câu gì? Không gửi thì nói không"},
        ]
    return [
        {"id": "retell", "type": "text", "ask": "Start with 'It says…' and retell what the video told you as you would to a friend. If you scrolled past or cannot, say so."},
        {"id": "clarity", "type": "scale", "ask": "Did you get it hearing it once (1 not at all, 5 completely)"},
        {"id": "intent", "type": "scale", "ask": "Will you save, send or act on it this week (1 certainly not, 5 certainly; 'sounds nice' does not count)"},
        {"id": "use", "type": "text", "ask": "What can you use in your own life after it (do, decide, know)? Say nothing if nothing"},
        {"id": "objections", "type": "tags", "ask": "One tag per problem: ad-feel if it reads as an ad, no-use if you can use nothing, unclear-<word> for each word or step you did not follow, hard-to-believe, too-long"},
        {"id": "send", "type": "text", "ask": "Who would you send it to, with what words? Say nobody if nobody"},
    ]


def experts(c: dict, platform: str, vi: bool) -> list[dict]:
    lab, mk = PLATFORMS[platform]["label"], ", ".join(c.get("markets") or [])
    if vi:
        lenses = [f"Giám đốc sáng tạo video ngắn trên {lab} ở {mk}: hook 2 giây đầu, nhịp, điểm rơi người xem",
                  "Giáo viên giỏi giải thích: người lạ nghe MỘT lần có hiểu và kể lại được không, thuật ngữ nào chưa được giải nghĩa",
                  "Người hoài nghi kiểm chứng: câu nào là quảng cáo, câu nào khó tin hoặc sai sự thật",
                  "Người thuộc nhóm khán giả ưu tiên: video này cho tôi dùng được gì"]
    else:
        lenses = [f"Short-video creative director on {lab} in {mk}: the 2-second hook, pacing, drop-off points",
                  "Expert explainer and teacher: can a stranger who hears it ONCE retell it; which term is never explained",
                  "Skeptical fact-checker: which line reads as an ad, which is hard to believe or untrue",
                  "A member of the priority audience: what can I actually use from this"]
    return [{"id": i, "lens": x} for i, x in zip(("creative", "teacher", "skeptic", "audience"), lenses)]


def audience(cd: Path, c: dict, n: int) -> dict:
    """Who meets the video: a filter over Micro Fish's persona library. The campaign may name it as `panel` in
    02-audience/segments.json ({"age": [18, 35], "zone": …, "match": "regex"}); a campaign in a language Micro Fish
    ships no library for names its own library and feed in the user config (`judge.library`, `judge.context`)."""
    p = cd / "02-audience/segments.json"
    panel = (load(p).get("panel") if p.is_file() else None) or AUDIENCE
    j = config().get("judge") or {}
    if not _vi(c) and not (j.get("library") and j.get("context")):
        raise SystemExit("Micro Fish ships a Vietnamese persona library and feed only. For this campaign's language, set "
                         "`judge.library` (a persona .jsonl) and `judge.context` (a JSON list of {account, content} posts "
                         "of the platform) in ~/.babysit/marketing-video/config.json, or use another honest pre-test")
    return {**panel, "n": n, "cap": True, **({"library": j["library"]} if j.get("library") else {})}


def brief(cd: Path, sim: str, subjects: list[tuple[str, str]], size: str, platform: str, goal: str,
          lenses: list[dict]) -> dict:
    """The Micro Fish brief. Every subject is posted by the same account under a neutral label, so nothing but the
    video tells the arms apart; the product one-liner and the key never enter it."""
    c = load(cd / "campaign.json")
    vi = _vi(c)
    j = config().get("judge") or {}
    account = str(c.get("product") or c.get("slug") or "channel")
    out = {"sim": sim, "goal": goal, "platform": PLATFORMS[platform]["label"], "language": "vi" if vi else "en",
           "subjects": [{"id": lab, "label": f"Video {lab.upper()}", "account": account, "content": text}
                        for lab, text in subjects],
           "audience": audience(cd, c, SIZES[size] * len(subjects)), "rounds": 3, "feed_size": 4,
           "metric": "depth",  # few people finish a short video: decide on how far they watched, not on who finished
           "questions": questions(vi), "experts": lenses}
    if j.get("context"):
        out["context"] = load(Path(j["context"]).expanduser())
    return out


# ---------- running ----------

def _sim_id(cd: Path, tag: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", load(cd / "campaign.json").get("slug", "mvf").lower()).strip("-")
    return f"{slug[:28]}-{tag}"[:40]


def run(out: Path, keep: bool = False) -> dict:
    """Play out/brief.json: add its personas, the feed rounds, the interviews, the experts, the report. A run the
    provider stopped (rate limit) keeps what it recorded; the same call continues it."""
    mf = micro_fish()
    e = env()
    try:
        r = subprocess.run([str(mf), "run", str(out / "brief.json")], env=e, capture_output=True, text=True)
    finally:
        if not keep:
            subprocess.run([str(mf), "stop"], env=e, capture_output=True, text=True)
    (out / "progress.log").write_text(r.stderr, encoding="utf-8")
    try:
        res = json.loads((r.stdout.strip().splitlines() or ["{}"])[-1])
    except json.JSONDecodeError:
        res = {}
    res = {"state": "failed", **res, "at": today().isoformat()}
    if res.get("report") and Path(res["report"]).is_file():
        shutil.copy2(res["report"], out / "answer.md")
    if res["state"] not in ("completed", "completed_with_failures"):
        dump(out / "run.json", res)
        raise SystemExit(f"Micro Fish {res['state']}: {res.get('reason') or r.stderr.strip()[-600:]}\n"
                         f"What it recorded is kept. Continue with `mvf judge resume {out}`; read {out / 'progress.log'} "
                         f"and {home() / 'serve.log'} (failure_kind) first if it stops again")
    return res


def tally(sim: str) -> dict | None:
    r = subprocess.run([str(micro_fish()), "tally", sim], env=env(), capture_output=True, text=True)
    try:
        return json.loads(r.stdout) if r.returncode == 0 else None
    except json.JSONDecodeError:
        return None


# ---------- reading the tally ----------

def summarize(t: dict, key: dict) -> dict:
    """Per subject, the numbers the decision rests on, keyed back to script ids. Behaviour first: `watch_pct` is
    the average share of the video its personas watched (`avg_watch_s` of `duration_s`), `like_rate` and
    `comment_rate` who reacted, `stopped` who did not scroll past, `rank_in_feed` where its watch share puts it
    among the ordinary posts the same people saw. `finished` is kept for reading only: few people finish a short
    video, so nothing is decided on it."""
    subjects = t.get("subjects") or {}
    blocking = (t.get("experts") or {}).get("blocking") or []
    out = {}
    for lab, meta in key.items():
        x = subjects.get(lab) or {}
        iv = x.get("interview") or {}
        tags = (iv.get("objections") or {}).get("tags") or []
        pct = lambda k: (x.get(k) or {}).get("pct")
        watch, dur = pct("depth"), meta.get("duration")
        top2 = lambda k: ((iv.get(k) or {}).get("top2") or {}).get("pct")
        out[lab] = {"id": meta["id"], "kind": meta["kind"], "n": x.get("reached"), "assigned": x.get("assigned"),
                    "watch_pct": watch, "watch_ci95": (x.get("depth") or {}).get("ci95"), "duration_s": dur,
                    "avg_watch_s": round(watch * dur / 100, 1) if watch is not None and dur else None,
                    "stopped": pct("stopped"), "finished": pct("full"),
                    "rank_in_feed": f"{x.get('rank_in_feed')}/{x.get('of')}" if x.get("rank_in_feed") else None,
                    "like_rate": pct("like"), "share_rate": pct("share"), "comment_rate": pct("comment"),
                    "interviewed": iv.get("answered"), "clarity_top2": top2("clarity"), "intent_top2": top2("intent"),
                    "ad_feel": sum(n for tg, n in tags if "ad-feel" in tg or "quang-cao" in tg),
                    "no_use": sum(n for tg, n in tags if "no-use" in tg),
                    "unclear": [tg for tg, _ in tags if tg.startswith("unclear-")],
                    "objections": tags[:5], "comments": [c.get("text") for c in (x.get("comments") or [])[:5]],
                    "expert_blocking": [f"{b.get('severity')}: {b.get('issue')} → {b.get('fix')}" for b in blocking
                                        if b.get("subject") in (lab, None)]}
    return out


def verdict(t: dict, key: dict) -> dict:
    """The leader, whether the gap is clear, and whether it beat the control. Each arm of the audience met one
    video, so the ranking is what people did: how much of it they watched, then who liked it, who commented.
    A control (a script the team already rejected) that ties or wins means the society cannot tell good from
    rejected: then nothing wins."""
    s = summarize(t, key)
    cmp_ = t.get("comparison") or {}

    def score(lab):
        x = s[lab]
        return ((x["watch_pct"] or 0), (x["like_rate"] or 0), (x["comment_rate"] or 0))
    cands = [lab for lab in s if s[lab]["kind"] == "candidate"]
    controls = [lab for lab in s if s[lab]["kind"] in {"control", "bench"}]
    if not cands:
        return {"winner": None, "why": "no candidate in the tally", "summary": s}
    best = max(cands, key=score)
    beaten = [c for c in controls if score(c) >= score(best)]
    clear = bool(cmp_.get("clear")) and cmp_.get("leader") == best
    why = []
    if beaten:
        why.append(f"{', '.join(s[c]['id'] for c in beaten)} ranks at or above the best candidate: the society cannot tell")
    if not clear:
        why.append("the lead is not clear (overlapping intervals): read the retells and the expert findings before picking")
    return {"winner": None if beaten else s[best]["id"], "leader": s[best]["id"], "clear": clear, "beats_control": not beaten,
            "why": "; ".join(why) or "clear leader above every control", "summary": s}


def _watched(x: dict) -> str:
    """`watched 11.2/32s (35%)`: the average watch time over the length."""
    if x.get("avg_watch_s") is None:
        return f"watched {x.get('watch_pct')}%"
    return f"watched {x['avg_watch_s']}/{x['duration_s']:.0f}s ({x['watch_pct']}%)"


def evidence_line(v: dict, size: str) -> str:
    parts = []
    for x in sorted(v["summary"].values(), key=lambda x: -(x["watch_pct"] or 0)):
        parts.append(f"{x['id']} n={x['n']} · {_watched(x)} · like {x['like_rate']}% · comment "
                     f"{x['comment_rate']}% · stopped {x['stopped']}% · rank in feed "
                     f"{x['rank_in_feed']} · clarity top2 {x['clarity_top2']}% · intent top2 {x['intent_top2']}% · "
                     f"ad-feel {x['ad_feel']}")
    return f"Micro Fish {size}, one arm per video: " + " | ".join(parts) + f". {v['why']}"


def retell_sheet(cd: Path, t: dict, key: dict) -> str:
    """Each candidate's intended retell beside what its personas said after the feed closed: the agent judges the match."""
    out = ["# Retell check — does the audience say what the script means?\n",
           "For each candidate, compare `spine.takeaway` (what a stranger should retell) and `spine.use` with what the "
           "personas answered. Each line names what that persona did with the video in the feed (skip, some, full). "
           "Write the verdict, quoting the persona lines, into decision.json `retell` for scripts, or as a review.json "
           "issue for a cut that is retold wrong. To ask one of them why: `micro-fish.sh ask <sim> \"<name>\" q.txt`.\n"]
    for lab, meta in key.items():
        if meta["kind"] != "candidate":
            continue
        sid = meta["id"].split("/")[0]
        sp = cd / "05-scripts" / f"{sid}.json"
        spine = load(sp).get("spine", {}) if sp.is_file() else {}
        x = (t.get("subjects") or {}).get(lab) or {}
        iv = x.get("interview") or {}
        out.append(f"## {meta['id']} (subject {lab})\n- intended retell: {spine.get('takeaway', '—')}\n"
                   f"- intended use: {spine.get('use', '—')}")
        for q, title in (("retell", "retold"), ("use", "could use"), ("send", "would send")):
            rows = (iv.get(q) or {}).get("answers") or []
            if rows:
                out.append(f"\n{title}:")
                out += [f"  - [{a.get('attention')}] {a.get('who')}: {a.get('text')}" for a in rows]
        if x.get("comments"):
            out.append("\ncomments in the feed:")
            out += [f"  - {c.get('who')}: {c.get('text')}" for c in x["comments"]]
        out.append("")
    return "\n".join(out)


def judge_scripts(cd: Path, items: list[tuple[dict, str, str]], round_: str, size: str, platform: str,
                  manifests: dict, bench: list[dict], dry: bool) -> Path:
    """items: (script, hook, kind) with kind candidate|control. Writes 06-pretest/<round>/microfish/."""
    c = load(cd / "campaign.json")
    vi = _vi(c)
    out = cd / "06-pretest" / round_ / "microfish"
    out.mkdir(parents=True, exist_ok=True)
    entries = [(f"{s['id']}/{h}", k, s, h) for s, h, k in items] + [(f"bench:{b['id']}", "bench", b, None) for b in bench]
    if len(entries) > len(LABELS):
        raise SystemExit(f"{len(entries)} videos: Micro Fish compares at most {len(LABELS)} (e.g. 3 candidates + 1 control)")
    import random
    order = list(entries)
    random.Random(round_).shuffle(order)
    key = {LABELS[i]: {"id": e[0], "kind": e[1]} for i, e in enumerate(order)}
    sim = _sim_id(cd, round_)
    goal = ("Video nào trong số này đáng sản xuất: người lạ gặp nó trong feed có dừng lại, xem được bao lâu, có thích, "
            "bình luận, và kể lại đúng điều video nói không" if vi else
            "Which of these videos is worth producing: does a stranger who meets it in a feed stop, how long do they "
            "watch, do they like or comment, and can they retell what it said")

    def stim(lab: str, e: tuple) -> str:
        if e[1] == "bench":
            key[lab]["duration"] = max((float(str(x.get("s", "0")).rpartition("-")[2] or 0) for x in e[2].get("rows") or []),
                                       default=0.0)
            return bench_stimulus(e[2], platform, vi)
        text, tl = script_stimulus(e[2], e[3], manifests.get(e[2]["id"]), platform, vi)
        key[lab]["duration"] = tl["duration"]
        return text

    b = brief(cd, sim, [(lab, stim(lab, e)) for lab, e in zip(LABELS, order)], size, platform, goal, experts(c, platform, vi))
    dump(out / "brief.json", b)
    dump(out / "key.json", {"sim": sim, "platform": platform, "size": size, "stimuli": key})
    if dry:
        print(f"{out / 'brief.json'} ({len(order)} videos, {b['audience']['n']} personas, {SIZES[size]} per video) · "
              f"key {out / 'key.json'} (never shown to the society)")
        return out
    require_ready()
    print(f"Micro Fish {size} on {choice()['provider']}/{choice()['model']}: sim {sim}, {len(order)} videos, "
          f"{SIZES[size]} personas each…", flush=True)
    dump(out / "run.json", {**run(out), "sim": sim})
    return collect(cd, out)


def _winner_objections(v: dict) -> list[str]:
    """What the rewrite must answer before voicing: the experts' blocking findings and the audience's top tags on
    the script that won (or leads), not on the ones that lost."""
    lead = next((x for x in v["summary"].values() if x["id"] == (v["winner"] or v["leader"])), None)
    if not lead:
        return []
    return lead["expert_blocking"][:6] + [f"audience: {tag} ×{n}" for tag, n in lead["objections"][:3]]


def collect(cd: Path, out: Path) -> Path:
    """Read the tally back, write judge.json and the retell sheet, draft the decision."""
    k = load(out / "key.json")
    run_ = load(out / "run.json") if (out / "run.json").is_file() else {}
    t = tally(k["sim"])
    if not t:
        raise SystemExit(f"no tally for sim {k['sim']} (state {run_.get('state')}); read {out / 'progress.log'} and "
                         f"{home() / 'serve.log'} (failure_kind). A stopped run continues with `mvf judge resume {out}`")
    dump(out / "tally.json", t)
    v = verdict(t, k["stimuli"])
    dump(out / "judge.json", {"sim": k["sim"], "size": k["size"],
                              "complete": t.get("complete"), "warnings": t.get("warnings"), "caveat": t.get("caveat"),
                              **{x: v[x] for x in ("winner", "leader", "clear", "beats_control", "why")},
                              "stimuli": v["summary"]})
    (out / "retell.md").write_text(retell_sheet(cd, t, k["stimuli"]), encoding="utf-8")
    dec = out.parent / "decision.json"
    draft = {"method": "simulated-panel", "run": f"micro-fish:{k['sim']}",
             "winner": v["winner"], "evidence": evidence_line(v, k["size"]), "retell": "",
             "objections": _winner_objections(v),
             "changes": [], "untested": [], "decided_at": today().isoformat(),
             "_todo": "read retell.md and answer.md; write `retell` (what personas retold about the winner, quoted) and "
                      "fix `winner` if the numbers and the retells disagree; then delete _todo"}
    target = dec if not dec.is_file() else out / "decision.suggested.json"
    dump(target, draft)
    print(f"judge: leader {v['leader']} ({'clear' if v['clear'] else 'not clear'}; "
          f"{'beats' if v['beats_control'] else 'does NOT beat'} the control) → {out / 'judge.json'}")
    print(f"decision draft → {target}; read {out / 'retell.md'}"
          + (f" and {out / 'answer.md'}" if (out / 'answer.md').is_file() else ""))
    for w in t.get("warnings") or []:
        print(f"WARN tally: {w}")
    print("key: " + ", ".join(f"{lab} = {s['id']} ({s['kind']})" for lab, s in k["stimuli"].items()))
    return out


def resume(cd: Path, out: Path) -> Path:
    """Continue a stopped run from what Micro Fish recorded, then read it back."""
    k = load(out / "key.json")
    require_ready()
    dump(out / "run.json", {**run(out), "sim": k["sim"]})
    return _read_video(cd, out, k) if out.parent.name == "judge" else collect(cd, out)  # <proj>/judge/<round> is a cut


def judge_video(cd: Path, proj: Path, round_: str, size: str, platform: str, dry: bool) -> Path:
    """The cut, met in a feed the way the scripts were: do people stop for it, how long do they watch, retell it. One subject,
    the whole audience; experts look for what the edit broke."""
    meta = load(proj / "mvf.json")
    script = load(cd / meta["script"])
    c = load(cd / "campaign.json")
    vi = _vi(c)
    out = proj / "judge" / round_
    out.mkdir(parents=True, exist_ok=True)
    sim = _sim_id(cd, f"{script['id'][:12]}-{round_}")
    goal = (("Video đã dựng xong: đăng được chưa, hay sửa gì trước? Người lạ gặp nó trong feed có dừng lại, xem được bao "
             "lâu, có thích, bình luận và kể lại đúng không") if vi else
            ("The video is cut: post it, or fix what first? Does a stranger who meets it in a feed stop, how long do "
             "they watch, do they like or comment, and do they retell it right"))
    ex = experts(c, platform, vi)
    ex[0] = {"id": "editor", "lens": (f"Biên tập viên video ngắn trên {PLATFORMS[platform]['label']}: cắt cảnh, nhịp, chữ trên màn hình, khung hình 0"
                                      if vi else f"Short-video editor on {PLATFORMS[platform]['label']}: cuts, pacing, on-screen text, frame 0")}
    sid = f"{script['id']}/{meta.get('hook') or 'H1'}"
    key = {"a": {"id": sid, "kind": "candidate", "duration": load(proj / "timeline.json")["duration"]}}
    dump(out / "brief.json", brief(cd, sim, [("a", video_stimulus(proj, script, platform, vi))], size, platform, goal, ex))
    dump(out / "key.json", {"sim": sim, "platform": platform, "size": size, "stimuli": key})
    if dry:
        print(f"{out / 'brief.json'} ({SIZES[size]} personas)")
        return out
    require_ready()
    print(f"Micro Fish {size}: judging the cut {proj.name} (sim {sim}, {SIZES[size]} personas)…", flush=True)
    dump(out / "run.json", {**run(out), "sim": sim})
    return _read_video(cd, out, load(out / "key.json"))


def _read_video(cd: Path, out: Path, k: dict) -> Path:
    t = tally(k["sim"])
    if not t:
        raise SystemExit(f"no tally for sim {k['sim']}; see {out / 'progress.log'}")
    dump(out / "tally.json", t)
    s = summarize(t, k["stimuli"])["a"]
    dump(out / "judge.json", {"sim": k["sim"], "size": k["size"], "complete": t.get("complete"), "summary": s,
                              "feed_rank": t.get("feed_rank"), "experts": t.get("experts"), "warnings": t.get("warnings"),
                              "caveat": t.get("caveat"), "judged_at": today().isoformat()})
    (out / "retell.md").write_text(retell_sheet(cd, t, k["stimuli"]), encoding="utf-8")
    print(f"judge video: n={s['n']} · {_watched(s)} · like {s['like_rate']}% · comment {s['comment_rate']}% · "
          f"stopped {s['stopped']}% · finished {s['finished']}% · rank in feed "
          f"{s['rank_in_feed']} · clarity top2 {s['clarity_top2']}% · intent top2 {s['intent_top2']}% · ad-feel "
          f"{s['ad_feel']} · expert P0/P1 {len(s['expert_blocking'])} → {out / 'judge.json'}; read {out / 'retell.md'}"
          + (f" and {out / 'answer.md'}" if (out / "answer.md").is_file() else "")
          + ", fix once, then export without judging again")
    for w in t.get("warnings") or []:
        print(f"WARN tally: {w}")
    return out


def video_judged(proj: Path) -> dict | None:
    js = sorted((proj / "judge").glob("*/judge.json")) if (proj / "judge").is_dir() else []
    return load(js[-1]) if js else None


def check_video_judge(proj: Path, rep: Report) -> None:
    j = video_judged(proj)
    if j is None:
        rep.warn("JV1", proj.name, "the cut was not judged: `mvf judge video` (Micro Fish) before delivery")
        return
    s = j.get("summary") or {}
    if s.get("expert_blocking"):
        rep.info("JV1", proj.name, f"Micro Fish experts flagged {len(s['expert_blocking'])} P0/P1 issue(s): "
                 + "; ".join(s["expert_blocking"][:3]))
