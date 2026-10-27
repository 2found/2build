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
// phase still needed repairs. A small Foreman ticket therefore defers its own
// gates to the phase: XS/S defer review-pr and qa, M defers qa. The phase
// commit is reviewed once (a `review` check in its integration evidence) and
// QA'd cumulatively, as before.

var sizeRank = map[string]int{"XS": 0, "S": 1, "M": 2, "L": 3}

// deferredGates are the gates a ticket of this size leaves to its phase.
func deferredGates(size string) []string {
	switch size {
	case "XS", "S":
		return []string{"review-pr", "qa"}
	case "M":
		return []string{"qa"}
	}
	return nil
}

// ticketGateDeferral returns the gates a Foreman child may leave to phase
// review and why. Size is the larger of the plan's ticket_size (M when unset)
// and the rubric applied to the worktree's own changes, so a plan cannot size
// itself out of review. A change whose paths hit the money/auth/
// irreversible-data floor defers nothing.
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
	loc := parseShortstat(gitOutIn(dir, append([]string{"diff", "--shortstat", "HEAD", tree, "--"}, own...)...))
	measured, _, _, _, _ := classifySize(own, loc)
	size := strings.ToUpper(strings.TrimSpace(doc.Get("pointers.ticket_size")))
	if _, ok := sizeRank[size]; !ok {
		size = "M"
	}
	if sizeRank[measured] > sizeRank[size] {
		size = measured
	}
	if hits := floorHits(strings.Join(own, "\n")); len(hits) > 0 {
		return nil, ""
	}
	gates := deferredGates(size)
	if len(gates) == 0 {
		return nil, ""
	}
	return gates, fmt.Sprintf("deferred to phase review: size %s (%d files, %d lines)", size, len(own), loc)
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
