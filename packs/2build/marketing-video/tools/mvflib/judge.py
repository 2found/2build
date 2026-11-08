"""The evaluator: Micro Fish, a simulated society on the user's local Soot, judges the candidate scripts and
the finished cut. A persona crowd reacts alone, then after reading each other; an expert panel reviews; an
analyst reports from deterministic statistics (the tally), which mvf reads back to draft the decision.

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

STIMULUS_CHARS = 3000  # Micro Fish refuses a longer stimulus
MESSAGE_BYTES = 16384  # Soot refuses a longer message (UTF-8 bytes: Vietnamese takes 1-3 per letter)
LABELS = "abcd"  # Micro Fish takes up to 4 stimuli
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
    deployment = home() / "sim/deployment.json"
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
    print(f"workspace   {home() / 'sim'} {'ok' if st['workspace'] else 'not created'}")
    cur = (soot_ai_config().get("tiers") or {}).get("flash") or soot_ai_config().get("model") or {}
    if not ch and cur.get("name"):
        print(f"hint        your Soot default is {cur.get('provider')}/{cur.get('name')}; Micro Fish makes many short "
              "calls, so a fast, cheap flash model fits best")
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
    if not (home() / "sim/deployment.json").is_file():
        args = [mf, "setup"] + (["--from", soot_forge] if soot_forge else [])
        r = subprocess.run(args, env=env(), text=True, capture_output=True)
        print(r.stdout.strip())
        if r.returncode:
            raise SystemExit(f"micro-fish setup failed:\n{r.stderr.strip()}\n(with a soot-forge checkout: "
                             "`mvf judge setup … --soot-forge <dir>`)")
    chk = subprocess.run([st["soot"], "check", "--config", str(home() / "sim/deployment.json"), "--secrets", "--show"],
                         env=env(), capture_output=True, text=True)
    first = next((ln for ln in chk.stdout.splitlines() if " model=" in ln), "")
    print((chk.stdout.splitlines() or [""])[0])
    if first:
        print(first.strip())
    if chk.returncode:
        raise SystemExit(f"soot check failed:\n{chk.stdout[-800:]}{chk.stderr[-800:]}")
    print(f"evaluator: Micro Fish on {provider}/{model} · config {ai_config_path()} · workspace {home() / 'sim'}")
    return ai_config_path()


def require_ready() -> None:
    st = status()
    if st["problems"]:
        print_status(st)
        raise SystemExit(NEEDS_CONTEXT)


# ---------- stimuli ----------

def _vi(c: dict) -> bool:
    return (c.get("languages") or ["en"])[0].lower().startswith("vi")


def _rows(beats: list[dict], tl_beats: list[dict]) -> list[tuple]:
    return [(t["start"], t["end"], (b.get("visual") or "").strip(), (b.get("onscreen") or "").strip().replace("\n", " / "),
             (b.get("vo") or "").strip()) for b, t in zip(beats, tl_beats)]


def stimulus(rows: list[tuple], duration: float, timing_kind: str, caption: str, platform: str, vi: bool,
             chars: int = STIMULUS_CHARS) -> str:
    """One video as a viewer meets it, second by second, inside Micro Fish's 3000-character stimulus.
    Pictures are shortened first; the voice is never cut, because understanding is what is tested."""
    lab = PLATFORMS[platform]["label"]
    head = (f"Video {lab}, {duration:.0f} giây ({'đo từ giọng thật' if timing_kind == 'measured' else 'thời lượng ước tính'}). "
            "Đọc như video đang chạy: HÌNH là thứ thấy, CHỮ là chữ trên màn hình, GIỌNG là lời nghe MỘT lần, đúng tốc độ nói."
            if vi else
            f"{lab} video, {duration:.0f} s ({'measured voice' if timing_kind == 'measured' else 'estimated length'}). Read it as "
            "it plays: PICTURE is what you see, TEXT is on screen, VOICE is heard ONCE at speaking pace.")
    P, T, V = ("HÌNH", "CHỮ", "GIỌNG") if vi else ("PICTURE", "TEXT", "VOICE")
    cap = (("Caption: " if vi else "Caption: ") + f"\"{caption.strip()}\"") if caption.strip() else ""
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
                     "stimulus. Shorten the on-screen text or split the test")


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
    pics = {}
    rv = proj / "review.json"
    notes = {}
    if rv.is_file():
        for i in load(rv).get("seen") or []:  # optional: {"at": s, "what": "..."} lines the reviewer wrote
            notes[float(i.get("at", -1))] = i.get("what", "")
    for b in script.get("beats") or []:
        pics[b["id"]] = (b.get("visual") or "").strip()
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

def _segments(cd: Path) -> str:
    p = cd / "02-audience/segments.json"
    if not p.is_file():
        return ""
    out = []
    for x in sorted(load(p).get("segments") or [], key=lambda x: x.get("priority", 9)):
        out.append(f"- {x.get('name')} ({x.get('ring', '?')}): {x.get('who')}. "
                   f"{'; '.join(x.get('pains') or [])}")
    return "\n".join(out)


def _questions(vi: bool, kind: str) -> list[str]:
    if vi:
        qs = ["attention: có dừng lướt trong 2 giây đầu không; watch_through: thật sự xem được bao nhiêu % trước khi lướt",
              "quote: mở đầu bằng 'Video nói là…' và kể lại điều video nói như kể cho bạn; không kể được thì nói thẳng",
              "clarity: có hiểu ngay khi nghe một lần không; chữ hay bước nào chưa hiểu thì gắn tag unclear-<từ>",
              "intent: tuần này có lưu, gửi, hoặc làm theo điều video nói không (không phải 'nghe hay')",
              "Video cho bạn dùng được gì trong đời mình (làm, quyết, biết)? Không gì thì gắn tag no-use",
              "Có thấy như quảng cáo không? Nếu có, gắn tag ad-feel; share: gửi cho ai, bằng câu gì",
              ]
        if kind == "scripts":
            qs.append("preferred: nếu chỉ lưu hoặc gửi MỘT video trong số này, chọn video nào")
        return qs
    qs = ["attention: do you stop scrolling in the first 2 s; watch_through: the % you would really watch",
          "quote: start with 'It says…' and retell what the video told you as you would to a friend; say so if you can't",
          "clarity: did you get it hearing it once; tag unclear-<word> for each word or step you did not follow",
          "intent: would you save, send or act on it this week (not 'sounds nice')",
          "What can you use in your own life after it (do, decide, know)? Tag no-use if nothing",
          "Does it feel like an ad? Tag ad-feel if so; share: who would you send it to, with what words"]
    if kind == "scripts":
        qs.append("preferred: if you could save or send only ONE of these videos, which one")
    return qs


def brief(cd: Path, sim: str, kind: str, stimuli: list[tuple[str, str]], size: str, platform: str,
          decision: str, experts: list[str]) -> str:
    c = load(cd / "campaign.json")
    vi = _vi(c)
    facts = cd / "01-product/facts.json"
    one = load(facts).get("one_liner", "") if facts.is_file() else ""
    qs = "\n".join(f"{i}. {q}" for i, q in enumerate(_questions(vi, kind), 1))
    stim = "\n\n".join(f"### {sid}\n{text}" for sid, text in stimuli)
    ids = ", ".join(f"`{s}`" for s, _ in stimuli)
    exp = "\n".join(f"- {e}" for e in experts)
    seg = _segments(cd)
    if vi:
        return f"""Chạy mô phỏng {size} ({SIZES[size]} persona), sim id `{sim}`, kind `video`.

