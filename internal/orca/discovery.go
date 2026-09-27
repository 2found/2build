package orca

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// CapAgentDiscovery gates the versioned discovery payload added by Orca
	// contract A. Older runtimes remain usable for explicit and pinned routes.
	CapAgentDiscovery = "agent.discovery.v1"
	CapQuotaSnapshots = "quota.snapshot.v1"

	discoverySchemaVersion = 1
)

var (
	ErrNoAgentDiscovery = errors.New("orca runtime does not serve " + CapAgentDiscovery)
	ErrNoQuotaSnapshots = errors.New("orca runtime does not serve " + CapQuotaSnapshots)
)

// AgentDiscovery is the minimal contract BBS consumes from the existing,
// read-only `orca agent-context --json` response. Contract A adds the
// `agentDiscovery` result when CapAgentDiscovery is advertised; no new Orca
// command is guessed or invoked by this adapter.
type AgentDiscovery struct {
	SchemaVersion         int               `json:"schemaVersion"`
	HostID                string            `json:"hostId"`
	ObservedAt            string            `json:"observedAt"`
	EffectiveDefaultAgent string            `json:"effectiveDefaultAgent"`
	Agents                []DiscoveredAgent `json:"agents"`
	QuotaSnapshots        []QuotaSnapshot   `json:"quotaSnapshots,omitempty"`
}

// DiscoveredAgent describes one selectable worker on HostID. Nil booleans
// distinguish a partial record from an explicit disabled/unrunnable value.
type DiscoveredAgent struct {
	ID              string   `json:"id"`
	Enabled         *bool    `json:"enabled"`
	Runnable        *bool    `json:"runnable"`
	Models          []string `json:"models,omitempty"`
	Efforts         []string `json:"efforts,omitempty"`
	LaunchOverrides []string `json:"launchOverrides,omitempty"`
}

// QuotaSnapshot is authoritative only when Orca explicitly supplies both the
// agent mapping and authority/freshness facts. Account-list provider summaries
// are deliberately not promoted to this type: they have no destination-host
// or account-pool mapping on their own.
type QuotaSnapshot struct {
	HostID        string        `json:"hostId"`
	PoolID        string        `json:"poolId"`
	Provider      string        `json:"provider"`
	AgentIDs      []string      `json:"agentIds"`
	Authoritative bool          `json:"authoritative"`
	Status        string        `json:"status"`
	Freshness     string        `json:"freshness"`
	ObservedAt    string        `json:"observedAt"`
	Windows       []QuotaWindow `json:"windows"`
}

// QuotaWindow is a provider or account-pool limit window. UsedPercent is a
// pointer because null means unknown, not zero usage.
type QuotaWindow struct {
	Name          string   `json:"name"`
	UsedPercent   *float64 `json:"usedPercent"`
	WindowMinutes *int     `json:"windowMinutes"`
	ResetsAt      string   `json:"resetsAt"`
}

// AgentDiscovery reads the versioned extension to agent-context only when the
// runtime advertises it. A missing capability never falls through to guessed
// commands or coordinator-local agent configuration.
func (c *Client) AgentDiscovery() (AgentDiscovery, error) {
	var d AgentDiscovery
	if !c.Supports(CapAgentDiscovery) {
		return d, ErrNoAgentDiscovery
	}
	raw, err := c.run("agent-context")
	if err != nil {
		return d, fmt.Errorf("orca agent discovery: %w", err)
	}
	return ParseAgentDiscovery(raw)
}

// QuotaSnapshots returns only the destination-host quota contract. Callers
// treat every error as unknown quota and continue under existing resource
// limits; it is never an account-switch or downgrade signal.
func (c *Client) QuotaSnapshots() ([]QuotaSnapshot, error) {
	if !c.Supports(CapQuotaSnapshots) {
		return nil, ErrNoQuotaSnapshots
	}
	d, err := c.AgentDiscovery()
	if err != nil {
		return nil, err
	}
	return d.QuotaSnapshots, nil
}

