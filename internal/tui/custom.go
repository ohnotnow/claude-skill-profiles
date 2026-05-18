package tui

// Single-pane variant of the TUI for `csp custom`. Edits the current
// project's .claude/settings.local.json directly rather than a named
// profile YAML. See ant ADR-003 (`ant show csp-Ed6UZ`) for the rationale.

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"claude-skill-profiles/internal/profile"
	"claude-skill-profiles/internal/settings"
	"claude-skill-profiles/internal/skill"
)

// RunCustom starts the TUI in custom mode: a single skills pane bound to the
// current project's .claude/settings.local.json. The editor is seeded from
// the project's existing skillOverrides if present, or from the user's
// ~/.claude/settings.json otherwise — so the user starts from "current state"
// rather than a blank slate.
func RunCustom() error {
	m, err := customModel()
	if err != nil {
		return err
	}
	prog := tea.NewProgram(m, tea.WithAltScreen())
	_, err = prog.Run()
	return err
}

// customModel builds a model ready for the custom-mode TUI. Compared to
// initialModel, it skips profile discovery entirely, seeds an in-memory
// profile from either the project file or (if absent) the global settings,
// and rebinds m.save to write straight to the project file.
func customModel() (*model, error) {
	skillsDir := skill.DefaultDir()
	skills, err := skill.Discover(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("discovering skills: %w", err)
	}

	projectPath, err := settings.Default()
	if err != nil {
		return nil, err
	}
	globalPath, _ := settings.GlobalPath() // empty path tolerated by the helper

	skillNames := make([]string, len(skills))
	for i, s := range skills {
		skillNames[i] = s.Name
	}
	p, err := buildCustomProfile(skillNames, projectPath, globalPath)
	if err != nil {
		return nil, err
	}

	// Ensure the project ends up with a real settings.local.json the moment
	// `csp custom` is invoked. Otherwise a user who opens the editor in a
	// brand-new project, sees their global state, and quits without touching
	// anything is left with no file on disk — which from their point of view
	// looks like csp ignored their request. Existing files are left alone:
	// only the user's first toggle should rewrite a file they already had.
	if err := ensureProjectFile(projectPath, p); err != nil {
		return nil, err
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 64

	m := &model{
		theme:       NewTheme(),
		skills:      skills,
		skillsDir:   skillsDir,
		input:       ti,
		focus:       paneSkills,
		profile:     p,
		customMode:  true,
		customPath:  projectPath,
	}
	m.save = func(p *profile.Profile) error {
		return settings.ApplySkillOverrides(projectPath, p.ToSkillOverrides())
	}
	m.recomputeFiltered()
	return m, nil
}

// ensureProjectFile writes the seeded profile to projectPath if no file is
// there yet. Existing files (even malformed ones) are left untouched — only
// the missing-file case is handled here, so we never silently overwrite
// settings a user (or another tool) already put in place.
func ensureProjectFile(projectPath string, p *profile.Profile) error {
	if _, err := os.Stat(projectPath); err == nil {
		return nil // file exists, leave it alone
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking %s: %w", projectPath, err)
	}
	return settings.ApplySkillOverrides(projectPath, p.ToSkillOverrides())
}

// buildCustomProfile decides what overrides to seed the custom-mode editor
// from and returns the resulting profile.
//
// Precedence: the project's existing skillOverrides win. If the project file
// is missing or has an empty skillOverrides block, we fall back to the user's
// global ~/.claude/settings.json — so a brand-new project opens with current
// global state rather than every skill defaulted to enabled.
//
// globalPath may be empty (e.g. if os.UserHomeDir fails); in that case the
// fallback is silently skipped and an empty source map is used.
func buildCustomProfile(skillNames []string, projectPath, globalPath string) (*profile.Profile, error) {
	projectOverrides, err := settings.ReadSkillOverrides(projectPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", projectPath, err)
	}
	source := projectOverrides
	if len(source) == 0 && globalPath != "" {
		if global, err := settings.ReadSkillOverrides(globalPath); err == nil {
			source = global
		}
	}
	return profile.SeedFromOverrides(skillNames, source), nil
}
