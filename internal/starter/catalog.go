// Package starter owns the portable starter catalog and project provenance.
package starter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const Source = "2found/2build-starters"
const LockPath = ".babysit/starter.lock.yaml"

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

type Template struct {
	Path    string     `json:"path"`
	Summary string     `json:"summary"`
	Dev     []string   `json:"dev"`
	Verify  [][]string `json:"verify"`
	// Revision changes only when this template or the common harness changes.
	Revision int    `json:"revision"`
	Upgrade  string `json:"upgrade"`
}

type Catalog struct {
	SchemaVersion int                 `json:"schema_version"`
	Source        string              `json:"source"`
	Version       string              `json:"version"`
	MinCLI        string              `json:"min_cli_version"`
	Templates     map[string]Template `json:"templates"`
}

type Lock struct {
	SchemaVersion    int    `yaml:"schema_version" json:"schema_version"`
	Source           string `yaml:"source" json:"source"`
	Release          string `yaml:"release" json:"release"`
	Template         string `yaml:"template" json:"template"`
	TemplateRevision int    `yaml:"template_revision" json:"template_revision"`
	Revision         string `yaml:"revision" json:"revision"`
}

func ParseCatalog(data []byte) (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("starter catalog: %w", err)
	}
	if c.SchemaVersion != 1 || c.Source != Source || !ValidVersion(c.Version) || !ValidVersion(c.MinCLI) || len(c.Templates) == 0 {
		return c, fmt.Errorf("unsupported or invalid starter catalog")
	}
	for id, t := range c.Templates {
		if !ValidName(id) || !safeRelative(t.Path) || !strings.HasPrefix(t.Path, "templates/") || t.Revision < 1 || t.Summary == "" || !safeRelative(t.Upgrade) || !strings.HasPrefix(t.Upgrade, "upgrades/") || len(t.Dev) == 0 || len(t.Verify) == 0 {
			return c, fmt.Errorf("invalid starter template %q", id)
		}
		for _, command := range append([][]string{t.Dev}, t.Verify...) {
			if len(command) == 0 || command[0] == "" {
				return c, fmt.Errorf("template %q has an empty command", id)
			}
		}
	}
	return c, nil
}

func ReadCatalog(root string) (Catalog, error) {
	b, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil {
		return Catalog{}, err
	}
	return ParseCatalog(b)
}

func ReadLock(project string) (Lock, error) {
	var l Lock
	b, err := os.ReadFile(filepath.Join(project, LockPath))
	if err != nil {
		return l, err
	}
	if err := yaml.Unmarshal(b, &l); err != nil {
		return l, err
	}
	if l.SchemaVersion != 1 || l.Source != Source || !ValidVersion(l.Release) || !ValidName(l.Template) || l.TemplateRevision < 1 || l.Revision == "" {
		return l, fmt.Errorf("unsupported or invalid starter lock")
	}
	return l, nil
}

// FindProject walks to the nearest starter lock without needing a Git checkout.
func FindProject(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, LockPath)); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func ValidName(name string) bool { return len(name) <= 64 && namePattern.MatchString(name) }
func ValidVersion(v string) bool {
	if !versionPattern.MatchString(v) {
		return false
	}
	for _, part := range strings.Split(v, ".") {
		if _, err := strconv.ParseUint(part, 10, 32); err != nil {
			return false
		}
	}
	return true
}

// CompareVersion accepts only stable X.Y.Z versions, the catalog's release contract.
func CompareVersion(a, b string) int {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, _ := strconv.ParseUint(x[i], 10, 32)
		bv, _ := strconv.ParseUint(y[i], 10, 32)
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func safeRelative(path string) bool {
	return path != "" && !strings.Contains(path, "\\") && !strings.Contains(path, ":") && !filepath.IsAbs(path) && filepath.ToSlash(filepath.Clean(path)) == path && path != "." && path != ".." && !strings.HasPrefix(path, "../")
}
