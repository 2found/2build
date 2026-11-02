import { useMemo, useState } from 'react';
import type { Snapshot, TicketStatus } from '../lib/data';
import { PLAN_ROLES } from '../lib/data';
import { TopBar } from '../components/TopBar';
import { DenseRow } from '../components/DenseRow';
import { SectionHeader } from '../components/SectionHeader';
import { StatusArc } from '../components/StatusArc';
import { Tag } from '../components/Tag';
import { Button } from '../components/Button';
import { EmptyState } from '../components/EmptyState';
import { Facet, Id, HC, HS, frame, muted, sec } from '../components/PlanParts';
import { useFilter } from '../contexts/FilterContext';
import {
  BOARD_COLUMNS, NO_FACETS, SETTLED, applyFacets, boardColumn, fmtDay, fmtWindow, planItems, ticketHref, uniq,
  type Facets, type PlanItem,
} from '../lib/plan';

const COLS = '20px 112px minmax(0,1fr) 140px 64px minmax(0,160px)';

function Who({ i }: { i: PlanItem }) {
  const { role, person, assignee } = i.p;
  return (
    <span className="text-xs truncate" style={sec}>
      {role || '—'}{person ? ` · ${person}` : ''}
      {assignee ? <span style={muted}> · {assignee}</span> : null}
    </span>
  );
}

function waitingOn(i: PlanItem): string {
  if (i.p.blocked_by.length) return i.p.blocked_by.join(', ');
  if (SETTLED.has(i.p.status)) return '';
  return i.p.ready ? 'ready' : '';
}

function open(id: string) {
  window.location.hash = ticketHref(id);
}

