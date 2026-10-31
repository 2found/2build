package cmd

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/ticket"
)

// projectFixture is a project whose one child "ready" (seed core) holds
// verified, uncommitted work on its ticket branch in repo and is sealed. The
// contract has no phases yet; phasedFixture adds them.
func projectFixture(t *testing.T) (*ticket.Store, *ticket.Store, string) {
	t.Helper()
	repo, env := initReadinessFixture(t)
	t.Chdir(repo)
	state := t.TempDir()
	mustMkdirAll(t, filepath.Join(state, "projects"))
	projectHome := filepath.Join(state, "projects", "project")
	if err := os.Rename(env.ProjectHome, projectHome); err != nil {
		t.Fatal(err)
	}
	env.ProjectHome = projectHome
	t.Setenv("BABYSIT_HOME", state)
	t.Setenv("BABYSIT_PROJECT_HOME", env.ProjectHome)
	t.Setenv("BABYSIT_TICKET", "ready")
	runGit(t, repo, "switch", "-c", env.Branch)
	mustWrite(t, filepath.Join(repo, "feature.txt"), "working journey\n")
	child := ticket.New(env)
	if err := ticket.WriteManifest(child.ManifestPath(), env.Ticket, "App", []ticket.Repo{{Name: "app", Canonical: repo, Worktree: repo, Branch: env.Branch, Base: "main"}}); err != nil {
		t.Fatal(err)
	}
	doc := ticket.ReadDoc(child.IndexPath())
	doc.Set("origin.project_seed", "core")
	doc.Set("parent", "parent")
	doc.Set("status", "in_review")
	if err := ticket.WriteDoc(child.IndexPath(), doc); err != nil {
		t.Fatal(err)
	}
	for _, g := range []struct{ gate, id string }{{"review-pr", "review-1"}, {"qa", "qa-1"}} {
		if _, err := setV2VerificationEvidence(env, evidenceForCurrentSnapshot(t, env, g.gate, g.id, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if err := projectSealChild(child, nil, "", false); err != nil {
		t.Fatal(err)
	}
	parent := storeForTicket(env, "parent")
	if err := ticket.WriteDoc(parent.IndexPath(), ticket.Doc{"id": "parent", "status": "decomposed", "assignee": "fm-test", "children": []interface{}{"ready"}}); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(parent.Home(), "requirement.md"), "Create and recover tasks\n")
	mustWrite(t, filepath.Join(parent.Home(), "plan.md"), "A real persisted task journey\n")
	c := projectContract{Version: 1, Audience: "small teams", Outcome: "Create a task and recover it after reload", NonGoals: []string{"billing"}, Seeds: []string{"core"}, Criteria: []projectCriterion{{ID: "AC1", Text: "Task persists across reload", Owner: "core", Checks: []string{"behavior", "failure"}}}, FirstJourney: []string{"AC1"}, ProductReview: true}
	if err := writeJSONAtomic(projectContractPath(parent), c); err != nil {
		t.Fatal(err)
	}
	if err := foreman.Save(foreman.Record{ID: "fm-test", ProjectDir: repo, Status: "running"}); err != nil {
		t.Fatal(err)
	}
	return parent, child, repo
}

// executionFixture is a one-phase project (p1, release none) with its phase
// ticket bound, ready to integrate.
func executionFixture(t *testing.T) (*ticket.Store, *ticket.Store, string) {
	t.Helper()
	parent, child, repo := phasedFixture(t, []projectPhase{{ID: "p1", Title: "Tasks", Seeds: []string{"core"}, Release: "none"}})
	if err := foremanProjectCommand("bind", []string{"parent", "--phase", "p1", "--child", "ph-p1"}); err != nil {
		t.Fatal(err)
	}
	return parent, child, repo
}

// integratePhase builds the phase commit and returns its candidate.
func integratePhase(t *testing.T, st *ticket.Store, phase string) projectCandidate {
	t.Helper()
	var err error
	out := captureStdout(t, func() { err = projectIntegrateCommand(st, map[string]string{"phase": phase}) })
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data projectCandidate `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func landPhase(t *testing.T, st *ticket.Store, phase string) error {
	t.Helper()
	var err error
	captureStdout(t, func() { err = projectLandCommand(st, map[string]string{"phase": phase}) })
	return err
}

// integrateAndLand builds the phase commit and lands it: phase QA runs on
// the landed commit.
func integrateAndLand(t *testing.T, st *ticket.Store, phase string) projectCandidate {
	t.Helper()
	c := integratePhase(t, st, phase)
	if err := landPhase(t, st, phase); err != nil {
		t.Fatal(err)
	}
	return c
}

// phaseQAProblems is nil once phase QA passed, else the phase's problems.
func phaseQAProblems(t *testing.T, st *ticket.Store, phase string) []string {
	t.Helper()
	s, err := projectRead(st)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Phases[projectPhaseIndex(s, phase)]
	if p.Integrated {
		return nil
	}
	return p.Problems
}

func executionEvidence(t *testing.T, st *ticket.Store, kind string) string {
	t.Helper()
	return phaseEvidence(t, st, kind, "p1")
}

func finishExecutionEvidence(t *testing.T, st *ticket.Store, id, kind string, failed bool) {
	t.Helper()
	code := 0
	if failed {
		code = 1
	}
	log := filepath.Join(t.TempDir(), "check.log")
	mustWrite(t, log, "Executed real fixture assertion: task survives reload\n")
	checks := []projectCheck{{Criterion: "AC1", Kind: "behavior", Command: []string{"fixture", "reload"}, ExitCode: &code, Log: log}, {Criterion: "AC1", Kind: "failure", Command: []string{"fixture", "invalid-task"}, ExitCode: &code, Log: log}}
	if kind == "product" {
		checks = []projectCheck{{Criterion: "AC1", Kind: "product", Command: []string{"fixture", "journey"}, ExitCode: &code, Log: log}}
	}
	result := filepath.Join(t.TempDir(), "results.json")
	if err := writeJSONAtomic(result, map[string]interface{}{"checks": checks, "findings": []projectFinding{}}); err != nil {
		t.Fatal(err)
	}
	var err error
	captureStdout(t, func() { err = projectEvidenceCommand(st, map[string]string{"attempt": id, "file": result}) })
	if err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() { err = projectEvidenceCommand(st, map[string]string{"attempt": id, "file": result}) })
	if err != nil {
		t.Fatalf("record retry after lost response: %v", err)
	}
}

// verifiedExecution integrates, lands and verifies the one phase.
func verifiedExecution(t *testing.T) (*ticket.Store, *ticket.Store, string) {
	t.Helper()
	parent, child, repo := executionFixture(t)
	integrateAndLand(t, parent, "p1")
	for _, kind := range []string{"integration", "product"} {
		finishExecutionEvidence(t, parent, executionEvidence(t, parent, kind), kind, false)
	}
	s, err := projectRead(parent)
	if err != nil || !s.Ready {
		t.Fatalf("valid project blocked: %+v %v", s.Blockers, err)
	}
	return parent, child, repo
}

func TestProjectCompletionAndCleanedChildRecovery(t *testing.T) {
	st, child, repo := verifiedExecution(t)
	var err error
	captureStdout(t, func() { err = projectComplete(st, "fm-test") })
	if err != nil {
		t.Fatal(err)
	}
	// The manifest still identifies retained source refs even when its worker
	// checkout no longer exists. Durable logs must remain sufficient.
	m, _ := ticket.ReadManifest(child.ManifestPath())
	m.Repos[0].Worktree = filepath.Join(t.TempDir(), "removed")
	if err := ticket.WriteManifest(child.ManifestPath(), m.Ticket, m.Title, m.Repos); err != nil {
		t.Fatal(err)
	}
	s, err := projectRead(st)
	if err != nil || !s.Ready || s.Completion != "verified" {
		t.Fatalf("cleanup lost verified result: %+v %v", s, err)
	}
	r, _ := foreman.Load("fm-test")
	if err := foremanCompletionCurrent(r); err != nil {
		t.Fatal(err)
	}
	if err := foremanHeartbeat([]string{"fm-test", "--status", "done"}); err != nil {
		t.Fatal(err)
	}
	// Later work in the checkout doesn't erase a landed phase.
	mustWrite(t, filepath.Join(repo, "next.txt"), "next project\n")
	if s, _ := projectRead(st); s.Completion != "verified" {
		t.Fatal("later checkout use invalidated completion")
	}
	doc := ticket.ReadDoc(st.IndexPath())
	doc.Set("assignee", "other")
	if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(st.Home(), "plan.md"), "changed after reassignment")
	if err := foremanCompletionCurrent(r); err == nil {
		t.Fatal("reassignment hid a stale completion receipt")
	}
}

// stubOrcaCLI puts a fake `orca` on ORCA_CLI_COMMAND that answers preflight
// and serves $TERMINALS as `terminal list`, failing when it is unset.
func stubOrcaCLI(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "orca")
	stub := `#!/bin/sh
case "$1" in
 status) echo '{"ok":true,"result":{"runtime":{"reachable":true,"capabilities":[]}}}' ;;
 terminal)
  if [ -z "$TERMINALS" ]; then echo '{"ok":false,"error":{"message":"runtime unreachable"}}'; exit 1; fi
  printf '{"ok":true,"result":{"terminals":%s}}\n' "$TERMINALS" ;;
esac
`
	if err := os.WriteFile(path, []byte(stub), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCA_CLI_COMMAND", path)
}

// Completion needs every Dispatch of the project settled and every terminal
// a Dispatch launched closed; a terminal it only reused belongs to its
// creator, and an unverifiable Orca blocks rather than passes.
func TestProjectCompleteSettledWorkerTerminals(t *testing.T) {
	for _, tc := range []struct {
		name, status, terminals string
		owns, done              bool
	}{
		{name: "closed terminal", status: "completed", owns: true, terminals: `[]`, done: true},
		{name: "reused terminal", status: "failed", terminals: `[{"handle":"term_w","agentIdentity":"codex"}]`, done: true},
		{name: "open dispatch", status: "dispatched", owns: true, terminals: `[]`},
		{name: "settled worker left open", status: "completed", owns: true, terminals: `[{"handle":"term_w","agentIdentity":""}]`},
		{name: "unverifiable terminals", status: "completed", owns: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _, _ := executionFixture(t)
			stubOrcaCLI(t)
			t.Setenv("TERMINALS", tc.terminals)
			task := foreman.Task{ID: "task-w", Foreman: "fm-test", Ticket: p.Env.Ticket, CreatedAt: foreman.Now(),
				Dispatches: []foreman.Dispatch{{ID: "task-w.d1", Status: tc.status, Terminal: "term_w", OwnsTerminal: tc.owns, LaunchedAt: foreman.Now()}}}
			mustMkdirAll(t, filepath.Join(foreman.TasksDir(), task.ID))
			data, _ := json.Marshal(task)
			mustWrite(t, filepath.Join(foreman.TasksDir(), task.ID, "task.json"), string(data))
			integrateAndLand(t, p, "p1")
			for _, kind := range []string{"integration", "product"} {
				finishExecutionEvidence(t, p, executionEvidence(t, p, kind), kind, false)
			}
			var err error
			captureStdout(t, func() { err = projectComplete(p, "fm-test") })
			if tc.done {
				if err != nil {
					t.Fatalf("settled worker wedged completion: %v", err)
				}
				r, _ := foreman.Load("fm-test")
				if r.Status != "done" {
					t.Fatalf("foreman not marked done: %+v", r)
				}
				if err := foremanCompletionCurrent(r); err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("completion ignored an open dispatch or an unclosed worker terminal")
			}
		})
	}
}

// A leftover ticket directory without index.json (e.g. bs-cli holding only
// review.rounds) is not a ticket: both the allDone scan inside projectComplete
// and the receipt check in foremanCompletionCurrent must skip it rather than
// wedge on leftover state.
func TestCompletionSkipsStubTicketDirs(t *testing.T) {
	p, _, _ := verifiedExecution(t)
	stub := filepath.Join(p.Env.ProjectHome, "tickets", "bs-stub")
	mustMkdirAll(t, stub)
	mustWrite(t, filepath.Join(stub, "review.rounds"), "1\n")
	var err error
	captureStdout(t, func() { err = projectComplete(p, "fm-test") })
	if err != nil {
		t.Fatalf("stub ticket dir wedged completion: %v", err)
	}
	r, _ := foreman.Load("fm-test")
	if r.Status != "done" {
		t.Fatalf("foreman not marked done: %+v", r)
	}
	if err := foremanCompletionCurrent(r); err != nil {
		t.Fatalf("stub ticket dir wedged receipt check: %v", err)
	}
}

// A ticket directory whose index.json exists but cannot be parsed or read is
// a real record in an unknown state — not a stub. Both completion scans must
// fail loudly on it instead of skipping it like a missing index.
func TestCompletionFailsOnMalformedTicketRecords(t *testing.T) {
	for _, tc := range []struct {
		name       string
		breakIndex func(t *testing.T, dir string)
	}{
		{name: "malformed index", breakIndex: func(t *testing.T, dir string) {
			mustWrite(t, filepath.Join(dir, "index.json"), "{not json")
		}},
		{name: "unreadable index", breakIndex: func(t *testing.T, dir string) {
			// A directory named index.json exists but cannot be read as a file.
			mustMkdirAll(t, filepath.Join(dir, "index.json"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _, _ := verifiedExecution(t)
			broken := filepath.Join(p.Env.ProjectHome, "tickets", "bs-broken")
			mustMkdirAll(t, broken)
			tc.breakIndex(t, broken)
			var err error
			captureStdout(t, func() { err = projectComplete(p, "fm-test") })
			if err == nil {
				t.Fatal("completion skipped a malformed ticket record")
			}
		})
		t.Run(tc.name+" receipt check", func(t *testing.T) {
			p, _, _ := verifiedExecution(t)
			var err error
			captureStdout(t, func() { err = projectComplete(p, "fm-test") })
			if err != nil {
				t.Fatal(err)
			}
			r, _ := foreman.Load("fm-test")
			broken := filepath.Join(p.Env.ProjectHome, "tickets", "bs-broken")
			mustMkdirAll(t, broken)
			tc.breakIndex(t, broken)
			if err := foremanCompletionCurrent(r); err == nil {
				t.Fatal("receipt check skipped a malformed ticket record")
			}
		})
	}
}

func TestProjectTimeOrderUsesInstant(t *testing.T) {
	if !projectTimeAfter("2026-09-24T06:00:00.1Z", "2026-09-24T06:00:00Z") || projectTimeAfter("2026-09-24T06:00:00Z", "2026-09-24T06:00:00.1Z") {
		t.Fatal("fractional timestamps were compared lexicographically")
	}
}

func TestProjectRejectsFalseCompletion(t *testing.T) {
	cases := map[string]func(*testing.T, *ticket.Store, *ticket.Store, string){
		"scope removed": func(t *testing.T, p, c *ticket.Store, repo string) {
			d := ticket.ReadDoc(p.IndexPath())
			d["children"] = []interface{}{}
			ticket.WriteDoc(p.IndexPath(), d)
		},
		"cancelled child": func(t *testing.T, p, c *ticket.Store, repo string) {
			d := ticket.ReadDoc(c.IndexPath())
			d.Set("status", "cancelled")
			ticket.WriteDoc(c.IndexPath(), d)
		},
		"stale approval": func(t *testing.T, p, c *ticket.Store, repo string) {
			mustWrite(t, filepath.Join(p.Home(), "plan.md"), "changed scope")
		},
		"changed child evidence": func(t *testing.T, p, c *ticket.Store, repo string) {
			mustWrite(t, filepath.Join(c.Home(), "evidence", "check.log"), "changed log")
		},
		"phase commit rewritten off base": func(t *testing.T, p, c *ticket.Store, repo string) {
			runGit(t, repo, "branch", "-f", "main", "main~1")
		},
		"held lease": func(t *testing.T, p, c *ticket.Store, repo string) {
			leasePublish(leaseDirOf(gitOutIn(repo, "rev-parse", "--absolute-git-dir")), leaseBodyFor("parent", leaseLong, "10"))
		},
		"missing integration": func(t *testing.T, p, c *ticket.Store, repo string) {
			paths, _ := filepath.Glob(filepath.Join(p.Home(), "project-evidence", "*.json"))
			for _, path := range paths {
				var e projectEvidence
				readProjectJSON(path, &e)
				if e.Kind == "integration" {
					os.Remove(path)
				}
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			p, c, repo := verifiedExecution(t)
			change(t, p, c, repo)
			s, err := projectRead(p)
			if err == nil && s.Ready {
				t.Fatalf("false completion: %+v", s)
			}
			if name == "stale approval" && (len(s.Coverage) == 0 || s.Coverage[0].State == "proven") {
				t.Fatal("stale approval retained apparently current coverage")
			}
			if err := projectComplete(p, "fm-test"); err == nil {
				t.Fatal("complete accepted stale/missing evidence")
			}
		})
	}
}

func TestProjectEvidenceCapturesInputsBeforeCheck(t *testing.T) {
	p, _, _ := executionFixture(t)
	cand := integrateAndLand(t, p, "p1")
	id := executionEvidence(t, p, "integration")
	mustWrite(t, filepath.Join(cand.Repos[0].Worktree, "feature.txt"), "changed after begin")
	result := filepath.Join(t.TempDir(), "results.json")
	mustWrite(t, result, `{"checks":[]}`)
	if err := projectEvidenceCommand(p, map[string]string{"attempt": id, "file": result}); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("late results accepted: %v", err)
	}
}

func TestProjectFailedCheckAndProductFindings(t *testing.T) {
	p, _, _ := executionFixture(t)
	cand := integrateAndLand(t, p, "p1")
	finishExecutionEvidence(t, p, executionEvidence(t, p, "integration"), "integration", true)
	finishExecutionEvidence(t, p, executionEvidence(t, p, "product"), "product", false)
	if phaseQAProblems(t, p, "p1") == nil {
		t.Fatal("phase QA passed with a failed check")
	}
	if _, err := os.Stat(cand.Repos[0].Worktree); err != nil {
		t.Fatal("failed phase QA removed the phase worktree its fixes need")
	}
	s, err := projectRead(p)
	if err != nil || s.Ready || s.Coverage[0].State == "proven" {
		t.Fatalf("failed check passed: %+v %v", s, err)
	}
}

func TestProjectHeartbeatCannotBypassCompletion(t *testing.T) {
	t.Setenv("BABYSIT_HOME", t.TempDir())
	if err := foreman.Save(foreman.Record{ID: "fm-test", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := foremanHeartbeat([]string{"fm-test", "--status", "DONE"}); err == nil {
		t.Fatal("raw done heartbeat bypassed validation")
	}
	r, _ := foreman.Load("fm-test")
	if r.Status != "running" {
		t.Fatal("rejected heartbeat mutated status")
	}
}

func TestProjectScopeContractInvalidatesApproval(t *testing.T) {
	p, _, _ := executionFixture(t)
	var c projectContract
	if err := readProjectJSON(projectContractPath(p), &c); err != nil {
		t.Fatal(err)
	}
	c.Criteria[0].Text = "New accepted behavior"
	if err := writeJSONAtomic(projectContractPath(p), c); err != nil {
		t.Fatal(err)
	}
	s, err := projectRead(p)
	if err != nil || s.Approval != "stale" || len(s.Dispatch) == 0 {
		t.Fatalf("changed scope kept approval: %+v %v", s, err)
	}
	if err := statusSet(p, "done", "developer"); err == nil {
		t.Fatal("status command bypassed project completion")
	}
}

func TestProjectEvidenceOnlyDelivery(t *testing.T) {
	p, child, repo := executionFixture(t)
	// Evidence-only work has no production code and stores its outputs on
	// the durable ticket. Re-approve the explicitly different contract.
	runGit(t, repo, "switch", "main")
	os.Remove(filepath.Join(repo, "feature.txt"))
	var c projectContract
	readProjectJSON(projectContractPath(p), &c)
	c.ArtifactsOnly = true
	c.ProductReview = false
	writeJSONAtomic(projectContractPath(p), c)
	if _, err := approvalPublish(p, "project-plan", "artifact scope", "developer"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := approvalResolve(p, "approve", "accepted", "developer"); err != nil {
		t.Fatal(err)
	}
	d := ticket.ReadDoc(child.IndexPath())
	d.Set("pointers.workflow", "prototype")
	ticket.WriteDoc(child.IndexPath(), d)
	mustWrite(t, child.VerdictPath("prototype"), "STATUS: DONE\nLearning validated with named observations\n")
	artifact := filepath.Join(child.Home(), "experiment.md")
	mustWrite(t, artifact, "Measured assumption and bounded production recommendation")
	input := filepath.Join(t.TempDir(), "artifacts.json")
	writeJSONAtomic(input, map[string]interface{}{"paths": []string{artifact}})
	if err := projectSealArtifacts(child, input); err != nil {
		t.Fatal(err)
	}
	var err error
	captureStdout(t, func() { err = projectIntegrateCommand(p, map[string]string{"phase": "p1"}) })
	if err == nil {
		t.Fatal("artifact-only phase built a phase commit")
	}
	finishExecutionEvidence(t, p, executionEvidence(t, p, "integration"), "integration", false)
	s, err := projectRead(p)
	if err != nil || !s.Ready || s.Delivery[0].State != "ARTIFACTS_READY" {
		t.Fatalf("artifacts rejected: %+v %v", s, err)
	}
	mustWrite(t, artifact, "changed conclusion")
	s, _ = projectRead(p)
	if s.Ready {
		t.Fatal("changed artifact remained accepted")
	}
}

func TestProjectWatchIgnoresSpinnerAndPreservesBoundedWait(t *testing.T) {
	p, _, repo := executionFixture(t)
	originalPath := os.Getenv("PATH")
	client, r, pane, _ := watchFixture(t)
	t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+originalPath)
	r.ProjectDir = repo
	now := time.Now().UTC().Truncate(time.Second)
	watchTick(client, r, testWatchOpts(), now)
	write(t, pane, "spinner keeps moving")
	if line := watchTick(client, r, testWatchOpts(), now.Add(11*time.Minute)); !strings.HasPrefix(line, "NUDGED") {
		t.Fatalf("spinner counted as progress: %s", line)
	}
	report := projectProgress{Version: 1, Ticket: p.Env.Ticket, Producer: r.ID, Phase: "qa", Summary: "Waiting for check", Updated: now.Format(time.RFC3339), WaitKind: "test", WaitReason: "browser check still running", WaitUntil: now.Add(13 * time.Minute).Format(time.RFC3339)}
	writeJSONAtomic(filepath.Join(p.Home(), "progress.json"), report)
	if line := watchTick(client, r, testWatchOpts(), now.Add(12*time.Minute)); !strings.HasPrefix(line, "WAITING") {
		t.Fatalf("valid bounded wait nudged: %s", line)
	}
	if s := watchLoad(r.ID); s.Nudges != 1 || s.RuntimeObservedAt == "" {
		t.Fatalf("wait changed retry budget or lost reachability: %+v", s)
	}
}

func TestProjectDashboardUsesSharedRead(t *testing.T) {
	p, _, _ := verifiedExecution(t)
	srv := &dashServer{stateDir: filepath.Dir(filepath.Dir(p.Env.ProjectHome))}
	// Server expects <state>/projects/<slug>; the test fixture can name its
	// state directory directly without changing process cwd in the handler.
	slug := filepath.Base(p.Env.ProjectHome)
	r := httptest.NewRequest("GET", "/api/tickets/"+slug+"/parent/project", nil)
	r.SetPathValue("project", slug)
	r.SetPathValue("ticket", "parent")
	w := httptest.NewRecorder()
	srv.handleProject(w, r)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var got projectSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want, err := projectRead(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != want.Subject || got.Ready != want.Ready || strings.Join(got.Blockers, "|") != strings.Join(want.Blockers, "|") {
		t.Fatalf("API diverged: %+v / %+v", got, want)
	}
}

func TestProjectProgressWaitsAndEvidence(t *testing.T) {
	p, _, _ := executionFixture(t)
	file := filepath.Join(t.TempDir(), "progress.json")
	evidence := filepath.Join(t.TempDir(), "milestone.md")
	mustWrite(t, evidence, "Verified task survives reload")
	input := projectProgress{Producer: "fm-test", Phase: "integration", Summary: "First journey demonstrated", Evidence: evidence}
	writeJSONAtomic(file, input)
	if err := projectProgressCommand(p, map[string]string{"file": file}); err != nil {
		t.Fatal(err)
	}
	var first projectProgress
	readProjectJSON(filepath.Join(p.Home(), "progress.json"), &first)
	if first.Health != "progressing" {
		t.Fatal(first)
	}
	input.WaitKind = "test"
	input.WaitReason = "Browser fixture running"
	input.WaitUntil = time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339)
	writeJSONAtomic(file, input)
	if err := projectProgressCommand(p, map[string]string{"file": file}); err != nil {
		t.Fatal(err)
	}
	var next projectProgress
	readProjectJSON(filepath.Join(p.Home(), "progress.json"), &next)
	if next.Health != "reported_wait" || next.Advanced != first.Advanced {
		t.Fatal("activity invented progress")
	}
	projectProgressHealth(&next, time.Now().Add(6*time.Minute))
	if next.Health != "wait_expired" {
		t.Fatal(next)
	}
}

// Seal and readiness are one evaluator: with qa missing, readiness is not
// ready, childSealable says no and seal refuses; a v1 checkpoint is refused by
// both with the migration command.
func TestSealAndReadinessAgreeOnMissingGate(t *testing.T) {
	repo, env := initReadinessFixture(t)
	t.Chdir(repo)
	t.Setenv("BABYSIT_PROJECT_HOME", env.ProjectHome)
	t.Setenv("BABYSIT_TICKET", env.Ticket)
	runGit(t, repo, "switch", "-c", env.Branch)
	child := ticket.New(env)
	if err := ticket.WriteManifest(child.ManifestPath(), env.Ticket, "App", []ticket.Repo{{Name: "app", Canonical: repo, Worktree: repo, Branch: env.Branch, Base: "main"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := setV2VerificationEvidence(env, evidenceForCurrentSnapshot(t, env, "review-pr", "review-1", 0)); err != nil {
		t.Fatal(err)
	}
	ready, err := evaluateReadinessIn(env, "review", repo)
	if err != nil || ready.Ready || !containsString(ready.ReasonCodes, "qa:missing") {
		t.Fatalf("readiness with qa missing = %+v, %v", ready, err)
	}
	if childSealable(child) {
		t.Fatal("childSealable with qa missing")
	}
	if err := projectSealChild(child, nil, "", false); err == nil || !strings.Contains(err.Error(), "qa") {
		t.Fatalf("seal with qa missing = %v, want refusal naming qa", err)
	}

	mustWrite(t, filepath.Join(child.Home(), "checkpoint.json"), `{"ticket":"ready","workflow":"builder"}`)
	if childSealable(child) {
		t.Fatal("childSealable on a v1 checkpoint")
	}
	if err := projectSealChild(child, nil, "", false); err == nil || !strings.Contains(err.Error(), "checkpoint --refresh") {
		t.Fatalf("seal on a v1 checkpoint = %v, want the refresh command", err)
	}
}

// A landed child's snapshot is the record: a shared input its own phase
// rewrote (the workflow file, here its plan) must not reopen it, while the
// same edit before land does.
func TestLandedChildSealIgnoresLaterInputs(t *testing.T) {
	parent, child, _ := executionFixture(t)
	mustWrite(t, filepath.Join(child.Home(), "plan.md"), "edited before land\n")
	if s, _ := projectRead(parent); !slices.Contains(s.Blockers, "child ready: evidence: changed since seal") {
		t.Fatalf("pre-land edit not caught: %+v", s.Blockers)
	}

	parent, child, _ = verifiedExecution(t)
	m, _ := ticket.ReadManifest(child.ManifestPath())
	m.Repos[0].Worktree = filepath.Join(t.TempDir(), "removed")
	if err := ticket.WriteManifest(child.ManifestPath(), m.Ticket, m.Title, m.Repos); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(child.Home(), "plan.md"), "edited after land\n")
	if s, err := projectRead(parent); err != nil || !s.Ready {
		t.Fatalf("landed child reopened: %+v %v", s.Blockers, err)
	}
}
