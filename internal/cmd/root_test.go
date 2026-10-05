package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInvalidCommandsReportErrorsBeforeSideEffects(t *testing.T) {
	bin := buildTestBBS(t)
	for _, args := range [][]string{
		{"does-not-exist"}, {"--not-a-flag"}, {"instal"},
		{"update", "typo"}, {"update", "check", "typo"}, {"update", "--snooze", "1", "extra"},
		{"setup", "--typo"}, {"install", "unsupported"}, {"install", "claude", "codex"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			dir := t.TempDir()
			command := exec.Command(bin, args...)
			command.Dir = dir
			command.Env = []string{"HOME=" + dir, "BABYSIT_HOME=" + filepath.Join(dir, "state"), "PATH=" + dir}
			out, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "Error:") || !strings.Contains(string(out), "--help") {
				t.Fatalf("expected actionable failure, got %v: %s", err, out)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid command created state: %v, %v", entries, err)
			}
		})
	}
}

func TestRemovedCommandsRejectWithoutCreatingState(t *testing.T) {
	bin := buildTestBBS(t)
	dir := t.TempDir()
	t.Setenv("BABYSIT_HOME", filepath.Join(dir, "state"))
	t.Setenv("BABYSIT_PROJECT_HOME", filepath.Join(dir, "project"))
	for _, args := range [][]string{
		{"autopilot", "probe"}, {"autopilot", "explain"},
		{"autopilot", "context"}, {"ticket", "get-pointer", "pr"},
		{"agent", "resolve"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := exec.Command(bin, args...)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("removed command succeeded: %s", out)
			}
			if args[0] != "agent" && !strings.Contains(string(out), "unknown subcommand: "+args[1]) {
				t.Fatalf("command failed for a reason other than removal: %s", out)
			}
		})
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected commands created state: %v, %v", entries, err)
	}
}

func TestResolveVersion(t *testing.T) {
	// Injected value wins outright: a release binary must report its tag even
	// when it happens to sit inside a checkout with a different VERSION.
	t.Run("injected wins over VERSION file", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("BABYSIT_DIR", dir)
		defer func(v string) { version = v }(version)
		version = "9.9.9"

		if got := resolveVersion(); got != "9.9.9" {
			t.Errorf("resolveVersion() = %q, want 9.9.9", got)
		}
	})

	// The `bbs setup` path: no ldflags, so the checkout's VERSION is the
	// source, and it must track a `git pull` without a rebuild.
	t.Run("falls back to VERSION file", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(" 1.56.0 \n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("BABYSIT_DIR", dir)
		defer func(v string) { version = v }(version)
		version = ""

		if got := resolveVersion(); got != "1.56.0" {
			t.Errorf("resolveVersion() = %q, want 1.56.0", got)
		}
	})

	// Neither source available — report that honestly rather than inventing a
	// number a user might paste into a bug report.
	t.Run("unknown when no source", func(t *testing.T) {
		t.Setenv("BABYSIT_DIR", filepath.Join(t.TempDir(), "absent"))
		defer func(v string) { version = v }(version)
		version = ""

		if got := resolveVersion(); got != "unknown" {
			t.Errorf("resolveVersion() = %q, want unknown", got)
		}
	})
}

// The guard exists to stop -h/--help from reaching actions with side effects
// (update pulls and relinks, update-check writes state).
func TestGuardHelp(t *testing.T) {
	newCmd := func(ran *bool) *cobra.Command {
		// DisableFlagParsing mirrors every command the guard wraps — and is what
		// keeps cobra's own --help handling out of the way, so this exercises the
		// guard rather than cobra.
		return guardHelp(&cobra.Command{
			Use:                "demo",
			Short:              "demo command",
			DisableFlagParsing: true,
			RunE: func(_ *cobra.Command, _ []string) error {
				*ran = true
				return nil
			},
		})
	}

	for _, flag := range []string{"--help", "-h"} {
		t.Run("intercepts "+flag, func(t *testing.T) {
			ran := false
			c := newCmd(&ran)
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetArgs([]string{flag})

			if err := c.Execute(); err != nil {
				t.Fatalf("Execute() = %v, want nil", err)
			}
			if ran {
				t.Error("action ran; the guard must short-circuit before any side effect")
			}
			if !bytes.Contains(out.Bytes(), []byte("Usage:")) {
				t.Errorf("no usage printed; got %q", out.String())
			}
		})
	}

	// Only a *leading* help flag is intercepted; the rest of the hand-rolled
	// argument contract each ported bin parses must pass through untouched.
	t.Run("passes other args through", func(t *testing.T) {
		ran := false
		c := newCmd(&ran)
		c.SetArgs([]string{"--snooze", "1"})

		if err := c.Execute(); err != nil {
			t.Fatalf("Execute() = %v, want nil", err)
		}
		if !ran {
			t.Error("action did not run for non-help args")
		}
	})
}

// Every command the guard wraps must actually be wrapped — a new side-effecting
// port added to the tree without guardHelp would regress silently.
func TestGuardedCommandsHandleHelp(t *testing.T) {
	guarded := []string{"update", "upgrade", "update-check", "autopilot"}
	for _, name := range guarded {
		t.Run(name, func(t *testing.T) {
			root := NewRootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs([]string{name, "--help"})

			if err := root.Execute(); err != nil {
				t.Fatalf("%s --help = %v, want nil", name, err)
			}
			if !bytes.Contains(out.Bytes(), []byte("Usage:")) {
				t.Errorf("%s --help printed no usage; got %q", name, out.String())
			}
		})
	}
}
