# Handoff Contracts
Downstream actors use the [status block](preamble.md#completion-status-protocol),
git diff/commits, and ticket artifacts under
`~/.babysit/projects/<slug>/tickets/<ticket>/` ([layout](ticket-layout.md)).
Only developer runs may also rely on conversation. Resolve identity via the
[preamble](preamble.md#ticket-consistency--the-four-layer-invariant).

## Verdicts per skill

| Skill | Verdict shape |
|-------|--------------|
| `autopilot` | `PLANNED` \| `BUILT` \| `FIXED` \| `HANDOFF` |
| `browse` | `CHECKED` |
| `implement` | `BUILT` \| `FIXED` \| `CHANGED` |
| `investigate` | `FIXED` \| `INVESTIGATED` |
| `plan-draft` | `PLANNED(<XS\|S\|M\|L>)` \| `DECOMPOSED(<N>)` |
| `qa` | `PASS` \| `FIXED(<N>)` \| `FAIL` |
| `review-pr` | `PASS` \| `FINDINGS(<N>)` \| `FIXED(<N>)` |
| `create-pr` | `PR_CREATED` |
| `design-ui` | `DESIGNED` |
| `recon` | `STEAL(<approach>)` \| `PASS` |
| `setup-project` | `CONFIGURED` |

New skills pick a one-line verdict and document it in their own SKILL.md.

## CHANGE_BRIEF — the primary file artifact
For each finished skill, write a brief with these single-line fields; use
`none` for inapplicable fields. Preserve any skill-specific sections below them.
```text
SUMMARY: <what changed and why>
FILES: <changed files>
APPROACH: <implementation approach>
BLAST_RADIUS: <existing behavior affected>
```
With a ticket, publish through the helper; never write numbered handoffs directly:
```bash
bbs ticket add-handoff --skill <skill> --status <status> --body-file <brief-path>
```
Without a ticket, skip ticket writes and return the brief in the response or a
caller-supplied artifact. Keep scratch files in the ticket directory or a temp
directory, outside the repo diff.

## Evidence paths
Paths below are relative to the ticket directory. Mutate metadata through `bbs ticket`.

| Path | Contents / writer |
|------|-------------------|
| `handoffs/<NNN>-<skill>.md` | Append-only briefs; `add-handoff` |
| `verdicts/<skill>.md` | Latest status block; `set-verdict --skill <skill> --body-file <report>` |
| `reviews/<skill>.md` | Latest review; `set-review --skill <skill> --body-file <report>` |
| `plan.md`, `design.md`, `manifest.md` | Canonical artifacts; skill writes, `set-pointer` registers |
| `evidence/*.{png,json}`, `report.md` | Screenshots, structured output, summary; skill writes |

### Typed evidence artifacts
`bbs ticket set-evidence` validates structure. Exit 2 → retry once, then escalate.
Check presence/structure, never a self-assigned score. Release gates still read
`verdicts/` ([approval contract](../../../docs/artifact-gated-approval.md)).

`verification` belongs to `implement`, `browse`, `investigate`: required `result`
(`PASS` or `FAIL`); optional `checks:[{cmd,result}]`, `before`, `after`.
```bash
bbs ticket set-evidence --kind verification --json '{"result":"PASS"}'
bbs ticket evidence-status --kind verification   # none | valid | malformed
```

## Git conventions
Git topology, review cards and landing policy belong to the caller/workflow.
Skills can run standalone in the current checkout.
