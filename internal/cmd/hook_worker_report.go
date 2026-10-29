package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/2found/2build/internal/foreman"
)

// runWorkerReportGate applies only to the calling Orca terminal: it blocks a
// Stop while a Foreman Dispatch launched into this exact terminal is still
// open. The hook never guesses a Dispatch from cwd, a ticket, or a sibling
// terminal, and needs no Orca call: bbs owns the Dispatch record.
func runWorkerReportGate(stdout io.Writer) {
	t, d := foreman.DispatchForTerminal(os.Getenv("ORCA_TERMINAL_HANDLE"))
	if d == nil {
		return
	}
	reason := fmt.Sprintf("Team Task %s (Dispatch %s) is still open. Before stopping, persist the handoff and report the actual outcome once: `bbs team task done %s --dispatch %s --outcome succeeded|failed --summary \"<did; found; left>\"` (failed when the work is incomplete). A nested skill finishing does not finish the Task. Do not wait for a separate Foreman reply.", t.ID, d.ID, t.ID, d.ID)
	if pending := foreman.PendingQuestions([]string{t.ID}); len(pending) > 0 {
		q := pending[0]
		reason = fmt.Sprintf("Team Task %s has your unanswered question %s. Keep waiting with `bbs team task ask %s --resume %s`; never ask it again.", t.ID, q.ID, t.ID, q.ID)
	}
	// Recheck on every Stop, including stop_hook_active continuations: that
	// flag is not evidence of a report. Interrupt/cancellation remains native.
	_ = json.NewEncoder(stdout).Encode(map[string]string{"decision": "block", "reason": reason})
}
