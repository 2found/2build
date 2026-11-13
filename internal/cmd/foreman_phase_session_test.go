package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/foreman"
)

// openPhaseTask creates a phase session Task on the fixture's ticket with an
// open Dispatch, as task launch leaves it.
func openPhaseTask(t *testing.T, kind string) string {
	t.Helper()
	task, err := foreman.CreateTask("fm-test", "ready", kind, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := foreman.UpdateTask(task.ID, func(tk *foreman.Task) error {
		tk.Dispatches = append(tk.Dispatches, foreman.Dispatch{ID: task.ID + ".d1", Status: foreman.DispatchOpen})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func stubTicketGate(t *testing.T, state string) {
	t.Helper()
	old := taskDoneGate
	taskDoneGate = func(string) (string, []string, error) { return state, nil, nil }
	t.Cleanup(func() { taskDoneGate = old })
	stubCompose(t, nil)
}

// A phase session records its build without settling, integrates and lands
// its own phase, and reports done only once that phase is verified; while it
// works, team next leaves the phase to it.
func TestPhaseSessionBuildsAndVerifiesItsOwnPhase(t *testing.T) {
	parent, child, _ := executionFixture(t)
	if err := os.Remove(filepath.Join(child.Home(), "project-seal.json")); err != nil {
		t.Fatal(err)
	}
	stubTicketGate(t, "deferred to phase review: review-pr, qa")
	task := openPhaseTask(t, phaseTaskKind)

	s, err := projectRead(parent)
	if err != nil {
		t.Fatal(err)
	}
	if steps := projectNext(parent, s); len(steps) == 0 || steps[0].Do != "wait" || steps[0].Target != task {
		t.Fatalf("next did not leave the phase to its session: %+v", steps)
	}

	var doneErr error
	out := captureStdout(t, func() {
		doneErr = foremanTask([]string{"done", task, "--outcome", "succeeded", "--summary", "s"})
	})
	if doneErr == nil || !strings.Contains(out, "GATE=not-ready") {
		t.Fatalf("done before the build was recorded: %v %q", doneErr, out)
	}

	mustWrite(t, filepath.Join(childWorktree(t, child), "feature.txt"), "phase work\n")
	out = captureStdout(t, func() { err = foremanTask([]string{"built", task, "--dispatch", task + ".d1"}) })
	if err != nil || !strings.Contains(out, "BUILT=ready") || !strings.Contains(out, "bbs team integrate parent --phase p1") {
		t.Fatalf("built: %v %q", err, out)
	}
	seal := readSeal(t, child)
	if got := gitOutIn(seal.Repos[0].Canonical, "show", seal.Repos[0].Head+":feature.txt"); got != "phase work" {
		t.Fatalf("recorded work = %q", got)
	}
	if tk, _ := foreman.LoadTask(task); tk.Current().Status != foreman.DispatchOpen || !strings.HasPrefix(tk.Current().Gate, "deferred") {
		t.Fatalf("built settled the session or lost its gate: %+v", tk.Current())
	}

	integrateAndLand(t, parent, "p1")
	out = captureStdout(t, func() {
		doneErr = foremanTask([]string{"done", task, "--outcome", "succeeded", "--summary", "s"})
	})
	if doneErr == nil || !strings.Contains(out, "phase p1 is landed") {
		t.Fatalf("done on a landed, unverified phase: %v %q", doneErr, out)
	}

	for _, kind := range []string{"integration", "product"} {
		finishExecutionEvidence(t, parent, executionEvidence(t, parent, kind), kind, false)
	}
	captureStdout(t, func() {
		doneErr = foremanTask([]string{"done", task, "--outcome", "succeeded", "--summary", "s"})
	})
	if doneErr != nil {
		t.Fatal(doneErr)
	}
	if tk, _ := foreman.LoadTask(task); tk.Current().Gate != "phase-verified: p1 delivered" {
		t.Fatalf("gate = %q", tk.Current().Gate)
	}
	if openPhaseSession(projectPhaseState{Seeds: []string{"core"}, Children: []string{"ready"}}) != "" {
		t.Fatal("a settled phase session still holds its phase")
	}
}

// Only a phase session records its build early; a ticket session's done
// records it.
func TestTaskBuiltIsForPhaseSessionsOnly(t *testing.T) {
	executionFixture(t)
	stubTicketGate(t, "ready")
	task := openPhaseTask(t, "ticket")
	var err error
	captureStdout(t, func() { err = foremanTask([]string{"built", task}) })
	if err == nil || !strings.Contains(err.Error(), "only a phase session") {
		t.Fatalf("built on a ticket session: %v", err)
	}
}

// A phase session's build still passes the ticket gate: a failed verdict is
// fixed in the warm session before the phase composes.
func TestTaskBuiltRefusesAnUnreadyTicket(t *testing.T) {
	_, child, _ := executionFixture(t)
	if err := os.Remove(filepath.Join(child.Home(), "project-seal.json")); err != nil {
		t.Fatal(err)
	}
	old := taskDoneGate
	taskDoneGate = func(string) (string, []string, error) { return "", []string{"qa:result_not_pass"}, nil }
	t.Cleanup(func() { taskDoneGate = old })
	stubCompose(t, nil)
	task := openPhaseTask(t, phaseTaskKind)
	var err error
	out := captureStdout(t, func() { err = foremanTask([]string{"built", task}) })
	if err == nil || !strings.Contains(out, "REASONS=qa:result_not_pass") {
		t.Fatalf("unready build recorded: %v %q", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(child.Home(), "project-seal.json")); statErr == nil {
		t.Fatal("a refused build recorded the ticket's work")
	}
}

// An auto phase's session releases it too, so next keeps waiting on the open
// session after the phase is delivered; any other release is the lead's.
func TestNextWaitsWhileAPhaseSessionReleases(t *testing.T) {
	parent, _, _ := executionFixture(t)
	task := openPhaseTask(t, phaseTaskKind)
	s := projectSnapshot{Ticket: "parent"}
	p := projectPhaseState{ID: "p1", Ticket: "ph-p1", Release: "auto", Seeds: []string{"core"}, Children: []string{"ready"}, Delivered: true, Integrated: true}
	if steps := phaseNext(parent, s, p); len(steps) != 1 || steps[0].Do != "wait" || steps[0].Target != task {
		t.Fatalf("next did not leave the release to the open session: %+v", steps)
	}
	if _, err := foreman.UpdateTask(task, func(tk *foreman.Task) error {
		tk.Current().Status = foreman.DispatchCompleted
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if steps := phaseNext(parent, s, p); len(steps) != 1 || steps[0].Do != "release" {
		t.Fatalf("next after the session settled: %+v", steps)
	}
}

// A phase session is for a phase of one ticket: on a phase of several it is
// refused at create and never holds the phase.
func TestPhaseSessionNeedsAOneTicketPhase(t *testing.T) {
	parent, _, _ := phasedFixture(t, []projectPhase{{ID: "p1", Title: "Tasks", Seeds: []string{"core", "extra"}, Release: "none"}}, "extra")
	if err := foremanProjectCommand("bind", []string{"parent", "--phase", "p1", "--child", "ph-p1"}); err != nil {
		t.Fatal(err)
	}
	if err := phaseSessionFits("ready"); err == nil || !strings.Contains(err.Error(), "phase p1 has 2 tickets") {
		t.Fatalf("phase session on a two-ticket phase: %v", err)
	}
	openPhaseTask(t, phaseTaskKind)
	s, err := projectRead(parent)
	if err != nil {
		t.Fatal(err)
	}
	if got := openPhaseSession(s.Phases[0]); got != "" {
		t.Fatalf("a phase session holds a two-ticket phase: %s", got)
	}
}

// A phase that cannot be read is not a verified phase: done is refused.
func TestPhaseDoneRefusesAnUnreadablePhase(t *testing.T) {
	executionFixture(t)
	old := taskDonePhaseGate
	taskDonePhaseGate = func(string) (string, []string, error) { return "", nil, os.ErrNotExist }
	t.Cleanup(func() { taskDonePhaseGate = old })
	task := openPhaseTask(t, phaseTaskKind)
	var err error
	captureStdout(t, func() { err = foremanTask([]string{"done", task, "--outcome", "succeeded", "--summary", "s"}) })
	if err == nil || !strings.Contains(err.Error(), "cannot be read") {
		t.Fatalf("done on an unreadable phase: %v", err)
	}
	if tk, _ := foreman.LoadTask(task); tk.Current().Status != foreman.DispatchOpen {
		t.Fatalf("a refused done settled the session: %+v", tk.Current())
	}
}
