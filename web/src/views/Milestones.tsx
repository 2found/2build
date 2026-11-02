import { useEffect, useMemo, useState } from 'react';
import { fetchTicketDetail, type DagState, type Snapshot, type TicketDag, type TicketStatus } from '../lib/data';
import { TopBar } from '../components/TopBar';
import { DenseRow } from '../components/DenseRow';
import { SectionHeader } from '../components/SectionHeader';
import { StatusArc } from '../components/StatusArc';
import { Tag, type TagTone } from '../components/Tag';
import { EmptyState } from '../components/EmptyState';
import { ErrorBox } from '../components/ErrorBox';
import { Bar, Id, HC, HS, frame, mono, muted, sec } from '../components/PlanParts';
import { useFilter } from '../contexts/FilterContext';
import { fmtDay, fmtWindow, localToday, milestoneProgress, planItems, ticketHref, type PlanItem } from '../lib/plan';

const STATE_TONE: Record<DagState, TagTone> = { done: 'ok', running: 'warn', ready: 'accent', waiting: 'muted', not_found: 'err' };

function go(hash: string) {
  window.location.hash = hash;
}

function MilestoneList({ items, today }: { items: PlanItem[]; today: string }) {
  const ms = items.filter(i => i.p.kind === 'milestone').sort((a, b) => (b.p.start || '').localeCompare(a.p.start || ''));
  const cols = '20px 112px minmax(0,1fr) 90px 150px 160px';
  if (!ms.length) return <EmptyState title="No milestones" body="Mark a ticket `bbs ticket set-kind milestone` and its phases `phase`." />;
  return (
    <div className="overflow-x-auto" style={frame}>
      <DenseRow columns={cols} header>
        <span className={HC} /><span className={HC} style={HS}>ID</span><span className={HC} style={HS}>Milestone</span>
        <span className={HC} style={HS}>Product</span><span className={HC} style={HS}>Window</span><span className={HC} style={HS}>Work done</span>
      </DenseRow>
      {ms.map(m => {
        const pr = milestoneProgress(items, m.p.id, today)!;
        return (
          <DenseRow key={m.p.id} columns={cols} onClick={() => go(`#/milestones/${m.p.id}`)} tabIndex={0} role="link">
            <span className="pl-2 flex items-center"><StatusArc status={m.p.status as TicketStatus} /></span>
            <span className="px-3"><Id id={m.p.id} /></span>
            <span className="px-3 text-sm truncate" title={m.p.title}>{m.p.title}</span>
            <span className="px-3 text-xs" style={mono}>{m.p.product || '—'}</span>
            <span className="px-3 text-xs" style={sec}>{fmtWindow(m.p.start, m.p.due)}</span>
            <span className="px-3"><Bar done={pr.workDone} total={pr.workTotal} /></span>
          </DenseRow>
        );
      })}
    </div>
  );
}

function Waves({ project, id, title, embedded }: { project: string; id: string; title: (id: string) => string; embedded?: TicketDag | null }) {
  const [fetched, setDag] = useState<TicketDag | null>(null);
  const [err, setErr] = useState('');
  const dag = embedded ?? fetched;
  useEffect(() => {
    if (embedded) return;
    const ctl = new AbortController();
    setErr('');
    fetchTicketDetail(project, id, ctl.signal).then(
      d => setDag(d.dag ?? null),
      e => { if (!ctl.signal.aborted) setErr(e instanceof Error ? e.message : String(e)); },
    );
    return () => ctl.abort();
  }, [project, id, embedded]);
  if (err) return <ErrorBox title="Waves unavailable" body={err} />;
  if (!dag) return <p className="text-xs" style={muted}>Loading waves…</p>;
  const nodes = new Map(dag.nodes.map(n => [n.id, n]));
  // Containers are part of the graph but not schedulable work; the waves show
  // the work tickets, as `bbs ticket dag` layers them.
  const waves = dag.waves
    .map(w => w.filter(nid => nid !== id && (nodes.get(nid)?.kind ?? 'work') === 'work'))
    .filter(w => w.length);
  return (
    <>
      {dag.cycles.length > 0 && <ErrorBox title="Dependency cycle" body={dag.cycles.map(c => c.join(' → ')).join('; ')} />}
      <div className="mt-2 grid gap-3" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))' }}>
        {waves.map((w, i) => (
          <div key={i} style={frame}>
            <div className="px-3 py-2 text-xs uppercase" style={{ ...muted, letterSpacing: 'var(--tracking-caption)', background: 'var(--surface-elevated)', borderBottom: '1px solid var(--border-hairline)' }}>
              Wave {i} · {w.length}
            </div>
            {w.map(nid => {
              const n = nodes.get(nid)!;
              const t = n.title || title(nid);
              return (
                <DenseRow key={nid} columns="20px minmax(0,1fr) 76px" onClick={() => go(ticketHref(nid))} tabIndex={0} role="link">
                  <span className="pl-2 flex items-center"><StatusArc status={n.status as TicketStatus} /></span>
                  <span className="px-2 text-xs truncate" title={`${nid} · ${t}`}>{t}</span>
                  <span className="px-2"><Tag tone={STATE_TONE[n.state]}>{n.state.replace('_', ' ')}</Tag></span>
                </DenseRow>
              );
            })}
          </div>
        ))}
      </div>
    </>
  );
}