// ParseAgentDiscovery parses the `agentDiscovery` member of the public
// agent-context result. Unknown fields are ignored for forward compatibility;
// missing required fields and old schema versions are rejected.
func ParseAgentDiscovery(raw json.RawMessage) (AgentDiscovery, error) {
	var result struct {
		AgentDiscovery json.RawMessage `json:"agentDiscovery"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return AgentDiscovery{}, fmt.Errorf("agent-context: %w", err)
	}
	if len(result.AgentDiscovery) == 0 || string(result.AgentDiscovery) == "null" {
		return AgentDiscovery{}, errors.New("agent-context: missing agentDiscovery contract")
	}
	var d AgentDiscovery
	if err := json.Unmarshal(result.AgentDiscovery, &d); err != nil {
		return AgentDiscovery{}, fmt.Errorf("agent-context agentDiscovery: %w", err)
	}
	if d.SchemaVersion != discoverySchemaVersion {
		return AgentDiscovery{}, fmt.Errorf("agent-context agentDiscovery: schema version %d is unsupported", d.SchemaVersion)
	}
	if strings.TrimSpace(d.HostID) == "" {
		return AgentDiscovery{}, errors.New("agent-context agentDiscovery: missing hostId")
	}
	if _, err := time.Parse(time.RFC3339, d.ObservedAt); err != nil {
		return AgentDiscovery{}, errors.New("agent-context agentDiscovery: invalid observedAt")
	}
	seen := make(map[string]struct{}, len(d.Agents))
	for _, agent := range d.Agents {
		if strings.TrimSpace(agent.ID) == "" || agent.Enabled == nil || agent.Runnable == nil {
			return AgentDiscovery{}, errors.New("agent-context agentDiscovery: incomplete agent record")
		}
		if _, exists := seen[agent.ID]; exists {
			return AgentDiscovery{}, fmt.Errorf("agent-context agentDiscovery: duplicate agent %q", agent.ID)
		}
		seen[agent.ID] = struct{}{}
	}
	return d, nil
}

// AccountRateLimits is the typed, local-only view of `orca account list
// --json`. It cannot prove a remote worker's quota or an agent-to-pool mapping.
type AccountRateLimits struct {
	Providers map[string]AccountRateLimit `json:"providers"`
}

// AccountRateLimit contains only non-secret rate-limit facts. Account IDs,
// credentials, and raw account-list output are intentionally omitted.
type AccountRateLimit struct {
	Status    string               `json:"status"`
	Timestamp string               `json:"timestamp"`
	Windows   []AccountQuotaWindow `json:"windows"`
}

// AccountQuotaWindow keeps nullable fields nullable so callers cannot infer
// availability from a partial or malformed provider record.
type AccountQuotaWindow struct {
	UsedPercent   *float64 `json:"usedPercent"`
	WindowMinutes *int     `json:"windowMinutes"`
	ResetsAt      string   `json:"resetsAt"`
}

// AccountList reads Orca's existing local account summary. Its provider-keyed
// records are useful for evidence but are never authoritative for admission
// without a separate destination-host/pool mapping from contract A.
func (c *Client) AccountList() (AccountRateLimits, error) {
	raw, err := c.run("account", "list")
	if err != nil {
		return AccountRateLimits{}, fmt.Errorf("orca account list: %w", err)
	}
	return ParseAccountRateLimits(raw)
}

// ParseAccountRateLimits reads provider records only; rateLimits metadata that
// does not match the provider record shape is ignored rather than guessed to be
// quota data.
func ParseAccountRateLimits(raw json.RawMessage) (AccountRateLimits, error) {
	var envelope struct {
		RateLimits json.RawMessage `json:"rateLimits"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return AccountRateLimits{}, fmt.Errorf("account list: %w", err)
	}
	if len(envelope.RateLimits) == 0 || string(envelope.RateLimits) == "null" {
		return AccountRateLimits{}, errors.New("account list: missing rateLimits")
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(envelope.RateLimits, &entries); err != nil {
		return AccountRateLimits{}, fmt.Errorf("account list rateLimits: %w", err)
	}
	out := AccountRateLimits{Providers: make(map[string]AccountRateLimit)}
	for provider, entry := range entries {
		var shape map[string]json.RawMessage
		if err := json.Unmarshal(entry, &shape); err != nil {
			continue
		}
		fields := 0
		if _, ok := shape["status"]; ok {
			fields++
		}
		if _, ok := shape["timestamp"]; ok {
			fields++
		}
		if _, ok := shape["error"]; ok {
			fields++
		}
		if _, ok := shape["windows"]; ok {
			fields++
		}
		if fields < 2 {
			continue
		}
		var record AccountRateLimit
		if err := json.Unmarshal(entry, &record); err != nil {
			return AccountRateLimits{}, fmt.Errorf("account list provider %q: %w", provider, err)
		}
		out.Providers[provider] = record
	}
	return out, nil
}
