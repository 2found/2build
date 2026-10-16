package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/orca"
	"github.com/2found/2build/internal/ticket"
)

const foremanAwaitUsage = `Usage:
  bbs foreman await --prereqs <ticket>:<task>[,<ticket>:<task>...]
                    [--timeout <sec>] [--interval <sec>]

Run by a Foreman phase worker, in its own session, between its plan checkpoint
and its first edit. Blocks until every prerequisite phase has settled: its Orca
Dispatch completed and its ticket passes readiness for review (or for push,
when the phase recorded deferred heavy checks in pointers.deferred_heavy).

Prints one PREREQ line per prerequisite and a final AWAIT line:
  AWAIT=ready      exit 0  merge each printed HEAD, in the order given
  AWAIT=not-ready  exit 3  a prerequisite settled without passing; report it
  AWAIT=timeout    exit 4  still running; run the same command again

--timeout defaults to 540 seconds so one call fits an agent's foreground
command limit; --interval (default 20) is the poll period.
`

// Dependent phases launch in the same batch as their prerequisites and wait
// here, inside their own session, so the Foreman is not woken once per DAG
// wave just to start the next phase. The check is the same mechanical gate the
// Foreman's light verification applies — a settled Dispatch plus readiness on
// the exact head — and the Foreman still verifies every phase before delivery.

type prereq struct{ ticket, task string }

type prereqState struct {
	State  string // waiting | ready | light-ready | not-ready
	Head   string
	Reason string
}

type prereqProbe func(p prereq) prereqState

func foremanAwait(args []string) error {
	_, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	prereqs, err := parsePrereqs(kv["prereqs"])
	if err != nil {
		return fmt.Errorf("foreman await: %w\n%s", err, foremanAwaitUsage)
	}
	timeout, err := awaitSeconds(kv, "timeout", 540)
	if err != nil {
		return err
	}
	interval, err := awaitSeconds(kv, "interval", 20)
	if err != nil {
		return err
	}
	c, err := orca.Preflight()
	if err != nil {
		return fmt.Errorf("foreman await: %w", err)
	}
	if !c.Orchestration() {
		return fmt.Errorf("foreman await: %w", orca.ErrNoOrchestration)
	}
	primary := baseOpsPrimary("foreman await")
	if primary == "" {
		os.Exit(2)
	}
	probe := liveProbe(c, primary, resolveProject())
	code := awaitPrereqs(prereqs, probe, timeout, interval, time.Sleep, time.Now, os.Stdout)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func parsePrereqs(raw string) ([]prereq, error) {
	var out []prereq
	seen := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item == "" {
			continue
		}
		t, task, ok := strings.Cut(item, ":")
		t, task = strings.TrimSpace(t), strings.TrimSpace(task)
		if !ok || t == "" || task == "" {
			return nil, fmt.Errorf("--prereqs entry %q is not <ticket>:<task>", item)
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, prereq{ticket: t, task: task})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--prereqs is required")
	}
	return out, nil
}

func awaitSeconds(kv map[string]string, key string, def int) (time.Duration, error) {
	v, ok := kv[key]
	if !ok {
		return time.Duration(def) * time.Second, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("foreman await: --%s must be a positive number of seconds", key)
	}
	return time.Duration(n) * time.Second, nil
}

