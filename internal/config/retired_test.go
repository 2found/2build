package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetRejectsRetiredAgentSettingsWithoutChangingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BABYSIT_STATE_DIR", dir)
	path := filepath.Join(dir, "config.yaml")
	before := []byte("# preserve legacy bytes\ntelemetry: off\nworker_model: old\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}

	for key := range retiredAgentSettings {
		if err := Set(key, "new-value"); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("Set(%q) error = %v, want retired-key error", key, err)
		}
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("config after rejected writes = %q, %v; want original %q", after, err, before)
	}
}

func TestWarnRetiredAgentSettingsWritesOneDiagnosticAndPreservesBytes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BABYSIT_STATE_DIR", dir)
	path := filepath.Join(dir, "config.yaml")
	before := []byte("worker_model: old\nforeman_effort: high\ntelemetry: off\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}

	var warning bytes.Buffer
	WarnRetiredAgentSettings(&warning)
	if got := strings.Count(warning.String(), "\n"); got != 1 || !strings.Contains(warning.String(), "Orca") || !strings.Contains(warning.String(), "explicit per-dispatch") {
		t.Fatalf("warning = %q; want one actionable Orca diagnostic", warning.String())
	}
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, before) {
		t.Fatalf("config after warning = %q, %v; want original %q", after, err, before)
	}
}

func TestWarnRetiredAgentSettingsSkipsUnrelatedConfig(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	if err := Set("telemetry", "off"); err != nil {
		t.Fatal(err)
	}

	var warning bytes.Buffer
	WarnRetiredAgentSettings(&warning)
	if warning.Len() != 0 {
		t.Fatalf("warning for unrelated config = %q", warning.String())
	}
}
