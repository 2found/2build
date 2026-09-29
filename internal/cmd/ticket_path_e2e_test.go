package cmd

// ticket_path_e2e_test.go ports the retired bbs-ticket-test script: the path/list broker
// smoke suite that exercised the compiled `bbs` as a subprocess against a
// tracer ticket under a scratch BABYSIT_HOME. It covers what the differential
// harness does not: exit-code contracts, selector validation, telemetry
// gating, the lint fixture, and the six lifecycle scenarios.
//
// The binary is built once into the test temp dir; each case execs it so
// os.Exit-heavy subcommands keep their real exit codes.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

var (
	testBBSBin     string
	testBBSBinOnce sync.Once
	testBBSBinErr  error
)

// buildTestBBS builds ./cmd/bbs once per test binary into a dir that outlives
// any single test's TempDir.
func buildTestBBS(t testing.TB) string {
	t.Helper()
	testBBSBinOnce.Do(func() {
		// Anchor at this file, not the cwd — earlier tests in the package
		// os.Chdir and leave the working directory anywhere.
		_, src, _, _ := runtime.Caller(0)
		repo := filepath.Dir(filepath.Dir(filepath.Dir(src)))
		dir, err := os.MkdirTemp("", "bbs-e2e-bin")
		if err != nil {
			testBBSBinErr = err
			return
		}
		out := filepath.Join(dir, "bbs")
		cmd := exec.Command("go", "build", "-o", out, "./cmd/bbs")
		cmd.Dir = repo
		if b, err := cmd.CombinedOutput(); err != nil {
			testBBSBinErr = fmt.Errorf("go build: %v\n%s", err, b)
			return
		}
		testBBSBin = out
	})
	if testBBSBinErr != nil {
		t.Fatalf("%v", testBBSBinErr)
	}
	return testBBSBin
}

// e2e is the tracer-ticket harness: env + ticket home + per-case assertions.
type e2e struct {
	t    *testing.T
	bin  string
	th   string
	env  []string
	home string
	repo string
}

func newE2E(t *testing.T) *e2e {
	t.Helper()
	e := &e2e{t: t, bin: buildTestBBS(t)}
	// Subprocess cwd: sibling tests os.Chdir into deleted dirs, so an
	// inherited cwd can be gone by the time we exec. The repo root is what
	// the bash ran under — slug env resolves the project from it.
	_, src, _, _ := runtime.Caller(0)
	e.repo = filepath.Dir(filepath.Dir(filepath.Dir(src)))

	e.home = t.TempDir()
	babysitHome := filepath.Join(e.home, ".babysit")
	if err := os.MkdirAll(babysitHome, 0o755); err != nil {
		t.Fatal(err)
	}

	base := []string{
		"HOME=" + e.home,
		"BABYSIT_HOME=" + babysitHome,
		"BBS_TICKET=bs-tracer",
		"BABYSIT_TICKET=bs-tracer",
		// Pin legacy gates far in the future so outcomes don't drift.
		"BBS_LEGACY_SUNSET=2099-01-01",
		"BBS_LEGACY_HARDFAIL=2099-01-01",
		"PATH=" + os.Getenv("PATH"),
	}
	// Resolve the project home the way the bash did — `bbs slug env`.
	slugOut, _, code := e.runWith(base, "slug", "env")
	if code != 0 {
		t.Fatalf("bbs slug env exit %d", code)
	}
	projectHome := ""
	for _, ln := range strings.Split(slugOut, "\n") {
		if strings.HasPrefix(ln, "BABYSIT_PROJECT_HOME=") {
			projectHome = strings.Trim(strings.TrimPrefix(ln, "BABYSIT_PROJECT_HOME="), `'"`)
		}
	}
	if projectHome == "" {
		projectHome = filepath.Join(babysitHome, "projects", "unknown")
	}
	e.env = append(base, "BABYSIT_PROJECT_HOME="+projectHome)
	e.th = filepath.Join(projectHome, "tickets", "bs-tracer")
	e.wipe()
	return e
}

// runWith executes bbs with extra env prepended.
func (e *e2e) runWith(env []string, args ...string) (stdout, stderr string, exitCode int) {
	cmd := exec.Command(e.bin, args...)
	cmd.Env = env
	var outBuf, errBuf strings.Builder
	cmd.Dir = e.repo
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			e.t.Fatalf("exec %v: %v", args, err)
		}
	}
	return strings.TrimSpace(outBuf.String()), errBuf.String(), exitCode
}

