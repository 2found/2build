package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func installTestFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestInstallMarketplace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs")
	}
	bin := buildTestBBS(t)
	for _, harness := range []string{"claude", "codex"} {
		for _, registered := range []bool{false, true} {
			t.Run(harness+"/registered="+map[bool]string{false: "false", true: "true"}[registered], func(t *testing.T) {
				dir := t.TempDir()
				list := "[]"
				if registered {
					list = `[{"name":"babysit"}]`
				}
				if harness == "codex" {
					list = `{"marketplaces":` + list + `}`
				}
				installTestFile(t, filepath.Join(dir, "bin", harness), "#!/bin/sh\necho \"$*\" >> \"$HOME/calls\"\nif [ \"$*\" = 'plugin marketplace list --json' ]; then\n  echo '"+list+"'\nfi\n", 0o755)
				command := exec.Command(bin, "install", harness)
				command.Dir = dir
				command.Env = []string{"HOME=" + dir, "PATH=" + filepath.Join(dir, "bin")}
				out, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("install: %v: %s", err, out)
				}
				refresh, verb := "update", "install"
				if harness == "codex" {
					refresh, verb = "upgrade", "add"
				}
				market := "plugin marketplace add lohi-ai/babysit"
				if registered {
					market = "plugin marketplace " + refresh + " babysit"
				}
				calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
				want := "plugin marketplace list --json\n" + market + "\nplugin " + verb + " bbs@babysit\n"
				if string(calls) != want {
					t.Fatalf("calls = %s, want %s", calls, want)
				}
			})
		}
	}
}

func TestInstallDetectionAndPartialFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs")
	}
	bin := buildTestBBS(t)
	dir := t.TempDir()
	installTestFile(t, filepath.Join(dir, "bin", "claude"), "#!/bin/sh\necho 'authentication failed' >&2\nexit 1\n", 0o755)
	installTestFile(t, filepath.Join(dir, "bin", "codex"), "#!/bin/sh\nif [ \"$*\" = 'plugin marketplace list --json' ]; then echo '{\"marketplaces\":[]}'; fi\n", 0o755)
	command := exec.Command(bin, "install")
	command.Dir = dir
	command.Env = []string{"HOME=" + dir, "PATH=" + filepath.Join(dir, "bin")}
	out, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "authentication failed") || !strings.Contains(string(out), "installed for codex") || strings.Contains(string(out), "installed for claude") {
		t.Fatalf("partial failure lost: %v: %s", err, out)
	}
}

func TestInstallIgnoresLeftoverConfigDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs")
	}
	bin := buildTestBBS(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	installTestFile(t, filepath.Join(dir, "bin", "codex"), "#!/bin/sh\necho \"$*\" >> \"$HOME/calls\"\nif [ \"$*\" = 'plugin marketplace list --json' ]; then echo '{\"marketplaces\":[]}'; fi\n", 0o755)
	command := exec.Command(bin, "install")
	command.Dir = dir
	command.Env = []string{"HOME=" + dir, "PATH=" + filepath.Join(dir, "bin")}
	out, err := command.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "installed for codex") {
		t.Fatalf("install: %v: %s", err, out)
	}
	if strings.Contains(string(out), "claude") {
		t.Fatalf("leftover ~/.claude treated as a harness: %s", out)
	}
}

func TestInstalledHarnessesIgnoresLeftoverDirs(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{".claude", ".codex", ".gemini/config"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(p)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	for _, name := range installedHarnesses(dir) {
		if name == "claude" || name == "codex" {
			t.Fatalf("leftover config dir detected as %s", name)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, ".gemini", "antigravity"), 0o755); err != nil {
		t.Fatal(err)
	}
	found := installedHarnesses(dir)
	for _, name := range found {
		if name == "antigravity" {
			return
		}
	}
	t.Fatalf("missing antigravity for .gemini/antigravity: %v", found)
}

func TestInstallMissingCLI(t *testing.T) {
	bin := buildTestBBS(t)
	dir := t.TempDir()
	command := exec.Command(bin, "install", "codex")
	command.Dir = dir
	command.Env = []string{"HOME=" + dir, "PATH=" + dir}
	out, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "codex CLI is not on PATH") {
		t.Fatalf("missing CLI: %v: %s", err, out)
	}
}