**Quyết định:** {decision}

**Thị trường:** {', '.join(c.get('markets') or [])} · **Ngôn ngữ:** {', '.join(c.get('languages') or [])} · **Nền tảng:** {PLATFORMS[platform]['label']}
**Sản phẩm (một câu, người xem KHÔNG được thấy):** {one}

**Khán giả (từ nghiên cứu):**
{seg}
Trộn cả người chưa từng nghe về chủ đề, người thích thể loại này, fan cứng, và người lướt nhanh thiếu kiên nhẫn; có người sẽ nói không.

**Các phương án** — dùng đúng stimulus id {ids}, nhãn trung lập, không gợi ý phương án nào là ưu tiên. Mỗi persona gặp video như trong feed: chưa từng nghe về người làm, nghe mỗi câu một lần.

{stim}

**Câu hỏi cho từng persona** (ghi vào đúng trường):
{qs}

**Chuyên gia (độc lập, song song vòng 0):**
{exp}

**Báo cáo:** xếp hạng theo preference và watch_through, nói rõ khoảng cách có `clear` không; với từng phương án: giây người xem bỏ đi và câu làm họ bỏ; người xem kể lại được gì (trích quote); điều họ dùng được; câu nào nghe như quảng cáo hoặc khó tin; sửa cụ thể bằng lời thoại. Trích số từ tally, giữ nguyên warnings và caveat.
"""
    return f"""Run a {size} simulation ({SIZES[size]} personas), sim id `{sim}`, kind `video`.

