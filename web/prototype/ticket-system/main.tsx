// Disposable design prototype (bs-w5p3c6mq); not part of production navigation.
// Data: the real LoHi milestone bs-nuae030p, read from ~/.babysit on 2026-11-02.
// "Today" is simulated as 2026-12-20, so a few Phase 1 tickets are done and
// their checks have come due — every such row is sample state, not the store.
import React, { useMemo, useState, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { Flag, Layers, Inbox, LineChart, CircleAlert, CalendarClock } from 'lucide-react';
import { TopBar } from '../../src/components/TopBar';
import { DenseRow } from '../../src/components/DenseRow';
import { Tag } from '../../src/components/Tag';
import { StatusArc } from '../../src/components/StatusArc';
import { SectionHeader } from '../../src/components/SectionHeader';
import { Button } from '../../src/components/Button';
import { Modal } from '../../src/components/Modal';
import { Field, inputStyle } from '../../src/components/Field';
import { EmptyState } from '../../src/components/EmptyState';
import type { TicketStatus } from '../../src/lib/data';
import '../../src/styles.css';
import raw from './fixture.json';

type Role = 'swe' | 'human' | 'marketing' | 'ops';
interface T {
  id: string; title: string; kind: 'milestone' | 'phase' | 'work'; parent: string | null;
  children: string[]; status: TicketStatus; role: Role; product: string; size?: string;
  target?: string; delta?: string; check?: string; blocked_by: string[]; blocks: string[];
  start?: string; due?: string; done_at?: string; decision?: 'go' | 'later' | 'drop';
  actual?: string; result?: 'hit' | 'partial' | 'miss'; approval?: string; foreman?: string;
}

const TODAY = '2026-12-20';
const NARROW = window.matchMedia('(max-width: 640px)').matches;
const WINDOWS: Record<string, [string, string]> = {
  'bs-nuae030p': ['2026-11-02', '2027-01-31'], 'bs-d577i8s5': ['2026-11-02', '2026-12-01'],
  'bs-otoiif8q': ['2026-12-02', '2026-12-31'], 'bs-6lt623rg': ['2027-01-01', '2027-01-31'],
};
// Simulated progress as of TODAY (sample only).
const SIM: Record<string, Partial<T>> = {
  'bs-d577i8s5': { status: 'done', done_at: '2026-12-01' },
  'bs-otoiif8q': { status: 'in_progress' },
  'bs-apnq1chk': { status: 'done', done_at: '2026-11-20' },
  'bs-9d5cxh4a': { status: 'done', done_at: '2026-11-13', actual: '+3,1 tr ₫ nạp trong 3 ngày (kỳ vọng +4 tr)', result: 'partial' },
  'bs-ad2x4k90': { status: 'done', done_at: '2026-11-08', actual: 'Tạo được chiến dịch 11.11 trong admin', result: 'hit' },
  'bs-v938tpde': { status: 'done', done_at: '2026-11-30' },
  'bs-qk31ouun': { status: 'done', done_at: '2026-11-18' },
  'bs-7uq1vdhi': { status: 'in_review', foreman: 'fm-lohi-90d' },
  'bs-reuqzthc': { status: 'in_progress', foreman: 'fm-lohi-90d' },
  'bs-amgiwogj': { status: 'blocked' },
  'bs-de0o67ql': { status: 'done', done_at: '2026-11-25' },
  'bs-y9e2wkjw': { status: 'planned', approval: 'plan' },
  'bs-iix6owd3': { status: 'in_progress', foreman: 'fm-lohi-90d' },
};
const TICKETS: T[] = (raw as any[]).map(r => {
  const w = WINDOWS[r.id];
  return {
    ...r, role: r.role as Role, product: r.product ?? 'lohi',
    title: r.title.replace(/^(MILESTONE|PHASE \d) · /, '').replace(/ \([^)]*20\d\d\)$/, ''),
    start: w?.[0], due: w?.[1], decision: 'go', ...SIM[r.id],
  } as T;
});
const BY = Object.fromEntries(TICKETS.map(t => [t.id, t]));
const settled = (s: string) => s === 'done' || s === 'cancelled' || s === 'duplicate';
const phaseOf = (t: T) => (t.kind === 'phase' ? t : t.parent ? BY[t.parent] : undefined);
const addDays = (d: string, n: number) => new Date(Date.parse(d) + n * 864e5).toISOString().slice(0, 10);
const checkDue = (t: T) => (t.done_at && t.check ? addDays(t.done_at, parseInt(t.check, 10)) : undefined);
const fmtD = (d?: string) => (d ? `${d.slice(8, 10)}/${d.slice(5, 7)}` : '—');
// Effective blockers include every ancestor's: a phase-3 ticket waits on what phase 3 waits on.
const blockers = (t: T): string[] => [...t.blocked_by, ...(t.parent && BY[t.parent] ? blockers(BY[t.parent]) : [])];
const ready = (t: T) => !settled(t.status) && blockers(t).every(b => settled(BY[b]?.status ?? 'done'));

