package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/orca"
	"github.com/2found/2build/internal/ticket"
)

type projectDelivery struct {
	Phase  string `json:"phase"`
	Repo   string `json:"repo"`
	Base   string `json:"base"`
	Commit string `json:"commit"`
	State  string `json:"state"` // LANDED | PENDING
}

type projectCompletion struct {
	Version  int               `json:"version"`
	Subject  string            `json:"subject"`
	At       string            `json:"at"`
	Foreman  string            `json:"foreman"`
	Cleanup  bool              `json:"cleanup_verified"`
	Delivery []projectDelivery `json:"delivery"`
}

func projectEvidenceProblems(st *ticket.Store, s projectSnapshot, ev projectEvidence, checkedOut bool) []string {
	problems := []string{}
	if ev.Version != 1 || ev.Subject != projectEvidenceSubject(s, ev) || ev.Producer == "" || (ev.Kind != "integration" && ev.Kind != "product" && ev.Kind != "journey") {
		return []string{"invalid or stale subject"}
	}
	children := projectPhaseChildren(s, ev.Phase)
	if _, err := parseProjectTime(ev.Started); err != nil {
		problems = append(problems, "missing start time")
	}
	if ev.Finished != "" {
		start, _ := parseProjectTime(ev.Started)
		end, err := parseProjectTime(ev.Finished)
		if err != nil || end.Before(start) {
			problems = append(problems, "invalid completion time")
		}
	}
	if ev.Kind == "product" {
		for _, child := range children {
			var seal projectSeal
			if readProjectJSON(filepath.Join(storeForTicket(st.Env, child.ID).Home(), "project-seal.json"), &seal) == nil && slices.Contains(seal.Owners, ev.Producer) {
				problems = append(problems, "product evaluator also produced child evidence: "+child.ID)
			}
		}
	}
	k := projectPhaseIndex(s, ev.Phase)
	if k < 0 {
		return append(problems, "unknown phase")
	}
	cand := s.Phases[k].Candidate
	if cand == nil && (s.Contract == nil || !s.Contract.ArtifactsOnly) {
		return append(problems, "phase commit missing or stale: run bbs foreman integrate")
	}
	if cand != nil {
		// The tested surface is the phase commit itself, in the phase's own
		// worktree; nothing else counts as phase QA.
		if len(ev.Surfaces) != len(cand.Repos) {
			problems = append(problems, "surfaces do not match the phase commit")
		}
		for _, r := range cand.Repos {
			found := false
			for _, surface := range ev.Surfaces {
				if surface.Dir == r.Worktree && surface.Head == r.Commit && surface.BaseHead == r.BaseHead {
					found = true
				}
			}
			if !found {
				problems = append(problems, "surface is not phase commit "+short(r.Commit)+" in "+r.Worktree)
				continue
			}
			if checkedOut {
				head, err := gitOutputStrict(r.Worktree, "rev-parse", "HEAD")
				status, statusErr := gitOutputStrict(r.Worktree, "status", "--porcelain", "--untracked-files=all")
				if err != nil || statusErr != nil || head != r.Commit || status != "" {
					problems = append(problems, "tested worktree not clean at the phase commit: "+r.Worktree)
				}
			}
			// Phase QA is cumulative: the commit must sit on every earlier
			// phase's commit for the same repository.
			for j := 0; j < k; j++ {
				if prev := s.Phases[j].Candidate; prev != nil {
					for _, pr := range prev.Repos {
						if pr.Canonical == r.Canonical && !gitOKIn(r.Canonical, "merge-base", "--is-ancestor", pr.Commit, r.Commit) {
							problems = append(problems, "phase commit omits phase "+prev.Phase)
						}
					}
				}
			}
		}
	}
	for _, check := range ev.Checks {
		if check.ExitCode == nil || *check.ExitCode != 0 || len(check.Command) == 0 {
			problems = append(problems, "failed or incomplete check: "+check.Criterion+"/"+check.Kind)
		}
		if d, ok := digestFile(check.Log); !ok || d != check.Digest {
			problems = append(problems, "missing or changed check evidence: "+check.Criterion+"/"+check.Kind)
		}
	}
	for _, finding := range ev.Findings {
		if finding.Severity != "minor" && finding.Severity != "nit" {
			problems = append(problems, "product finding: "+finding.Summary)
		}
	}
	if ev.Preview != "" {
		u, err := url.Parse(ev.Preview)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			problems = append(problems, "invalid preview URL")
		}
		// Runtime is an executable revision probe, persisted just like a check.
		matched := false
		for _, check := range ev.Checks {
			if check.Kind != "runtime" || check.ExitCode == nil || *check.ExitCode != 0 {
				continue
			}
			b, err := os.ReadFile(check.Log)
			if err == nil && strings.TrimSpace(string(b)) == ev.Runtime {
				matched = true
			}
		}
		head := false
		for _, surface := range ev.Surfaces {
			if surface.Head == ev.Runtime {
				head = true
			}
		}
		if !matched || !head {
			problems = append(problems, "preview needs a passing runtime probe containing the tested HEAD")
		}
	}
	return problems
}

