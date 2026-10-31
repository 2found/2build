package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// `bbs team next` reads the project snapshot and suggests what to do now.
// It is advice, not a gate: every rule it reflects is enforced by the verb it
// names (the same projectPhaseReasons readiness uses), so a Foreman that
// knows better may do something else and the CLI still keeps the invariants.
// Steps come in the order the work flows; `wait` steps say who holds the ball.

type foremanStep struct {
	Do      string `json:"do"` // design | publish | wait | revise | bind | create | build | refresh | seal | integrate | verify | land | release | skip | repair | complete | done
	Phase   string `json:"phase,omitempty"`
	Target  string `json:"target,omitempty"`
	Command string `json:"command,omitempty"`
	Why     string `json:"why"`
}

func projectNext(st *ticket.Store, s projectSnapshot) []foremanStep {
	parent := s.Ticket
	if s.Contract == nil {
		return []foremanStep{{Do: "design", Command: "bbs team contract " + parent + " --file project.json",
			Why: "no valid project contract: write the basic design (requirement, design, prototype, plan) and its phases, seeds and criteria"}}
	}
	doc := ticket.ReadDoc(st.IndexPath())
	publish := "BABYSIT_TICKET=" + parent + " bbs ticket approval publish --kind project-plan"
	if doc.Get("approval.kind") == "project-plan" || doc.Get("approval.kind") == "" {
		switch s.Approval {
		case "pending":
			return []foremanStep{{Do: "wait", Target: "human", Command: "BABYSIT_TICKET=" + parent + " bbs ticket approval await",
				Why: "the basic design is in review (a lead under --auto self-resolves it)"}}
		case "redirected":
			return []foremanStep{{Do: "revise", Command: publish,
				Why: "the reviewer asked for changes: read the note and comments, revise the design, publish again"}}
		case "approved":
		default: // none, stale, dropped
			return []foremanStep{{Do: "publish", Command: publish,
				Why: "the current basic design has no approval (" + orDefault(s.Approval, "none") + ")"}}
		}
	}
	if control := doc.Get("control.state"); control != "" {
		return []foremanStep{{Do: "wait", Target: "human", Why: "the project is " + control}}
	}

	var steps []foremanStep
	for _, p := range s.Phases {
		if projectPhaseResolved(p) {
			continue
		}
		if p.LastFailed {
			steps = append(steps, foremanStep{Do: "repair", Phase: p.ID, Why: "stop the line: the last release of phase " + p.ID + " failed; nothing later moves until it ships. Fix forward (a repair to the ticket that broke, then integrate, land, verify) or retry the release"})
			return append(steps, phaseNext(st, s, p)...)
		}
		steps = append(steps, phaseNext(st, s, p)...)
		// The next phase builds while this one runs phase QA or waits on its
		// release; it never overtakes an unlanded phase.
		if !p.Delivered {
			return steps
		}
	}
	if len(steps) > 0 {
		return steps
	}
	if s.Completion == "verified" {
		return []foremanStep{{Do: "done", Why: "every phase is resolved and completion is verified"}}
	}
	return []foremanStep{{Do: "complete", Command: "bbs team complete " + parent + " --foreman " + orDefault(s.Coordinator["id"], "<id>"),
		Why: "every phase is resolved; record completion (it reports any cleanup left)"}}
}

