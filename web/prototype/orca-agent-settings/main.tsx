// Disposable design prototype; not part of production navigation.
import { createRoot } from 'react-dom/client';
import { TopBar } from '../../src/components/TopBar';
import '../../src/styles.css';

const params = new URLSearchParams(location.search);
document.documentElement.dataset.theme = params.get('theme') === 'dark' ? 'dark' : 'light';
createRoot(document.getElementById('root')!).render(<main>
  <TopBar title="Settings" />
  <div className="agent-settings space-y-6" style={{ padding: 'var(--pad-page)', maxWidth: 'var(--content-max)', fontSize: 13 }}>
    <section className="space-y-2" aria-labelledby="agent-heading">
      <h2 id="agent-heading" className="font-medium text-sm">Coding agents are configured in Orca.</h2>
      <p style={{ color: 'var(--text-secondary)' }}>Choose your enabled agents and default agent in Orca. Foreman uses Orca's agent configuration when starting new workers.</p>
      <p style={{ color: 'var(--text-secondary)' }}>Existing workers keep their recorded agent and model when resumed.</p>
    </section>
  </div>
</main>);
