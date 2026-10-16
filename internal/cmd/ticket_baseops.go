package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/2found/2build/internal/git"
	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// This file holds the shared git helpers of the ticket base-ops family. Of the
// retired bbs-ticket.bash ops only the `surface` lease survives (ticket_surface.go);
// refresh, compose, serve and land went with the git-flow profiles — workers
// never commit, and Foreman lands one commit per phase.

// ─── dir-aware git + shared helpers ──────────────────────────────────────────

func gitCOut(dir string, args ...string) string {
	return gitOut(append([]string{"-C", dir}, args...)...)
}

func gitCOK(dir string, args ...string) bool {
	return gitOK(append([]string{"-C", dir}, args...)...)
}

// gitCRun is gitCOK plus git's own last line of output, so a caller can say
// what went wrong instead of guessing.
func gitCRun(dir string, args ...string) (bool, string) {
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := c.CombinedOutput()
	last := ""
	for _, ln := range strings.Split(string(out), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			last = ln
		}
	}
	return err == nil, last
}

func insideWorkTree() bool { return gitOK("rev-parse", "--is-inside-work-tree") }

// baseOpsPrimary resolves the primary checkout that surface/serve/land
// all mutate, behind the two guards they all need first. It prints the reason
// and returns "" when there is nothing to work on, so every caller is one
// `if primary == "" { return 2 }`.
func baseOpsPrimary(cmd string) string {
	if !haveGit() {
		fmt.Fprintf(os.Stderr, "%s: git not found\n", cmd)
		return ""
	}
	if !insideWorkTree() {
		fmt.Fprintf(os.Stderr, "%s: not in a git work tree\n", cmd)
		return ""
	}
	if p := gitPrimary(); p != "" {
		return p
	}
	return gitOut("rev-parse", "--show-toplevel")
}

// ticketBranch finds the local branch a ticket was cut on. Under the default
// trunk mode there is none — babysit cut nothing — so the miss is the common
// case, and it prints the BLOCK that says so before returning "".
func ticketBranch(t string) string {
	for _, ln := range strings.Split(gitOut("for-each-ref", "--format=%(refname:short)", "refs/heads/*/"+t+"_*"), "\n") {
		if ln != "" {
			return ln
		}
	}
	fmt.Fprintln(os.Stderr, "STATUS: BLOCKED")
	fmt.Fprintf(os.Stderr, "REASON: no local branch matches '*/%s_*' — unknown ticket or branch never cut.\n", t)
	fmt.Fprintln(os.Stderr, retarget("RECOMMENDATION: check the id (bbs-ticket session list), or run ensure for it first."))
	return ""
}

func haveGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// gitPrimary is the primary checkout of the current repo — the first entry of
// `git worktree list --porcelain`, shared with every other topology read via
// internal/git.
func gitPrimary() string {
	p, _ := git.PrimaryWorktree()
	return p
}

