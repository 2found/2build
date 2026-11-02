package ticket

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Planning fields (bs-w5p3c6mq). All are additive keys on index.json — a reader
// that predates them sees unknown fields and preserves them, which is what lets
// every existing skill keep reading the same record. The canonical shapes:
//
//	title        string
//	kind         milestone | phase | work            (absent = work)
//	product      slug
//	assigned_to  {role, person}                       (assignee stays the lead id)
//	labels       [string]
//	window       {start, due}                         (YYYY-MM-DD)
//	done_at      ISO timestamp, stamped when status becomes done
//	loop         {stage, target_outcome, expected_delta, check_after_days,
//	              check_from, evidence_refs, decision{...}, actual{...}}
//	import       {plan, key}
//
// Everything derived from them — check due date, readiness, progress — is
// computed on read here and never stored, so it cannot drift.

const (
	KindMilestone = "milestone"
	KindPhase     = "phase"
	KindWork      = "work"
)

var (
	Kinds       = []string{KindMilestone, KindPhase, KindWork}
	Roles       = []string{"swe", "human", "marketing", "ops"}
	LoopStages  = []string{"insight", "product", "marketing", "operation"}
	Decisions   = []string{"go", "later", "drop"}
	Results     = []string{"hit", "partial", "miss"}
	productRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	labelRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,63}$`)
	dateRe      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	h1Re        = regexp.MustCompile(`^#\s+(.+?)\s*$`)
	personRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{0,63}$`)
	evidenceMax = 512
)

func oneOf(v string, set []string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

// ValidKind, ValidRole … are the one place each enum lives; the CLI, the
// importer and the dashboard all reject through them.
func ValidKind(v string) error     { return enumErr("kind", v, Kinds) }
func ValidRole(v string) error     { return enumErr("role", v, Roles) }
func ValidStage(v string) error    { return enumErr("stage", v, LoopStages) }
func ValidDecision(v string) error { return enumErr("decision", v, Decisions) }
func ValidResult(v string) error   { return enumErr("result", v, Results) }

func enumErr(name, v string, set []string) error {
	if oneOf(v, set) {
		return nil
	}
	return fmt.Errorf("invalid %s %q (valid: %s)", name, v, strings.Join(set, "|"))
}

func ValidProduct(v string) error {
	if productRe.MatchString(v) {
		return nil
	}
	return fmt.Errorf("invalid product %q (lowercase letters, digits, dash; e.g. lohi, 2novel)", v)
}

func ValidLabel(v string) error {
	if labelRe.MatchString(v) {
		return nil
	}
	return fmt.Errorf("invalid label %q (letters, digits, . _ : / -; max 64)", v)
}

func ValidPerson(v string) error {
	if personRe.MatchString(v) {
		return nil
	}
	return fmt.Errorf("invalid person %q (letters, digits, . _ @ -)", v)
}

// ValidDate accepts a real calendar date in YYYY-MM-DD.
func ValidDate(v string) error {
	if dateRe.MatchString(v) {
		if _, err := time.Parse("2006-01-02", v); err == nil {
			return nil
		}
	}
	return fmt.Errorf("invalid date %q (want YYYY-MM-DD)", v)
}

// ValidEvidence keeps a ref to one line of bounded text: a URL, a ticket path
// or an id. Refs are pointers, never the evidence itself (CL1.2 "refs instead
// of text").
func ValidEvidence(v string) error {
	if strings.TrimSpace(v) == "" || len(v) > evidenceMax || strings.ContainsAny(v, "\n\r") {
		return fmt.Errorf("invalid evidence ref %q (one line, 1–%d chars)", v, evidenceMax)
	}
	return nil
}

// KindOf reads kind with its default.
func KindOf(d Doc) string {
	if k := d.Get("kind"); k != "" {
		return k
	}
	return KindWork
}

// Title is the record's title, falling back to the requirement's first H1 for
// tickets written before the field existed. The fallback is read-only: only
// `set-title` or `migrate` writes the field.
func Title(home string, d Doc) string {
	if t := strings.TrimSpace(d.Get("title")); t != "" {
		return t
	}
	return RequirementH1(filepath.Join(home, "requirement.md"))
}

// RequirementH1 returns the first `# ` heading in the first 40 lines, or "".
func RequirementH1(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for i := 0; i < 40 && sc.Scan(); i++ {
		if m := h1Re.FindStringSubmatch(sc.Text()); m != nil {
			return m[1]
		}
	}
	return ""
}

// CheckDue derives when a loop check comes due: the anchor plus
// check_after_days. The anchor is `loop.check_from` when set, else the work
// shipping (`done_at`) for work, else the window start for a milestone or
// phase. ok=false means no check is scheduled yet (no days, or no anchor —
// e.g. work not done).
func CheckDue(d Doc) (string, bool) {
	days, err := strconv.Atoi(d.Get("loop.check_after_days"))
	if err != nil || days < 0 {
		return "", false
	}
	anchor := d.Get("loop.check_from")
	if anchor == "" && KindOf(d) == KindWork {
		if at := d.Get("done_at"); len(at) >= 10 {
			anchor = at[:10]
		}
	}
	if anchor == "" && KindOf(d) != KindWork {
		anchor = d.Get("window.start")
	}
	t, err := time.Parse("2006-01-02", anchor)
	if err != nil {
		return "", false
	}
	return t.AddDate(0, 0, days).Format("2006-01-02"), true
}

// CheckOverdue reports a due check with no recorded actual — the "check-after
// passed, actual_delta missing" signal. A dropped or later opportunity has
// nothing to check.
func CheckOverdue(d Doc, today string) bool {
	if d.Value("loop.actual") != nil {
		return false
	}
	if v := d.Get("loop.decision.value"); v == "drop" || v == "later" {
		return false
	}
	due, ok := CheckDue(d)
	return ok && due <= today
}

// Today is the local calendar date the founder lives in.
func Today() string { return time.Now().Format("2006-01-02") }
