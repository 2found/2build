package orca

import (
	"encoding/json"
	"strings"
	"testing"
)

// Captured Orca 1.4.220 native-worker-start-receipt.json: native OMP defaults
// do not provide effective model/effort evidence.
const nativeOMPLaunchReceipt = `{
  "id": "9dede372-fdd0-4df6-b6c4-b61be9e7ecbc",
  "ok": true,
  "result": {
    "runId": "run_a37e8267c21d",
    "taskId": "task_40c8502da042",
    "dispatchId": "ctx_bc8a95e0a84e",
    "state": "ready",
    "stage": "input_accepted",
    "turnStart": "unsupported",
    "setup": {
      "requested": "not_applicable",
      "effective": "not_applicable",
      "source": "existing_worktree",
      "hookFound": false,
      "startupPolicy": "start-immediately",
      "state": "not_applicable"
    },
    "launch": {
      "requested": {"agent": "omp", "model": null, "effort": null},
      "effective": {"agent": "omp", "model": null, "effort": null}
    },
    "mode": {
      "mode": "terminal",
      "preferred": "terminal",
      "reason": "user_default",
      "detail": "Started a terminal agent worker, the default for new agent tabs in your settings."
    },
    "timeoutMs": 60000,
    "effects": [
      {"kind": "worktree", "action": "reused", "id": "91ebf70a-aae2-4bae-b1b7-daddd97b540b::/Users/chantran/Desktop/work/2market"},
      {"kind": "setup", "action": "not_applicable", "state": "not_applicable"},
      {"kind": "terminal", "role": "agent", "action": "created", "id": "term_1b4deab7-7372-4196-b0f1-d6791cb724c3", "surface": "visible"},
      {"kind": "dispatch_input", "role": "agent", "id": "term_1b4deab7-7372-4196-b0f1-d6791cb724c3", "state": "accepted"}
    ],
    "prompt": {
      "requestId": "41b604e1-b8bf-402f-a781-4f565dc6fc20",
      "stages": ["input_accepted"],
      "provider": "unsupported",
      "observation": "unsupported",
      "processIncarnation": "79c756a1-42af-4168-a219-459988021a52",
      "generation": 6,
      "baselineWorkingSequence": 0,
      "baselineExplicitWorkingStartedAt": null,
      "baselinePermissionSequence": 0
    },
    "residualResources": [],
    "mutation": {"requestId": "41b604e1-b8bf-402f-a781-4f565dc6fc20", "replayed": false}
  },
  "_meta": {"runtimeId": "9ffc2e23-1a24-4db9-b7d9-d6fd40384446"}
}`

func TestValidateLaunchReceiptAcceptsCurrentNativeOMPEnvelopeAndResult(t *testing.T) {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(nativeOMPLaunchReceipt), &envelope); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{name: "CLI envelope", raw: []byte(nativeOMPLaunchReceipt)},
		{name: "result payload", raw: envelope.Result},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateLaunchReceipt(tc.raw, LaunchRequest{AgentID: "omp"})
			if err != nil {
				t.Fatal(err)
			}
			if got.DispatchID != "ctx_bc8a95e0a84e" || got.Effective.AgentID == nil || *got.Effective.AgentID != "omp" ||
				got.Effective.HostID != "" || got.Effective.Model != nil || got.Effective.Effort != nil {
				t.Fatalf("native receipt = %+v", got)
			}
		})
	}
}

