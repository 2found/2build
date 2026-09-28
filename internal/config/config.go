// Package config owns the single babysit config file
// (~/.babysit/config.yaml).
//
// Scalar get/set operations preserve existing text. Structured writers update
// only their YAML subtree under a shared lock, so workspace registrations cannot
// overwrite each other.
package config

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/reallongnguyen/babysit/internal/ticket"
	"gopkg.in/yaml.v3"
)

// configHeader is written verbatim on the first `set` into a fresh file. It
// began as a byte-for-byte copy of CONFIG_HEADER in the former bin/bbs-config;
// that script is gone, so the header is owned here and documents new keys as
// they land.
const configHeader = `# babysit configuration — edit freely, changes take effect on next skill run.
# Docs: https://github.com/reallongnguyen/babysit
#
# ─── Behavior ────────────────────────────────────────────────────────
# proactive: true           # Auto-invoke skills when the request matches one.
#                           # Set to false to only run skills explicitly typed.
#
# ─── Telemetry ───────────────────────────────────────────────────────
# telemetry: local          # off | local
#                           #   off   — no data recorded
#                           #   local — JSONL to ~/.babysit/analytics/ (never leaves machine)
# ─── Foreman ─────────────────────────────────────────────────────────
# parallel_max_workers: 4         # hard ceiling for one Foreman; keeps enough
#                                 # laptop headroom for the coordinator and OS.
#                                 # Host safety is also enforced by the global
#                                 # weighted pool.
# parallel_global_units: auto     # machine-global weighted worker capacity
#                                 # shared by every Foreman. auto derives a
#                                 # host CPU/RAM budget; a positive
#                                 # value may only lower that budget.
# foreman_status_interval: 3600   # seconds between full reconciliation ticks:
#                                 #   the Foreman skill's fallback audit and
#                                 #   bbs foreman watch's status/idle prompt
#                                 #   defaults share this one value. Deliveries
#                                 #   still wake the foreman immediately; this
#                                 #   is only the missed-event/restart backup.
#                                 #   bbs foreman watch --status-interval
#                                 #   overrides it for that watcher.
# ─── Workspaces ──────────────────────────────────────────────────────
# workspaces:               # managed by bbs config workspace; repo paths,
#                           # roles, metadata, and harness version live here.
#
# ─── Updates ─────────────────────────────────────────────────────────
# auto_upgrade: false       # true = silently run bbs-upgrade on session start
# update_check: true        # false = suppress upgrade-available notifications
#
`

var retiredAgentSettings = map[string]struct{}{
	"worker_agent": {}, "worker_provider": {}, "worker_model": {}, "worker_effort": {},
	"foreman_agent": {}, "foreman_provider": {}, "foreman_model": {}, "foreman_effort": {},
}

const retiredAgentSettingsNotice = "legacy worker/foreman agent settings are ignored; configure enabled agents and the default in Orca, or pass an explicit per-dispatch agent/model/effort"

func isRetiredAgentSetting(key string) bool {
	_, ok := retiredAgentSettings[key]
	return ok
}

// WarnRetiredAgentSettings emits one migration diagnostic when the config file
// still contains any retired launch preference. It does not modify the file.
func WarnRetiredAgentSettings(w io.Writer) {
	b, err := Read()
	if err != nil {
		return
	}
	var doc yaml.Node
	if yaml.Unmarshal(b, &doc) != nil || len(doc.Content) == 0 {
		return
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if isRetiredAgentSetting(mapping.Content[i].Value) {
			fmt.Fprintln(w, retiredAgentSettingsNotice)
			return
		}
	}
}

// Dir returns the babysit state directory, honoring BABYSIT_STATE_DIR
// (default ~/.babysit) — matching bin/bbs-config.
func Dir() string {
	if d := os.Getenv("BABYSIT_STATE_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".babysit")
}

// Path returns the config file path.
func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

// Read returns the config bytes. A missing file is an empty config.
func Read() ([]byte, error) {
	b, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return nil, nil
	}
	return b, err
}

// Update serializes one read-modify-write against the single config file.
func Update(mutate func([]byte) ([]byte, error)) ([]byte, error) {
	return UpdatePath(Path(), mutate)
}

