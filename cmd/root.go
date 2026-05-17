// Package cmd wires the csp CLI subcommands together.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "csp",
	Short: "Manage Claude Code skill exposure profiles",
	Long: `csp manages named profiles of Claude Code skill exposure settings.

Define a profile once (which skills are enabled, name-only, user-invocable-only,
or off) and apply it to any project with: csp apply <name>.

Profiles live in ~/.config/csp/profiles/<name>.yaml and edit Claude Code's
.claude/settings.local.json when applied. Running csp with no arguments opens
the interactive TUI — the primary way to edit profiles.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

// Execute is the entry point called from main().
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
