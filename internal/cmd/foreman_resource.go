package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/2found/2build/internal/config"
	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/orca"
)

const foremanResourceUsage = `Usage:
  bbs foreman resource status
  bbs foreman resource reserve <foreman-id> --ticket <ticket> --task <task> --profile <profile> [--agent <id>] [--host <host-id>]
  bbs foreman resource release <lease-id>

Profiles: plan, standard, android-simulator, ios-simulator, local-ml
status prints FOREMAN_MAX_WORKERS: parallel_max_workers, or min(16, max(2, CPUs-2)) when unset/auto.
`

var newResourceBroker = foreman.DefaultResourceBroker

func foremanResource(args []string) error {
	if len(args) == 0 {
		fmt.Print(foremanResourceUsage)
		return nil
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "status":
		return foremanResourceStatus(rest)
	case "reserve":
		return foremanResourceReserve(rest)
	case "release":
		return foremanResourceRelease(rest)
	case "help", "--help", "-h":
		fmt.Print(foremanResourceUsage)
		return nil
	}
	return fmt.Errorf("foreman resource: unknown subcommand %q\n%s", sub, foremanResourceUsage)
}

func foremanResourceStatus(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("foreman resource status: unexpected arguments\n%s", foremanResourceUsage)
	}
	capUnits, err := configuredResourceCap()
	if err != nil {
		return err
	}
	broker := newResourceBroker()
	status, err := broker.Status(capUnits)
	if err != nil {
		return err
	}
	maxWorkers, err := configuredMaxWorkers(status.Host.CPUs)
	if err != nil {
		return err
	}
	// status is the reconcile entry point every foreman wake runs, so the
	// cross-check lives here rather than in skill prose a caller can skip: a
	// lease whose Dispatch has settled is released on the spot.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := orca.PreflightContext(ctx)
	released := reconcileResourceLeases(ctx, broker, status.Leases, c, time.Now())
	if len(released) > 0 {
		status, err = broker.Status(capUnits)
		if err != nil {
			return err
		}
	}
	printResourceStatus(status)
	fmt.Printf("FOREMAN_MAX_WORKERS=%d\n", maxWorkers)
	for _, id := range released {
		fmt.Println("RELEASED_LEASE=" + id)
	}
	return nil
}

