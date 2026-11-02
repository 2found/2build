import { useCallback, useEffect, useMemo, useState } from 'react';
import type { Snapshot } from '../lib/data';
import { fetchCompany, logHours, recordScore, type CompanySummary, type DecisionLogRow } from '../lib/api';
import { TopBar } from '../components/TopBar';
import { DenseRow } from '../components/DenseRow';
import { SectionHeader } from '../components/SectionHeader';
import { Button } from '../components/Button';
import { Modal } from '../components/Modal';
import { Tag } from '../components/Tag';
import { Field, inputStyle } from '../components/Field';
import { EmptyState } from '../components/EmptyState';
import { ErrorBox } from '../components/ErrorBox';
import { Facet, Id, RecordActualModal, HC, HS, frame, mono, muted, sec, type LoopTarget } from '../components/PlanParts';
import { gateProps, useControlPlane, useMutation } from '../contexts/ControlContext';
import { SCORE_METRICS, decisionCsv, scoreGrid, fmtDay, hoursGrid, localToday, uniq } from '../lib/plan';

const RESULT_TONE = { hit: 'ok', partial: 'warn', miss: 'err' } as const;
const DECISION_TONE = { go: 'ok', later: 'muted', drop: 'err' } as const;

function num(s: string): number | undefined {
  const t = s.trim().replace(',', '.');
  return t === '' ? undefined : Number(t);
}

function ScoreForm({ products, onClose, onDone }: { products: string[]; onClose: () => void; onDone: () => void }) {
  const { reason } = useControlPlane();
  const { run, pending, error } = useMutation();
  const [v, setV] = useState({ product: products[0] ?? '', week: localToday(), revenue: '', active_users: '', support_cases: '', cost: '', note: '' });
  const bad = SCORE_METRICS.some(([k]) => Number.isNaN(num(v[k]) ?? 0));
  const submit = async () => {
    const ok = await run(() => recordScore({
      product: v.product.trim(), week: v.week, note: v.note.trim() || undefined,
      revenue: num(v.revenue), active_users: num(v.active_users), support_cases: num(v.support_cases), cost: num(v.cost),
    }), `scoreboard ${v.product} recorded`);
    if (ok) { onDone(); onClose(); }
  };
  return (
    <Modal open onClose={onClose} title="Record a week" width={480} actions={<>
      <Button variant="ghost" onClick={onClose}>Cancel</Button>
      <Button variant="primary" size="lg" disabled={!v.product.trim() || bad || pending} {...gateProps(reason)} onClick={submit}>{pending ? 'Recording…' : 'Record'}</Button>
    </>}>
      <div className="grid gap-3" style={{ gridTemplateColumns: '1fr 1fr' }}>
        <Field label="Product">{fid => <input id={fid} list="company-products" value={v.product} onChange={e => setV({ ...v, product: e.target.value })} style={inputStyle} />}</Field>
        <Field label="Week of" hint="Any day; filed under its Monday.">{(fid, hid) => <input id={fid} aria-describedby={hid} type="date" value={v.week} onChange={e => setV({ ...v, week: e.target.value })} style={inputStyle} />}</Field>
        {SCORE_METRICS.map(([k, label]) => (
          <Field key={k} label={label}>{fid => <input id={fid} inputMode="decimal" value={v[k]} onChange={e => setV({ ...v, [k]: e.target.value })} style={inputStyle} />}</Field>
        ))}
      </div>
      <div className="mt-3"><Field label="Note">{fid => <input id={fid} value={v.note} onChange={e => setV({ ...v, note: e.target.value })} style={inputStyle} />}</Field></div>
      <datalist id="company-products">{products.map(p => <option key={p} value={p} />)}</datalist>
      {error && <p role="alert" className="text-xs mt-3" style={{ color: 'var(--status-blocked-text)' }}>{error}</p>}
      <p className="text-xs mt-3" style={muted}>Leave a metric blank to keep it unrecorded. A second record for the same week replaces the first in this view; the file keeps both.</p>
    </Modal>
  );
}

