---
name: semantic-decision
description: Make an explicit, structured judgment from evidence and bounded choices. Use for task sizing, skill routing, orchestration need, test impact, review findings, recovery, decision tiers, or whether human/deeper review is warranted. Defaults to the current LLM; configured Cloudflare Clef/Clef-flash can replace the provider.
---

> **Prerequisite — the `bbs` CLI.** Every command below shells out to `bbs`.
> Install it first: `brew install 2found/2build/bbs` (macOS/Linux), the release
> tarball on Linux, or WSL/Git-Bash on Windows (no Windows binary is published);
> `go run ./cmd/bbs setup` from a checkout works on any OS. Without `bbs` the
> skill reports `BBS_DEGRADED` and uses the calling LLM fallback;
> retain the request, choice and cited reason in the handoff.

# semantic-decision
A reusable step inside the work, independent of its provider. Follow the
[Auto-Decision Framework](../shared/auto-decision-framework.md); mechanical
facts need no model, and User Challenges retain their escalation channel.
Shared refs are filesystem paths beside this skill's directory, so read them
by path, not as `skill://`.

1. Read [the contract](../shared/semantic-decision.md). Take the caller's
   evidence, decision kind, bounded choices and applicable rubric from context
   or files. Keep cited evidence and criteria in a request JSON file; no ticket
   is required. Missing evidence: gather it or return `NEEDS_CONTEXT` only when
   the fact cannot be derived, never silently invent it.
2. Run `bbs semantic-decision --kind <kind> --input <request.json>`.
   `NEEDS_LLM` is the normal default provider: make the judgment in this
   session. `FALLBACK_LLM` uses the same path after a Cloudflare failure or
   restriction. Record the LLM's choice and cited reason per question with
   `--llm-answers <answers.json>` against the same request. Never launch a
   second model for the LLM provider or alter user config to complete the step.
3. Reconcile `DECIDED` answers with evidence and the consumer's mandatory
   floors. If a Cloudflare answer conflicts, record an LLM resolution with the
   discrepancy. Keep the structured result and evidence with the caller's
   artifact; return choices to the caller, which owns execution and permission.

Output: `STATUS: DONE`, `VERDICT: DECIDED`, `PROVIDER: llm|cloudflare`,
`DECISIONS: <result JSON/path>`, `EVIDENCE: <request and cited rationale>`.
An unresolved User Challenge uses the preamble's `NEEDS_CONTEXT` channel.