func projectEvidenceCommand(st *ticket.Store, kv map[string]string) error {
	return withLock(st, func() error {
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		if s.Contract == nil || (kv["begin"] == "1" && len(s.Dispatch) > 0) {
			return fmt.Errorf("project not ready: %v", s.Dispatch)
		}
		if kv["carry"] == "1" {
			return projectEvidenceCarry(st, s, kv)
		}
		if kv["begin"] == "1" {
			var spec struct {
				Kind     string `json:"kind"`
				Producer string `json:"producer"`
				Phase    string `json:"phase"`
			}
			if err := readProjectJSON(kv["file"], &spec); err != nil {
				return err
			}
			if spec.Phase == "" && kv["phase"] != "" {
				spec.Phase = kv["phase"]
			}
			ev := projectEvidence{Version: 1, ID: newInvocationID(), Kind: spec.Kind, Producer: spec.Producer, Phase: spec.Phase, Started: time.Now().UTC().Format(time.RFC3339Nano), Surfaces: []projectSurface{}, Checks: []projectCheck{}, Findings: []projectFinding{}}
			ev.Subject = projectEvidenceSubject(s, ev)
			k := projectPhaseIndex(s, ev.Phase)
			if k < 0 {
				return fmt.Errorf("evidence needs the phase it verifies")
			}
			if ev.Kind != "journey" {
				if reasons := projectPhaseReasons(s, "verify", ev.Phase); len(reasons) > 0 {
					return fmt.Errorf("phase not ready to verify: %v", reasons)
				}
			}
			if c := s.Phases[k].Candidate; c != nil {
				for _, r := range c.Repos {
					ev.Surfaces = append(ev.Surfaces, projectSurface{Dir: r.Worktree, Ref: phaseCandidateRef(st.Env.Ticket, ev.Phase), Head: r.Commit, BaseRef: r.Base, BaseHead: r.BaseHead})
				}
			}
			if p := projectEvidenceProblems(st, s, ev, true); len(p) > 0 {
				return fmt.Errorf("cannot begin verification: %v", p)
			}
			if err := writeJSONAtomic(filepath.Join(st.Home(), "project-attempts", ev.ID+".json"), ev); err != nil {
				return err
			}
			projectEvent(st, "verification_started", ev.ID, ev.Producer, ev.Kind)
			printV2Envelope(ev)
			return nil
		}
		id := kv["attempt"]
		if !idRe.MatchString(id) {
			return fmt.Errorf("evidence needs --begin --file spec.json or --attempt ID --file results.json")
		}
		var ev projectEvidence
		if err := readProjectJSON(filepath.Join(st.Home(), "project-attempts", id+".json"), &ev); err != nil {
			return err
		}
		var results struct {
			Checks   []projectCheck   `json:"checks"`
			Findings []projectFinding `json:"findings"`
			Preview  string           `json:"preview"`
			Runtime  string           `json:"runtime"`
		}
		if err := readProjectJSON(kv["file"], &results); err != nil {
			return err
		}
		// Compare inputs before accepting results; late recollection cannot turn
		// an old test into proof for the current tree.
		if p := projectEvidenceProblems(st, s, ev, true); len(p) > 0 {
			return fmt.Errorf("verification inputs changed: %v", p)
		}
		ev.Checks, ev.Findings, ev.Preview, ev.Runtime = results.Checks, results.Findings, results.Preview, results.Runtime
		if len(ev.Checks) == 0 {
			return fmt.Errorf("verification needs executed checks")
		}
		for i := range ev.Checks {
			check := &ev.Checks[i]
			if check.ExitCode == nil || len(check.Command) == 0 || strings.TrimSpace(check.Command[0]) == "" {
				return fmt.Errorf("check needs command and exit_code")
			}
			known := false
			for _, ac := range s.Contract.Criteria {
				if ac.ID == check.Criterion && (slices.Contains(ac.Checks, check.Kind) || check.Kind == "product" || check.Kind == "runtime") {
					known = true
				}
			}
			if !known {
				return fmt.Errorf("unknown acceptance criterion or check: %s/%s", check.Criterion, check.Kind)
			}
			// Archive logs in the durable ticket, independent of worktree cleanup.
			b, err := os.ReadFile(check.Log)
			if err != nil {
				return err
			}
			if len(b) == 0 {
				return fmt.Errorf("empty check evidence")
			}
			check.Log = filepath.Join(st.Home(), "project-evidence", ev.ID, fmt.Sprintf("check-%d.log", i))
			if err := writeProjectImmutable(check.Log, b); err != nil {
				return err
			}
			check.Digest, _ = digestFile(check.Log)
		}
		path := filepath.Join(st.Home(), "project-evidence", ev.ID+".json")
		var prior projectEvidence
		if err := readProjectJSON(path, &prior); err == nil {
			// A lost response must be retryable without changing the receipt's
			// completion time. Immutable comparison still rejects new results.
			ev.Finished = prior.Finished
		} else if !os.IsNotExist(err) {
			return err
		} else {
			ev.Finished = time.Now().UTC().Format(time.RFC3339Nano)
		}
		body, _ := json.MarshalIndent(ev, "", "  ")
		if err := writeProjectImmutable(path, body); err != nil {
			return err
		}
		projectEvent(st, "verification_recorded", ev.ID, ev.Producer, ev.Kind)
		printV2Envelope(map[string]interface{}{"evidence": ev, "problems": projectEvidenceProblems(st, s, ev, true)})
		return nil
	})
}

