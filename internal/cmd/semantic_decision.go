package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/2found/2build/internal/decision"
	"github.com/spf13/cobra"
)

func newSemanticDecisionCmd() *cobra.Command {
	var input, kind, llmAnswers string
	cmd := &cobra.Command{
		Use: "semantic-decision", Short: "Validate and record the calling LLM's bounded judgment",
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
			if llmAnswers == "" {
				return fmt.Errorf("semantic-decision records the calling LLM's judgment in one call: add --llm-answers <answers.json> (choice and reason per question ID)")
			}
			if input == "-" && llmAnswers == "-" {
				return fmt.Errorf("input and llm-answers cannot both read stdin")
			}
			start := time.Now()
			var answers map[string]decision.Answer
			if err := readDecisionJSON(llmAnswers, cmd.InOrStdin(), &answers); err != nil {
				return err
			}
			result, err := decision.ResolveLLM(request, answers)
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
	cmd.Flags().StringVar(&llmAnswers, "llm-answers", "", "Calling LLM's answers JSON file (choice and reason per question ID); required")
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

func logSemanticDecision(kind string, request decision.Request, result decision.Result, elapsed time.Duration) error {
	if telemetryMode() == "off" {
		return nil
	}
	data, _ := json.Marshal(request)
	choices := map[string]string{}
	for id, a := range result.Answers {
		choices[id] = a.Choice
	}
	// Correlate decisions without storing source, criteria or reasons in global analytics.
	record := struct {
		TS           string            `json:"ts"`
		Kind         string            `json:"kind"`
		DecisionKind string            `json:"decision_kind"`
		RequestID    string            `json:"request_id"`
		Ticket       string            `json:"ticket,omitempty"`
		Status       string            `json:"status"`
		Provider     string            `json:"provider"`
		Reason       string            `json:"reason,omitempty"`
		Choices      map[string]string `json:"choices,omitempty"`
		DurationMS   int64             `json:"duration_ms"`
	}{time.Now().UTC().Format(time.RFC3339), "semantic-decision", kind, fmt.Sprintf("%x", sha256.Sum256(data)),
		os.Getenv("BABYSIT_TICKET"), result.Status, result.Provider, result.Reason, choices, elapsed.Milliseconds()}
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