// A blocker that is a container (phase) stands for all of its work tickets.
function deps(t: T): string[] {
  return blockers(t).flatMap(b => (BY[b]?.children.length ? BY[b].children : [b]));
}
function waves(ids: string[]): string[][] {
  const left = new Set(ids); const out: string[][] = [];
  while (left.size) {
    const w = [...left].filter(id => deps(BY[id]).every(b => !left.has(b)));
    if (!w.length) break;
    w.forEach(id => left.delete(id)); out.push(w);
  }
  return out;
}

const frame: React.CSSProperties = { border: '1px solid var(--border-hairline)', borderRadius: 'var(--radius-md)', overflow: 'hidden' };
const mono: React.CSSProperties = { fontFamily: 'var(--font-mono)', fontSize: 12 };
const HC = 'px-3 py-1 text-xs font-medium uppercase flex items-center';
const muted: React.CSSProperties = { color: 'var(--text-muted)' };
const sec: React.CSSProperties = { color: 'var(--text-secondary)' };
const HS: React.CSSProperties = { color: 'var(--text-muted)', letterSpacing: 'var(--tracking-caption)' };

function Id({ id }: { id: string }) { return <span style={{ ...mono, color: 'var(--accent)', whiteSpace: 'nowrap' }}>{id}</span>; }
function RoleTag({ t }: { t: T }) {
  return <span className="text-xs" style={sec}>{t.role}{t.foreman ? <span style={{ ...mono, ...muted }}> · {t.foreman}</span> : null}</span>;
}