// Reconciliation is global and runs before admission, even if the original
// foreman vanished. No Orca I/O happens while the broker lock is held. Orca
// is needed only to see whether an open Dispatch's worker exited; without it
// those leases are held.
func reconcileResourceLeases(ctx context.Context, b *foreman.ResourceBroker, leases []foreman.ResourceLease, c *orca.Client, now time.Time) []string {
	if len(leases) == 0 {
		return nil
	}
	var released []string
	var terms []orca.Terminal
	termsErr := errors.New("Orca runtime unavailable")
	termsRead := false
	lastChecked := ""
	defer func() {
		if lastChecked != "" {
			if err := b.AdvanceReconciliation(lastChecked); err != nil {
				fmt.Fprintf(os.Stderr, "RESOURCE_RECOVERY_CHECKPOINT=%v\n", err)
			}
		}
	}()
	for _, lease := range leases {
		if ctx.Err() != nil {
			break
		}
		lastChecked = lease.ID
		dispatch, status, err := taskDispatchState(lease.Task)
		if err != nil {
			fmt.Fprintf(os.Stderr, "RESOURCE_HELD=%s REASON=%v\n", lease.ID, err)
			continue
		}
		// The terminal attempt observed before reserve belongs to the previous
		// generation, not the worker this reservation is about to launch.
		unstarted := dispatch == "" || dispatch == lease.PreviousDispatch
		reclaim := false
		if unstarted {
			lastReserved := lease.AcquiredAt
			if lease.RenewedAt != "" {
				lastReserved = lease.RenewedAt
			}
			acquired, err := time.Parse(time.RFC3339Nano, lastReserved)
			owner, ownerErr := foreman.Load(lease.ForemanID)
			heartbeat, heartbeatErr := time.Parse(time.RFC3339, owner.Heartbeat)
			stale := errors.Is(ownerErr, os.ErrNotExist) || (ownerErr == nil && (heartbeatErr != nil || now.Sub(heartbeat) >= foreman.StaleAfter))
			reclaim = err == nil && now.Sub(acquired) >= foreman.StaleAfter && stale
		} else if dispatchTerminal(status) {
			reclaim = true
		} else {
			if !termsRead && c != nil {
				terms, termsErr = c.Terminals()
			}
			termsRead = true
			if termsErr != nil {
				fmt.Fprintf(os.Stderr, "RESOURCE_HELD=%s REASON=%v\n", lease.ID, termsErr)
				continue
			}
			task, _ := foreman.LoadTask(lease.Task)
			if w, ok := taskWorkerFor(task.DispatchByID(dispatch), terms, now); ok && w.Exited {
				// Fence the exact generation, never the Task's replacement.
				_, err := foreman.UpdateTask(lease.Task, func(t *foreman.Task) error {
					if cur := t.Current(); cur == nil || cur.ID != dispatch {
						return fmt.Errorf("dispatch %s was superseded", dispatch)
					}
					return t.Current().Settle(foreman.DispatchFailed, "worker exited without reporting")
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "RESOURCE_HELD=%s REASON=%v\n", lease.ID, err)
					continue
				}
				reclaim = true
			} else {
				fmt.Fprintf(os.Stderr, "RESOURCE_HELD=%s DISPATCH=%s REASON=worker live or unverifiable\n", lease.ID, dispatch)
			}
		}
		if reclaim {
			if ok, err := b.ReleaseObserved(lease); err != nil {
				fmt.Fprintf(os.Stderr, "RESOURCE_HELD=%s REASON=%v\n", lease.ID, err)
			} else if ok {
				released = append(released, lease.ID)
			}
		}
	}
	return released
}

// dispatchTerminal is a settled Dispatch. "dispatched" and "" (no Dispatch
// yet) are not; a missing Dispatch is handled separately with launch grace
// and owner liveness evidence.
func dispatchTerminal(status string) bool { return foreman.DispatchSettled(status) }

