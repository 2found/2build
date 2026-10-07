package decision

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func request() Request {
	return Request{State: json.RawMessage(`"cited evidence"`), Questions: map[string]Question{
		"size": {Type: "choice", Instructions: "Choose size", Criteria: map[string]string{"S": "small", "L": "large"}},
	}}
}

const validResponse = `{"success":true,"result":{"answers":{"size":{"choice":"S","confidence":0.9,"probabilities":{"S":0.9,"L":0.1}}}}}`

func TestCloudflareContract(t *testing.T) {
	for _, model := range []string{"clef", "clef-flash"} {
		t.Run(model, func(t *testing.T) {
			client := Client{AccountID: "account", Token: "token", Model: model, HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "POST" || r.URL.String() != "https://api.cloudflare.com/client/v4/accounts/account/ai/run/@cf/cloudflare/"+model || r.Header.Get("Authorization") != "Bearer token" {
					t.Fatalf("bad request: %v", r.URL)
				}
				if _, ok := r.Context().Deadline(); !ok {
					t.Fatal("missing timeout")
				}
				var payload struct {
					Model string
					Request
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Model != model || payload.Questions["size"].Type != "choice" || string(payload.State) != `"cited evidence"` {
					t.Fatalf("bad payload: %+v", payload)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(validResponse)), Header: make(http.Header)}, nil
			})}}
			result, err := client.Decide(context.Background(), request())
			if err != nil || result.Status != "DECIDED" || result.Provider != "cloudflare" || result.Answers["size"].Choice != "S" {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
}

func TestProviderFallback(t *testing.T) {
	cases := []struct {
		name, body, reason string
		status             int
	}{
		{"http", `secret provider body`, "http_429", 429},
		{"redirect", ``, "http_302", 302},
		{"api error", `{"success":false}`, "invalid_response", 200},
		{"malformed", `nope`, "invalid_response", 200},
		{"oversized", strings.Repeat("x", (1<<20)+1), "invalid_response", 200},
		{"missing answers", `{"success":true,"result":{"answers":{}}}`, "invalid_answers", 200},
		{"unknown choice", strings.Replace(validResponse, `"choice":"S"`, `"choice":"X"`, 1), "invalid_answers", 200},
		{"missing confidence", strings.Replace(validResponse, `"confidence":0.9,`, "", 1), "invalid_answers", 200},
		{"null confidence", strings.Replace(validResponse, `"confidence":0.9`, `"confidence":null`, 1), "invalid_answers", 200},
		{"wrong confidence", strings.Replace(validResponse, `"confidence":0.9`, `"confidence":0.8`, 1), "invalid_answers", 200},
		{"missing probability", strings.Replace(validResponse, `,"L":0.1`, "", 1), "invalid_answers", 200},
		{"bad sum", strings.Replace(validResponse, `"L":0.1`, `"L":0.4`, 1), "invalid_answers", 200},
		{"low confidence", strings.ReplaceAll(strings.ReplaceAll(validResponse, "0.9", "0.6"), "0.1", "0.4"), "uncertain", 200},
		{"tie", strings.ReplaceAll(strings.ReplaceAll(validResponse, "0.9", "0.5"), "0.1", "0.5"), "uncertain", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := Client{AccountID: "a", Token: "t", HTTP: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}}
			result, err := client.Decide(context.Background(), request())
			if err != nil || result.Status != "FALLBACK_LLM" || result.Reason != tc.reason || len(result.Answers) != 0 {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
	client := Client{AccountID: "a", Token: "t", HTTP: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { return nil, errors.New("secret transport detail") })}}
	result, err := client.Decide(context.Background(), request())
	if err != nil || result.Reason != "transport_error" {
		t.Fatalf("%+v %v", result, err)
	}
	result, err = (Client{}).Decide(context.Background(), request())
	if err != nil || result.Reason != "missing_credentials" {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestFallbackIsAtomicAcrossQuestions(t *testing.T) {
	r := request()
	r.Questions["other"] = r.Questions["size"]
	client := Client{AccountID: "a", Token: "t", HTTP: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(validResponse))}, nil
	})}}
	result, err := client.Decide(context.Background(), r)
	if err != nil || result.Status != "FALLBACK_LLM" || len(result.Answers) != 0 {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestLLMFallback(t *testing.T) {
	answers := map[string]Answer{"size": {Choice: "L", Reason: "Evidence: breaking API in file.go:3"}}
	result, err := ResolveLLM(request(), answers)
	if err != nil || result.Provider != "llm" || result.Answers["size"].Confidence != nil {
		t.Fatalf("%+v %v", result, err)
	}
	for _, bad := range []map[string]Answer{nil, {"size": {Choice: "X", Reason: "x"}}, {"size": {Choice: "S"}}, {"other": {Choice: "S", Reason: "x"}}} {
		if _, err := ResolveLLM(request(), bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

func TestInvalidRequestNeverCallsProvider(t *testing.T) {
	client := Client{AccountID: "a", Token: "t", HTTP: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { t.Fatal("network called"); return nil, nil })}}
	for _, r := range []Request{{}, {State: json.RawMessage(`null`), Questions: request().Questions}, {State: json.RawMessage(`"x"`), Questions: map[string]Question{"bad id": request().Questions["size"]}}} {
		if _, err := client.Decide(context.Background(), r); err == nil {
			t.Fatalf("accepted %+v", r)
		}
	}
}
