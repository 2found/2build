package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/reallongnguyen/babysit/internal/agent"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "detect agents and resolve launch settings"}
	detect := &cobra.Command{
		Use: "detect", Short: "identify the current agent (not the launch default)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			d := agent.Detect()
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(d)
			}
			fmt.Fprintln(cmd.OutOrStdout(), d.Agent)
			return nil
		},
	}
	list := &cobra.Command{
		Use: "list", Short: "list supported agents and their installed paths", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			installed := agent.Installations()
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(installed)
			}
			for _, i := range installed {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", i.Agent, orDefault(i.Path, "not installed"))
			}
			return nil
		},
	}
	var opts agent.Options
	var role string
	resolve := &cobra.Command{
		Use: "resolve", Short: "show effective agent/provider/model/effort for a role", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := agent.ResolveWith(role+"_agent", opts)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return errSilent
			}
			result := map[string]string{"agent": p.Name, "bin": p.Bin, "provider": p.Provider,
				"model": p.Model, "effort": p.Effort, "skill_prefix": p.SkillRef("")}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			for _, key := range []string{"agent", "bin", "provider", "model", "effort", "skill_prefix"} {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", key, result[key])
			}
			return nil
		},
	}
	resolve.Flags().StringVar(&role, "role", "worker", "worker or foreman")
	resolve.Flags().StringVar(&opts.Agent, "agent", "", "agent name or auto")
	resolve.Flags().StringVar(&opts.Provider, "provider", "", "native provider identifier")
	resolve.Flags().StringVar(&opts.Model, "model", "", "native model identifier or role")
	resolve.Flags().StringVar(&opts.Effort, "effort", "", "native reasoning/thinking level")
	resolve.Flags().StringVar(&opts.Dir, "dir", "", "destination repository")
	for _, sub := range []*cobra.Command{detect, list, resolve} {
		sub.Flags().Bool("json", false, "print JSON")
		cmd.AddCommand(sub)
	}
	return cmd
}