// ─── Plan: list + board, faceted ────────────────────────────────────────────
const FACETS = ['product', 'milestone', 'phase', 'role', 'status'] as const;
function Facet({ label, value, options, onChange }: { label: string; value: string; options: string[][]; onChange: (v: string) => void }) {
  return (
    <label className="text-xs flex items-center gap-1" style={sec}>
      <span className="uppercase" style={{ letterSpacing: 'var(--tracking-caption)', ...muted }}>{label}</span>
      <select value={value} onChange={e => onChange(e.target.value)} style={{ ...inputStyle, minHeight: 24, padding: '2px 6px', width: 'auto', fontSize: 12 }}>
        <option value="">any</option>
        {options.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
      </select>
    </label>
  );
}

function Plan({ open }: { open: (id: string) => void }) {
  const [f, setF] = useState<Record<string, string>>({ product: 'lohi', milestone: 'bs-nuae030p' });
  const [mode, setMode] = useState<'list' | 'board'>('list');
  const phases = TICKETS.filter(t => t.kind === 'phase');
  const rows = TICKETS.filter(t => t.kind === 'work').filter(t =>
    (!f.product || t.product === f.product) && (!f.phase || t.parent === f.phase) &&
    (!f.role || t.role === f.role) && (!f.status || t.status === f.status));
  const cols = '20px 112px 1fr 150px 56px 120px';
  return (
    <>
      <TopBar title="Plan" count={rows.length} actions={
        <div className="flex gap-1" role="group" aria-label="Layout">
          <Button size="sm" variant={mode === 'list' ? 'secondary' : 'ghost'} onClick={() => setMode('list')}>List</Button>
          <Button size="sm" variant={mode === 'board' ? 'secondary' : 'ghost'} onClick={() => setMode('board')}>Board</Button>
        </div>} />
      <div className="flex flex-wrap gap-4 items-center" style={{ padding: '12px var(--pad-page)', borderBottom: '1px solid var(--border-hairline)' }}>
        <Facet label="Product" value={f.product} options={[['lohi', 'lohi'], ['2novel', '2novel'], ['soot', 'soot'], ['2build', '2build']]} onChange={v => setF({ ...f, product: v })} />
        <Facet label="Milestone" value={f.milestone} options={[['bs-nuae030p', 'LoHi doanh thu 90 ngày']]} onChange={v => setF({ ...f, milestone: v })} />
        <Facet label="Phase" value={f.phase ?? ''} options={phases.map(p => [p.id, p.title])} onChange={v => setF({ ...f, phase: v })} />
        <Facet label="Role" value={f.role ?? ''} options={[['swe', 'swe'], ['human', 'human'], ['marketing', 'marketing'], ['ops', 'ops']]} onChange={v => setF({ ...f, role: v })} />
        <Facet label="Status" value={f.status ?? ''} options={['backlog', 'planned', 'in_progress', 'in_review', 'blocked', 'done'].map(s => [s, s])} onChange={v => setF({ ...f, status: v })} />
      </div>
      <div style={{ padding: 'var(--pad-page)' }}>
        {rows.length === 0 ? <EmptyState title="No tickets match" body="Clear a facet to widen the list." /> : mode === 'list' ? (
          <div className="space-y-6">
            {phases.filter(p => !f.phase || p.id === f.phase).map(p => {
              const list = rows.filter(r => r.parent === p.id);
              if (!list.length) return null;
              return (
                <SectionHeader key={p.id} title={`${p.title} · ${fmtD(p.start)}–${fmtD(p.due)}`} count={list.length}>
                  <div className="mt-1" style={frame}>
                    <DenseRow columns={cols} header>
                      <span className={HC} /><span className={HC} style={HS}>ID</span><span className={HC} style={HS}>Title</span><span className={HC} style={HS}>Role</span><span className={HC} style={HS}>Check</span><span className={HC} style={HS}>Waiting on</span>
                    </DenseRow>
                    {list.map(t => (
                      <DenseRow key={t.id} columns={cols} onClick={() => open(t.id)} tabIndex={0} role="link">
                        <span className="pl-2 flex items-center"><StatusArc status={t.status} /></span>
                        <span className="px-3"><Id id={t.id} /></span>
                        <span className="px-3 text-sm truncate">{t.title}</span>
                        <span className="px-3 truncate"><RoleTag t={t} /></span>
                        <span className="px-3 text-xs" style={sec}>{t.check ? `${t.check}d` : '—'}</span>
                        <span className="px-3 text-xs truncate" style={muted}>{blockers(t).filter(b => !settled(BY[b].status)).join(', ') || (settled(t.status) ? '' : 'ready')}</span>
                      </DenseRow>
                    ))}
                  </div>
                </SectionHeader>
              );
            })}
          </div>
        ) : (
          <div className="grid gap-3" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))' }}>
            {(['backlog', 'planned', 'in_progress', 'in_review', 'blocked', 'done'] as TicketStatus[]).map(s => {
              const list = rows.filter(r => r.status === s || (s === 'backlog' && r.status === 'triage'));
              return (
                <section key={s} aria-label={s}>
                  <div className="flex items-center gap-2 mb-2"><StatusArc status={s} /><Tag status={s}>{s.replace('_', ' ')}</Tag><span className="text-xs" style={muted}>{list.length}</span></div>
                  <div className="space-y-2">
                    {list.map(t => (
                      <button key={t.id} type="button" onClick={() => open(t.id)} className="block w-full text-left p-2" style={{ ...frame, background: 'var(--surface-bg)', cursor: 'pointer' }}>
                        <div className="flex justify-between gap-2"><Id id={t.id} /><RoleTag t={t} /></div>
                        <div className="text-sm mt-1" style={{ color: 'var(--text-primary)' }}>{t.title}</div>
                        <div className="text-xs mt-1" style={muted}>{phaseOf(t)?.title}</div>
                      </button>
                    ))}
                  </div>
                </section>
              );
            })}
          </div>
        )}
      </div>
    </>
  );
}

