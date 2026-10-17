package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/2found/2build/internal/ticket"
)

// This file ports the append-only artifact family of the retired bbs-ticket.bash:
// add-handoff, set-review, set-evidence, evidence-status. All are
// file I/O over the Layout C ticket home; none touch git.

// runAddHandoff ports add-handoff (tests/fixtures/bbs-ticket.reference:1340-1375): write the next
// numbered <NNN>-<skill>-<status>.md handoff, update handoffs/LATEST, and append
// a history row. Holds the index lock while allocating the sequence number.
func runAddHandoff(args []string) {
	env := resolveEnv()
	needTicket(env)
	var skill, status, body, bodyFile string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--skill":
			skill, i = valueOf(args, i, "--skill"), i+1
		case "--status":
			status, i = valueOf(args, i, "--status"), i+1
		case "--body":
			body, i = valueOf(args, i, "--body"), i+1
		case "--body-file":
			bodyFile, i = valueOf(args, i, "--body-file"), i+1
		}
	}
	if skill == "" || status == "" {
		fmt.Fprintln(os.Stderr, "add-handoff: --skill and --status required")
		os.Exit(2)
	}
	skill = safePathComponent("handoff", "skill", skill)

	st := ticket.New(env)
	st.EnsureDirs()
	acquireOrDie(st) // released explicitly below — os.Exit skips defers, leaking the lock

	th := st.Home()
	seq := nextHandoffSeq(th)
	statusSlug := keepChars(strings.ToLower(status), "abcdefghijklmnopqrstuvwxyz0123456789_-")
	if statusSlug == "" {
		fmt.Fprintf(os.Stderr, "add-handoff: --status '%s' has no a-z0-9_- characters\n", status)
		os.Exit(2)
	}
	fn := seq + "-" + skill + "-" + statusSlug + ".md"
	hp := filepath.Join(th, "handoffs", fn)

	if bodyFile != "" && fileExists(bodyFile) {
		b, err := os.ReadFile(bodyFile)
		if err == nil {
			_ = os.WriteFile(hp, b, 0o644)
		}
	} else {
		if body == "" {
			body = "<no body>"
		}
		_ = os.WriteFile(hp, []byte(body+"\n"), 0o644)
	}
	// Plain-text LATEST pointer — symlinks are unreliable on some filesystems.
	_ = os.WriteFile(filepath.Join(th, "handoffs", "LATEST"), []byte(fn+"\n"), 0o644)
	st.HistoryAppendExtra("handoff", skill, fmt.Sprintf(`{"status":"%s","file":"%s"}`, status, fn))
	st.ReleaseLock()
	fmt.Println(hp)
	os.Exit(0)
}

// nextHandoffSeq mirrors bash next_handoff_seq: the max NNN prefix among
// handoffs/[0-9][0-9][0-9]-*.md, plus one, 0-padded to 3 digits.
func nextHandoffSeq(home string) string {
	max := 0
	matches, _ := filepath.Glob(filepath.Join(home, "handoffs", "[0-9][0-9][0-9]-*.md"))
	for _, f := range matches {
		base := filepath.Base(f)
		if i := strings.IndexByte(base, '-'); i > 0 {
			if n, err := strconv.Atoi(base[:i]); err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("%03d", max+1)
}

// invalidLatestName mirrors the bash case guard ""|*/*|*..*|.* on a LATEST value.
func invalidLatestName(name string) bool {
	return name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") || strings.HasPrefix(name, ".")
}

// runSetReview ports set-review (tests/fixtures/bbs-ticket.reference:1475-1500): overwrite
// reviews/<skill>.md and append a history row. Unknown args fail loud (exit 2).
func runSetReview(args []string) {
	env := resolveEnv()
	needTicket(env)
	var skill, body, bodyFile string
	haveBody := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--skill":
			skill, i = valueOf(args, i, "--skill"), i+1
		case "--body":
			body, i, haveBody = valueOf(args, i, "--body"), i+1, true
		case "--body-file":
			bodyFile, i = valueOf(args, i, "--body-file"), i+1
		default:
			fmt.Fprintf(os.Stderr, "set-review: unknown arg '%s' (usage: set-review --skill S [--body MD | --body-file FILE])\n", args[i])
			os.Exit(2)
		}
	}
	if skill == "" {
		fmt.Fprintln(os.Stderr, "set-review: --skill required")
		os.Exit(2)
	}
	skill = safePathComponent("review", "skill", skill)

	st := ticket.New(env)
	st.EnsureDirs()
	rp := filepath.Join(st.Home(), "reviews", skill+".md")
	if bodyFile != "" {
		b, err := os.ReadFile(bodyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "set-review: --body-file '%s' not found\n", bodyFile)
			os.Exit(2)
		}
		_ = os.WriteFile(rp, b, 0o644)
	} else {
		if !haveBody || body == "" {
			body = "<no review>"
		}
		_ = os.WriteFile(rp, []byte(body+"\n"), 0o644)
	}
	st.HistoryAppend("review", skill)
	fmt.Println(rp)
	os.Exit(0)
}

