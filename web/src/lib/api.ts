// The mutation half of the control plane. Reads live in data.ts; these are the
// POSTs the dashboard makes on the human's behalf, all of them against the
// localhost server that served the page.
//
// Over file:// there is no server, so callers must gate on
// `dataSource() === 'server'` and render controls disabled rather than let a
// click fail — see data.ts.

export interface ApiError extends Error {
  status: number;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  return send<T>('POST', path, body);
}

function get<T>(path: string): Promise<T> {
  return send<T>('GET', path);
}


async function send<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: method === 'GET' || method === 'DELETE' ? undefined : JSON.stringify(body ?? {}),
  });
  // The server answers every failure with {"error": "..."} — a plain status
  // code tells the human nothing about which precondition they missed.
  const payload = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = new Error(payload.error ?? `${res.status} ${res.statusText}`) as ApiError;
    err.status = res.status;
    throw err;
  }
  return payload as T;
}

export interface CreateTicketResult {
  ticket: string;
  home: string;
  requirement: string;
}

/** `title` defaults to the requirement's first line. */
export function createTicket(project: string, requirement: string, title?: string) {
  return post<CreateTicketResult>('/api/tickets', {
    project,
    requirement,
    title: title ?? '',
  });
}

export interface AssignResult {
  ticket: string;
  assignee: string;
  /** How the wake went: sent | unreachable | orca-unavailable | unknown-foreman.
   *  Absent when the call unassigned. Never an error — the assignment is on
   *  disk either way and the foreman picks it up on its next tick. */
  wake?: string;
  wake_detail?: string;
}

/** An empty `foreman` unassigns. */
export function assignTicket(project: string, ticket: string, foreman: string) {
  return post<AssignResult>(`/api/tickets/${project}/${ticket}/assign`, { foreman });
}

export interface StatusResult {
  ticket: string;
  status: string;
}

/** `status` must be one of the ten lifecycle rungs; the server 400s otherwise. */
export function setTicketStatus(project: string, ticket: string, status: string) {
  return post<StatusResult>(`/api/tickets/${project}/${ticket}/status`, { status });
}

export interface DeleteResult {
  deleted: string;
  /** Where the record went — the ticket dir is moved, not unlinked. */
  trash: string;
}

/** Moves the ticket's directory to `~/.babysit/trash/<project>/`. Branches and
 *  worktrees in the repo are untouched. */
export function deleteTicket(project: string, ticket: string) {
  return send<DeleteResult>('DELETE', `/api/tickets/${project}/${ticket}`);
}

export type ControlAction = 'pause' | 'cancel' | 'resume' | 'restore';

export interface ControlResult {
  ticket: string;
  control: string;
  status: string;
}

export function controlTicket(project: string, ticket: string, action: ControlAction, note = '') {
  return post<ControlResult>(`/api/tickets/${project}/${ticket}/control`, { action, note });
}

export type ApprovalAction = 'approve' | 'redirect' | 'drop';

export interface ApprovalResult {
  ticket: string;
  /** approved | redirected | dropped */
  approval: string;
}

/** `redirect` needs a note, or at least one inline comment; the server rejects
 *  a redirect carrying neither with a 400. */
export function resolveApproval(
  project: string,
  ticket: string,
  action: ApprovalAction,
  note = '',
) {
  return post<ApprovalResult>(`/api/tickets/${project}/${ticket}/approval`, { action, note });
}

export interface CommentDraft {
  target: 'requirement' | 'plan' | 'design' | 'prototype';
  anchor?: string;
  excerpt?: string;
  body: string;
}

/** Anchored feedback on one paragraph or one element. Accepted only while the
 *  record is pending; a resolved record answers with a 409. */
export function commentOnApproval(project: string, ticket: string, draft: CommentDraft) {
  return post<{ ticket: string; comment: string }>(
    `/api/tickets/${project}/${ticket}/approval/comment`,
    draft,
  );
}

/** Where `Open in new tab` points. Served mode only — over file:// the frame
 *  falls back to the HTML embedded in the snapshot. */
