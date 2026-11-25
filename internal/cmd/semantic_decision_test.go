package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/config"
	"github.com/2found/2build/internal/decision"
	"github.com/2found/2build/internal/foreman"
)

const semanticInput = `{"state":"private source evidence","questions":{"size":{"type":"choice","instructions":"Size?","criteria":{"S":"small","L":"large"}}}}`

func semanticSetup(t *testing.T) string {
	t.Helper()
	state := t.TempDir()
	repo := t.TempDir()
	t.Setenv("BABYSIT_STATE_DIR", state)
	t.Setenv("BABYSIT_ANALYTICS_DIR", "")
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
func TestSemanticLLMCompletionAndTelemetry(t *testing.T) {
	state := semanticSetup(t)
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
	if len(rows) != 1 {
		t.Fatal(string(data))
	}
	var row map[string]any
	json.Unmarshal([]byte(rows[0]), &row)
	if row["decision_kind"] != "task-size" || row["provider"] != "llm" || row["status"] != "DECIDED" {
		t.Fatal(row)
	}
	config.Set("telemetry", "off")
	semanticRun(t, "--kind", "task-size", "--llm-answers", "answers.json")
	after, _ := os.ReadFile(filepath.Join(state, "analytics", "decisions.jsonl"))
	if !bytes.Equal(data, after) {
		t.Fatal("opt-out ignored")
	}
}
func TestSemanticInvalidConfigAndInput(t *testing.T) {
	semanticSetup(t)
	// The one-call form is required; the error names it.
	if _, err := semanticRun(t); err == nil || !strings.Contains(err.Error(), "--llm-answers") {
		t.Fatalf("missing --llm-answers: %v", err)
	}
	// Legacy provider/model keys from older configs are inert.
	config.Set("semantic_decision_provider", "cloudflare")
	config.Set("semantic_decision_model", "clef")
	os.WriteFile("answers.json", []byte(`{"size":{"choice":"S","reason":"cited"}}`), 0o600)
	if result, err := semanticRun(t, "--llm-answers", "answers.json"); err != nil || result.Provider != "llm" {
		t.Fatal(result, err)
	}
	os.WriteFile("bad.json", []byte(`{"size":{"choice":"X","reason":"cited"}}`), 0o600)
	if _, err := semanticRun(t, "--llm-answers", "bad.json"); err == nil {
		t.Fatal("answer outside criteria accepted")
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
