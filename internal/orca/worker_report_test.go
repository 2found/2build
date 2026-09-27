package orca

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPendingWorkerDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, reply, want string
		failure           bool
	}{
		{"unreported", `{"ok":true,"result":{"dispatchId":"ctx-current","messages":[],"count":0}}`, "ctx-current", false},
		{"ordinary or settled", `{"ok":true,"result":{"messages":[],"count":0}}`, "", false},
		{"coordinator", `{"ok":true,"result":{"runId":"run-1","messages":[],"count":0}}`, "", false},
		{"previous report cannot satisfy new dispatch", `{"ok":true,"result":{"dispatchId":"ctx-new","messages":[{"type":"worker_done","payload":{"dispatchId":"ctx-old","outcome":"succeeded"}}],"count":1}}`, "ctx-new", false},
		{"rejected report", `{"ok":true,"result":{"dispatchId":"ctx-current","messages":[{"type":"worker_done","payload":{"_orcaLifecycleRejection":{"code":"wrong_dispatch"}}}],"count":1}}`, "ctx-current", false},
		{"fenced owner", `{"ok":false,"error":{"code":"consumer_fenced"}}`, "", false},
		{"cancelled assignment", `{"ok":false,"error":{"code":"dispatch_inactive"}}`, "", false},
		{"unavailable", `{"ok":false,"error":{"code":"runtime_unavailable"}}`, "", true},
		{"malformed", `not json`, "", true},
		{"missing assignment fields", `{"ok":true,"result":{}}`, "", true},
		{"missing envelope", `{"messages":[],"count":0}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := fakeOrca(t, `printf '%s\n' "$BBS_TEST_REPLY"`)
			t.Setenv("BBS_TEST_REPLY", tc.reply)
			got, err := PendingWorkerDispatch(context.Background(), "term-worker")
			if (err != nil) != tc.failure || got != tc.want {
				t.Fatalf("got %q, %v; want %q, failure=%v", got, err, tc.want, tc.failure)
			}
			if got := strings.TrimSpace(calls(t, log)); got != "orchestration check --terminal term-worker --peek --json" {
				t.Fatalf("must only inspect this worker's assignment: %q", got)
			}
		})
	}
}

func TestPendingWorkerDispatchSkipsUnidentifiedSessions(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got, err := PendingWorkerDispatch(context.Background(), ""); got != "" || err != nil {
		t.Fatalf("ordinary session consulted Orca: %q, %v", got, err)
	}
}

func TestPendingWorkerDispatchBoundsUnavailableRuntime(t *testing.T) {
	fakeOrca(t, `exec sleep 30`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := PendingWorkerDispatch(ctx, "term-worker"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("hook did not respect its deadline")
	}
}

func TestPendingWorkerDispatchRefusesCommandFailureWithSuccessBody(t *testing.T) {
	fakeOrca(t, `echo '{"ok":true,"result":{"messages":[],"count":0}}'; exit 1`)
	if _, err := PendingWorkerDispatch(context.Background(), "term-worker"); err == nil {
		t.Fatal("failed command was treated as successful verification")
	}
}