// UpdatePath is Update for a dashboard server started with an explicit state
// directory. All writers share the same lock and atomic-write behavior.
func UpdatePath(path string, mutate func([]byte) ([]byte, error)) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	unlock, err := acquireLock(path)
	if err != nil {
		return nil, err
	}
	defer unlock()

	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		current, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	next, err := mutate(current)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(current, next) {
		return next, nil
	}
	target := path
	if info, statErr := os.Lstat(target); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		target, err = filepath.EvalSymlinks(target)
		if err != nil {
			return nil, err
		}
	}
	if err := ticket.WriteAtomic(target, next); err != nil {
		return nil, err
	}
	return next, nil
}

func acquireLock(path string) (func(), error) {
	lockPath := path + ".lock"
	for tries := 0; ; tries++ {
		if err := os.Mkdir(lockPath, 0o755); err == nil {
			return func() { _ = os.RemoveAll(lockPath) }, nil
		}
		if tries >= 50 {
			return nil, fmt.Errorf("config: failed to acquire lock after 5s (%s)", lockPath)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Get returns the value for a top-level key and whether it was present.
// A missing file, unparseable file, or absent key yields ("", false) — never
// an error — mirroring the bash `grep ... 2>/dev/null || true` behavior. When
// a key appears more than once, the last occurrence wins (like `tail -1`).
func Get(key string) (string, bool) {
	b, err := os.ReadFile(Path())
	if err != nil {
		return "", false
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil || len(doc.Content) == 0 {
		return "", false
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode {
		return "", false
	}
	val, found := "", false
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			val = m.Content[i+1].Value
			found = true
		}
	}
	return val, found
}

// Set writes key/value into the config file, preserving all existing content.
// The value is truncated to its first line (matching `head -1`). On a fresh
// file the documented header is seeded first. An existing `key:` line is
// replaced in place; otherwise `key: value` is appended.
func Set(key, value string) error {
	if isRetiredAgentSetting(key) {
		return fmt.Errorf("config key %q is retired; configure agents in Orca", key)
	}
	if i := strings.IndexByte(value, '\n'); i >= 0 {
		value = value[:i]
	}
	// Values are opaque strings: @slow, provider/model and values with ':' or
	// '#' must survive YAML without becoming aliases, maps or comments.
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	value = strings.TrimSuffix(string(encoded), "\n")
	_, err = Update(func(b []byte) ([]byte, error) {
		content := string(b)
		if len(b) == 0 {
			content = configHeader
		}
		prefix := key + ":"
		lines := strings.Split(content, "\n")
		matched := false
		for i, ln := range lines {
			if strings.HasPrefix(ln, prefix) {
				lines[i] = key + ": " + value
				matched = true
			}
		}
		if matched {
			content = strings.Join(lines, "\n")
		} else {
			if content != "" && !strings.HasSuffix(content, "\n") {
				// A hand-edited file may lack a final newline; appending without
				// one would splice the new key onto the last line, corrupting both.
				content += "\n"
			}
			content += key + ": " + value + "\n"
		}
		return []byte(content), nil
	})
	return err
}

// List returns the raw file bytes (or nil if the file is missing), matching
// `cat 2>/dev/null || true`.
func List() []byte {
	b, _ := Read()
	return b
}

// ForemanStatusIntervalSeconds is the one configured reconciliation interval:
// the default for `bbs foreman watch --status-interval` and the bound the
// Foreman skill applies to its `check --wait` timeout. Absent or empty config
// falls back to the default, matching Get's missing-file convention; a
// present-but-invalid value is an error, never a silent tight loop. The upper
// bound is what a time.Duration of whole seconds can hold — the same
// multiplication the watcher performs.
func ForemanStatusIntervalSeconds() (int, error) {
	const def = 3600
	v, ok := Get("foreman_status_interval")
	if !ok || strings.TrimSpace(v) == "" {
		return def, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 || int64(n) > math.MaxInt64/int64(time.Second) {
		return 0, fmt.Errorf("config foreman_status_interval needs a positive number of seconds, got '%s'", v)
	}
	return n, nil
}
