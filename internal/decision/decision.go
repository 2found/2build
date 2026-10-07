// Package decision evaluates evidence against explicit choices using Cloudflare
// Clef. An unavailable or uncertain provider hands judgment back to the caller.
package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type Request struct {
	State     json.RawMessage     `json:"state"`
	Questions map[string]Question `json:"questions"`
}

type Answer struct {
	Choice        string             `json:"choice"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Reason        string             `json:"reason,omitempty"`
}

type Result struct {
	Status   string            `json:"status"`
	Provider string            `json:"provider"`
	Model    string            `json:"model,omitempty"`
	Reason   string            `json:"reason,omitempty"`
	Answers  map[string]Answer `json:"answers,omitempty"`
}

var questionID = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,100}$`)

func (r Request) Validate() error {
	if !json.Valid(r.State) || string(r.State) == "null" || string(r.State) == `""` {
		return fmt.Errorf("state must contain evidence")
	}
	if len(r.Questions) < 1 || len(r.Questions) > 64 {
		return fmt.Errorf("questions must contain 1–64 entries")
	}
	for id, q := range r.Questions {
		if !questionID.MatchString(id) || q.Type != "choice" || strings.TrimSpace(q.Instructions) == "" || len(q.Criteria) < 2 {
			return fmt.Errorf("question %q needs a valid id, type choice, instructions and at least two criteria", id)
		}
		for option, description := range q.Criteria {
			if strings.TrimSpace(option) == "" || strings.TrimSpace(description) == "" {
				return fmt.Errorf("question %q has an empty criterion", id)
			}
		}
	}
	return nil
}

// ResolveLLM validates the caller's completed judgment; it never invents a
// confidence value for a model that did not supply probabilities.
func ResolveLLM(r Request, answers map[string]Answer) (Result, error) {
	if err := r.Validate(); err != nil {
		return Result{}, err
	}
	if len(answers) != len(r.Questions) {
		return Result{}, fmt.Errorf("LLM must answer every question")
	}
	clean := make(map[string]Answer, len(answers))
	for id, q := range r.Questions {
		a, ok := answers[id]
		if !ok || q.Criteria[a.Choice] == "" || strings.TrimSpace(a.Reason) == "" {
			return Result{}, fmt.Errorf("LLM answer %q needs an allowed choice and evidence-based reason", id)
		}
		clean[id] = Answer{Choice: a.Choice, Reason: a.Reason}
	}
	return Result{Status: "DECIDED", Provider: "llm", Answers: clean}, nil
}

type Client struct {
	AccountID string
	Token     string
	Model     string
	HTTP      *http.Client
}

func fallback(model, reason string) Result {
	return Result{Status: "FALLBACK_LLM", Provider: "llm", Model: model, Reason: reason}
}

func (c Client) Decide(ctx context.Context, r Request) (Result, error) {
	if err := r.Validate(); err != nil {
		return Result{}, err
	}
	model := c.Model
	if model == "" {
		model = "clef-flash"
	}
	if model != "clef" && model != "clef-flash" {
		return Result{}, fmt.Errorf("model must be clef or clef-flash")
	}
	if c.AccountID == "" || c.Token == "" {
		return fallback(model, "missing_credentials"), nil
	}
	payload, err := json.Marshal(struct {
		Model string `json:"model"`
		Request
	}{model, r})
	if err != nil {
		return Result{}, err
	}
	endpoint := "https://api.cloudflare.com/client/v4/accounts/" + url.PathEscape(c.AccountID) + "/ai/run/@cf/cloudflare/" + model
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{}
	}
	// Do not forward evidence or credentials through provider redirects.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := copyClient.Do(req)
	if err != nil {
		return fallback(model, "transport_error"), nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fallback(model, fmt.Sprintf("http_%d", response.StatusCode)), nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return fallback(model, "invalid_response"), nil
	}
	var envelope struct {
		Success bool `json:"success"`
		Result  struct {
			Answers map[string]Answer `json:"answers"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &envelope) != nil || !envelope.Success {
		return fallback(model, "invalid_response"), nil
	}
	answers := envelope.Result.Answers
	if len(answers) != len(r.Questions) {
		return fallback(model, "invalid_answers"), nil
	}
	for id, q := range r.Questions {
		a, ok := answers[id]
		if !ok || q.Criteria[a.Choice] == "" || a.Confidence == nil || len(a.Probabilities) != len(q.Criteria) {
			return fallback(model, "invalid_answers"), nil
		}
		total, best := 0.0, -1.0
		for choice := range q.Criteria {
			p, exists := a.Probabilities[choice]
			if !exists || math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
				return fallback(model, "invalid_answers"), nil
			}
			total += p
			if p > best {
				best = p
			}
		}
		selected := a.Probabilities[a.Choice]
		if math.Abs(total-1) > 0.01 || selected != best || math.Abs(*a.Confidence-selected) > 0.01 {
			return fallback(model, "invalid_answers"), nil
		}
		// This is a routing threshold, not a guarantee that the judgment is correct.
		if best < 0.75 {
			return fallback(model, "uncertain"), nil
		}
	}
	return Result{Status: "DECIDED", Provider: "cloudflare", Model: model, Answers: answers}, nil
}
