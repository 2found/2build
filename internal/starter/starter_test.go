package starter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func catalogFixture() Catalog {
	return Catalog{SchemaVersion: 1, Source: Source, Version: "0.1.0", MinCLI: "1.95.0", Templates: map[string]Template{
		"hono-bun": {Path: "templates/hono-bun", Revision: 1, Summary: "Initial starter", Upgrade: "upgrades/hono-bun.md", Dev: []string{"bun", "run", "dev"}, Verify: [][]string{{"bun", "run", "test"}}},
	}}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sourceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	b, err := json.Marshal(catalogFixture())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "catalog.json"), string(b))
	writeFile(t, filepath.Join(root, "common", "AGENTS.md"), "# __PROJECT_NAME__\n")
	writeFile(t, filepath.Join(root, "common", ".babysit", "git-flow.yaml"), "profile: __GIT_PROFILE__\n")
	writeFile(t, filepath.Join(root, "templates", "hono-bun", "package.json"), `{"name":"__PROJECT_NAME__"}`)
	return root
}

func TestGenerateComposesPortableHarnessAndProvenance(t *testing.T) {
	root := sourceFixture(t)
	writeFile(t, filepath.Join(root, "templates/hono-bun/node_modules/unused"), "installed dependency")
	writeFile(t, filepath.Join(root, "templates/hono-bun/dist/unused"), "build output")
	target := filepath.Join(t.TempDir(), "my-api")
	verified := false
	lock, err := Generate(root, target, "hono-bun", "startup", "", func(stage string, template Template) error {
		verified = true
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatal("target published before verification")
		}
		b, err := os.ReadFile(filepath.Join(stage, "package.json"))
		if err != nil || string(b) != `{"name":"my-api"}` || len(template.Verify) != 1 {
			t.Fatalf("bad generated input: %s, %v", b, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !verified || lock.Release != "0.1.0" || lock.TemplateRevision != 1 || !strings.HasPrefix(lock.Revision, "sha256:") {
		t.Fatalf("missing verification/provenance: %+v", lock)
	}
	stored, err := ReadLock(target)
	if err != nil || stored != lock {
		t.Fatalf("lock did not round trip: %+v, %v", stored, err)
	}
	for path, expected := range map[string]string{"AGENTS.md": "# my-api\n", ".babysit/git-flow.yaml": "profile: startup\n"} {
		b, err := os.ReadFile(filepath.Join(target, path))
		if err != nil || string(b) != expected {
			t.Fatalf("%s: %q %v", path, b, err)
		}
	}
	for _, path := range []string{"node_modules", "dist"} {
		if _, err := os.Stat(filepath.Join(target, path)); !os.IsNotExist(err) {
			t.Fatalf("copied %s", path)
		}
	}
	project, err := FindProject(filepath.Join(target, ".babysit"))
	if err != nil || project != target {
		t.Fatalf("nested provenance lookup failed: %s %v", project, err)
	}
	_, err = Generate(root, target, "hono-bun", "pet", "", nil)
	if err == nil {
		t.Fatal("overwrote an existing project")
	}
	unchanged, _ := ReadLock(target)
	if unchanged != lock {
		t.Fatal("rejected generation changed project")
	}
	second, err := Generate(root, filepath.Join(t.TempDir(), "second-api"), "hono-bun", "pet", "", nil)
	if err != nil || second.Revision != lock.Revision {
		t.Fatalf("local snapshot digest is not reproducible: %v", err)
	}
}

func TestGenerateFailuresLeaveNoTargetOrStagingFiles(t *testing.T) {
	for _, scenario := range []string{"check fails", "collision", "symlink", "template ancestor symlink", "secret", "unknown template", "invalid profile", "invalid name", "target race"} {
		t.Run(scenario, func(t *testing.T) {
			root, parent := sourceFixture(t), t.TempDir()
			target, id, profile := filepath.Join(parent, "my-api"), "hono-bun", "pet"
			var verify func(string, Template) error
			switch scenario {
			case "check fails":
				verify = func(string, Template) error { return errors.New("test failed") }
			case "collision":
				writeFile(t, filepath.Join(root, "templates/hono-bun/AGENTS.md"), "collision")
			case "symlink":
				if err := os.Symlink(filepath.Join(root, "catalog.json"), filepath.Join(root, "templates/hono-bun/link")); err != nil {
					t.Skip(err)
				}
			case "template ancestor symlink":
				templates := filepath.Join(root, "templates")
				if err := os.Rename(templates, filepath.Join(root, "other")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "other"), templates); err != nil {
					t.Skip(err)
				}
			case "secret":
				writeFile(t, filepath.Join(root, "common/.babysit/.env"), "DO_NOT_SHIP=placeholder")
			case "unknown template":
				id = "missing"
			case "invalid profile":
				profile = "unknown"
			case "invalid name":
				target = filepath.Join(parent, "bad name")
			case "target race":
				verify = func(string, Template) error { writeFile(t, filepath.Join(target, "sentinel"), "preserve"); return nil }
			}
			if _, err := Generate(root, target, id, profile, "", verify); err == nil {
				t.Fatal("expected generation rejection")
			}
			entries, err := os.ReadDir(parent)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "target race" {
				if b, _ := os.ReadFile(filepath.Join(target, "sentinel")); string(b) != "preserve" || len(entries) != 1 {
					t.Fatal("race overwrote target or leaked staging")
				}
			} else if len(entries) != 0 {
				t.Fatalf("left files behind after failure: %v", entries)
			}
		})
	}
}

func TestCatalogRejectsTraversalAndUnsupportedVersions(t *testing.T) {
	for _, path := range []string{"../outside", "/tmp/outside", "templates/../../outside", `templates\outside`, "templates/../outside"} {
		catalog := catalogFixture()
		entry := catalog.Templates["hono-bun"]
		entry.Path = path
		catalog.Templates["hono-bun"] = entry
		b, _ := json.Marshal(catalog)
		if _, err := ParseCatalog(b); err == nil {
			t.Fatalf("accepted unsafe template path %q", path)
		}
	}
	for _, version := range []string{"1.2", "1.2.3-beta", "v1.2.3", "01.2.3", "99999999999999.2.3"} {
		if ValidVersion(version) {
			t.Fatalf("accepted unsupported version %q", version)
		}
	}
	if CompareVersion("0.10.0", "0.9.9") <= 0 || CompareVersion("1.0.0", "0.99.99") <= 0 {
		t.Fatal("versions compared lexically")
	}
}

func TestRelevantUpdatesAndVersionOrdering(t *testing.T) {
	lock := Lock{Source: Source, Release: "0.1.0", Template: "hono-bun", TemplateRevision: 1}
	for _, tc := range []struct {
		version  string
		revision int
		status   string
	}{
		{"0.1.0", 1, "up_to_date"}, {"0.2.0", 1, "up_to_date"},
		{"0.2.0", 2, "update_available"}, {"0.0.9", 2, "ahead"},
	} {
		c := catalogFixture()
		c.Version = tc.version
		entry := c.Templates["hono-bun"]
		entry.Revision = tc.revision
		c.Templates["hono-bun"] = entry
		result := Compare(lock, Release{Catalog: c, Revision: strings.Repeat("a", 40)})
		if result.Status != tc.status {
			t.Fatalf("%+v: %+v", tc, result)
		}
		if tc.status == "update_available" && !strings.Contains(result.UpgradeURL, strings.Repeat("a", 40)) {
			t.Fatal("upgrade guide was not pinned")
		}
	}
	c := catalogFixture()
	delete(c.Templates, "hono-bun")
	if got := Compare(lock, Release{Catalog: c}); got.Status != "template_missing" || got.Warning == "" {
		t.Fatalf("missing template not surfaced: %+v", got)
	}
}

func TestReleaseCheckCachesAndFallsBackWithoutChangingProject(t *testing.T) {
	root := sourceFixture(t)
	project := filepath.Join(t.TempDir(), "my-api")
	lock, err := Generate(root, project, "hono-bun", "pet", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(project, LockPath))
	c := catalogFixture()
	c.Version = "0.2.0"
	entry := c.Templates["hono-bun"]
	entry.Revision = 2
	c.Templates["hono-bun"] = entry
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	calls := 0
	fetch := func(context.Context, string) (Release, error) {
		calls++
		return Release{Catalog: c, Revision: strings.Repeat("a", 40)}, nil
	}
	first := Check(context.Background(), lock, cachePath, false, fetch)
	second := Check(context.Background(), lock, cachePath, false, fetch)
	if first.Status != "update_available" || first.Cached || !second.Cached || calls != 1 {
		t.Fatalf("cache failed: %+v %+v %d", first, second, calls)
	}
	failure := func(context.Context, string) (Release, error) { calls++; return Release{}, errors.New("offline") }
	stale := Check(context.Background(), lock, cachePath, true, failure)
	if !stale.Stale || !stale.Cached || stale.Warning == "" || stale.Status != "update_available" || calls != 2 {
		t.Fatalf("offline lost cached release: %+v", stale)
	}
	throttled := Check(context.Background(), lock, cachePath, false, failure)
	if !throttled.Cached || calls != 2 {
		t.Fatalf("offline check repeated network work: %+v %d", throttled, calls)
	}
	unknown := Check(context.Background(), lock, filepath.Join(t.TempDir(), "missing/cache.json"), false, failure)
	if unknown.Status != "unknown" || unknown.Warning == "" || unknown.CheckedAt != nil {
		t.Fatalf("offline reported success without evidence: %+v", unknown)
	}
	after, _ := os.ReadFile(filepath.Join(project, LockPath))
	if string(before) != string(after) {
		t.Fatal("release check rewrote project provenance")
	}
	oldCache := catalogCache{Catalog: c, Revision: strings.Repeat("a", 40), CheckedAt: time.Now().Add(-25 * time.Hour), AttemptedAt: time.Now().Add(-25 * time.Hour)}
	if err := writeCache(cachePath, oldCache); err != nil {
		t.Fatal(err)
	}
	refreshed := Check(context.Background(), lock, cachePath, false, fetch)
	if refreshed.Cached {
		t.Fatal("expired cache was not refreshed")
	}
}

func TestRemoteFetchPinsCatalogAndRejectsInvalidReleases(t *testing.T) {
	for _, scenario := range []string{"stable", "prerelease", "tag mismatch", "catalog mismatch", "bad commit", "HTTP error", "oversize"} {
		t.Run(scenario, func(t *testing.T) {
			sha := strings.Repeat("a", 40)
			sawPinned := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "HTTP error" {
					w.WriteHeader(503)
					return
				}
				switch {
				case strings.Contains(r.URL.Path, "/releases/"):
					tag := "v0.1.0"
					if scenario == "tag mismatch" {
						tag = "v0.2.0"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "prerelease": scenario == "prerelease"})
				case strings.Contains(r.URL.Path, "/commits/"):
					commit := sha
					if scenario == "bad commit" {
						commit = "main"
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"sha": commit})
				case strings.HasSuffix(r.URL.Path, "/catalog.json"):
					sawPinned = strings.Contains(r.URL.Path, "/"+sha+"/")
					c := catalogFixture()
					if scenario == "catalog mismatch" {
						c.Version = "0.2.0"
					}
					if scenario == "oversize" {
						_, _ = w.Write([]byte(strings.Repeat("x", (1<<20)+1)))
						return
					}
					_ = json.NewEncoder(w).Encode(c)
				default:
					t.Errorf("unexpected release URL: %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client := Client{HTTP: server.Client(), API: server.URL, Raw: server.URL}
			release, err := client.Fetch(context.Background(), "0.1.0")
			if scenario == "stable" {
				if err != nil || !sawPinned || release.Revision != sha {
					t.Fatalf("unverified release: %+v %v", release, err)
				}
			} else if err == nil {
				t.Fatalf("accepted invalid release %s", scenario)
			}
		})
	}
}
