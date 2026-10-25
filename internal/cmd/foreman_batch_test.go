package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/orca"
)

// batchWorld is the test's view of the bbs task store and the stub Orca,
// rewritten by tests between ticks: Dispatch status per task ("" = created,
// never launched), pending questions (question id → task), exited dispatch
// ids, and pane text per terminal handle. The Foreman is term_0 and each
// worker is term_w_<task>; save() writes the tasks into $BABYSIT_HOME/tasks.
type batchWorld struct {
	Dispatch map[string]string `json:"dispatch"`
	Asked    map[string]string `json:"asked"`
	// Missing tasks have no terminal in `terminal list`; Broken tasks have
	// an unreadable task.json.
	Missing  []string          `json:"missing"`
	Broken   []string          `json:"broken"`
	PaneDown bool              `json:"pane_down"`
	Attempts map[string]string `json:"attempts,omitempty"`
	Exited   []string          `json:"exited"`
	Panes    map[string]string `json:"panes"`
	// Extra are other tasks in the store (not in the batch), NoTerm lists
	// tasks whose Dispatch recorded no terminal, and CloseFails makes
	// `terminal close` error.
	Extra      []extraTask `json:"extra,omitempty"`
	NoTerm     []string    `json:"noterm,omitempty"`
	CloseFails bool        `json:"close_fails,omitempty"`
	// Terminals is the derived `terminal list` the stub serves.
	Terminals []map[string]any `json:"terminals"`
}

type extraTask struct {
	Task, Dispatch, Status, Terminal string
}

type batchFixture struct {
	client   *orca.Client
	record   foreman.Record
	state    string
	log      string
	launched string
	world    batchWorld
}

