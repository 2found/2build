package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForemanResourceReserveDefersOnlyMappedFreshExhaustion(t *testing.T) {
	home := resourceCLIQuotaFixture(t)
	writeQuotaOrca(t, home, `"orchestration.contract.v1","agent.discovery.v1","quota.snapshot.v1"`, `{"ok":true,"result":{"agentDiscovery":{"schemaVersion":1,"hostId":"host-a","observedAt":"2026-09-27T12:00:00Z","effectiveDefaultAgent":"codex","agents":[{"id":"codex","enabled":true,"runnable":true,"models":["tier-model"],"efforts":["high"],"launchOverrides":["model","effort"]}],"quotaSnapshots":[{"hostId":"host-a","poolId":"pool-1","provider":"codex","agentIds":["codex"],"authoritative":true,"status":"ok","freshness":"fresh","observedAt":"2026-09-27T12:00:00Z","windows":[{"name":"weekly","usedPercent":100,"windowMinutes":10080,"resetsAt":"2026-09-28T12:00:00Z"}]}]}}}`, `{"ok":true,"result":{"rateLimits":{"codex":{"status":"ok","timestamp":"2026-09-27T12:00:00Z","windows":[]}}}}`)
	out := captureStdout(t, func() {
		if err := foremanResource([]string{"reserve", "fm-a", "--ticket", "bs-quota", "--task", "task-quota", "--profile", "standard", "--agent", "codex", "--host", "host-a"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "ADMISSION=deferred\n") || !strings.Contains(out, "QUOTA_STATUS=deferred\n") || !strings.Contains(out, "QUOTA_RECHECK_AT=2026-09-28T12:00:00Z\n") || strings.Contains(out, "LEASE=") {
		t.Fatalf("exhausted quota admission = %q", out)
	}
	if strings.Contains(out, "GLOBAL_USED=") {
		t.Fatal("quota deferral touched the CPU/RAM reservation broker")
	}
	matches, err := filepath.Glob(filepath.Join(home, "projects", "*", "tickets", "bs-quota", "handoffs", "*-foreman-quota.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("quota handoff files = %v, %v", matches, err)
	}
	body, err := os.ReadFile(matches[0])
	if err != nil || !strings.Contains(string(body), `"authoritative": true`) || !strings.Contains(string(body), "2026-09-28T12:00:00Z") {
		t.Fatalf("quota evidence handoff = %q, %v", body, err)
	}
}

func TestForemanResourceReserveKeepsUnknownQuotaUnderExistingLimits(t *testing.T) {
	home := resourceCLIQuotaFixture(t)
	writeQuotaOrca(t, home, `"orchestration.contract.v1","agent.discovery.v1"`, `{"ok":true,"result":{"agentDiscovery":{"schemaVersion":1,"hostId":"host-a","observedAt":"2026-09-27T12:00:00Z","effectiveDefaultAgent":"codex","agents":[{"id":"codex","enabled":true,"runnable":true}],"quotaSnapshots":[]}}}`, `{"ok":true,"result":{"rateLimits":{"codex":{"status":"ok","timestamp":"2026-09-27T12:00:00Z","windows":[]}}}}`)
	out := captureStdout(t, func() {
		if err := foremanResource([]string{"reserve", "fm-a", "--ticket", "bs-unknown", "--task", "task-unknown", "--profile", "standard", "--agent", "codex", "--host", "host-a"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "ADMISSION=reserved\n") || !strings.Contains(out, "QUOTA_STATUS=unknown\n") || !strings.Contains(out, "QUOTA_REASON=quota-snapshots-unavailable\n") {
		t.Fatalf("unknown quota admission = %q", out)
	}
}

func TestForemanResourceReserveDoesNotDeferAnActiveDispatch(t *testing.T) {
	home := resourceCLIQuotaFixture(t)
	writeQuotaOrca(t, home, `"orchestration.contract.v1","agent.discovery.v1","quota.snapshot.v1"`, `{"ok":true,"result":{"agentDiscovery":{"schemaVersion":1,"hostId":"host-a","observedAt":"2026-09-27T12:00:00Z","effectiveDefaultAgent":"codex","agents":[{"id":"codex","enabled":true,"runnable":true}],"quotaSnapshots":[{"hostId":"host-a","poolId":"pool-1","provider":"codex","agentIds":["codex"],"authoritative":true,"status":"ok","freshness":"fresh","observedAt":"2026-09-27T12:00:00Z","windows":[{"name":"weekly","usedPercent":100,"windowMinutes":10080,"resetsAt":"2026-09-28T12:00:00Z"}]}]}}}`, `{"ok":true,"result":{"rateLimits":{}}}`)
	t.Setenv("BBS_DISPATCH", `{"ok":true,"result":{"dispatch":{"id":"ctx-active","status":"dispatched"}}}`)
	out := captureStdout(t, func() {
		if err := foremanResource([]string{"reserve", "fm-a", "--ticket", "bs-active", "--task", "task-active", "--profile", "standard", "--agent", "codex", "--host", "host-a"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "ADMISSION=reserved\n") || !strings.Contains(out, "QUOTA_STATUS=not-applicable\n") || !strings.Contains(out, "dispatch-not-new-admission") || !strings.Contains(out, "LEASE=") {
		t.Fatalf("active-dispatch reservation = %q", out)
	}
}

func resourceCLIQuotaFixture(t *testing.T) string {
	t.Helper()
	resourceCLIFixture(t)
	home := os.Getenv("BABYSIT_HOME")
	t.Setenv("BABYSIT_PROJECT_HOME", filepath.Join(home, "projects", "quota-test"))
	return home
}

func writeQuotaOrca(t *testing.T, home, caps, discovery, accounts string) {
	t.Helper()
	path := filepath.Join(home, "bin", "orca")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	stub := `#!/bin/sh
case "$1" in
  status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":[` + caps + `]}}}' ;;
  agent-context) printf '%s\n' "$BBS_DISCOVERY" ;;
  account) printf '%s\n' "$BBS_ACCOUNTS" ;;
  orchestration)
    case "$2" in
      dispatch-show) printf '%s\n' "$BBS_DISPATCH" ;;
    esac ;;
esac
`
	if err := os.WriteFile(path, []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BBS_DISPATCH", `{"ok":true,"result":{"dispatch":null}}`)
	t.Setenv("BBS_DISCOVERY", discovery)
	t.Setenv("BBS_ACCOUNTS", accounts)
	t.Setenv("ORCA_CLI_COMMAND", path)
}
