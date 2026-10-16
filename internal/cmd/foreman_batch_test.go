package cmd

import (
	"encoding/json"
	"fmt"
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
	Asked     []string          `json:"asked"`
	Missing   []string          `json:"missing"`
	Released  []string          `json:"released"`
	Broken    []string          `json:"broken"`
	PaneDown  bool              `json:"pane_down"`
	Attempts  map[string]string `json:"attempts,omitempty"`
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
	f.world = batchWorld{Dispatch: map[string]string{}, Exited: []string{}, Asked: []string{}, Missing: []string{}, Released: []string{}, Broken: []string{}, Panes: map[string]string{"term_0": "foreman: idle"}}
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
    return w.get("attempts", {}).get(task, "ctx-" + task)
def fail(msg):
    print(json.dumps({"ok": False, "error": {"code": "runtime_error", "message": msg}}))
    sys.exit(1)
cmd = args[:2]
if args[:1] != ["status"] and arg("--task") in w["broken"]:
    fail("dispatch-show unavailable")
if args[0] == "status":
    out({"runtime": {"reachable": True, "capabilities": ["orchestration.contract.v1"]}})
elif cmd == ["terminal", "list"]:
    out({"terminals": [{"handle": "term_0", "title": "bbs foreman", "connected": True, "worktreePath": "/repo"}]})
elif cmd == ["terminal", "send"] and w["pane_down"] and arg("--terminal") == "term_0":
    fail("terminal not connected")
elif cmd == ["terminal", "read"]:
    out({"terminal": {"tail": w["panes"].get(arg("--terminal"), "").splitlines()}})
elif cmd == ["orchestration", "dispatch-show"]:
    task = arg("--task")
    status = w["dispatch"].get(task)
    out({"dispatch": {"id": ctx(task), "status": status, "run_id": "run-1"} if status else None})
elif cmd == ["orchestration", "check"] and "--peek" in args:
    if w["asked"]:
        out({"messages": [{"id": i, "type": "question"} for i in w["asked"]], "count": len(w["asked"])})
    else:
        out({"messages": [], "count": w["questions"]})
elif cmd == ["orchestration", "check"]:
    out({"deliveryId": "delivery-1", "count": w["mail"]} if w["mail"] else {"count": 0})
elif cmd == ["orchestration", "send"]:
    w["mail"] += 1
    json.dump(w, open(` + pyQuote(f.state) + `, "w"))
    out({"message": {"id": "msg-%d" % w["mail"]}})
elif cmd == ["orchestration", "worker-list"]:
    rows = [{"dispatchId": ctx(t), "dispatchStatus": s,
             "terminalState": "released" if t in w["released"] else "active",
             "agentTerminalHandle": "term_w_" + t,
             "projection": {"liveness": {"verdict": "exited" if ctx(t) in w["exited"] else "live"}}}
            for t, s in w["dispatch"].items() if t not in w["missing"]]
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

// wakes returns the Foreman wake prompts typed since the last sends() call.
func (f *batchFixture) wakes(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, s := range f.sends(t) {
		if strings.HasPrefix(s, "terminal send") && strings.Contains(s, "--terminal term_0") {
			out = append(out, s)
		}
	}
	return out
}

// A dry run of one Foreman project: phase A, phase B waiting on A inside its
// own session, and an L phase C asking for plan review — all handed over as
// one batch. Orca hiccups, a worker row vanishing for a while and an
// unreachable Foreman pane at settlement must not cost an extra wake or lose
// the one owed: Foreman hears each plan question and the settled batch, and
// nothing else.
func TestBatchDryRunWakesOnlyForQuestionsAndTheSettledBatch(t *testing.T) {
	f := newBatchFixture(t, "grok")
	f.start(t, "phase-a", "phase-b", "phase-c")
	now := time.Now()
	at := func(min int) time.Time { return now.Add(time.Duration(min) * time.Minute) }
	var wakes []string
	tick := func(min int) string {
		t.Helper()
		line := f.tick(t, at(min))
		wakes = append(wakes, f.wakes(t)...)
		return line
	}
	busy := func(task string, min int) {
		f.world.Panes["term_w_"+task] = fmt.Sprintf("%s: working (%dm)", task, min)
	}

	// Two hours of work. B sits in `bbs foreman await` (its spinner moves);
	// C asks for its L plan review, is answered, then asks again.
	for min := 0; min <= 120; min += 5 {
		busy("phase-a", min)
		busy("phase-b", min)
		busy("phase-c", min)
		switch min {
		case 10:
			f.world.Asked = []string{"msg-c1"}
		case 20:
			// Foreman answered c1 and C's next question landed within one
			// watcher interval: the unread count never moved.
			f.world.Asked = []string{"msg-c2"}
		case 25:
			f.world.Asked = []string{}
		case 30, 35:
			f.world.Broken = []string{"phase-a"} // dispatch-show failing
		case 40:
			f.world.Broken = []string{}
			f.world.Missing = []string{"phase-b"} // Orca restarted: no row yet
		case 55:
			f.world.Missing = []string{}
		case 60:
			f.world.Dispatch["phase-a"] = "completed"
		case 90:
			f.world.Dispatch["phase-c"] = "completed"
		}
		f.save(t)
		if line := tick(min); strings.Contains(line, "WOKE") && min != 10 && min != 20 {
			t.Fatalf("unexpected wake at %dm: %q", min, line)
		}
	}
	if len(wakes) != 2 || !strings.Contains(wakes[0], "messages:1") || !strings.Contains(wakes[1], "messages:1") {
		t.Fatalf("question wakes = %v", wakes)
	}

	// B settles while the Foreman pane cannot take input: the wake is owed,
	// not dropped, and delivered once the pane answers.
	f.world.Dispatch["phase-b"] = "completed"
	f.world.PaneDown = true
	f.save(t)
	if line := f.tick(t, at(125)); !strings.Contains(line, "UNREACHABLE fm-batch") {
		t.Fatalf("settle with pane down = %q", line)
	}
	f.sends(t) // the refused attempt reached nobody
	if b, _ := loadBatch(f.record.ID); !b.open() {
		t.Fatal("undelivered settle closed the batch")
	}
	f.world.PaneDown = false
	f.save(t)
	if line := tick(126); !strings.Contains(line, "WOKE fm-batch") || !strings.Contains(line, "3 completed") {
		t.Fatalf("settled wake = %q", line)
	}
	if line := tick(127); strings.Contains(line, "WOKE") {
		t.Fatalf("second settled wake: %q", line)
	}
	if len(wakes) != 3 {
		t.Fatalf("Foreman woke %d times, want 3: %v", len(wakes), wakes)
	}
}

// An open Dispatch whose worker never shows up (or vanished for good) would
// hold the batch open forever; after the stall budget it is reported lost,
// once.
func TestBatchReportsALostWorkerAfterTheStallBudget(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b")
	f.world.Missing = []string{"task-a"}
	f.save(t)
	now := time.Now()
	for _, min := range []int{0, 10, 20, 29} {
		f.world.Panes["term_w_task-b"] = fmt.Sprintf("task-b: %d", min)
		f.save(t)
		if line := f.tick(t, now.Add(time.Duration(min)*time.Minute)); line != "" {
			t.Fatalf("lost reported early at %dm: %q", min, line)
		}
	}
	line := f.tick(t, now.Add(30*time.Minute))
	if !strings.Contains(line, "lost:task-a") || strings.Contains(line, "task-b") {
		t.Fatalf("lost wake = %q", line)
	}
	if line := f.tick(t, now.Add(31*time.Minute)); strings.Contains(line, "lost:task-a") {
		t.Fatalf("lost reported twice: %q", line)
	}
	out := captureStdout(t, func() { _ = foremanBatchCommand([]string{"show", f.record.ID}) })
	if !strings.Contains(out, "ATTENTION=lost:task-a") || !strings.Contains(out, "UNSEEN_SINCE=") {
		t.Fatalf("show = %q", out)
	}
}

// A released terminal (or a user takeover) under a Dispatch that never
// reported is the same dead end as a missing row.
func TestBatchTreatsAReleasedTerminalUnderAnOpenDispatchAsLost(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a")
	f.world.Released = []string{"task-a"}
	f.save(t)
	now := time.Now()
	f.tick(t, now)
	if line := f.tick(t, now.Add(30*time.Minute)); !strings.Contains(line, "lost:task-a") {
		t.Fatalf("released open dispatch = %q", line)
	}
}

// Attention whose wake could not be delivered stays owed.
func TestBatchRetriesAnUndeliveredAttentionWake(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b")
	f.world.Exited = []string{"ctx-task-a"}
	f.world.PaneDown = true
	f.save(t)
	now := time.Now()
	if line := f.tick(t, now); !strings.Contains(line, "UNREACHABLE") {
		t.Fatalf("pane down = %q", line)
	}
	f.world.PaneDown = false
	f.save(t)
	if line := f.tick(t, now.Add(time.Minute)); !strings.Contains(line, "WOKE fm-batch") || !strings.Contains(line, "exited:task-a") {
		t.Fatalf("retried wake = %q", line)
	}
}

// The Foreman retries a failed phase in an early wake without re-arming the
// batch: the new attempt reopens the member, so the batch does not settle
// (and the Foreman is not woken) while the retry still runs, and the retry
// owes its own exited wake.
func TestBatchFollowsARetriedDispatch(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b")
	now := time.Now()
	f.world.Dispatch["task-a"] = "failed"
	f.world.Exited = []string{"ctx-task-a"}
	f.save(t)
	f.tick(t, now)
	f.sends(t)

	f.world.Attempts = map[string]string{"task-a": "ctx-task-a-2"}
	f.world.Dispatch["task-a"] = "dispatched"
	f.world.Dispatch["task-b"] = "completed"
	f.world.Panes["term_w_task-a"] = "task-a: retrying"
	f.save(t)
	if line := f.tick(t, now.Add(time.Minute)); line != "" {
		t.Fatalf("retry in flight produced output: %q", line)
	}
	b, _ := loadBatch(f.record.ID)
	if !b.open() || b.Tasks[0].Dispatch != "ctx-task-a-2" || b.Tasks[0].Status != "dispatched" {
		t.Fatalf("retried member = %+v", b.Tasks[0])
	}

	f.world.Exited = []string{"ctx-task-a-2"}
	f.save(t)
	if line := f.tick(t, now.Add(2*time.Minute)); !strings.Contains(line, "exited:task-a") {
		t.Fatalf("retried attempt's exit = %q", line)
	}
}

// A runtime that reports only a count still wakes when it grows.
func TestBatchQuestionCountFallback(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a")
	if !batchNewQuestion(&foremanBatch{Questions: 1}, 2, nil) || batchNewQuestion(&foremanBatch{Questions: 1}, 1, nil) {
		t.Fatal("count fallback")
	}
	if !batchNewQuestion(&foremanBatch{Questions: 1, QuestionIDs: []string{"a"}}, 1, []string{"b"}) {
		t.Fatal("a new id with the same count must wake")
	}
}

// The worker-side check-in only revives a watcher while some batch still owes
// its Foreman a wake.
func TestAnyOpenBatchTracksTheOwedWake(t *testing.T) {
	f := newBatchFixture(t, "grok")
	if anyOpenBatch("") {
		t.Fatal("no batch yet")
	}
	f.start(t, "task-a")
	if !anyOpenBatch("") || !anyOpenBatch(f.record.ID) || anyOpenBatch("fm-other") {
		t.Fatal("open batch not seen")
	}
	f.world.Dispatch["task-a"] = "completed"
	f.save(t)
	f.tick(t, time.Now())
	if anyOpenBatch("") {
		t.Fatal("settled batch still owes a wake")
	}
}

// Tickets launched together reach their L plan reviews close together; the
// attention window turns those questions, and an exit inside the window,
// into one Foreman wake. Nothing is dropped: the held attention is delivered
// when the window closes.
func TestBatchAttentionWindowCoalescesEarlyWakes(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b", "task-c")
	o := batchWatchOpts()
	o.attentionWindow = 2 * time.Minute
	now := time.Now()
	tick := func(min int) string {
		t.Helper()
		for _, task := range []string{"task-a", "task-b", "task-c"} {
			f.world.Panes["term_w_"+task] = fmt.Sprintf("%s: %d", task, min)
		}
		f.save(t)
		return watchTick(f.client, f.record, o, now.Add(time.Duration(min)*time.Minute))
	}
	f.world.Asked = []string{"msg-a"}
	if line := tick(0); strings.Contains(line, "WOKE") {
		t.Fatalf("woke before the window closed: %q", line)
	}
	f.world.Asked = []string{"msg-a", "msg-b"}
	f.world.Exited = []string{"ctx-task-c"}
	if line := tick(1); strings.Contains(line, "WOKE") {
		t.Fatalf("woke before the window closed: %q", line)
	}
	out := captureStdout(t, func() { _ = foremanBatchCommand([]string{"show", f.record.ID}) })
	if !strings.Contains(out, "ATTENTION=exited:task-c") {
		t.Fatalf("held attention not shown: %q", out)
	}
	line := tick(2)
	if !strings.Contains(line, "WOKE") || !strings.Contains(line, "messages:2") || !strings.Contains(line, "exited:task-c") {
		t.Fatalf("coalesced wake = %q", line)
	}
	if n := len(f.wakes(t)); n != 1 {
		t.Fatalf("Foreman woke %d times, want 1", n)
	}
	if line := tick(3); strings.Contains(line, "WOKE") {
		t.Fatalf("delivered attention woke again: %q", line)
	}
}
