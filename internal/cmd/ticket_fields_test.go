package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

func fieldStore(t *testing.T, ph, id string) *ticket.Store {
	t.Helper()
	st := ticket.New(identity.Env{ProjectHome: ph, Ticket: id, Slug: "demo"})
	if err := withLock(st, func() error {
		d := st.LoadForMutate()
		return ticket.WriteDoc(st.IndexPath(), d)
	}); err != nil {
		t.Fatal(err)
	}
	return st
}

func historyCount(t *testing.T, st *ticket.Store, event string) int {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(st.Home(), "history.jsonl"))
	return strings.Count(string(b), `"event":"`+event+`"`)
}

func TestFieldsApplyIsIdempotentAndLogged(t *testing.T) {
	st := fieldStore(t, t.TempDir(), "bs-a")
	w1, _ := titleWrite("Ship the thing")
	w2, _ := assignedToWrite("swe", "")
	for i := 0; i < 2; i++ {
		if _, err := fieldsApply(st, []fieldWrite{w1, w2}, "developer"); err != nil {
			t.Fatal(err)
		}
	}
	if n := historyCount(t, st, "field_set"); n != 2 {
		t.Fatalf("field_set events = %d, want 2 (second apply is a no-op)", n)
	}
	d := ticket.ReadDoc(st.IndexPath())
	if d.Get("title") != "Ship the thing" || d.Get("assigned_to.role") != "swe" {
		t.Fatalf("fields not written: %v", d)
	}
	if d.Value("assignee") != nil {
		t.Fatalf("assign-to touched the lead claim: %v", d.Value("assignee"))
	}
	labels, _ := labelsWith(d, []string{"b", "a", "a"}, nil)
	if strings.Join(labels, ",") != "a,b" {
		t.Fatalf("labels = %v", labels)
	}
}

func TestValidatorsRejectBadInput(t *testing.T) {
	if _, err := assignedToWrite("foreman", ""); err == nil {
		t.Fatal("unknown role accepted")
	}
	if _, err := windowWrite("2026-11-10", "2026-11-01"); err == nil {
		t.Fatal("due before start accepted")
	}
	if _, err := windowWrite("2026-02-30", ""); err == nil {
		t.Fatal("impossible date accepted")
	}
	if _, err := titleWrite("two\nlines"); err == nil {
		t.Fatal("multi-line title accepted")
	}
	if _, err := productWrite("Lohi AI"); err == nil {
		t.Fatal("bad product slug accepted")
	}
}

func TestDecideGoNeedsTargetAndHuman(t *testing.T) {
	st := fieldStore(t, t.TempDir(), "bs-opp")
	if err := decideSet(st, "go", "", nil, "long", "developer"); err == nil {
		t.Fatal("go without a target outcome accepted")
	}
	tgt, exp, days := "D7 retention", "+3pp", 14
	ws, err := loopWrites(ticket.ReadDoc(st.IndexPath()), loopSpec{Target: &tgt, Expected: &exp, CheckAfter: &days})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fieldsApply(st, ws, "developer"); err != nil {
		t.Fatal(err)
	}
	if err := decideSet(st, "go", "ship", []string{"https://e/1"}, "long", "developer"); err != nil {
		t.Fatal(err)
	}
	d := ticket.ReadDoc(st.IndexPath())
	if d.Get("loop.decision.value") != "go" || d.Get("loop.decision.by") != "long" {
		t.Fatalf("decision not projected: %v", d.Value("loop"))
	}
	if historyCount(t, st, "loop_decision") != 1 {
		t.Fatal("loop_decision history row missing")
	}

	t.Setenv("BABYSIT_FOREMAN", "")
	t.Setenv("OPENCLAW_SESSION", "")
	for _, role := range []string{"orca", "scanner", "mayor"} {
		t.Setenv("AGENT_ROLE", role)
		if humanGate("decide") == nil {
			t.Fatalf("decide allowed for AGENT_ROLE=%s", role)
		}
	}
	t.Setenv("AGENT_ROLE", "developer")
	if humanGate("decide") != nil {
		t.Fatal("decide refused for a developer")
	}
	t.Setenv("BABYSIT_FOREMAN", "f-1")
	if humanGate("decide") == nil {
		t.Fatal("decide allowed inside a Team run")
	}
}

func TestCheckOverdueAfterDone(t *testing.T) {
	st := fieldStore(t, t.TempDir(), "bs-w")
	days := 7
	ws, _ := loopWrites(ticket.Doc{}, loopSpec{CheckAfter: &days})
	if _, err := fieldsApply(st, ws, "developer"); err != nil {
		t.Fatal(err)
	}
	if _, ok := ticket.CheckDue(ticket.ReadDoc(st.IndexPath())); ok {
		t.Fatal("work not done yet has a check date")
	}
	if err := statusSet(st, "done", "developer"); err != nil {
		t.Fatal(err)
	}
	d := ticket.ReadDoc(st.IndexPath())
	due, ok := ticket.CheckDue(d)
	if !ok || !ticket.CheckOverdue(d, "2999-01-01") || ticket.CheckOverdue(d, "2000-01-01") {
		t.Fatalf("check due=%q ok=%v", due, ok)
	}
	if err := actualSet(st, "+1pp", "hit", "", nil, "long", "developer"); err != nil {
		t.Fatal(err)
	}
	if ticket.CheckOverdue(ticket.ReadDoc(st.IndexPath()), "2999-01-01") {
		t.Fatal("a recorded actual still reads overdue")
	}
}

