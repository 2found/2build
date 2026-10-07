package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/2found/2build/internal/agent"
	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "agent", Short: "detect agents and list installed coding-agent CLIs", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
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
	detect.Flags().Bool("json", false, "print JSON")
	list.Flags().Bool("json", false, "print JSON")
	cmd.AddCommand(detect, list)
	return cmd
}
