package workspace

import (
	"strings"
	"testing"
)

func TestRepoMetadataStaleness(t *testing.T) {
	old := "1.50.0"
	repo := Repo{GitURL: "web.git", HarnessVersion: &old}
	if !repo.Stale("1.57.1") {
		t.Fatal("older harness_version should read stale")
	}
	current := "1.57.1"
	repo.HarnessVersion = &current
	if repo.Stale(current) {
		t.Fatal("current harness_version should not read stale")
	}
}

func TestResolverMembershipByPath(t *testing.T) {
	TestHome(t)
	top := t.TempDir()
	if err := AddRepo("acme", Repo{GitURL: "web.git", Path: top, Role: "fe"}); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(top, "")
	if resolver.Err() != nil || !resolver.Registered() || resolver.Name() != "acme" {
		t.Fatalf("path membership failed: err=%v registered=%v name=%q", resolver.Err(), resolver.Registered(), resolver.Name())
	}
}

func TestResolverMembershipByGitURL(t *testing.T) {
	TestHome(t)
	if err := AddRepo("acme", Repo{GitURL: "web.git"}); err != nil {
		t.Fatal(err)
	}
	if resolver := NewResolver(t.TempDir(), "web.git"); resolver.Err() != nil || !resolver.Registered() {
		t.Fatalf("git url should establish membership: %v", resolver.Err())
	}
}

func TestResolverRejectsDuplicateMembership(t *testing.T) {
	TestHome(t)
	top := t.TempDir()
	for _, name := range []string{"acme", "other"} {
		if err := AddRepo(name, Repo{GitURL: name + ".git", Path: top}); err != nil {
			t.Fatal(err)
		}
	}
	resolver := NewResolver(top, "")
	if resolver.Err() == nil || !strings.Contains(resolver.Err().Error(), "multiple workspace entries") {
		t.Fatalf("duplicate membership must fail clearly, got %v", resolver.Err())
	}
}

func TestResolverRolePathAndFanOut(t *testing.T) {
	TestHome(t)
	top := t.TempDir()
	if err := AddRepo("acme", Repo{GitURL: "web.git", Path: top, Role: "fe", RepoType: TypePolyrepo}); err != nil {
		t.Fatal(err)
	}
	if err := AddRepo("acme", Repo{GitURL: "api.git", Path: "/tmp/api", Role: "be"}); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolver(top, "")
	if path, ok := resolver.RolePath("be"); !ok || path != "/tmp/api" {
		t.Fatalf("role be should resolve to /tmp/api, got %q ok=%v", path, ok)
	}
	if !resolver.FanOut() {
		t.Fatal("polyrepo must keep fan-out on")
	}
}

func TestMonorepoDisablesFanOut(t *testing.T) {
	TestHome(t)
	top := t.TempDir()
	if err := AddRepo("acme", Repo{GitURL: "mono.git", Path: top, RepoType: TypeMonorepo}); err != nil {
		t.Fatal(err)
	}
	if NewResolver(top, "").FanOut() {
		t.Fatal("monorepo must skip sibling fan-out")
	}
}

func TestUnregisteredRepoResolverIsInert(t *testing.T) {
	TestHome(t)
	resolver := NewResolver(t.TempDir(), "")
	if resolver.Err() != nil || resolver.Registered() || resolver.Name() != "" {
		t.Fatalf("unregistered repo must be inert, got err=%v registered=%v name=%q", resolver.Err(), resolver.Registered(), resolver.Name())
	}
	if !resolver.FanOut() {
		t.Fatal("unregistered repo must keep fan-out enabled")
	}
}
