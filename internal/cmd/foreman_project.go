package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

func parseProjectTime(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }

func projectTimeAfter(value, previous string) bool {
	v, err := parseProjectTime(value)
	if err != nil {
		return false
	}
	p, err := parseProjectTime(previous)
	return err != nil || v.After(p)
}

func projectStore(id string) (*ticket.Store, error) {
	if !idRe.MatchString(id) {
		return nil, fmt.Errorf("foreman: needs a plain ticket ID")
	}
	env := identity.Resolve()
	env.Ticket = id
	st := ticket.New(env)
	_, err := ticket.ReadDocStrict(st.IndexPath())
	return st, err
}

func projectRead(st *ticket.Store) (projectSnapshot, error) {
	s := projectSnapshot{Version: 1, Ticket: st.Env.Ticket, Observed: isoNow(), Children: []projectChild{}, Coverage: []projectCoverage{}, Evidence: []projectEvidence{}, Dispatch: []string{}, Blockers: []string{}, Completion: "pending", Usage: skillUsage{Reason: "provider usage not observed"}}
	doc, err := ticket.ReadDocStrict(st.IndexPath())
	if err != nil {
		return s, err
	}
	s.Coordinator = map[string]string{"agent_liveness": "unknown"}
	if r, err := foreman.Load(doc.Get("assignee")); err == nil {
		s.Coordinator["id"] = r.ID
		s.Coordinator["heartbeat"] = r.Heartbeat
		s.Coordinator["runtime_observed_at"] = watchLoad(r.ID).RuntimeObservedAt
	}
	var c projectContract
	if err := readProjectJSON(projectContractPath(st), &c); err != nil {
		s.Dispatch = append(s.Dispatch, "contract: missing or unreadable project.json")
	} else if err := validateProjectContract(c); err != nil {
		s.Dispatch = append(s.Dispatch, "contract: "+err.Error())
	} else {
		s.Contract = &c
	}
	s.Approval, _, _ = approvalRead(st)
	if s.Approval != "approved" || doc.Get("approval.kind") != "project-plan" {
		s.Dispatch = append(s.Dispatch, "approval: current project-plan approval required")
	}
	if control := doc.Get("control.state"); control != "" {
		s.Dispatch = append(s.Dispatch, "control: "+control)
	}
	if c.Deadline != "" {
		if deadline, err := parseProjectTime(c.Deadline); err == nil && time.Now().After(deadline) {
			s.Dispatch = append(s.Dispatch, "budget: project deadline exceeded; finish verified work or revise scope")
		}
	}
	g, err := ticket.BuildGraph(st.Env.ProjectHome, st.Env.Ticket)
	if err != nil {
		return s, err
	}
	if len(g.Cycles) > 0 {
		s.Dispatch = append(s.Dispatch, "graph: dependency cycle")
	}
	bindings := map[string]string{}
	phaseNodes, leafParents, external := map[string]string{}, map[string]string{}, map[string]bool{}
	// Only material, stable inputs enter the subject. Status, progress and
	// worktree removal must not rewrite an accepted product revision.
	inputs := map[string]interface{}{"contract": c, "approval": doc.Get("approval.artifact_revision"), "approval_state": s.Approval, "run": doc.Get("pointers.orca_run")}
	for _, node := range g.Nodes {
		cs := storeForTicket(st.Env, node.ID)
		cd, readErr := ticket.ReadDocStrict(cs.IndexPath())
		if readErr != nil || node.Dangling {
			s.Blockers = append(s.Blockers, "child "+node.ID+": missing or unreadable")
			inputs[node.ID] = "missing"
			continue
		}
		if node.External {
			if node.Status != "done" || cd.Get("control.state") != "" {
				s.Blockers = append(s.Blockers, "dependency "+node.ID+": not completed")
			}
			// External dependency revisions must also invalidate integrated QA.
			m, me := ticket.ReadManifest(cs.ManifestPath())
			refs := map[string]string{}
			if me != nil || len(m.Repos) == 0 {
				s.Blockers = append(s.Blockers, "dependency "+node.ID+": revision unavailable")
			} else {
				for _, r := range m.Repos {
					refs[r.Name] = gitOutIn(r.Canonical, "rev-parse", "--verify", r.Branch+"^{commit}")
					if refs[r.Name] == "" {
						s.Blockers = append(s.Blockers, "dependency "+node.ID+": revision unavailable")
					}
				}
			}
			inputs[node.ID] = refs
			external[node.ID] = true
			continue
		}
		if control := cd.Get("control.state"); control != "" {
			s.Blockers = append(s.Blockers, "child "+node.ID+": "+control)
		}
		if node.Status == "cancelled" || node.Status == "duplicate" {
			s.Blockers = append(s.Blockers, "child "+node.ID+": required scope was "+node.Status)
		}
		// A phase ticket groups one release increment's tickets and carries
		// its release approval; it is never a leaf, even before it has children.
		if phase := cd.Get("origin.project_phase"); phase != "" {
			if prior := phaseNodes[phase]; prior != "" {
				s.Blockers = append(s.Blockers, "phase "+phase+": also bound to "+prior)
			}
			if node.Parent != st.Env.Ticket {
				s.Blockers = append(s.Blockers, "phase "+phase+": ticket "+node.ID+" must be a direct child of "+st.Env.Ticket)
			}
			phaseNodes[phase] = node.ID
			inputs[node.ID] = node.Children
			continue
		}
		if len(node.Children) != 0 {
			inputs[node.ID] = node.Children
			continue
		}
		leafParents[node.ID] = node.Parent
		child := projectChild{ID: node.ID, Title: cd.Get("title"), Seed: cd.Get("origin.project_seed"), State: node.State, BlockedBy: node.BlockedBy, Repos: []projectRepo{}, Problems: []string{}}
		if child.Seed == "" || !slices.Contains(c.Seeds, child.Seed) {
			child.Problems = append(child.Problems, "scope: seed is not in approved contract")
		} else if prior := bindings[child.Seed]; prior != "" {
			child.Problems = append(child.Problems, "scope: seed also bound to "+prior)
		} else {
			bindings[child.Seed] = child.ID
		}
		var seal projectSeal
		if err := readProjectJSON(filepath.Join(cs.Home(), "project-seal.json"), &seal); err != nil {
			child.Problems = append(child.Problems, "verification: seal required before delivery or cleanup")
		} else {
			child.Repos = seal.Repos
			child.Waiver = seal.Waiver
			if c.ArtifactsOnly != (len(seal.Artifacts) > 0) {
				child.Problems = append(child.Problems, "seal does not match approved delivery type")
			}
			problems, landed := validateProjectSeal(cs, seal)
			child.Problems = append(child.Problems, problems...)
			child.landed = landed
		}
		var progress projectProgress
		if readProjectJSON(filepath.Join(cs.Home(), "progress.json"), &progress) == nil {
			projectProgressHealth(&progress, time.Now())
			child.Progress = &progress
		}
		for _, p := range child.Problems {
			s.Blockers = append(s.Blockers, "child "+child.ID+": "+p)
		}
		s.Children = append(s.Children, child)
		inputs[child.ID] = map[string]interface{}{"seed": child.Seed, "dependencies": child.BlockedBy, "seal": seal}
	}
	for _, seed := range c.Seeds {
		if bindings[seed] == "" {
			s.Blockers = append(s.Blockers, "scope: no child bound to "+seed)
		}
	}
	for _, child := range s.Children {
		if child.landed {
			// Its phase commit composed and verified both sides.
			continue
		}
		for _, dependency := range child.BlockedBy {
			for _, upstream := range s.Children {
				if upstream.ID != dependency {
					continue
				}
				for _, depRepo := range upstream.Repos {
					for _, repo := range child.Repos {
						if repo.Canonical == depRepo.Canonical && !gitOKIn(repo.Canonical, "merge-base", "--is-ancestor", depRepo.Head, repo.Head) {
							s.Blockers = append(s.Blockers, "dependency "+dependency+": current revision not verified in "+child.ID)
						}
					}
				}
			}
		}
	}
	s.Subject = digestJSON(inputs)
	if s.Contract != nil {
		projectPhaseSetup(st, &s, inputs, phaseNodes, leafParents, external)
	}
	// Evidence is immutable; only current receipts contribute coverage. Failed
	// attempts remain visible, but a later current pass may repair them.
	paths, err := filepath.Glob(filepath.Join(st.Home(), "project-evidence", "*.json"))
	if err != nil {
		return s, err
	}
	for _, path := range paths {
		var ev projectEvidence
		if err := readProjectJSON(path, &ev); err != nil {
			s.Blockers = append(s.Blockers, "evidence: unreadable "+filepath.Base(path))
			continue
		}
		ev.Problems = projectEvidenceProblems(st, s, ev, false)
		ev.State = "passed"
		if len(ev.Problems) > 0 {
			ev.State = "failed"
		}
		if ev.Subject != projectEvidenceSubject(s, ev) {
			ev.State = "stale"
		}
		s.Evidence = append(s.Evidence, ev)
	}
	if s.Contract != nil {
		projectPhaseProgress(st, &s)
	}
	s.Delivery = projectObserveDelivery(st, s)
	// An expired work budget prevents new starts, never acceptance of already
	// verified work. All other dispatch prerequisites also gate completion.
	for _, p := range s.Dispatch {
		if !strings.HasPrefix(p, "budget:") {
			s.Blockers = append(s.Blockers, p)
		}
	}
	var receipt projectCompletion
	completionRead := readProjectJSON(filepath.Join(st.Home(), "completion.json"), &receipt) == nil
	currentReceipt := completionRead && receipt.Version == 1 && receipt.Cleanup && receipt.Subject == projectCompletionSubject(s)
	if !currentReceipt {
		s.Blockers = append(s.Blockers, projectCleanupProblems(st, s)...)
	}
	s.Ready = len(s.Blockers) == 0
	if completionRead {
		s.Completion = "stale"
		if s.Ready && currentReceipt {
			s.Completion = "verified"
		}
	}
	return s, nil
}

