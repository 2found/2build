package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/ticket"
)

func contractState(t *testing.T, st *ticket.Store, id string) contractStatus {
	t.Helper()
	for _, s := range contractStatuses(st, ".") {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("no pin for %s", id)
	return contractStatus{}
}

// commitOnMain moves base without touching the ticket's checkout.
func commitOnMain(t *testing.T, repo string, files map[string]string) {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "main")
	runGit(t, repo, "worktree", "add", "-q", wt, "main")
	for path, body := range files {
		mustMkdirAll(t, filepath.Dir(filepath.Join(wt, path)))
		mustWrite(t, filepath.Join(wt, path), body)
	}
	runGit(t, wt, "add", "-A")
	runGit(t, wt, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "base moves")
	runGit(t, repo, "worktree", "remove", "--force", wt)
}

func TestContractPinTracksVersionAndGatesTheTicket(t *testing.T) {
	_, child, repo := projectFixture(t)
	mustMkdirAll(t, filepath.Join(repo, ".babysit"))
	mustWrite(t, filepath.Join(repo, ".babysit", "contracts.yaml"), "api:\n  files: [api.yaml, schema/]\n  checks: [\"true\"]\n")
	mustWrite(t, filepath.Join(repo, "api.yaml"), "get /search\n")
	mustMkdirAll(t, filepath.Join(repo, "schema"))
	mustWrite(t, filepath.Join(repo, "schema", "novel.json"), "{}\n")

	if err := runContract([]string{"pin", "missing"}); err == nil {
		t.Fatal("pinned a contract that does not exist")
	}
	captureStdout(t, func() {
		if err := runContract([]string{"pin", "api"}); err != nil {
			t.Fatal(err)
		}
	})
	if s := contractState(t, child, "api"); s.State != "current" || s.Pinned == "" || s.Pinned != s.Here {
		t.Fatalf("fresh pin = %+v", s)
	}
	if r := contractReadinessReasons(child, "."); len(r) != 0 {
		t.Fatalf("a current pin gated the ticket: %v", r)
	}

	// A file added under a directory entry is a new version too.
	mustWrite(t, filepath.Join(repo, "schema", "chapter.json"), "{}\n")
	if s := contractState(t, child, "api"); s.State != "changed" || !strings.Contains(s.Next, "bbs contract pin api") {
		t.Fatalf("edited contract = %+v", s)
	}
	r, err := evaluateReadinessIn(child.Env, "review", repo)
	if err != nil {
		t.Fatal(err)
	}
	if !hasReason(r.ReasonCodes, "contract:api:changed") || r.Ready {
		t.Fatalf("readiness ignored a changed contract: %v", r.ReasonCodes)
	}
	captureStdout(t, func() { _ = runContract([]string{"pin", "api"}) })
	if s := contractState(t, child, "api"); s.State != "current" {
		t.Fatalf("re-pin did not clear: %+v", s)
	}

	// Base has another version: informational, the phase integrates on it.
	commitOnMain(t, repo, map[string]string{".babysit/contracts.yaml": "api:\n  files: [api.yaml]\n", "api.yaml": "get /search?q\n"})
	if s := contractState(t, child, "api"); s.State != "behind" || s.Base == "" {
		t.Fatalf("newer base version = %+v", s)
	}
	if r := contractReadinessReasons(child, "."); len(r) != 0 {
		t.Fatalf("a newer base version gated the ticket: %v", r)
	}

	if err := os.Remove(filepath.Join(repo, ".babysit", "contracts.yaml")); err != nil {
		t.Fatal(err)
	}
	if r := contractReadinessReasons(child, "."); !hasReason(r, "contract:api:gone") {
		t.Fatalf("a removed contract did not gate: %v", r)
	}
}

func TestContractIndexValidation(t *testing.T) {
	dir := t.TempDir()
	mustMkdirAll(t, filepath.Join(dir, ".babysit"))
	for name, body := range map[string]string{
		"no files":     "api: {}\n",
		"outside repo": "api:\n  files: [../secrets]\n",
		"absolute":     "api:\n  files: [/etc/passwd]\n",
		"not yaml":     "api: [\n",
	} {
		mustWrite(t, filepath.Join(dir, contractIndexPath), body)
		if _, err := readContracts(dir, ""); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := os.Remove(filepath.Join(dir, contractIndexPath)); err != nil {
		t.Fatal(err)
	}
	if all, err := readContracts(dir, ""); err != nil || len(all) != 0 {
		t.Fatalf("no index must mean no contracts: %v %v", all, err)
	}
}

func TestIntegrateRefusesAnotherForemansContract(t *testing.T) {
	st, _, repo := executionFixture(t)
	commitOnMain(t, repo, map[string]string{".babysit/contracts.yaml": "feature:\n  owner: fm-other\n  files: [feature.txt]\n"})
	var err error
	captureStdout(t, func() { err = projectIntegrateCommand(st, map[string]string{"phase": "p1"}) })
	if err == nil || !strings.Contains(err.Error(), "feature (owner fm-other)") {
		t.Fatalf("phase changed another Foreman's contract: %v", err)
	}
}

func TestIntegrateRecordsOwnContractChecksForPhaseQA(t *testing.T) {
	st, _, repo := executionFixture(t)
	commitOnMain(t, repo, map[string]string{".babysit/contracts.yaml": "feature:\n  owner: fm-test\n  files: [feature.txt]\n  checks: [\"test -s feature.txt\"]\n"})
	cand := integrateAndLand(t, st, "p1")
	if got := cand.Repos[0].Contracts["feature"]; len(got) != 1 || got[0] != "test -s feature.txt" {
		t.Fatalf("contracts_changed = %v", cand.Repos[0].Contracts)
	}
	s, _ := projectRead(st)
	steps := projectNext(st, s)
	if len(steps) != 1 || steps[0].Do != "verify" || !strings.Contains(steps[0].Why, "feature: test -s feature.txt") {
		t.Fatalf("next after land = %+v", steps)
	}
}
