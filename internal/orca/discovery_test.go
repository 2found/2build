package orca

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAgentDiscoveryRequiresAdvertisedCapability(t *testing.T) {
	log := fakeOrca(t, `case "$1" in
  status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":["orchestration.contract.v1"]}}}' ;;
  agent-context) echo '{"ok":true,"result":{}}' ;;
esac`)
	client, err := Preflight()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.AgentDiscovery(); !errors.Is(err, ErrNoAgentDiscovery) {
		t.Fatalf("AgentDiscovery error = %v, want capability unavailable", err)
	}
	if strings.Contains(readLog(t, log), "agent-context") {
		t.Fatal("adapter invoked agent-context without the discovery capability")
	}
}

func TestAgentDiscoveryReadsVersionedAgentContext(t *testing.T) {
	log := fakeOrca(t, `case "$1" in
  status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":["agent.discovery.v1","quota.snapshot.v1"]}}}' ;;
  agent-context) echo "$DISCOVERY" ;;
esac`)
	t.Setenv("DISCOVERY", `{"ok":true,"result":{"agentDiscovery":{"schemaVersion":1,"hostId":"host-a","observedAt":"2026-09-27T12:00:00Z","effectiveDefaultAgent":"codex","agents":[{"id":"codex","enabled":true,"runnable":true,"models":["tier-model"],"efforts":["high"],"launchOverrides":["model","effort"]}],"quotaSnapshots":[]}}}`)
	client, err := Preflight()
	if err != nil {
		t.Fatal(err)
	}
	d, err := client.AgentDiscovery()
	if err != nil {
		t.Fatal(err)
	}
	if d.HostID != "host-a" || d.EffectiveDefaultAgent != "codex" || len(d.Agents) != 1 || d.Agents[0].ID != "codex" {
		t.Fatalf("discovery = %+v", d)
	}
	if !strings.Contains(readLog(t, log), "agent-context --json") {
		t.Fatal("adapter did not use the existing agent-context command")
	}
	if _, err := client.QuotaSnapshots(); err != nil {
		t.Fatalf("advertised quota snapshot unavailable: %v", err)
	}
}

func TestParseAgentDiscoveryRejectsOldAndPartialContracts(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"missing contract", `{"commands":[]}`},
		{"old schema", `{"agentDiscovery":{"schemaVersion":0,"hostId":"host-a","observedAt":"2026-09-27T12:00:00Z"}}`},
		{"missing host", `{"agentDiscovery":{"schemaVersion":1,"observedAt":"2026-09-27T12:00:00Z"}}`},
		{"partial agent", `{"agentDiscovery":{"schemaVersion":1,"hostId":"host-a","observedAt":"2026-09-27T12:00:00Z","agents":[{"id":"codex","enabled":true}]}}`},
		{"malformed json", `{"agentDiscovery":`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseAgentDiscovery(json.RawMessage(tc.raw)); err == nil {
				t.Fatal("invalid or partial discovery was accepted")
			}
		})
	}
}

func TestAccountListParsesProviderWindowsWithoutInventingAuthority(t *testing.T) {
	got, err := ParseAccountRateLimits(json.RawMessage(`{"rateLimits":{"codex":{"status":"ok","timestamp":"2026-09-27T12:00:00Z","error":"sensitive provider detail","windows":[{"usedPercent":100,"windowMinutes":60,"resetsAt":"2026-09-27T13:00:00Z"}]},"partial":{"status":"ok","timestamp":"2026-09-27T12:00:00Z","windows":null},"partialMissingWindows":{"status":"ok","timestamp":"2026-09-27T12:00:00Z","error":""},"metadata":{"description":"not a provider record"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Providers) != 3 {
		t.Fatalf("provider records = %+v", got.Providers)
	}
	codex := got.Providers["codex"]
	if len(codex.Windows) != 1 || codex.Windows[0].UsedPercent == nil || *codex.Windows[0].UsedPercent != 100 {
		t.Fatalf("codex limits = %+v", codex)
	}
	if _, ok := got.Providers["metadata"]; ok {
		t.Fatal("rateLimits metadata was treated as provider quota")
	}
	if got.Providers["partial"].Windows != nil {
		t.Fatal("null windows were inferred as available")
	}
	if got.Providers["partialMissingWindows"].Windows != nil {
		t.Fatal("missing windows were inferred as available")
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "sensitive provider detail") {
		t.Fatal("free-form account errors were persisted")
	}
}

func TestAccountListUsesLocalOrcaOnly(t *testing.T) {
	log := fakeOrca(t, `case "$1" in
  status) echo '{"ok":true,"result":{"runtime":{"reachable":true}}}' ;;
  account) echo '{"ok":true,"result":{"rateLimits":{"codex":{"status":"ok","timestamp":"2026-09-27T12:00:00Z","windows":[]}}}}' ;;
esac`)
	client, err := Preflight()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.AccountList(); err != nil {
		t.Fatal(err)
	}
	calls := readLog(t, log)
	if !strings.Contains(calls, "account list --json") {
		t.Fatalf("account list not read through Orca CLI: %q", calls)
	}
}

func TestValidateLaunchReceiptRequiresMatchingEffectiveSettings(t *testing.T) {
	raw := []byte(`{"ok":true,"result":{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":{"agent":"codex","hostId":"host-a","model":"tier-model","effort":"high"}}}}`)
	got, err := ValidateLaunchReceipt(raw, LaunchRequest{AgentID: "codex", HostID: "host-a", Model: "tier-model", Effort: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if got.DispatchID != "ctx-1" || got.Effective.Model == nil || *got.Effective.Model != "tier-model" {
		t.Fatalf("receipt = %+v", got)
	}
	if _, err := ValidateLaunchReceipt(raw, LaunchRequest{AgentID: "codex", HostID: "host-a", Model: "other"}); err == nil {
		t.Fatal("effective model mismatch was accepted")
	}
	if _, err := ValidateLaunchReceipt([]byte(`{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted"}`), LaunchRequest{AgentID: "codex"}); err == nil {
		t.Fatal("missing effective receipt was accepted")
	}
}

func TestQuotaSnapshotsRequireAdvertisedCapability(t *testing.T) {
	log := fakeOrca(t, `case "$1" in
  status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":["agent.discovery.v1"]}}}' ;;
esac`)
	client, err := Preflight()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.QuotaSnapshots(); !errors.Is(err, ErrNoQuotaSnapshots) {
		t.Fatalf("QuotaSnapshots error = %v, want quota capability unavailable", err)
	}
	if strings.Contains(readLog(t, log), "agent-context") {
		t.Fatal("adapter read discovery without quota capability")
	}
}
