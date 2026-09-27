// FilterContext state, reducer, and serialization helpers.

export interface FilterState {
  project: string;
  status: string[];
  phase: string[];
  label: string[];
  /** Foreman ids. Empty = any assignee. */
  foreman: string[];
  /** Control states: active | paused | cancelled. Empty = the default view,
   *  which hides control-cancelled tickets — hidden, never deleted. */
  control: string[];
}

export type FilterAction =
  | { type: 'replace'; payload: FilterState }
  | { type: 'setProject'; payload: string }
  | { type: 'toggle'; facet: 'status' | 'phase' | 'label' | 'foreman' | 'control'; value: string }
  | { type: 'clear' };

export function initialFilterState(activeProject?: string): FilterState {
  return { project: activeProject ?? 'all', status: [], phase: [], label: [], foreman: [], control: [] };
}
export function normalizeProjectState(state: FilterState, projects: readonly string[]): FilterState {
  const project = state.project === 'all' || projects.includes(state.project)
    ? state.project
    : projects[0] ?? '';
  return project === state.project ? state : { ...state, project };
}

export function filterReducer(state: FilterState, action: FilterAction): FilterState {
  switch (action.type) {
    case 'replace':
      return action.payload;
    case 'setProject':
      // Reset per-view filter chips when switching project
      return { project: action.payload, status: [], phase: [], label: [], foreman: [], control: [] };
    case 'toggle': {
      const arr = state[action.facet];
      const next = arr.includes(action.value)
        ? arr.filter(v => v !== action.value)
        : [...arr, action.value];
      return { ...state, [action.facet]: next };
    }
    case 'clear':
      return { ...state, status: [], phase: [], label: [], foreman: [], control: [] };
    default:
      return state;
  }
}

/** Serialize filter state to a query string (without leading '?'). */
export function serializeFilter(state: FilterState): string {
  const params: string[] = [];
  if (state.project) {
    params.push(`project=${encodeURIComponent(state.project)}`);
  }
  if (state.status.length > 0) {
    params.push(`status=${state.status.map(encodeURIComponent).join(',')}`);
  }
  if (state.phase.length > 0) {
    params.push(`phase=${state.phase.map(encodeURIComponent).join(',')}`);
  }
  if (state.label.length > 0) {
    params.push(`label=${state.label.map(encodeURIComponent).join(',')}`);
  }
  if (state.foreman.length > 0) {
    params.push(`foreman=${state.foreman.map(encodeURIComponent).join(',')}`);
  }
  if (state.control.length > 0) {
    params.push(`control=${state.control.map(encodeURIComponent).join(',')}`);
  }
  return params.join('&');
}

const FILTER_QUERY_KEYS: Record<string, true> = {
  project: true,
  status: true,
  phase: true,
  label: true,
  foreman: true,
  control: true,
};

/** Replace filter-owned query parameters while preserving every other raw parameter. */
export function mergeFilterQuery(query: string, state: FilterState): string {
  const unrelated = query.split('&').filter(part => {
    if (!part) return false;
    const equals = part.indexOf('=');
    const key = equals < 0 ? part : part.slice(0, equals);
    return !Object.hasOwn(FILTER_QUERY_KEYS, key);
  });
  const serialized = serializeFilter(state);
  return [...unrelated, serialized].filter(Boolean).join('&');
}

/** Parse a query string (without leading '?') into FilterState. */
export function parseFilterQuery(query: string, defaultProject = 'all'): FilterState {
  const state = initialFilterState(defaultProject);
  if (!query) return state;
  for (const part of query.split('&')) {
    const eq = part.indexOf('=');
    if (eq < 0) continue;
    const key = part.slice(0, eq);
    const rawVal = part.slice(eq + 1);
    // URL-decode the full value before comma-splitting
    const decoded = decodeURIComponent(rawVal);
    if (key === 'project') {
      state.project = decoded || defaultProject;
    } else if (key === 'status' || key === 'phase' || key === 'label' || key === 'foreman' || key === 'control') {
      state[key] = decoded.split(',').filter(Boolean);
    }
  }
  return state;
}
