package cmd

import (
	"path/filepath"
	"testing"

	"github.com/2found/2build/internal/ticket"
)

func TestWeekOfIsMonday(t *testing.T) {
	for in, want := range map[string]string{"2026-11-02": "2026-11-02", "2026-11-08": "2026-11-02", "2026-11-09": "2026-11-09"} {
		if got, _ := weekOf(in); got != want {
			t.Errorf("weekOf(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestCompanySeriesLatestWins(t *testing.T) {
	home := t.TempDir()
	one, two := 1.0, 2.0
	for _, v := range []*float64{&one, &two} {
		if err := scoreRecord(home, scoreRow{Week: "2026-11-02", Product: "lohi", Revenue: v}); err != nil {
			t.Fatal(err)
		}
	}
	if err := scoreRecord(home, scoreRow{Week: "2026-11-02", Product: "lohi"}); err == nil {
		t.Fatal("empty scoreboard row accepted")
	}
	s := latestScores(home)
	if len(s) != 1 || *s[0].Revenue != 2 {
		t.Fatalf("scores = %+v", s)
	}
	_ = hoursRecord(home, hoursRow{Week: "2026-11-02", Product: "lohi", Stage: "marketing", Hours: 6})
	_ = hoursRecord(home, hoursRow{Week: "2026-11-02", Product: "lohi", Stage: "marketing", Hours: 7.5})
	if err := hoursRecord(home, hoursRow{Week: "2026-11-02", Product: "lohi", Stage: "sales", Hours: 1}); err == nil {
		t.Fatal("unknown stage accepted")
	}
	if h := latestHours(home); len(h) != 1 || h[0].Hours != 7.5 {
		t.Fatalf("hours = %+v", h)
	}
}

func TestDecisionLogKeepsSupersededAndReadsActual(t *testing.T) {
	home := t.TempDir()
	ph := filepath.Join(home, "projects", "demo")
	st := fieldStore(t, ph, "bs-opp")
	tgt, days := "retention", 30
	ws, _ := loopWrites(ticket.Doc{}, loopSpec{Target: &tgt, CheckAfter: &days})
	_, _ = fieldsApply(st, ws, "developer")
	_ = decideSet(st, "later", "not now", nil, "long", "developer")
	_ = decideSet(st, "go", "now", nil, "long", "developer")
	_ = actualSet(st, "+2pp", "hit", "", nil, "long", "developer")
	rows := decisionLog([]string{ph}, "2026-11-02")
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	cur, old := rows[0], rows[1]
	if cur.Superseded || cur.Decision != "go" || cur.Result != "hit" || cur.ActualDelta == nil {
		t.Fatalf("current row = %+v", cur)
	}
	if !old.Superseded || old.Decision != "later" || old.ActualDelta != nil {
		t.Fatalf("superseded row = %+v", old)
	}
	s := companySummaryFor(home, 4)
	if s.HitRate.Go != 1 || s.HitRate.Hit != 1 {
		t.Fatalf("hit rate = %+v", s.HitRate)
	}
}
