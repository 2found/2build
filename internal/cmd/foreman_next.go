package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// `bbs foreman next` reads the project snapshot and suggests what to do now.
// It is advice, not a gate: every rule it reflects is enforced by the verb it
// names (the same projectPhaseReasons readiness uses), so a Foreman that
// knows better may do something else and the CLI still keeps the invariants.
// Steps come in the order the work flows; `wait` steps say who holds the ball.

type foremanStep struct {
	Do      string `json:"do"` // design | publish | wait | revise | bind | create | build | refresh | seal | integrate | verify | land | release | repair | complete | done
	Phase   string `json:"phase,omitempty"`
	Target  string `json:"target,omitempty"`
	Command string `json:"command,omitempty"`
	Why     string `json:"why"`
}

func projectNext(st *ticket.Store, s projectSnapshot) []foremanStep {
	parent := s.Ticket
	if s.Contract == nil {
		return []foremanStep{{Do: "design", Command: "bbs foreman contract " + parent + " --file project.json",
			Why: "no valid project contract: write the basic design (requirement, design, prototype, plan) and its phases, seeds and criteria"}}
	}
	doc := ticket.ReadDoc(st.IndexPath())
	publish := "BABYSIT_TICKET=" + parent + " bbs ticket approval publish --kind project-plan"
	if doc.Get("approval.kind") == "project-plan" || doc.Get("approval.kind") == "" {
		switch s.Approval {
		case "pending":
			return []foremanStep{{Do: "wait", Target: "human", Command: "BABYSIT_TICKET=" + parent + " bbs ticket approval await",
				Why: "the basic design is in review (a Foreman under --auto self-resolves it)"}}
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
			steps = append(steps, foremanStep{Do: "repair", Phase: p.ID, Why: "stop the line: the last release of phase " + p.ID + " failed; nothing later moves until it ships. Fix forward (a repair to the ticket that broke, then integrate, verify, land) or retry the release"})
			return append(steps, phaseNext(st, s, p)...)
		}
		steps = append(steps, phaseNext(st, s, p)...)
		// The next phase builds while this one waits on its release; it never
		// overtakes an undelivered phase.
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
	return []foremanStep{{Do: "complete", Command: "bbs foreman complete " + parent + " --foreman " + orDefault(s.Coordinator["id"], "<id>"),
		Why: "every phase is resolved; record completion (it reports any cleanup left)"}}
}

func phaseNext(st *ticket.Store, s projectSnapshot, p projectPhaseState) []foremanStep {
	parent := s.Ticket
	step := func(do, target, command, why string) foremanStep {
		return foremanStep{Do: do, Phase: p.ID, Target: target, Command: command, Why: why}
	}
	if p.Ticket == "" {
		return []foremanStep{step("bind", "", "bbs foreman bind "+parent+" --phase "+p.ID+" --child <phase-ticket>",
			"create a phase ticket under "+parent+" and bind it; its tickets go under it")}
	}
	switch {
	case p.Delivered:
		if p.Release == "ask" && !p.ApprovalGo {
			if doc := ticket.ReadDoc(storeForTicket(st.Env, p.Ticket).IndexPath()); doc.Get("approval.kind") == "release" && doc.Get("approval.state") == "pending" {
				return []foremanStep{step("wait", "human", "", "release go for phase "+p.ID+" is with the human")}
			}
			return []foremanStep{step("release", "human", "bbs foreman release "+parent+" --phase "+p.ID+" --request",
				"phase "+p.ID+" is landed; ask the human for the release go")}
		}
		return []foremanStep{step("release", "", "bbs foreman release "+parent+" --phase "+p.ID+" --file receipt.json",
			"phase "+p.ID+" is landed; run release-prd, then record its verdict")}
	case p.Integrated:
		return []foremanStep{step("land", "", "bbs foreman land "+parent+" --phase "+p.ID, "phase QA passed on the phase commit")}
	case p.Sealed && p.Candidate == nil && !s.Contract.ArtifactsOnly:
		return []foremanStep{step("integrate", "", "bbs foreman integrate "+parent+" --phase "+p.ID, "every ticket of phase "+p.ID+" is sealed")}
	case p.Sealed:
		why := "run phase QA on the phase commit and record it (bbs foreman evidence --begin, then --attempt)"
		if checks := phaseContractChecks(p); len(checks) > 0 {
			why += "; it changes contracts, so include their consumers' checks: " + strings.Join(checks, "; ")
		}
		return []foremanStep{step("verify", "", "bbs foreman evidence "+parent+" --phase "+p.ID+" --begin --file spec.json", why)}
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
			steps = append(steps, step("create", seed, "bbs foreman bind "+parent+" --seed "+seed+" --child <ticket>",
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
		case child.State == ticket.StateReady:
			steps = append(steps, step("build", child.ID, "", "ready: dispatch it to a worker session"))
		default:
			steps = append(steps, step("wait", child.ID, "", "worker "+child.State+"; seal it once its gates pass"))
		}
	}
	if len(sealable) > 0 {
		ids := strings.Join(sealable, " ")
		steps = append([]foremanStep{step("seal", ids, "bbs foreman seal "+ids, "review-pr and qa pass on their current trees")}, steps...)
	}
	return steps
}

// childSealable: the check `bbs foreman seal` makes, read-only.
func childSealable(cs *ticket.Store) bool {
	m, err := ticket.ReadManifest(cs.ManifestPath())
	if err != nil || len(m.Repos) != 1 {
		return false
	}
	env := cs.Env
	env.Branch = m.Repos[0].Branch
	r, err := evaluateReadinessIn(env, "review", m.Repos[0].Worktree)
	return err == nil && r.Enforced && r.Ready
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
