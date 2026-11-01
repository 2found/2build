package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/ticket"
)

type carryCheck struct {
	criterion, kind string
	code            int
}

func recordChecks(t *testing.T, st *ticket.Store, kind string, checks ...carryCheck) string {
	t.Helper()
	id := phaseEvidence(t, st, kind, "p1")
	log := filepath.Join(t.TempDir(), "check.log")
	mustWrite(t, log, "Executed real fixture assertion\n")
	var cs []projectCheck
	for _, c := range checks {
		code := c.code
		cs = append(cs, projectCheck{Criterion: c.criterion, Kind: c.kind, Command: []string{"fixture", c.kind}, ExitCode: &code, Log: log})
	}
	result := filepath.Join(t.TempDir(), "results.json")
	if err := writeJSONAtomic(result, map[string]interface{}{"checks": cs, "findings": []projectFinding{}}); err != nil {
		t.Fatal(err)
	}
	var err error
	captureStdout(t, func() { err = projectEvidenceCommand(st, map[string]string{"attempt": id, "file": result}) })
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// repairAndReintegrate records a small repair and rebuilds the phase commit,
// so the earlier evidence belongs to the previous subject.
func repairAndReintegrate(t *testing.T, st, child *ticket.Store) {
	t.Helper()
	m, _ := ticket.ReadManifest(child.ManifestPath())
	if err := os.WriteFile(filepath.Join(m.Repos[0].Worktree, "feature.txt"), []byte("repaired journey\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := projectRecordChild(child); err != nil {
		t.Fatal(err)
	}
	integrateAndLand(t, st, "p1")
}

func carry(t *testing.T, st *ticket.Store, kv map[string]string) (string, error) {
	t.Helper()
	kv["carry"], kv["phase"] = "1", "p1"
	var err error
	out := captureStdout(t, func() { err = projectEvidenceCommand(st, kv) })
	return out, err
}

func TestEvidenceCarryForwardSatisfiesReadiness(t *testing.T) {
	st, child, _ := executionFixture(t)
	integrateAndLand(t, st, "p1")
	// Phase QA found one issue (a failed runtime probe); the rest passed.
	oldInt := recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 0}, carryCheck{"AC1", "runtime", 1})
	oldProd := recordChecks(t, st, "product", carryCheck{"AC1", "product", 0})
	repairAndReintegrate(t, st, child)

	if s, _ := projectRead(st); s.Phases[0].Integrated {
		t.Fatal("phase integrated on a new subject without evidence")
	}
	if _, err := carry(t, st, map[string]string{"from": oldInt}); err == nil {
		t.Fatal("carry without --reason accepted")
	}
	out, err := carry(t, st, map[string]string{"from": oldInt + "," + oldProd, "reason": "repair touched docs only"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "carried_from") {
		t.Fatalf("carried checks not marked: %s", out)
	}
	s, err := projectRead(st)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Phases[0]
	if !p.Integrated || p.State != "delivered" {
		t.Fatalf("carried evidence did not satisfy readiness: %s %v", p.State, p.Problems)
	}
	noted := false
	for _, n := range p.Notes {
		noted = noted || strings.Contains(n, "carries checks from") && strings.Contains(n, "repair touched docs only")
	}
	if !noted {
		t.Fatalf("carried evidence not shown as carried: %v", p.Notes)
	}
	if r := phaseQAProblems(t, st, "p1"); len(r) != 0 {
		t.Fatalf("phase QA unmet after carry: %v", r)
	}
}

func TestEvidenceCarryNeverCarriesFailedChecks(t *testing.T) {
	st, child, _ := executionFixture(t)
	integrateAndLand(t, st, "p1")
	old := recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 1})
	oldProd := recordChecks(t, st, "product", carryCheck{"AC1", "product", 0})
	repairAndReintegrate(t, st, child)

	out, err := carry(t, st, map[string]string{"from": old + "," + oldProd, "reason": "partial"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "AC1/failure (not passed") {
		t.Fatalf("failed check not reported as not carried: %s", out)
	}
	if r := phaseQAProblems(t, st, "p1"); !hasReason(r, "criterion AC1: missing failure") {
		t.Fatalf("failed check satisfied readiness: %v", r)
	}
	if _, err := carry(t, st, map[string]string{"from": old, "except": "AC1", "reason": "x"}); err == nil {
		t.Fatal("carry with everything excepted accepted")
	}
}

func TestPhaseEvidenceStandsAcrossReintegration(t *testing.T) {
	st, child, _ := executionFixture(t)
	integrateAndLand(t, st, "p1")
	old := recordChecks(t, st, "integration", carryCheck{"AC1", "failure", 0})
	oldProd := recordChecks(t, st, "product", carryCheck{"AC1", "product", 0})
	repairAndReintegrate(t, st, child)

	// Evidence recorded before the reintegration stands: the fresh partial
	// attempt plus the earlier one satisfy the cumulative check, no carry.
	recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0})
	_, _ = old, oldProd
	if r := phaseQAProblems(t, st, "p1"); len(r) != 0 {
		t.Fatalf("earlier phase evidence went stale on reintegration: %v", r)
	}
}

