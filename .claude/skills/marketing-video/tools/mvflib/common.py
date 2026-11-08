"""Shared helpers: campaign and project layout, JSON IO, findings, word counting."""
from __future__ import annotations

import datetime as dt
import json
import os
import re
import unicodedata
from dataclasses import dataclass, field
from pathlib import Path

LIB = Path(__file__).resolve().parent
TOOLS = LIB.parent
SKILL = TOOLS.parent
HYPERFRAMES = "0.8.153"  # pinned HyperFrames CLI; bump after `npx hyperframes upgrade --check` on a finished film

PLATFORMS = json.loads((LIB / "platforms.json").read_text(encoding="utf-8"))
# 2.5D visual style presets (stickman, ink, anime): defaults for art prompts, the look and the stick rig
STYLES = json.loads((LIB / "styles.json").read_text(encoding="utf-8"))["styles"]


def campaign_style(cd: Path) -> str | None:
    """The campaign's default style preset (campaign.json `style`): what a direction or art.json without its own
    `preset` takes. None or "none" is no preset (an older campaign that never named one keeps its look)."""
    p = Path(cd) / "campaign.json"
    try:
        st = json.loads(p.read_text(encoding="utf-8")).get("style") if p.is_file() else None
    except (OSError, ValueError):
        return None
    return st if st and st != "none" else None
# Every delivery target platforms.json describes; `upload_of` marks an encode variant of another network's
# target (x-api is X uploaded through the API), which a campaign never names on its own.
PLATFORM_IDS = tuple(k for k, v in PLATFORMS.items() if isinstance(v, dict) and "hard" in v)
CAMPAIGN_PLATFORMS = tuple(p for p in PLATFORM_IDS if not PLATFORMS[p].get("upload_of"))
# What a viewer can use after the video. A retold story, a feeling or a debate is not one: those are how a
# video holds and spreads, on top of a benefit. references/05-script.md#the-benefit
BENEFITS = {
    "lesson": "a principle the viewer applies in their own life: the situation, the usual wrong move, what to do instead",
    "decide": "help deciding: whether this is for them, what to pick, where to start",
    "answer": "the answer to a question they already have, settled with evidence",
    "match": "something that fits their taste, with the reason it fits",
    "how-to": "a skill or steps they can do today",
}
# Benefits whose script must turn the point onto the viewer in an `apply` beat.
APPLY_BENEFITS = {"lesson", "decide", "match", "how-to"}


def family(platform: str) -> str:
    """The network a delivery target belongs to: audiences, trends and copy are per network
    (youtube-long is YouTube, x-api is X)."""
    return (PLATFORMS.get(platform) or {}).get("family", platform)


def copy_kind(platform: str) -> str:
    """How a platform's text is written: `caption` (one caption under the video), `post` (X: a post and a
    thread) or `youtube` (title, description, tags)."""
    return (PLATFORMS.get(platform) or {}).get("copy", "caption")

# Stage folders, in flow order. `mvf status` walks them.
STAGES = [
    ("brief", "campaign.json"),
    ("material", "00-material/index.json"),
    ("product", "01-product/facts.json"),
    ("audience", "02-audience/segments.json"),
    ("trends", "03-trends/trends.json"),
    ("research", "03-research/research.json"),
    ("strategy", "04-strategy/angles.json"),
    ("scripts", "05-scripts"),
    ("pretest", "06-pretest"),
    ("art", "07-video/art/art.json"),
    ("video", "07-video"),
    ("delivery", "08-delivery"),
]

ID_RE = {
    "fact": re.compile(r"^F\d{1,3}$"),
    "segment": re.compile(r"^A\d{1,2}$"),
    "trend": re.compile(r"^T\d{1,3}$"),
    "angle": re.compile(r"^G\d{1,2}$"),
    "beat": re.compile(r"^B\d{2}$"),
    "hook": re.compile(r"^H\d$"),
    "script": re.compile(r"^(s\d{2}|ctl)(-[a-z0-9]+)*$"),  # ctl-…: a control, written to lose
}

# Per-project, machine-local state: extensions, reference voices, config. `mvf extensions init`
# creates it with a .gitignore of "*", so nothing in it is ever committed.
LOCAL = Path(".babysit/marketing-video")


def today() -> dt.date:
    return parse_date(os.environ.get("MVF_TODAY", "")) or dt.date.today()


def parse_date(s) -> dt.date | None:
    try:
        return dt.date.fromisoformat(str(s)[:10])
    except ValueError:
        return None


def load(path: Path):
    return json.loads(Path(path).read_text(encoding="utf-8"))


def dump(path: Path, data) -> None:
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    tmp.replace(path)


def find_campaign(start: Path) -> Path:
    """The campaign dir is the nearest ancestor (or self) holding campaign.json."""
    p = Path(start).resolve()
    for d in [p, *p.parents]:
        if (d / "campaign.json").is_file():
            return d
    raise SystemExit(f"no campaign.json at or above {start}; run `mvf init` first")


