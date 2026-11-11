package cmd

import (
	"strings"
	"testing"
)

// Each verb's help is its own entry: the flags a skill passes are in it, and
// the rest of the group is not.
func TestUsageForNarrowsToOneVerb(t *testing.T) {
	cases := []struct {
		text, prefix, verb string
		want               []string
		absent             string
	}{
		{ticketUsageText, "", "ensure", []string{"--no-branch", "--from-input-file", "--mode trunk|worktree"}, "set-pointer"},
		{ticketUsageText, "", "approval", []string{"self-resolve --team", "approval await"}, "ensure"},
		{ticketUsageText, "", "restore", []string{"clear a cancel"}, "clear a pause"},
		{ticketUsageText, "", "surface", []string{"acquire", "--ttl-min"}, "board"},
		{foremanUsage, "bbs team ", "evidence", []string{"--begin --file", "--carry --from"}, "bbs team land"},
		{foremanUsage, "bbs team ", "task", []string{"create|launch"}, "bbs team resource"},
		{foremanTaskUsage, "bbs team task ", "done", []string{"--no-gate", "--outcome"}, "task launch"},
		{autopilotHelp, "bbs autopilot ", "verification", []string{"begin --gate review-pr|qa", "record --attempt"}, "checkpoint"},
	}
	for _, c := range cases {
		got := usageFor(c.text, c.prefix, c.verb)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: missing %q in:\n%s", c.verb, w, got)
			}
		}
		if strings.Contains(got, c.absent) {
			t.Errorf("%s: help still carries %q:\n%s", c.verb, c.absent, got)
		}
	}
	if got := usageFor(foremanUsage, "bbs team ", "no-such-verb"); got != foremanUsage {
		t.Errorf("an unknown verb should get the whole usage")
	}
}