func TestEvidenceCarryRefusesOtherPhaseSource(t *testing.T) {
	st, child, _ := executionFixture(t)
	integrateAndLand(t, st, "p1")
	old := recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0})
	var ev projectEvidence
	dir := filepath.Join(st.Home(), "project-evidence")
	if err := readProjectJSON(filepath.Join(dir, old+".json"), &ev); err != nil {
		t.Fatal(err)
	}
	ev.Phase, ev.ID = "p2", "other1"
	body, _ := json.Marshal(ev)
	mustWrite(t, filepath.Join(dir, "other1.json"), string(body))
	repairAndReintegrate(t, st, child)

	if _, err := carry(t, st, map[string]string{"from": "other1", "reason": "x"}); err == nil || !strings.Contains(err.Error(), "phase") {
		t.Fatalf("source from another phase = %v", err)
	}
	if _, err := carry(t, st, map[string]string{"from": "nosuch", "reason": "x"}); err == nil {
		t.Fatal("unknown source accepted")
	}
}

// Phase QA is cumulative, but an earlier phase's passed checks carry into a
// later phase: phase 2 re-tests only its own criterion and what it touched.
// Cumulative phase QA counts earlier phases' passing evidence: phase 2 tests
// only its own criteria, with no carry step.
func TestEarlierPhaseEvidenceStandsForCumulativeQA(t *testing.T) {
	st, child, repo := phasedFixture(t, []projectPhase{
		{ID: "p1", Title: "Skeleton", Seeds: []string{"core"}, Release: "none"},
		{ID: "p2", Title: "Extra", Seeds: []string{"extra"}, Release: "none"},
	}, "extra")
	for _, phase := range []string{"p1", "p2"} {
		if err := foremanProjectCommand("bind", []string{"parent", "--phase", phase, "--child", "ph-" + phase}); err != nil {
			t.Fatal(err)
		}
	}
	integrateAndLand(t, st, "p1")
	p1Int := phaseEvidence(t, st, "integration", "p1")
	finishExecutionEvidence(t, st, p1Int, "integration", false)
	p1Prod := phaseEvidence(t, st, "product", "p1")
	finishExecutionEvidence(t, st, p1Prod, "product", false)

	// Phase 2's one ticket, cut from the base phase 1 landed on.
	wt := filepath.Join(t.TempDir(), "extra")
	runGit(t, repo, "worktree", "add", "-q", "-b", "feat/extra_x", wt, "main")
	mustWrite(t, filepath.Join(wt, "extra.txt"), "extra journey\n")
	env := child.Env
	env.Ticket, env.Branch = "extra", "feat/extra_x"
	ex := storeForTicket(env, "extra")
	mustMkdirAll(t, ex.Home())
	mustWrite(t, filepath.Join(ex.Home(), "checkpoint.json"), `{"schema_version":2,"ticket":"extra","run_id":"run-1","revision":1,"workflow":"builder"}`)
	if err := ticket.WriteManifest(ex.ManifestPath(), "extra", "App", []ticket.Repo{{Name: "app", Canonical: repo, Worktree: wt, Branch: env.Branch, Base: "main"}}); err != nil {
		t.Fatal(err)
	}
	if err := ticket.WriteDoc(ex.IndexPath(), ticket.Doc{"id": "extra", "parent": "ph-p2", "status": "in_review", "origin": map[string]interface{}{"project_seed": "extra"}}); err != nil {
		t.Fatal(err)
	}
	ph2 := storeForTicket(env, "ph-p2")
	doc := ticket.ReadDoc(ph2.IndexPath())
	doc["children"] = []interface{}{"extra"}
	if err := ticket.WriteDoc(ph2.IndexPath(), doc); err != nil {
		t.Fatal(err)
	}
	if err := projectRecordChild(ex); err != nil {
		t.Fatal(err)
	}
	integrateAndLand(t, st, "p2")
	if r := phaseQAProblems(t, st, "p2"); hasReason(r, "criterion AC1: missing") || !hasReason(r, "criterion AC-extra: missing behavior") {
		t.Fatalf("phase 2 should owe only its own criteria: %v", r)
	}

	var err error
	log := filepath.Join(t.TempDir(), "check.log")
	mustWrite(t, log, "Executed real fixture assertion\n")
	code := 0
	for _, kind := range []string{"integration", "product"} {
		id := phaseEvidence(t, st, kind, "p2")
		check := projectCheck{Criterion: "AC-extra", Kind: "behavior", Command: []string{"fixture", "extra"}, ExitCode: &code, Log: log}
		if kind == "product" {
			check.Kind = "product"
		}
		result := filepath.Join(t.TempDir(), "results.json")
		checks := []projectCheck{check}
		if kind == "integration" {
			// extra never reviewed its own diff, so the phase reviews it once.
			checks = append(checks, projectCheck{Criterion: "p2", Kind: "review", Command: []string{"fixture", "review"}, ExitCode: &code, Log: log})
		}
		if err := writeJSONAtomic(result, map[string]interface{}{"checks": checks, "findings": []projectFinding{}}); err != nil {
			t.Fatal(err)
		}
		captureStdout(t, func() { err = projectEvidenceCommand(st, map[string]string{"attempt": id, "file": result}) })
		if err != nil {
			t.Fatal(err)
		}
	}
	if r := phaseQAProblems(t, st, "p2"); len(r) != 0 {
		t.Fatalf("phase 1 evidence + fresh phase 2 checks did not satisfy readiness: %v", r)
	}
}