// validateProjectSeal reports what invalidates a child's seal, and whether the
// child has landed: every checkout it was cut in moved off its base (its phase
// landed into it) or was removed. A landed child's snapshot is the record, so
// later edits to shared inputs (a workflow its own phase rewrote) and repairs
// to its dependencies no longer reopen it.
func validateProjectSeal(st *ticket.Store, seal projectSeal) ([]string, bool) {
	problems := []string{}
	if len(seal.Artifacts) > 0 {
		return validateArtifactSeal(st, seal), false
	}
	if seal.Version != 1 || seal.Ticket != st.Env.Ticket || len(seal.Repos) == 0 {
		return []string{"invalid seal"}, false
	}
	m, err := ticket.ReadManifest(st.ManifestPath())
	if err != nil || len(m.Repos) != len(seal.Repos) {
		return append(problems, "manifest changed or unavailable"), false
	}
	landed := true
	for _, r := range seal.Repos {
		mr := m.FindRepo(func(m ticket.Repo) bool { return m.Canonical == r.Canonical && m.Branch == r.Branch })
		if mr == nil {
			problems = append(problems, "manifest changed")
			landed = false
			continue
		}
		if gitOutIn(r.Canonical, "rev-parse", "--verify", "-q", ticketSnapshotRef(st.Env.Ticket)+"^{commit}") != r.Head {
			problems = append(problems, "snapshot changed or unavailable: "+ticketSnapshotRef(st.Env.Ticket))
		}
		// Work edited after the seal is unverified.
		if mr.Worktree != "" {
			if fi, err := os.Stat(mr.Worktree); err == nil && fi.IsDir() && gitOutIn(mr.Worktree, "rev-parse", "--verify", "-q", "HEAD^{commit}") == r.BaseHead {
				landed = false
				if tree, err := worktreeTree(mr.Worktree); err != nil || tree != treeOf(r.Canonical, r.Head) {
					problems = append(problems, "worktree changed since seal: "+mr.Worktree)
				}
			}
		}
	}
	if !landed {
		inputs, _, err := projectChildInputs(st)
		if err != nil {
			problems = append(problems, "evidence: "+err.Error())
		} else if inputs != seal.Inputs {
			problems = append(problems, "evidence: changed since seal")
		}
	}
	return problems, landed && len(problems) == 0
}