func TestValidateLaunchReceiptRejectsUnknownOrMismatchedOverrides(t *testing.T) {
	known := `{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":{"agent":"codex","hostId":"host-a","model":"tier-model","effort":"high"}}}`
	for _, tc := range []struct {
		name      string
		raw       string
		requested LaunchRequest
		wantError string
	}{
		{name: "agent mismatch", raw: nativeOMPLaunchReceipt, requested: LaunchRequest{AgentID: "codex"}, wantError: "effective agent"},
		{name: "unknown host", raw: nativeOMPLaunchReceipt, requested: LaunchRequest{AgentID: "omp", HostID: "host-a"}, wantError: "effective host"},
		{name: "host mismatch", raw: known, requested: LaunchRequest{AgentID: "codex", HostID: "host-b"}, wantError: "effective host"},
		{name: "unknown selected model", raw: nativeOMPLaunchReceipt, requested: LaunchRequest{AgentID: "omp", Model: "selected-model"}, wantError: "effective model is unknown"},
		{name: "selected model mismatch", raw: known, requested: LaunchRequest{AgentID: "codex", Model: "other-model"}, wantError: "effective model"},
		{name: "unknown selected effort", raw: nativeOMPLaunchReceipt, requested: LaunchRequest{AgentID: "omp", Effort: "high"}, wantError: "effective effort is unknown"},
		{name: "selected effort mismatch", raw: known, requested: LaunchRequest{AgentID: "codex", Effort: "low"}, wantError: "effective effort"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateLaunchReceipt([]byte(tc.raw), tc.requested); err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("ValidateLaunchReceipt error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestValidateLaunchReceiptRejectsMissingMalformedOrFailedEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{name: "malformed JSON", raw: `{"dispatchId":`},
		{name: "non-object", raw: `[]`},
		{name: "null", raw: `null`},
		{name: "failed envelope", raw: `{"ok":false,"result":{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":{"agent":"omp"}}}}`},
		{name: "missing envelope result", raw: `{"ok":true}`},
		{name: "null envelope result", raw: `{"ok":true,"result":null}`},
		{name: "malformed envelope result", raw: `{"ok":true,"result":[]}`},
		{name: "missing readiness", raw: `{"dispatchId":"ctx-1","launch":{"effective":{"agent":"omp"}}}`},
		{name: "failed readiness", raw: `{"dispatchId":"ctx-1","state":"failed","stage":"input_accepted","launch":{"effective":{"agent":"omp"}}}`},
		{name: "pending readiness", raw: `{"dispatchId":"ctx-1","state":"starting","stage":"input_accepted","launch":{"effective":{"agent":"omp"}}}`},
		{name: "input not accepted", raw: `{"dispatchId":"ctx-1","state":"ready","stage":"input_pending","launch":{"effective":{"agent":"omp"}}}`},
		{name: "missing input acceptance", raw: `{"dispatchId":"ctx-1","state":"ready","launch":{"effective":{"agent":"omp"}}}`},
		{name: "missing dispatch", raw: `{"state":"ready","stage":"input_accepted","launch":{"effective":{"agent":"omp"}}}`},
		{name: "missing launch", raw: `{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted"}`},
		{name: "missing effective", raw: `{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"requested":{"agent":"omp"}}}`},
		{name: "null effective", raw: `{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":null}}`},
		{name: "missing effective agent", raw: `{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":{"model":null,"effort":null}}}`},
		{name: "malformed effective", raw: `{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":"omp"}}`},
		{name: "malformed model", raw: `{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":{"agent":"omp","model":42}}}`},
		{name: "retired dispatch shape", raw: `{"dispatch":{"id":"ctx-1"},"state":"ready","stage":"input_accepted","launch":{"effective":{"agent":"omp"}}}`},
		{name: "retired agent shape", raw: `{"dispatchId":"ctx-1","state":"ready","stage":"input_accepted","launch":{"effective":{"agentId":"omp"}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateLaunchReceipt([]byte(tc.raw), LaunchRequest{AgentID: "omp"}); err == nil {
				t.Fatal("invalid worker-start receipt was accepted")
			}
		})
	}
}

// Orca 1.4.220 reports no effective launch settings when supervising a terminal
// created with a native startup command. Only the exact transport is attested.
func TestValidateLaunchReceiptRequiresExactReusedTerminal(t *testing.T) {
	const raw = `{"ok":true,"result":{"dispatchId":"ctx-reused","state":"ready","stage":"input_accepted","launch":{"effective":{"agent":null,"model":null,"effort":null}},"effects":[{"kind":"terminal","role":"agent","action":"reused","id":"term-native"},{"kind":"dispatch_input","role":"agent","id":"term-native","state":"accepted"}]}}`
	request := LaunchRequest{AgentID: "omp", TerminalHandle: "term-native"}
	got, err := ValidateLaunchReceipt([]byte(raw), request)
	if err != nil {
		t.Fatal(err)
	}
	if got.DispatchID != "ctx-reused" || got.TerminalHandle != request.TerminalHandle ||
		got.Effective.AgentID != nil || got.Effective.Model != nil || got.Effective.Effort != nil {
		t.Fatalf("reused transport fabricated effective settings: %+v", got)
	}
	for _, tc := range []struct {
		name      string
		raw       string
		requested LaunchRequest
	}{
		{name: "no expected terminal", raw: raw, requested: LaunchRequest{AgentID: "omp"}},
		{name: "wrong terminal", raw: raw, requested: LaunchRequest{AgentID: "omp", TerminalHandle: "term-other"}},
		{name: "model unverified", raw: raw, requested: LaunchRequest{AgentID: "omp", TerminalHandle: "term-native", Model: "@slow"}},
		{name: "effort unverified", raw: raw, requested: LaunchRequest{AgentID: "omp", TerminalHandle: "term-native", Effort: "medium"}},
		{name: "host unverified", raw: raw, requested: LaunchRequest{AgentID: "omp", TerminalHandle: "term-native", HostID: "remote"}},
		{name: "not reused", raw: strings.Replace(raw, `"action":"reused"`, `"action":"created"`, 1), requested: request},
		{name: "input not accepted", raw: strings.Replace(raw, `"state":"accepted"`, `"state":"pending"`, 1), requested: request},
		{name: "input to another terminal", raw: strings.Replace(raw, `"id":"term-native","state":"accepted"`, `"id":"term-other","state":"accepted"`, 1), requested: request},
		{name: "missing effective", raw: strings.Replace(raw, `"effective":{"agent":null,"model":null,"effort":null}`, `"effective":null`, 1), requested: request},
		{name: "contradictory agent", raw: strings.Replace(raw, `"agent":null`, `"agent":"codex"`, 1), requested: request},
		{name: "not ready", raw: strings.Replace(raw, `"state":"ready"`, `"state":"failed"`, 1), requested: request},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateLaunchReceipt([]byte(tc.raw), tc.requested); err == nil {
				t.Fatal("unproven reused-terminal launch was accepted")
			}
		})
	}
}