func TestInstallAntigravityPreservesReferencesAndUnownedFiles(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	dst := filepath.Join(dir, "plugins", "bbs")
	installTestFile(t, filepath.Join(source, "autopilot", "SKILL.md"), "../references/preamble.md", 0o644)
	installTestFile(t, filepath.Join(source, "references", "preamble.md"), "bootstrap", 0o644)
	installTestFile(t, filepath.Join(source, "autopilot", "scripts", "check"), "#!/bin/sh\n", 0o755)
	for i := 0; i < 2; i++ {
		if err := installAntigravityBundle(source, dst); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(dst, "skills", "autopilot", "..", "references", "preamble.md"))
		if err != nil || string(body) != "bootstrap" {
			t.Fatalf("reference broken: %s, %v", body, err)
		}
		info, err := os.Stat(filepath.Join(dst, "skills", "autopilot", "scripts", "check"))
		if err != nil || (runtime.GOOS != "windows" && info.Mode()&0o111 == 0) {
			t.Fatalf("script lost executable mode: %v", err)
		}
	}
	// Refresh replaces only our bundle, including removing obsolete files.
	installTestFile(t, filepath.Join(dst, "skills", "obsolete", "SKILL.md"), "old", 0o644)
	if err := installAntigravityBundle(source, dst); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(dst, "skills", "obsolete", "SKILL.md")) {
		t.Fatal("obsolete skill survived reinstall")
	}
	if err := os.Remove(filepath.Join(dst, antigravityInstallMarker)); err != nil {
		t.Fatal(err)
	}
	if err := installAntigravityBundle(source, dst); err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatalf("unowned plugin replaced: %v", err)
	}
	if !fileExists(filepath.Join(dst, "skills", "autopilot", "SKILL.md")) {
		t.Fatal("unowned files removed")
	}
}

func TestUpgradeAntigravityUpdatesOnlyManagedSurfaces(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	installTestFile(t, filepath.Join(source, "autopilot", "SKILL.md"), "fresh", 0o644)
	installTestFile(t, filepath.Join(source, "references", "preamble.md"), "ref", 0o644)
	cli := filepath.Join(dir, ".gemini", "antigravity-cli", "plugins", "bbs")
	if err := installAntigravityBundle(source, cli); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, ".gemini", "config", "plugins", "bbs")
	installTestFile(t, filepath.Join(config, "skills", "unmanaged", "SKILL.md"), "keep", 0o644)
	checkout := filepath.Join(dir, "checkout")
	installTestFile(t, filepath.Join(checkout, "VERSION"), "1.2.3", 0o644)
	installTestFile(t, filepath.Join(checkout, ".claude", "skills", "autopilot", "SKILL.md"), "updated", 0o644)
	installTestFile(t, filepath.Join(checkout, ".claude", "skills", "references", "preamble.md"), "ref", 0o644)
	t.Setenv("HOME", dir)
	t.Setenv("BABYSIT_DIR", checkout)
	updated, err := upgradeAntigravityPlugin()
	if err != nil || !updated {
		t.Fatalf("upgrade: updated=%v err=%v", updated, err)
	}
	body, err := os.ReadFile(filepath.Join(cli, "skills", "autopilot", "SKILL.md"))
	if err != nil || string(body) != "updated" {
		t.Fatalf("managed surface not refreshed: %s, %v", body, err)
	}
	kept, err := os.ReadFile(filepath.Join(config, "skills", "unmanaged", "SKILL.md"))
	if err != nil || string(kept) != "keep" {
		t.Fatalf("unmanaged surface replaced: %s, %v", kept, err)
	}
}

