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

// PendingMessagesFor lists unread messages of the given types addressed to
// terminal without consuming them: `check --peek` filters rows by type and
// never marks them read, so the recipient still receives them in its next
// Delivery. The terminal is explicit because the caller is a watcher outside it.
// It returns the unread count and the ids of the rows it saw; ids is nil when
// the runtime reports only a count.
func (c *Client) PendingMessagesFor(terminal, types string) (int, []string, error) {
	if !c.Orchestration() {
		return 0, nil, ErrNoOrchestration
	}
	raw, err := c.run("orchestration", "check", "--terminal", terminal, "--peek", "--types", types)
	if err != nil {
		return 0, nil, err
	}
	var result struct {
		Count    *int `json:"count"`
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, nil, fmt.Errorf("orca check --peek: %w", err)
	}
	var ids []string
	for _, m := range result.Messages {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	n := len(result.Messages)
	if result.Count != nil {
		n = *result.Count
	}
	return n, ids, nil
}

// HoldMailbox opens (or replays) the coordinator's Delivery for run without
// acknowledging it and reports whether one is outstanding. Orca types a
// "You have N orchestration messages" pointer into an idle coordinator for
// every arriving message unless that mailbox has an unacknowledged Delivery
// (measured on Orca 1.4.224), so holding one is what keeps worker_done from
// waking it per worker. The coordinator still receives the same messages:
// its next `check` replays this Delivery, and its ack releases the hold.
func (c *Client) HoldMailbox(run, terminal string) (bool, error) {
	if !c.Orchestration() {
		return false, ErrNoOrchestration
	}
	raw, err := c.run("orchestration", "check", "--run", run, "--terminal", terminal)
	if err != nil {
		return false, err
	}
	var result struct {
		DeliveryID string `json:"deliveryId"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return false, fmt.Errorf("orca check: %w", err)
	}
	return result.DeliveryID != "", nil
}

// SendMessage enqueues a durable message for a terminal or Run address, sent
// as from: a caller outside any Orca terminal (the launchd watcher) has no
// sender of its own, and Orca refuses the send without one.
func (c *Client) SendMessage(from, to, subject, body string) error {
	_, err := c.run("orchestration", "send", "--from", from, "--to", to, "--subject", subject, "--body", body)
	return err
}
