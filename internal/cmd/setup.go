package cmd

// newSetupCmd ports the retired setup-skills script as `bbs setup` — build the multicall
// binary at the checkout root and link it into ~/.claude/ and ~/.local/bin/.
// Skills themselves are installed by the agent's plugin system; this command
// only wires the CLI half, plus the repo's pre-commit hook.
//
// Layout change: the bash built into a bin/ directory; the port builds ./bbs
// at the checkout root. babysitDir() accepts either shape.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "setup [--dry-run] [--full] [--uninstall]",
		Short:              "build bbs into the checkout and link it onto PATH",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return runSetup(setupCtx{args: args, out: os.Stdout, err: os.Stderr})
		},
	}
}

const setupUsage = `Usage: bbs setup [--dry-run] [--full] [--uninstall]

Install the babysit CLI: build ./bbs, symlink it into ~/.claude/ and
~/.local/bin/, and install the repo's pre-commit hook. Skills install
separately via a coding-agent plugin system:

  /plugin marketplace add <path-to-this-repo>
  /plugin install bbs@babysit
or:
  codex plugin marketplace add <path-to-this-repo>
  codex plugin add bbs@babysit

  --dry-run    Show what would be done without making changes
  --full       Run all steps + print the plugin registration commands
  --uninstall  Remove all installed bin symlinks
`

// Legacy argv0 aliases ever linked into ~/.claude/ by earlier installers.
// Swept on both install and uninstall — they dangle otherwise.
var setupStaleAliases = []string{
	"bbs-autopilot", "bbs-config", "bbs-dashboard", "bbs-design", "bbs-env",
	"bbs-qa-config", "bbs-secrets", "bbs-slug", "bbs-ticket", "bbs-update",
	"bbs-update-check", "bbs-upgrade",
}

// The uninstall sweep additionally removes these legacy bin names.
var setupUninstallBins = []string{
	"bbs", "babysit", "bbs-config", "bbs-dashboard", "bbs-design", "bbs-env",
	"bbs-learnings-log", "bbs-learnings-search", "bbs-autopilot", "bbs-pipeline",
	"bbs-formula", "bbs-run", "bbs-slug", "bbs-telemetry-log", "bbs-ticket",
	"bbs-qa-config", "bbs-secrets", "bbs-update-check", "bbs-update",
	"bbs-upgrade", "bbs-codex-competitive", "bbs-analytics-cron",
}

type setupCtx struct {
	args       []string
	out, err   io.Writer
	dryRun     bool
	full       bool
	uninstall  bool
	projectDir string
	skillsDir  string // <project>/.claude/skills
	globalDir  string // ~/.claude
	localBin   string // ~/.local/bin
	installed  []string
	failed     error // first action failure — propagated to the exit code
}

func (c *setupCtx) info(format string, a ...any) { fmt.Fprintf(c.out, "→ "+format+"\n", a...) }
func (c *setupCtx) ok(format string, a ...any)   { fmt.Fprintf(c.out, "✓ "+format+"\n", a...) }
func (c *setupCtx) warn(format string, a ...any) { fmt.Fprintf(c.out, "! "+format+"\n", a...) }
func (c *setupCtx) fail(format string, a ...any) {
	fmt.Fprintf(c.err, "✗ "+format+"\n", a...)
	os.Exit(1)
}

// run performs or narrates an action under --dry-run. The bash original ran
// under `set -e` — the first failure aborted the install; the port records
// it and propagates through doInstall/doUninstall so `bbs setup` (and `bbs
// update`'s in-process call) exits non-zero instead of reporting success.
func (c *setupCtx) run(desc string, f func() error) {
	if c.dryRun {
		fmt.Fprintf(c.out, "  [dry-run] %s\n", desc)
		return
	}
	if err := f(); err != nil {
		fmt.Fprintf(c.err, "%s: %v\n", desc, err)
		if c.failed == nil {
			c.failed = fmt.Errorf("%s: %w", desc, err)
		}
	}
}

func (c *setupCtx) rm(path string) { c.run("rm "+path, func() error { return os.Remove(path) }) }
func (c *setupCtx) ln(oldname, dst string) {
	c.run(fmt.Sprintf("ln -s %s %s", oldname, dst), func() error { return linkOrCopy(oldname, dst) })
}

