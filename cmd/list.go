package cmd

import (
	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/profile"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available profiles",
	Long:  `List every profile in ~/.config/csp/profiles/, one per line.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(cmd)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command) error {
	names, err := profile.DefaultStore().List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		cmd.Println("No profiles yet. Create one with: csp new <name>")
		return nil
	}
	for _, n := range names {
		cmd.Println(n)
	}
	return nil
}
