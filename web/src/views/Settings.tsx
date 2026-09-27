import { useCallback, useEffect, useRef, useState } from 'react';
import { Button } from '../components/Button';
import { ErrorBox } from '../components/ErrorBox';
import { Field, inputStyle } from '../components/Field';
import { SectionHeader } from '../components/SectionHeader';
import { TopBar } from '../components/TopBar';
import { useControlPlane, useMutation } from '../contexts/ControlContext';
import { getAgentSettings, saveAgentSettings, type AgentSettings, type AgentSettingsValues } from '../lib/api';

const AGENT_LABELS: Record<string, string> = { claude: 'Claude Code', codex: 'Codex', omp: 'OMP', grok: 'Grok', cursor: 'Cursor' };
const controlStyle = { ...inputStyle, minHeight: 44 };
const secondary = { color: 'var(--text-secondary)' };
const LEGACY_KEYS = [
  'worker_agent',
  'worker_provider',
  'worker_model',
  'worker_effort',
  'foreman_model',
  'foreman_effort',
] as const satisfies readonly (keyof AgentSettingsValues)[];

const frameStyle: React.CSSProperties = {
  border: '1px solid var(--border-hairline)',
  borderRadius: 'var(--radius-md)',
  backgroundColor: 'var(--surface-bg)',
  overflow: 'hidden',
};
const frameHeaderStyle: React.CSSProperties = {
  padding: '10px 12px',
  borderBottom: '1px solid var(--border-emphasis)',
  backgroundColor: 'var(--surface-elevated)',
};
const mutedAction = { color: 'var(--text-muted)', fontSize: 12 };
const ROUTES = [
  { work: 'Implementation, repairs, QA, delivery', behavior: 'Standard route for execution and verification.', role: '@normal' },
  { work: 'Planning, design, code review, diagnosis', behavior: 'Stronger route for judgment-heavy work.', role: '@slow' },
  { work: 'Hard critical planning/review', behavior: 'Maximum route; never selected just because a ticket is large.', role: '@plan' },
] as const;

