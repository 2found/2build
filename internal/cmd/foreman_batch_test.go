package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/orca"
)

// batchWorld is the stub Orca's state, rewritten by tests between ticks:
// Dispatch status per task, the Foreman's unread question count, exited
// dispatch ids, pane text per terminal handle, and how many messages sit in
// the Foreman's Run mailbox (a consuming check opens a Delivery only when
// there is one). The Foreman is term_0 and each worker is term_w_<task>.
type batchWorld struct {
	Dispatch  map[string]string `json:"dispatch"`
	Questions int               `json:"questions"`
	Mail      int               `json:"mail"`
	Exited    []string          `json:"exited"`
	Panes     map[string]string `json:"panes"`
}

type batchFixture struct {
	client *orca.Client
	record foreman.Record
	state  string
	log    string
	world  batchWorld
}

func newBatchFixture(t *testing.T, agentName string) *batchFixture {
	t.Helper()
	dir := t.TempDir()
	f := &batchFixture{state: filepath.Join(dir, "state.json"), log: filepath.Join(dir, "calls.log")}
	f.world = batchWorld{Dispatch: map[string]string{}, Exited: []string{}, Panes: map[string]string{"term_0": "foreman: idle"}}
	f.save(t)
	script := `#!/usr/bin/python3
import json, sys
args = sys.argv[1:]
open(` + pyQuote(f.log) + `, "a").write(" ".join(args) + "\n")
w = json.load(open(` + pyQuote(f.state) + `))
def arg(name):
    return args[args.index(name) + 1] if name in args else ""
def out(result):
    print(json.dumps({"ok": True, "result": result}))
def ctx(task):
    return "ctx-" + task
cmd = args[:2]
if args[0] == "status":
    out({"runtime": {"reachable": True, "capabilities": ["orchestration.contract.v1"]}})
elif cmd == ["terminal", "list"]:
    out({"terminals": [{"handle": "term_0", "title": "bbs foreman", "connected": True, "worktreePath": "/repo"}]})
elif cmd == ["terminal", "read"]:
    out({"terminal": {"tail": w["panes"].get(arg("--terminal"), "").splitlines()}})
elif cmd == ["orchestration", "dispatch-show"]:
    task = arg("--task")
    status = w["dispatch"].get(task)
    out({"dispatch": {"id": ctx(task), "status": status, "run_id": "run-1"} if status else None})
elif cmd == ["orchestration", "check"] and "--peek" in args:
    out({"messages": [], "count": w["questions"]})
elif cmd == ["orchestration", "check"]:
    out({"deliveryId": "delivery-1", "count": w["mail"]} if w["mail"] else {"count": 0})
elif cmd == ["orchestration", "send"]:
    w["mail"] += 1
    json.dump(w, open(` + pyQuote(f.state) + `, "w"))
    out({"message": {"id": "msg-%d" % w["mail"]}})
elif cmd == ["orchestration", "worker-list"]:
    rows = [{"dispatchId": ctx(t), "dispatchStatus": s, "terminalState": "active",
             "agentTerminalHandle": "term_w_" + t,
             "projection": {"liveness": {"verdict": "exited" if ctx(t) in w["exited"] else "live"}}}
            for t, s in w["dispatch"].items()]
    out({"workers": rows, "page": {"hasMore": False}})
else:
    out({})
`
	bin := filepath.Join(dir, "orca")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCA_CLI_COMMAND", bin)
	t.Setenv("BABYSIT_HOME", t.TempDir())
	client, err := orca.Preflight()
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	f.client = client
	f.record = foreman.Record{ID: "fm-batch", WorkspaceTitle: "bbs foreman", Agent: agentName, Heartbeat: foreman.Now()}
	if err := foreman.Save(f.record); err != nil {
		t.Fatal(err)
	}
	return f
}

func pyQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (f *batchFixture) save(t *testing.T) {
	t.Helper()
	// The stub owns the mailbox count (its send appends); keep it.
	var prev batchWorld
	if data, err := os.ReadFile(f.state); err == nil && json.Unmarshal(data, &prev) == nil {
		f.world.Mail = prev.Mail
	}
	data, _ := json.Marshal(f.world)
	if err := os.WriteFile(f.state, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// sends returns every `terminal send` text typed since the last call.
func (f *batchFixture) sends(t *testing.T) []string {
	t.Helper()
	data, _ := os.ReadFile(f.log)
	_ = os.WriteFile(f.log, nil, 0o644)
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "terminal send") || strings.HasPrefix(line, "orchestration send") {
			out = append(out, line)
		}
	}
	return out
}

func (f *batchFixture) start(t *testing.T, tasks ...string) {
	t.Helper()
	for _, task := range tasks {
		f.world.Dispatch[task] = "dispatched"
		f.world.Panes["term_w_"+task] = task + ": working"
	}
	f.save(t)
	captureStdout(t, func() {
		if err := foremanBatchCommand([]string{f.record.ID, "--tasks", strings.Join(tasks, ",")}); err != nil {
			t.Fatal(err)
		}
	})
	f.sends(t)
}

