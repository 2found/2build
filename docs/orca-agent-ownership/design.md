# Settings ownership notice

User/job: a BBS operator needs to know where to configure agents. Keep the existing Settings route/navigation; replace its editable form with static guidance. No new quota dashboard or duplicate Orca settings UI.

Reuse `TopBar`, `Settings.tsx`'s content wrapper, heading scale and spacing, and the real `web/src/styles.css` tokens from `DESIGN.md`. No new primitives. The isolated prototype imports those assets/components directly and is not wired into production routes.

Copy: **Coding agents are configured in Orca.** “Choose your enabled agents and default agent in Orca. Foreman uses Orca's agent configuration when starting new workers.” Secondary copy: “Existing workers keep their recorded agent and model when resumed.” No unverified Orca deep link.

First-run checklist: preserve its row structure and replace step 1 body with “Configure your agents in Orca, then start Foreman in your project.” Keep its Settings link as “Agent setup” so it opens this guidance, not a dead form. Git/QA/workspace setup stays unchanged.

States: this static notice is identical when Orca is running, unavailable or has no configured agent; no fetch, loading spinner or synthetic error belongs here. Runtime discovery failures appear in existing Foreman run status/evidence. Read-only snapshots retain their existing TopBar badge. Responsive wrapping, existing theme variables and semantic h1/h2 headings apply; no new interactive controls.

Prototype: `web/prototype/orca-agent-settings/index.html`. Start with `npm --prefix web run dev -- --host 127.0.0.1 --port 5199` and open `http://127.0.0.1:5199/prototype/orca-agent-settings/`. `?before=1` renders the current Settings component for comparison; `?theme=dark` selects the existing dark theme.

Verification: token/component inventory and accessibility guidance inspected; existing Settings and proposed notice rendered through Vite. Desktop/light and 390px/mobile/dark screenshots inspected; mobile scroll width equals viewport (390px), no form controls, and no browser errors. Screenshots: `/tmp/bbs-orca-settings-desktop.png`, `/tmp/bbs-orca-settings-mobile.png`. Dev server and browser closed. Production/API integration checks remain implementation work.
