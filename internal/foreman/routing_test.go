package foreman

import (
	"strings"
	"testing"
	"time"

	"github.com/reallongnguyen/babysit/internal/orca"
)

func routeDiscovery(defaultAgent string) *orca.AgentDiscovery {
	return &orca.AgentDiscovery{
		SchemaVersion: 1, HostID: "host-a", ObservedAt: "2026-09-27T12:00:00Z",
		EffectiveDefaultAgent: defaultAgent,
		Agents: []orca.DiscoveredAgent{
			{ID: "codex", Enabled: new(true), Runnable: new(true), Models: []string{"tier-model"}, Efforts: []string{"high"}, LaunchOverrides: []string{"model", "effort"}},
			{ID: "claude", Enabled: new(true), Runnable: new(true), Models: []string{"tier-model"}, Efforts: []string{"high"}, LaunchOverrides: []string{"model", "effort"}},
			{ID: "disabled", Enabled: new(false), Runnable: new(true), Models: []string{"tier-model"}, LaunchOverrides: []string{"model"}},
			{ID: "unrunnable", Enabled: new(true), Runnable: new(false), Models: []string{"tier-model"}, LaunchOverrides: []string{"model"}},
		},
	}
}

func TestResolveRouteUsesExplicitPinnedThenChangingOrcaDefault(t *testing.T) {
	d := routeDiscovery("codex")
	got, err := ResolveRoute(RouteRequest{Model: "tier-model", Effort: "high"}, d, nil)
	if err != nil || got.Agent != "codex" || got.Source != "orca-default" {
		t.Fatalf("default route = %+v, %v", got, err)
	}
	d.EffectiveDefaultAgent = "claude"
	got, err = ResolveRoute(RouteRequest{Model: "tier-model", Effort: "high"}, d, nil)
	if err != nil || got.Agent != "claude" || got.Source != "orca-default" {
		t.Fatalf("changed default route = %+v, %v", got, err)
	}
	got, err = ResolveRoute(RouteRequest{ExplicitAgent: "codex", PinnedAgent: "claude", Model: "tier-model", Effort: "high"}, d, nil)
	if err != nil || got.Agent != "codex" || got.Source != "explicit" {
		t.Fatalf("explicit route = %+v, %v", got, err)
	}
	got, err = ResolveRoute(RouteRequest{PinnedAgent: "codex", PinnedModel: "tier-model", PinnedEffort: "high", Model: "tier-model", Effort: "high"}, d, nil)
	if err != nil || got.Agent != "codex" || got.Source != "pinned" {
		t.Fatalf("pinned route = %+v, %v", got, err)
	}
}

func TestResolveRouteCarriesDecisionProvenance(t *testing.T) {
	got, err := ResolveRoute(RouteRequest{
		ExplicitAgent: "codex", PinnedAgent: "codex",
		PinnedModel: "tier-model", PinnedEffort: "high",
		Model: "tier-model", Effort: "high", ExactSession: true,
		TaskComplexity: "hard", PhaseClass: "critical", SelectedTier: "max",
		OverrideProvenance:    "none",
		PinnedModelProvenance: "reviewer_model", PinnedEffortProvenance: "reviewer_effort",
	}, nil, orca.ErrNoAgentDiscovery)
	if err != nil {
		t.Fatal(err)
	}
	if got.Complexity != "hard" || got.PhaseClass != "critical" || got.SelectedTier != "max" ||
		got.OverrideProvenance != "none" || got.PinnedModel != "tier-model" ||
		got.PinnedModelProvenance != "reviewer_model" || got.PinnedEffort != "high" ||
		got.PinnedEffortProvenance != "reviewer_effort" || !got.ExactSession {
		t.Fatalf("route decision provenance = %+v", got)
	}
}

func TestResolveRouteValidatesSelectedTierWithoutChangingModel(t *testing.T) {
	for _, tier := range []string{"", "flash", "pro", "max", "ultra"} {
		got, err := ResolveRoute(RouteRequest{
			ExplicitAgent: "codex", Model: "explicit-model", SelectedTier: tier,
		}, nil, orca.ErrNoAgentDiscovery)
		if tier == "ultra" {
			if err == nil || !strings.Contains(err.Error(), "selected-tier") {
				t.Fatalf("invalid tier accepted: %+v, %v", got, err)
			}
			continue
		}
		if err != nil || got.SelectedTier != tier || got.Model != "explicit-model" {
			t.Fatalf("tier %q changed explicit route: %+v, %v", tier, got, err)
		}
	}
}

