package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/2found/2build/internal/orca"
)

// runWorkerReportGate applies only to the calling Orca terminal. The hook
// never guesses a handle from cwd, a ticket, or a focused sibling terminal.
func runWorkerReportGate(stdout io.Writer) {
	terminal := os.Getenv("ORCA_TERMINAL_HANDLE")
	if terminal == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	dispatch, err := orca.PendingWorkerDispatch(ctx, terminal)
	if err == nil && dispatch == "" {
		return
	}
	var reason string
	if err != nil {
		reason = fmt.Sprintf("Cannot verify this Orca worker's report: %v. Restore access to the same Orca runtime and retry. Preserve the handoff; do not claim delivery or invent a Dispatch.", err)
	} else {
		reason = fmt.Sprintf("Orca Dispatch %s is still active. Before stopping, read coordinator follow-ups and report the actual terminal outcome with worker_done using the exact command and authority in your injected Orca preamble. Persist the handoff first; use --outcome failed if the work is incomplete. A nested skill finishing does not finish the Dispatch. If you already sent the report, inspect its receipt and follow Orca's recovery contract instead of sending a duplicate. Do not wait for a separate Foreman reply.", dispatch)
	}
	// Recheck on every Stop, including stop_hook_active continuations: that
	// flag is not evidence of a report. Interrupt/cancellation remains native.
	_ = json.NewEncoder(stdout).Encode(map[string]string{"decision": "block", "reason": reason})
}
