import { TopBar } from '../components/TopBar';

const secondary = { color: 'var(--text-secondary)' };

export function Settings() {
  return <>
    <TopBar title="Settings" />
    <div className="space-y-6" style={{ padding: 'var(--pad-page)', maxWidth: 'var(--content-max)', fontSize: 13 }}>
      <section className="space-y-2" aria-labelledby="agent-heading">
        <h2 id="agent-heading" className="font-medium text-sm">Coding agents are configured in Orca.</h2>
        <p style={secondary}>Choose your enabled agents and default agent in Orca. Foreman uses Orca's agent configuration when starting new workers.</p>
        <p style={secondary}>Existing workers keep their recorded agent and model when resumed.</p>
      </section>
    </div>
  </>;
}
