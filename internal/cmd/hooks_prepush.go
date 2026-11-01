package cmd

// Team lands a phase commit on local base before its QA, so parallel work
// builds on it at once. A human `git push` of base in between would publish
// unverified work: land records each landed commit, and `bbs hooks pre-push`
// refuses a push that carries one whose phase QA has not passed. A passed
// phase drops its record; `git push --no-verify` overrides.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/2found/2build/internal/ticket"
)

// Only an explicit refusal (exit 1) stops the push; a missing or older bbs
// never does.
const prePushScript = "#!/bin/sh\n# bbs: refuse pushing a landed Team phase before its QA passed\ncommand -v bbs >/dev/null 2>&1 || exit 0\nbbs hooks pre-push \"$@\"\n[ $? -eq 1 ] && exit 1\nexit 0\n"

// recordLandedPhase marks commit as landed-but-unverified and installs the
// pre-push hook unless the repository already has its own.
func recordLandedPhase(canonical, commit, parent, phase string) {
	common := gitOutIn(canonical, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if common == "" {
		return
	}
	dir := filepath.Join(common, "bbs-landed")
	if os.MkdirAll(dir, 0o755) != nil || os.WriteFile(filepath.Join(dir, commit), []byte(parent+" "+phase+"\n"), 0o644) != nil {
		return
	}
	hooks := gitOutIn(canonical, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if hooks == "" {
		return
	}
	path := filepath.Join(hooks, "pre-push")
	if b, err := os.ReadFile(path); err == nil {
		if !strings.Contains(string(b), "bbs hooks pre-push") {
			fmt.Fprintln(os.Stderr, "NOTE=pre-push hook exists in "+canonical+"; add `bbs hooks pre-push \"$@\"` to it so base is not pushed before phase QA")
		}
		return
	}
	if os.MkdirAll(hooks, 0o755) == nil {
		_ = os.WriteFile(path, []byte(prePushScript), 0o755)
	}
}

// prePushPhaseVerified reports whether a phase passed its QA. Tests replace it.
var prePushPhaseVerified = func(parent, phase string) (bool, error) {
	env, err := ticket.ResolveProject()
	if err != nil {
		return false, err
	}
	s, err := projectRead(storeForTicket(env, parent))
	if err != nil {
		return false, err
	}
	for _, p := range s.Phases {
		if p.ID == phase {
			return p.Integrated, nil
		}
	}
	return true, nil
}

// runPrePushHook reads git's pre-push lines (<local ref> <local sha> <remote
// ref> <remote sha>) and returns the exit code.
func runPrePushHook(dir string, stdin io.Reader, stderr io.Writer) int {
	common := gitOutIn(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	marks, _ := filepath.Glob(filepath.Join(common, "bbs-landed", "*"))
	if common == "" || len(marks) == 0 {
		return 0
	}
	var refused []string
	sc := bufio.NewScanner(stdin)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 4 || strings.Trim(f[1], "0") == "" {
			continue
		}
		local, remote := f[1], f[3]
		for _, m := range marks {
			commit := filepath.Base(m)
			if !gitOKIn(dir, "merge-base", "--is-ancestor", commit, local) ||
				(strings.Trim(remote, "0") != "" && gitOKIn(dir, "merge-base", "--is-ancestor", commit, remote)) {
				continue
			}
			b, _ := os.ReadFile(m)
			parent, phase, _ := strings.Cut(strings.TrimSpace(string(b)), " ")
			ok, err := prePushPhaseVerified(parent, phase)
			if err != nil {
				fmt.Fprintf(stderr, "bbs pre-push: cannot read %s phase %s (%v); not blocking\n", parent, phase, err)
				continue
			}
			if ok {
				_ = os.Remove(m)
				continue
			}
			refused = append(refused, fmt.Sprintf("%s phase %s (%s)", parent, phase, commit[:min(12, len(commit))]))
		}
	}
	if len(refused) == 0 {
		return 0
	}
	fmt.Fprintln(stderr, "bbs pre-push: refusing to push Team phase work whose QA has not passed: "+strings.Join(refused, ", "))
	fmt.Fprintln(stderr, "  the phase ships with its release once QA passes; `git push --no-verify` pushes anyway")
	return 1
}