// ─── Milestone: progress per phase + DAG waves ──────────────────────────────
function Bar({ done, total }: { done: number; total: number }) {
  return (
    <div className="flex items-center gap-2">
      <div style={{ flex: 1, height: 4, borderRadius: 'var(--radius-pill)', background: 'var(--surface-sunken)' }}>
        <div style={{ width: `${total ? (100 * done) / total : 0}%`, height: 4, borderRadius: 'var(--radius-pill)', background: 'var(--status-completed-text)' }} />
      </div>
      <span className="text-xs" style={{ ...sec, minWidth: 40 }}>{done}/{total}</span>
    </div>
  );
}
function Milestone({ open }: { open: (id: string) => void }) {
  const m = BY['bs-nuae030p'];
  const work = TICKETS.filter(t => t.kind === 'work');
  const ws = waves(work.map(t => t.id));
  const elapsed = (Date.parse(TODAY) - Date.parse(m.start!)) / (Date.parse(m.due!) - Date.parse(m.start!));
  return (
    <>
      <TopBar title={m.title} breadcrumb={<span style={muted}>Milestones /</span>} />
      <div className="space-y-8" style={{ padding: 'var(--pad-page)', maxWidth: 'var(--content-max)' }}>
        <section className="grid gap-4" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))' }}>
          {[['Product', 'lohi'], ['Window', `${fmtD(m.start)} 2026 – ${fmtD(m.due)} 2027`], ['Target outcome', m.target], ['Expected delta', m.delta], ['Check', `${parseInt(m.check ?? '0', 10)} days after start · read ${addDays(m.start!, parseInt(m.check ?? '0', 10)).split('-').reverse().join('/')}`], ['Owner', 'human · founder']].map(([k, v]) => (
            <div key={k}><div className="text-xs uppercase" style={{ ...muted, letterSpacing: 'var(--tracking-caption)' }}>{k}</div><div className="text-sm mt-1">{v}</div></div>
          ))}
        </section>
        <section>
          <SectionHeader title="Time vs. work" />
          <div className="mt-2 space-y-2 text-xs" style={sec}>
            <div>Time elapsed <Bar done={Math.round(elapsed * 100)} total={100} /></div>
            <div>Work done <Bar done={work.filter(t => t.status === 'done').length} total={work.length} /></div>
          </div>
        </section>
        <section>
          <SectionHeader title="Phases" count={3} />
          <div className="mt-1" style={frame}>
            <DenseRow columns="20px 112px 1fr 120px 180px" header><span className={HC} /><span className={HC} style={HS}>ID</span><span className={HC} style={HS}>Phase</span><span className={HC} style={HS}>Window</span><span className={HC} style={HS}>Progress</span></DenseRow>
            {TICKETS.filter(t => t.kind === 'phase').map(p => {
              const kids = p.children.map(c => BY[c]);
              const late = !settled(p.status) && p.due! < TODAY;
              return (
                <DenseRow key={p.id} columns="20px 112px 1fr 120px 180px" onClick={() => open(p.id)} tabIndex={0} role="link">
                  <span className="pl-2 flex items-center"><StatusArc status={p.status} /></span>
                  <span className="px-3"><Id id={p.id} /></span>
                  <span className="px-3 text-sm truncate">{p.title}</span>
                  <span className="px-3 text-xs" style={{ color: late ? 'var(--status-blocked-text)' : 'var(--text-secondary)' }}>{fmtD(p.start)}–{fmtD(p.due)}{late ? ' · late' : ''}</span>
                  <span className="px-3"><Bar done={kids.filter(k => k.status === 'done').length} total={kids.length} /></span>
                </DenseRow>
              );
            })}
          </div>
        </section>
        <section>
          <SectionHeader title="Waves" count={ws.length} />
          <p className="text-xs mt-1" style={muted}>A wave starts when everything it is blocked by is done. Same layering as <code style={mono}>bbs ticket dag bs-nuae030p</code>, now with titles.</p>
          <div className="mt-2 grid gap-3" style={{ gridTemplateColumns: `repeat(auto-fit, minmax(240px, 1fr))` }}>
            {ws.map((w, i) => (
              <div key={i} style={frame}>
                <div className="px-3 py-2 text-xs uppercase" style={{ ...muted, letterSpacing: 'var(--tracking-caption)', background: 'var(--surface-elevated)', borderBottom: '1px solid var(--border-hairline)' }}>Wave {i} · {w.length}</div>
                {w.map(id => {
                  const t = BY[id];
                  const state = settled(t.status) ? 'done' : t.status === 'in_progress' || t.status === 'in_review' ? 'running' : ready(t) ? 'ready' : 'waiting';
                  return (
                    <DenseRow key={id} columns="20px 1fr 76px" onClick={() => open(id)} tabIndex={0} role="link">
                      <span className="pl-2 flex items-center"><StatusArc status={t.status} /></span>
                      <span className="px-2 text-xs truncate" title={t.title}>{t.title}</span>
                      <span className="px-2"><Tag tone={state === 'done' ? 'ok' : state === 'running' ? 'warn' : state === 'ready' ? 'accent' : 'muted'}>{state}</Tag></span>
                    </DenseRow>
                  );
                })}
              </div>
            ))}
          </div>
        </section>
      </div>
    </>
  );
}