func TestResolveRouteFallsBackFromIncompatiblePinButNeverChangesExactSession(t *testing.T) {
	d := routeDiscovery("claude")
	got, err := ResolveRoute(RouteRequest{PinnedAgent: "codex", PinnedModel: "older-model", Model: "tier-model"}, d, nil)
	if err != nil || got.Agent != "claude" || got.Source != "orca-default" {
		t.Fatalf("incompatible pin route = %+v, %v", got, err)
	}
	got, err = ResolveRoute(RouteRequest{PinnedAgent: "codex", PinnedModel: "tier-model", PinnedEffort: "high", ExactSession: true}, d, nil)
	if err != nil || got.Agent != "codex" || got.Source != "pinned" || got.Model != "tier-model" || got.Effort != "high" {
		t.Fatalf("exact resume route = %+v, %v", got, err)
	}
	got, err = ResolveRoute(RouteRequest{ExplicitAgent: "codex", PinnedAgent: "codex", PinnedModel: "tier-model", PinnedEffort: "high", ExactSession: true}, d, nil)
	if err != nil || got.Agent != "codex" || got.Model != "tier-model" || got.Effort != "high" {
		t.Fatalf("explicit exact resume route = %+v, %v", got, err)
	}
	if _, err := ResolveRoute(RouteRequest{PinnedAgent: "codex", PinnedModel: "tier-model", Model: "other", ExactSession: true}, d, nil); err == nil {
		t.Fatal("exact-session resume accepted a changed model")
	}
	if _, err := ResolveRoute(RouteRequest{ExplicitAgent: "claude", PinnedAgent: "codex", ExactSession: true}, d, nil); err == nil {
		t.Fatal("exact-session resume accepted a conflicting explicit agent")
	}
}

func TestResolveRouteKeepsExplicitAndPinnedWhenDiscoveryIsUnavailable(t *testing.T) {
	for _, request := range []RouteRequest{
		{ExplicitAgent: "codex"},
		{PinnedAgent: "codex", PinnedModel: "saved-model", ExactSession: true},
	} {
		got, err := ResolveRoute(request, nil, orca.ErrNoAgentDiscovery)
		if err != nil || got.Agent != "codex" || got.Discovery != "unavailable" {
			t.Fatalf("compatibility route = %+v, %v", got, err)
		}
	}
	_, err := ResolveRoute(RouteRequest{}, nil, orca.ErrNoAgentDiscovery)
	if err == nil || !strings.Contains(err.Error(), "orca status --json") || !strings.Contains(err.Error(), "effective default") {
		t.Fatalf("unavailable new route error = %v", err)
	}
}

func TestResolveRouteRequiresRunnableHostAndModelCapabilities(t *testing.T) {
	d := routeDiscovery("codex")
	for _, tc := range []struct {
		name string
		req  RouteRequest
		d    *orca.AgentDiscovery
	}{
		{name: "disabled", req: RouteRequest{ExplicitAgent: "disabled"}, d: d},
		{name: "unrunnable", req: RouteRequest{ExplicitAgent: "unrunnable"}, d: d},
		{name: "remote mismatch", req: RouteRequest{ExplicitAgent: "codex", DestinationHost: "host-b"}, d: d},
		{name: "unsupported model", req: RouteRequest{ExplicitAgent: "codex", Model: "missing"}, d: d},
		{name: "unsupported effort", req: RouteRequest{ExplicitAgent: "codex", Effort: "low"}, d: d},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveRoute(tc.req, tc.d, nil); err == nil {
				t.Fatal("unsupported route was accepted")
			}
		})
	}
}

func quotaSnapshot(pool, provider string, agents []string, windows ...orca.QuotaWindow) orca.QuotaSnapshot {
	return orca.QuotaSnapshot{
		HostID: "host-a", PoolID: pool, Provider: provider, AgentIDs: agents,
		Authoritative: true, Status: "ok", Freshness: "fresh",
		ObservedAt: "2026-09-27T12:00:00Z", Windows: windows,
	}
}

