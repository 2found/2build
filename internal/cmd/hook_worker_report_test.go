package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerReportGate(t *testing.T) {
	for _, tc := range []struct {
		name, terminal, reply string
		block                 bool
	}{
		{"outside Orca", "", `invalid`, false},
		{"ordinary Orca session", "term-1", `{"ok":true,"result":{"messages":[],"count":0}}`, false},
		{"report missing", "term-1", `{"ok":true,"result":{"dispatchId":"ctx-1","messages":[],"count":0}}`, true},
		{"verification unavailable", "term-1", `{"ok":false,"error":{"code":"runtime_unavailable"}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cli := filepath.Join(t.TempDir(), "orca")
			if err := os.WriteFile(cli, []byte("#!/bin/sh\nprintf '%s\\n' \"$BBS_TEST_REPLY\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("ORCA_CLI_COMMAND", cli)
			t.Setenv("ORCA_TERMINAL_HANDLE", tc.terminal)
			t.Setenv("BBS_TEST_REPLY", tc.reply)
			// Repeated Stop invocations must still block while the same
			// Dispatch is active; a continuation is not a delivery receipt.
			for range 2 {
				var out bytes.Buffer
				runWorkerReportGate(&out)
				if !tc.block {
					if out.Len() != 0 {
						t.Fatalf("unexpected objection: %s", out.String())
					}
					continue
				}
				var decision map[string]string
				if err := json.Unmarshal(out.Bytes(), &decision); err != nil || decision["decision"] != "block" || decision["reason"] == "" {
					t.Fatalf("invalid native Stop decision: %s (%v)", out.String(), err)
				}
			}
			if tc.name == "report missing" {
				t.Setenv("BBS_TEST_REPLY", `{"ok":true,"result":{"messages":[],"count":0}}`)
				var out bytes.Buffer
				runWorkerReportGate(&out)
				if strings.TrimSpace(out.String()) != "" {
					t.Fatalf("accepted settlement did not release the stop: %s", out.String())
				}
			}
		})
	}
}
