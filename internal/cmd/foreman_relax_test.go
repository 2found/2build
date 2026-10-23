package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/2found/2build/internal/ticket"
)

func TestSealWaivableOnlyCoversMissingOrStaleGates(t *testing.T) {
	rest, states := sealWaivable(
		[]string{"review-pr:stale_tree_digest", "review-pr:stale_surface", "qa:missing", "qa:material_findings", "checkpoint:missing"},
		[]string{"review-pr", "qa"})
	if !slices.Equal(rest, []string{"qa:material_findings", "checkpoint:missing"}) {
		t.Fatalf("non-waivable reasons = %v", rest)
	}
	if states["review-pr"] != "stale" || states["qa"] != "missing" {
		t.Fatalf("states = %v", states)
	}
	if rest, _ := sealWaivable([]string{"qa:result_not_pass", "review-pr:status_blocked"}, []string{"qa", "review-pr"}); len(rest) != 2 {
		t.Fatalf("a failed or BLOCKED verdict was waived: %v", rest)
	}
	if rest, _ := sealWaivable([]string{"qa:missing"}, []string{"review-pr"}); len(rest) != 1 {
		t.Fatalf("waiving review-pr covered qa: %v", rest)
	}
	if _, err := parseSealWaiver(map[string]string{"waive": "qa"}); err == nil {
		t.Fatal("--waive without --reason accepted")
	}
	if _, err := parseSealWaiver(map[string]string{"waive": "lint", "reason": "x"}); err == nil {
		t.Fatal("unknown gate waived")
	}
}

func TestSealWaiverSealsStaleGatesAndIsVisible(t *testing.T) {
	parent, child, _ := executionFixture(t)
	// The repair changed code after the gates were recorded.
	mustWrite(t, filepath.Join(child.Home(), "..", "..", "..", "..", "feature.txt"), "x")
	m, _ := ticket.ReadManifest(child.ManifestPath())
	mustWrite(t, filepath.Join(m.Repos[0].Worktree, "feature.txt"), "repaired journey\n")

	err := projectSealChild(child, nil, "")
	if err == nil || !strings.Contains(err.Error(), "stale_tree_digest") || !strings.Contains(err.Error(), "--waive") {
		t.Fatalf("seal on stale gates = %v, want refusal naming --waive", err)
	}
	if err := projectSealChild(child, []string{"qa"}, "docs only"); err == nil {
		t.Fatal("waiving qa alone covered a stale review-pr")
	}
	if err := projectSealChild(child, []string{"review-pr", "qa"}, "verified in the repair handoff"); err != nil {
		t.Fatal(err)
	}
	var seal projectSeal
	if err := readProjectJSON(filepath.Join(child.Home(), "project-seal.json"), &seal); err != nil {
		t.Fatal(err)
	}
	w := seal.Waiver
	if w == nil || w.Reason != "verified in the repair handoff" || w.At == "" || w.States["qa"] != "stale" || w.States["review-pr"] != "stale" || !slices.Equal(w.Gates, []string{"qa", "review-pr"}) {
		t.Fatalf("waiver = %+v", w)
	}
	s, err := projectRead(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range s.Blockers {
		if strings.Contains(b, "waive") || strings.Contains(b, "stale_tree") {
			t.Fatalf("a waiver became a blocker: %v", s.Blockers)
		}
	}
	if s.Children[0].Waiver == nil || len(s.Phases[0].Notes) == 0 || !strings.Contains(s.Phases[0].Notes[0], "verified in the repair handoff") {
		t.Fatalf("waiver not visible: child=%+v notes=%v", s.Children[0].Waiver, s.Phases[0].Notes)
	}
}

func TestSealWaiverNeverOverridesFindings(t *testing.T) {
	_, child, _ := executionFixture(t)
	m, _ := ticket.ReadManifest(child.ManifestPath())
	mustWrite(t, filepath.Join(m.Repos[0].Worktree, "feature.txt"), "repaired journey\n")
	// A stale gate that also carries a material finding is not just stale.
	if rest, _ := sealWaivable([]string{"review-pr:stale_tree_digest", "review-pr:material_findings"}, []string{"review-pr"}); len(rest) != 1 {
		t.Fatalf("material finding waived: %v", rest)
	}
}

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

// repairAndReintegrate waives the stale gates of a small repair and rebuilds
// the phase commit, so the earlier evidence belongs to the previous subject.
func repairAndReintegrate(t *testing.T, st, child *ticket.Store) {
	t.Helper()
	m, _ := ticket.ReadManifest(child.ManifestPath())
	if err := os.WriteFile(filepath.Join(m.Repos[0].Worktree, "feature.txt"), []byte("repaired journey\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := projectSealChild(child, []string{"review-pr", "qa"}, "small repair"); err != nil {
		t.Fatal(err)
	}
	integratePhase(t, st, "p1")
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
	integratePhase(t, st, "p1")
	oldInt := recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0}, carryCheck{"AC1", "failure", 0})
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
	if !p.Integrated || p.State != "integrated" {
		t.Fatalf("carried evidence did not satisfy readiness: %s %v", p.State, p.Problems)
	}
	noted := false
	for _, n := range p.Notes {
		noted = noted || strings.Contains(n, "carries checks from") && strings.Contains(n, "repair touched docs only")
	}
	if !noted {
		t.Fatalf("carried evidence not shown as carried: %v", p.Notes)
	}
	if r := phaseReadiness(t, st, "land", "p1"); len(r) != 0 {
		t.Fatalf("land blocked after carry: %v", r)
	}
}

func TestEvidenceCarryNeverCarriesFailedChecks(t *testing.T) {
	st, child, _ := executionFixture(t)
	integratePhase(t, st, "p1")
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
	if r := phaseReadiness(t, st, "land", "p1"); !hasReason(r, "criterion AC1: missing failure") {
		t.Fatalf("failed check satisfied readiness: %v", r)
	}
	if _, err := carry(t, st, map[string]string{"from": old, "except": "AC1", "reason": "x"}); err == nil {
		t.Fatal("carry with everything excepted accepted")
	}
}

func TestEvidenceCarryPlusFreshPartialAttempt(t *testing.T) {
	st, child, _ := executionFixture(t)
	integratePhase(t, st, "p1")
	old := recordChecks(t, st, "integration", carryCheck{"AC1", "failure", 0})
	oldProd := recordChecks(t, st, "product", carryCheck{"AC1", "product", 0})
	repairAndReintegrate(t, st, child)

	recordChecks(t, st, "integration", carryCheck{"AC1", "behavior", 0})
	if r := phaseReadiness(t, st, "land", "p1"); !hasReason(r, "criterion AC1: missing failure") {
		t.Fatalf("fresh partial attempt alone satisfied the cumulative check: %v", r)
	}
	if _, err := carry(t, st, map[string]string{"from": old + "," + oldProd, "reason": "failure path untouched"}); err != nil {
		t.Fatal(err)
	}
	if r := phaseReadiness(t, st, "land", "p1"); len(r) != 0 {
		t.Fatalf("fresh partial + carried did not satisfy readiness: %v", r)
	}
}

func TestEvidenceCarryRefusesOtherPhaseSource(t *testing.T) {
	st, child, _ := executionFixture(t)
	integratePhase(t, st, "p1")
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
