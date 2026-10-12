package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// batchFixture serves Dispatch state from files so a test can settle tasks
// between polls: <dir>/<task> holds the status, <dir>/questions the unread
// question count, <dir>/exited the dispatch ids whose worker is gone.
func batchFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	stub := `#!/bin/sh
D="` + dir + `"
case "$1" in
  status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":["orchestration.contract.v1"]}}}' ;;
  orchestration)
    case "$2" in
      dispatch-show)
        task="$4"
        if [ -f "$D/$task" ]; then
          echo "{\"ok\":true,\"result\":{\"dispatch\":{\"id\":\"ctx-$task\",\"status\":\"$(cat "$D/$task")\",\"run_id\":\"run-1\"}}}"
        else
          echo '{"ok":true,"result":{"dispatch":null}}'
        fi ;;
      check) echo "{\"ok\":true,\"result\":{\"messages\":[],\"count\":$(cat "$D/questions" 2>/dev/null || echo 0)}}" ;;
      worker-list)
        rows=""
        for id in $(cat "$D/exited" 2>/dev/null); do
          rows="$rows${rows:+,}{\"dispatchId\":\"$id\",\"dispatchStatus\":\"dispatched\",\"terminalState\":\"active\",\"projection\":{\"liveness\":{\"verdict\":\"exited\"}}}"
        done
        echo "{\"ok\":true,\"result\":{\"workers\":[$rows],\"page\":{\"hasMore\":false}}}" ;;
      *) echo '{"ok":true,"result":{}}' ;;
    esac ;;
  *) echo '{"ok":true,"result":{}}' ;;
esac
`
	bin := filepath.Join(dir, "orca")
	if err := os.WriteFile(bin, []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCA_CLI_COMMAND", bin)
	return dir
}

func writeBatchFile(t *testing.T, dir, name, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeBatchClock advances time by each requested sleep and runs onSleep, so
// a test controls what the next poll observes without real waiting.
func fakeBatchClock(t *testing.T, onSleep func(n int)) *int {
	t.Helper()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sleeps := 0
	oldSleep, oldNow := waitBatchSleep, waitBatchNow
	waitBatchNow = func() time.Time { return now }
	waitBatchSleep = func(d time.Duration) {
		sleeps++
		now = now.Add(d)
		if onSleep != nil {
			onSleep(sleeps)
		}
	}
	t.Cleanup(func() { waitBatchSleep, waitBatchNow = oldSleep, oldNow })
	return &sleeps
}

func TestWaitBatchReturnsOnceWhenEveryDispatchSettles(t *testing.T) {
	dir := batchFixture(t)
	writeBatchFile(t, dir, "task-a", "completed")
	writeBatchFile(t, dir, "task-b", "dispatched")
	writeBatchFile(t, dir, "task-c", "dispatched")
	sleeps := fakeBatchClock(t, func(n int) {
		switch n {
		case 1:
			writeBatchFile(t, dir, "task-b", "failed")
		case 2:
			writeBatchFile(t, dir, "task-c", "completed")
		}
	})
	out := captureStdout(t, func() {
		if err := foremanWaitBatch([]string{"--tasks", "task-a,task-b,task-c"}); err != nil {
			t.Fatal(err)
		}
	})
	// A failed Dispatch is settled: it waits for its batch like a success.
	for _, want := range []string{"BATCH=settled\n", "BATCH_SETTLED=3/3\n",
		"TASK=task-b DISPATCH=ctx-task-b STATUS=failed\n", "TASK=task-c DISPATCH=ctx-task-c STATUS=completed\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
	if *sleeps != 2 {
		t.Fatalf("sleeps = %d, want 2", *sleeps)
	}
}

func TestWaitBatchWakesEarlyForQuestion(t *testing.T) {
	dir := batchFixture(t)
	writeBatchFile(t, dir, "task-a", "dispatched")
	writeBatchFile(t, dir, "questions", "1")
	sleeps := fakeBatchClock(t, nil)
	out := captureStdout(t, func() {
		if err := foremanWaitBatch([]string{"--tasks", "task-a"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "BATCH=attention\n") || !strings.Contains(out, "ATTENTION=messages:1\n") || *sleeps != 0 {
		t.Fatalf("sleeps=%d out=%q", *sleeps, out)
	}
}

func TestWaitBatchWakesEarlyForWorkerThatExitedUnsettled(t *testing.T) {
	dir := batchFixture(t)
	writeBatchFile(t, dir, "task-a", "dispatched")
	writeBatchFile(t, dir, "task-b", "dispatched")
	fakeBatchClock(t, func(n int) { writeBatchFile(t, dir, "exited", "ctx-task-b") })
	out := captureStdout(t, func() {
		if err := foremanWaitBatch([]string{"--tasks", "task-a,task-b"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "BATCH=attention\n") || !strings.Contains(out, "ATTENTION=exited:task-b\n") ||
		strings.Contains(out, "exited:task-a") {
		t.Fatalf("out=%q", out)
	}
}

func TestWaitBatchTimesOutWithoutSettling(t *testing.T) {
	dir := batchFixture(t)
	writeBatchFile(t, dir, "task-a", "dispatched")
	sleeps := fakeBatchClock(t, nil)
	out := captureStdout(t, func() {
		if err := foremanWaitBatch([]string{"--tasks", "task-a", "--timeout", "1m", "--interval", "20s"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "BATCH=timeout\n") || !strings.Contains(out, "BATCH_SETTLED=0/1\n") || *sleeps != 3 {
		t.Fatalf("sleeps=%d out=%q", *sleeps, out)
	}
}

func TestWaitBatchRejectsUnlaunchedTaskAndBadFlags(t *testing.T) {
	batchFixture(t)
	fakeBatchClock(t, nil)
	if err := foremanWaitBatch([]string{"--tasks", "task-missing"}); err == nil || !strings.Contains(err.Error(), "has no Dispatch") {
		t.Fatalf("unlaunched task: %v", err)
	}
	if err := foremanWaitBatch(nil); err == nil || !strings.Contains(err.Error(), "--tasks is required") {
		t.Fatalf("missing tasks: %v", err)
	}
	if err := foremanWaitBatch([]string{"--tasks", "a", "--timeout", "0s"}); err == nil {
		t.Fatal("zero timeout accepted")
	}
}
