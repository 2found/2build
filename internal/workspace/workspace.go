// Package workspace stores multi-repo topology inside the single babysit
// configuration file at ~/.babysit/config.yaml.
package workspace

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"

	"github.com/reallongnguyen/babysit/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	TypeMonorepo = "monorepo"
	TypePolyrepo = "polyrepo"
)

// Repo is one workspace member. GitURL is its machine-independent identity;
// Path is this machine's checkout. The remaining fields describe that checkout
// and replace the former committed <repo>/.babysit/config.yaml.
type Repo struct {
	GitURL         string  `yaml:"git_url"`
	Path           string  `yaml:"path,omitempty"`
	Role           string  `yaml:"role,omitempty"`
	HarnessVersion *string `yaml:"harness_version,omitempty"`
	Name           string  `yaml:"name,omitempty"`
	Description    string  `yaml:"description,omitempty"`
	RepoType       string  `yaml:"repo_type,omitempty"`
}

func (r Repo) Stale(current string) bool {
	return r.HarnessVersion != nil && *r.HarnessVersion != "" && *r.HarnessVersion != current
}

func (r Repo) validate() error {
	if r.GitURL == "" {
		return fmt.Errorf("repo needs a git url")
	}
	switch r.RepoType {
	case "", TypeMonorepo, TypePolyrepo:
		return nil
	default:
		return fmt.Errorf("repo_type %q: must be %s or %s", r.RepoType, TypeMonorepo, TypePolyrepo)
	}
}

// Workspace is a named repo set under the top-level workspaces mapping.
type Workspace struct {
	Version int    `yaml:"version,omitempty"`
	Name    string `yaml:"-"`
	Repos   []Repo `yaml:"repos,omitempty"`
}

type registry struct {
	Workspaces map[string]Workspace `yaml:"workspaces,omitempty"`
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func ValidName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("workspace name %q: must be letters, digits, dot, dash or underscore, starting alphanumeric", name)
	}
	return nil
}

func readRegistry() (registry, error) {
	b, err := config.Read()
	if err != nil {
		return registry{}, err
	}
	return decodeRegistry(b)
}

func decodeRegistry(b []byte) (registry, error) {
	r := registry{Workspaces: map[string]Workspace{}}
	if len(b) == 0 {
		return r, nil
	}
	if err := yaml.Unmarshal(b, &r); err != nil {
		return registry{}, fmt.Errorf("%s: %w", config.Path(), err)
	}
	if r.Workspaces == nil {
		r.Workspaces = map[string]Workspace{}
	}
	for name, w := range r.Workspaces {
		if err := ValidName(name); err != nil {
			return registry{}, err
		}
		w.Name = name
		if w.Version == 0 {
			w.Version = 1
		}
		for _, repo := range w.Repos {
			if err := repo.validate(); err != nil {
				return registry{}, fmt.Errorf("workspace %s: %w", name, err)
			}
		}
		r.Workspaces[name] = w
	}
	return r, nil
}

func updateRegistry(mutate func(map[string]Workspace) error) error {
	_, err := config.Update(func(b []byte) ([]byte, error) {
		r, err := decodeRegistry(b)
		if err != nil {
			return nil, err
		}
		if err := mutate(r.Workspaces); err != nil {
			return nil, err
		}
		return replaceWorkspaces(b, r.Workspaces)
	})
	return err
}

// replaceWorkspaces changes only the workspaces node, preserving every other
// setting and comment in config.yaml.
func replaceWorkspaces(b []byte, workspaces map[string]Workspace) ([]byte, error) {
	var doc yaml.Node
	if len(b) > 0 {
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, err
		}
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must contain a YAML mapping", config.Path())
	}
	var value yaml.Node
	if err := value.Encode(workspaces); err != nil {
		return nil, err
	}
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "workspaces" {
			root.Content[i+1] = &value
			return yaml.Marshal(&doc)
		}
	}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "workspaces"}, &value)
	return yaml.Marshal(&doc)
}

func Load(name string) (Workspace, error) {
	if err := ValidName(name); err != nil {
		return Workspace{}, err
	}
	r, err := readRegistry()
	if err != nil {
		return Workspace{}, err
	}
	w, ok := r.Workspaces[name]
	if !ok {
		return Workspace{}, fmt.Errorf("workspace %s: %w", name, fs.ErrNotExist)
	}
	return w, nil
}

func List() ([]Workspace, error) {
	r, err := readRegistry()
	if err != nil {
		return nil, err
	}
	out := make([]Workspace, 0, len(r.Workspaces))
	for _, w := range r.Workspaces {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func Create(name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	return updateRegistry(func(all map[string]Workspace) error {
		if _, exists := all[name]; !exists {
			all[name] = Workspace{Version: 1, Name: name}
		}
		return nil
	})
}

// AddRepo appends or updates a repo, keyed by git URL. The config package lock
// covers the complete read-modify-write, so concurrent workspace and settings
// commands cannot drop each other's changes.
func AddRepo(name string, repo Repo) error {
	if err := ValidName(name); err != nil {
		return err
	}
	if err := repo.validate(); err != nil {
		return fmt.Errorf("workspace %s: %w", name, err)
	}
	return updateRegistry(func(all map[string]Workspace) error {
		w := all[name]
		w.Name, w.Version = name, 1
		for i := range w.Repos {
			if w.Repos[i].GitURL == repo.GitURL {
				current := &w.Repos[i]
				if repo.Path != "" {
					current.Path = repo.Path
				}
				if repo.Role != "" {
					current.Role = repo.Role
				}
				if repo.HarnessVersion != nil {
					current.HarnessVersion = repo.HarnessVersion
				}
				if repo.Name != "" {
					current.Name = repo.Name
				}
				if repo.Description != "" {
					current.Description = repo.Description
				}
				if repo.RepoType != "" {
					current.RepoType = repo.RepoType
				}
				all[name] = w
				return nil
			}
		}
		w.Repos = append(w.Repos, repo)
		all[name] = w
		return nil
	})
}

// Find matches a checkout to exactly one registered workspace by path or git
// URL. Duplicate registrations are a conflict rather than an arbitrary choice.
func Find(toplevel, gitURL string) (Workspace, Repo, bool, error) {
	all, err := List()
	if err != nil {
		return Workspace{}, Repo{}, false, err
	}
	var foundW Workspace
	var foundR Repo
	found := false
	for _, w := range all {
		for _, repo := range w.Repos {
			matches := toplevel != "" && repo.Path != "" && samePath(repo.Path, toplevel)
			matches = matches || (gitURL != "" && repo.GitURL == gitURL)
			if !matches {
				continue
			}
			if found {
				return Workspace{}, Repo{}, false, fmt.Errorf("repo matches multiple workspace entries (%s and %s); remove the duplicate from %s", foundW.Name, w.Name, config.Path())
			}
			foundW, foundR, found = w, repo, true
		}
	}
	return foundW, foundR, found, nil
}

// Used by error messages and tests.
func ConfigPath() string { return config.Path() }
