// The planning block on a ticket (bs-w5p3c6mq): what the ticket is for, who
// owns it, its window and its loop fields, with the founder's two actions —
// decide and record actual. Edits go through POST /fields, the same
// validators and writer as `bbs ticket set-*`; only changed fields log.

import { useState, type ReactNode } from 'react';
import type { PlanRow } from '../lib/data';
import { LOOP_STAGES, PLAN_ROLES } from '../lib/data';
import { setTicketFields, type FieldsPatch } from '../lib/api';
import { Button } from '../components/Button';
import { Modal } from '../components/Modal';
import { Tag } from '../components/Tag';
import { Field, inputStyle } from '../components/Field';
import { DecideModal, Id, RecordActualModal, muted, sec } from '../components/PlanParts';
import { gateProps, useControlPlane, useMutation } from '../contexts/ControlContext';
import { fmtDay, fmtWindow } from '../lib/plan';

interface Draft {
  title: string; kind: string; product: string; role: string; person: string; labels: string;
  start: string; due: string; stage: string; target: string; expected: string; checkAfter: string; checkFrom: string;
}

function draftOf(p: PlanRow): Draft {
  const l = p.loop ?? {};
  return {
    title: p.title, kind: p.kind, product: p.product, role: p.role, person: p.person, labels: p.labels.join(', '),
    start: p.start, due: p.due, stage: l.stage ?? '', target: l.target_outcome ?? '', expected: l.expected_delta ?? '',
    checkAfter: l.check_after_days ? String(l.check_after_days) : '', checkFrom: l.check_from ?? '',
  };
}

/** Only what changed goes to the server, so the history shows the edit and nothing else. */
export function patchOf(was: Draft, now: Draft): FieldsPatch {
  const patch: FieldsPatch = {};
  if (now.title !== was.title) patch.title = now.title.trim();
  if (now.kind !== was.kind) patch.kind = now.kind;
  if (now.product !== was.product) patch.product = now.product.trim();
  if (now.role !== was.role || now.person !== was.person) patch.assigned_to = { role: now.role, person: now.person.trim() };
  const split = (s: string) => s.split(',').map(x => x.trim()).filter(Boolean);
  const [before, after] = [split(was.labels), split(now.labels)];
  const add = after.filter(l => !before.includes(l));
  const remove = before.filter(l => !after.includes(l));
  if (add.length) patch.labels_add = add;
  if (remove.length) patch.labels_remove = remove;
  if (now.start !== was.start || now.due !== was.due) patch.window = { start: now.start, due: now.due };
  const loop: NonNullable<FieldsPatch['loop']> = {};
  if (now.stage !== was.stage) loop.stage = now.stage;
  if (now.target !== was.target) loop.target_outcome = now.target.trim();
  if (now.expected !== was.expected) loop.expected_delta = now.expected.trim();
  if (now.checkAfter !== was.checkAfter) loop.check_after_days = now.checkAfter.trim() === '' ? 0 : Number(now.checkAfter);
  if (now.checkFrom !== was.checkFrom) loop.check_from = now.checkFrom;
  if (Object.keys(loop).length) patch.loop = loop;
  return patch;
}

function EditModal({ project, p, onClose }: { project: string; p: PlanRow; onClose: () => void }) {
  const { reason } = useControlPlane();
  const { run, pending, error } = useMutation();
  const [was] = useState(() => draftOf(p));
  const [d, setD] = useState(was);
  const patch = patchOf(was, d);
  const empty = Object.keys(patch).length === 0;
  const badDays = d.checkAfter.trim() !== '' && !/^\d+$/.test(d.checkAfter.trim());
  const bind = (k: keyof Draft) => ({ value: d[k], onChange: (e: { target: { value: string } }) => setD({ ...d, [k]: e.target.value }), style: inputStyle });
  const submit = async () => {
    if (await run(() => setTicketFields(project, p.id, patch), `${p.id} updated`)) onClose();
  };
  return (
    <Modal open onClose={onClose} title="Edit planning" width={600} actions={<>
      <Button variant="ghost" onClick={onClose}>Cancel</Button>
      <Button variant="primary" size="lg" disabled={empty || badDays || pending} {...gateProps(reason)} onClick={submit}>{pending ? 'Saving…' : 'Save'}</Button>
    </>}>
      <div className="grid gap-3" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))' }}>
        <div style={{ gridColumn: '1 / -1' }}><Field label="Title">{fid => <input id={fid} {...bind('title')} />}</Field></div>
        <Field label="Kind">{fid => <select id={fid} {...bind('kind')}><option value="work">work</option><option value="phase">phase</option><option value="milestone">milestone</option></select>}</Field>
        <Field label="Product">{fid => <input id={fid} {...bind('product')} />}</Field>
        <Field label="Role">{fid => <select id={fid} {...bind('role')}><option value="">—</option>{PLAN_ROLES.map(r => <option key={r} value={r}>{r}</option>)}</select>}</Field>
        <Field label="Person">{fid => <input id={fid} {...bind('person')} />}</Field>
        <Field label="Start">{fid => <input id={fid} type="date" {...bind('start')} />}</Field>
        <Field label="Due">{fid => <input id={fid} type="date" {...bind('due')} />}</Field>
        <div style={{ gridColumn: '1 / -1' }}><Field label="Labels" hint="Comma-separated.">{(fid, hid) => <input id={fid} aria-describedby={hid} {...bind('labels')} />}</Field></div>
        <Field label="Stage">{fid => <select id={fid} {...bind('stage')}><option value="">—</option>{LOOP_STAGES.map(s => <option key={s} value={s}>{s}</option>)}</select>}</Field>
        <Field label="Check after (days)">{fid => <input id={fid} inputMode="numeric" {...bind('checkAfter')} />}</Field>
        <Field label="Check from" hint="Defaults to done date (work) or window start.">{(fid, hid) => <input id={fid} aria-describedby={hid} type="date" {...bind('checkFrom')} />}</Field>
        <div style={{ gridColumn: '1 / -1' }}><Field label="Target outcome">{fid => <input id={fid} {...bind('target')} />}</Field></div>
        <div style={{ gridColumn: '1 / -1' }}><Field label="Expected delta">{fid => <input id={fid} {...bind('expected')} />}</Field></div>
      </div>
      {error && <p role="alert" className="text-xs mt-3" style={{ color: 'var(--status-blocked-text)' }}>{error}</p>}
    </Modal>
  );
}