// bbs execs `bbs <args>` — the bash's `bbs() { "${BBS_TICKET_BIN[@]}" "$@"; }`.
func (e *e2e) bbs(args ...string) (stdout, stderr string, exitCode int) {
	return e.runWith(e.env, args...)
}

func (e *e2e) ticket(args ...string) (stdout, stderr string, exitCode int) {
	return e.bbs(append([]string{"ticket"}, args...)...)
}

// wipe resets the tracer ticket home.
func (e *e2e) wipe() {
	_ = os.RemoveAll(e.th)
	if err := os.MkdirAll(e.th, 0o755); err != nil {
		e.t.Fatal(err)
	}
}

func (e *e2e) expectExit(name string, want int, args ...string) {
	e.t.Helper()
	_, _, got := e.ticket(args...)
	if got != want {
		e.t.Errorf("%s: exit=%d want=%d", name, got, want)
	}
}

func (e *e2e) expectStdout(name string, wantExit int, wantOut string, args ...string) {
	e.t.Helper()
	out, _, got := e.ticket(args...)
	if got != wantExit || out != wantOut {
		e.t.Errorf("%s: exit=%d/%d out=%q want=%q", name, got, wantExit, out, wantOut)
	}
}

func (e *e2e) expectStderrContains(name string, wantExit int, needle string, args ...string) {
	e.t.Helper()
	_, errOut, got := e.ticket(args...)
	if got != wantExit || !strings.Contains(errOut, needle) {
		e.t.Errorf("%s: exit=%d/%d err=%q want substring %q", name, got, wantExit, errOut, needle)
	}
}

func (e *e2e) expectStdoutLines(name string, wantExit, wantLines int, args ...string) {
	e.t.Helper()
	out, _, got := e.ticket(args...)
	n := 0
	if out != "" {
		n = len(strings.Split(out, "\n"))
	}
	if got != wantExit || n != wantLines {
		e.t.Errorf("%s: exit=%d/%d lines=%d/%d", name, got, wantExit, n, wantLines)
	}
}

