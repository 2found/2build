import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  type Dispatch,
  type ReactNode,
} from 'react';
import type { Snapshot } from '../lib/data';
import {
  filterReducer,
  initialFilterState,
  mergeFilterQuery,
  parseFilterQuery,
  serializeFilter,
  type FilterAction,
  type FilterState,
} from '../lib/filter';
import { parseHash, replaceHashQuery } from '../lib/hash';

interface FilterContextValue {
  state: FilterState;
  dispatch: Dispatch<FilterAction>;
}

const FilterContext = createContext<FilterContextValue | null>(null);

function normalizeProjectState(state: FilterState, projects: readonly string[]): FilterState {
  const project = state.project === 'all' || projects.includes(state.project)
    ? state.project
    : projects[0] ?? '';
  return project === state.project ? state : { ...state, project };
}

function deriveInitialState(
  activeProject: string | null | undefined,
  projects: readonly string[],
): FilterState {
  const { query } = parseHash(window.location.hash);
  const state = query
    ? parseFilterQuery(query, activeProject ?? 'all')
    : initialFilterState(activeProject ?? 'all');
  return normalizeProjectState(state, projects);
}

export function FilterProvider({
  children,
  activeProject,
  projects,
}: {
  children: ReactNode;
  activeProject?: string | null;
  projects: Snapshot['projects'];
}) {
  const projectSlugs = useMemo(() => Object.keys(projects).sort(), [projects]);
  const [storedState, reduce] = useReducer(
    filterReducer,
    undefined,
    () => deriveInitialState(activeProject, projectSlugs),
  );
  const state = useMemo(
    () => normalizeProjectState(storedState, projectSlugs),
    [storedState, projectSlugs],
  );
  const dispatch = useCallback<Dispatch<FilterAction>>((action) => {
    const next = normalizeProjectState(filterReducer(state, action), projectSlugs);
    reduce({ type: 'replace', payload: next });
  }, [state, projectSlugs]);

  // Keep state and URL canonical when snapshot projects change or filters update.
  useEffect(() => {
    if (state.project !== storedState.project) {
      reduce({ type: 'replace', payload: state });
    }
    const { query } = parseHash(window.location.hash);
    replaceHashQuery(mergeFilterQuery(query, state));
  }, [state, storedState]);

  // Sync from hash (e.g., user pastes deep-link URL).
  // Empty-query nav (in-app ticket link clicks, sidebar nav without project=,
  // window.location.hash assignments) preserves current filter state and
  // re-writes the URL to include it. Without this, parseFilterQuery('', default)
  // would reset state to meta.active_project on every empty-query transition,
  // making the active project auto-flip whenever any link omits ?project=.
  // Explicit ?project=all still clears (parsed as project='all'); deep-links
  // with empty query intentionally inherit the current session's project.
  useEffect(() => {
    const onHashChange = () => {
      const { query } = parseHash(window.location.hash);
      if (!query) {
        const serialized = serializeFilter(state);
        if (serialized) replaceHashQuery(serialized);
        return;
      }
      const incoming = normalizeProjectState(
        parseFilterQuery(query, activeProject ?? 'all'),
        projectSlugs,
      );
      const serialized = serializeFilter(state);
      const incomingSerial = serializeFilter(incoming);
      if (incomingSerial === serialized) {
        const canonicalQuery = mergeFilterQuery(query, incoming);
        if (query !== canonicalQuery) replaceHashQuery(canonicalQuery);
        return;
      }
      dispatch({ type: 'replace', payload: incoming });
    };
    window.addEventListener('hashchange', onHashChange);
    return () => window.removeEventListener('hashchange', onHashChange);
  }, [state, activeProject, projectSlugs, dispatch]);
  return (
    <FilterContext.Provider value={{ state, dispatch }}>
      {children}
    </FilterContext.Provider>
  );
}

export function useFilter(): FilterContextValue {
  const ctx = useContext(FilterContext);
  if (!ctx) throw new Error('useFilter must be used inside <FilterProvider>');
  return ctx;
}

// Non-throwing variant for components rendered both inside and outside the
// provider (e.g. Layout in the no-snapshot fallback path).
export function useFilterOptional(): FilterContextValue | null {
  return useContext(FilterContext);
}
