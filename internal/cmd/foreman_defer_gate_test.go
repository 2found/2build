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

// A small ticket seals without its own review and QA; the phase then needs a
// review check on its commit before it counts as integrated.
func TestSealDefersASmallTicketsGatesToPhaseReview(t *testing.T) {
	st, child, _ := executionFixture(t)
	setTicketSize(t, child, "S")
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "small change\n")

	if err := projectSealChild(child, nil, "", false); err != nil {
		t.Fatal(err)
	}
	w := readSeal(t, child).Waiver
	if w == nil || !w.PhaseReview || !slices.Equal(w.Gates, []string{"qa", "review-pr"}) || !strings.Contains(w.Reason, "size S") {
		t.Fatalf("waiver = %+v", w)
	}
	integratePhase(t, st, "p1")
	recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 0})
	recordChecks(t, st, "product", carryCheck{"AC1", "product", 0})
	if r := phaseReadiness(t, st, "land", "p1"); !hasReason(r, "phase review") {
		t.Fatalf("phase without a review check is landable: %v", r)
	}
	recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 0}, carryCheck{"p1", "review", 0})
	if r := phaseReadiness(t, st, "land", "p1"); len(r) != 0 {
		t.Fatalf("reviewed phase not landable: %v", r)
	}
}

func TestSealDefersNothingItShouldNot(t *testing.T) {
	cases := []struct {
		name, size, path, body string
	}{
		{"M keeps review", "M", "feature.txt", "change\n"},
		{"unset size is M", "", "feature.txt", "change\n"},
		{"diff outgrows its size", "S", "feature.txt", strings.Repeat("line\n", 400)},
		{"auth path", "S", "auth/token.go", "package auth\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, child, _ := executionFixture(t)
			if c.size != "" {
				setTicketSize(t, child, c.size)
			}
			mustMkdirAll(t, filepath.Dir(filepath.Join(childWorktree(t, child), c.path)))
			mustWrite(t, filepath.Join(childWorktree(t, child), c.path), c.body)
			err := projectSealChild(child, nil, "", false)
			if err == nil || !strings.Contains(err.Error(), "review-pr:stale") {
				t.Fatalf("seal = %v, want stale review-pr refusal", err)
			}
		})
	}
}

// An M ticket defers QA only: with review current, a stale QA seals.
func TestTaskDoneGateDefersQAOfAnMTicket(t *testing.T) {
	_, child, _ := executionFixture(t)
	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "change\n")
	gates, _, ok := deferralCovers(childWorktree(t, child), child.Home(), ticket.ReadDoc(child.IndexPath()), []string{"qa:stale_tree_digest"})
	if !ok || !slices.Equal(gates, []string{"qa"}) {
		t.Fatalf("M deferral = %v %v", gates, ok)
	}
	if _, _, ok := deferralCovers(childWorktree(t, child), child.Home(), ticket.ReadDoc(child.IndexPath()), []string{"qa:result_not_pass"}); ok {
		t.Fatal("a failed QA verdict was deferred")
	}
}

// A phase fixup lands in the owning ticket's worktree, reseals it and makes
// the next phase commit need a fresh review; a review is never carried.
func TestSealPhaseFixup(t *testing.T) {
	st, child, _ := executionFixture(t)
	integratePhase(t, st, "p1")
	oldInt := recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 0}, carryCheck{"p1", "review", 0})
	oldProd := recordChecks(t, st, "product", carryCheck{"AC1", "product", 0})

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
	integratePhase(t, st, "p1")
	out, err := carry(t, st, map[string]string{"from": oldInt + "," + oldProd, "reason": "fixup touched copy only"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "p1/review (a review is never carried)") {
		t.Fatalf("review carried: %s", out)
	}
	if r := phaseReadiness(t, st, "land", "p1"); !hasReason(r, "phase review") {
		t.Fatalf("fixup landable without reviewing its delta: %v", r)
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