func projectCheckPassed(ev projectEvidence, criterion, kind string) bool {
	for _, check := range ev.Checks {
		if check.Criterion == criterion && check.Kind == kind && check.ExitCode != nil && *check.ExitCode == 0 {
			if digest, ok := digestFile(check.Log); ok && digest == check.Digest {
				return true
			}
		}
	}
	return false
}

// sealWaivable splits readiness reason codes into what a waiver cannot cover
// and the state of each waived gate. Only a gate that is missing or stale can
// be waived: a BLOCKED/failed verdict, a limited result or unresolved material
// findings recorded by review-pr still stop the seal.
func sealWaivable(codes, waive []string) (rest []string, states map[string]string) {
	states = map[string]string{}
	for _, code := range codes {
		gate, why, _ := strings.Cut(code, ":")
		if slices.Contains(waive, gate) && (why == "missing" || strings.HasPrefix(why, "stale_")) {
			if why == "missing" || states[gate] == "" {
				states[gate] = strings.SplitN(why, "_", 2)[0]
			}
			continue
		}
		rest = append(rest, code)
	}
	return rest, states
}

func parseSealWaiver(kv map[string]string) ([]string, error) {
	if kv["waive"] == "" {
		if kv["reason"] != "" {
			return nil, fmt.Errorf("--reason belongs to --waive")
		}
		return nil, nil
	}
	if strings.TrimSpace(kv["reason"]) == "" {
		return nil, fmt.Errorf("--waive needs --reason \"<why the gate is not needed>\"")
	}
	var gates []string
	for _, g := range strings.Split(kv["waive"], ",") {
		g = strings.TrimSpace(g)
		if g != "review-pr" && g != "qa" {
			return nil, fmt.Errorf("--waive takes review-pr and/or qa, not %q", g)
		}
		if !slices.Contains(gates, g) {
			gates = append(gates, g)
		}
	}
	return gates, nil
}