function HoursForm({ products, stages, onClose, onDone }: { products: string[]; stages: string[]; onClose: () => void; onDone: () => void }) {
  const { reason } = useControlPlane();
  const { run, pending, error } = useMutation();
  const [v, setV] = useState({ product: products[0] ?? '', stage: stages[0] ?? '', hours: '', week: localToday(), note: '' });
  const hours = num(v.hours);
  const submit = async () => {
    const ok = await run(() => logHours({ product: v.product.trim(), stage: v.stage, hours: hours ?? 0, week: v.week, note: v.note.trim() || undefined }),
      `hours ${v.product} ${v.stage} logged`);
    if (ok) { onDone(); onClose(); }
  };
  return (
    <Modal open onClose={onClose} title="Log founder hours" width={480} actions={<>
      <Button variant="ghost" onClick={onClose}>Cancel</Button>
      <Button variant="primary" size="lg" disabled={!v.product.trim() || hours === undefined || Number.isNaN(hours) || pending} {...gateProps(reason)} onClick={submit}>{pending ? 'Logging…' : 'Log'}</Button>
    </>}>
      <div className="grid gap-3" style={{ gridTemplateColumns: '1fr 1fr' }}>
        <Field label="Product">{fid => <input id={fid} list="company-products-h" value={v.product} onChange={e => setV({ ...v, product: e.target.value })} style={inputStyle} />}</Field>
        <Field label="Stage">{fid => (
          <select id={fid} value={v.stage} onChange={e => setV({ ...v, stage: e.target.value })} style={inputStyle}>
            {stages.map(s => <option key={s} value={s}>{s === 'soot' ? 'building soot' : s}</option>)}
          </select>)}</Field>
        <Field label="Hours" hint="The week's total for this cell.">{(fid, hid) => <input id={fid} aria-describedby={hid} inputMode="decimal" value={v.hours} onChange={e => setV({ ...v, hours: e.target.value })} style={inputStyle} />}</Field>
        <Field label="Week of">{fid => <input id={fid} type="date" value={v.week} onChange={e => setV({ ...v, week: e.target.value })} style={inputStyle} />}</Field>
      </div>
      <div className="mt-3"><Field label="Note">{fid => <input id={fid} value={v.note} onChange={e => setV({ ...v, note: e.target.value })} style={inputStyle} />}</Field></div>
      <datalist id="company-products-h">{products.map(p => <option key={p} value={p} />)}</datalist>
      {error && <p role="alert" className="text-xs mt-3" style={{ color: 'var(--status-blocked-text)' }}>{error}</p>}
    </Modal>
  );
}

