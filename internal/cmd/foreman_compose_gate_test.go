package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/foreman"
)

func stubCompose(t *testing.T, conflicts map[string][]string) {
	t.Helper()
	old := taskDoneCompose
	taskDoneCompose = func(foreman.Task) (map[string][]string, error) { return conflicts, nil }
	t.Cleanup(func() { taskDoneCompose = old })
}

// A ticket whose work conflicts with a sibling that already reported is
// refused while its worker is warm, naming the sibling to adopt; --no-gate
// still reports.
func TestForemanTaskDoneRefusesWorkThatDoesNotCompose(t *testing.T) {
	fm, _ := taskFixture(t)
	old := taskDoneGate
	taskDoneGate = func(string) (string, []string, error) { return "ready", nil, nil }
	t.Cleanup(func() { taskDoneGate = old })
	stubCompose(t, map[string][]string{"bs-sib": {"api/routes.ts", "api/index.ts"}})

	task := createTask(t, fm, "--kind", "ticket")
	captureStdout(t, func() {
		if err := foremanTask([]string{"launch", task, "--agent", "codex", "--worktree", t.TempDir()}); err != nil {
			t.Fatal(err)
		}
	})
	var err error
	out := captureStdout(t, func() {
		err = foremanTask([]string{"done", task, "--outcome", "succeeded", "--summary", "s"})
	})
	if err == nil || !strings.Contains(out, "GATE=not-composing") || !strings.Contains(out, "CONFLICT_WITH=bs-sib: api/routes.ts, api/index.ts") || !strings.Contains(out, "bbs ticket adopt bs-sib") {
		t.Fatalf("conflicting done: %v %q", err, out)
	}
	if _, status, _ := taskDispatchState(task); status != foreman.DispatchOpen {
		t.Fatalf("refused done settled the dispatch as %s", status)
	}
	stubCompose(t, nil)
	captureStdout(t, func() {
		err = foremanTask([]string{"done", task, "--outcome", "succeeded", "--summary", "s"})
	})
	if tk, _ := foreman.LoadTask(task); err != nil || tk.Current().Gate != "ready" {
		t.Fatalf("composing done: %v", err)
	}
}

// Stale verdicts stand only when every path changed since the reviewed tree
// came verbatim from an adopted snapshot and lies outside the ticket's own diff.
func TestAdoptionCarry(t *testing.T) {
	stale := []string{"review-pr:stale_tree_digest", "qa:stale_tree_digest", "qa:stale_surface"}
	setup := func(t *testing.T) (wt, home, reviewTree string) {
		_, wts := snapshotRepo(t, "a", "b")
		mustWrite(t, filepath.Join(wts["b"], "ui.txt"), "screen\n")
		reviewTree, err := worktreeTree(wts["b"])
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(wts["a"], "api.txt"), "contract\n")
		snap, err := sealTicketSnapshot(wts["a"], "a", nil)
		if err != nil {
			t.Fatal(err)
		}
		if files, err := adoptSnapshots(wts["b"], []string{snap}); err != nil || len(files) != 0 {
			t.Fatalf("adopt = %v %v", files, err)
		}
		home = t.TempDir()
		if err := recordAdopted(home, []string{snap}); err != nil {
			t.Fatal(err)
		}
		return wts["b"], home, reviewTree
	}

	t.Run("disjoint adoption carries", func(t *testing.T) {
		wt, home, rt := setup(t)
		why, ok := adoptionCarry(wt, home, rt, stale)
		if !ok || !strings.Contains(why, "api.txt") {
			t.Fatalf("carry = %q %v", why, ok)
		}
	})
	t.Run("an own edit after review does not", func(t *testing.T) {
		wt, home, rt := setup(t)
		mustWrite(t, filepath.Join(wt, "ui.txt"), "screen v2\n")
		if _, ok := adoptionCarry(wt, home, rt, stale); ok {
			t.Fatal("carried a change to the reviewed diff")
		}
	})
	t.Run("a new unreviewed file does not", func(t *testing.T) {
		wt, home, rt := setup(t)
		mustWrite(t, filepath.Join(wt, "extra.txt"), "x\n")
		if _, ok := adoptionCarry(wt, home, rt, stale); ok {
			t.Fatal("carried a file no snapshot brought")
		}
	})
	t.Run("a deletion does not", func(t *testing.T) {
		wt, home, rt := setup(t)
		if err := os.Remove(filepath.Join(wt, "shared.txt")); err != nil {
			t.Fatal(err)
		}
		if _, ok := adoptionCarry(wt, home, rt, stale); ok {
			t.Fatal("carried a deletion")
		}
	})
	t.Run("other reasons and a moved base do not", func(t *testing.T) {
		wt, home, rt := setup(t)
		for _, codes := range [][]string{
			{"review-pr:stale_tree_digest", "review-pr:material_findings"},
			{"review-pr:stale_base_sha"},
			{"qa:missing"},
			nil,
		} {
			if _, ok := adoptionCarry(wt, home, rt, codes); ok {
				t.Fatalf("carried %v", codes)
			}
		}
		if _, ok := adoptionCarry(wt, home, "", stale); ok {
			t.Fatal("carried without a review_tree")
		}
	})
}

// The compose check merges the reporting worktree, uncommitted, against a
// sibling's snapshot; work the worktree already adopted is shared, not a
// conflict.
func TestWorktreeCommitComposesAgainstASibling(t *testing.T) {
	_, wts := snapshotRepo(t, "a", "b")
	mustWrite(t, filepath.Join(wts["a"], "shared.txt"), "A\ntwo\nthree\n")
	snap, err := sealTicketSnapshot(wts["a"], "a", nil)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(wts["b"], "shared.txt"), "B\ntwo\nthree\n")
	mine, err := worktreeCommit(wts["b"], nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, c, err := mergeCommits(wts["b"], mine, []string{snap}); err != nil || strings.Join(c[snap], ",") != "shared.txt" {
		t.Fatalf("conflict not found: %v %v", c, err)
	}
	if gitOutIn(wts["b"], "status", "--porcelain") != "M shared.txt" && gitOutIn(wts["b"], "status", "--porcelain") != " M shared.txt" {
		t.Fatalf("check touched the worktree: %q", gitOutIn(wts["b"], "status", "--porcelain"))
	}
	if _, err := adoptSnapshots(wts["b"], []string{snap}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(wts["b"], "shared.txt"), "AB\ntwo\nthree\n")
	mine, err = worktreeCommit(wts["b"], []string{snap})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, c, err := mergeCommits(wts["b"], mine, []string{snap}); err != nil || len(c) != 0 {
		t.Fatalf("resolved adoption still conflicts: %v %v", c, err)
	}
}
