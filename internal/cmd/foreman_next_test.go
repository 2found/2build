package cmd

import (
	"strings"
	"testing"

	"github.com/2found/2build/internal/ticket"
)

func nextSteps(t *testing.T, st *ticket.Store) []foremanStep {
	t.Helper()
	s, err := projectRead(st)
	if err != nil {
		t.Fatal(err)
	}
	return projectNext(st, s)
}

func stepDos(steps []foremanStep) string {
	var out []string
	for _, s := range steps {
		out = append(out, s.Do+":"+s.Phase+":"+s.Target)
	}
	return strings.Join(out, " ")
}

func TestForemanNextBeforeApproval(t *testing.T) {
	st, _, _ := phasedFixture(t, []projectPhase{{ID: "p1", Title: "Tasks", Seeds: []string{"core"}, Release: "none"}})
	if got := stepDos(nextSteps(t, st)); got != "bind:p1:" {
		t.Fatalf("approved design: %s", got)
	}
	var c projectContract
	if err := readProjectJSON(projectContractPath(st), &c); err != nil {
		t.Fatal(err)
	}
	c.NonGoals = append(c.NonGoals, "offline mode")
	if err := writeJSONAtomic(projectContractPath(st), c); err != nil {
		t.Fatal(err)
	}
	if got := stepDos(nextSteps(t, st)); got != "publish::" {
		t.Fatalf("unapproved design: %s", got)
	}
	if _, err := approvalPublish(st, "project-plan", "review", "developer"); err != nil {
		t.Fatal(err)
	}
	if got := stepDos(nextSteps(t, st)); got != "wait::human" {
		t.Fatalf("design in review: %s", got)
	}
	if _, _, err := approvalResolve(st, "redirect", "split phase 1", "human"); err != nil {
		t.Fatal(err)
	}
	if got := stepDos(nextSteps(t, st)); got != "revise::" {
		t.Fatalf("redirected design: %s", got)
	}
}

// next walks the same path the verbs enforce, one suggestion at a time.
func TestForemanNextFollowsThePhaseFlow(t *testing.T) {
	st, _, repo := phasedFixture(t, []projectPhase{
		{ID: "p1", Title: "Walking skeleton", Seeds: []string{"core"}, Release: "ask"},
		{ID: "p2", Title: "Extra", Seeds: []string{"extra"}, Release: "auto"},
	}, "extra")
	expect := func(want string) []foremanStep {
		t.Helper()
		steps := nextSteps(t, st)
		if got := stepDos(steps); got != want {
			t.Fatalf("next = %s, want %s (%+v)", got, want, steps)
		}
		return steps
	}
	if steps := expect("bind:p1:"); !strings.Contains(steps[0].Command, "bind parent --phase p1") {
		t.Fatalf("bind command = %q", steps[0].Command)
	}
	for _, p := range []string{"p1", "p2"} {
		if err := foremanProjectCommand("bind", []string{"parent", "--phase", p, "--child", "ph-" + p}); err != nil {
			t.Fatal(err)
		}
	}
	expect("integrate:p1:")
	integratePhase(t, st, "p1")
	expect("verify:p1:")
	for _, kind := range []string{"integration", "product"} {
		finishExecutionEvidence(t, st, phaseEvidence(t, st, kind, "p1"), kind, false)
	}
	expect("land:p1:")
	if err := landPhase(t, st, "p1"); err != nil {
		t.Fatal(err)
	}
	// Phase 2 builds while phase 1 waits on the human's release go.
	expect("release:p1:human create:p2:extra")
	release := func(verdict string) {
		t.Helper()
		var err error
		captureStdout(t, func() { err = projectReleaseCommand(st, map[string]string{"phase": "p1", "request": "1"}) })
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := approvalResolve(storeForTicket(st.Env, "ph-p1"), "approve", "ship", "human"); err != nil {
			t.Fatal(err)
		}
		if err := recordRelease(t, st, repo, "p1", verdict); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	captureStdout(t, func() { err = projectReleaseCommand(st, map[string]string{"phase": "p1", "request": "1"}) })
	if err != nil {
		t.Fatal(err)
	}
	expect("wait:p1:human create:p2:extra")
	if _, _, err := approvalResolve(storeForTicket(st.Env, "ph-p1"), "approve", "ship", "human"); err != nil {
		t.Fatal(err)
	}
	expect("release:p1: create:p2:extra")
	if err := recordRelease(t, st, repo, "p1", "ROLLED_BACK"); err != nil {
		t.Fatal(err)
	}
	// A failed release stops the line: only phase 1's fix or retry is offered.
	if steps := expect("repair:p1: release:p1:human"); !strings.Contains(steps[0].Why, "stop the line") {
		t.Fatalf("failed release: %+v", steps)
	}
	release("RELEASED")
	expect("create:p2:extra")
}

// One read on a wake: the batch view carries each owned project's next steps.
func TestBatchShowCarriesNextSteps(t *testing.T) {
	phasedFixture(t, []projectPhase{{ID: "p1", Title: "Tasks", Seeds: []string{"core"}, Release: "none"}})
	out := captureStdout(t, func() { printForemanNext("fm-test") })
	if !strings.Contains(out, "NEXT=parent bind phase=p1") || !strings.Contains(out, "$ bbs foreman bind parent --phase p1") {
		t.Fatalf("next lines = %q", out)
	}
	if out := captureStdout(t, func() { printForemanNext("fm-other") }); out != "" {
		t.Fatalf("another Foreman's project leaked: %q", out)
	}
}

// Sealing a settled batch is one call; a ticket that is not ready does not
// hold back the others.
func TestSealManyReportsEachTicket(t *testing.T) {
	executionFixture(t)
	var err error
	out := captureStdout(t, func() { err = foremanSealMany([]string{"ready", "missing-ticket"}) })
	if err == nil || !strings.Contains(out, "SEALED=ready") || !strings.Contains(out, "NOT_SEALED=missing-ticket") {
		t.Fatalf("seal many = %v %q", err, out)
	}
}
