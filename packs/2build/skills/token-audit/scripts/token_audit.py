#!/usr/bin/env python3
"""Measure where agent sessions spend tokens, and what every session pays up front.

  token_audit.py usage   [--days N | --since YYYY-MM-DD] [--harness auto|claude|codex|omp]
                         [--project SUBSTR] [--top N] [--json]
  token_audit.py context [--repo PATH] [--days N] [--json]

`usage` reads local session logs (Claude Code ~/.claude/projects, Codex
~/.codex/sessions, OMP ~/.omp/agent/sessions) and attributes weighted tokens to
main thread vs subagents, active skill (phase), model, token component, tool
result carry, and friction (errored calls, --help lookups, repeated commands,
repeated skill runs). `context` inventories the background context every
session loads: listed skills per root, their use in the window, and the
instruction/memory files with their size limits.

Weighted tokens are input-token equivalents: input 1, cache write 5m 1.25,
cache write 1h 2.0, cache read 0.1, output 5. Shares are comparable within one
report; they are not prices. Output contains counts, paths and command keys
only, never message text.
"""
import argparse
import collections
import datetime as dt
import glob
import json
import os
import re
import sys

HOME = os.path.expanduser('~')
W = {'input': 1.0, 'write_5m': 1.25, 'write_1h': 2.0, 'read': 0.1, 'output': 5.0}
SKILL_LIST_DESC_CAP = 1536          # Claude Code skillListingMaxDescChars default
MEMORY_LIMITS = (200, 24_400)        # auto-memory index: lines, bytes read at start
INSTRUCTION_WARN = 40_000            # Claude Code warns above ~40k chars per CLAUDE.md
BUILTIN_COMMANDS = {'compact', 'clear', 'model', 'effort', 'goal', 'reload-plugins', 'plugin', 'login',
                    'logout', 'config', 'resume', 'status', 'usage', 'usage-credits', 'rate-limit-options',
                    'fast', 'help', 'memory', 'mcp', 'permissions', 'hooks', 'agents', 'exit', 'cost'}


# ---------------------------------------------------------------- helpers
def load_jsonl(path):
    rows = []
    try:
        with open(path, errors='replace') as fh:
            for line in fh:
                try:
                    rows.append(json.loads(line))
                except ValueError:
                    pass
    except OSError:
        pass
    return rows


def weighted(t):
    return sum(W[k] * t.get(k, 0) for k in W)


def text_len(c):
    if isinstance(c, str):
        return len(c)
    if isinstance(c, list):
        return sum(text_len(x.get('text', '') if isinstance(x, dict) else x) for x in c)
    if isinstance(c, dict):
        return text_len(c.get('text') or c.get('output') or '')
    return 0


def command_key(cmd):
    """Stable, content-free key for a shell command: tool + subcommand."""
    cmd = re.sub(r'^(\s*cd [^&;]+(&&|;)\s*)+', '', cmd.strip())
    cmd = re.sub(r'^([A-Z_][A-Z0-9_]*=\S+\s+)+', '', cmd)
    toks = cmd.split()
    if not toks:
        return 'bash:?'
    head = os.path.basename(toks[0])[:24]
    if head == 'bbs' and len(toks) > 2:
        return 'bash:bbs ' + ' '.join(t for t in toks[1:3] if not t.startswith('-'))
    if head in ('git', 'pnpm', 'bun', 'npm', 'go', 'make', 'gh', 'orca', 'agent-browser',
                'docker', 'kubectl', 'gcloud', 'python3', 'cargo') and len(toks) > 1:
        return f'bash:{head} {toks[1]}'
    return 'bash:' + head


def skill_name(s):
    return re.sub(r'^[\w.-]+:', '', s or '?')


class Session:
    def __init__(self, harness, path, project):
        self.harness, self.path, self.project = harness, path, project
        # ('turn', tokens, model) | ('result', key, chars, errored) | ('skill', name) | ('compact',)
        self.events = []
        self.subagents = []       # (label, [(tokens, model), ...])
        self.calls = collections.Counter()
        self.commands = collections.Counter()
        self.last_ts = ''

    def turns(self):
        return [e for e in self.events if e[0] == 'turn']


