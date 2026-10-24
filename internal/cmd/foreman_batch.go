package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/2found/2build/internal/agent"
	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/orca"
)

const foremanBatchUsage = `Usage:
  bbs foreman batch <foreman-id> --tasks <task,task,...>
  bbs foreman batch show <foreman-id>

Hands a dispatched batch to the detached watcher, after which the Foreman ends
its turn. The watcher supervises the workers (nudging one whose pane sits idle
with its Dispatch open) and wakes the Foreman, by a prompt in its terminal, only
when the whole batch has settled or cannot settle without it: an unanswered
worker question, an exited worker, a worker that stayed idle through its
nudges, or an open Dispatch with no live worker for as long (lost). batch show
prints each pending question with its answer command. A failed member the Foreman retries reopens on its new Dispatch. Re-running
batch replaces the watched set, keeping nudge state.
`

// Every coordinator wake re-reads its whole context, which dominates token
// cost in a wide fan-out. The watcher therefore owns waiting and worker
// liveness, and a batch costs the coordinator one wake, not one per worker or
// per timeout.

type foremanBatch struct {
	ID        string        `json:"id"`
	CreatedAt string        `json:"created_at"`
	Tasks     []batchMember `json:"tasks"`
	// Questions is the unanswered worker question count last seen, and
	// QuestionIDs their ids: a wake is owed for an id not seen before, so a
	// second question arriving after the first was answered still wakes.
	Questions   int      `json:"questions,omitempty"`
	QuestionIDs []string `json:"question_ids,omitempty"`
	// Reported holds the attention keys (exited:<task>, stalled:<task>,
	// lost:<task>) already delivered, so each wakes the Foreman once per
	// Dispatch attempt.
	Reported []string `json:"reported,omitempty"`
	// Owed is attention seen but not yet delivered, held since OwedSince for
	// the attention window so close-together attention shares one wake.
	Owed      []string `json:"owed,omitempty"`
	OwedSince string   `json:"owed_since,omitempty"`
	// WokeAt closes the batch: the settled wake was delivered.
	WokeAt string `json:"woke_at,omitempty"`
}

type batchMember struct {
	Task     string `json:"task"`
	Dispatch string `json:"dispatch,omitempty"`
	Status   string `json:"status,omitempty"`
	// Fingerprint/Since are the worker pane's idle clock; Pending marks a
	// nudge whose own echo has not been seen, as in watchState.
	Fingerprint string `json:"fingerprint,omitempty"`
	Since       string `json:"since,omitempty"`
	Nudges      int    `json:"nudges,omitempty"`
	Pending     bool   `json:"pending,omitempty"`
	Stalled     bool   `json:"stalled,omitempty"`
	// Unseen is when the watcher last lost sight of this open Dispatch: its
	// state unreadable, or no live worker terminal for it. A member unseen
	// for the whole stall budget is reported lost instead of waited on
	// forever — nothing else would ever settle it.
	Unseen string `json:"unseen,omitempty"`
	// Closed is set once the watcher has dealt with this settled Dispatch's
	// agent terminal (closed it, or found nothing to close).
	Closed bool `json:"closed,omitempty"`
}

func batchPath(id string) string { return filepath.Join(watchDir(), id+".batch.json") }

func loadBatch(id string) (foremanBatch, bool) {
	var b foremanBatch
	data, err := os.ReadFile(batchPath(id))
	if err != nil || json.Unmarshal(data, &b) != nil || b.ID == "" {
		return foremanBatch{}, false
	}
	return b, true
}

func saveBatch(id string, b foremanBatch) error {
	if err := os.MkdirAll(watchDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp := batchPath(id) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, batchPath(id))
}

func (b foremanBatch) open() bool { return b.WokeAt == "" }

func (b foremanBatch) settled() int {
	n := 0
	for _, m := range b.Tasks {
		if dispatchTerminal(m.Status) {
			n++
		}
	}
	return n
}

func (b foremanBatch) reported(key string) bool {
	for _, k := range b.Reported {
		if k == key {
			return true
		}
	}
	return false
}

// forget drops a task's delivered attention when it gets a new Dispatch: the
// retried attempt owes its own exited/stalled/lost wake.
func (b *foremanBatch) forget(task string) {
	kept := b.Reported[:0]
	for _, k := range b.Reported {
		if _, t, _ := strings.Cut(k, ":"); t != task {
			kept = append(kept, k)
		}
	}
	b.Reported = kept
	owed := b.Owed[:0]
	for _, k := range b.Owed {
		if _, t, _ := strings.Cut(k, ":"); t != task {
			owed = append(owed, k)
		}
	}
	b.Owed = owed
}

