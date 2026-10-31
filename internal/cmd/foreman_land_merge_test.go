package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func outValue(out, key string) string {
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(line, key+"="); ok {
			return v
		}
	}
	return ""
}

// moveMain commits a file on main from a checkout of it.
func moveMain(t *testing.T, repo, file, content string) string {
	t.Helper()
	other := filepath.Join(t.TempDir(), "main")
	runGit(t, repo, "worktree", "add", "-q", other, "main")
	mustWrite(t, filepath.Join(other, file), content)
	runGit(t, other, "add", ".")
	runGit(t, other, "commit", "-qm", "base moves")
	return other
}

// Base moving after integrate is normal: land composes the phase again on
// the new tip and lands it, with no round trip through the lead.
func TestLandReintegratesWhenBaseMoved(t *testing.T) {
	st, _, repo := executionFixture(t)
	integratePhase(t, st, "p1")
	moveMain(t, repo, "hotfix.txt", "hotfix\n")
	moved := gitOutIn(repo, "rev-parse", "main")
	var err error
	out := captureStdout(t, func() { err = projectLandCommand(st, map[string]string{"phase": "p1"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "REINTEGRATED=p1") {
		t.Fatalf("land did not say it re-integrated: %s", out)
	}
	if gitOutIn(repo, "rev-parse", "main^") != moved {
		t.Fatal("phase did not land as one commit on the moved base")
	}
	if gitOutIn(repo, "show", "main:hotfix.txt") != "hotfix" || gitOutIn(repo, "show", "main:feature.txt") != "working journey" {
		t.Fatal("landed commit lost the base change or the phase")
	}
}

// A conflict with base is laid out in the phase worktree, resolved there and
// committed by the next integrate — no repair session.
func TestIntegrateLaysOutConflictsForResolution(t *testing.T) {
	st, _, repo := executionFixture(t)
	moveMain(t, repo, "feature.txt", "base journey\n")
	var err error
	out := captureStdout(t, func() { err = projectIntegrateCommand(st, map[string]string{"phase": "p1"}) })
	if err == nil || !strings.Contains(err.Error(), "laid out") {
		t.Fatalf("conflict not laid out: %v", err)
	}
	wt := outValue(out, "RESOLVE_IN")
	if wt == "" || !strings.Contains(out, "CONFLICT=ready: feature.txt") {
		t.Fatalf("output = %s", out)
	}
	b, _ := os.ReadFile(filepath.Join(wt, "feature.txt"))
	if !strings.Contains(string(b), "<<<<<<< ") {
		t.Fatalf("no markers to resolve: %q", b)
	}
	// Markers left in place refuse the commit.
	captureStdout(t, func() { err = projectIntegrateCommand(st, map[string]string{"phase": "p1"}) })
	if err == nil || !strings.Contains(err.Error(), "conflict markers remain") {
		t.Fatalf("unresolved markers accepted: %v", err)
	}
	mustWrite(t, filepath.Join(wt, "feature.txt"), "base journey\nworking journey\n")
	c := integratePhase(t, st, "p1")
	if len(c.Repos) != 1 || strings.Join(c.Repos[0].Resolved, ",") != "feature.txt" {
		t.Fatalf("resolution not recorded: %+v", c.Repos)
	}
	if got := gitOutIn(repo, "show", c.Repos[0].Commit+":feature.txt"); got != "base journey\nworking journey" {
		t.Fatalf("phase commit lost the resolution: %q", got)
	}
	if !strings.Contains(gitOutIn(repo, "log", "-1", "--format=%B", c.Repos[0].Commit), "Conflicts resolved on integrate: feature.txt") {
		t.Fatal("phase commit does not name the resolved files")
	}
	if _, err := os.Stat(projectResolutionPath(st, "p1")); !os.IsNotExist(err) {
		t.Fatal("pending resolution kept after commit")
	}
	if err := landPhase(t, st, "p1"); err != nil {
		t.Fatal(err)
	}
}

// Local edits in the base checkout that overlap the phase stop the land and
// are listed for the human; once allowed, --commit-local commits them on base
// and the phase lands on top (here after resolving the conflict they make).
func TestLandCommitLocalOnDirtyBase(t *testing.T) {
	st, _, repo := executionFixture(t)
	integratePhase(t, st, "p1")
	main := filepath.Join(t.TempDir(), "main")
	runGit(t, repo, "worktree", "add", "-q", main, "main")
	mustWrite(t, filepath.Join(main, "feature.txt"), "someone's draft\n")
	before := gitOutIn(repo, "rev-parse", "main")

	var err error
	out := captureStdout(t, func() { err = projectLandCommand(st, map[string]string{"phase": "p1"}) })
	if err == nil || !strings.Contains(out, "LOCAL_CHANGES=") || !strings.Contains(out, "feature.txt") || !strings.Contains(out, "--commit-local") {
		t.Fatalf("overlap not reported: %v\n%s", err, out)
	}
	if gitOutIn(repo, "rev-parse", "main") != before {
		t.Fatal("base moved before the human allowed it")
	}

	out = ""
	out = captureStdout(t, func() {
		err = projectLandCommand(st, map[string]string{"phase": "p1", "commit-local": "wip: draft before phase p1"})
	})
	if !strings.Contains(out, "COMMITTED_LOCAL=") || gitOutIn(repo, "log", "-1", "--format=%s", "main") != "wip: draft before phase p1" {
		t.Fatalf("local changes not committed: %v\n%s", err, out)
	}
	if gitOutIn(main, "status", "--porcelain") != "" {
		t.Fatal("base checkout still dirty after commit-local")
	}
	if err == nil || !strings.Contains(err.Error(), "laid out") {
		t.Fatalf("conflict with the committed draft not laid out: %v", err)
	}
	mustWrite(t, filepath.Join(outValue(out, "RESOLVE_IN"), "feature.txt"), "someone's draft\nworking journey\n")
	integratePhase(t, st, "p1")
	if err := landPhase(t, st, "p1"); err != nil {
		t.Fatal(err)
	}
	if got := gitOutIn(repo, "show", "main:feature.txt"); got != "someone's draft\nworking journey" {
		t.Fatalf("landed content = %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(main, "feature.txt")); string(b) != "someone's draft\nworking journey\n" {
		t.Fatalf("base checkout not updated: %q", b)
	}
}

// A fix after land that conflicts with what base gained since is laid out on
// the base tip, resolved in place and lands as one fix commit.
func TestPhaseFixConflictingWithBaseIsResolvedInPlace(t *testing.T) {
	st, child, repo := executionFixture(t)
	integrateAndLand(t, st, "p1")
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "fixed copy\n")
	if err := foremanProjectCommand("seal", []string{"ready", "--fixup", "AC1 copy"}); err != nil {
		t.Fatal(err)
	}
	moveMain(t, repo, "feature.txt", "base edit\n")
	var err error
	out := captureStdout(t, func() { err = projectIntegrateCommand(st, map[string]string{"phase": "p1"}) })
	if err == nil || !strings.Contains(out, "CONFLICT=base: feature.txt") {
		t.Fatalf("fix conflict not laid out: %v\n%s", err, out)
	}
	mustWrite(t, filepath.Join(outValue(out, "RESOLVE_IN"), "feature.txt"), "base edit, fixed copy\n")
	tip := gitOutIn(repo, "rev-parse", "main")
	fix := integratePhase(t, st, "p1")
	if err := landPhase(t, st, "p1"); err != nil {
		t.Fatal(err)
	}
	if gitOutIn(repo, "rev-parse", "main^") != tip || gitOutIn(repo, "show", "main:feature.txt") != "base edit, fixed copy" ||
		!strings.HasPrefix(gitOutIn(repo, "log", "-1", "--format=%s", "main"), "Fix: ") || fix.Repos[0].Whole == "" {
		t.Fatalf("fix did not land as one resolved fix commit: %+v", fix.Repos)
	}
}

// Base drift names only the files a ticket changes that base changed too,
// with different content.
func TestBaseDriftNamesOverlappingFiles(t *testing.T) {
	repo := gitRepo(t, "main")
	for _, f := range []string{"a.txt", "b.txt", "d.txt"} {
		mustWrite(t, filepath.Join(repo, f), "start\n")
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "files")
	wt := filepath.Join(t.TempDir(), "ticket")
	runGit(t, repo, "worktree", "add", "-q", "-b", "feat/x", wt, "main")
	if n, files := baseDrift(wt, "main"); n != 0 || files != nil {
		t.Fatalf("drift on an unmoved base: %d %v", n, files)
	}
	mustWrite(t, filepath.Join(wt, "a.txt"), "mine\n")
	mustWrite(t, filepath.Join(wt, "b.txt"), "mine\n")
	mustWrite(t, filepath.Join(wt, "d.txt"), "same\n")
	mustWrite(t, filepath.Join(repo, "a.txt"), "theirs\n")
	runGit(t, repo, "commit", "-qam", "base moves")
	mustWrite(t, filepath.Join(repo, "c.txt"), "new\n")
	mustWrite(t, filepath.Join(repo, "d.txt"), "same\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "more")
	n, files := baseDrift(wt, "main")
	if n != 2 || strings.Join(files, ",") != "a.txt" {
		t.Fatalf("drift = %d %v", n, files)
	}
}
