"""Procedural sound: one-shot SFX and mood beds, seeded, no model (numpy).

Local music models do not run usably on a laptop and a catalog track is the
user's to license, so a directed video still gets a score: pentatonic plucks, a
pad, a drone and drums placed on the measured timeline. Every function returns
float64 mono at SR; one-shots peak near 1.0 so a cue's dB is its peak level.
Ported from the storyshort kit."""
from __future__ import annotations

import numpy as np

SR = 48000
NOTE = {n: i for i, n in enumerate("C C# D D# E F F# G G# A A# B".split())}


def rng(seed=7):
    return np.random.default_rng(seed)


def hz(name: str) -> float:
    return 440.0 * 2 ** ((NOTE[name[:-1]] + 12 * (int(name[-1]) + 1) - 69) / 12)


def tt(d):
    return np.arange(int(d * SR)) / SR


def lp(x, cutoff, order=2):
    X = np.fft.rfft(x)
    f = np.fft.rfftfreq(len(x), 1 / SR)
    return np.fft.irfft(X / (1 + (f / cutoff) ** (2 * order)) ** 0.5, len(x))


def hp(x, cutoff, order=2):
    X = np.fft.rfft(x)
    f = np.fft.rfftfreq(len(x), 1 / SR) + 1e-9
    return np.fft.irfft(X / (1 + (cutoff / f) ** (2 * order)) ** 0.5, len(x))


def place(buf, sig, at, gain=1.0):
    i = int(round(at * SR))
    if i >= len(buf) or i + len(sig) <= 0:
        return
    s0 = max(0, -i)
    i = max(0, i)
    j = min(len(buf), i + len(sig) - s0)
    buf[i:j] += gain * sig[s0:s0 + j - i]


def env(n, a=0.01, r=0.3):
    t = np.arange(n) / SR
    d = n / SR
    return np.minimum(1, t / max(a, 1e-4)) * np.clip((d - t) / max(r, 1e-4), 0, 1)


def norm(x, peak=1.0):
    m = np.max(np.abs(x)) if len(x) else 0
    return x * (peak / m) if m > 0 else x


# ---------- tonal ----------

def pluck(f, dur=3.5, vel=0.6, dark=1.0, seed=7):
    """Modal plucked string (guzheng-ish)."""
    t = tt(dur)
    ph = 2 * np.pi * f * t
    x = np.zeros(len(t))
    for k in range(1, 9):
        inh = np.sqrt(1 + 0.0004 * k * k)
        tau = 2.4 / (1 + 0.45 * (k - 1) * dark) * (220 / f) ** 0.3
        x += (1 / k ** 1.15) * np.sin(k * inh * ph) * np.exp(-t / tau)
    click = rng(seed).normal(0, 1, len(t)) * np.exp(-t / 0.004) * 0.08
    return vel * (x + click) * (1 - np.exp(-t / 0.003))


def pad(freqs, dur, attack=2.0, release=2.5, bright=0.18):
    t = tt(dur)
    x = np.zeros(len(t))
    for i, f in enumerate(freqs):
        for det in (-1.2, 0.0, 1.2):
            ff = f * 2 ** (det / 1200)
            x += np.sin(2 * np.pi * ff * t + i) + bright * np.sin(4 * np.pi * ff * t) + 0.06 * np.sin(6 * np.pi * ff * t)
    x *= 0.85 + 0.15 * np.sin(2 * np.pi * 0.11 * t)
    e = np.minimum(1, t / attack) * np.minimum(1, np.maximum(0, (dur - t) / release))
    return x / (3 * max(1, len(freqs))) * e


def drone(f, dur, beat=0.09):
    t = tt(dur)
    x = np.sin(2 * np.pi * f * t) + 0.5 * np.sin(2 * np.pi * f * 1.5 * t + 0.3) + 0.35 * np.sin(2 * np.pi * (f * 2 + beat) * t)
    return x * env(len(t), 1.5, 1.5) / 1.85


def bell(f=880, dur=3.0, vel=1.0):
    t = tt(dur)
    x = sum(a * np.sin(2 * np.pi * f * m * t) * np.exp(-t / d) for m, a, d in
            [(1, 1.0, 1.9), (2.002, 0.45, 1.1), (2.997, 0.3, 0.8), (4.0, 0.18, 0.5), (0.5, 0.25, 2.6)])
    return vel * x * (1 - np.exp(-t / 0.004)) / 2.2


