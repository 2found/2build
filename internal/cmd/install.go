package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const babysitMarketplace = "2found/2build"

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "install [claude|codex|antigravity|omp]",
		Short:     "install 2build into one harness, or all detected harnesses",
		Long:      "Install 2build for the current user. With no argument, detect Claude Code, Codex, Antigravity and OMP from PATH, Antigravity config directories, and Codex/Antigravity apps. Claude Code, Codex and OMP require their CLI on PATH. Restart the affected harness after installation.",
		ValidArgs: []string{"claude", "codex", "antigravity", "omp"},
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		RunE: func(_ *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			targets := args
			if len(targets) == 0 {
				targets = installedHarnesses(home)
			}
			if len(targets) == 0 {
				return fmt.Errorf("no supported harness detected — install Claude Code, Codex, Antigravity or OMP, then run 'bbs install'; or specify 'bbs install <harness>'")
			}
			var failures []string
			for _, target := range targets {
				fmt.Printf("→ Installing 2build for %s...\n", target)
				if target == "antigravity" {
					err = installAntigravity(home)
				} else if target == "omp" {
					err = installOmpPlugin()
				} else {
					err = installMarketplacePlugin(target)
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "✗ %s: %v\n", target, err)
					failures = append(failures, target)
					continue
				}
				fmt.Printf("✓ 2build installed for %s. Restart %s to load the skills.\n", target, target)
			}
			if len(failures) > 0 {
				return fmt.Errorf("installation failed for %s — fix the errors above, then retry 'bbs install <harness>'", strings.Join(failures, ", "))
			}
			return nil
		},
	}
}

func installedHarnesses(home string) []string {
	var found []string
	for _, h := range []struct {
		name, app string
		commands  []string
		dirs      []string
	}{
		{"claude", "", []string{"claude"}, nil},
		{"codex", "Codex", []string{"codex"}, nil},
		{"omp", "", []string{"omp"}, nil},
		{"antigravity", "Antigravity", []string{"antigravity", "agy"}, []string{".gemini/antigravity", ".gemini/antigravity-cli"}},
	} {
		detected := false
		for _, bin := range h.commands {
			detected = detected || hasCmd(bin)
		}
		for _, dir := range h.dirs {
			detected = detected || isDir(filepath.Join(home, filepath.FromSlash(dir)))
		}
		for _, dir := range []string{filepath.Join(home, "Applications"), "/Applications"} {
			if h.app != "" {
				detected = detected || isDir(filepath.Join(dir, h.app+".app"))
			}
		}
		if detected {
			found = append(found, h.name)
		}
	}
	return found
}

func installMarketplacePlugin(harness string) error {
	if !hasCmd(harness) {
		return fmt.Errorf("%s CLI is not on PATH — install/enable its CLI, then run 'bbs install %s'", harness, harness)
	}
	args := []string{"plugin", "marketplace", "list", "--json"}
	list := exec.Command(harness, args...)
	list.Stderr = os.Stderr
	out, err := list.Output()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w — check that your harness supports plugins", harness, strings.Join(args, " "), err)
	}
	type marketplace struct {
		Name string `json:"name"`
	}
	var markets []marketplace
	if harness == "claude" {
		err = json.Unmarshal(out, &markets)
	} else {
		var result struct {
			Marketplaces []marketplace `json:"marketplaces"`
		}
		err = json.Unmarshal(out, &result)
		markets = result.Marketplaces
	}
	if err != nil {
		return fmt.Errorf("cannot read %s marketplace list: %w", harness, err)
	}
	registered := false
	for _, market := range markets {
		registered = registered || market.Name == "babysit"
	}
	verb := "install"
	refresh := "update"
	if harness == "codex" {
		verb, refresh = "add", "upgrade"
	}
	args = []string{"plugin", "marketplace", "add", babysitMarketplace}
	if registered {
		args = []string{"plugin", "marketplace", refresh, "babysit"}
	}
	for _, command := range [][]string{args, {"plugin", verb, "bbs@babysit"}} {
		if err := runVisible(harness, command...); err != nil {
			return fmt.Errorf("%s %s: %w", harness, strings.Join(command, " "), err)
		}
	}
	return nil
}

// OMP's marketplace list is not JSON. Re-running `plugin install` fails when
// the plugin is already present, so refresh uses `upgrade` in that case.
// See https://omp.sh/docs/plugins .
func installOmpPlugin() error {
	if !hasCmd("omp") {
		return fmt.Errorf("omp CLI is not on PATH — install/enable its CLI, then run 'bbs install omp'")
	}
	list := exec.Command("omp", "plugin", "marketplace", "list")
	list.Stderr = os.Stderr
	out, err := list.Output()
	if err != nil {
		return fmt.Errorf("omp plugin marketplace list failed: %w — check that your harness supports plugins", err)
	}
	if marketplaceNameListed(out, "babysit") {
		if err := runVisible("omp", "plugin", "marketplace", "update", "babysit"); err != nil {
			return fmt.Errorf("omp plugin marketplace update babysit: %w", err)
		}
	} else if err := runVisible("omp", "plugin", "marketplace", "add", babysitMarketplace); err != nil {
		return fmt.Errorf("omp plugin marketplace add %s: %w", babysitMarketplace, err)
	}
	if ompPluginInstalled() {
		if err := runVisible("omp", "plugin", "upgrade", "bbs@babysit"); err != nil {
			return fmt.Errorf("omp plugin upgrade bbs@babysit: %w", err)
		}
		return nil
	}
	if err := runVisible("omp", "plugin", "install", "bbs@babysit"); err != nil {
		return fmt.Errorf("omp plugin install bbs@babysit: %w", err)
	}
	return nil
}