func (f *batchFixture) tick(t *testing.T, now time.Time) string {
	t.Helper()
	return watchTick(f.client, f.record, batchWatchOpts(), now)
}

func batchWatchOpts() watchOpts {
	o := testWatchOpts()
	o.workerIdle, o.workerNudges = 10*time.Minute, 2
	return o
}

func TestBatchStartRejectsUnlaunchedTask(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.world.Dispatch["task-a"] = "dispatched"
	f.save(t)
	err := foremanBatchCommand([]string{f.record.ID, "--tasks", "task-a,task-missing"})
	if err == nil || !strings.Contains(err.Error(), "task-missing has no Dispatch") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := loadBatch(f.record.ID); ok {
		t.Fatal("rejected batch was recorded")
	}
	if err := foremanBatchCommand([]string{f.record.ID}); err == nil || !strings.Contains(err.Error(), "--tasks is required") {
		t.Fatalf("missing tasks: %v", err)
	}
}

// The coordinator hears nothing while members settle one by one, and exactly
// one wake (Orca message + prompt) once the last one does.
func TestBatchWakesForemanOnceWhenAllSettle(t *testing.T) {
	f := newBatchFixture(t, "grok") // no compaction command: wakes directly
	f.start(t, "task-a", "task-b")
	now := time.Now()

	f.world.Dispatch["task-a"] = "completed"
	f.save(t)
	if line := f.tick(t, now); line != "" {
		t.Fatalf("partial batch produced output: %q", line)
	}
	if sends := f.sends(t); len(sends) != 0 {
		t.Fatalf("partial batch woke someone: %v", sends)
	}
	// Hours later the Foreman pane has not moved; the hourly status prompt
	// and the stall nudge stay held while its batch is open. The worker
	// is still busy, so nobody is nudged.
	f.world.Panes["term_w_task-b"] = "task-b: running tests"
	f.save(t)
	if line := f.tick(t, now.Add(3*time.Hour)); line != "" || len(f.sends(t)) != 0 {
		t.Fatalf("held Foreman was prompted: %q", line)
	}

	f.world.Dispatch["task-b"] = "failed"
	f.save(t)
	line := f.tick(t, now.Add(3*time.Hour+time.Minute))
	if !strings.Contains(line, "WOKE fm-batch") || !strings.Contains(line, "1 completed, 1 failed") {
		t.Fatalf("settled wake = %q", line)
	}
	sends := f.sends(t)
	if len(sends) != 2 || !strings.HasPrefix(sends[0], "orchestration send --to run:run-1") ||
		!strings.Contains(sends[1], "bbs foreman batch show fm-batch") {
		t.Fatalf("wake sends = %v", sends)
	}
	b, _ := loadBatch(f.record.ID)
	if b.open() {
		t.Fatal("settled batch still open")
	}
	// Closed: later ticks fall back to the ordinary watcher, quietly.
	if line := f.tick(t, now.Add(3*time.Hour+2*time.Minute)); strings.Contains(line, "WOKE") {
		t.Fatalf("closed batch woke again: %q", line)
	}
}

func TestBatchCompactsBeforeTheSettledWake(t *testing.T) {
	f := newBatchFixture(t, "claude")
	f.start(t, "task-a")
	now := time.Now()
	f.world.Dispatch["task-a"] = "completed"
	f.save(t)
	if line := f.tick(t, now); !strings.Contains(line, "COMPACTING fm-batch") {
		t.Fatalf("expected compaction, got %q", line)
	}
	if sends := f.sends(t); len(sends) != 1 || !strings.Contains(sends[0], "--text /compact Foreman fm-batch batch boundary") {
		t.Fatalf("compact send = %v", sends)
	}
	// Compaction still rendering: the pane changes, so no wake yet.
	f.world.Panes["term_0"] = "compacting..."
	f.save(t)
	if line := f.tick(t, now.Add(time.Minute)); line != "" {
		t.Fatalf("woke during compaction: %q", line)
	}
	// Pane held still for a tick: compaction is done, deliver the wake.
	if line := f.tick(t, now.Add(2*time.Minute)); !strings.Contains(line, "WOKE fm-batch") {
		t.Fatalf("post-compact wake = %q", line)
	}
	if b, _ := loadBatch(f.record.ID); b.open() {
		t.Fatal("batch still open after wake")
	}
}

