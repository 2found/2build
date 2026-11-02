import { useMemo, useState, type ReactNode } from 'react';
import { CalendarClock, CircleAlert, Flag, Inbox, Scale } from 'lucide-react';
import type { Snapshot } from '../lib/data';
import { TopBar } from '../components/TopBar';
import { DenseRow } from '../components/DenseRow';
import { SectionHeader } from '../components/SectionHeader';
import { Button } from '../components/Button';
import { DecideModal, Id, RecordActualModal, frame, muted, sec, type LoopTarget } from '../components/PlanParts';
import { gateProps, useControlPlane } from '../contexts/ControlContext';
import { useFilter } from '../contexts/FilterContext';
import { fmtDay, needsYou, planItems, ticketHref, type PlanItem } from '../lib/plan';

const ic = { size: 14, strokeWidth: 1.75, 'aria-hidden': true } as const;

function Queue({ title, icon, rows, why, action }: {
  title: string; icon: ReactNode; rows: PlanItem[];
  why: (i: PlanItem) => ReactNode; action?: (i: PlanItem) => ReactNode;
}) {
  return (
    <SectionHeader title={title} count={rows.length}>
      {rows.length === 0 ? <p className="text-xs mt-1" style={muted}>Nothing here.</p> : (
        <div className="mt-1" style={frame}>
          {rows.map(i => (
            <DenseRow key={i.p.id} columns="28px 112px minmax(0,1fr) minmax(0,240px) auto" onClick={() => { window.location.hash = ticketHref(i.p.id); }} tabIndex={0} role="link">
              <span className="pl-2 flex items-center" style={muted}>{icon}</span>
              <span className="px-3"><Id id={i.p.id} /></span>
              <span className="px-3 text-sm truncate" title={i.p.title}>{i.p.title}</span>
              <span className="px-3 text-xs truncate max-[639px]:hidden" style={sec}>{why(i)}</span>
              <span className="px-2">{action?.(i)}</span>
            </DenseRow>
          ))}
        </div>
      )}
    </SectionHeader>
  );
}

function target(i: PlanItem): LoopTarget {
  return { project: i.project, id: i.p.id, title: i.p.title, target: i.p.loop?.target_outcome, expected: i.p.loop?.expected_delta };
}

export function NeedsYou({ snapshot }: { snapshot: Snapshot }) {
  const { state } = useFilter();
  const { reason } = useControlPlane();
  const items = useMemo(() => planItems(snapshot, state.project), [snapshot, state.project]);
  const q = useMemo(() => needsYou(items), [items]);
  const title = useMemo(() => new Map(items.map(i => [i.p.id, i.p.title])), [items]);
  const [deciding, setDeciding] = useState<LoopTarget | null>(null);
  const [recording, setRecording] = useState<LoopTarget | null>(null);
  const total = q.gates.length + q.undecided.length + q.checks.length + q.mine.length + q.blocked.length;
  const stop = (fn: () => void) => (e: React.MouseEvent) => { e.stopPropagation(); fn(); };

  return (
    <>
      <TopBar title="Needs you" count={total} warnings={snapshot.meta.warnings} />
      <div className="space-y-8 px-6 py-4" style={{ maxWidth: 'var(--content-max)' }}>
        <Queue title="Waiting on your go" icon={<Flag {...ic} />} rows={q.gates}
          why={i => `${i.t.approval?.kind ?? 'design'} checkpoint · the agent is waiting`} />
        <Queue title="Decide go / later / drop" icon={<Scale {...ic} />} rows={q.undecided}
          why={i => i.p.loop?.target_outcome ?? ''}
          action={i => <Button size="sm" variant="ghost" {...gateProps(reason)} onClick={stop(() => setDeciding(target(i)))}>Decide</Button>} />
        <Queue title="Checks due" icon={<CalendarClock {...ic} />} rows={q.checks}
          why={i => `due ${fmtDay(i.p.check_due)} · ${i.p.loop?.target_outcome ?? ''}`}
          action={i => <Button size="sm" variant="ghost" {...gateProps(reason)} onClick={stop(() => setRecording(target(i)))}>Record actual</Button>} />
        <Queue title="Your tickets, ready now" icon={<Inbox {...ic} />} rows={q.mine}
          why={i => [title.get(i.p.phase), i.p.due ? `due ${fmtDay(i.p.due)}` : ''].filter(Boolean).join(' · ')} />
        <Queue title="Blocked" icon={<CircleAlert {...ic} />} rows={q.blocked}
          why={() => 'status blocked · say what unblocks it'} />
      </div>
      {deciding && <DecideModal t={deciding} onClose={() => setDeciding(null)} />}
      {recording && <RecordActualModal t={recording} onClose={() => setRecording(null)} />}
    </>
  );
}
