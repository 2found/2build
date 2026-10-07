package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/reallongnguyen/babysit/internal/config"
	"github.com/reallongnguyen/babysit/internal/decision"
	"github.com/reallongnguyen/babysit/internal/env"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newSemanticDecisionCmd() *cobra.Command {
	var input, kind, llmAnswers string
	cmd := &cobra.Command{
		Use: "semantic-decision", Short: "Run a bounded judgment step (LLM by default; optional Cloudflare Clef)",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var request decision.Request
			if err := readDecisionJSON(input, cmd.InOrStdin(), &request); err != nil {
				return err
			}
			if err := request.Validate(); err != nil {
				return err
			}
			if !decisionKinds[kind] {
				return fmt.Errorf("unknown decision kind %q", kind)
			}
			start := time.Now()
			var result decision.Result
			var err error
			if llmAnswers != "" {
				if input == "-" && llmAnswers == "-" {
					return fmt.Errorf("input and llm-answers cannot both read stdin")
				}
				var answers map[string]decision.Answer
				if err = readDecisionJSON(llmAnswers, cmd.InOrStdin(), &answers); err != nil {
					return err
				}
				result, err = decision.ResolveLLM(request, answers)
			} else {
				policy := semanticDecisionPolicy(kind)
				if policy != "" {
					status := "FALLBACK_LLM"
					if policy == "provider_llm" {
						status = "NEEDS_LLM"
					}
					result = decision.Result{Status: status, Provider: "llm", Reason: policy}
				} else {
					model, _ := config.Get("semantic_decision_model")
					result, err = (decision.Client{
						AccountID: decisionCredential("CLOUDFLARE_ACCOUNT_ID"),
						Token:     decisionCredential("CLOUDFLARE_API_TOKEN"), Model: model,
					}).Decide(cmd.Context(), request)
				}
			}
			if err != nil {
				return err
			}
			if err := logSemanticDecision(kind, request, result, time.Since(start)); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "semantic-decision: telemetry: %v\n", err)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().StringVar(&input, "input", "-", "JSON request file, or - for stdin (state and choice questions)")
	cmd.Flags().StringVar(&kind, "kind", "custom", "task-size, task-complexity, testcase, review-finding, skill-route, orchestration, deep-review, decision-tier, human-review, recovery, or custom")
	cmd.Flags().StringVar(&llmAnswers, "llm-answers", "", "Record calling LLM's answers from JSON file (choice and reason per question)")
	return cmd
}

func readDecisionJSON(path string, stdin io.Reader, dst any) error {
	reader := stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, (256<<10)+1))
	if err != nil {
		return err
	}
	if len(data) > 256<<10 {
		return fmt.Errorf("semantic-decision input exceeds 256 KiB; narrow the evidence")
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("semantic-decision JSON: %w", err)
	}
	return nil
}

func decisionCredential(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	for _, kv := range env.ParseFile(filepath.Join(config.Dir(), ".env")) {
		if kv.Key == key {
			return kv.Val
		}
	}
	return ""
}

func logSemanticDecision(kind string, request decision.Request, result decision.Result, elapsed time.Duration) error {
	if telemetryMode() == "off" {
		return nil
	}
	data, _ := json.Marshal(request)
	choices := map[string]string{}
	for id, a := range result.Answers {
		choices[id] = a.Choice
	}
	// Correlate fallback and completion without storing source, criteria, reasons,
	// credentials, or provider response bodies in global analytics.
	record := struct {
		TS           string            `json:"ts"`
		Kind         string            `json:"kind"`
		DecisionKind string            `json:"decision_kind"`
		RequestID    string            `json:"request_id"`
		Ticket       string            `json:"ticket,omitempty"`
		Status       string            `json:"status"`
		Provider     string            `json:"provider"`
		Model        string            `json:"model,omitempty"`
		Reason       string            `json:"reason,omitempty"`
		Choices      map[string]string `json:"choices,omitempty"`
		DurationMS   int64             `json:"duration_ms"`
	}{time.Now().UTC().Format(time.RFC3339), "semantic-decision", kind, fmt.Sprintf("%x", sha256.Sum256(data)),
		os.Getenv("BABYSIT_TICKET"), result.Status, result.Provider, result.Model, result.Reason, choices, elapsed.Milliseconds()}
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(skillUsagePath()), "decisions.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(encoded, '\n'))
	return err
}

var decisionKinds = map[string]bool{
	"task-size": true, "task-complexity": true, "testcase": true, "review-finding": true,
	"skill-route": true, "orchestration": true, "deep-review": true, "custom": true,
	"decision-tier": true, "human-review": true, "recovery": true,
}

// Only user config grants external inference. Repository policy can subtract
// capabilities, never enable them or select credentials/provider/model.
func semanticDecisionPolicy(kind string) string {
	provider, _ := config.Get("semantic_decision_provider")
	if provider == "" || provider == "llm" {
		return "provider_llm"
	}
	if provider != "cloudflare" {
		return "invalid_provider"
	}
	root := gitOut("rev-parse", "--show-toplevel")
	if root == "" {
		root = secretsFindRepoRoot()
	}
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, ".babysit", "semantic-decision.yaml"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		return "invalid_project_policy"
	}
	var policy struct {
		Enabled      *bool     `yaml:"enabled"`
		AllowedKinds *[]string `yaml:"allowed_kinds"`
	}
	if yaml.Unmarshal(data, &policy) != nil {
		return "invalid_project_policy"
	}
	if policy.Enabled != nil && !*policy.Enabled {
		return "project_disabled"
	}
	if policy.AllowedKinds != nil {
		for _, allowed := range *policy.AllowedKinds {
			if allowed == kind {
				return ""
			}
		}
		return "project_kind_denied"
	}
	return ""
}
