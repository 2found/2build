## Decompose and prepare topology

Before step 2, pass **Project design checkpoint** in the project contract.
Step 1 drafts seeds and interfaces; it does not authorize child worktrees or
production dispatch. Reuse current approved artifacts on resume.

1. Use the approved parent plan/manifest from the planning worker. Children are
   **independent, testable, releasable units**, each with its own branch, verdicts
   and revert; never split one coherent change across siblings merely to widen
   the DAG. A large child uses explicit phases inside its own ticket. A proposed
   sub-ticket needing its own worktree returns to Foreman for approval against
   accepted scope, bidirectional linkage and DAG admission; workers never own topology.
   Preserve real `blocked_by`/`blocks` dependencies, shared interfaces, edit
   ownership, and acceptance-to-seed/evidence mapping. Missing accepted coverage
   needs a child or repair; changed scope or non-delegable decisions escalate.
   Persist the assigned archetype workflow with
   `BABYSIT_TICKET="$TICKET" bbs ticket set-pointer workflow <workflow>`;
   default to `builder` only for ordinary production work. Lifecycle promotion
   needs the prerequisite handoff's `LIFECYCLE` and `TRIGGER` evidence; never
   pre-admit speculative growth/maintenance work before its signal exists.
2. For each accepted seed, run `bbs ticket ensure --mode=worktree
   --from-input-file "$SEED_PATH" --reason foreman-decompose` from the
   canonical repo/base. `ensure` owns the ticket id, branch naming, and
   initial checkout — it only cuts on its slow path, so never pre-create the
   ticket id: a resolved `BABYSIT_TICKET` forces the fast-path no-op and no
   worktree is made. Parse and persist its `TICKET`/`BRANCH`/`WORKTREE`
   output; never `eval` it. Existing child → read `manifest.yaml` and reuse
   its exact branch/worktree instead of calling `ensure` again.
3. Initialize each child as a sub-ticket from inside its own worktree:
   `bbs ticket init --parent <parent> --origin-type sub_ticket --seed <seed
   path> --plan <parent plan> --position <n> --worktree <path>`. Running it
   from the worktree records the ticket branch in `pointers.branch`; from
   the primary it would record `main`. `init --parent` sets only the child's
   parent field, so add the parent-side membership after initialization:
   `BABYSIT_TICKET="$PARENT" bbs ticket add-child "$CHILD"`.
   For each dependency, add `blocked_by` on the dependent and `blocks` on the
   prerequisite with `BABYSIT_TICKET="$DEPENDENT" bbs ticket add-relation
   blocked_by "$PREREQUISITE"` and
   `BABYSIT_TICKET="$PREREQUISITE" bbs ticket add-relation blocks "$DEPENDENT"`.
   When moving a dependency, remove both old sides with
   `BABYSIT_TICKET="$DEPENDENT" bbs ticket remove-relation blocked_by "$OLD_PREREQUISITE"` and
   `BABYSIT_TICKET="$OLD_PREREQUISITE" bbs ticket remove-relation blocks "$DEPENDENT"`.
   Relationship commands lock one ticket index and append history; never edit
   `index.json` directly. Write `requirement.md` and assign parent and
   children to this foreman.
   Bind the accepted seed with `bbs foreman bind "$PARENT" --seed <key>
   --child "$TICKET"`, initialize code children with contract version 2 per
   execution.md, and include their parent acceptance IDs in the Task.
   The moment both sides of every relation are linked, emit the DAG with the
   dispatch plan — see [runtime](references/runtime.md), **Intake and DAG**.
4. Validate the primary checkout, `git worktree list`, every recorded path,
   branch head, and configured base before dispatch. Recreate a missing clean
   worktree only from its recorded branch. A dirty or divergent worktree is a
   recovery case, not permission to replace it.