// linkOrCopy mirrors `ln -sf`: create dst as a symlink, replacing whatever
// sits there. On Windows (where symlinks need privileges the bash port
// sidestepped by copying) it falls back to copying the file.
func linkOrCopy(oldname, dst string) error {
	if runtime.GOOS == "windows" {
		src, err := os.ReadFile(oldname)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, src, 0o755)
	}
	_ = os.Remove(dst) // ln -sf replaces a stale link; a missing dst is fine
	return os.Symlink(oldname, dst)
}

// setupBinaryPath returns the path the running binary resolves to, used to
// find the checkout when setup runs from an installed link.
func setupBinaryPath() string {
	argv0 := os.Args[0]
	if !strings.ContainsRune(argv0, filepath.Separator) {
		if p, err := exec.LookPath(argv0); err == nil {
			argv0 = p
		}
	}
	if resolved, err := filepath.EvalSymlinks(argv0); err == nil {
		return resolved
	}
	return argv0
}

// setupProjectDir finds the checkout: BABYSIT_DIR first, then the directory
// holding the resolved binary (a checkout-root ./bbs, a legacy in-tree
// bin-dir build, or a skills-dir copy), else cwd.
func setupProjectDir() string {
	if d := os.Getenv("BABYSIT_DIR"); d != "" {
		return d
	}
	dir := filepath.Dir(setupBinaryPath())
	for _, cand := range []string{dir, filepath.Dir(dir)} {
		if isDir(filepath.Join(cand, ".claude", "skills")) {
			return cand
		}
	}
	if wd, err := os.Getwd(); err == nil && isDir(filepath.Join(wd, ".claude", "skills")) {
		return wd
	}
	return dir
}

func runSetup(c setupCtx) error {
	for _, a := range c.args {
		switch a {
		case "--dry-run":
			c.dryRun = true
		case "--full":
			c.full = true
		case "--uninstall":
			c.uninstall = true
		case "--help", "-h":
			fmt.Fprint(c.out, setupUsage)
			return nil
		default:
			c.warn("Unknown option: %s", a)
		}
	}
	if c.projectDir == "" {
		c.projectDir = setupProjectDir()
	}
	if abs, err := filepath.Abs(c.projectDir); err == nil {
		c.projectDir = abs
	}
	c.skillsDir = filepath.Join(c.projectDir, ".claude", "skills")
	home, _ := os.UserHomeDir()
	c.globalDir = filepath.Join(home, ".claude")
	c.localBin = filepath.Join(home, ".local", "bin")

	if c.uninstall {
		return c.doUninstall()
	}
	return c.doInstall()
}

// ─── uninstall ─────────────────────────────────────────────────────────────

func (c *setupCtx) doUninstall() error {
	c.info("Uninstalling Babysit bins...")

	// Legacy skill symlinks from pre-plugin installs (~/.claude/skills/bbs:*).
	if entries, err := os.ReadDir(filepath.Join(c.globalDir, "skills")); err == nil {
		for _, e := range entries {
			p := filepath.Join(c.globalDir, "skills", e.Name())
			if strings.HasPrefix(e.Name(), "bbs:") && isSymlink(p) {
				c.rm(p)
				c.ok("Removed legacy symlink %s", e.Name())
			}
		}
	}
	if isSymlink(filepath.Join(c.globalDir, "env-resolve")) {
		c.rm(filepath.Join(c.globalDir, "env-resolve"))
		c.ok("Removed env-resolve (deprecated)")
	}
	if existsOrSymlink(filepath.Join(c.localBin, "bbs")) {
		c.rm(filepath.Join(c.localBin, "bbs"))
		c.ok("Removed bbs from ~/.local/bin/")
	}
	if fileExists(filepath.Join(c.localBin, "babysit")) {
		c.rm(filepath.Join(c.localBin, "babysit"))
		c.ok("Removed babysit shim (legacy) from ~/.local/bin/")
	}
	for _, name := range setupUninstallBins {
		p := filepath.Join(c.globalDir, name)
		if isSymlink(p) {
			c.rm(p)
			c.ok("Removed %s", name)
		}
	}

	c.uninstallPreCommitHook()

	// Legacy Codex skill symlinks pointing at this checkout or the old
	// skills-dir shape.
	codexSkills := filepath.Join(filepath.Dir(c.globalDir), ".codex", "skills")
	if entries, err := os.ReadDir(codexSkills); err == nil {
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), "bbs:") {
				continue
			}
			p := filepath.Join(codexSkills, e.Name())
			target, err := os.Readlink(p)
			if err != nil {
				continue
			}
			old := filepath.Join(c.globalDir, "skills", "babysit", ".claude", "skills")
			if strings.HasPrefix(target, old+string(filepath.Separator)) ||
				strings.HasPrefix(target, c.skillsDir+string(filepath.Separator)) {
				c.rm(p)
				c.ok("Removed legacy Codex symlink %s", e.Name())
			}
		}
	}

	fmt.Fprintln(c.out, "\nUninstall complete.")
	fmt.Fprintln(c.out, "Skills installed via a plugin system must be uninstalled in that agent.")
	return c.failed
}

