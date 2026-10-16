package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// This file is the surface lease: one QA session at a time on a resource the
// tickets of a repo share (a dev database, a fixed port, a device) —
// acquisition, reentrancy, staleness and release of <gitdir>/bbs-qa-lease.
// `bbs ticket surface acquire|release|status` in ticket_baseops.go drive it.

// ─── the surface lease ───────────────────────────────────────────────────────
//
// One lock guards the shared surface: the lease at <gitdir>/bbs-qa-lease. A QA
// session holds it long; a short lease is a brief op that must not interleave
// with a session.
//
// There used to be a second mutex (bbs-merge-base.lock) for the merge itself,
// with the lease layered over it as an advisory guard. Guard and action then
// lived in different critical sections, so every check had to be repeated once
// the lock was finally held, and three separate races came out of that seam.
// One lock has no seam to race in.
//
// The single contention rule: **wait out a short holder, block on a long one.**
// A merge lands in seconds, so waiting for it costs nothing and leaves the
// surface genuinely still. A QA session runs for as long as someone is testing,
// so the operator is told whose lease it is instead of hanging.
const (
	leaseShort = "short" // one surface op
	leaseLong  = "long"  // a whole QA session
	// shortTTLMin bounds how long a surface op may hold the lease. The lock it
	// replaces had no timeout at all, so a single killed merge wedged every
	// ticket in the repo until someone deleted the directory by hand. A merge
	// takes seconds; five minutes is slack for a slow fetch on a large repo,
	// and a holder past it is dead.
	shortTTLMin = 5
	// leaseWait is how long to wait out a short holder before blocking.
	leaseWait = 30 * time.Second
)

// ─── the lease directory ─────────────────────────────────────────────────────

// ownerlessGrace bounds how long a lease directory may sit without a readable
// owner before another run may take it over. leasePublish installs a lease
// atomically, so a live acquire never leaves one — only a lease written by an
// older bbs (mkdir, then write) or a corrupt one can. Two minutes is an
// enormous margin next to the microseconds a publish takes, and it keeps a
// broken lease from wedging the pool until someone runs release --force.
const ownerlessGrace = 2 * time.Minute

func leaseDirOf(gitdir string) string { return filepath.Join(gitdir, "bbs-qa-lease") }

func leaseBodyFor(owner, kind, ttl string) string {
	return fmt.Sprintf("owner=%s\nkind=%s\npid=%d\nsince=%s\nsince_epoch=%d\nttl_min=%s\n",
		owner, kind, os.Getpid(), isoNow(), time.Now().Unix(), ttl)
}

// leaseStale reports whether the lease in dir may be taken over, and the
// reason to print. An unreadable owner is NOT staleness on its own: reading
// it as stale is what let two runs each claim the one test surface — a racer
// that caught the lease before its owner file landed deleted it and published
// its own, and both printed ACQUIRED=1.
func leaseStale(dir, owner string, age, ttl int64) (bool, string) {
	if owner == "" {
		if fi, err := os.Stat(dir); err != nil || time.Since(fi.ModTime()) > ownerlessGrace {
			return true, "lease has no readable owner"
		}
		return false, ""
	}
	if age > ttl {
		return true, fmt.Sprintf("%dmin > %dmin ttl", age, ttl)
	}
	return false, ""
}

// leasePublish installs body as the lease at dir, atomically: the owner file
// is written inside a temp directory that is then renamed into place, so the
// lease never exists on disk without its owner. rename(2) fails with ENOTEMPTY
// when dir already holds a published lease, which is what makes this the
// mutual-exclusion primitive — a bare os.Mkdir published an empty directory
// that every other racer read as abandoned.
func leasePublish(dir, body string) bool {
	tmp, err := os.MkdirTemp(filepath.Dir(dir), filepath.Base(dir)+".tmp-")
	if err != nil {
		return false
	}
	if err := os.WriteFile(filepath.Join(tmp, "owner"), []byte(body), 0o644); err != nil {
		_ = os.RemoveAll(tmp)
		return false
	}
	if err := os.Rename(tmp, dir); err != nil {
		_ = os.RemoveAll(tmp)
		return false
	}
	return true
}

// leaseRead returns the first `key=…` value from <dir>/owner (sed -n
// 's/^key=//p' | head -1).
func leaseRead(dir, key string) string {
	b, err := os.ReadFile(filepath.Join(dir, "owner"))
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(ln, key+"="); ok {
			return v
		}
	}
	return ""
}

