package cmd

import (
	"fmt"
	"regexp"
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
// floor, or privacy/tenant paths, still reviews its own diff; only its qa waits for the phase.

// privacyPattern widens the review floor for a ticket's own paths only (plan
// approval keeps floorHits): a p2 phase QA (2026-11) found a paired person's
// memory surviving erasure and an eraser crossing tenants, in files the money/
// auth/data words do not name.
var privacyPattern = regexp.MustCompile(`(?i)(privacy|tenant|erasure|eraser|retention|isolation)`)

func reviewFloorHits(paths string) []string {
	hits := floorHits(paths)
	for _, m := range privacyPattern.FindAllString(paths, -1) {
		if k := strings.ToLower(m); !slices.Contains(hits, k) {
			hits = append(hits, k)
		}
	}
	return hits
}

// ticketGateDeferral returns the gates a Foreman child leaves to phase review
// and why. A change whose paths hit the floor keeps its own review-pr.
func ticketGateDeferral(dir string, doc ticket.Doc) ([]string, string) {
	if doc.Get("parent") == "" {
		return nil, ""
	}
	tree, err := worktreeTree(dir)
	if err != nil {
		return nil, ""
	}
	own := nameOnlyDiff(dir, "HEAD", tree)
	if len(own) == 0 {
		return nil, ""
	}
	if hits := reviewFloorHits(strings.Join(own, "\n")); len(hits) > 0 {
		return []string{"qa"}, fmt.Sprintf("deferred to phase review: qa (%d files; %s keeps its own review)", len(own), strings.Join(hits, ", "))
	}
	return []string{"review-pr", "qa"}, fmt.Sprintf("deferred to phase review: review-pr, qa (%d files)", len(own))
}

// deferralCovers: every readiness code is a missing gate this
// ticket defers to its phase. A failed verdict or open finding still stops it.
func deferralCovers(dir string, doc ticket.Doc, codes []string) ([]string, string, bool) {
	gates, why := ticketGateDeferral(dir, doc)
	if len(gates) == 0 {
		return nil, "", false
	}
	for _, code := range codes {
		gate, state, _ := strings.Cut(code, ":")
		if state != "missing" || !slices.Contains(gates, gate) {
			return gates, why, false
		}
	}
	return gates, why, true
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
