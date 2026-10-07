package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/2found/2build/internal/config"
	"github.com/2found/2build/internal/starter"
	"github.com/2found/2build/internal/ticket"
)

// skillPreamble is the human-readable enter mode. JSON enter remains the
// telemetry-only API used by callers that manage their own bootstrap.
func skillPreamble(rec skillRuntimeRecord) {
	env := resolveEnv() // Conflicting identity must fail before writing state.
	if env.Ticket != "" {
		// init owns ticket locking and manifests and exits after completion.
		if err := exec.Command(selfBin(), "ticket", "init").Run(); err != nil {
			fmt.Fprintf(os.Stderr, "BBS_DEGRADED: ticket init: %v\n", err)
		}
	}

	check := exec.Command(selfBin(), "update", "check")
	if out, err := check.Output(); err == nil && len(out) > 0 {
		fmt.Fprint(os.Stderr, string(out))
	}
	cwd, _ := os.Getwd()
	if enabled, _ := config.Get("update_check"); enabled != "false" {
		starterNotice(cwd, os.Stderr, func(lock starter.Lock) starter.CheckResult {
			return checkStarterRelease(context.Background(), lock, false)
		})
	}
	sessions := refreshSkillSession(rec.InvocationID, cwd, env.Ticket)
	proactive, _ := config.Get("proactive")
	ref := "/bbs:"
	switch rec.Harness {
	case "codex":
		ref = "$bbs:"
	case "omp", "cursor":
		ref = "/"
	}
	for _, field := range [][2]string{
		{"SKILL", rec.Skill}, {"SESSION_ID", rec.InvocationID},
		{"SESSIONS_ACTIVE", fmt.Sprint(sessions)}, {"SLUG", env.Slug},
		{"BRANCH", rec.Branch}, {"REPO", rec.Repo}, {"INVOKER", rec.Invoker},
		{"AGENT", rec.Harness}, {"SKILL_REF", ref}, {"TICKET", orDefault(env.Ticket, "<none>")},
		{"PROJECT_HOME", env.ProjectHome}, {"PROACTIVE", orDefault(proactive, "true")},
		{"TELEMETRY", telemetryMode()}, {"SPAWNED", fmt.Sprint(os.Getenv("OPENCLAW_SESSION") != "")},
	} {
		fmt.Printf("%s: %s\n", field[0], field[1])
	}

	a := &apState{slug: env.Slug, branch: env.Branch, ticket: env.Ticket, stateRoot: env.ProjectHome}
	if snapshot, err := collectAutopilotSnapshot(a, ""); err == nil {
		fmt.Println("AUTOPILOT_CONTRACT: v2")
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"snapshot_id": snapshot.SnapshotID, "state_revision": snapshot.StateRevision,
			"ticket": snapshot.Ticket, "run": snapshot.Run, "git": snapshot.Git,
			"policy": snapshot.Policy, "gates": snapshot.Gates, "obligations": snapshot.Obligations,
		})
	} else if env.Ticket != "" {
		recover := exec.Command(selfBin(), "autopilot", "recover")
		recover.Stdout = os.Stdout
		_ = recover.Run()
	}
	if err := appendSkillRuntime(rec); err != nil {
		fmt.Fprintf(os.Stderr, "BBS_DEGRADED: skill telemetry: %v\n", err)
	}
}

func refreshSkillSession(invocation, cwd, ticketID string) int {
	dir := ticket.SessionsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0
	}
	// Each invocation owns its marker, so nested skills cannot remove each other.
	_ = os.WriteFile(filepath.Join(dir, invocation+".active"), nil, 0o644)
	sid := os.Getenv("BABYSIT_SESSION")
	if sid == "" {
		for _, source := range [][2]string{
			{"CLAUDE_CODE_SESSION_ID", "cc-"}, {"CODEX_SESSION_ID", "cx-"},
			{"CODEX_THREAD_ID", "cx-"}, {"GROK_SESSION_ID", "grok-"},
		} {
			if id := os.Getenv(source[0]); id != "" {
				sid = source[1] + id
				break
			}
		}
	}
	writeSessionRecord(dir, sid, cwd, ticketID)
	sweepStaleSessions(dir, time.Now())
	entries, _ := os.ReadDir(dir)
	count := 0
	// .active markers are live sessions; <sid>.yaml records linger for the
	// sweep window after exit, so counting all files reports 2× the truth.
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".active") {
			count++
		}
	}
	return count
}