func projectSealChild(st *ticket.Store, waive []string, reason string, fixup bool) error {
	return withLock(st, func() error {
		inputs, owners, err := projectChildInputs(st)
		if err != nil {
			return err
		}
		m, err := ticket.ReadManifest(st.ManifestPath())
		if err != nil {
			return err
		}
		if len(m.Repos) != 1 {
			return fmt.Errorf("one verified repository per child is required; split cross-repo work into dependent children")
		}
		seal := projectSeal{Version: 1, Ticket: st.Env.Ticket, At: isoNow(), Inputs: inputs, Owners: owners, Repos: []projectRepo{}}
		// A phase fixup is reviewed and re-tested on the phase commit.
		phaseReview := fixup
		for _, r := range m.Repos {
			if !filepath.IsAbs(r.Canonical) || !filepath.IsAbs(r.Worktree) {
				return fmt.Errorf("manifest needs absolute canonical and worktree paths")
			}
			env := st.Env
			env.Branch = r.Branch
			ready, err := evaluateReadinessIn(env, "review", r.Worktree)
			if err != nil {
				return err
			}
			if !ready.Ready {
				if len(waive) == 0 {
					// Gates staled only by adopting work outside the reviewed
					// diff stand; no worker is launched to re-stamp them.
					doc := ticket.ReadDoc(st.IndexPath())
					if why, ok := adoptionCarry(r.Worktree, st.Home(), doc.Get("pointers.review_tree"), ready.ReasonCodes); ok {
						waive, reason = []string{"review-pr", "qa"}, "carried: "+why
					} else if gates, why, ok := deferralCovers(r.Worktree, st.Home(), doc, ready.ReasonCodes); ok {
						waive, reason, phaseReview = gates, why, slices.Contains(gates, "review-pr")
					}
				}
				rest, states := sealWaivable(ready.ReasonCodes, waive)
				if len(rest) > 0 {
					return fmt.Errorf("child not verified: %v; if the gates are not needed or the work was verified in its handoff, seal with --waive <review-pr|qa>[,<gate>] --reason \"<text>\" (only missing or stale gates can be waived)", rest)
				}
				w := &projectWaiver{Gates: []string{}, Reason: strings.TrimSpace(reason), At: isoNow(), States: states, PhaseReview: phaseReview}
				for gate := range states {
					w.Gates = append(w.Gates, gate)
				}
				slices.Sort(w.Gates)
				seal.Waiver = w
			}
			cut := gitOutIn(r.Worktree, "rev-parse", "--verify", "-q", "HEAD^{commit}")
			if cut == "" || gitOutIn(r.Worktree, "branch", "--show-current") != r.Branch {
				return fmt.Errorf("worktree is not on the declared ticket branch")
			}
			// The worker's uncommitted tree is the verified work; the seal
			// pins it as a snapshot the phase composes from.
			head, err := sealTicketSnapshot(r.Worktree, st.Env.Ticket, readAdopted(st.Home()))
			if err != nil {
				return err
			}
			seal.Repos = append(seal.Repos, projectRepo{Canonical: r.Canonical, Branch: r.Branch, Head: head, Base: baseBranchIn(r.Canonical), BaseHead: cut})
		}
		if err := writeJSONAtomic(filepath.Join(st.Home(), "project-seal.json"), seal); err != nil {
			return err
		}
		projectEvent(st, "child_sealed", "", "", "")
		return nil
	})
}

