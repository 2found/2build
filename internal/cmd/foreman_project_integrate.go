package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/2found/2build/internal/orca"
	"github.com/2found/2build/internal/ticket"
)

// One phase, one commit. Tickets never commit; their sealed snapshots are
// composed into a single phase commit on top of local base (`bbs team
// integrate`), `bbs team land` fast-forwards base to it at once, and phase QA
// runs on exactly that landed commit in a dedicated worktree. A QA fix lands
// as one fix commit; base history never gains a ticket's commit.

type projectCandidate struct {
	Version  int                    `json:"version"`
	Phase    string                 `json:"phase"`
	Subject  string                 `json:"subject"`
	Message  string                 `json:"message"`
	BuiltAt  string                 `json:"built_at"`
	LandedAt string                 `json:"landed_at,omitempty"`
	Repos    []projectCandidateRepo `json:"repos"`
}

type projectCandidateRepo struct {
	Canonical string   `json:"canonical"`
	Base      string   `json:"base"`
	BaseHead  string   `json:"base_head"`
	Commit    string   `json:"commit"`
	Worktree  string   `json:"worktree"`
	Tickets   []string `json:"tickets"`
	// Contracts the phase changes; their checks belong in phase QA.
	Contracts map[string][]string `json:"contracts_changed,omitempty"`
	// A fix after land: the phase is composed again on PhaseBase (the base
	// it was first built on) as Whole, and Commit carries only what changed
	// against the landed Whole. Empty on a first build (BaseHead, Commit).
	PhaseBase string `json:"phase_base,omitempty"`
	Whole     string `json:"whole,omitempty"`
}

func projectCandidatePath(st *ticket.Store, phase string) string {
	return filepath.Join(st.Home(), "project-phases", phase+".json")
}

// projectReadCandidate returns the phase's candidate when it was built from
// the phase's current subject and its commits still exist.
func projectReadCandidate(st *ticket.Store, p projectPhaseState) *projectCandidate {
	var c projectCandidate
	if readProjectJSON(projectCandidatePath(st, p.ID), &c) != nil || c.Version != 1 || c.Subject != p.Subject {
		return nil
	}
	for _, r := range c.Repos {
		if !gitOKIn(r.Canonical, "cat-file", "-e", r.Commit+"^{commit}") {
			return nil
		}
	}
	return &c
}

// projectCandidateLanded: every repo's phase commit is on its local base.
func projectCandidateLanded(c *projectCandidate) bool {
	if c == nil {
		return false
	}
	for _, r := range c.Repos {
		if !gitOKIn(r.Canonical, "merge-base", "--is-ancestor", r.Commit, "refs/heads/"+r.Base) {
			return false
		}
	}
	return true
}

// projectPhaseOrder sorts a phase's tickets so prerequisites compose first.
func projectPhaseOrder(children []projectChild) []projectChild {
	in := map[string]bool{}
	for _, c := range children {
		in[c.ID] = true
	}
	sorted := slices.Clone(children)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	var out []projectChild
	done := map[string]bool{}
	for len(out) < len(sorted) {
		progressed := false
		for _, c := range sorted {
			if done[c.ID] {
				continue
			}
			ready := true
			for _, dep := range c.BlockedBy {
				ready = ready && (!in[dep] || done[dep])
			}
			if ready {
				out = append(out, c)
				done[c.ID] = true
				progressed = true
			}
		}
		if !progressed { // a cycle is reported by the graph; keep going
			for _, c := range sorted {
				if !done[c.ID] {
					out = append(out, c)
					done[c.ID] = true
				}
			}
		}
	}
	return out
}

func commitIdentityEnv(dir string) []string {
	if gitOKIn(dir, "var", "GIT_COMMITTER_IDENT") && gitOKIn(dir, "var", "GIT_AUTHOR_IDENT") {
		return nil
	}
	return snapshotIdentity
}

