package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reallongnguyen/babysit/internal/config"
	"github.com/reallongnguyen/babysit/internal/foreman"
)

func TestAgentResolveCLIAndOpaqueConfigValues(t *testing.T) {
	fakeOrcaFor(t)
	for key, value := range map[string]string{"worker_agent": "omp", "worker_provider": "custom", "worker_model": "@slow", "worker_effort": "high"} {
		if err := config.Set(key, value); err != nil {
			t.Fatal(err)
		}
		if got, _ := config.Get(key); got != value {
			t.Fatalf("config lost %s: %q", key, got)
		}
	}
	root := NewRootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"agent", "resolve", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["agent"] != "omp" || got["provider"] != "custom" || got["model"] != "@slow" || got["effort"] != "high" || got["skill_prefix"] != "/" {
		t.Fatalf("wrong resolved settings: %s", output.Bytes())
	}
	if err := config.Set("worker_model", "custom: model #literal"); err != nil {
		t.Fatal(err)
	}
	if got, _ := config.Get("worker_model"); got != "custom: model #literal" {
		t.Fatalf("opaque value corrupted: %q", got)
	}
}

func TestWorkerLaunchUsesConfiguredAndFlagPreferences(t *testing.T) {
	fakeOrcaFor(t)
	for key, value := range map[string]string{"worker_agent": "omp", "worker_provider": "custom", "worker_model": "@slow", "worker_effort": "medium"} {
		if err := config.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	out := captureStdout(t, func() {
		if err := foremanWorkerCommand([]string{"--prompt", "ship it", "--skill", "autopilot", "--model", "chosen", "--effort", "high"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"omp --auto-approve", "--provider 'custom'", "--model 'chosen'", "--thinking 'high'", "'/autopilot ship it'"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestForemanRecoveryPinsLaunchPreferences(t *testing.T) {
	log, titles := fakeOrcaFor(t)
	for key, value := range map[string]string{"foreman_agent": "omp", "foreman_provider": "custom", "foreman_model": "@slow", "foreman_effort": "high"} {
		if err := config.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := foremanSpawn([]string{"fm-settings", "--dir", t.TempDir(), "--model", "original"}); err != nil {
		t.Fatal(err)
	}
	rec, err := foreman.Load("fm-settings")
	if err != nil || rec.Model != "original" || rec.Provider != "custom" || rec.Effort != "high" {
		t.Fatalf("preferences not recorded: %+v %v", rec, err)
	}
	if err := os.WriteFile(titles, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.Set("foreman_model", "changed"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BABYSIT_MODEL", "env-change")
	if _, err := spawnForeman("fm-settings", "", "", ""); err != nil {
		t.Fatal(err)
	}
	calls := readCalls(t, log)
	for _, want := range []string{"--provider 'custom'", "--model 'original'", "--thinking 'high'", "--continue"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("recovery dropped %s:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "env-change") || strings.Contains(calls, "--model 'changed'") {
		t.Fatalf("recovery changed settings: %s", calls)
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
