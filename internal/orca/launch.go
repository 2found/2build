package orca

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// AgentLaunch is one `orca agent launch` of a worker CLI in a worktree. Orca
// starts the agent the way its new-agent tab does and types prompt in; it
// injects no orchestration preamble, so the worker carries only the bbs
// protocol its prompt names.
type AgentLaunch struct {
	Agent    string
	Worktree string
	Model    string
	Effort   string
	Prompt   string
}

// LaunchAgent starts the agent as a terminal and returns Orca's raw result.
func (c *Client) LaunchAgent(l AgentLaunch) (json.RawMessage, error) {
	wt := l.Worktree
	if resolved, err := filepath.EvalSymlinks(wt); err == nil {
		wt = resolved
	}
	args := []string{"agent", "launch", "--agent", l.Agent, "--worktree", "path:" + wt, "--surface", "terminal"}
	if l.Model != "" {
		args = append(args, "--model", l.Model)
	}
	if l.Effort != "" {
		args = append(args, "--effort", l.Effort)
	}
	if l.Prompt != "" {
		args = append(args, "--prompt", l.Prompt)
	}
	return c.run(args...)
}

// TerminalInfo is what `terminal show` attests about a launched worker.
type TerminalInfo struct {
	Handle          string `json:"handle"`
	WorktreePath    string `json:"worktreePath"`
	AgentIdentity   string `json:"agentIdentity"`
	ExecutionHostID string `json:"executionHostId"`
}

// ShowTerminal reads one terminal by exact handle.
func (c *Client) ShowTerminal(handle string) (TerminalInfo, error) {
	raw, err := c.run("terminal", "show", "--terminal", handle)
	if err != nil {
		return TerminalInfo{}, err
	}
	var wrap struct {
		Terminal TerminalInfo `json:"terminal"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return TerminalInfo{}, fmt.Errorf("terminal show: %w", err)
	}
	return wrap.Terminal, nil
}

// LaunchRequest is what Foreman asked for. Orca attests the agent and host
// of the terminal; it does not report the effective model or effort, so the
// worker confirms those from its own session before ticket work.
type LaunchRequest struct {
	AgentID        string
	HostID         string
	Model          string
	Effort         string
	TerminalHandle string
}

// EffectiveLaunch preserves unknown settings instead of deriving them from
// the request.
type EffectiveLaunch struct {
	AgentID *string `json:"agentId"`
	HostID  string  `json:"hostId"`
	Model   *string `json:"model"`
	Effort  *string `json:"effort"`
}

// LaunchReceipt is the sanitized evidence of one Dispatch's launch.
type LaunchReceipt struct {
	DispatchID     string          `json:"dispatchId"`
	TerminalHandle string          `json:"terminalHandle"`
	Reused         bool            `json:"reused,omitempty"`
	PromptOutcome  string          `json:"promptOutcome,omitempty"`
	Effective      EffectiveLaunch `json:"effective"`
}

// ParseAgentLaunch reads the terminal handle and prompt delivery from an
// `agent launch` result (the CLI envelope or its result object). A chat
// surface or an undelivered prompt is not a usable worker.
func ParseAgentLaunch(raw []byte) (handle, promptOutcome string, err error) {
	var envelope struct {
		OK     *bool           `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", "", fmt.Errorf("agent launch result is invalid JSON: %w", err)
	}
	if envelope.OK != nil {
		if !*envelope.OK {
			return "", "", errors.New("agent launch reports failure")
		}
		raw = envelope.Result
	}
	var result struct {
		Outcome struct {
			Kind   string `json:"kind"`
			Handle string `json:"handle"`
		} `json:"outcome"`
		Prompt struct {
			Outcome string `json:"outcome"`
		} `json:"prompt"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", "", fmt.Errorf("agent launch result is invalid: %w", err)
	}
	if result.Outcome.Kind != "terminal" || result.Outcome.Handle == "" {
		return "", "", fmt.Errorf("agent launch did not start a terminal agent (outcome %q)", result.Outcome.Kind)
	}
	switch result.Prompt.Outcome {
	case "handed-to-terminal", "journaled":
	default:
		return result.Outcome.Handle, result.Prompt.Outcome, fmt.Errorf("agent launch did not deliver the prompt (outcome %q)", result.Prompt.Outcome)
	}
	return result.Outcome.Handle, result.Prompt.Outcome, nil
}

// VerifyLaunch checks the launched terminal against the request: the agent
// Orca detected in it and its execution host. Model and effort stay unknown.
func VerifyLaunch(receipt *LaunchReceipt, info TerminalInfo, requested LaunchRequest) error {
	if info.AgentIdentity != "" {
		agent := info.AgentIdentity
		receipt.Effective.AgentID = &agent
	}
	receipt.Effective.HostID = info.ExecutionHostID
	if requested.TerminalHandle != "" && receipt.TerminalHandle != requested.TerminalHandle {
		return fmt.Errorf("launch used terminal %q, requested %q", receipt.TerminalHandle, requested.TerminalHandle)
	}
	if info.Handle != "" && info.Handle != receipt.TerminalHandle {
		return fmt.Errorf("terminal show returned %q for %q", info.Handle, receipt.TerminalHandle)
	}
	if receipt.Effective.AgentID == nil {
		return errors.New("launched terminal reports no agent")
	}
	if !strings.EqualFold(*receipt.Effective.AgentID, requested.AgentID) {
		return fmt.Errorf("launched agent %q does not match requested agent %q", *receipt.Effective.AgentID, requested.AgentID)
	}
	if requested.HostID != "" && receipt.Effective.HostID != requested.HostID {
		return fmt.Errorf("launched host %q does not match requested host %q", receipt.Effective.HostID, requested.HostID)
	}
	return nil
}
