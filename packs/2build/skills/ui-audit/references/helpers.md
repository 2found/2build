# Driver helpers (agent-browser)
Set up the session, stealth and cleanup per the `browse` skill (read it; do not
duplicate). These wrappers sit on top. Put them in the scratchpad, not the repo.
Refs (`@eN`) are per snapshot: re-snapshot before each action.

## ab: pinned session
```bash
#!/bin/bash
exec agent-browser --session "${AUDIT_SESSION:?set to the browse session}" "$@"
```

## tab.sh: click by accessible text, screenshot
```bash
#!/bin/bash
# tab.sh <shot-name> <label>
./ab find text "$2" click >/dev/null 2>&1; sleep 2
./ab screenshot "$OUT/shots/$1.png" >/dev/null
```

## geo.sh: computed geometry per role (Map § Geometry)
Selectors are yours: read them from the snapshot or source, one per role, and
keep the same map for every screen. Collect one JSON line per screen, then
table role × screen; a role with more than one value is the finding.
```bash
#!/bin/bash
# geo.sh <screen-name> '{"gutter":"main","header":"main > header","title":"h1","card":".card","row":"[role=row]","control":"button"}'
./ab eval "(() => { const roles = $2, n = v => Math.round(parseFloat(v) * 10) / 10, out = { screen: '$1' };
  for (const [role, sel] of Object.entries(roles)) {
    const el = document.querySelector(sel); if (!el) { out[role] = null; continue; }
    const s = getComputedStyle(el), r = el.getBoundingClientRect();
    out[role] = { h: n(r.height), pad: [s.paddingTop, s.paddingRight, s.paddingBottom, s.paddingLeft].map(n).join(' '),
      gap: s.rowGap, font: n(s.fontSize) + '/' + s.fontWeight, radius: s.borderTopLeftRadius };
  }
  return JSON.stringify(out); })()"
```
`null` means the screen lacks that role or uses other markup for it, which is
worth a look by itself. Reading computed styles changes nothing, so this
`eval` is allowed inside a walk.

## States and speed: driver verbs
```bash
./ab network route "**/api/tasks*" --abort       # error state; `network unroute` after
./ab set offline on                              # whole-app failure; `off` after
./ab record start "$OUT/shots/map/cold-home.mp4" "$URL" --contact-sheet   # loading idiom, layout jump; then `record stop`
./ab network requests --clear; ./ab open "$URL"; ./ab network requests --type xhr,fetch   # one open: duplicates, unbounded lists
./ab vitals --json                               # LCP, CLS, INP per screen
```
A response too fast to film is held by a delaying proxy you start and stop.
Routes and offline mode stay inside the audit session; clear them before the
next screen.

## connect.sh: paste a token into the app's connect form
Token comes from a file or env the owner already has, never a literal. Labels
and field names are parameters.
```bash
#!/bin/bash
# connect.sh   env: OPEN_LABEL FIELD_NAME SUBMIT_LABEL TOKEN_FILE (mode 600)
A() { ./ab "$@"; }
TOK=$(cat "${TOKEN_FILE:?}")
if ! A snapshot -i | rg -q "textbox \"$FIELD_NAME\""; then
  A find role button click --name "$OPEN_LABEL" >/dev/null 2>&1; sleep 1
fi
ref() { A snapshot -i | sed -n "s/.*$1 \[ref=\(e[0-9]*\)\].*/\1/p" | head -1; }
A fill "@$(ref "textbox \"$FIELD_NAME\"")" "$TOK" >/dev/null
A click "@$(ref "button \"$SUBMIT_LABEL\"")" >/dev/null; sleep 5
```
Verify the connected state from the snapshot text, not the screenshot (a
screenshot of the form would leak the token). Delete `TOKEN_FILE` after.

## pf.sh: act inside an iframe
Re-resolves the frame ref every call, because the ref changes on re-render, then
returns to the main frame so the next call starts clean.
```bash
#!/bin/bash
# pf.sh <agent-browser args>   env: FRAME_NAME (optional name filter)
A() { ./ab "$@"; }
A frame main >/dev/null 2>&1
REF=$(A snapshot -i 2>&1 | sed -n "s/.*Iframe \"[^\"]*${FRAME_NAME:-}[^\"]*\" \[ref=\(e[0-9]*\)\].*/\1/p" | head -1)
[ -z "$REF" ] && { echo "no iframe"; exit 3; }
A frame "@$REF" >/dev/null 2>&1
[ "$1" != snapshot ] && A snapshot -i >/dev/null 2>&1   # fresh refs for the verb
A "$@"; RC=$?
A frame main >/dev/null 2>&1
exit $RC
```
Use `./pf.sh snapshot -i` to read refs, then `./pf.sh click @eN`. Wheel scroll
that works in the frame but not the page (or the reverse) is itself a finding.