**Decision:** {decision}

**Markets:** {', '.join(c.get('markets') or [])} · **Languages:** {', '.join(c.get('languages') or [])} · **Platform:** {PLATFORMS[platform]['label']}
**Product (one line, viewers do NOT see this):** {one}

**Audience (from research):**
{seg}
Mix people who have never heard of the topic, fans of the kind of thing, core fans and impatient scrollers; include people who will say no.

**Variants** — use exactly the stimulus ids {ids}, neutral labels, no hint of a favourite. Each persona meets each video in a feed: they know nothing about who made it and hear each line once.

{stim}

**Per-persona questions** (record each in its field):
{qs}

**Experts (independent, in parallel with round 0):**
{exp}

**Report:** rank by preference and watch_through and say whether the gap is `clear`; per variant: the second viewers leave and the line that lost them; what viewers retold (quote it); what they could use; which lines read as an ad or are hard to believe; concrete wording fixes. Cite tally numbers only, keep the warnings and the caveat.
"""


def experts(c: dict, platform: str, vi: bool) -> list[str]:
    lab, mk = PLATFORMS[platform]["label"], ", ".join(c.get("markets") or [])
    if vi:
        return [f"Giám đốc sáng tạo video ngắn trên {lab} ở {mk}: hook 2 giây đầu, nhịp, điểm rơi người xem",
                "Giáo viên giỏi giải thích: người lạ nghe MỘT lần có hiểu và kể lại được không, thuật ngữ nào chưa được giải nghĩa",
                "Người hoài nghi kiểm chứng: câu nào là quảng cáo, câu nào khó tin hoặc sai sự thật",
                "Người thuộc nhóm khán giả ưu tiên: video này cho tôi dùng được gì"]
    return [f"Short-video creative director on {lab} in {mk}: the 2-second hook, pacing, drop-off points",
            "Expert explainer and teacher: can a stranger who hears it ONCE retell it; which term is never explained",
            "Skeptical fact-checker: which line reads as an ad, which is hard to believe or untrue",
            "A member of the priority audience: what can I actually use from this"]


# ---------- running ----------

def _sim_id(cd: Path, tag: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", load(cd / "campaign.json").get("slug", "mvf").lower()).strip("-")
    return f"{slug[:28]}-{tag}"[:40]


def run(out: Path, text: str, conversation: str | None = None, keep: bool = False) -> dict:
    """Start the server, send one message and wait (10-30 min for a simulation), save the answer beside the brief."""
    mf = micro_fish()
    e = env()
    r = subprocess.run([str(mf), "start"], env=e, capture_output=True, text=True)
    if r.returncode:
        raise SystemExit(f"micro-fish start failed:\n{r.stdout}{r.stderr}")
    msg = out / (f"followup-{today().isoformat()}.txt" if conversation else "message.txt")
    msg.write_text(text, encoding="utf-8")
    try:
        r = subprocess.run([str(mf), "ask", str(msg), *([conversation] if conversation else [])], env=e,
                           capture_output=True, text=True)
    finally:
        if not keep:
            subprocess.run([str(mf), "stop"], env=e, capture_output=True, text=True)
    info = dict(re.findall(r"(\w+)=(\S*)", r.stdout))
    ans = Path(info.get("answer", ""))
    res = {"state": info.get("state", "failed"), "conversation": info.get("conversation") or conversation,
           "answer": None, "progress": info.get("progress"), "at": today().isoformat()}
    if ans.is_file():
        name = "answer.md" if not conversation else f"answer-{len(list(out.glob('answer*.md')))}.md"
        shutil.copy2(ans, out / name)
        res["answer"] = name
    if info.get("progress") and Path(info["progress"]).is_file():
        shutil.copy2(info["progress"], out / "progress.log")
    return res


def tally(sim: str) -> dict | None:
    r = subprocess.run([str(micro_fish()), "tally", sim], env=env(), capture_output=True, text=True)
    try:
        return json.loads(r.stdout) if r.returncode == 0 else None
    except json.JSONDecodeError:
        return None


# ---------- reading the tally ----------

def summarize(t: dict, key: dict) -> dict:
    """Per stimulus, from the last recorded round: the numbers the decision rests on, keyed back to script ids."""
    rounds = t.get("by_round") or {}
    last = rounds[max(rounds, key=int)] if rounds else {}
    pref = t.get("preference") or {}
    shares = pref.get("shares") or {}
    obj = t.get("objections") or {}
    blocking = (t.get("experts") or {}).get("blocking") or []
    out = {}
    for lab, meta in key.items():
        m = last.get(lab) or {}
        tags = (obj.get(lab) or {}).get("last") or (obj.get(lab) or {}).get("first") or []
        out[lab] = {"id": meta["id"], "kind": meta["kind"], "n": m.get("n"),
                    "attention": m.get("attention_mean"), "watch_through": m.get("watch_through_mean"),
                    "clarity_top2": m.get("clarity_top2"), "intent_top2": m.get("intent_top2"),
                    "share_rate": m.get("share_rate"), "sentiment": m.get("sentiment_mean"),
                    "preference_pct": (shares.get(lab) or {}).get("pct"),
                    "ad_feel": sum(n for tg, n in tags if "ad-feel" in tg or "quang-cao" in tg),
                    "no_use": sum(n for tg, n in tags if "no-use" in tg or "unclear-value" in tg),
                    "unclear": [tg for tg, _ in tags if tg.startswith("unclear-") and tg != "unclear-value"],
                    "objections": tags[:5],
                    "expert_blocking": [f"{b.get('severity')}: {b.get('issue')}" for b in blocking if b.get("stimulus") == lab]}
    return out


def verdict(t: dict, key: dict) -> dict:
    """The leader, whether the gap is clear, and whether it beat the control. A control (a script the team already
    rejected) that ties or wins means the panel cannot tell good from rejected: then nothing wins."""
    s = summarize(t, key)
    pref = t.get("preference") or {}

    def score(lab):
        x = s[lab]
        return ((x["preference_pct"] or 0), (x["watch_through"] or 0), (x["intent_top2"] or 0))
    cands = [lab for lab in s if s[lab]["kind"] == "candidate"]
    controls = [lab for lab in s if s[lab]["kind"] in {"control", "bench"}]
    if not cands:
        return {"winner": None, "why": "no candidate in the tally", "summary": s}
    best = max(cands, key=score)
    beaten = [c for c in controls if score(c) >= score(best)]
    clear = bool(pref.get("clear")) and pref.get("leader") == best
    why = []
    if beaten:
        why.append(f"{', '.join(s[c]['id'] for c in beaten)} ranks at or above the best candidate: the panel cannot tell")
    if not clear:
        why.append("the lead is not clear (overlapping intervals): read the quotes and the expert findings before picking")
    return {"winner": None if beaten else s[best]["id"], "leader": s[best]["id"], "clear": clear, "beats_control": not beaten,
            "why": "; ".join(why) or "clear leader above every control", "summary": s}


def evidence_line(v: dict, size: str) -> str:
    s = v["summary"]
    parts = []
    for lab, x in sorted(s.items(), key=lambda kv: -(kv[1]["preference_pct"] or 0)):
        parts.append(f"{x['id']} pref {x['preference_pct']}% · watch {x['watch_through']}% · attention {x['attention']} · "
                     f"clarity top2 {x['clarity_top2']}% · intent top2 {x['intent_top2']}% · ad-feel {x['ad_feel']}")
    n = next((x["n"] for x in s.values() if x.get("n")), None)
    return f"Micro Fish {size}, n={n}: " + " | ".join(parts) + f". {v['why']}"


def retell_sheet(cd: Path, t: dict, key: dict) -> str:
    """Each candidate's intended retell beside what the crowd actually said: the agent judges the match."""
    quotes = t.get("quotes") or {}
    out = ["# Retell check — does the crowd say what the script means?\n",
           "For each candidate, compare `spine.takeaway` (what a stranger should retell) and `spine.use` with the "
           "recorded quotes. Write the verdict, quoting the persona lines, into decision.json `retell` for scripts, or as a "
           "review.json issue for a cut that is retold wrong.\n"]
    for lab, meta in key.items():
        if meta["kind"] != "candidate":
            continue
        sid = meta["id"].split("/")[0]
        sp = cd / "05-scripts" / f"{sid}.json"
        spine = load(sp).get("spine", {}) if sp.is_file() else {}
        out.append(f"## {meta['id']} (stimulus {lab})\n- intended retell: {spine.get('takeaway', '—')}\n"
                   f"- intended use: {spine.get('use', '—')}")
        q = quotes.get(lab) or {}
        for side in ("positive", "negative"):
            for x in q.get(side) or []:
                txt = x.get("quote") if isinstance(x, dict) else x
                who = (x.get("persona") or x.get("id") or "") if isinstance(x, dict) else ""
                out.append(f"  - {side} {who}: {txt}")
        out.append("")
    return "\n".join(out)


