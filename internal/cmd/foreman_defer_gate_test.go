package cmd

import (
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
func TestSealDefersTicketGatesToPhaseReview(t *testing.T) {
	st, child, _ := executionFixture(t)
	setTicketSize(t, child, "L")
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), strings.Repeat("line\n", 400))

	if err := projectSealChild(child, nil, "", false); err != nil {
		t.Fatal(err)
	}
	w := readSeal(t, child).Waiver
	if w == nil || !w.PhaseReview || !slices.Equal(w.Gates, []string{"qa", "review-pr"}) || !strings.Contains(w.Reason, "deferred to phase review") {
		t.Fatalf("waiver = %+v", w)
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
func TestSealKeepsReviewOfAFloorPath(t *testing.T) {
	_, child, _ := executionFixture(t)
	mustMkdirAll(t, filepath.Join(childWorktree(t, child), "auth"))
	mustWrite(t, filepath.Join(childWorktree(t, child), "auth", "token.go"), "package auth\n")
	if err := projectSealChild(child, nil, "", false); err == nil || !strings.Contains(err.Error(), "review-pr:stale") {
		t.Fatalf("seal = %v, want stale review-pr refusal", err)
	}
	gates, _, ok := deferralCovers(childWorktree(t, child), child.Home(), ticket.ReadDoc(child.IndexPath()), []string{"qa:stale_tree_digest"})
	if !ok || !slices.Equal(gates, []string{"qa"}) {
		t.Fatalf("floor deferral = %v %v", gates, ok)
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
		gates, _, ok := deferralCovers(childWorktree(t, child), child.Home(), ticket.ReadDoc(child.IndexPath()), []string{"review-pr:stale_tree_digest", "qa:stale_tree_digest"})
		if !ok || !slices.Equal(gates, []string{"review-pr", "qa"}) {
			t.Fatalf("size %q deferral = %v %v", size, gates, ok)
		}
		if _, _, ok := deferralCovers(childWorktree(t, child), child.Home(), ticket.ReadDoc(child.IndexPath()), []string{"qa:result_not_pass"}); ok {
			t.Fatal("a failed QA verdict was deferred")
		}
	}
}

// A phase fixup after land is made in the owning ticket's worktree and
// resealed; the next build is one fix commit on the landed phase, and needs a
// fresh review of that delta — the old review is not carried.
func TestSealPhaseFixup(t *testing.T) {
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
	if err := foremanProjectCommand("seal", []string{"ready", "--fixup", "AC1 copy"}); err == nil || !strings.Contains(err.Error(), "still owns") {
		t.Fatalf("fixup under a live dispatch = %v", err)
	}
	if _, err := foreman.UpdateTask(task.ID, func(tk *foreman.Task) error { return tk.Current().Settle(foreman.DispatchCompleted, "") }); err != nil {
		t.Fatal(err)
	}
	if err := foremanProjectCommand("seal", []string{"ready", "--fixup", "AC1 copy", "--waive", "qa"}); err == nil {
		t.Fatal("--fixup with --waive accepted")
	}
	if err := foremanProjectCommand("seal", []string{"ready", "--fixup", "AC1 copy"}); err != nil {
		t.Fatal(err)
	}
	if w := readSeal(t, child).Waiver; w == nil || !w.PhaseReview || w.Reason != "phase fixup: AC1 copy" {
		t.Fatalf("fixup waiver = %+v", w)
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
	if !strings.Contains(out, "p1/review (a review covers one phase diff; review the fix)") {
		t.Fatalf("review carried: %s", out)
	}
	if r := phaseQAProblems(t, st, "p1"); !hasReason(r, "phase review") {
		t.Fatalf("fixup passed QA without reviewing its delta: %v", r)
	}
}

// A small ticket that never ran review-pr or qa seals, and stays sealed.
func TestSealDefersGatesThatNeverRan(t *testing.T) {
	st, child, _ := executionFixture(t)
	setTicketSize(t, child, "XS")
	path := filepath.Join(child.Home(), "checkpoint.json")
	var cp map[string]interface{}
	if err := readProjectJSON(path, &cp); err != nil {
		t.Fatal(err)
	}
	delete(cp, "accepted_evidence")
	if err := writeJSONAtomic(path, cp); err != nil {
		t.Fatal(err)
	}
	if err := projectSealChild(child, nil, "", false); err != nil {
		t.Fatal(err)
	}
	if w := readSeal(t, child).Waiver; w == nil || w.States["review-pr"] != "missing" || w.States["qa"] != "missing" {
		t.Fatalf("waiver = %+v", w)
	}
	s, err := projectRead(st)
	if err != nil {
		t.Fatal(err)
	}
	if p := s.Children[0].Problems; len(p) != 0 {
		t.Fatalf("deferred child has problems: %v", p)
	}
}

// await judges a prerequisite the way task done and seal do: gates a ticket
// leaves to its phase do not stop its dependents.
func TestAwaitReadsASizeDeferralAsReady(t *testing.T) {
	_, child, repo := executionFixture(t)
	setTicketSize(t, child, "S")
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "small change\n")
	state, reasons := ticketReportState(repo, child.Env, child.Env.Ticket, false, []string{"review-pr:stale_tree_digest", "qa:stale_tree_digest"})
	if state != "ready" || len(reasons) != 1 || !strings.Contains(reasons[0], "deferred to phase review") {
		t.Fatalf("state = %q %v", state, reasons)
	}
	if state, _ := ticketReportState(repo, child.Env, child.Env.Ticket, false, []string{"qa:result_not_pass"}); state != "" {
		t.Fatalf("a failed QA verdict was read as ready: %q", state)
	}
}
