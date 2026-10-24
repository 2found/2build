package orca

import (
	"strings"
	"testing"
)

// Captured from Orca 1.4.239 `agent launch --surface terminal --json`.
const agentLaunchResult = `{
  "id": "1354b0d4-3c92-4097-8523-83d8dc71595a",
  "ok": true,
  "result": {
    "outcome": {"kind": "terminal", "handle": "term_f00cccb1", "paneKey": "9c98:05ab"},
    "worktreeId": "2044c296::/repo",
    "receipt": {"mode": "terminal", "preferred": "terminal", "reason": "requested", "detail": "Started a terminal agent, as requested."},
    "prompt": {"delivery": "submit", "outcome": "handed-to-terminal"},
    "operationId": "1792874168743-94410751"
  }
}`

func TestParseAgentLaunchReadsTerminalAndPrompt(t *testing.T) {
	handle, outcome, err := ParseAgentLaunch([]byte(agentLaunchResult))
	if err != nil {
		t.Fatal(err)
	}
	if handle != "term_f00cccb1" || outcome != "handed-to-terminal" {
		t.Fatalf("handle=%q outcome=%q", handle, outcome)
	}
}

func TestParseAgentLaunchRefusesChatAndUndeliveredPrompt(t *testing.T) {
	chat := strings.Replace(agentLaunchResult, `"kind": "terminal"`, `"kind": "structured"`, 1)
	if _, _, err := ParseAgentLaunch([]byte(chat)); err == nil {
		t.Fatal("a chat surface was accepted as a worker terminal")
	}
	lost := strings.Replace(agentLaunchResult, `"handed-to-terminal"`, `"dropped"`, 1)
	handle, _, err := ParseAgentLaunch([]byte(lost))
	if err == nil || handle != "term_f00cccb1" {
		t.Fatalf("undelivered prompt: handle=%q err=%v (want the handle and an error)", handle, err)
	}
	if _, _, err := ParseAgentLaunch([]byte(`{"ok":false}`)); err == nil {
		t.Fatal("a failed launch was accepted")
	}
}

func TestVerifyLaunchChecksAgentAndHost(t *testing.T) {
	info := TerminalInfo{Handle: "term_1", AgentIdentity: "claude", ExecutionHostID: "local"}
	receipt := LaunchReceipt{DispatchID: "task-1.d1", TerminalHandle: "term_1"}
	if err := VerifyLaunch(&receipt, info, LaunchRequest{AgentID: "Claude", HostID: "local"}); err != nil {
		t.Fatal(err)
	}
	if receipt.Effective.AgentID == nil || *receipt.Effective.AgentID != "claude" || receipt.Effective.Model != nil {
		t.Fatalf("effective = %+v (model must stay unknown)", receipt.Effective)
	}
	for name, tc := range map[string]struct {
		info TerminalInfo
		req  LaunchRequest
	}{
		"wrong agent":      {info, LaunchRequest{AgentID: "codex"}},
		"wrong host":       {info, LaunchRequest{AgentID: "claude", HostID: "remote"}},
		"no agent":         {TerminalInfo{Handle: "term_1"}, LaunchRequest{AgentID: "claude"}},
		"another terminal": {info, LaunchRequest{AgentID: "claude", TerminalHandle: "term_2"}},
	} {
		r := LaunchReceipt{DispatchID: "task-1.d1", TerminalHandle: "term_1"}
		if err := VerifyLaunch(&r, tc.info, tc.req); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
