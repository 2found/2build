package cmd

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	contextArtifactBudget = 8 * 1024
	contextLogBudget      = 2 * 1024
)

type contextArtifact struct {
	Role         string   `json:"role"`
	Path         string   `json:"path"`
	Digest       string   `json:"digest,omitempty"`
	RequiredFor  []string `json:"required_for,omitempty"`
	Exists       bool     `json:"exists"`
	Excerpt      string   `json:"excerpt,omitempty"`
	Truncated    bool     `json:"truncated,omitempty"`
	RequiresRead bool     `json:"requires_read,omitempty"`
}

type contextLog struct {
	Path      string `json:"path"`
	Excerpt   string `json:"excerpt"`
	Truncated bool   `json:"truncated,omitempty"`
}

type contextProjection struct {
	Snapshot    *autopilotSnapshot   `json:"snapshot"`
	Artifacts   []contextArtifact    `json:"artifacts"`
	Logs        []contextLog         `json:"logs"`
	Obligations []snapshotObligation `json:"obligations"`
}

func (a *apState) recoverV2(args []string) {
	if argValue(args, "--since") != "" {
		failV2("USAGE", "recover --json does not accept --since", false, nil, 2)
	}
	snapshot, err := collectAutopilotSnapshot(a, argValue(args, "--ticket"))
	if err != nil {
		writeSnapshotError(err)
	}
	projection := buildContextProjection(snapshot)
	action := "continue"
	if snapshot.ActiveAttempt != nil {
		action = "reconcile_active_attempt"
	} else if snapshot.Run == nil {
		action = "direct_skill"
	}
	printV2Envelope(map[string]interface{}{
		"action": action, "snapshot_id": snapshot.SnapshotID,
		"state_revision": snapshot.StateRevision, "projection": projection,
	})
}

func buildContextProjection(snapshot *autopilotSnapshot) contextProjection {
	projection := contextProjection{Snapshot: snapshot, Obligations: snapshot.Obligations, Artifacts: []contextArtifact{}, Logs: []contextLog{}}
	remaining := contextArtifactBudget
	for _, artifact := range orderedContextArtifacts(snapshot.Artifacts) {
		item := contextArtifact{Role: artifact.Role, Path: artifact.Path, Digest: artifact.Digest, RequiredFor: artifact.RequiredFor, Exists: artifact.Exists}
		if artifact.Exists && remaining > 0 {
			limit := 2400
			if remaining < limit {
				limit = remaining
			}
			item.Excerpt, item.Truncated = boundedFileExcerpt(artifact.Path, limit)
			remaining -= len(item.Excerpt)
		}
		item.RequiresRead = item.Exists && (item.Truncated || item.Excerpt == "")
		projection.Artifacts = append(projection.Artifacts, item)
	}
	if snapshot.Ticket != nil && snapshot.ActiveAttempt != nil {
		if log := contextAttemptLog(snapshot); log != nil {
			projection.Logs = append(projection.Logs, *log)
		}
	}
	return projection
}

func orderedContextArtifacts(in []snapshotArtifact) []snapshotArtifact {
	priority := map[string]int{"requirement": 0, "plan": 1, "workflow": 2, "latest_handoff": 3, "checkpoint": 4, "manifest": 5}
	out := append([]snapshotArtifact(nil), in...)
	sort.SliceStable(out, func(i, j int) bool { return priority[out[i].Role] < priority[out[j].Role] })
	return out
}

func boundedFileExcerpt(path string, limit int) (string, bool) {
	if limit <= 0 {
		return "", true
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", false
	}
	if info.Size() <= int64(limit) {
		b, _ := io.ReadAll(io.LimitReader(f, int64(limit)))
		return string(b), false
	}
	marker := "\n… [truncated; read full artifact at path] …\n"
	if limit <= len(marker) {
		return "", true
	}
	each := (limit - len(marker)) / 2
	head := make([]byte, each)
	_, _ = io.ReadFull(f, head)
	tail := make([]byte, each)
	_, _ = f.ReadAt(tail, info.Size()-int64(each))
	return string(head) + marker + string(tail), true
}

var sensitiveLogLine = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|authorization|private key)`)

func contextAttemptLog(snapshot *autopilotSnapshot) *contextLog {
	result, _ := snapshot.ActiveAttempt["result"].(map[string]interface{})
	path := stringValue(result["log_path"])
	if path == "" {
		failure, _ := snapshot.ActiveAttempt["failure"].(map[string]interface{})
		path = stringValue(failure["log_path"])
	}
	if path == "" || snapshot.Ticket == nil {
		return nil
	}
	ticketHome := ""
	for _, artifact := range snapshot.Artifacts {
		if artifact.Role == "checkpoint" {
			ticketHome = filepath.Dir(artifact.Path)
			break
		}
	}
	if ticketHome == "" {
		return nil
	}
	resolved, err := resolveEvidencePath(path, ticketHome, snapshot.Ticket.Worktree)
	if err != nil {
		return nil
	}
	excerpt, truncated := boundedFileExcerpt(resolved, contextLogBudget)
	lines := strings.Split(excerpt, "\n")
	for i, line := range lines {
		if sensitiveLogLine.MatchString(line) {
			lines[i] = "[REDACTED]"
		}
	}
	return &contextLog{Path: resolved, Excerpt: strings.Join(lines, "\n"), Truncated: truncated}
}