func foremanProjectCommand(verb string, args []string) error {
	id, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	st, err := projectStore(id)
	if err != nil {
		return err
	}
	switch verb {
	case "contract":
		var c projectContract
		if err := readProjectJSON(kv["file"], &c); err != nil {
			return err
		}
		if err := validateProjectContract(c); err != nil {
			return err
		}
		if err := validateHandoverRelease(c); err != nil {
			return err
		}
		return withLock(st, func() error {
			doc, err := ticket.ReadDocStrict(st.IndexPath())
			if err != nil {
				return err
			}
			if doc.Get("approval.state") == "pending" {
				return fmt.Errorf("resolve or redirect the pending project review before replacing scope")
			}
			if err := writeJSONAtomic(projectContractPath(st), c); err != nil {
				return err
			}
			doc.Set("pointers.project_contract", "project.json")
			if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
				return err
			}
			projectEvent(st, "contract_written", "", "", "")
			return nil
		})
	case "bind":
		if !idRe.MatchString(kv["child"]) {
			return fmt.Errorf("bind needs --child and --seed (or --phase)")
		}
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		if len(s.Dispatch) > 0 {
			return fmt.Errorf("project not approved: %v", s.Dispatch)
		}
		if kv["phase"] != "" {
			return projectBindPhase(st, s, kv["phase"], kv["child"])
		}
		if !slices.Contains(s.Contract.Seeds, kv["seed"]) {
			return fmt.Errorf("seed is outside approved scope")
		}
		found := false
		for _, child := range s.Children {
			if child.ID == kv["child"] {
				found = true
			}
			if child.Seed == kv["seed"] && child.ID != kv["child"] {
				return fmt.Errorf("seed already bound to %s", child.ID)
			}
		}
		if !found {
			return fmt.Errorf("child must already belong to the ticket DAG")
		}
		cs := storeForTicket(st.Env, kv["child"])
		return withLock(cs, func() error {
			doc, err := ticket.ReadDocStrict(cs.IndexPath())
			if err != nil {
				return err
			}
			doc.Set("origin.project_seed", kv["seed"])
			return ticket.WriteDoc(cs.IndexPath(), doc)
		})
	case "seal":
		if kv["file"] != "" {
			return projectSealArtifacts(st, kv["file"])
		}
		if fix := strings.TrimSpace(kv["fixup"]); fix != "" {
			if kv["waive"] != "" || kv["reason"] != "" {
				return fmt.Errorf("--fixup takes no --waive/--reason; it waives the stale gates itself")
			}
			if d := openTicketDispatch(st.Env.Ticket); d != "" {
				return fmt.Errorf("--fixup: dispatch %s still owns this ticket's worktree; only a settled ticket takes a phase fixup", d)
			}
			return projectSealChild(st, []string{"review-pr", "qa"}, "phase fixup: "+fix, true)
		}
		waive, err := parseSealWaiver(kv)
		if err != nil {
			return err
		}
		return projectSealChild(st, waive, kv["reason"], false)
	case "evidence":
		return projectEvidenceCommand(st, kv)
	case "progress":
		return projectProgressCommand(st, kv)
	case "complete":
		return projectComplete(st, kv["foreman"])
	case "recap":
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		path, err := writeProjectRecap(st, s)
		if err != nil {
			return err
		}
		fmt.Println("RECAP=" + path)
		return nil
	case "release":
		return projectReleaseCommand(st, kv)
	case "integrate":
		return projectIntegrateCommand(st, kv)
	case "land":
		return projectLandCommand(st, kv)
	case "next":
		return projectNextCommand(st, kv)
	case "snapshot", "readiness":
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		if verb == "readiness" {
			action := kv["action"]
			reasons := s.Blockers
			switch {
			case action == "finish":
			case kv["phase"] != "" && (action == "dispatch" || action == "integrate" || action == "land" || action == "release"):
				reasons = projectPhaseReasons(s, action, kv["phase"])
			case action == "dispatch":
				reasons = s.Dispatch
			default:
				return fmt.Errorf("readiness requires --action finish, dispatch [--phase <id>], or integrate|land|release --phase <id>")
			}
			out := map[string]interface{}{"action": action, "ready": len(reasons) == 0, "reason_codes": reasons, "subject": s.Subject}
			if k := projectPhaseIndex(s, kv["phase"]); k >= 0 {
				out["phase"] = s.Phases[k]
			}
			printV2Envelope(out)
		} else {
			printV2Envelope(s)
		}
		return nil
	}
	return fmt.Errorf("unknown project command")
}