// leaseKind reads a lease's kind. One written by an older bbs has no kind= at
// all — read it as long, which is what every lease meant back then.
func leaseKind(dir string) string { return orDefault(leaseRead(dir, "kind"), leaseLong) }

// leaseRewrite replaces the owner of a lease we already hold. Write-then-rename
// so a concurrent reader never sees the truncated file and mistakes this lease
// for an ownerless one.
func leaseRewrite(dir, body string) {
	tmp := filepath.Join(dir, ".owner.tmp")
	if os.WriteFile(tmp, []byte(body), 0o644) == nil {
		_ = os.Rename(tmp, filepath.Join(dir, "owner"))
	}
}

// leaseAge returns how many minutes the lease has stood, and its declared ttl.
func leaseAge(dir string) (age, ttl int64) {
	now := time.Now().Unix()
	return (now - parseIntOr(leaseRead(dir, "since_epoch"), now)) / 60,
		parseIntOr(leaseRead(dir, "ttl_min"), 60)
}

// leaseResult is what an acquire attempt came to. blocked non-nil means the
// lease is someone else's and the caller should report and give up; the caller
// owns the wording, since "would change the surface mid-QA" and "one QA session
// at a time" are the same fact told to two different audiences.
type leaseResult struct {
	blocked    *leaseHolder
	refreshed  bool   // the lease was already ours
	stoleOwner string // non-empty when a stale lease was taken over
	stoleWhy   string
}

type leaseHolder struct {
	owner    string // "" = a peer is publishing right now
	kind     string
	age, ttl int64
}

// leaseAcquire takes the surface lease at dir for owner, as kind.
func leaseAcquire(dir, owner, kind, ttl string) leaseResult {
	deadline := time.Now().Add(leaseWait)
	for {
		if leasePublish(dir, leaseBodyFor(owner, kind, ttl)) {
			return leaseResult{}
		}
		held := leaseRead(dir, "owner")
		// Ours already — but only a long lease is reentrant. Our own *short*
		// lease is a peer op mid-merge (same ticket, another process), and
		// stepping on it would run two merges in one checkout.
		if held != "" && held == owner && leaseKind(dir) == leaseLong {
			if kind == leaseLong {
				// A QA session refreshing itself. A surface op that finds the
				// session's lease leaves the body alone — rewriting it would
				// cut the QA ttl down to a merge's.
				leaseRewrite(dir, leaseBodyFor(owner, kind, ttl))
			}
			return leaseResult{refreshed: true}
		}
		age, heldTTL := leaseAge(dir)
		if stale, _ := leaseStale(dir, held, age, heldTTL); stale {
			// One attempt, then report whatever is there. Retrying a steal in a
			// loop is how a run spins for as long as a peer keeps re-taking it.
			if stoleFrom, why, ok := leaseSteal(dir, owner, kind, ttl); ok {
				return leaseResult{stoleOwner: stoleFrom, stoleWhy: why}
			}
			age, heldTTL = leaseAge(dir)
			return leaseResult{blocked: &leaseHolder{leaseRead(dir, "owner"), leaseKind(dir), age, heldTTL}}
		}
		if heldKind := leaseKind(dir); heldKind == leaseLong || time.Now().After(deadline) {
			return leaseResult{blocked: &leaseHolder{held, heldKind, age, heldTTL}}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// leaseSteal takes over a lease already judged stale. Read-then-replace needs
// its own mutex: without one, every run that read the same stale lease deletes
// it and publishes, and each reports a successful steal. The re-check under the
// lock catches a peer that got there first.
func leaseSteal(dir, owner, kind, ttl string) (stoleFrom, why string, ok bool) {
	stealLock := dir + ".steal"
	if !lockAcquire(stealLock, 50) {
		return "", "", false
	}
	defer lockRelease(stealLock)
	held := leaseRead(dir, "owner")
	age, heldTTL := leaseAge(dir)
	stale, reason := leaseStale(dir, held, age, heldTTL)
	if !stale {
		return "", "", false
	}
	_ = os.RemoveAll(dir)
	if !leasePublish(dir, leaseBodyFor(owner, kind, ttl)) {
		return "", "", false
	}
	return orUnknown(held), reason, true
}