func TestAssessQuotaDefersOnlyFreshMappedExhaustionAndRechecksAllWindows(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 30, 0, 0, time.UTC)
	d := routeDiscovery("codex")
	d.QuotaSnapshots = []orca.QuotaSnapshot{quotaSnapshot("pool-1", "codex", []string{"codex"},
		orca.QuotaWindow{Name: "hour", UsedPercent: new(100.0), WindowMinutes: new(60), ResetsAt: "2026-09-27T13:00:00Z"},
		orca.QuotaWindow{Name: "week", UsedPercent: new(100.0), WindowMinutes: new(10080), ResetsAt: "2026-09-28T12:00:00Z"},
	)}
	got := AssessQuota(d, "codex", "host-a", now)
	if got.Status != "deferred" || got.Reason != "authoritative-quota-exhausted" || got.RecheckAt != "2026-09-28T12:00:00Z" {
		t.Fatalf("exhausted admission = %+v", got)
	}
	got = AssessQuota(d, "codex", "host-a", time.Date(2026, 9, 27, 13, 30, 0, 0, time.UTC))
	if got.Status != "deferred" || got.RecheckAt != "2026-09-28T12:00:00Z" {
		t.Fatalf("one reset window was not rechecked against remaining exhaustion: %+v", got)
	}
	got = AssessQuota(d, "codex", "host-a", time.Date(2026, 9, 28, 13, 0, 0, 0, time.UTC))
	if got.Status != "available" {
		t.Fatalf("reset quota = %+v", got)
	}
}

func TestAssessQuotaRequiresEveryMappedPoolAndSupportsSharedPools(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 30, 0, 0, time.UTC)
	exhausted := orca.QuotaWindow{Name: "weekly", UsedPercent: new(100.0), WindowMinutes: new(10080), ResetsAt: "2026-09-28T12:00:00Z"}
	available := orca.QuotaWindow{Name: "weekly", UsedPercent: new(80.0), WindowMinutes: new(10080), ResetsAt: "2026-09-28T12:00:00Z"}
	d := routeDiscovery("codex")
	d.QuotaSnapshots = []orca.QuotaSnapshot{
		quotaSnapshot("shared-pool", "codex", []string{"codex", "claude"}, exhausted),
		quotaSnapshot("other-pool", "openai", []string{"codex"}, available),
	}
	got := AssessQuota(d, "codex", "host-a", now)
	if got.Status != "available" {
		t.Fatalf("one non-exhausted mapped pool must allow admission: %+v", got)
	}
	got = AssessQuota(d, "claude", "host-a", now)
	if got.Status != "deferred" || len(got.Pools) != 1 || got.Pools[0].PoolID != "shared-pool" {
		t.Fatalf("shared pool mapping = %+v", got)
	}
}

func TestAssessQuotaUnknownEvidenceNeverDefers(t *testing.T) {
	now := time.Date(2026, 9, 27, 12, 30, 0, 0, time.UTC)
	valid := quotaSnapshot("pool", "codex", []string{"codex"}, orca.QuotaWindow{Name: "week", UsedPercent: new(100.0), WindowMinutes: new(10080), ResetsAt: "2026-09-28T12:00:00Z"})
	for _, tc := range []struct {
		name string
		edit func(*orca.AgentDiscovery)
		want string
	}{
		{name: "stale", edit: func(d *orca.AgentDiscovery) { d.QuotaSnapshots[0].Freshness = "stale" }, want: "quota-snapshot-not-fresh"},
		{name: "source error", edit: func(d *orca.AgentDiscovery) { d.QuotaSnapshots[0].Status = "error" }, want: "quota-source-not-ok"},
		{name: "unsupported mapping", edit: func(d *orca.AgentDiscovery) { d.QuotaSnapshots[0].Authoritative = false }, want: "quota-mapping-not-authoritative"},
		{name: "remote host mismatch", edit: func(d *orca.AgentDiscovery) { d.QuotaSnapshots[0].HostID = "host-b" }, want: "quota-host-mismatch"},
		{name: "unmapped", edit: func(d *orca.AgentDiscovery) { d.QuotaSnapshots[0].AgentIDs = nil }, want: "agent-pool-mapping-unavailable"},
		{name: "missing usage", edit: func(d *orca.AgentDiscovery) { d.QuotaSnapshots[0].Windows[0].UsedPercent = nil }, want: "quota-window-incomplete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := routeDiscovery("codex")
			d.QuotaSnapshots = []orca.QuotaSnapshot{valid}
			tc.edit(d)
			got := AssessQuota(d, "codex", "host-a", now)
			if got.Status == "deferred" || got.Reason != tc.want {
				t.Fatalf("admission = %+v, want unknown reason %q", got, tc.want)
			}
		})
	}
	if got := AssessQuota(nil, "codex", "host-a", now); got.Status != "unknown" || got.Reason != "agent-discovery-unavailable" {
		t.Fatalf("missing discovery = %+v", got)
	}
}
