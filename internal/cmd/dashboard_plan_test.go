package cmd

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardPlanWrites(t *testing.T) {
	s, home := sandboxServer(t)
	t.Setenv("BBS_PERSON", "founder")
	const base = "/api/tickets/proj/bs-aaaa1111"

	w := post(t, s, base+"/fields", `{"title":"New title","kind":"phase","product":"lohi",
		"assigned_to":{"role":"marketing"},"labels_add":["growth"],"window":{"start":"2026-11-01","due":"2026-11-30"},
		"loop":{"target_outcome":"revenue","expected_delta":"+5m","check_after_days":30}}`)
	if w.Code != 200 {
		t.Fatalf("fields: %d %s", w.Code, w.Body)
	}
	idx := readIndex(t, home)
	if idx["title"] != "New title" || idx["kind"] != "phase" || idx["product"] != "lohi" {
		t.Fatalf("fields not written: %v", idx)
	}
	if w := post(t, s, base+"/fields", `{"assigned_to":{"role":"foreman"}}`); w.Code != 400 {
		t.Fatalf("bad role: %d", w.Code)
	}
	if w := post(t, s, base+"/decision", `{"decision":"go","note":"ship"}`); w.Code != 200 {
		t.Fatalf("decision: %d %s", w.Code, w.Body)
	}
	if by := readIndex(t, home)["loop"].(map[string]interface{})["decision"].(map[string]interface{})["by"]; by != "founder" {
		t.Fatalf("decision by = %v", by)
	}
	if w := post(t, s, base+"/actual", `{"delta":"+4m","result":"bogus"}`); w.Code != 400 {
		t.Fatalf("bad result accepted: %d", w.Code)
	}
	if w := post(t, s, base+"/actual", `{"delta":"+4m","result":"hit"}`); w.Code != 200 {
		t.Fatalf("actual: %d %s", w.Code, w.Body)
	}

	// Cross-origin writes are refused like every other mutation.
	r := httptest.NewRequest("POST", base+"/decision", strings.NewReader(`{"decision":"drop"}`))
	r.Header.Set("Origin", "http://evil.example")
	rw := httptest.NewRecorder()
	s.mux().ServeHTTP(rw, r)
	if rw.Code != 403 {
		t.Fatalf("cross-origin decision: %d", rw.Code)
	}

	if w := post(t, s, "/api/company/scoreboard", `{"product":"lohi","week":"2026-11-04","revenue":43150000}`); w.Code != 200 {
		t.Fatalf("scoreboard: %d %s", w.Code, w.Body)
	}
	if w := post(t, s, "/api/company/hours", `{"product":"lohi","stage":"marketing","hours":6,"week":"2026-11-04"}`); w.Code != 200 {
		t.Fatalf("hours: %d %s", w.Code, w.Body)
	}
	cw := send(t, s, "GET", "/api/company?weeks=104")
	var sum companySummary
	if err := json.Unmarshal(cw.Body.Bytes(), &sum); err != nil || cw.Code != 200 {
		t.Fatalf("company: %d %v", cw.Code, err)
	}
	if len(sum.Scoreboard) != 1 || len(sum.Hours) != 1 || sum.HitRate.Hit != 1 || len(sum.Decisions) != 1 {
		t.Fatalf("summary = %+v", sum)
	}

	// The snapshot carries the plan projection on the summary row.
	snap := send(t, s, "GET", "/api/snapshot")
	if !strings.Contains(snap.Body.String(), `"planning":`) || !strings.Contains(snap.Body.String(), `"New title"`) {
		t.Fatalf("snapshot lacks plan projection")
	}
}