// uninstallPreCommitHook removes our hook: a symlink into a checkout's old
// hooks dir (legacy installs), or a file we own (marker match).
func (c *setupCtx) uninstallPreCommitHook() {
	hook := filepath.Join(c.projectDir, ".git", "hooks", "pre-commit")
	if isSymlink(hook) {
		target, err := os.Readlink(hook)
		if err == nil && strings.HasSuffix(target, "bin/hooks/pre-commit") {
			c.rm(hook)
			c.ok("Removed pre-commit hook (symlink)")
		}
		return
	}
	if !fileExists(hook) {
		return
	}
	body := readRegular(hook)
	// A pure shim file is ours outright — remove it.
	if strings.TrimSpace(body) == strings.TrimSpace(preCommitShim) ||
		strings.TrimSpace(body) == "#!/usr/bin/env bash\nexec bbs hooks pre-commit \"$@\"" {
		c.run("rm "+hook, func() error { return os.Remove(hook) })
		c.ok("Removed pre-commit hook (shim)")
		return
	}
	if !strings.Contains(body, "Babysit workflow lint") &&
		!strings.Contains(body, "bin/hooks/pre-commit") &&
		!strings.Contains(body, "hooks pre-commit") {
		return
	}
	out := stripPreCommitFragment(body) + "\n"
	if strings.TrimSpace(out) == "" {
		c.run("rm "+hook, func() error { return os.Remove(hook) })
		c.ok("Removed pre-commit hook")
		return
	}
	c.run("removed babysit fragment from "+hook, func() error {
		return os.WriteFile(hook, []byte(out), 0o755)
	})
	c.ok("Removed babysit fragment from pre-commit hook")
}

// ─── install ───────────────────────────────────────────────────────────────