# ---------------------------------------------------------------- Claude Code
def claude_tokens(u):
    cc = u.get('cache_creation') or {}
    w1 = cc.get('ephemeral_1h_input_tokens', 0) or 0
    w5 = cc.get('ephemeral_5m_input_tokens')
    if w5 is None:
        w5 = max(0, (u.get('cache_creation_input_tokens') or 0) - w1)
    return {'input': u.get('input_tokens') or 0, 'write_5m': w5, 'write_1h': w1,
            'read': u.get('cache_read_input_tokens') or 0, 'output': u.get('output_tokens') or 0}


def claude_walk(rows, s, collect_calls=True):
    seen, pending = set(), {}
    for d in rows:
        s.last_ts = max(s.last_ts, d.get('timestamp') or '')
        typ = d.get('type')
        if typ == 'system' and d.get('subtype') == 'compact_boundary':
            s.events.append(('compact',))
        m = d.get('message') or {}
        if typ == 'assistant':
            for c in m.get('content') or []:
                if not isinstance(c, dict) or c.get('type') != 'tool_use':
                    continue
                name, inp = c.get('name'), c.get('input') or {}
                if name == 'Skill':
                    s.events.append(('skill', skill_name(inp.get('skill'))))
                    key = 'skill:' + skill_name(inp.get('skill'))
                elif name == 'Bash':
                    cmd = inp.get('command', '')
                    key = command_key(cmd)
                    if collect_calls:
                        s.commands[cmd.strip()] += 1
                elif name == 'Read':
                    p = inp.get('file_path', '')
                    key = 'read:skill' if re.search(r'/skills/|/plugins/', p) else 'read:file'
                else:
                    key = name or '?'
                pending[c.get('id')] = key
                if collect_calls:
                    s.calls[key] += 1
            mid = m.get('id')
            if mid and mid not in seen and m.get('usage'):
                seen.add(mid)
                s.events.append(('turn', claude_tokens(m['usage']), m.get('model') or '?'))
        elif typ == 'user':
            cont = m.get('content')
            if isinstance(cont, list):
                for c in cont:
                    if isinstance(c, dict) and c.get('type') == 'tool_result':
                        s.events.append(('result', pending.get(c.get('tool_use_id'), '?'),
                                         text_len(c.get('content')), bool(c.get('is_error'))))
            elif isinstance(cont, str):
                for mm in re.finditer(r'<command-name>/?([^<]+)</command-name>', cont):
                    if mm.group(1) not in BUILTIN_COMMANDS:
                        s.events.append(('skill', skill_name(mm.group(1))))


def claude_sessions(since, project):
    root = os.path.join(HOME, '.claude', 'projects')
    for path in glob.glob(os.path.join(root, '*', '*.jsonl')):
        proj = os.path.basename(os.path.dirname(path))
        if project and project not in proj:
            continue
        if dt.datetime.fromtimestamp(os.path.getmtime(path)).strftime('%Y-%m-%d') < since:
            continue
        s = Session('claude', path, proj)
        claude_walk(load_jsonl(path), s)
        if s.last_ts[:10] < since:
            continue
        for sp in glob.glob(path[:-6] + '/subagents/agent-*.jsonl'):
            meta = {}
            try:
                meta = json.load(open(sp[:-6] + '.meta.json'))
            except (OSError, ValueError):
                pass
            sub = Session('claude', sp, proj)
            claude_walk(load_jsonl(sp), sub, collect_calls=False)
            label = meta.get('description') or meta.get('agentType') or '?'
            s.subagents.append((label, [(e[1], e[2]) for e in sub.turns()]))
        yield s


