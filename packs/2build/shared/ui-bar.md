# UI bar

What a product's UI meets, whoever builds it. `design-ui` designs to it,
`implement` builds to it, `qa` checks the changed screens against it and
`ui-audit` measures the whole product. Rows carry ids so a design note, a QA
case and an audit finding cite the same rule. The project's design authority
may set its own number for any row; where it is silent these defaults hold.
A row is proven by a count, a measurement or a screenshot, never by "looks
fine" and never by reading code, except that a counted source scan proves
C1 and C3–C5. A row a change touches and does not meet is named in
the design note or handoff, never left silent.

The bar is about use, not look: visual direction stays with the brief and
the design authority. Accessibility (WCAG 2.2 AA) and the agent path
([agent-operable](agent-operable.md)) are separate bars, not restated here.

## Jobs: the user finishes what they came for
| id | Bar | Proof |
|---|---|---|
| J1 | Design starts from the user's jobs (trigger → done) in the flow inventory. Every screen, region (a visually separate block: header, tile row, list, side panel) and control serves at least one. | region → flow id; a region no job uses is cut |
| J2 | A job finishes where it starts: no leaving for another screen to fetch, copy or remember what this one needs. | actions ÷ ideal ≤ 1.5; screen switches beyond the ideal path 0; values retyped or carried in the head 0 |
| J3 | Every state an object can reach (the backend's own enums and lifecycle) has a visible form and a way out: retry, edit, cancel, undo, delete. | lifecycle matrix: states with no screen 0, with no exit 0 |
| J4 | Each screen shows the next step, and a finished action shows its result where the user is looking. | per step: next step visible without scrolling or guessing |

## One owner: nothing lives in two places
| id | Bar | Proof |
|---|---|---|
| O1 | A fact (number, list, status) and an action each have one owner screen. Elsewhere at most a one-line summary that links to the owner, never a second copy with its own controls. | ownership matrix: duplicates 0 |
| O2 | No two navigation entries, tabs or screens answer the same question. A new surface names what it replaces; "nothing" needs a job no existing screen can own. | screen pairs sharing most of their content 0 |
| O3 | One thing has one name, one icon and one number everywhere. | terms per concept 1; contradicting values 0 |

## Simple: the screen reads at a glance
| id | Bar | Proof |
|---|---|---|
| S1 | One primary action per view; the rest are quiet. | primary-styled controls per view ≤ 1 |
| S2 | A stranger shown only the screenshot names what the screen is for and what to do first. | cold read by a fresh rater matches the intended purpose and first action |
| S3 | Few choices at once: top-level navigation ≤ 7, tabs on a page ≤ 5, visible row actions ≤ 2 plus overflow, depth ≤ 2 from a flow's start. Rare and advanced options sit behind a disclosure; a default replaces a required decision. | counts |
| S4 | The work surface (what the user edits or decides on) owns the first viewport; chrome, summaries and help do not push it below the fold. | work surface starts in the top 40% of the first viewport, at the common and the narrowest width |

## Scannable: layout and icons before prose
| id | Bar | Proof |
|---|---|---|
| T1 | A row is a title plus at most one meta line. No helper paragraph over one sentence; explanation lives in one ⓘ popover or guide link per group. | rows over 2 lines 0; helper paragraphs 0 |
| T2 | A count or status is an icon plus a number or word, aligned in a shared column, with the full sentence as its accessible name and tooltip. | counts written as sentences 0 |
| T3 | Nothing is said twice on a screen, reassurance repeated per card included. | repeated facts 0 |
| T4 | Never cut: an error with its recovery, a blocker, destructive-confirm wording, form labels and validation, legal copy, the word beside a colour dot, a price, or the only text saying what a write or charge will do. | over-cutting fails too |

## One system: same job, same component, same geometry
| id | Bar | Proof |
|---|---|---|
| C1 | One idiom per job. Page shell and header, list row, card, form footer, dialog or sheet, loading, empty, error and partial failure each come from one shared component named in the design authority. The list is open: any job two screens both do (a stat tile, a banner) joins it. A second idiom is drift even when built from legal primitives. | idioms per job, counted in source and on screen: 1 |
| C2 | Loading is a skeleton shaped like the content that replaces it, so nothing jumps when data lands; a spinner belongs only inside the control that started an action. Loading, empty and error look and behave the same on every screen. | state matrix, screens × states |
| C3 | Each geometry role has one value, on the project's scale, across screens: page gutter, header height, section gap, card padding, row height, title size, radius. | computed styles per role: 1 value |
| C4 | A feature composes shared components; it never redraws, restyles or locally copies one. A new shared component, variant or token is a design decision recorded in the design authority. | local look-alikes 0; literal or off-scale values 0 |
| C5 | A drift class seen twice becomes a rule in the project's mechanical UI check, so it cannot return. | the rule fails on a planted case |

## Fast: measured on the build users run
| id | Bar | Proof |
|---|---|---|
| P1 | A screen shows usable content within 1 s warm and 2.5 s cold (LCP); anything slower shows its skeleton within 100 ms. | measured with production-shaped data |
| P2 | An action gives feedback within 100 ms and finishes or shows progress within 1 s (INP ≤ 200 ms); past 10 s it shows progress and can be cancelled or left. | measured per main action |
| P3 | Nothing moves after first paint (CLS ≤ 0.1): no content jumping as data, images or banners arrive. | measured; frames of a cold open |
| P4 | Opening a screen fires no duplicate request, no unbounded list fetch and no request whose result it never shows. Long lists paginate or virtualize. | request log of one open |
| P5 | Returning to a screen already seen shows its last content at once and refreshes in place; it does not start over from a skeleton. | back-navigation frames |

## Real data: what production holds, not the fixture
| id | Bar | Proof |
|---|---|---|
| D1 | Designed and tested with production-shaped data: the longest real list, a missing or null value, an 80-character name, a large number. | one shot per shape |
| D2 | Numbers read as people say them (unit, grouping, locale). A missing value is "–", never a measured-looking 0 and never a crash. Long text truncates with its full value reachable. | shots of the null and large-number cases |
| D3 | A long list opens collapsed, paged or filtered to what the job needs. | shot at production volume |
