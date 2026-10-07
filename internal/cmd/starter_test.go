package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/2found/2build/internal/starter"
)

func commandStarterFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"common/.babysit", "templates/hono-bun"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(root, "catalog.json"), `{
  "schema_version":1,"source":"2found/2build-starters","version":"0.1.0","min_cli_version":"1.95.0",
  "templates":{"hono-bun":{"path":"templates/hono-bun","revision":1,"summary":"Initial starter","upgrade":"upgrades/hono-bun.md","dev":["bun","run","dev"],"verify":[["go","version"]]}}
}`)
	mustWrite(t, filepath.Join(root, "common", "AGENTS.md"), "# __PROJECT_NAME__\n")
	mustWrite(t, filepath.Join(root, "common", ".babysit", "git-flow.yaml"), "profile: __GIT_PROFILE__\n")
	mustWrite(t, filepath.Join(root, "templates", "hono-bun", "package.json"), `{"name":"__PROJECT_NAME__"}`)
	return root
}

func runStarterCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errOut.String(), err
}

func useStarterVersion(t *testing.T) {
	t.Helper()
	old := version
	version = "1.95.0"
	t.Cleanup(func() { version = old })
}

func TestBootstrapLocalSourceVerifiesAndReturnsCleanJSON(t *testing.T) {
	useStarterVersion(t)
	source, target := commandStarterFixture(t), filepath.Join(t.TempDir(), "my-api")
	out, log, err := runStarterCommand(t, "bootstrap", target, "--source", source, "--json")
	if err != nil {
		t.Fatalf("bootstrap failed: %v %s", err, log)
	}
	var result struct {
		Directory string       `json:"directory"`
		Verified  bool         `json:"verified"`
		Starter   starter.Lock `json:"starter"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil || result.Directory != target || !result.Verified || result.Starter.Release != "0.1.0" || !strings.Contains(log, "go version") {
		t.Fatalf("bad JSON handoff: %s log=%s err=%v", out, log, err)
	}
	policy, _ := os.ReadFile(filepath.Join(target, ".babysit/git-flow.yaml"))
	if string(policy) != "profile: startup\n" {
		t.Fatalf("default policy: %s", policy)
	}
	_, _, err = runStarterCommand(t, "bootstrap", target, "--source", source, "--no-verify")
	if err == nil {
		t.Fatal("CLI overwrote target")
	}
	for _, args := range [][]string{{"bootstrap", "--help"}, {"starter", "check", "--help"}} {
		out, _, err := runStarterCommand(t, args...)
		if err != nil || !strings.Contains(out, "Usage:") {
			t.Fatalf("help ran an action: %s %v", out, err)
		}
	}
}

func TestBootstrapRejectsUnavailableVersionAndFailedChecks(t *testing.T) {
	useStarterVersion(t)
	source, parent := commandStarterFixture(t), t.TempDir()
	target := filepath.Join(parent, "my-api")
	version = "1.94.2"
	_, _, err := runStarterCommand(t, "bootstrap", target, "--source", source)
	if err == nil || !strings.Contains(err.Error(), "requires bbs >= 1.95.0") {
		t.Fatalf("missing version guard: %v", err)
	}
	version = "1.95.0"
	b, _ := os.ReadFile(filepath.Join(source, "catalog.json"))
	mustWrite(t, filepath.Join(source, "catalog.json"), strings.ReplaceAll(string(b), `"go","version"`, `"missing-bbs-starter-runtime"`))
	_, _, err = runStarterCommand(t, "bootstrap", target, "--source", source)
	if err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("claimed readiness without runtime: %v", err)
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 0 {
		t.Fatal("failed bootstrap left project or staging files")
	}
	out, _, err := runStarterCommand(t, "bootstrap", target, "--source", source, "--no-verify", "--json")
	if err != nil || !strings.Contains(out, `"verified":false`) {
		t.Fatalf("skipped checks not disclosed: %s %v", out, err)
	}
}

func TestStarterCheckFindsNestedProjectAndDoesNotChangeIt(t *testing.T) {
	useStarterVersion(t)
	source, target := commandStarterFixture(t), filepath.Join(t.TempDir(), "my-api")
	if _, _, err := runStarterCommand(t, "bootstrap", target, "--source", source, "--no-verify"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(target, starter.LockPath))
	b, _ := os.ReadFile(filepath.Join(source, "catalog.json"))
	mustWrite(t, filepath.Join(source, "catalog.json"), strings.ReplaceAll(strings.ReplaceAll(string(b), `"version":"0.1.0"`, `"version":"0.2.0"`), `"revision":1`, `"revision":2`))
	out, _, err := runStarterCommand(t, "starter", "check", "--dir", filepath.Join(target, ".babysit"), "--source", source, "--json")
	var result starter.CheckResult
	if err != nil || json.Unmarshal([]byte(out), &result) != nil || result.Status != "update_available" || result.UpgradeURL == "" {
		t.Fatalf("bad release check: %s %v", out, err)
	}
	after, _ := os.ReadFile(filepath.Join(target, starter.LockPath))
	if !bytes.Equal(before, after) {
		t.Fatal("check applied an update")
	}
	if _, _, err := runStarterCommand(t, "starter", "check", "--dir", t.TempDir(), "--source", source); err == nil {
		t.Fatal("missing lock not reported")
	}
}

func TestStarterNoticeIsAdvisoryAndSkipsNonStarterProjects(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	calls := 0
	check := func(lock starter.Lock) starter.CheckResult {
		calls++
		return starter.CheckResult{Status: "update_available", Template: lock.Template, Current: lock.Release, Latest: "0.2.0", Summary: "New harness", UpgradeURL: "https://example.invalid/guide"}
	}
	starterNotice(dir, &out, check)
	if calls != 0 || out.Len() != 0 {
		t.Fatal("ordinary repo incurred a starter network check")
	}
	source, target := commandStarterFixture(t), filepath.Join(dir, "my-api")
	if _, err := starter.Generate(source, target, "hono-bun", "pet", "", nil); err != nil {
		t.Fatal(err)
	}
	starterNotice(target, &out, check)
	if calls != 1 || !strings.Contains(out.String(), "STARTER_UPDATE_AVAILABLE") {
		t.Fatalf("workflow missed release notice: %s", out.String())
	}
	out.Reset()
	starterNotice(target, &out, func(starter.Lock) starter.CheckResult {
		return starter.CheckResult{Status: "unknown", Warning: "offline"}
	})
	if !strings.Contains(out.String(), "offline") {
		t.Fatal("offline check was silently presented as current")
	}
	out.Reset()
	mustWrite(t, filepath.Join(target, starter.LockPath), "schema_version: 99\n")
	starterNotice(target, &out, check)
	if calls != 1 || !strings.Contains(out.String(), "BBS_DEGRADED") {
		t.Fatal("malformed provenance stopped workflow or reached network")
	}
}

func TestStarterPreambleReportsCachedUpdatesAndHonorsUserConfig(t *testing.T) {
	bin := buildTestBBS(t)
	source := commandStarterFixture(t)
	project := filepath.Join(t.TempDir(), "my-api")
	if _, err := starter.Generate(source, project, "hono-bun", "pet", "", nil); err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	t.Setenv("BABYSIT_STATE_DIR", state)
	t.Setenv("BABYSIT_PROJECT_HOME", filepath.Join(state, "project"))
	t.Setenv("BABYSIT_TICKET", "")
	t.Setenv("BABYSIT_DIR", source)
	mustWrite(t, filepath.Join(source, "VERSION"), "1.95.0\n")
	t.Setenv("BABYSIT_REMOTE_URL", "file://"+filepath.Join(source, "VERSION"))
	catalog, err := starter.ReadCatalog(source)
	if err != nil {
		t.Fatal(err)
	}
	catalog.Version = "0.2.0"
	entry := catalog.Templates["hono-bun"]
	entry.Revision = 2
	catalog.Templates["hono-bun"] = entry
	cache, err := json.Marshal(map[string]any{"catalog": catalog, "revision": strings.Repeat("a", 40), "checked_at": time.Now().UTC(), "attempted_at": time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(state, "starter"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(state, "starter/catalog-cache.json"), string(cache))
	for _, enabled := range []bool{true, false} {
		policy := "telemetry: off\nupdate_check: false\n"
		if enabled {
			policy = "telemetry: off\nupdate_check: true\n"
		}
		mustWrite(t, filepath.Join(state, "config.yaml"), policy)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		command := exec.CommandContext(ctx, bin, "skill", "enter", "--name", "implement")
		command.Dir = project
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		cancel()
		if err != nil || !strings.Contains(stdout.String(), "SKILL: implement") {
			t.Fatalf("starter probe disrupted skill entry: %v %s %s", err, stdout.String(), stderr.String())
		}
		if strings.Contains(stderr.String(), "STARTER_UPDATE_AVAILABLE") != enabled {
			t.Fatalf("update_check=%v notice=%s", enabled, stderr.String())
		}
	}
}