def gong(f=110, dur=4.0):
    t = tt(dur)
    x = sum(a * np.sin(2 * np.pi * f * m * t + m) * np.exp(-t / d) for m, a, d in
            [(1, 1.0, 2.2), (1.47, 0.6, 1.6), (2.09, 0.5, 1.1), (2.56, 0.35, 0.8), (3.3, 0.2, 0.5)])
    return x * (1 - np.exp(-t / 0.01)) / 2.2


# ---------- one-shots ----------

def taiko(pitch=58, seed=3):
    t = tt(2.0)
    f = pitch + 60 * np.exp(-t / 0.05)
    return np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-t / 0.45) + lp(rng(seed).normal(0, 1, len(t)), 900) * np.exp(-t / 0.05)


def hit(pitch=45, seed=41, body=0.5):
    """Trailer low hit: sub thump, short noise body, metallic tail (body ≤ 0.4 under a spoken word)."""
    t = tt(2.2)
    f = pitch + 140 * np.exp(-t / 0.03)
    sub = np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-t / 0.5)
    n = rng(seed).normal(0, 1, len(t))
    tail = sum(np.sin(2 * np.pi * fr * t) * np.exp(-t / 0.7) for fr in (311, 467, 709)) * 0.08
    return (1.3 * sub + body * lp(n, 1800) * np.exp(-t / 0.06) + tail * body / 0.7) * (1 - np.exp(-t / 0.001))


def boom(dur=3.0, seed=11):
    t = tt(dur)
    f = 30 + 90 * np.exp(-t / 0.12)
    sub = np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-t / 0.9)
    n = rng(seed).normal(0, 1, len(t))
    return (1.2 * sub + lp(n, 2500) * np.exp(-t / 0.18) + lp(n[::-1].copy(), 180) * np.exp(-t / 1.2) * 2.5
            + hp(n, 2000) * np.exp(-t / 0.03) * 0.6) * (1 - np.exp(-t / 0.002))


def thump(pitch=70, seed=67):
    t = tt(0.6)
    f = pitch + 50 * np.exp(-t / 0.02)
    return np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-t / 0.1) + 0.5 * lp(rng(seed).normal(0, 1, len(t)), 1400) * np.exp(-t / 0.025)


def stamp(seed=5):
    t = tt(0.5)
    n = rng(seed).normal(0, 1, len(t))
    return thump(90, seed) [: len(t)] + 0.8 * hp(lp(n, 5000), 1200) * np.exp(-t / 0.012)


def whoosh(dur=0.7, seed=13, rise=True, bright=3500):
    t = tt(dur)
    n = rng(seed).normal(0, 1, len(t))
    shape = np.sin(np.pi * np.clip(t / dur, 0, 1)) ** (1.5 if rise else 3)
    # a sweep: low-passed early, open late, the doppler of something passing the lens
    x = hp(lp(n, bright), 300)
    hi = hp(n, 2500) * np.clip(t / dur, 0, 1) ** 2
    return x * shape + 0.35 * hi * shape


def whip(seed=21):
    """A whip pan: a short, bright, fast-attack whoosh with a crack at its peak."""
    t = tt(0.32)
    n = rng(seed).normal(0, 1, len(t))
    shape = np.exp(-((t - 0.16) ** 2) / 0.0025)
    crack = hp(n, 3000) * np.exp(-((t - 0.17) ** 2) / 0.00004) * 0.6
    return hp(lp(n, 7000), 600) * shape + crack


def brush(dur=0.6, seed=31):
    """An ink brush stroke: soft papery noise that swells and drags."""
    t = tt(dur)
    n = rng(seed).normal(0, 1, len(t))
    shape = np.sin(np.pi * np.clip(t / dur, 0, 1)) ** 0.8 * (1 + 0.25 * np.sin(2 * np.pi * 9 * t))
    return lp(hp(n, 900), 4500) * shape


def air(dur=0.5, seed=33):
    return whoosh(dur, seed, rise=True, bright=1800) * 0.6


def tick(f=2400, seed=5):
    t = tt(0.06)
    return np.sin(2 * np.pi * f * t) * np.exp(-t / 0.006) + hp(rng(seed).normal(0, 1, len(t)), 3000) * np.exp(-t / 0.002) * 0.5


def ticks(dur=1.2, seed=5):
    """A counter running: ticks that start fast and slow with the count's ease."""
    out = np.zeros(int((dur + 0.1) * SR))
    t, k = 0.0, 0
    while t < dur:
        place(out, tick(2400 + 200 * (k % 2), seed + k), t, 0.8)
        t += 0.035 + 0.12 * (t / dur) ** 2
        k += 1
    return out


