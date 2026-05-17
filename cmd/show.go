package cmd

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/profile"
)

var showCmd = &cobra.Command{
	Use:   "show <profile>",
	Short: "Display a profile's skill -> state mappings",
	Long: `Print every skill in the profile grouped by state, plus the file path.
Useful for eyeballing, grepping, or piping into another tool.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runShow(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}

var stateLabels = map[profile.State]string{
	profile.StateEnabled:       "Enabled",
	profile.StateNameOnly:      "Name-only",
	profile.StateUserInvocable: "User-invocable-only",
	profile.StateOff:           "Off",
}

func runShow(cmd *cobra.Command, profileName string) error {
	store := profile.DefaultStore()
	p, err := store.Load(profileName)
	if err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			return fmt.Errorf("no such profile %q (try: csp list)", profileName)
		}
		return err
	}
	path, _ := store.Path(profileName)

	groups := make(map[profile.State][]string, len(profile.AllStates))
	for skillName, state := range p.Skills {
		groups[state] = append(groups[state], skillName)
	}
	for _, names := range groups {
		sort.Strings(names)
	}

	cmd.Printf("%s  (%s)\n", profileName, path)
	for _, state := range profile.AllStates {
		names := groups[state]
		cmd.Printf("\n%s (%d)\n", stateLabels[state], len(names))
		if len(names) == 0 {
			cmd.Println("  —")
			continue
		}
		for _, n := range names {
			cmd.Printf("  %s\n", n)
		}
	}
	return nil
}
