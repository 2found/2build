package cmd

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/2found/2build/internal/git"
	"github.com/2found/2build/internal/ticket"
)

// This file ports the `ensure` subcommand of the retired bbs-ticket.bash: the
// idempotent ticket-creation entry point. On a branch that already encodes a
// ticket it is a no-op that optionally seeds requirement.md; otherwise it
// derives a new ticket id + slug and cuts the branch per git-flow mode (trunk /
// branch / worktree), guarded by the safe-cut gate. It is the only bbs-ticket
// subcommand that mutates git, so the differential harness exercises it inside a
// throwaway repo.

var ensureTypeOK = map[string]bool{"feat": true, "fix": true, "chore": true, "bug": true, "refactor": true}

func runEnsure(args []string) {
	fromInput, slugHint, typ, promptReason := "", "", "feat", ""
	forceNoBranch := false
	modeFlag, fromRef := "", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from-input":
			v := valueAt(args, i)
			// Foot-gun guard: a short single-line value that is an existing file
			// almost always means "use this file's contents" — refuse with a hint.
			if fi, err := os.Stat(v); err == nil && !fi.IsDir() && !strings.Contains(v, "\n") && len(v) < 256 {
				fmt.Fprintf(os.Stderr, "ensure: --from-input got '%s' which is an existing file path.\n", v)
				fmt.Fprintf(os.Stderr, "  Did you mean: --from-input-file '%s'\n", v)
				fmt.Fprintln(os.Stderr, "  (--from-input takes literal text; use --from-input-file for paths)")
				os.Exit(2)
			}
			fromInput, i = v, i+1
		case "--from-input-file":
			if b, err := os.ReadFile(valueAt(args, i)); err == nil {
				fromInput = strings.TrimRight(string(b), "\n")
			}
			i++
		case "--slug-hint":
			slugHint, i = valueAt(args, i), i+1
		case "--type":
			typ, i = valueAt(args, i), i+1
		case "--reason":
			promptReason, i = valueAt(args, i), i+1
		case "--no-branch":
			forceNoBranch = true
		case "--mode":
			modeFlag, i = valueAt(args, i), i+1
		case "--from-ref":
			fromRef, i = valueAt(args, i), i+1
		default:
			if strings.HasPrefix(args[i], "--mode=") {
				modeFlag = strings.TrimPrefix(args[i], "--mode=")
			}
		}
	}

	if !ensureTypeOK[typ] {
		fmt.Fprintf(os.Stderr, "ensure: invalid --type '%s' (feat|fix|chore|bug|refactor)\n", typ)
		os.Exit(2)
	}
	switch modeFlag {
	case "", "trunk", "worktree":
	default:
		fmt.Fprintf(os.Stderr, "ensure: invalid --mode '%s' (trunk|worktree)\n", modeFlag)
		os.Exit(2)
	}

	env := resolveEnv()

	// Fast-path: branch already encodes a ticket.
	if env.Ticket != "" {
		st := ticket.New(env)
		reqPath := filepath.Join(st.Home(), "requirement.md")
		if fromInput != "" && !fileExists(reqPath) {
			st.EnsureDirs()
			_ = os.WriteFile(reqPath, []byte(fromInput+"\n"), 0o644)
			acquireOrDie(st)
			doc := st.LoadForMutate()
			doc.Set("pointers.requirement", "requirement.md")
			werr := ticket.WriteDoc(st.IndexPath(), doc)
			st.ReleaseLock()
			if werr != nil {
				fmt.Fprintln(os.Stderr, werr)
				os.Exit(1)
			}
			st.HistoryAppendExtra("requirement_seeded", actorRole(), "")
			fmt.Printf("TICKET=%s\n", env.Ticket)
			fmt.Printf("BRANCH=%s\n", env.Branch)
			fmt.Printf("TICKET_HOME=%s\n", st.Home())
			fmt.Println("CREATED=0")
			fmt.Printf("REQUIREMENT=%s\n", reqPath)
		} else {
			fmt.Printf("TICKET=%s\n", env.Ticket)
			fmt.Printf("BRANCH=%s\n", env.Branch)
			fmt.Printf("TICKET_HOME=%s\n", st.Home())
			fmt.Println("CREATED=0")
			if fileExists(reqPath) {
				fmt.Printf("REQUIREMENT=%s\n", reqPath)
			}
		}
		os.Exit(0)
	}

	// Slow-path: create a ticket branch.
	if !haveGit() {
		fmt.Fprintln(os.Stderr, "ensure: git not found — cannot create ticket branch")
		os.Exit(2)
	}
	if !insideWorkTree() {
		fmt.Fprintln(os.Stderr, "ensure: not in a git work tree — cannot create ticket branch")
		os.Exit(2)
	}

	// trunk (default): the ticket rides the current checkout. worktree: a
	// dedicated linked worktree on a scratch branch cut from local base (or
	// --from-ref). Workers never commit, so the branch never moves; the
	// ticket's work is its worktree's uncommitted tree.
	gfMode := modeFlag
	if gfMode == "" || forceNoBranch {
		gfMode = "trunk"
	}

	newTicket := newTicketID()
	ticketID := ""
	branch := ""
	baseBranch := ""
	ensureWorktree, ensureCanonical := "", "."

	if gfMode == "trunk" {
		ticketID = newTicket
		branch = env.Branch
		if branch == "unknown" || branch == "" {
			if b := gitOut("branch", "--show-current"); b != "" {
				branch = b
			} else {
				branch = "unknown"
			}
		}
		fmt.Fprintf(os.Stderr, "ensure: mode=trunk — %s stays on branch '%s' (no branch cut)\n", ticketID, branch)
		fmt.Fprintf(os.Stderr, "ensure: identity is env-carried; run: export BABYSIT_TICKET=%s\n", ticketID)
	} else {
		newSlug := ""
		if slugHint != "" {
			newSlug = deriveSlug(slugHint)
		}
		if newSlug == "" && fromInput != "" {
			newSlug = deriveSlug(fromInput)
		}
		if newSlug == "" {
			fmt.Fprintln(os.Stderr, "ensure: refusing to cut a worktree with no slug — pass --from-input or --slug-hint")
			os.Exit(2)
		}
		newBranch := fmt.Sprintf("%s/%s_%s", typ, newTicket, newSlug)
		baseBranch = baseBranchIn("")
		// Local base: a phase's tickets start from what the previous phase
		// landed, which origin lacks until a release pushes it.
		srcRef := fromRef
		if srcRef == "" {
			for _, ref := range []string{"refs/heads/" + baseBranch, "refs/remotes/origin/" + baseBranch} {
				if gitOK("rev-parse", "--verify", "--quiet", ref+"^{commit}") {
					srcRef = ref
					break
				}
			}
		}
		if srcRef == "" || !gitOK("rev-parse", "--verify", "--quiet", srcRef+"^{commit}") {
			fmt.Fprintf(os.Stderr, "ensure: cannot cut from '%s' — not a commit (base branch '%s')\n", orDefault(srcRef, baseBranch), baseBranch)
			os.Exit(2)
		}
		ensTop := gitOut("rev-parse", "--show-toplevel")
		if ensTop == "" {
			ensTop, _ = os.Getwd()
		}
		ensureWorktree = filepath.Join(ensTop, ".babysit", "worktrees", newTicket+"_"+newSlug)
		ensureCanonical = ensTop
		if _, err := os.Lstat(ensureWorktree); err == nil {
			fmt.Fprintf(os.Stderr, "ensure: worktree path %s already exists — remove it or resume the ticket there.\n", ensureWorktree)
			os.Exit(2)
		}
		excludeWorktreesIn(ensTop)
		_ = os.MkdirAll(filepath.Dir(ensureWorktree), 0o755)
		// `git worktree add` is not safe to run concurrently in one repo:
		// two of them race on .git/worktrees/<name>/commondir and one dies
		// reading the other's half-built directory. A foreman dispatching a
		// batch is exactly that shape, so serialize the add on its own lock.
		// Fail open on timeout: a leaked lock dir would otherwise block every
		// new ticket in the repo for good.
		wtLock := filepath.Join(gitCommonDir(), "bbs-worktree-add.lock")
		locked := lockAcquire(wtLock, 300)
		addOK := gitOK("worktree", "add", "--no-track", "-b", newBranch, "--", ensureWorktree, srcRef)
		if locked {
			lockRelease(wtLock) // os.Exit skips defers — release by hand
		}
		if !addOK {
			fmt.Fprintf(os.Stderr, "ensure: git worktree add failed (%s → %s)\n", srcRef, ensureWorktree)
			os.Exit(2)
		}
		ticketID = newTicket
		branch = newBranch
		fmt.Fprintf(os.Stderr, "ensure: worktree ready at %s (scratch branch %s at %s)\n", ensureWorktree, newBranch, srcRef)
		fmt.Fprintln(os.Stderr, "ensure: current checkout untouched — cd into the worktree before doing any work")
	}

	// Init the ticket under one lock.
	stEnv := env
	stEnv.Ticket = ticketID
	stEnv.Branch = branch
	st := ticket.New(stEnv)
	st.EnsureDirs()
	acquireOrDie(st)
	doc := st.LoadForMutate()
	doc.EnsureDefaults(ticketID)
	doc.Set("id", ticketID)
	doc.Set("pointers.branch", branch)
	reqPath := ""
	if fromInput != "" {
		reqPath = filepath.Join(st.Home(), "requirement.md")
		_ = os.WriteFile(reqPath, []byte(fromInput+"\n"), 0o644)
		doc.Set("pointers.requirement", "requirement.md")
	}
	werr := ticket.WriteDoc(st.IndexPath(), doc)

	// Seed manifest.yaml if absent (non-fatal on failure).
	mPath := st.ManifestPath()
	var mErr error
	if werr == nil && !fileExists(mPath) {
		repos := []ticket.Repo{{
			Name:      orDefault(stEnv.Slug, "repo"),
			Branch:    branch,
			Canonical: ensureCanonical, // absolute for a worktree: seal and integrate resolve the repo from it
			Worktree:  orDefault(ensureWorktree, "."),
			Base:      orDefault(baseBranch, "main"),
			Pushed:    "false",
		}}
		mErr = ticket.WriteManifest(mPath, ticketID, "", repos)
	}
	st.ReleaseLock()
	if werr != nil {
		fmt.Fprintln(os.Stderr, werr)
		os.Exit(1)
	}

	// History after the index write (bash appends inside the locked section, but
	// history.jsonl is a separate append-only file — order relative to the lock
	// doesn't affect its contents).
	initExtra := ""
	if promptReason != "" {
		b, _ := json.Marshal(map[string]string{"reason": promptReason})
		initExtra = string(b)
	}
	st.HistoryAppendExtra("ticket_initialized", actorRole(), initExtra)
	if fromInput != "" {
		st.HistoryAppendExtra("requirement_seeded", actorRole(), "")
	}
	if mErr != nil {
		fmt.Fprintln(os.Stderr, "ensure: warning — manifest.yaml seed failed (continuing)")
	}

	fmt.Printf("TICKET=%s\n", ticketID)
	fmt.Printf("BRANCH=%s\n", branch)
	fmt.Printf("TICKET_HOME=%s\n", st.Home())
	fmt.Println("CREATED=1")
	if ensureWorktree != "" {
		fmt.Printf("WORKTREE=%s\n", ensureWorktree)
	}
	if reqPath != "" {
		fmt.Printf("REQUIREMENT=%s\n", reqPath)
	}
	if gfMode == "trunk" {
		fmt.Printf("export BABYSIT_TICKET=%s\n", ticketID)
	}
	os.Exit(0)
}

