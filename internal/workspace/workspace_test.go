package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/reallongnguyen/babysit/internal/config"
)

func TestCreateAndLoadRoundTripInUnifiedConfig(t *testing.T) {
	TestHome(t)
	if err := config.Set("telemetry", "off"); err != nil {
		t.Fatal(err)
	}
	if err := Create("acme"); err != nil {
		t.Fatal(err)
	}
	if err := Create("acme"); err != nil {
		t.Fatalf("second Create should be a no-op: %v", err)
	}
	workspace, err := Load("acme")
	if err != nil || workspace.Name != "acme" || workspace.Version != 1 {
		t.Fatalf("got %+v err=%v", workspace, err)
	}
	body, err := os.ReadFile(config.Path())
	if err != nil || !strings.Contains(string(body), `telemetry: "off"`) || !strings.Contains(string(body), "workspaces:") {
		t.Fatalf("settings and workspaces must share one file: %q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(config.Dir(), "workspaces")); !os.IsNotExist(err) {
		t.Fatalf("legacy workspace directory should not be created: %v", err)
	}
}

func TestAddRepoAppendsAndUpdatesByGitURL(t *testing.T) {
	TestHome(t)
	if err := AddRepo("acme", Repo{GitURL: "git@github.com:acme/web.git", Path: "/tmp/web", Role: "fe"}); err != nil {
		t.Fatal(err)
	}
	if err := AddRepo("acme", Repo{GitURL: "git@github.com:acme/api.git", Path: "/tmp/api", Role: "be"}); err != nil {
		t.Fatal(err)
	}
	if err := AddRepo("acme", Repo{GitURL: "git@github.com:acme/web.git", Path: "/tmp/web2", Role: "fe"}); err != nil {
		t.Fatal(err)
	}
	workspace, err := Load("acme")
	if err != nil || len(workspace.Repos) != 2 || workspace.Repos[0].Path != "/tmp/web2" {
		t.Fatalf("update by git URL failed: %+v err=%v", workspace, err)
	}
}

func TestAddRepoConcurrentDoesNotDropEntries(t *testing.T) {
	TestHome(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = AddRepo("acme", Repo{GitURL: string(rune('a'+i)) + ".git", Path: "/tmp/" + string(rune('a'+i))})
		}(i)
	}
	wg.Wait()
	workspace, err := Load("acme")
	if err != nil || len(workspace.Repos) != 8 {
		t.Fatalf("concurrent add-repo dropped entries: %+v err=%v", workspace, err)
	}
}

func TestValidNameRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"../escape", ".hidden", "a/b", "", "/abs"} {
		if err := ValidName(bad); err == nil {
			t.Fatalf("ValidName(%q) should reject", bad)
		}
	}
	if err := ValidName("acme-1.0_x"); err != nil {
		t.Fatalf("ValidName rejected a legal name: %v", err)
	}
}

func TestMalformedUnifiedConfigIsNeverOverwritten(t *testing.T) {
	TestHome(t)
	broken := []byte("workspaces: [oops\n")
	if err := os.WriteFile(config.Path(), broken, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddRepo("acme", Repo{GitURL: "api.git", Path: "/tmp/api"}); err == nil {
		t.Fatal("add-repo onto malformed config must fail")
	}
	after, _ := os.ReadFile(config.Path())
	if string(after) != string(broken) {
		t.Fatalf("broken config changed: %q", after)
	}
}

func TestSamePathIgnoresSymlinkSpelling(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if !SamePath(real, link) || SamePath(real, dir) {
		t.Fatal("path canonicalization is incorrect")
	}
}

func TestSamePathFoldsCaseOnInsensitiveFS(t *testing.T) {
	old := fsCaseInsensitive
	defer func() { fsCaseInsensitive = old }()
	dir := t.TempDir()
	upper, lower := filepath.Join(dir, "REPO"), filepath.Join(dir, "repo")
	fsCaseInsensitive = true
	if !SamePath(upper, lower) {
		t.Fatal("case-insensitive filesystems must fold case")
	}
	fsCaseInsensitive = false
	if SamePath(upper, lower) {
		t.Fatal("case-sensitive filesystems must not fold case")
	}
}