# ---------------------------------------------------------------- Codex
def codex_sessions(since, project):
    root = os.path.join(HOME, '.codex', 'sessions')
    for path in glob.glob(os.path.join(root, '*', '*', '*', '*.jsonl')):
        if dt.datetime.fromtimestamp(os.path.getmtime(path)).strftime('%Y-%m-%d') < since:
            continue
        rows = load_jsonl(path)
        cwd = next((r.get('payload', {}).get('cwd') for r in rows
                    if r.get('type') == 'session_meta'), '') or ''
        if project and project not in cwd.replace('/', '-'):
            continue
        s = Session('codex', path, cwd)
        model, pending = '?', {}
        for r in rows:
            s.last_ts = max(s.last_ts, r.get('timestamp') or '')
            p = r.get('payload') or {}
            pt = p.get('type')
            if r.get('type') == 'turn_context':
                model = p.get('model') or model
            elif r.get('type') == 'compacted' or pt == 'context_compacted':
                s.events.append(('compact',))
            elif pt == 'token_count':
                u = (p.get('info') or {}).get('last_token_usage')
                if u:
                    cached = u.get('cached_input_tokens') or 0
                    s.events.append(('turn', {'input': max(0, (u.get('input_tokens') or 0) - cached),
                                              'read': cached, 'output': u.get('output_tokens') or 0},
                                     model))
            elif pt in ('function_call', 'custom_tool_call', 'local_shell_call'):
                name, args = p.get('name') or pt, p.get('arguments') or p.get('input') or ''
                cmd = ''
                try:
                    a = json.loads(args) if isinstance(args, str) else args
                    cmd = a.get('cmd') or a.get('command') or ''
                    cmd = ' '.join(cmd) if isinstance(cmd, list) else cmd
                except (ValueError, AttributeError):
                    pass
                key = command_key(cmd) if cmd else name
                if cmd:
                    s.commands[cmd.strip()] += 1
                s.calls[key] += 1
                pending[p.get('call_id')] = key
            elif pt in ('function_call_output', 'custom_tool_call_output'):
                out = p.get('output')
                s.events.append(('result', pending.get(p.get('call_id'), '?'), text_len(out),
                                 bool(re.search(r'"exit_code":\s*[1-9]|Process exited with code [1-9]',
                                                out if isinstance(out, str) else ''))))
            elif pt == 'message' and p.get('role') == 'user':
                for mm in re.finditer(r'\$(?:[\w-]+:)?([a-z][\w-]+)', json.dumps(p.get('content'))):
                    s.events.append(('skill', mm.group(1)))
        if s.last_ts[:10] >= since:
            yield s


# ---------------------------------------------------------------- OMP
def omp_sessions(since, project):
    root = os.path.join(HOME, '.omp', 'agent', 'sessions')
    for path in glob.glob(os.path.join(root, '*', '*.jsonl')):
        proj = os.path.basename(os.path.dirname(path))
        if project and project not in proj:
            continue
        if dt.datetime.fromtimestamp(os.path.getmtime(path)).strftime('%Y-%m-%d') < since:
            continue
        s = Session('omp', path, proj)
        pending = {}
        for r in load_jsonl(path):
            s.last_ts = max(s.last_ts, r.get('timestamp') or '')
            t = r.get('type') or ''
            if 'compact' in t:
                s.events.append(('compact',))
            elif t == 'custom_message' and r.get('customType') == 'skill-prompt':
                mm = re.search(r'invoked the "([^"]+)" skill', r.get('content') or '')
                if mm:
                    s.events.append(('skill', skill_name(mm.group(1))))
            elif t == 'message':
                m = r.get('message') or {}
                if m.get('role') == 'assistant':
                    for c in m.get('content') or []:
                        if isinstance(c, dict) and c.get('type') == 'toolCall':
                            a = c.get('arguments') or {}
                            cmd = a.get('command', '') if isinstance(a, dict) else ''
                            key = command_key(cmd) if cmd else (c.get('name') or '?')
                            if cmd:
                                s.commands[cmd.strip()] += 1
                            s.calls[key] += 1
                            pending[c.get('id')] = key
                    u = m.get('usage')
                    if u and (u.get('totalTokens') or 0) > 0:
                        s.events.append(('turn', {'input': u.get('input') or 0,
                                                  'write_5m': u.get('cacheWrite') or 0,
                                                  'read': u.get('cacheRead') or 0,
                                                  'output': u.get('output') or 0},
                                         f"{m.get('provider', '?')}/{m.get('model', '?')}"))
                elif m.get('role') == 'toolResult':
                    s.events.append(('result', pending.get(m.get('toolCallId'), m.get('toolName') or '?'),
                                     text_len(m.get('content')), bool(m.get('isError'))))
        if s.last_ts[:10] >= since:
            yield s


