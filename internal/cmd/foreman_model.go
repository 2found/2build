package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/2found/2build/internal/config"
	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/orca"
)

// foremanModel is an offline policy lookup, not a launch or capability check.
func foremanModel(args []string) error {
	id, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	if id != "" {
		return fmt.Errorf("foreman model: unexpected argument %q", id)
	}
	for key := range kv {
		switch key {
		case "agent", "complexity", "phase-class", "dir", "json", "quota", "host":
		default:
			return fmt.Errorf("foreman model: unknown flag --%s", key)
		}
	}
	dir := kv["dir"]
	if dir == "" {
		dir, err = os.Getwd()
	} else {
		dir, err = filepath.Abs(dir)
	}
	if err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("foreman model: --dir must be a directory")
	}
	if root := gitOutIn(dir, "rev-parse", "--show-toplevel"); root != "" {
		dir = root
	}
	policy, err := config.LoadForemanModels(dir)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if kv["agent"] == "" && kv["complexity"] == "" && kv["phase-class"] == "" && kv["quota"] == "" {
		return encoder.Encode(policy)
	}
	if strings.TrimSpace(kv["agent"]) == "" {
		return fmt.Errorf("foreman model: --agent is required for a model lookup")
	}
	for _, key := range []string{"complexity", "phase-class"} {
		if strings.TrimSpace(kv[key]) == "" {
			kv[key] = "normal"
		}
	}
	agent := strings.ToLower(strings.TrimSpace(kv["agent"]))
	complexity := kv["complexity"]
	if complexity == "critical" {
		complexity = "hard"
	}
	tier, binding, err := policy.Select(agent, complexity, kv["phase-class"])
	if err != nil {
		return err
	}
	var offload *config.OffloadDecision
	if kv["quota"] != "" {
		quota, discovery := foremanModelQuota(agent, kv["host"])
		decision, offloadBinding := policy.SelectOffload(agent, tier, foreman.WeeklyUsedPercent(quota))
		if decision.Status == "applied" && !offloadAgentRunnable(discovery, decision.Agent) {
			decision.Status, decision.Reason, decision.Agent = "not-applied", "offload-agent-unavailable", ""
		}
		if decision.Status == "applied" {
			agent, binding = decision.Agent, offloadBinding
		}
		offload = &decision
	}
	return encoder.Encode(struct {
		Agent        string `json:"agent"`
		Complexity   string `json:"complexity"`
		PhaseClass   string `json:"phaseClass"`
		SelectedTier string `json:"selectedTier"`
		config.ModelBinding
		Offload *config.OffloadDecision `json:"offload,omitempty"`
	}{agent, complexity, kv["phase-class"], tier, binding, offload})
}

// foremanModelQuota reads the destination host's quota for the selected agent.
// Any Orca failure is unknown quota, which never offloads.
var foremanModelQuota = func(agent, host string) (foreman.QuotaAdmission, *orca.AgentDiscovery) {
	now := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := orca.PreflightContext(ctx)
	if err != nil {
		quota := foreman.AssessQuota(nil, agent, host, now)
		quota.Reason = "orca-runtime-unavailable"
		return quota, nil
	}
	discovery, err := client.AgentDiscovery()
	if err != nil {
		quota := foreman.AssessQuota(nil, agent, host, now)
		quota.Reason = "agent-discovery-unavailable"
		return quota, nil
	}
	if !client.Supports(orca.CapQuotaSnapshots) {
		discovery.QuotaSnapshots = nil
	}
	return foreman.AssessQuota(&discovery, agent, host, now), &discovery
}

func offloadAgentRunnable(discovery *orca.AgentDiscovery, agent string) bool {
	if discovery == nil {
		return false
	}
	for _, candidate := range discovery.Agents {
		if candidate.ID == agent {
			return candidate.Enabled != nil && *candidate.Enabled && candidate.Runnable != nil && *candidate.Runnable
		}
	}
	return false
}
