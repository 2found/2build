package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/2found/2build/internal/ticket"
)

// A phased project ships in ordered release increments. Each phase owns a set
// of approved seeds, is integrated and QA'd cumulatively (everything in phases
// up to and including it), delivered to base, and released before a later
// phase may integrate on top of it. A phase ticket — a direct child of the
// parent, parent of that phase's tickets — carries the phase's release
// approval, which only a human may grant.
//
// The order is enforced here, not in prose: dispatch of phase k needs phase
// k-1 integrated and delivered (its tickets build on it) and everything
// before k-1 release-resolved, so the next phase can build while the previous
// one waits on its release; integrating phase k needs every earlier phase
// release-resolved, so a release never ships work that was not its own.

type projectPhase struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Seeds   []string `json:"seeds"`
	Release string   `json:"release"` // auto | ask | none
}

type projectPhaseState struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Release  string           `json:"release"`
	Ticket   string           `json:"ticket"`
	Seeds    []string         `json:"seeds"`
	Children []string         `json:"children"`
	Subject  string           `json:"subject"`
	State    string           `json:"state"`
	Evidence []string         `json:"evidence"`
	Releases []projectRelease `json:"releases"`
	Problems []string         `json:"problems"`
	// Notes are transparency lines (seal waivers, carried evidence); never blockers.
	Notes []string `json:"notes,omitempty"`

	Sealed, Integrated, Delivered, Released bool `json:"-"`
	// Failed counts ROLLED_BACK/PARTIAL receipts; LastFailed is true while
	// the newest receipt is one of them (the line is stopped).
	Failed     int  `json:"failed_releases"`
	LastFailed bool `json:"-"`
	ApprovalGo bool `json:"-"`
	// Candidate is the phase commit built from the current subject, if any.
	Candidate *projectCandidate `json:"candidate,omitempty"`
}

type projectRelease struct {
	Version int                  `json:"version"`
	ID      string               `json:"id"`
	Phase   string               `json:"phase"`
	Subject string               `json:"subject"`
	At      string               `json:"at"`
	Verdict string               `json:"verdict"`
	Tag     string               `json:"tag,omitempty"`
	Record  string               `json:"record"`
	Repos   []projectReleaseRepo `json:"repos"`
}

type projectReleaseRepo struct {
	Dir string `json:"dir"`
	SHA string `json:"sha"`
}

type projectReleaseRequest struct {
	Version  int               `json:"version"`
	Parent   string            `json:"parent"`
	Phase    string            `json:"phase"`
	Subject  string            `json:"subject"`
	Evidence []string          `json:"evidence"`
	Heads    map[string]string `json:"heads"`
	// FailedBefore pins the go to the attempt after this many failures: a go
	// given before a failed release never authorizes the retry.
	FailedBefore int    `json:"failed_before"`
	Note         string `json:"note,omitempty"`
}

var releaseVerdicts = map[string]bool{"RELEASED": true, "ROLLED_BACK": true, "PARTIAL": true}

func validateProjectPhases(c projectContract) error {
	if len(c.Phases) == 0 {
		return fmt.Errorf("project contract needs phases (a one-release project is one phase)")
	}
	owner, ids := map[string]string{}, map[string]bool{}
	for _, p := range c.Phases {
		if !idRe.MatchString(p.ID) || ids[p.ID] || strings.TrimSpace(p.Title) == "" || len(p.Seeds) == 0 {
			return fmt.Errorf("phase %q needs a unique ID, title and seeds", p.ID)
		}
		if p.Release != "auto" && p.Release != "ask" && p.Release != "none" {
			return fmt.Errorf("phase %s: release must be auto, ask or none", p.ID)
		}
		ids[p.ID] = true
		for _, seed := range p.Seeds {
			if !slices.Contains(c.Seeds, seed) || owner[seed] != "" {
				return fmt.Errorf("phase %s: seed %q is unknown or already in phase %s", p.ID, seed, owner[seed])
			}
			owner[seed] = p.ID
		}
	}
	for _, seed := range c.Seeds {
		if owner[seed] == "" {
			return fmt.Errorf("seed %s belongs to no phase", seed)
		}
	}
	// The walking skeleton ships first: the first journey is phase 1's job.
	for _, ac := range c.Criteria {
		if slices.Contains(c.FirstJourney, ac.ID) && owner[ac.Owner] != c.Phases[0].ID {
			return fmt.Errorf("first_journey criterion %s must be owned by phase %s", ac.ID, c.Phases[0].ID)
		}
	}
	return nil
}

