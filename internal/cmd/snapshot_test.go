package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotRepo is a repo on main with one commit and a linked worktree per
// name, each on its own scratch branch cut from main.
func snapshotRepo(t *testing.T, names ...string) (string, map[string]string) {
	t.Helper()
	repo := gitRepo(t, "main")
	mustWrite(t, filepath.Join(repo, "shared.txt"), "one\ntwo\nthree\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "shared")
	wts := map[string]string{}
	for _, n := range names {
		wt := filepath.Join(t.TempDir(), n)
		runGit(t, repo, "worktree", "add", "-q", "-b", "feat/"+n, wt, "main")
		wts[n] = wt
	}
	return repo, wts
}

func TestSealSnapshotsUncommittedWorkWithoutTouchingTheCheckout(t *testing.T) {
	repo, wts := snapshotRepo(t, "a")
	wt := wts["a"]
	head := gitOutIn(wt, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(wt, "new.txt"), "new\n")
	mustWrite(t, filepath.Join(wt, "shared.txt"), "ONE\ntwo\nthree\n")
	runGit(t, wt, "add", "shared.txt") // a staged change survives untouched
	status := gitOutIn(wt, "status", "--porcelain")

	snap, err := sealTicketSnapshot(wt, "a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := gitOutIn(repo, "show", snap+":new.txt"); got != "new" {
		t.Fatalf("snapshot lacks untracked work: %q", got)
	}
	if gitOutIn(wt, "rev-parse", "HEAD") != head || gitOutIn(wt, "status", "--porcelain") != status {
		t.Fatal("sealing moved HEAD or changed the index")
	}
	if gitOutIn(repo, "rev-parse", snap+"^") != head {
		t.Fatal("first snapshot is not parented on the cut commit")
	}
	if again, _ := sealTicketSnapshot(wt, "a", nil); again != snap {
		t.Fatal("re-sealing an unchanged tree made a new snapshot")
	}
	mustWrite(t, filepath.Join(wt, "new.txt"), "newer\n")
	next, err := sealTicketSnapshot(wt, "a", nil)
	if err != nil || next == snap || gitOutIn(repo, "rev-parse", next+"^") != snap {
		t.Fatalf("re-seal is not a child of the previous snapshot: %s %v", next, err)
	}
	if strings.Contains(gitOutIn(repo, "branch", "--contains", next), "feat/a") {
		t.Fatal("a snapshot reached a branch")
	}
}

func TestMergeCommitsReportsConflictsPerSnapshot(t *testing.T) {
	repo, wts := snapshotRepo(t, "a", "b", "c")
	mustWrite(t, filepath.Join(wts["a"], "shared.txt"), "A\ntwo\nthree\n")
	mustWrite(t, filepath.Join(wts["b"], "shared.txt"), "B\ntwo\nthree\n")
	mustWrite(t, filepath.Join(wts["c"], "other.txt"), "c\n")
	snaps := map[string]string{}
	for n, wt := range wts {
		s, err := sealTicketSnapshot(wt, n, nil)
		if err != nil {
			t.Fatal(err)
		}
		snaps[n] = s
	}
	base := gitOutIn(repo, "rev-parse", "main")
	_, _, conflicts, err := mergeCommits(repo, base, []string{snaps["a"], snaps["c"]})
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("disjoint snapshots conflicted: %v %v", conflicts, err)
	}
	_, _, conflicts, err = mergeCommits(repo, base, []string{snaps["a"], snaps["b"], snaps["c"]})
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || strings.Join(conflicts[snaps["b"]], ",") != "shared.txt" {
		t.Fatalf("conflict not attributed to the later snapshot: %v", conflicts)
	}

	// Repair: b adopts a, resolves, reseals with a as a parent; the phase
	// then composes.
	files, err := adoptSnapshots(wts["b"], []string{snaps["a"]})
	if err != nil || strings.Join(files, ",") != "shared.txt" {
		t.Fatalf("adopt conflict = %v %v", files, err)
	}
	if gitOutIn(wts["b"], "rev-parse", "HEAD") != base {
		t.Fatal("adopt moved HEAD")
	}
	mustWrite(t, filepath.Join(wts["b"], "shared.txt"), "AB\ntwo\nthree\n")
	resealed, err := sealTicketSnapshot(wts["b"], "b", []string{snaps["a"]})
	if err != nil {
		t.Fatal(err)
	}
	tree, _, conflicts, err := mergeCommits(repo, base, []string{snaps["a"], resealed, snaps["c"]})
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("repaired phase still conflicts: %v %v", conflicts, err)
	}
	if got := gitOutIn(repo, "cat-file", "-p", tree+":shared.txt"); !strings.HasPrefix(got, "AB") {
		t.Fatalf("composed tree lost the resolution: %q", got)
	}
}

func TestAdoptCarriesPrerequisiteWorkAsUncommittedChanges(t *testing.T) {
	_, wts := snapshotRepo(t, "a", "b")
	mustWrite(t, filepath.Join(wts["a"], "api.txt"), "contract\n")
	snap, err := sealTicketSnapshot(wts["a"], "a", nil)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(wts["b"], "ui.txt"), "screen\n")
	files, err := adoptSnapshots(wts["b"], []string{snap})
	if err != nil || len(files) != 0 {
		t.Fatalf("clean adopt = %v %v", files, err)
	}
	for name, want := range map[string]string{"api.txt": "contract\n", "ui.txt": "screen\n"} {
		if b, _ := os.ReadFile(filepath.Join(wts["b"], name)); string(b) != want {
			t.Fatalf("%s = %q", name, b)
		}
	}
	if gitOutIn(wts["b"], "diff", "--cached", "--name-only") != "" {
		t.Fatal("adopt staged changes; adopted work must stay uncommitted")
	}
}

