package foreman

import (
	"strings"
	"time"

	"github.com/reallongnguyen/babysit/internal/orca"
)

// QuotaWindowEvidence is sanitized per-window evidence suitable for a durable
// ticket/Dispatch handoff. No account identifiers or raw account-list output
// are retained.
type QuotaWindowEvidence struct {
	Name          string  `json:"name"`
	UsedPercent   float64 `json:"usedPercent"`
	WindowMinutes int     `json:"windowMinutes"`
	ResetsAt      string  `json:"resetsAt"`
}

// QuotaPoolEvidence keeps the opaque, non-secret pool key and its authority,
// freshness, source timestamp, and all applicable windows.
type QuotaPoolEvidence struct {
	PoolID        string                `json:"poolId"`
	Provider      string                `json:"provider"`
	HostID        string                `json:"hostId"`
	Status        string                `json:"status"`
	Freshness     string                `json:"freshness"`
	Authoritative bool                  `json:"authoritative"`
	ObservedAt    string                `json:"observedAt"`
	Windows       []QuotaWindowEvidence `json:"windows,omitempty"`
}

// QuotaAdmission is the decision and its audit evidence. Unknown is distinct
// from available: unknown never blocks a launch but is never described as
// unlimited or available.
type QuotaAdmission struct {
	Status     string              `json:"status"`
	Reason     string              `json:"reason"`
	HostID     string              `json:"hostId,omitempty"`
	Agent      string              `json:"agent,omitempty"`
	ObservedAt []string            `json:"observedAt,omitempty"`
	RecheckAt  string              `json:"recheckAt,omitempty"`
	Pools      []QuotaPoolEvidence `json:"pools,omitempty"`
}

// AssessQuota defers only when every mapped pool is fresh, authoritative, and
// has at least one still-exhausted window. Each unknown mapping makes the
// snapshot inconclusive, so the existing CPU/RAM and worker limits still apply.
func AssessQuota(discovery *orca.AgentDiscovery, agent, destinationHost string, now time.Time) QuotaAdmission {
	result := QuotaAdmission{Status: "unknown", Agent: agent}
	if discovery == nil {
		result.Reason = "agent-discovery-unavailable"
		return result
	}
	result.HostID = discovery.HostID
	if destinationHost != "" && destinationHost != discovery.HostID {
		result.Reason = "destination-host-mismatch"
		return result
	}
	if strings.TrimSpace(agent) == "" {
		result.Reason = "selected-agent-unknown"
		return result
	}

	var applicable []orca.QuotaSnapshot
	for _, snapshot := range discovery.QuotaSnapshots {
		if containsFold(snapshot.AgentIDs, agent) {
			applicable = append(applicable, snapshot)
		}
	}
	if len(applicable) == 0 {
		result.Reason = "agent-pool-mapping-unavailable"
		return result
	}

	allPoolsExhausted := true
	var latestReset time.Time
	for _, snapshot := range applicable {
		pool := QuotaPoolEvidence{
			PoolID: snapshot.PoolID, Provider: snapshot.Provider, HostID: snapshot.HostID,
			Status: snapshot.Status, Freshness: snapshot.Freshness,
			Authoritative: snapshot.Authoritative, ObservedAt: snapshot.ObservedAt,
		}
		observed, err := time.Parse(time.RFC3339, snapshot.ObservedAt)
		if err != nil || observed.After(now) || snapshot.HostID != discovery.HostID ||
			snapshot.PoolID == "" || snapshot.Provider == "" || !snapshot.Authoritative ||
			snapshot.Status != "ok" || snapshot.Freshness != "fresh" || len(snapshot.Windows) == 0 {
			result.Pools = append(result.Pools, pool)
			result.ObservedAt = append(result.ObservedAt, snapshot.ObservedAt)
			result.Reason = quotaUnknownReason(snapshot, discovery.HostID)
			return result
		}
		result.ObservedAt = append(result.ObservedAt, snapshot.ObservedAt)
		poolExhausted := false
		for _, window := range snapshot.Windows {
			if window.UsedPercent == nil || window.WindowMinutes == nil || *window.WindowMinutes <= 0 ||
				*window.UsedPercent < 0 || *window.UsedPercent > 100 {
				result.Pools = append(result.Pools, pool)
				result.Reason = "quota-window-incomplete"
				return result
			}
			reset, err := time.Parse(time.RFC3339, window.ResetsAt)
			if err != nil {
				result.Pools = append(result.Pools, pool)
				result.Reason = "quota-reset-time-invalid"
				return result
			}
			pool.Windows = append(pool.Windows, QuotaWindowEvidence{
				Name: window.Name, UsedPercent: *window.UsedPercent,
				WindowMinutes: *window.WindowMinutes, ResetsAt: window.ResetsAt,
			})
			if *window.UsedPercent >= 100 && reset.After(now) {
				poolExhausted = true
				if reset.After(latestReset) {
					latestReset = reset
				}
			}
		}
		if !poolExhausted {
			allPoolsExhausted = false
		}
		result.Pools = append(result.Pools, pool)
	}
	if allPoolsExhausted {
		result.Status = "deferred"
		result.Reason = "authoritative-quota-exhausted"
		result.RecheckAt = latestReset.Format(time.RFC3339)
		return result
	}
	result.Status = "available"
	result.Reason = "no-exhausted-mapped-window"
	return result
}

func quotaUnknownReason(snapshot orca.QuotaSnapshot, discoveryHost string) string {
	switch {
	case snapshot.HostID != discoveryHost:
		return "quota-host-mismatch"
	case !snapshot.Authoritative:
		return "quota-mapping-not-authoritative"
	case snapshot.Freshness != "fresh":
		return "quota-snapshot-not-fresh"
	case snapshot.Status != "ok":
		return "quota-source-not-ok"
	default:
		return "quota-snapshot-incomplete"
	}
}

func containsFold(values []string, value string) bool {
	for _, candidate := range values {
		if sameIdentity(candidate, value) {
			return true
		}
	}
	return false
}
