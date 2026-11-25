---
name: semantic-decision
description: Make an explicit, structured judgment from evidence and bounded choices. Use for task sizing, skill routing, orchestration need, test impact, review findings, recovery, decision tiers, or whether human/deeper review is warranted. The current LLM judges in-session; one CLI call validates and records it.
user-invocable: false
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and uses the calling LLM fallback;
> retain the request, choice and cited reason in the handoff.

# semantic-decision
A reusable step inside the work. Follow the
[Auto-Decision Framework](../../shared/auto-decision-framework.md); mechanical
facts need no model, and User Challenges retain their escalation channel.
Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`. The [contract](../../shared/semantic-decision.md)
lists kinds, choices and floors; read it only for a kind you do not know.

Judge in this session, then record with one call. Never launch a second model.

1. Write both files from the evidence in context (no ticket needed). `state` is
   redacted, cited evidence plus the rubric; one question per judgment, IDs of
   your choosing (1-64 questions, 256 KiB). Missing evidence: gather it;
   `NEEDS_CONTEXT` only when it cannot be derived.
   ```json
   // request.json
   {"state": {"candidate": "...", "evidence": ["src/a.go:12 ..."], "rubric": "..."},
    "questions": {"q1": {"type": "choice", "instructions": "Classify against the evidence.",
      "criteria": {"CONFIRMED": "reachable trigger evidenced", "REFUTED": "cited disproof"}}}}
   // answers.json - one entry per question; choice is a criteria key
   {"q1": {"choice": "CONFIRMED", "reason": "src/a.go:12 reaches X with Y."}}
   ```
2. `bbs semantic-decision --kind <kind> --input request.json --llm-answers answers.json`
   prints the `DECIDED` result and appends to `decisions.jsonl`. A nonzero exit
   names the invalid field: fix the file and rerun. Old or missing CLI: report
   `BBS_DEGRADED` and put the choice and reason in the handoff.
3. Apply the consumer's mandatory floors from the contract; a choice never grants
   execution, spending, release or approval. Keep request and result with the
   caller's artifact and return the choices to it.

Output: `STATUS: DONE`, `VERDICT: DECIDED`, `PROVIDER: llm`,
`DECISIONS: <result JSON/path>`, `EVIDENCE: <request and cited rationale>`.
An unresolved User Challenge uses the preamble's `NEEDS_CONTEXT` channel.
