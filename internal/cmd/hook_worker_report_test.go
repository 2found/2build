package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/2found/2build/internal/foreman"
)

func TestWorkerReportGate(t *testing.T) {
	for _, tc := range []struct {
		name, terminal, status string
		asked                  bool
		block                  string
	}{
		{name: "outside Orca", status: "dispatched"},
		{name: "ordinary Orca session", terminal: "term-other", status: "dispatched"},
		{name: "report missing", terminal: "term-1", status: "dispatched", block: "bbs foreman task done task-w --dispatch task-w.d1"},
		{name: "question pending", terminal: "term-1", status: "dispatched", asked: true, block: "--resume"},
		{name: "reported", terminal: "term-1", status: "completed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BABYSIT_HOME", t.TempDir())
			t.Setenv("ORCA_TERMINAL_HANDLE", tc.terminal)
			seedTask(t, "task-w", "task-w.d1", tc.status, "term-1", time.Now())
			if tc.asked {
				if _, err := foreman.Ask("task-w", "which?"); err != nil {
					t.Fatal(err)
				}
			}
			// Repeated Stop invocations must still block while the same
			// Dispatch is open; a continuation is not a report.
			for range 2 {
				var out bytes.Buffer
				runWorkerReportGate(&out)
				if tc.block == "" {
					if out.Len() != 0 {
						t.Fatalf("unexpected objection: %s", out.String())
					}
					continue
				}
				var decision map[string]string
				if err := json.Unmarshal(out.Bytes(), &decision); err != nil || decision["decision"] != "block" || !strings.Contains(decision["reason"], tc.block) {
					t.Fatalf("invalid native Stop decision: %s (%v)", out.String(), err)
				}
			}
		})
	}
}
