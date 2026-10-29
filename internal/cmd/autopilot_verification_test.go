package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerificationProducerAndCrashRetry(t *testing.T) {
	repo, env := initReadinessFixture(t)
	t.Chdir(repo)
	a := &apState{slug: env.Slug, branch: env.Branch, ticket: env.Ticket, stateRoot: env.ProjectHome}
	for _, gate := range []string{"review-pr", "qa"} {
		var err error
		out := captureStdout(t, func() {
			err = a.verificationCommand([]string{"begin", "--gate", gate, "--owner", "worker", "--handle", "session-test", "--transport", "test", "--harness", "test"})
		})
		if err != nil {
			t.Fatal(err)
		}
		var e struct {
			Data attemptRecord `json:"data"`
		}
		if err := json.Unmarshal([]byte(out), &e); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "result.json")
		if err := writeJSONAtomic(file, map[string]interface{}{"checks": []map[string]interface{}{{"argv": []string{"git", "diff", "--check"}, "cwd": repo, "exit_code": 0, "log_path": filepath.Join(env.ProjectHome, "tickets", env.Ticket, "evidence", "check.log")}}, "unresolved_findings": []interface{}{}, "limitations": []interface{}{}}); err != nil {
			t.Fatal(err)
		}
		args := []string{"record", "--attempt", e.Data.ID, "--file", file}
		captureStdout(t, func() { err = a.verificationCommand(args) })
		if err != nil {
			t.Fatal(err)
		}
		if gate == "qa" {
			// Simulate a crash after writing pending evidence but before the
			// running attempt was advanced to completed.
			home := filepath.Join(env.ProjectHome, "tickets", env.Ticket)
			if err := writeAttemptRecord(filepath.Join(home, "attempts", e.Data.ID+".json"), &e.Data); err != nil {
				t.Fatal(err)
			}
			cp, err := readStrictObject(filepath.Join(home, "checkpoint.json"), true)
			if err != nil {
				t.Fatal(err)
			}
			cp["active_attempt_id"] = e.Data.ID
			if err := writeJSONAtomic(filepath.Join(home, "checkpoint.json"), cp); err != nil {
				t.Fatal(err)
			}
		}
		captureStdout(t, func() { err = a.verificationCommand(args) })
		if err != nil {
			t.Fatalf("crash retry failed: %v", err)
		}
	}
	ready, err := evaluateReadiness(env, "review")
	if err != nil || !ready.Ready {
		t.Fatalf("produced evidence unusable: %+v %v", ready, err)
	}
	// Evidence binds content: an edit after the gate makes it stale.
	mustWrite(t, filepath.Join(repo, "tracked.txt"), "edited after the gate\n")
	ready, err = evaluateReadiness(env, "review")
	if err != nil || ready.Ready || !strings.Contains(strings.Join(ready.ReasonCodes, ","), "tree_digest") {
		t.Fatalf("post gate edit accepted: %+v %v", ready, err)
	}
}