// ─── Needs you ──────────────────────────────────────────────────────────────
function Queue({ title, icon, rows, why, open }: { title: string; icon: React.ReactNode; rows: T[]; why: (t: T) => React.ReactNode; open: (id: string) => void }) {
  return (
    <SectionHeader title={title} count={rows.length}>
      {rows.length === 0 ? <p className="text-xs mt-1" style={muted}>Nothing here.</p> : (
        <div className="mt-1" style={frame}>
          {rows.map(t => (
            <DenseRow key={t.id} columns={NARROW ? '28px 1fr' : '28px 112px 1fr 240px'} onClick={() => open(t.id)} tabIndex={0} role="link">
              <span className="pl-2 flex items-center" style={muted}>{icon}</span>
              {NARROW ? (
                <span className="px-2 py-1 min-w-0">
                  <span className="block text-sm truncate">{t.title}</span>
                  <span className="block text-xs truncate" style={sec}><Id id={t.id} /> · {why(t)}</span>
                </span>
              ) : (<>
                <span className="px-3"><Id id={t.id} /></span>
                <span className="px-3 text-sm truncate">{t.title}</span>
                <span className="px-3 text-xs truncate" style={sec}>{why(t)}</span>
              </>)}
            </DenseRow>
          ))}
        </div>
      )}
    </SectionHeader>
  );
}
function Needs({ open }: { open: (id: string) => void }) {
  const work = TICKETS.filter(t => t.kind !== 'milestone');
  const gates = work.filter(t => t.approval);
  const human = work.filter(t => t.role === 'human' && t.kind === 'work' && ready(t) && t.status !== 'blocked');
  const blocked = work.filter(t => t.status === 'blocked');
  const due = work.filter(t => { const d = checkDue(t); return d && d <= TODAY && !t.actual; });
  const total = gates.length + human.length + blocked.length + due.length;
  const ic = { size: 14, strokeWidth: 1.75, 'aria-hidden': true } as const;
  return (
    <>
      <TopBar title="Needs you" count={total} />
      <div className="space-y-8" style={{ padding: 'var(--pad-page)', maxWidth: 'var(--content-max)' }}>
        <Queue title="Waiting on your go" icon={<Flag {...ic} />} rows={gates} open={open} why={t => `${t.approval} review · agent is blocked`} />
        <Queue title="Checks due" icon={<CalendarClock {...ic} />} rows={due} open={open} why={t => `due ${fmtD(checkDue(t))} · ${t.target}`} />
        <Queue title="Your tickets, ready now" icon={<Inbox {...ic} />} rows={human} open={open} why={t => `${phaseOf(t)?.title ?? ''} · ends ${fmtD(phaseOf(t)?.due)}`} />
        <Queue title="Blocked" icon={<CircleAlert {...ic} />} rows={blocked} open={open} why={() => 'status blocked · say what unblocks it'} />
      </div>
    </>
  );
}