def interview_text(vi: bool, sim: str, ids: list[str]) -> str:
    """Follow-up in the same conversation: the moderator interviews personas from their records."""
    names = ", ".join(f"`{i}`" for i in ids)
    if vi:
        return (f"Phỏng vấn trong sim `{sim}`: với mỗi phương án {names}, chọn 4 persona đã xem (1 người chưa từng nghe "
                "về chủ đề, 1 fan thể loại, 1 fan cứng, 1 người lướt nhanh). Mỗi người, đúng giọng của họ và chỉ từ những "
                "gì họ đã ghi: (1) kể lại video nói gì trong 1-2 câu, (2) từ hay bước nào chưa hiểu, (3) dùng được gì "
                "trong đời mình, (4) gửi cho ai. Nếu họ không kể lại được, nói thẳng như vậy, không bịa.")
    return (f"Interview in sim `{sim}`: for each variant {names}, pick 4 personas who saw it (one who never heard of the "
            "topic, one fan of the kind of thing, one core fan, one impatient scroller). For each, in their voice and "
            "only from what they recorded: (1) retell what the video said in 1-2 sentences, (2) which word or step "
            "they did not follow, (3) what they can use in their own life, (4) who they would send it to. If they "
            "could not retell it, say so; do not invent.")


def fit(build) -> str:
    """The brief at the longest stimuli that keep the whole message inside Soot's limit: pictures shorten first,
    the voice never (see stimulus)."""
    for chars in (STIMULUS_CHARS, 2500, 2000, 1600, 1200):
        text = build(chars)
        if len(text.encode("utf-8")) <= MESSAGE_BYTES:
            return text
    raise SystemExit(f"the brief is {len(text.encode('utf-8'))} bytes even with short pictures; Soot takes "
                     f"{MESSAGE_BYTES}. Compare fewer videos or shorten the segments in 02-audience")


