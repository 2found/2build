package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextProjectionIsBoundedKeepsRequiredReadsAndRedactsLogs(t *testing.T) {
	repo := initSnapshotRepo(t)
	t.Chdir(repo)
	t.Setenv("BBS_BASE_BRANCH", "main")
	project := filepath.Join(t.TempDir(), "project")
	home := filepath.Join(project, "tickets", "ap-05")
	mustMkdirAll(t, filepath.Join(home, "attempts"))
	mustMkdirAll(t, filepath.Join(home, "logs"))
	requirement := "MUST KEEP THIS FIRST\n" + strings.Repeat("requirement detail\n", 500) + "MUST KEEP THIS LAST\n"
	plan := "PLAN START\n" + strings.Repeat("plan detail\n", 500) + "PLAN END\n"
	mustWrite(t, filepath.Join(home, "index.json"), `{"id":"ap-05","origin":{"type":"standalone"},"control":null}`)
	mustWrite(t, filepath.Join(home, "checkpoint.json"), `{"schema_version":2,"run_id":"run-05","revision":3,"ticket":"ap-05","workflow":"builder","branch":"main","active_attempt_id":"implement-1"}`)
	mustWrite(t, filepath.Join(home, "requirement.md"), requirement)
	mustWrite(t, filepath.Join(home, "plan.md"), plan)
	mustWrite(t, filepath.Join(home, "logs", "worker.log"), "normal line\nAPI_TOKEN=do-not-leak\nfinished\n")
	mustWrite(t, filepath.Join(home, "attempts", "implement-1.json"), `{"schema_version":2,"id":"implement-1","ticket":"ap-05","run_id":"run-05","gate":"implement","state":"waiting","revision":2,"owner":"worker","idempotency_key":"key","assignment":{"prompt":"TOP SECRET SOURCE BODY","prohibited_operations":["push"]},"runtime":{"kind":"native","handle":"h","transport_id":"t"},"result":{"log_path":"logs/worker.log","raw_output":"DO NOT EMIT"},"created_at":"2026-09-10T00:00:00Z","updated_at":"2026-09-10T00:01:00Z"}`)
	a := &apState{slug: "project", branch: "main", ticket: "ap-05", stateRoot: project}

	snapshot, err := collectAutopilotSnapshot(a, "")
	if err != nil {
		t.Fatal(err)
	}
	projection := buildContextProjection(snapshot)
	total := 0
	foundRequired := false
	for _, artifact := range projection.Artifacts {
		total += len(artifact.Excerpt)
		if artifact.Role == "requirement" {
			foundRequired = artifact.RequiresRead && artifact.Truncated && strings.Contains(artifact.Excerpt, "MUST KEEP THIS FIRST") && strings.Contains(artifact.Excerpt, "MUST KEEP THIS LAST")
		}
	}
	if total > contextArtifactBudget || !foundRequired {
		t.Fatalf("bounded required artifact contract failed: bytes=%d artifacts=%+v", total, projection.Artifacts)
	}
	if len(projection.Logs) != 1 || strings.Contains(projection.Logs[0].Excerpt, "do-not-leak") || !strings.Contains(projection.Logs[0].Excerpt, "[REDACTED]") {
		t.Fatalf("safe log excerpt contract failed: %+v", projection.Logs)
	}
	if got := stringValue(snapshot.ActiveAttempt["liveness"]); got != "unknown" {
		t.Fatalf("native liveness must remain unknown without adapter evidence, got %q", got)
	}
	encoded, _ := json.Marshal(snapshot.ActiveAttempt)
	if strings.Contains(string(encoded), "TOP SECRET") || strings.Contains(string(encoded), "DO NOT EMIT") {
		t.Fatalf("snapshot leaked unbounded attempt bodies: %s", encoded)
	}
}

func TestContextNoTicketDoesNotCreateCache(t *testing.T) {
	repo := initSnapshotRepo(t)
	t.Chdir(repo)
	t.Setenv("BBS_BASE_BRANCH", "main")
	project := filepath.Join(t.TempDir(), "missing")
	a := &apState{slug: "project", branch: "main", stateRoot: project}
	snapshot, err := collectAutopilotSnapshot(a, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = buildContextProjection(snapshot)
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Fatalf("no-ticket context initialized state: %v", err)
	}
}