5. Start dependents only after prerequisite gates pass. Before Build, dispatch
   an integration worker on the normal phase route to merge verified prerequisite
   heads into the dependent worktree; record exact SHAs in its handoff. Conflicts
   go to supervised repair workers. A later prerequisite repair invalidates
   affected dependent gates and parent integration evidence: merge the new heads
   and reverify in dependency order, only at settled worker boundaries.
   Never merge into a live writer's tree or accept gates for untested revisions.

## Resource admission

Resolve the worker bound on every fresh invocation or resume:

```bash
MAX_WORKERS="$(bbs config get parallel_max_workers 2>/dev/null || true)"
[ -n "$MAX_WORKERS" ] || MAX_WORKERS=4
```

`MAX_WORKERS` must be a positive integer or report `BLOCKED`. It is a per-Foreman
ceiling; every Dispatch also reserves machine-global weighted capacity before
`worker-start`. The broker serializes admission across foremen and derives the
host CPU/RAM budget. `parallel_global_units` can lower that budget, never raise
it. Current resource pressure queues starts without stopping running workers.

Classify the Task from its requirement, plan, and acceptance commands:

| Profile | Use for |
|---|---|
| `plan` | planning, design feedback, and other read-only work |
| `standard` | ordinary implementation, compilation, and tests |
| `android-simulator` | Android emulator/device acceptance |
| `ios-simulator` | iOS simulator acceptance |
| `local-ml` | local model loading, training, or inference |

If workload evidence is ambiguous between `standard` and a heavy profile, use
the heavy profile. Simulator profiles reserve the shared mobile stack and GPU;
`local-ml` reserves the GPU. Before each new or reused Dispatch:

```bash
RESOURCE_OUT="$(bbs foreman resource reserve "$FOREMAN_ID" \
  --ticket "$TICKET" --task "$ORCA_TASK_ID" --profile "$RESOURCE_PROFILE")"
```

Parse `ADMISSION` and `LEASE` from the output; never `eval` it. `queued` means
leave that Task pending and dispatch other admitted work: resource backpressure
is not a failed attempt. `reserved` means immediately persist the lease id as
`pointers.resource_lease` on that ticket, then call `worker-start`. If worker
creation fails, release the lease before retrying. Keep one writer per child worktree;
never exceed `MAX_WORKERS` even when global capacity remains. The broker also
checks `parallel_max_workers` atomically, counting current reservations rather
than historical worker rows.
A reservation is keyed by Foreman + Orca Task and is idempotent across resume.
After an interruption or a delayed launch, heartbeat the foreman and repeat
`reserve` immediately before `worker-start`; persist the returned lease id again.
Each replacement reservation has a new id, so an old cleanup cannot release it.
A launch reservation with no new Dispatch is reclaimed after ten minutes if its
owner is stale or missing. A live Dispatch never expires merely because the
foreman stopped heartbeating or the laptop slept.

`reserve`, `status` and the detached watcher reconcile leases. They stop exited
agents by exact Dispatch id and verify terminal state before release. Live or
unverifiable workers retain capacity (`RESOURCE_HELD`); never release by age.
Retry proven stopped Tasks through their failed Dispatch, preserving worktrees
and checkpoints. Release settled leases with
`bbs foreman resource release "$RESOURCE_LEASE"`, clear `pointers.resource_lease`,
and reserve the next Task's profile before reusing a worker.

## Worker envelope

Every worker Task spec must establish the execution envelope before naming its
ticket work: this is a supervised Orca Dispatch, its effective
`AGENT_ROLE=orca`, and it is already spawned. It also names the resource
profile Foreman reserved for this Task. The worker invokes the installed skill
directly in that turn, skips any developer `/goal` copy/paste handoff, uses
Orca `ask` for a genuine User Challenge, and follows the injected lifecycle
through exactly one `worker_done`. This statement in the Task spec is
load-bearing because `worker-start --agent` does not expose an environment
option; never assume a coordinator shell export reached the worker process.
