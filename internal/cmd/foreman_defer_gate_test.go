package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/ticket"
)

func setTicketSize(t *testing.T, st *ticket.Store, size string) {
	t.Helper()
	doc := ticket.ReadDoc(st.IndexPath())
	doc.Set("pointers.ticket_size", size)
	if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
		t.Fatal(err)
	}
}

func childWorktree(t *testing.T, st *ticket.Store) string {
	t.Helper()
	m, err := ticket.ReadManifest(st.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	return m.Repos[0].Worktree
}

// dropGates leaves a ticket that never ran review-pr or qa, as a ticket off
// the floor does.
func dropGates(t *testing.T, st *ticket.Store) {
	t.Helper()
	path := filepath.Join(st.Home(), "checkpoint.json")
	var cp map[string]interface{}
	if err := readProjectJSON(path, &cp); err != nil {
		t.Fatal(err)
	}
	delete(cp, "accepted_evidence")
	if err := writeJSONAtomic(path, cp); err != nil {
		t.Fatal(err)
	}
}

func readSeal(t *testing.T, st *ticket.Store) projectSeal {
	t.Helper()
	var seal projectSeal
	if err := readProjectJSON(filepath.Join(st.Home(), "project-seal.json"), &seal); err != nil {
		t.Fatal(err)
	}
	return seal
}

// A ticket of any size seals without its own review and QA; the phase then
// needs a review check on its commit before it counts as integrated.
func TestPhaseReviewsItsDiffOnce(t *testing.T) {
	st, child, _ := executionFixture(t)
	setTicketSize(t, child, "L")
	dropGates(t, child)
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), strings.Repeat("line\n", 400))

	if err := projectRecordChild(child); err != nil {
		t.Fatal(err)
	}
	if s, err := projectRead(st); err != nil || len(s.Children[0].Problems) != 0 {
		t.Fatalf("recorded work without gates has problems: %+v %v", s.Children, err)
	}
	integrateAndLand(t, st, "p1")
	recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 0})
	recordChecks(t, st, "product", carryCheck{"AC1", "product", 0})
	if r := phaseQAProblems(t, st, "p1"); !hasReason(r, "phase review") {
		t.Fatalf("phase without a review check passed QA: %v", r)
	}
	recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 0}, carryCheck{"p1", "review", 0})
	if r := phaseQAProblems(t, st, "p1"); len(r) != 0 {
		t.Fatalf("reviewed phase did not pass QA: %v", r)
	}
}

// A money/auth/irreversible-data change keeps its own review.
func TestFloorPathKeepsItsReview(t *testing.T) {
	_, child, _ := executionFixture(t)
	mustMkdirAll(t, filepath.Join(childWorktree(t, child), "auth"))
	mustWrite(t, filepath.Join(childWorktree(t, child), "auth", "token.go"), "package auth\n")
	dropGates(t, child)
	if _, _, ok := deferralCovers(childWorktree(t, child), ticket.ReadDoc(child.IndexPath()), []string{"review-pr:missing", "qa:missing"}); ok {
		t.Fatal("a floor path deferred its own review")
	}
	gates, _, ok := deferralCovers(childWorktree(t, child), ticket.ReadDoc(child.IndexPath()), []string{"qa:missing"})
	if !ok || !slices.Equal(gates, []string{"qa"}) {
		t.Fatalf("floor deferral = %v %v", gates, ok)
	}
}

// A privacy or tenant path keeps its own review too.
func TestPrivacyPathKeepsItsReview(t *testing.T) {
	_, child, _ := executionFixture(t)
	mustMkdirAll(t, filepath.Join(childWorktree(t, child), "memory"))
	mustWrite(t, filepath.Join(childWorktree(t, child), "memory", "eraser.go"), "package memory\n")
	dropGates(t, child)
	if _, _, ok := deferralCovers(childWorktree(t, child), ticket.ReadDoc(child.IndexPath()), []string{"review-pr:missing", "qa:missing"}); ok {
		t.Fatal("a privacy path deferred its own review")
	}
}

// Every size defers both gates; a failed verdict is never deferred.
func TestTaskDoneGateDefersBothGatesAtEverySize(t *testing.T) {
	for _, size := range []string{"", "XS", "S", "M", "L"} {
		_, child, _ := executionFixture(t)
		if size != "" {
			setTicketSize(t, child, size)
		}
		mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "change\n")
		gates, _, ok := deferralCovers(childWorktree(t, child), ticket.ReadDoc(child.IndexPath()), []string{"review-pr:missing", "qa:missing"})
		if !ok || !slices.Equal(gates, []string{"review-pr", "qa"}) {
			t.Fatalf("size %q deferral = %v %v", size, gates, ok)
		}
		if _, _, ok := deferralCovers(childWorktree(t, child), ticket.ReadDoc(child.IndexPath()), []string{"qa:result_not_pass"}); ok {
			t.Fatal("a failed QA verdict was deferred")
		}
	}
}

