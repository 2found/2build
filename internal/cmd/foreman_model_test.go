package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reallongnguyen/babysit/internal/config"
)

func TestForemanModelLookupFromNestedRepoAndWorktree(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	repo := gitRepo(t, "main")
	worktree := filepath.Join(t.TempDir(), "worker")
	gitIn(t, repo, "worktree", "add", "-b", "worker", worktree)
	for _, root := range []string{repo, worktree} {
		settingsDir := filepath.Join(root, ".babysit")
		if err := os.MkdirAll(settingsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		model := "model-" + filepath.Base(root)
		body := `{"foreman":{"models":{"tiers":{"max":{"codex":{"model":"` + model + `"}}}}}}`
		if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		nested := filepath.Join(root, "src", "nested")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		out := captureStdout(t, func() {
			if err := dispatchForeman([]string{"model", "--dir", nested, "--agent", "codex", "--complexity", "critical", "--phase-class", "critical", "--json"}); err != nil {
				t.Fatal(err)
			}
		})
		var got map[string]string
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		if got["model"] != model || got["effort"] != "high" || got["selectedTier"] != "max" || got["complexity"] != "hard" || got["agent"] != "codex" {
			t.Fatalf("lookup from %s = %s", nested, out)
		}
	}
}

func TestForemanModelPolicyAndErrors(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	out := captureStdout(t, func() {
		if err := foremanModel([]string{"--json"}); err != nil {
			t.Fatal(err)
		}
	})
	var got config.ForemanModels
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.Tiers["flash"]["codex"].Model != "gpt-6-luna" {
		t.Fatalf("policy output %s: %v", out, err)
	}
	for _, args := range [][]string{
		{"--complexity", "normal"}, {"--typo", "value"}, {"unexpected"},
		{"--agent", "codex", "--complexity", "ultra"},
		{"--agent", "codex", "--phase-class", "review"},
		{"--dir", filepath.Join(t.TempDir(), "missing")},
	} {
		if err := foremanModel(args); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
	out = captureStdout(t, func() {
		if err := foremanModel([]string{"--agent", "omp", "--complexity", "simple", "--phase-class", "normal"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, `"effort"`) || !strings.Contains(out, `"@normal"`) {
		t.Fatalf("OMP binding = %s", out)
	}
}

func TestForemanModelDefaultsMissingClassification(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	for _, tc := range []struct {
		args                    []string
		complexity, phase, tier string
	}{
		{nil, "normal", "normal", "flash"},
		{[]string{"--complexity", "normal"}, "normal", "normal", "flash"},
		{[]string{"--phase-class", "normal"}, "normal", "normal", "flash"},
		{[]string{"--phase-class", "critical"}, "normal", "critical", "pro"},
		{[]string{"--complexity", "hard"}, "hard", "normal", "pro"},
		{[]string{"--complexity", "hard", "--phase-class", "critical"}, "hard", "critical", "max"},
	} {
		out := captureStdout(t, func() {
			if err := foremanModel(append([]string{"--agent", "codex", "--json"}, tc.args...)); err != nil {
				t.Fatal(err)
			}
		})
		var got map[string]string
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		if got["complexity"] != tc.complexity || got["phaseClass"] != tc.phase || got["selectedTier"] != tc.tier {
			t.Fatalf("args %v: %s", tc.args, out)
		}
	}
}

func TestForemanModelMissingClassificationHonorsRepoOverride(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	t.Chdir(repo)
	settingsDir := filepath.Join(repo, ".babysit")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"foreman":{"models":{"routing":{"normal":{"normal":"pro"}},"tiers":{"pro":{"codex":{"model":"repo-model","effort":"low"}}}}}}`
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := foremanModel([]string{"--agent", "codex", "--json"}); err != nil {
			t.Fatal(err)
		}
	})
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got["selectedTier"] != "pro" || got["model"] != "repo-model" || got["effort"] != "low" {
		t.Fatalf("missing classification ignored repo override: %s", out)
	}
}
