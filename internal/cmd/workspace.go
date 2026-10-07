package cmd

import (
	"fmt"
	"os"

	"github.com/2found/2build/internal/qaconfig"
	"github.com/2found/2build/internal/workspace"
	"github.com/spf13/cobra"
)

const workspaceUsage = `Usage:
  bbs config workspace list
  bbs config workspace show [<name>]
  bbs config workspace create <name>
  bbs config workspace add-repo <name> --git-url <url> [--path <dir>] [--role <fe|be|shared>] [--repo-type <monorepo|polyrepo>]

Workspace registrations live under the top-level workspaces mapping in
~/.babysit/config.yaml. There is no per-repository config file.
`

// Hidden legacy spelling; the documented surface is `bbs config workspace`.
func newWorkspaceCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "workspace",
		Short:              "multi-repo registry: which repos belong to one product",
		Hidden:             true,
		DisableFlagParsing: true,
		RunE:               func(_ *cobra.Command, args []string) error { return runWorkspace(args) },
	}
}

func runWorkspace(args []string) error {
	if err := dispatchWorkspace(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return errSilent
	}
	return nil
}

func dispatchWorkspace(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, workspaceUsage)
		return errSilent
	}
	switch args[0] {
	case "list":
		return workspaceList()
	case "show":
		return workspaceShow(args[1:])
	case "create":
		return workspaceCreate(args[1:])
	case "add-repo":
		return workspaceAddRepo(args[1:])
	default:
		fmt.Fprint(os.Stderr, workspaceUsage)
		return errSilent
	}
}

func workspaceList() error {
	all, err := workspace.List()
	if err != nil {
		return err
	}
	if len(all) == 0 {
		fmt.Println("no workspaces registered")
		return nil
	}
	for _, w := range all {
		fmt.Printf("workspace %s (%d repos)\n", w.Name, len(w.Repos))
	}
	return nil
}

func workspaceShow(args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		top := qaconfig.RepoToplevel()
		if top == "" {
			return fmt.Errorf("not in a git repo — pass a workspace name")
		}
		resolver := workspace.NewResolver(top, originURL(top))
		if err := resolver.Err(); err != nil {
			return err
		}
		repo, ok := resolver.Repo()
		if !ok {
			fmt.Printf("%s: not registered in %s\n", top, workspace.ConfigPath())
			return nil
		}
		name = resolver.Name()
		fmt.Printf("repo:      %s\n", top)
		fmt.Printf("workspace: %s\n", name)
		fmt.Printf("harness:   %s\n", harnessDisplay(repo))
		if repo.RepoType != "" {
			fmt.Printf("repo_type: %s\n", repo.RepoType)
		}
	}
	w, err := workspace.Load(name)
	if err != nil {
		return err
	}
	fmt.Printf("workspace %s\n", w.Name)
	for _, repo := range w.Repos {
		path := repo.Path
		if path == "" {
			path = "(not on this machine)"
		}
		role := ""
		if repo.Role != "" {
			role = "  role=" + repo.Role
		}
		fmt.Printf("  %s  %s%s\n", repo.GitURL, path, role)
	}
	return nil
}

func harnessDisplay(repo workspace.Repo) string {
	if repo.HarnessVersion == nil || *repo.HarnessVersion == "" {
		return "not set (run bbs:setup-project to record it)"
	}
	if current := resolveVersion(); repo.Stale(current) {
		return *repo.HarnessVersion + " (stale — current is " + current + ")"
	}
	return *repo.HarnessVersion
}

func workspaceCreate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: bbs config workspace create <name>")
	}
	if err := workspace.Create(args[0]); err != nil {
		return err
	}
	fmt.Printf("workspace %s ready (%s)\n", args[0], workspace.ConfigPath())
	return nil
}

func workspaceAddRepo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: bbs config workspace add-repo <name> --git-url <url> [--path <dir>] [--role <role>] [--repo-type <type>]")
	}
	name, rest := args[0], args[1:]
	var repo workspace.Repo
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--git-url":
			repo.GitURL, i = valueAt(rest, i), i+1
		case "--path":
			repo.Path, i = valueAt(rest, i), i+1
		case "--role":
			repo.Role, i = valueAt(rest, i), i+1
		case "--repo-type":
			repo.RepoType, i = valueAt(rest, i), i+1
		case "--name":
			repo.Name, i = valueAt(rest, i), i+1
		case "--description":
			repo.Description, i = valueAt(rest, i), i+1
		default:
			return fmt.Errorf("unknown add-repo option %q", rest[i])
		}
	}
	if repo.GitURL == "" {
		return fmt.Errorf("add-repo needs --git-url (the machine-independent half of a repo's identity)")
	}
	if version := resolveVersion(); version != "unknown" {
		repo.HarnessVersion = &version
	}
	if err := workspace.AddRepo(name, repo); err != nil {
		return err
	}
	fmt.Printf("workspace %s: added %s (%s)\n", name, repo.GitURL, workspace.ConfigPath())
	return nil
}

func originURL(toplevel string) string {
	return gitOut("-C", toplevel, "remote", "get-url", "origin")
}
