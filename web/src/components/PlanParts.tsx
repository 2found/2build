// Pieces the planning views share (bs-w5p3c6mq): the id link, the progress
// bar, a facet select, and the two founder forms — decide and record actual.
// Both forms call the same core writer the CLI verbs use; nothing here edits
// a file.

import { useState, type CSSProperties } from 'react';
import { Button } from './Button';
import { Modal } from './Modal';
import { Field, inputStyle } from './Field';
import { gateProps, useControlPlane, useMutation } from '../contexts/ControlContext';
import { decideTicket, recordActual } from '../lib/api';
import { ticketHref } from '../lib/plan';

export const frame: CSSProperties = { border: '1px solid var(--border-hairline)', borderRadius: 'var(--radius-md)', overflow: 'hidden', backgroundColor: 'var(--surface-bg)' };
export const mono: CSSProperties = { fontFamily: 'var(--font-mono)', fontSize: 12 };
export const muted: CSSProperties = { color: 'var(--text-muted)' };
export const sec: CSSProperties = { color: 'var(--text-secondary)' };
export const HC = 'px-3 py-1 text-xs font-medium uppercase flex items-center';
export const HS: CSSProperties = { color: 'var(--text-muted)', letterSpacing: 'var(--tracking-caption)' };

export function Id({ id }: { id: string }) {
  return (
    <a href={ticketHref(id)} onClick={e => e.stopPropagation()} className="hover:underline" style={{ ...mono, color: 'var(--accent)', whiteSpace: 'nowrap' }}>
      {id}
    </a>
  );
}

export function Bar({ done, total, label }: { done: number; total: number; label?: string }) {
  const pct = total ? Math.round((100 * done) / total) : 0;
  return (
    <div className="flex items-center gap-2" role="meter" aria-valuemin={0} aria-valuemax={total} aria-valuenow={done} aria-label={label}>
      <div style={{ flex: 1, height: 4, borderRadius: 'var(--radius-pill)', background: 'var(--surface-sunken)' }}>
        <div style={{ width: `${pct}%`, height: 4, borderRadius: 'var(--radius-pill)', background: 'var(--status-completed-text)' }} />
      </div>
      <span className="text-xs" style={{ ...sec, minWidth: 40, fontVariantNumeric: 'tabular-nums' }}>{label === 'time' ? `${pct}%` : `${done}/${total}`}</span>
    </div>
  );
}