def judge_scripts(cd: Path, items: list[tuple[dict, str, str]], round_: str, size: str, platform: str,
                  manifests: dict, bench: list[dict], dry: bool, interview: bool = True) -> Path:
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
    cands = [e[0] for e in entries if e[1] == "candidate"]
    decision = ((f"Chọn MỘT trong {len(cands)} kịch bản (và hook của nó) để sản xuất thành video duy nhất. Tiêu chí theo thứ tự: "
                 "người lạ nghe một lần kể lại đúng điều video nói; họ dùng được gì; họ lưu hay gửi; họ xem tới đâu. "
                 "Có một phương án đối chứng đã bị loại; nếu nó ngang hoặc hơn, hãy nói thẳng.")
                if vi else
                (f"Pick ONE of {len(cands)} scripts (with its hook) to produce as the only video. In order: a stranger who "
                 "hears it once retells what it said; what they can use; whether they save or send it; how far they "
                 "watch. One variant is a control the team already rejected; if it ties or wins, say so."))
    def stim(e: tuple, chars: int) -> str:
        if e[1] == "bench":
            return bench_stimulus(e[2], platform, vi, chars)
        return script_stimulus(e[2], e[3], manifests.get(e[2]["id"]), platform, vi, chars)[0]

    text = fit(lambda chars: brief(cd, sim, "scripts", [(lab, stim(e, chars)) for lab, e in zip(LABELS, order)], size,
                                   platform, decision, experts(c, platform, vi)))
    (out / "brief.txt").write_text(text, encoding="utf-8")
    dump(out / "key.json", {"sim": sim, "platform": platform, "size": size, "stimuli": key})
    if dry:
        print(f"{out / 'brief.txt'} ({len(text)} chars, {len(text.encode('utf-8'))} bytes) · key {out / 'key.json'} (never shown to the panel)")
        return out
    require_ready()
    print(f"Micro Fish {size} on {choice()['provider']}/{choice()['model']}: sim {sim}, {len(order)} videos; this takes "
          "10-30 minutes…", flush=True)
    res = run(out, text)
    dump(out / "run.json", {**res, "sim": sim})
    return collect(cd, out, interview)


