package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/profile"
	"claude-skill-profiles/internal/settings"
)

var applyCmd = &cobra.Command{
	Use:   "apply <profile>",
	Short: "Apply a profile to this project's .claude/settings.local.json",
	Long: `Write the named profile's skill exposure settings to
.claude/settings.local.json in the current directory. The skillOverrides key
is replaced wholesale; other top-level keys are left untouched.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runApply(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, profileName string) error {
	p, err := profile.DefaultStore().Load(profileName)
	if err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			return fmt.Errorf("no such profile %q (try: csp list)", profileName)
		}
		return err
	}

	path, err := settings.Default()
	if err != nil {
		return err
	}

	overrides := p.ToSkillOverrides()
	if err := settings.ApplySkillOverrides(path, overrides); err != nil {
		return err
	}

	cmd.Printf("Applied profile %q to %s (%d non-default override(s))\n", profileName, path, len(overrides))
	return nil
}
