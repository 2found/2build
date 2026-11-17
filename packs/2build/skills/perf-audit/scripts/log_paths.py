#!/usr/bin/env python3
"""Aggregate JSON request logs into a hot-path table by route template.

Input: log lines on stdin or in files. A line may be a JSON object, a JSON
object behind a prefix (`api-1 | {...}`, `docker logs -t` timestamps), or a
Docker json-file record whose `log` field holds the JSON. Plain-text access
logs are not parsed: the service must log JSON with a path and a duration.

Templates, in order:
  1. a logged route template (`routePath`, `route`) wins over the raw path;
  2. --routes FILE: known templates, one per line (`/novel/:slug/chapters/:n`;
     a `:name` segment matches any one value). Use it for static routes that
     share a shape with a parameter route (`/novel/search` beside
     `/novel/<slug>`) and for the audited feature's routes;
  3. otherwise the raw path, query string dropped: numeric, uuid, hex, email,
     long-token and mixed id segments collapse at once (`:n`, `:id`, `:hash`,
     `:email`, `:token`), and a segment position with more than --max-distinct
     values among paths of the same prefix and length becomes `:param`.
Only templates and numbers are printed, never raw paths, ids or other fields.
A short window can still leave a rare public literal (a slug): read the
templates before sharing them.

Per template, sorted by total time: count, share, p50/p95/p99 over requests
that did not fail (status < 400; p99 only with >= 100 of them, else max is the
tail), 4xx/429/5xx shares, share carrying a user id (counted, never printed:
identified traffic usually bypasses shared caches) and total seconds.
Summary: window, requests per full hour (partial edge hours dropped), slow
requests, burst seconds - seconds in which >= --burst-templates distinct
templates were all slower than --slow-ms (many unrelated routes slow in the
same second, health checks included, is a blocked event loop or CPU
starvation, not a slow dependency) - and templates whose 429 share suggests a
client retry loop. --only tables just the --routes templates and reports the
rest as one "outside" total.

Examples:
  docker logs --since 24h -t <container> 2>&1 | log_paths.py --where message=request
  log_paths.py --routes feature-routes.txt --only --json app.log
"""
import argparse
import json
import math
import re
import sys
from collections import Counter, defaultdict
from datetime import datetime, timezone

PATH_FIELDS = ["routePath", "route", "path", "url", "req.url", "request.path", "request.url",
               "httpRequest.requestUrl", "http.target", "uri"]
DURATION_FIELDS = ["durationMs", "duration_ms", "duration", "responseTime",
                   "response_time", "latency_ms", "latencyMs", "elapsed_ms", "took"]
STATUS_FIELDS = ["status", "statusCode", "status_code", "res.statusCode",
                 "httpRequest.status", "http.status_code"]
METHOD_FIELDS = ["method", "req.method", "request.method",
                 "httpRequest.requestMethod", "http.method"]
TIME_FIELDS = ["time", "timestamp", "ts", "@timestamp", "date", "_time"]
USER_FIELDS = ["userId", "user_id", "uid", "user.id", "sub"]

