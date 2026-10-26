package cmd

import (
	"fmt"
	"os"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/orca"
)

// batchCloseSettled closes the agent terminal of every batch member whose
// Dispatch has settled. A worker that reported done is never messaged again
// (follow-ups go to a fresh session), and an idle agent holds memory and burns
// tokens on auto-compaction, so only `terminal close` ends it.
//
// Scope is the batch itself: a terminal is closed only when this Dispatch's
// launch created it, and never when it is the Foreman's own, the caller's, or
// still carries another open Dispatch. Best effort: any error is logged and
// never blocks the wake; the land-time close is the backstop.
func batchCloseSettled(client *orca.Client, r foreman.Record, b *foremanBatch, o watchOpts) []string {
	if o.keepSettled {
		return nil
	}
	var protected map[string]bool
	var lines []string
	for i := range b.Tasks {
		m := &b.Tasks[i]
		if m.Closed || !dispatchTerminal(m.Status) || m.Dispatch == "" {
			continue
		}
		t, err := foreman.LoadTask(m.Task)
		if err != nil {
			lines = append(lines, fmt.Sprintf("CLOSE-WORKER-SKIPPED %s — %v", m.Task, err))
			continue
		}
		if protected == nil {
			protected = map[string]bool{"": true, os.Getenv("ORCA_TERMINAL_HANDLE"): true}
			if h, err := client.Ref(foremanTerm(client, r)); err == nil {
				protected[h] = true
			}
			for _, other := range foreman.ListTasks("") {
				if d := other.Current(); d != nil && !foreman.DispatchSettled(d.Status) {
					protected[d.Terminal] = true
				}
			}
		}
		d := t.DispatchByID(m.Dispatch)
		if d == nil || !d.OwnsTerminal || protected[d.Terminal] {
			m.Closed = true
			continue
		}
		// A failed close stays unmarked so the next tick retries it.
		if err := client.CloseHandle(d.Terminal); err != nil {
			lines = append(lines, fmt.Sprintf("CLOSE-WORKER-FAILED %s — %v", m.Task, err))
		} else {
			m.Closed = true
			lines = append(lines, fmt.Sprintf("CLOSED-WORKER %s (%s)", m.Task, d.Terminal))
		}
	}
	return lines
}
