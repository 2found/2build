package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/ticket"
)

func TestProjectPhaseContractValidation(t *testing.T) {
	base := func() projectContract {
		return projectContract{Version: 1, Audience: "teams", Outcome: "ship", Seeds: []string{"core", "extra"},
			Criteria:     []projectCriterion{{ID: "AC1", Text: "core works", Owner: "core", Checks: []string{"behavior"}}, {ID: "AC2", Text: "extra works", Owner: "extra", Checks: []string{"behavior"}}},
			FirstJourney: []string{"AC1"},
			Phases:       []projectPhase{{ID: "p1", Title: "Skeleton", Seeds: []string{"core"}, Release: "auto"}, {ID: "p2", Title: "Extra", Seeds: []string{"extra"}, Release: "ask"}}}
	}
	if err := validateProjectContract(base()); err != nil {
		t.Fatalf("valid phased contract rejected: %v", err)
	}
	for name, mutate := range map[string]func(*projectContract){
		"unassigned seed":     func(c *projectContract) { c.Phases[1].Seeds = []string{"core"} },
		"seed in two phases":  func(c *projectContract) { c.Phases[1].Seeds = []string{"extra", "core"} },
		"bad release":         func(c *projectContract) { c.Phases[0].Release = "later" },
		"first journey later": func(c *projectContract) { c.FirstJourney = []string{"AC2"} },
		"duplicate phase id":  func(c *projectContract) { c.Phases[1].ID = "p1" },
		"phase without seeds": func(c *projectContract) {
			c.Phases = append(c.Phases, projectPhase{ID: "p3", Title: "x", Release: "none"})
		},
		"phase without a title": func(c *projectContract) { c.Phases[0].Title = " " },
	} {
		c := base()
		mutate(&c)
		if err := validateProjectContract(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// phasedFixture turns the execution fixture into a phased project: the
// sealed child "ready" (seed core) sits under phase ticket ph-p1.
func phasedFixture(t *testing.T, phases []projectPhase, extraSeeds ...string) (*ticket.Store, *ticket.Store, string) {
	t.Helper()
	parent, child, repo := projectFixture(t)
	var c projectContract
	if err := readProjectJSON(projectContractPath(parent), &c); err != nil {
		t.Fatal(err)
	}
	c.Seeds = append(c.Seeds, extraSeeds...)
	for _, seed := range extraSeeds {
		c.Criteria = append(c.Criteria, projectCriterion{ID: "AC-" + seed, Text: seed + " works", Owner: seed, Checks: []string{"behavior"}})
	}
	c.Phases = phases
	if err := validateProjectContract(c); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(projectContractPath(parent), c); err != nil {
		t.Fatal(err)
	}
	var phaseTickets []interface{}
	for _, p := range phases {
		id := "ph-" + p.ID
		phaseTickets = append(phaseTickets, id)
		doc := ticket.Doc{"id": id, "parent": "parent", "status": "decomposed", "children": []interface{}{}}
		if p.ID == phases[0].ID {
			doc["children"] = []interface{}{"ready"}
		}
		ps := storeForTicket(parent.Env, id)
		mustMkdirAll(t, ps.Home())
		if err := ticket.WriteDoc(ps.IndexPath(), doc); err != nil {
			t.Fatal(err)
		}
	}
	doc := ticket.ReadDoc(parent.IndexPath())
	doc["children"] = phaseTickets
	if err := ticket.WriteDoc(parent.IndexPath(), doc); err != nil {
		t.Fatal(err)
	}
	cd := ticket.ReadDoc(child.IndexPath())
	cd.Set("parent", "ph-"+phases[0].ID)
	if err := ticket.WriteDoc(child.IndexPath(), cd); err != nil {
		t.Fatal(err)
	}
	if _, err := approvalPublish(parent, "project-plan", "review phases", "developer"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := approvalResolve(parent, "approve", "accepted", "developer"); err != nil {
		t.Fatal(err)
	}
	return parent, child, repo
}

func phaseReadiness(t *testing.T, st *ticket.Store, action, phase string) []string {
	t.Helper()
	s, err := projectRead(st)
	if err != nil {
		t.Fatal(err)
	}
	return projectPhaseReasons(s, action, phase)
}

func phaseEvidence(t *testing.T, st *ticket.Store, kind, phase string) string {
	t.Helper()
	spec := filepath.Join(t.TempDir(), "spec.json")
	if err := writeJSONAtomic(spec, map[string]interface{}{"kind": kind, "producer": "fresh-evaluator", "phase": phase}); err != nil {
		t.Fatal(err)
	}
	var err error
	out := captureStdout(t, func() { err = projectEvidenceCommand(st, map[string]string{"begin": "1", "file": spec}) })
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data projectEvidence `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data.ID
}

func recordRelease(t *testing.T, st *ticket.Store, repo, phase, verdict string) error {
	t.Helper()
	record := filepath.Join(t.TempDir(), "release.md")
	mustWrite(t, record, "VERDICT: "+verdict+"\n")
	in := filepath.Join(t.TempDir(), "receipt.json")
	if err := writeJSONAtomic(in, map[string]interface{}{"verdict": verdict, "tag": "v1", "record": record, "repos": []projectReleaseRepo{{Dir: repo, SHA: gitOutIn(repo, "rev-parse", "main")}}}); err != nil {
		t.Fatal(err)
	}
	var err error
	captureStdout(t, func() { err = projectReleaseCommand(st, map[string]string{"phase": phase, "file": in}) })
	return err
}

func hasReason(reasons []string, needle string) bool {
	for _, r := range reasons {
		if strings.Contains(r, needle) {
			return true
		}
	}
	return false
}

func TestPhasedProjectReleasesPhaseByPhase(t *testing.T) {
	st, _, repo := phasedFixture(t, []projectPhase{
		{ID: "p1", Title: "Walking skeleton", Seeds: []string{"core"}, Release: "ask"},
		{ID: "p2", Title: "Extra", Seeds: []string{"extra"}, Release: "auto"},
	}, "extra")

	if r := phaseReadiness(t, st, "dispatch", "p1"); !hasReason(r, "bind a phase ticket") {
		t.Fatalf("dispatch without a phase ticket: %v", r)
	}
	bind := func(phase string) {
		t.Helper()
		if err := foremanProjectCommand("bind", []string{"parent", "--phase", phase, "--child", "ph-" + phase}); err != nil {
			t.Fatal(err)
		}
	}
	bind("p1")
	bind("p2")
	if err := foremanProjectCommand("bind", []string{"parent", "--phase", "p1", "--child", "ph-p2"}); err == nil {
		t.Fatal("a phase took a second ticket")
	}
	if r := phaseReadiness(t, st, "dispatch", "p1"); len(r) != 0 {
		t.Fatalf("phase 1 dispatch blocked: %v", r)
	}
	if r := phaseReadiness(t, st, "dispatch", "p2"); !hasReason(r, "phase p1: integrate and land it first") {
		t.Fatalf("phase 2 dispatched before phase 1 landed: %v", r)
	}
	if r := phaseReadiness(t, st, "integrate", "p2"); !hasReason(r, "phase p1: release it first") {
		t.Fatalf("phase 2 integration ahead of phase 1 release: %v", r)
	}

	// Phase 1 becomes one commit, lands on base at once, then is verified
	// cumulatively and reviewed as a product there.
	base := gitOutIn(repo, "rev-parse", "main")
	cand := integratePhase(t, st, "p1")
	if r := cand.Repos[0]; r.BaseHead != base || gitOutIn(repo, "rev-parse", r.Commit+"^") != base || gitOutIn(repo, "rev-parse", "main") != base {
		t.Fatalf("phase commit is not one commit on untouched base: %+v", r)
	}
	if got := gitOutIn(cand.Repos[0].Worktree, "show", "HEAD:feature.txt"); got != "working journey" {
		t.Fatalf("phase commit lacks the ticket's uncommitted work: %q", got)
	}
	if again := integratePhase(t, st, "p1"); again.Repos[0].Commit != cand.Repos[0].Commit {
		t.Fatal("re-integrating an unchanged phase built a different commit")
	}
	if r := phaseReadiness(t, st, "verify", "p1"); !hasReason(r, "not landed yet") {
		t.Fatalf("phase QA before land: %v", r)
	}

	// Land: base fast-forwards to exactly the phase commit, before any QA.
	if err := landPhase(t, st, "p1"); err != nil {
		t.Fatal(err)
	}
	if gitOutIn(repo, "rev-parse", "main") != cand.Repos[0].Commit {
		t.Fatal("base is not the phase commit")
	}
	if _, err := os.Stat(cand.Repos[0].Worktree); err != nil {
		t.Fatal("land removed the phase worktree phase QA runs in")
	}
	s, _ := projectRead(st)
	if p := s.Phases[0]; p.State != "landed" {
		t.Fatalf("phase 1 after land = %s %v", p.State, p.Problems)
	}
	if r := phaseReadiness(t, st, "dispatch", "p2"); len(r) != 0 {
		t.Fatalf("phase 2 must build while phase 1 runs its QA: %v", r)
	}
	if r := phaseReadiness(t, st, "release", "p1"); !hasReason(r, "not landed and verified") {
		t.Fatalf("released before phase QA: %v", r)
	}
	if r := phaseReadiness(t, st, "integrate", "p1"); !hasReason(r, "already landed") {
		t.Fatalf("re-integrated a landed phase: %v", r)
	}

	landClosed = nil
	for _, kind := range []string{"integration", "product"} {
		finishExecutionEvidence(t, st, phaseEvidence(t, st, kind, "p1"), kind, false)
	}
	s, _ = projectRead(st)
	if p := s.Phases[0]; p.State != "delivered" || len(p.Evidence) != 2 {
		t.Fatalf("phase 1 after QA = %s %v %v", p.State, p.Evidence, p.Problems)
	}
	if _, err := os.Stat(cand.Repos[0].Worktree); err == nil {
		t.Fatal("phase worktree kept after phase QA passed")
	}
	if !slices.Contains(landClosed, cand.Repos[0].Worktree) {
		t.Fatalf("cleanup did not close the phase worktree's terminals: %v", landClosed)
	}
	if r := phaseReadiness(t, st, "release", "p1"); !hasReason(r, "human release approval required") {
		t.Fatalf("ask phase released without a human go: %v", r)
	}
	if err := recordRelease(t, st, repo, "p1", "RELEASED"); err == nil {
		t.Fatal("receipt recorded without the human go")
	}

	// The go is human-only and bound to this exact phase subject.
	var err error
	captureStdout(t, func() { err = projectReleaseCommand(st, map[string]string{"phase": "p1", "request": "1"}) })
	if err != nil {
		t.Fatal(err)
	}
	ph := storeForTicket(st.Env, "ph-p1")
	full := "Coverage: phase 1 criteria AC1 passed\nHost: N/A — no host page in this phase\nReuse: existing release-prd flow\nPrototype: N/A — non-UI release decision\nScope: phase p1 tickets only\n"
	if code, _, reason := selfResolveGate(ph, ph.Env, foreman.Record{ID: "fm-test", Auto: true}, full); code != exitGrant {
		t.Fatalf("an --auto foreman with a full rubric reached a release go: code %d (%s)", code, reason)
	}
	if _, _, err := approvalResolve(ph, "approve", "ship it", "human"); err != nil {
		t.Fatal(err)
	}
	if r := phaseReadiness(t, st, "release", "p1"); len(r) != 0 {
		t.Fatalf("approved release blocked: %v", r)
	}

	// A failed release stops the line; the old go does not cover a retry.
	if err := recordRelease(t, st, repo, "p1", "ROLLED_BACK"); err != nil {
		t.Fatal(err)
	}
	s, _ = projectRead(st)
	if s.Phases[0].State != "release_failed" {
		t.Fatalf("failed release state = %s", s.Phases[0].State)
	}
	if r := phaseReadiness(t, st, "dispatch", "p2"); !hasReason(r, "stop the line") {
		t.Fatalf("later phase kept moving past a failed release: %v", r)
	}
	if r := phaseReadiness(t, st, "release", "p1"); !hasReason(r, "human release approval required") {
		t.Fatalf("pre-failure go reused for the retry: %v", r)
	}
	captureStdout(t, func() { err = projectReleaseCommand(st, map[string]string{"phase": "p1", "request": "1"}) })
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := approvalResolve(ph, "approve", "retry", "human"); err != nil {
		t.Fatal(err)
	}
	if err := recordRelease(t, st, repo, "p1", "RELEASED"); err != nil {
		t.Fatal(err)
	}
	if err := recordRelease(t, st, repo, "p1", "RELEASED"); err != nil {
		t.Fatalf("lost-response retry of a recorded release: %v", err)
	}
	s, _ = projectRead(st)
	if s.Phases[0].State != "released" {
		t.Fatalf("phase 1 = %s", s.Phases[0].State)
	}
	if r := phaseReadiness(t, st, "integrate", "p2"); hasReason(r, "phase p1") || !hasReason(r, "no child bound to extra") {
		t.Fatalf("phase 2 integration gate after phase 1 release: %v", r)
	}
	if s.Ready || !hasReason(s.Blockers, "phase p2") {
		t.Fatalf("project finished with an unshipped phase: %v", s.Blockers)
	}
}

func TestSinglePhaseProjectCompletesAfterItsRelease(t *testing.T) {
	st, _, repo := phasedFixture(t, []projectPhase{{ID: "p1", Title: "All of it", Seeds: []string{"core"}, Release: "auto"}})
	if err := foremanProjectCommand("bind", []string{"parent", "--phase", "p1", "--child", "ph-p1"}); err != nil {
		t.Fatal(err)
	}
	integrateAndLand(t, st, "p1")
	// Unscoped evidence is not proof for a phased project.
	spec := filepath.Join(t.TempDir(), "spec.json")
	if err := writeJSONAtomic(spec, map[string]interface{}{"kind": "integration", "producer": "qa"}); err != nil {
		t.Fatal(err)
	}
	var err error
	captureStdout(t, func() { err = projectEvidenceCommand(st, map[string]string{"begin": "1", "file": spec}) })
	if err == nil {
		t.Fatal("phased project accepted evidence without a phase")
	}
	for _, kind := range []string{"integration", "product"} {
		finishExecutionEvidence(t, st, phaseEvidence(t, st, kind, "p1"), kind, false)
	}
	s, _ := projectRead(st)
	if s.Ready || !hasReason(s.Blockers, "phase p1: delivered") {
		t.Fatalf("finished before the auto release: %v", s.Blockers)
	}
	// An auto phase gets one automatic fix-forward; the second failure
	// escalates its next attempt to a human go.
	for i := 0; i < 2; i++ {
		if r := phaseReadiness(t, st, "release", "p1"); len(r) != 0 {
			t.Fatalf("attempt %d blocked: %v", i+1, r)
		}
		if err := recordRelease(t, st, repo, "p1", "PARTIAL"); err != nil {
			t.Fatal(err)
		}
	}
	if r := phaseReadiness(t, st, "release", "p1"); !hasReason(r, "failed to release twice") {
		t.Fatalf("third attempt without a human: %v", r)
	}
	captureStdout(t, func() { err = projectReleaseCommand(st, map[string]string{"phase": "p1", "request": "1"}) })
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := approvalResolve(storeForTicket(st.Env, "ph-p1"), "approve", "try again", "human"); err != nil {
		t.Fatal(err)
	}
	if err := recordRelease(t, st, repo, "p1", "RELEASED"); err != nil {
		t.Fatal(err)
	}
	s, _ = projectRead(st)
	if !s.Ready {
		t.Fatalf("released single phase blocked: %v", s.Blockers)
	}
	for _, row := range s.Coverage {
		if row.State != "proven" {
			t.Fatalf("coverage %s = %s", row.ID, row.State)
		}
	}
	captureStdout(t, func() { err = projectComplete(st, "fm-test") })
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := projectRead(st); s.Completion != "verified" {
		t.Fatalf("completion = %s %v", s.Completion, s.Blockers)
	}
}

func TestContractAsksOnlyAtHandover(t *testing.T) {
	st, _, _ := phasedFixture(t, []projectPhase{
		{ID: "p1", Title: "Walking skeleton", Seeds: []string{"core"}, Release: "none"},
		{ID: "p2", Title: "Extra", Seeds: []string{"extra"}, Release: "ask"},
	}, "extra")
	var c projectContract
	if err := readProjectJSON(projectContractPath(st), &c); err != nil {
		t.Fatal(err)
	}
	for _, modes := range [][2]string{{"none", "ask"}, {"auto", "ask"}, {"auto", "none"}, {"ask", "ask"}} {
		c.Phases[0].Release, c.Phases[1].Release = modes[0], modes[1]
		in := filepath.Join(t.TempDir(), "contract.json")
		if err := writeJSONAtomic(in, c); err != nil {
			t.Fatal(err)
		}
		err := foremanProjectCommand("contract", []string{"parent", "--file", in})
		if midAsk := modes[0] == "ask"; midAsk != (err != nil) {
			t.Fatalf("%v: contract err = %v", modes, err)
		}
	}
}

func TestAutoForemanCannotApproveALoweredReleaseMode(t *testing.T) {
	st, _, _ := phasedFixture(t, []projectPhase{
		{ID: "p1", Title: "Walking skeleton", Seeds: []string{"core"}, Release: "none"},
		{ID: "p2", Title: "Extra", Seeds: []string{"extra"}, Release: "auto"},
	}, "extra")
	revise := func(p1, p2 string) (int, string) {
		t.Helper()
		var c projectContract
		if err := readProjectJSON(projectContractPath(st), &c); err != nil {
			t.Fatal(err)
		}
		c.Phases[0].Release, c.Phases[1].Release = p1, p2
		in := filepath.Join(t.TempDir(), "contract.json")
		if err := writeJSONAtomic(in, c); err != nil {
			t.Fatal(err)
		}
		if err := foremanProjectCommand("contract", []string{"parent", "--file", in}); err != nil {
			t.Fatal(err)
		}
		if _, err := approvalPublish(st, "project-plan", "revised phases", "developer"); err != nil {
			t.Fatal(err)
		}
		code, _, reason := selfResolveGate(st, st.Env, foreman.Record{ID: "fm-test", Auto: true}, projectRubric)
		return code, reason
	}

	// Raising a gate is the lead's to approve under --auto: p1 (none) now
	// ships with the p2 handover instead of an auto release.
	if code, reason := revise("none", "ask"); code != 0 {
		t.Fatalf("moving both seeds behind the handover was refused: %d %s", code, reason)
	}
	if _, _, err := approvalResolve(st, "approve", "auto review", "foreman"); err != nil {
		t.Fatal(err)
	}
	// Lowering any approved gate — even one raised by that approval — is not,
	// including a none phase that would now ship in an auto release.
	for _, modes := range [][2]string{{"auto", "ask"}, {"none", "auto"}} {
		code, reason := revise(modes[0], modes[1])
		if code != exitGrant || !strings.Contains(reason, "lowers an approved release mode") {
			t.Fatalf("%v: lowered gate self-resolved: %d %s", modes, code, reason)
		}
		if _, _, err := approvalResolve(st, "redirect", "keep the gates", "foreman"); err != nil {
			t.Fatal(err)
		}
	}
}

// A money plan under a basic design the human approved is not asked again;
// under one a Foreman approved for itself it still goes to a human.
func TestHumanApprovedDesignCoversTicketFloor(t *testing.T) {
	st, child, _ := phasedFixture(t, []projectPhase{{ID: "p1", Title: "Payments", Seeds: []string{"core"}, Release: "ask"}})
	if err := os.WriteFile(filepath.Join(child.Home(), "plan.md"), []byte("Charge the card with Stripe payments"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := approvalPublish(child, "plan", "ticket plan", "developer"); err != nil {
		t.Fatal(err)
	}
	rec := foreman.Record{ID: "fm-test"}
	if code, _, reason := selfResolveGate(child, child.Env, rec, projectRubric); code != 0 {
		t.Fatalf("human-approved design did not cover the plan: %d %s", code, reason)
	}
	if err := foreman.Save(rec); err != nil {
		t.Fatal(err)
	}
	if _, err := approvalPublish(st, "project-plan", "self review", "developer"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := approvalResolve(st, "approve", "auto review", "fm-test"); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := selfResolveGate(child, child.Env, rec, projectRubric); code != exitFloor {
		t.Fatalf("Foreman-approved design cleared the floor: %d", code)
	}
}
