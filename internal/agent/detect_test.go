package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps").CombinedOutput()
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if err == nil && !strings.Contains(string(output), "omp") {
		err = fmt.Errorf("probe output did not contain marker %q", "omp")
	}
	if err != nil {
		t.Skipf("environment cannot execute test ps stubs: %v", err)
	}

	if got := Detect(); got.Agent != "omp" || got.Source != "parent process" {
		t.Fatalf("got %+v", got)
	}
}

func TestInstalledAgentDoesNotImplyCurrentHarness(t *testing.T) {
	isolate(t)
	t.Setenv("BABYSIT_CURRENT_AGENT", "")
	t.Setenv("BABYSIT_AGENT", "omp")
	for _, key := range []string{"OMP_SESSION_ID", "GROK_SESSION_ID", "GROK_AGENT", "CLAUDE_CODE_SESSION_ID", "CLAUDECODE", "CURSOR_AGENT", "CURSOR_TRACE_ID"} {
		t.Setenv(key, "")
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("PATH"), "omp"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	installed := false
	for _, item := range Installations() {
		if item.Agent == "omp" && item.Path != "" {
			installed = true
		}
	}
	if !installed {
		t.Fatal("fake OMP executable not reported as installed")
	}
	if got := Detect(); got.Agent != "unknown" {
		t.Fatalf("launch preference or PATH became runtime identity: %+v", got)
	}
}
