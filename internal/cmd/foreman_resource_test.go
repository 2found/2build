package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/2found/2build/internal/config"
	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/orca"
)

func resourceCLIFixture(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("BABYSIT_HOME", home)
	t.Setenv("BABYSIT_STATE_DIR", home)
	// status reconciles leases against Orca; a missing binary is the "Orca
	// unreachable" path, which holds every lease — the default for tests that
	// are not about reconciliation.
	t.Setenv("ORCA_CLI_COMMAND", filepath.Join(home, "no-orca"))
	if err := foreman.Save(foreman.Record{ID: "fm-a", Heartbeat: foreman.Now()}); err != nil {
		t.Fatal(err)
	}
	old := newResourceBroker
	newResourceBroker = func() *foreman.ResourceBroker {
		return &foreman.ResourceBroker{
			Dir: filepath.Join(home, "resources"),
			Probe: func() foreman.HostResources {
				return foreman.HostResources{CPUs: 8, TotalMemoryBytes: 32 << 30}
			},
		}
	}
	t.Cleanup(func() { newResourceBroker = old })
}

func TestForemanResourceReserveStatusRelease(t *testing.T) {
	resourceCLIFixture(t)
	out := captureStdout(t, func() {
		if err := foremanResource([]string{
			"reserve", "fm-a", "--ticket", "bs-a", "--task", "task-a", "--profile", "standard",
		}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "ADMISSION=reserved\n") || !strings.Contains(out, "GLOBAL_BUDGET=8\n") {
		t.Fatalf("reserve output: %q", out)
	}
	var lease string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "LEASE=") {
			lease = strings.TrimPrefix(line, "LEASE=")
		}
	}
	if lease == "" {
		t.Fatalf("reserve omitted lease: %q", out)
	}

	out = captureStdout(t, func() {
		if err := foremanResource([]string{"status"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "GLOBAL_USED=1\n") || !strings.Contains(out, "ACTIVE_LEASE="+lease) {
		t.Fatalf("status output: %q", out)
	}

	out = captureStdout(t, func() {
		if err := foremanResource([]string{"release", lease}); err != nil {
			t.Fatal(err)
		}
	})
	if out != "RELEASED="+lease+"\n" {
		t.Fatalf("release output: %q", out)
	}
}

func TestForemanResourceRequiresRegisteredOwner(t *testing.T) {
	resourceCLIFixture(t)
	err := foremanResource([]string{
		"reserve", "fm-missing", "--ticket", "bs-a", "--task", "task-a", "--profile", "plan",
	})
	if err == nil || !strings.Contains(err.Error(), "foreman fm-missing") {
		t.Fatalf("error = %v", err)
	}
}

// The self-heal path: status is what every foreman wake runs, so it owns the
// lease cross-check. A settled Dispatch releases its lease; an open Dispatch
// whose worker cannot be checked and a fresh lease with no Task yet hold.
func TestForemanResourceStatusReleasesTerminalLeases(t *testing.T) {
	resourceCLIFixture(t)
	// task-done settled, task-live still dispatched (its worker unverifiable
	// with Orca unreachable), task-none has no Task record at all.
	seedTask(t, "task-done", "ctx_1", "completed", "term_1", time.Now())
	seedTask(t, "task-live", "ctx_2", "dispatched", "term_2", time.Now())

	reserve := func(task string) string {
		status, err := newResourceBroker().Reserve(foreman.ResourceRequest{ForemanID: "fm-a", Ticket: "bs-a", Task: task, Profile: "standard"}, 0)
		if err != nil {
			t.Fatal(err)
		}
		return status.Lease.ID
	}
	doneLease := reserve("task-done")
	liveLease := reserve("task-live")
	noneLease := reserve("task-none")

	out := captureStdout(t, func() {
		if err := foremanResource([]string{"status"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "RELEASED_LEASE="+doneLease+"\n") {
		t.Fatalf("terminal dispatch lease not released: %q", out)
	}
	if strings.Contains(out, "ACTIVE_LEASE="+doneLease) {
		t.Fatalf("released lease still active: %q", out)
	}
	for _, lease := range []string{liveLease, noneLease} {
		if !strings.Contains(out, "ACTIVE_LEASE="+lease) {
			t.Fatalf("unproven lease %s was dropped: %q", lease, out)
		}
	}
	// GLOBAL_USED counts only the two held leases now.
	if !strings.Contains(out, "GLOBAL_USED=2\n") {
		t.Fatalf("status output: %q", out)
	}
}

func TestConfiguredResourceCapValidation(t *testing.T) {
	resourceCLIFixture(t)
	if got, err := configuredResourceCap(); err != nil || got != 0 {
		t.Fatalf("missing cap = %d, %v", got, err)
	}
	if err := config.Set("parallel_global_units", "3"); err != nil {
		t.Fatal(err)
	}
	if got, err := configuredResourceCap(); err != nil || got != 3 {
		t.Fatalf("configured cap = %d, %v", got, err)
	}
	if err := config.Set("parallel_global_units", "many"); err != nil {
		t.Fatal(err)
	}
	if _, err := configuredResourceCap(); err == nil {
		t.Fatal("invalid cap was accepted")
	}
}

// recoveryClient is an Orca whose `terminal list` serves $TERMINALS, or fails
// when it is unset (Orca lost contact).
func recoveryClient(t *testing.T) *orca.Client {
	t.Helper()
	home := os.Getenv("BABYSIT_HOME")
	stub := `#!/bin/sh
case "$1 $2" in
 "status "*) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":[]}}}' ;;
 "terminal list")
  if [ -z "$TERMINALS" ]; then echo '{"ok":false,"error":{"message":"runtime unreachable"}}'; exit 1; fi
  printf '{"ok":true,"result":{"terminals":%s}}\n' "$TERMINALS" ;;
 *) echo '{"ok":true,"result":{}}' ;;
esac
`
	path := filepath.Join(home, "orca-recovery")
	if err := os.WriteFile(path, []byte(stub), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCA_CLI_COMMAND", path)
	c, err := orca.Preflight()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestResourceRecoveryAfterOwnerInterruption(t *testing.T) {
	for _, tc := range []struct {
		name, owner, dispatch, worker string
		age                           time.Duration
		released                      bool
	}{
		{name: "abandoned launch", owner: "stale", age: time.Hour, released: true},
		{name: "deleted owner", owner: "missing", age: time.Hour, released: true},
		{name: "live owner still launching", owner: "live", age: time.Hour},
		{name: "startup grace", owner: "stale", age: time.Minute},
		{name: "live worker outlives owner", owner: "stale", age: time.Hour, dispatch: "dispatched", worker: "live"},
		{name: "lost contact is not exit", owner: "missing", age: time.Hour, dispatch: "dispatched", worker: "unverifiable"},
		{name: "vanished terminal is not exit", owner: "stale", age: time.Hour, dispatch: "dispatched", worker: "gone"},
		{name: "exited agent in live terminal", owner: "stale", age: time.Hour, dispatch: "dispatched", worker: "exited", released: true},
		{name: "exited worker with healthy owner", owner: "live", age: time.Hour, dispatch: "dispatched", worker: "exited", released: true},
		{name: "just launched worker", owner: "stale", age: time.Hour, dispatch: "dispatched", worker: "starting"},
		{name: "completed worker", owner: "stale", age: time.Hour, dispatch: "completed", released: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resourceCLIFixture(t)
			now := time.Now().UTC()
			switch tc.owner {
			case "missing":
				if err := os.Remove(filepath.Join(foreman.Dir(), "fm-a.yaml")); err != nil {
					t.Fatal(err)
				}
			case "stale":
				if err := foreman.Save(foreman.Record{ID: "fm-a", Heartbeat: now.Add(-time.Hour).Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
			}
			launched := now.Add(-time.Hour)
			if tc.worker == "starting" {
				launched = now
			}
			if tc.dispatch != "" {
				seedTask(t, "task-zombie", "ctx-zombie", tc.dispatch, "term_z", launched)
			}
			switch tc.worker {
			case "live":
				t.Setenv("TERMINALS", `[{"handle":"term_z","agentIdentity":"codex"}]`)
			case "exited", "starting":
				t.Setenv("TERMINALS", `[{"handle":"term_z","agentIdentity":""}]`)
			case "gone":
				t.Setenv("TERMINALS", `[]`)
			default:
				t.Setenv("TERMINALS", "")
			}
			c := recoveryClient(t)
			b := newResourceBroker()
			b.Now = func() time.Time { return now.Add(-tc.age) }
			seeded, err := b.Reserve(foreman.ResourceRequest{ForemanID: "fm-a", Ticket: "bs-a", Task: "task-zombie", Profile: "ios-simulator"}, 0)
			if err != nil {
				t.Fatal(err)
			}
			released := reconcileResourceLeases(context.Background(), b, seeded.Leases, c, now)
			if (len(released) == 1) != tc.released {
				t.Fatalf("released=%v want %v", released, tc.released)
			}
			if tc.worker == "exited" {
				task, _ := foreman.LoadTask("task-zombie")
				if d := task.Current(); d.Status != foreman.DispatchFailed || !strings.Contains(d.Reason, "exited") {
					t.Fatalf("exited worker's dispatch = %+v", d)
				}
			}
			status, err := b.Status(0)
			if err != nil {
				t.Fatal(err)
			}
			if (status.Used == 0) != tc.released {
				t.Fatalf("capacity=%d", status.Used)
			}
		})
	}
}

func TestReserveReclaimsOtherForemansCapacity(t *testing.T) {
	resourceCLIFixture(t)
	if err := foreman.Save(foreman.Record{ID: "fm-new", Heartbeat: foreman.Now()}); err != nil {
		t.Fatal(err)
	}
	b := newResourceBroker()
	if _, err := b.Reserve(foreman.ResourceRequest{ForemanID: "fm-a", Ticket: "bs-a", Task: "old", Profile: "ios-simulator"}, 0); err != nil {
		t.Fatal(err)
	}
	seedTask(t, "old", "old-done", "completed", "term_old", time.Now())
	recoveryClient(t)
	out := captureStdout(t, func() {
		if err := foremanResourceReserve([]string{"fm-new", "--ticket", "bs-new", "--task", "new", "--profile", "ios-simulator"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "ADMISSION=reserved\n") || !strings.Contains(out, "RELEASED_LEASE=") {
		t.Fatalf("new owner blocked by completed worker: %s", out)
	}
	// The Task's previous terminal Dispatch must not release the pending retry.
	out = captureStdout(t, func() {
		if err := foremanResourceStatus(nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "GLOBAL_USED=4\n") || strings.Contains(out, "RELEASED_LEASE=") {
		t.Fatalf("old completion consumed new reservation: %s", out)
	}
}

func TestResourceReserveEnforcesConfiguredWorkerLimit(t *testing.T) {
	resourceCLIFixture(t)
	if err := config.Set("parallel_max_workers", "1"); err != nil {
		t.Fatal(err)
	}
	reserve := func(task string) string {
		return captureStdout(t, func() {
			if err := foremanResourceReserve([]string{"fm-a", "--ticket", "bs-a", "--task", task, "--profile", "plan"}); err != nil {
				t.Fatal(err)
			}
		})
	}
	if out := reserve("one"); !strings.Contains(out, "ADMISSION=reserved\n") {
		t.Fatal(out)
	}
	if out := reserve("one"); !strings.Contains(out, "ADMISSION=reserved\n") {
		t.Fatalf("retry not idempotent: %s", out)
	}
	if out := reserve("two"); !strings.Contains(out, "ADMISSION=queued\n") {
		t.Fatal(out)
	}
}

// The fixture host has 8 CPUs, so the auto bound is min(16, max(2, 8-2)) = 6.
func TestResourceReserveDefaultsToCPUDerivedWorkers(t *testing.T) {
	resourceCLIFixture(t)
	reserve := func(task string) string {
		return captureStdout(t, func() {
			if err := foremanResourceReserve([]string{"fm-a", "--ticket", "bs-a", "--task", task, "--profile", "plan"}); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, task := range []string{"one", "two", "three", "four", "five", "six"} {
		if out := reserve(task); !strings.Contains(out, "ADMISSION=reserved\n") {
			t.Fatalf("%s: %s", task, out)
		}
	}
	if out := reserve("seven"); !strings.Contains(out, "ADMISSION=queued\n") ||
		!strings.Contains(out, "foreman worker limit reached: 6 of 6") {
		t.Fatalf("default worker limit not enforced: %s", out)
	}
	out := captureStdout(t, func() {
		if err := foremanResource([]string{"status"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "FOREMAN_MAX_WORKERS=6\n") {
		t.Fatalf("status omitted effective worker bound: %q", out)
	}
	if err := config.Set("parallel_max_workers", "auto"); err != nil {
		t.Fatal(err)
	}
	if got, err := configuredMaxWorkers(32); err != nil || got != 16 {
		t.Fatalf("auto on 32 CPUs = %d, %v", got, err)
	}
	if got, err := configuredMaxWorkers(2); err != nil || got != 2 {
		t.Fatalf("auto on 2 CPUs = %d, %v", got, err)
	}
	if err := config.Set("parallel_max_workers", "zero"); err != nil {
		t.Fatal(err)
	}
	if _, err := configuredMaxWorkers(8); err == nil {
		t.Fatal("invalid parallel_max_workers accepted")
	}
}

func TestWatchRecoversCapacityWithoutOpenForeman(t *testing.T) {
	resourceCLIFixture(t)
	old := time.Now().Add(-time.Hour)
	if err := foreman.Save(foreman.Record{ID: "fm-a", Heartbeat: old.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	b := newResourceBroker()
	b.Now = func() time.Time { return old }
	if _, err := b.Reserve(foreman.ResourceRequest{ForemanID: "fm-a", Ticket: "bs-a", Task: "orphan", Profile: "standard"}, 0); err != nil {
		t.Fatal(err)
	}
	recoveryClient(t)
	out := captureStdout(t, func() {
		if err := foremanWatch([]string{"--once"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "RELEASED_LEASE=") || !strings.Contains(out, "nothing to watch") {
		t.Fatalf("watch exited without recovering orphan: %s", out)
	}
}

func TestSlowLeaseCannotStarveOtherForemanRecovery(t *testing.T) {
	resourceCLIFixture(t)
	b := newResourceBroker()
	for _, task := range []string{"slow", "finished"} {
		if _, err := b.Reserve(foreman.ResourceRequest{ForemanID: "fm-a", Ticket: "bs-a", Task: task, Profile: "plan"}, 0); err != nil {
			t.Fatal(err)
		}
	}
	status, err := b.Status(0)
	if err != nil {
		t.Fatal(err)
	}
	// Force the first lease to consume the whole tick's budget: its Dispatch
	// is open, so reconciliation asks Orca for terminals, which hangs.
	seedTask(t, status.Leases[0].Task, "open", "dispatched", "term_slow", time.Now().Add(-time.Hour))
	seedTask(t, status.Leases[1].Task, "done", "completed", "term_done", time.Now().Add(-time.Hour))
	path := filepath.Join(os.Getenv("BABYSIT_HOME"), "orca-slow")
	stub := `#!/bin/sh
case "$1" in
 status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":[]}}}' ;;
 terminal) exec sleep 30 ;;
esac
`
	if err := os.WriteFile(path, []byte(stub), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCA_CLI_COMMAND", path)
	for tick := 0; tick < 2; tick++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		c, err := orca.PreflightContext(ctx)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		status, err := b.Status(0)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		released := reconcileResourceLeases(ctx, b, status.Leases, c, time.Now())
		cancel()
		if tick == 1 && len(released) != 1 {
			t.Fatalf("slow lease starved recovery again: %v", released)
		}
	}
}
