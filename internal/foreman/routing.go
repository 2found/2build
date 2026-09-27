package foreman

import (
	"fmt"
	"strings"

	"github.com/reallongnguyen/babysit/internal/orca"
)

// RouteRequest carries the run's explicit intent and the durable route from a
// prior compatible phase. Model and Effort are the already-selected phase
// policy, not defaults to infer in the route resolver.
type RouteRequest struct {
	ExplicitAgent          string
	PinnedAgent            string
	PinnedModel            string
	PinnedEffort           string
	Model                  string
	Effort                 string
	DestinationHost        string
	ExactSession           bool
	TaskComplexity         string
	PhaseClass             string
	SelectedTier           string
	OverrideProvenance     string
	PinnedModelProvenance  string
	PinnedEffortProvenance string
}

// RouteEvidence is persisted with the task before launch; it separates what
// Foreman requested from what destination-host Orca discovery observed.
type RouteEvidence struct {
	Source                 string `json:"source"`
	RequestedAgent         string `json:"requestedAgent,omitempty"`
	PinnedAgent            string `json:"pinnedAgent,omitempty"`
	Agent                  string `json:"agent"`
	RequestedHost          string `json:"destinationHost,omitempty"`
	HostID                 string `json:"hostId,omitempty"`
	Model                  string `json:"model,omitempty"`
	Effort                 string `json:"effort,omitempty"`
	Complexity             string `json:"complexity,omitempty"`
	PhaseClass             string `json:"phaseClass,omitempty"`
	SelectedTier           string `json:"selectedTier,omitempty"`
	OverrideProvenance     string `json:"overrideProvenance,omitempty"`
	PinnedModel            string `json:"pinnedModel,omitempty"`
	PinnedModelProvenance  string `json:"pinnedModelProvenance,omitempty"`
	PinnedEffort           string `json:"pinnedEffort,omitempty"`
	PinnedEffortProvenance string `json:"pinnedEffortProvenance,omitempty"`
	ExactSession           bool   `json:"exactSession"`
	Discovery              string `json:"discovery"`
	ObservedAt             string `json:"observedAt,omitempty"`
}

