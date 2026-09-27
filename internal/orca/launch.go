package orca

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// LaunchRequest is the exact worker-start intent that the effective receipt
// must confirm. Empty model/effort mean the caller requested native defaults;
// those fields remain unknown unless Orca reports them.
type LaunchRequest struct {
	AgentID string
	HostID  string
	Model   string
	Effort  string
}

// EffectiveLaunch contains only non-secret fields required to prove that Orca
// started the requested agent and honored supported model/effort overrides.
type EffectiveLaunch struct {
	AgentID string `json:"agentId"`
	HostID  string `json:"hostId"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
}

// LaunchReceipt is the sanitized subset of worker-start's durable receipt.
type LaunchReceipt struct {
	DispatchID string          `json:"dispatchId"`
	Effective  EffectiveLaunch `json:"effective"`
}

// ValidateLaunchReceipt rejects missing or mismatched effective evidence. It
// accepts either the CLI envelope or its result object, but never a successful
// process exit without launch.effective.
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
		Dispatch struct {
			ID string `json:"id"`
		} `json:"dispatch"`
		Launch struct {
			Effective EffectiveLaunch `json:"effective"`
		} `json:"launch"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return LaunchReceipt{}, fmt.Errorf("worker-start receipt result is invalid: %w", err)
	}
	receipt := LaunchReceipt{DispatchID: result.Dispatch.ID, Effective: result.Launch.Effective}
	if receipt.DispatchID == "" || receipt.Effective.AgentID == "" {
		return receipt, errors.New("worker-start receipt is missing dispatch id or effective agent")
	}
	if !strings.EqualFold(receipt.Effective.AgentID, requested.AgentID) {
		return receipt, fmt.Errorf("worker-start effective agent %q does not match requested agent %q", receipt.Effective.AgentID, requested.AgentID)
	}
	if requested.HostID != "" && receipt.Effective.HostID != requested.HostID {
		return receipt, fmt.Errorf("worker-start effective host %q does not match requested host %q", receipt.Effective.HostID, requested.HostID)
	}
	if requested.Model != "" && receipt.Effective.Model != requested.Model {
		return receipt, fmt.Errorf("worker-start effective model %q does not match requested model %q", receipt.Effective.Model, requested.Model)
	}
	if requested.Effort != "" && receipt.Effective.Effort != requested.Effort {
		return receipt, fmt.Errorf("worker-start effective effort %q does not match requested effort %q", receipt.Effective.Effort, requested.Effort)
	}
	return receipt, nil
}
