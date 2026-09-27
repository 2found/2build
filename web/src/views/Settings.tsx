import { useCallback, useEffect, useRef, useState } from 'react';
import { Button } from '../components/Button';
import { ErrorBox } from '../components/ErrorBox';
import { Field, inputStyle } from '../components/Field';
import { TopBar } from '../components/TopBar';
import { useControlPlane, useMutation } from '../contexts/ControlContext';
import { getAgentSettings, saveAgentSettings, type AgentSettings, type AgentSettingsValues } from '../lib/api';

const AGENT_LABELS: Record<string, string> = { claude: 'Claude Code', codex: 'Codex', omp: 'OMP', grok: 'Grok', cursor: 'Cursor' };
const controlStyle = { ...inputStyle, minHeight: 44 };
const secondary = { color: 'var(--text-secondary)' };

export function Settings() {
  const { mode, canMutate, reason } = useControlPlane();
  const mutation = useMutation();
  const [data, setData] = useState<AgentSettings | null>(null);
  const [draft, setDraft] = useState<AgentSettingsValues | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [showSaveError, setShowSaveError] = useState(false);
  const readonly = mode === 'readonly';
  // Last call wins: a rapid Reload → scope-switch must not let the earlier
  // fetch overwrite the newer result.
  const gen = useRef(0);

  const loadSettings = useCallback(() => {
    if (readonly) { setLoading(false); return; }
    const g = ++gen.current;
    setLoading(true);
    setLoadError(null);
    setSaved(false);
    setShowSaveError(false);
    getAgentSettings().then(result => {
      if (gen.current === g) { setData(result); setDraft(result.values); }
    }).catch((err: unknown) => {
      if (gen.current === g) setLoadError(err instanceof Error ? err.message : String(err));
    }).finally(() => { if (gen.current === g) setLoading(false); });
  }, [readonly]);

  useEffect(loadSettings, [loadSettings]);

  const dirty = !!data && !!draft && (Object.keys(draft) as (keyof AgentSettingsValues)[]).some(key => draft[key] !== data.values[key]);
  const disabled = !canMutate || loading || mutation.pending || !!loadError;

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!data || !draft || disabled || !dirty) return;
    setSaved(false);
    setShowSaveError(true);
    const ok = await mutation.run(async () => {
      const result = await saveAgentSettings(data.revision, draft);
      setData({ ...data, ...result });
      setDraft(result.values);
    }, 'Agent settings saved. Changes apply to new runs.');
    setSaved(ok);
  }

  function change(key: keyof AgentSettingsValues, value: string) {
    setDraft(prev => prev ? { ...prev, [key]: value } : prev);
    setSaved(false);
  }

  return <>
    <TopBar title="Settings" />
    <div className="agent-settings space-y-6" style={{ padding: 'var(--pad-page)', maxWidth: 'var(--content-max)', fontSize: 13 }}>
      <div className="space-y-2">
        <h2 className="font-medium text-sm">Coding agents</h2>
        <p style={secondary}>Choose how workers and foremen run. Changes apply to new runs; you can edit or clear them later.</p>
      </div>
      {reason && <p role="status" style={secondary}>{reason}</p>}
      <div className="space-y-2">
        <p style={secondary}>All babysit settings and workspace registrations live in one machine-local file.</p>
        {data && <p className="font-mono break-all" style={secondary}>{data.path}</p>}
      </div>
      {loading && <p role="status" style={secondary}>Loading agent settings…</p>}
      {loadError && <ErrorBox title="Could not load settings" body={loadError} />}
      {data && draft && !loading && !loadError && <form onSubmit={save} className="space-y-6" aria-busy={mutation.pending}>
        <p style={secondary}>{data.detected.agent === 'unknown'
          ? 'No active agent detected. Automatic selection uses an installed CLI, falling back to Claude Code.'
          : `Detected agent: ${AGENT_LABELS[data.detected.agent] ?? data.detected.agent}.`}</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {(['worker', 'foreman'] as const).map(role => {
            const title = role === 'worker' ? 'Worker' : 'Foreman';
            const agentKey = `${role}_agent` as const;
            const selected = draft[agentKey];
            return <fieldset key={role} className="space-y-4 min-w-0" disabled={disabled}>
              <legend className="font-medium text-sm mb-4">{title}</legend>
              <Field label="Agent">{id => <select id={id} style={controlStyle} value={selected}
                aria-label={`${title} agent`} onChange={e => change(agentKey, e.target.value)}>
                <option value="">Automatic detection (default)</option>
                <option value="auto">Automatic detection</option>
                {data.agents.map(item => <option key={item.agent} value={item.agent}>{AGENT_LABELS[item.agent] ?? item.agent} · {item.path ? 'installed' : 'not found on PATH'}</option>)}
                {selected && selected !== 'auto' && !data.agents.some(item => item.agent === selected) && <option value={selected}>{selected} · unrecognized</option>}
              </select>}</Field>
              {(['provider', 'model', 'effort'] as const).map(field => {
                const key = `${role}_${field}` as const;
                return <Field key={field} label={field[0].toUpperCase() + field.slice(1)}>{id => <>
                  <input id={id} name={key} aria-label={`${title} ${field}`} aria-describedby={`${id}-hint`} style={controlStyle}
                    className="font-mono" value={draft[key]} maxLength={1024} autoComplete="off" spellCheck={false}
                    onChange={e => change(key, e.target.value)} />
                  <p id={`${id}-hint`} className="mt-1 break-words text-xs" style={secondary}>
                    Blank uses the agent default.
                  </p>
                </>}</Field>;
              })}
            </fieldset>;
          })}
        </div>
        <div className="space-y-2" style={secondary}>
          <p>Use native provider, model and reasoning identifiers. Authentication stays in your agent.</p>
          <p>Priority: command flags → environment → config.yaml → automatic detection / agent defaults.</p>
        </div>
        {showSaveError && mutation.error && <ErrorBox title="Could not save settings" body={mutation.error} />}
        <div className="flex flex-wrap items-center gap-3" style={{ borderTop: '1px solid var(--border-hairline)', paddingTop: 'var(--pad-section)' }}>
          <Button type="submit" variant="primary" size="lg" style={{ minHeight: 44 }} disabled={disabled || !dirty}>{mutation.pending ? 'Saving…' : 'Save settings'}</Button>
          <Button size="lg" style={{ minHeight: 44 }} disabled={disabled || !dirty} onClick={() => { setDraft(data.values); setSaved(false); setShowSaveError(false); }}>Discard changes</Button>
          {saved && <p role="status" style={secondary}>Saved. Changes apply to new runs.</p>}
          {dirty && !mutation.pending && <p style={secondary}>Unsaved changes</p>}
        </div>
      </form>}
      {!readonly && <div className="flex flex-wrap items-center gap-3">
        <Button size="lg" style={{ minHeight: 44 }} disabled={!canMutate || loading || mutation.pending} onClick={loadSettings}>Reload settings</Button>
        {dirty && <p style={secondary}>Reload discards unsaved changes.</p>}
      </div>}
    </div>
  </>;
}