function MilestoneDetail({ items, id, today, embedded }: { items: PlanItem[]; id: string; today: string; embedded?: TicketDag | null }) {
  const pr = milestoneProgress(items, id, today);
  const title = (tid: string) => items.find(i => i.p.id === tid)?.p.title || tid;
  if (!pr) return <ErrorBox title="Milestone not found" body={`${id} is not a ticket in this project scope.`} />;
  const m = pr.milestone.p;
  const loop = m.loop ?? {};
  const facts: [string, string][] = [
    ['Product', m.product || '—'],
    ['Window', fmtWindow(m.start, m.due)],
    ['Target outcome', loop.target_outcome || '—'],
    ['Expected delta', loop.expected_delta || '—'],
    ['Check', loop.check_after_days ? `${loop.check_after_days} days · read ${fmtDay(m.check_due)}` : '—'],
    ['Owner', [m.role, m.person].filter(Boolean).join(' · ') || '—'],
    ['Decision', loop.decision ? `${loop.decision.value} · ${loop.decision.by}` : '—'],
  ];
  return (
    <div className="space-y-8" style={{ maxWidth: 'var(--content-max)' }}>
      <section className="grid gap-4" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))' }}>
        {facts.map(([k, v]) => (
          <div key={k}><div className="text-xs uppercase" style={{ ...muted, letterSpacing: 'var(--tracking-caption)' }}>{k}</div><div className="text-sm mt-1">{v}</div></div>
        ))}
      </section>
      <section>
        <SectionHeader title="Time vs. work" />
        <div className="mt-2 space-y-2 text-xs" style={sec}>
          {pr.elapsed !== null && <div>Time elapsed <Bar done={Math.round(pr.elapsed * 100)} total={100} label="time" /></div>}
          <div>Work done <Bar done={pr.workDone} total={pr.workTotal} label="work" /></div>
        </div>
      </section>
      <SectionHeader title="Phases" count={pr.phases.length}>
        {pr.phases.length === 0 ? <p className="text-xs mt-1" style={muted}>No phase tickets under this milestone.</p> : (
          <div className="mt-1 overflow-x-auto" style={frame}>
            <DenseRow columns="20px 112px minmax(0,1fr) 150px 180px" header>
              <span className={HC} /><span className={HC} style={HS}>ID</span><span className={HC} style={HS}>Phase</span><span className={HC} style={HS}>Window</span><span className={HC} style={HS}>Progress</span>
            </DenseRow>
            {pr.phases.map(ph => (
              <DenseRow key={ph.item.p.id} columns="20px 112px minmax(0,1fr) 150px 180px" onClick={() => go(ticketHref(ph.item.p.id))} tabIndex={0} role="link">
                <span className="pl-2 flex items-center"><StatusArc status={ph.item.p.status as TicketStatus} /></span>
                <span className="px-3"><Id id={ph.item.p.id} /></span>
                <span className="px-3 text-sm truncate" title={ph.item.p.title}>{ph.item.p.title}</span>
                <span className="px-3 text-xs" style={{ color: ph.late ? 'var(--status-blocked-text)' : 'var(--text-secondary)' }}>
                  {fmtDay(ph.item.p.start)}–{fmtDay(ph.item.p.due)}{ph.late ? ' · late' : ''}
                </span>
                <span className="px-3"><Bar done={ph.done} total={ph.total} /></span>
              </DenseRow>
            ))}
          </div>
        )}
      </SectionHeader>
      <section>
        <SectionHeader title="Waves" />
        <p className="text-xs mt-1" style={muted}>A wave starts when everything it is blocked by is done — the same layering as <code style={mono}>bbs ticket dag {id}</code>.</p>
        <Waves project={pr.milestone.project} id={id} title={title} embedded={embedded} />
      </section>
    </div>
  );
}

export function Milestones({ snapshot, id }: { snapshot: Snapshot; id: string | null }) {
  const { state } = useFilter();
  const items = useMemo(() => planItems(snapshot, state.project), [snapshot, state.project]);
  const today = localToday();
  const m = id ? items.find(i => i.p.id === id) : undefined;
  return (
    <>
      <TopBar
        title={m ? m.p.title : 'Milestones'}
        warnings={snapshot.meta.warnings}
        breadcrumb={id ? (
          <div className="flex items-center gap-2 min-w-0">
            <a href="#/milestones" className="hover:underline shrink-0" style={{ ...muted, fontSize: 13 }}>Milestones</a>
            <span style={muted}>/</span>
            <span className="truncate" style={{ fontSize: 14, fontWeight: 500 }}>{m?.p.title || id}</span>
          </div>
        ) : undefined}
      />
      <div className="px-6 py-4">
        {id ? <MilestoneDetail items={items} id={id} today={today} embedded={m ? snapshot.projects[m.project]?.ticketDetail?.[id]?.dag : null} /> : <MilestoneList items={items} today={today} />}
      </div>
    </>
  );
}
