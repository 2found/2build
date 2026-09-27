package cmd

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/reallongnguyen/babysit/internal/agent"
	"github.com/reallongnguyen/babysit/internal/config"
	"gopkg.in/yaml.v3"
)

var agentSettingKeys = []string{
	"worker_agent", "worker_provider", "worker_model", "worker_effort",
	"foreman_agent", "foreman_provider", "foreman_model", "foreman_effort",
}
var errSettingsConflict = errors.New("settings changed")

type agentSettingsResponse struct {
	Path     string               `json:"path"`
	Revision string               `json:"revision"`
	Values   map[string]string    `json:"values"`
	Agents   []agent.Installation `json:"agents"`
	Detected agent.Detection      `json:"detected"`
}

func (s *dashServer) agentSettingsPath() string {
	return filepath.Join(s.stateDir, "config.yaml")
}

func readAgentSettings(path string) (*yaml.Node, map[string]string, string, error) {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, "", err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, nil, "", fmt.Errorf("cannot read %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map", HeadComment: strings.TrimSpace(string(b))}}}
	}
	if doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil, "", fmt.Errorf("%s must contain a YAML mapping", path)
	}
	// Decode also rejects duplicate keys, so editing cannot leave ambiguous
	// preferences that the CLI and browser would interpret differently.
	var fields map[string]yaml.Node
	if err := doc.Decode(&fields); err != nil {
		return nil, nil, "", err
	}
	values := make(map[string]string, len(agentSettingKeys))
	for _, key := range agentSettingKeys {
		n := fields[key]
		value := ""
		if n.Kind != 0 {
			if err := n.Decode(&value); err != nil {
				return nil, nil, "", fmt.Errorf("%s must be a string", key)
			}
		}
		values[key] = value
	}
	revision := settingsRevision(path, b)
	return &doc, values, revision, nil
}

// settingsRevision is the optimistic-concurrency token: a save only accepts a
// baseline that hashes this file's bytes exactly, and reports the hash of the
// bytes it wrote so a later external edit still conflicts.
func settingsRevision(path string, b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(append([]byte(path+"\x00"), b...)))
}

func (s *dashServer) agentSettings() (agentSettingsResponse, error) {
	path := s.agentSettingsPath()
	_, values, revision, err := readAgentSettings(path)
	if err != nil {
		return agentSettingsResponse{}, err
	}
	return agentSettingsResponse{
		Path: path, Values: values, Revision: revision,
		Agents: agent.Installations(), Detected: agent.Detect(),
	}, nil
}

func (s *dashServer) handleAgentSettings(w http.ResponseWriter, r *http.Request) {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	result, err := s.agentSettings()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *dashServer) handleSaveAgentSettings(w http.ResponseWriter, r *http.Request) {
	if !s.guard(w, r) {
		return
	}
	var req struct {
		Revision string            `json:"revision"`
		Values   map[string]string `json:"values"`
	}
	if !decode(w, r, &req) {
		return
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	path := s.agentSettingsPath()
	doc, values, revision, err := readAgentSettings(path)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Revision != revision {
		writeErr(w, http.StatusConflict, "Settings changed since you opened them. Reload settings before saving; your unsaved changes are still in the form.")
		return
	}
	if len(req.Values) != len(agentSettingKeys) {
		writeErr(w, http.StatusBadRequest, "values must contain all eight worker and foreman settings")
		return
	}
	for _, key := range agentSettingKeys {
		value, ok := req.Values[key]
		if !ok || len(value) > 1024 || strings.ContainsFunc(value, unicode.IsControl) {
			writeErr(w, http.StatusBadRequest, key+" must be a single-line string of at most 1024 bytes")
			return
		}
		values[key] = strings.TrimSpace(value)
	}

	for _, role := range []string{"worker", "foreman"} {
		name := agent.Normalize(values[role+"_agent"])
		values[role+"_agent"] = name
		if name == "" || name == "auto" {
			continue
		}
		p, err := agent.ByName(name)
		if err == nil {
			p.Provider = values[role+"_provider"]
			p.Model = values[role+"_model"]
			p.Effort = values[role+"_effort"]
			err = p.ValidateSettings()
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, role+": "+err.Error())
			return
		}
	}
	// Edit only the allowlisted nodes, retaining comments and unrelated fields.
	mapping := doc.Content[0]
	for _, key := range agentSettingKeys {
		var target *yaml.Node
		for i := 0; i < len(mapping.Content); i += 2 {
			if mapping.Content[i].Value == key {
				target = mapping.Content[i+1]
				break
			}
		}
		// Anchors and aliases couple this key to other entries: rewriting the
		// scalar would silently change every alias, and dropping the anchor
		// would dangle it. Reject instead of choosing a corruption.
		if target != nil && (target.Anchor != "" || target.Kind == yaml.AliasNode) {
			writeErr(w, http.StatusBadRequest, key+": a YAML anchor or alias is unsupported — set a plain value first")
			return
		}
		if target == nil {
			target = &yaml.Node{}
			mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, target)
		}
		*target = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: values[key],
			HeadComment: target.HeadComment, LineComment: target.LineComment, FootComment: target.FootComment}
	}
	b, err := yaml.Marshal(doc)
	if err == nil {
		_, err = config.UpdatePath(path, func(current []byte) ([]byte, error) {
			if settingsRevision(path, current) != req.Revision {
				return nil, errSettingsConflict
			}
			return b, nil
		})
	}
	if errors.Is(err, errSettingsConflict) {
		writeErr(w, http.StatusConflict, "Settings changed since you opened them. Reload settings before saving; your unsaved changes are still in the form.")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"values": values, "revision": settingsRevision(path, b),
	})
}
