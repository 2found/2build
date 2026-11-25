# OMP launch: `native-terminal`

Read only when the selected agent is OMP (`launchMode` `native-terminal`);
[worker-routing](worker-routing.md) owns the route and the other agents.

Resolve the selected alias on the execution host before launch, using its live
OMP role binding. Put the expected resolved provider/model/effort in the Task
spec. Require the worker to compare actual session metadata before ticket work;
unknown or conflicting settings must be reported, never repaired by switching
the live model. An omitted effort uses the selected role's native binding.

Render an idle native command, create an Orca terminal in the assignment
worktree, wait for the agent to come up, then launch the Task into that exact
terminal:

```bash
bbs team worker-command --startup-only --dir "$REPO" \
  --agent omp --model <selected-model> [--effort <selected-effort>]

orca terminal create --worktree <assignment-worktree> \
  --command "<exact rendered startup command>" --json
orca terminal wait --terminal "$TERMINAL" --for tui-idle --json

bbs team task launch "$TASK" --agent omp --worktree <assignment-worktree> \
  --terminal "$TERMINAL" --created-terminal
```

`--startup-only` rejects `--prompt` and `--skill`. Do not type the assignment
by hand. Execute native preflight and startup on the selected execution host;
remote placement must be supported explicitly; never fall back to a
coordinator-local launch. A launch into `--terminal` verifies the agent Orca
detects there and reports `transport-matched`: it does not attest native
settings, so persist the worker's observed agent/model/effort separately.
`--created-terminal` records that this Dispatch owns the terminal, so the
watcher closes it at settle; without it the terminal is never closed by
the lead. Never close a preexisting user's terminal or an uncertain live
worker.
