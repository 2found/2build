package ticket

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDocs lays out a project of records from id -> JSON.
func writeDocs(t *testing.T, recs map[string]string) string {
	t.Helper()
	ph := t.TempDir()
	for id, js := range recs {
		dir := filepath.Join(ph, "tickets", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(js), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ph
}

// The kiem-lai shape: work under phase 2 inherits phase 2's blocker on phase
// 1, and phase 1 (a container whose status nobody advances) stands for its work.
var phased = map[string]string{
	"m":  `{"kind":"milestone","product":"lohi","children":["p1","p2"]}`,
	"p1": `{"kind":"phase","parent":"m","status":"planned","children":["a"]}`,
	"p2": `{"kind":"phase","parent":"m","status":"planned","children":["b"],"relations":{"blocked_by":["p1"]}}`,
	"a":  `{"parent":"p1","status":"planned","assigned_to":{"role":"swe"}}`,
	"b":  `{"parent":"p2","status":"planned","assigned_to":{"role":"swe"}}`,
}

func TestPlanInheritsPhaseBlockers(t *testing.T) {
	ph := writeDocs(t, phased)
	rows := map[string]PlanRow{}
	for _, r := range LoadPlan(ph, "2026-11-02") {
		rows[r.ID] = r
	}
	if !rows["a"].Ready || rows["b"].Ready {
		t.Fatalf("a ready=%v b ready=%v", rows["a"].Ready, rows["b"].Ready)
	}
	if strings.Join(rows["b"].BlockedBy, ",") != "a" {
		t.Fatalf("b waits on %v, want the open work under p1", rows["b"].BlockedBy)
	}
	if rows["b"].Milestone != "m" || rows["b"].Phase != "p2" || rows["b"].Product != "lohi" {
		t.Fatalf("b placement = %+v", rows["b"])
	}
	f := PlanFilter{Role: "swe", Ready: true}
	var picked []string
	for _, r := range LoadPlan(ph, "2026-11-02") {
		if f.Match(r) {
			picked = append(picked, r.ID)
		}
	}
	if strings.Join(picked, ",") != "a" {
		t.Fatalf("pick-up queue = %v", picked)
	}
}

func TestDAGInheritsBelowRootOnly(t *testing.T) {
	ph := writeDocs(t, phased)
	g, err := BuildGraph(ph, "m")
	if err != nil {
		t.Fatal(err)
	}
	wave := map[string]int{}
	state := map[string]string{}
	for _, n := range g.Nodes {
		wave[n.ID], state[n.ID] = n.Wave, n.State
	}
	if wave["b"] <= wave["a"] || state["b"] != StateWaiting || state["a"] != StateReady {
		t.Fatalf("waves %v states %v", wave, state)
	}
	// Rooted at the phase, the phase's own blocker is context, not inherited:
	// a flat project keeps its waves.
	g2, err := BuildGraph(ph, "p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(g2.Nodes) != 1 || g2.Nodes[0].State != StateReady {
		t.Fatalf("rooted at p2: %+v", g2.Nodes)
	}
}

func TestParseWindowShapes(t *testing.T) {
	for in, want := range map[string]string{
		"02/12 – 31/12/2026": "2026-12-02..2026-12-31",
		"15/12 – 10/01/2027": "2026-12-15..2027-01-10",
		"11/2026 – 01/2027":  "2026-11-01..2027-01-31",
		"next month":         "",
		"31/02 – 01/03/2026": "",
	} {
		s, d, ok := ParseWindow(in)
		got := ""
		if ok {
			got = s + ".." + d
		}
		if got != want {
			t.Errorf("ParseWindow(%q) = %q, want %q", in, got, want)
		}
	}
}

// Work blocked by its own phase must not end up waiting on its siblings, and an
// explicit self-edge on a container stays visible as a cycle.
func TestContainerBlockerEdgeCases(t *testing.T) {
	ph := writeDocs(t, map[string]string{
		"m": `{"kind":"milestone","children":["p"]}`,
		"p": `{"kind":"phase","parent":"m","children":["x","y"]}`,
		"x": `{"parent":"p","status":"planned","relations":{"blocked_by":["p"]}}`,
		"y": `{"parent":"p","status":"planned","relations":{"blocked_by":["p"]}}`,
	})
	g, err := BuildGraph(ph, "m")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Cycles) != 0 {
		t.Fatalf("siblings blocked by their own phase formed cycles %v", g.Cycles)
	}
	for _, r := range LoadPlan(ph, "2026-11-02") {
		if (r.ID == "x" || r.ID == "y") && !r.Ready {
			t.Fatalf("%s not ready: waits on %v", r.ID, r.BlockedBy)
		}
	}

	ph2 := writeDocs(t, map[string]string{
		"m": `{"kind":"milestone","children":["p"]}`,
		"p": `{"kind":"phase","parent":"m","children":["x"],"relations":{"blocked_by":["p"]}}`,
		"x": `{"parent":"p","status":"planned"}`,
	})
	g2, err := BuildGraph(ph2, "m")
	if err != nil {
		t.Fatal(err)
	}
	if len(g2.Cycles) == 0 {
		t.Fatal("explicit container self-edge was dropped instead of reported")
	}
}

// Plan readiness agrees with the DAG on a one-sided `blocks` link and on a
// container marked done over open work.
func TestPlanBlockersMatchDAG(t *testing.T) {
	ph := writeDocs(t, map[string]string{
		"a":  `{"status":"planned","relations":{"blocks":["b"]}}`,
		"b":  `{"status":"planned"}`,
		"p1": `{"kind":"phase","status":"done","children":["l"]}`,
		"l":  `{"parent":"p1","status":"in_progress"}`,
		"c":  `{"status":"planned","relations":{"blocked_by":["p1"]}}`,
	})
	got := map[string][]string{}
	for _, r := range LoadPlan(ph, "2026-11-02") {
		got[r.ID] = r.BlockedBy
	}
	if fmt.Sprint(got["b"]) != "[a]" {
		t.Fatalf("b waits on %v, want [a] (one-sided blocks)", got["b"])
	}
	if fmt.Sprint(got["c"]) != "[l]" {
		t.Fatalf("c waits on %v, want [l] (done phase over open leaf)", got["c"])
	}
}