// projectIntegrateCommand builds (or reuses) the phase commit and checks it
// out in the phase's integration worktree.
func projectIntegrateCommand(st *ticket.Store, kv map[string]string) error {
	return withLock(st, func() error {
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		k := projectPhaseIndex(s, kv["phase"])
		if k < 0 {
			return fmt.Errorf("integrate needs --phase <id> from the contract")
		}
		if reasons := projectPhaseReasons(s, "integrate", kv["phase"]); len(reasons) > 0 {
			return fmt.Errorf("phase not ready to integrate: %v", reasons)
		}
		p := s.Phases[k]
		if s.Contract.ArtifactsOnly {
			return fmt.Errorf("artifact-only projects have no code to integrate")
		}
		message := strings.TrimSpace(kv["message"])
		if message == "" {
			message = p.Title + " (phase " + p.ID + ")"
		}
		var phaseChildren []projectChild
		for _, child := range s.Children {
			if slices.Contains(p.Children, child.ID) {
				phaseChildren = append(phaseChildren, child)
			}
		}
		byRepo := map[string][]projectChild{}
		var repos []string
		for _, child := range projectPhaseOrder(phaseChildren) {
			for _, r := range child.Repos {
				if byRepo[r.Canonical] == nil {
					repos = append(repos, r.Canonical)
				}
				byRepo[r.Canonical] = append(byRepo[r.Canonical], child)
			}
		}
		prior := projectReadCandidate(st, p)
		// The last candidate under any subject: its worktree is reused, and
		// when it landed, this build is a fix on top of it.
		var last projectCandidate
		if readProjectJSON(projectCandidatePath(st, p.ID), &last) != nil || last.Version != 1 {
			last = projectCandidate{}
		}
		c := projectCandidate{Version: 1, Phase: p.ID, Subject: p.Subject, Message: message, BuiltAt: isoNow(), Repos: []projectCandidateRepo{}}
		for _, canonical := range repos {
			base := baseBranchIn(canonical)
			baseHead := gitOutIn(canonical, "rev-parse", "--verify", "-q", "refs/heads/"+base+"^{commit}")
			if baseHead == "" {
				return fmt.Errorf("%s: local base branch %s not found", canonical, base)
			}
			r := projectCandidateRepo{Canonical: canonical, Base: base, BaseHead: baseHead, Tickets: []string{}}
			// A retry reuses the phase's worktree wherever it was created
			// (Orca's workspace dir or .babysit/worktrees/).
			var landed *projectCandidateRepo
			for i, pr := range last.Repos {
				if pr.Canonical != canonical {
					continue
				}
				if fi, err := os.Stat(pr.Worktree); pr.Worktree != "" && err == nil && fi.IsDir() {
					r.Worktree = pr.Worktree
				}
				if last.LandedAt != "" && gitOKIn(canonical, "merge-base", "--is-ancestor", pr.Commit, baseHead) {
					landed = &last.Repos[i]
				}
			}
			snapshots, owner := []string{}, map[string]string{}
			for _, child := range byRepo[canonical] {
				for _, cr := range child.Repos {
					if cr.Canonical == canonical {
						snapshots = append(snapshots, cr.Head)
						owner[cr.Head] = child.ID
					}
				}
				r.Tickets = append(r.Tickets, child.ID)
			}
			// Same subject, same base: the tested commit is reused, so a
			// retry never invalidates QA already recorded against it.
			if prior != nil && prior.Message == message {
				for _, pr := range prior.Repos {
					if pr.Canonical == canonical && pr.BaseHead == baseHead {
						r.Commit, r.PhaseBase, r.Whole = pr.Commit, pr.PhaseBase, pr.Whole
					}
				}
			}
			composeOn := baseHead
			if r.Commit == "" && landed != nil {
				composeOn = orDefault(landed.PhaseBase, landed.BaseHead)
			}
			if r.Commit == "" {
				tree, _, conflicts, err := mergeCommits(canonical, composeOn, snapshots)
				if err != nil {
					return err
				}
				if len(conflicts) > 0 {
					var parts []string
					for _, snap := range snapshots {
						if files := conflicts[snap]; len(files) > 0 {
							parts = append(parts, owner[snap]+": "+strings.Join(files, ", "))
						}
					}
					return fmt.Errorf("phase %s does not compose; repair the later ticket (bbs ticket adopt the earlier one, resolve, reseal): %s", p.ID, strings.Join(parts, "; "))
				}
				body := message + "\n\nPhase " + p.ID + " of " + st.Env.Ticket + ": " + strings.Join(r.Tickets, ", ")
				if landed != nil {
					// Base already holds the landed phase: land only the fix.
					whole, err := commitTree(canonical, commitIdentityEnv(canonical), tree, body, composeOn)
					if err != nil {
						return err
					}
					if tree, err = mergeTreeOnBase(canonical, orDefault(landed.Whole, landed.Commit), baseHead, whole); err != nil {
						return fmt.Errorf("phase %s fix does not apply on base: %v", p.ID, err)
					}
					r.PhaseBase, r.Whole, body = composeOn, whole, "Fix: "+body
				}
				if r.Commit, err = commitTree(canonical, commitIdentityEnv(canonical), tree, body, baseHead); err != nil {
					return err
				}
			}
			changed, err := contractsChanged(canonical, baseHead, r.Commit)
			if err != nil {
				return err
			}
			if conflicts := contractOwnerConflicts(changed, ticket.ReadDoc(st.IndexPath()).Get("assignee")); len(conflicts) > 0 {
				return fmt.Errorf("phase %s changes contracts another team owns: %s; ask the owner for the change (bbs ticket assign a ticket to it) and build against its new version", p.ID, strings.Join(conflicts, ", "))
			}
			if len(changed) > 0 {
				after, _ := readContracts(canonical, r.Commit)
				r.Contracts = map[string][]string{}
				for id := range changed {
					r.Contracts[id] = after[id].Checks
				}
			}
			if _, err := gitPlumb(canonical, nil, "update-ref", phaseCandidateRef(st.Env.Ticket, p.ID), r.Commit); err != nil {
				return err
			}
			if r.Worktree, err = checkoutDetached(canonical, r.Worktree, st.Env.Ticket+"_phase-"+p.ID, r.Commit); err != nil {
				return err
			}
			c.Repos = append(c.Repos, r)
		}
		if err := writeJSONAtomic(projectCandidatePath(st, p.ID), c); err != nil {
			return err
		}
		projectEvent(st, "phase_integrated", p.ID, "", p.Subject)
		printV2Envelope(c)
		return nil
	})
}

