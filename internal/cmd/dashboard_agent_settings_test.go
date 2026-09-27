package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reallongnguyen/babysit/internal/agent"
	"github.com/reallongnguyen/babysit/internal/config"
	"gopkg.in/yaml.v3"
)

func agentSettingsServer(t *testing.T) *dashServer {
	t.Helper()
	s, _ := sandboxServer(t)
	t.Setenv("BABYSIT_STATE_DIR", s.stateDir)
	t.Setenv("BABYSIT_CURRENT_AGENT", "codex")
	for _, prefix := range []string{"BABYSIT_", "BABYSIT_WORKER_", "BABYSIT_FOREMAN_"} {
		for _, field := range []string{"AGENT", "PROVIDER", "MODEL", "EFFORT"} {
			t.Setenv(prefix+field, "")
		}
	}
	s.currentDir = t.TempDir()
	if out, err := exec.Command("git", "-C", s.currentDir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	return s
}

func fetchAgentSettings(t *testing.T, s *dashServer) agentSettingsResponse {
	t.Helper()
	w := send(t, s, http.MethodGet, "/api/agent-settings")
	if w.Code != http.StatusOK {
		t.Fatalf("GET settings: %d %s", w.Code, w.Body.String())
	}
	var result agentSettingsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func saveSettings(t *testing.T, s *dashServer, settings agentSettingsResponse) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	return post(t, s, "/api/agent-settings", string(b))
}

func writeSettingsFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDashboardAgentSettingsRoundTrip(t *testing.T) {
	s := agentSettingsServer(t)
	path := filepath.Join(s.stateDir, "config.yaml")
	writeSettingsFile(t, path, "# keep my preferences\ntelemetry: off\ncustom:\n  api_key: private-value\nworker_model: old # keep my note\n")
	settings := fetchAgentSettings(t, s)
	if settings.Detected.Agent != "codex" || len(settings.Agents) != len(agent.Names()) || len(settings.Values) != 8 {
		t.Fatalf("incomplete metadata: %+v", settings)
	}
	b, _ := json.Marshal(settings)
	if strings.Contains(string(b), "private-value") || strings.Contains(string(b), "api_key") {
		t.Fatal("GET leaked unrelated config")
	}
	settings.Values["worker_agent"] = "oh-my-pi"
	settings.Values["worker_provider"] = "my-provider"
	settings.Values["worker_model"] = "@slow"
	settings.Values["worker_effort"] = "high"
	settings.Values["foreman_agent"] = "codex"
	settings.Values["foreman_provider"] = "local"
	settings.Values["foreman_model"] = "custom:model # literal"
	settings.Values["foreman_effort"] = "medium"
	w := saveSettings(t, s, settings)
	if w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	got := fetchAgentSettings(t, s)
	var saved struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || saved.Revision != got.Revision {
		t.Fatalf("save returned wrong revision: %s", w.Body.String())
	}
	if got.Revision == settings.Revision || got.Values["worker_agent"] != "omp" {
		t.Fatalf("save did not normalize/persist: %+v", got)
	}
	for key, value := range got.Values {
		if actual, ok := config.Get(key); !ok || actual != value {
			t.Fatalf("CLI read %s = %q, expected %q", key, actual, value)
		}
	}
	worker, err := agent.ResolveWith(agent.WorkerKey, agent.Options{Dir: s.currentDir})
	if err != nil || worker.Name != "omp" || worker.Model != "@slow" || worker.Provider != "my-provider" {
		t.Fatalf("worker resolver: %+v, %v", worker, err)
	}
	foreman, err := agent.ResolveWith(agent.ForemanKey, agent.Options{Dir: s.currentDir})
	if err != nil || foreman.Name != "codex" || foreman.Model != "custom:model # literal" {
		t.Fatalf("foreman resolver: %+v, %v", foreman, err)
	}
	after, _ := os.ReadFile(path)
	var content map[string]interface{}
	if err := yaml.Unmarshal(after, &content); err != nil {
		t.Fatalf("invalid YAML after save: %v", err)
	}
	if content["telemetry"] != "off" || content["custom"].(map[string]interface{})["api_key"] != "private-value" || !strings.Contains(string(after), "# keep my preferences") || !strings.Contains(string(after), "# keep my note") {
		t.Fatalf("unrelated fields/comments lost: %s", after)
	}
	// Clearing a preference returns to the native default without switching roles.
	got.Values["worker_model"] = ""
	if w := saveSettings(t, s, got); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	worker, err = agent.ResolveWith(agent.WorkerKey, agent.Options{Dir: s.currentDir})
	if err != nil || worker.Model != "" || worker.Name != "omp" {
		t.Fatalf("clear failed: %+v, %v", worker, err)
	}
}

// An anchored or aliased allowlisted key couples the setting to other entries:
// rewriting the scalar would silently retarget every alias, and dropping the
// anchor would dangle them. The save must refuse instead of choosing either.
func TestDashboardAgentSettingsRejectsAnchoredSettings(t *testing.T) {
	s := agentSettingsServer(t)
	path := filepath.Join(s.stateDir, "config.yaml")
	writeSettingsFile(t, path, "worker_model: &model old\nother_model: *model\n")
	settings := fetchAgentSettings(t, s)
	settings.Values["worker_model"] = "new-model"
	w := saveSettings(t, s, settings)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("accepted anchored save: %d %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(path)
	if string(after) != "worker_model: &model old\nother_model: *model\n" {
		t.Fatalf("rejected save still rewrote the file: %s", after)
	}
}

func TestDashboardAgentSettingsRejectsInvalidAndStaleWrites(t *testing.T) {
	s := agentSettingsServer(t)
	settings := fetchAgentSettings(t, s) // missing config is valid
	for _, tc := range []struct{ key, value string }{
		{"worker_agent", "missing-agent"}, {"worker_model", "line\nbreak"},
		{"worker_model", strings.Repeat("x", 1025)}, {"unknown_key", "anything"},
		{"worker_provider", "openai"}, {"worker_effort", "high"},
	} {
		t.Run(tc.key+"/"+tc.value[:min(12, len(tc.value))], func(t *testing.T) {
			request := settings
			request.Values = make(map[string]string)
			for k, v := range settings.Values {
				request.Values[k] = v
			}
			request.Values["worker_agent"] = "cursor"
			request.Values[tc.key] = tc.value
			if w := saveSettings(t, s, request); w.Code != http.StatusBadRequest {
				t.Fatalf("accepted invalid settings: %d %s", w.Code, w.Body.String())
			}
			if _, err := os.Stat(settings.Path); !os.IsNotExist(err) {
				t.Fatal("invalid save touched disk")
			}
		})
	}
	writeSettingsFile(t, settings.Path, "worker_model: edited-in-terminal\n")
	if w := saveSettings(t, s, settings); w.Code != http.StatusConflict {
		t.Fatalf("stale save: %d %s", w.Code, w.Body.String())
	}
	if value, _ := config.Get("worker_model"); value != "edited-in-terminal" {
		t.Fatal("stale write overwrote terminal edit")
	}
	settings = fetchAgentSettings(t, s)
	b, _ := json.Marshal(settings)
	r := httptest.NewRequest(http.MethodPost, "/api/agent-settings", strings.NewReader(string(b)))
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	s.mux().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin save: %d", w.Code)
	}
}

func TestDashboardAgentSettingsMalformedConfigIsUntouched(t *testing.T) {
	s := agentSettingsServer(t)
	for _, content := range []string{"worker_agent: [", "- not-a-mapping\n", "worker_agent: omp\nworker_agent: codex\n", "worker_model: [one, two]\n"} {
		writeSettingsFile(t, filepath.Join(s.stateDir, "config.yaml"), content)
		if w := send(t, s, http.MethodGet, "/api/agent-settings"); w.Code != http.StatusBadRequest {
			t.Fatalf("accepted malformed config %q: %d", content, w.Code)
		}
		if w := post(t, s, "/api/agent-settings", `{"revision":""}`); w.Code != http.StatusBadRequest {
			t.Fatalf("save malformed config: %d", w.Code)
		}
		after, _ := os.ReadFile(filepath.Join(s.stateDir, "config.yaml"))
		if string(after) != content {
			t.Fatalf("malformed file changed: %s", after)
		}
	}
}

func TestDashboardAgentSettingsPreservesCommentOnlyConfig(t *testing.T) {
	s := agentSettingsServer(t)
	path := filepath.Join(s.stateDir, "config.yaml")
	writeSettingsFile(t, path, "# team defaults\n# Leave models blank to use native config.\n")
	settings := fetchAgentSettings(t, s)
	settings.Values["worker_agent"] = "auto"
	if w := saveSettings(t, s, settings); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "# team defaults") || !strings.Contains(string(after), "# Leave models blank") {
		t.Fatalf("comments lost: %s", after)
	}
}

func TestDashboardAgentSettingsPreservesSymlink(t *testing.T) {
	s := agentSettingsServer(t)
	target := filepath.Join(t.TempDir(), "config.yaml")
	writeSettingsFile(t, target, "telemetry: off\n")
	path := filepath.Join(s.stateDir, "config.yaml")
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	settings := fetchAgentSettings(t, s)
	settings.Values["worker_agent"] = "codex"
	if w := saveSettings(t, s, settings); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if got, err := os.Readlink(path); err != nil || got != target {
		t.Fatalf("config symlink replaced: %q, %v", got, err)
	}
	_, values, _, err := readAgentSettings(target)
	if err != nil || values["worker_agent"] != "codex" {
		t.Fatalf("target not updated: %v, %v", values, err)
	}
}
