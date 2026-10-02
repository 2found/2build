package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reallongnguyen/babysit/internal/foreman"
)

func TestWorkerLaunchUsesExplicitAgentModelAndEffort(t *testing.T) {
	fakeOrcaFor(t)
	path := filepath.Join(os.Getenv("BABYSIT_STATE_DIR"), "config.yaml")
	if err := os.WriteFile(path, []byte("worker_agent: grok\nworker_provider: stale-provider\nworker_model: stale-model\nworker_effort: low\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := foremanWorkerCommand([]string{"--prompt", "ship it", "--skill", "autopilot", "--agent", "omp", "--model", "chosen", "--effort", "high"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"omp --auto-approve", "--model 'chosen'", "--thinking 'high'", "'/autopilot ship it'"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
	if strings.Contains(out, "stale-model") || strings.Contains(out, "stale-provider") {
		t.Fatalf("retired preferences affected launch: %s", out)
	}
}

func TestForemanRecoveryPinsHistoricalLaunchSettings(t *testing.T) {
	log, titles := fakeOrcaFor(t)
	path := filepath.Join(os.Getenv("BABYSIT_STATE_DIR"), "config.yaml")
	if err := os.WriteFile(path, []byte("foreman_agent: grok\nforeman_provider: stale-provider\nforeman_model: stale-model\nforeman_effort: low\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BABYSIT_FOREMAN_AGENT", "grok")
	if _, err := foremanSpawn([]string{"fm-settings", "--dir", t.TempDir(), "--agent", "omp", "--model", "original", "--effort", "high"}); err != nil {
		t.Fatal(err)
	}
	rec, err := foreman.Load("fm-settings")
	if err != nil || rec.Agent != "omp" || rec.Model != "original" || rec.Provider != "" || rec.Effort != "high" {
		t.Fatalf("new launch used retired settings: %+v %v", rec, err)
	}
	rec.Provider = "custom"
	if err := foreman.Save(rec); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(titles, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BABYSIT_MODEL", "env-change")
	t.Setenv("BABYSIT_FOREMAN_MODEL", "env-change")
	if _, err := spawnForeman("fm-settings", "", "", ""); err != nil {
		t.Fatal(err)
	}
	calls := readCalls(t, log)
	for _, want := range []string{"--provider 'custom'", "--model 'original'", "--thinking 'high'", "--continue"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("recovery dropped %s:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "env-change") || strings.Contains(calls, "stale-model") {
		t.Fatalf("recovery changed pinned settings: %s", calls)
	}
	if err := os.WriteFile(titles, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := foremanSpawn([]string{"fm-settings", "--model", "conflict"}); err == nil || !strings.Contains(err.Error(), "pinned model") {
		t.Fatalf("expected pinned-model error, got %v", err)
	}
}

func TestCursorSpawnUsesNativeDialectAndColdRecovery(t *testing.T) {
	log, _ := fakeOrcaFor(t)
	if _, err := foremanSpawn([]string{"fm-cursor", "--dir", t.TempDir(), "--agent", "cursor", "--model", "configured-model"}); err != nil {
		t.Fatal(err)
	}
	rec, err := foreman.Load("fm-cursor")
	if err != nil || rec.Agent != "cursor" || rec.Session != "" {
		t.Fatalf("invalid Cursor session handle: %+v %v", rec, err)
	}
	if calls := readCalls(t, log); !strings.Contains(calls, "cursor-agent --yolo --model 'configured-model' '/foreman --foreman-id fm-cursor'") {
		t.Fatalf("wrong Cursor command: %s", calls)
	}
}

func TestSkillRuntimeUsesCurrentAgentNotWorkerSelection(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	t.Setenv("BABYSIT_CURRENT_AGENT", "cursor")
	t.Setenv("BABYSIT_AGENT", "omp")
	if rec := currentSkillRuntimeRecord("implement"); rec.Harness != "cursor" {
		t.Fatalf("launch override reported as current harness: %s", rec.Harness)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("BABYSIT_STATE_DIR"), "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("read-only detection wrote config")
	}
}