// mergeTreeOnBase three-way merges theirs into ours over an explicit merge
// base and returns the tree; a conflict is an error naming the files.
func mergeTreeOnBase(dir, base, ours, theirs string) (string, error) {
	c := exec.Command("git", "merge-tree", "--write-tree", "--name-only", "--no-messages", "--merge-base="+base, ours, theirs)
	c.Dir = dir
	out, err := c.Output()
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 && len(lines) > 1 {
		return "", fmt.Errorf("conflicts in %s", strings.Join(lines[1:], ", "))
	}
	if err != nil || lines[0] == "" {
		return "", fmt.Errorf("git merge-tree failed: %v", err)
	}
	return lines[0], nil
}

// checkoutDetached puts the phase's dedicated worktree at commit and returns
// its path. An existing path is reset — it belongs to the phase, so anything
// left in it is discarded. Otherwise it is created through Orca when the lead
// runs in an Orca terminal (Orca's location rule; the scratch branch Orca
// requires is dropped once detached), else with git under .babysit/worktrees/.
func checkoutDetached(canonical, path, name, commit string) (string, error) {
	legacy := filepath.Join(canonical, ".babysit", "worktrees", name)
	if path == "" {
		if fi, err := os.Stat(legacy); err == nil && fi.IsDir() {
			path = legacy
		}
	}
	if path != "" {
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			if _, err := gitPlumb(path, nil, "checkout", "--detach", "--force", commit); err != nil {
				return "", err
			}
			_, err := gitPlumb(path, nil, "clean", "-fdq")
			return path, err
		}
	}
	_, _ = gitPlumb(canonical, nil, "worktree", "prune")
	if wt, ok := orcaWorktreeAdd(canonical, name, "bbs-phase/"+name, commit); ok {
		if _, err := gitPlumb(wt.Path, nil, "checkout", "--detach", "--force", commit); err != nil {
			return "", err
		}
		_, _ = gitPlumb(canonical, nil, "branch", "-D", wt.Branch)
		return wt.Path, nil
	}
	if path == "" {
		path = legacy
	}
	excludeWorktreesIn(canonical)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	_, err := gitPlumb(canonical, nil, "worktree", "add", "--detach", path, commit)
	return path, err
}

// branchCheckout returns the worktree that has branch checked out, if any.
// A submodule's main worktree is listed as its git dir
// (.git/modules/<name>, core.worktree pointing back), so the listed path is
// resolved to its working tree.
func branchCheckout(canonical, branch string) string {
	dir := ""
	for _, line := range strings.Split(gitOutIn(canonical, "worktree", "list", "--porcelain"), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			dir = strings.TrimPrefix(line, "worktree ")
		case line == "branch refs/heads/"+branch:
			if top := gitOutIn(dir, "rev-parse", "--show-toplevel"); top != "" {
				return top
			}
			return dir
		}
	}
	return ""
}

