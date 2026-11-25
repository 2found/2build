// Package decision validates the calling LLM's judgment of evidence against
// explicit choices. It never launches a model.
package decision

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
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
	Choice string `json:"choice"`
	Reason string `json:"reason,omitempty"`
}

type Result struct {
	Status   string            `json:"status"`
	Provider string            `json:"provider"`
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

// ResolveLLM validates the caller's completed judgment against the request.
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
