package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/2found/2build/internal/agent"
	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/orca"
	"github.com/2found/2build/internal/ticket"
)

const foremanTaskUsage = `Usage:
  bbs team task create <foreman-id> --ticket <ticket> --kind <kind> --spec-file <path>
  bbs team task launch <task> --agent <id> --worktree <path> [--model <id>] [--effort <level>]
                          [--host <host-id>] [--skill <name>] [--terminal <handle> [--created-terminal]]
  bbs team task show <task> [--json] [--spec]
  bbs team task list [<foreman-id>] [--open]
  bbs team task done <task> --outcome succeeded|failed --summary <text>
                        [--dispatch <id>] [--report-path <path>] [--files-modified <a,b>]
                        [--no-gate <why>]
  bbs team task ask <task> (--body <question> | --resume <question-id>) [--timeout <sec>]
  bbs team task answer <task> --id <question-id> (--body <text> | --body-file <path>)
  bbs team task abandon <task> --reason <text>

A Task is one Foreman assignment; each launch of a worker for it is a
Dispatch. bbs keeps both on disk: Orca only hosts the worker's terminal.
launch starts the agent with ` + "`orca agent launch`" + ` (or types the assignment into
--terminal, a live session or one you created with ` + "`orca terminal create`" + `) and
records the launch evidence on the ticket. The worker reports with done, asks
with ask (it blocks until answered or --timeout, default 540s, capped below
the prompt-cache TTL), and sends nothing else.

--kind is required so the gate is a choice, never an omission: a succeeded
done for a ticket-session kind (ticket, repair, refresh, redirect) is refused until the ticket passes readiness (review, or light when
heavy checks are deferred), so a missing gate is fixed by the warm session
that owns it, not by a cold follow-up. --no-gate <why> reports anyway.
`

// taskWorkerExitGrace keeps a just-launched worker whose agent Orca has not
// detected yet from reading as exited.
const taskWorkerExitGrace = 2 * time.Minute

func foremanTask(args []string) error {
	if len(args) == 0 {
		fmt.Print(foremanTaskUsage)
		return nil
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "create":
		return foremanTaskCreate(rest)
	case "launch":
		return foremanTaskLaunch(rest)
	case "show":
		return foremanTaskShow(rest)
	case "list":
		return foremanTaskList(rest)
	case "done":
		return foremanTaskDone(rest)
	case "ask":
		return foremanTaskAsk(rest)
	case "answer":
		return foremanTaskAnswer(rest)
	case "abandon":
		return foremanTaskAbandon(rest)
	case "help", "--help", "-h":
		fmt.Print(foremanTaskUsage)
		return nil
	}
	return fmt.Errorf("foreman task: unknown subcommand %q\n%s", sub, foremanTaskUsage)
}

func taskArgs(verb string, args []string, required ...string) (string, map[string]string, error) {
	id, kv, err := foremanFlags(args)
	if err != nil {
		return "", nil, err
	}
	if id == "" {
		return "", nil, fmt.Errorf("foreman task %s: needs an id\n%s", verb, foremanTaskUsage)
	}
	for _, key := range required {
		if strings.TrimSpace(kv[key]) == "" {
			return "", nil, fmt.Errorf("foreman task %s: --%s is required\n%s", verb, key, foremanTaskUsage)
		}
	}
	return id, kv, nil
}

func foremanTaskCreate(args []string) error {
	id, kv, err := taskArgs("create", args, "ticket", "kind", "spec-file")
	if err != nil {
		return err
	}
	if _, err := foreman.Load(id); err != nil {
		return fmt.Errorf("foreman task create: %w", err)
	}
	spec, err := os.ReadFile(kv["spec-file"])
	if err != nil {
		return fmt.Errorf("foreman task create: %w", err)
	}
	t, err := foreman.CreateTask(id, kv["ticket"], kv["kind"], spec)
	if err != nil {
		return fmt.Errorf("foreman task create: %w", err)
	}
	fmt.Println("TASK=" + t.ID)
	fmt.Println("SPEC=" + foreman.SpecPath(t.ID))
	return nil
}

