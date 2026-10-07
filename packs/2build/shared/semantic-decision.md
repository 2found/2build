# Semantic decision
Contract for the [semantic-decision skill](../semantic-decision/skill.md),
a judgment step governed by the Auto-Decision Framework. The default provider
is the calling `llm`; configured Cloudflare uses `clef-flash` or `clef`, with
`llm` fallback. The CLI
never launches another LLM. Use after gathering evidence, before selecting
among bounded choices; skip mechanical facts and explicit user selections.

## Provider configuration
The step always runs; no configuration means the calling LLM makes the decision.
**Only an explicit user request authorizes changing user settings**; credentials
alone never select an external provider. To use Cloudflare, the user sets:
```sh
bbs config set semantic_decision_provider cloudflare
bbs config set semantic_decision_model clef-flash
```
These keys live in user `~/.babysit/config.yaml` (or `BABYSIT_STATE_DIR`).
`cloudflare` and `llm` are the allowed providers; `clef` and `clef-flash` are
the Cloudflare models. Credentials resolve environment first, then the user
state directory's `.env`: `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_API_TOKEN`.
Project dotenv/config cannot supply credentials, select external inference, or select
provider/model. Do not export project credentials into the command's environment.

Optional project `.babysit/semantic-decision.yaml` can only restrict consent:
```yaml
enabled: false                     # forces llm; true grants no external access
allowed_kinds: [task-size, testcase] # omitted = all; [] = none
```
Malformed/unreadable project policy falls back to LLM. A provider
result never grants execution, orchestration, spending, release, or escalation
permission. Workflow prerequisites and explicit authorization remain authoritative.

## Call contract
Write a JSON request to a scratch/evidence file. `state` contains only relevant,
redacted evidence: requirement, cited code excerpts, candidate, caller/flow map,
and the applicable rubric. Never include credentials or whole environment dumps.
Use stable IDs; each question has `type: choice`, `instructions`, and `criteria`
(mapping option ID to meaning). Batch independent questions sharing context in
one request (1–64 questions; 256 KiB maximum); narrow evidence rather than truncate.
```json
{
  "state": {"candidate": "...", "evidence": ["src/a.go:12 ..."], "rubric": "..."},
  "questions": {
    "finding_1": {
      "type": "choice",
      "instructions": "Classify this finding against the cited evidence and verdict ladder.",
      "criteria": {
        "CONFIRMED": "Concrete reachable trigger and wrong result are evidenced.",
        "PLAUSIBLE": "Mechanism is real; trigger needs verification.",
        "REFUTED": "Cited code or invariant proves the candidate false."
      }
    }
  }
}
```
```sh
bbs semantic-decision --kind review-finding --input request.json
```
Stdout is JSON, never a bare verdict:
- `status: DECIDED`, `provider: cloudflare`: `answers` keyed by question ID,
  each with `choice`, `confidence`, `probabilities`.
- `status: NEEDS_LLM`, `provider: llm`: the normal default route; **no answers
  yet**. Decide in the current session against the evidence/rubric.
- `status: FALLBACK_LLM`, `provider: llm`: Cloudflare could not decide or project
  policy restricts it. Complete the same step using the current LLM.
  For either LLM route, record answers (each `choice` + cited `reason`) via:
  `bbs semantic-decision --kind review-finding --input request.json --llm-answers answers.json`.
  This returns `DECIDED` with `provider: llm` and no fabricated probabilities.
- Nonzero exit: invalid input/config; correct the request before proceeding.
  Old/missing CLI: report `BBS_DEGRADED`, use the same LLM fallback and include
  the decision/reason in the handoff.

Fallback occurs for restricted external inference, absent credentials, provider
errors/timeouts (20s), malformed/incomplete answers, or uncertain answers. Current
routing threshold: winning probability ≥0.75; this is not a correctness
guarantee. A failed question sends the whole batch to fallback;
no partial acceptance. Never blindly retry a provider or block an unattended run
for its credentials. Reconcile returned choices with cited evidence and mandatory
floors; if they conflict, use and record the LLM fallback with the discrepancy.
Keep request, response and fallback rationale in the task's evidence/handoff.

CLI telemetry appends to `decisions.jsonl`, respecting `telemetry: off` and
`BABYSIT_ANALYTICS_DIR`. It records kind, request hash, provider/model, outcome,
fallback reason, choices and latency; not source text, credentials or reasoning.
The matching hash correlates fallback with its completion.

## Consumers
| Kind | Evidence and bounded choices | Non-negotiable floor |
|------|------------------------------|----------------------|
| `task-size` | Requirement, expected footprint, API/migration/dependency changes + full [size rubric](ticket-size-rubric.md); `XS/S/M/L`. | Largest matching rubric row; unknowns default M. File/LOC counts alone do not prove safe XS. |
| `task-complexity` | Assignment requirement, plan and acceptance commands + Foreman task-complexity rubric; `simple/normal/hard`. | Weak evidence stays normal; concrete high-impact risk stays hard. Model-tier lookup remains deterministic. |
| `testcase` | Candidate case/test, criterion and caller/state/route impact map; `required/adjacent/unaffected`. | Every acceptance criterion, deviation, unresolved finding and touched flow's non-happy path retains coverage. `unaffected` needs evidence before excluding a case. |
| `review-finding` | Candidate + relevant diff/enclosing code/callers/guards + effort's verdict ladder; `CONFIRMED/PLAUSIBLE/REFUTED`. | REFUTED requires a cited disproof. Uncertainty never drops a finding. Preserve independent verification. |
| `deep-review` | Remaining risk, verification gaps, review effort and findings; `deepen/sufficient`. | Cannot skip the selected effort's mandatory phases or pass unresolved material findings. |
| `skill-route` | User intent + available skills/archetype mandates; candidate skill/workflow names as choices. | Explicit skill/workflow selection wins; eligibility and prerequisite gates still run. |
| `orchestration` | Proposed decomposition, dependencies, shared state and verification boundaries; `single-ticket/multi-ticket`. | Recommendation only; Foreman owns dispatch/topology and its approvals. |
| `decision-tier` | Proposed action, evidence, user direction and authority; `Mechanical/Taste/User Challenge` from the Auto-Decision Framework. | Cannot downgrade known User Challenges or missing authority; no recursive classification. |
| `human-review` | Checkpoint policy, artifact revision, rubric, gaps and delegated authority; `proceed/revise/needs-human/blocked`. | Apply the owner policy first; see below. |
| `recovery` | Blocker evidence, cause, prior attempts and available inputs; `recoverable/needs-human`. | Preserve triage's retry limit and execution authority; provider failure is not a human blocker. |
| `custom` | Another bounded judgment with explicit criteria and evidence. | Auto-Decision Framework applies; never classify away a User Challenge. |

Cloudflare wire contract: [Clef-flash API](https://developers.cloudflare.com/ai/models/%40cf/cloudflare/clef-flash/).
The provider sends `model`, `state`, `questions` to Workers AI and validates
its `result.answers`; skills consume only the shared result above.

## Human review
First read the checkpoint owner's current policy and durable state. Explicit
`--stop-after`, human-held approval, Foreman's parent checkpoint without `--auto`,
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