func foremanResourceReserve(args []string) error {
	id, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("foreman resource reserve: needs a foreman id\n%s", foremanResourceUsage)
	}
	if _, err := foreman.Load(id); err != nil {
		return fmt.Errorf("foreman resource reserve: %w", err)
	}
	for _, key := range []string{"ticket", "task", "profile"} {
		if strings.TrimSpace(kv[key]) == "" {
			return fmt.Errorf("foreman resource reserve: --%s is required", key)
		}
	}
	capUnits, err := configuredResourceCap()
	if err != nil {
		return err
	}
	broker := newResourceBroker()
	before, err := broker.Status(capUnits)
	if err != nil {
		return err
	}
	maxWorkers, err := configuredMaxWorkers(before.Host.CPUs)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, _ := orca.PreflightContext(ctx)
	previous := ""
	dispatch, dispatchStatus, err := taskDispatchState(kv["task"])
	if err != nil {
		return fmt.Errorf("resource reserve: cannot inspect task: %w", err)
	}
	quotaEligible := dispatch == "" || dispatchTerminal(dispatchStatus)
	if dispatchTerminal(dispatchStatus) {
		previous = dispatch
	}
	released := reconcileResourceLeases(ctx, broker, before.Leases, c, time.Now())
	var quota foreman.QuotaAdmission
	var accounts *orca.AccountRateLimits
	accountStatus := "unavailable"
	switch {
	case quotaEligible:
		quota, accounts, accountStatus = foremanQuotaSnapshot(c, kv["agent"], kv["host"], time.Now().UTC())
	default:
		quota = foreman.QuotaAdmission{
			Status: "unknown", Reason: "dispatch-not-new-admission", Agent: kv["agent"],
		}
	}
	if quota.Status == "deferred" {
		evidence := resourceQuotaEvidence{
			Ticket: kv["ticket"], Task: kv["task"], Foreman: id, Agent: kv["agent"],
			DestinationHost: kv["host"], Quota: quota, AccountStatus: accountStatus,
			LocalAccountRateLimits: accounts, ResourceAdmission: "not-attempted",
		}
		path, err := appendForemanHandoff(kv["ticket"], kv["task"], "quota", evidence)
		if err != nil {
			return fmt.Errorf("resource reserve: cannot persist quota evidence: %w", err)
		}
		printQuotaAdmission(quota, path)
		fmt.Println("ADMISSION=deferred")
		fmt.Println("REASON=" + quota.Reason)
		return nil
	}
	status, err := broker.Reserve(foreman.ResourceRequest{
		ForemanID:        id,
		Ticket:           kv["ticket"],
		Task:             kv["task"],
		Profile:          kv["profile"],
		MaxWorkers:       maxWorkers,
		PreviousDispatch: previous,
	}, capUnits)
	if err != nil {
		evidence := resourceQuotaEvidence{
			Ticket: kv["ticket"], Task: kv["task"], Foreman: id, Agent: kv["agent"],
			DestinationHost: kv["host"], Quota: quota, AccountStatus: accountStatus,
			LocalAccountRateLimits: accounts, ResourceAdmission: "error",
		}
		if _, persistErr := appendForemanHandoff(kv["ticket"], kv["task"], "quota", evidence); persistErr != nil {
			return fmt.Errorf("resource reserve: %v (quota handoff failed: %w)", err, persistErr)
		}
		return err
	}
	printResourceStatus(status)
	evidence := resourceQuotaEvidence{
		Ticket: kv["ticket"], Task: kv["task"], Foreman: id, Agent: kv["agent"],
		DestinationHost: kv["host"], Quota: quota, AccountStatus: accountStatus,
		LocalAccountRateLimits: accounts, ResourceAdmission: status.Admission,
		ResourceReason: status.Reason,
	}
	if status.Lease != nil {
		evidence.Lease = status.Lease.ID
	}
	path, err := appendForemanHandoff(kv["ticket"], kv["task"], "quota", evidence)
	if err != nil {
		if status.Lease != nil {
			_, _ = broker.Release(status.Lease.ID)
		}
		return fmt.Errorf("resource reserve: cannot persist quota evidence: %w", err)
	}
	printQuotaAdmission(quota, path)
	for _, id := range released {
		fmt.Println("RELEASED_LEASE=" + id)
	}
	return nil
}

func foremanResourceRelease(args []string) error {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		return fmt.Errorf("foreman resource release: needs one lease id\n%s", foremanResourceUsage)
	}
	released, err := newResourceBroker().Release(args[0])
	if err != nil {
		return err
	}
	if released {
		fmt.Println("RELEASED=" + args[0])
	} else {
		fmt.Println("RELEASED=none")
	}
	return nil
}

// configuredMaxWorkers resolves parallel_max_workers: unset or auto derives
// the bound from the host CPU count; a positive integer is used as given.
func configuredMaxWorkers(cpus int) (int, error) {
	value, ok := config.Get("parallel_max_workers")
	value = strings.TrimSpace(value)
	if !ok || value == "" || value == "auto" {
		return foreman.DefaultMaxWorkers(cpus), nil
	}
	workers, err := strconv.Atoi(value)
	if err != nil || workers <= 0 {
		return 0, fmt.Errorf("config parallel_max_workers needs 'auto' or a positive integer, got %q", value)
	}
	return workers, nil
}

func configuredResourceCap() (int, error) {
	value, ok := config.Get("parallel_global_units")
	value = strings.TrimSpace(value)
	if !ok || value == "" || value == "auto" {
		return 0, nil
	}
	units, err := strconv.Atoi(value)
	if err != nil || units <= 0 {
		return 0, fmt.Errorf("config parallel_global_units needs 'auto' or a positive integer, got %q", value)
	}
	return units, nil
}

