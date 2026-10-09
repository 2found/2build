---
name: fix-pr
description: Address unresolved review comments on an open pull request — fix on the PR head branch, reply in-thread, resolve threads, push. Use after a human or bot review leaves comments on a PR.
---
# fix-pr
Address a PR's unresolved review threads; leave disagreements for the reviewer. One repo's PR per invocation — a cross-repo ticket's sibling PR needs its own run in that repo.
## Flow
1. Resolve the PR: `bbs ticket get pointers.pr`, else a PR URL/number from conversation, else the current branch's PR (`gh pr view --json url,number`). None → `NEEDS_CONTEXT` naming what's missing. Resolve `GH_ACCOUNT` the same way create-pr does (`eval "$(bbs secrets load | sed -E '/^export CLOUDFLARE_(ACCOUNT_ID|API_TOKEN)=/d')"` → `gh auth switch -u "$GH_ACCOUNT"` when set).
2. Fetch unresolved threads — GraphQL only; REST cannot list resolution state.
   The query below is the first page, not the complete review: request
   `pageInfo { hasNextPage endCursor }` on both connections and continue with
   their `after` cursors, fetching further comments per thread when needed.
   Do not declare all threads addressed from a truncated response.

   ```bash
   gh api graphql -f query='
   query($owner:String!,$repo:String!,$pr:Int!){
     repository(owner:$owner,name:$repo){
       pullRequest(number:$pr){
         reviewThreads(first:100){ nodes{
           id isResolved isOutdated path line
           comments(first:20){ nodes{ databaseId author{login} body } } } } } } }' \
     -F owner="$OWNER" -F repo="$REPO" -F pr="$NUMBER" \
     --jq '.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved|not)'
   ```
3. Use `../semantic-decision/SKILL.md` kind `review-finding` with each
   candidate, current code/callers and cited disproof to assess
   CONFIRMED/PLAUSIBLE/REFUTED; a reviewer's identity is not evidence.
   Shared refs are filesystem paths beside this skill's directory, so read
   them by path, not as `skill://`.
   Work on the PR's verified head branch. Use its ticket worktree when one
   exists; otherwise the matching current checkout is valid. Do not edit a
   shared composed QA surface. Same discipline as `review-pr --fix`: apply what's right, skip with a stated reason what isn't (wrong, out of scope, or a genuine disagreement — those go back to the reviewer as a reply, not a silent skip).
4. Re-verify fixes before push using `implement`'s checks and `qa` when user
   flows change. Let `qa` own shared-surface preparation and cleanup when in
   a ticket worktree; do not run a second lease/compose protocol here.
5. Commit only the intended fixes and push the verified PR head branch.
6. Close the loop per thread: reply in-thread via REST, resolve via GraphQL (resolution is GraphQL-only):
   ```bash
   gh api "repos/$OWNER/$REPO/pulls/comments/$DATABASE_ID/replies" -f body="<what changed + commit sha, or why skipped>"
   gh api graphql -f query='mutation($t:ID!){ resolveReviewThread(input:{threadId:$t}){ thread{ id } } }' -F t="$THREAD_ID"
   ```
   Skipped-by-disagreement threads get the reply but stay unresolved — the reviewer closes them.
## Rules
- Never force-push; never rewrite published history to "clean up" fix commits.
- Don't resolve a thread you didn't act on or answer.
- Comments demanding out-of-scope work: reply proposing a follow-up ticket, leave unresolved, note it in the summary.
## Output
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: PR_FIXED
PR: <url>
SUMMARY: <n threads addressed, m skipped + why; commits pushed>
NEXT: human re-review
```
