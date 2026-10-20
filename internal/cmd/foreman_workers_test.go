package cmd

import (
	"strings"
	"testing"

	"github.com/2found/2build/internal/foreman"
)

func TestSpawnPinsExplicitWorkersAndDefaultsToForemanHarness(t *testing.T) {
	log, _ := fakeOrcaFor(t)
	dir := trustedDir(t)
	out := captureStdout(t, func() {
		if _, err := foremanSpawn([]string{"fm-workers", "--dir", dir, "--agent", "claude",
			"--implement-worker", "omp", "--plan-worker", "codex"}); err != nil {
			t.Fatal(err)
		}
	})
	rec, err := foreman.Load("fm-workers")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Agent != "claude" || rec.ImplementWorker != "omp" || rec.PlanWorker != "codex" || rec.QAWorker != "" {
		t.Fatalf("worker pins: %+v", rec)
	}
	if !strings.Contains(out, "workers plan=codex implement=omp qa=claude") {
		t.Fatalf("spawn output missing resolved workers: %s", out)
	}
	calls := readCalls(t, log)
	if !strings.Contains(calls, "--implement-worker omp") || !strings.Contains(calls, "--plan-worker codex") {
		t.Fatalf("spawn prompt missing worker flags:\n%s", calls)
	}
	if strings.Contains(calls, "--qa-worker") {
		t.Fatalf("empty qa-worker should stay omitted:\n%s", calls)
	}
}

func TestSpawnRejectsUnknownWorkerHarness(t *testing.T) {
	fakeOrcaFor(t)
	_, err := foremanSpawn([]string{"fm-bad", "--dir", trustedDir(t), "--implement-worker", "antigravity"})
	if err == nil || !strings.Contains(err.Error(), "--implement-worker") || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("want named agent list, got %v", err)
	}
}

func TestAdoptRejectsUnknownWorkerHarnessBeforeRename(t *testing.T) {
	log, _ := fakeOrcaFor(t)
	setFakeCurrent(t, log, "claude", "/repo")
	err := foremanAdopt([]string{"fm-bad-adopt", "--plan-worker", "antigravity"})
	if err == nil || !strings.Contains(err.Error(), "--plan-worker") {
		t.Fatalf("want named agent list, got %v", err)
	}
	if _, loadErr := foreman.Load("fm-bad-adopt"); loadErr == nil {
		t.Fatal("failed adopt saved a record")
	}
	if strings.Contains(readCalls(t, log), "terminal rename") {
		t.Fatalf("invalid worker pin renamed the Orca terminal:\n%s", readCalls(t, log))
	}
}

func TestAdoptPersistsWorkerPins(t *testing.T) {
	log, _ := fakeOrcaFor(t)
	setFakeCurrent(t, log, "claude", "/repo")
	out := captureStdout(t, func() {
		if err := foremanAdopt([]string{"fm-adopt-workers", "--qa-worker", "omp"}); err != nil {
			t.Fatal(err)
		}
	})
	rec, err := foreman.Load("fm-adopt-workers")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Agent != "claude" || rec.QAWorker != "omp" || rec.ImplementWorker != "" {
		t.Fatalf("adopt worker pins: %+v", rec)
	}
	if !strings.Contains(out, "workers plan=claude implement=claude qa=omp") {
		t.Fatalf("adopt output missing resolved workers: %s", out)
	}
}

func TestForemanModelQuotaOffloadWasRemoved(t *testing.T) {
	t.Setenv("BABYSIT_STATE_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	err := foremanModel([]string{"--agent", "claude", "--quota", "--host", "host-a"})
	if err == nil || !strings.Contains(err.Error(), "--quota offload was removed") {
		t.Fatalf("want migration error, got %v", err)
	}
}
