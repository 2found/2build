package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

func decisionFixture(t *testing.T) (*ticket.Store, projectSnapshot, string) {
	t.Helper()
	st := ticket.New(identity.Env{ProjectHome: t.TempDir(), Ticket: "bs-project"})
	if err := ticket.WriteDoc(st.IndexPath(), ticket.Doc{}); err != nil {
		t.Fatal(err)
	}
	repo := gitRepo(t, "main")
	s := projectSnapshot{Ticket: st.Env.Ticket, Children: []projectChild{{ID: "bs-child", Repos: []projectRepo{{Canonical: repo}}}}}
	return st, s, repo
}

func logRow(t *testing.T, st *ticket.Store, kv map[string]string) (string, error) {
	t.Helper()
	var err error
	out := captureStdout(t, func() { err = projectLogCommand(st, kv) })
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(out), "DECISION=")), err
}

func TestDecisionLogRejectsIncompleteAndOversizedRows(t *testing.T) {
	st, _, _ := decisionFixture(t)
	full := map[string]string{"phase": "design", "decision": "Postgres over SQLite", "why": "spec C2: multi-user", "evidence": "design.md", "result": "verified"}
	for _, missing := range []string{"phase", "decision", "why", "evidence", "result"} {
		kv := map[string]string{}
		for k, v := range full {
			if k != missing {
				kv[k] = v
			}
		}
		if _, err := logRow(t, st, kv); err == nil {
			t.Fatalf("row without --%s was accepted", missing)
		}
	}
	big := map[string]string{}
	for k, v := range full {
		big[k] = v
	}
	big["why"] = strings.Repeat("x", decisionRowMax)
	if _, err := logRow(t, st, big); err == nil || !strings.Contains(err.Error(), "under") {
		t.Fatalf("oversized row: err = %v", err)
	}
	if _, err := logRow(t, st, map[string]string{"phase": "p1", "decision": "d", "why": "w", "evidence": "e", "result": "verified", "supersedes": "deadbeef"}); err == nil {
		t.Fatal("--supersedes of a missing row was accepted")
	}
	if _, err := os.Stat(projectDecisionsPath(st)); !os.IsNotExist(err) {
		t.Fatalf("rejected rows reached the trail: %v", err)
	}
}

func TestDecisionAuditFlagsUnresolvedEvidenceAndHidesSuperseded(t *testing.T) {
	st, s, repo := decisionFixture(t)
	head := gitOutIn(repo, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(st.Home(), "selection.md"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]string{
		"real-sha":    {"evidence": head[:10], "result": "verified"},
		"fake-sha":    {"evidence": "0123456789abcdef", "result": "verified"},
		"home-path":   {"evidence": "selection.md:2", "result": "verified"},
		"repo-path":   {"evidence": "f.txt", "result": "verified"},
		"past-eof":    {"evidence": "f.txt:9", "result": "verified"},
		"no-path":     {"evidence": "docs/missing.md", "result": "verified"},
		"url":         {"evidence": "https://example.invalid/pr/1", "result": "verified"},
		"rubric":      {"evidence": "rubric row Scope", "result": "inconclusive"},
		"to-supplant": {"evidence": "0000000aaaa", "result": "open"},
	}
	ids := map[string]string{}
	for name, kv := range rows {
		kv["phase"], kv["decision"], kv["why"] = "p1", name, "test"
		id, err := logRow(t, st, kv)
		if err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	if _, err := logRow(t, st, map[string]string{"phase": "p1", "decision": "corrected", "why": "w", "evidence": head, "result": "verified", "supersedes": ids["to-supplant"]}); err != nil {
		t.Fatal(err)
	}
	all, err := readDecisionRows(st)
	if err != nil || len(all) != len(rows)+1 {
		t.Fatalf("trail has %d rows (err %v), want %d", len(all), err, len(rows)+1)
	}
	live, hidden, flags := auditDecisions(st, s, all)
	if hidden != 1 || len(live) != len(rows) {
		t.Fatalf("live %d hidden %d", len(live), hidden)
	}
	flagged := map[string]string{}
	for _, f := range flags {
		flagged[f.Row] += f.Reason + ";"
	}
	for _, name := range []string{"fake-sha", "past-eof", "no-path", "rubric"} {
		if flagged[ids[name]] == "" {
			t.Errorf("%s not flagged", name)
		}
	}
	for _, name := range []string{"real-sha", "home-path", "repo-path", "url", "to-supplant"} {
		if why := flagged[ids[name]]; why != "" {
			t.Errorf("%s flagged: %s", name, why)
		}
	}

	page := renderProjectRecap(st, s)
	for _, want := range []string{"<h2>Check first</h2>", "<h2>Decisions</h2>", "1 superseded row(s) hidden", "not found in the project"} {
		if !strings.Contains(page, want) {
			t.Fatalf("recap missing %q", want)
		}
	}
	if strings.Contains(page, ">to-supplant<") {
		t.Fatal("superseded row rendered")
	}
}

func TestRecapSaysWhenThereIsNoTrail(t *testing.T) {
	st, s, _ := decisionFixture(t)
	page := renderProjectRecap(st, s)
	if !strings.Contains(page, "No decision trail") || strings.Contains(page, "Check first") {
		t.Fatal("recap without decisions.jsonl must say so and raise no flags")
	}
}
