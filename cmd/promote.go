package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/profile"
	"claude-skill-profiles/internal/settings"
	"claude-skill-profiles/internal/skill"
)

var promoteForce bool

var promoteCmd = &cobra.Command{
	Use:   "promote <name>",
	Short: "Save the current project's skillOverrides as a new named profile",
	Long: `Read .claude/settings.local.json in the current directory and write its
skillOverrides out as a fresh profile at ~/.config/csp/profiles/<name>.yaml.

Useful after tweaking a project's overrides — typically via 'csp custom' —
when the result is a pattern worth reusing across other projects.

Refuses to overwrite an existing profile; use --force to replace.
Refuses if the project has no skillOverrides to lift.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := settings.Default()
		if err != nil {
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
		return runPromote(cmd.OutOrStdout(), profile.DefaultStore(), projectPath, names, args[0], promoteForce)
	},
}

func init() {
	promoteCmd.Flags().BoolVarP(&promoteForce, "force", "f", false, "overwrite an existing profile with the same name")
	rootCmd.AddCommand(promoteCmd)
}

// runPromote is the testable core: read overrides from projectPath, seed a
// profile keyed on the discovered skill names, and save it under profileName
// in store. Refuses if there are no overrides to promote, or if the target
// profile already exists and force is false.
func runPromote(out io.Writer, store *profile.Store, projectPath string, skillNames []string, profileName string, force bool) error {
	if err := profile.ValidateName(profileName); err != nil {
		return err
	}

	overrides, err := settings.ReadSkillOverrides(projectPath)
	if err != nil {
		return err
	}
	if len(overrides) == 0 {
		return fmt.Errorf("nothing to promote: %s has no skillOverrides", projectPath)
	}

	p := profile.SeedFromOverrides(skillNames, overrides)
	if err := store.Save(profileName, p, force); err != nil {
		if errors.Is(err, profile.ErrExists) {
			return fmt.Errorf("profile %q already exists (use --force to overwrite)", profileName)
		}
		return err
	}

	profilePath, _ := store.Path(profileName)
	nonDefault := len(p.ToSkillOverrides())
	fmt.Fprintf(out, "Promoted %s → profile %q at %s\n", projectPath, profileName, profilePath)
	fmt.Fprintf(out, "Wrote %d skill entries (%d non-default)\n", len(skillNames), nonDefault)
	return nil
}
