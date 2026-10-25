#!/usr/bin/env python3
"""Measure Foreman and worker token cost from Claude Code transcripts.

Reads ~/.claude/projects/**.jsonl (or --projects) and prints the metrics that
docs/foreman-cache-economics.md tracks, so a later run can be compared with the
recorded baseline:

    python3 scripts/foreman-cache-report.py --until 2026-10-25T12:00   # baseline window
    python3 scripts/foreman-cache-report.py --since 2026-10-26         # after the change
    python3 scripts/foreman-cache-report.py --since 2026-10-26 --json  # machine-readable

Cost unit is input-equivalent tokens at Anthropic prompt-cache prices: a cache
read counts 0.1, a 5-minute cache write 1.25, a 1-hour write 2, uncached input
1. Output tokens are reported separately.

Session selection (Claude Code only; other agents keep no such transcripts):
  worker   a main transcript in a project directory whose name contains
           "-babysit-worktrees-" (one directory per ticket worktree); its
           subagents/ transcripts are counted as that worker's subagents
  foreman  a main transcript with a user prompt invoking /bbs:foreman; each
           non-harness user prompt starts one wake, tagged by its source
A session is in the window when its first request is.
"""
import argparse
import json
import os
import statistics
import sys
from datetime import datetime, timezone

KEEP_WARM = "Prompt-cache keep-warm from bbs foreman watch"
HARNESS = ("<local-command-stdout>", "<command-name>/compact", "<command-name>/clear",
           "<command-name>/reload-plugins", "/compact")


def wake_source(text):
    """Who started a Foreman turn: the bbs watcher (skill prompt), a watcher
    keep-warm, a pre-bbs-task Orca orchestration message, or anyone else
    (the human, a background-task notification)."""
    t = text.lstrip()
    if KEEP_WARM in t:
        return "keep_warm"
    if t.startswith(("/bbs:foreman", "$bbs:foreman", "<command-message>bbs:foreman")):
        return "watcher"
    if t.startswith("You have ") and "orchestration message" in t[:60]:
        return "orchestration"
    return "other"


def invokes_foreman(text):
    t = text.lstrip()
    return t.startswith(("/bbs:foreman", "$bbs:foreman", "<command-message>bbs:foreman"))


def ts(s):
    return datetime.fromisoformat(s.replace("Z", "+00:00")).timestamp()


def parse_bound(s):
    if not s:
        return None
    d = datetime.fromisoformat(s)
    if d.tzinfo is None:
        d = d.replace(tzinfo=timezone.utc)
    return d.timestamp()


def user_text(e):
    c = (e.get("message") or {}).get("content")
    if isinstance(c, str):
        return c
    if isinstance(c, list):
        return "\n".join(b.get("text", "") for b in c if isinstance(b, dict) and b.get("type") == "text")
    return ""


def is_tool_result(e):
    c = (e.get("message") or {}).get("content")
    return isinstance(c, list) and any(isinstance(b, dict) and b.get("type") == "tool_result" for b in c)


def load(path):
    """Requests (deduplicated by request id), user prompts, compactions and
    the raw text of tool results, in time order."""
    reqs, order, prompts, compacts = {}, [], [], []
    results = []
    for line in open(path, errors="ignore"):
        try:
            e = json.loads(line)
        except ValueError:
            continue
        if not isinstance(e, dict) or "timestamp" not in e:
            continue
        if e.get("isCompactSummary"):
            compacts.append(ts(e["timestamp"]))
            continue
        if e.get("type") == "user":
            if is_tool_result(e):
                for b in e["message"]["content"]:
                    if isinstance(b, dict) and b.get("type") == "tool_result":
                        c = b.get("content")
                        results.append(c if isinstance(c, str) else json.dumps(c))
            elif not e.get("isMeta"):
                prompts.append((ts(e["timestamp"]), user_text(e)))
            continue
        if e.get("type") != "assistant":
            continue
        m = e.get("message") or {}
        u, rid = m.get("usage"), e.get("requestId") or m.get("id")
        if not u or not rid or rid in reqs:
            continue
        cc = u.get("cache_creation") or {}
        c5, c1 = cc.get("ephemeral_5m_input_tokens") or 0, cc.get("ephemeral_1h_input_tokens") or 0
        create = u.get("cache_creation_input_tokens") or 0
        if c5 + c1 == 0:
            c5 = create
        reqs[rid] = dict(t=ts(e["timestamp"]), read=u.get("cache_read_input_tokens") or 0, create=create,
                         c5=c5, c1=c1, inp=u.get("input_tokens") or 0, out=u.get("output_tokens") or 0)
        order.append(rid)
    rs = sorted((reqs[r] for r in order), key=lambda r: r["t"])
    return rs, prompts, compacts, results