SOURCES = {'claude': claude_sessions, 'codex': codex_sessions, 'omp': omp_sessions}


# ---------------------------------------------------------------- usage report
def median(xs):
    xs = sorted(xs)
    return xs[len(xs) // 2] if xs else 0


def usage(args):
    since = args.since or (dt.date.today() - dt.timedelta(days=args.days)).isoformat()
    names = list(SOURCES) if args.harness == 'auto' else [args.harness]
    sessions = [s for n in names for s in SOURCES[n](since, args.project)]
    tot = collections.Counter()
    comp = collections.Counter()
    phase = collections.Counter()
    model = collections.Counter()
    sub_label = collections.Counter()
    sub_n = collections.Counter()
    carry = collections.Counter()
    chars = collections.Counter()
    calls = collections.Counter()
    errors = collections.Counter()
    repeats = collections.Counter()
    skill_runs = collections.Counter()
    skill_repeat = collections.Counter()
    helps = 0
    base, peak, nturns, compactions = [], [], [], 0
    by_harness = collections.Counter()

    for s in sessions:
        turns = s.turns()
        if not turns:
            continue
        ctx = [t[1].get('input', 0) + t[1].get('write_5m', 0) + t[1].get('write_1h', 0)
               + t[1].get('read', 0) for t in turns]
        base.append(ctx[0])
        peak.append(max(ctx))
        nturns.append(len(turns))
        calls.update(s.calls)
        helps += sum(n for c, n in s.commands.items() if re.search(r'\s(--help|-h)\b|\shelp$', c))
        for c, n in s.commands.items():
            if n >= 3:
                repeats[command_key(c)] += n - 1
        runs = collections.Counter(e[1] for e in s.events if e[0] == 'skill')
        skill_runs.update(runs)
        for k, n in runs.items():
            if n > 1:
                skill_repeat[k] += n - 1
        # turns remaining until the next compaction, for result carry
        remaining, left = [0] * len(s.events), 0
        for i in range(len(s.events) - 1, -1, -1):
            kind = s.events[i][0]
            left = 0 if kind == 'compact' else left + (kind == 'turn')
            remaining[i] = left
        active = 'pre-skill'
        for i, e in enumerate(s.events):
            if e[0] == 'compact':
                compactions += 1
            elif e[0] == 'skill':
                active = e[1]
            elif e[0] == 'turn':
                w = weighted(e[1])
                tot['main'] += w
                by_harness[s.harness] += w
                phase[active] += w
                model[('main', e[2])] += w
                for k in W:
                    comp[('main', k)] += W[k] * e[1].get(k, 0)
            elif e[0] == 'result':
                chars[e[1]] += e[2]
                carry[e[1]] += e[2] * remaining[i]
                if e[3]:
                    errors[e[1]] += 1
        for label, sturns in s.subagents:
            sw = 0
            for tk, mdl in sturns:
                w = weighted(tk)
                sw += w
                model[('sub', mdl)] += w
                for k in W:
                    comp[('sub', k)] += W[k] * tk.get(k, 0)
            tot['sub'] += sw
            by_harness[s.harness] += sw
            key = re.sub(r'\s+', ' ', label.strip())[:60]
            sub_label[key] += sw
            sub_n[key] += 1

    all_w = tot['main'] + tot['sub'] or 1
    top = args.top
    rep = {
        'window': {'since': since, 'harnesses': names, 'project': args.project or '*'},
        'sessions': len(nturns),
        'weighted_total': round(all_w),
        'by_harness': {k: round(v / all_w, 3) for k, v in by_harness.most_common()},
        'main_share': round(tot['main'] / all_w, 3),
        'sub_share': round(tot['sub'] / all_w, 3),
        'components': {f'{a}:{k}': round(v / all_w, 3) for (a, k), v in comp.most_common() if v},
        'context': {'first_turn_median': median(base), 'peak_median': median(peak),
                    'turns_per_session_median': median(nturns), 'compactions': compactions},
        'by_phase': [(k, round(v / all_w, 3)) for k, v in phase.most_common(top)],
        'by_model': [(f'{a}:{m}', round(v / all_w, 3)) for (a, m), v in model.most_common(top)],
        'subagents': [(k, sub_n[k], round(v / all_w, 3), round(v / sub_n[k]))
                      for k, v in sub_label.most_common(top)],
        # carried result tokens are re-read as cache on every later turn (weight 0.1)
        'tool_results': [(k, calls[k], round(chars[k] / 4), round(v / 4 * W['read'] / all_w, 3))
                         for k, v in carry.most_common(top)],
        'friction': {'errored_calls': errors.most_common(top), 'help_lookups': helps,
                     'repeated_commands': repeats.most_common(top),
                     'repeated_skill_runs': skill_repeat.most_common(top)},
        'skill_runs': skill_runs.most_common(top),
    }
    if args.json:
        json.dump(rep, sys.stdout, indent=1)
        print()
        return
    pct = lambda x: f'{x:.0%}' if x >= 0.01 else f'{x:.1%}'
    print(f"window since {since} · harness {','.join(names)} · project {rep['window']['project']}")
    print(f"sessions {rep['sessions']} · weighted {all_w/1e6:.1f}M · main {pct(rep['main_share'])} "
          f"· subagents {pct(rep['sub_share'])} · by harness {rep['by_harness']}")
    c = rep['context']
    print(f"context: first turn {c['first_turn_median']:,} · peak {c['peak_median']:,} "
          f"· turns/session {c['turns_per_session_median']} · compactions {c['compactions']}")
    print('\ncomponents (share of weighted)')
    for k, v in rep['components'].items():
        print(f'  {k:16s} {pct(v)}')
    for title, rows in (('main thread by active skill', rep['by_phase']), ('by model', rep['by_model'])):
        print(f'\n{title}')
        for k, v in rows:
            print(f'  {k:40s} {pct(v)}')
    print('\nsubagents (label · runs · share · avg weighted/run)')
    for k, n, v, avg in rep['subagents']:
        print(f'  {k:60s} {n:4d} {pct(v):>5s} {avg/1000:7.0f}k')
    print('\ntool results (key · calls · ~tokens returned · share carried as cache reads)')
    for k, n, toks, v in rep['tool_results']:
        print(f'  {k:40s} {n:6d} {toks/1000:8.0f}k {pct(v):>6s}')
    f = rep['friction']
    print(f"\nfriction: --help lookups {f['help_lookups']}")
    for title in ('errored_calls', 'repeated_commands', 'repeated_skill_runs'):
        print(f'  {title}: ' + ', '.join(f'{k} {n}' for k, n in f[title]))
    print('\nskill runs: ' + ', '.join(f'{k} {n}' for k, n in rep['skill_runs']))


# ---------------------------------------------------------------- context report
def frontmatter(path):
    try:
        text = open(path, errors='replace').read()
    except OSError:
        return {}
    m = re.match(r'\A---\n(.*?)\n---', text, re.S)
    out, key = {}, None
    for line in (m.group(1).splitlines() if m else []):
        mm = re.match(r'^([\w-]+):\s*(.*)$', line)
        if mm:
            key, val = mm.group(1), mm.group(2).strip()
            out[key] = '' if val in ('>-', '>', '|', '|-') else val.strip('"\'')
        elif key and line.startswith(' '):
            out[key] = (out[key] + ' ' + line.strip()).strip()
    return out


def settings_files(repo):
    return [os.path.join(HOME, '.claude', 'settings.json'),
            os.path.join(repo, '.claude', 'settings.json'),
            os.path.join(repo, '.claude', 'settings.local.json')]


def read_json(path):
    try:
        return json.load(open(path))
    except (OSError, ValueError):
        return {}


def plugin_skill_roots():
    roots = []
    settings = read_json(os.path.join(HOME, '.claude', 'settings.json'))
    enabled = [k for k, v in (settings.get('enabledPlugins') or {}).items() if v]
    markets = read_json(os.path.join(HOME, '.claude', 'plugins', 'known_marketplaces.json'))
    installed = (read_json(os.path.join(HOME, '.claude', 'plugins', 'installed_plugins.json'))
                 .get('plugins') or {})
    for pid in enabled:
        plugin, _, market = pid.partition('@')
        src = (markets.get(market) or {}).get('source') or {}
        base = src.get('path') if src.get('source') == 'directory' else None
        if not base and installed.get(pid):
            base = installed[pid][-1].get('installPath')
        if base:
            for cand in ('skills', '.claude/skills'):
                if os.path.isdir(os.path.join(base, cand)):
                    roots.append((f'plugin {pid}', os.path.join(base, cand), plugin))
                    break
    for d in glob.glob(os.path.join(HOME, '.claude', 'skills', 'synced', '*')):
        if os.path.isdir(d):
            roots.append(('claude.ai synced', d, 'anthropic-skills'))
    return roots


def nested_skill_roots(repo, depth=3):
    out = []
    skip = {'node_modules', '.git', '.next', 'dist', 'build', 'worktrees', '.babysit'}
    for cur, dirs, _ in os.walk(repo):
        rel = os.path.relpath(cur, repo)
        dirs[:] = [d for d in dirs if d not in skip and (d == '.claude' or not d.startswith('.'))]
        if rel.count(os.sep) >= depth:
            dirs[:] = []
        if rel != '.' and os.path.isdir(os.path.join(cur, '.claude', 'skills')):
            out.append((f'nested {rel}/', os.path.join(cur, '.claude', 'skills'), rel.split(os.sep)[-1]))
    return out


def skill_usage(days):
    """Invocations per skill name across all harness logs in the window."""
    since = (dt.date.today() - dt.timedelta(days=days)).isoformat()
    used = collections.Counter()
    for s in (x for fn in SOURCES.values() for x in fn(since, '')):
        for e in s.events:
            if e[0] == 'skill':
                used[e[1]] += 1
    return used


def context(args):
    repo = os.path.abspath(args.repo)
    overrides = {}
    for f in settings_files(repo):
        overrides.update(read_json(f).get('skillOverrides') or {})
    roots = [('user ~/.claude/skills', os.path.join(HOME, '.claude', 'skills'), None),
             ('project .claude/skills', os.path.join(repo, '.claude', 'skills'), None),
             ('agents ~/.agents/skills (Codex/OMP)', os.path.join(HOME, '.agents', 'skills'), None),
             ('codex ~/.codex/skills', os.path.join(HOME, '.codex', 'skills'), None),
             ('project .agents/skills', os.path.join(repo, '.agents', 'skills'), None)]
    roots += plugin_skill_roots() + nested_skill_roots(repo)
    used = skill_usage(args.days)
    skills, stale, seen_dirs = [], [], set()
    for label, root, prefix in roots:
        if not os.path.isdir(root):
            continue
        for entry in sorted(os.listdir(root)):
            d = os.path.join(root, entry)
            if entry.startswith('.') or not os.path.isdir(d) or entry in ('synced', 'references', 'lib', 'data'):
                continue
            sk = os.path.join(d, 'SKILL.md')
            if not os.path.isfile(sk):
                stale.append((label, d))
                continue
            real = os.path.realpath(d)
            fm = frontmatter(sk)
            name = fm.get('name') or entry
            full = f'{prefix}:{name}' if prefix else name
            state = overrides.get(full, overrides.get(name, 'on'))
            manual = fm.get('disable-model-invocation', '').lower() == 'true'
            desc = (fm.get('description') or '') + ' ' + (fm.get('when_to_use') or '')
            listed = 0 if manual or state in ('off', 'user-invocable-only') else (
                len(full) + 4 if state == 'name-only' else len(full) + 4 + min(len(desc.strip()), SKILL_LIST_DESC_CAP))
            skills.append({'root': label, 'name': full, 'path': d, 'listed_chars': listed,
                           'manual_only': manual, 'override': state, 'uses': used.get(name, 0),
                           'duplicate_of': real if real in seen_dirs else None})
            seen_dirs.add(real)
    files = []
    slug = re.sub(r'[^A-Za-z0-9]', '-', repo)
    for label, p in (('global CLAUDE.md', os.path.join(HOME, '.claude', 'CLAUDE.md')),
                     ('project CLAUDE.md', os.path.join(repo, 'CLAUDE.md')),
                     ('project CLAUDE.local.md', os.path.join(repo, 'CLAUDE.local.md')),
                     ('project AGENTS.md (Codex/OMP)', os.path.join(repo, 'AGENTS.md')),
                     ('global AGENTS.md (Codex)', os.path.join(HOME, '.codex', 'AGENTS.md')),
                     ('auto-memory MEMORY.md', os.path.join(HOME, '.claude', 'projects', slug, 'memory', 'MEMORY.md'))):
        if os.path.isfile(p):
            text = open(p, errors='replace').read()
            same = os.path.realpath(p) in {os.path.realpath(f['path']) for f in files}
            files.append({'file': label, 'path': p, 'bytes': len(text.encode()), 'lines': text.count('\n') + 1,
                          'same_as_other': same})
    listed = sum(s['listed_chars'] for s in skills if not s['root'].startswith(('nested', 'agents', 'codex')))
    rep = {'repo': repo, 'usage_window_days': args.days,
           'claude_listing_chars': listed, 'claude_listing_tokens_est': round(listed / 3.5),
           'skills': skills, 'stale_dirs': stale, 'instruction_files': files,
           'memory_limits': {'lines': MEMORY_LIMITS[0], 'bytes': MEMORY_LIMITS[1]},
           'overrides': overrides}
    if args.json:
        json.dump(rep, sys.stdout, indent=1)
        print()
        return
    print(f'repo {repo} · skill use counted over {args.days} days (all harness logs)')
    print(f"Claude Code skill listing ≈ {listed:,} chars ≈ {rep['claude_listing_tokens_est']:,} tokens "
          f"(user + project + plugins; nested roots load only when work touches them)")
    by_root = collections.defaultdict(list)
    for s in skills:
        by_root[s['root']].append(s)
    for root, rows in by_root.items():
        print(f"\n{root}: {len(rows)} skills, {sum(r['listed_chars'] for r in rows):,} listed chars")
        for r in sorted(rows, key=lambda r: (-r['listed_chars'], r['name'])):
            flags = [f for f, on in (('manual-only', r['manual_only']), (f"override={r['override']}", r['override'] != 'on'),
                                     ('UNUSED', r['uses'] == 0 and r['listed_chars']),
                                     ('dup', r['duplicate_of'])) if on]
            print(f"  {r['name']:44s} {r['listed_chars']:5d}  uses {r['uses']:4d}  {' '.join(flags)}")
    if stale:
        print('\nskill-root dirs without SKILL.md (stale copies, scanned but never listed):')
        for label, d in stale:
            print(f'  {label}: {d}')
    print('\ninstruction / memory files (loaded every session)')
    for f in files:
        warn = ''
        if f['file'].startswith('auto-memory') and (f['lines'] > MEMORY_LIMITS[0] * 0.85 or f['bytes'] > MEMORY_LIMITS[1] * 0.85):
            warn = f'  NEAR LIMIT ({MEMORY_LIMITS[0]} lines / {MEMORY_LIMITS[1]:,} B are read)'
        elif f['bytes'] > INSTRUCTION_WARN:
            warn = '  LARGE'
        dup = '  (same file as another entry)' if f['same_as_other'] else ''
        print(f"  {f['file']:32s} {f['bytes']:7,} B {f['lines']:5d} lines{warn}{dup}")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = ap.add_subparsers(dest='cmd', required=True)
    u = sub.add_parser('usage')
    u.add_argument('--days', type=int, default=14)
    u.add_argument('--since')
    u.add_argument('--harness', choices=['auto', *SOURCES], default='auto')
    u.add_argument('--project', default='', help='substring of the project dir / cwd slug')
    u.add_argument('--top', type=int, default=15)
    u.add_argument('--json', action='store_true')
    c = sub.add_parser('context')
    c.add_argument('--repo', default=os.getcwd())
    c.add_argument('--days', type=int, default=30)
    c.add_argument('--json', action='store_true')
    args = ap.parse_args()
    (usage if args.cmd == 'usage' else context)(args)


if __name__ == '__main__':
    main()
