package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/profile"
)

var editCmd = &cobra.Command{
	Use:   "edit <profile>",
	Short: "Open a profile in $EDITOR (escape hatch; the TUI is the primary editor)",
	Long: `Open ~/.config/csp/profiles/<name>.yaml in $EDITOR for hand-editing.
The TUI (csp with no arguments) is the primary way to edit profiles; this is
the power-user / bulk-edit escape hatch.

Falls back to vi if $EDITOR is unset. After the editor exits, the profile is
re-parsed; an unparseable file is reported but not auto-reverted — re-run csp
edit to fix it.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEdit(cmd, args[0])
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, profileName string) error {
	store := profile.DefaultStore()
	path, err := store.Path(profileName)
	if err != nil {
		return err
	}

	// Refuse to edit a profile that doesn't exist — `csp new <name>` is the
	// right command for creating one.
	if _, err := store.Load(profileName); err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			return fmt.Errorf("no such profile %q (create one with: csp new %s)", profileName, profileName)
		}
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	ed := exec.Command(editor, path)
	ed.Stdin = os.Stdin
	ed.Stdout = os.Stdout
	ed.Stderr = os.Stderr
	if err := ed.Run(); err != nil {
		return fmt.Errorf("editor %q exited with error: %w", editor, err)
	}

	// Validate post-edit. Don't auto-revert; let the user fix it.
	if _, err := store.Load(profileName); err != nil {
		return fmt.Errorf("profile is no longer valid after editing — fix the file and rerun:\n  %s\n\n%w", path, err)
	}
	cmd.Printf("Saved %s\n", path)
	return nil
}
