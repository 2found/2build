# Repair 1 — Settings section headings

Replaced the two non-collapsible `SectionHeader` uses in `web/src/views/Settings.tsx` with static level-3 headings. The uppercase 11px muted caption, frame spacing, and right-side action text remain visually aligned with the existing headers; the shared `SectionHeader` component is unchanged.

Verification:
- `npm --prefix web run build` — passed.
- Fixture-backed browser smoke check of `#/settings` at 1440×900 and 390×844 — both sections rendered, each header contained zero disabled buttons, and the accessibility tree exposed “Foreman session” and “Automatic workers” as level-3 headings.