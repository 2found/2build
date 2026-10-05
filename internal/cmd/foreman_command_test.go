package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestForemanMailboxRemoved(t *testing.T) {
	if verb := os.Getenv("BBS_TEST_FOREMAN_MAILBOX"); verb != "" {
		_ = dispatchForeman([]string{"mailbox", verb})
		return
	}
	for _, verb := range []string{"status", "bind", "dispatch", "wait", "reply", "done"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestForemanMailboxRemoved$")
		cmd.Env = append(os.Environ(), "BBS_TEST_FOREMAN_MAILBOX="+verb)
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(out), "unknown subcommand 'mailbox'") {
			t.Fatalf("mailbox %s should be unavailable, got %v: %s", verb, err, out)
		}
	}
	if strings.Contains(foremanUsage, "mailbox") {
		t.Fatal("help still advertises the removed mailbox command")
	}
}

func TestWorkerStartupRejectsTaskInputBeforeDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, flag, value string
	}{
		{"prompt", "--prompt", "ship it"},
		{"empty prompt", "--prompt", ""},
		{"skill", "--skill", "autopilot"},
		{"empty skill", "--skill", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			out := captureStdout(t, func() {
				err = foremanWorkerCommand([]string{"--startup-only", tc.flag, tc.value, "--agent", "omp"})
			})
			if err == nil || !strings.Contains(err.Error(), "--startup-only cannot be combined with "+tc.flag) {
				t.Fatalf("expected task-input conflict, got %v", err)
			}
			if out != "" {
				t.Fatalf("conflicting flags emitted a launch command: %q", out)
			}
		})
	}
}

func TestWorkerStartupLeavesNativeOMPIdleWithLiteralSettings(t *testing.T) {
	for _, tc := range []struct {
		name, model, effort string
	}{
		{"model alias", "@slow", ""},
		{"model and effort", "@slow", "high"},
		{"shell metacharacters", "provider/model'; $(printf injected > injected) `printf injected > injected`", "high"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeOrcaFor(t)
			routeCommandFixture(t, `"agent.discovery.v1"`)
			t.Setenv("DISCOVERY", `{"ok":true,"result":{"agentDiscovery":{"schemaVersion":1,"hostId":"host-a","observedAt":"2026-10-05T00:00:00Z","effectiveDefaultAgent":"omp","agents":[{"id":"omp","enabled":true,"runnable":true,"launchOverrides":[]}]}}}`)
			dir := t.TempDir()
			t.Setenv("EXPECTED_MODEL", tc.model)
			t.Setenv("EXPECTED_EFFORT", tc.effort)
			t.Setenv("READY_FILE", filepath.Join(dir, "ready"))
			// This consumer refuses any positional input, including an empty
			// prompt, rather than accepting a task before Orca's Dispatch.
			script := `#!/bin/sh
approved=no
model_set=no
effort=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --auto-approve) approved=yes; shift ;;
    --model)
      [ "$#" -ge 2 ] && [ "$2" = "$EXPECTED_MODEL" ] || exit 21
      model_set=yes; shift 2 ;;
    --thinking)
      [ "$#" -ge 2 ] && [ "$2" = "$EXPECTED_EFFORT" ] || exit 22
      effort=$2; shift 2 ;;
    *) printf '%s\n' 'unexpected task input or option' >&2; exit 23 ;;
  esac
done
[ "$approved" = yes ] && [ "$model_set" = yes ] && [ "$effort" = "$EXPECTED_EFFORT" ] || exit 24
printf '%s' idle > "$READY_FILE"
`
			if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "omp"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			args := []string{"--startup-only", "--agent", "omp", "--model", tc.model, "--dir", dir, "--host", "host-a"}
			if tc.effort != "" {
				args = append(args, "--effort", tc.effort)
			}
			var renderErr error
			command := captureStdout(t, func() {
				renderErr = foremanWorkerCommand(args)
			})
			if renderErr != nil {
				t.Fatal(renderErr)
			}
			process := exec.Command("/bin/sh", "-c", command)
			process.Dir = dir
			if out, err := process.CombinedOutput(); err != nil {
				t.Fatalf("native startup rejected command: %v: %s", err, out)
			}
			state, err := os.ReadFile(os.Getenv("READY_FILE"))
			if err != nil || string(state) != "idle" {
				t.Fatalf("process did not reach idle state: %q, %v", state, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "injected")); !os.IsNotExist(err) {
				t.Fatalf("model shell syntax was executed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(os.Getenv("BABYSIT_STATE_DIR"), "config.yaml")); !os.IsNotExist(err) {
				t.Fatalf("startup rendering modified global config: %v", err)
			}
		})
	}
}

func TestWorkerStartupRefusesFailedPreflight(t *testing.T) {
	for _, tc := range []struct {
		name, agent, want string
	}{
		{"missing binary", "omp", "on PATH"},
		{"untrusted directory", "grok", "trust"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeOrcaFor(t)
			if tc.agent == "omp" {
				if err := os.Remove(filepath.Join(os.Getenv("PATH"), "omp")); err != nil {
					t.Fatal(err)
				}
			}
			var err error
			out := captureStdout(t, func() {
				err = foremanWorkerCommand([]string{"--startup-only", "--agent", tc.agent, "--dir", t.TempDir()})
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected preflight refusal mentioning %q, got %v", tc.want, err)
			}
			if out != "" {
				t.Fatalf("failed preflight emitted a launch command: %q", out)
			}
		})
	}
}

func TestWorkerStartupCannotOverrideExactSessionSettings(t *testing.T) {
	fakeOrcaFor(t)
	var err error
	out := captureStdout(t, func() {
		err = foremanWorkerCommand([]string{
			"--startup-only", "--exact-session", "--pinned-agent", "omp",
			"--pinned-model", "@slow", "--model", "different-model",
		})
	})
	if err == nil || !strings.Contains(err.Error(), "do not match the recorded route") {
		t.Fatalf("startup bypassed exact-session pin: %v", err)
	}
	if out != "" {
		t.Fatalf("conflicting exact-session settings emitted a launch command: %q", out)
	}
}
