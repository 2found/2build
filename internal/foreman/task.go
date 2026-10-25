package foreman

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/2found/2build/internal/identity"
)

// A Task is one Foreman assignment (a ticket session, a follow-up, phase QA,
// a release) and its Dispatches, one per worker launched for it. bbs owns this
// lifecycle on disk; Orca only hosts the worker's terminal and worktree. That
// keeps the worker protocol to three bbs commands (done, ask, progress) with
// no mailbox to hold, acknowledge or heartbeat.

// Dispatch statuses. Only completed and failed are terminal; "" means the
// Task has no Dispatch yet.
const (
	DispatchOpen      = "dispatched"
	DispatchCompleted = "completed"
	DispatchFailed    = "failed"
)

type Task struct {
	ID         string     `json:"id"`
	Foreman    string     `json:"foreman"`
	Ticket     string     `json:"ticket"`
	Kind       string     `json:"kind,omitempty"`
	CreatedAt  string     `json:"created_at"`
	Dispatches []Dispatch `json:"dispatches,omitempty"`
}

type Dispatch struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Agent    string `json:"agent,omitempty"`
	Model    string `json:"model,omitempty"`
	Effort   string `json:"effort,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Terminal string `json:"terminal,omitempty"`
	// OwnsTerminal marks a terminal this Dispatch's launch created; only those
	// are closed at settle. A reused session's terminal belongs to its creator.
	OwnsTerminal  bool     `json:"owns_terminal,omitempty"`
	LaunchedAt    string   `json:"launched_at"`
	SettledAt     string   `json:"settled_at,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	ReportPath    string   `json:"report_path,omitempty"`
	FilesModified []string `json:"files_modified,omitempty"`
	// Reason says why a Dispatch failed without its worker's report
	// (abandoned by Foreman, worker exited).
	Reason string `json:"reason,omitempty"`
	// Gate is the readiness verdict a succeeded ticket-session report passed:
	// ready, light-ready, skipped: <why>, or unverified: <error>.
	Gate string `json:"gate,omitempty"`
}

// Question is a worker's blocking ask. It stays pending until Foreman
// answers; the asking worker polls for the answer.
type Question struct {
	ID         string `json:"id"`
	Task       string `json:"task"`
	Dispatch   string `json:"dispatch"`
	Body       string `json:"body"`
	AskedAt    string `json:"asked_at"`
	Answer     string `json:"answer,omitempty"`
	AnsweredAt string `json:"answered_at,omitempty"`
}

// ErrTaskNotFound is a Task id with no record. Callers that only ask "is a
// worker running for it" read it as "not dispatched yet".
var ErrTaskNotFound = errors.New("task not found")

func TasksDir() string { return filepath.Join(identity.BabysitHome(), "tasks") }

func taskDir(id string) string { return filepath.Join(TasksDir(), id) }

func taskPath(id string) string { return filepath.Join(taskDir(id), "task.json") }

// SpecPath is the Task's assignment, written once at create.
func SpecPath(id string) string { return filepath.Join(taskDir(id), "spec.md") }

func questionPath(task, id string) string {
	return filepath.Join(taskDir(task), "questions", id+".json")
}

func validTaskPart(kind, id string) error {
	if id == "" || id != filepath.Base(id) || strings.HasPrefix(id, ".") || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("invalid %s id %q", kind, id)
	}
	return nil
}

func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateTask persists a new Task and its spec, returning its id.
func CreateTask(foremanID, ticketID, kind string, spec []byte) (Task, error) {
	t := Task{
		ID:        "task-" + time.Now().UTC().Format("20060102-150405") + "-" + randomSuffix(),
		Foreman:   foremanID,
		Ticket:    ticketID,
		Kind:      kind,
		CreatedAt: Now(),
	}
	if err := os.MkdirAll(filepath.Join(taskDir(t.ID), "questions"), 0o755); err != nil {
		return Task{}, err
	}
	if err := os.WriteFile(SpecPath(t.ID), spec, 0o644); err != nil {
		return Task{}, err
	}
	return t, writeJSON(taskPath(t.ID), t)
}

func LoadTask(id string) (Task, error) {
	var t Task
	if err := validTaskPart("task", id); err != nil {
		return t, err
	}
	data, err := os.ReadFile(taskPath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return t, fmt.Errorf("task %s: %w", id, ErrTaskNotFound)
		}
		return t, err
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return t, fmt.Errorf("task %s: %w", id, err)
	}
	return t, nil
}

// UpdateTask applies fn to the Task under its lock, so a worker's report and
// Foreman's abandon cannot overwrite each other.
func UpdateTask(id string, fn func(*Task) error) (Task, error) {
	if err := validTaskPart("task", id); err != nil {
		return Task{}, err
	}
	unlock, err := lockResources(filepath.Join(taskDir(id), ".lock"))
	if err != nil {
		return Task{}, err
	}
	defer unlock()
	t, err := LoadTask(id)
	if err != nil {
		return t, err
	}
	if err := fn(&t); err != nil {
		return t, err
	}
	return t, writeJSON(taskPath(id), t)
}

