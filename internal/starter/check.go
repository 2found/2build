package starter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CheckResult struct {
	Status     string     `json:"status"`
	Source     string     `json:"source"`
	Template   string     `json:"template"`
	Current    string     `json:"current"`
	Latest     string     `json:"latest,omitempty"`
	Summary    string     `json:"summary,omitempty"`
	UpgradeURL string     `json:"upgrade_url,omitempty"`
	MinCLI     string     `json:"min_cli_version,omitempty"`
	CheckedAt  *time.Time `json:"checked_at,omitempty"`
	Cached     bool       `json:"cached"`
	Stale      bool       `json:"stale"`
	Warning    string     `json:"warning,omitempty"`
}

type catalogCache struct {
	Catalog     Catalog   `json:"catalog"`
	Revision    string    `json:"revision"`
	CheckedAt   time.Time `json:"checked_at"`
	AttemptedAt time.Time `json:"attempted_at"`
	Warning     string    `json:"warning,omitempty"`
}

func Compare(lock Lock, release Release) CheckResult {
	c := release.Catalog
	r := CheckResult{Status: "up_to_date", Source: lock.Source, Template: lock.Template, Current: lock.Release, Latest: c.Version, MinCLI: c.MinCLI}
	t, ok := c.Templates[lock.Template]
	if !ok {
		r.Status, r.Warning = "template_missing", "this template is absent from the latest catalog; project files were not changed"
		return r
	}
	if CompareVersion(c.Version, lock.Release) < 0 {
		r.Status = "ahead"
	} else if CompareVersion(c.Version, lock.Release) > 0 && t.Revision > lock.TemplateRevision {
		r.Status, r.Summary = "update_available", t.Summary
		ref := "v" + c.Version
		if commitPattern.MatchString(release.Revision) {
			ref = release.Revision
		}
		r.UpgradeURL = "https://github.com/" + Source + "/blob/" + ref + "/" + t.Upgrade
	}
	return r
}

// Check caches release data, never project decisions, so a shared cache is
// safe across templates and installed versions. Failures are advisory.
func Check(ctx context.Context, lock Lock, cachePath string, force bool, fetch func(context.Context, string) (Release, error)) CheckResult {
	now := time.Now().UTC()
	var cache catalogCache
	if b, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(b, &cache)
	}
	hasCatalog := false
	if b, err := json.Marshal(cache.Catalog); err == nil {
		_, err = ParseCatalog(b)
		hasCatalog = err == nil && commitPattern.MatchString(cache.Revision)
	}
	ttl := 24 * time.Hour
	if cache.Warning != "" {
		ttl = 15 * time.Minute
	}
	age := now.Sub(cache.AttemptedAt)
	cached := !force && age >= 0 && age < ttl && (hasCatalog || cache.Warning != "")
	if !cached {
		release, err := fetch(ctx, "")
		cache.AttemptedAt = now
		if err != nil {
			cache.Warning = fmt.Sprintf("could not check starter releases: %v", err)
		} else {
			cache = catalogCache{Catalog: release.Catalog, Revision: release.Revision, CheckedAt: now, AttemptedAt: now}
			hasCatalog = true
		}
		// A cache IO failure must not hide a successful release check or stop work.
		_ = writeCache(cachePath, cache)
	}
	result := CheckResult{Status: "unknown", Source: lock.Source, Template: lock.Template, Current: lock.Release}
	if hasCatalog {
		result = Compare(lock, Release{Catalog: cache.Catalog, Revision: cache.Revision})
	}
	result.Cached = cached || (hasCatalog && cache.Warning != "")
	if !cache.CheckedAt.IsZero() {
		result.CheckedAt = &cache.CheckedAt
	}
	if cache.Warning != "" {
		result.Warning = cache.Warning
	}
	result.Stale = hasCatalog && cache.Warning != ""
	return result
}

func writeCache(path string, cache catalogCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".starter-cache-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	encodeErr := json.NewEncoder(f).Encode(cache)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
