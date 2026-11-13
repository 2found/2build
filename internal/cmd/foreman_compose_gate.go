package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// Measured runs (2026-10) spent most of a phase's wall time and tokens on
// cold follow-up sessions for siblings that edited the same seam files. The
// overlap is reported here, while the reporting worker is still warm; the lead
// resolves it where the phase composes.

// taskDoneCompose returns the phase siblings this ticket's worktree does not
// merge with, as sibling ticket -> conflicted paths. A sibling counts once its
// own ticket session reported success. Tests replace it.
var taskDoneCompose = func(t foreman.Task) (map[string][]string, error) {
	primary := baseOpsPrimary("foreman task done")
	if primary == "" {
		return nil, fmt.Errorf("no primary checkout")
	}
	env, err := ticket.ResolveProject()
	if err != nil {
		return nil, err
	}
	parent := ticket.ReadDoc(storeForTicket(env, t.Ticket).IndexPath()).Get("parent")
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
		if seen[sib] || !buildsTicket(other.Kind) || d == nil || d.Status != foreman.DispatchCompleted {
			continue
		}
		seen[sib] = true
		if ticket.ReadDoc(storeForTicket(env, sib).IndexPath()).Get("parent") != parent {
			continue
		}
		if _, c, err := ticketWorktree(primary, env, sib); err != nil || c != canonical {
			continue
		}
		snap, err := ticketSnapshot(primary, env, sib)
		if err != nil {
			continue
		}
		if mine == "" {
			if mine, err = worktreeCommit(dir); err != nil {
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

// ticketSnapshot captures a sibling ticket's worktree on its snapshot ref.
func ticketSnapshot(primary string, env identity.Env, t string) (string, error) {
	dir, _, err := ticketWorktree(primary, env, t)
	if err != nil {
		return "", err
	}
	return sealTicketSnapshot(dir, t)
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

// worktreeCommit captures dir's working tree as an unreferenced commit on HEAD.
func worktreeCommit(dir string) (string, error) {
	tree, err := worktreeTree(dir)
	if err != nil {
		return "", err
	}
	var parents []string
	if head := gitOutIn(dir, "rev-parse", "--verify", "-q", "HEAD^{commit}"); head != "" {
		parents = append(parents, head)
	}
	return commitTree(dir, snapshotIdentity, tree, "bbs worktree (plumbing)", parents...)
}

func nameOnlyDiff(dir, from, to string) []string {
	out := gitOutIn(dir, "diff", "--name-only", "--no-renames", from, to)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}
