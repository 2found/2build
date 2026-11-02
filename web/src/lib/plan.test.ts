import { describe, expect, it } from 'vitest';
import type { PlanRow, Snapshot, TicketSummary } from './data';
import {
  NO_FACETS, applyFacets, boardColumn, decisionCsv, fmtDay, hoursGrid, milestoneProgress, needsYou, planItems, scoreGrid,
} from './plan';
import { patchOf } from '../views/PlanningPanel';

function row(id: string, over: Partial<PlanRow> = {}): PlanRow {
  return {
    id, title: id, kind: 'work', status: 'backlog', product: 'lohi', role: 'swe', person: '', assignee: '',
    labels: [], parent: '', children: [], milestone: '', phase: '', start: '', due: '', done_at: '', control: '',
    blocked_by: [], ready: false, check_due: '', check_overdue: false, loop: null, updated_at: '', ...over,
  };
}

function snap(projects: Record<string, PlanRow[]>, approvals: Record<string, 'pending'> = {}): Snapshot {
  const out: Record<string, unknown> = {};
  for (const [slug, rows] of Object.entries(projects)) {
    out[slug] = {
      tickets: rows.map(r => ({ id: r.id, title: r.title, status: r.status, planning: r, approval: approvals[r.id] ? { state: 'pending' } : null }) as unknown as TicketSummary),
      ticketDetail: {}, timeline: [], analytics: { rows: [], per_skill: [], per_day: [], outcome: [] },
    };
  }
  return { projects: out } as unknown as Snapshot;
}

// The LoHi shape: one milestone, two phases, work under each.
const M = row('m', { kind: 'milestone', milestone: 'm', start: '2026-11-02', due: '2027-01-31' });
const P1 = row('p1', { kind: 'phase', parent: 'm', milestone: 'm', phase: 'p1', start: '2026-11-02', due: '2026-12-01' });
const P2 = row('p2', { kind: 'phase', parent: 'm', milestone: 'm', phase: 'p2', start: '2026-12-02', due: '2026-12-31' });
const W1 = row('w1', { parent: 'p1', milestone: 'm', phase: 'p1', status: 'done' });
const W2 = row('w2', { parent: 'p1', milestone: 'm', phase: 'p1', role: 'human', ready: true });
const W3 = row('w3', { parent: 'p2', milestone: 'm', phase: 'p2', blocked_by: ['w2'], status: 'blocked' });
const ALL = [M, P1, P2, W1, W2, W3];

describe('planItems', () => {
  it('scopes to one project or merges all, skipping rows without a projection', () => {
    const s = snap({ a: [W1], b: [W2] });
    (s.projects.b.tickets as TicketSummary[]).push({ id: 'old' } as TicketSummary);
    expect(planItems(s, 'a').map(i => i.p.id)).toEqual(['w1']);
    expect(planItems(s, 'all').map(i => [i.project, i.p.id])).toEqual([['a', 'w1'], ['b', 'w2']]);
  });
});

describe('needsYou', () => {
  it('fills each queue from server-derived facts only', () => {
    const undecided = row('o', { loop: { target_outcome: '+4tr' } });
    const decided = row('d', { loop: { target_outcome: '+4tr', decision: { value: 'go', at: null, by: 'x', note: null } } });
    const due = row('c', { status: 'done', check_overdue: true });
    const s = snap({ a: [...ALL, undecided, decided, due] }, { p2: 'pending' });
    const q = needsYou(planItems(s, 'a'));
    expect(q.gates.map(i => i.p.id)).toEqual(['p2']);
    expect(q.undecided.map(i => i.p.id)).toEqual(['o']);
    expect(q.checks.map(i => i.p.id)).toEqual(['c']);
    expect(q.mine.map(i => i.p.id)).toEqual(['w2']);
    expect(q.blocked.map(i => i.p.id)).toEqual(['w3']);
  });

  it('drops settled and cancelled tickets from the work queues', () => {
    const s = snap({ a: [row('x', { role: 'human', ready: true, control: 'cancelled' }), row('y', { status: 'cancelled', loop: { target_outcome: 't' } })] });
    const q = needsYou(planItems(s, 'a'));
    expect(q.mine).toEqual([]);
    expect(q.undecided).toEqual([]);
  });
});

