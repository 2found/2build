package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Land records its commit and installs the hook; a push of base carrying it is
// refused until the phase QA passes, then allowed and the record dropped.
func TestPrePushRefusesUnverifiedPhase(t *testing.T) {
	st, _, repo := executionFixture(t)
	land := integrateAndLand(t, st, "p1")
	commit := land.Repos[0].Commit
	hook := gitOutIn(repo, "rev-parse", "--path-format=absolute", "--git-path", "hooks/pre-push")
	if b, err := os.ReadFile(hook); err != nil || !strings.Contains(string(b), "bbs hooks pre-push") {
		t.Fatalf("pre-push hook not installed: %v", err)
	}
	verified := false
	old := prePushPhaseVerified
	prePushPhaseVerified = func(parent, phase string) (bool, error) {
		if parent != st.Env.Ticket || phase != "p1" {
			t.Fatalf("checked %s %s", parent, phase)
		}
		return verified, nil
	}
	t.Cleanup(func() { prePushPhaseVerified = old })
	line := "refs/heads/main " + gitOutIn(repo, "rev-parse", "main") + " refs/heads/main " + strings.Repeat("0", 40) + "\n"
	if code := runPrePushHook(repo, strings.NewReader(line), io.Discard); code != 1 {
		t.Fatalf("unverified phase pushed (exit %d)", code)
	}
	verified = true
	if code := runPrePushHook(repo, strings.NewReader(line), io.Discard); code != 0 {
		t.Fatalf("verified phase refused (exit %d)", code)
	}
	common := gitOutIn(repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if _, err := os.Stat(filepath.Join(common, "bbs-landed", commit)); !os.IsNotExist(err) {
		t.Fatal("verified phase record kept")
	}
}

// A repository's own pre-push hook is never replaced.
func TestLandKeepsAnExistingPrePushHook(t *testing.T) {
	st, _, repo := executionFixture(t)
	hook := gitOutIn(repo, "rev-parse", "--path-format=absolute", "--git-path", "hooks/pre-push")
	mustMkdirAll(t, filepath.Dir(hook))
	mustWrite(t, hook, "#!/bin/sh\nexit 0\n")
	integrateAndLand(t, st, "p1")
	if b, _ := os.ReadFile(hook); string(b) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("existing hook replaced: %q", b)
	}
}