// batchFinal is a Dispatch no retry reopens. A failed attempt is settled for
// the batch, but it is still re-read: when the Foreman retries the task, the
// new attempt reopens the member instead of leaving it unsupervised.
func batchFinal(status string) bool { return status == "completed" || status == "circuit_broken" }

// batchLostAfter is how long an open Dispatch may go unobserved: the same
// budget a silent worker gets (idle clock plus every nudge).
func batchLostAfter(o watchOpts) time.Duration {
	return o.workerIdle * time.Duration(o.workerNudges+1)
}

func foremanBatchCommand(args []string) error {
	if len(args) > 0 && args[0] == "show" {
		if len(args) != 2 {
			return fmt.Errorf("foreman batch show: needs a foreman id\n%s", foremanBatchUsage)
		}
		b, ok := loadBatch(args[1])
		if !ok {
			return fmt.Errorf("foreman batch show: no batch recorded for %s", args[1])
		}
		printBatchState(b)
		printForemanNext(args[1])
		return nil
	}
	id, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("foreman batch: needs a foreman id\n%s", foremanBatchUsage)
	}
	r, err := foreman.Load(id)
	if err != nil {
		return fmt.Errorf("foreman batch: %w", err)
	}
	var tasks []string
	seen := map[string]bool{}
	for _, task := range strings.Split(kv["tasks"], ",") {
		if task = strings.TrimSpace(task); task != "" && !seen[task] {
			seen[task] = true
			tasks = append(tasks, task)
		}
	}
	if len(tasks) == 0 {
		return fmt.Errorf("foreman batch: --tasks is required\n%s", foremanBatchUsage)
	}
	c, err := orca.Preflight()
	if err != nil {
		return fmt.Errorf("foreman batch: %w", err)
	}
	r = rebindForemanToCurrent(c, r)
	prev, _ := loadBatch(id)
	carried := map[string]batchMember{}
	if prev.open() {
		for _, m := range prev.Tasks {
			carried[m.Task] = m
		}
	}
	now := time.Now().UTC()
	b := foremanBatch{ID: fmt.Sprintf("%s-%d", id, now.Unix()), CreatedAt: now.Format(time.RFC3339)}
	for _, task := range tasks {
		dispatch, status, err := taskDispatchState(task)
		if err != nil {
			return fmt.Errorf("foreman batch: task %s: %w", task, err)
		}
		if dispatch == "" {
			return fmt.Errorf("foreman batch: task %s has no Dispatch; launch it before handing it over", task)
		}
		m := carried[task]
		if m.Dispatch != dispatch {
			m = batchMember{}
		}
		m.Task, m.Dispatch, m.Status = task, dispatch, status
		b.Tasks = append(b.Tasks, m)
	}
	if err := saveBatch(id, b); err != nil {
		return fmt.Errorf("foreman batch: %w", err)
	}
	spawnWatcher()
	ensureWatchSupervisor()
	fmt.Println("BATCH=" + b.ID)
	fmt.Printf("BATCH_TASKS=%d\n", len(b.Tasks))
	fmt.Println("NEXT=end this turn; bbs foreman watch wakes this Foreman when the batch settles or needs attention")
	return nil
}

func printBatchState(b foremanBatch) {
	state := "open"
	if !b.open() {
		state = "settled"
	}
	fmt.Println("BATCH=" + b.ID)
	fmt.Println("BATCH_STATE=" + state)
	fmt.Printf("BATCH_SETTLED=%d/%d\n", b.settled(), len(b.Tasks))
	for _, m := range b.Tasks {
		status := m.Status
		if status == "" {
			status = "unknown"
		}
		line := fmt.Sprintf("TASK=%s DISPATCH=%s STATUS=%s", m.Task, m.Dispatch, status)
		if m.Nudges > 0 {
			line += fmt.Sprintf(" NUDGES=%d", m.Nudges)
		}
		if m.Stalled {
			line += " STALLED=1"
		}
		if m.Unseen != "" {
			line += " UNSEEN_SINCE=" + m.Unseen
		}
		if m.Closed {
			line += " WORKER_CLOSED=1"
		}
		fmt.Println(line)
	}
	for _, key := range append(slices.Clone(b.Reported), b.Owed...) {
		fmt.Println("ATTENTION=" + key)
	}
	if b.Questions > 0 {
		fmt.Printf("ATTENTION=messages:%d\n", b.Questions)
	}
	for _, q := range foreman.PendingQuestions(b.taskIDs()) {
		fmt.Printf("QUESTION=%s TASK=%s ASKED=%s BODY=%s\n", q.ID, q.Task, q.AskedAt, strconv.Quote(q.Body))
		fmt.Printf("ANSWER_WITH=bbs foreman task answer %s --id %s --body \"<answer>\"\n", q.Task, q.ID)
	}
}

