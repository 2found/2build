package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func routeCommandFixture(t *testing.T, caps string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("BABYSIT_HOME", home)
	t.Setenv("BABYSIT_STATE_DIR", home)
	t.Setenv("BABYSIT_PROJECT_HOME", filepath.Join(home, "projects", "route-test"))
	t.Setenv("BABYSIT_TICKET", "")
	t.Setenv("BBS_TICKET", "")
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	stub := `#!/bin/sh
case "$1" in
  status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":[` + caps + `]}}}' ;;
  agent-context) printf '%s\n' "$DISCOVERY" ;;
  account) printf '%s\n' "$ACCOUNTS" ;;
  orchestration)
    case "$2" in
      dispatch-show) echo '{"ok":true,"result":{"dispatch":null}}' ;;
    esac ;;
esac
`
	path := filepath.Join(bin, "orca")
	if err := os.WriteFile(path, []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCA_CLI_COMMAND", path)
	return home
}

func TestForemanRoutePersistsExplicitRouteWhenDiscoveryContractIsMissing(t *testing.T) {
	home := routeCommandFixture(t, `"orchestration.contract.v1"`)
	out := captureStdout(t, func() {
		if err := foremanRoute([]string{
			"--ticket", "bs-child", "--task", "task-1", "--agent", "codex",
			"--pinned-agent", "codex", "--pinned-model", "tier-model", "--pinned-effort", "high",
			"--model", "tier-model", "--effort", "high", "--host", "host-a", "--exact-session",
			"--complexity", "hard", "--phase-class", "critical", "--selected-tier", "max",
			"--override-provenance", "none", "--pinned-model-provenance", "reviewer_model",
			"--pinned-effort-provenance", "reviewer_effort",
		}); err != nil {
			t.Fatal(err)
		}
	})
	var got routeHandoff
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("route output %q: %v", out, err)
	}
	if got.Route.Agent != "codex" || got.Route.Source != "explicit" || got.Route.Discovery != "unavailable" ||
		got.Route.RequestedHost != "host-a" || got.Route.HostID != "" || got.Handoff == "" ||
		got.Route.Complexity != "hard" || got.Route.PhaseClass != "critical" || got.Route.SelectedTier != "max" ||
		got.Route.OverrideProvenance != "none" || got.Route.PinnedModel != "tier-model" ||
		got.Route.PinnedEffort != "high" || got.Route.PinnedModelProvenance != "reviewer_model" ||
		got.Route.PinnedEffortProvenance != "reviewer_effort" || !got.Route.ExactSession {
		t.Fatalf("route evidence = %+v", got)
	}
	contents, err := os.ReadFile(got.Handoff)
	if err != nil || !strings.Contains(string(contents), `"task": "task-1"`) ||
		!strings.Contains(string(contents), `"source": "explicit"`) ||
		!strings.Contains(string(contents), `"complexity": "hard"`) ||
		!strings.Contains(string(contents), `"phaseClass": "critical"`) ||
		!strings.Contains(string(contents), `"selectedTier": "max"`) ||
		!strings.Contains(string(contents), `"overrideProvenance": "none"`) ||
		!strings.Contains(string(contents), `"pinnedModel": "tier-model"`) ||
		!strings.Contains(string(contents), `"pinnedModelProvenance": "reviewer_model"`) ||
		!strings.Contains(string(contents), `"pinnedEffort": "high"`) ||
		!strings.Contains(string(contents), `"pinnedEffortProvenance": "reviewer_effort"`) ||
		!strings.Contains(string(contents), `"exactSession": true`) {
		t.Fatalf("durable route handoff = %q, %v", contents, err)
	}
	if !strings.HasPrefix(got.Handoff, filepath.Join(home, "projects")) {
		t.Fatalf("handoff escaped configured babysit home: %s", got.Handoff)
	}
}

func TestForemanRouteUsesDestinationDefaultAndPersistsHandoff(t *testing.T) {
	home := routeCommandFixture(t, `"agent.discovery.v1"`)
	t.Setenv("DISCOVERY", `{"ok":true,"result":{"agentDiscovery":{"schemaVersion":1,"hostId":"host-a","observedAt":"2026-09-27T12:00:00Z","effectiveDefaultAgent":"codex","agents":[{"id":"codex","enabled":true,"runnable":true,"models":["tier-model"],"efforts":["high"],"launchOverrides":["model","effort"]}]}}}`)
	out := captureStdout(t, func() {
		if err := foremanRoute([]string{"--ticket", "bs-child", "--task", "task-default", "--model", "tier-model", "--effort", "high", "--host", "host-a"}); err != nil {
			t.Fatal(err)
		}
	})
	var got routeHandoff
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("route output %q: %v", out, err)
	}
	if got.Route.Agent != "codex" || got.Route.Source != "orca-default" || got.Route.HostID != "host-a" ||
		got.Route.Model != "tier-model" || got.Route.Effort != "high" || got.Route.ExactSession {
		t.Fatalf("default route evidence = %+v", got.Route)
	}
	contents, err := os.ReadFile(got.Handoff)
	if err != nil || !strings.Contains(string(contents), `"exactSession": false`) ||
		!strings.HasPrefix(got.Handoff, filepath.Join(home, "projects")) {
		t.Fatalf("default route handoff = %s, %v", got.Handoff, err)
	}
}

