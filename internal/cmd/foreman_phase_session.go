package cmd

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/ticket"
)

// A phase session (Task kind "phase") builds a phase that is one ticket and
// verifies it in the same warm session, instead of handing the landed commit
// to a cold integration worker that would load the phase again. Once its
// focused checks pass, `task built` records the ticket's work (the snapshot a
// ticket session's done records); the session then integrates and lands the
// phase, reviews and QAs the landed commit, fixes forward in its own ticket
// worktree, and reports `task done` only once the phase is verified.
const phaseTaskKind = "phase"

func foremanTaskBuilt(args []string) error {
	id, kv, err := taskArgs("built", args)
	if err != nil {
		return err
	}
	t, err := foreman.LoadTask(id)
	if err != nil {
		return fmt.Errorf("foreman task built: %w", err)
	}
	if t.Kind != phaseTaskKind || t.Ticket == "" {
		return fmt.Errorf("foreman task built: task %s is kind %q; only a phase session (--kind phase) records its build before done — a ticket session's done records it", t.ID, t.Kind)
	}
	d := t.Current()
	if d == nil || foreman.DispatchSettled(d.Status) {
		return fmt.Errorf("foreman task built: task %s has no open dispatch", t.ID)
	}
	if kv["dispatch"] != "" && kv["dispatch"] != d.ID {
		return fmt.Errorf("foreman task built: dispatch %s is not the open one (%s); do not report for it", kv["dispatch"], d.ID)
	}
	project, phase, err := ticketPhase(t.Ticket)
	if err != nil {
		return fmt.Errorf("foreman task built: %w", err)
	}
	gate, err := ticketGateVerdict(t, kv, "the phase session's context")
	if err != nil {
		return fmt.Errorf("foreman task built: %w", err)
	}
	recorded, err := taskRecordProjectWork(t.Ticket, kv["artifacts"])
	if err != nil {
		return fmt.Errorf("foreman task built: %w", err)
	}
	if !recorded {
		return fmt.Errorf("foreman task built: ticket %s is not a ticket of a team project", t.Ticket)
	}
	if _, err := foreman.UpdateTask(id, func(t *foreman.Task) error {
		t.DispatchByID(d.ID).Gate = gate
		return nil
	}); err != nil {
		return fmt.Errorf("foreman task built: %w", err)
	}
	target := project + " --phase " + phase.ID
	fmt.Println("BUILT=" + t.Ticket)
	fmt.Println("GATE=" + strconv.Quote(gate))
	fmt.Println("NEXT=run `bbs team integrate " + target + "`, then `bbs team land " + target + "` (bbs team next answers wait while this session is open); review and QA the landed commit, fix forward in this ticket's worktree, then report task done once the phase is verified")
	return nil
}

// taskDonePhaseGateFor refuses a phase session's succeeded report until its
// phase is landed and verified: the session holds the phase's context, so
// the missing review or QA is cheapest here. Unlike the ticket gate, a read
// that fails refuses too: nothing after this report verifies the phase.
func taskDonePhaseGateFor(t foreman.Task, kv map[string]string) (string, error) {
	if d := t.Current(); d != nil && foreman.DispatchSettled(d.Status) {
		return "", nil
	}
	if why := strings.TrimSpace(kv["no-gate"]); why != "" {
		return "skipped: " + why, nil
	}
	state, reasons, err := taskDonePhaseGate(t.Ticket)
	switch {
	case err != nil:
		return "", fmt.Errorf("the phase of %s cannot be read: %s; report again once it can, or pass --no-gate \"<why>\"", t.Ticket, firstLineOf(err.Error()))
	case state != "":
		return state, nil
	}
	fmt.Println("GATE=not-ready")
	fmt.Println("REASONS=" + strings.Join(reasons, ","))
	fmt.Println("NEXT=you still hold this phase's context: record its review and QA on the landed commit (bbs team evidence), fix forward, and report again; --outcome failed when it cannot pass")
	return "", fmt.Errorf("the phase of %s is not verified: %s", t.Ticket, strings.Join(reasons, ","))
}

// taskDonePhaseGate reads the phase holding ticket: "phase-verified: …"
// once it is delivered (landed and verified), released or skipped, or "" with
// reasons. Tests replace it.
var taskDonePhaseGate = func(ticketID string) (string, []string, error) {
	_, p, err := ticketPhase(ticketID)
	if err != nil {
		return "", nil, err
	}
	switch state := projectPhaseStateName(p); state {
	case "delivered", "released", "skipped":
		return "phase-verified: " + p.ID + " " + state, nil, nil
	default:
		return "", append([]string{"phase " + p.ID + " is " + state}, p.Problems...), nil
	}
}

// ticketPhase finds the project and the phase holding ticket.
func ticketPhase(ticketID string) (string, projectPhaseState, error) {
	env, err := ticket.ResolveProject()
	if err != nil {
		return "", projectPhaseState{}, err
	}
	project := projectOfTicket(ticketID)
	if project == "" {
		return "", projectPhaseState{}, fmt.Errorf("ticket %s has no project", ticketID)
	}
	s, err := projectRead(storeForTicket(env, project))
	if err != nil {
		return "", projectPhaseState{}, err
	}
	for _, p := range s.Phases {
		if slices.Contains(p.Children, ticketID) {
			return project, p, nil
		}
	}
	return "", projectPhaseState{}, fmt.Errorf("ticket %s is in no phase of %s", ticketID, project)
}

// phaseSessionFits refuses a phase session on a phase of several tickets: it
// would hold the whole phase while its siblings wait on a dispatch.
func phaseSessionFits(ticketID string) error {
	_, p, err := ticketPhase(ticketID)
	if err != nil {
		return fmt.Errorf("--kind phase needs a ticket bound under its phase: %w", err)
	}
	if len(p.Seeds) != 1 {
		return fmt.Errorf("phase %s has %d tickets: --kind phase is for a phase of exactly one; use --kind ticket and an integration session", p.ID, len(p.Seeds))
	}
	return nil
}

// projectOfTicket walks a phase's ticket up to its project: ticket → phase
// ticket → project. "" when the ticket is not under a phase.
func projectOfTicket(ticketID string) string {
	env, err := ticket.ResolveProject()
	if err != nil {
		return ""
	}
	phase := ticket.ReadDoc(storeForTicket(env, ticketID).IndexPath()).Get("parent")
	if phase == "" {
		return ""
	}
	return ticket.ReadDoc(storeForTicket(env, phase).IndexPath()).Get("parent")
}

// openPhaseSession names the phase session that holds a one-ticket phase, if
// its Dispatch is still open: the phase is that session's to integrate, land
// and verify, so nobody else should.
func openPhaseSession(p projectPhaseState) string {
	if len(p.Seeds) != 1 {
		return ""
	}
	for _, task := range foreman.ListTasks("") {
		if task.Kind != phaseTaskKind || !slices.Contains(p.Children, task.Ticket) {
			continue
		}
		if d := task.Current(); d != nil && !foreman.DispatchSettled(d.Status) {
			return task.ID
		}
	}
	return ""
}

// buildsTicket: the Task kinds whose session builds a ticket's work.
func buildsTicket(kind string) bool {
	return gatedTaskKinds[kind] || kind == phaseTaskKind
}
