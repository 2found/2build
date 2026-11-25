# File size budget

A large file costs every agent that touches it: it is read whole, held in
context and re-read on every later turn, and an edit to it needs a read first.
A 5,000-line file is ~60k tokens before any work starts. This file owns the
budget; skills link here.

**Budget.** 800 lines per hand-written source file, unless the repo's
instruction entrypoint (`AGENTS.md` / `CLAUDE.md`) states its own
(`File size budget: <n> lines`). Out of scope: generated code, lockfiles,
vendored code, data and fixtures, snapshots, migrations, prototypes.

**Writing (implement).**
- New responsibility → new file, beside the code it belongs with, named for
  what it does. Never append a new feature to a file already past the budget.
- A file under the budget may not cross it, and one already past it may not
  grow, unless the plan accepted that file's growth. Put the new code in a
  new module the big file imports; when the change reworks a region of the
  big file anyway, extracting that region is in scope (log it as a
  Deviation if the plan did not name it).
- Do not split unrelated code just to meet the budget: that is a planned
  refactor ticket, not a side effect.

**Reading (every skill).** In a file over ~400 lines, locate with
`rg -n '<symbol>' <file>` and read the range with offset/limit; never read
it whole. Re-read only the range you are about to edit.

**Planning (plan-draft).** The plan names the new files for new
responsibilities. When a touched file is past the budget, it names the module
the new code goes in, or states why the file must grow.

**Check (mechanical).** Lines added per file and the size after:

```bash
git diff --numstat "$BASE" -- . | while read -r add del f; do
  [ -f "$f" ] && n=$(($(wc -l < "$f"))) && [ "$add" != "-" ] && [ "$add" -gt "$del" ] &&
    [ "$n" -gt "${BUDGET:-800}" ] && echo "OVER $f $n lines (+$((add - del)))"
done
```

Untracked new files: `git ls-files -o --exclude-standard | xargs wc -l`.
An `OVER` line on a hand-written file the plan did not accept is a failed
check.