// ─── Company: decision log + scoreboard + founder hours ─────────────────────
const WEEKS = ['2026-W45', '2026-W46', '2026-W47', '2026-W48', '2026-W49', '2026-W50', '2026-W51'];
const REV = [10.2, 13.9, 9.8, 10.6, 11.4, 12.0, 12.7];
const STAGES = ['insight', 'product', 'marketing', 'operation', 'building soot'];
const HOURS: Record<string, number[]> = { lohi: [2, 6, 7, 4, 0], '2novel': [1, 3, 1, 1, 0], '2found': [0, 1, 0, 2, 9] };
function Company({ open, record }: { open: (id: string) => void; record: (id: string) => void }) {
  const log = TICKETS.filter(t => t.kind === 'work').slice(0, 9);
  const cols = '84px 112px 1fr 80px 56px 1fr';
  return (
    <>
      <TopBar title="Company loop" actions={<Button size="sm" variant="secondary">Export CSV</Button>} />
      <div className="space-y-8" style={{ padding: 'var(--pad-page)' }}>
        <SectionHeader title="Scoreboard · lohi · weekly" action={<Button size="sm" variant="ghost">Add week</Button>}>
          <div className="mt-1 overflow-x-auto" style={frame}>
            <DenseRow columns={`140px repeat(${WEEKS.length}, minmax(72px, 1fr))`} header>
              <span className={HC} style={HS}>Metric</span>{WEEKS.map(w => <span key={w} className={HC} style={{ ...HS, ...mono }}>{w.slice(5)}</span>)}
            </DenseRow>
            {[['Revenue (tr ₫)', REV], ['Active users', [5210, 5480, 5390, 5620, 5900, 6010, 6230]], ['Support cases', [41, 38, 52, 33, 30, 29, 31]], ['Hosting + AI (tr ₫)', [2.1, 2.1, 2.3, 2.2, 2.4, 2.4, 2.5]]].map(([k, v]) => (
              <DenseRow key={k as string} columns={`140px repeat(${WEEKS.length}, minmax(72px, 1fr))`}>
                <span className="px-3 text-xs" style={sec}>{k as string}</span>
                {(v as number[]).map((n, i) => <span key={i} className="px-3 text-sm">{n.toLocaleString('vi-VN')}</span>)}
              </DenseRow>
            ))}
          </div>
          <p className="text-xs mt-1" style={muted}>Product picker: lohi · 2novel · 2found. Sample numbers.</p>
        </SectionHeader>
        <SectionHeader title="Founder hours · last week · product × stage" action={<Button size="sm" variant="ghost">Log hours</Button>}>
          <div className="mt-1 overflow-x-auto" style={frame}>
            <DenseRow columns={`100px repeat(${STAGES.length}, minmax(88px, 1fr)) 64px`} header>
              <span className={HC} style={HS}>Product</span>{STAGES.map(s => <span key={s} className={HC} style={HS}>{s}</span>)}<span className={HC} style={HS}>Total</span>
            </DenseRow>
            {Object.entries(HOURS).map(([p, h]) => (
              <DenseRow key={p} columns={`100px repeat(${STAGES.length}, minmax(88px, 1fr)) 64px`}>
                <span className="px-3 text-xs" style={mono}>{p}</span>
                {h.map((n, i) => <span key={i} className="px-3 text-sm" style={n ? undefined : muted}>{n || '—'}</span>)}
                <span className="px-3 text-sm font-medium">{h.reduce((a, b) => a + b, 0)}</span>
              </DenseRow>
            ))}
          </div>
          <p className="text-xs mt-1" style={muted}>"Building soot" is reported apart and never netted against savings (§7).</p>
        </SectionHeader>
        <SectionHeader title="Decision log" count={log.length}>
          <div className="mt-1 overflow-x-auto" style={frame}>
            <DenseRow columns={cols} header><span className={HC} style={HS}>At</span><span className={HC} style={HS}>Ticket</span><span className={HC} style={HS}>Target outcome · expected</span><span className={HC} style={HS}>Decision</span><span className={HC} style={HS}>Check</span><span className={HC} style={HS}>Actual</span></DenseRow>
            {log.map(t => {
              const d = checkDue(t);
              return (
                <DenseRow key={t.id} columns={cols} onClick={() => open(t.id)} tabIndex={0} role="link">
                  <span className="px-3 text-xs" style={sec}>02/11</span>
                  <span className="px-3"><Id id={t.id} /></span>
                  <span className="px-3 text-xs truncate" title={`${t.target} · ${t.delta}`}>{t.target} <span style={muted}>· {t.delta}</span></span>
                  <span className="px-3"><Tag tone="ok">go</Tag></span>
                  <span className="px-3 text-xs" style={sec}>{t.check}d</span>
                  <span className="px-3 text-xs truncate">
                    {t.actual ? <><Tag tone={t.result === 'hit' ? 'ok' : t.result === 'partial' ? 'warn' : 'err'}>{t.result}</Tag> <span style={sec}>{t.actual}</span></>
                      : d && d <= TODAY ? <Button size="sm" variant="ghost" onClick={e => { e.stopPropagation(); record(t.id); }}>Record actual · due {fmtD(d)}</Button>
                        : <span style={muted}>{d ? `due ${fmtD(d)}` : 'after done'}</span>}
                  </span>
                </DenseRow>
              );
            })}
          </div>
          <p className="text-xs mt-1" style={muted}>Hit rate (go, checked): 1 hit · 1 partial · 0 miss. Every row is a history event; nothing here edits a file.</p>
        </SectionHeader>
      </div>
    </>
  );
}