// gitCommonDir returns the repository's shared git dir — the one holding
// worktrees/, so every checkout of the repo agrees on the path — resolved for
// the process cwd via internal/git. When git cannot answer (not a repo) the
// historical ".git" fallback stands: the worktree-add lock then lands in
// cwd/.git, only as wrong as it always was.
func gitCommonDir() string {
	if d := git.CommonDirIn(""); d != "" {
		return d
	}
	if abs, err := filepath.Abs(".git"); err == nil {
		return abs
	}
	return ".git"
}

// newTicketID mirrors bash: bs-<8 lowercase-alnum> from crypto rand, epoch
// fallback.
func newTicketID() string {
	const cs = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		for i := range b {
			b[i] = cs[int(b[i])%len(cs)]
		}
		return "bs-" + string(b)
	}
	s := fmt.Sprintf("%d", time.Now().Unix())
	if len(s) > 8 {
		s = s[len(s)-8:]
	}
	return "bs-" + s
}

// deriveSlug mirrors the bash derive_slug pipeline: lowercase, non-[a-z0-9] →
// space, first ≤5 words joined by '-', truncate to 48 bytes, strip trailing '-'.
func deriveSlug(raw string) string {
	lower := strings.ToLower(raw)
	var sb strings.Builder
	for i := 0; i < len(lower); i++ {
		c := lower[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			sb.WriteByte(c)
		} else {
			sb.WriteByte(' ')
		}
	}
	words := strings.Fields(sb.String())
	if len(words) > 5 {
		words = words[:5]
	}
	joined := strings.Join(words, "-")
	if len(joined) > 48 {
		joined = joined[:48]
	}
	return strings.TrimRight(joined, "-")
}

// excludeWorktreesIn excludes .babysit/worktrees/ locally in dir's repository
// so worktrees nested there never show as untracked work.
func excludeWorktreesIn(dir string) {
	excl := gitOutIn(dir, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
	if excl == "" {
		return
	}
	if b, err := os.ReadFile(excl); err == nil {
		for _, ln := range strings.Split(string(b), "\n") {
			// TrimRight "\r": core.autocrlf=true checkouts carry CRLF.
			if strings.TrimRight(ln, "\r") == ".babysit/worktrees/" {
				return
			}
		}
	}
	_ = os.MkdirAll(filepath.Dir(excl), 0o755)
	if f, err := os.OpenFile(excl, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.WriteString(".babysit/worktrees/\n")
		f.Close()
	}
}
