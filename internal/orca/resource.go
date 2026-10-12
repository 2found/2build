package orca

import (
	"encoding/json"
	"errors"
)

// Worker is one supervised Dispatch's terminal as the fleet view reports it.
type Worker struct {
	Dispatch string
	// Handle is the agent's terminal; empty once released or taken over by a user.
	Handle string
	// Exited is the fleet's agent verdict (not merely PTY liveness).
	Exited bool
}

// ExitedWorkers reads the fleet's agent verdict (not merely PTY liveness).
// Scope and pagination matter: an old foreman's workers may be on later pages.
func (c *Client) ExitedWorkers(run string) (map[string]bool, error) {
	workers, err := c.Workers(run)
	if err != nil {
		return nil, err
	}
	exited := map[string]bool{}
	for _, w := range workers {
		exited[w.Dispatch] = w.Exited
	}
	return exited, nil
}

// Workers lists every supervised worker of a Run, following page cursors.
func (c *Client) Workers(run string) ([]Worker, error) {
	if run == "" {
		return nil, errors.New("worker-list needs a run")
	}
	var workers []Worker
	cursor := ""
	seen := map[string]bool{}
	for {
		args := []string{"orchestration", "worker-list", "--run", run, "--include-remote"}
		if cursor != "" {
			args = append(args, "--cursor", cursor)
		}
		raw, err := c.run(args...)
		if err != nil {
			return nil, err
		}
		var page struct {
			Workers []struct {
				DispatchID          string `json:"dispatchId"`
				DispatchStatus      string `json:"dispatchStatus"`
				TerminalState       string `json:"terminalState"`
				AgentTerminalHandle string `json:"agentTerminalHandle"`
				Resource            *struct {
					RetainedReason string `json:"retainedReason"`
				} `json:"resource"`
				Projection struct {
					Liveness struct {
						Verdict string `json:"verdict"`
					} `json:"liveness"`
				} `json:"projection"`
			} `json:"workers"`
			Page struct {
				HasMore    bool   `json:"hasMore"`
				NextCursor string `json:"nextCursor"`
			} `json:"page"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		for _, worker := range page.Workers {
			// A settled dispatch whose terminal was reused by a later dispatch
			// (retained) or already closed (released) cannot still be running:
			// its liveness row is gone with the terminal, which projects as
			// "unverifiable" even though the dispatch itself is completed.
			settled := worker.DispatchStatus == "completed" || worker.DispatchStatus == "failed"
			terminalGone := worker.TerminalState == "retained" || worker.TerminalState == "released"
			// A context-only Dispatch (native-terminal launch) is always
			// retained with no resource row while its agent still works, so
			// only a released or user-taken-over terminal loses its handle.
			handle := worker.AgentTerminalHandle
			if worker.TerminalState == "released" || (worker.Resource != nil && worker.Resource.RetainedReason == "user_takeover") {
				handle = ""
			}
			workers = append(workers, Worker{
				Dispatch: worker.DispatchID,
				Handle:   handle,
				Exited:   worker.Projection.Liveness.Verdict == "exited" || (settled && terminalGone),
			})
		}
		if !page.Page.HasMore {
			return workers, nil
		}
		cursor = page.Page.NextCursor
		if cursor == "" || seen[cursor] {
			return nil, errors.New("worker-list returned an invalid page cursor")
		}
		seen[cursor] = true
	}
}

// StopExitedWorker fences a proven exited attempt before capacity is reused.
// Callers must re-read Dispatch state; an acknowledged request alone is not
// proof that stop settled, especially on a remote host.
func (c *Client) StopExitedWorker(dispatch string) error {
	_, err := c.run("orchestration", "worker-stop", "--dispatch", dispatch)
	return err
}