// projectEvidenceCarry records current-subject evidence made of checks that
// already passed in earlier attempts of the same phase, so a small repair
// after phase QA re-tests only what it touched. Failed checks are never
// carried; one record is written per evidence kind found in the sources.
func projectEvidenceCarry(st *ticket.Store, s projectSnapshot, kv map[string]string) error {
	phase, reason := kv["phase"], strings.TrimSpace(kv["reason"])
	if projectPhaseIndex(s, phase) < 0 || kv["from"] == "" || reason == "" {
		return fmt.Errorf("evidence --carry needs --phase <id> --from <attempt-id>[,<attempt-id>] --reason \"<text>\" [--except <AC-id,...>]")
	}
	if reasons := projectPhaseReasons(s, "verify", phase); len(reasons) > 0 {
		return fmt.Errorf("phase not ready to verify: %v", reasons)
	}
	var except []string
	for _, id := range strings.Split(kv["except"], ",") {
		if id = strings.TrimSpace(id); id != "" {
			except = append(except, id)
		}
	}
	k := projectPhaseIndex(s, phase)
	byKind := map[string]*projectEvidence{}
	var kinds, skipped []string
	for _, id := range strings.Split(kv["from"], ",") {
		id = strings.TrimSpace(id)
		if !idRe.MatchString(id) {
			return fmt.Errorf("invalid source attempt id %q", id)
		}
		var src projectEvidence
		if err := readProjectJSON(filepath.Join(st.Home(), "project-evidence", id+".json"), &src); err != nil {
			return fmt.Errorf("source attempt %s has no recorded results: %w", id, err)
		}
		if src.Phase != phase {
			return fmt.Errorf("source attempt %s belongs to phase %q, not %q", id, src.Phase, phase)
		}
		if src.Kind != "integration" && src.Kind != "product" {
			return fmt.Errorf("source attempt %s is %s evidence; only integration and product checks carry", id, src.Kind)
		}
		ev := byKind[src.Kind]
		if ev == nil {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			ev = &projectEvidence{Version: 1, ID: newInvocationID(), Kind: src.Kind, Producer: src.Producer, Phase: phase, Started: now, Finished: now,
				Surfaces: []projectSurface{}, Checks: []projectCheck{}, Findings: []projectFinding{}, Carried: &projectCarry{Reason: reason, Except: except}}
			ev.Subject = projectEvidenceSubject(s, *ev)
			if c := s.Phases[k].Candidate; c != nil {
				for _, r := range c.Repos {
					ev.Surfaces = append(ev.Surfaces, projectSurface{Dir: r.Worktree, Ref: phaseCandidateRef(st.Env.Ticket, phase), Head: r.Commit, BaseRef: r.Base, BaseHead: r.BaseHead})
				}
			}
			byKind[src.Kind] = ev
			kinds = append(kinds, src.Kind)
		}
		ev.Carried.From = append(ev.Carried.From, src.ID)
		ev.Carried.FromSubject = append(ev.Carried.FromSubject, src.Subject)
		for _, check := range src.Checks {
			label := check.Criterion + "/" + check.Kind
			switch {
			case slices.Contains(except, check.Criterion):
				skipped = append(skipped, label+" (excepted)")
			case !projectCheckPassed(src, check.Criterion, check.Kind) || len(check.Command) == 0:
				skipped = append(skipped, label+" (not passed in "+src.ID+")")
			case slices.ContainsFunc(ev.Checks, func(c projectCheck) bool { return c.Criterion == check.Criterion && c.Kind == check.Kind }):
			default:
				check.CarriedFrom = src.ID
				ev.Checks = append(ev.Checks, check)
			}
		}
	}
	var out []projectEvidence
	for _, kind := range kinds {
		ev := byKind[kind]
		if len(ev.Checks) == 0 {
			return fmt.Errorf("nothing to carry for %s evidence: no passed check outside --except", kind)
		}
		if p := projectEvidenceProblems(st, s, *ev, false); len(p) > 0 {
			return fmt.Errorf("cannot carry: %v", p)
		}
		out = append(out, *ev)
	}
	for _, ev := range out {
		body, _ := json.MarshalIndent(ev, "", "  ")
		if err := writeProjectImmutable(filepath.Join(st.Home(), "project-evidence", ev.ID+".json"), body); err != nil {
			return err
		}
		projectEvent(st, "verification_carried", ev.ID, ev.Producer, ev.Kind)
	}
	printV2Envelope(map[string]interface{}{"carried": out, "not_carried": skipped})
	return nil
}

