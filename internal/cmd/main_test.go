package cmd

import (
	"os"
	"testing"
)

// realCloseLandedWorkerTerminals is the production closer, for the tests
// that exercise it against a fake Orca.
var realCloseLandedWorkerTerminals = closeLandedWorkerTerminals

// landClosed collects the worktrees phase cleanup asked Orca to close
// terminals in; tests never reach the developer's real Orca.
var landClosed []string

// TestMain drops an inherited Orca terminal handle: run from an Orca terminal,
// worktree creation would otherwise go to the real app. Tests that need one
// set it with t.Setenv.
func TestMain(m *testing.M) {
	os.Unsetenv("ORCA_TERMINAL_HANDLE")
	closeLandedWorkerTerminals = func(paths []string) []string {
		landClosed = append(landClosed, paths...)
		return []string{}
	}
	os.Exit(m.Run())
}
