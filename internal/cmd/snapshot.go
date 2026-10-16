package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Ticket snapshots. Workers never commit: a ticket's work is whatever is in
// its worktree, verified by content (tree_digest). The CLI captures that
// content as a commit object on a hidden ref — refs/bbs/tickets/<id> — without
// touching the worktree's HEAD, index or branch. Snapshots are plumbing, never
// history: a phase lands as ONE commit built from its tickets' snapshots, and
// no snapshot ever reaches a branch.
//
// A re-snapshot of the same ticket takes the previous snapshot as its parent,
// and snapshots a ticket adopted from its prerequisites become extra parents,
// so git's merge base sees exactly what each side already shares when a phase
// is composed.

const zeroOID = "0000000000000000000000000000000000000000"

func ticketSnapshotRef(id string) string { return "refs/bbs/tickets/" + id }

func phaseCandidateRef(parent, phase string) string {
	return "refs/bbs/phases/" + parent + "/" + phase
}

// gitPlumb runs git in dir with extra environment and returns trimmed stdout.
// stderr is folded into the error so callers can report what git said.
func gitPlumb(dir string, env []string, args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// worktreeTree writes dir's working tree — tracked and untracked files,
// .gitignore honored — as a tree object. It works on a copy of the index, so
// the checkout, its index and every ref stay exactly as they were.
func worktreeTree(dir string) (string, error) {
	tmp, err := os.CreateTemp("", "bbs-index-*")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if real, err := gitPlumb(dir, nil, "rev-parse", "--path-format=absolute", "--git-path", "index"); err == nil {
		// Seeding from the real index keeps its stat cache: `add -A` then
		// rehashes only what changed instead of the whole tree.
		if src, err := os.Open(real); err == nil {
			_, _ = io.Copy(tmp, src)
			src.Close()
		}
	}
	tmp.Close()
	env := []string{"GIT_INDEX_FILE=" + path}
	if fi, _ := os.Stat(path); fi == nil || fi.Size() == 0 {
		os.Remove(path)
		if gitOKIn(dir, "rev-parse", "--verify", "-q", "HEAD") {
			if _, err := gitPlumb(dir, env, "read-tree", "HEAD"); err != nil {
				return "", err
			}
		}
	}
	if _, err := gitPlumb(dir, env, "add", "-A", "--", "."); err != nil {
		return "", err
	}
	return gitPlumb(dir, env, "write-tree")
}

// snapshotIdentity is the fixed author of plumbing commits, so a snapshot
// works in a repo with no user identity and never reads as a person's commit.
var snapshotIdentity = []string{
	"GIT_AUTHOR_NAME=2build", "GIT_AUTHOR_EMAIL=bbs@2build.invalid",
	"GIT_COMMITTER_NAME=2build", "GIT_COMMITTER_EMAIL=bbs@2build.invalid",
}

func commitTree(dir string, env []string, tree, msg string, parents ...string) (string, error) {
	args := []string{"commit-tree", tree, "-m", msg}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	return gitPlumb(dir, env, args...)
}

func treeOf(dir, rev string) string {
	return gitOutIn(dir, "rev-parse", "--verify", "-q", rev+"^{tree}")
}

// sealTicketSnapshot captures dir's working tree on the ticket's hidden ref and
// returns the snapshot commit. An unchanged tree reuses the current snapshot,
// so repeated calls are idempotent.
func sealTicketSnapshot(dir, ticketID string, adopted []string) (string, error) {
	tree, err := worktreeTree(dir)
	if err != nil {
		return "", err
	}
	ref := ticketSnapshotRef(ticketID)
	prev := gitOutIn(dir, "rev-parse", "--verify", "-q", ref+"^{commit}")
	parents := []string{}
	if prev != "" {
		parents = append(parents, prev)
	} else if head := gitOutIn(dir, "rev-parse", "--verify", "-q", "HEAD^{commit}"); head != "" {
		parents = append(parents, head)
	}
	for _, a := range adopted {
		covered := false
		for _, p := range parents {
			covered = covered || gitOKIn(dir, "merge-base", "--is-ancestor", a, p)
		}
		if !covered {
			parents = append(parents, a)
		}
	}
	if prev != "" && treeOf(dir, prev) == tree && len(parents) == 1 {
		return prev, nil
	}
	commit, err := commitTree(dir, snapshotIdentity, tree, "bbs snapshot "+ticketID+" (plumbing; never lands)", parents...)
	if err != nil {
		return "", err
	}
	old := prev
	if old == "" {
		old = zeroOID
	}
	if _, err := gitPlumb(dir, nil, "update-ref", ref, commit, old); err != nil {
		return "", fmt.Errorf("snapshot ref moved concurrently: %w", err)
	}
	return commit, nil
}

// mergeCommits folds commits into a single tree with git's three-way merge,
// starting at first. On conflict the returned tree still exists and carries
// conflict markers; conflicts names the conflicted paths per merged commit.
func mergeCommits(dir, first string, others []string) (tree, tip string, conflicts map[string][]string, err error) {
	tip = first
	tree = treeOf(dir, first)
	conflicts = map[string][]string{}
	for _, other := range others {
		if gitOKIn(dir, "merge-base", "--is-ancestor", other, tip) {
			continue
		}
		c := exec.Command("git", "merge-tree", "--write-tree", "--name-only", "--no-messages", tip, other)
		c.Dir = dir
		out, runErr := c.Output()
		lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		code := 0
		if ee, ok := runErr.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if runErr != nil {
			return "", "", nil, runErr
		}
		if code > 1 || len(lines) == 0 || lines[0] == "" {
			return "", "", nil, fmt.Errorf("git merge-tree %s %s failed (exit %d)", tip, other, code)
		}
		tree = lines[0]
		if code == 1 {
			for _, f := range lines[1:] {
				if f != "" && !slices.Contains(conflicts[other], f) {
					conflicts[other] = append(conflicts[other], f)
				}
			}
		}
		tip, err = commitTree(dir, snapshotIdentity, tree, "bbs compose (plumbing)", tip, other)
		if err != nil {
			return "", "", nil, err
		}
	}
	return tree, tip, conflicts, nil
}

// adoptSnapshots brings other tickets' snapshots into dir's working tree as
// uncommitted changes, three-way merged with whatever the worktree already
// holds. HEAD and the branch never move. Conflicted paths are left with
// markers for the worker to resolve and are returned.
func adoptSnapshots(dir string, commits []string) ([]string, error) {
	own, err := worktreeTree(dir)
	if err != nil {
		return nil, err
	}
	head := gitOutIn(dir, "rev-parse", "--verify", "-q", "HEAD^{commit}")
	if head == "" {
		return nil, fmt.Errorf("adopt needs a checkout with a commit")
	}
	start := head
	if treeOf(dir, head) != own {
		if start, err = commitTree(dir, snapshotIdentity, own, "bbs worktree (plumbing)", head); err != nil {
			return nil, err
		}
	}
	tree, _, conflicts, err := mergeCommits(dir, start, commits)
	if err != nil {
		return nil, err
	}
	if tree != own {
		if _, err := gitPlumb(dir, nil, "read-tree", "--reset", "-u", tree); err != nil {
			return nil, err
		}
		// Back to an index that matches HEAD: the adopted work is the
		// worker's uncommitted change set, exactly like its own edits.
		if _, err := gitPlumb(dir, nil, "reset", "-q"); err != nil {
			return nil, err
		}
	}
	var files []string
	for _, fs := range conflicts {
		for _, f := range fs {
			if !slices.Contains(files, f) {
				files = append(files, f)
			}
		}
	}
	slices.Sort(files)
	return files, nil
}

// adoptedPath records the snapshots a ticket adopted, so its own snapshot
// names them as parents.
func adoptedPath(home string) string { return filepath.Join(home, "adopted.json") }

func readAdopted(home string) []string {
	var out []string
	_ = readProjectJSON(adoptedPath(home), &out)
	return out
}

func recordAdopted(home string, commits []string) error {
	out := readAdopted(home)
	for _, c := range commits {
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return writeJSONAtomic(adoptedPath(home), out)
}
