package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
with its Dispatch open) and wakes the Foreman, by Orca message plus a prompt in
its terminal, only when the whole batch has settled or cannot settle without
it: an unread question/escalation, an exited worker, a worker that stayed
idle through its nudges, or an open Dispatch with no live worker for as long
(lost). A failed member the Foreman retries reopens on its new Dispatch. A
settled batch is compacted first when the agent supports it. Re-running batch
replaces the watched set, keeping nudge state.
`

// Every coordinator wake re-reads its whole context, which dominates token
// cost in a wide fan-out. The watcher therefore owns waiting and worker
// liveness, and a batch costs the coordinator one wake, not one per worker or
// per timeout.

type foremanBatch struct {
	ID        string        `json:"id"`
	CreatedAt string        `json:"created_at"`
	Tasks     []batchMember `json:"tasks"`
	// Questions is the unread question/escalation count last seen, and
	// QuestionIDs those rows' ids: a wake is owed for an id not seen before,
	// so a second question arriving after the first was read still wakes.
	Questions   int      `json:"questions,omitempty"`
	QuestionIDs []string `json:"question_ids,omitempty"`
	// Reported holds the attention keys (exited:<task>, stalled:<task>,
	// lost:<task>) already delivered, so each wakes the Foreman once per
	// Dispatch attempt.
	Reported []string `json:"reported,omitempty"`
	// CompactAt marks the compaction typed at settlement; the wake follows
	// once the Foreman pane holds still. PaneFingerprint tracks that.
	CompactAt       string `json:"compact_at,omitempty"`
	PaneFingerprint string `json:"pane_fingerprint,omitempty"`
	// WokeAt closes the batch: the settled wake was delivered.
	WokeAt string `json:"woke_at,omitempty"`
}

type batchMember struct {
	Task     string `json:"task"`
	Dispatch string `json:"dispatch,omitempty"`
	Status   string `json:"status,omitempty"`
	Run      string `json:"run,omitempty"`
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
}

const batchCompactSettleMax = 10 * time.Minute

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
	if !c.Orchestration() {
		return fmt.Errorf("foreman batch: %w", orca.ErrNoOrchestration)
	}
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
		state, err := c.DispatchStateFor(task)
		if err != nil {
			return fmt.Errorf("foreman batch: task %s: %w", task, err)
		}
		if state.ID == "" {
			return fmt.Errorf("foreman batch: task %s has no Dispatch; launch it before handing it over", task)
		}
		m := carried[task]
		if m.Dispatch != state.ID {
			m = batchMember{}
		}
		m.Task, m.Dispatch, m.Status, m.Run = task, state.ID, state.Status, state.RunID
		b.Tasks = append(b.Tasks, m)
	}
	if err := saveBatch(id, b); err != nil {
		return fmt.Errorf("foreman batch: %w", err)
	}
	batchHold(c, r, b)
	spawnWatcher()
	ensureWatchSupervisor()
	fmt.Println("BATCH=" + b.ID)
	fmt.Printf("BATCH_TASKS=%d\n", len(b.Tasks))
	fmt.Println("NEXT=end this turn; bbs foreman watch wakes this Foreman when the batch settles or needs attention")
	return nil
}

func printBatchState(b foremanBatch) {
	state := "open"
	switch {
	case !b.open():
		state = "settled"
	case b.CompactAt != "":
		state = "compacting"
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
		fmt.Println(line)
	}
	for _, key := range b.Reported {
		fmt.Println("ATTENTION=" + key)
	}
	if b.Questions > 0 {
		fmt.Printf("ATTENTION=messages:%d\n", b.Questions)
	}
}

// batchTick is one watcher pass over a Foreman's open batch. It returns the
// lines worth printing; a batch that is simply running prints nothing.
func batchTick(client *orca.Client, r foreman.Record, b *foremanBatch, o watchOpts, now time.Time) []string {
	if b.CompactAt != "" {
		return batchAfterCompact(client, r, b, o, now)
	}
	stamp := now.UTC().Format(time.RFC3339)
	for i := range b.Tasks {
		m := &b.Tasks[i]
		if batchFinal(m.Status) {
			continue
		}
		state, err := client.DispatchStateFor(m.Task)
		if err != nil || state.ID == "" {
			if !dispatchTerminal(m.Status) && m.Unseen == "" {
				m.Unseen = stamp
			}
			continue
		}
		if state.ID != m.Dispatch {
			b.forget(m.Task)
			*m = batchMember{Task: m.Task}
		}
		m.Dispatch, m.Status, m.Run = state.ID, state.Status, state.RunID
	}
	if b.settled() == len(b.Tasks) {
		return batchSettle(client, r, b, o, now)
	}
	batchHold(client, r, *b)

	var lines, attention []string
	questions, questionIDs, questionsRead := b.Questions, b.QuestionIDs, false
	if handle, err := client.Ref(r.WorkspaceTitle); err == nil {
		if n, ids, err := client.PendingMessagesFor(handle, "question,escalation"); err == nil {
			if batchNewQuestion(b, n, ids) {
				attention = append(attention, fmt.Sprintf("messages:%d", n))
			}
			questions, questionIDs, questionsRead = n, ids, true
		}
	}
	workers := map[string]orca.Worker{}
	runs := map[string]bool{}
	for _, m := range b.Tasks {
		if !dispatchTerminal(m.Status) && m.Run != "" && !runs[m.Run] {
			runs[m.Run] = true
			list, err := client.Workers(m.Run)
			if err != nil {
				continue
			}
			for _, w := range list {
				workers[w.Dispatch] = w
			}
		}
	}
	for i := range b.Tasks {
		m := &b.Tasks[i]
		if dispatchTerminal(m.Status) {
			m.Unseen = ""
			continue
		}
		w, ok := workers[m.Dispatch]
		if !ok || (w.Handle == "" && !w.Exited) {
			// No worker row (Orca restarted, the worker never started, the
			// list is failing) or a released/taken-over terminal: nothing
			// here can settle this Dispatch, so bound the wait.
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
	if len(attention) > 0 {
		sort.Strings(attention)
		summary := fmt.Sprintf("batch %s needs you (%d/%d settled): %s", b.ID, b.settled(), len(b.Tasks), strings.Join(attention, ", "))
		line := batchWake(client, r, *b, summary)
		lines = append(lines, line)
		if strings.HasPrefix(line, "UNREACHABLE") {
			// Undelivered: keep every key owed, so the next tick retries the
			// same wake rather than dropping it.
			return lines
		}
		for _, key := range attention {
			if !strings.HasPrefix(key, "messages:") {
				b.Reported = append(b.Reported, key)
			}
		}
	}
	if questionsRead {
		b.Questions, b.QuestionIDs = questions, questionIDs
	}
	return lines
}

// batchNewQuestion reports whether the unread question/escalation set holds a
// message the Foreman has not been woken for. Ids make this exact; a runtime
// that reports only a count falls back to "the count grew".
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
	nudge := fmt.Sprintf("Continue Dispatch %s. If its phase is complete, persist the evidence and send its worker_done now; if you are blocked, ask the coordinator with orca orchestration ask.", m.Dispatch)
	if err := client.SendEnterHandle(handle, nudge); err != nil {
		return fmt.Sprintf("WORKER-UNREACHABLE %s — %v", m.Task, err), ""
	}
	m.Nudges++
	m.Pending = true
	m.Since = stamp
	return fmt.Sprintf("NUDGED-WORKER %s (%d/%d) after %s idle", m.Task, m.Nudges, o.workerNudges, roundMin(o.workerIdle)), ""
}

// batchSettle compacts the coordinator before the one wake a batch earns, so
// the next batch starts from a small context; durable state is on disk by
// design and compaction is already a cold-resume boundary for Foreman.
func batchSettle(client *orca.Client, r foreman.Record, b *foremanBatch, o watchOpts, now time.Time) []string {
	prof, err := agent.ByName(batchAgent(r))
	if err == nil && prof.Compact != "" {
		focus := fmt.Sprintf("Foreman %s batch boundary: keep the foreman id, parent ticket, Orca Run and open decisions; everything else is durable on disk.", r.ID)
		if err := client.SendEnter(r.WorkspaceTitle, prof.Compact+" "+focus); err == nil {
			b.CompactAt = now.UTC().Format(time.RFC3339)
			b.PaneFingerprint = ""
			return []string{fmt.Sprintf("COMPACTING %s — batch %s settled", r.ID, b.ID)}
		}
	}
	return []string{batchFinish(client, r, b, now)}
}

// batchAfterCompact waits until the Foreman pane holds still for one tick
// (compaction finished) or the bound expires, then delivers the settled wake.
func batchAfterCompact(client *orca.Client, r foreman.Record, b *foremanBatch, o watchOpts, now time.Time) []string {
	started, _ := time.Parse(time.RFC3339, b.CompactAt)
	pane, err := client.CapturePane(r.WorkspaceTitle, o.lines)
	if err == nil && now.Sub(started) < batchCompactSettleMax {
		fp := paneFingerprint(pane)
		if fp != b.PaneFingerprint {
			b.PaneFingerprint = fp
			return nil
		}
	}
	return []string{batchFinish(client, r, b, now)}
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

// batchWake delivers one notice: a durable Orca message (best effort; it is a
// record in the Foreman's next Delivery) and the prompt that actually starts
// its turn — the held mailbox mutes Orca's own pointer, so the prompt is the
// only doorbell.
func batchWake(client *orca.Client, r foreman.Record, b foremanBatch, summary string) string {
	instruction := summary + ". Run `bbs foreman batch show " + r.ID + "`, then consume the Orca Delivery."
	note := ""
	if run := batchRun(b); run != "" {
		if err := client.SendMessage("run:"+run, "bbs foreman batch", summary); err != nil {
			note = " (orca message failed: " + err.Error() + ")"
		}
	}
	if _, err := watchSend(client, r, instruction); err != nil {
		return fmt.Sprintf("UNREACHABLE %s — %s", r.ID, err)
	}
	return fmt.Sprintf("WOKE %s — %s%s", r.ID, summary, note)
}

// batchRun is the Orca Run whose mailbox the Foreman consumes: the one its
// batch members were dispatched in.
func batchRun(b foremanBatch) string {
	for _, m := range b.Tasks {
		if m.Run != "" {
			return m.Run
		}
	}
	return ""
}

// batchHold keeps an unacknowledged Delivery open on the Foreman's mailbox for
// as long as its batch runs, so worker reports queue without Orca typing a
// pointer into the Foreman per message (orca.HoldMailbox). Delivery needs a
// message, so an empty mailbox gets this batch's notice first. Best effort: a
// failed hold only costs extra Orca wakes, never a lost message.
func batchHold(client *orca.Client, r foreman.Record, b foremanBatch) {
	run := batchRun(b)
	handle, err := client.Ref(r.WorkspaceTitle)
	if run == "" || err != nil {
		return
	}
	if held, err := client.HoldMailbox(run, handle); err != nil || held {
		return
	}
	notice := fmt.Sprintf("batch %s (%d tasks) is supervised by bbs foreman watch; it wakes you when the batch settles or needs you", b.ID, len(b.Tasks))
	if client.SendMessage("run:"+run, "bbs foreman batch", notice) == nil {
		_, _ = client.HoldMailbox(run, handle)
	}
}

func batchAgent(r foreman.Record) string {
	if r.Agent == "" {
		return agent.Default
	}
	return r.Agent
}