def rumble(dur=0.8, seed=17):
    t = tt(dur)
    return lp(rng(seed).normal(0, 1, len(t)), 160) * env(len(t), 0.02, dur * 0.7) * 3


def sword(seed=19):
    """A blade drawn: a hiss, then the ring."""
    t = tt(2.2)
    w = hp(rng(seed).normal(0, 1, len(t)), 2500) * np.exp(-((t - 0.08) ** 2) / 0.002)
    ring = sum(a * np.sin(2 * np.pi * f * t) * np.exp(-np.clip(t - 0.12, 0, None) / d) * (t > 0.12) for f, a, d in
               [(2160, 1.0, 1.0), (3011, 0.6, 0.7), (4390, 0.4, 0.4)])
    return w + 0.5 * ring


def heartbeat():
    t = tt(0.9)
    out = np.zeros(len(t))
    for at, g in ((0.0, 1.0), (0.22, 0.7)):
        u = np.clip(t - at, 0, None)
        out += g * np.sin(2 * np.pi * 55 * u) * np.exp(-u / 0.08) * (t >= at)
    return out


def riser(dur=2.0, seed=17):
    t = tt(dur)
    n = hp(rng(seed).normal(0, 1, len(t)), 800)
    sweep = np.sin(2 * np.pi * np.cumsum(200 + 1400 * (t / dur) ** 2) / SR)
    return (0.6 * n + 0.4 * sweep) * (t / dur) ** 2.2


# ---------- ambiences (length = dur) ----------

def rain(dur, seed=23):
    n = rng(seed).normal(0, 1, int(dur * SR))
    bed = hp(lp(n, 6000), 400)
    drops = np.zeros_like(bed)
    r = rng(seed + 1)
    if len(bed) > 1000:
        for i in r.choice(len(bed) - 800, min(len(bed) - 800, int(dur * 140)), replace=False):
            k = np.arange(400)
            drops[i:i + 400] += r.uniform(0.2, 1) * np.sin(2 * np.pi * r.uniform(2500, 6000) * k / SR) * np.exp(-k / 40)
    return norm(bed + 0.5 * drops) * env(len(bed), 0.8, 0.8)


def wind(dur, seed=29):
    t = tt(dur)
    n = rng(seed).normal(0, 1, len(t))
    gust = 0.55 + 0.45 * np.sin(2 * np.pi * 0.17 * t) * np.sin(2 * np.pi * 0.07 * t + 1)
    return norm(lp(n, 700) * gust + 0.3 * hp(lp(n, 2500), 900) * gust ** 2) * env(len(t), 0.8, 0.8)


def room(dur, seed=37):
    """Room tone: a quiet, dark noise floor so a cut to silence never sounds like a dropout."""
    t = tt(dur)
    return norm(lp(rng(seed).normal(0, 1, len(t)), 400)) * env(len(t), 0.5, 0.5)


def pop(seed=9):
    """A UI pop: a short pitched blip that drops, for an item or a badge landing."""
    t = tt(0.12)
    f = 900 * np.exp(-t / 0.03) + 380
    return np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-t / 0.035)


def click(seed=7):
    """A tap on glass: a dry transient with a tiny body."""
    t = tt(0.05)
    return hp(rng(seed).normal(0, 1, len(t)), 2000) * np.exp(-t / 0.0025) + 0.4 * np.sin(2 * np.pi * 1600 * t) * np.exp(-t / 0.008)


def ding(f=1320):
    """A notification ding: two bright partials, short."""
    t = tt(0.9)
    return (np.sin(2 * np.pi * f * t) + 0.35 * np.sin(2 * np.pi * f * 2.76 * t)) * np.exp(-t / 0.22)


ONE_SHOTS = {
    "pop": lambda s: pop(s), "click": lambda s: click(s), "ding": lambda s: ding(), "ping": lambda s: ding(1760),
    "hit": lambda s: hit(44 + 3 * (s % 3), seed=s, body=0.35), "boom": lambda s: boom(seed=s), "thump": lambda s: thump(seed=s),
    "stamp": lambda s: stamp(seed=s), "whoosh": lambda s: whoosh(0.7, seed=s), "whoosh-short": lambda s: whoosh(0.35, seed=s),
    "whip": lambda s: whip(seed=s), "brush": lambda s: brush(seed=s), "air": lambda s: air(seed=s), "swish": lambda s: whoosh(0.3, seed=s, bright=5000),
    "tick": lambda s: tick(seed=s), "rumble": lambda s: rumble(seed=s), "sword": lambda s: sword(seed=s), "heartbeat": lambda s: heartbeat(),
    "riser": lambda s: riser(seed=s), "bell": lambda s: bell(hz("G5"), 3.0), "gong": lambda s: gong(), "taiko": lambda s: taiko(seed=s),
    # fallbacks for kinds a recorded library usually supplies
    "typing": lambda s: ticks(0.7, s), "impact": lambda s: boom(1.8, s), "chime": lambda s: bell(hz("D6"), 2.0),
}
TIMED = {"ticks": ticks}            # one-shots that take the effect's duration
AMBIENT = {"rain": rain, "wind": wind, "room": room}