export function Facet({ label, value, options, onChange }: {
  label: string; value: string; options: [string, string][]; onChange: (v: string) => void;
}) {
  return (
    <label className="text-xs flex items-center gap-1" style={sec}>
      <span className="uppercase" style={{ letterSpacing: 'var(--tracking-caption)', ...muted }}>{label}</span>
      <select value={value} onChange={e => onChange(e.target.value)} style={{ ...inputStyle, minHeight: 24, padding: '2px 6px', width: 'auto', fontSize: 12, maxWidth: 220 }}>
        <option value="">any</option>
        {options.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
      </select>
    </label>
  );
}

function lines(s: string): string[] {
  return s.split('\n').map(l => l.trim()).filter(Boolean);
}

export interface LoopTarget {
  project: string;
  id: string;
  title: string;
  target?: string | null;
  expected?: string | null;
}

export function RecordActualModal({ t, onClose }: { t: LoopTarget; onClose: () => void }) {
  const { reason } = useControlPlane();
  const { run, pending, error } = useMutation();
  const [delta, setDelta] = useState('');
  const [result, setResult] = useState('');
  const [note, setNote] = useState('');
  const [evidence, setEvidence] = useState('');
  const submit = async () => {
    if (await run(() => recordActual(t.project, t.id, delta.trim(), result, note.trim(), lines(evidence)), `${t.id} actual recorded`)) onClose();
  };
  return (
    <Modal open onClose={onClose} title="Record actual" width={480} actions={<>
      <Button variant="ghost" onClick={onClose}>Cancel</Button>
      <Button variant="primary" size="lg" disabled={!result || !delta.trim() || pending} {...gateProps(reason)} onClick={submit}>
        {pending ? 'Recording…' : 'Record'}
      </Button>
    </>}>
      <p className="text-xs mb-3" style={sec}><Id id={t.id} /> · {t.title}{t.target ? ` · ${t.target}` : ''}{t.expected ? ` · expected ${t.expected}` : ''}</p>
      <div className="space-y-3">
        <Field label="Actual delta" hint="What moved, in the metric's own unit.">{(fid, hid) => <input id={fid} aria-describedby={hid} value={delta} onChange={e => setDelta(e.target.value)} style={inputStyle} />}</Field>
        <Field label="Result" hint="Hit = moved at least half of the expected delta (§7).">{fid => (
          <select id={fid} value={result} onChange={e => setResult(e.target.value)} style={inputStyle}>
            <option value="">Choose…</option><option value="hit">hit</option><option value="partial">partial</option><option value="miss">miss</option>
          </select>)}</Field>
        <Field label="Note">{fid => <input id={fid} value={note} onChange={e => setNote(e.target.value)} style={inputStyle} />}</Field>
        <Field label="Evidence refs" hint="One URL or ticket path per line.">{fid => <textarea id={fid} rows={2} value={evidence} onChange={e => setEvidence(e.target.value)} style={inputStyle} />}</Field>
      </div>
      {error && <p role="alert" className="text-xs mt-3" style={{ color: 'var(--status-blocked-text)' }}>{error}</p>}
      <p className="text-xs mt-3" style={muted}>Same writer as <code style={mono}>bbs ticket record-actual</code>, recorded as you. A later record supersedes this one; nothing is erased.</p>
    </Modal>
  );
}

export function DecideModal({ t, onClose }: { t: LoopTarget; onClose: () => void }) {
  const { reason } = useControlPlane();
  const { run, pending, error } = useMutation();
  const [decision, setDecision] = useState<'' | 'go' | 'later' | 'drop'>('');
  const [note, setNote] = useState('');
  const [evidence, setEvidence] = useState('');
  const submit = async () => {
    if (!decision) return;
    if (await run(() => decideTicket(t.project, t.id, decision, note.trim(), lines(evidence)), `${t.id}: ${decision}`)) onClose();
  };
  return (
    <Modal open onClose={onClose} title="Decide" width={480} actions={<>
      <Button variant="ghost" onClick={onClose}>Cancel</Button>
      <Button variant="primary" size="lg" disabled={!decision || pending} {...gateProps(reason)} onClick={submit}>
        {pending ? 'Recording…' : 'Record decision'}
      </Button>
    </>}>
      <p className="text-xs mb-3" style={sec}><Id id={t.id} /> · {t.title}{t.target ? ` · ${t.target}` : ''}</p>
      <div className="space-y-3">
        <Field label="Decision" hint={t.target ? undefined : 'Go needs a target outcome — set it under Planning first.'}>{(fid, hid) => (
          <select id={fid} aria-describedby={hid} value={decision} onChange={e => setDecision(e.target.value as typeof decision)} style={inputStyle}>
            <option value="">Choose…</option><option value="go">go</option><option value="later">later</option><option value="drop">drop</option>
          </select>)}</Field>
        <Field label="Note">{fid => <input id={fid} value={note} onChange={e => setNote(e.target.value)} style={inputStyle} />}</Field>
        <Field label="Evidence refs" hint="One URL or ticket path per line.">{fid => <textarea id={fid} rows={2} value={evidence} onChange={e => setEvidence(e.target.value)} style={inputStyle} />}</Field>
      </div>
      {error && <p role="alert" className="text-xs mt-3" style={{ color: 'var(--status-blocked-text)' }}>{error}</p>}
      <p className="text-xs mt-3" style={muted}>Same writer as <code style={mono}>bbs ticket decide</code>; appends one row to the decision log as you.</p>
    </Modal>
  );
}
