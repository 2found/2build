package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/ticket"
)

// Measured runs (2026-10) spent half of a phase's tokens on per-ticket
// review-pr and QA that phase QA then repeated on the merged commit, and the
// phase still needed repairs. A Foreman ticket therefore runs focused checks
// only and leaves review-pr and qa to its phase, whatever its size: the phase
// commit is reviewed once (a `review` check in its integration evidence) and
// QA'd cumulatively. A change that hits the money/auth/irreversible-data
// floor still reviews its own diff; only its qa waits for the phase.

// ticketGateDeferral returns the gates a Foreman child leaves to phase review
// and why. A change whose paths hit the floor keeps its own review-pr.
func ticketGateDeferral(dir, home string, doc ticket.Doc) ([]string, string) {
	if doc.Get("parent") == "" {
		return nil, ""
	}
	tree, err := worktreeTree(dir)
	if err != nil {
		return nil, ""
	}
	adopted := readAdopted(home)
	var own []string
	for _, path := range nameOnlyDiff(dir, "HEAD", tree) {
		if !adoptedVerbatim(dir, tree, path, adopted) {
			own = append(own, path)
		}
	}
	if len(own) == 0 {
		return nil, ""
	}
	if hits := floorHits(strings.Join(own, "\n")); len(hits) > 0 {
		return []string{"qa"}, fmt.Sprintf("deferred to phase review: qa (%d files; %s keeps its own review)", len(own), strings.Join(hits, ", "))
	}
	return []string{"review-pr", "qa"}, fmt.Sprintf("deferred to phase review: review-pr, qa (%d files)", len(own))
}

// adoptedVerbatim: path's content in tree came unchanged from an adopted
// snapshot, so it is a sibling's work, not this ticket's.
func adoptedVerbatim(dir, tree, path string, adopted []string) bool {
	want := gitOutIn(dir, "rev-parse", "--verify", "-q", tree+":"+path)
	if want == "" {
		return false
	}
	return slices.ContainsFunc(adopted, func(snap string) bool {
		return gitOutIn(dir, "rev-parse", "--verify", "-q", snap+":"+path) == want
	})
}

// deferralCovers: every readiness code is a missing or stale gate this
// ticket defers to its phase. A failed verdict or open finding still stops it.
func deferralCovers(dir, home string, doc ticket.Doc, codes []string) ([]string, string, bool) {
	gates, why := ticketGateDeferral(dir, home, doc)
	if len(gates) == 0 {
		return nil, "", false
	}
	rest, _ := sealWaivable(codes, gates)
	return gates, why, len(rest) == 0
}

// openTicketDispatch is the id of a gated Dispatch still running for t, or "".
func openTicketDispatch(t string) string {
	for _, task := range foreman.ListTasks("") {
		if d := task.Current(); task.Ticket == t && gatedTaskKinds[task.Kind] && d != nil && !foreman.DispatchSettled(d.Status) {
			return d.ID
		}
	}
	return ""
}