// lockAcquire / lockRelease mirror tests/fixtures/lib/lock.sh: a spin-mkdir mutex at 100ms
// intervals. maxTries=300 ≈ 30s, matching bbs_lock_acquire "$LOCK" 300.
func lockAcquire(lockdir string, maxTries int) bool {
	for i := 0; i < maxTries; i++ {
		if err := os.Mkdir(lockdir, 0o755); err == nil {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func lockRelease(lockdir string) { _ = os.RemoveAll(lockdir) }

// The surface lease lives in ticket_surface.go.

func parseIntOr(s string, def int64) int64 {
	if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
		return n
	}
	return def
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// ─── shared helpers ──────────────────────────────────────────────────────────

// storeForTicket returns a Store bound to a specific ticket id under the same
// project home — the Go form of bash `TICKET=<id> history_append …`.
func storeForTicket(base identity.Env, ticketID string) *ticket.Store {
	e := base
	e.Ticket = ticketID
	return ticket.New(e)
}

// ─── surface ─────────────────────────────────────────────────────────────────

func runSurface(args []string) { os.Exit(surfaceCmd(args)) }

// surfaceCmd manages the shared test-surface lease: one QA session at a
// time holds a repository's ports and services.
//
//	acquire  take the long lease
//	release  drop the lease
//	status   who holds the lease, for how long, under which ttl
func surfaceCmd(args []string) int {
	verb := ""
	if len(args) > 0 {
		verb = args[0]
		args = args[1:]
	}
	switch verb {
	case "acquire", "release", "status":
		return surfaceLease(verb, args)
	default:
		fmt.Fprintln(os.Stderr, retarget("usage: bbs-ticket surface <acquire|release|status> [--ttl-min N] [--force]"))
		return 2
	}
}

// ─── surface lease verbs ─────────────────────────────────────────────────────

// surfaceLease runs the lease verbs of `bbs ticket surface`: acquire, release,
// status — the lease handler under the lifecycle name: same
// lease dir, same owner file, same contention rule.
func surfaceLease(verb string, args []string) int {
	qlTicket, ttl, force := "", "60", false
	needValue := func(flag string, i int) (string, bool) {
		if i+1 >= len(args) {
			fmt.Fprintf(os.Stderr, "surface %s: %s needs a value\n", verb, flag)
			return "", false
		}
		return args[i+1], true
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ticket":
			if verb == "status" {
				fmt.Fprintf(os.Stderr, "surface status: --ticket does not apply\n")
				os.Exit(2)
			}
			v, ok := needValue("--ticket", i)
			if !ok {
				os.Exit(2)
			}
			qlTicket, i = v, i+1
		case "--ttl-min":
			if verb != "acquire" {
				fmt.Fprintf(os.Stderr, "surface %s: --ttl-min only applies to acquire\n", verb)
				os.Exit(2)
			}
			v, ok := needValue("--ttl-min", i)
			if !ok {
				os.Exit(2)
			}
			ttl, i = v, i+1
		case "--force":
			if verb != "release" {
				fmt.Fprintf(os.Stderr, "surface %s: --force only applies to release\n", verb)
				os.Exit(2)
			}
			force = true
		default:
			fmt.Fprintf(os.Stderr, "surface %s: unknown arg '%s'\n", verb, args[i])
			os.Exit(2)
		}
	}
	var env identity.Env
	// status never needs a ticket; release --force overrides ownership;
	// explicit --ticket supplies its own. Only acquire/release paths that
	// use the current ticket as owner need the ladder.
	if qlTicket != "" || verb == "status" || (verb == "release" && force) {
		env = resolveProject()
	} else {
		env = resolveEnv()
	}
	if qlTicket == "" {
		qlTicket = env.Ticket
	}
	if !haveGit() {
		fmt.Fprintln(os.Stderr, "surface: git not found")
		os.Exit(2)
	}
	if !insideWorkTree() {
		fmt.Fprintln(os.Stderr, "surface: not in a git work tree")
		os.Exit(2)
	}
	primary := gitPrimary()
	if primary == "" {
		primary = gitOut("rev-parse", "--show-toplevel")
	}
	gitdir := gitCOut(primary, "rev-parse", "--absolute-git-dir")
	qlDir := leaseDirOf(gitdir)
	read := func(k string) string { return leaseRead(qlDir, k) }

	switch verb {
	case "acquire":
		if qlTicket == "" {
			fmt.Fprintln(os.Stderr, "surface acquire: no owner — no ticket in scope and no --ticket given")
			os.Exit(2)
		}
		// A QA session is a long hold of the same lease every surface op takes,
		// so leaseAcquire waits out a merge already landing before granting it:
		// a lease published in that gap would cover a surface about to change.
		// Waiting is the point — the merge finishes in seconds, and the surface
		// is then genuinely still.
		res := leaseAcquire(qlDir, qlTicket, leaseLong, ttl)
		if b := res.blocked; b != nil {
			fmt.Fprintln(os.Stderr, "STATUS: BLOCKED")
			if b.owner == "" {
				fmt.Fprintln(os.Stderr, "REASON: another run is taking the surface lease right now — one QA session at a time on the shared surface.")
				fmt.Fprintln(os.Stderr, "RECOMMENDATION: re-run acquire in a moment; a lease still ownerless after two minutes is treated as stale and stolen.")
				os.Exit(2)
			}
			if b.kind == leaseShort {
				fmt.Fprintf(os.Stderr, "REASON: '%s' has held the surface for 30s and is still landing — the lease would cover a surface still changing.\n", b.owner)
				fmt.Fprintf(os.Stderr, "RECOMMENDATION: re-run acquire; if that run died, its lease lapses %d minutes after it started.\n", shortTTLMin)
				os.Exit(2)
			}
			fmt.Fprintf(os.Stderr, "REASON: surface lease held by '%s' (%dmin into a %dmin lease) — one QA session at a time on the shared surface.\n", b.owner, b.age, b.ttl)
			fmt.Fprintln(os.Stderr, retarget("RECOMMENDATION: wait and re-run acquire, or 'bbs-ticket surface release --force' if that run is dead."))
			os.Exit(2)
		}
		if res.stoleOwner != "" {
			fmt.Fprintf(os.Stderr, "surface acquire: stole stale lease from '%s' (%s)\n", res.stoleOwner, res.stoleWhy)
			if env.Ticket != "" {
				ticket.New(env).HistoryAppendExtra("surface_lease_steal", actorRole(),
					fmt.Sprintf(`{"owner":"%s","stolen_from":"%s","ttl_min":%s}`, qlTicket, res.stoleOwner, ttl))
			}
			fmt.Printf("OWNER=%s\nTTL_MIN=%s\nACQUIRED=1\nSTOLE_FROM=%s\n", qlTicket, ttl, res.stoleOwner)
			os.Exit(0)
		}
		if res.refreshed {
			fmt.Printf("OWNER=%s\nTTL_MIN=%s\nACQUIRED=1\nREFRESHED=1\n", qlTicket, ttl)
			os.Exit(0)
		}
		if env.Ticket != "" {
			ticket.New(env).HistoryAppendExtra("surface_lease_acquire", actorRole(),
				fmt.Sprintf(`{"owner":"%s","ttl_min":%s}`, qlTicket, ttl))
		}
		fmt.Printf("OWNER=%s\nTTL_MIN=%s\nACQUIRED=1\n", qlTicket, ttl)
		os.Exit(0)
	case "release":
		if fi, err := os.Stat(qlDir); err != nil || !fi.IsDir() {
			fmt.Println("FREE=1")
			os.Exit(0)
		}
		owner := read("owner")
		if !force && owner != "" && owner != qlTicket {
			fmt.Fprintln(os.Stderr, "STATUS: BLOCKED")
			ticketDisp := qlTicket
			if ticketDisp == "" {
				ticketDisp = "<none>"
			}
			fmt.Fprintf(os.Stderr, "REASON: surface lease belongs to '%s', not '%s' — releasing someone else's lease mid-QA corrupts their verdict.\n", owner, ticketDisp)
			fmt.Fprintln(os.Stderr, "RECOMMENDATION: let the owner release it, or pass --force if that run is dead.")
			os.Exit(2)
		}
		_ = os.RemoveAll(qlDir)
		if env.Ticket != "" {
			ticket.New(env).HistoryAppendExtra("surface_lease_release", actorRole(),
				fmt.Sprintf(`{"owner":"%s"}`, orUnknown(owner)))
		}
		fmt.Printf("RELEASED=1\nOWNER=%s\n", orUnknown(owner))
		os.Exit(0)
	case "status":
		if fi, err := os.Stat(qlDir); err != nil || !fi.IsDir() {
			fmt.Println("FREE")
			os.Exit(0)
		}
		age, _ := leaseAge(qlDir)
		fmt.Printf("OWNER=%s\n", read("owner"))
		fmt.Printf("AGE_MIN=%d\n", age)
		fmt.Printf("TTL_MIN=%s\n", read("ttl_min"))
		os.Exit(0)
	}
	return 2
}
