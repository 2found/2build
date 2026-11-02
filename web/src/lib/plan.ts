// Planning views over the snapshot (bs-w5p3c6mq). The server derives every
// planning fact — placement, effective blockers, readiness, check dates — in
// internal/ticket/plan.go; these helpers only select and group rows. Nothing
// here re-derives a blocker or a wave, so the board and `bbs ticket find`
// cannot disagree.

import type { PlanRow, Snapshot, TicketSummary } from './data';

export interface PlanItem {
  project: string;
  t: TicketSummary;
  p: PlanRow;
}

export function planItems(snapshot: Snapshot, project: string): PlanItem[] {
  const out: PlanItem[] = [];
  for (const [slug, block] of Object.entries(snapshot.projects)) {
    if (project !== 'all' && slug !== project) continue;
    for (const t of block.tickets) if (t.planning) out.push({ project: slug, t, p: t.planning });
  }
  return out;
}

export const SETTLED = new Set(['done', 'cancelled', 'duplicate']);

export function localToday(now = new Date()): string {
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, '0');
  const d = String(now.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

/** 2026-12-02 → 02/12 (the founder's day/month reading order). */
export function fmtDay(d?: string | null): string {
  return d && d.length >= 10 ? `${d.slice(8, 10)}/${d.slice(5, 7)}` : '—';
}

export function fmtWindow(start?: string, due?: string): string {
  if (!start && !due) return '—';
  return `${fmtDay(start)} – ${fmtDay(due)}${due ? ' ' + due.slice(0, 4) : ''}`;
}

export interface Facets {
  product: string;
  milestone: string;
  phase: string;
  role: string;
  status: string;
  label: string;
}

export const NO_FACETS: Facets = { product: '', milestone: '', phase: '', role: '', status: '', label: '' };

export function applyFacets(items: PlanItem[], f: Facets): PlanItem[] {
  return items.filter(({ p }) =>
    (!f.product || p.product === f.product) &&
    (!f.milestone || p.milestone === f.milestone) &&
    (!f.phase || p.phase === f.phase) &&
    (!f.role || p.role === f.role) &&
    (!f.status || p.status === f.status) &&
    (!f.label || p.labels.includes(f.label)));
}

export function uniq(vals: string[]): string[] {
  return [...new Set(vals.filter(Boolean))].sort();
}

export interface NeedsYou {
  /** A design checkpoint an agent is blocked on. */
  gates: PlanItem[];
  /** An opportunity with a target outcome and no decision yet. */
  undecided: PlanItem[];
  /** check_after_days passed with no actual recorded. */
  checks: PlanItem[];
  /** Human-role work that can start now. */
  mine: PlanItem[];
  blocked: PlanItem[];
}

export function needsYou(items: PlanItem[]): NeedsYou {
  const open = items.filter(i => !SETTLED.has(i.p.status) && i.p.control !== 'cancelled');
  return {
    gates: items.filter(i => i.t.approval?.state === 'pending'),
    undecided: open.filter(i => !!i.p.loop?.target_outcome && !i.p.loop?.decision),
    checks: items.filter(i => i.p.check_overdue),
    mine: open.filter(i => i.p.kind === 'work' && i.p.role === 'human' && i.p.ready),
    blocked: open.filter(i => i.p.status === 'blocked'),
  };
}

export interface PhaseProgress {
  item: PlanItem;
  done: number;
  total: number;
  late: boolean;
}

export interface MilestoneProgress {
  milestone: PlanItem;
  phases: PhaseProgress[];
  workDone: number;
  workTotal: number;
  /** 0..1 of the window elapsed; null without a window. */
  elapsed: number | null;
}

function countDone(rows: PlanItem[]) {
  return rows.filter(r => r.p.status === 'done').length;
}

export function milestoneProgress(items: PlanItem[], id: string, today: string): MilestoneProgress | null {
  const milestone = items.find(i => i.p.id === id);
  if (!milestone) return null;
  const under = items.filter(i => i.p.milestone === id && i.p.id !== id);
  const work = under.filter(i => i.p.kind === 'work' && i.p.status !== 'cancelled');
  const phases = under
    .filter(i => i.p.kind === 'phase')
    .sort((a, b) => (a.p.start || a.p.title).localeCompare(b.p.start || b.p.title))
    .map(item => {
      const kids = work.filter(w => w.p.phase === item.p.id);
      return {
        item,
        done: countDone(kids),
        total: kids.length,
        late: !SETTLED.has(item.p.status) && !!item.p.due && item.p.due < today && countDone(kids) < kids.length,
      };
    });
  const { start, due } = milestone.p;
  let elapsed: number | null = null;
  if (start && due && due > start) {
    elapsed = Math.min(1, Math.max(0, (Date.parse(today) - Date.parse(start)) / (Date.parse(due) - Date.parse(start))));
  }
  return { milestone, phases, workDone: countDone(work), workTotal: work.length, elapsed };
}

/** Board columns: triage folds into backlog, the rarer rungs into their nearest. */
export const BOARD_COLUMNS = ['backlog', 'planned', 'in_progress', 'in_review', 'blocked', 'done'] as const;

export function boardColumn(status: string): (typeof BOARD_COLUMNS)[number] {
  switch (status) {
    case 'triage':
      return 'backlog';
    case 'decomposed':
      return 'in_progress';
    case 'cancelled':
    case 'duplicate':
      return 'done';
  }
  return (BOARD_COLUMNS as readonly string[]).includes(status) ? (status as (typeof BOARD_COLUMNS)[number]) : 'backlog';
}

export function ticketHref(id: string): string {
  return `#/tickets/${encodeURIComponent(id)}`;
}

// ─── Company loop (§7 baseline, CL4.3 decision log) ─────────────────────────

export const SCORE_METRICS = [
  ['revenue', 'Revenue'],
  ['active_users', 'Active users'],
  ['support_cases', 'Support cases'],
  ['cost', 'Hosting + AI cost'],
] as const;

type ScoreKey = (typeof SCORE_METRICS)[number][0];
type ScoreLike = { week: string; product: string } & { [K in ScoreKey]: number | null };

/** metric → value per week (in `weeks` order) for one product; null = not recorded. */
export function scoreGrid(rows: ScoreLike[], product: string, weeks: string[]): Record<ScoreKey, (number | null)[]> {
  const at = new Map(rows.filter(r => r.product === product).map(r => [r.week, r]));
  const out = {} as Record<ScoreKey, (number | null)[]>;
  for (const [k] of SCORE_METRICS) out[k] = weeks.map(w => at.get(w)?.[k] ?? null);
  return out;
}

/** product → hours per stage (in `stages` order) for one week. */
export function hoursGrid(rows: { week: string; product: string; stage: string; hours: number }[], week: string, stages: string[]) {
  const out = new Map<string, number[]>();
  for (const r of rows) {
    if (r.week !== week) continue;
    const i = stages.indexOf(r.stage);
    if (i < 0) continue;
    if (!out.has(r.product)) out.set(r.product, stages.map(() => 0));
    out.get(r.product)![i] += r.hours;
  }
  return [...out.entries()].sort(([a], [b]) => a.localeCompare(b));
}

const CSV_COLS = [
  'at', 'stage', 'opportunity_ref', 'decision', 'note', 'evidence_refs', 'target_outcome',
  'expected_delta', 'check_after_days', 'actual_delta', 'result', 'product', 'project', 'title', 'by', 'superseded',
] as const;

function csvCell(v: unknown): string {
  const s = Array.isArray(v) ? v.join(' ') : v == null ? '' : String(v);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** The CL4.3 log as CSV — the same columns `bbs company log --csv` prints. */
export function decisionCsv(rows: Record<string, unknown>[]): string {
  return [CSV_COLS.join(','), ...rows.map(r => CSV_COLS.map(c => csvCell(r[c])).join(','))].join('\n') + '\n';
}