func (b foremanBatch) taskIDs() []string {
	ids := make([]string, 0, len(b.Tasks))
	for _, m := range b.Tasks {
		ids = append(ids, m.Task)
	}
	return ids
}

// batchTick is one watcher pass over a Foreman's open batch. It returns the
// lines worth printing; a batch that is simply running prints nothing.
func batchTick(client *orca.Client, r foreman.Record, b *foremanBatch, o watchOpts, now time.Time) []string {
	stamp := now.UTC().Format(time.RFC3339)
	for i := range b.Tasks {
		m := &b.Tasks[i]
		if batchFinal(m.Status) {
			continue
		}
		dispatch, status, err := taskDispatchState(m.Task)
		if err != nil || dispatch == "" {
			if !dispatchTerminal(m.Status) && m.Unseen == "" {
				m.Unseen = stamp
			}
			continue
		}
		if dispatch != m.Dispatch {
			b.forget(m.Task)
			*m = batchMember{Task: m.Task}
		}
		m.Dispatch, m.Status = dispatch, status
	}
	closeLines := batchCloseSettled(client, r, b, o)
	if b.settled() == len(b.Tasks) {
		return append(closeLines, batchFinish(client, r, b, now))
	}
	lines, attention := closeLines, []string(nil)
	var questionIDs []string
	for _, q := range foreman.PendingQuestions(b.taskIDs()) {
		questionIDs = append(questionIDs, q.ID)
	}
	questions := len(questionIDs)
	if batchNewQuestion(b, questions, questionIDs) {
		attention = append(attention, fmt.Sprintf("messages:%d", questions))
	}
	terms, termsErr := client.Terminals()
	for i := range b.Tasks {
		m := &b.Tasks[i]
		if dispatchTerminal(m.Status) {
			m.Unseen = ""
			continue
		}
		if termsErr != nil {
			// Orca unreachable: nothing is observed this tick, nothing lost.
			continue
		}
		var d *foreman.Dispatch
		if t, err := foreman.LoadTask(m.Task); err == nil {
			d = t.DispatchByID(m.Dispatch)
		}
		w, ok := taskWorkerFor(d, terms, now)
		if !ok {
			// No terminal (Orca restarted, the worker never started, it was
			// closed): nothing here can settle this Dispatch, so bound the
			// wait.
			if m.Unseen == "" {
				m.Unseen = stamp
			}
			if since, err := time.Parse(time.RFC3339, m.Unseen); err == nil && now.Sub(since) >= batchLostAfter(o) {
				if key := "lost:" + m.Task; !b.reported(key) {
					attention = append(attention, key)
				}
			}
			continue
		}
		m.Unseen = ""
		if w.Exited {
			if key := "exited:" + m.Task; !b.reported(key) {
				attention = append(attention, key)
			}
			continue
		}
		if line, key := batchWatchWorker(client, m, w.Handle, o, now); line != "" || key != "" {
			if line != "" {
				lines = append(lines, line)
			}
			if key != "" && !b.reported(key) {
				attention = append(attention, key)
			}
		}
	}
	question := false
	for _, key := range attention {
		if strings.HasPrefix(key, "messages:") {
			question = true
		} else if !slices.Contains(b.Owed, key) {
			b.Owed = append(b.Owed, key)
		}
	}
	if len(b.Owed) > 0 || question {
		if b.OwedSince == "" {
			b.OwedSince = stamp
		}
		if since, err := time.Parse(time.RFC3339, b.OwedSince); err == nil && now.Sub(since) < o.attentionWindow {
			// Held, not dropped: the question stays unseen and Owed keeps
			// the rest until the window closes or the batch settles.
			return lines
		}
		attention = slices.Clone(b.Owed)
		if question {
			attention = append(attention, fmt.Sprintf("messages:%d", questions))
		}
		sort.Strings(attention)
		summary := fmt.Sprintf("batch %s needs you (%d/%d settled): %s", b.ID, b.settled(), len(b.Tasks), strings.Join(attention, ", "))
		line := batchWake(client, r, *b, summary)
		lines = append(lines, line)
		if strings.HasPrefix(line, "UNREACHABLE") {
			// Undelivered: keep every key owed, so the next tick retries the
			// same wake rather than dropping it.
			return lines
		}
		b.Reported = append(b.Reported, b.Owed...)
		b.Owed, b.OwedSince = nil, ""
	}
	b.Questions, b.QuestionIDs = questions, questionIDs
	return lines
}