func (c *setupCtx) doInstall() error {
	c.info("Validating project structure...")

	if !isDir(c.skillsDir) {
		c.fail(".claude/skills/ not found at %s — run from the checkout", c.skillsDir)
	}
	if !fileExists(filepath.Join(c.projectDir, ".claude-plugin", "plugin.json")) {
		c.warn(".claude-plugin/plugin.json missing — plugin install won't work")
	}
	skillNames := c.skillNames()
	c.ok("Found %d skills in project (managed by plugin system — see plugin.json)", len(skillNames))

	c.run("mkdir -p "+c.globalDir, func() error { return os.MkdirAll(c.globalDir, 0o755) })

	// Stale bbs-<sub> aliases from installs that predate the space form.
	for _, alias := range setupStaleAliases {
		p := filepath.Join(c.globalDir, alias)
		if isSymlink(p) {
			c.rm(p)
			c.ok("Removed stale %s alias", alias)
		}
	}
	c.sweepLegacySkillLinks(filepath.Join(c.globalDir, "skills"), true)
	c.sweepLegacySkillLinks(filepath.Join(filepath.Dir(c.globalDir), ".codex", "skills"), false)

	// Build the multicall binary at the checkout root.
	c.info("Building bbs Go CLI...")
	binPath := filepath.Join(c.projectDir, "bbs")
	if _, err := exec.LookPath("go"); err != nil {
		c.warn("go not found — skipping bbs build (install Go to enable the bbs CLI)")
	} else if c.dryRun {
		fmt.Fprintln(c.out, "  [dry-run] go build -o bbs ./cmd/bbs")
	} else {
		build := exec.Command("go", "build", "-o", binPath, "./cmd/bbs")
		build.Dir = c.projectDir
		build.Stderr = c.err
		if err := build.Run(); err == nil {
			// Some Go versions emit bbs.exe for -o bbs on Windows — keep both
			// names so POSIX and native callers both resolve.
			exe := binPath + ".exe"
			if fileExists(exe) && !fileExists(binPath) {
				if b, err := os.ReadFile(exe); err == nil {
					_ = os.WriteFile(binPath, b, 0o755)
				}
			} else if fileExists(binPath) && runtime.GOOS == "windows" {
				if b, err := os.ReadFile(binPath); err == nil {
					_ = os.WriteFile(exe, b, 0o755)
				}
			}
			c.ok("bbs built → bbs")
		} else {
			c.warn("go build failed — bbs will not resolve until built")
		}
	}

	c.info("Installing babysit bins...")
	c.linkBin("bbs", c.globalDir)

	c.info("Linking bbs onto PATH...")
	// Pre-Go installs left a bun shim here pointing at the deleted babysitter/
	// CLI.
	localBBS := filepath.Join(c.localBin, "bbs")
	if fileExists(localBBS) && !isSymlink(localBBS) &&
		strings.Contains(readRegular(localBBS), "babysitter/src/cli") {
		c.rm(localBBS)
		c.warn("Removed legacy bun shim from ~/.local/bin/bbs")
	}
	if !isDir(c.localBin) {
		c.run("mkdir -p "+c.localBin, func() error { return os.MkdirAll(c.localBin, 0o755) })
	}
	c.linkBin("bbs", c.localBin)

	if !pathHasDir(os.Getenv("PATH"), c.localBin) {
		c.warn("~/.local/bin is not on your PATH — add it to run `bbs` from a shell")
	}

	c.validateSkills(skillNames)
	c.installPreCommitHook()

	if c.full {
		fmt.Fprintf(c.out, `
Post-install: register the plugin
Claude Code:

  /plugin marketplace add %[1]s
  /plugin install bbs@babysit

Codex CLI:

  codex plugin marketplace add %[1]s
  codex plugin add bbs@babysit

Grok Build:
  grok plugin install https://github.com/lohi-ai/babysit

OMP hooks (skills are configured separately; see docs/operations.md):
  omp --extension "%[1]s/hooks/omp.ts"

Hook prerequisite: bbs on PATH (or bbs in ~/.local/bin).
`, c.projectDir)
	}

	fmt.Fprint(c.out, "\nBabysit bins installed.\n\n")
	fmt.Fprintf(c.out, "Skills:      %d in .claude/skills/ — register in Claude Code:\n", len(skillNames))
	fmt.Fprintf(c.out, "               /plugin marketplace add %s\n", c.projectDir)
	fmt.Fprintln(c.out, "               /plugin install bbs@babysit")
	fmt.Fprintln(c.out, "             or Codex CLI:")
	fmt.Fprintf(c.out, "               codex plugin marketplace add %s\n", c.projectDir)
	fmt.Fprintln(c.out, "               codex plugin add bbs@babysit")
	if len(c.installed) > 0 {
		sort.Strings(c.installed)
		fmt.Fprintf(c.out, "Bins:        ~/.claude/{%s}\n", strings.Join(c.installed, ","))
	} else {
		fmt.Fprintln(c.out, "Bins:        none linked")
	}
	if isSymlink(localBBS) {
		fmt.Fprintln(c.out, "PATH:        ~/.local/bin/bbs")
	}
	fmt.Fprintln(c.out, "References:  .claude/skills/references/")
	fmt.Fprintln(c.out, "\nTo uninstall: bbs setup --uninstall")
	return c.failed
}