# ---------- mood beds ----------

SCALES = {"minor-penta": [0, 3, 5, 7, 10, 12, 15], "major-penta": [0, 2, 4, 7, 9, 12, 14]}
MOODS = {
    # bpm, root, scale, progression (semitones from root), pluck density, drum
    "wuxia":   {"bpm": 72, "root": "D3", "scale": "minor-penta", "prog": [[0, 7, 12], [-4, 3, 8], [-2, 5, 10], [-5, 2, 7]], "dens": 0.45, "drum": "sparse"},
    "tender":  {"bpm": 66, "root": "F3", "scale": "major-penta", "prog": [[0, 4, 7], [-3, 0, 4], [-7, -3, 0], [-5, -1, 2]], "dens": 0.35, "drum": None},
    "tension": {"bpm": 84, "root": "A2", "scale": "minor-penta", "prog": [[0, 7], [1, 8], [0, 7], [-1, 6]], "dens": 0.15, "drum": "pulse"},
    "epic":    {"bpm": 92, "root": "D3", "scale": "minor-penta", "prog": [[0, 7, 12], [-4, 3, 8], [-7, 0, 5], [-5, 2, 7]], "dens": 0.6, "drum": "drive"},
    "mystery": {"bpm": 60, "root": "E3", "scale": "minor-penta", "prog": [[0, 7], [-2, 5], [-4, 3], [-5, 2]], "dens": 0.25, "drum": None},
}


def mood_bed(mood: str, dur: float, seed: int = 7) -> np.ndarray:
    """A bed in one mood: a pad on a four-bar loop, a low drone, sparse plucks on the scale, drums by mood."""
    m = MOODS[mood]
    r = rng(seed)
    out = np.zeros(int((dur + 3) * SR))
    base = hz(m["root"])
    beat = 60 / m["bpm"]
    bar = 4 * beat
    sc = SCALES[m["scale"]]
    place(out, drone(base / 2, dur + 2.5, beat=0.11) * 0.55, 0)
    t, k, note = 0.0, 0, 2
    while t < dur:
        ch = m["prog"][k % len(m["prog"])]
        place(out, pad([base * 2 ** (s / 12) for s in ch], bar + 1.4, attack=0.7, release=1.2, bright=0.1) * 0.9, t)
        place(out, pluck(base * 2 ** ((ch[0] - 12) / 12), 2.6, 0.5, dark=1.4, seed=k), t)
        for q in range(8):
            if r.uniform() < m["dens"] * (0.9 if q % 2 == 0 else 0.5):
                note = int(np.clip(note + r.choice([-2, -1, 1, 1, 2]), 0, len(sc) - 1))
                f = base * 2 ** (sc[note] / 12) * 2
                place(out, pluck(f, 2.2, 0.32 + 0.1 * r.uniform(), seed=k * 8 + q), t + q * bar / 8)
        if m["drum"] == "sparse" and k % 2 == 0:
            place(out, taiko(52, seed=k) * 0.55, t)
        elif m["drum"] == "pulse":
            for q in range(4):
                place(out, heartbeat() * 0.6, t + q * beat)
        elif m["drum"] == "drive":
            for q in range(4):
                place(out, taiko(55 if q % 2 == 0 else 62, seed=k * 4 + q) * (0.8 if q == 0 else 0.45), t + q * beat)
        t, k = t + bar, k + 1
    x = out[: int(dur * SR)]
    return norm(x) * env(len(x), 0.6, min(1.5, dur / 3))


def reverb(x, secs=2.2, wet=0.25, seed=1):
    n = int(secs * SR)
    t = np.arange(n) / SR
    ir = rng(seed).normal(0, 1, n) * np.exp(-t / (secs / 5))
    ir[: int(0.012 * SR)] = 0
    ir /= np.sqrt(np.sum(ir ** 2))
    m = len(x) + n
    y = np.fft.irfft(np.fft.rfft(x, m) * np.fft.rfft(ir, m), m)[: len(x)]
    return (1 - wet) * x + wet * y