func projectPhaseIndex(s projectSnapshot, id string) int {
	for i, p := range s.Phases {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// projectPhaseChildren returns the leaf children cumulatively in phases ≤ id.
func projectPhaseChildren(s projectSnapshot, id string) []projectChild {
	k := projectPhaseIndex(s, id)
	var out []projectChild
	for i := 0; i <= k; i++ {
		for _, child := range s.Children {
			if slices.Contains(s.Phases[i].Seeds, child.Seed) {
				out = append(out, child)
			}
		}
	}
	return out
}

// projectEvidenceSubject is the subject a receipt must carry to be current.
func projectEvidenceSubject(s projectSnapshot, ev projectEvidence) string {
	if k := projectPhaseIndex(s, ev.Phase); k >= 0 {
		return s.Phases[k].Subject
	}
	return "unknown phase"
}

// projectPhaseSetup binds phase tickets and computes each phase's subject:
// the shared approved inputs plus every leaf in phases up to it.
func projectPhaseSetup(st *ticket.Store, s *projectSnapshot, inputs map[string]interface{}, phaseNodes, leafParents map[string]string, external map[string]bool) {
	shared := map[string]interface{}{}
	for _, key := range []string{"contract", "approval", "approval_state", "run"} {
		shared[key] = inputs[key]
	}
	for id := range external {
		shared[id] = inputs[id]
	}
	for phase := range phaseNodes {
		known := false
		for _, p := range s.Contract.Phases {
			known = known || p.ID == phase
		}
		if !known {
			s.Blockers = append(s.Blockers, "phase "+phase+": not in the approved contract")
		}
	}
	nodes := map[string]interface{}{}
	for _, p := range s.Contract.Phases {
		ps := projectPhaseState{ID: p.ID, Title: p.Title, Release: p.Release, Ticket: phaseNodes[p.ID], Seeds: p.Seeds, Children: []string{}, Evidence: []string{}, Releases: []projectRelease{}, Problems: []string{}}
		for _, child := range s.Children {
			if !slices.Contains(p.Seeds, child.Seed) {
				continue
			}
			ps.Children = append(ps.Children, child.ID)
			nodes[child.ID] = inputs[child.ID]
			if ps.Ticket != "" && leafParents[child.ID] != ps.Ticket {
				s.Blockers = append(s.Blockers, "child "+child.ID+": belongs under phase ticket "+ps.Ticket)
			}
		}
		ps.Subject = digestJSON(map[string]interface{}{"phase": p.ID, "shared": shared, "nodes": nodes})
		ps.Candidate = projectReadCandidate(st, ps)
		s.Phases = append(s.Phases, ps)
	}
}

// projectPhaseProgress derives each phase's integration, delivery and
// release state and the finish blockers they imply.
func projectPhaseProgress(st *ticket.Store, s *projectSnapshot) {
	releases := projectReadReleases(st)
	for k := range s.Phases {
		p := &s.Phases[k]
		children := projectPhaseChildren(*s, p.ID)
		// Sealed: every seed of this phase is bound to a child with no problems.
		p.Sealed = true
		for _, seed := range p.Seeds {
			bound := false
			for _, child := range children {
				if child.Seed == seed {
					bound = true
					if len(child.Problems) > 0 {
						p.Sealed = false
						p.Problems = append(p.Problems, "child "+child.ID+" not sealed")
					}
				}
			}
			if !bound {
				p.Sealed = false
				p.Problems = append(p.Problems, "no child bound to "+seed)
			}
		}
		var unreviewed []string
		for _, child := range children {
			if w := child.Waiver; w != nil {
				p.Notes = append(p.Notes, fmt.Sprintf("child %s sealed without %s (%s) on %s: %s", child.ID, strings.Join(w.Gates, ", "), projectWaiverStates(w), w.At, w.Reason))
				if w.PhaseReview {
					unreviewed = append(unreviewed, child.ID)
				}
			}
		}
		latest := map[string]projectEvidence{}
		current := map[string][]projectEvidence{}
		for _, ev := range s.Evidence {
			if ev.Phase == p.ID && ev.Subject == p.Subject {
				if ev.State == "passed" {
					current[ev.Kind] = append(current[ev.Kind], ev)
				}
				if projectTimeAfter(ev.Finished, latest[ev.Kind].Finished) {
					latest[ev.Kind] = ev
				}
			}
		}
		p.Integrated = p.Sealed
		kinds := []string{"integration"}
		if s.Contract.ProductReview {
			kinds = append(kinds, "product")
		}
		for _, kind := range kinds {
			ev, ok := latest[kind]
			switch {
			case !ok:
				p.Integrated = false
				p.Problems = append(p.Problems, kind+": current phase evidence required")
			case ev.State != "passed":
				p.Integrated = false
				for _, problem := range ev.Problems {
					p.Problems = append(p.Problems, kind+": "+problem)
				}
			default:
				p.Evidence = append(p.Evidence, ev.ID)
				// A fresh partial attempt and a carried record together
				// satisfy the cumulative check below.
				for _, other := range current[kind] {
					if other.ID != ev.ID && (other.Carried != nil || ev.Carried != nil) {
						p.Evidence = append(p.Evidence, other.ID)
					}
				}
			}
			for _, other := range current[kind] {
				if c := other.Carried; c != nil {
					p.Notes = append(p.Notes, fmt.Sprintf("%s evidence %s carries checks from %s: %s", kind, other.ID, strings.Join(c.From, ","), c.Reason))
				}
			}
		}
		// Work no ticket session reviewed is reviewed once, on the phase commit.
		if len(unreviewed) > 0 && !projectCheckPassedAny(current["integration"], p.ID, "review") {
			p.Integrated = false
			p.Problems = append(p.Problems, "phase review: integration evidence needs a passing "+p.ID+"/review check (review-pr on the phase diff; unreviewed: "+strings.Join(unreviewed, ", ")+")")
		}
		// Phase QA is cumulative: every criterion owned by phases ≤ k.
		for _, ac := range s.Contract.Criteria {
			if !projectSeedInPhases(*s, ac.Owner, k) {
				continue
			}
			for _, kind := range ac.Checks {
				if !projectCheckPassedAny(current["integration"], ac.ID, kind) {
					p.Integrated = false
					p.Problems = append(p.Problems, "criterion "+ac.ID+": missing "+kind)
				}
			}
			if s.Contract.ProductReview && !projectCheckPassedAny(current["product"], ac.ID, "product") {
				p.Integrated = false
				p.Problems = append(p.Problems, "criterion "+ac.ID+": missing product")
			}
		}
		p.Delivered = p.Integrated && (s.Contract.ArtifactsOnly || projectCandidateLanded(p.Candidate))
		if p.Integrated && !p.Delivered {
			p.Problems = append(p.Problems, "delivery: phase commit not landed on base (bbs foreman land)")
		}
		for _, r := range releases {
			if r.Phase != p.ID {
				continue
			}
			p.Releases = append(p.Releases, r)
			if r.Verdict != "RELEASED" {
				p.Failed++
			}
		}
		if n := len(p.Releases); n > 0 {
			last := p.Releases[n-1]
			p.LastFailed = last.Verdict != "RELEASED"
			p.Released = !p.LastFailed && last.Subject == p.Subject
		}
		if p.Ticket != "" {
			p.ApprovalGo = projectReleaseApproved(st, *p)
		}
		p.State = projectPhaseStateName(*p)
	}
	// Coverage reads each criterion from the latest phase that proved it.
	for _, ac := range s.Contract.Criteria {
		row := projectCoverage{projectCriterion: ac, State: "pending", Evidence: []string{}}
		for k := len(s.Phases) - 1; k >= 0; k-- {
			if s.Phases[k].Integrated && projectSeedInPhases(*s, ac.Owner, k) {
				row.State = "proven"
				row.Evidence = s.Phases[k].Evidence
				break
			}
		}
		s.Coverage = append(s.Coverage, row)
	}
	for _, p := range s.Phases {
		if !projectPhaseResolved(p) {
			reason := p.State
			if len(p.Problems) > 0 {
				reason += ": " + strings.Join(p.Problems, "; ")
			}
			s.Blockers = append(s.Blockers, "phase "+p.ID+": "+reason)
		}
	}
}

func projectSeedInPhases(s projectSnapshot, seed string, k int) bool {
	for i := 0; i <= k && i < len(s.Phases); i++ {
		if slices.Contains(s.Phases[i].Seeds, seed) {
			return true
		}
	}
	return false
}

func projectPhaseStateName(p projectPhaseState) string {
	switch {
	case p.LastFailed:
		return "release_failed"
	case p.Released:
		return "released"
	case p.Delivered:
		return "delivered"
	case p.Integrated:
		return "integrated"
	case p.Sealed:
		return "sealed"
	case len(p.Children) > 0:
		return "building"
	}
	return "pending"
}

// projectPhaseResolved: a release phase is done when released; a phase that
// ships with a later one is done when delivered.
func projectPhaseResolved(p projectPhaseState) bool {
	if p.Release == "none" {
		return p.Delivered
	}
	return p.Released && p.Delivered
}

// projectSameRepo accepts a linked worktree of the canonical checkout, so a
// phase can integrate beside a dirty shared primary.
func projectSameRepo(dir, canonical string) bool {
	if dir == canonical {
		return true
	}
	common := func(d string) string {
		out := gitOutIn(d, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if out == "" {
			return ""
		}
		if real, err := filepath.EvalSymlinks(out); err == nil {
			return real
		}
		return out
	}
	a := common(dir)
	return a != "" && a == common(canonical)
}

func projectReadReleases(st *ticket.Store) []projectRelease {
	paths, _ := filepath.Glob(filepath.Join(st.Home(), "project-releases", "*.json"))
	var out []projectRelease
	for _, path := range paths {
		var r projectRelease
		if readProjectJSON(path, &r) == nil && r.Version == 1 {
			out = append(out, r)
		}
	}
	slices.SortStableFunc(out, func(a, b projectRelease) int { return strings.Compare(a.At, b.At) })
	return out
}

func projectReleaseRequestPath(phaseTicket *ticket.Store) string {
	return filepath.Join(phaseTicket.Home(), "release-request.json")
}

// projectReleaseApproved: a current human `release` approval on the phase
// ticket for exactly this phase subject.
func projectReleaseApproved(st *ticket.Store, p projectPhaseState) bool {
	ps := storeForTicket(st.Env, p.Ticket)
	doc := ticket.ReadDoc(ps.IndexPath())
	if doc.Get("approval.kind") != "release" {
		return false
	}
	if state, _, _ := approvalRead(ps); state != "approved" {
		return false
	}
	var req projectReleaseRequest
	return readProjectJSON(projectReleaseRequestPath(ps), &req) == nil && req.Subject == p.Subject && req.Phase == p.ID && req.FailedBefore == p.Failed
}

// projectPhaseReasons answers readiness for one phase action.
func projectPhaseReasons(s projectSnapshot, action, phase string) []string {
	k := projectPhaseIndex(s, phase)
	if k < 0 {
		return []string{"phase: unknown phase " + phase}
	}
	reasons := []string{}
	for _, p := range s.Dispatch {
		if action == "dispatch" || !strings.HasPrefix(p, "budget:") {
			reasons = append(reasons, p)
		}
	}
	p := s.Phases[k]
	for j := 0; j < k; j++ {
		prev := s.Phases[j]
		if prev.LastFailed {
			reasons = append(reasons, "stop the line: phase "+prev.ID+" release failed; fix forward and re-release it first")
		}
		switch {
		case action == "dispatch" && j == k-1:
			if !prev.Delivered {
				reasons = append(reasons, "phase "+prev.ID+": integrate and deliver it first ("+prev.State+")")
			}
		case !projectPhaseResolved(prev):
			reasons = append(reasons, "phase "+prev.ID+": release it first ("+prev.State+")")
		}
	}
	switch action {
	case "dispatch":
		if p.Ticket == "" {
			reasons = append(reasons, "phase "+p.ID+": bind a phase ticket first (bbs foreman bind --phase)")
		}
	case "integrate", "verify":
		if !p.Sealed {
			// Only the seal problems block here; evidence comes after integrate.
			var unsealed []string
			for _, problem := range p.Problems {
				if strings.HasPrefix(problem, "child ") || strings.HasPrefix(problem, "no child bound") {
					unsealed = append(unsealed, problem)
				}
			}
			reasons = append(reasons, "phase "+p.ID+": "+strings.Join(unsealed, "; "))
		}
		if action == "verify" && p.Candidate == nil && !s.Contract.ArtifactsOnly {
			reasons = append(reasons, "phase "+p.ID+": no current phase commit (bbs foreman integrate)")
		}
		if projectCandidateLanded(p.Candidate) {
			reasons = append(reasons, "phase "+p.ID+": already landed")
		}
	case "land":
		if !p.Integrated {
			reasons = append(reasons, "phase "+p.ID+": phase QA has not passed on the current phase commit ("+strings.Join(p.Problems, "; ")+")")
		}
		if p.Candidate == nil && !s.Contract.ArtifactsOnly {
			reasons = append(reasons, "phase "+p.ID+": no current phase commit (bbs foreman integrate)")
		}
	case "release", "request":
		if p.Release == "none" {
			reasons = append(reasons, "phase "+p.ID+": ships with a later phase (release: none)")
		}
		if p.Released {
			reasons = append(reasons, "phase "+p.ID+": already released")
		}
		if !p.Delivered {
			reasons = append(reasons, "phase "+p.ID+": not integrated and delivered ("+p.State+")")
		}
		if p.Failed >= 2 && action == "release" && !p.ApprovalGo {
			reasons = append(reasons, "stop the line: phase "+p.ID+" failed to release twice; a human release approval is required")
		} else if p.Release == "ask" && action == "release" && !p.ApprovalGo {
			reasons = append(reasons, "phase "+p.ID+": human release approval required (bbs foreman release --request)")
		}
	default:
		return []string{"readiness requires --action dispatch|integrate|land|release|finish"}
	}
	return reasons
}

// projectReleaseCommand records a release request (human approval) or a
// release receipt for one phase.
func projectReleaseCommand(st *ticket.Store, kv map[string]string) error {
	return withLock(st, func() error {
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		if len(s.Phases) == 0 {
			return fmt.Errorf("release: the project contract has no phases")
		}
		k := projectPhaseIndex(s, kv["phase"])
		if k < 0 {
			return fmt.Errorf("release needs --phase <id> from the contract")
		}
		p := s.Phases[k]
		if kv["request"] == "1" {
			if reasons := projectPhaseReasons(s, "request", p.ID); len(reasons) > 0 {
				return fmt.Errorf("phase not releasable: %v", reasons)
			}
			if p.Ticket == "" {
				return fmt.Errorf("phase %s has no phase ticket to carry the approval", p.ID)
			}
			heads := map[string]string{}
			if p.Candidate != nil {
				for _, r := range p.Candidate.Repos {
					heads[r.Canonical] = r.Commit
				}
			}
			ps := storeForTicket(st.Env, p.Ticket)
			req := projectReleaseRequest{Version: 1, Parent: st.Env.Ticket, Phase: p.ID, Subject: p.Subject, Evidence: p.Evidence, Heads: heads, FailedBefore: p.Failed, Note: kv["note"]}
			if err := writeJSONAtomic(projectReleaseRequestPath(ps), req); err != nil {
				return err
			}
			note := kv["note"]
			if note == "" {
				note = "Release phase " + p.ID + " (" + p.Title + ") to production?"
			}
			conflict, err := approvalPublish(ps, "release", note, actorRole())
			if err != nil {
				return err
			}
			projectEvent(st, "release_requested", p.ID, "", p.Subject)
			printV2Envelope(map[string]interface{}{"phase": p.ID, "ticket": p.Ticket, "approval": "pending", "conflict": conflict, "request": req})
			return nil
		}
		var in struct {
			Verdict string               `json:"verdict"`
			Tag     string               `json:"tag"`
			Record  string               `json:"record"`
			Repos   []projectReleaseRepo `json:"repos"`
		}
		if err := readProjectJSON(kv["file"], &in); err != nil {
			return err
		}
		if !releaseVerdicts[in.Verdict] {
			return fmt.Errorf("release verdict must be RELEASED, ROLLED_BACK or PARTIAL")
		}
		if in.Verdict == "RELEASED" {
			for _, r := range p.Releases {
				if r.Verdict == "RELEASED" && r.Subject == p.Subject {
					printV2Envelope(r)
					return nil
				}
			}
		}
		if reasons := projectPhaseReasons(s, "release", p.ID); len(reasons) > 0 {
			return fmt.Errorf("phase not releasable: %v", reasons)
		}
		if !filepath.IsAbs(in.Record) {
			return fmt.Errorf("release needs the absolute path of its release record")
		}
		if _, err := os.Stat(in.Record); err != nil {
			return fmt.Errorf("release record: %w", err)
		}
		if in.Verdict != "ROLLED_BACK" {
			// A release ships every phase up to this one: each phase commit
			// must be in the released revision of its repository.
			for j := 0; j <= k; j++ {
				c := s.Phases[j].Candidate
				if c == nil {
					continue
				}
				for _, repo := range c.Repos {
					found := false
					for _, r := range in.Repos {
						if fullSHARe.MatchString(r.SHA) && projectSameRepo(r.Dir, repo.Canonical) {
							found = true
							if !gitOKIn(repo.Canonical, "merge-base", "--is-ancestor", repo.Commit, r.SHA) {
								return fmt.Errorf("released %s does not contain phase %s commit %s", r.SHA, c.Phase, short(repo.Commit))
							}
						}
					}
					if !found {
						return fmt.Errorf("release receipt names no released SHA for %s", repo.Canonical)
					}
				}
			}
		}
		r := projectRelease{Version: 1, ID: newInvocationID(), Phase: p.ID, Subject: p.Subject, At: time.Now().UTC().Format(time.RFC3339Nano), Verdict: in.Verdict, Tag: in.Tag, Record: in.Record, Repos: in.Repos}
		if r.Repos == nil {
			r.Repos = []projectReleaseRepo{}
		}
		if err := writeJSONAtomic(filepath.Join(st.Home(), "project-releases", r.ID+".json"), r); err != nil {
			return err
		}
		projectEvent(st, "release_recorded", r.ID, p.ID, r.Verdict)
		printV2Envelope(r)
		return nil
	})
}

// The release floor is the strictest release mode a human-or-Foreman approved
// contract ever gave each seed. A later contract may raise it freely, but one
// that lowers it (ask → auto/none) has dropped a human release gate, so only a
// human may approve it — `--auto` delegates design review, never releases.
var releaseRank = map[string]int{"none": 0, "auto": 1, "ask": 2}

func projectReleaseFloorPath(st *ticket.Store) string {
	return filepath.Join(st.Home(), "project-release-floor.json")
}

func projectSeedReleases(st *ticket.Store) map[string]string {
	var c projectContract
	if readProjectJSON(projectContractPath(st), &c) != nil {
		return nil
	}
	modes := map[string]string{}
	for _, seed := range c.Seeds {
		modes[seed] = "" // unphased: no release gate at all
	}
	for _, p := range c.Phases {
		for _, seed := range p.Seeds {
			modes[seed] = p.Release
		}
	}
	return modes
}

func projectRaiseReleaseFloor(st *ticket.Store) error {
	floor := map[string]string{}
	_ = readProjectJSON(projectReleaseFloorPath(st), &floor)
	for seed, mode := range projectSeedReleases(st) {
		if releaseRank[mode] > releaseRank[floor[seed]] {
			floor[seed] = mode
		}
	}
	if len(floor) == 0 {
		return nil
	}
	return writeJSONAtomic(projectReleaseFloorPath(st), floor)
}

func projectReleaseLowered(st *ticket.Store) []string {
	floor := map[string]string{}
	if readProjectJSON(projectReleaseFloorPath(st), &floor) != nil {
		return nil
	}
	modes := projectSeedReleases(st)
	lowered := []string{}
	for seed, was := range floor {
		// A seed dropped from scope is a scope change, not a dropped gate.
		if now, ok := modes[seed]; ok && releaseRank[now] < releaseRank[was] {
			if now == "" {
				now = "unphased"
			}
			lowered = append(lowered, seed+": "+was+" → "+now)
		}
	}
	slices.Sort(lowered)
	return lowered
}

// projectCheckPassedAny reads the union of the current passed records of one
// kind: fresh partial attempts and carried records add up.
func projectCheckPassedAny(evs []projectEvidence, criterion, kind string) bool {
	for _, ev := range evs {
		if projectCheckPassed(ev, criterion, kind) {
			return true
		}
	}
	return false
}

func projectWaiverStates(w *projectWaiver) string {
	var parts []string
	for _, g := range w.Gates {
		parts = append(parts, g+" "+w.States[g])
	}
	return strings.Join(parts, ", ")
}