// A phase QA fix after land is made in the owning ticket's worktree; the
// next integrate takes it and the next build is one fix commit on the landed phase, and the
// phase review already recorded stands — the fix is never re-reviewed.
func TestPhaseQAFixLandsAsOneFixCommit(t *testing.T) {
	st, child, repo := executionFixture(t)
	first := integrateAndLand(t, st, "p1")
	oldInt := recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 0}, carryCheck{"p1", "review", 0})
	recordChecks(t, st, "product", carryCheck{"AC1", "product", 1})

	task, err := foreman.CreateTask("fm-test", "ready", "repair", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := foreman.UpdateTask(task.ID, func(tk *foreman.Task) error {
		tk.Dispatches = append(tk.Dispatches, foreman.Dispatch{ID: task.ID + ".d1", Status: foreman.DispatchOpen})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "fixed copy\n")
	s, err := projectRead(st)
	if err != nil {
		t.Fatal(err)
	}
	if ids := projectUnsealedEdits(st, s, s.Phases[0]); len(ids) != 0 {
		t.Fatalf("edits under a live dispatch taken: %v", ids)
	}
	if _, err := foreman.UpdateTask(task.ID, func(tk *foreman.Task) error { return tk.Current().Settle(foreman.DispatchCompleted, "") }); err != nil {
		t.Fatal(err)
	}
	// Other work lands on base meanwhile; the fix still applies as a delta.
	other := filepath.Join(t.TempDir(), "other")
	runGit(t, repo, "worktree", "add", "-q", other, "main")
	mustWrite(t, filepath.Join(other, "hotfix.txt"), "hotfix\n")
	runGit(t, other, "add", ".")
	runGit(t, other, "commit", "-qm", "hotfix")
	hotfix := gitOutIn(repo, "rev-parse", "main")
	fix := integrateAndLand(t, st, "p1")
	if gitOutIn(repo, "rev-parse", "main^") != hotfix || !gitOKIn(repo, "merge-base", "--is-ancestor", first.Repos[0].Commit, "main") ||
		gitOutIn(repo, "show", "main:feature.txt") != "fixed copy" || gitOutIn(repo, "show", "main:hotfix.txt") != "hotfix" {
		t.Fatal("the fix did not land as one commit on top of the landed phase and the other work")
	}
	if fix.Repos[0].Worktree != first.Repos[0].Worktree {
		t.Fatalf("fix built in a new phase worktree: %s", fix.Repos[0].Worktree)
	}
	out, err := carry(t, st, map[string]string{"from": oldInt, "reason": "fixup touched copy only"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "p1/review (the phase review already stands)") {
		t.Fatalf("review carried: %s", out)
	}
	if r := phaseQAProblems(t, st, "p1"); hasReason(r, "phase review") {
		t.Fatalf("fixup demanded a fresh phase review: %v", r)
	}
}

// task done reads gates a ticket leaves to its phase as ready; a failed
// verdict still stops it.
func TestReportStateReadsASizeDeferralAsReady(t *testing.T) {
	_, child, repo := executionFixture(t)
	setTicketSize(t, child, "S")
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "small change\n")
	state, reasons := ticketReportState(repo, child.Env, child.Env.Ticket, false, []string{"review-pr:missing", "qa:missing"})
	if state != "ready" || len(reasons) != 1 || !strings.Contains(reasons[0], "deferred to phase review") {
		t.Fatalf("state = %q %v", state, reasons)
	}
	if state, _ := ticketReportState(repo, child.Env, child.Env.Ticket, false, []string{"qa:result_not_pass"}); state != "" {
		t.Fatalf("a failed QA verdict was read as ready: %q", state)
	}
}

// A succeeded task done records the ticket's work for its phase; nobody seals.
func TestTaskDoneRecordsTicketWork(t *testing.T) {
	st, child, _ := executionFixture(t)
	if err := os.Remove(filepath.Join(child.Home(), "project-seal.json")); err != nil {
		t.Fatal(err)
	}
	if s, _ := projectRead(st); len(s.Children[0].Problems) == 0 {
		t.Fatal("a ticket with no recorded work counted as built")
	}
	task, err := foreman.CreateTask("fm-test", "ready", "ticket", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := foreman.UpdateTask(task.ID, func(tk *foreman.Task) error {
		tk.Dispatches = append(tk.Dispatches, foreman.Dispatch{ID: task.ID + ".d1", Status: foreman.DispatchOpen})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "final work\n")
	if err := taskDoneRecordWork(task.ID, map[string]string{"outcome": "succeeded"}); err != nil {
		t.Fatal(err)
	}
	seal := readSeal(t, child)
	if got := gitOutIn(seal.Repos[0].Canonical, "show", seal.Repos[0].Head+":feature.txt"); got != "final work" {
		t.Fatalf("recorded work = %q", got)
	}
	if s, _ := projectRead(st); len(s.Children[0].Problems) != 0 {
		t.Fatalf("recorded ticket still has problems: %v", s.Children[0].Problems)
	}
}