func TestInstallAntigravityFromCheckout(t *testing.T) {
	bin := buildTestBBS(t)
	dir := t.TempDir()
	checkout := filepath.Join(dir, "checkout")
	installTestFile(t, filepath.Join(checkout, "VERSION"), "1.2.3", 0o644)
	installTestFile(t, filepath.Join(checkout, ".claude", "skills", "autopilot", "SKILL.md"), "skill", 0o644)
	installTestFile(t, filepath.Join(checkout, ".claude", "skills", "references", "preamble.md"), "ref", 0o644)
	if err := os.MkdirAll(filepath.Join(dir, ".gemini", "antigravity-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(bin, "install", "antigravity")
	command.Dir = dir
	command.Env = []string{"HOME=" + dir, "PATH=" + dir, "BABYSIT_DIR=" + checkout}
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("install antigravity: %v: %s", err, out)
	}
	for _, surface := range []string{"config", "antigravity-cli"} {
		if !fileExists(filepath.Join(dir, ".gemini", surface, "plugins", "bbs", "skills", "autopilot", "SKILL.md")) {
			t.Fatalf("no skills for %s: %s", surface, out)
		}
	}
}

func TestInstallAntigravityDownloadFailurePreservesInstalledBundle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs")
	}
	bin := buildTestBBS(t)
	dir := t.TempDir()
	release := filepath.Join(dir, "release")
	installTestFile(t, filepath.Join(release, "VERSION"), "1.2.3", 0o644)
	installTestFile(t, filepath.Join(dir, "bin", "git"), `#!/bin/sh
echo "$*" >> "$HOME/calls"
if [ "$DOWNLOAD_FAIL" = 1 ]; then echo 'network unavailable' >&2; exit 1; fi
for arg do destination="$arg"; done
/bin/mkdir -p "$destination/.claude/skills/autopilot" "$destination/.claude/skills/references"
echo 'skill from release' > "$destination/.claude/skills/autopilot/SKILL.md"
echo 'reference' > "$destination/.claude/skills/references/preamble.md"
`, 0o755)
	for _, fail := range []string{"0", "1"} {
		command := exec.Command(bin, "install", "antigravity")
		command.Dir = dir
		command.Env = []string{"HOME=" + dir, "PATH=" + filepath.Join(dir, "bin"), "BABYSIT_DIR=" + release, "DOWNLOAD_FAIL=" + fail}
		out, err := command.CombinedOutput()
		if (err != nil) != (fail == "1") {
			t.Fatalf("download fail=%s: %v: %s", fail, err, out)
		}
		body, err := os.ReadFile(filepath.Join(dir, ".gemini", "config", "plugins", "bbs", "skills", "autopilot", "SKILL.md"))
		if err != nil || string(body) != "skill from release\n" {
			t.Fatalf("installed bundle lost: %s, %v", body, err)
		}
	}
	calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
	if !strings.Contains(string(calls), "clone --depth 1 --branch v1.2.3 https://github.com/lohi-ai/babysit.git") {
		t.Fatalf("download was not pinned to CLI version: %s", calls)
	}
}

func TestInstallMarketplaceFailureStopsBeforePluginMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs")
	}
	bin := buildTestBBS(t)
	for _, failure := range []string{"invalid-json", "marketplace-add"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			installTestFile(t, filepath.Join(dir, "bin", "claude"), `#!/bin/sh
echo "$*" >> "$HOME/calls"
if [ "$3" = list ]; then
  if [ "$INSTALL_FAILURE" = invalid-json ]; then echo 'not json'; else echo '[]'; fi
elif [ "$3" = add ]; then
  echo 'marketplace unavailable' >&2
  exit 1
fi
`, 0o755)
			command := exec.Command(bin, "install", "claude")
			command.Dir = dir
			command.Env = []string{"HOME=" + dir, "PATH=" + filepath.Join(dir, "bin"), "INSTALL_FAILURE=" + failure}
			out, err := command.CombinedOutput()
			if err == nil || strings.Contains(string(out), "✓") {
				t.Fatalf("claimed success: %v: %s", err, out)
			}
			calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
			if strings.Contains(string(calls), "plugin install") {
				t.Fatalf("installed despite failed marketplace: %s", calls)
			}
		})
	}
}

func TestUpgradeAdditionalHomebrewCopy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs")
	}
	for _, scenario := range []string{"absent", "installed", "failed"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PATH", dir)
			t.Setenv("HOME", dir)
			t.Setenv("BREW_SCENARIO", scenario)
			installTestFile(t, filepath.Join(dir, "brew"), `#!/bin/sh
echo "$*" >> "$HOME/calls"
if [ "$1" = list ]; then
  [ "$BREW_SCENARIO" = absent ] && exit 1
  echo 'bbs 1.80.8'
elif [ "$BREW_SCENARIO" = failed ]; then
  [ "$1" = update ] && exit 0
  exit 1
fi
`, 0o755)
			updated, err := upgradeHomebrewCopy()
			if (err != nil) != (scenario == "failed") {
				t.Fatalf("%s: %v", scenario, err)
			}
			if updated != (scenario == "installed") {
				t.Fatalf("%s: updated=%v", scenario, updated)
			}
			calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
			upgraded := strings.Contains(string(calls), "upgrade bbs")
			if upgraded != (scenario != "absent") {
				t.Fatalf("wrong Homebrew action: %s", calls)
			}
			if scenario == "installed" && !strings.Contains(string(calls), "update\n") {
				t.Fatalf("tap not refreshed before upgrade: %s", calls)
			}
		})
	}
}