func writeProjectImmutable(path string, body []byte) error {
	if prior, err := os.ReadFile(path); err == nil {
		if bytes.Equal(prior, body) {
			return nil
		}
		return fmt.Errorf("immutable evidence already differs: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Publish only a fully written file. A crash must not leave a partial
	// immutable receipt that prevents all subsequent recovery attempts.
	f, err := os.CreateTemp(filepath.Dir(path), ".evidence-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(body)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Link(f.Name(), path); err != nil {
		if prior, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(prior, body) {
			return nil
		}
		return err
	}
	return nil
}

func projectCompletionSubject(s projectSnapshot) string {
	var receipts []projectEvidence
	for _, ev := range s.Evidence {
		if ev.Subject == projectEvidenceSubject(s, ev) && (ev.Kind == "integration" || ev.Kind == "product") {
			ev.State = ""
			ev.Problems = nil
			receipts = append(receipts, ev)
		}
	}
	body := map[string]interface{}{"subject": s.Subject, "evidence": receipts, "delivery": s.Delivery}
	if len(s.Phases) > 0 {
		var releases []projectRelease
		for _, p := range s.Phases {
			releases = append(releases, p.Releases...)
		}
		body["releases"] = releases
	}
	return digestJSON(body)
}

func projectCleanupProblems(st *ticket.Store, s projectSnapshot) []string {
	problems := []string{}
	doc := ticket.ReadDoc(st.IndexPath())
	if run := doc.Get("pointers.orca_run"); run != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err := orca.PreflightContext(ctx)
		if err != nil {
			problems = append(problems, "cleanup: Orca runtime unavailable")
		} else {
			workers, err := client.ExitedWorkers(run)
			if err != nil {
				problems = append(problems, "cleanup: worker state unknown")
			} else {
				for _, id := range sortedBoolKeys(workers) {
					if !workers[id] {
						problems = append(problems, "cleanup: worker "+id+" remains live or unverified")
					}
				}
			}
		}
		cancel()
	}
	var resources struct {
		Version        int                     `json:"version"`
		Leases         []foreman.ResourceLease `json:"leases"`
		ReconcileAfter string                  `json:"reconcile_after"`
	}
	err := readProjectJSON(filepath.Join(identity.BabysitHome(), "resources", "foreman-leases.json"), &resources)
	if (err != nil && !os.IsNotExist(err)) || (err == nil && resources.Version != 1) {
		problems = append(problems, "cleanup: resource state unreadable")
	}
	for _, lease := range resources.Leases {
		belongs := lease.Ticket == st.Env.Ticket
		for _, child := range s.Children {
			if child.ID == lease.Ticket {
				belongs = true
			}
		}
		if belongs {
			problems = append(problems, "cleanup: resource lease still held: "+lease.ID)
		}
	}
	for _, p := range s.Phases {
		if p.Candidate == nil {
			continue
		}
		for _, r := range p.Candidate.Repos {
			if _, err := os.Stat(r.Worktree); err == nil {
				problems = append(problems, "cleanup: phase worktree remains: "+r.Worktree)
			}
			if gitdir := gitOutIn(r.Canonical, "rev-parse", "--absolute-git-dir"); gitdir != "" {
				if _, err := os.Stat(leaseDirOf(gitdir)); err == nil {
					problems = append(problems, "cleanup: surface lease still held in "+r.Canonical)
				}
			}
		}
	}
	return problems
}

func sortedBoolKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func projectComplete(st *ticket.Store, id string) error {
	if err := foreman.ValidID(id); err != nil {
		return fmt.Errorf("complete needs --foreman ID")
	}
	r, err := foreman.Load(id)
	if err != nil {
		return err
	}
	return withLock(st, func() error {
		if doc := ticket.ReadDoc(st.IndexPath()); doc.Get("assignee") != id {
			return fmt.Errorf("parent is not claimed by this foreman")
		}
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		if !s.Ready {
			return fmt.Errorf("project not ready: %v", s.Blockers)
		}
		check, err := projectRead(st)
		if err != nil {
			return err
		}
		if !check.Ready || projectCompletionSubject(check) != projectCompletionSubject(s) {
			return fmt.Errorf("project changed during completion; re-read readiness")
		}
		receipt := projectCompletion{Version: 1, Subject: projectCompletionSubject(s), At: isoNow(), Foreman: id, Cleanup: true, Delivery: s.Delivery}
		if err := writeJSONAtomic(filepath.Join(st.Home(), "completion.json"), receipt); err != nil {
			return err
		}
		doc, err := ticket.ReadDocStrict(st.IndexPath())
		if err != nil {
			return err
		}
		doc.Set("status", "done")
		if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
			return err
		}
		// Other assigned projects must complete independently. A completed
		// project never closes a coordinator still responsible for more work.
		allDone := true
		ids, err := ticket.TicketIDs(st.Env.ProjectHome)
		if err != nil {
			return err
		}
		for _, ticketID := range ids {
			other := storeForTicket(st.Env, ticketID)
			d, e := ticket.ReadDocStrict(other.IndexPath())
			if e != nil {
				// A stub directory (no index.json) is not a ticket — skip it
				// rather than wedging completion on leftover state.
				var re *ticket.ReadError
				if errors.As(e, &re) && re.Kind == ticket.KindMissing {
					continue
				}
				return e
			}
			if d.Get("assignee") != id || ticketID == st.Env.Ticket || d.Get("parent") != "" {
				continue
			}
			otherState, e := projectRead(other)
			if e != nil || otherState.Completion != "verified" {
				allDone = false
			}
		}
		if allDone {
			r, err = foreman.Load(id)
			if err != nil {
				return err
			}
			r.Status = "done"
			r.Heartbeat = foreman.Now()
			r.Completion = filepath.Join(st.Home(), "completion.json")
			if err := foreman.Save(r); err != nil {
				return err
			}
		}
		projectEvent(st, "project_completed", "", id, s.Subject)
		printV2Envelope(receipt)
		return nil
	})
}

