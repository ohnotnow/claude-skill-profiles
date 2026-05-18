package cmd

import (
	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/tui"
)

var customCmd = &cobra.Command{
	Use:   "custom",
	Short: "Edit this project's .claude/settings.local.json directly",
	Long: `Open the skills editor pointed at the current project's
.claude/settings.local.json rather than at a named profile. Every toggle
saves directly to the project file. Useful for one-off projects that
don't fit any named profile, or for tweaking a profile-applied baseline
without persisting the tweaks as a new profile.

If no .claude/settings.local.json exists yet, the editor is seeded from
~/.claude/settings.json (the user's global Claude Code config) so you
start from current global state, not a blank slate. The first toggle
writes the file (and creates the .claude/ directory if needed).

See also: csp promote <name> — lift these ad-hoc edits into a reusable
named profile.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunCustom()
	},
}

func init() {
	rootCmd.AddCommand(customCmd)
}