// ─── Ticket panel + record form ─────────────────────────────────────────────
function Panel({ id, close, record }: { id: string; close: () => void; record: (id: string) => void }) {
  const t = BY[id];
  const props: [string, React.ReactNode][] = [
    ['Kind', t.kind], ['Product', t.product], ['Role', <RoleTag t={t} />], ['Status', <span className="flex items-center gap-2"><StatusArc status={t.status} /><Tag status={t.status}>{t.status.replace('_', ' ')}</Tag></span>],
    ['Phase', phaseOf(t)?.title ?? '—'], ['Window', t.start ? `${fmtD(t.start)} – ${fmtD(t.due)}` : `inherits phase · ${fmtD(phaseOf(t)?.due)}`], ['Size', t.size ?? '—'],
    ['Blocked by', t.blocked_by.length ? t.blocked_by.map(b => <div key={b}><Id id={b} /></div>) : '—'], ['Blocks', t.blocks.length ? t.blocks.map(b => <div key={b}><Id id={b} /></div>) : '—'],
    ['Stage', t.role === 'swe' ? 'product' : 'marketing'], ['Target', t.target ?? '—'], ['Expected', t.delta ?? '—'], ['Check after', t.check ? `${t.check} days · due ${fmtD(checkDue(t))}` : '—'],
    ['Decision', <Tag tone="ok">{t.decision}</Tag>], ['Actual', t.actual ?? '—'],
  ];
  return (
    <Modal open onClose={close} title={t.title} width={560} actions={<>
      <Button variant="ghost" onClick={close}>Close</Button>
      {t.kind === 'work' && !t.actual && <Button variant="primary" onClick={() => record(id)}>Record actual</Button>}
    </>}>
      <div className="text-xs mb-3"><Id id={t.id} /></div>
      <dl className="grid gap-y-2 gap-x-4 text-sm" style={{ gridTemplateColumns: '110px 1fr' }}>
        {props.map(([k, v]) => <><dt key={k + 'k'} className="text-xs" style={muted}>{k}</dt><dd key={k + 'v'} className="m-0">{v}</dd></>)}
      </dl>
    </Modal>
  );
}
function RecordForm({ id, close }: { id: string; close: () => void }) {
  const t = BY[id];
  const [result, setResult] = useState('');
  return (
    <Modal open onClose={close} title="Record actual" width={480} actions={<>
      <Button variant="ghost" onClick={close}>Cancel</Button>
      <Button variant="primary" disabled={!result} onClick={close}>Record</Button>
    </>}>
      <p className="text-xs mb-3" style={sec}><Id id={t.id} /> · {t.target} · expected {t.delta}</p>
      <div className="space-y-3">
        <Field label="Actual delta" hint="What moved, in the metric's own unit.">{(fid, hid) => <input id={fid} aria-describedby={hid} style={inputStyle} />}</Field>
        <Field label="Result" hint="Hit = moved at least half of the expected delta (§7).">{fid => (
          <select id={fid} value={result} onChange={e => setResult(e.target.value)} style={inputStyle}>
            <option value="">Choose…</option><option value="hit">hit</option><option value="partial">partial</option><option value="miss">miss</option>
          </select>)}</Field>
        <Field label="Evidence refs" hint="One URL or ticket path per line.">{fid => <textarea id={fid} rows={2} style={inputStyle} />}</Field>
      </div>
      <p className="text-xs mt-3" style={muted}>Runs <code style={mono}>bbs ticket record-actual</code> as you. Appends a history event; a later record supersedes it, nothing is erased.</p>
    </Modal>
  );
}

