# Auditing a live system
Default is read-only. Everything below overrides convenience.
- **Allowlist.** Before the walk, write the mutating clicks you may make (each
  with its expected effect). Anything else is observe-only. Record every
  mutation actually made in the report header.
- **Read-only proxy.** To run the real UI on real data, put a proxy you own in
  front of the live backend that forwards reads and refuses every write.
  Report how many writes it refused and that none reached the server; say
  which screens could not load through it. A failure that exists only
  because of the harness (proxy origin, URLs an edge rewrites) is a limit in
  the report header, not a finding; confirm on the shipped origin when you can.
- **Not your decisions.** Never answer cards, approve, raise budgets, or change
  settings, credentials or schedules for the owner. A confirm dialog may be
  opened and cancelled to prove it exists.
- **Secrets.** Only via a mode-600 file filled from a shell variable
  (`umask 077`); never echo, type into a transcript, or screenshot them. Delete
  the file afterwards. Missing secret: `BLOCKED`, name the file/var.
- **Isolated browser.** Use the `browse` session exports and cleanup; close the
  exact session at the end. Never reuse the human's browser.
- **Servers.** Kill only dev servers/PIDs you started; leave the live runtime.
- **Origin / CORS / auth gates** on a dev origin: report as "blocked here"
  (journey verdict BLOCKED, check the underlying data read-only instead). Do
  not add origins, change config or call the API by hand to get around them.
- **Anything needing mutation** (run a job, answer a card, trigger a failure):
  run it on an isolated snapshot copy of the data with a mock/demo provider,
  separate home dir and ports, outbound dependencies stubbed or pointed at a
  dead address, schedules paused. Say in the report which findings came from the
  copy and which from live.
- **Cleanup line** in the report: session closed, PIDs stopped, temp secrets
  deleted, copies removed or kept (path).
