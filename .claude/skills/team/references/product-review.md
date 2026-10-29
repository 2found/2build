# Product evaluator assignment

Input: accepted parent outcome, audience, criterion IDs, design/prototype, exact
integrated revisions, runtime URL/probe, and existing QA evidence. The phase
integration session runs it right after its integration attempt, on the same
runtime ([phases](phases.md#3-cumulative-qa-and-product-review)). Start a
separate read-only worker (`bbs team task create --kind product-review` +
`task launch`, lifecycle per [worker-lifecycle.md](worker-lifecycle.md)) only
when that session cannot: its harness lacks the browser or runtime the review
needs, or the contract names a different evaluator model. No code edits
while evaluating (the integration session fixes findings only after both
attempts are recorded), topology changes, nested workers, release authority,
or approval authority.

Judge the running product from the user's entry point. Complete the first
usable journey without implementation narration, then check the remaining
accepted journeys at final review. Exercise real actions, persisted state after
reload, cross-feature transitions, empty/error/permission states relevant to the
scope, keyboard use and a narrow viewport for UI work. A polished screenshot
with a disconnected action is a material failure.

Use the accepted design and host product as the comparison. Name observable
defects rather than arbitrary aesthetic preferences. Capture steps, expected and
actual behavior, criterion ID, tested SHA/runtime, screenshot/log paths, severity
and the owning seed. Compare against the previously verified candidate after a
repair; avoid repeated cosmetic churn that worsens usability.

Use `../semantic-decision/SKILL.md` kind `review-finding` to classify each
observed finding's severity (`material/minor/nit`) from the criterion and runtime
evidence. A broken criterion stays material; this classification grants no
approval authority and never replaces exercising the journey.

Return checks for every assigned criterion and unresolved findings. `material`
blocks acceptance; `minor` and `nit` are nonblocking only when the criterion still
works. Unavailable runtime or missing evidence is a gap, never PASS. Submit via
the parent's evidence attempt; the lead dispatches fixes to the owning child —
one repair per child carrying all its findings — and re-checks only the
criteria they touch. Stop within the project's work budget and report remaining
gaps explicitly.
