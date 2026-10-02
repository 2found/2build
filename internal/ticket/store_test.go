package ticket

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/reallongnguyen/babysit/internal/identity"
)

func TestMutationAndHistoryKeepAgentContext(t *testing.T) {
	st := New(identity.Env{ProjectHome: t.TempDir(), Ticket: "bs-context", Branch: "main"})
	st.EnsureDirs()
	body := `{"id":"bs-context","status":"planned","pointers":{"plan":"custom/plan.md","requirement":"requirement.md"},"origin":{"type":"sub_ticket","parent":"parent","seed":"sub-tickets/001.md"},"children":["child"],"relations":{"blocked_by":["dependency"]},"siblings":[{"repo":"other","ticket":"peer"}],"custom":{"large_number":9007199254740993,"notes":["do not discard"]}}`
	if err := os.WriteFile(st.IndexPath(), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	before := ReadDoc(st.IndexPath())
	doc := st.LoadForMutate()
	doc.Set("status", "in_progress")
	if err := WriteDoc(st.IndexPath(), doc); err != nil {
		t.Fatal(err)
	}
	after := ReadDoc(st.IndexPath())
	before["status"] = "in_progress"
	delete(after, "updated_at")
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("mutation lost context: %#v", after)
	}

	st.HistoryAppend("started", "agent")
	st.HistoryAppendExtra("blocked", "agent", `{"ticket":"wrong","event":"wrong","note":"missing API","context":{"retry_from":"implement"}}`)
	history, err := os.ReadFile(filepath.Join(st.Home(), "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(history)), "\n")
	if len(lines) != 2 {
		t.Fatalf("history was overwritten: %s", history)
	}
	for i, event := range []string{"started", "blocked"} {
		var row map[string]interface{}
		if err := json.Unmarshal([]byte(lines[i]), &row); err != nil {
			t.Fatal(err)
		}
		if row["ticket"] != "bs-context" || row["event"] != event || row["actor"] != "agent" {
			t.Fatalf("history identity changed: %v", row)
		}
		if i == 1 && (row["note"] != "missing API" || row["context"].(map[string]interface{})["retry_from"] != "implement") {
			t.Fatalf("history context lost: %v", row)
		}
	}
}