def _winner_objections(v: dict) -> list[str]:
    """What the rewrite must answer before voicing: the experts' blocking findings and the crowd's top tags on the
    script that won (or leads), not on the ones that lost."""
    lead = next((x for x in v["summary"].values() if x["id"] == (v["winner"] or v["leader"])), None)
    if not lead:
        return []
    return lead["expert_blocking"][:6] + [f"crowd: {tag} ×{n}" for tag, n in lead["objections"][:3]]


def collect(cd: Path, out: Path, interview: bool = True) -> Path:
    """Read the tally back, write judge.json and the retell sheet, interview the leaders, draft the decision."""
    k = load(out / "key.json")
    run_ = load(out / "run.json") if (out / "run.json").is_file() else {}
    t = tally(k["sim"])
    if not t:
        raise SystemExit(f"no tally for sim {k['sim']} (state {run_.get('state')}); read {out / 'progress.log'} and "
                         f"~/.babysit/marketing-video/micro-fish/serve.log (failure_kind). A stopped run resumes with "
                         f"`mvf judge resume {out}`")
    dump(out / "tally.json", t)
    v = verdict(t, k["stimuli"])
    dump(out / "judge.json", {"sim": k["sim"], "conversation": run_.get("conversation"), "size": k["size"],
                              "complete": t.get("complete"), "warnings": t.get("warnings"), "caveat": t.get("caveat"),
                              **{x: v[x] for x in ("winner", "leader", "clear", "beats_control", "why")},
                              "stimuli": v["summary"]})
    (out / "retell.md").write_text(retell_sheet(cd, t, k["stimuli"]), encoding="utf-8")
    if interview and run_.get("conversation"):
        c = load(cd / "campaign.json")
        top = sorted((x for x in v["summary"].items() if x[1]["kind"] == "candidate"),
                     key=lambda kv: -(kv[1]["preference_pct"] or 0))[:2]
        res = run(out, interview_text(_vi(c), k["sim"], [lab for lab, _ in top]), run_["conversation"])
        if res.get("answer"):
            (out / "interview.md").write_text((out / res["answer"]).read_text(encoding="utf-8"), encoding="utf-8")
    dec = out.parent / "decision.json"
    draft = {"method": "simulated-panel", "run": f"micro-fish:{k['sim']}" + (f" conversation {run_['conversation']}"
                                                                            if run_.get("conversation") else ""),
             "winner": v["winner"], "evidence": evidence_line(v, k["size"]), "retell": "",
             "objections": _winner_objections(v),
             "changes": [], "untested": [], "decided_at": today().isoformat(),
             "_todo": "read answer.md, retell.md and interview.md; write `retell` (what personas retold about the winner, "
                      "quoted) and fix `winner` if the numbers and the quotes disagree; then delete _todo"}
    target = dec if not dec.is_file() else out / "decision.suggested.json"
    dump(target, draft)
    print(f"judge: leader {v['leader']} ({'clear' if v['clear'] else 'not clear'}; "
          f"{'beats' if v['beats_control'] else 'does NOT beat'} the control) → {out / 'judge.json'}")
    print(f"decision draft → {target}; read {out / 'answer.md'}, {out / 'retell.md'}"
          + (f", {out / 'interview.md'}" if (out / 'interview.md').is_file() else ""))
    for w in t.get("warnings") or []:
        print(f"WARN tally: {w}")
    return out


