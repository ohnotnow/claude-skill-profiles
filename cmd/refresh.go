package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/tui"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Triage newly-installed skills across every profile",
	Long: `Open a TUI for triaging skills that have appeared in ~/.claude/skills/ but
aren't yet present in every profile.

The screen flips the usual layout: new skills on the left, profiles on the
right. For each new skill you can set the exposure state in each profile
individually (1/2/3/4), bulk-set across every profile in one keystroke (a1-4),
or accept the safe default (enter — writes user-invocable-only into any profile
still missing the skill).

If everything is already in sync, csp refresh prints a one-liner and exits.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := tui.RunRefresh()
		if err == nil {
			return nil
		}
		// RunRefresh returns these as errors so the TUI doesn't open; surface
		// them as friendly messages, not Go errors.
		msg := err.Error()
		if strings.HasPrefix(msg, "no profiles") || strings.HasPrefix(msg, "nothing to triage") {
			cmd.Println(msg)
			return nil
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}
