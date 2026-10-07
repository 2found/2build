package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/2found/2build/internal/config"
	"github.com/2found/2build/internal/starter"
	"github.com/spf13/cobra"
)

func newBootstrapCmd() *cobra.Command {
	var template, profile, source, release string
	var noVerify, asJSON bool
	cmd := &cobra.Command{
		Use:   "bootstrap <directory>",
		Short: "Create and verify a project from a versioned 2build starter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if !starter.ValidName(filepath.Base(target)) {
				return fmt.Errorf("project name must be lowercase kebab-case (max 64 characters)")
			}
			if profile != "pet" && profile != "startup" && profile != "enterprise" {
				return fmt.Errorf("profile must be pet, startup, or enterprise")
			}
			if _, err := os.Lstat(target); !os.IsNotExist(err) {
				return fmt.Errorf("target already exists or cannot be accessed: %s", target)
			}
			if release != "" && !starter.ValidVersion(release) {
				return fmt.Errorf("release must be a stable X.Y.Z version")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			var catalog starter.Catalog
			revision := ""
			if source == "" {
				remote, err := starter.NewClient().Fetch(ctx, release)
				if err != nil {
					return fmt.Errorf("load starter release: %w; use --source for a local catalog", err)
				}
				catalog, revision = remote.Catalog, remote.Revision
				var cleanup func()
				source, cleanup, err = starter.Checkout(ctx, remote)
				if err != nil {
					return err
				}
				defer cleanup()
			} else {
				catalog, err = starter.ReadCatalog(source)
				if err != nil {
					return err
				}
				if release != "" && release != catalog.Version {
					return fmt.Errorf("local catalog version %s does not match requested release %s", catalog.Version, release)
				}
			}
			cliVersion := resolveVersion()
			if !starter.ValidVersion(cliVersion) || starter.CompareVersion(cliVersion, catalog.MinCLI) < 0 {
				return fmt.Errorf("starter requires bbs >= %s; current version is %s", catalog.MinCLI, cliVersion)
			}
			verify := func(dir string, t starter.Template) error {
				for _, argv := range t.Verify {
					fmt.Fprintf(cmd.ErrOrStderr(), "→ %s\n", strings.Join(argv, " "))
					command := exec.CommandContext(cmd.Context(), argv[0], argv[1:]...)
					command.Dir = dir
					command.Stdout, command.Stderr = cmd.ErrOrStderr(), cmd.ErrOrStderr()
					if err := command.Run(); err != nil {
						return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
					}
				}
				return nil
			}
			if noVerify {
				verify = nil
			}
			lock, err := starter.Generate(source, target, template, profile, revision, verify)
			if err != nil {
				return err
			}
			dev := catalog.Templates[template].Dev
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"directory": target, "verified": !noVerify, "starter": lock, "dev": dev})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s from %s %s (%s).\n", target, lock.Source, lock.Release, lock.Template)
			if noVerify {
				fmt.Fprintln(cmd.OutOrStdout(), "Verification skipped; install dependencies and run the checks in README.md before claiming readiness.")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Template verification commands passed.")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Next: read %s, then run %s from the project directory.\n", filepath.Join(target, "AGENTS.md"), strings.Join(dev, " "))
			return nil
		},
	}
	cmd.Flags().StringVar(&template, "template", "hono-bun", "Starter template ID")
	cmd.Flags().StringVar(&profile, "profile", "startup", "Git policy: pet, startup, or enterprise")
	cmd.Flags().StringVar(&source, "source", "", "Local starter catalog directory (development/offline generation)")
	cmd.Flags().StringVar(&release, "version", "", "Stable starter release X.Y.Z (default: latest published release)")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "Generate files without installing dependencies or running checks")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the project handoff as JSON; verification logs go to stderr")
	return cmd
}

func newStarterCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "starter", Short: "Inspect starter provenance and relevant release updates"}
	var dir, source string
	var force, asJSON bool
	check := &cobra.Command{
		Use: "check", Short: "Report available starter changes without modifying project files", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			project, err := starter.FindProject(dir)
			if err != nil {
				return fmt.Errorf("find %s: %w", starter.LockPath, err)
			}
			lock, err := starter.ReadLock(project)
			if err != nil {
				return err
			}
			var result starter.CheckResult
			if source != "" {
				catalog, err := starter.ReadCatalog(source)
				if err != nil {
					return err
				}
				result = starter.Compare(lock, starter.Release{Catalog: catalog})
				checkedAt := time.Now().UTC()
				result.CheckedAt = &checkedAt
			} else {
				result = checkStarterRelease(cmd.Context(), lock, force)
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			printStarterResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
	check.Flags().StringVar(&dir, "dir", ".", "Project directory (also searches parents)")
	check.Flags().StringVar(&source, "source", "", "Compare against a local catalog instead of GitHub")
	check.Flags().BoolVar(&force, "force", false, "Refresh release information instead of using the cache")
	check.Flags().BoolVar(&asJSON, "json", false, "Print a machine-readable release check")
	cmd.AddCommand(check)
	return cmd
}

func checkStarterRelease(ctx context.Context, lock starter.Lock, force bool) starter.CheckResult {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return starter.Check(ctx, lock, filepath.Join(config.Dir(), "starter", "catalog-cache.json"), force, starter.NewClient().Fetch)
}

func printStarterResult(out io.Writer, result starter.CheckResult) {
	if result.Status == "update_available" {
		fmt.Fprintf(out, "STARTER_UPDATE_AVAILABLE %s %s %s\n%s\nUpgrade guide: %s\n", result.Template, result.Current, result.Latest, result.Summary, result.UpgradeURL)
		if result.MinCLI != "" {
			fmt.Fprintf(out, "Minimum CLI: bbs %s\n", result.MinCLI)
		}
	} else {
		fmt.Fprintf(out, "STARTER_%s %s %s", strings.ToUpper(result.Status), result.Template, result.Current)
		if result.Latest != "" {
			fmt.Fprintf(out, " (latest catalog: %s)", result.Latest)
		}
		fmt.Fprintln(out)
	}
	if result.Warning != "" {
		fmt.Fprintf(out, "Starter check: %s\n", result.Warning)
	}
	if result.Stale {
		fmt.Fprintln(out, "Release information is cached and stale.")
	}
}

func starterNotice(dir string, out io.Writer, check func(starter.Lock) starter.CheckResult) {
	project, err := starter.FindProject(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		fmt.Fprintf(out, "BBS_DEGRADED: starter check: %v\n", err)
		return
	}
	lock, err := starter.ReadLock(project)
	if err != nil {
		fmt.Fprintf(out, "BBS_DEGRADED: starter lock: %v\n", err)
		return
	}
	result := check(lock)
	if result.Status == "update_available" || result.Warning != "" {
		printStarterResult(out, result)
	}
}