func TestForemanRouteBlocksUnpinnedSelectionWithoutContractA(t *testing.T) {
	routeCommandFixture(t, `"orchestration.contract.v1"`)
	writeRetiredAgentConfig(t, "worker_model: legacy-model\n")
	var err error
	stderr := captureStderr(t, func() {
		err = foremanRoute([]string{"--ticket", "bs-child", "--task", "task-2"})
	})
	if err == nil || !strings.Contains(err.Error(), "agent.discovery.v1") || !strings.Contains(err.Error(), "orca agent-context --json") {
		t.Fatalf("missing discovery error = %v", err)
	}
	if got := strings.Count(stderr, "legacy worker/foreman agent settings are ignored"); got != 1 {
		t.Fatalf("got %d retirement diagnostics after failed route, want one: %q", got, stderr)
	}
}

func TestForemanRouteVerifyPersistsMatchingReceiptAndRateLimit(t *testing.T) {
	home := routeCommandFixture(t, `"orchestration.contract.v1"`)
	receipt := filepath.Join(home, "receipt.json")
	if err := os.WriteFile(receipt, []byte(`{"ok":true,"result":{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":{"agent":"codex","hostId":"host-a","model":"tier-model","effort":"high"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := foremanRouteVerify([]string{"--ticket", "bs-child", "--task", "task-3", "--agent", "codex", "--host", "host-a", "--model", "tier-model", "--effort", "high", "--receipt-file", receipt}); err != nil {
			t.Fatal(err)
		}
	})
	var got launchHandoff
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("receipt output %q: %v", out, err)
	}
	if got.Verification != "matched" || got.Receipt == nil || got.Receipt.DispatchID != "ctx-1" {
		t.Fatalf("receipt evidence = %+v", got)
	}
	if _, err := os.Stat(got.Handoff); err != nil {
		t.Fatalf("launch handoff not persisted: %v", err)
	}

	out = captureStdout(t, func() {
		if err := foremanRouteVerify([]string{"--ticket", "bs-child", "--task", "task-4", "--agent", "codex", "--rate-limited"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, `"verification":"rate-limited"`) || !strings.Contains(out, "re-read quota snapshots before retry") {
		t.Fatalf("rate-limit evidence = %q", out)
	}
}

func TestForemanRouteVerifyNativeOMPReceipt(t *testing.T) {
	for _, tc := range []struct {
		name      string
		state     string
		stage     string
		model     string
		wantError string
	}{
		{name: "native defaults", state: "ready", stage: "input_accepted"},
		{name: "selected model lacks evidence", state: "ready", stage: "input_accepted", model: "selected-model", wantError: "effective model is unknown"},
		{name: "failed readiness", state: "failed", stage: "input_accepted", wantError: "not ready"},
		{name: "input not accepted", state: "ready", stage: "input_pending", wantError: "not ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := routeCommandFixture(t, `"orchestration.contract.v1"`)
			receipt := filepath.Join(home, "receipt.json")
			raw := `{"ok":true,"result":{"dispatchId":"ctx-native","state":"` + tc.state + `","stage":"` + tc.stage + `","launch":{"effective":{"agent":"omp","model":null,"effort":null}},"prompt":{"requestId":"private-request"},"effects":[{"id":"private-terminal"}]}}`
			if err := os.WriteFile(receipt, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"--ticket", "bs-child", "--task", "task-native", "--agent", "omp", "--receipt-file", receipt}
			if tc.model != "" {
				args = append(args, "--model", tc.model)
			}
			var verifyErr error
			out := captureStdout(t, func() {
				verifyErr = foremanRouteVerify(args)
			})
			var got launchHandoff
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("receipt output %q: %v", out, err)
			}
			if tc.wantError == "" {
				if verifyErr != nil || got.Verification != "matched" || got.Receipt == nil {
					t.Fatalf("native receipt rejected: %+v, %v", got, verifyErr)
				}
				if got.Receipt.DispatchID != "ctx-native" || got.Receipt.Effective.AgentID == nil || *got.Receipt.Effective.AgentID != "omp" ||
					got.Receipt.Effective.Model != nil || got.Receipt.Effective.Effort != nil {
					t.Fatalf("native evidence = %+v", got.Receipt)
				}
			} else if verifyErr == nil || !strings.Contains(verifyErr.Error(), tc.wantError) ||
				got.Verification != "mismatch" || got.Receipt != nil || !strings.Contains(got.Reason, tc.wantError) {
				t.Fatalf("invalid receipt not rejected: %+v, %v", got, verifyErr)
			}
			persisted, err := os.ReadFile(got.Handoff)
			if err != nil || !strings.Contains(string(persisted), `"verification": "`+got.Verification+`"`) {
				t.Fatalf("durable launch handoff = %q, %v", persisted, err)
			}
			if strings.Contains(string(persisted), "private-request") || strings.Contains(string(persisted), "private-terminal") {
				t.Fatalf("launch handoff leaked arbitrary response fields: %q", persisted)
			}
		})
	}
}
