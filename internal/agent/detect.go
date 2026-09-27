package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Normalize accepts common product names, but never guesses from a model ID.
func Normalize(name string) string {
	switch name = strings.ToLower(strings.TrimSpace(name)); name {
	case "claude-code", "claude code":
		return "claude"
	case "cursor-agent":
		return "cursor"
	case "oh-my-pi":
		return "omp"
	}
	return name
}

type Detection struct {
	Agent  string `json:"agent"`
	Source string `json:"source"`
}

// Detect reports the current harness, not an installed executable or a role's
// launch default. Nearest ancestor wins over inherited outer-session markers.
func Detect() Detection {
	if name := Normalize(os.Getenv("BABYSIT_CURRENT_AGENT")); name != "" && name != "auto" {
		return Detection{name, "BABYSIT_CURRENT_AGENT"}
	}
	if name := ancestorAgent(); name != "" {
		return Detection{name, "parent process"}
	}
	return detectEnv(os.Getenv)
}

func detectEnv(getenv func(string) string) Detection {
	for _, marker := range []struct{ agent, key string }{
		{"grok", "GROK_SESSION_ID"}, {"grok", "GROK_AGENT"},
		{"codex", "CODEX_THREAD_ID"}, {"codex", "CODEX_SESSION_ID"},
		{"claude", "CLAUDE_CODE_SESSION_ID"}, {"claude", "CLAUDECODE"},
		{"cursor", "CURSOR_AGENT"},
	} {
		if v := strings.TrimSpace(getenv(marker.key)); v != "" && v != "0" && v != "false" {
			return Detection{marker.agent, marker.key}
		}
	}
	return Detection{"unknown", "no session marker"}
}

func ancestorAgent() string {
	// ps is optional (e.g. Windows). Inspect executable names only, never argv
	// that can contain prompts or credentials. Bound both depth and runtime.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for pid, depth := os.Getppid(), 0; pid > 1 && depth < 12; depth++ {
		out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "ppid=,comm=").Output()
		if err != nil {
			break
		}
		fields := strings.Fields(string(out))
		if len(fields) < 2 {
			break
		}
		executable := strings.Join(fields[1:], " ")
		if name := processAgent(executable); name != "" {
			return name
		}
		// comm is a bare name (≤15 chars on Linux): the Cursor CLI binary is
		// literally `agent` inside a cursor-agent install dir, so a name-only
		// check cannot prove it. /proc/<pid>/exe gives the real path there.
		if exe, linkErr := os.Readlink("/proc/" + strconv.Itoa(pid) + "/exe"); linkErr == nil {
			if name := processAgent(exe); name != "" {
				return name
			}
		}
		parent, err := strconv.Atoi(fields[0])
		if err != nil || parent == pid {
			break
		}
		pid = parent
	}
	return ""
}

func processAgent(executable string) string {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(executable)), ".exe")
	for name, profile := range profiles {
		if base == profile.Bin {
			return name
		}
	}
	for _, name := range []string{"codex", "grok"} {
		if strings.HasPrefix(base, name+"-") {
			return name // versioned/platform-specific native binaries
		}
	}
	// The Cursor editor itself and a generic `agent` executable are not proof
	// of an agent session. Cursor also ships `agent` in its CLI install tree.
	if base == "agent" && strings.Contains(filepath.ToSlash(executable), "/cursor-agent/") {
		return "cursor"
	}
	return ""
}

type Installation struct {
	Agent string `json:"agent"`
	Bin   string `json:"bin"`
	Path  string `json:"path,omitempty"`
}

// Installations is deterministic; it does not start agents or inspect auth.
func Installations() []Installation {
	out := make([]Installation, 0, len(profiles))
	for _, name := range Names() {
		p := profiles[name]
		path, _ := exec.LookPath(p.Bin)
		out = append(out, Installation{name, p.Bin, path})
	}
	return out
}
