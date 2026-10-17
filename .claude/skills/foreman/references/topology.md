## Decompose and prepare topology

Before any phase opens, pass **Project design checkpoint** in the project
contract. Basic design drafts phases, seeds and interfaces; it does not
authorize worktrees or production dispatch. Reuse current approved artifacts
on resume. Tickets are created per phase, when [the phase opens](phases.md#1-open-the-phase),
under that phase's ticket.

1. Use the approved manifest from the basic-design worker. Each **ticket** is
   an **independent, testable unit** of one phase, with its own worktree,
   verdicts and revert, run end to end by **one worker session**, which can
   work for hours or days. Any size on the
   [size rubric](../references/ticket-size-rubric.md) up to `L` is a valid
   ticket; every ticket boundary costs a cold session, a seal and a full
   review/QA cycle, so **prefer fewer, larger tickets**. Split only along a real
   seam: a dependency another ticket needs first, work that genuinely runs in
   parallel, a separately verifiable and revertible outcome, or scope one
   coherent plan cannot hold (unrelated journeys or subsystems). Never split
   one coherent change across siblings merely to widen the DAG, and never
   split inside a worker. A worker that finds its ticket beyond one coherent plan, or a
   sub-ticket needing its own worktree, returns the proposed split to Foreman
   for approval against accepted scope, bidirectional linkage and DAG
   admission; workers never own topology.
   Preserve real `blocked_by`/`blocks` dependencies, shared interfaces, edit
   ownership, and acceptance-to-seed/evidence mapping. Missing accepted coverage
   needs a child or repair; changed scope or non-delegable decisions escalate.
   Persist the assigned archetype workflow with
   `BABYSIT_TICKET="$TICKET" bbs ticket set-pointer workflow <workflow>`;
   default to `builder` only for ordinary production work. Lifecycle promotion
   needs the prerequisite handoff's `LIFECYCLE` and `TRIGGER` evidence; never
   pre-admit speculative growth/maintenance work before its signal exists.
2. For each seed of the opening phase, run `bbs ticket ensure --mode=worktree
   --from-input-file "$SEED_PATH" --reason foreman-decompose` from the
   canonical repo, so the ticket starts from the local base the previous phase
   landed on. `ensure` owns the ticket id, scratch branch naming, and
   initial checkout — it only cuts on its slow path, so never pre-create the
   ticket id: a resolved `BABYSIT_TICKET` forces the fast-path no-op and no
   worktree is made. Parse and persist its `TICKET`/`BRANCH`/`WORKTREE`
   output; never `eval` it. Existing child → read `manifest.yaml` and reuse
   its exact branch/worktree instead of calling `ensure` again.
3. Initialize each child as a sub-ticket of its **phase ticket** from inside
   its own worktree: `bbs ticket init --parent <phase-ticket> --origin-type sub_ticket --seed <seed
   path> --plan <parent plan> --position <n> --worktree <path>`. Running it
   from the worktree records the ticket branch in `pointers.branch`; from
   the primary it would record `main`. `init --parent` sets only the child's
   parent field, so add the parent-side membership after initialization:
   `BABYSIT_TICKET="$PHASE_TICKET" bbs ticket add-child "$CHILD"`. Dependency
   edges stay inside one phase; a later phase already starts from an earlier
   one's code.
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
   --child "$TICKET"` and include their parent acceptance IDs in the Task.
   The moment both sides of every relation are linked, emit the DAG with the
   dispatch plan — see [runtime](references/runtime.md), **Intake and DAG**.
4. Validate every recorded worktree path and the base before dispatch. A
   missing worktree of a sealed ticket is recreated by `bbs ticket ensure` and
   `bbs ticket adopt` of its own snapshot; one that was never sealed is
   re-dispatched. A worktree with unexpected changes is a recovery case, not
   permission to replace it.
5. Dependents launch with their prerequisites. The dependent's assignment
   names each prerequisite as `<ticket>:<orca-task>`; after its plan is
   approved, its ticket worker runs `bbs foreman await --prereqs
   <ticket>:<task>,...`, which blocks until each prerequisite's Dispatch has
   completed and its ticket passes readiness (review, or push when
   `pointers.deferred_heavy` defers its QA) and is sealed, then **adopts** each
   sealed snapshot into the worktree as uncommitted changes (`SNAPSHOT=` per
   prerequisite). `CONFLICT=` files (`AWAIT=conflict`, exit 5) are resolved in
   the same session against the accepted shared interfaces — no separate
   integration worker. `AWAIT=not-ready` (a prerequisite settled without
   passing) ends that Dispatch without adopting; once the prerequisite passes,
   send the dependent a `refresh`. A later prerequisite repair invalidates
   affected dependent gates and phase evidence: send each affected dependent a
   `refresh` (it runs `bbs ticket adopt <prerequisite>` and re-verifies) in
   dependency order, only at settled boundaries. Never adopt into a live
   writer's tree or accept gates for untested content.

## Resource admission

Resolve the worker bound on every fresh invocation or resume: parse
`FOREMAN_MAX_WORKERS` from `bbs foreman resource status` (never `eval`). It is
`parallel_max_workers` when that is a positive integer, otherwise (unset or
`auto`) min(16, max(2, CPUs - 2)); an invalid value fails the command, so
report `BLOCKED`. It is the batch size and a per-Foreman
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
`local-ml` reserves the GPU. `plan` and `standard` cost one unit, heavy
profiles four; a ticket worker is `standard` (`plan` for an evidence-only
ticket), and a heavy Task is normally the consolidated heavy QA from SKILL.md.
Admit runnable tickets before dependents that would only wait: a waiting
dependent holds its slot and lease, so it gets one only after every ready
ticket has its own. Before each new or reused Dispatch:

Resolve the agent route first with [worker routing](worker-routing.md), then
pass its selected agent and destination host into quota admission:

```bash
RESOURCE_OUT="$(bbs foreman resource reserve "$FOREMAN_ID" \
  --ticket "$TICKET" --task "$ORCA_TASK_ID" --profile "$RESOURCE_PROFILE" \
  --agent "$ROUTE_AGENT" --host "$ROUTE_HOST")"
```

Parse `ADMISSION`, `QUOTA_STATUS`, `QUOTA_RECHECK_AT` and `LEASE`; never `eval`
the output. `QUOTA_STATUS=deferred` means every mapped pool is fresh,
authoritative and exhausted: keep the Task pending, do not call `worker-start`,
and repeat route plus admission no earlier than `QUOTA_RECHECK_AT`. `unknown`,
`available` and `not-applicable` retain the existing CPU/RAM and worker-count
rules. The local `orca account list` view is advisory only; it cannot prove
destination-host quota or agent-to-pool mapping. Quota never changes a live
worker's route or account.

`queued` means leave that Task pending and dispatch other admitted work:
resource backpressure is not a failed attempt. `reserved` means immediately
persist the lease id as `pointers.resource_lease` on that ticket, then call
`worker-start`. If worker creation fails or reports a rate limit, persist the
launch result, release the lease before retrying, and re-read quota before a
new admission. Keep one writer per child worktree; never exceed `FOREMAN_MAX_WORKERS`
even when global capacity remains. The broker also checks
`parallel_max_workers` atomically, counting current reservations rather than
historical worker rows.

A reservation is keyed by Foreman + Orca Task and is idempotent across resume.
After an interruption or a delayed launch, heartbeat the foreman and repeat
route plus `reserve` immediately before `worker-start`; persist the returned
lease id again. Each replacement reservation has a new id, so an old cleanup
cannot release it. A launch reservation with no new Dispatch is reclaimed after
ten minutes if its owner is stale or missing. A live Dispatch never expires
merely because the foreman stopped heartbeating or the laptop slept.

`reserve`, `status` and the detached watcher reconcile leases. They stop exited
agents by exact Dispatch id and verify terminal state before release. Live or
unverifiable workers retain capacity (`RESOURCE_HELD`); never release by age.
Retry proven stopped Tasks through their failed Dispatch, preserving worktrees
and checkpoints. Release settled leases with
`bbs foreman resource release "$RESOURCE_LEASE"`, clear `pointers.resource_lease`,
and reserve the next Task's profile before reusing a worker. A retained idle
ticket session holds no lease.

## Worker envelope

Every worker Task spec must establish the execution envelope before naming its
ticket work: this is a supervised Orca Dispatch, its effective
`AGENT_ROLE=orca`, and it is already spawned. It invokes autopilot with
`--auto`, so the worker sets its own goal where its agent can and otherwise
works the loop directly; Foreman never types `/goal` or any prompt into a
worker pane. It also names the resource
profile Foreman reserved for this Task, and whether it is a ticket assignment
or a follow-up (`repair`, `refresh`, `redirect`, `deliver`) on an existing
ticket session, with the trigger evidence and the gates it invalidates. The worker invokes the installed skill
directly in that turn, skips any developer `/goal` copy/paste handoff, uses
Orca `ask` for a genuine User Challenge, and follows the injected lifecycle
through exactly one `worker_done`. This statement in the Task spec is
load-bearing because `worker-start --agent` does not expose an environment
option; never assume a coordinator shell export reached the worker process.
