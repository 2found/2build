package cmd

import (
	"errors"
	"os"
	"os/exec"
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
