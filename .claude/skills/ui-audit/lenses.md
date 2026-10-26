# ui-audit lenses
Run each lens on every screenshot in LOOK. They list what agents driving a UI
habitually miss; general usability knowledge is assumed. Tag findings with
the lens name.

## visible — is it really on screen?
- Element in the tree but 0-sized, clipped by a parent, behind another
  layer, off-canvas, or the same color as its background.
- Overlays: scrim covers the wrong area, sheet/popup cut by the window
  chrome, two layers open at once, focus inside a hidden layer.
- Text truncated without a way to read it; labels wrapping into the next
  control; numbers overflowing their box; scrollbars inside scrollbars.
- The primary action below the fold with no sign that the page scrolls.

## orient — does the persona know where they are and what to do?
- Can you say in one sentence what this screen is for and what it wants
  from the persona? If not, the screen has no job.
- One clear primary action; secondary actions visibly quieter; destructive
  actions last and separated.
- Title/breadcrumb matches the control that opened it; back/close returns
  to where the persona came from.
- Several entry points or buttons doing nearly the same thing.

## words — would the persona understand every word?
- Developer vocabulary: ids, slugs, file paths, model/tool names, HTTP or
  JSON terms, internal product codenames, config keys.
- Untranslated strings in a localized UI; mixed languages in one sentence.
- Template leaks (`{{name}}`, `undefined`, `NaN`, `[object Object]`),
  placeholder filler, lorem ipsum.
- Label says what the *system* does instead of what the *persona* gets
  ("Validate config" vs "Check before saving").

## state — does the screen tell the truth right now?
- Loading with no indicator; empty state that looks like an error or like
  "nothing to do" when data failed to load.
- Stale items still claiming urgency; counts that disagree with the list
  they summarize or with each other; times without dates or zones.
- Duplicate names the persona cannot tell apart.
- Test/junk records visible to the persona.

## feedback — after acting, is the result obvious?
- Visible response within ~1 s; progress for longer waits.
- Success says what happened and what is next; it does not vanish before
  it can be read.
- A disabled control says why and what would enable it.
- An action that silently needs a second step (save then "apply", create
  then "publish") without saying so.

## errors — calm, specific, recoverable
- Says what happened in persona terms, whether anything was lost, and one
  next step. Not "something went wrong", not a stack/error code alone.
- Tone: alarm colors and words only for real danger; an expected condition
  (service stopped, not configured yet) is information, not an error.
- The message is right about the cause (e.g. "busy" when it is stopped).
- Typed input survives the error.

## effort — how much work is it?
- Steps beyond the ideal path; screens visited only to reach another.
- Re-typing information the app already has; choosing from lists the
  persona cannot evaluate (raw catalogs, technical options without
  defaults).
- Decisions forced before the persona has the information to make them.

## consistency — same thing, same way
- The same concept named differently on two screens; the same action
  styled differently; spacing, radius, type scale or icon style off from the
  rest of the app or the design authority.
- Patterns that differ for no reason (one form auto-saves, the next needs
  Save; one list opens a sheet, another navigates away).

## access — works without the happy hardware
- Keyboard: focus visible, order logical, Escape closes the top layer,
  focus returns to the opener.
- Narrow window: nothing clipped or overlapped, no horizontal scroll.
- Other color scheme: contrast holds, no hardcoded light/dark surfaces.
- Touch-sized targets where the platform needs them.
