package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/2found/2build/internal/ticket"
)

// The decision trail is the project's own, append-only record of the
// judgments the team took for the human: what, why, the evidence it rested on
// and where it ended. Mechanical events (seal, integrate, land, release) are
// already in project state; the trail holds what state cannot reconstruct, so
// a resumed lead can still write "Decisions taken for you" truthfully.

const decisionRowMax = 4096

type decisionRow struct {
	ID         string `json:"id"`
	At         string `json:"ts"`
	Phase      string `json:"phase"`
	Ticket     string `json:"ticket,omitempty"`
	Actor      string `json:"actor"`
	Decision   string `json:"decision"`
	Why        string `json:"why"`
	Evidence   string `json:"evidence"`
	Result     string `json:"result"`
	Supersedes string `json:"supersedes,omitempty"`
}

// A decisionFlag is a row the human should look at before trusting it.
type decisionFlag struct {
	Row    string
	Reason string
}

func projectDecisionsPath(st *ticket.Store) string {
	return filepath.Join(st.Home(), "decisions.jsonl")
}

func readDecisionRows(st *ticket.Store) ([]decisionRow, error) {
	f, err := os.Open(projectDecisionsPath(st))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rows []decisionRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, decisionRowMax), decisionRowMax*2)
	for sc.Scan() {
		var r decisionRow
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.ID != "" {
			rows = append(rows, r)
		}
	}
	return rows, sc.Err()
}

func projectLogCommand(st *ticket.Store, kv map[string]string) error {
	r := decisionRow{
		At: isoNow(), Phase: strings.TrimSpace(kv["phase"]), Ticket: strings.TrimSpace(kv["ticket"]),
		Actor: strings.TrimSpace(orDefault(kv["actor"], "lead")), Decision: strings.TrimSpace(kv["decision"]),
		Why: strings.TrimSpace(kv["why"]), Evidence: strings.TrimSpace(kv["evidence"]),
		Result: strings.TrimSpace(kv["result"]), Supersedes: strings.TrimSpace(kv["supersedes"]),
	}
	if r.Phase == "" || r.Decision == "" || r.Why == "" || r.Evidence == "" || r.Result == "" {
		return fmt.Errorf("log needs --phase, --decision, --why, --evidence and --result")
	}
	sum := sha256.Sum256([]byte(r.At + r.Ticket + r.Decision))
	r.ID = hex.EncodeToString(sum[:4])
	return withLock(st, func() error {
		if r.Supersedes != "" {
			rows, _ := readDecisionRows(st)
			found := false
			for _, x := range rows {
				found = found || x.ID == r.Supersedes
			}
			if !found {
				return fmt.Errorf("--supersedes %s: no such row in the trail", r.Supersedes)
			}
		}
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if len(line)+1 > decisionRowMax {
			return fmt.Errorf("row is %d bytes; keep it under %d (evidence is a pointer, not prose)", len(line)+1, decisionRowMax)
		}
		f, err := os.OpenFile(projectDecisionsPath(st), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Write(append(line, '\n')); err != nil {
			return err
		}
		fmt.Println("DECISION=" + r.ID)
		return nil
	})
}

var (
	decisionSHA  = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	decisionPath = regexp.MustCompile(`^([^\s:]+(?:/[^\s:]*|\.[A-Za-z][A-Za-z0-9]*))(?::(\d+))?$`)
)

// auditDecisions returns the current rows (superseded ones hidden), how many
// were superseded, and the flags: evidence that does not resolve, and results
// that are not verified. Deterministic and offline; a URL is recorded, never
// fetched.
func auditDecisions(st *ticket.Store, s projectSnapshot, rows []decisionRow) ([]decisionRow, int, []decisionFlag) {
	superseded := map[string]bool{}
	for _, r := range rows {
		if r.Supersedes != "" {
			superseded[r.Supersedes] = true
		}
	}
	repos := projectRepoDirs(s)
	var live []decisionRow
	var flags []decisionFlag
	for _, r := range rows {
		if superseded[r.ID] {
			continue
		}
		live = append(live, r)
		if why := decisionEvidenceProblem(st.Home(), repos, r.Evidence); why != "" {
			flags = append(flags, decisionFlag{Row: r.ID, Reason: why})
		}
		switch strings.ToLower(r.Result) {
		case "failed", "open", "inconclusive":
			flags = append(flags, decisionFlag{Row: r.ID, Reason: "result is " + r.Result + ", not verified"})
		}
	}
	return live, len(rows) - len(live), flags
}

func projectRepoDirs(s projectSnapshot) []string {
	seen := map[string]bool{}
	var dirs []string
	add := func(d string) {
		if d != "" && !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	for _, c := range s.Children {
		for _, r := range c.Repos {
			add(r.Canonical)
		}
	}
	for _, p := range s.Phases {
		if p.Candidate != nil {
			for _, r := range p.Candidate.Repos {
				add(r.Canonical)
			}
		}
	}
	add(gitOutIn(".", "rev-parse", "--show-toplevel"))
	return dirs
}

func decisionEvidenceProblem(home string, repos []string, ev string) string {
	switch {
	case strings.HasPrefix(ev, "http://"), strings.HasPrefix(ev, "https://"):
		return ""
	case decisionSHA.MatchString(ev):
		for _, d := range repos {
			if gitOKIn(d, "cat-file", "-e", ev+"^{commit}") {
				return ""
			}
		}
		return "commit " + ev + " not found in the project's repositories"
	}
	m := decisionPath.FindStringSubmatch(ev)
	if m == nil {
		return "" // a rubric row or other named pointer: recorded, not resolved
	}
	candidates := []string{m[1]}
	if !filepath.IsAbs(m[1]) {
		candidates = []string{filepath.Join(home, m[1])}
		for _, d := range repos {
			candidates = append(candidates, filepath.Join(d, m[1]))
		}
	}
	for _, p := range candidates {
		body, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if m[2] == "" {
			return ""
		}
		n, _ := strconv.Atoi(m[2])
		if lines := strings.Count(string(body), "\n") + 1; n >= 1 && n <= lines {
			return ""
		}
		return fmt.Sprintf("%s has no line %s", m[1], m[2])
	}
	return "path " + m[1] + " not found"
}