// ResolveRoute applies explicit intent, then a compatible recorded route, then
// the Orca effective default. Missing discovery blocks only a new unpinned
// route; explicit and pinned routes keep working for older Orca versions.
func ResolveRoute(req RouteRequest, discovery *orca.AgentDiscovery, discoveryErr error) (RouteEvidence, error) {
	if req.ExactSession && req.PinnedAgent == "" {
		return RouteEvidence{}, fmt.Errorf("exact-session resume requires its recorded agent route")
	}
	if req.ExactSession && req.ExplicitAgent != "" && !sameIdentity(req.ExplicitAgent, req.PinnedAgent) {
		return RouteEvidence{}, fmt.Errorf("exact-session route is pinned to %q; cannot resume it as %q", req.PinnedAgent, req.ExplicitAgent)
	}
	if req.ExactSession {
		if (req.Model != "" && req.Model != req.PinnedModel) || (req.Effort != "" && req.Effort != req.PinnedEffort) {
			return RouteEvidence{}, fmt.Errorf("exact-session route settings do not match the recorded route")
		}
		if req.Model == "" {
			req.Model = req.PinnedModel
		}
		if req.Effort == "" {
			req.Effort = req.PinnedEffort
		}
	}

	route := RouteEvidence{
		RequestedAgent:         strings.TrimSpace(req.ExplicitAgent),
		PinnedAgent:            strings.TrimSpace(req.PinnedAgent),
		RequestedHost:          strings.TrimSpace(req.DestinationHost),
		Model:                  req.Model,
		Effort:                 req.Effort,
		Complexity:             strings.TrimSpace(req.TaskComplexity),
		PhaseClass:             strings.TrimSpace(req.PhaseClass),
		SelectedTier:           strings.TrimSpace(req.SelectedTier),
		OverrideProvenance:     strings.TrimSpace(req.OverrideProvenance),
		PinnedModel:            strings.TrimSpace(req.PinnedModel),
		PinnedModelProvenance:  strings.TrimSpace(req.PinnedModelProvenance),
		PinnedEffort:           strings.TrimSpace(req.PinnedEffort),
		PinnedEffortProvenance: strings.TrimSpace(req.PinnedEffortProvenance),
		ExactSession:           req.ExactSession,
	}
	if route.RequestedAgent != "" {
		route.Agent, route.Source = route.RequestedAgent, "explicit"
	} else if req.PinnedAgent != "" && (req.ExactSession || pinCompatible(req)) {
		route.Agent, route.Source = strings.TrimSpace(req.PinnedAgent), "pinned"
		if route.Model == "" {
			route.Model = req.PinnedModel
		}
		if route.Effort == "" {
			route.Effort = req.PinnedEffort
		}
	} else {
		if discoveryErr != nil || discovery == nil {
			return RouteEvidence{}, fmt.Errorf("new unpinned worker selection requires Orca agent discovery; upgrade Orca to a release advertising %s and configure a destination-host effective default (inspect `orca status --json` and `orca agent-context --json`)", orca.CapAgentDiscovery)
		}
		if strings.TrimSpace(discovery.EffectiveDefaultAgent) == "" {
			return RouteEvidence{}, fmt.Errorf("Orca agent discovery has no effective default for host %q; configure a destination-host default in Orca or pass an explicit --agent", discovery.HostID)
		}
		route.Agent, route.Source = strings.TrimSpace(discovery.EffectiveDefaultAgent), "orca-default"
	}
	if route.Agent == "" {
		return RouteEvidence{}, fmt.Errorf("selected worker agent is empty")
	}

	if discoveryErr != nil || discovery == nil {
		route.Discovery = "unavailable"
		return route, nil
	}
	if req.DestinationHost != "" && req.DestinationHost != discovery.HostID {
		return RouteEvidence{}, fmt.Errorf("Orca discovery host %q does not match destination host %q", discovery.HostID, req.DestinationHost)
	}
	agent, ok := discoveredAgent(*discovery, route.Agent)
	if !ok {
		return RouteEvidence{}, fmt.Errorf("agent %q is not advertised on destination host %q", route.Agent, discovery.HostID)
	}
	if agent.Enabled == nil || agent.Runnable == nil {
		return RouteEvidence{}, fmt.Errorf("agent %q discovery record is incomplete on host %q", route.Agent, discovery.HostID)
	}
	if !*agent.Enabled || !*agent.Runnable {
		return RouteEvidence{}, fmt.Errorf("agent %q is not enabled and runnable on destination host %q", route.Agent, discovery.HostID)
	}
	if route.Model != "" && !contains(agent.Models, route.Model) {
		return RouteEvidence{}, fmt.Errorf("agent %q does not advertise required model %q on host %q", route.Agent, route.Model, discovery.HostID)
	}
	if route.Effort != "" && (!contains(agent.Efforts, route.Effort) || !contains(agent.LaunchOverrides, "effort")) {
		return RouteEvidence{}, fmt.Errorf("agent %q does not advertise required effort %q on host %q", route.Agent, route.Effort, discovery.HostID)
	}
	if route.Model != "" && !contains(agent.LaunchOverrides, "model") {
		return RouteEvidence{}, fmt.Errorf("agent %q cannot accept the required model override on host %q", route.Agent, discovery.HostID)
	}
	route.HostID = discovery.HostID
	route.Discovery = "available"
	route.ObservedAt = discovery.ObservedAt
	return route, nil
}

func pinCompatible(req RouteRequest) bool {
	if req.Model != "" && req.Model != req.PinnedModel {
		return false
	}
	if req.Effort != "" && req.Effort != req.PinnedEffort {
		return false
	}
	return true
}

func discoveredAgent(discovery orca.AgentDiscovery, name string) (orca.DiscoveredAgent, bool) {
	for _, agent := range discovery.Agents {
		if sameIdentity(agent.ID, name) {
			return agent, true
		}
	}
	return orca.DiscoveredAgent{}, false
}

func sameIdentity(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