def resume(cd: Path, out: Path) -> Path:
    run_ = load(out / "run.json")
    if not run_.get("conversation"):
        raise SystemExit(f"{out / 'run.json'} has no conversation id to resume")
    c = load(cd / "campaign.json")
    res = run(out, "làm tiếp" if _vi(c) else "continue", run_["conversation"])
    dump(out / "run.json", {**run_, **{k: v for k, v in res.items() if v}})
    return collect(cd, out)


def judge_video(cd: Path, proj: Path, round_: str, size: str, platform: str, dry: bool) -> Path:
    """The cut, judged like the scripts were: does the picture carry the hook muted, where do people leave, can they
    retell it. One stimulus; experts look for what the edit broke."""
    meta = load(proj / "mvf.json")
    script = load(cd / meta["script"])
    c = load(cd / "campaign.json")
    vi = _vi(c)
    out = proj / "judge" / round_
    out.mkdir(parents=True, exist_ok=True)
    sim = _sim_id(cd, f"{script['id'][:12]}-{round_}")
    key = {"a": {"id": f"{script['id']}/{meta.get('hook') or 'H1'}", "kind": "candidate"}}
    decision = (("Video đã dựng xong: đăng được chưa, hay sửa gì trước? Người lạ nghe một lần có kể lại đúng không, bỏ đi ở "
                 "giây nào, hình có kể được câu chuyện khi tắt tiếng không.") if vi else
                ("The video is cut: post it, or fix what first? Can a stranger who hears it once retell it, where do people "
                 "leave, does the picture tell it with the sound off?"))
    ex = experts(c, platform, vi)
    ex[0] = (f"Biên tập viên video ngắn trên {PLATFORMS[platform]['label']}: cắt cảnh, nhịp, chữ trên màn hình, khung hình 0"
             if vi else f"Short-video editor on {PLATFORMS[platform]['label']}: cuts, pacing, on-screen text, frame 0")
    text = fit(lambda chars: brief(cd, sim, "video", [("a", video_stimulus(proj, script, platform, vi, chars))], size,
                                   platform, decision, ex))
    (out / "brief.txt").write_text(text, encoding="utf-8")
    dump(out / "key.json", {"sim": sim, "platform": platform, "size": size, "stimuli": key})
    if dry:
        print(f"{out / 'brief.txt'} ({len(text)} chars)")
        return out
    require_ready()
    print(f"Micro Fish {size}: judging the cut {proj.name} (sim {sim}); 10-30 minutes…", flush=True)
    res = run(out, text)
    dump(out / "run.json", {**res, "sim": sim})
    t = tally(sim)
    if not t:
        raise SystemExit(f"no tally for sim {sim} (state {res.get('state')}); see {out / 'progress.log'}")
    dump(out / "tally.json", t)
    s = summarize(t, key)["a"]
    dump(out / "judge.json", {"sim": sim, "conversation": res.get("conversation"), "size": size, "complete": t.get("complete"),
                              "summary": s, "experts": t.get("experts"), "warnings": t.get("warnings"),
                              "caveat": t.get("caveat"), "judged_at": today().isoformat()})
    (out / "retell.md").write_text(retell_sheet(cd, t, key), encoding="utf-8")
    print(f"judge video: watch {s['watch_through']}% · attention {s['attention']} · clarity top2 {s['clarity_top2']}% · "
          f"intent top2 {s['intent_top2']}% · ad-feel {s['ad_feel']} · expert P0/P1 {len(s['expert_blocking'])} → "
          f"{out / 'judge.json'}; read {out / 'answer.md'} and {out / 'retell.md'}, fix once, then export without judging again")
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
