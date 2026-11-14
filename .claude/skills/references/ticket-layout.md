# Ticket Layout (Layout C)
A ticket is a folder under `~/.babysit/projects/<slug>/tickets/<ticket>/` —
open it and answer *what is this ticket, what state, what next* with bounded
reads. `manifest.yaml` is the identity anchor (schema:
[docs/identity.md](../../../docs/identity.md)); resolution ladder: env
`BABYSIT_TICKET` → `manifest.yaml` cwd-match → branch regex.
```
tickets/<ticket>/
├── index.json                       authoritative metadata + relations + pointers
├── requirement.md                   verbatim user intent (write-once anchor)
├── design.md / plan.md / manifest.md   owner-skill snapshots (optional)
├── handoffs/                        append-only change briefs, 001-…; LATEST holds current filename
├── verdicts/<skill>.md              worker's own status block (overwrite per run)
├── reviews/<skill>.md               a gate's opinion on someone else's work
├── history.jsonl                    append-only event timeline (`tail -20` = what happened)
├── evidence/<skill>/…               per-skill blobs
├── sub-tickets/<N>-<slug>.md        decomposition seeds (only if split)
├── checkpoint.json                  workflow-step state, owned by `bbs autopilot`
└── .index.lock/                     mkdir lock for index.json mutations
```
Every writable file is name-partitioned by skill, append-only, or
lock-protected; all metadata mutations go through `bbs ticket` — never edit
`index.json` by hand. Append-only files only grow: write them through their
verb, read a bounded slice (`tail`, `jq`, the latest handoff), never Read one whole.
## index.json
All fields optional except `id`; unknown fields preserved. Key fields:
`status`, `phase`, `parent`/`children`, `relations` (`blocks`, `blocked_by`,
`duplicate_of`, `related`), `siblings` (`{role, repo, ticket}` for cross-repo),
`labels`, `assignee`, `control`, `origin`, `pointers` (`branch`, `pr`, `plan`,
`design`, `requirement`, `manifest`), and the planning fields below.
**Status enum** (set via `bbs ticket set-status`, never derived from
verdicts): `triage` → `backlog` → `planned` / `decomposed` → `in_progress` →
`in_review` → `done`; plus `blocked`, `cancelled`, `duplicate`.

`reconcile` advances the rung from *facts* — files on disk, a pushed flag, a
merge in git — and only ever forward: `requirement.md` → `backlog`, `plan.md`
→ `planned`, `manifest.md` → `decomposed`, a pushed manifest or a
`pointers.pr` → `in_review`, and a ticket branch's own commits merged into base
(a PR a human merged and pulled) → `done`. A Team ticket's scratch branch
never gains a commit, so the lead sets `done` when its phase lands. `in_progress` is nobody's derivation —
nothing on disk distinguishes it. Verdicts stay out on purpose: a ticket with
`qa` and `review-pr` both DONE that nobody closed out is *not* finished, and
deriving `done` from the verdicts would hide precisely that gap. A
squash-merged PR leaves no local trace of its branch, so those tickets rest at
`in_review`.
**`control`** is the human override axis, separate from status:
`{state: paused|cancelled, prior_status, note, actor, at}`, `null` when the
ticket is uncontrolled. Set by `bbs ticket pause|cancel`, cleared by
`resume|restore`. A control action **never** writes `status` — status stays the
derived rung so clearing the control returns the ticket exactly where it was,
and `reconcile` skips any ticket with a `control.state`. Distinguish it from
`status: cancelled`, which is a *terminal* rung meaning the work was abandoned;
`control.state: cancelled` means a human pulled it out of dispatch and can put
it back.
**`assignee`** names the owning lead (`bbs ticket assign <id>|--none`).
Assignment lives on the ticket, so a lead's inbox is derived by scanning
tickets — there is no parallel queue that can drift. It is the lead's claim,
not the role: who does the work is `assigned_to`.
**Planning fields** (set only through the verbs; each changed field appends
one `field_set` history row, an unchanged value writes nothing):
`title` (`set-title`; else the requirement H1), `kind`
(`milestone|phase|work`, `set-kind`; absent = `work`), `product`
(`set-product`; work inherits its milestone's), `assigned_to{role,person}`
(`assign-to swe|human|marketing|ops [--person]`), `labels` (`label
--add/--remove`), `window{start,due}` (`set-window`), `done_at` (stamped on
the first `done`), and `loop{stage, target_outcome, expected_delta,
check_after_days, check_from, evidence_refs, decision, actual}` (`loop set`).
`decide go|later|drop` and `record-actual` write `loop.decision` /
`loop.actual` plus a `loop_decision` / `loop_actual` history row — the CL4.3
decision log is those rows (`bbs company log`). `decide` is human-only: it
exits 3 under a Team or OpenClaw session or a non-developer/dashboard role.
`bbs ticket find --role swe --ready --json` is the agent pick-up queue: work
tickets in `triage|backlog|planned`, uncontrolled, with no open effective
blocker (own, inherited from ancestors, a container blocker expanded to its
open work). The dashboard and `find` read the same projection
(`internal/ticket/plan.go`). `bbs ticket import plan.yaml` creates/updates a
plan idempotently (keys resolve by `id`, then `import.{plan,key}`);
`bbs ticket migrate` backfills a pre-planning store from the H1 and header
bullets, filling only empty fields.
**`origin.type`**: `standalone` (default) | `sub_ticket` (has `parent`,
`seed`, `plan`, `position`) | `hotfix` | `design-initiated` (has `design_doc`).
Set at creation time: `bbs ticket init --parent <id> --origin-type sub_ticket
--seed <path> --position <n>`. `set-parent` writes only the parent link, so a
child left at the `standalone` default will not route to `builder`'s child mode.

Links are symmetric: `init --parent` / `set-parent` also add the child to the
parent's `children` (and drop it from a previous parent's), `add-child` sets
the child's `parent`, and `add-relation` / `remove-relation
<blocks|blocked_by|related> <target>` write the inverse on the target
(`duplicate_of` is one-way and clears only when the target matches). A link
that would close a parent loop exits 2. Each side appends its own history row
under its own lock. `repair-relations [--all] [--dry-run]` adds missing
inverses on a store written before this; a child listed by two parents is
reported, never rewritten. The DAG walks `children`, not `parent`.
## Bootstrap: how tickets come into being
Tickets resolve through the identity ladder — `BABYSIT_TICKET` env →
`manifest.yaml` cwd-match → branch regex
`(feat|fix|chore|bug|refactor)/<id>_<slug>` — the preamble derives `$TICKET`
and runs `bbs ticket init`. Entry-point skills invoked without a ticket (e.g.
from `main`) run the universal entry hook early instead of failing:
```bash
ENSURE_OUT=$(bbs ticket ensure --no-branch \
  --from-input "$USER_REQUEST" \
  --type feat \
  --reason <skill-name>-entry)
