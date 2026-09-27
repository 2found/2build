# Repair 6 — create-intent project normalization

`TicketsList` now uses `useFilter()`'s normalized `state.project` for create-intent validation. The `create` flag is still read from the URL; project resolution has one canonical path through `FilterContext`.

## Verification

- `npm --prefix web run build` — passed.
- Browser fixture: duplicate `project=deleted&project=beta&create=1` opened the modal on beta and showed only beta's ticket; duplicate project values canonicalized to one project, the foreign parameters survived, and `create` was removed.
- Browser fixture: raw `project=a+b` opened the modal on the literal `a+b` project and showed its ticket. An invalid project fell back to `a+b`, with the modal selection and ticket scope agreeing.
- Browser fixture: `project=all` showed all three tickets and did not auto-open the modal; an empty project snapshot also left it closed.
- Read-only snapshot kept the create action disabled. At 390px viewport width, the modal's Create ticket button measured 44px high.
- Screenshot: [repair-6.png](repair-6.png).

The Vite fixture run supplied `window.__BBS_DATA__` through a browser init script; the dev server's absent `data.js` produced one expected failed resource request. No application behavior depended on that request.

## Deviations

None.