// batchNewQuestion reports whether the pending question set holds one the
// Foreman has not been woken for.
func batchNewQuestion(b *foremanBatch, n int, ids []string) bool {
	if len(ids) == 0 {
		return n > b.Questions
	}
	seen := map[string]bool{}
	for _, id := range b.QuestionIDs {
		seen[id] = true
	}
	for _, id := range ids {
		if !seen[id] {
			return true
		}
	}
	return false
}

// batchWatchWorker advances one worker's idle clock. A pane unchanged for
// --worker-idle with its Dispatch still open gets a continue/report nudge; once
// the nudges are spent it is reported as stalled instead of poked forever.
func batchWatchWorker(client *orca.Client, m *batchMember, handle string, o watchOpts, now time.Time) (line, attentionKey string) {
	pane, err := client.ReadHandle(handle, o.lines)
	if err != nil {
		return "", ""
	}
	fp := paneFingerprint(pane)
	stamp := now.UTC().Format(time.RFC3339)
	if fp != m.Fingerprint {
		if m.Pending {
			// First change after a nudge is most likely its own echo.
			m.Pending = false
		} else {
			m.Nudges, m.Stalled = 0, false
		}
		m.Fingerprint, m.Since = fp, stamp
		return "", ""
	}
	since, err := time.Parse(time.RFC3339, m.Since)
	if err != nil {
		m.Since = stamp
		return "", ""
	}
	if now.Sub(since) < o.workerIdle {
		return "", ""
	}
	if m.Nudges >= o.workerNudges {
		if m.Stalled {
			return "", ""
		}
		m.Stalled = true
		return "", "stalled:" + m.Task
	}
	nudge := fmt.Sprintf("Continue Task %s (Dispatch %s). If it is complete, persist the evidence and run `bbs foreman task done %s --dispatch %s` now; if you are blocked on a decision, use `bbs foreman task ask %s`.", m.Task, m.Dispatch, m.Task, m.Dispatch, m.Task)
	if err := client.SendEnterHandle(handle, nudge); err != nil {
		return fmt.Sprintf("WORKER-UNREACHABLE %s — %v", m.Task, err), ""
	}
	m.Nudges++
	m.Pending = true
	m.Since = stamp
	return fmt.Sprintf("NUDGED-WORKER %s (%d/%d) after %s idle", m.Task, m.Nudges, o.workerNudges, roundMin(o.workerIdle)), ""
}

func batchFinish(client *orca.Client, r foreman.Record, b *foremanBatch, now time.Time) string {
	counts := map[string]int{}
	for _, m := range b.Tasks {
		counts[m.Status]++
	}
	var parts []string
	for _, status := range []string{"completed", "failed", "circuit_broken"} {
		if counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[status], status))
		}
	}
	summary := fmt.Sprintf("batch %s settled (%s)", b.ID, strings.Join(parts, ", "))
	line := batchWake(client, r, *b, summary)
	if !strings.HasPrefix(line, "UNREACHABLE") {
		b.WokeAt = now.UTC().Format(time.RFC3339)
	}
	return line
}

// batchWake starts the Foreman's turn with one prompt naming the batch.
func batchWake(client *orca.Client, r foreman.Record, b foremanBatch, summary string) string {
	instruction := summary + ". Run `bbs foreman batch show " + r.ID + "` (batch, attention, pending questions and each project's next steps)."
	if _, err := watchSend(client, r, instruction); err != nil {
		return fmt.Sprintf("UNREACHABLE %s — %s", r.ID, err)
	}
	return fmt.Sprintf("WOKE %s — %s", r.ID, summary)
}

func batchAgent(r foreman.Record) string {
	if r.Agent == "" {
		return agent.Default
	}
	return r.Agent
}
