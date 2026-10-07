package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reallongnguyen/babysit/internal/config"
	"github.com/reallongnguyen/babysit/internal/decision"
	"github.com/reallongnguyen/babysit/internal/foreman"
)

const semanticInput = `{"state":"private source evidence","questions":{"size":{"type":"choice","instructions":"Size?","criteria":{"S":"small","L":"large"}}}}`

func semanticSetup(t *testing.T) string {
	t.Helper()
	state := t.TempDir()
	repo := t.TempDir()
	t.Setenv("BABYSIT_STATE_DIR", state)
	t.Setenv("BABYSIT_ANALYTICS_DIR", "")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	if err := os.Mkdir(filepath.Join(repo, ".babysit"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	return state
}
func semanticRun(t *testing.T, args ...string) (decision.Result, error) {
	t.Helper()
	cmd := newSemanticDecisionCmd()
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(semanticInput))
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err := cmd.Execute()
	if err != nil {
		return decision.Result{}, err
	}
	var result decision.Result
	if err = json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(output.String(), err)
	}
	return result, nil
}
func TestSemanticConsentAndProjectRestrictions(t *testing.T) {
	semanticSetup(t)
	// Credentials alone do not select external inference.
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "present-account")
	t.Setenv("CLOUDFLARE_API_TOKEN", "present-token")
	// A cloned repo cannot grant consent or select a provider/model.
	os.WriteFile(".babysit/semantic-decision.yaml", []byte("enabled: true\nprovider: attacker\nmodel: attacker\n"), 0o600)
	result, err := semanticRun(t)
	if err != nil || result.Reason != "provider_llm" || result.Status != "NEEDS_LLM" {
		t.Fatalf("%+v %v", result, err)
	}
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	config.Set("semantic_decision_provider", "cloudflare")
	result, err = semanticRun(t)
	if err != nil || result.Reason != "missing_credentials" || result.Model != "clef-flash" {
		t.Fatalf("%+v %v", result, err)
	}
	cases := []struct{ policy, kind, want string }{
		{"enabled: false\n", "custom", "project_disabled"},
		{"allowed_kinds: []\n", "custom", "project_kind_denied"},
		{"allowed_kinds: [task-size]\n", "testcase", "project_kind_denied"},
		{"allowed_kinds: [task-size]\n", "task-size", "missing_credentials"},
		{"enabled: [broken\n", "task-size", "invalid_project_policy"},
	}
	for _, tc := range cases {
		os.WriteFile(".babysit/semantic-decision.yaml", []byte(tc.policy), 0o600)
		result, err = semanticRun(t, "--kind", tc.kind)
		if err != nil || result.Reason != tc.want {
			t.Fatalf("%s: %+v %v", tc.policy, result, err)
		}
	}
}
func TestSemanticCredentialsOnlyUserScope(t *testing.T) {
	state := semanticSetup(t)
	const key = "BBS_TEST_SEMANTIC_TOKEN"
	os.Unsetenv(key)
	t.Cleanup(func() { os.Unsetenv(key) })
	os.WriteFile(".babysit/.env", []byte(key+"=project-secret\n"), 0o600)
	if got := decisionCredential(key); got != "" {
		t.Fatalf("project credential read: %q", got)
	}
	os.WriteFile(filepath.Join(state, ".env"), []byte(key+"=user-secret\n"), 0o600)
	if got := decisionCredential(key); got != "user-secret" {
		t.Fatal(got)
	}
	t.Setenv(key, "shell-secret")
	if got := decisionCredential(key); got != "shell-secret" {
		t.Fatal(got)
	}
	t.Setenv(key, "")
	if got := decisionCredential(key); got != "" {
		t.Fatal("empty shell override ignored")
	}
}

