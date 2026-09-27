package orca

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Orca owns worker communication and lifecycle. Babysit reads Dispatch state
// here only to reconcile its machine-global resource leases.

// CapOrchestration is the runtime capability required for Dispatch inspection.
const CapOrchestration = "orchestration.contract.v1"

// ErrNoOrchestration means this runtime does not advertise CapOrchestration.
var ErrNoOrchestration = errors.New("orca runtime does not serve " + CapOrchestration)

// Supports reports whether the runtime advertised a capability at Preflight.
// The list is read once, at status time, rather than per call: it changes only
// when the app restarts, and a probe per call would put an RPC in front of
// every resource check.
func (c *Client) Supports(capability string) bool {
	for _, have := range c.caps {
		if have == capability {
			return true
		}
	}
	return false
}

// Orchestration reports whether the runtime supports Dispatch inspection.
func (c *Client) Orchestration() bool { return c.Supports(CapOrchestration) }

// DispatchState identifies one attempt independently of its worker terminal.
// dispatch_contexts.status is 'pending', 'dispatched', 'completed', 'failed',
// or 'circuit_broken'; the last three are terminal.
type DispatchState struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	RunID  string `json:"run_id"`
}

// DispatchStateFor reads the exact current attempt, including its Run.
func (c *Client) DispatchStateFor(taskID string) (DispatchState, error) {
	var dispatch DispatchState
	if !c.Orchestration() {
		return dispatch, ErrNoOrchestration
	}
	if taskID == "" {
		return dispatch, errors.New("orca dispatch-show: needs a task")
	}
	raw, err := c.run("orchestration", "dispatch-show", "--task", taskID)
	if err != nil {
		return dispatch, err
	}
	var wrap struct {
		Dispatch json.RawMessage `json:"dispatch"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return dispatch, fmt.Errorf("orca dispatch-show: %w", err)
	}
	if len(wrap.Dispatch) == 0 {
		return dispatch, errors.New("orca dispatch-show: missing dispatch field")
	}
	if string(wrap.Dispatch) != "null" {
		if err := json.Unmarshal(wrap.Dispatch, &dispatch); err != nil {
			return dispatch, err
		}
		if dispatch.ID == "" || dispatch.Status == "" {
			return dispatch, errors.New("orca dispatch-show: incomplete dispatch")
		}
	}
	return dispatch, nil
}