UNIT = {"ms": 1.0, "s": 1000.0, "us": 0.001, "µs": 0.001, "ns": 0.000001, "m": 60000.0, "h": 3600000.0}
EMAIL = re.compile(r"@|%40")
FIXED = [
    (re.compile(r"^\d+$"), ":n"),
    (re.compile(r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"), ":id"),
    (re.compile(r"^[0-9a-fA-F]{16,}$"), ":hash"),
    # 24+ chars of token alphabet (base64url, JWT), unless it is a kebab-case word slug
    (re.compile(r"^(?![a-z]+(?:-[a-z]+)+$)[A-Za-z0-9_.~+=-]{24,}$"), ":token"),
    (re.compile(r"^(?=.*\d)(?=.*[A-Za-z])[A-Za-z0-9_-]{8,}$"), ":id"),
]
PREFIX_TIME = re.compile(r"\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?")
GO_DURATION = re.compile(r"^(?:\d+(?:\.\d+)?(?:h|m(?!s)|s|ms|us|µs|ns))+$")
GO_PART = re.compile(r"(\d+(?:\.\d+)?)(h|ms|m|s|us|µs|ns)")


def get(obj, dotted):
    cur = obj
    for part in dotted.split("."):
        if isinstance(cur, dict) and part in cur:
            cur = cur[part]
        else:
            return None
    return cur


def first(obj, fields):
    for f in fields:
        v = get(obj, f)
        if v not in (None, ""):
            return v
    return None


def parse_line(line):
    """The JSON record on a line; a timestamp outside it (Docker) fills `_time`."""
    i = line.find("{")
    if i < 0:
        return None
    try:
        obj = json.loads(line[i:])
    except ValueError:
        return None
    if not isinstance(obj, dict):
        return None
    if isinstance(obj.get("log"), str) and "{" in obj["log"]:
        inner = parse_line(obj["log"])
        if inner is not None:
            if first(inner, TIME_FIELDS) is None and obj.get("time"):
                inner["_time"] = obj["time"]
            return inner
    if first(obj, TIME_FIELDS) is None:
        m = PREFIX_TIME.search(line[:i])
        if m:
            obj["_time"] = m.group(0)
    return obj


def to_ms(v, unit):
    if isinstance(v, bool):
        return None
    if isinstance(v, (int, float)):
        return float(v) * UNIT[unit]
    if isinstance(v, str):
        s = v.strip()
        try:
            return float(s) * UNIT[unit]
        except ValueError:
            pass
        if GO_DURATION.match(s):  # 1.5ms, 2s, 1m2.5s: the suffix wins over --duration-unit
            return sum(float(n) * UNIT[u] for n, u in GO_PART.findall(s))
    return None


def to_epoch(v):
    if isinstance(v, (int, float)) and not isinstance(v, bool):
        return v / 1000.0 if v > 1e11 else float(v)
    if isinstance(v, str):
        s = v.strip().replace("Z", "+00:00").replace(" ", "T", 1).replace(",", ".")
        s = re.sub(r"\.(\d+)", lambda m: "." + (m.group(1) + "000000")[:6], s, count=1)
        try:
            dt = datetime.fromisoformat(s)
        except ValueError:
            try:
                return to_epoch(float(s))
            except ValueError:
                return None
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt.timestamp()
    return None


def clean_path(p):
    p = re.sub(r"^[a-zA-Z][a-zA-Z0-9+.-]*://[^/]*", "", str(p))
    return p.split("?", 1)[0].split("#", 1)[0] or "/"


def split(p):
    return p.strip("/").split("/") if p.strip("/") else []


def fixed_segment(seg):
    if EMAIL.search(seg):
        return ":email"
    for rx, rep in FIXED:
        if rx.match(seg):
            return rep
    return seg


def known_matcher(known):
    # Most specific first: a static `/books/style-presets` beats `/books/:id`.
    parsed = sorted((split(t) for t in known), key=lambda ts: sum(s.startswith(":") for s in ts))

    def match(path):
        segs = split(path)
        for ts in parsed:
            if len(ts) == len(segs) and all(a.startswith(":") or a == b for a, b in zip(ts, segs)):
                return "/" + "/".join(ts)
        return None
    return match


def templates(paths, max_distinct):
    """Map each raw path to a template, collapsing high-cardinality positions."""
    segs = {p: [fixed_segment(s) for s in split(p)] for p in paths}
    tmpl = {p: [] for p in paths}
    depth = max((len(s) for s in segs.values()), default=0)
    for d in range(depth):
        # Cardinality per (prefix, path length): `/novel/search` stays literal
        # next to thirty `/novel/<slug>/chapter/<n>` paths.
        groups = defaultdict(set)
        for p, s in segs.items():
            if d < len(s):
                groups[(tuple(tmpl[p]), len(s))].add(s[d])
        for p, s in segs.items():
            if d < len(s):
                key = (tuple(tmpl[p]), len(s))
                tmpl[p].append(":param" if len(groups[key]) > max_distinct else s[d])
    return {p: "/" + "/".join(t) for p, t in tmpl.items()}


def pct(sorted_vals, q):
    if not sorted_vals:
        return None
    return sorted_vals[max(0, math.ceil(q * len(sorted_vals)) - 1)]


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("files", nargs="*", help="log files (default: stdin)")
    for f in ("path", "duration", "status", "method", "time", "user"):
        ap.add_argument(f"--{f}-field", help=f"dotted field name for the {f} (default: common names)")
    ap.add_argument("--duration-unit", choices=["ms", "s", "us", "ns"], default="ms",
                    help="unit of numeric durations (default ms)")
    ap.add_argument("--where", action="append", default=[], metavar="FIELD=VALUE",
                    help="keep only records where FIELD equals VALUE (repeatable)")
    ap.add_argument("--rule", action="append", default=[], metavar="REGEX=REPLACEMENT",
                    help="rewrite raw paths before templating (repeatable)")
    ap.add_argument("--routes", metavar="FILE", help="known route templates, one per line, matched first")
    ap.add_argument("--only", action="store_true", help="table only the --routes templates")
    ap.add_argument("--max-distinct", type=int, default=10)
    ap.add_argument("--slow-ms", type=float, default=1000.0)
    ap.add_argument("--burst-templates", type=int, default=5)
    ap.add_argument("--top", type=int, default=25)
    ap.add_argument("--json", action="store_true")
    a = ap.parse_args()
    if a.only and not a.routes:
        ap.error("--only needs --routes")

    rules = []
    for r in a.rule:
        rx, sep, rep = r.rpartition("=")
        if not sep or not rx:
            ap.error(f"--rule needs REGEX=REPLACEMENT: {r!r}")
        rules.append((re.compile(rx), rep))
    wheres = []
    for w in a.where:
        k, sep, v = w.partition("=")
        if not sep:
            ap.error(f"--where needs FIELD=VALUE: {w!r}")
        wheres.append((k, v))
    known = []
    if a.routes:
        with open(a.routes, encoding="utf-8") as fh:
            known = [clean_path(l.split("#", 1)[0].strip()) for l in fh if l.split("#", 1)[0].strip()]
    match_known = known_matcher(known)
    known_set = {"/" + "/".join(split(t)) for t in known}

    def pick(obj, flag, defaults):
        return get(obj, flag) if flag else first(obj, defaults)

    records = []  # (path, ms, status, method, epoch, identified)
    skipped = Counter()
    streams = [open(f, encoding="utf-8", errors="replace") for f in a.files] or [sys.stdin]
    for stream in streams:
        for line in stream:
            if not line.strip():
                continue
            obj = parse_line(line)
            if obj is None:
                skipped["not_json"] += 1
                continue
            if any(str(get(obj, k)) != v for k, v in wheres):
                skipped["filtered"] += 1
                continue
            path = pick(obj, a.path_field, PATH_FIELDS)
            dur = to_ms(pick(obj, a.duration_field, DURATION_FIELDS), a.duration_unit)
            if path is None or dur is None:
                skipped["no_path_or_duration"] += 1
                continue
            path = clean_path(path)
            for rx, rep in rules:
                path = rx.sub(rep, path)
            try:
                status = int(pick(obj, a.status_field, STATUS_FIELDS))
            except (TypeError, ValueError):
                status = None
            method = pick(obj, a.method_field, METHOD_FIELDS)
            records.append((path, dur, status, str(method).upper() if method else "",
                            to_epoch(pick(obj, a.time_field, TIME_FIELDS)),
                            pick(obj, a.user_field, USER_FIELDS) not in (None, "")))

    tmap = templates({r[0] for r in records if match_known(r[0]) is None}, a.max_distinct)

    def key_of(path, method):
        return (method + " " if method else "") + (match_known(path) or tmap[path])

    by = defaultdict(list)
    for path, dur, status, method, ts, ident in records:
        by[key_of(path, method)].append((dur, status, ident))

    total_ms = sum(r[1] for r in records) or 1.0
    rnd = lambda x: None if x is None else round(x, 1)
    rows = []
    for key, vals in by.items():
        n = len(vals)
        ok = sorted(v[0] for v in vals if v[1] is None or v[1] < 400)
        st = [v[1] for v in vals if v[1] is not None]
        share = lambda f: round(100.0 * sum(1 for s in st if f(s)) / len(st), 2) if st else None
        all_ms = [v[0] for v in vals]
        rows.append({
            "template": key, "count": n, "ok_count": len(ok),
            "share_pct": round(100.0 * n / len(records), 2),
            "p50_ms": rnd(pct(ok, 0.50)), "p95_ms": rnd(pct(ok, 0.95)),
            "p99_ms": rnd(pct(ok, 0.99)) if len(ok) >= 100 else None,
            "max_ms": rnd(max(all_ms)),
            "mean_ms": round(sum(all_ms) / n, 1),
            "total_s": round(sum(all_ms) / 1000.0, 1),
            "time_share_pct": round(100.0 * sum(all_ms) / total_ms, 2),
            "4xx_pct": share(lambda s: 400 <= s < 500),
            "429_pct": share(lambda s: s == 429),
            "5xx_pct": share(lambda s: s >= 500),
            "identified_pct": round(100.0 * sum(1 for v in vals if v[2]) / n, 1),
        })
    rows.sort(key=lambda r: r["total_s"], reverse=True)

    in_feature = lambda r: r["template"].split(" ", 1)[-1] in known_set
    shown = [r for r in rows if in_feature(r)] if a.only else rows
    outside = [r for r in rows if not in_feature(r)] if a.only else []

    stamps = sorted(r[4] for r in records if r[4] is not None)
    hours = Counter(int(t // 3600) for t in stamps)
    edge = {int(stamps[0] // 3600), int(stamps[-1] // 3600)} if stamps else set()
    full_hours = sorted(c for h, c in hours.items() if h not in edge)
    slow_by_sec = defaultdict(set)
    for path, dur, status, method, ts, ident in records:
        if ts is not None and dur >= a.slow_ms:
            slow_by_sec[int(ts)].add(key_of(path, method))
    bursts = sorted(((s, t) for s, t in slow_by_sec.items() if len(t) >= a.burst_templates),
                    key=lambda x: (-len(x[1]), x[0]))
    iso = lambda t: datetime.fromtimestamp(t, timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    span_h = (stamps[-1] - stamps[0]) / 3600.0 if len(stamps) > 1 else 0.0
    median = pct(sorted(r[1] for r in records), 0.5)
    warnings = []
    if median is not None and (median < 0.5 or median > 60000):
        warnings.append(f"median duration {median:g} ms looks implausible: check --duration-unit / --duration-field")
    if records and not stamps:
        warnings.append("no timestamps: traffic level and burst seconds unavailable (--time-field, or docker logs -t)")
    summary = {
        "records": len(records), "skipped_lines": dict(skipped), "templates": len(rows),
        "window_utc": [iso(stamps[0]), iso(stamps[-1])] if stamps else None,
        "requests_per_hour": ({"overall": round(len(stamps) / span_h, 1) if span_h else None,
                               "full_hours": len(full_hours),
                               "min": full_hours[0] if full_hours else None,
                               "median": full_hours[len(full_hours) // 2] if full_hours else None,
                               "max": full_hours[-1] if full_hours else None} if stamps else None),
        "slow_ms": a.slow_ms,
        "slow_requests": sum(1 for r in records if r[1] >= a.slow_ms),
        "burst_seconds": len(bursts),
        "top_bursts": [{"second_utc": iso(s), "slow_templates": len(t),
                        "includes_health": any("health" in x for x in t)} for s, t in bursts[:5]],
        "retry_signal": [r["template"] for r in rows if (r["429_pct"] or 0) >= 5.0],
        "outside_feature": ({"templates": len(outside), "count": sum(r["count"] for r in outside),
                             "total_s": round(sum(r["total_s"] for r in outside), 1)} if a.only else None),
        "warnings": warnings,
    }

    if a.json:
        json.dump({"summary": summary, "paths": shown}, sys.stdout, indent=1, ensure_ascii=False)
        print()
        return 0 if records else 1
    if not records:
        print(f"no records with a path and a duration (skipped: {dict(skipped)}); "
              "set --path-field / --duration-field, or check the log is JSON")
        return 1
    s = summary
    for w in warnings:
        print("WARNING: " + w)
    sk = ", ".join(f"{k} {v}" for k, v in skipped.items()) or "0"
    print(f"records {s['records']} · skipped {sk} · templates {s['templates']}")
    if s["window_utc"]:
        rph = s["requests_per_hour"]
        line = f"window {s['window_utc'][0]} → {s['window_utc'][1]} · requests/hour overall {rph['overall']}"
        if rph["full_hours"]:
            line += f" · {rph['full_hours']} full hours: min {rph['min']} median {rph['median']} max {rph['max']}"
        print(line)
    print(f"slow (≥ {a.slow_ms:g} ms): {s['slow_requests']} · burst seconds "
          f"(≥ {a.burst_templates} templates slow at once): {s['burst_seconds']}")
    for b in s["top_bursts"]:
        print(f"  {b['second_utc']}  {b['slow_templates']} templates" + ("  incl. health" if b["includes_health"] else ""))
    if s["retry_signal"]:
        print("429 ≥ 5 % (possible client retry loop): " + ", ".join(s["retry_signal"]))
    print("percentiles over status < 400; max and total over all requests; user % = share carrying a user id")
    print()
    fmt = lambda v: "—" if v is None else f"{v:g}"
    print("| template | count | share % | p50 ms | p95 ms | p99 ms | max ms | 4xx % | 5xx % | user % | total s | time % |")
    print("|---|---|---|---|---|---|---|---|---|---|---|---|")
    for r in shown[:a.top]:
        print(f"| `{r['template']}` | {r['count']} | {fmt(r['share_pct'])} | {fmt(r['p50_ms'])} | {fmt(r['p95_ms'])} "
              f"| {fmt(r['p99_ms'])} | {fmt(r['max_ms'])} | {fmt(r['4xx_pct'])} | {fmt(r['5xx_pct'])} "
              f"| {fmt(r['identified_pct'])} | {fmt(r['total_s'])} | {fmt(r['time_share_pct'])} |")
    rest = shown[a.top:]
    if rest:
        print(f"| (other {len(rest)} templates) | {sum(r['count'] for r in rest)} | | | | | | | | | "
              f"{round(sum(r['total_s'] for r in rest), 1):g} | |")
    if s["outside_feature"]:
        o = s["outside_feature"]
        print(f"| (outside the feature: {o['templates']} templates) | {o['count']} | | | | | | | | | {o['total_s']:g} | |")
    return 0


if __name__ == "__main__":
    sys.exit(main())