export function Plan({ snapshot }: { snapshot: Snapshot }) {
  const { state } = useFilter();
  const all = useMemo(() => planItems(snapshot, state.project), [snapshot, state.project]);
  const [f, setF] = useState<Facets>(NO_FACETS);
  const [mode, setMode] = useState<'list' | 'board'>('list');
  const byId = useMemo(() => new Map(all.map(i => [i.p.id, i])), [all]);
  const label = (id: string) => byId.get(id)?.p.title || id;

  const work = useMemo(() => applyFacets(all.filter(i => i.p.kind === 'work'), f), [all, f]);
  const milestones = all.filter(i => i.p.kind === 'milestone');
  const phases = all.filter(i => i.p.kind === 'phase' && (!f.milestone || i.p.milestone === f.milestone));

  // List groups: each phase in window order, then work outside any phase.
  const groups = useMemo(() => {
    const ids = uniq(work.map(w => w.p.phase));
    const ordered = ids
      .map(id => byId.get(id))
      .filter((p): p is PlanItem => !!p)
      .sort((a, b) => (a.p.start || '~').localeCompare(b.p.start || '~') || a.p.title.localeCompare(b.p.title));
    const out = ordered.map(p => ({ key: p.p.id, title: `${p.p.title} · ${fmtWindow(p.p.start, p.p.due)}`, rows: work.filter(w => w.p.phase === p.p.id) }));
    const loose = work.filter(w => !w.p.phase);
    if (loose.length) out.push({ key: '', title: 'No phase', rows: loose });
    return out;
  }, [work, byId]);

  const set = (k: keyof Facets) => (v: string) => setF({ ...f, [k]: v, ...(k === 'milestone' ? { phase: '' } : {}) });

  return (
    <>
      <TopBar title="Plan" count={work.length} warnings={snapshot.meta.warnings} actions={
        <div className="flex gap-1" role="group" aria-label="Layout">
          <Button size="sm" variant={mode === 'list' ? 'secondary' : 'ghost'} aria-pressed={mode === 'list'} onClick={() => setMode('list')}>List</Button>
          <Button size="sm" variant={mode === 'board' ? 'secondary' : 'ghost'} aria-pressed={mode === 'board'} onClick={() => setMode('board')}>Board</Button>
        </div>} />
      <div className="flex flex-wrap gap-4 items-center px-6 py-3" style={{ borderBottom: '1px solid var(--border-hairline)' }}>
        <Facet label="Product" value={f.product} options={uniq(all.map(i => i.p.product)).map(p => [p, p])} onChange={set('product')} />
        <Facet label="Milestone" value={f.milestone} options={milestones.map(m => [m.p.id, m.p.title || m.p.id])} onChange={set('milestone')} />
        <Facet label="Phase" value={f.phase} options={phases.map(p => [p.p.id, p.p.title || p.p.id])} onChange={set('phase')} />
        <Facet label="Role" value={f.role} options={PLAN_ROLES.map(r => [r, r])} onChange={set('role')} />
        <Facet label="Status" value={f.status} options={uniq(all.map(i => i.p.status)).map(s => [s, s.replace('_', ' ')])} onChange={set('status')} />
        <Facet label="Label" value={f.label} options={uniq(all.flatMap(i => i.p.labels)).map(l => [l, l])} onChange={set('label')} />
        {Object.values(f).some(Boolean) && <Button size="sm" variant="ghost" onClick={() => setF(NO_FACETS)}>Clear</Button>}
      </div>
      <div className="px-6 py-4">
        {work.length === 0 ? (
          <EmptyState title={all.length ? 'No tickets match' : 'No planned work'} body={all.length ? 'Clear a facet to widen the list.' : 'Tickets show here once they carry a kind — run `bbs ticket migrate` on an older store.'} />
        ) : mode === 'list' ? (
          <div className="space-y-6">
            {groups.map(g => (
              <SectionHeader key={g.key || 'none'} title={g.title} count={g.rows.length}>
                <div className="mt-1 overflow-x-auto" style={frame}>
                  <DenseRow columns={COLS} header>
                    <span className={HC} /><span className={HC} style={HS}>ID</span><span className={HC} style={HS}>Title</span>
                    <span className={HC} style={HS}>Role</span><span className={HC} style={HS}>Due</span><span className={HC} style={HS}>Waiting on</span>
                  </DenseRow>
                  {g.rows.map(i => (
                    <DenseRow key={i.p.id} columns={COLS} onClick={() => open(i.p.id)} tabIndex={0} role="link">
                      <span className="pl-2 flex items-center"><StatusArc status={i.p.status as TicketStatus} /></span>
                      <span className="px-3"><Id id={i.p.id} /></span>
                      <span className="px-3 text-sm truncate" title={i.p.title}>{i.p.title}</span>
                      <span className="px-3 min-w-0 flex"><Who i={i} /></span>
                      <span className="px-3 text-xs" style={sec}>{fmtDay(i.p.due)}</span>
                      <span className="px-3 text-xs truncate" style={i.p.ready ? { color: 'var(--accent)' } : muted}
                        title={i.p.blocked_by.map(label).join('\n')}>{waitingOn(i)}</span>
                    </DenseRow>
                  ))}
                </div>
              </SectionHeader>
            ))}
          </div>
        ) : (
          <div className="grid gap-3" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))' }}>
            {BOARD_COLUMNS.map(s => {
              const list = work.filter(i => boardColumn(i.p.status) === s);
              return (
                <section key={s} aria-label={s.replace('_', ' ')}>
                  <div className="flex items-center gap-2 mb-2">
                    <StatusArc status={s} /><Tag status={s}>{s.replace('_', ' ')}</Tag><span className="text-xs" style={muted}>{list.length}</span>
                  </div>
                  <div className="space-y-2">
                    {list.map(i => (
                      <a key={i.p.id} href={ticketHref(i.p.id)} className="block p-2" style={{ ...frame, textDecoration: 'none' }}>
                        <div className="flex justify-between gap-2 min-w-0"><span style={{ fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--accent)' }}>{i.p.id}</span><Who i={i} /></div>
                        <div className="text-sm mt-1" style={{ color: 'var(--text-primary)' }}>{i.p.title}</div>
                        <div className="text-xs mt-1 truncate" style={muted}>{[label(i.p.phase || ''), i.p.ready ? 'ready' : ''].filter(Boolean).join(' · ')}</div>
                      </a>
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