// ─── Shell ──────────────────────────────────────────────────────────────────
const VIEWS = [['plan', 'Plan', Layers], ['milestone', 'Milestones', Flag], ['needs', 'Needs you', Inbox], ['company', 'Company', LineChart]] as const;
function App() {
  const [view, setView] = useState(location.hash.slice(1) || 'needs');
  const [panel, setPanel] = useState<string | null>(null);
  const [rec, setRec] = useState<string | null>(null);
  useEffect(() => { location.hash = view; }, [view]);
  useEffect(() => {
    const on = () => setView(location.hash.slice(1) || 'needs');
    window.addEventListener('hashchange', on);
    return () => window.removeEventListener('hashchange', on);
  }, []);
  const open = (id: string) => setPanel(id);
  const record = (id: string) => { setPanel(null); setRec(id); };
  const body = useMemo(() => ({ plan: <Plan open={open} />, milestone: <Milestone open={open} />, needs: <Needs open={open} />, company: <Company open={open} record={record} /> } as Record<string, React.ReactElement>)[view], [view]);
  return (
    <div className="flex flex-col sm:flex-row min-h-screen" style={{ background: 'var(--surface-bg)', color: 'var(--text-primary)' }}>
      <nav className="flex sm:flex-col gap-1 p-3 sm:w-60 shrink-0 overflow-x-auto" style={{ background: 'var(--surface-nav)', borderRight: '1px solid var(--border-nav)' }} aria-label="Prototype views">
        <div className="hidden sm:block text-sm font-medium px-2 py-2" style={{ fontFamily: 'var(--font-display)' }}>2build</div>
        {VIEWS.map(([k, label, Icon]) => (
          <button key={k} type="button" onClick={() => setView(k)} aria-current={view === k ? 'page' : undefined}
            className="flex items-center gap-2 px-2 text-sm text-left whitespace-nowrap" style={{ minHeight: 32, borderRadius: 'var(--radius-sm)', background: view === k ? 'var(--surface-nav-elevated)' : 'transparent', color: view === k ? 'var(--text-nav-active)' : 'var(--text-nav)', border: 'none', cursor: 'pointer' }}>
            <Icon size={14} strokeWidth={1.75} aria-hidden /> {label}
          </button>
        ))}
        <div className="hidden sm:block mt-auto text-xs px-2 pt-4" style={muted}>Prototype · sample state as of {TODAY}</div>
      </nav>
      <main className="flex-1 min-w-0">{body}</main>
      {panel && <Panel id={panel} close={() => setPanel(null)} record={record} />}
      {rec && <RecordForm id={rec} close={() => setRec(null)} />}
    </div>
  );
}

const params = new URLSearchParams(location.search);
document.documentElement.dataset.theme = params.get('theme') === 'dark' ? 'dark' : 'light';
createRoot(document.getElementById('root')!).render(<App />);