func printResourceStatus(status foreman.ResourceStatus) {
	if status.Admission != "" {
		fmt.Println("ADMISSION=" + status.Admission)
	}
	if status.Reason != "" {
		fmt.Println("REASON=" + status.Reason)
	} else {
		fmt.Println("PRESSURE=ok")
	}
	if status.Lease != nil {
		fmt.Println("LEASE=" + status.Lease.ID)
		fmt.Println("PROFILE=" + status.Lease.Profile)
		fmt.Printf("UNITS=%d\n", status.Lease.Units)
	}
	fmt.Printf("GLOBAL_BUDGET=%d\n", status.Budget)
	fmt.Printf("GLOBAL_USED=%d\n", status.Used)
	fmt.Printf("HOST_CPUS=%d\n", status.Host.CPUs)
	fmt.Printf("HOST_TOTAL_MEMORY_BYTES=%d\n", status.Host.TotalMemoryBytes)
	if status.Host.MemoryKnown {
		fmt.Printf("HOST_AVAILABLE_MEMORY_BYTES=%d\n", status.Host.AvailableMemoryBytes)
	}
	if status.Host.LoadKnown {
		fmt.Printf("HOST_LOAD_1=%.2f\n", status.Host.Load1)
	}
	for _, lease := range status.Leases {
		fmt.Printf("ACTIVE_LEASE=%s FOREMAN=%s TICKET=%s PROFILE=%s UNITS=%d TASK=%s\n",
			lease.ID, lease.ForemanID, lease.Ticket, lease.Profile, lease.Units, lease.Task)
	}
}

type resourceQuotaEvidence struct {
	Ticket                 string                  `json:"ticket"`
	Task                   string                  `json:"task"`
	Foreman                string                  `json:"foreman"`
	Agent                  string                  `json:"agent,omitempty"`
	DestinationHost        string                  `json:"destinationHost,omitempty"`
	Quota                  foreman.QuotaAdmission  `json:"quota"`
	AccountStatus          string                  `json:"accountStatus"`
	LocalAccountRateLimits *orca.AccountRateLimits `json:"localAccountRateLimits,omitempty"`
	ResourceAdmission      string                  `json:"resourceAdmission"`
	ResourceReason         string                  `json:"resourceReason,omitempty"`
	Lease                  string                  `json:"lease,omitempty"`
}

func foremanQuotaSnapshot(client *orca.Client, agent, host string, now time.Time) (foreman.QuotaAdmission, *orca.AccountRateLimits, string) {
	accountsStatus := "unavailable"
	var accounts *orca.AccountRateLimits
	if client != nil {
		if result, err := client.AccountList(); err == nil {
			accounts = &result
			accountsStatus = "read-advisory-only"
		}
	}
	if client == nil || !client.Supports(orca.CapAgentDiscovery) {
		result := foreman.AssessQuota(nil, agent, host, now)
		if client == nil {
			result.Reason = "orca-runtime-unavailable"
		} else {
			result.Reason = "agent-discovery-unavailable"
		}
		return result, accounts, accountsStatus
	}
	discovery, err := client.AgentDiscovery()
	if err != nil {
		result := foreman.AssessQuota(nil, agent, host, now)
		result.Reason = "agent-discovery-read-failed"
		return result, accounts, accountsStatus
	}
	if !client.Supports(orca.CapQuotaSnapshots) {
		discovery.QuotaSnapshots = nil
		result := foreman.AssessQuota(&discovery, agent, host, now)
		result.Reason = "quota-snapshots-unavailable"
		return result, accounts, accountsStatus
	}
	return foreman.AssessQuota(&discovery, agent, host, now), accounts, accountsStatus
}

func printQuotaAdmission(quota foreman.QuotaAdmission, handoff string) {
	fmt.Println("QUOTA_STATUS=" + quota.Status)
	fmt.Println("QUOTA_REASON=" + quota.Reason)
	if quota.HostID != "" {
		fmt.Println("QUOTA_HOST=" + quota.HostID)
	}
	if quota.Agent != "" {
		fmt.Println("QUOTA_AGENT=" + quota.Agent)
	}
	if quota.RecheckAt != "" {
		fmt.Println("QUOTA_RECHECK_AT=" + quota.RecheckAt)
	}
	fmt.Println("QUOTA_HANDOFF=" + handoff)
}