func TestSemanticProjectPolicyUsesGitRoot(t *testing.T) {
	semanticSetup(t)
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-q")
	if err := config.Set("semantic_decision_provider", "cloudflare"); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "apps", "foo")
	if err := os.MkdirAll(filepath.Join(nested, ".babysit"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	for _, policy := range []string{"enabled: false\n", "allowed_kinds: []\n"} {
		if err := os.WriteFile(filepath.Join(root, ".babysit", "semantic-decision.yaml"), []byte(policy), 0o600); err != nil {
			t.Fatal(err)
		}
		result, err := semanticRun(t)
		want := "project_disabled"
		if strings.HasPrefix(policy, "allowed_kinds") {
			want = "project_kind_denied"
		}
		if err != nil || result.Status != "FALLBACK_LLM" || result.Reason != want {
			t.Fatalf("%s: %+v %v", policy, result, err)
		}
	}
}
func TestSemanticLLMCompletionAndTelemetry(t *testing.T) {
	state := semanticSetup(t)
	first, err := semanticRun(t, "--kind", "task-size")
	if err != nil || first.Status != "NEEDS_LLM" {
		t.Fatal(first, err)
	}
	os.WriteFile("answers.json", []byte(`{"size":{"choice":"L","reason":"private analysis file.go:2"}}`), 0o600)
	result, err := semanticRun(t, "--kind", "task-size", "--llm-answers", "answers.json")
	if err != nil || result.Status != "DECIDED" || result.Provider != "llm" || result.Answers["size"].Choice != "L" {
		t.Fatal(result, err)
	}
	data, err := os.ReadFile(filepath.Join(state, "analytics", "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private") || strings.Contains(string(data), "small") {
		t.Fatal("source or reasoning leaked into telemetry")
	}
	rows := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(rows) != 2 {
		t.Fatal(string(data))
	}
	var a, b map[string]any
	json.Unmarshal([]byte(rows[0]), &a)
	json.Unmarshal([]byte(rows[1]), &b)
	if a["request_id"] != b["request_id"] || b["decision_kind"] != "task-size" {
		t.Fatal(a, b)
	}
	config.Set("telemetry", "off")
	semanticRun(t)
	after, _ := os.ReadFile(filepath.Join(state, "analytics", "decisions.jsonl"))
	if !bytes.Equal(data, after) {
		t.Fatal("opt-out ignored")
	}
}
func TestSemanticInvalidConfigAndInput(t *testing.T) {
	semanticSetup(t)
	config.Set("semantic_decision_provider", "cloudflare")
	config.Set("semantic_decision_provider", "llm")
	result, err := semanticRun(t)
	if err != nil || result.Reason != "provider_llm" {
		t.Fatal(result, err)
	}
	config.Set("semantic_decision_provider", "unknown")
	result, err = semanticRun(t)
	if err != nil || result.Reason != "invalid_provider" {
		t.Fatal(result, err)
	}
	if _, err := semanticRun(t, "--model", "clef"); err == nil {
		t.Fatal("caller may not override user model")
	}
	if _, err := semanticRun(t, "--kind", "typo"); err == nil {
		t.Fatal("unknown kind accepted")
	}
	var request decision.Request
	for _, input := range []string{semanticInput + `{}`, strings.Repeat(" ", 256<<10) + semanticInput} {
		if err := readDecisionJSON("-", strings.NewReader(input), &request); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}

func TestSemanticHumanReviewDoesNotApproveCheckpoint(t *testing.T) {
	semanticSetup(t)
	st, env := projectApprovalFixture(t)
	before, err := os.ReadFile(st.IndexPath())
	if err != nil {
		t.Fatal(err)
	}
	request := `{"state":"Reviewer supplied evidence against the accepted plan.","questions":{"review":{"type":"choice","instructions":"Is the plan evidence sufficient?","criteria":{"proceed":"Evidence sufficient within delegated scope","revise":"Repairable evidence gap","needs-human":"Missing user decision","blocked":"Unavailable capability"}}}}`
	if err := os.WriteFile("request.json", []byte(request), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("answers.json", []byte(`{"review":{"choice":"proceed","reason":"Coverage and scope match the supplied plan evidence."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := semanticRun(t, "--kind", "human-review", "--input", "request.json", "--llm-answers", "answers.json")
	if err != nil || result.Status != "DECIDED" || result.Answers["review"].Choice != "proceed" {
		t.Fatal(result, err)
	}
	after, err := os.ReadFile(st.IndexPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("semantic judgment changed approval state", err)
	}
	if code, _, _ := selfResolveGate(st, env, foreman.Record{ID: "fm-project"}, projectRubric); code != exitGrant {
		t.Fatalf("semantic proceed bypassed explicit --auto: %d", code)
	}
}