export function Settings() {
  const { mode, canMutate, reason } = useControlPlane();
  const mutation = useMutation();
  const [data, setData] = useState<AgentSettings | null>(null);
  const [draft, setDraft] = useState<AgentSettingsValues | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [showSaveError, setShowSaveError] = useState(false);
  const [conflict, setConflict] = useState(false);
  const readonly = mode === 'readonly';
  // Last call wins: a rapid Reload → scope-switch must not let the earlier
  // fetch overwrite the newer result.
  const gen = useRef(0);

  const loadSettings = useCallback(() => {
    const g = ++gen.current;
    setData(null);
    setDraft(null);
    setLoading(true);
    setLoadError(null);
    setSaved(false);
    setShowSaveError(false);
    setConflict(false);
    if (readonly) { setLoading(false); return; }
    getAgentSettings().then(result => {
      if (gen.current === g) { setData(result); setDraft({ ...result.values }); }
    }).catch((err: unknown) => {
      if (gen.current === g) setLoadError(err instanceof Error ? err.message : String(err));
    }).finally(() => { if (gen.current === g) setLoading(false); });
  }, [readonly]);

  useEffect(() => {
    loadSettings();
    return () => { gen.current += 1; };
  }, [loadSettings]);

  const dirty = !!data && !!draft && (Object.keys(data.values) as (keyof AgentSettingsValues)[])
    .some(key => draft[key] !== data.values[key]);
  const disabled = !canMutate || loading || mutation.pending || !!loadError;
  const overrides = data ? LEGACY_KEYS.filter(key => data.values[key] !== '') : [];
  const canClear = !!draft && overrides.some(key => draft[key] !== '');
  const pendingClearCount = data && draft
    ? overrides.filter(key => data.values[key] !== '' && draft[key] === '').length
    : 0;

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!data || !draft || disabled || !dirty || conflict) return;
    setSaved(false);
    setShowSaveError(true);
    const ok = await mutation.run(async () => {
      try {
        const result = await saveAgentSettings(data.revision, draft);
        setData({ ...data, ...result });
        setDraft({ ...result.values });
      } catch (err: unknown) {
        if (typeof err === 'object' && err !== null && 'status' in err && err.status === 409) {
          setConflict(true);
        }
        throw err;
      }
    }, 'Saved. New Foreman sessions use this configuration; running sessions stay pinned.');
    setSaved(ok);
  }

  function change(key: 'foreman_agent' | 'foreman_provider', value: string) {
    setDraft(prev => prev ? { ...prev, [key]: value } : prev);
    setSaved(false);
    setShowSaveError(false);
  }

  function clearOverrides() {
    if (!draft) return;
    const next = { ...draft };
    for (const key of overrides) next[key] = '';
    setDraft(next);
    setSaved(false);
    setShowSaveError(false);
  }

  function discard() {
    if (!data) return;
    setDraft({ ...data.values });
    setSaved(false);
    setShowSaveError(false);
  }

  function reload() {
    if (dirty && !window.confirm('Reloading settings will discard your unsaved changes. Continue?')) return;
    loadSettings();
  }

  return <>
    <TopBar title="Settings" />
    <div className="agent-settings space-y-6 max-[767px]:!p-4" style={{
      padding: 'var(--pad-page)',
      maxWidth: 'var(--content-max)',
      width: '100%',
      minWidth: 0,
      fontSize: 13,
    }}>
      <div className="space-y-1">
        <h2 className="font-medium text-sm">Foreman sessions</h2>
        <p style={secondary}>Configure new Foreman sessions. Workers are selected automatically for each task and phase.</p>
      </div>

      {reason && <p role="status" style={secondary}>{reason}</p>}
      <div className="space-y-1">
        <p style={secondary}>All Babysit settings and workspace registrations live in one machine-local file.</p>
        {data && <p className="font-mono break-all" style={secondary}>{data.path}</p>}
      </div>

      {readonly && <div role="status" style={{
        padding: '10px 12px',
        border: '1px solid var(--border-hairline)',
        borderRadius: 'var(--radius-md)',
        backgroundColor: 'var(--surface-elevated)',
        color: 'var(--text-secondary)',
      }}>{reason || 'This saved snapshot cannot change machine settings.'}</div>}
      {loading && <p role="status" style={secondary}>Loading Foreman session settings…</p>}
      {loadError && <div className="space-y-3">
        <ErrorBox title="Could not load Foreman session settings" body={loadError} />
        <Button size="lg" style={{ minHeight: 44 }} disabled={loading || mutation.pending} onClick={loadSettings}>Retry</Button>
      </div>}

      {data && draft && !loading && !loadError && !readonly && <form onSubmit={save} className="space-y-4" aria-busy={mutation.pending}>
        <section style={frameStyle} aria-label="Foreman session settings">
          <div style={frameHeaderStyle}>
            <SectionHeader title="Foreman session" action={<span style={mutedAction}>Applies to new sessions</span>} />
          </div>
          <div className="space-y-4" style={{ padding: 'var(--pad-section)' }}>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <fieldset className="min-w-0" disabled={disabled}>
                <Field label="Agent" hint="Foreman keeps this agent for the life of its session.">{id => <select
                  id={id}
                  name="foreman_agent"
                  style={controlStyle}
                  value={draft.foreman_agent}
                  onChange={e => change('foreman_agent', e.target.value)}
                >
                  <option value="">Automatic detection (default)</option>
                  <option value="auto">Automatic detection</option>
                  {data.agents.map(item => <option key={item.agent} value={item.agent}>
                    {AGENT_LABELS[item.agent] ?? item.agent} · {item.path ? 'installed' : 'not found on PATH'}
                  </option>)}
                  {draft.foreman_agent && draft.foreman_agent !== 'auto' && !data.agents.some(item => item.agent === draft.foreman_agent) &&
                    <option value={draft.foreman_agent}>{draft.foreman_agent} · unrecognized</option>}
                </select>}</Field>
              </fieldset>
              <fieldset className="min-w-0" disabled={disabled}>
                <Field label="Provider" hint="Optional native provider identifier. Authentication stays in your agent.">{id => <input
                  id={id}
                  name="foreman_provider"
                  type="text"
                  className="font-mono"
                  style={controlStyle}
                  value={draft.foreman_provider}
                  maxLength={1024}
                  autoComplete="off"
                  spellCheck={false}
                  onChange={e => change('foreman_provider', e.target.value)}
                />}</Field>
              </fieldset>
            </div>

            {showSaveError && mutation.error && <ErrorBox
              title={conflict ? 'Settings changed elsewhere' : 'Could not save settings'}
              body={conflict
                ? `${mutation.error} Reload settings to discard this draft and resolve the conflict.`
                : mutation.error}
            />}
            {mutation.pending && <p role="status" style={secondary}>Saving settings…</p>}
            <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3" style={{
              borderTop: '1px solid var(--border-hairline)',
              paddingTop: 'var(--pad-section)',
            }}>
              <Button type="submit" variant="primary" size="lg" className="w-full sm:w-auto" style={{ minHeight: 44 }}
                disabled={disabled || !dirty || conflict}>
                {mutation.pending ? 'Saving…' : 'Save for new sessions'}
              </Button>
              <Button size="lg" className="w-full sm:w-auto" style={{ minHeight: 44 }} disabled={disabled || !dirty} onClick={discard}>
                Discard changes
              </Button>
              {saved && <p role="status" style={secondary}>
                Saved. New Foreman sessions use this configuration; running sessions stay pinned.
              </p>}
              {dirty && !mutation.pending && <p role="status" style={secondary}>Unsaved changes</p>}
            </div>
          </div>
        </section>
      </form>}

      {!readonly && data && draft && !loading && !loadError && <div className="flex flex-col sm:flex-row sm:items-center gap-3">
        <Button size="lg" className="w-full sm:w-auto" style={{ minHeight: 44 }} disabled={loading || mutation.pending} onClick={reload}>
          Reload settings
        </Button>
        {dirty && <p style={secondary}>
          {conflict ? 'Reload settings to resolve the conflict; reloading discards your unsaved changes.' : 'Reloading settings discards your unsaved changes.'}
        </p>}
      </div>}

      {data && draft && !loading && !loadError && overrides.length > 0 && <section
        className="space-y-2"
        style={{
          border: '1px solid var(--border-emphasis)',
          borderRadius: 'var(--radius-md)',
          backgroundColor: 'var(--status-blocked-bg)',
          overflow: 'hidden',
        }}
        aria-label="Existing launch overrides"
      >
        <div className="space-y-1" style={{ padding: '14px 12px 10px' }}>
          <h2 className="font-medium text-sm" style={{ color: 'var(--status-blocked-text)' }}>Existing launch overrides need attention</h2>
          <p style={secondary}>These values were configured outside this form. They remain visible so a hidden default cannot change a run.</p>
        </div>
        <div>
          {overrides.map((key, index) => <div key={key} className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] gap-1 sm:gap-3 px-3 py-2"
            style={index > 0 ? { borderTop: '1px solid color-mix(in oklch, var(--status-blocked-text) 20%, transparent)' } : undefined}>
            <code className="font-mono" style={{ overflowWrap: 'anywhere' }}>{key}</code>
            <code className="font-mono" style={{ overflowWrap: 'anywhere' }}>{data.values[key]}</code>
          </div>)}
        </div>
        <div className="space-y-1 px-3 pb-3" style={secondary}>
          <p><code className="font-mono">worker_*</code> may affect direct worker resolution or legacy launch paths; routed Foreman model and effort still come from task and phase.</p>
          <p><code className="font-mono">foreman_model</code> and <code className="font-mono">foreman_effort</code> pin a new Foreman session outside the preferred provider-only path.</p>
        </div>
        <div className="flex flex-col sm:flex-row sm:items-center gap-3 px-3 py-3"
          style={{ borderTop: '1px solid color-mix(in oklch, var(--status-blocked-text) 20%, transparent)' }}>
          <Button size="lg" className="w-full sm:w-auto" style={{ minHeight: 44 }}
            disabled={!canMutate || loading || mutation.pending || !canClear}
            onClick={clearOverrides}
          >Clear listed overrides</Button>
          {pendingClearCount > 0 && <p role="status" style={secondary}>
            {pendingClearCount} {pendingClearCount === 1 ? 'override will' : 'overrides will'} be cleared when you save. Discard changes to restore them.
          </p>}
        </div>
      </section>}

      <section style={frameStyle} aria-label="Automatic worker routing">
        <div style={frameHeaderStyle}>
          <SectionHeader title="Automatic workers" action={<span style={mutedAction}>Task complexity + phase routing</span>} />
        </div>
        <div role="list">
          {ROUTES.map((route, index) => <div key={route.role} role="listitem"
            className="grid grid-cols-1 md:grid-cols-[minmax(160px,0.8fr)_minmax(0,1fr)_96px] gap-2 md:gap-4 p-3 min-w-0"
            style={index > 0 ? { borderTop: '1px solid var(--border-hairline)' } : undefined}>
            <strong>{route.work}</strong>
            <span style={secondary}>{route.behavior}</span>
            <code className="font-mono" style={{ color: 'var(--accent)' }}>{route.role}</code>
          </div>)}
        </div>
        <div className="mx-3 mb-3 p-3" style={{
          borderLeft: '3px solid var(--accent)',
          backgroundColor: 'var(--accent-bg-subtle)',
          color: 'var(--text-secondary)',
        }}>
          Foreman selects a route from task complexity and phase. A running worker stays pinned to its launch route.
          OMP owns role bindings and native authentication. If an OMP role is unavailable, dispatch blocks and names it;
          Babysit never substitutes another model.
        </div>
      </section>
    </div>
  </>;
}

