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
//
// It also points telemetry at a scratch directory. Tests that build an
// identity.Env by hand never set BABYSIT_STATE_DIR, so their project events
// went to the developer's real ~/.babysit/analytics (97% of its rows by
// 2026-11). Tests that assert on telemetry clear this and set their own state.
func TestMain(m *testing.M) {
	os.Unsetenv("ORCA_TERMINAL_HANDLE")
	analytics, err := os.MkdirTemp("", "bbs-test-analytics-")
	if err != nil {
		panic(err)
	}
	os.Setenv("BABYSIT_ANALYTICS_DIR", analytics)
	closeLandedWorkerTerminals = func(paths []string) []string {
		landClosed = append(landClosed, paths...)
		return []string{}
	}
	code := m.Run()
	os.RemoveAll(analytics)
	os.Exit(code)
}