// projectLandCommand fast-forwards each repo's local base to the phase
// commit as soon as the phase is sealed and integrated; phase QA runs on the
// landed commit, and its worktrees stay for QA and fixes until it passes.
func projectLandCommand(st *ticket.Store, kv map[string]string) error {
	return withLock(st, func() error {
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		k := projectPhaseIndex(s, kv["phase"])
		if k < 0 {
			return fmt.Errorf("land needs --phase <id> from the contract")
		}
		if reasons := projectPhaseReasons(s, "land", kv["phase"]); len(reasons) > 0 {
			return fmt.Errorf("phase not ready to land: %v", reasons)
		}
		p := s.Phases[k]
		c := p.Candidate
		if c == nil {
			return fmt.Errorf("phase %s has no phase commit to land (artifact-only phases are delivered by their evidence)", p.ID)
		}
		// Base moved since integrate: nothing is tested yet, so rebuilding the
		// phase commit on the new tip is cheap. Refuse before anything moves.
		for _, r := range c.Repos {
			ref := "refs/heads/" + r.Base
			tip := gitOutIn(r.Canonical, "rev-parse", "--verify", "-q", ref+"^{commit}")
			if tip != r.BaseHead && !gitOKIn(r.Canonical, "merge-base", "--is-ancestor", r.Commit, ref) {
				return fmt.Errorf("%s moved since phase %s was integrated (%s → %s); run bbs team integrate again, then land", r.Base, p.ID, short(r.BaseHead), short(tip))
			}
		}
		for _, r := range c.Repos {
			ref := "refs/heads/" + r.Base
			if gitOKIn(r.Canonical, "merge-base", "--is-ancestor", r.Commit, ref) {
				continue
			}
			if dir := branchCheckout(r.Canonical, r.Base); dir != "" {
				if err := fastForwardCheckout(dir, ref, r.BaseHead, r.Commit); err != nil {
					return fmt.Errorf("cannot fast-forward %s in %s: %v", r.Base, dir, err)
				}
			} else if _, err := gitPlumb(r.Canonical, nil, "update-ref", ref, r.Commit, r.BaseHead); err != nil {
				return err
			}
		}
		c.LandedAt = isoNow()
		if err := writeJSONAtomic(projectCandidatePath(st, p.ID), *c); err != nil {
			return err
		}
		projectEvent(st, "phase_landed", p.ID, "", p.Subject)
		printV2Envelope(map[string]interface{}{"phase": p.ID, "landed": c.Repos, "next": "phase QA on the landed commit: bbs team evidence " + st.Env.Ticket + " --phase " + p.ID + " --begin --file spec.json"})
		return nil
	})
}

