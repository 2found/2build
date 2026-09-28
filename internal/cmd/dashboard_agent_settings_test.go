package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashboardAgentSettingsReturnsGoneWithoutTouchingConfig(t *testing.T) {
	s, _ := sandboxServer(t)
	path := filepath.Join(s.stateDir, "config.yaml")
	before := []byte("# preserve legacy bytes\nworker_model: [malformed\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method string
		body   string
	}{{http.MethodGet, ""}, {http.MethodPost, "not-json"}} {
		t.Run(tc.method, func(t *testing.T) {
			var response *httptest.ResponseRecorder
			if tc.method == http.MethodPost {
				response = post(t, s, "/api/agent-settings", tc.body)
			} else {
				response = send(t, s, tc.method, "/api/agent-settings")
			}
			if response.Code != http.StatusGone {
				t.Fatalf("status = %d, body = %s; want 410", response.Code, response.Body.String())
			}
			var result map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatalf("response is not JSON: %v", err)
			}
			if !strings.Contains(result["error"], "Orca") || !strings.Contains(result["error"], "resume") {
				t.Fatalf("retirement message = %q", result["error"])
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, before) {
				t.Fatalf("config after %s = %q, %v; want original %q", tc.method, after, err, before)
			}
		})
	}
}
