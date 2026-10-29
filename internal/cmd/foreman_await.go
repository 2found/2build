package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

const foremanAwaitUsage = `Usage:
  bbs team await --prereqs <ticket>:<task>[,<ticket>:<task>...]
                    [--timeout <sec>] [--interval <sec>]

Run by a Foreman phase worker, in its own session, between its plan checkpoint
and its first edit. Blocks until every prerequisite phase has settled: its
Task's Dispatch completed and its ticket passes readiness for review (or for push,
when the phase recorded deferred heavy checks in pointers.deferred_heavy).

When every prerequisite is ready, their work (each a snapshot of the
prerequisite's uncommitted worktree) is merged into this worktree as
uncommitted changes. Nothing is committed and HEAD never moves.

Prints one PREREQ line per prerequisite and a final AWAIT line:
  AWAIT=ready      exit 0  prerequisites' work is in this worktree
  AWAIT=conflict   exit 5  adopted with conflict markers in the CONFLICT files;
                           resolve them before the first edit
  AWAIT=not-ready  exit 3  a prerequisite settled without passing; report it
  AWAIT=timeout    exit 4  still running; run the same command again

A Task not dispatched yet, or a readiness read failing fewer than three times
in a row, is still waiting. --timeout defaults to 540 seconds so one call fits an agent's foreground
command limit, shortened to end a minute inside the agent's prompt-cache
lifetime (config foreman_cache_ttl) so the re-run reads the cache; --interval
(default 20) is the poll period.
`

// Dependent phases launch in the same batch as their prerequisites and wait
// here, inside their own session, so the Foreman is not woken once per DAG
// wave just to start the next phase. The check is the same mechanical gate the
// Foreman's light verification applies — a settled Dispatch plus readiness on
// the exact head — and the Foreman still verifies every phase before delivery.

type prereq struct{ ticket, task string }

const (
	awaitReadinessAttempts = 3
)

type prereqState struct {
	State  string // waiting | ready | light-ready | not-ready
	Head   string
	Reason string
}

type prereqProbe func(p prereq) prereqState

// adoptFunc merges ready prerequisites' snapshots into the waiting worktree
// and returns the paths left with conflict markers.
type adoptFunc func(heads []string) ([]string, error)

