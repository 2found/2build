package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeModelSettings(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestForemanModelDefaults(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	p, err := LoadForemanModels(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ complexity, phase, tier string }{
		{"simple", "normal", "lite"}, {"simple", "critical", "flash"}, {"simple", "design", "pro"},
		{"normal", "normal", "flash"}, {"normal", "critical", "flash"}, {"normal", "design", "max"},
		{"hard", "normal", "pro"}, {"hard", "critical", "max"}, {"hard", "design", "max"},
		{"critical", "critical", "max"},
	} {
		for agent, bindings := range map[string]map[string]ModelBinding{
			"codex": {"lite": {"gpt-6-luna", "high"}, "flash": {"gpt-6.1-sol", "high"},
				"pro": {"gpt-6.1-sol", "high"}, "max": {"gpt-6-astra", "high"}},
			"claude": {"lite": {"haiku", ""}, "flash": {"sonnet", "high"},
				"pro": {"opus", "medium"}, "max": {"opus", "high"}},
			"omp": {"lite": {"@smol", ""}, "flash": {"", ""}, "pro": {"@slow", ""}, "max": {"@plan", ""}},
		} {
			tier, binding, err := p.Select(agent, tc.complexity, tc.phase)
			if err != nil || tier != tc.tier || binding != bindings[tier] {
				t.Fatalf("%s %+v: %s %+v, %v", agent, tc, tier, binding, err)
			}
		}
	}
}

func TestForemanModelSettingsPrecedence(t *testing.T) {
	global, repo := t.TempDir(), t.TempDir()
	t.Setenv("BABYSIT_STATE_DIR", global)
	globalPath := filepath.Join(global, "settings.json")
	repoPath := filepath.Join(repo, ".babysit", "settings.json")
	globalJSON := `{"other":{"anything":true},"foreman":{"models":{"routing":{"simple":{"critical":"pro","normal":"flash"},"normal":{"critical":"max"}},"tiers":{"flash":{"codex":{"model":"global-flash","effort":"low"}},"max":{"codex":{"model":"global-max"}}}}}}`
	repoJSON := `{"foreman":{"models":{"routing":{"simple":{"critical":"max"},"normal":{"normal":"pro"}},"tiers":{"flash":{"codex":{"effort":""}},"max":{"codex":{"model":"repo-max"}},"pro":{"custom":{"model":"provider/custom"}}}}}}`
	writeModelSettings(t, globalPath, globalJSON)
	writeModelSettings(t, repoPath, repoJSON)
	p, err := LoadForemanModels(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ agent, complexity, phase, tier, model, effort string }{
		{"codex", "simple", "normal", "flash", "global-flash", ""},
		{"codex", "simple", "critical", "max", "repo-max", "high"},
		{"codex", "normal", "normal", "pro", "gpt-6.1-sol", "high"},
		{"codex", "normal", "design", "max", "repo-max", "high"},
		{"codex", "normal", "critical", "max", "repo-max", "high"},
		{"custom", "hard", "normal", "pro", "provider/custom", ""},
	} {
		tier, binding, err := p.Select(tc.agent, tc.complexity, tc.phase)
		if err != nil || tier != tc.tier || binding.Model != tc.model || binding.Effort != tc.effort {
			t.Fatalf("%+v: %s %+v, %v", tc, tier, binding, err)
		}
	}
	for path, want := range map[string]string{globalPath: globalJSON, repoPath: repoJSON} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("settings changed: %s: %v", path, err)
		}
	}
	// A later load must not inherit mutations to built-in maps from this load.
	p, err = LoadForemanModels("")
	if err != nil || p.Routing["simple"]["critical"] != "pro" || p.Tiers["max"]["codex"].Model != "global-max" || p.Tiers["flash"]["codex"].Effort != "low" {
		t.Fatalf("policy leaked across loads: %+v, %v", p, err)
	}
}

func TestForemanModelInvalidSettings(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	for _, body := range []string{
		`{`,
		`null`,
		`{"foreman":{"models":null}}`,
		`{"foreman":{"models":{"tier":{}}}}`,
		`{"foreman":{"models":{"routing":{"hard":{"normal":"ultra"}}}}}`,
		`{"foreman":{"models":{"routing":{"easy":{"normal":"flash"}}}}}`,
		`{"foreman":{"models":{"routing":{"hard":{"review":"pro"}}}}}`,
		`{"foreman":{"models":{"tiers":{"ultra":{}}}}}`,
		`{"foreman":{"models":{"tiers":{"pro":{"codex":{"model":""}}}}}}`,
		`{"foreman":{"models":{"tiers":{"pro":{"codex":{"effort":null}}}}}}`,
		`{"foreman":{"models":{"tiers":{"pro":{"codex":{"effort":123}}}}}}`,
		`{"foreman":{"models":{"tiers":{"pro":{"codex":{"modle":"typo"}}}}}}`,
		`{"foreman":{"models":{"tiers":{"pro":{"codex":null}}}}}`,
		`{"foreman":{"models":{"tiers":{"pro":{"custom":{"effort":"high"}}}}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			repo := t.TempDir()
			path := filepath.Join(repo, ".babysit", "settings.json")
			writeModelSettings(t, path, body)
			if _, err := LoadForemanModels(repo); err == nil || !strings.Contains(err.Error(), path) {
				t.Fatalf("expected settings path error for %s: %v", body, err)
			}
		})
	}
}

func TestForemanModelInvalidLookup(t *testing.T) {
	p := defaultForemanModels()
	for _, args := range [][3]string{{"codex", "easy", "normal"}, {"codex", "normal", "review"}, {"unknown", "normal", "normal"}} {
		if _, _, err := p.Select(args[0], args[1], args[2]); err == nil {
			t.Fatalf("accepted invalid lookup: %v", args)
		}
	}
}

func TestForemanModelIgnoresOffloadSettings(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	writeModelSettings(t, filepath.Join(repo, ".babysit", "settings.json"),
		`{"foreman":{"models":{"offload":{"agent":"omp","weeklyUsedPercent":50}}}}`)
	p, err := LoadForemanModels(repo)
	if err != nil {
		t.Fatal(err)
	}
	tier, binding, err := p.Select("claude", "normal", "critical")
	if err != nil || tier != "flash" || binding.Model != "sonnet" {
		t.Fatalf("offload settings must not change routing: %s %+v %v", tier, binding, err)
	}
}