func TestVerificationProducerRejectsRepairDuringGate(t *testing.T) {
	repo, env := initReadinessFixture(t)
	t.Chdir(repo)
	a := &apState{slug: env.Slug, branch: env.Branch, ticket: env.Ticket, stateRoot: env.ProjectHome}
	var err error
	out := captureStdout(t, func() {
		err = a.verificationCommand([]string{"begin", "--gate", "qa", "--owner", "worker", "--handle", "test", "--transport", "test"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var e struct {
		Data attemptRecord `json:"data"`
	}
	json.Unmarshal([]byte(out), &e)
	mustWrite(t, filepath.Join(repo, "tracked.txt"), "repair during gate")
	file := filepath.Join(t.TempDir(), "result.json")
	writeJSONAtomic(file, map[string]interface{}{"checks": []map[string]interface{}{{"argv": []string{"test"}, "cwd": repo, "exit_code": 0, "log_path": filepath.Join(env.ProjectHome, "tickets", env.Ticket, "evidence", "check.log")}}})
	if err := a.verificationCommand([]string{"record", "--attempt", e.Data.ID, "--file", file}); err == nil || !strings.Contains(err.Error(), "subject changed") {
		t.Fatalf("stale check accepted: %v", err)
	}
	if rec := readAttemptRecord(filepath.Join(env.ProjectHome, "tickets", env.Ticket, "attempts", e.Data.ID+".json")); rec == nil || rec.State != "failed" {
		t.Fatal("stale attempt retained writer ownership")
	}
}

// A gate needs a ticket and a checkpoint this bbs can read: no ticket is a
// rejection, not a panic, and a newer schema is named as such rather than as v1.
func TestVerificationBeginRefusesWithoutUsableCheckpoint(t *testing.T) {
	repo, env := initReadinessFixture(t)
	t.Chdir(repo)
	begin := []string{"begin", "--gate", "qa", "--owner", "worker", "--handle", "session-test", "--transport", "test", "--harness", "test"}
	none := &apState{slug: env.Slug, branch: env.Branch, stateRoot: env.ProjectHome}
	if err := none.verificationCommand(begin); err == nil || !strings.Contains(err.Error(), "needs a ticket") {
		t.Fatalf("begin without a ticket = %v, want a ticket refusal", err)
	}

	cpPath := filepath.Join(env.ProjectHome, "tickets", env.Ticket, "checkpoint.json")
	a := &apState{slug: env.Slug, branch: env.Branch, ticket: env.Ticket, stateRoot: env.ProjectHome}
	if err := os.Remove(cpPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := a.verificationCommand(begin); err == nil || !strings.Contains(err.Error(), "checkpoint --refresh") {
		t.Fatalf("begin without a checkpoint = %v, want the refresh command", err)
	}

	mustWrite(t, cpPath, `{"schema_version":3,"ticket":"`+env.Ticket+`","run_id":"run-x"}`)
	if err := a.verificationCommand(begin); err == nil || !strings.Contains(err.Error(), "newer than this bbs supports") {
		t.Fatalf("begin on schema_version 3 = %v, want CHECKPOINT_UNSUPPORTED", err)
	}
}

// A gate binds the ticket's own change: paths dirty when the ticket started
// belong to someone else and stay out of the subject; a change to a ticket
// file, or a new file joining the change, voids the gate.
func TestVerificationSubjectIsScopedToTicketChange(t *testing.T) {
	repo, env := initReadinessFixture(t)
	t.Chdir(repo)
	mustWrite(t, filepath.Join(repo, "owner-wip.txt"), "owner work in progress\n")
	a := &apState{slug: env.Slug, branch: env.Branch, ticket: env.Ticket, stateRoot: env.ProjectHome}
	if err := a.checkpointV2(checkpointV2Input{Ticket: env.Ticket, Workflow: "builder", Step: "init", Status: "in_progress"}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, "ticket.txt"), "ticket work\n")

	begin := func() (string, string) {
		var err error
		out := captureStdout(t, func() {
			err = a.verificationCommand([]string{"begin", "--gate", "qa", "--owner", "worker", "--handle", "test", "--transport", "test"})
		})
		if err != nil {
			t.Fatal(err)
		}
		var e struct {
			Data attemptRecord `json:"data"`
		}
		if err := json.Unmarshal([]byte(out), &e); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "result.json")
		writeJSONAtomic(file, map[string]interface{}{"checks": []map[string]interface{}{{"argv": []string{"test"}, "cwd": repo, "exit_code": 0, "log_path": filepath.Join(env.ProjectHome, "tickets", env.Ticket, "evidence", "check.log")}}})
		return e.Data.ID, file
	}
	record := func(id, file string) error {
		var err error
		captureStdout(t, func() { err = a.verificationCommand([]string{"record", "--attempt", id, "--file", file}) })
		return err
	}
	reasons := func() string {
		ready, err := evaluateReadiness(env, "review")
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(ready.ReasonCodes, ",")
	}

	// Unrelated edits during the gate, to a path dirty at ticket start, do not void it.
	id, file := begin()
	mustWrite(t, filepath.Join(repo, "owner-wip.txt"), "owner kept typing\n")
	if err := record(id, file); err != nil {
		t.Fatalf("unrelated edit voided the gate: %v", err)
	}
	if got := reasons(); strings.Contains(got, "tree_digest") {
		t.Fatalf("unrelated edit made evidence stale: %s", got)
	}

	// A change to a ticket file voids it, at record time and in readiness.
	id, file = begin()
	mustWrite(t, filepath.Join(repo, "ticket.txt"), "ticket work, edited\n")
	if err := record(id, file); err == nil || !strings.Contains(err.Error(), "subject changed") {
		t.Fatalf("ticket file edit accepted: %v", err)
	}

	// A new ticket file voids a recorded gate.
	id, file = begin()
	if err := record(id, file); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, "second-ticket-file.txt"), "new\n")
	if got := reasons(); !strings.Contains(got, "qa:stale_tree_digest") {
		t.Fatalf("new ticket file kept the gate current: %s", got)
	}
	// ...and so does a deletion inside the scope.
	if err := os.Remove(filepath.Join(repo, "second-ticket-file.txt")); err != nil {
		t.Fatal(err)
	}
	id, file = begin()
	if err := record(id, file); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repo, "ticket.txt")); err != nil {
		t.Fatal(err)
	}
	if got := reasons(); !strings.Contains(got, "qa:stale_tree_digest") {
		t.Fatalf("deleting a ticket file kept the gate current: %s", got)
	}
	mustWrite(t, filepath.Join(repo, "ticket.txt"), "ticket work, edited\n")

	// A claimed baseline path is the ticket's too: edits to it void the gate.
	id, file = begin()
	if err := a.claimPaths(env, []string{"owner-wip.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := record(id, file); err == nil || !strings.Contains(err.Error(), "subject changed") {
		t.Fatalf("claim did not change the subject: %v", err)
	}
	id, file = begin()
	mustWrite(t, filepath.Join(repo, "owner-wip.txt"), "now the ticket edits it too\n")
	if err := record(id, file); err == nil || !strings.Contains(err.Error(), "subject changed") {
		t.Fatalf("claimed path edit accepted: %v", err)
	}
}

// A checkpoint without a baseline keeps the whole-tree subject.
func TestVerificationWithoutBaselineBindsWholeTree(t *testing.T) {
	repo, env := initReadinessFixture(t)
	t.Chdir(repo)
	a := &apState{slug: env.Slug, branch: env.Branch, ticket: env.Ticket, stateRoot: env.ProjectHome}
	var err error
	out := captureStdout(t, func() {
		err = a.verificationCommand([]string{"begin", "--gate", "qa", "--owner", "worker", "--handle", "test", "--transport", "test"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var e struct {
		Data attemptRecord `json:"data"`
	}
	json.Unmarshal([]byte(out), &e)
	mustWrite(t, filepath.Join(repo, "unrelated.txt"), "x\n")
	file := filepath.Join(t.TempDir(), "result.json")
	writeJSONAtomic(file, map[string]interface{}{"checks": []map[string]interface{}{{"argv": []string{"test"}, "cwd": repo, "exit_code": 0, "log_path": filepath.Join(env.ProjectHome, "tickets", env.Ticket, "evidence", "check.log")}}})
	if err := a.verificationCommand([]string{"record", "--attempt", e.Data.ID, "--file", file}); err == nil || !strings.Contains(err.Error(), "subject changed") {
		t.Fatalf("whole-tree fallback lost: %v", err)
	}
}