func foremanAwait(args []string) error {
	_, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	prereqs, err := parsePrereqs(kv["prereqs"])
	if err != nil {
		return fmt.Errorf("foreman await: %w\n%s", err, foremanAwaitUsage)
	}
	timeout, err := awaitSeconds(kv, "timeout", blockingWaitSeconds())
	if err != nil {
		return err
	}
	interval, err := awaitSeconds(kv, "interval", 20)
	if err != nil {
		return err
	}
	primary := baseOpsPrimary("foreman await")
	if primary == "" {
		os.Exit(2)
	}
	// A dependent waiting here is proof a batch is running: restart its
	// watcher if it died, or nobody wakes the Foreman when the batch settles.
	ensureWatcher()
	env := resolveProject()
	probe := liveProbe(primary, env)
	adopt := func(heads []string) ([]string, error) {
		dir := gitOutIn("", "rev-parse", "--show-toplevel")
		if dir == "" {
			return nil, fmt.Errorf("not inside a worktree")
		}
		conflicts, err := adoptSnapshots(dir, heads)
		if err != nil {
			return nil, err
		}
		if env.Ticket != "" {
			if err := recordAdopted(storeForTicket(env, env.Ticket).Home(), heads); err != nil {
				return nil, err
			}
		}
		return conflicts, nil
	}
	code := awaitPrereqs(prereqs, probe, adopt, timeout, interval, time.Sleep, time.Now, os.Stdout)
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
func awaitPrereqs(prereqs []prereq, probe prereqProbe, adopt adoptFunc, timeout, interval time.Duration, sleep func(time.Duration), now func() time.Time, out io.Writer) int {
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
		var conflicts []string
		if result == "ready" && adopt != nil {
			heads := []string{}
			for _, p := range prereqs {
				if h := last[p.ticket].Head; h != "" {
					heads = append(heads, h)
				}
			}
			c, err := adopt(heads)
			switch {
			case err != nil:
				fmt.Fprintln(out, "ADOPT_ERROR="+strconv.Quote(firstLineOf(err.Error())))
				result, code = "not-ready", 3
			case len(c) > 0:
				conflicts = c
				result, code = "conflict", 5
			}
		}
		for _, p := range prereqs {
			s := last[p.ticket]
			line := fmt.Sprintf("PREREQ=%s TASK=%s STATE=%s", p.ticket, p.task, s.State)
			if s.Head != "" {
				line += " SNAPSHOT=" + s.Head
			}
			if s.Reason != "" {
				line += " REASON=" + strconv.Quote(s.Reason)
			}
			fmt.Fprintln(out, line)
		}
		for _, f := range conflicts {
			fmt.Fprintln(out, "CONFLICT="+f)
		}
		fmt.Fprintln(out, "AWAIT="+result)
		switch result {
		case "ready":
			fmt.Fprintln(out, "NEXT=the prerequisites' work is now in this worktree as uncommitted changes; reconcile the plan with it, then edit")
		case "conflict":
			fmt.Fprintln(out, "NEXT=resolve the conflict markers in each CONFLICT file (keep both sides' intent), then continue; never commit")
		case "not-ready":
			fmt.Fprintln(out, "NEXT=do not edit yet; report `bbs team task done --outcome failed` (BLOCKED) naming the prerequisite — the lead starts a refresh when it is repaired")
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

// liveProbe reads one prerequisite's settlement: its Task's Dispatch first, so
// readiness is only judged once its worker has stopped writing, then the same
// readiness evaluator land uses against the ticket's own worktree.
func liveProbe(primary string, env identity.Env) prereqProbe {
	readinessErrors := map[string]int{}
	return func(p prereq) prereqState {
		id, status, err := taskDispatchState(p.task)
		switch {
		case err != nil:
			return prereqState{State: "waiting", Reason: "task unreadable: " + firstLineOf(err.Error())}
		case id == "":
			return prereqState{State: "waiting", Reason: "not dispatched yet"}
		case !dispatchTerminal(status):
			return prereqState{State: "waiting", Reason: "dispatch " + status}
		case status != "completed":
			return prereqState{State: "not-ready", Reason: "dispatch " + status}
		}
		ready, reasons, _, err := ticketV2Readiness(primary, env, p.ticket, "review")
		if err != nil {
			// A read racing the finishing worker's last write (git lock, a
			// verdict mid-rename) clears on the next poll; only a repeated
			// failure is the prerequisite's.
			readinessErrors[p.ticket]++
			reason := "readiness: " + readinessErrorReason(err)
			if readinessErrors[p.ticket] < awaitReadinessAttempts {
				return prereqState{State: "waiting", Reason: fmt.Sprintf("%s (attempt %d/%d)", reason, readinessErrors[p.ticket], awaitReadinessAttempts)}
			}
			return prereqState{State: "not-ready", Reason: reason}
		}
		delete(readinessErrors, p.ticket)
		state, reasons := ticketReportState(primary, env, p.ticket, ready, reasons)
		if state != "" {
			snap, err := prereqSnapshot(primary, env, p.ticket)
			if err != nil {
				return prereqState{State: "not-ready", Reason: "snapshot: " + firstLineOf(err.Error())}
			}
			return prereqState{State: state, Head: snap, Reason: strings.Join(reasons, ",")}
		}
		return prereqState{State: "not-ready", Reason: strings.Join(reasons, ",")}
	}
}

// ticketReportState is the gate a finished ticket session passes: review
// readiness ("ready"), or light readiness when its heavy checks are deferred
// to a consolidated run ("light-ready"). "" means not ready, with reasons.
func ticketReportState(primary string, env identity.Env, t string, reviewReady bool, reasons []string) (string, []string) {
	if reviewReady {
		return "ready", nil
	}
	if deferred := ticket.ReadDoc(storeForTicket(env, t).IndexPath()).Get("pointers.deferred_heavy"); deferred != "" {
		if ok, _, _, err := ticketV2Readiness(primary, env, t, "light"); err == nil && ok {
			return "light-ready", []string{"deferred heavy: " + deferred}
		}
	}
	return "", reasons
}

// readinessErrorReason renders a failed readiness run by its error envelope
// (`CODE: message`) rather than the raw JSON it printed.
func readinessErrorReason(err error) string {
	msg := err.Error()
	if i := strings.IndexByte(msg, '{'); i >= 0 {
		var env struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(msg[i:]), &env) == nil && env.Error.Code != "" {
			return firstLineOf(env.Error.Code + ": " + env.Error.Message)
		}
	}
	return firstLineOf(msg)
}

// prereqSnapshot captures a verified prerequisite's worktree on its
// snapshot ref; the waiting ticket adopts that commit, never a branch.
func prereqSnapshot(primary string, env identity.Env, t string) (string, error) {
	st := storeForTicket(env, t)
	m, err := ticket.ReadManifest(st.ManifestPath())
	if err != nil {
		return "", err
	}
	repo := m.FindRepo(func(r ticket.Repo) bool { return r.Worktree != "" })
	if repo == nil {
		return "", fmt.Errorf("ticket %s has no worktree", t)
	}
	dir := repo.Worktree
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(primary, dir)
	}
	return sealTicketSnapshot(dir, t, readAdopted(st.Home()))
}