def cost(r):
    return r["read"] * 0.1 + r["c5"] * 1.25 + r["c1"] * 2 + r["inp"]


def ctx(r):
    return r["read"] + r["create"] + r["inp"]


def cold(r):
    """A request that rewrote most of a non-trivial context."""
    return ctx(r) > 20000 and r["create"] > 0.5 * ctx(r)


def expiry_waste(rs, compacts):
    """Cost above a cache read for cold requests not explained by compaction."""
    waste = 0.0
    for prev, r in zip(rs, rs[1:]):
        if cold(r) and not any(prev["t"] <= c <= r["t"] + 5 for c in compacts):
            waste += cost(r) - ctx(r) * 0.1
    return waste


def totals(rs):
    t = dict(read=0, c5=0, c1=0, inp=0, out=0, cost=0.0)
    for r in rs:
        for k in ("read", "c5", "c1", "inp", "out"):
            t[k] += r[k]
        t["cost"] += cost(r)
    created = t["c5"] + t["c1"]
    t["hit_ratio"] = t["read"] / (t["read"] + created) if t["read"] + created else 0
    return t


def med(xs):
    return statistics.median(xs) if xs else 0


def mean(xs):
    return statistics.mean(xs) if xs else 0


def in_window(rs, since, until):
    return rs and (since is None or rs[0]["t"] >= since) and (until is None or rs[0]["t"] < until)


def main_transcripts(root):
    for d in sorted(os.listdir(root)):
        full = os.path.join(root, d)
        if not os.path.isdir(full):
            continue
        for f in sorted(os.listdir(full)):
            if f.endswith(".jsonl"):
                yield d, os.path.join(full, f)


def workers(root, since, until):
    sessions, worktrees = [], {}
    for d, path in main_transcripts(root):
        if "-babysit-worktrees-" not in d:
            continue
        rs, prompts, compacts, results = load(path)
        if not in_window(rs, since, until):
            continue
        subs = []
        sub_dir = os.path.join(path[:-len(".jsonl")], "subagents")
        if os.path.isdir(sub_dir):
            for f in sorted(os.listdir(sub_dir)):
                if f.endswith(".jsonl"):
                    srs = load(os.path.join(sub_dir, f))[0]
                    if srs:
                        subs.append(totals(srs))
        s = dict(worktree=d.split("-babysit-worktrees-", 1)[1], start=rs[0]["t"], turns=len(rs),
                 totals=totals(rs), base_ctx=ctx(rs[0]), ramp=sum(cost(r) for r in rs[:25]),
                 waste=expiry_waste(rs, compacts), compactions=len(compacts),
                 gate_refusals=sum(x.count("GATE=not-ready") for x in results),
                 subagents=subs)
        sessions.append(s)
        worktrees.setdefault(s["worktree"], []).append(s)
    return sessions, worktrees