// awaitPrereqs polls until every prerequisite is ready, any one settled
// without passing, or the timeout expires. It prints a line only when a
// prerequisite's state changes, so a long wait costs the worker's context
// almost nothing, then the final state of every prerequisite.
func awaitPrereqs(prereqs []prereq, probe prereqProbe, timeout, interval time.Duration, sleep func(time.Duration), now func() time.Time, out io.Writer) int {
	deadline := now().Add(timeout)
	last := map[string]prereqState{}
	for {
		ready, failed := 0, false
		for _, p := range prereqs {
			s := probe(p)
			if prev, ok := last[p.ticket]; !ok || prev.State != s.State || prev.Reason != s.Reason {
				fmt.Fprintf(out, "%s %s %s\n", now().UTC().Format("15:04:05"), p.ticket, describePrereq(s))
			}
			last[p.ticket] = s
			switch s.State {
			case "ready", "light-ready":
				ready++
			case "not-ready":
				failed = true
			}
		}
		result, code := "", 0
		switch {
		case failed:
			result, code = "not-ready", 3
		case ready == len(prereqs):
			result = "ready"
		case !now().Add(interval).Before(deadline):
			result, code = "timeout", 4
		}
		if result == "" {
			sleep(interval)
			continue
		}
		for _, p := range prereqs {
			s := last[p.ticket]
			line := fmt.Sprintf("PREREQ=%s TASK=%s STATE=%s", p.ticket, p.task, s.State)
			if s.Head != "" {
				line += " HEAD=" + s.Head
			}
			if s.Reason != "" {
				line += " REASON=" + strconv.Quote(s.Reason)
			}
			fmt.Fprintln(out, line)
		}
		fmt.Fprintln(out, "AWAIT="+result)
		switch result {
		case "ready":
			fmt.Fprintln(out, "NEXT=merge each HEAD in the order listed (git merge --no-edit <HEAD>), then reconcile the plan with the merged code")
		case "not-ready":
			fmt.Fprintln(out, "NEXT=do not merge; report worker_done BLOCKED naming the prerequisite — Foreman sends this session a refresh when it is repaired")
		default:
			fmt.Fprintln(out, "NEXT=run the same command again")
		}
		return code
	}
}

func describePrereq(s prereqState) string {
	d := s.State
	if s.Reason != "" {
		d += " (" + s.Reason + ")"
	}
	return d
}

// liveProbe reads one prerequisite's settlement: the Orca Dispatch first, so
// readiness is only judged once its worker has stopped writing, then the same
// readiness evaluator land uses against the ticket's own worktree.
func liveProbe(c *orca.Client, primary string, env identity.Env) prereqProbe {
	return func(p prereq) prereqState {
		d, err := c.DispatchStateFor(p.task)
		switch {
		case err != nil:
			return prereqState{State: "waiting", Reason: "dispatch unreadable: " + firstLineOf(err.Error())}
		case d.ID == "":
			return prereqState{State: "waiting", Reason: "not dispatched yet"}
		case !dispatchTerminal(d.Status):
			return prereqState{State: "waiting", Reason: "dispatch " + d.Status}
		case d.Status != "completed":
			return prereqState{State: "not-ready", Reason: "dispatch " + d.Status}
		}
		enforced, ready, reasons, head, err := ticketV2Readiness(primary, env, p.ticket, "review")
		if err != nil {
			return prereqState{State: "not-ready", Reason: "readiness: " + firstLineOf(err.Error())}
		}
		if ready {
			return prereqState{State: "ready", Head: prereqHead(primary, p.ticket, enforced, head)}
		}
		deferred := ticket.ReadDoc(storeForTicket(env, p.ticket).IndexPath()).Get("pointers.deferred_heavy")
		if deferred != "" {
			if enforced, ok, _, head, err := ticketV2Readiness(primary, env, p.ticket, "push"); err == nil && ok {
				return prereqState{State: "light-ready", Head: prereqHead(primary, p.ticket, enforced, head), Reason: "deferred heavy: " + deferred}
			}
		}
		return prereqState{State: "not-ready", Reason: strings.Join(reasons, ",")}
	}
}

// prereqHead is the verified head v2 readiness returns; a legacy ticket has
// none, so its branch tip is the revision its verdicts were written against.
func prereqHead(primary, t string, enforced bool, head string) string {
	if enforced && head != "" {
		return head
	}
	if b := ticketBranch(t); b != "" {
		return gitOutIn(primary, "rev-parse", b)
	}
	return head
}