func TestRelationsAreSymmetric(t *testing.T) {
	ph := t.TempDir()
	parent, old := fieldStore(t, ph, "bs-p"), fieldStore(t, ph, "bs-old")
	child, peer := fieldStore(t, ph, "bs-c"), fieldStore(t, ph, "bs-x")
	get := func(st *ticket.Store, path string) string { return ticket.ReadDoc(st.IndexPath()).Get(path) }

	if err := parentSet(child, "bs-old", "developer"); err != nil {
		t.Fatal(err)
	}
	if err := parentSet(child, "bs-p", "developer"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(get(parent, "children"), "bs-c") || strings.Contains(get(old, "children"), "bs-c") {
		t.Fatalf("reparent: new=%s old=%s", get(parent, "children"), get(old, "children"))
	}
	if err := childAdd(old, "bs-c", "developer"); err == nil {
		t.Fatal("add-child stole a child with another parent")
	} else if _, ok := err.(*usageErr); !ok {
		t.Fatalf("conflict is not a usage error: %v", err)
	}
	if err := childAdd(child, "bs-p", "developer"); err == nil {
		t.Fatal("add-child closed a loop")
	}
	if err := relationEdit(child, "blocked_by", "bs-x", true, "developer"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(get(peer, "relations.blocks"), "bs-c") {
		t.Fatal("blocked_by did not write blocks on the target")
	}
	if err := relationEdit(peer, "blocks", "bs-c", true, "developer"); err != nil {
		t.Fatal(err)
	}
	if historyCount(t, peer, "relation_added") != 1 {
		t.Fatal("re-adding an inverse already in place logged again")
	}
	if err := relationEdit(child, "blocked_by", "bs-x", false, "developer"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(get(peer, "relations.blocks"), "bs-c") {
		t.Fatal("remove did not drop the inverse")
	}
	// A target without a record is never created.
	if err := relationEdit(child, "related", "bs-ghost", true, "developer"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ph, "tickets", "bs-ghost")); err == nil {
		t.Fatal("inverse invented a ticket directory")
	}
}

func TestRepairRelationsAddsMissingInverses(t *testing.T) {
	ph := t.TempDir()
	a, b := fieldStore(t, ph, "bs-a"), fieldStore(t, ph, "bs-b")
	c := fieldStore(t, ph, "bs-c")
	// One-sided links, as written before symmetric verbs existed.
	_ = withLock(a, func() error {
		d := a.LoadForMutate()
		_ = setJSON(d, "relations.blocks", []string{"bs-b"})
		_ = setJSON(d, "children", []string{"bs-c"})
		return ticket.WriteDoc(a.IndexPath(), d)
	})
	_ = withLock(b, func() error {
		d := b.LoadForMutate()
		d.Set("parent", "bs-z") // a parent with no record needs no inverse
		return ticket.WriteDoc(b.IndexPath(), d)
	})
	fixes, err := planRepairs(ph, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 2 {
		t.Fatalf("fixes = %+v", fixes)
	}
	if n, err := applyRepairs(a, fixes, "developer"); err != nil || n != 2 {
		t.Fatalf("applied %d %v", n, err)
	}
	if ticket.ReadDoc(c.IndexPath()).Get("parent") != "bs-a" ||
		!strings.Contains(ticket.ReadDoc(b.IndexPath()).Get("relations.blocked_by"), "bs-a") {
		t.Fatal("repair did not write the inverses")
	}
	again, _ := planRepairs(ph, nil)
	if len(again) != 0 {
		t.Fatalf("repair is not idempotent: %+v", again)
	}
}

func TestEmptyLabelsAndDoneAtStamping(t *testing.T) {
	st := fieldStore(t, t.TempDir(), "bs-e")
	l, _ := labelsWith(ticket.ReadDoc(st.IndexPath()), nil, []string{"nope"})
	changed, err := fieldsApply(st, []fieldWrite{{"labels", l}}, "developer")
	if err != nil || len(changed) != 0 || historyCount(t, st, "field_set") != 0 {
		t.Fatalf("removing an absent label changed %v (err %v)", changed, err)
	}
	// reconcile's writer stamps done_at like set-status does.
	if err := applyStatus(st, "done"); err != nil {
		t.Fatal(err)
	}
	if ticket.ReadDoc(st.IndexPath()).Get("done_at") == "" {
		t.Fatal("applyStatus done left done_at empty")
	}
}