export function prototypeUrl(project: string, ticket: string) {
  return `/api/tickets/${project}/${ticket}/prototype`;
}

export interface ReadinessResult {
  available: boolean;
  action: 'push' | 'pr' | 'land';
  ready?: boolean;
  reason_codes?: string[];
  reason?: string;
}

/** Reads the CLI evaluator through the served dashboard; it never derives readiness from verdict text. */
export function ticketReadiness(project: string, ticket: string, action: ReadinessResult['action'] = 'pr') {
  return get<ReadinessResult>(`/api/tickets/${project}/${ticket}/readiness?action=${action}`);
}

export interface SpawnResult {
  spawned: string;
  dir: string;
}

export function spawnForeman(dir: string, id = '', command = '') {
  return post<SpawnResult>('/api/foremen', { id, dir, command });
}

export interface RetireResult {
  retired: string;
  /** The server's own line — says so when the Orca terminal was left open. */
  detail: string;
}

export function retireForeman(id: string, keepWorkspace = false) {
  return post<RetireResult>(`/api/foremen/${encodeURIComponent(id)}/retire`, {
    keep_workspace: keepWorkspace,
  });
}

// ─── planning fields and the company loop (bs-w5p3c6mq) ─────────────────────

export interface FieldsPatch {
  title?: string;
  kind?: string;
  product?: string;
  assigned_to?: { role: string; person: string };
  labels_add?: string[];
  labels_remove?: string[];
  window?: { start: string; due: string };
  loop?: {
    stage?: string;
    target_outcome?: string;
    expected_delta?: string;
    check_after_days?: number;
    check_from?: string;
    evidence_add?: string[];
  };
}

/** Same validators and writer as `bbs ticket set-*`; only changed fields log. */
export function setTicketFields(project: string, ticket: string, patch: FieldsPatch) {
  return post<{ ticket: string; changed: string[] }>(`/api/tickets/${project}/${ticket}/fields`, patch);
}

/** `bbs ticket decide` as the person running the dashboard. `go` needs a target outcome. */
export function decideTicket(project: string, ticket: string, decision: 'go' | 'later' | 'drop', note: string, evidence: string[]) {
  return post<{ ticket: string; decision: string }>(`/api/tickets/${project}/${ticket}/decision`, { decision, note, evidence });
}

export function recordActual(project: string, ticket: string, delta: string, result: string, note: string, evidence: string[]) {
  return post<{ ticket: string; result: string }>(`/api/tickets/${project}/${ticket}/actual`, { delta, result, note, evidence });
}

export interface ScoreRow {
  at: string; week: string; product: string;
  revenue: number | null; active_users: number | null; support_cases: number | null;
  cost: number | null; releases: number | null; note?: string; by: string;
}
export interface HoursRow { at: string; week: string; product: string; stage: string; hours: number; note?: string; by: string }
export interface DecisionLogRow {
  at: string; project: string; product: string; stage: string; opportunity_ref: string; title: string;
  decision: string; by: string; note: string; evidence_refs: string[]; target_outcome: string;
  expected_delta: string; check_after_days: number | null; check_due: string; check_overdue: boolean;
  actual_delta: string | null; result: string; superseded: boolean;
}
export interface CompanySummary {
  today: string;
  weeks: string[];
  scoreboard: ScoreRow[];
  hours: HoursRow[];
  stages: string[];
  decisions: DecisionLogRow[];
  hit_rate: { go: number; checked: number; hit: number; partial: number; miss: number; overdue: number };
}

export function fetchCompany(weeks = 8) {
  return get<CompanySummary>(`/api/company?weeks=${weeks}`);
}

export function recordScore(row: { product: string; week: string; revenue?: number; active_users?: number; support_cases?: number; cost?: number; releases?: number; note?: string }) {
  return post<{ product: string; week: string }>('/api/company/scoreboard', row);
}

export function logHours(row: { product: string; stage: string; hours: number; week: string; note?: string }) {
  return post<{ product: string; week: string }>('/api/company/hours', row);
}