func foremanCompletionCurrent(r foreman.Record) error {
	if r.Completion == "" {
		return fmt.Errorf("validated project completion receipt required")
	}
	home := filepath.Dir(r.Completion)
	env := identity.Env{ProjectHome: filepath.Dir(filepath.Dir(home)), Ticket: filepath.Base(home)}
	current, err := projectRead(ticket.New(env))
	if err != nil {
		return err
	}
	if current.Completion != "verified" {
		return fmt.Errorf("completion receipt is no longer current: %v", current.Blockers)
	}
	ids, err := ticket.TicketIDs(env.ProjectHome)
	if err != nil {
		return err
	}
	for _, id := range ids {
		st := storeForTicket(env, id)
		doc, err := ticket.ReadDocStrict(st.IndexPath())
		if err != nil {
			var re *ticket.ReadError
			if errors.As(err, &re) && re.Kind == ticket.KindMissing {
				continue
			}
			return err
		}
		if doc.Get("assignee") != r.ID || doc.Get("parent") != "" {
			continue
		}
		s, err := projectRead(st)
		if err != nil {
			return err
		}
		if s.Completion != "verified" {
			return fmt.Errorf("project %s has no current completion receipt: %v", id, s.Blockers)
		}
	}
	var receipt projectCompletion
	if err := readProjectJSON(r.Completion, &receipt); err != nil {
		return err
	}
	if receipt.Foreman != r.ID || !receipt.Cleanup {
		return fmt.Errorf("completion owner/cleanup mismatch")
	}
	return nil
}
