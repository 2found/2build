package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/2found/2build/internal/foreman"
)

// taskFixture is a Foreman record plus a stub Orca that launches every agent
// into term_new and reports $STUB_AGENT as the agent it detected there.
func taskFixture(t *testing.T) (foremanID, log string) {
	t.Helper()
	home := routeCommandFixture(t, ``)
	log = filepath.Join(home, "orca.log")
	stub := `#!/usr/bin/python3
import json, os, sys
args = sys.argv[1:]
open(` + pyQuote(log) + `, "a").write(" ".join(args) + "\n")
def arg(name):
    return args[args.index(name) + 1] if name in args else ""
def out(result):
    print(json.dumps({"ok": True, "result": result}))
if args[0] == "status":
    out({"runtime": {"reachable": True, "capabilities": []}})
elif args[:2] == ["agent", "launch"]:
    out({"outcome": {"kind": "terminal", "handle": "term_new"}, "prompt": {"outcome": "handed-to-terminal"}})
elif args[:2] == ["terminal", "show"]:
    out({"terminal": {"handle": arg("--terminal"), "agentIdentity": os.environ.get("STUB_AGENT", "codex"), "executionHostId": "local"}})
else:
    out({})
`
	bin := filepath.Join(home, "bin", "orca")
	if err := os.WriteFile(bin, []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := foreman.Save(foreman.Record{ID: "fm-task", Agent: "codex", Heartbeat: foreman.Now()}); err != nil {
		t.Fatal(err)
	}
	return "fm-task", log
}

func createTask(t *testing.T, foremanID string) string {
	t.Helper()
	spec := filepath.Join(t.TempDir(), "spec.md")
	if err := os.WriteFile(spec, []byte("Run autopilot for bs-child.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := foremanTask([]string{"create", foremanID, "--ticket", "bs-child", "--spec-file", spec}); err != nil {
			t.Fatal(err)
		}
	})
	id := strings.TrimPrefix(strings.SplitN(out, "\n", 2)[0], "TASK=")
	if !strings.HasPrefix(id, "task-") {
		t.Fatalf("create output %q", out)
	}
	return id
}