// foremanSealMany seals each named ticket independently: one ticket that is
// not ready does not hold back the rest of a settled batch.
func foremanSealMany(args []string) error {
	var ids []string
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		ids, args = append(ids, args[0]), args[1:]
	}
	if len(ids) <= 1 {
		return foremanProjectCommand("seal", append(ids, args...))
	}
	for _, a := range args {
		if a == "--file" {
			return fmt.Errorf("seal: --file seals one evidence-only ticket at a time")
		}
	}
	failed := 0
	for _, id := range ids {
		if err := foremanProjectCommand("seal", append([]string{id}, args...)); err != nil {
			failed++
			fmt.Printf("NOT_SEALED=%s %v\n", id, err)
			continue
		}
		fmt.Println("SEALED=" + id)
	}
	if failed > 0 {
		return fmt.Errorf("seal: %d of %d tickets not sealed", failed, len(ids))
	}
	return nil
}

// projectBindPhase marks a direct child of the parent as one phase's ticket:
// the parent of that phase's tickets and the holder of its release approval.
func projectBindPhase(st *ticket.Store, s projectSnapshot, phase, child string) error {
	if projectPhaseIndex(s, phase) < 0 {
		return fmt.Errorf("phase is not in the approved contract")
	}
	for _, p := range s.Phases {
		if p.ID == phase && p.Ticket != "" && p.Ticket != child {
			return fmt.Errorf("phase already bound to %s", p.Ticket)
		}
	}
	cs := storeForTicket(st.Env, child)
	return withLock(cs, func() error {
		doc, err := ticket.ReadDocStrict(cs.IndexPath())
		if err != nil {
			return err
		}
		if doc.Get("parent") != st.Env.Ticket {
			return fmt.Errorf("phase ticket must be a direct child of %s", st.Env.Ticket)
		}
		if seed := doc.Get("origin.project_seed"); seed != "" {
			return fmt.Errorf("ticket is already bound to seed %s", seed)
		}
		doc.Set("origin.project_phase", phase)
		return ticket.WriteDoc(cs.IndexPath(), doc)
	})
}
