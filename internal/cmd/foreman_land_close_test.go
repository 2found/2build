package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLandClosesOnlyTerminalsInLandedWorktrees(t *testing.T) {
	dir := t.TempDir()
	landed := filepath.Join(dir, "ticket-wt")
	if err := os.Mkdir(landed, 0o755); err != nil {
		t.Fatal(err)
	}
	// Orca reports the physical path; land names the one it recorded.
	link := filepath.Join(dir, "ticket-link")
	if err := os.Symlink(landed, link); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> ` + log + `
case "$1 $2" in
  "terminal list") printf '%s' '{"ok":true,"result":{"terminals":[` +
		`{"handle":"t_worker","worktreePath":"` + landed + `"},` +
		`{"handle":"t_self","worktreePath":"` + landed + `"},` +
		`{"handle":"t_other","worktreePath":"/elsewhere"}]}}' ;;
  *) echo '{"ok":true,"result":{"runtime":{"reachable":true}}}' ;;
esac
`
	bin := filepath.Join(dir, "orca")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCA_CLI_COMMAND", bin)
	t.Setenv("ORCA_TERMINAL_HANDLE", "t_self")

	closed := realCloseLandedWorkerTerminals([]string{link})
	if !slices.Equal(closed, []string{"t_worker"}) {
		t.Fatalf("closed %v, want only the landed worker", closed)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "t_self") || strings.Contains(string(calls), "t_other") {
		t.Fatalf("closed a terminal outside the landed worktree or the caller's own:\n%s", calls)
	}
	if !strings.Contains(string(calls), "terminal close --terminal t_worker --tab") {
		t.Fatalf("worker terminal not closed:\n%s", calls)
	}
}

func TestLandCloseWithoutOrcaIsANoOp(t *testing.T) {
	t.Setenv("ORCA_CLI_COMMAND", filepath.Join(t.TempDir(), "missing-orca"))
	if closed := realCloseLandedWorkerTerminals([]string{t.TempDir()}); len(closed) != 0 {
		t.Fatalf("closed %v without Orca", closed)
	}
}