func newBatchFixture(t *testing.T, agentName string) *batchFixture {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("BABYSIT_HOME", t.TempDir())
	f := &batchFixture{state: filepath.Join(dir, "state.json"), log: filepath.Join(dir, "calls.log"),
		launched: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}
	f.world = batchWorld{Dispatch: map[string]string{}, Asked: map[string]string{}, Exited: []string{}, Missing: []string{}, Broken: []string{}, Panes: map[string]string{"term_0": "foreman: idle"}}
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
def fail(msg):
    print(json.dumps({"ok": False, "error": {"code": "runtime_error", "message": msg}}))
    sys.exit(1)
cmd = args[:2]
if args[0] == "status":
    out({"runtime": {"reachable": True, "capabilities": []}})
elif cmd == ["terminal", "list"]:
    out({"terminals": w["terminals"]})
elif cmd == ["terminal", "send"] and w["pane_down"] and arg("--terminal") == "term_0":
    fail("terminal not connected")
elif cmd == ["terminal", "close"] and w.get("close_fails"):
    fail("close failed")
elif cmd == ["terminal", "read"]:
    out({"terminal": {"tail": w["panes"].get(arg("--terminal"), "").splitlines()}})
else:
    out({})
`
	bin := filepath.Join(dir, "orca")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCA_CLI_COMMAND", bin)
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

func (f *batchFixture) ctx(task string) string {
	if id := f.world.Attempts[task]; id != "" {
		return id
	}
	return "ctx-" + task
}

// writeTask replaces one task's record in the bbs store.
func (f *batchFixture) writeTask(t *testing.T, id, dispatch, status, terminal string) {
	t.Helper()
	task := foreman.Task{ID: id, Foreman: f.record.ID, Ticket: "T-" + id, CreatedAt: f.launched}
	if status != "" {
		task.Dispatches = []foreman.Dispatch{{ID: dispatch, Status: status, Agent: "codex", Terminal: terminal, OwnsTerminal: true, LaunchedAt: f.launched}}
	}
	dir := filepath.Join(foreman.TasksDir(), id)
	if err := os.MkdirAll(filepath.Join(dir, "questions"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(task)
	if slices.Contains(f.world.Broken, id) {
		data = []byte("{not json")
	}
	if err := os.WriteFile(filepath.Join(dir, "task.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *batchFixture) save(t *testing.T) {
	t.Helper()
	terms := []map[string]any{{"handle": "term_0", "title": "bbs foreman", "connected": true, "worktreePath": "/repo", "agentIdentity": f.record.Agent}}
	for task, status := range f.world.Dispatch {
		handle := "term_w_" + task
		if slices.Contains(f.world.NoTerm, task) {
			handle = ""
		}
		f.writeTask(t, task, f.ctx(task), status, handle)
		_ = os.RemoveAll(filepath.Join(foreman.TasksDir(), task, "questions"))
		if handle != "" && status != "" && !slices.Contains(f.world.Missing, task) {
			identity := "codex"
			if slices.Contains(f.world.Exited, f.ctx(task)) {
				identity = ""
			}
			terms = append(terms, map[string]any{"handle": handle, "connected": true, "agentIdentity": identity})
		}
	}
	for _, x := range f.world.Extra {
		f.writeTask(t, x.Task, x.Dispatch, x.Status, x.Terminal)
	}
	for qid, task := range f.world.Asked {
		q := foreman.Question{ID: qid, Task: task, Dispatch: f.ctx(task), Body: "plan review?", AskedAt: f.launched}
		dir := filepath.Join(foreman.TasksDir(), task, "questions")
		_ = os.MkdirAll(dir, 0o755)
		data, _ := json.Marshal(q)
		if err := os.WriteFile(filepath.Join(dir, qid+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f.world.Terminals = terms
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
		if strings.HasPrefix(line, "terminal send") {
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

func (f *batchFixture) tickClosing(t *testing.T, now time.Time) string {
	t.Helper()
	o := batchWatchOpts()
	o.keepSettled = false
	return watchTick(f.client, f.record, o, now)
}

func (f *batchFixture) tick(t *testing.T, now time.Time) string {
	t.Helper()
	return watchTick(f.client, f.record, batchWatchOpts(), now)
}

func batchWatchOpts() watchOpts {
	o := testWatchOpts()
	o.workerIdle, o.workerNudges = 10*time.Minute, 2
	// Settled-worker closing has its own tests; the rest assert on quiet ticks.
	o.keepSettled = true
	return o
}

func TestBatchStartRejectsUnlaunchedTask(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.world.Dispatch["task-a"] = "dispatched"
	f.world.Dispatch["task-new"] = ""
	f.save(t)
	err := foremanBatchCommand([]string{f.record.ID, "--tasks", "task-a,task-new"})
	if err == nil || !strings.Contains(err.Error(), "task-new has no Dispatch") {
		t.Fatalf("err = %v", err)
	}
	err = foremanBatchCommand([]string{f.record.ID, "--tasks", "task-a,task-missing"})
	if err == nil || !strings.Contains(err.Error(), "task-missing has no Dispatch") {
		t.Fatalf("unknown task err = %v", err)
	}
	if _, ok := loadBatch(f.record.ID); ok {
		t.Fatal("rejected batch was recorded")
	}
	if err := foremanBatchCommand([]string{f.record.ID}); err == nil || !strings.Contains(err.Error(), "--tasks is required") {
		t.Fatalf("missing tasks: %v", err)
	}
}

// The coordinator hears nothing while members settle one by one, and exactly
// one wake prompt once the last one does.
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
	if len(sends) != 1 || !strings.Contains(sends[0], "--terminal term_0") ||
		!strings.Contains(sends[0], "bbs foreman batch show fm-batch") {
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

func TestBatchWakesEarlyForQuestionAndExitedWorker(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b")
	now := time.Now()

	f.world.Asked = map[string]string{"q-a1": "task-a"}
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
			!strings.Contains(sends[0], "--terminal term_w_task-a") || !strings.Contains(sends[0], "bbs foreman task done task-a") {
			t.Fatalf("nudge at %s: line=%q sends=%v", at.Sub(now), line, sends)
		}
	}
	if line := f.tick(t, now.Add(5*time.Minute)); line != "" {
		t.Fatalf("nudged before idle threshold: %q", line)
	}
	nudged(now.Add(11 * time.Minute))
	// The nudge's own echo is not progress and does not refund the budget.
	f.world.Panes["term_w_task-a"] = "task-a: working\n> Continue Task"
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
// one batch. An unreadable task record, a worker terminal vanishing for a while and an
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
			f.world.Asked = map[string]string{"msg-c1": "phase-c"}
		case 20:
			// Foreman answered c1 and C's next question landed within one
			// watcher interval: the unread count never moved.
			f.world.Asked = map[string]string{"msg-c2": "phase-c"}
		case 25:
			f.world.Asked = map[string]string{}
		case 30, 35:
			f.world.Broken = []string{"phase-a"} // task record unreadable
		case 40:
			f.world.Broken = []string{}
			f.world.Missing = []string{"phase-b"} // Orca restarted: no terminal yet
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

// A Dispatch that recorded no terminal can never report either: it is lost
// on the same budget as a vanished one.
func TestBatchTreatsAnOpenDispatchWithoutTerminalAsLost(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a")
	f.world.NoTerm = []string{"task-a"}
	f.save(t)
	now := time.Now()
	f.tick(t, now)
	if line := f.tick(t, now.Add(30*time.Minute)); !strings.Contains(line, "lost:task-a") {
		t.Fatalf("terminal-less open dispatch = %q", line)
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
	f.world.Asked = map[string]string{"msg-a": "task-a"}
	if line := tick(0); strings.Contains(line, "WOKE") {
		t.Fatalf("woke before the window closed: %q", line)
	}
	f.world.Asked = map[string]string{"msg-a": "task-a", "msg-b": "task-b"}
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

// closes returns every `terminal close` call logged so far.
func (f *batchFixture) closes(t *testing.T) []string {
	t.Helper()
	data, _ := os.ReadFile(f.log)
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "terminal close") {
			out = append(out, line)
		}
	}
	return out
}

// A settled member's agent terminal closes at once; running members, other
// Tasks, the Foreman, the caller and a terminal shared with an open Dispatch
// stay.
func TestBatchClosesSettledWorkerTerminalsOnly(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b", "task-c", "task-d")
	t.Setenv("ORCA_TERMINAL_HANDLE", "term_w_task-d")
	f.world.Dispatch["task-a"] = "completed"
	f.world.Dispatch["task-c"] = "failed"
	f.world.Dispatch["task-d"] = "completed" // its terminal is the caller's own
	f.world.Extra = []extraTask{
		{"task-foreign", "ctx-foreign", "completed", "term_foreign"},
		{"task-reuse", "ctx-reuse", "dispatched", "term_w_task-c"}, // shared with an open Dispatch
	}
	f.save(t)
	line := f.tickClosing(t, time.Now())
	if !strings.Contains(line, "CLOSED-WORKER task-a (term_w_task-a)") {
		t.Fatalf("line = %q", line)
	}
	closes := f.closes(t)
	if len(closes) != 1 || !strings.Contains(closes[0], "--terminal term_w_task-a") {
		t.Fatalf("closes = %v", closes)
	}
	b, _ := loadBatch(f.record.ID)
	if !b.Tasks[0].Closed || b.Tasks[1].Closed {
		t.Fatalf("closed flags = %+v", b.Tasks)
	}
	// Done once: a later tick does not close it again.
	f.tickClosing(t, time.Now().Add(time.Minute))
	if n := len(f.closes(t)); n != 1 {
		t.Fatalf("closed again: %d calls", n)
	}
}

func TestBatchCloseSkipsMissingHandleAndSurvivesOrcaError(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b")
	f.world.Dispatch["task-a"] = "completed"
	f.world.Dispatch["task-b"] = "completed"
	f.world.NoTerm = []string{"task-a"}
	f.world.CloseFails = true
	f.save(t)
	line := f.tickClosing(t, time.Now())
	if !strings.Contains(line, "CLOSE-WORKER-FAILED task-b") || !strings.Contains(line, "WOKE fm-batch") {
		t.Fatalf("a close error blocked the wake: %q", line)
	}
	if closes := f.closes(t); len(closes) != 1 || strings.Contains(closes[0], "task-a") {
		t.Fatalf("closes = %v", closes)
	}
}

func TestBatchKeepSettledLeavesTerminalOpen(t *testing.T) {
	f := newBatchFixture(t, "codex")
	f.start(t, "task-a", "task-b")
	f.world.Dispatch["task-a"] = "completed"
	f.save(t)
	f.tick(t, time.Now())
	if closes := f.closes(t); len(closes) != 0 {
		t.Fatalf("closes = %v", closes)
	}
}