func (c *setupCtx) skillNames() []string {
	var names []string
	if entries, err := os.ReadDir(c.skillsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "references" || e.Name() == "shared" ||
				strings.HasPrefix(e.Name(), ".") {
				continue
			}
			names = append(names, e.Name())
		}
	}
	return names
}

// sweepLegacySkillLinks removes pre-plugin bbs:* symlinks. Under ~/.claude
// every bbs: symlink goes; under ~/.codex only links pointing at this
// checkout or the old skills-dir shape.
func (c *setupCtx) sweepLegacySkillLinks(dir string, claude bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	n := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "bbs:") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if !isSymlink(p) {
			continue
		}
		if !claude {
			target, err := os.Readlink(p)
			if err != nil {
				continue
			}
			old := filepath.Join(c.globalDir, "skills", "babysit", ".claude", "skills")
			if !strings.HasPrefix(target, old+string(filepath.Separator)) &&
				!strings.HasPrefix(target, c.skillsDir+string(filepath.Separator)) {
				continue
			}
		}
		c.rm(p)
		n++
	}
	if n > 0 {
		c.warn("Removed %d legacy skill symlink(s) — plugin system owns skill registration now", n)
	}
}

// linkBin ports link_bin: src is <project>/<name>, dst under dstDir.
func (c *setupCtx) linkBin(name, dstDir string) {
	src := filepath.Join(c.projectDir, name)
	dst := filepath.Join(dstDir, name)
	track := dstDir == c.globalDir

	if !fileExists(src) {
		c.warn("%s not found in the checkout", name)
		return
	}
	if isSymlink(dst) {
		if cur, err := os.Readlink(dst); err == nil && cur == src {
			c.ok("%s up-to-date", name)
			if track {
				c.installed = append(c.installed, name)
			}
			return
		}
		c.ln(src, dst)
		c.warn("%s re-pointed", name)
		if track {
			c.installed = append(c.installed, name)
		}
		return
	}
	if fileExists(dst) {
		c.warn("%s exists and is not a symlink — skipping (remove manually to replace)", name)
		return
	}
	c.ln(src, dst)
	c.ok("%s → %s", name, dst)
	if track {
		c.installed = append(c.installed, name)
	}
}

// validateSkills ports Step 3: every skill dir needs SKILL.md and the shared
// references must exist.
func (c *setupCtx) validateSkills(skillNames []string) {
	c.info("Validating skills...")
	errors := 0
	for _, name := range skillNames {
		if !fileExists(filepath.Join(c.skillsDir, name, "SKILL.md")) {
			c.warn("%s: missing SKILL.md", name)
			errors++
		}
	}
	for _, ref := range []string{"preamble.md", "auto-decision-framework.md", "handoff-contracts.md"} {
		if !fileExists(filepath.Join(c.skillsDir, "references", ref)) {
			c.warn("references/%s missing", ref)
			errors++
		}
	}
	if errors == 0 {
		c.ok("All skills valid")
	} else {
		c.warn("%d validation issue(s) found", errors)
	}
}

// The pre-commit hook is a shim that resolves a working bbs like the bash
// hook's _bbs_resolve did: PATH first, then the two install locations (a GUI
// git client may lack ~/.local/bin). Probe with `hooks --help` — running the
// hook as the probe would conflate a lint failure with a broken binary.
// Fails closed: a commit with no working bbs errors rather than skipping the
// lint, matching the deleted bin/hooks/pre-commit.
const preCommitResolve = `BBS=""
for _c in bbs "$HOME/.local/bin/bbs" "$HOME/.claude/bbs"; do
  if command -v "$_c" >/dev/null 2>&1 && "$_c" hooks --help >/dev/null 2>&1; then
    BBS="$_c"
    break
  fi
done
if [ -z "$BBS" ]; then
  echo "pre-commit: no working bbs — run 'bbs setup' from a checkout, or brew install lohi-ai/babysit/bbs." >&2
  exit 1
fi
"$BBS" hooks pre-commit "$@"
`

