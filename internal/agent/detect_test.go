package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeMarkers(t *testing.T) {
	for marker, want := range map[string]string{
		"CODEX_THREAD_ID": "codex", "CODEX_SESSION_ID": "codex", "CLAUDECODE": "claude",
		"CLAUDE_CODE_SESSION_ID": "claude", "GROK_SESSION_ID": "grok", "GROK_AGENT": "grok",
		"CURSOR_AGENT": "cursor",
	} {
		t.Run(marker, func(t *testing.T) {
			d := detectEnv(func(key string) string {
				if key == marker {
					return "1"
				}
				return ""
			})
			if d.Agent != want || d.Source != marker {
				t.Fatalf("got %+v", d)
			}
		})
	}
	for _, irrelevant := range []string{"OPENAI_API_KEY", "CODEX_HOME", "OMP_PROFILE", "PI_CODING_AGENT_DIR", "TERM_PROGRAM", "AGENT"} {
		if got := detectEnv(func(key string) string {
			if key == irrelevant {
				return "1"
			}
			return ""
		}); got.Agent != "unknown" {
			t.Fatalf("configuration marker %s was treated as identity: %+v", irrelevant, got)
		}
	}
}

func TestProcessRecognitionExcludesEditorAndPromptText(t *testing.T) {
	for executable, want := range map[string]string{
		"/opt/tools/omp": "omp", "/opt/bin/claude": "claude",
		"/opt/bin/codex-aarch64-apple-darwin": "codex", "/opt/bin/grok-1.0": "grok",
		"/opt/cursor-agent/versions/1/agent": "cursor", "/opt/bin/cursor-agent": "cursor",
		"/Applications/Cursor.app/Contents/MacOS/Cursor": "", "/opt/bin/agent": "",
		"/tmp/codex/not-an-agent": "", "sh": "", "node": "",
	} {
		if got := processAgent(executable); got != want {
			t.Errorf("processAgent(%q) = %q, want %q", executable, got, want)
		}
	}
}

func TestNearestAgentProcessBeatsInheritedSession(t *testing.T) {
	isolate(t)
	t.Setenv("BABYSIT_CURRENT_AGENT", "")
	t.Setenv("CODEX_THREAD_ID", "outer-session")
	if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "ps"), []byte("#!/bin/sh\necho '1 /opt/tools/omp'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Detect(); got.Agent != "omp" || got.Source != "parent process" {
		t.Fatalf("got %+v", got)
	}
}

func TestAutomaticSelectionAndExplicitOverrides(t *testing.T) {
	isolate(t)
	t.Setenv("BABYSIT_CURRENT_AGENT", "cursor-agent")
	p, err := Resolve(WorkerKey, "")
	if err != nil || p.Name != "cursor" {
		t.Fatalf("current agent lost: %+v %v", p, err)
	}
	writeGlobal(t, "worker_agent: omp\n")
	if p, _ := Resolve(WorkerKey, ""); p.Name != "omp" {
		t.Fatal("detection overrode config")
	}
	t.Setenv("BABYSIT_AGENT", "grok")
	if p, _ := Resolve(WorkerKey, "claude-code"); p.Name != "claude" {
		t.Fatal("explicit flag lost")
	}
	if p, _ := Resolve(WorkerKey, "auto"); p.Name != "cursor" {
		t.Fatal("explicit auto did not detect current agent")
	}
	if _, err := Resolve(WorkerKey, "missing-cli"); err == nil {
		t.Fatal("unknown explicit agent silently fell back")
	}
}

func TestInstalledFallbackDoesNotImplyCurrentHarness(t *testing.T) {
	isolate(t)
	t.Setenv("BABYSIT_CURRENT_AGENT", "")
	for _, key := range []string{"OMP_SESSION_ID", "GROK_SESSION_ID", "GROK_AGENT", "CLAUDE_CODE_SESSION_ID", "CLAUDECODE", "CURSOR_AGENT", "CURSOR_TRACE_ID"} {
		t.Setenv(key, "")
	}
	if got := Detect(); got.Agent != "unknown" {
		t.Fatalf("invented current agent: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "omp"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Resolve(WorkerKey, "")
	if err != nil || p.Name != "omp" {
		t.Fatalf("installed agent not selected: %+v %v", p, err)
	}
	if got := Detect(); got.Agent != "unknown" {
		t.Fatalf("PATH became runtime identity: %+v", got)
	}
	if p, _ := Resolve(WorkerKey, "grok"); p.Preflight() == nil {
		t.Fatal("missing explicitly selected CLI fell back")
	}
}