func TestBatchWakesEarlyForQuestionAndExitedWorker(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b")
	now := time.Now()

	f.world.Questions = 1
	f.save(t)
	if line := f.tick(t, now); !strings.Contains(line, "messages:1") {
		t.Fatalf("question wake = %q", line)
	}
	f.sends(t)
	// The same unread question does not wake again.
	if line := f.tick(t, now.Add(time.Minute)); line != "" {
		t.Fatalf("repeated question wake: %q", line)
	}

	f.world.Exited = []string{"ctx-task-b"}
	f.save(t)
	line := f.tick(t, now.Add(2*time.Minute))
	if !strings.Contains(line, "exited:task-b") || strings.Contains(line, "exited:task-a") {
		t.Fatalf("exited wake = %q", line)
	}
	if line := f.tick(t, now.Add(3*time.Minute)); strings.Contains(line, "exited:task-b") {
		t.Fatalf("exited worker reported twice: %q", line)
	}
	if b, _ := loadBatch(f.record.ID); !b.open() {
		t.Fatal("attention closed the batch")
	}
}

// An idle worker is nudged (not the coordinator) and only after the nudges
// are spent does the coordinator hear about it.
func TestBatchNudgesIdleWorkerThenReportsStall(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a")
	now := time.Now()
	f.tick(t, now) // first sighting starts the worker's idle clock

	nudged := func(at time.Time) {
		t.Helper()
		line := f.tick(t, at)
		sends := f.sends(t)
		if !strings.Contains(line, "NUDGED-WORKER task-a") || len(sends) != 1 ||
			!strings.Contains(sends[0], "--terminal term_w_task-a") || !strings.Contains(sends[0], "worker_done") {
			t.Fatalf("nudge at %s: line=%q sends=%v", at.Sub(now), line, sends)
		}
	}
	if line := f.tick(t, now.Add(5*time.Minute)); line != "" {
		t.Fatalf("nudged before idle threshold: %q", line)
	}
	nudged(now.Add(11 * time.Minute))
	// The nudge's own echo is not progress and does not refund the budget.
	f.world.Panes["term_w_task-a"] = "task-a: working\n> Continue Dispatch"
	f.save(t)
	f.tick(t, now.Add(12*time.Minute))
	nudged(now.Add(23 * time.Minute))

	line := f.tick(t, now.Add(34*time.Minute))
	if !strings.Contains(line, "stalled:task-a") || !strings.Contains(line, "WOKE fm-batch") {
		t.Fatalf("stall report = %q", line)
	}
	if line := f.tick(t, now.Add(45*time.Minute)); line != "" {
		t.Fatalf("stalled worker poked again: %q", line)
	}
}

func TestBatchRestartKeepsNudgeStateForSameDispatch(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a")
	b, _ := loadBatch(f.record.ID)
	b.Tasks[0].Nudges = 1
	if err := saveBatch(f.record.ID, b); err != nil {
		t.Fatal(err)
	}
	f.start(t, "task-a", "task-b")
	b, _ = loadBatch(f.record.ID)
	if len(b.Tasks) != 2 || b.Tasks[0].Nudges != 1 || b.Tasks[1].Nudges != 0 {
		t.Fatalf("re-armed batch = %+v", b.Tasks)
	}
	out := captureStdout(t, func() {
		if err := foremanBatchCommand([]string{"show", f.record.ID}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "BATCH_STATE=open\n") || !strings.Contains(out, "BATCH_SETTLED=0/2\n") ||
		!strings.Contains(out, "TASK=task-a DISPATCH=ctx-task-a STATUS=dispatched NUDGES=1\n") {
		t.Fatalf("show = %q", out)
	}
}

// Orca types a pointer into an idle coordinator for every arriving message
// unless its mailbox has an unacknowledged Delivery. Handing a batch over
// therefore opens one — seeding the empty mailbox with the batch notice — and
// each tick only re-asserts it, without adding mail while it is held.
func TestBatchHoldsTheForemanMailboxOpen(t *testing.T) {
	f := newBatchFixture(t, "grok")
	f.world.Dispatch["a"] = "dispatched"
	f.world.Panes["term_w_a"] = "a: working"
	f.save(t)
	captureStdout(t, func() {
		if err := foremanBatchCommand([]string{f.record.ID, "--tasks", "a"}); err != nil {
			t.Fatal(err)
		}
	})
	data, _ := os.ReadFile(f.log)
	var holds, sends []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "orchestration check --run run-1 --terminal term_0") {
			holds = append(holds, line)
		}
		if strings.HasPrefix(line, "orchestration send") {
			sends = append(sends, line)
		}
	}
	if len(holds) != 2 || len(sends) != 1 || !strings.Contains(sends[0], "--to run:run-1") {
		t.Fatalf("empty mailbox not seeded then held: holds=%v sends=%v", holds, sends)
	}
	_ = os.WriteFile(f.log, nil, 0o644)
	now := time.Now()
	for i := 0; i < 3; i++ {
		f.tick(t, now.Add(time.Duration(i)*time.Minute))
	}
	data, _ = os.ReadFile(f.log)
	if strings.Contains(string(data), "orchestration send") || strings.Count(string(data), "orchestration check --run run-1 --terminal term_0 --json") != 3 {
		t.Fatalf("held mailbox should be re-asserted once per tick without new mail:\n%s", data)
	}
}
