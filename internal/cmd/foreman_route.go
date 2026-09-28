package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reallongnguyen/babysit/internal/config"
	"github.com/reallongnguyen/babysit/internal/foreman"
	"github.com/reallongnguyen/babysit/internal/orca"
	"github.com/reallongnguyen/babysit/internal/ticket"
)

type routeHandoff struct {
	Ticket  string                `json:"ticket"`
	Task    string                `json:"task"`
	Route   foreman.RouteEvidence `json:"route"`
	Handoff string                `json:"handoff,omitempty"`
}

type launchHandoff struct {
	Ticket       string              `json:"ticket"`
	Task         string              `json:"task"`
	Receipt      *orca.LaunchReceipt `json:"receipt,omitempty"`
	Verification string              `json:"verification"`
	Reason       string              `json:"reason,omitempty"`
	Handoff      string              `json:"handoff,omitempty"`
}

func foremanRoute(args []string) error {
	if len(args) > 0 && args[0] == "verify" {
		return foremanRouteVerify(args[1:])
	}
	_, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	for _, key := range []string{"ticket", "task"} {
		if strings.TrimSpace(kv[key]) == "" {
			return fmt.Errorf("foreman route: --%s is required", key)
		}
	}
	route, err := resolveForemanRoute(kv)
	if err != nil {
		return err
	}
	evidence := routeHandoff{Ticket: kv["ticket"], Task: kv["task"], Route: route}
	path, err := appendForemanHandoff(kv["ticket"], kv["task"], "route", evidence)
	if err != nil {
		return fmt.Errorf("foreman route: cannot persist route evidence: %w", err)
	}
	evidence.Handoff = path
	return json.NewEncoder(os.Stdout).Encode(evidence)
}

func resolveForemanRoute(kv map[string]string) (foreman.RouteEvidence, error) {
	discovery, discoveryErr := foremanAgentDiscovery()
	return resolveForemanRouteWithDiscovery(kv, discovery, discoveryErr)
}

func foremanAgentDiscovery() (*orca.AgentDiscovery, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := orca.PreflightContext(ctx)
	if err != nil {
		return nil, err
	}
	discovery, err := client.AgentDiscovery()
	if err != nil {
		return nil, err
	}
	return &discovery, nil
}

func resolveForemanRouteWithDiscovery(kv map[string]string, discovery *orca.AgentDiscovery, discoveryErr error) (foreman.RouteEvidence, error) {
	explicit := kv["agent"]
	if strings.EqualFold(strings.TrimSpace(explicit), "auto") {
		explicit = ""
	}
	return foreman.ResolveRoute(foreman.RouteRequest{
		ExplicitAgent: explicit, PinnedAgent: kv["pinned-agent"],
		PinnedModel: kv["pinned-model"], PinnedEffort: kv["pinned-effort"],
		Model: kv["model"], Effort: kv["effort"],
		DestinationHost: kv["host"], ExactSession: kv["exact-session"] != "",
		TaskComplexity: kv["complexity"], PhaseClass: kv["phase-class"],
		SelectedTier: kv["selected-tier"], OverrideProvenance: kv["override-provenance"],
		PinnedModelProvenance:  kv["pinned-model-provenance"],
		PinnedEffortProvenance: kv["pinned-effort-provenance"],
	}, discovery, discoveryErr)
}

func hasConfiguredWorkerAgent() bool {
	for _, name := range []string{"BABYSIT_WORKER_AGENT", "BABYSIT_AGENT"} {
		if value := os.Getenv(name); value != "" {
			value = strings.TrimSpace(value)
			return value != "" && !strings.EqualFold(value, "auto")
		}
	}
	value, _ := config.Get("worker_agent")
	value = strings.TrimSpace(value)
	return value != "" && !strings.EqualFold(value, "auto")
}

func foremanRouteVerify(args []string) error {
	_, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	for _, key := range []string{"ticket", "task", "agent"} {
		if strings.TrimSpace(kv[key]) == "" {
			return fmt.Errorf("foreman route verify: --%s is required", key)
		}
	}
	evidence := launchHandoff{Ticket: kv["ticket"], Task: kv["task"]}
	if kv["rate-limited"] != "" {
		evidence.Verification = "rate-limited"
		evidence.Reason = "worker-start returned a rate-limit response; re-read quota snapshots before retry"
		path, err := appendForemanHandoff(kv["ticket"], kv["task"], "launch", evidence)
		if err != nil {
			return fmt.Errorf("foreman route verify: cannot persist launch evidence: %w", err)
		}
		evidence.Handoff = path
		return json.NewEncoder(os.Stdout).Encode(evidence)
	}
	if strings.TrimSpace(kv["receipt-file"]) == "" {
		return fmt.Errorf("foreman route verify: needs --receipt-file <json> or --rate-limited")
	}
	raw, err := os.ReadFile(kv["receipt-file"])
	if err != nil {
		return fmt.Errorf("foreman route verify: read receipt: %w", err)
	}
	receipt, verifyErr := orca.ValidateLaunchReceipt(raw, orca.LaunchRequest{
		AgentID: kv["agent"], HostID: kv["host"], Model: kv["model"], Effort: kv["effort"],
	})
	if verifyErr != nil {
		evidence.Verification = "mismatch"
		evidence.Reason = verifyErr.Error()
	} else {
		evidence.Verification = "matched"
		evidence.Receipt = &receipt
	}
	path, err := appendForemanHandoff(kv["ticket"], kv["task"], "launch", evidence)
	if err != nil {
		return fmt.Errorf("foreman route verify: cannot persist launch evidence: %w", err)
	}
	evidence.Handoff = path
	if err := json.NewEncoder(os.Stdout).Encode(evidence); err != nil {
		return err
	}
	return verifyErr
}

// appendForemanHandoff writes one numbered ticket handoff under the normal
// index lock. The caller supplies only typed, sanitized routing or quota data.
func appendForemanHandoff(ticketID, taskID, kind string, evidence any) (string, error) {
	ticketID = safePathComponent("handoff", "ticket", ticketID)
	if strings.TrimSpace(taskID) == "" {
		return "", fmt.Errorf("handoff task id is empty")
	}
	env := resolveProject()
	env.Ticket = ticketID
	store := ticket.New(env)
	if err := os.MkdirAll(filepath.Join(store.Home(), "handoffs"), 0o755); err != nil {
		return "", err
	}
	if err := store.AcquireLock(); err != nil {
		return "", err
	}
	defer store.ReleaseLock()
	filename := nextHandoffSeq(store.Home()) + "-foreman-" + safePathComponent("handoff", "kind", kind) + ".md"
	path := filepath.Join(store.Home(), "handoffs", filename)
	payload, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return "", err
	}
	body := "# Foreman " + kind + " evidence\n\n```json\n" + string(payload) + "\n```\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(store.Home(), "handoffs", "LATEST"), []byte(filename+"\n"), 0o644); err != nil {
		return "", err
	}
	extra, _ := json.Marshal(map[string]string{"status": kind, "file": filename, "task": taskID})
	store.HistoryAppendExtra("handoff", "foreman", string(extra))
	return path, nil
}
