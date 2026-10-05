package orca

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// LaunchRequest is the worker-start intent. Existing terminals attest only
// transport; their native agent/model settings need separate session evidence.
type LaunchRequest struct {
	AgentID        string
	HostID         string
	Model          string
	Effort         string
	TerminalHandle string
}

// EffectiveLaunch preserves unknown settings instead of deriving them from
// the requested command or a reused terminal.
type EffectiveLaunch struct {
	AgentID *string `json:"agentId"`
	HostID  string  `json:"hostId"`
	Model   *string `json:"model"`
	Effort  *string `json:"effort"`
}

// LaunchReceipt is the sanitized subset of worker-start's durable receipt.
type LaunchReceipt struct {
	DispatchID     string          `json:"dispatchId"`
	TerminalHandle string          `json:"terminalHandle,omitempty"`
	Effective      EffectiveLaunch `json:"effective"`
}

// ValidateLaunchReceipt rejects missing or mismatched effective evidence. It
// accepts either the CLI envelope or its result object, but requires a ready
// worker with accepted input and launch.effective.
func ValidateLaunchReceipt(raw []byte, requested LaunchRequest) (LaunchReceipt, error) {
	var envelope struct {
		OK     *bool           `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return LaunchReceipt{}, fmt.Errorf("worker-start receipt is invalid JSON: %w", err)
	}
	if envelope.OK != nil {
		if !*envelope.OK {
			return LaunchReceipt{}, errors.New("worker-start receipt reports failure")
		}
		if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
			return LaunchReceipt{}, errors.New("worker-start receipt is missing result")
		}
		raw = envelope.Result
	}
	var result struct {
		DispatchID string `json:"dispatchId"`
		State      string `json:"state"`
		Stage      string `json:"stage"`
		Launch     struct {
			Effective *struct {
				Agent  *string `json:"agent"`
				HostID string  `json:"hostId"`
				Model  *string `json:"model"`
				Effort *string `json:"effort"`
			} `json:"effective"`
		} `json:"launch"`
		Effects []struct {
			Kind   string `json:"kind"`
			Role   string `json:"role"`
			Action string `json:"action"`
			ID     string `json:"id"`
			State  string `json:"state"`
		} `json:"effects"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return LaunchReceipt{}, fmt.Errorf("worker-start receipt result is invalid: %w", err)
	}
	if result.State != "ready" || result.Stage != "input_accepted" {
		return LaunchReceipt{}, fmt.Errorf("worker-start receipt is not ready with accepted input: state %q, stage %q", result.State, result.Stage)
	}
	if result.Launch.Effective == nil {
		return LaunchReceipt{}, errors.New("worker-start receipt is missing launch.effective")
	}
	receipt := LaunchReceipt{
		DispatchID: result.DispatchID,
		Effective: EffectiveLaunch{
			AgentID: result.Launch.Effective.Agent,
			HostID:  result.Launch.Effective.HostID,
			Model:   result.Launch.Effective.Model,
			Effort:  result.Launch.Effective.Effort,
		},
	}
	if receipt.DispatchID == "" {
		return receipt, errors.New("worker-start receipt is missing dispatch id")
	}
	if requested.TerminalHandle != "" {
		var reused, accepted bool
		for _, effect := range result.Effects {
			if effect.Role != "agent" || effect.ID != requested.TerminalHandle {
				continue
			}
			reused = reused || (effect.Kind == "terminal" && effect.Action == "reused")
			accepted = accepted || (effect.Kind == "dispatch_input" && effect.State == "accepted")
		}
		if !reused || !accepted {
			return receipt, fmt.Errorf("worker-start receipt does not confirm reused terminal %q with accepted input", requested.TerminalHandle)
		}
		receipt.TerminalHandle = requested.TerminalHandle
	} else if receipt.Effective.AgentID == nil || *receipt.Effective.AgentID == "" {
		return receipt, errors.New("worker-start receipt is missing effective agent")
	}
	if receipt.Effective.AgentID != nil && !strings.EqualFold(*receipt.Effective.AgentID, requested.AgentID) {
		return receipt, fmt.Errorf("worker-start effective agent %q does not match requested agent %q", *receipt.Effective.AgentID, requested.AgentID)
	}
	if requested.HostID != "" && receipt.Effective.HostID != requested.HostID {
		return receipt, fmt.Errorf("worker-start effective host %q does not match requested host %q", receipt.Effective.HostID, requested.HostID)
	}
	if requested.Model != "" {
		if receipt.Effective.Model == nil {
			return receipt, fmt.Errorf("worker-start effective model is unknown, requested model %q", requested.Model)
		}
		if *receipt.Effective.Model != requested.Model {
			return receipt, fmt.Errorf("worker-start effective model %q does not match requested model %q", *receipt.Effective.Model, requested.Model)
		}
	}
	if requested.Effort != "" {
		if receipt.Effective.Effort == nil {
			return receipt, fmt.Errorf("worker-start effective effort is unknown, requested effort %q", requested.Effort)
		}
		if *receipt.Effective.Effort != requested.Effort {
			return receipt, fmt.Errorf("worker-start effective effort %q does not match requested effort %q", *receipt.Effective.Effort, requested.Effort)
		}
	}
	return receipt, nil
}