// taskWorkerPrompt is the whole worker protocol. The assignment itself stays
// on disk, so the prompt is short enough to type into any agent TUI.
func taskWorkerPrompt(skillRef, task, dispatch string) string {
	prompt := fmt.Sprintf("You are a bbs Team worker for Task %s, Dispatch %s. "+
		"Read your assignment with `bbs team task show %s --spec` and run all of it in this session. "+
		"Send no status or heartbeat messages; record milestones with `bbs team progress` only when the assignment asks. "+
		"Ask the lead only for a decision outside the assignment: `bbs team task ask %s --body \"<question>\"` (it blocks until answered). "+
		"When finished, persist the handoff, then run exactly once: "+
		"`bbs team task done %s --dispatch %s --outcome succeeded|failed --summary \"<did; found; left>\"`.",
		task, dispatch, task, task, task, dispatch)
	if skillRef != "" {
		prompt = skillRef + " " + prompt
	}
	return prompt
}

func foremanTaskLaunch(args []string) error {
	id, kv, err := taskArgs("launch", args, "agent", "worktree")
	if err != nil {
		return err
	}
	t, err := foreman.LoadTask(id)
	if err != nil {
		return fmt.Errorf("foreman task launch: %w", err)
	}
	skillRef := ""
	if kv["skill"] != "" {
		prof, err := agent.ByName(agent.Normalize(kv["agent"]))
		if err != nil {
			return fmt.Errorf("foreman task launch: %w", err)
		}
		skillRef = prof.SkillRef(kv["skill"])
	}
	client, err := orca.Preflight()
	if err != nil {
		return fmt.Errorf("foreman task launch: %w", err)
	}
	reuse := kv["terminal"]
	owns := reuse == "" || kv["created-terminal"] != ""
	for _, d := range t.Dispatches {
		if reuse != "" && d.Terminal == reuse && d.OwnsTerminal {
			owns = true
		}
	}
	t, err = foreman.UpdateTask(id, func(t *foreman.Task) error {
		_, err := t.OpenDispatch(foreman.Dispatch{
			Agent: kv["agent"], Model: kv["model"], Effort: kv["effort"],
			Worktree: kv["worktree"], Terminal: reuse, OwnsTerminal: owns,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("foreman task launch: %w", err)
	}
	d := t.Current()
	receipt := orca.LaunchReceipt{DispatchID: d.ID, TerminalHandle: reuse, Reused: reuse != ""}
	prompt := taskWorkerPrompt(skillRef, t.ID, d.ID)
	var launchErr error
	// sendFailed: the assignment never reached the reused terminal, so no
	// worker runs this Dispatch even though the handle is known.
	sendFailed := false
	if reuse != "" {
		launchErr = client.SendEnterHandle(reuse, prompt)
		sendFailed = launchErr != nil
		receipt.PromptOutcome = "sent"
	} else {
		raw, err := client.LaunchAgent(orca.AgentLaunch{
			Agent: kv["agent"], Worktree: kv["worktree"], Model: kv["model"], Effort: kv["effort"], Prompt: prompt,
		})
		if err == nil {
			receipt.TerminalHandle, receipt.PromptOutcome, err = orca.ParseAgentLaunch(raw)
		}
		launchErr = err
	}
	if launchErr == nil {
		info, err := client.ShowTerminal(receipt.TerminalHandle)
		if err == nil {
			err = orca.VerifyLaunch(&receipt, info, orca.LaunchRequest{
				AgentID: kv["agent"], HostID: kv["host"], TerminalHandle: reuse,
			})
		}
		launchErr = err
	}
	evidence := launchHandoff{Ticket: t.Ticket, Task: t.ID, Receipt: &receipt, Verification: "matched"}
	if reuse != "" {
		evidence.Verification = "transport-matched"
	}
	if launchErr != nil {
		evidence.Verification, evidence.Reason = "mismatch", launchErr.Error()
	}
	if receipt.TerminalHandle != "" && receipt.TerminalHandle != reuse {
		if _, err := foreman.UpdateTask(id, func(t *foreman.Task) error {
			t.DispatchByID(d.ID).Terminal = receipt.TerminalHandle
			return nil
		}); err != nil {
			return fmt.Errorf("foreman task launch: record terminal: %w", err)
		}
	}
	if receipt.TerminalHandle == "" || sendFailed {
		// Nothing started: settle the Dispatch so the Task can be relaunched.
		_, _ = foreman.UpdateTask(id, func(t *foreman.Task) error {
			return t.DispatchByID(d.ID).Settle(foreman.DispatchFailed, "launch failed: "+launchErr.Error())
		})
	}
	if t.Ticket != "" {
		path, err := appendForemanHandoff(t.Ticket, t.ID, "launch", evidence)
		if err != nil {
			return fmt.Errorf("foreman task launch: cannot persist launch evidence: %w", err)
		}
		fmt.Println("LAUNCH_HANDOFF=" + path)
	}
	fmt.Println("TASK=" + t.ID)
	fmt.Println("DISPATCH=" + d.ID)
	if receipt.TerminalHandle != "" {
		fmt.Println("TERMINAL=" + receipt.TerminalHandle)
	}
	fmt.Println("VERIFICATION=" + evidence.Verification)
	if launchErr != nil {
		return fmt.Errorf("foreman task launch: %w", launchErr)
	}
	return nil
}

func foremanTaskShow(args []string) error {
	id, kv, err := taskArgs("show", args)
	if err != nil {
		return err
	}
	t, err := foreman.LoadTask(id)
	if err != nil {
		return err
	}
	if kv["spec"] != "" {
		data, err := os.ReadFile(foreman.SpecPath(id))
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	}
	if kv["json"] != "" {
		return json.NewEncoder(os.Stdout).Encode(t)
	}
	printTask(t)
	for _, q := range foreman.PendingQuestions([]string{t.ID}) {
		fmt.Printf("QUESTION=%s DISPATCH=%s ASKED=%s BODY=%s\n", q.ID, q.Dispatch, q.AskedAt, strconv.Quote(q.Body))
	}
	return nil
}

func printTask(t foreman.Task) {
	line := fmt.Sprintf("TASK=%s TICKET=%s", t.ID, orDefault(t.Ticket, "-"))
	if t.Kind != "" {
		line += " KIND=" + t.Kind
	}
	if d := t.Current(); d != nil {
		line += fmt.Sprintf(" DISPATCH=%s STATUS=%s", d.ID, d.Status)
		if d.Terminal != "" {
			line += " TERMINAL=" + d.Terminal
		}
		if d.ReportPath != "" {
			line += " REPORT=" + d.ReportPath
		}
		if d.Summary != "" {
			line += " SUMMARY=" + strconv.Quote(d.Summary)
		}
		if d.Reason != "" {
			line += " REASON=" + strconv.Quote(d.Reason)
		}
		if d.Gate != "" {
			line += " GATE=" + strconv.Quote(d.Gate)
		}
	} else {
		line += " STATUS=pending"
	}
	fmt.Println(line)
}

func foremanTaskList(args []string) error {
	id, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	for _, t := range foreman.ListTasks(id) {
		if d := t.Current(); kv["open"] != "" && (d == nil || foreman.DispatchSettled(d.Status)) {
			continue
		}
		printTask(t)
	}
	return nil
}

func foremanTaskDone(args []string) error {
	id, kv, err := taskArgs("done", args, "outcome", "summary")
	if err != nil {
		return err
	}
	status := ""
	switch kv["outcome"] {
	case "succeeded":
		status = foreman.DispatchCompleted
	case "failed":
		status = foreman.DispatchFailed
	default:
		return fmt.Errorf("foreman task done: --outcome must be succeeded or failed")
	}
	if p := kv["report-path"]; p != "" {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("foreman task done: --report-path: %w", err)
		}
	}
	gate, err := taskDoneGateFor(id, kv)
	if err != nil {
		return err
	}
	t, err := foreman.UpdateTask(id, func(t *foreman.Task) error {
		d := t.Current()
		if kv["dispatch"] != "" {
			d = t.DispatchByID(kv["dispatch"])
		}
		if d == nil {
			return fmt.Errorf("task %s has no dispatch %q", t.ID, kv["dispatch"])
		}
		if cur := t.Current(); cur.ID != d.ID {
			return fmt.Errorf("dispatch %s was superseded by %s; do not report for it", d.ID, cur.ID)
		}
		if foreman.DispatchSettled(d.Status) {
			// A retried report: Settle refuses a conflicting status, and the
			// first report's summary, paths and gate stay the record.
			return d.Settle(status, "")
		}
		if err := d.Settle(status, ""); err != nil {
			return err
		}
		d.Summary, d.ReportPath = kv["summary"], kv["report-path"]
		if gate != "" {
			d.Gate = gate
		}
		if files := kv["files-modified"]; files != "" {
			d.FilesModified = strings.Split(files, ",")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("foreman task done: %w", err)
	}
	printTask(t)
	fmt.Println("NEXT=end this turn and stay idle; bbs team watch reports this to the lead")
	return nil
}

// gatedTaskKinds are the Task kinds that end a ticket session: their worker
// owns the ticket's gates, so it reports success only once they pass.
var gatedTaskKinds = map[string]bool{"ticket": true, "repair": true, "refresh": true, "redirect": true}

// taskDoneGate judges a ticket's readiness for task done: "ready",
// "light-ready", or "" with reasons. Tests replace it.
var taskDoneGate = func(t string) (string, []string, error) {
	primary := baseOpsPrimary("foreman task done")
	if primary == "" {
		return "", nil, fmt.Errorf("no primary checkout")
	}
	env, err := ticket.ResolveProject()
	if err != nil {
		return "", nil, err
	}
	ready, reasons, _, err := ticketV2Readiness(primary, env, t, "review")
	if err != nil {
		return "", nil, err
	}
	state, reasons := ticketReportState(primary, env, t, ready, reasons)
	if state == "" {
		if why, ok := ticketAdoptionCarry(primary, env, t, reasons); ok {
			return "carried: " + why, nil, nil
		}
		if dir, _, err := ticketWorktree(primary, env, t); err == nil {
			st := storeForTicket(env, t)
			if _, why, ok := deferralCovers(dir, st.Home(), ticket.ReadDoc(st.IndexPath()), reasons); ok {
				return why, nil, nil
			}
		}
	}
	return state, reasons, nil
}

// taskDoneGateFor returns the gate verdict to record on a succeeded report,
// or refuses it while the ticket is not ready. A retried report for an
// already-settled Dispatch is not judged again, and a readiness read that
// fails is recorded as unverified rather than blocking the report.
func taskDoneGateFor(id string, kv map[string]string) (string, error) {
	if kv["outcome"] != "succeeded" {
		return "", nil
	}
	t, err := foreman.LoadTask(id)
	if err != nil {
		return "", fmt.Errorf("foreman task done: %w", err)
	}
	if !gatedTaskKinds[t.Kind] || t.Ticket == "" {
		return "", nil
	}
	if d := t.Current(); d != nil && foreman.DispatchSettled(d.Status) {
		return "", nil
	}
	if why := strings.TrimSpace(kv["no-gate"]); why != "" {
		return "skipped: " + why, nil
	}
	if conflicts, err := taskDoneCompose(t); err == nil && len(conflicts) > 0 {
		sibs := make([]string, 0, len(conflicts))
		for sib := range conflicts {
			sibs = append(sibs, sib)
		}
		sort.Strings(sibs)
		fmt.Println("GATE=not-composing")
		for _, sib := range sibs {
			fmt.Println("CONFLICT_WITH=" + sib + ": " + strings.Join(conflicts[sib], ", "))
		}
		fmt.Println("NEXT=a phase sibling already reported work these files conflict with: run bbs ticket adopt " + strings.Join(sibs, " ") + " in this worktree, resolve each CONFLICT file, re-review only the resolution delta, re-run QA the resolution touches, then run done again")
		return "", fmt.Errorf("foreman task done: ticket %s does not compose with %s", t.Ticket, strings.Join(sibs, ","))
	}
	state, reasons, err := taskDoneGate(t.Ticket)
	switch {
	case err != nil:
		return "unverified: " + firstLineOf(err.Error()), nil
	case state != "":
		return state, nil
	}
	fmt.Println("GATE=not-ready")
	fmt.Println("REASONS=" + strings.Join(reasons, ","))
	fmt.Println("NEXT=you still hold this ticket's context: record the missing gates (review-pr, qa) on the final tree and run done again; --no-gate \"<why>\" only when a gate cannot apply, --outcome failed when the work did not land")
	return "", fmt.Errorf("foreman task done: ticket %s is not ready: %s", t.Ticket, strings.Join(reasons, ","))
}

func foremanTaskAsk(args []string) error {
	id, kv, err := taskArgs("ask", args)
	if err != nil {
		return err
	}
	timeout, err := awaitSeconds(kv, "timeout", blockingWaitSeconds())
	if err != nil {
		return err
	}
	var q foreman.Question
	switch {
	case kv["resume"] != "":
		q, err = foreman.LoadQuestion(id, kv["resume"])
	case strings.TrimSpace(kv["body"]) != "":
		q, err = foreman.Ask(id, kv["body"])
		// The watcher wakes Foreman for it; make sure one is running.
		ensureWatcher()
	default:
		return fmt.Errorf("foreman task ask: needs --body <question> or --resume <question-id>")
	}
	if err != nil {
		return fmt.Errorf("foreman task ask: %w", err)
	}
	deadline := time.Now().Add(timeout)
	for {
		if q.AnsweredAt != "" {
			fmt.Println("QUESTION=" + q.ID)
			fmt.Println("ANSWER=" + strconv.Quote(q.Answer))
			return nil
		}
		if !time.Now().Before(deadline) {
			fmt.Println("QUESTION_PENDING=" + q.ID)
			fmt.Printf("NEXT=run `bbs team task ask %s --resume %s` to keep waiting; never ask again\n", id, q.ID)
			os.Exit(4)
		}
		time.Sleep(5 * time.Second)
		if q, err = foreman.LoadQuestion(id, q.ID); err != nil {
			return fmt.Errorf("foreman task ask: %w", err)
		}
	}
}

func foremanTaskAnswer(args []string) error {
	id, kv, err := taskArgs("answer", args, "id")
	if err != nil {
		return err
	}
	body := kv["body"]
	if path := kv["body-file"]; path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("foreman task answer: %w", err)
		}
		body = string(data)
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("foreman task answer: needs --body or --body-file")
	}
	q, err := foreman.AnswerQuestion(id, kv["id"], body)
	if err != nil {
		return fmt.Errorf("foreman task answer: %w", err)
	}
	fmt.Println("ANSWERED=" + q.ID)
	return nil
}

func foremanTaskAbandon(args []string) error {
	id, kv, err := taskArgs("abandon", args, "reason")
	if err != nil {
		return err
	}
	t, err := foreman.UpdateTask(id, func(t *foreman.Task) error {
		d := t.Current()
		if d == nil {
			return fmt.Errorf("task %s has no dispatch", t.ID)
		}
		return d.Settle(foreman.DispatchFailed, "abandoned: "+kv["reason"])
	})
	if err != nil {
		return fmt.Errorf("foreman task abandon: %w", err)
	}
	printTask(t)
	return nil
}

// taskDispatchState is a Task's current Dispatch as the watcher, await and
// resource reconciliation read it: no Dispatch yet (or no Task record yet,
// as when capacity is reserved before create) is ID "".
func taskDispatchState(task string) (string, string, error) {
	t, err := foreman.LoadTask(task)
	if errors.Is(err, foreman.ErrTaskNotFound) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	if d := t.Current(); d != nil {
		return d.ID, d.Status, nil
	}
	return "", "", nil
}

// taskWorker is a Dispatch's worker terminal as Orca shows it.
type taskWorker struct {
	Handle string
	Live   bool
	Exited bool
}

// taskWorkerFor finds the Dispatch's terminal among the live ones. Missing
// means closed or lost to an Orca restart; present with no agent means the
// agent CLI exited and left its shell.
func taskWorkerFor(d *foreman.Dispatch, terms []orca.Terminal, now time.Time) (taskWorker, bool) {
	if d == nil || d.Terminal == "" {
		return taskWorker{}, false
	}
	for _, term := range terms {
		if term.Handle != d.Terminal {
			continue
		}
		if term.AgentIdentity == "" {
			launched, err := time.Parse(time.RFC3339, d.LaunchedAt)
			if err == nil && now.Sub(launched) < taskWorkerExitGrace {
				return taskWorker{Handle: term.Handle, Live: true}, true
			}
			return taskWorker{Handle: term.Handle, Exited: true}, true
		}
		return taskWorker{Handle: term.Handle, Live: true}, true
	}
	return taskWorker{}, false
}
