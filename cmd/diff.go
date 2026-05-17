package cmd

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/profile"
	"claude-skill-profiles/internal/settings"
)

var diffCmd = &cobra.Command{
	Use:   "diff <profile>",
	Short: "Show what `csp apply <profile>` would change",
	Long: `Compare the current .claude/settings.local.json's skillOverrides block
against what the named profile would produce, without writing anything.

Skills set to 'enabled' are absent from skillOverrides by design (enabled is
the default).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDiff(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, profileName string) error {
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
	current, err := settings.ReadSkillOverrides(path)
	if err != nil {
		return err
	}
	target := p.ToSkillOverrides()

	changed := computeChanges(current, target)
	if len(changed) == 0 {
		cmd.Printf("No changes — profile %q matches %s\n", profileName, path)
		return nil
	}

	cmd.Printf("Applying %q to %s would:\n\n", profileName, path)
	for _, c := range changed {
		switch {
		case c.before == "":
			cmd.Printf("  + %s: %s\n", c.skill, c.after)
		case c.after == "":
			cmd.Printf("  - %s: %s (back to default 'enabled')\n", c.skill, c.before)
		default:
			cmd.Printf("  ~ %s: %s -> %s\n", c.skill, c.before, c.after)
		}
	}
	return nil
}

type change struct {
	skill  string
	before string // "" means absent (i.e. enabled)
	after  string // "" means absent (i.e. enabled)
}

func computeChanges(before, after map[string]string) []change {
	keys := map[string]struct{}{}
	for k := range before {
		keys[k] = struct{}{}
	}
	for k := range after {
		keys[k] = struct{}{}
	}

	var out []change
	for k := range keys {
		b, a := before[k], after[k]
		if b == a {
			continue
		}
		out = append(out, change{skill: k, before: b, after: a})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].skill < out[j].skill })
	return out
}