func (e *e2e) write(rel, content string) {
	p := filepath.Join(e.th, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func TestTicketPathResolver(t *testing.T) {
	e := newE2E(t)

	e.expectStdout("path plan --write prints canonical", 0, filepath.Join(e.th, "plan.md"),
		"path", "plan", "--write")
	if !isDir(e.th) {
		t.Error("plan --write parent exists")
	}
	e.expectExit("path plan --read (none)", 1, "path", "plan", "--read")

	e.write("plan.md", "the plan\n")
	e.expectStdout("path plan --read (canonical hit)", 0, filepath.Join(e.th, "plan.md"),
		"path", "plan", "--read")

	// Legacy-only hit (Layout B → C migration).
	_ = os.Remove(filepath.Join(e.th, "plan.md"))
	e.write("plan-feature/plan.md", "legacy plan\n")
	e.expectStdout("path plan --read (legacy hit)", 0, filepath.Join(e.th, "plan-feature", "plan.md"),
		"path", "plan", "--read")

	// Both present → canonical wins, BBS_PATH_DUAL on stderr.
	e.write("plan.md", "canonical plan\n")
	e.expectStdout("path plan --read (both → canonical)", 0, filepath.Join(e.th, "plan.md"),
		"path", "plan", "--read")
	e.expectStderrContains("BBS_PATH_DUAL emitted when both exist", 0, "BBS_PATH_DUAL",
		"path", "plan", "--read")
	_ = os.RemoveAll(filepath.Join(e.th, "plan-feature"))
	_ = os.Remove(filepath.Join(e.th, "plan.md"))
}

func TestTicketPathWriterRejection(t *testing.T) {
	e := newE2E(t)
	e.expectExit("handoff --write rejected", 2, "path", "handoff", "--skill", "build", "--write")
	e.expectStderrContains("handoff --write hint", 2, "add-handoff",
		"path", "handoff", "--skill", "build", "--write")
	e.expectExit("verdict --write rejected", 2, "path", "verdict", "--skill", "build", "--write")
	e.expectExit("review --write rejected", 2, "path", "review", "--skill", "build", "--write")
}

func TestTicketPathSelectorValidation(t *testing.T) {
	e := newE2E(t)
	e.expectExit("skill ../etc → exit 3", 3, "path", "verdict", "--skill", "../etc", "--read")
	e.expectExit("name ../wat → exit 3", 3, "path", "evidence", "--skill", "build", "--name", "../wat", "--write")
	e.expectExit("skill /abs → exit 3", 3, "path", "verdict", "--skill", "/abs", "--read")
	e.expectExit("skill foo/bar → exit 3", 3, "path", "verdict", "--skill", "foo/bar", "--read")
	e.expectExit("skill 'foo bar' → exit 2 (forbidden chars)", 2, "path", "verdict", "--skill", "foo bar", "--read")
	e.expectExit("seq abc → exit 2", 2, "path", "sub-ticket", "--seq", "abc", "--slug", "x", "--read")
	e.expectExit("missing required --skill → exit 2", 2, "path", "verdict", "--read")
	e.expectExit("missing required --slug on sub-ticket write → exit 2", 2, "path", "sub-ticket", "--seq", "1", "--write")
	e.expectExit("missing required --name on evidence write → exit 2", 2, "path", "evidence", "--skill", "build", "--write")
}

func TestTicketPathHelpAndUnknown(t *testing.T) {
	e := newE2E(t)
	e.expectExit("no args → exit 2 (kind table)", 2, "path")
	e.expectStderrContains("no args prints kind table", 2, "Kinds (canonical Layout C", "path")
	e.expectExit("unknown kind → exit 2", 2, "path", "bogus")
	e.expectStderrContains("unknown kind hint", 2, "unknown kind", "path", "bogus")
	e.expectExit("kind without mode → exit 2 (per-kind help)", 2, "path", "plan")
	e.expectStderrContains("per-kind help shows synopsis", 2, "Canonical:", "path", "plan")
}

func TestTicketList(t *testing.T) {
	e := newE2E(t)
	e.expectStdout("list verdict empty → exit 0", 0, "", "list", "verdict")
	e.expectStdout("list handoff empty → exit 0", 0, "", "list", "handoff")
	e.expectStdout("list sub-ticket empty → exit 0", 0, "", "list", "sub-ticket")

	e.write("verdicts/zeta.md", "")
	e.write("verdicts/alpha.md", "")
	e.write("verdicts/build.md", "")
	e.expectStdoutLines("list verdict (3)", 0, 3, "list", "verdict")
	out, _, _ := e.ticket("list", "verdict")
	if first := strings.Split(out, "\n")[0]; first != filepath.Join(e.th, "verdicts", "alpha.md") {
		t.Errorf("list verdict sorted: got %q", first)
	}

	e.ticket("add-handoff", "--skill", "build", "--status", "done", "--body", "x")
	e.ticket("add-handoff", "--skill", "quality", "--status", "done", "--body", "y")
	e.ticket("add-handoff", "--skill", "build", "--status", "done", "--body", "z")
	e.expectStdoutLines("list handoff (all 3)", 0, 3, "list", "handoff")
	e.expectStdoutLines("list handoff --skill build (2)", 0, 2, "list", "handoff", "--skill", "build")
	e.expectStdoutLines("list handoff --skill quality (1)", 0, 1, "list", "handoff", "--skill", "quality")
	e.expectStdoutLines("list handoff --skill missing (0)", 0, 0, "list", "handoff", "--skill", "nonexistent")

	latest, _, _ := e.ticket("path", "handoff", "--skill", "build", "--latest", "--read")
	if !strings.HasSuffix(latest, "003-build-done.md") {
		t.Errorf("handoff --latest picks highest seq: got %q", latest)
	}
	e.expectExit("handoff --latest missing skill → exit 1", 1,
		"path", "handoff", "--skill", "never", "--latest", "--read")
}

func TestTicketPathEvidenceAndSubTicket(t *testing.T) {
	e := newE2E(t)
	e.expectStdout("evidence --write canonical", 0, filepath.Join(e.th, "evidence", "quality", "report.txt"),
		"path", "evidence", "--skill", "quality", "--name", "report.txt", "--write")
	if !isDir(filepath.Join(e.th, "evidence", "quality")) {
		t.Error("evidence --write created parent dir")
	}
	e.write("evidence/quality/report.txt", "data\n")
	e.expectStdout("evidence --read hit", 0, filepath.Join(e.th, "evidence", "quality", "report.txt"),
		"path", "evidence", "--skill", "quality", "--name", "report.txt", "--read")

	e.expectStdout("sub-ticket --write canonical", 0, filepath.Join(e.th, "sub-tickets", "001-api.md"),
		"path", "sub-ticket", "--seq", "1", "--slug", "api", "--write")
	e.write("sub-tickets/001-api.md", "x\n")
	e.expectStdout("sub-ticket --read hit", 0, filepath.Join(e.th, "sub-tickets", "001-api.md"),
		"path", "sub-ticket", "--seq", "1", "--slug", "api", "--read")
}

func TestTicketPathSimpleKinds(t *testing.T) {
	e := newE2E(t)
	e.expectStdout("checkpoint --write", 0, filepath.Join(e.th, "checkpoint.json"), "path", "checkpoint", "--write")
	e.expectStdout("index --write", 0, filepath.Join(e.th, "index.json"), "path", "index", "--write")
	e.expectStdout("requirement --write", 0, filepath.Join(e.th, "requirement.md"), "path", "requirement", "--write")
	e.expectStdout("design --write", 0, filepath.Join(e.th, "design.md"), "path", "design", "--write")
	e.expectStdout("manifest --write", 0, filepath.Join(e.th, "manifest.md"), "path", "manifest", "--write")
	e.expectStdout("history --write", 0, filepath.Join(e.th, "history.jsonl"), "path", "history", "--write")
	e.expectStdout("home --read", 0, e.th, "path", "home", "--read")
}

func TestTicketPathTelemetry(t *testing.T) {
	e := newE2E(t)
	telemetry := filepath.Join(e.home, ".babysit", "analytics", "path-fallbacks.jsonl")
	count := func() int {
		b, err := os.ReadFile(telemetry)
		if err != nil {
			return 0
		}
		return len(strings.Split(strings.TrimRight(string(b), "\n"), "\n"))
	}

	// Trigger a legacy hit — telemetry appends by default.
	_ = os.RemoveAll(filepath.Join(e.th, "plan-feature"))
	_ = os.Remove(filepath.Join(e.th, "plan.md"))
	e.write("plan-feature/plan.md", "legacy\n")
	before := count()
	e.ticket("path", "plan", "--read")
	if after := count(); after <= before {
		t.Errorf("telemetry appended on legacy hit (default ON): before=%d after=%d", before, after)
	}

	before = count()
	env := append(append([]string{}, e.env...), "BBS_PATH_TELEMETRY=0")
	e.runWith(env, "ticket", "path", "plan", "--read")
	if after := count(); after != before {
		t.Errorf("telemetry suppressed by BBS_PATH_TELEMETRY=0: before=%d after=%d", before, after)
	}
}

func TestTicketLintFixture(t *testing.T) {
	e := newE2E(t)
	_, src, _, _ := runtime.Caller(0)
	repo := filepath.Dir(filepath.Dir(filepath.Dir(src)))
	fixture := filepath.Join(repo, ".claude", "skills", "references", "lint-test-fixtures.md")
	if _, err := os.Stat(fixture); err != nil {
		t.Skip("lint-test-fixtures.md missing")
	}
	emptyManifest := filepath.Join(t.TempDir(), "manifest.txt")
	out, _, code := e.ticket("lint", "--mode", "enforce", "--path", fixture, "--manifest", emptyManifest)
	lines := 0
	if out != "" {
		lines = len(strings.Split(out, "\n"))
	}
	if code != 1 || lines != 5 {
		t.Errorf("linter fixture: want exit 1 + 5 hits, got exit=%d lines=%d\n%s", code, lines, out)
	}
	if strings.Contains(out, "lint:allow-direct-path") {
		t.Error("linter fixture: allow-marker line was not filtered out")
	}
}

// The six lifecycle scenarios: multi-skill workflows resuming from disk
// artifacts written by earlier skills.
func TestTicketLifecycleScenarios(t *testing.T) {
	e := newE2E(t)

	// Scenario 1 — office-hours full flow.
	e.wipe()
	e.expectStdout("S1: office-hours design --write", 0, filepath.Join(e.th, "design.md"), "path", "design", "--write")
	e.write("design.md", "design body\n")
	e.expectStdout("S1: plan-feature reads design", 0, filepath.Join(e.th, "design.md"), "path", "design", "--read")
	e.expectStdout("S1: plan-feature plan --write", 0, filepath.Join(e.th, "plan.md"), "path", "plan", "--write")
	e.write("plan.md", "plan body\n")
	e.expectStdout("S1: decompose reads plan", 0, filepath.Join(e.th, "plan.md"), "path", "plan", "--read")
	e.expectStdout("S1: decompose sub-ticket 001", 0, filepath.Join(e.th, "sub-tickets", "001-step-one.md"),
		"path", "sub-ticket", "--seq", "1", "--slug", "step-one", "--write")
	e.write("sub-tickets/001-step-one.md", "x\n")
	e.expectStdout("S1: decompose sub-ticket 002", 0, filepath.Join(e.th, "sub-tickets", "002-step-two.md"),
		"path", "sub-ticket", "--seq", "2", "--slug", "step-two", "--write")
	e.write("sub-tickets/002-step-two.md", "x\n")
	e.expectStdoutLines("S1: list sub-ticket enumerates both", 0, 2, "list", "sub-ticket")

	// Scenario 2 — plan-feature entry (skip office-hours).
	e.wipe()
	e.write("design.md", "design\n")
	e.expectStdout("S2: plan-feature reads existing design", 0, filepath.Join(e.th, "design.md"), "path", "design", "--read")
	e.expectExit("S2: plan --read missing → exit 1", 1, "path", "plan", "--read")
	e.expectStdout("S2: plan --write canonical", 0, filepath.Join(e.th, "plan.md"), "path", "plan", "--write")
	e.write("plan.md", "plan\n")
	e.expectStdout("S2: plan --read after write", 0, filepath.Join(e.th, "plan.md"), "path", "plan", "--read")

	// Scenario 3 — decompose entry (large requirement).
	e.wipe()
	e.write("requirement.md", "req\n")
	e.write("plan.md", "plan\n")
	e.expectStdout("S3: decompose reads requirement", 0, filepath.Join(e.th, "requirement.md"), "path", "requirement", "--read")
	e.expectStdout("S3: decompose reads plan", 0, filepath.Join(e.th, "plan.md"), "path", "plan", "--read")
	for i := 1; i <= 5; i++ {
		seq := strconv.Itoa(i)
		slug := "step-" + seq
		e.expectStdout("S3: write sub-ticket "+seq, 0,
			filepath.Join(e.th, "sub-tickets", "00"+seq+"-"+slug+".md"),
			"path", "sub-ticket", "--seq", seq, "--slug", slug, "--write")
		e.write("sub-tickets/00"+seq+"-"+slug+".md", "x\n")
	}
	e.expectStdoutLines("S3: list sub-ticket (5)", 0, 5, "list", "sub-ticket")
	e.expectStdout("S3: read by seq+slug", 0, filepath.Join(e.th, "sub-tickets", "003-step-3.md"),
		"path", "sub-ticket", "--seq", "3", "--slug", "step-3", "--read")
	e.expectStdout("S3: read by seq-only fallback", 0, filepath.Join(e.th, "sub-tickets", "004-step-4.md"),
		"path", "sub-ticket", "--seq", "4", "--read")
	e.expectExit("S3: bad slug → exit 3", 3, "path", "sub-ticket", "--slug", "../etc", "--read")

	// Scenario 4 — implement entry (small requirement).
	e.wipe()
	e.write("requirement.md", "small req\n")
	e.write("plan.md", "plan\n")
	e.expectStdout("S4: implement reads requirement", 0, filepath.Join(e.th, "requirement.md"), "path", "requirement", "--read")
	e.expectStdout("S4: implement reads plan", 0, filepath.Join(e.th, "plan.md"), "path", "plan", "--read")
	e.expectStderrContains("S4: handoff --write redirects to add-handoff", 2, "add-handoff",
		"path", "handoff", "--skill", "implement", "--write")
	e.expectStderrContains("S4: verdict --write redirects to set-verdict", 2, "set-verdict",
		"path", "verdict", "--skill", "implement", "--write")
	e.ticket("add-handoff", "--skill", "implement", "--status", "FIXED(1 commit)", "--brief", "added cmd")
	e.ticket("set-verdict", "--skill", "implement", "--body", "STATUS: DONE\ntests green")
	e.expectExit("S4: read implement handoff back", 0, "path", "handoff", "--skill", "implement", "--latest", "--read")
	e.expectStdout("S4: read implement verdict back", 0, filepath.Join(e.th, "verdicts", "implement.md"),
		"path", "verdict", "--skill", "implement", "--read")

	// Scenario 5 — work build workflow.
	e.wipe()
	e.expectStdout("S5: ticket init creates index.json", 0, filepath.Join(e.th, "index.json"), "init")
	e.expectExit("S5: set-pointer base_branch", 0, "set-pointer", "base_branch", "main")
	e.expectStdout("S5: get pointers.base_branch", 0, "main", "get", "pointers.base_branch")
	brief := filepath.Join(t.TempDir(), "brief.md")
	_ = os.WriteFile(brief, []byte("## Build\n- 1 commit\n"), 0o644)
	e.expectStdout("S5: add-handoff --body-file", 0, filepath.Join(e.th, "handoffs", "001-build-done.md"),
		"add-handoff", "--skill", "build", "--status", "DONE", "--body-file", brief)
	e.expectStdout("S5: checkpoint --write", 0, filepath.Join(e.th, "checkpoint.json"), "path", "checkpoint", "--write")
	e.expectStdout("S5: evidence --write under build namespace", 0, filepath.Join(e.th, "evidence", "build", "log.txt"),
		"path", "evidence", "--skill", "build", "--name", "log.txt", "--write")

	// Scenario 6 — work quality workflow (builds on scenario 5 state).
	e.expectStdout("S6: discover build handoff via --latest", 0, filepath.Join(e.th, "handoffs", "001-build-done.md"),
		"path", "handoff", "--skill", "build", "--latest", "--read")
	e.expectExit("S6: implement-handoff fallback miss → exit 1", 1,
		"path", "handoff", "--skill", "implement", "--latest", "--read")
	e.expectStdout("S6: write quality summary in evidence/quality", 0,
		filepath.Join(e.th, "evidence", "quality", "quality-summary.md"),
		"path", "evidence", "--skill", "quality", "--name", "quality-summary.md", "--write")
	e.write("evidence/quality/quality-summary.md", "summary\n")
	e.expectStdout("S6: read quality summary back", 0, filepath.Join(e.th, "evidence", "quality", "quality-summary.md"),
		"path", "evidence", "--skill", "quality", "--name", "quality-summary.md", "--read")
	e.ticket("set-verdict", "--skill", "quality", "--body", "STATUS: DONE\nall green")
	e.expectStdout("S6: read quality verdict", 0, filepath.Join(e.th, "verdicts", "quality.md"),
		"path", "verdict", "--skill", "quality", "--read")
}

func TestTicketTypedEvidence(t *testing.T) {
	e := newE2E(t)
	e.expectStdout("EV: status none before any write", 0, "none", "evidence-status", "--kind", "verification")
	e.expectStdout("EV: verification valid write returns path", 0, filepath.Join(e.th, "evidence", "verification", "result.json"),
		"set-evidence", "--kind", "verification", "--json", `{"result":"PASS","after":"BUILT(S)"}`)
	e.expectStdout("EV: verification status valid", 0, "valid", "evidence-status", "--kind", "verification")
	e.expectExit("EV: bad result value → exit 2", 2, "set-evidence", "--kind", "verification", "--json", `{"result":"MAYBE"}`)
	e.expectExit("EV: not-json → exit 2", 2, "set-evidence", "--kind", "risk-gate", "--json", "nope")
	e.expectStderrContains("EV: missing required field names it", 2, "high_risk",
		"set-evidence", "--kind", "risk-gate", "--json", `{"reason":"x"}`)
	e.expectExit("EV: unknown kind → exit 2", 2, "set-evidence", "--kind", "bogus", "--json", "{}")
	e.ticket("set-evidence", "--kind", "risk-gate", "--json", `{"high_risk":false,"reason":"isolated"}`)
	e.expectStdout("EV: risk-gate status valid", 0, "valid", "evidence-status", "--kind", "risk-gate")
	e.ticket("set-evidence", "--kind", "adversarial", "--json", `{"disproven":[],"unverified":["claim X"]}`)
	e.expectStdout("EV: adversarial status valid", 0, "valid", "evidence-status", "--kind", "adversarial")
	e.write("evidence/adversarial/result.json", "garbage")
	e.expectStdout("EV: malformed-on-disk → malformed", 0, "malformed", "evidence-status", "--kind", "adversarial")
}
