package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// PendingWorkerDispatch inspects this terminal's current assignment without
// consuming or acknowledging mail. Orca removes the active binding when an
// accepted worker_done settles the Dispatch. Ticket names and old reports are
// deliberately not used to infer ownership. No binding also covers ordinary
// terminals and assignments whose ownership Orca has revoked.
func PendingWorkerDispatch(ctx context.Context, terminal string) (string, error) {
	if terminal == "" {
		return "", nil
	}
	bin, err := lookPath()
	if err != nil {
		return "", fmt.Errorf("cannot find Orca: %w", err)
	}
	// A Stop hook must not open the app or wait indefinitely for a down host.
	cmd := exec.CommandContext(ctx, bin, "orchestration", "check", "--terminal", terminal, "--peek", "--json")
	cmd.WaitDelay = 500 * time.Millisecond
	out, runErr := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	var response struct {
		OK     bool `json:"ok"`
		Result *struct {
			DispatchID string            `json:"dispatchId"`
			Messages   []json.RawMessage `json:"messages"`
			Count      *int              `json:"count"`
		} `json:"result"`
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &response); err != nil {
		return "", errors.New("Orca did not return a valid assignment response")
	}
	if !response.OK && response.Error != nil {
		switch response.Error.Code {
		case "consumer_fenced", "dispatch_inactive":
			// The worker contract explicitly forbids another report after
			// ownership is revoked. Do not keep this process reporting forever.
			return "", nil
		default:
			return "", fmt.Errorf("Orca assignment check failed (%s)", response.Error.Code)
		}
	}
	if runErr != nil {
		return "", fmt.Errorf("Orca assignment check failed: %w", runErr)
	}
	if !response.OK || response.Result == nil || response.Result.Count == nil || response.Result.Messages == nil {
		return "", errors.New("Orca assignment response is incomplete")
	}
	return response.Result.DispatchID, nil
}
