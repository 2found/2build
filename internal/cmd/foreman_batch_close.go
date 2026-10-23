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
// tokens on auto-compaction. Orca keeps such a terminal open (user_owned,
// retained), so only `terminal close` ends it.
//
// Scope is the batch itself: a terminal is closed only when the Run's
// worker-list names it as the agent terminal of a settled member Dispatch, and
// never when it is the Foreman's own, the caller's, or shared with a Dispatch
// that is still open. Best effort: any error is logged and never blocks the
// wake; the land-time close is the backstop.
func batchCloseSettled(client *orca.Client, r foreman.Record, b *foremanBatch, o watchOpts) []string {
	if o.keepSettled {
		return nil
	}
	lists := map[string][]orca.Worker{}
	var protected map[string]bool
	var lines []string
	for i := range b.Tasks {
		m := &b.Tasks[i]
		if m.Closed || !dispatchTerminal(m.Status) || m.Run == "" || m.Dispatch == "" {
			continue
		}
		list, ok := lists[m.Run]
		if !ok {
			var err error
			if list, err = client.Workers(m.Run); err != nil {
				lines = append(lines, fmt.Sprintf("CLOSE-WORKER-SKIPPED %s — %v", m.Task, err))
				continue
			}
			lists[m.Run] = list
		}
		if protected == nil {
			protected = map[string]bool{"": true, os.Getenv("ORCA_TERMINAL_HANDLE"): true}
			if h, err := client.Ref(foremanTerm(client, r)); err == nil {
				protected[h] = true
			}
		}
		for _, w := range list {
			if !dispatchTerminal(w.Status) {
				protected[w.Terminal] = true
			}
		}
		for _, w := range list {
			if w.Dispatch != m.Dispatch {
				continue
			}
			m.Closed = true
			if protected[w.Terminal] {
				break
			}
			if err := client.CloseHandle(w.Terminal); err != nil {
				lines = append(lines, fmt.Sprintf("CLOSE-WORKER-FAILED %s — %v", m.Task, err))
			} else {
				lines = append(lines, fmt.Sprintf("CLOSED-WORKER %s (%s)", m.Task, w.Terminal))
			}
			break
		}
	}
	return lines
}
