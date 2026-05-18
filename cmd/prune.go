package cmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"claude-skill-profiles/internal/profile"
	"claude-skill-profiles/internal/skill"
)

var pruneDryRun bool

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove profile entries for skills no longer installed",
	Long: `Reconcile every profile against the current contents of ~/.claude/skills/.
Entries referring to skills that no longer exist on disk are dropped.

Typical use is paired with skill removal — e.g. a shell alias that removes a
skill from ~/.claude/skills/ and then runs 'csp prune' to tidy every profile
in one go. The TUI already auto-prunes on launch, so this is mainly for the
headless workflow.

  csp prune              # remove stale entries and report
  csp prune --dry-run    # show what would be removed, without writing`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		skills, err := skill.Discover(skill.DefaultDir())
		if err != nil {
			return err
		}
		known := make([]string, len(skills))
		for i, s := range skills {
			known[i] = s.Name
		}
		return runPrune(cmd.OutOrStdout(), profile.DefaultStore(), known, pruneDryRun)
	},
}

func init() {
	pruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false, "show what would be removed without writing")
	rootCmd.AddCommand(pruneCmd)
}

// runPrune is the testable core: prune every profile in store against the
// known skill list, writing a human-readable summary to out. When dryRun is
// true no files are written.
func runPrune(out io.Writer, store *profile.Store, known []string, dryRun bool) error {
	if dryRun {
		return runPruneDry(out, store, known)
	}
	removed, errs := profile.PruneAll(store, known)
	for name, err := range errs {
		fmt.Fprintf(out, "Warning: %s: %v\n", name, err)
	}
	if len(removed) == 0 {
		fmt.Fprintln(out, "Nothing to prune — all profiles are clean")
		return nil
	}
	total := 0
	for _, r := range removed {
		total += len(r)
	}
	fmt.Fprintf(out, "Pruned %d stale skill entr%s across %d profile(s):\n",
		total, plural(total, "y", "ies"), len(removed))
	for _, name := range sortedKeys(removed) {
		fmt.Fprintf(out, "  %s: %s\n", name, joinComma(removed[name]))
	}
	return nil
}

// runPruneDry mirrors runPrune but loads profiles read-only and never saves.
func runPruneDry(out io.Writer, store *profile.Store, known []string) error {
	names, err := store.List()
	if err != nil {
		return fmt.Errorf("listing profiles: %w", err)
	}
	keep := make(map[string]bool, len(known))
	for _, n := range known {
		keep[n] = true
	}
	preview := map[string][]string{}
	for _, name := range names {
		p, err := store.Load(name)
		if err != nil {
			fmt.Fprintf(out, "Warning: %s: %v\n", name, err)
			continue
		}
		var stale []string
		for skill := range p.Skills {
			if !keep[skill] {
				stale = append(stale, skill)
			}
		}
		if len(stale) > 0 {
			sort.Strings(stale)
			preview[name] = stale
		}
	}
	if len(preview) == 0 {
		fmt.Fprintln(out, "Nothing to prune — all profiles are clean")
		return nil
	}
	total := 0
	for _, r := range preview {
		total += len(r)
	}
	fmt.Fprintf(out, "Would prune %d stale skill entr%s across %d profile(s):\n",
		total, plural(total, "y", "ies"), len(preview))
	for _, name := range sortedKeys(preview) {
		fmt.Fprintf(out, "  %s: %s\n", name, joinComma(preview[name]))
	}
	return nil
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
