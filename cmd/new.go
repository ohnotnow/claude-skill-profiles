package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/profile"
	"claude-skill-profiles/internal/settings"
	"claude-skill-profiles/internal/skill"
)

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Scaffold a new profile from discovered skills",
	Long: `Create a fresh profile at ~/.config/csp/profiles/<name>.yaml, seeded
with every skill discovered under ~/.claude/skills/ defaulting to 'enabled'.
Refuses to overwrite an existing profile.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runNew(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}

func runNew(cmd *cobra.Command, profileName string) error {
	if err := profile.ValidateName(profileName); err != nil {
		return err
	}

	skills, err := skill.Discover(skill.DefaultDir())
	if err != nil {
		return err
	}
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}

	// Seed from the global ~/.claude/settings.json so the profile starts as a
	// snapshot of current state, not a blank slate. Missing file is fine.
	var globalOverrides map[string]string
	if globalPath, err := settings.GlobalPath(); err == nil {
		globalOverrides, _ = settings.ReadSkillOverrides(globalPath)
	}

	p := profile.SeedFromOverrides(names, globalOverrides)

	store := profile.DefaultStore()
	if err := store.Save(profileName, p, false); err != nil {
		if errors.Is(err, profile.ErrExists) {
			return fmt.Errorf("profile %q already exists (use `csp edit %s` to modify it)", profileName, profileName)
		}
		return err
	}

	path, _ := store.Path(profileName)
	cmd.Printf("Created profile %q at %s\n", profileName, path)
	cmd.Printf("Seeded %d skill(s) — %d non-default from ~/.claude/settings.json\n", len(skills), len(p.ToSkillOverrides()))
	cmd.Printf("Edit interactively with: csp\n")
	cmd.Printf("Or open in $EDITOR with:  csp edit %s\n", profileName)
	return nil
}
