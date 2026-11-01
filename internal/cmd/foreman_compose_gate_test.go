package cmd

import (
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

// A ticket whose work conflicts with a sibling that already reported still
// reports: the conflict is resolved where the phase composes, so the report
// only warns, naming the sibling and files.
func TestForemanTaskDoneWarnsOnWorkThatDoesNotCompose(t *testing.T) {
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
	if err != nil || !strings.Contains(out, "COMPOSE_WARNING=bs-sib: api/routes.ts, api/index.ts") {
		t.Fatalf("conflicting done: %v %q", err, out)
	}
	if tk, _ := foreman.LoadTask(task); tk.Current().Gate != "ready" {
		t.Fatalf("gate = %q", tk.Current().Gate)
	}
}

// The compose check merges the reporting worktree, uncommitted, against a
// sibling's snapshot without touching the worktree.
func TestWorktreeCommitComposesAgainstASibling(t *testing.T) {
	_, wts := snapshotRepo(t, "a", "b")
	mustWrite(t, filepath.Join(wts["a"], "shared.txt"), "A\ntwo\nthree\n")
	snap, err := sealTicketSnapshot(wts["a"], "a")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(wts["b"], "shared.txt"), "B\ntwo\nthree\n")
	mine, err := worktreeCommit(wts["b"])
	if err != nil {
		t.Fatal(err)
	}
	if _, _, c, err := mergeCommits(wts["b"], mine, []string{snap}); err != nil || strings.Join(c[snap], ",") != "shared.txt" {
		t.Fatalf("conflict not found: %v %v", c, err)
	}
	if gitOutIn(wts["b"], "status", "--porcelain") != "M shared.txt" && gitOutIn(wts["b"], "status", "--porcelain") != " M shared.txt" {
		t.Fatalf("check touched the worktree: %q", gitOutIn(wts["b"], "status", "--porcelain"))
	}
}