export function PlanningPanel({ project, p, title }: { project: string; p: PlanRow; title: (id: string) => string }) {
  const { reason } = useControlPlane();
  const [modal, setModal] = useState<'' | 'edit' | 'decide' | 'actual'>('');
  const l = p.loop ?? {};
  const target = { project, id: p.id, title: p.title, target: l.target_outcome, expected: l.expected_delta };
  const link = (id: string) => id ? <span title={title(id)}><Id id={id} /> <span style={muted}>{title(id) !== id ? title(id) : ''}</span></span> : '—';
  const rows: [string, ReactNode][] = [
    ['Kind', p.kind],
    ['Product', p.product || '—'],
    ['Owner', [p.role, p.person].filter(Boolean).join(' · ') || '—'],
    ['Labels', p.labels.length ? p.labels.join(', ') : '—'],
    ['Milestone', link(p.milestone === p.id ? '' : p.milestone)],
    ['Phase', link(p.phase === p.id ? '' : p.phase)],
    ['Window', fmtWindow(p.start, p.due)],
    ['Waiting on', p.blocked_by.length ? <span className="flex flex-wrap gap-2">{p.blocked_by.map(b => <Id key={b} id={b} />)}</span> : p.ready ? <Tag tone="accent">ready</Tag> : '—'],
    ['Stage', l.stage || '—'],
    ['Target', l.target_outcome || '—'],
    ['Expected', l.expected_delta || '—'],
    ['Check', l.check_after_days ? <span style={p.check_overdue ? { color: 'var(--status-blocked-text)' } : undefined}>{l.check_after_days} days{p.check_due ? ` · due ${fmtDay(p.check_due)}` : ''}{p.check_overdue ? ' · overdue' : ''}</span> : '—'],
    ['Decision', l.decision ? <span><Tag tone={l.decision.value === 'go' ? 'ok' : l.decision.value === 'drop' ? 'err' : 'muted'}>{l.decision.value}</Tag> <span className="text-xs" style={sec}>{l.decision.by}{l.decision.at ? ` · ${fmtDay(l.decision.at)}` : ''}{l.decision.note ? ` · ${l.decision.note}` : ''}</span></span> : '—'],
    ['Actual', l.actual ? <span><Tag tone={l.actual.result === 'hit' ? 'ok' : l.actual.result === 'partial' ? 'warn' : 'err'}>{l.actual.result}</Tag> <span className="text-xs" style={sec}>{l.actual.delta} · {l.actual.by}</span></span> : '—'],
  ];
  return (
    <section aria-label="Planning" className="p-3" style={{ border: '1px solid var(--border-hairline)', borderRadius: 'var(--radius-md)' }}>
      <div className="flex items-center justify-between gap-2 mb-2 flex-wrap">
        <span className="text-xs font-medium uppercase" style={{ ...muted, letterSpacing: 'var(--tracking-caption)' }}>Planning</span>
        <div className="flex gap-2">
          <Button size="sm" variant="ghost" {...gateProps(reason)} onClick={() => setModal('edit')}>Edit</Button>
          <Button size="sm" variant="ghost" {...gateProps(reason)} onClick={() => setModal('decide')}>Decide</Button>
          {l.decision?.value === 'go' && <Button size="sm" variant="ghost" {...gateProps(reason)} onClick={() => setModal('actual')}>Record actual</Button>}
        </div>
      </div>
      <dl className="grid gap-y-1.5 gap-x-4 text-sm" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))' }}>
        {rows.map(([k, v]) => (
          <div key={k} className="flex gap-3 min-w-0">
            <dt className="text-xs shrink-0" style={{ ...muted, width: 76, paddingTop: 2 }}>{k}</dt>
            <dd className="m-0 min-w-0 break-words">{v}</dd>
          </div>
        ))}
      </dl>
      {modal === 'edit' && <EditModal project={project} p={p} onClose={() => setModal('')} />}
      {modal === 'decide' && <DecideModal t={target} onClose={() => setModal('')} />}
      {modal === 'actual' && <RecordActualModal t={target} onClose={() => setModal('')} />}
    </section>
  );
}
