package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/2found/2build/internal/identity"
)

// scriptedProbe returns each prerequisite's states in order, repeating the
// last one, and a fake clock that advances by every sleep.
func scriptedProbe(script map[string][]prereqState) (prereqProbe, func(time.Duration), func() time.Time, *int) {
	calls := map[string]int{}
	clock := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	sleeps := 0
	probe := func(p prereq) prereqState {
		states := script[p.ticket]
		i := calls[p.ticket]
		calls[p.ticket]++
		if i >= len(states) {
			i = len(states) - 1
		}
		return states[i]
	}
	sleep := func(d time.Duration) { clock = clock.Add(d); sleeps++ }
	now := func() time.Time { return clock }
	return probe, sleep, now, &sleeps
}

func TestAwaitReturnsHeadsOnceEveryPrerequisiteSettles(t *testing.T) {
	probe, sleep, now, sleeps := scriptedProbe(map[string][]prereqState{
		"bs-a": {{State: "waiting", Reason: "dispatch dispatched"}, {State: "ready", Head: "aaa"}},
		"bs-b": {
			{State: "waiting", Reason: "dispatch dispatched"},
			{State: "waiting", Reason: "dispatch dispatched"},
			{State: "light-ready", Head: "bbb", Reason: "deferred heavy: ios-simulator"},
		},
	})
	var out bytes.Buffer
	var adopted []string
	adopt := func(heads []string) ([]string, error) { adopted = heads; return nil, nil }
	code := awaitPrereqs([]prereq{{"bs-a", "task-a"}, {"bs-b", "task-b"}}, probe, adopt, time.Hour, 20*time.Second, sleep, now, &out)
	if code != 0 {
		t.Fatalf("code = %d, want 0\n%s", code, out.String())
	}
	got := out.String()
	for _, want := range []string{
		"PREREQ=bs-a TASK=task-a STATE=ready SNAPSHOT=aaa",
		"PREREQ=bs-b TASK=task-b STATE=light-ready SNAPSHOT=bbb",
		"AWAIT=ready",
		"NEXT=the prerequisites' work is now in this worktree as uncommitted changes",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if *sleeps != 2 {
		t.Fatalf("sleeps = %d, want 2", *sleeps)
	}
	// An unchanged waiting state is printed once, not once per poll.
	if n := strings.Count(got, "bs-b waiting"); n != 1 {
		t.Fatalf("bs-b waiting printed %d times:\n%s", n, got)
	}
	// Order is the order given, so prerequisites are adopted in DAG order.
	if strings.Index(got, "PREREQ=bs-a") > strings.Index(got, "PREREQ=bs-b") || strings.Join(adopted, ",") != "aaa,bbb" {
		t.Fatalf("prerequisites out of order (adopted %v):\n%s", adopted, got)
	}
}

func TestAwaitReportsConflictsLeftByAdoption(t *testing.T) {
	probe, sleep, now, _ := scriptedProbe(map[string][]prereqState{
		"bs-a": {{State: "ready", Head: "aaa"}},
	})
	var out bytes.Buffer
	adopt := func([]string) ([]string, error) { return []string{"src/shared.go"}, nil }
	code := awaitPrereqs([]prereq{{"bs-a", "task-a"}}, probe, adopt, time.Hour, 20*time.Second, sleep, now, &out)
	got := out.String()
	if code != 5 || !strings.Contains(got, "CONFLICT=src/shared.go") || !strings.Contains(got, "AWAIT=conflict") {
		t.Fatalf("code=%d, conflict outcome missing:\n%s", code, got)
	}
}

func TestAwaitStopsAtTheFirstPrerequisiteThatSettlesWithoutPassing(t *testing.T) {
	probe, sleep, now, _ := scriptedProbe(map[string][]prereqState{
		"bs-a": {{State: "waiting", Reason: "dispatch dispatched"}},
		"bs-b": {{State: "not-ready", Reason: "qa:missing"}},
	})
	var out bytes.Buffer
	adopt := func([]string) ([]string, error) { t.Fatal("a not-ready wait must adopt nothing"); return nil, nil }
	code := awaitPrereqs([]prereq{{"bs-a", "task-a"}, {"bs-b", "task-b"}}, probe, adopt, time.Hour, 20*time.Second, sleep, now, &out)
	if code != 3 {
		t.Fatalf("code = %d, want 3\n%s", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, `PREREQ=bs-b TASK=task-b STATE=not-ready REASON="qa:missing"`) || !strings.Contains(got, "AWAIT=not-ready") {
		t.Fatalf("not-ready outcome missing:\n%s", got)
	}
	if strings.Contains(got, "SNAPSHOT=") {
		t.Fatalf("a not-ready wait must not hand out snapshots:\n%s", got)
	}
}

func TestAwaitTimesOutWithinItsBound(t *testing.T) {
	probe, sleep, now, sleeps := scriptedProbe(map[string][]prereqState{
		"bs-a": {{State: "waiting", Reason: "dispatch dispatched"}},
	})
	var out bytes.Buffer
	code := awaitPrereqs([]prereq{{"bs-a", "task-a"}}, probe, nil, 60*time.Second, 20*time.Second, sleep, now, &out)
	if code != 4 {
		t.Fatalf("code = %d, want 4\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "AWAIT=timeout") || !strings.Contains(out.String(), "NEXT=run the same command again") {
		t.Fatalf("timeout outcome missing:\n%s", out.String())
	}
	// 60s budget at 20s polls: sleep twice, then the next sleep would cross the deadline.
	if *sleeps != 2 {
		t.Fatalf("sleeps = %d, want 2", *sleeps)
	}
}

func TestParsePrereqs(t *testing.T) {
	got, err := parsePrereqs(" bs-a:task-1, bs-b:task-2 ,bs-a:task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (prereq{"bs-a", "task-1"}) || got[1] != (prereq{"bs-b", "task-2"}) {
		t.Fatalf("parsed %+v", got)
	}
	for _, bad := range []string{"", "bs-a", "bs-a:", ":task"} {
		if _, err := parsePrereqs(bad); err == nil {
			t.Fatalf("parsePrereqs(%q) accepted", bad)
		}
	}
}

// A prerequisite Task that is unreadable or not launched yet is a wait; one
// whose Dispatch failed is not-ready.
func TestLiveProbeWaitsOnAnUnlaunchedTask(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.world.Dispatch["task-new"] = ""
	f.world.Dispatch["task-bad"] = "failed"
	f.save(t)
	probe := liveProbe(t.TempDir(), identity.Env{})
	if s := probe(prereq{"bs-a", "task-gone"}); s.State != "waiting" || s.Reason != "not dispatched yet" {
		t.Fatalf("unknown task = %+v", s)
	}
	if s := probe(prereq{"bs-a", "task-new"}); s.State != "waiting" || s.Reason != "not dispatched yet" {
		t.Fatalf("unlaunched task = %+v", s)
	}
	if s := probe(prereq{"bs-a", "task-bad"}); s.State != "not-ready" {
		t.Fatalf("failed task = %+v", s)
	}
}

// A completed prerequisite whose readiness cannot be read is retried before
// it is declared not-ready; a later good read resets the count.
func TestLiveProbeRetriesAFailingReadinessRead(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.world.Dispatch["task-a"] = "completed"
	f.save(t)
	probe := liveProbe(filepath.Join(t.TempDir(), "missing"), identity.Env{})
	for i := 1; i < awaitReadinessAttempts; i++ {
		if s := probe(prereq{"bs-a", "task-a"}); s.State != "waiting" || !strings.Contains(s.Reason, "readiness") {
			t.Fatalf("attempt %d = %+v", i, s)
		}
	}
	if s := probe(prereq{"bs-a", "task-a"}); s.State != "not-ready" {
		t.Fatalf("final attempt = %+v", s)
	}
}