describe('milestoneProgress', () => {
  it('counts work per phase in window order and flags a late phase', () => {
    const pr = milestoneProgress(planItems(snap({ a: ALL }), 'a'), 'm', '2026-12-10')!;
    expect(pr.phases.map(p => [p.item.p.id, p.done, p.total, p.late])).toEqual([['p1', 1, 2, true], ['p2', 0, 1, false]]);
    expect([pr.workDone, pr.workTotal]).toEqual([1, 3]);
    expect(pr.elapsed).toBeCloseTo(38 / 90, 2);
  });

  it('returns null for an unknown id and no elapsed without a window', () => {
    const items = planItems(snap({ a: [row('m2', { kind: 'milestone' })] }), 'a');
    expect(milestoneProgress(items, 'nope', '2026-01-01')).toBeNull();
    expect(milestoneProgress(items, 'm2', '2026-01-01')!.elapsed).toBeNull();
  });
});

describe('facets and board', () => {
  it('ands every set facet', () => {
    const items = planItems(snap({ a: ALL }), 'a').filter(i => i.p.kind === 'work');
    expect(applyFacets(items, { ...NO_FACETS, phase: 'p1', role: 'human' }).map(i => i.p.id)).toEqual(['w2']);
    expect(applyFacets(items, NO_FACETS)).toHaveLength(3);
  });

  it('folds rare statuses into a column', () => {
    expect(['triage', 'decomposed', 'cancelled', 'weird', 'in_review'].map(boardColumn)).toEqual(['backlog', 'in_progress', 'done', 'backlog', 'in_review']);
  });

  it('formats day/month', () => {
    expect(fmtDay('2026-12-02')).toBe('02/12');
    expect(fmtDay('')).toBe('—');
  });
});

describe('company helpers', () => {
  it('grids scores per product and week, null where unrecorded', () => {
    const rows = [{ week: '2026-11-02', product: 'lohi', revenue: 10, active_users: null, support_cases: 3, cost: 2 }];
    const g = scoreGrid(rows, 'lohi', ['2026-10-26', '2026-11-02']);
    expect(g.revenue).toEqual([null, 10]);
    expect(g.active_users).toEqual([null, null]);
  });

  it('grids hours for one week, unknown stages ignored', () => {
    const rows = [
      { week: 'w', product: 'lohi', stage: 'product', hours: 6 },
      { week: 'w', product: '2found', stage: 'soot', hours: 9 },
      { week: 'w', product: 'lohi', stage: 'nope', hours: 1 },
      { week: 'x', product: 'lohi', stage: 'product', hours: 99 },
    ];
    expect(hoursGrid(rows, 'w', ['product', 'soot'])).toEqual([['2found', [0, 9]], ['lohi', [6, 0]]]);
  });

  it('quotes CSV cells the way the CLI log does', () => {
    const csv = decisionCsv([{ at: 'a', note: 'x, "y"', evidence_refs: ['u1', 'u2'], superseded: false }]);
    const [head, line] = csv.trim().split('\n');
    expect(head.split(',')[0]).toBe('at');
    expect(line).toContain('"x, ""y"""');
    expect(line).toContain('u1 u2');
    expect(line.endsWith(',false')).toBe(true);
  });
});

describe('patchOf', () => {
  const base = {
    title: 't', kind: 'work', product: 'lohi', role: 'swe', person: '', labels: 'a, b', start: '', due: '',
    stage: '', target: '', expected: '', checkAfter: '', checkFrom: '',
  };
  it('sends only what changed', () => {
    expect(patchOf(base, base)).toEqual({});
    expect(patchOf(base, { ...base, labels: 'b, c', checkAfter: '7', person: 'long' })).toEqual({
      assigned_to: { role: 'swe', person: 'long' }, labels_add: ['c'], labels_remove: ['a'], loop: { check_after_days: 7 },
    });
  });
});