// preCommitShim is the whole-file hook we install; preCommitFragment is the
// marked block appended into a pre-existing hook. Same resolver either way —
// install paths must not produce different lint contracts.
const preCommitShim = "#!/usr/bin/env bash\n" + preCommitResolve
const preCommitMarker = "# ── Babysit workflow lint ────────────────────────────"
const preCommitFragment = preCommitMarker + "\n" + preCommitResolve

// stripPreCommitFragment removes every generated artifact — the appended
// fragment (any era: current resolver shim, `command -v bbs` guard, legacy
// bin/hooks/pre-commit script execs) — while keeping every other line.
func stripPreCommitFragment(body string) string {
	shimLines := map[string]bool{}
	for _, ln := range strings.Split(preCommitResolve, "\n") {
		shimLines[ln] = true
	}
	var kept []string
	inLegacyIf := false
	for _, ln := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(ln)
		switch {
		case strings.Contains(ln, "Babysit workflow lint"):
			inLegacyIf = true // the legacy fragment was marker + if…fi
		case shimLines[ln]:
			// exact generated resolver line (if/for/then/done/fi included —
			// they are verbatim shim text)
		case inLegacyIf:
			if trim == "fi" {
				inLegacyIf = false
			}
		case strings.Contains(ln, "bin/hooks/pre-commit"),
			strings.Contains(ln, "bbs hooks pre-commit"),
			strings.Contains(ln, "command -v bbs "):
			// legacy exec lines and the earlier `if command -v bbs` guard
		default:
			kept = append(kept, ln)
		}
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// installPreCommitHook writes the hook the bash either symlinked or appended.
// The port installs a shim that execs `bbs hooks pre-commit` — no tracked
// script to point at.
func (c *setupCtx) installPreCommitHook() {
	c.info("Installing pre-commit hook...")
	hooksDir := filepath.Join(c.projectDir, ".git", "hooks")
	hook := filepath.Join(hooksDir, "pre-commit")
	if !isDir(hooksDir) {
		c.warn(".git/hooks/ not found — skipping hook install")
		return
	}
	writeShim := func() {
		c.run("write "+hook, func() error {
			// os.WriteFile follows a symlink — remove it first so the shim
			// replaces the link instead of scribbling on its target.
			if isSymlink(hook) {
				_ = os.Remove(hook)
			}
			return os.WriteFile(hook, []byte(preCommitShim), 0o755)
		})
		c.ok("pre-commit hook → .git/hooks/pre-commit")
	}
	if isSymlink(hook) {
		target, _ := os.Readlink(hook)
		// Replace links into a checkout's old hooks script; leave
		// anything else alone.
		if strings.HasSuffix(target, "bin/hooks/pre-commit") {
			writeShim()
		} else {
			c.warn("pre-commit hook is a symlink to %s — skipping", target)
		}
		return
	}
	if !fileExists(hook) {
		writeShim()
		return
	}
	body := readRegular(hook)
	if strings.Contains(body, "hooks pre-commit") {
		// Our shim or fragment (current or earlier-era) is already wired —
		// every generated form resolves through `bbs hooks pre-commit`.
		c.ok("pre-commit hook already invokes babysit lint")
		return
	}
	if strings.Contains(body, "Babysit workflow lint") ||
		strings.Contains(body, "bin/hooks/pre-commit") {
		// A stale generated artifact lives in this hook — replace just that
		// fragment, keeping everything else the user wrote.
		cleaned := stripPreCommitFragment(body)
		if strings.TrimSpace(cleaned) == "" {
			writeShim()
			return
		}
		c.warn("pre-commit hook contains a stale babysit fragment — replacing it")
		body = cleaned
	}
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += "\n" + preCommitFragment + "\n"
	c.run("append babysit lint to "+hook, func() error {
		return os.WriteFile(hook, []byte(body), 0o755)
	})
	c.ok("pre-commit hook appended (babysit workflow lint)")
}

// ─── small predicates ──────────────────────────────────────────────────────

func isSymlink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func existsOrSymlink(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// pathHasDir is the bash case ":$PATH:" match: an exact component, not a
// substring.
func pathHasDir(pathEnv, dir string) bool {
	for _, comp := range filepath.SplitList(pathEnv) {
		if comp == dir {
			return true
		}
	}
	return false
}
