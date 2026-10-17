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
	Offload Offload                            `json:"offload"`
}

// Offload moves a new cheap-tier worker off an agent whose weekly quota runs
// hot onto Agent, which is billed elsewhere. An empty Agent disables it.
type Offload struct {
	Agent             string   `json:"agent"`
	From              []string `json:"from"`
	Tiers             []string `json:"tiers"`
	WeeklyUsedPercent float64  `json:"weeklyUsedPercent"`
}

type ModelBinding struct {
	Model  string `json:"model"`
	Effort string `json:"effort,omitempty"`
}

func defaultForemanModels() ForemanModels {
	return ForemanModels{
		// Ticket workers (critical) implement on flash and reach max only for
		// hard tasks; max is otherwise reserved for basic design.
		Routing: map[string]map[string]string{
			"simple": {"normal": "lite", "critical": "flash", "design": "pro"},
			"normal": {"normal": "flash", "critical": "flash", "design": "max"},
			"hard":   {"normal": "pro", "critical": "max", "design": "max"},
		},
		// Flash is each agent's implementation workhorse: sonnet, gpt-6.1-sol,
		// OMP's normal role. Max is opus/astra/OMP's plan role.
		Tiers: map[string]map[string]ModelBinding{
			"lite":  {"codex": {"gpt-6-luna", "high"}, "claude": {"haiku", ""}, "omp": {"@smol", ""}},
			"flash": {"codex": {"gpt-6.1-sol", "high"}, "claude": {"sonnet", "high"}, "omp": {"@normal", ""}},
			"pro":   {"codex": {"gpt-6.1-sol", "high"}, "claude": {"opus", "medium"}, "omp": {"@slow", ""}},
			"max":   {"codex": {"gpt-6-astra", "high"}, "claude": {"opus", "high"}, "omp": {"@plan", ""}},
		},
		Offload: Offload{Agent: "omp", From: []string{"claude", "codex"}, Tiers: []string{"lite", "flash"}, WeeklyUsedPercent: 70},
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
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(settings.Foreman.Models, &sections); err != nil {
		return fmt.Errorf("foreman.models: %w", err)
	}
	if sections == nil {
		return fmt.Errorf("foreman.models must be an object")
	}
	for section, raw := range sections {
		if section == "offload" {
			if err := p.overlayOffload(raw); err != nil {
				return err
			}
			continue
		}
		var entries map[string]map[string]json.RawMessage
		if (section != "routing" && section != "tiers") || json.Unmarshal(raw, &entries) != nil || entries == nil {
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
						return fmt.Errorf("routing.%s.%s needs lite, flash, pro or max", key, phase)
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
				if !validAgentName(agent) {
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

// overlayOffload merges per field; `"agent": ""` turns offload off.
func (p *ForemanModels) overlayOffload(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return fmt.Errorf("foreman.models.offload must be an object")
	}
	for field, raw := range fields {
		switch field {
		case "agent":
			var agent string
			if err := json.Unmarshal(raw, &agent); err != nil || (agent != "" && !validAgentName(agent)) {
				return fmt.Errorf("offload.agent must be an agent name or \"\"")
			}
			p.Offload.Agent = agent
		case "from":
			var agents []string
			if err := json.Unmarshal(raw, &agents); err != nil {
				return fmt.Errorf("offload.from must be a list of agent names")
			}
			for _, agent := range agents {
				if !validAgentName(agent) {
					return fmt.Errorf("offload.from: invalid agent name %q", agent)
				}
			}
			p.Offload.From = agents
		case "tiers":
			var tiers []string
			if err := json.Unmarshal(raw, &tiers); err != nil {
				return fmt.Errorf("offload.tiers must be a list of tiers")
			}
			for _, tier := range tiers {
				if p.Tiers[tier] == nil {
					return fmt.Errorf("offload.tiers: unknown model tier %q", tier)
				}
			}
			p.Offload.Tiers = tiers
		case "weeklyUsedPercent":
			var percent *float64
			if err := json.Unmarshal(raw, &percent); err != nil || percent == nil || *percent < 0 || *percent > 100 {
				return fmt.Errorf("offload.weeklyUsedPercent must be a number from 0 to 100")
			}
			p.Offload.WeeklyUsedPercent = *percent
		default:
			return fmt.Errorf("unknown offload field %q", field)
		}
	}
	return nil
}

func validAgentName(agent string) bool {
	return agent != "" && agent == strings.ToLower(strings.TrimSpace(agent))
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
		return "", ModelBinding{}, fmt.Errorf("unknown phase class %q; use normal, critical or design", phase)
	}
	binding, ok := p.Tiers[tier][agent]
	if !ok {
		return "", ModelBinding{}, fmt.Errorf("no %s model for agent %q; configure foreman.models.tiers.%s.%s in settings.json or supply an explicit phase route", tier, agent, tier, agent)
	}
	return tier, binding, nil
}

// OffloadDecision says whether a new worker moves to the offload agent.
// WeeklyUsedPercent is nil when no authoritative weekly window was observed.
type OffloadDecision struct {
	Status            string   `json:"status"`
	Reason            string   `json:"reason"`
	From              string   `json:"from,omitempty"`
	Agent             string   `json:"agent,omitempty"`
	WeeklyUsedPercent *float64 `json:"weeklyUsedPercent,omitempty"`
	Threshold         float64  `json:"threshold"`
}

// SelectOffload applies the offload rule to an already selected route. Unknown
// quota never offloads; pro and max work stays on the selected agent unless
// settings list those tiers.
func (p ForemanModels) SelectOffload(agent, tier string, weeklyUsed *float64) (OffloadDecision, ModelBinding) {
	o := p.Offload
	d := OffloadDecision{Status: "not-applied", From: agent, WeeklyUsedPercent: weeklyUsed, Threshold: o.WeeklyUsedPercent}
	switch {
	case o.Agent == "":
		d.Reason = "offload-disabled"
	case agent == o.Agent || !contains(o.From, agent):
		d.Reason = "agent-not-offloaded"
	case !contains(o.Tiers, tier):
		d.Reason = "tier-not-offloaded"
	case weeklyUsed == nil:
		d.Reason = "weekly-quota-unknown"
	case *weeklyUsed <= o.WeeklyUsedPercent:
		d.Reason = "weekly-quota-below-threshold"
	default:
		binding, ok := p.Tiers[tier][o.Agent]
		if !ok {
			d.Reason = "no-offload-binding"
			return d, ModelBinding{}
		}
		d.Status, d.Reason, d.Agent = "applied", "weekly-quota-above-threshold", o.Agent
		return d, binding
	}
	return d, ModelBinding{}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
