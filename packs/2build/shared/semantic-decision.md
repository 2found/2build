# Semantic decision
Contract for the [semantic-decision skill](../skills/semantic-decision/SKILL.md),
a judgment step governed by the Auto-Decision Framework. The calling LLM
judges in-session; the CLI validates its answers against the request and
records them. It never launches another model. Use after gathering evidence,
before selecting among bounded choices; skip mechanical facts and explicit user
selections. A choice never grants execution, orchestration, spending, release
or escalation permission; workflow prerequisites and explicit authorization
remain authoritative.

## Call contract
`state` holds only relevant, redacted evidence (requirement, cited excerpts,
candidate, caller/flow map, rubric); never credentials or environment dumps.
Each question has a stable ID, `type: choice`, `instructions` and `criteria`
(option ID to meaning). Batch independent questions sharing context in one
request (1-64 questions; 256 KiB maximum); narrow evidence rather than truncate.
Answers map each question ID to `choice` (a criteria key) and a cited `reason`.
One call records everything:
```sh
bbs semantic-decision --kind <kind> --input request.json --llm-answers answers.json
```
Stdout is `{"status":"DECIDED","provider":"llm","answers":{...}}`. Nonzero exit:
invalid request, answer or kind; correct the file and rerun. Without
`--llm-answers` the CLI fails and names this form. Old/missing CLI: report
`BBS_DEGRADED`, keep the choice and reason in the handoff.

Reconcile choices with cited evidence and the floors below. Telemetry appends to
`decisions.jsonl`, respecting `telemetry: off` and `BABYSIT_ANALYTICS_DIR`: kind,
request hash, outcome, choices and latency; not source text or reasoning.

## Consumers
| Kind | Evidence and bounded choices | Non-negotiable floor |
|------|------------------------------|----------------------|
| `task-size` | Requirement, expected footprint, API/migration/dependency changes + full [size rubric](ticket-size-rubric.md); `XS/S/M/L`. | Largest matching rubric row; unknowns default M. File/LOC counts alone do not prove safe XS. |
| `task-complexity` | Assignment requirement, plan and acceptance commands + the lead task-complexity rubric; `simple/normal/hard`. | Weak evidence stays normal; hard only when the assignment's own diff changes a named money, auth, security, live-data or concurrency invariant. Model-tier lookup remains deterministic. |
| `testcase` | Candidate case/test, criterion and caller/state/route impact map; `required/adjacent/unaffected`. | Every acceptance criterion, deviation, unresolved finding and touched flow's non-happy path retains coverage. `unaffected` needs evidence before excluding a case. |
| `review-finding` | Candidate + relevant diff/enclosing code/callers/guards + effort's verdict ladder; `CONFIRMED/PLAUSIBLE/REFUTED`. | REFUTED requires a cited disproof. Uncertainty never drops a finding. Preserve independent verification. |
| `deep-review` | Remaining risk, verification gaps, review effort and findings; `deepen/sufficient`. | Cannot skip the selected effort's mandatory phases or pass unresolved material findings. |
| `skill-route` | User intent + available skill/workflow descriptions; candidate skill/workflow names as choices. | Explicit skill/workflow selection wins; eligibility and prerequisite gates still run. |
| `orchestration` | Proposed decomposition, dependencies, shared state and verification boundaries; `single-ticket/multi-ticket`. | Recommendation only; the lead owns dispatch/topology and its approvals. |
| `decision-tier` | Proposed action, evidence, user direction and authority; `Mechanical/Taste/User Challenge` from the Auto-Decision Framework. | Cannot downgrade known User Challenges or missing authority; no recursive classification. |
| `human-review` | Checkpoint policy, artifact revision, rubric, gaps and delegated authority; `proceed/revise/needs-human/blocked`. | Apply the owner policy first; see below. |
| `recovery` | Blocker evidence, cause, prior attempts and available inputs; `recoverable/needs-human`. | Preserve triage's retry limit and execution authority; a failed CLI call is not a human blocker. |
| `custom` | Another bounded judgment with explicit criteria and evidence. | Auto-Decision Framework applies; never classify away a User Challenge. |

## Human review
First read the checkpoint owner's current policy and durable state. Explicit
`--stop-after`, human-held approval, the lead's parent checkpoint without `--auto`,
holds/grant bounds and non-delegable floors determine the route mechanically.
Do not call a model to waive them. A checkpoint that policy already requires
from a human stays human-held; gather reviewable artifacts before escalation.

For an eligible autonomous review or an unresolved need for human input, use
kind `human-review` with accepted direction, exact artifact revision, criterion
coverage, named rubric evidence, unresolved gaps, proposed action and authority:
- `proceed`: evidence satisfies the rubric within the accepted direction and
  delegated scope. Submit the normal approval/verdict operation; this choice
  is neither an approval record nor permission to execute.
- `revise`: a named defect or evidence gap is locally repairable. Return it to
  its owner, repair and re-review; do not ask the human to perform routine QA.
- `needs-human`: a non-derivable user decision/permission or materially different
  direction is required. Use the preamble's channel and name the exact question.
- `blocked`: required evidence/capability is unavailable or the owner's retry
  budget is exhausted. Record the gaps; do not fabricate approval or wait for
  a human when no human action can resolve them.

Never infer permission from confidence, silence, elapsed time, an LLM result,
or a rubric merely containing five nonempty lines. Re-read artifact freshness
and use the normal gate (`approval self-resolve`, readiness, etc.) before acting;
any refusal still routes according to that gate. Missing evidence cannot return
`proceed`. A changed artifact invalidates the judgment as well as its approval.
