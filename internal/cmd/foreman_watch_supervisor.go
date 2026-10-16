package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/2found/2build/internal/identity"
)

// The OS supervisor is the last link of the watcher's restart chain. A watcher
// that dies is revived by the next bbs process that runs while a batch is open
// (ensureWatcher, from the session hook and `foreman await`) — but when every
// worker and the Foreman are idle, nothing runs, and the batch's one wake is
// owed by a process that no longer exists. launchd (macOS) or systemd --user
// (Linux) closes that gap: the unit restarts a watcher that exits non-zero or
// by signal, and starts one at login after a reboot.
//
// The unit runs `bbs foreman watch --supervised`, which stands by behind a
// watcher that already holds the lock and exits 0 once no batch is open, so a
// quiet machine costs one short process per login. The unit is installed per
// BABYSIT_HOME (test homes never touch the real one) and rewritten only when
// its content changes. BBS_WATCH_SUPERVISOR=off opts out.

const supervisorLabelPrefix = "dev.2found.bbs.foreman-watch"

// supervisorRun is the launchctl/systemctl seam.
var supervisorRun = func(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

type watchSupervisor struct {
	kind  string // launchd | systemd
	label string
	path  string
	body  string
}

func supervisorDisabled() bool {
	return testing.Testing() || strings.EqualFold(os.Getenv("BBS_WATCH_SUPERVISOR"), "off")
}

// supervisorFor describes the unit for this BABYSIT_HOME, or ok=false on a
// platform without a supported per-user supervisor.
func supervisorFor(goos, userHome, babysitHome, bin string, env map[string]string) (watchSupervisor, bool) {
	sum := sha256.Sum256([]byte(babysitHome))
	suffix := hex.EncodeToString(sum[:4])
	logPath := filepath.Join(babysitHome, "watch", "watch.log")
	// A deleted or moved binary exits 0: a unit must never restart-loop on a
	// path that is gone, and the next batch rewrites it with the new path.
	script := fmt.Sprintf("[ -x %[1]s ] || exit 0; exec %[1]s foreman watch --supervised", shellQuote(bin))
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	switch goos {
	case "darwin":
		label := supervisorLabelPrefix + "." + suffix
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>` + label + `</string>
  <key>ProgramArguments</key>
  <array><string>/bin/sh</string><string>-c</string><string>` + html.EscapeString(script) + `</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>ThrottleInterval</key><integer>60</integer>
  <key>ProcessType</key><string>Background</string>
  <key>StandardOutPath</key><string>` + html.EscapeString(logPath) + `</string>
  <key>StandardErrorPath</key><string>` + html.EscapeString(logPath) + `</string>
  <key>EnvironmentVariables</key>
  <dict>
`)
		for _, k := range keys {
			fmt.Fprintf(&b, "    <key>%s</key><string>%s</string>\n", html.EscapeString(k), html.EscapeString(env[k]))
		}
		b.WriteString("  </dict>\n</dict>\n</plist>\n")
		return watchSupervisor{kind: "launchd", label: label,
			path: filepath.Join(userHome, "Library", "LaunchAgents", label+".plist"), body: b.String()}, true
	case "linux":
		label := "bbs-foreman-watch-" + suffix
		var b strings.Builder
		b.WriteString("[Unit]\nDescription=bbs foreman watch (" + babysitHome + ")\n\n[Service]\nType=simple\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "Environment=%s\n", strconv.Quote(k+"="+env[k]))
		}
		fmt.Fprintf(&b, "ExecStart=/bin/sh -c %s\n", strconv.Quote(script))
		fmt.Fprintf(&b, "Restart=on-failure\nRestartSec=60\nStandardOutput=append:%s\nStandardError=append:%s\n\n[Install]\nWantedBy=default.target\n", logPath, logPath)
		return watchSupervisor{kind: "systemd", label: label,
			path: filepath.Join(userHome, ".config", "systemd", "user", label+".service"), body: b.String()}, true
	}
	return watchSupervisor{}, false
}

// supervisorEnv is what the watcher needs to find the same state, Orca and
// tools as the process that opened the batch.
func supervisorEnv() map[string]string {
	env := map[string]string{"BABYSIT_HOME": identity.BabysitHome()}
	for _, k := range []string{"PATH", "HOME", "ORCA_CLI_COMMAND", "ORCA_DEV_REPO_ROOT", "ORCA_RUNTIME_ID"} {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}
	return env
}

// supervisorBin prefers the stable `bbs` on PATH (a Homebrew symlink survives
// upgrades; the resolved Cellar path does not) when it is this same binary.
func supervisorBin() string {
	self := selfBin()
	if p, err := exec.LookPath("bbs"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			if real, err := filepath.EvalSymlinks(abs); err == nil && real == self {
				return abs
			}
		}
	}
	return self
}

func currentSupervisor() (watchSupervisor, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return watchSupervisor{}, false
	}
	return supervisorFor(runtime.GOOS, home, identity.BabysitHome(), supervisorBin(), supervisorEnv())
}

// ensureWatchSupervisor installs (or refreshes) and starts the unit. Called
// when a batch is handed over; best-effort, because the in-process restart
// chain still works without it.
func ensureWatchSupervisor() {
	if supervisorDisabled() {
		return
	}
	s, ok := currentSupervisor()
	if !ok {
		return
	}
	if err := installSupervisor(s); err != nil {
		fmt.Fprintf(os.Stderr, "foreman watch: supervisor not installed: %v\n", err)
	}
}

func installSupervisor(s watchSupervisor) error {
	changed := true
	if old, err := os.ReadFile(s.path); err == nil && string(old) == s.body {
		changed = false
	}
	if changed {
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return err
		}
		tmp := s.path + ".tmp"
		if err := os.WriteFile(tmp, []byte(s.body), 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, s.path); err != nil {
			return err
		}
	}
	switch s.kind {
	case "launchd":
		domain := "gui/" + strconv.Itoa(os.Getuid())
		if changed {
			_, _ = supervisorRun("launchctl", "bootout", domain+"/"+s.label)
		}
		// bootstrap fails harmlessly when already loaded; kickstart without
		// -k starts a stopped job and leaves a running one alone.
		_, _ = supervisorRun("launchctl", "bootstrap", domain, s.path)
		if out, err := supervisorRun("launchctl", "kickstart", domain+"/"+s.label); err != nil {
			return fmt.Errorf("launchctl kickstart: %v %s", err, strings.TrimSpace(out))
		}
	case "systemd":
		if changed {
			if out, err := supervisorRun("systemctl", "--user", "daemon-reload"); err != nil {
				return fmt.Errorf("systemctl daemon-reload: %v %s", err, strings.TrimSpace(out))
			}
			_, _ = supervisorRun("systemctl", "--user", "enable", s.label+".service")
		}
		if out, err := supervisorRun("systemctl", "--user", "start", s.label+".service"); err != nil {
			return fmt.Errorf("systemctl start: %v %s", err, strings.TrimSpace(out))
		}
	}
	return nil
}

func uninstallSupervisor(s watchSupervisor) error {
	switch s.kind {
	case "launchd":
		_, _ = supervisorRun("launchctl", "bootout", "gui/"+strconv.Itoa(os.Getuid())+"/"+s.label)
	case "systemd":
		_, _ = supervisorRun("systemctl", "--user", "disable", "--now", s.label+".service")
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if s.kind == "systemd" {
		_, _ = supervisorRun("systemctl", "--user", "daemon-reload")
	}
	return nil
}

const foremanSupervisorUsage = `Usage:
  bbs foreman supervisor status|install|uninstall
    The per-user launchd/systemd unit that restarts a dead watcher while a
    batch is open. install runs automatically on every batch handover.`

func foremanSupervisor(args []string) error {
	if len(args) != 1 {
		return errors.New(foremanSupervisorUsage)
	}
	s, ok := currentSupervisor()
	if !ok {
		fmt.Println("SUPERVISOR=unsupported")
		return nil
	}
	switch args[0] {
	case "status":
		state := "absent"
		if b, err := os.ReadFile(s.path); err == nil {
			state = "installed"
			if string(b) != s.body {
				state = "stale"
			}
		}
		fmt.Println("SUPERVISOR=" + s.kind)
		fmt.Println("LABEL=" + s.label)
		fmt.Println("UNIT=" + s.path)
		fmt.Println("STATE=" + state)
		return nil
	case "install":
		if err := installSupervisor(s); err != nil {
			return err
		}
		fmt.Println("SUPERVISOR_INSTALLED=" + s.path)
		return nil
	case "uninstall":
		if err := uninstallSupervisor(s); err != nil {
			return err
		}
		fmt.Println("SUPERVISOR_REMOVED=" + s.path)
		return nil
	}
	return errors.New(foremanSupervisorUsage)
}