// Current is the Task's latest Dispatch, or nil before its first launch.
func (t Task) Current() *Dispatch {
	if len(t.Dispatches) == 0 {
		return nil
	}
	return &t.Dispatches[len(t.Dispatches)-1]
}

// DispatchByID finds one Dispatch of the Task.
func (t Task) DispatchByID(id string) *Dispatch {
	for i := range t.Dispatches {
		if t.Dispatches[i].ID == id {
			return &t.Dispatches[i]
		}
	}
	return nil
}

func DispatchSettled(status string) bool {
	return status == DispatchCompleted || status == DispatchFailed
}

// OpenDispatch starts the Task's next Dispatch. A Task whose current Dispatch
// is still open refuses: one Task has one live worker.
func (t *Task) OpenDispatch(d Dispatch) (*Dispatch, error) {
	if cur := t.Current(); cur != nil && !DispatchSettled(cur.Status) {
		return nil, fmt.Errorf("task %s: dispatch %s is still open", t.ID, cur.ID)
	}
	d.ID = fmt.Sprintf("%s.d%d", t.ID, len(t.Dispatches)+1)
	d.Status = DispatchOpen
	d.LaunchedAt = Now()
	t.Dispatches = append(t.Dispatches, d)
	return t.Current(), nil
}

// Settle closes an open Dispatch. Settling an already settled Dispatch with
// the same status is a no-op, so a retried report is safe; a conflicting one
// is refused.
func (d *Dispatch) Settle(status, reason string) error {
	if DispatchSettled(d.Status) {
		if d.Status == status {
			return nil
		}
		return fmt.Errorf("dispatch %s already settled as %s", d.ID, d.Status)
	}
	d.Status, d.Reason, d.SettledAt = status, reason, Now()
	return nil
}

// ListTasks returns every Task, oldest first; foremanID filters when set.
func ListTasks(foremanID string) []Task {
	entries, err := os.ReadDir(TasksDir())
	if err != nil {
		return nil
	}
	var out []Task
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := LoadTask(e.Name())
		if err != nil || (foremanID != "" && t.Foreman != foremanID) {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out
}

// DispatchForTerminal finds the open Dispatch running in terminal, if any.
func DispatchForTerminal(handle string) (Task, *Dispatch) {
	if handle == "" {
		return Task{}, nil
	}
	for _, t := range ListTasks("") {
		if d := t.Current(); d != nil && d.Terminal == handle && !DispatchSettled(d.Status) {
			return t, d
		}
	}
	return Task{}, nil
}

// Ask records a pending question on the Task's open Dispatch.
func Ask(taskID, body string) (Question, error) {
	t, err := LoadTask(taskID)
	if err != nil {
		return Question{}, err
	}
	d := t.Current()
	if d == nil || DispatchSettled(d.Status) {
		return Question{}, fmt.Errorf("task %s has no open dispatch", taskID)
	}
	q := Question{
		ID:       "q-" + time.Now().UTC().Format("150405") + "-" + randomSuffix(),
		Task:     taskID,
		Dispatch: d.ID,
		Body:     body,
		AskedAt:  Now(),
	}
	if err := os.MkdirAll(filepath.Dir(questionPath(taskID, q.ID)), 0o755); err != nil {
		return Question{}, err
	}
	return q, writeJSON(questionPath(taskID, q.ID), q)
}

func LoadQuestion(taskID, id string) (Question, error) {
	var q Question
	if err := validTaskPart("task", taskID); err != nil {
		return q, err
	}
	if err := validTaskPart("question", id); err != nil {
		return q, err
	}
	data, err := os.ReadFile(questionPath(taskID, id))
	if err != nil {
		return q, fmt.Errorf("question %s/%s: %w", taskID, id, err)
	}
	return q, json.Unmarshal(data, &q)
}

// AnswerQuestion records Foreman's answer once; a second answer is refused.
func AnswerQuestion(taskID, id, answer string) (Question, error) {
	q, err := LoadQuestion(taskID, id)
	if err != nil {
		return q, err
	}
	if q.AnsweredAt != "" {
		return q, fmt.Errorf("question %s is already answered", id)
	}
	q.Answer, q.AnsweredAt = answer, Now()
	return q, writeJSON(questionPath(taskID, id), q)
}

// PendingQuestions lists the unanswered questions of the given Tasks, oldest
// first.
func PendingQuestions(taskIDs []string) []Question {
	var out []Question
	for _, id := range taskIDs {
		if validTaskPart("task", id) != nil {
			continue
		}
		entries, _ := os.ReadDir(filepath.Join(taskDir(id), "questions"))
		for _, e := range entries {
			name, ok := strings.CutSuffix(e.Name(), ".json")
			if !ok {
				continue
			}
			if q, err := LoadQuestion(id, name); err == nil && q.AnsweredAt == "" {
				out = append(out, q)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AskedAt < out[j].AskedAt })
	return out
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
