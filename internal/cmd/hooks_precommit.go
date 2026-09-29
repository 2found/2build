package cmd

// runPreCommitHook ports the retired pre-commit hook script as `bbs hooks pre-commit`: it
// lints staged workflow .md files (from the index, not the working tree) and
// refuses staged qa.yaml files carrying inline credential literals.
//
// The bash resolved a working `bbs` before linting; here the hook IS bbs —
// `bbs hooks pre-commit` exists by construction, so no resolution step.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var qaSecretRe = regexp.MustCompile(`^[[:space:]]+(password|token|secret|api_key)[[:space:]]*:`)

func runPreCommitHook() {
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(os.Stderr, "pre-commit: git not found — cannot lint staged files.")
		os.Exit(1)
	}

	staged, ok := gitStagedFiles(
		".claude/skills/autopilot/workflows/*.md",
		".claude/workflows/*.md",
	)
	if !ok {
		os.Exit(1) // bash: git diff failure exits the hook outright
	}
	qaStaged, ok := gitStagedFiles(".babysit/qa.yaml", ".babysit/qa.local.yaml")
	if !ok {
		os.Exit(1)
	}

	errors := 0

	// Lint staged workflow files against their index content.
	for _, wf := range staged {
		b, err := exec.Command("git", "show", ":"+wf).Output()
		if err != nil {
			errors++
			continue
		}
		tmp, err := os.CreateTemp("", "bbs-lint-*.md")
		if err != nil {
			errors++
			continue
		}
		if _, err := tmp.Write(b); err != nil {
			errors++
		}
		_ = tmp.Close()
		errors += lintWorkflowFile(tmp.Name())
		_ = os.Remove(tmp.Name())
	}

	// Credential-leak guard on staged qa.yaml files.
	for _, qf := range qaStaged {
		b, err := exec.Command("git", "show", ":"+qf).Output()
		if err != nil {
			continue
		}
		var hits []string
		for i, ln := range strings.Split(string(b), "\n") {
			if qaSecretRe.MatchString(ln) {
				hits = append(hits, fmt.Sprintf("%d:%s", i+1, ln))
			}
		}
		if len(hits) > 0 {
			fmt.Fprintf(os.Stderr, "pre-commit: %s contains inline credential literals (offending lines):\n", qf)
			fmt.Fprintln(os.Stderr, strings.Join(hits, "\n"))
			fmt.Fprintln(os.Stderr, "Use credentials.{username_env,password_env} (env-var names) or move secrets to .babysit/qa.local.yaml (gitignored).")
			errors++
		}
	}

	if errors > 0 {
		fmt.Fprintf(os.Stderr, "\npre-commit: %d lint error(s) — fix before committing.\n", errors)
		os.Exit(1)
	}
}

// gitStagedFiles is `git diff --cached --name-only --diff-filter=ACM` over
// the given pathspecs; ok=false when the index cannot be read at all.
func gitStagedFiles(pathspecs ...string) ([]string, bool) {
	args := append([]string{"diff", "--cached", "--name-only", "--diff-filter=ACM", "--"}, pathspecs...)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pre-commit: git diff --cached failed — cannot enumerate staged files")
		return nil, false
	}
	var files []string
	for _, ln := range strings.Split(string(out), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			files = append(files, filepath.ToSlash(ln))
		}
	}
	return files, true
}