func phaseNext(st *ticket.Store, s projectSnapshot, p projectPhaseState) []foremanStep {
	parent := s.Ticket
	step := func(do, target, command, why string) foremanStep {
		return foremanStep{Do: do, Phase: p.ID, Target: target, Command: command, Why: why}
	}
	if p.Ticket == "" {
		return []foremanStep{step("bind", "", "bbs team bind "+parent+" --phase "+p.ID+" --child <phase-ticket>",
			"create a phase ticket under "+parent+" and bind it; its tickets go under it")}
	}
	switch {
	case p.Delivered && p.Integrated:
		if p.Release == "ask" && !p.ApprovalGo {
			ps := storeForTicket(st.Env, p.Ticket)
			if doc := ticket.ReadDoc(ps.IndexPath()); doc.Get("approval.kind") == "release" {
				var req projectReleaseRequest
				current := readProjectJSON(projectReleaseRequestPath(ps), &req) == nil && req.Subject == p.Subject
				switch state, _, note := approvalRead(ps); {
				case state == "pending":
					return []foremanStep{step("wait", "human", "", "the handover of phase "+p.ID+" is with the human for review and the release go")}
				case state == "redirected" && current:
					return []foremanStep{step("repair", "", "BABYSIT_TICKET="+p.Ticket+" bbs ticket approval comments",
						"the human sent the handover back: "+orDefault(note, "see the comments")+". Fix what the approved scope covers as repairs on the owning tickets (reseal, integrate, land, phase QA), then hand over again; a request beyond the approved scope goes back to basic-design review as a contract change")}
				case state == "dropped" && current:
					return []foremanStep{step("wait", "human", "", "the human declined the release of phase "+p.ID+": "+orDefault(note, "no reason given"))}
				}
			}
			return []foremanStep{step("release", "human", "bbs team release "+parent+" --phase "+p.ID+" --request",
				"phase "+p.ID+" is landed and verified; ask the human for the release go")}
		}
		if len(p.Outside) > 0 && !p.ApprovalGo {
			return releaseConfirmNext(st, s, p)
		}
		return []foremanStep{step("release", "", "bbs team release "+parent+" --phase "+p.ID+" --file receipt.json",
			"phase "+p.ID+" is landed and verified; run release-prd, then record its verdict")}
	case p.Sealed && p.Candidate == nil && !s.Contract.ArtifactsOnly:
		return []foremanStep{step("integrate", "", "bbs team integrate "+parent+" --phase "+p.ID, "every ticket of phase "+p.ID+" is sealed")}
	case p.Sealed && !p.Delivered && !s.Contract.ArtifactsOnly:
		return []foremanStep{step("land", "", "bbs team land "+parent+" --phase "+p.ID, "land the phase commit now; phase QA runs on the landed commit")}
	case p.Sealed:
		why := "run phase QA on the landed phase commit and record it (bbs team evidence --begin, then --attempt); a failure is fixed forward (reseal --fixup, integrate, land, re-check what changed)"
		if checks := phaseContractChecks(p); len(checks) > 0 {
			why += "; it changes contracts, so include their consumers' checks: " + strings.Join(checks, "; ")
		}
		return []foremanStep{step("verify", "", "bbs team evidence "+parent+" --phase "+p.ID+" --begin --file spec.json", why)}
	}

	var steps []foremanStep
	if reasons := projectPhaseReasons(s, "dispatch", p.ID); len(reasons) > 0 {
		return []foremanStep{step("wait", "", "", strings.Join(reasons, "; "))}
	}
	var sealable []string
	bound := map[string]bool{}
	for _, child := range s.Children {
		bound[child.Seed] = true
	}
	for _, seed := range p.Seeds {
		if !bound[seed] {
			steps = append(steps, step("create", seed, "bbs team bind "+parent+" --seed "+seed+" --child <ticket>",
				"seed "+seed+" has no ticket: create one under phase ticket "+p.Ticket+", bind it, dispatch it"))
		}
	}
	for _, child := range s.Children {
		if !slices.Contains(p.Seeds, child.Seed) {
			continue
		}
		cs := storeForTicket(st.Env, child.ID)
		if stale := contractReadinessReasons(cs, ""); len(stale) > 0 {
			steps = append(steps, step("refresh", child.ID, "BABYSIT_TICKET="+child.ID+" bbs contract status",
				"a contract it builds against changed ("+strings.Join(stale, ", ")+"): send a refresh to its worker session"))
			continue
		}
		if len(child.Problems) == 0 {
			continue
		}
		switch {
		case childSealable(cs):
			sealable = append(sealable, child.ID)
		case ticket.ReadDoc(cs.IndexPath()).Get("pointers.resource_lease") != "":
			// A held lease is a launched worker: dispatching again would duplicate it.
			steps = append(steps, step("wait", child.ID, "", "its worker holds a lease; the batch wake reports it"))
		case child.State == ticket.StateReady:
			steps = append(steps, step("build", child.ID, "", "ready: dispatch it to a worker session"))
		default:
			steps = append(steps, step("wait", child.ID, "", "worker "+child.State+"; seal it once its gates pass"))
		}
	}
	if len(sealable) > 0 {
		ids := strings.Join(sealable, " ")
		steps = append([]foremanStep{step("seal", ids, "bbs team seal "+ids, "review-pr and qa pass on their current trees, or the ticket's size defers them to phase review")}, steps...)
	}
	return steps
}

// childSealable: the check `bbs team seal` makes, read-only.
func childSealable(cs *ticket.Store) bool {
	m, err := ticket.ReadManifest(cs.ManifestPath())
	if err != nil || len(m.Repos) != 1 {
		return false
	}
	env := cs.Env
	env.Branch = m.Repos[0].Branch
	r, err := evaluateReadinessIn(env, "review", m.Repos[0].Worktree)
	if err != nil {
		return false
	}
	if r.Ready {
		return true
	}
	_, _, ok := deferralCovers(m.Repos[0].Worktree, cs.Home(), ticket.ReadDoc(cs.IndexPath()), r.ReasonCodes)
	return ok
}

