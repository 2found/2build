package cmd

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// Measured runs (2026-10) spent most of a phase's wall time and tokens on
// cold follow-up sessions: siblings that edited the same seam files and only
// learned so when integrate refused the phase, and refreshes that re-ran a
// full review after adopting work that touched none of the ticket's own files.
// Both are decided mechanically here, while the reporting worker is still warm.

// taskDoneCompose returns the phase siblings this ticket's worktree does not
// merge with, as sibling ticket -> conflicted paths. A sibling counts once its
// own ticket session reported success: the earlier finisher keeps its tree and
// the later one adopts it now, instead of in a cold repair after integrate.
// Tests replace it.
var taskDoneCompose = func(t foreman.Task) (map[string][]string, error) {
	primary := baseOpsPrimary("foreman task done")
	if primary == "" {
		return nil, fmt.Errorf("no primary checkout")
	}
	env, err := ticket.ResolveProject()
	if err != nil {
		return nil, err
	}
	me := storeForTicket(env, t.Ticket)
	parent := ticket.ReadDoc(me.IndexPath()).Get("parent")
	if parent == "" {
		return nil, nil
	}
	dir, canonical, err := ticketWorktree(primary, env, t.Ticket)
	if err != nil {
		return nil, err
	}
	var mine string
	conflicts := map[string][]string{}
	seen := map[string]bool{t.Ticket: true}
	for _, other := range foreman.ListTasks(t.Foreman) {
		sib := other.Ticket
		d := other.Current()
		if seen[sib] || !gatedTaskKinds[other.Kind] || d == nil || d.Status != foreman.DispatchCompleted {
			continue
		}
		seen[sib] = true
		if ticket.ReadDoc(storeForTicket(env, sib).IndexPath()).Get("parent") != parent {
			continue
		}
		if _, c, err := ticketWorktree(primary, env, sib); err != nil || c != canonical {
			continue
		}
		snap, err := prereqSnapshot(primary, env, sib)
		if err != nil {
			continue
		}
		if mine == "" {
			if mine, err = worktreeCommit(dir, readAdopted(me.Home())); err != nil {
				return nil, err
			}
		}
		_, _, merged, err := mergeCommits(dir, mine, []string{snap})
		if err != nil {
			return nil, err
		}
		if files := merged[snap]; len(files) > 0 {
			conflicts[sib] = files
		}
	}
	return conflicts, nil
}

// ticketWorktree is a ticket's worktree and the canonical repo it belongs to.
func ticketWorktree(primary string, env identity.Env, t string) (dir, canonical string, err error) {
	m, err := ticket.ReadManifest(storeForTicket(env, t).ManifestPath())
	if err != nil {
		return "", "", err
	}
	repo := m.FindRepo(func(r ticket.Repo) bool { return r.Worktree != "" })
	if repo == nil {
		return "", "", fmt.Errorf("ticket %s has no worktree", t)
	}
	dir = repo.Worktree
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(primary, dir)
	}
	return dir, repo.Canonical, nil
}

// worktreeCommit captures dir's working tree as an unreferenced commit on
// HEAD and the snapshots it adopted, so a merge sees what it already shares.
func worktreeCommit(dir string, adopted []string) (string, error) {
	tree, err := worktreeTree(dir)
	if err != nil {
		return "", err
	}
	var parents []string
	if head := gitOutIn(dir, "rev-parse", "--verify", "-q", "HEAD^{commit}"); head != "" {
		parents = append(parents, head)
	}
	for _, a := range adopted {
		if gitOKIn(dir, "cat-file", "-e", a+"^{commit}") {
			parents = append(parents, a)
		}
	}
	return commitTree(dir, snapshotIdentity, tree, "bbs worktree (plumbing)", parents...)
}

// carriableGateCodes are the readiness codes an adoption can explain: the
// verdict is intact, only the tree under it moved.
var carriableGateCodes = []string{"stale_tree_digest", "stale_surface"}

// adoptionCarry decides whether stale review-pr/qa verdicts may stand: every
// path changed since the reviewed tree came verbatim from an adopted snapshot
// and none of them is a path this ticket itself changed. A moved base, any
// other reason code, or a missing review_tree is never carried.
func adoptionCarry(dir, home, reviewTree string, codes []string) (string, bool) {
	if reviewTree == "" || len(codes) == 0 {
		return "", false
	}
	for _, code := range codes {
		gate, why, _ := strings.Cut(code, ":")
		if (gate != "review-pr" && gate != "qa") || !slices.Contains(carriableGateCodes, why) {
			return "", false
		}
	}
	adopted := readAdopted(home)
	if len(adopted) == 0 || !gitOKIn(dir, "cat-file", "-e", reviewTree+"^{tree}") {
		return "", false
	}
	current, err := worktreeTree(dir)
	if err != nil {
		return "", false
	}
	moved := nameOnlyDiff(dir, reviewTree, current)
	if len(moved) == 0 {
		return "", false
	}
	own := nameOnlyDiff(dir, "HEAD", reviewTree)
	for _, path := range moved {
		if slices.Contains(own, path) {
			return "", false
		}
		// A deletion is never carried: a snapshot that lacks the path would
		// match it without having removed it.
		want := gitOutIn(dir, "rev-parse", "--verify", "-q", current+":"+path)
		if want == "" {
			return "", false
		}
		verbatim := false
		for _, snap := range adopted {
			verbatim = verbatim || gitOutIn(dir, "rev-parse", "--verify", "-q", snap+":"+path) == want
		}
		if !verbatim {
			return "", false
		}
	}
	shown := moved
	if len(shown) > 5 {
		shown = append(slices.Clone(shown[:5]), fmt.Sprintf("+%d more", len(moved)-5))
	}
	return "adopted work outside the reviewed diff: " + strings.Join(shown, ", "), true
}

func nameOnlyDiff(dir, from, to string) []string {
	out := gitOutIn(dir, "diff", "--name-only", "--no-renames", from, to)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// ticketAdoptionCarry is adoptionCarry for a ticket by id.
func ticketAdoptionCarry(primary string, env identity.Env, t string, codes []string) (string, bool) {
	dir, _, err := ticketWorktree(primary, env, t)
	if err != nil {
		return "", false
	}
	st := storeForTicket(env, t)
	return adoptionCarry(dir, st.Home(), ticket.ReadDoc(st.IndexPath()).Get("pointers.review_tree"), codes)
}