// One Task runs launch → done with no orchestration call: the worker gets a
// short bootstrap prompt (no heartbeat), the launch is verified against the
// terminal Orca shows, and a second worker cannot open while one is live.
func TestForemanTaskLaunchAndReport(t *testing.T) {
	fm, log := taskFixture(t)
	task := createTask(t, fm)
	out := captureStdout(t, func() {
		if err := foremanTask([]string{"launch", task, "--agent", "codex", "--worktree", t.TempDir(), "--model", "m1"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "DISPATCH="+task+".d1\n") || !strings.Contains(out, "TERMINAL=term_new\n") ||
		!strings.Contains(out, "VERIFICATION=matched\n") || !strings.Contains(out, "LAUNCH_HANDOFF=") {
		t.Fatalf("launch output %q", out)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "orchestration") || !strings.Contains(string(calls), "--surface terminal") ||
		!strings.Contains(string(calls), "bbs foreman task show "+task+" --spec") {
		t.Fatalf("orca calls:\n%s", calls)
	}
	tk, _ := foreman.LoadTask(task)
	if d := tk.Current(); d.Terminal != "term_new" || !d.OwnsTerminal || d.Status != foreman.DispatchOpen {
		t.Fatalf("dispatch = %+v", d)
	}
	if err := foremanTask([]string{"launch", task, "--agent", "codex", "--worktree", t.TempDir()}); err == nil || !strings.Contains(err.Error(), "still open") {
		t.Fatalf("second launch err = %v", err)
	}
	if got, d := foreman.DispatchForTerminal("term_new"); d == nil || got.ID != task {
		t.Fatal("open dispatch not found by terminal")
	}

	done := []string{"done", task, "--dispatch", task + ".d1", "--outcome", "succeeded", "--summary", "did it"}
	captureStdout(t, func() {
		if err := foremanTask(done); err != nil {
			t.Fatal(err)
		}
		// A retried report is harmless; a contradicting one is refused.
		if err := foremanTask(done); err != nil {
			t.Fatalf("retried done: %v", err)
		}
	})
	if err := foremanTask([]string{"done", task, "--outcome", "failed", "--summary", "x"}); err == nil {
		t.Fatal("conflicting report accepted")
	}
	if _, d := foreman.DispatchForTerminal("term_new"); d != nil {
		t.Fatal("settled dispatch still blocks its terminal")
	}
	if id, status, _ := taskDispatchState(task); id != task+".d1" || status != foreman.DispatchCompleted {
		t.Fatalf("state = %s %s", id, status)
	}
}

// A terminal running a different agent than requested is a mismatch: the
// launch errors loud and the Dispatch keeps its terminal so Foreman can
// abandon it and close the pane.
func TestForemanTaskLaunchRejectsTheWrongAgent(t *testing.T) {
	fm, _ := taskFixture(t)
	t.Setenv("STUB_AGENT", "claude")
	task := createTask(t, fm)
	var err error
	out := captureStdout(t, func() {
		err = foremanTask([]string{"launch", task, "--agent", "codex", "--worktree", t.TempDir()})
	})
	if err == nil || !strings.Contains(out, "VERIFICATION=mismatch") {
		t.Fatalf("mismatch: %v %q", err, out)
	}
	captureStdout(t, func() {
		if err := foremanTask([]string{"abandon", task, "--reason", "wrong agent"}); err != nil {
			t.Fatal(err)
		}
	})
	tk, _ := foreman.LoadTask(task)
	if d := tk.Current(); d.Status != foreman.DispatchFailed || d.Terminal != "term_new" || !strings.Contains(d.Reason, "wrong agent") {
		t.Fatalf("abandoned dispatch = %+v", d)
	}
}

// Reusing a live session types the bootstrap into it; a terminal this Task's
// earlier launch did not create is never owned (so never closed at settle).
func TestForemanTaskLaunchReusesASession(t *testing.T) {
	fm, log := taskFixture(t)
	task := createTask(t, fm)
	captureStdout(t, func() {
		if err := foremanTask([]string{"launch", task, "--agent", "codex", "--worktree", t.TempDir(), "--terminal", "term_old"}); err != nil {
			t.Fatal(err)
		}
	})
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "agent launch") || !strings.Contains(string(calls), "terminal send --terminal term_old") {
		t.Fatalf("orca calls:\n%s", calls)
	}
	tk, _ := foreman.LoadTask(task)
	if d := tk.Current(); d.Terminal != "term_old" || d.OwnsTerminal {
		t.Fatalf("reused dispatch = %+v", d)
	}
}

func TestForemanTaskAnswerReachesTheAsker(t *testing.T) {
	fm, _ := taskFixture(t)
	task := createTask(t, fm)
	captureStdout(t, func() {
		if err := foremanTask([]string{"launch", task, "--agent", "codex", "--worktree", t.TempDir()}); err != nil {
			t.Fatal(err)
		}
	})
	q, err := foreman.Ask(task, "approve the L plan?")
	if err != nil {
		t.Fatal(err)
	}
	if pending := foreman.PendingQuestions([]string{task}); len(pending) != 1 || pending[0].ID != q.ID {
		t.Fatalf("pending = %+v", pending)
	}
	captureStdout(t, func() {
		if err := foremanTask([]string{"answer", task, "--id", q.ID, "--body", "approved"}); err != nil {
			t.Fatal(err)
		}
	})
	if err := foremanTask([]string{"answer", task, "--id", q.ID, "--body", "again"}); err == nil {
		t.Fatal("second answer accepted")
	}
	out := captureStdout(t, func() {
		if err := foremanTask([]string{"ask", task, "--resume", q.ID}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, `ANSWER="approved"`) || len(foreman.PendingQuestions([]string{task})) != 0 {
		t.Fatalf("resume = %q", out)
	}
}

// seedTask writes a Task with one Dispatch (none when status is "") into the
// store, launched at launched.
func seedTask(t *testing.T, id, dispatch, status, terminal string, launched time.Time) {
	t.Helper()
	task := foreman.Task{ID: id, Foreman: "fm-a", Ticket: "bs-a", CreatedAt: launched.UTC().Format(time.RFC3339)}
	if status != "" {
		task.Dispatches = []foreman.Dispatch{{ID: dispatch, Status: status, Agent: "codex", Terminal: terminal, OwnsTerminal: true, LaunchedAt: launched.UTC().Format(time.RFC3339)}}
	}
	dir := filepath.Join(foreman.TasksDir(), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(task)
	if err := os.WriteFile(filepath.Join(dir, "task.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