// A submodule lists its main worktree as the git dir; land must resolve the
// real checkout or every phase path reads as a local edit.
func TestBranchCheckoutResolvesASubmoduleGitDir(t *testing.T) {
	repo, _ := snapshotRepo(t)
	gitDir := filepath.Join(t.TempDir(), "modules", "repo")
	if err := os.MkdirAll(filepath.Dir(gitDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(repo, ".git"), gitDir); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, ".git"), "gitdir: "+gitDir+"\n")
	runGit(t, repo, "config", "core.worktree", repo)
	want, _ := filepath.EvalSymlinks(repo)
	got, _ := filepath.EvalSymlinks(branchCheckout(repo, "main"))
	if got != want {
		t.Fatalf("branchCheckout = %q, want the working tree %q", got, want)
	}
}

func TestFastForwardCheckoutKeepsLocalWork(t *testing.T) {
	setup := func(t *testing.T) (repo, old, next string) {
		repo, _ = snapshotRepo(t)
		old = gitOutIn(repo, "rev-parse", "main")
		mustWrite(t, filepath.Join(repo, "phase.txt"), "phase\n")
		mustWrite(t, filepath.Join(repo, "shared.txt"), "one\ntwo\nTHREE\n")
		tree, err := worktreeTree(repo)
		if err != nil {
			t.Fatal(err)
		}
		next, err = commitTree(repo, nil, tree, "phase", old)
		if err != nil {
			t.Fatal(err)
		}
		runGit(t, repo, "checkout", "-q", "--", "shared.txt")
		os.Remove(filepath.Join(repo, "phase.txt"))
		return repo, old, next
	}

	t.Run("unrelated local edits ride along", func(t *testing.T) {
		repo, old, next := setup(t)
		mustWrite(t, filepath.Join(repo, "f.txt"), "someone's edit\n")
		if err := fastForwardCheckout(repo, "refs/heads/main", old, next); err != nil {
			t.Fatal(err)
		}
		if gitOutIn(repo, "rev-parse", "HEAD") != next {
			t.Fatal("base did not move")
		}
		if b, _ := os.ReadFile(filepath.Join(repo, "f.txt")); string(b) != "someone's edit\n" {
			t.Fatal("local edit lost")
		}
		if got := strings.TrimSpace(gitOutIn(repo, "status", "--porcelain")); got != "M f.txt" {
			t.Fatalf("status after land = %q", got)
		}
	})
	t.Run("the phase's own uncommitted content is accepted", func(t *testing.T) {
		repo, old, next := setup(t)
		mustWrite(t, filepath.Join(repo, "phase.txt"), "phase\n")
		if err := fastForwardCheckout(repo, "refs/heads/main", old, next); err != nil {
			t.Fatal(err)
		}
		if got := gitOutIn(repo, "status", "--porcelain"); got != "" {
			t.Fatalf("trunk checkout not clean after landing its own work: %q", got)
		}
	})
	t.Run("an overlapping edit refuses before anything moves", func(t *testing.T) {
		repo, old, next := setup(t)
		mustWrite(t, filepath.Join(repo, "shared.txt"), "mine\n")
		err := fastForwardCheckout(repo, "refs/heads/main", old, next)
		if err == nil || !strings.Contains(err.Error(), "shared.txt") {
			t.Fatalf("overlap not refused: %v", err)
		}
		if gitOutIn(repo, "rev-parse", "HEAD") != old {
			t.Fatal("base moved despite the refusal")
		}
		if b, _ := os.ReadFile(filepath.Join(repo, "shared.txt")); string(b) != "mine\n" {
			t.Fatal("refusal touched the local edit")
		}
	})
}

func TestLandRefusesWhenBaseMovedSinceIntegrate(t *testing.T) {
	st, _, repo := executionFixture(t)
	integratePhase(t, st, "p1")
	for _, kind := range []string{"integration", "product"} {
		finishExecutionEvidence(t, st, executionEvidence(t, st, kind), kind, false)
	}
	// Someone else lands on base after the phase was built and tested.
	other := filepath.Join(t.TempDir(), "other")
	runGit(t, repo, "worktree", "add", "-q", other, "main")
	mustWrite(t, filepath.Join(other, "hotfix.txt"), "hotfix\n")
	runGit(t, other, "add", ".")
	runGit(t, other, "commit", "-qm", "hotfix")
	err := landPhase(t, st, "p1")
	if err == nil || !strings.Contains(err.Error(), "integrate again") {
		t.Fatalf("land over a moved base: %v", err)
	}
	// Rebuilding gives a new commit; the old QA no longer covers it.
	cand := integratePhase(t, st, "p1")
	if gitOutIn(repo, "rev-parse", cand.Repos[0].Commit+"^") != gitOutIn(repo, "rev-parse", "main") {
		t.Fatal("rebuilt phase commit is not on the moved base")
	}
	if err := landPhase(t, st, "p1"); err == nil {
		t.Fatal("landed a rebuilt phase commit on the old QA")
	}
	for _, kind := range []string{"integration", "product"} {
		finishExecutionEvidence(t, st, executionEvidence(t, st, kind), kind, false)
	}
	if err := landPhase(t, st, "p1"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(other, "hotfix.txt")); string(b) != "hotfix\n" {
		t.Fatal("land lost the hotfix")
	}
	if gitOutIn(repo, "rev-list", "--count", "main~1..main") != "1" || gitOutIn(repo, "rev-parse", "main") != cand.Repos[0].Commit {
		t.Fatal("phase did not land as exactly one commit")
	}
}