var evidenceReq = map[string][]string{
	"verification": {"result"},
	"risk-gate":    {"high_risk"},
	"adversarial":  {"disproven", "unverified"},
}

// runSetEvidence ports set-evidence (tests/fixtures/bbs-ticket.reference:1506-1547): validate a typed
// JSON evidence blob on write (so a malformed blob never lands silently) and
// persist it at evidence/<kind>/result.json.
func runSetEvidence(args []string) {
	env := resolveEnv()
	needTicket(env)
	var kind, jsonStr, jsonFile string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--kind":
			kind, i = valueOf(args, i, "--kind"), i+1
		case "--json":
			jsonStr, i = valueOf(args, i, "--json"), i+1
		case "--json-file":
			jsonFile, i = valueOf(args, i, "--json-file"), i+1
		}
	}
	if _, ok := evidenceReq[kind]; !ok {
		fmt.Fprintln(os.Stderr, "set-evidence: --kind must be verification|risk-gate|adversarial")
		os.Exit(2)
	}
	if jsonFile != "" && fileExists(jsonFile) {
		if b, err := os.ReadFile(jsonFile); err == nil {
			jsonStr = string(b)
		}
	}
	if jsonStr == "" {
		fmt.Fprintln(os.Stderr, "set-evidence: --json or --json-file required")
		os.Exit(2)
	}
	// Verification evidence has one shape — the producer's schema_version 2
	// envelope; `bbs autopilot verification begin/record` writes it.
	if kind == "verification" {
		if evidenceSchemaVersion(jsonStr) != 2 {
			fmt.Fprintln(os.Stderr, "set-evidence: verification evidence must be a schema_version 2 envelope; record gates with `bbs autopilot verification begin/record`")
			os.Exit(2)
		}
		path, err := setV2VerificationEvidence(env, []byte(jsonStr))
		if err != nil {
			fmt.Fprintln(os.Stderr, "set-evidence:", err)
			os.Exit(2)
		}
		fmt.Println(path)
		os.Exit(0)
	}
	if msg := validateEvidence(kind, jsonStr); msg != "" {
		fmt.Fprint(os.Stderr, msg)
		os.Exit(2)
	}

	st := ticket.New(env)
	st.EnsureDirs()
	_ = os.MkdirAll(filepath.Join(st.Home(), "evidence", kind), 0o755)
	ep := filepath.Join(st.Home(), "evidence", kind, "result.json")
	_ = os.WriteFile(ep, []byte(jsonStr+"\n"), 0o644)
	st.HistoryAppend("evidence", kind)
	fmt.Println(ep)
	os.Exit(0)
}

// validateEvidence returns "" when the blob is valid, else the exact stderr text
// (with trailing newline) the bash python validator emits.
func validateEvidence(kind, jsonStr string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return fmt.Sprintf("set-evidence: not valid JSON: %v\n", err)
	}
	d, ok := v.(map[string]interface{})
	if !ok {
		return "set-evidence: top-level JSON must be an object\n"
	}
	var missing []string
	for _, k := range evidenceReq[kind] {
		if _, present := d[k]; !present {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Sprintf("set-evidence: %s missing field(s): %s\n", kind, strings.Join(missing, ", "))
	}
	return ""
}

// runEvidenceStatus ports evidence-status (tests/fixtures/bbs-ticket.reference:1552-1577): {none|
// valid|malformed} for a typed evidence artifact, by set-evidence's rules.
func runEvidenceStatus(args []string) {
	env := resolveEnv()
	needTicket(env)
	var kind string
	for i := 0; i < len(args); i++ {
		if args[i] == "--kind" {
			kind, i = valueOf(args, i, "--kind"), i+1
		}
	}
	if _, ok := evidenceReq[kind]; !ok {
		fmt.Fprintln(os.Stderr, "evidence-status: --kind must be verification|risk-gate|adversarial")
		os.Exit(2)
	}
	ep := filepath.Join(ticket.New(env).Home(), "evidence", kind, "result.json")
	b, err := os.ReadFile(ep)
	if err != nil {
		fmt.Println("none")
		os.Exit(0)
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		fmt.Println("malformed")
		os.Exit(0)
	}
	d, ok := v.(map[string]interface{})
	if !ok {
		fmt.Println("malformed")
		os.Exit(0)
	}
	for _, k := range evidenceReq[kind] {
		if _, present := d[k]; !present {
			fmt.Println("malformed")
			os.Exit(0)
		}
	}
	fmt.Println("valid")
	os.Exit(0)
}