def project_root(start: Path | None = None) -> Path:
    """The repository the campaign lives in: the nearest ancestor with .babysit/ or .git, else the start dir."""
    p = Path(start or Path.cwd()).resolve()
    for d in [p, *p.parents]:
        if (d / ".babysit").is_dir() or (d / ".git").exists():
            return d
    return p


def local_dir(start: Path | None = None) -> Path:
    return project_root(start) / LOCAL


def user_dir() -> Path:
    """Per-OS-user state that no repo owns: the evaluator choice (Soot provider and model) and its Micro Fish
    workspace. $BABYSIT_HOME moves it (tests point it at a temp dir)."""
    return Path(os.environ.get("BABYSIT_HOME") or Path.home() / ".babysit") / "marketing-video"


def user_config() -> dict:
    p = user_dir() / "config.json"
    return load(p) if p.is_file() else {}


def save_user_config(patch: dict) -> Path:
    p = user_dir() / "config.json"
    dump(p, {**user_config(), **patch})
    return p


def config(start: Path | None = None) -> dict:
    """Machine config: ~/.babysit/marketing-video/config.json (this OS user), overridden key by key by the
    repo's .babysit/marketing-video/config.json: {"heavy": [cmd…], "voice_engine": "…", "judge": {…}}."""
    p = local_dir(start) / "config.json"
    return {**user_config(), **(load(p) if p.is_file() else {})}


def heavy_prefix(start: Path | None = None) -> list[str]:
    """A command prefix for model and render jobs (a queue or lock script), from MVF_HEAVY or config.heavy."""
    env = os.environ.get("MVF_HEAVY", "").strip()
    if env:
        return env.split()
    h = config(start).get("heavy") or []
    return [h] if isinstance(h, str) else list(h)


def words(text: str) -> list[str]:
    """Spoken words: letters/digits runs, NFC, punctuation dropped."""
    return re.findall(r"[\w'’%]+", unicodedata.normalize("NFC", text or ""))


def say_map(say: dict | None) -> dict:
    """How the brand's names are said (campaign brand.say), keys matched case-insensitively."""
    return dict(say or {})


def fold(text: str) -> list[str]:
    """Tone/case/punctuation-insensitive tokens for comparing script and ASR."""
    s = unicodedata.normalize("NFD", (text or "").lower().replace("đ", "d"))
    s = "".join(ch for ch in s if unicodedata.category(ch) != "Mn")
    return re.findall(r"\w+", s)


@dataclass
class Report:
    """FAIL blocks the next stage, WARN needs a decision, INFO is context."""
    items: list = field(default_factory=list)

    def add(self, level: str, code: str, where: str, msg: str) -> None:
        self.items.append({"level": level, "code": code, "where": where, "msg": msg})

    def fail(self, code, where, msg):
        self.add("FAIL", code, where, msg)

    def warn(self, code, where, msg):
        self.add("WARN", code, where, msg)

    def info(self, code, where, msg):
        self.add("INFO", code, where, msg)

    def extend(self, other: "Report") -> None:
        self.items.extend(other.items)

    @property
    def fails(self) -> int:
        return sum(i["level"] == "FAIL" for i in self.items)

    @property
    def warns(self) -> int:
        return sum(i["level"] == "WARN" for i in self.items)

    def print(self, title: str = "") -> None:
        if title:
            print(f"== {title}")
        for i in sorted(self.items, key=lambda i: ("FAIL", "WARN", "INFO").index(i["level"])):
            print(f"{i['level']:4} {i['code']:<4} {i['where']}: {i['msg']}")
        print(f"   {self.fails} FAIL, {self.warns} WARN")


def x_weight(text: str) -> int:
    """X's weighted post length (twitter-text v3 config): code points in the
    listed ranges weigh 1, everything else (CJK, emoji) 2; every URL counts 23."""
    ranges = ((0, 4351), (8192, 8205), (8208, 8223), (8242, 8247))
    url = re.compile(r"https?://\S+|\b[\w-]+\.(?:com|net|org|io|dev|ai|app|co|vn|me|gg|ly|uk)(?:/\S*)?", re.I)
    n = 23 * len(url.findall(text))
    s = unicodedata.normalize("NFC", url.sub("", text))
    i = 0
    while i < len(s):
        cp = ord(s[i])
        i += 1
        if _is_emoji(cp):
            n += 2  # one emoji, whatever modifiers, flags or ZWJ joins follow it
            while i < len(s):
                c = ord(s[i])
                if c in (0xFE0E, 0xFE0F) or 0x1F3FB <= c <= 0x1F3FF or (0x1F1E6 <= cp <= 0x1F1FF and 0x1F1E6 <= c <= 0x1F1FF):
                    i += 1
                elif c == 0x200D and i + 1 < len(s):
                    i += 2
                else:
                    break
            continue
        n += 1 if any(a <= cp <= b for a, b in ranges) else 2
    return n


def _is_emoji(cp: int) -> bool:
    return 0x1F000 <= cp <= 0x1FAFF or 0x2600 <= cp <= 0x27BF or 0x2B00 <= cp <= 0x2BFF


def hashtags(text: str) -> list[str]:
    return re.findall(r"(?<!\w)#[\w]+", text or "")