// phaseContractChecks lists the checks of the contracts a phase commit changes.
func phaseContractChecks(p projectPhaseState) []string {
	if p.Candidate == nil {
		return nil
	}
	var out []string
	for _, r := range p.Candidate.Repos {
		for id, checks := range r.Contracts {
			if len(checks) == 0 {
				out = append(out, id+": no checks (read its consumers' code)")
			}
			for _, c := range checks {
				out = append(out, id+": "+c)
			}
		}
	}
	return out
}

// printForemanNext appends each owned project's next steps to a batch read,
// so one command on a wake shows both what settled and what to do about it.
func printForemanNext(id string) {
	env := identity.Resolve()
	entries, _ := os.ReadDir(filepath.Join(env.ProjectHome, "tickets"))
	for _, e := range entries {
		st := storeForTicket(env, e.Name())
		if !e.IsDir() || ticket.ReadDoc(st.IndexPath()).Get("assignee") != id {
			continue
		}
		if _, err := os.Stat(projectContractPath(st)); err != nil {
			continue
		}
		s, err := projectRead(st)
		if err != nil {
			continue
		}
		for _, x := range projectNext(st, s) {
			line := "NEXT=" + s.Ticket + " " + x.Do
			if x.Phase != "" {
				line += " phase=" + x.Phase
			}
			if x.Target != "" {
				line += " target=" + x.Target
			}
			fmt.Println(line + " — " + x.Why)
			if x.Command != "" {
				fmt.Println("  $ " + x.Command)
			}
		}
	}
}

func projectNextCommand(st *ticket.Store, kv map[string]string) error {
	s, err := projectRead(st)
	if err != nil {
		return err
	}
	steps := projectNext(st, s)
	if kv["json"] == "1" {
		printV2Envelope(map[string]interface{}{"ticket": s.Ticket, "steps": steps})
		return nil
	}
	for _, x := range steps {
		head := x.Do
		if x.Phase != "" {
			head += " [" + x.Phase + "]"
		}
		if x.Target != "" {
			head += " " + x.Target
		}
		fmt.Println(head + ": " + x.Why)
		if x.Command != "" {
			fmt.Println("  $ " + x.Command)
		}
	}
	return nil
}

// releaseConfirmNext: an auto phase whose release would also carry work
// outside the project (other sessions' commits, uncommitted changes on base)
// releases only on a human go. The human may release or skip; an unanswered
// confirmation skips at its deadline, so the line never waits on them.
func releaseConfirmNext(st *ticket.Store, s projectSnapshot, p projectPhaseState) []foremanStep {
	parent := s.Ticket
	step := func(do, target, command, why string) []foremanStep {
		return []foremanStep{{Do: do, Phase: p.ID, Target: target, Command: command, Why: why}}
	}
	skip := func(reason string) []foremanStep {
		return step("skip", "", "bbs team release "+parent+" --phase "+p.ID+" --skip --reason "+shellQuote(reason),
			"phase "+p.ID+": "+reason+"; skip this release and continue — the phase ships with a later release")
	}
	ps := storeForTicket(st.Env, p.Ticket)
	var req projectReleaseRequest
	if ticket.ReadDoc(ps.IndexPath()).Get("approval.kind") == "release" &&
		readProjectJSON(projectReleaseRequestPath(ps), &req) == nil && req.Subject == p.Subject && req.FailedBefore == p.Failed {
		switch state, _, note := approvalRead(ps); state {
		case "pending":
			if at, err := time.Parse(time.RFC3339, req.Deadline); err == nil && !time.Now().Before(at) {
				return skip("no answer to the release confirmation by " + req.Deadline)
			}
			return step("wait", "human", "", "the human is asked to confirm the release of phase "+p.ID+" over work outside this project; unanswered by "+orDefault(req.Deadline, "its deadline")+" it is skipped. Everything else continues")
		case "dropped":
			if projectPhaseIndex(s, p.ID) == len(s.Phases)-1 {
				return step("wait", "human", "", "the human declined the release of phase "+p.ID+": "+orDefault(note, "no reason given"))
			}
			return skip("the human chose to skip it" + orDefault(prefixed(": ", note), ""))
		case "redirected":
			return step("repair", "", "BABYSIT_TICKET="+p.Ticket+" bbs ticket approval comments",
				"the human answered the release confirmation of phase "+p.ID+" with feedback: "+orDefault(note, "see the comments")+". Act on it, then ask again or skip")
		}
	}
	why := "phase " + p.ID + " is landed and verified, but base carries work outside this project (" + strings.Join(p.Outside, " | ") + "): "
	if projectPhaseIndex(s, p.ID) == len(s.Phases)-1 {
		why += "ask the human to confirm the release"
	} else {
		why += "ask the human to release or skip (recommended); unanswered by the deadline (default 30m) it is skipped"
	}
	return step("release", "human", "bbs team release "+parent+" --phase "+p.ID+" --request", why)
}

func prefixed(prefix, s string) string {
	if s == "" {
		return ""
	}
	return prefix + s
}
