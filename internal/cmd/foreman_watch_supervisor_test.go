//go:build unix

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSupervisorUnitsAreScopedAndCannotRestartLoop(t *testing.T) {
	env := map[string]string{"PATH": "/usr/bin:/a&b", "BABYSIT_HOME": "/h/one"}
	mac, ok := supervisorFor("darwin", "/Users/u", "/h/one", "/opt/bbs", env)
	if !ok || mac.kind != "launchd" {
		t.Fatalf("darwin unit = %+v, %v", mac, ok)
	}
	for _, want := range []string{
		"<key>Label</key><string>" + mac.label + "</string>",
		"<key>SuccessfulExit</key><false/>",
		"<key>RunAtLoad</key><true/>",
		"[ -x &#39;/opt/bbs&#39; ] || exit 0; exec &#39;/opt/bbs&#39; foreman watch --supervised",
		"<key>PATH</key><string>/usr/bin:/a&amp;b</string>",
		"/h/one/watch/watch.log",
	} {
		if !strings.Contains(mac.body, want) {
			t.Fatalf("plist missing %q:\n%s", want, mac.body)
		}
	}
	if mac.path != filepath.Join("/Users/u/Library/LaunchAgents", mac.label+".plist") {
		t.Fatalf("plist path = %s", mac.path)
	}
	again, _ := supervisorFor("darwin", "/Users/u", "/h/one", "/opt/bbs", env)
	other, _ := supervisorFor("darwin", "/Users/u", "/h/two", "/opt/bbs", env)
	if again.body != mac.body || other.label == mac.label {
		t.Fatalf("label must be stable per BABYSIT_HOME and distinct across homes: %s %s", mac.label, other.label)
	}
	if runtime.GOOS == "darwin" {
		if plutil, err := exec.LookPath("plutil"); err == nil {
			p := filepath.Join(t.TempDir(), "u.plist")
			if err := os.WriteFile(p, []byte(mac.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(plutil, "-lint", p).CombinedOutput(); err != nil {
				t.Fatalf("plist does not lint: %v %s", err, out)
			}
		}
	}

	linux, ok := supervisorFor("linux", "/home/u", "/h/one", "/opt/bbs", env)
	if !ok || linux.kind != "systemd" {
		t.Fatalf("linux unit = %+v, %v", linux, ok)
	}
	for _, want := range []string{"Restart=on-failure", `Environment="PATH=/usr/bin:/a&b"`, "foreman watch --supervised", "[ -x '/opt/bbs' ] || exit 0"} {
		if !strings.Contains(linux.body, want) {
			t.Fatalf("unit missing %q:\n%s", want, linux.body)
		}
	}
	if _, ok := supervisorFor("windows", "C:/u", "/h/one", "bbs", env); ok {
		t.Fatal("windows must have no supervisor")
	}
}

func TestInstallSupervisorRewritesOnlyOnChange(t *testing.T) {
	var calls []string
	old := supervisorRun
	supervisorRun = func(name string, args ...string) (string, error) {
		calls = append(calls, name+" "+args[0])
		return "", nil
	}
	t.Cleanup(func() { supervisorRun = old })
	home := t.TempDir()
	s, _ := supervisorFor("darwin", home, "/h/one", "/opt/bbs", nil)

	if err := installSupervisor(s); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(calls, ","); got != "launchctl bootout,launchctl bootstrap,launchctl kickstart" {
		t.Fatalf("first install calls = %s", got)
	}
	calls = nil
	if err := installSupervisor(s); err != nil {
		t.Fatal(err)
	}
	// Unchanged: never bootout a running watcher, only make sure it runs.
	if got := strings.Join(calls, ","); got != "launchctl bootstrap,launchctl kickstart" {
		t.Fatalf("repeat install calls = %s", got)
	}
	if err := uninstallSupervisor(s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Fatalf("unit left behind: %v", err)
	}
}

func TestSupervisedWatchStandsByOnlyWhileABatchIsOpen(t *testing.T) {
	t.Setenv("BABYSIT_HOME", t.TempDir())
	t.Setenv("ORCA_CLI_COMMAND", filepath.Join(t.TempDir(), "missing-orca"))
	release, err := acquireWatchLock("")
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	// Nothing open: a supervised start behind a live watcher exits at once.
	out := captureStdout(t, func() {
		if err := foremanWatch([]string{"--supervised", "--interval", "1"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "already running") {
		t.Fatalf("supervised watcher should stand by silently: %s", out)
	}

	// Open batch: it waits for the lock, takes over when the holder dies,
	// and a failure after the batch closed exits 0 instead of looping.
	if err := saveBatch("fm-a", foremanBatch{ID: "fm-a-1", Tasks: []batchMember{{Task: "t", Status: "running"}}}); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		release()
		time.Sleep(1500 * time.Millisecond)
		_ = os.Remove(batchPath("fm-a"))
	}()
	start := time.Now()
	out = captureStdout(t, func() {
		if err := foremanWatch([]string{"--supervised", "--interval", "1"}); err != nil {
			t.Fatalf("supervised failure with no open batch must exit 0: %v", err)
		}
	})
	if time.Since(start) < time.Second {
		t.Fatalf("supervised watcher did not stand by while the batch was open: %s", out)
	}
	if !strings.Contains(out, "ORCA-DOWN") || !strings.Contains(out, "not restarting") {
		t.Fatalf("missing takeover outcome: %s", out)
	}
}