def foremen(root, since, until):
    out = []
    for _, path in main_transcripts(root):
        rs, prompts, compacts, results = load(path)
        if not in_window(rs, since, until) or not any(invokes_foreman(p) for _, p in prompts):
            continue
        # A wake is one user prompt and the requests up to the next one.
        prompts = [(t, x) for t, x in prompts if not x.lstrip().startswith(HARNESS)]
        wakes, bounds = [], [t for t, _ in prompts] + [float("inf")]
        for i, (t, text) in enumerate(prompts):
            w = [r for r in rs if t <= r["t"] < bounds[i + 1]]
            if w:
                wakes.append(dict(turns=len(w), cost=sum(cost(r) for r in w), cold=cold(w[0]),
                                  source=wake_source(text)))
        out.append(dict(path=path, totals=totals(rs), wakes=wakes,
                        kind_creates=sum(x.count("KIND=") for x in results)))
    return out


def report(root, since, until):
    sessions, worktrees = workers(root, since, until)
    fm = foremen(root, since, until)
    wt_sessions = [len(v) for v in worktrees.values()]
    wt_cost = [sum(s["totals"]["cost"] for s in v) for v in worktrees.values()]
    wsum = totals([])
    for k in ("read", "c5", "c1", "inp", "out", "cost"):
        wsum[k] = sum(s["totals"][k] for s in sessions)
    created = wsum["c5"] + wsum["c1"]
    wsum["hit_ratio"] = wsum["read"] / (wsum["read"] + created) if wsum["read"] + created else 0
    subs = [x for s in sessions for x in s["subagents"]]
    sub_cost = sum(x["cost"] for x in subs)
    sub_create = sum(x["c5"] * 1.25 + x["c1"] * 2 for x in subs)
    sub_per_wt = [sum(x["cost"] for s in v for x in s["subagents"]) for v in worktrees.values()]
    turns = sum(s["turns"] for s in sessions)
    wakes = [w for f in fm for w in f["wakes"]]
    real = [w for w in wakes if w["source"] != "keep_warm"]
    kw = [w for w in wakes if w["source"] == "keep_warm"]
    by_source = {}
    for src in ("watcher", "orchestration", "other"):
        ws = [w for w in wakes if w["source"] == src]
        by_source[src] = dict(wakes=len(ws), cost=round(sum(w["cost"] for w in ws)),
                              turns_median=med([w["turns"] for w in ws]),
                              cost_median=round(med([w["cost"] for w in ws])),
                              cold=sum(1 for w in ws if w["cold"]))
    return {
        "window": {k: datetime.fromtimestamp(v, timezone.utc).isoformat() if v is not None else None
                   for k, v in (("since", since), ("until", until))},
        "workers": {
            "sessions": len(sessions),
            "worktrees": len(worktrees),
            "sessions_per_worktree_median": med(wt_sessions),
            "sessions_per_worktree_mean": round(mean(wt_sessions), 2),
            "worktrees_with_extra_session": sum(1 for n in wt_sessions if n > 1),
            "extra_sessions": sum(n - 1 for n in wt_sessions),
            "cost_total": round(wsum["cost"]),
            "cost_read": round(wsum["read"] * 0.1),
            "cost_create_1h": round(wsum["c1"] * 2),
            "cost_create_5m": round(wsum["c5"] * 1.25),
            "output_tokens": wsum["out"],
            "cache_hit_ratio": round(wsum["hit_ratio"], 4),
            "expiry_waste_share": round(sum(s["waste"] for s in sessions) / wsum["cost"], 4) if wsum["cost"] else 0,
            "turns": turns,
            "cost_per_turn": round(wsum["cost"] / turns) if turns else 0,
            "base_context_median": round(med([s["base_ctx"] for s in sessions])),
            "ramp_first_25_turns_median": round(med([s["ramp"] for s in sessions])),
            "cost_per_session_median": round(med([s["totals"]["cost"] for s in sessions])),
            "cost_per_worktree_median": round(med(wt_cost)),
            "gate_refusals": sum(s["gate_refusals"] for s in sessions),
            "sessions_with_gate_refusal": sum(1 for s in sessions if s["gate_refusals"]),
        },
        "subagents": {
            "count": len(subs),
            "cost_total": round(sub_cost),
            "creation_share": round(sub_create / sub_cost, 4) if sub_cost else 0,
            "cost_per_worktree_median": round(med(sub_per_wt)),
        },
        "foreman": {
            "sessions": len(fm),
            "cost_total": round(sum(f["totals"]["cost"] for f in fm)),
            "wakes": len(real),
            "turns_per_wake_median": med([w["turns"] for w in real]),
            "turns_per_wake_mean": round(mean([w["turns"] for w in real]), 1),
            "cost_per_wake_median": round(med([w["cost"] for w in real])),
            "cold_wake_share": round(sum(1 for w in real if w["cold"]) / len(real), 4) if real else 0,
            "cold_wake_cost": round(sum(w["cost"] for w in real if w["cold"])),
            "by_source": by_source,
            "keep_warms": len(kw),
            "keep_warm_cost": round(sum(w["cost"] for w in kw)),
            "keep_warm_cold": sum(1 for w in kw if w["cold"]),
        },
    }