function download(name: string, body: string) {
  const url = URL.createObjectURL(new Blob([body], { type: 'text/csv;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}

function fmtNum(n: number | null) {
  return n === null ? '—' : n.toLocaleString();
}

export function Company({ snapshot }: { snapshot: Snapshot }) {
  const { mode, reason } = useControlPlane();
  const [data, setData] = useState<CompanySummary | null>(null);
  const [err, setErr] = useState('');
  const [tick, setTick] = useState(0);
  const reload = useCallback(() => setTick(t => t + 1), []);
  const [product, setProduct] = useState('');
  const [week, setWeek] = useState('');
  const [form, setForm] = useState<'' | 'score' | 'hours'>('');
  const [recording, setRecording] = useState<LoopTarget | null>(null);
  const [showSuperseded, setShowSuperseded] = useState(false);

  useEffect(() => {
    if (mode === 'readonly') return;
    let live = true;
    fetchCompany(8).then(d => { if (live) { setData(d); setErr(''); } }, e => { if (live) setErr(e instanceof Error ? e.message : String(e)); });
    return () => { live = false; };
    // The snapshot poll is the clock: a write elsewhere shows up on the next tick.
  }, [mode, tick, snapshot]);

  const products = useMemo(() => uniq([
    ...(data?.scoreboard ?? []).map(r => r.product), ...(data?.hours ?? []).map(r => r.product),
    ...(data?.decisions ?? []).map(r => r.product),
  ]), [data]);
  const shownProduct = product || products[0] || '';
  const weeks = useMemo(() => [...(data?.weeks ?? [])].reverse(), [data]); // oldest → newest, left → right
  const hoursWeek = week || data?.weeks[0] || '';

  if (mode === 'readonly') {
    return (
      <>
        <TopBar title="Company loop" warnings={snapshot.meta.warnings} />
        <div className="px-6 py-4"><EmptyState title="Served mode only" body="The scoreboard, founder hours and decision log are read live from ~/.babysit/company. Start `bbs dashboard`, or read them with `bbs company show`." /></div>
      </>
    );
  }

  const log: DecisionLogRow[] = (data?.decisions ?? []).filter(r => showSuperseded || !r.superseded);
  const hr = data?.hit_rate;
  const grid = data ? scoreGrid(data.scoreboard, shownProduct, weeks) : null;
  const hours = data ? hoursGrid(data.hours, hoursWeek, data.stages) : [];
  const stageLabel = (s: string) => (s === 'soot' ? 'building soot' : s);
  const scoreCols = `150px repeat(${Math.max(weeks.length, 1)}, minmax(72px, 1fr))`;
  const hoursCols = `110px repeat(${data?.stages.length ?? 5}, minmax(80px, 1fr)) 64px`;
  const logCols = '84px 112px minmax(160px,1fr) 70px 70px minmax(180px,1fr)';

  return (
    <>
      <TopBar title="Company loop" warnings={snapshot.meta.warnings} actions={
        <Button size="sm" variant="secondary" disabled={!data} onClick={() => data && download(`decision-log-${data.today}.csv`, decisionCsv(log as unknown as Record<string, unknown>[]))}>Export CSV</Button>
      } />
      <div className="space-y-8 px-6 py-4">
        {err && <ErrorBox title="Company data unavailable" body={err} />}
        {!data && !err && <p className="text-sm" style={muted}>Loading…</p>}
        {data && (<>
          <SectionHeader title={`Scoreboard · ${shownProduct || 'no product yet'} · weekly`} action={
            <div className="flex items-center gap-3">
              {products.length > 1 && <Facet label="Product" value={product} options={products.map(p => [p, p])} onChange={setProduct} />}
              <Button size="sm" variant="ghost" {...gateProps(reason)} onClick={() => setForm('score')}>Record week</Button>
            </div>}>
            {!data.scoreboard.length ? <p className="text-xs mt-1" style={muted}>No weeks recorded. <code style={mono}>bbs company scoreboard record --product P --revenue N …</code></p> : (
              <div className="mt-1 overflow-x-auto" style={frame}>
                <DenseRow columns={scoreCols} header>
                  <span className={HC} style={HS}>Metric</span>
                  {weeks.map(w => <span key={w} className={HC} style={{ ...HS, ...mono }}>{fmtDay(w)}</span>)}
                </DenseRow>
                {SCORE_METRICS.map(([k, label]) => (
                  <DenseRow key={k} columns={scoreCols}>
                    <span className="px-3 text-xs" style={sec}>{label}</span>
                    {grid![k].map((n, i) => <span key={i} className="px-3 text-sm" style={{ fontVariantNumeric: 'tabular-nums', ...(n === null ? muted : {}) }}>{fmtNum(n)}</span>)}
                  </DenseRow>
                ))}
              </div>
            )}
          </SectionHeader>

          <SectionHeader title="Founder hours · product × stage" action={
            <div className="flex items-center gap-3">
              <Facet label="Week" value={week} options={data.weeks.map(w => [w, `week of ${fmtDay(w)}`])} onChange={setWeek} />
              <Button size="sm" variant="ghost" {...gateProps(reason)} onClick={() => setForm('hours')}>Log hours</Button>
            </div>}>
            {!hours.length ? <p className="text-xs mt-1" style={muted}>No hours logged for the week of {fmtDay(hoursWeek)}.</p> : (
              <div className="mt-1 overflow-x-auto" style={frame}>
                <DenseRow columns={hoursCols} header>
                  <span className={HC} style={HS}>Product</span>
                  {data.stages.map(s => <span key={s} className={HC} style={HS}>{stageLabel(s)}</span>)}
                  <span className={HC} style={HS}>Total</span>
                </DenseRow>
                {hours.map(([p, h]) => (
                  <DenseRow key={p} columns={hoursCols}>
                    <span className="px-3 text-xs" style={mono}>{p}</span>
                    {h.map((n, i) => <span key={i} className="px-3 text-sm" style={n ? undefined : muted}>{n || '—'}</span>)}
                    <span className="px-3 text-sm font-medium">{h.reduce((a, b) => a + b, 0)}</span>
                  </DenseRow>
                ))}
              </div>
            )}
            <p className="text-xs mt-1" style={muted}>"Building soot" is reported apart and never netted against savings (§7).</p>
          </SectionHeader>

          <SectionHeader title="Decision log" count={log.length} action={
            <label className="text-xs flex items-center gap-1" style={sec}>
              <input type="checkbox" checked={showSuperseded} onChange={e => setShowSuperseded(e.target.checked)} /> show superseded
            </label>}>
            {hr && <p className="text-xs mt-1" style={sec}>
              Go decisions {hr.go} · checked {hr.checked}: {hr.hit} hit · {hr.partial} partial · {hr.miss} miss
              {hr.checked ? ` · hit rate ${Math.round((100 * hr.hit) / hr.checked)}%` : ''}
              {hr.overdue ? <span style={{ color: 'var(--status-blocked-text)' }}> · {hr.overdue} check{hr.overdue > 1 ? 's' : ''} overdue</span> : null}
            </p>}
            {!log.length ? <p className="text-xs mt-1" style={muted}>No decisions recorded. <code style={mono}>bbs ticket decide go --note …</code></p> : (
              <div className="mt-1 overflow-x-auto" style={frame}>
                <DenseRow columns={logCols} header>
                  <span className={HC} style={HS}>At</span><span className={HC} style={HS}>Ticket</span><span className={HC} style={HS}>Target · expected</span>
                  <span className={HC} style={HS}>Decision</span><span className={HC} style={HS}>Check</span><span className={HC} style={HS}>Actual</span>
                </DenseRow>
                {log.map((r, i) => (
                  <DenseRow key={`${r.opportunity_ref}-${r.at}-${i}`} columns={logCols}>
                    <span className="px-3 text-xs" style={sec} title={r.at}>{fmtDay(r.at)}</span>
                    <span className="px-3 min-w-0 truncate" title={r.title}><Id id={r.opportunity_ref} /></span>
                    <span className="px-3 text-xs truncate" title={`${r.title}\n${r.target_outcome} · ${r.expected_delta}`} style={r.superseded ? muted : undefined}>
                      {r.target_outcome || r.title} {r.expected_delta && <span style={muted}>· {r.expected_delta}</span>}
                    </span>
                    <span className="px-3 flex items-center"><Tag tone={DECISION_TONE[r.decision as keyof typeof DECISION_TONE] ?? 'muted'}>{r.superseded ? `${r.decision} (old)` : r.decision}</Tag></span>
                    <span className="px-3 text-xs" style={r.check_overdue ? { color: 'var(--status-blocked-text)' } : sec}>{r.check_due ? fmtDay(r.check_due) : r.check_after_days ? `${r.check_after_days}d` : '—'}</span>
                    <span className="px-3 text-xs truncate flex items-center gap-1">
                      {r.result ? <><Tag tone={RESULT_TONE[r.result as keyof typeof RESULT_TONE] ?? 'muted'}>{r.result}</Tag> <span style={sec} title={r.actual_delta ?? ''}>{r.actual_delta}</span></>
                        : r.decision === 'go' && !r.superseded ? (
                          <Button size="sm" variant="ghost" {...gateProps(reason)} onClick={() => setRecording({ project: r.project, id: r.opportunity_ref, title: r.title, target: r.target_outcome, expected: r.expected_delta })}>
                            Record actual{r.check_due ? ` · due ${fmtDay(r.check_due)}` : ''}
                          </Button>
                        ) : <span style={muted}>—</span>}
                    </span>
                  </DenseRow>
                ))}
              </div>
            )}
            <p className="text-xs mt-1" style={muted}>Every row is a ticket history event (<code style={mono}>bbs company log</code>); nothing here edits a file by hand.</p>
          </SectionHeader>
        </>)}
      </div>
      {form === 'score' && <ScoreForm products={products} onClose={() => setForm('')} onDone={reload} />}
      {form === 'hours' && data && <HoursForm products={products} stages={data.stages} onClose={() => setForm('')} onDone={reload} />}
      {recording && <RecordActualModal t={recording} onClose={() => { setRecording(null); reload(); }} />}
    </>
  );
}