// fastForwardCheckout moves branch ref from old to new in the checkout dir
// that has it, carrying local changes along. Every path the phase changes
// must hold, in both the index and the working tree, either its old content
// (it is updated) or already its new content — a trunk ticket's own
// uncommitted work, which the phase commit now records. Any other local edit
// on such a path refuses the whole land before anything moves; paths the
// phase does not touch are never read or written. Nothing is stashed or reset.
func fastForwardCheckout(dir, ref, old, new string) error {
	type change struct{ path, blob string }
	var updates []change
	var overlap []string
	for _, path := range strings.Split(gitOutIn(dir, "diff", "--name-only", "--no-renames", old, new), "\n") {
		if path == "" {
			continue
		}
		oldBlob := gitOutIn(dir, "rev-parse", "--verify", "-q", old+":"+path)
		newBlob := gitOutIn(dir, "rev-parse", "--verify", "-q", new+":"+path)
		wt := ""
		if _, err := os.Lstat(filepath.Join(dir, path)); err == nil {
			wt = gitOutIn(dir, "hash-object", "--", path)
		}
		idx := ""
		if fields := strings.Fields(gitOutIn(dir, "ls-files", "-s", "--", path)); len(fields) >= 2 {
			idx = fields[1]
		}
		switch {
		case wt != oldBlob && wt != newBlob, idx != oldBlob && idx != newBlob:
			overlap = append(overlap, path)
		default:
			updates = append(updates, change{path, newBlob})
		}
	}
	if len(overlap) > 0 {
		return fmt.Errorf("local changes there overlap the phase (%s); the owner of that checkout must commit or move them, then land again", strings.Join(overlap, ", "))
	}
	if _, err := gitPlumb(dir, nil, "update-ref", ref, new, old); err != nil {
		return err
	}
	for _, u := range updates {
		if u.blob == "" {
			if _, err := gitPlumb(dir, nil, "rm", "-q", "--cached", "--ignore-unmatch", "--", u.path); err != nil {
				return err
			}
			if err := os.Remove(filepath.Join(dir, u.path)); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if _, err := gitPlumb(dir, nil, "checkout", new, "--", u.path); err != nil {
			return err
		}
	}
	return nil
}

// projectRemovePhaseWorktrees drops the integration worktree and every
// ticket worktree whose content still equals its sealed snapshot — the
// snapshot ref keeps that work. Anything else is kept and reported.
func projectRemovePhaseWorktrees(st *ticket.Store, s projectSnapshot, p projectPhaseState, c *projectCandidate) (removed, kept, closed []string) {
	removed, kept = []string{}, []string{}
	type target struct{ canonical, path, branch string }
	var targets []target
	drop := func(canonical, path, branch string) {
		targets = append(targets, target{canonical, path, branch})
	}
	for _, r := range c.Repos {
		if _, err := os.Stat(r.Worktree); err == nil {
			drop(r.Canonical, r.Worktree, "")
		}
	}
	for _, child := range s.Children {
		if !slices.Contains(p.Children, child.ID) {
			continue
		}
		m, err := ticket.ReadManifest(storeForTicket(st.Env, child.ID).ManifestPath())
		if err != nil {
			continue
		}
		for _, sealed := range child.Repos {
			mr := m.FindRepo(func(x ticket.Repo) bool { return x.Canonical == sealed.Canonical && x.Branch == sealed.Branch })
			if mr == nil || mr.Worktree == "" || mr.Worktree == sealed.Canonical {
				continue
			}
			if _, err := os.Stat(mr.Worktree); err != nil {
				continue
			}
			if tree, err := worktreeTree(mr.Worktree); err != nil || tree != treeOf(sealed.Canonical, sealed.Head) {
				kept = append(kept, mr.Worktree)
				continue
			}
			drop(sealed.Canonical, mr.Worktree, sealed.Branch)
		}
	}
	// The phase's ticket sessions and its QA worker were kept only for
	// repairs; once it is landed and verified nothing reuses them, so their agents go with
	// their worktrees instead of holding memory until someone closes them.
	paths := make([]string, 0, len(targets))
	for _, t := range targets {
		paths = append(paths, t.path)
	}
	closed = closeLandedWorkerTerminals(paths)
	for _, t := range targets {
		if _, err := gitPlumb(t.canonical, nil, "worktree", "remove", "--force", t.path); err != nil {
			kept = append(kept, t.path)
			continue
		}
		removed = append(removed, t.path)
		if t.branch != "" {
			_, _ = gitPlumb(t.canonical, nil, "branch", "-D", t.branch)
		}
	}
	return removed, kept, closed
}

// closeLandedWorkerTerminals closes every Orca terminal working in paths and
// returns their handles. Best effort: without Orca there is nothing to close,
// and the caller's own terminal is never closed.
var closeLandedWorkerTerminals = func(paths []string) []string {
	closed := []string{}
	if len(paths) == 0 {
		return closed
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := orca.PreflightContext(ctx)
	if err != nil {
		return closed
	}
	terms, err := client.Terminals()
	if err != nil {
		return closed
	}
	want := map[string]bool{}
	for _, path := range paths {
		want[physicalPath(path)] = true
	}
	self := os.Getenv("ORCA_TERMINAL_HANDLE")
	for _, t := range terms {
		if t.Handle == "" || t.Handle == self || t.WorktreePath == "" || !want[physicalPath(t.WorktreePath)] {
			continue
		}
		if client.Close(t.Handle) == nil {
			closed = append(closed, t.Handle)
		}
	}
	return closed
}

func physicalPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return filepath.Clean(path)
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	if sha == "" {
		return "(none)"
	}
	return sha
}

// projectObserveDelivery reports, per phase and repository, whether the
// phase commit is on local base. An artifact-only phase delivers its sealed
// artifacts once its evidence passes.
func projectObserveDelivery(st *ticket.Store, s projectSnapshot) []projectDelivery {
	rows := []projectDelivery{}
	for _, p := range s.Phases {
		if s.Contract != nil && s.Contract.ArtifactsOnly {
			d := projectDelivery{Phase: p.ID, State: "PENDING"}
			if p.Integrated {
				d.State = "ARTIFACTS_READY"
			}
			rows = append(rows, d)
			continue
		}
		if p.Candidate == nil {
			continue
		}
		for _, r := range p.Candidate.Repos {
			d := projectDelivery{Phase: p.ID, Repo: r.Canonical, Base: r.Base, Commit: r.Commit, State: "PENDING"}
			if gitOKIn(r.Canonical, "merge-base", "--is-ancestor", r.Commit, "refs/heads/"+r.Base) {
				d.State = "LANDED"
			}
			rows = append(rows, d)
		}
	}
	return rows
}