func marketplaceNameListed(out []byte, name string) bool {
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return true
		}
	}
	return false
}

func ompPluginInstalled() bool {
	out, err := exec.Command("omp", "plugin", "list", "--json").Output()
	if err != nil {
		return false
	}
	var result struct {
		Marketplace []struct {
			ID string `json:"id"`
		} `json:"marketplace"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return strings.Contains(string(out), "bbs@babysit")
	}
	for _, plugin := range result.Marketplace {
		if plugin.ID == "bbs@babysit" {
			return true
		}
	}
	return false
}


// Antigravity's plugin layout keeps skills and their sibling references
// together. See https://antigravity.google/docs/plugins/ . Do not copy Claude
// hooks: Antigravity has a separate hook protocol.
func installAntigravity(home string) error {
	destinations := []string{filepath.Join(home, ".gemini", "config", "plugins", "bbs")}
	if hasCmd("agy") || isDir(filepath.Join(home, ".gemini", "antigravity-cli")) {
		destinations = append(destinations, filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "bbs"))
	}
	return installAntigravityAt(destinations)
}

const antigravityInstallMarker = ".bbs-managed"

func upgradeAntigravityPlugin() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	var destinations []string
	for _, surface := range []string{"config", "antigravity-cli"} {
		dst := filepath.Join(home, ".gemini", surface, "plugins", "bbs")
		if fileExists(filepath.Join(dst, antigravityInstallMarker)) {
			destinations = append(destinations, dst)
		}
	}
	if len(destinations) == 0 {
		return false, nil
	}
	fmt.Println("→ Updating the 2build skills (antigravity)...")
	return true, installAntigravityAt(destinations)
}

func installAntigravityAt(destinations []string) error {
	source := filepath.Join(babysitDir(), ".claude", "skills")
	if !isDir(source) {
		if !hasCmd("git") {
			return fmt.Errorf("git is required to download the Antigravity skill pack")
		}
		v := resolveVersion()
		// `bbs update` may have just replaced the executable through Homebrew.
		// Read the version at the invocation path, not only this process's old
		// ldflags, before selecting the matching skills release.
		if out, err := exec.Command(setupBinaryPath(), "--version").Output(); err == nil {
			if installed := strings.TrimSpace(strings.TrimPrefix(string(out), "bbs ")); versionRe.MatchString(installed) {
				v = installed
			}
		}
		if v == "unknown" {
			return fmt.Errorf("cannot determine the CLI version — install a released bbs or set BABYSIT_DIR to a 2build checkout")
		}
		tmp, err := os.MkdirTemp("", "bbs-antigravity-source-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		checkout := filepath.Join(tmp, "source")
		if err := runVisible("git", "clone", "--depth", "1", "--branch", "v"+v, "https://github.com/"+babysitMarketplace+".git", checkout); err != nil {
			return fmt.Errorf("download 2build v%s skills: %w", v, err)
		}
		source = filepath.Join(checkout, ".claude", "skills")
	}
	if !fileExists(filepath.Join(source, "autopilot", "SKILL.md")) || !isDir(filepath.Join(source, "references")) {
		return fmt.Errorf("incomplete 2build skill pack at %s", source)
	}
	for _, dst := range destinations {
		if err := installAntigravityBundle(source, dst); err != nil {
			return err
		}
		fmt.Printf("  Skills: %s\n", dst)
	}
	return nil
}

func installAntigravityBundle(source, dst string) error {
	if existsOrSymlink(dst) && (isSymlink(dst) || !fileExists(filepath.Join(dst, antigravityInstallMarker))) {
		return fmt.Errorf("refusing to replace unmanaged plugin %s — move it aside before retrying", dst)
	}
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".bbs-install-")
	if err != nil {
		return err
	}
	defer func() {
		if stage != "" {
			_ = os.RemoveAll(stage)
		}
	}()
	bundle := filepath.Join(stage, "bundle")
	if err := os.CopyFS(filepath.Join(bundle, "skills"), os.DirFS(source)); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bundle, "plugin.json"), []byte("{\"name\":\"bbs\",\"description\":\"2build autonomous product-building skills\"}\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bundle, antigravityInstallMarker), []byte("bbs install\n"), 0o644); err != nil {
		return err
	}
	backup := filepath.Join(stage, "previous")
	if existsOrSymlink(dst) {
		if err := os.Rename(dst, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(bundle, dst); err != nil {
		if existsOrSymlink(backup) {
			if restoreErr := os.Rename(backup, dst); restoreErr != nil {
				// Preserve the backup if restoring it fails.
				stage = ""
				return fmt.Errorf("install failed: %v; restore failed: %v; previous plugin at %s", err, restoreErr, backup)
			}
		}
		return err
	}
	return nil
}
