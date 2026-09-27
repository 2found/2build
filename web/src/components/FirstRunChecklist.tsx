import { Button } from './Button';
import { gateProps } from '../contexts/ControlContext';
import { SectionHeader } from './SectionHeader';

export function FirstRunChecklist({
  onCreateTicket,
  createTicketReason = '',
}: {
  onCreateTicket?: () => void;
  createTicketReason?: string;
}) {
  const hasProjects = onCreateTicket !== undefined;
  const rowClass = 'grid grid-cols-[2rem_minmax(0,1fr)] items-start gap-3 px-4 py-3 md:grid-cols-[2rem_minmax(0,1fr)_auto] md:items-center';
  const actionClass = 'col-start-2 mt-2 w-full md:col-start-auto md:mt-0 md:w-auto';

  return (
    <section
      aria-label="Getting started"
      className="overflow-hidden"
      style={{
        border: '1px solid var(--border-hairline)',
        borderRadius: 'var(--radius-md)',
        backgroundColor: 'var(--surface-bg)',
      }}
    >
      <div
        className="px-3"
        style={{ backgroundColor: 'var(--surface-elevated)', borderBottom: '1px solid var(--border-emphasis)' }}
      >
        <SectionHeader title="Getting started" count={3} />
      </div>
      <ol>
        <li className={rowClass}>
          <span
            aria-hidden="true"
            className="grid h-7 w-7 shrink-0 place-items-center rounded-full border font-mono text-xs"
            style={{ borderColor: 'var(--border-emphasis)', color: 'var(--text-secondary)' }}
          >
            1
          </span>
          <div className="min-w-0">
            <h3 className="mb-0.5 text-[13px] font-medium" style={{ color: 'var(--text-primary)' }}>
              Confirm the Foreman session
            </h3>
            <p className="m-0 text-[13px]" style={{ color: 'var(--text-secondary)' }}>
              Choose the agent/provider once. Authentication stays in that agent.
            </p>
          </div>
          <a
            href="#/settings"
            className={hasProjects
              ? `${actionClass} inline-flex min-h-11 items-center text-sm font-medium underline-offset-2 hover:underline`
              : `${actionClass} inline-flex min-h-11 items-center justify-center border px-3 font-medium no-underline`}
            style={hasProjects
              ? { color: 'var(--accent)' }
              : {
                  backgroundColor: 'var(--accent)',
                  color: 'var(--accent-fg)',
                  borderColor: 'var(--accent)',
                  borderRadius: 'var(--radius-sm)',
                }}
          >
            Open settings
          </a>
        </li>
        <li
          className={rowClass}
          style={{ borderTop: '1px solid var(--border-hairline)' }}
        >
          <span
            aria-hidden="true"
            className="grid h-7 w-7 shrink-0 place-items-center rounded-full border font-mono text-xs"
            style={{ borderColor: 'var(--border-emphasis)', color: 'var(--text-secondary)' }}
          >
            2
          </span>
          <div className="min-w-0">
            <h3 className="mb-0.5 text-[13px] font-medium" style={{ color: 'var(--text-primary)' }}>
              {hasProjects ? 'Create the first ticket' : 'Start the first project'}
            </h3>
            <p className="m-0 text-[13px]" style={{ color: 'var(--text-secondary)' }}>
              {hasProjects
                ? 'Create a ticket here, or invoke Autopilot in the agent for one serial change.'
                : 'Open your agent in a repository and invoke the Foreman skill with the project goal.'}
            </p>
          </div>
          {onCreateTicket && (
            <Button
              variant="primary"
              size="lg"
              className={actionClass}
              onClick={onCreateTicket}
              {...gateProps(createTicketReason)}
            >
              Create first ticket
            </Button>
          )}
        </li>
        <li
          className={rowClass}
          style={{ borderTop: '1px solid var(--border-hairline)' }}
        >
          <span
            aria-hidden="true"
            className="grid h-7 w-7 shrink-0 place-items-center rounded-full border font-mono text-xs"
            style={{ borderColor: 'var(--border-emphasis)', color: 'var(--text-secondary)' }}
          >
            3
          </span>
          <div className="min-w-0">
            <h3 className="mb-0.5 text-[13px] font-medium" style={{ color: 'var(--text-primary)' }}>
              Follow the next action
            </h3>
            <p className="m-0 text-[13px]" style={{ color: 'var(--text-secondary)' }}>
              Babysit shows plan/prototype review, blocked work, QA evidence, and finish status in Tickets and Home.
            </p>
          </div>
        </li>
      </ol>
    </section>
  );
}
