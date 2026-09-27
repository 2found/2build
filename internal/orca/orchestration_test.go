package orca

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func calls(t *testing.T, log string) string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		return ""
	}
	return string(b)
}

func TestCapabilityGateReadsTheAdvertisedList(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		caps := `"terminal.multiplex.v1"`
		if enabled {
			caps += `,"orchestration.contract.v1"`
		}
		log := fakeOrca(t, `case "$1" in
 status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":[`+caps+`]}}}' ;;
 esac`)
		c, err := Preflight()
		if err != nil {
			t.Fatal(err)
		}
		if c.Orchestration() != enabled || !c.Supports("terminal.multiplex.v1") || c.Supports("nope.v1") {
			t.Fatal("capability gate does not match the advertised list")
		}
		if !enabled {
			if _, err := c.DispatchStateFor("task_1"); !errors.Is(err, ErrNoOrchestration) {
				t.Fatalf("want ErrNoOrchestration, got %v", err)
			}
			if strings.Contains(calls(t, log), "dispatch-show") {
				t.Fatal("queried a runtime without the required capability")
			}
		}
	}
}

func TestDispatchStatePreservesAttemptAndRun(t *testing.T) {
	log := fakeOrca(t, `case "$1" in
 status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":["orchestration.contract.v1"]}}}' ;;
 orchestration) echo '{"ok":true,"result":{"dispatch":{"id":"ctx_9","status":"dispatched","run_id":"run_1"}}}' ;;
 esac`)
	c, err := Preflight()
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.DispatchStateFor("task_1")
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "ctx_9" || d.Status != "dispatched" || d.RunID != "run_1" {
		t.Fatalf("lost resource lease identity: %+v", d)
	}
	if !strings.Contains(calls(t, log), "dispatch-show --task task_1") {
		t.Fatalf("wrong task inspected: %s", calls(t, log))
	}
}
