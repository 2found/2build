package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ForemanModels owns the task/phase policy; agent discovery and launch
// capability checks remain the responsibility of the route resolver.
type ForemanModels struct {
	Routing map[string]map[string]string       `json:"routing"`
	Tiers   map[string]map[string]ModelBinding `json:"tiers"`
}

type ModelBinding struct {
	Model  string `json:"model"`
	Effort string `json:"effort,omitempty"`
}

func defaultForemanModels() ForemanModels {
	return ForemanModels{
		Routing: map[string]map[string]string{
			"simple": {"normal": "flash", "critical": "flash"},
			"normal": {"normal": "flash", "critical": "pro"},
			"hard":   {"normal": "pro", "critical": "max"},
		},
		Tiers: map[string]map[string]ModelBinding{
			"flash": {"codex": {"gpt-6-luna", "high"}, "claude": {"opus", "high"}, "omp": {"@normal", ""}},
			"pro":   {"codex": {"gpt-5.6-sol", "high"}, "claude": {"opus", "high"}, "omp": {"@slow", ""}},
			"max":   {"codex": {"gpt-6-astra", "high"}, "claude": {"opus", "high"}, "omp": {"@plan", ""}},
		},
	}
}

// LoadForemanModels overlays individual fields, global first then repo.
// Other settings namespaces are ignored. Missing files use defaults; invalid
// policy is an error, never a silent fallback to a different model.
func LoadForemanModels(repo string) (ForemanModels, error) {
	policy := defaultForemanModels()
	paths := []string{filepath.Join(Dir(), "settings.json")}
	if repo != "" {
		paths = append(paths, filepath.Join(repo, ".babysit", "settings.json"))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err == nil {
			err = policy.overlay(data)
		}
		if err != nil {
			return ForemanModels{}, fmt.Errorf("settings %s: %w", path, err)
		}
	}
	return policy, nil
}

func (p *ForemanModels) overlay(data []byte) error {
	var settings *struct {
		Foreman struct {
			Models json.RawMessage `json:"models"`
		} `json:"foreman"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return err
	}
	if settings == nil {
		return fmt.Errorf("settings must be an object")
	}
	if len(settings.Foreman.Models) == 0 {
		return nil
	}
	var sections map[string]map[string]map[string]json.RawMessage
	if err := json.Unmarshal(settings.Foreman.Models, &sections); err != nil {
		return fmt.Errorf("foreman.models: %w", err)
	}
	if sections == nil {
		return fmt.Errorf("foreman.models must be an object")
	}
	for section, entries := range sections {
		if (section != "routing" && section != "tiers") || entries == nil {
			return fmt.Errorf("invalid foreman.models section %q", section)
		}
		for key, values := range entries {
			if values == nil {
				return fmt.Errorf("foreman.models.%s.%s must be an object", section, key)
			}
			if section == "routing" {
				phases, ok := p.Routing[key]
				if !ok {
					return fmt.Errorf("unknown task complexity %q", key)
				}
				for phase, raw := range values {
					var tier string
					if err := json.Unmarshal(raw, &tier); err != nil || p.Tiers[tier] == nil {
						return fmt.Errorf("routing.%s.%s needs flash, pro or max", key, phase)
					}
					if _, ok := phases[phase]; !ok {
						return fmt.Errorf("unknown phase class %q", phase)
					}
					phases[phase] = tier
				}
				continue
			}
			agents, ok := p.Tiers[key]
			if !ok {
				return fmt.Errorf("unknown model tier %q", key)
			}
			for agent, raw := range values {
				if agent == "" || agent != strings.ToLower(strings.TrimSpace(agent)) {
					return fmt.Errorf("invalid agent name %q", agent)
				}
				var fields map[string]*string
				if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
					return fmt.Errorf("tiers.%s.%s needs a model/effort object", key, agent)
				}
				binding := agents[agent]
				for field, value := range fields {
					if value == nil {
						return fmt.Errorf("tiers.%s.%s.%s must be a string", key, agent, field)
					}
					switch field {
					case "model":
						binding.Model = *value
					case "effort":
						binding.Effort = *value
					default:
						return fmt.Errorf("unknown model binding field %q", field)
					}
				}
				if strings.TrimSpace(binding.Model) == "" {
					return fmt.Errorf("tiers.%s.%s.model must not be empty", key, agent)
				}
				agents[agent] = binding
			}
		}
	}
	return nil
}

func (p ForemanModels) Select(agent, complexity, phase string) (string, ModelBinding, error) {
	if complexity == "critical" { // legacy task complexity
		complexity = "hard"
	}
	phases, ok := p.Routing[complexity]
	if !ok {
		return "", ModelBinding{}, fmt.Errorf("unknown task complexity %q; use simple, normal or hard", complexity)
	}
	tier, ok := phases[phase]
	if !ok {
		return "", ModelBinding{}, fmt.Errorf("unknown phase class %q; use normal or critical", phase)
	}
	binding, ok := p.Tiers[tier][agent]
	if !ok {
		return "", ModelBinding{}, fmt.Errorf("no %s model for agent %q; configure foreman.models.tiers.%s.%s in settings.json or supply an explicit phase route", tier, agent, tier, agent)
	}
	return tier, binding, nil
}
