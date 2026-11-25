package decision

import (
	"encoding/json"
	"testing"
)

func request() Request {
	return Request{State: json.RawMessage(`"cited evidence"`), Questions: map[string]Question{
		"size": {Type: "choice", Instructions: "Choose size", Criteria: map[string]string{"S": "small", "L": "large"}},
	}}
}

func TestResolveLLM(t *testing.T) {
	answers := map[string]Answer{"size": {Choice: "L", Reason: "Evidence: breaking API in file.go:3"}}
	result, err := ResolveLLM(request(), answers)
	if err != nil || result.Status != "DECIDED" || result.Provider != "llm" || result.Answers["size"].Choice != "L" {
		t.Fatalf("%+v %v", result, err)
	}
	for _, bad := range []map[string]Answer{nil, {"size": {Choice: "X", Reason: "x"}}, {"size": {Choice: "S"}}, {"other": {Choice: "S", Reason: "x"}}} {
		if _, err := ResolveLLM(request(), bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

func TestInvalidRequestRejected(t *testing.T) {
	answers := map[string]Answer{"size": {Choice: "S", Reason: "x"}}
	for _, r := range []Request{{}, {State: json.RawMessage(`null`), Questions: request().Questions}, {State: json.RawMessage(`"x"`), Questions: map[string]Question{"bad id": request().Questions["size"]}}} {
		if _, err := ResolveLLM(r, answers); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
}