def human(rep):
    m = lambda v: "%.1fM" % (v / 1e6)
    k = lambda v: "%.0fk" % (v / 1e3)
    w, s, f = rep["workers"], rep["subagents"], rep["foreman"]
    print("workers   %d sessions over %d worktrees; sessions/worktree median %s mean %s; "
          "%d worktrees needed an extra session (%d extra)" % (
              w["sessions"], w["worktrees"], w["sessions_per_worktree_median"], w["sessions_per_worktree_mean"],
              w["worktrees_with_extra_session"], w["extra_sessions"]))
    print("          cost %s (read %s, 1h writes %s, 5m writes %s); hit ratio %.1f%%; expiry waste %.1f%%" % (
        m(w["cost_total"]), m(w["cost_read"]), m(w["cost_create_1h"]), m(w["cost_create_5m"]),
        100 * w["cache_hit_ratio"], 100 * w["expiry_waste_share"]))
    print("          %d turns at %s/turn; base context %s; first-25-turn ramp %s; "
          "median session %s, median worktree %s" % (
              w["turns"], k(w["cost_per_turn"]), k(w["base_context_median"]), m(w["ramp_first_25_turns_median"]),
              m(w["cost_per_session_median"]), m(w["cost_per_worktree_median"])))
    print("          task done gate refusals %d (in %d sessions)" % (w["gate_refusals"], w["sessions_with_gate_refusal"]))
    print("subagents %d, cost %s, %.0f%% cache creation, median %s per worktree" % (
        s["count"], m(s["cost_total"]), 100 * s["creation_share"], m(s["cost_per_worktree_median"])))
    print("foreman   %d sessions, cost %s; %d wakes, turns/wake median %s mean %s, median %s/wake; "
          "cold wakes %.1f%% (%s)" % (
              f["sessions"], m(f["cost_total"]), f["wakes"], f["turns_per_wake_median"], f["turns_per_wake_mean"],
              m(f["cost_per_wake_median"]), 100 * f["cold_wake_share"], m(f["cold_wake_cost"])))
    for src, b in f["by_source"].items():
        print("          %-13s %4d wakes, %s total, turns median %s, median %s, %d cold" % (
            src, b["wakes"], m(b["cost"]), b["turns_median"], k(b["cost_median"]), b["cold"]))
    print("          keep-warms %d costing %s (%d landed cold)" % (f["keep_warms"], m(f["keep_warm_cost"]), f["keep_warm_cold"]))


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--projects", default=os.path.expanduser("~/.claude/projects"))
    ap.add_argument("--since", help="ISO date/time (UTC unless offset given); sessions starting at or after")
    ap.add_argument("--until", help="ISO date/time; sessions starting before")
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args()
    rep = report(a.projects, parse_bound(a.since), parse_bound(a.until))
    if a.json:
        json.dump(rep, sys.stdout, indent=2)
        print()
    else:
        human(rep)


if __name__ == "__main__":
    main()