TICKET=$(printf '%s\n' "$ENSURE_OUT" | sed -n 's/^TICKET=//p')
# Parse, never eval — TICKET_HOME/WORKTREE are unquoted paths. Carry the id
# per command: BABYSIT_TICKET="$TICKET" <cmd> (exports don't survive
# per-call shells).
```
`ensure` is idempotent. Fast-path (identity already resolved — env, manifest
cwd-match, or a ticket branch): no-op, prints `CREATED=0` + ticket env.
Slow-path: generates `bs-<8hex>`, seeds `requirement.md`, prints `CREATED=1`
+ ticket env. Under `trunk` — the default, and everything without an
explicit `--mode` — that is all it does: no branch is cut and the
`TICKET=<id>` line carries the identity (prefix later commands with
`BABYSIT_TICKET`). Autopilot always runs it this way — `--no-branch` —
because it works on the checkout it was started in. Only a lead dispatch
or an explicit human `--mode=worktree` cuts a worktree from local base and
prints `WORKTREE=<path>` — cd there; see [git-flow.md](git-flow.md).
## Accessing ticket files
**Construct paths via `bbs ticket path` / `bbs ticket list` — never by
concatenating `$TH/`, `$TICKET_HOME/`, etc.** The broker resolves canonical →
legacy (pre-Layout-C tickets keep working) and validates selectors.
Kinds: `home`; single files `index requirement design plan manifest history
checkpoint`; `handoff` (`--skill`, `--seq N` or `--latest`); `verdict` /
`review` (`--skill`); `evidence` (`--skill`, `--name`); `sub-ticket`
(`--seq`, `--slug` for write). `--read` returns the first existing candidate;
`--write` returns the canonical target and mkdir-ps the parent; append-only
kinds reject `--write` — use `add-handoff` / `set-verdict` / `set-review`.
```bash
# Exit codes from `bbs ticket path … --read`:
#   0 — found (canonical or legacy hit)
#   1 — not found (canonical AND every legacy candidate missing)
#   2 — usage / missing required selector
#   3 — security (selector failed _safe_path_component)
# Treat 0/1 as data signals; let 2/3 propagate.
PLAN="$(bbs ticket path plan --read 2>/dev/null)"; rc=$?
case $rc in
  0) ;;             # found — $PLAN is the path
  1) PLAN="" ;;     # absent — fall back / skip
  *) exit "$rc" ;;  # usage / security — surface the failure
esac
[ -n "$PLAN" ] && cat "$PLAN"
```
(Don't use `X="$(…)" || X=""` — it swallows exit 2/3 into "not found".)
`bbs ticket lint` flags raw `$TH/` constructions in bash fences; a
genuinely needed bypass takes a trailing `# lint:allow-direct-path <why>`.
## Typical skill usage
```bash
bbs ticket init                                  # startup — idempotent

cp "$PLAN_DRAFT" "$(bbs ticket path plan --write)"   # primary artifact
bbs ticket set-pointer plan plan.md
bbs ticket set-status planned
bbs ticket set-phase plan-draft

bbs ticket add-handoff --skill plan-draft --status DONE --body-file "${TMPDIR:-/tmp}/brief.md"
bbs ticket set-verdict  --skill plan-draft --body-file "${TMPDIR:-/tmp}/verdict.md"

ORIGIN_TYPE="$(bbs ticket get origin.type)"      # reads, e.g. sub-ticket check
```
Custom timeline events: `bbs ticket append-history --event <e> --extra-json
'{…}'` (core fields `ts ticket branch event actor` are protected).
`evidence/quality/` is shared between the `quality` workflow and `bbs:qa` —
grep by filename, don't assume one writer.
