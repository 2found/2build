package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/ticket"
)

func TestImportIsIdempotentAndNeverRewritesRequirement(t *testing.T) {
	ph := t.TempDir()
	fieldStore(t, ph, "bs-exist000")
	p := importPlan{Plan: "h1", Product: "lohi", Tickets: []importEntry{
		{Key: "m", Kind: "milestone", Title: "Revenue"},
		{Key: "a", Parent: "m", Role: "swe", BlockedBy: []string{"bs-exist000"}, Requirement: "# A\n"},
	}}
	res, err := importProject(ph, p, false, "developer")
	if err != nil || res[0].Action != "create" || res[1].Action != "create" {
		t.Fatalf("first import: %+v %v", res, err)
	}
	a := peerStore(ticket.New(identityFor(ph, "")), res[1].ID)
	req := filepath.Join(a.Home(), "requirement.md")
	_ = os.WriteFile(req, []byte("# edited by a human\n"), 0o644)
	p.Tickets[1].Requirement = "# A, re-imported\n"
	res2, err := importProject(ph, p, false, "developer")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res2 {
		if r.Action != "unchanged" {
			t.Fatalf("re-import changed %+v", r)
		}
	}
	if b, _ := os.ReadFile(req); string(b) != "# edited by a human\n" {
		t.Fatalf("import rewrote requirement.md: %q", b)
	}
	d := ticket.ReadDoc(a.IndexPath())
	if d.Get("parent") != res[0].ID || d.Get("assigned_to.role") != "swe" || d.Get("product") != "lohi" {
		t.Fatalf("fields: %v", d)
	}
	if !strings.Contains(ticket.ReadDoc(filepath.Join(ph, "tickets", "bs-exist000", "index.json")).Get("relations.blocks"), res[1].ID) {
		t.Fatal("blocked_by inverse missing on the existing ticket")
	}
}

func TestImportValidatesBeforeWriting(t *testing.T) {
	ph := t.TempDir()
	for _, p := range []importPlan{
		{Plan: "h1", Tickets: []importEntry{{Key: "x", Role: "foreman"}}},
		{Plan: "h1", Tickets: []importEntry{{Key: "x"}, {Key: "x"}}},
		{Plan: "h1", Tickets: []importEntry{{Key: "x", Parent: "y"}, {Key: "y", Parent: "x"}}},
		{Plan: "h1", Tickets: []importEntry{{Key: "x", BlockedBy: []string{"y"}}, {Key: "y", BlockedBy: []string{"x"}}}},
		{Plan: "h1", Tickets: []importEntry{{Key: "x", BlockedBy: []string{"x"}}}},
		{Plan: "h1", Tickets: []importEntry{{Key: "x", Status: "done"}}},
		{Plan: "", Tickets: []importEntry{{Key: "x"}}},
	} {
		if _, err := importProject(ph, p, false, "developer"); err == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
	if ents, _ := os.ReadDir(filepath.Join(ph, "tickets")); len(ents) != 0 {
		t.Fatalf("a refused plan wrote %d records", len(ents))
	}
}

func TestMigrateBackfillsHeaderAndMovesRole(t *testing.T) {
	ph := t.TempDir()
	st := fieldStore(t, ph, "bs-aaaaaaaa")
	_ = withLock(st, func() error {
		d := st.LoadForMutate()
		d.Set("assignee", "swe")
		d.Set("status", "done")
		return ticket.WriteDoc(st.IndexPath(), d)
	})
	st.HistoryAppendExtra("status_changed", "developer", `{"from":"planned","to":"done"}`)
	_ = os.WriteFile(filepath.Join(st.Home(), "requirement.md"), []byte(strings.Join([]string{
		"# PHASE 2 · Listen more (02/12 – 31/12/2026)", "",
		"- **Loại:** phase (con của milestone)",
		"- **Product:** lohi",
		"- **Assignee:** human (founder)",
		"- **Target outcome:** reader_to_listener_7d",
		"- **Expected delta:** +30%",
		"- **Check after days:** 30 (read 01/02)",
		"- **Quyết định:** go — founder 2026-11-02",
		"- **Bằng chứng:** https://claude.ai/artifact/X, bs-bbbbbbbb",
		"- **Key:** listen", "",
		"## Body", "- **Owns:** not a header bullet",
	}, "\n")), 0o644)
	if _, err := migrateProject(ph, false, "migrate"); err != nil {
		t.Fatal(err)
	}
	d := ticket.ReadDoc(st.IndexPath())
	want := map[string]string{
		"title": "Phase 2 · Listen more", "kind": "phase", "product": "lohi",
		"assigned_to.role": "swe", "assignee": "", "window.start": "2026-12-02", "window.due": "2026-12-31",
		"loop.check_after_days": "30", "loop.decision.value": "go", "loop.decision.at": "2026-11-02T00:00:00Z",
		"import.key": "listen",
	}
	for k, v := range want {
		if got := d.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if d.Get("done_at") == "" || len(strList(d.Value("loop.evidence_refs"))) != 2 {
		t.Errorf("done_at=%q evidence=%v", d.Get("done_at"), d.Value("loop.evidence_refs"))
	}
	rep, err := migrateProject(ph, false, "migrate")
	if err != nil || rep.Changed != 0 {
		t.Fatalf("second migrate changed %d (%v)", rep.Changed, err)
	}
}
