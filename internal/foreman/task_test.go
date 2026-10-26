package foreman

import (
	"errors"
	"strings"
	"testing"
)

func TestTaskDispatchLifecycle(t *testing.T) {
	t.Setenv("BABYSIT_HOME", t.TempDir())
	task, err := CreateTask("fm-a", "bs-a", "ticket", []byte("spec"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTask("task-nope"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("missing task err = %v", err)
	}
	open := func() error {
		_, err := UpdateTask(task.ID, func(t *Task) error {
			_, err := t.OpenDispatch(Dispatch{Terminal: "term_1", OwnsTerminal: true})
			return err
		})
		return err
	}
	if err := open(); err != nil {
		t.Fatal(err)
	}
	if err := open(); err == nil || !strings.Contains(err.Error(), "still open") {
		t.Fatalf("second open = %v", err)
	}
	if _, err := Ask(task.ID, "ok?"); err != nil {
		t.Fatal(err)
	}
	if len(PendingQuestions([]string{task.ID})) != 1 {
		t.Fatal("question not pending")
	}
	if _, err := UpdateTask(task.ID, func(t *Task) error { return t.Current().Settle(DispatchFailed, "abandoned") }); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateTask(task.ID, func(t *Task) error { return t.Current().Settle(DispatchCompleted, "") }); err == nil {
		t.Fatal("conflicting settle accepted")
	}
	if len(PendingQuestions([]string{task.ID})) != 0 {
		t.Fatal("a settled dispatch's question still asks for an answer")
	}
	if _, err := Ask(task.ID, "late?"); err == nil {
		t.Fatal("asked on a settled dispatch")
	}
	// The retry is a new Dispatch of the same Task.
	if err := open(); err != nil {
		t.Fatal(err)
	}
	got, _ := LoadTask(task.ID)
	if len(got.Dispatches) != 2 || got.Current().ID != task.ID+".d2" {
		t.Fatalf("dispatches = %+v", got.Dispatches)
	}
	if tk, d := DispatchForTerminal("term_1"); d == nil || tk.ID != task.ID || d.ID != task.ID+".d2" {
		t.Fatal("open dispatch not found by terminal")
	}
	if len(ListTasks("fm-a")) != 1 || len(ListTasks("fm-b")) != 0 {
		t.Fatal("list filter")
	}
}
