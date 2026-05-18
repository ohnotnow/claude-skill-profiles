// Package tui implements the Bubble Tea profile browser/editor — the primary
// way users interact with csp.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"claude-skill-profiles/internal/profile"
	"claude-skill-profiles/internal/settings"
	"claude-skill-profiles/internal/skill"
)

// Run starts the TUI and blocks until the user quits. It returns any error
// raised by Bubble Tea itself (the TUI handles internal errors by surfacing
// them in the status line, not by returning).
func Run() error {
	m, err := initialModel()
	if err != nil {
		return err
	}
	prog := tea.NewProgram(m, tea.WithAltScreen())
	_, err = prog.Run()
	return err
}

// RunRefresh starts the TUI directly in the refresh screen. Used by
// `csp refresh`. Returns an error if no profiles exist or there's nothing to
// triage — callers (the cobra command) are expected to surface a friendly
// message instead of opening an empty TUI.
func RunRefresh() error {
	m, err := initialModel()
	if err != nil {
		return err
	}
	if err := m.enterRefresh(false); err != nil {
		return err
	}
	prog := tea.NewProgram(m, tea.WithAltScreen())
	_, err = prog.Run()
	return err
}

// --- model ---

type screen int

const (
	screenMain screen = iota
	screenRefresh
)

type focusPane int

const (
	paneProfiles focusPane = iota
	paneSkills
)

type inputMode int

const (
	modeNormal inputMode = iota
	modeFilter
	modeNewName
	modeConfirmApply
	modeConfirmDelete
)

type sortMode int

const (
	sortAlpha sortMode = iota
	sortByState
)

type model struct {
	width, height int
	theme         *Theme

	store     *profile.Store
	skills    []skill.Skill
	skillsDir string

	profiles    []string
	profileIdx  int
	profile     *profile.Profile
	profileName string

	focus       focusPane
	skillIdx    int
	skillOffset int

	sortMode  sortMode
	filter    string
	filtered  []int // indices into skills, after filter+sort

	mode    inputMode
	input   textinput.Model
	confirm string // text shown while in a confirm mode

	// pendingAll is set after the user presses `a` in the skills pane,
	// arming a bulk-set on the next 1/2/3/4 keypress (vim-style prefix).
	pendingAll bool

	status string
	err    error

	quitting bool

	// save is the callback the skills-pane uses to persist the currently-loaded
	// profile. The main TUI sets this to write back to the named profile YAML
	// via m.store; csp custom replaces it with a callback that writes directly
	// to the project's .claude/settings.local.json. The skills-pane never calls
	// m.store.Save directly — it goes through this hook so the editor widget
	// is reusable across save targets.
	save func(*profile.Profile) error

	// customMode is true when the TUI is editing the project's
	// .claude/settings.local.json directly rather than a named profile.
	// In this mode the profile pane is hidden, profile-management keys are
	// disabled, and the header advertises the project file path. See ADR-003.
	customMode bool
	customPath string

	// Refresh-screen state. `screen` decides which screen Update/View dispatch
	// to; `refresh` carries the refresh screen's own model (lazily created).
	// `returnsToMain` is true when refresh was entered via the `r` keypress
	// in the main TUI — `esc`/`q` returns to the main screen instead of
	// quitting the program.
	screen        screen
	refresh       *refreshState
	returnsToMain bool
}

func initialModel() (*model, error) {
	store := profile.DefaultStore()
	skillsDir := skill.DefaultDir()
	skills, err := skill.Discover(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("discovering skills: %w", err)
	}

	// Auto-prune on launch: drop any profile entries referring to skills that
	// no longer exist on disk. Silent by design — the user's mental model is
	// "if I deleted a skill, csp should reflect reality" (ant ADR-002).
	// Individual profile load/save errors are ignored here; if a profile is
	// genuinely broken the user will see it when they try to open it.
	known := make([]string, len(skills))
	for i, s := range skills {
		known[i] = s.Name
	}
	profile.PruneAll(store, known)

	names, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("listing profiles: %w", err)
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 64

	m := &model{
		theme:     NewTheme(),
		store:     store,
		skills:    skills,
		skillsDir: skillsDir,
		profiles:  names,
		input:     ti,
		focus:     paneProfiles,
	}
	m.save = func(p *profile.Profile) error {
		return m.store.Save(m.profileName, p, true)
	}

	if len(names) > 0 {
		if err := m.loadProfile(0); err != nil {
			m.err = err
		}
	}
	m.recomputeFiltered()
	return m, nil
}

// loadProfile reads the profile at profiles[idx] into m.profile.
func (m *model) loadProfile(idx int) error {
	if idx < 0 || idx >= len(m.profiles) {
		m.profile = nil
		m.profileName = ""
		return nil
	}
	p, err := m.store.Load(m.profiles[idx])
	if err != nil {
		return err
	}
	m.profile = p
	m.profileName = m.profiles[idx]
	m.profileIdx = idx
	m.skillIdx = 0
	m.skillOffset = 0
	m.filter = ""
	return nil
}

// recomputeFiltered rebuilds m.filtered from m.skills using the current filter
// and sort mode.
func (m *model) recomputeFiltered() {
	idx := make([]int, 0, len(m.skills))
	for i, s := range m.skills {
		if m.filter == "" || strings.Contains(strings.ToLower(s.Name), strings.ToLower(m.filter)) {
			idx = append(idx, i)
		}
	}
	switch m.sortMode {
	case sortAlpha:
		// skills are already alphabetised by Discover()
	case sortByState:
		sort.SliceStable(idx, func(i, j int) bool {
			si, sj := m.stateOf(idx[i]), m.stateOf(idx[j])
			if si != sj {
				return stateRank(si) < stateRank(sj)
			}
			return m.skills[idx[i]].Name < m.skills[idx[j]].Name
		})
	}
	m.filtered = idx
	if m.skillIdx >= len(m.filtered) {
		m.skillIdx = max0(len(m.filtered) - 1)
	}
}

func (m *model) stateOf(skillIdx int) profile.State {
	if m.profile == nil {
		return profile.StateEnabled
	}
	return m.profile.Get(m.skills[skillIdx].Name)
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// --- tea.Model ---

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.screen == screenRefresh {
			return m.handleRefreshKey(msg)
		}
		return m.handleKey(msg)
	case editorFinishedMsg:
		if msg.err != nil {
			m.status = "Editor exited with error: " + msg.err.Error()
			return m, nil
		}
		// Reload the profile in case the user edited it externally.
		for i, n := range m.profiles {
			if n == msg.name {
				if err := m.loadProfile(i); err != nil {
					m.status = "Reload failed: " + err.Error()
					return m, nil
				}
				m.recomputeFiltered()
				m.status = "Reloaded " + msg.name + " after editor"
				return m, nil
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C must always quit, regardless of mode.
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	// Mode-specific handling first.
	switch m.mode {
	case modeFilter:
		return m.handleFilterKey(msg)
	case modeNewName:
		return m.handleNewNameKey(msg)
	case modeConfirmApply:
		return m.handleConfirmApply(msg)
	case modeConfirmDelete:
		return m.handleConfirmDelete(msg)
	}

	// Global keys.
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	}

	// Custom mode has no profile pane — every keypress goes to the skills pane.
	if m.customMode {
		return m.handleSkillsKey(msg)
	}

	switch m.focus {
	case paneProfiles:
		return m.handleProfilesKey(msg)
	case paneSkills:
		return m.handleSkillsKey(msg)
	}
	return m, nil
}

func (m *model) handleProfilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if len(m.profiles) > 0 {
			m.focus = paneSkills
			m.clearStatus()
		}
		return m, nil
	case "j", "down":
		if len(m.profiles) == 0 {
			return m, nil
		}
		next := m.profileIdx + 1
		if next >= len(m.profiles) {
			next = len(m.profiles) - 1
		}
		if err := m.loadProfile(next); err != nil {
			m.err = err
		}
		m.recomputeFiltered()
	case "k", "up":
		if len(m.profiles) == 0 {
			return m, nil
		}
		next := m.profileIdx - 1
		if next < 0 {
			next = 0
		}
		if err := m.loadProfile(next); err != nil {
			m.err = err
		}
		m.recomputeFiltered()
	case "enter", "l", "right":
		if len(m.profiles) > 0 {
			m.focus = paneSkills
		}
	case "n":
		m.startNewName()
	case "a":
		if m.profileName != "" {
			m.startConfirmApply()
		}
	case "e":
		if m.profileName != "" {
			return m, m.editInExternalEditor()
		}
	case "d":
		if m.profileName != "" {
			m.startConfirmDelete()
		}
	case "r":
		if err := m.enterRefresh(true); err != nil {
			// enterRefresh returns errors as friendly messages — surface
			// them on the status line and stay on the main screen.
			m.status = err.Error()
		}
	case "R":
		m.reloadAll()
	}
	return m, nil
}

func (m *model) handleSkillsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// vim-style prefix: `a` then 1/2/3/4 sets every filtered skill at once.
	// Any other follow-up key cancels the prefix.
	if m.pendingAll {
		m.pendingAll = false
		switch msg.String() {
		case "1":
			m.setAllFiltered(profile.StateEnabled)
		case "2":
			m.setAllFiltered(profile.StateNameOnly)
		case "3":
			m.setAllFiltered(profile.StateUserInvocable)
		case "4":
			m.setAllFiltered(profile.StateOff)
		case "esc":
			m.status = "Cancelled"
		default:
			m.status = "Cancelled (a needs 1/2/3/4)"
		}
		return m, nil
	}

	switch msg.String() {
	case "a":
		if m.profile == nil || len(m.filtered) == 0 {
			return m, nil
		}
		m.pendingAll = true
		scope := "all"
		if m.filter != "" {
			scope = "filtered"
		}
		m.status = fmt.Sprintf("a → press 1/2/3/4 to set %d %s skill(s)  (esc cancels)", len(m.filtered), scope)
		return m, nil
	case "j", "down":
		if m.skillIdx < len(m.filtered)-1 {
			m.skillIdx++
		}
	case "k", "up":
		if m.skillIdx > 0 {
			m.skillIdx--
		}
	case "g", "home":
		m.skillIdx = 0
	case "G", "end":
		m.skillIdx = max0(len(m.filtered) - 1)
	case "esc", "h", "left":
		// In custom mode there's no profile pane to return to; swallow the key.
		if m.customMode {
			return m, nil
		}
		m.focus = paneProfiles
		m.clearStatus()
	case "1":
		m.setHighlightedState(profile.StateEnabled, true)
	case "2":
		m.setHighlightedState(profile.StateNameOnly, true)
	case "3":
		m.setHighlightedState(profile.StateUserInvocable, true)
	case "4":
		m.setHighlightedState(profile.StateOff, true)
	case "tab":
		m.cycleHighlightedState(+1)
	case "shift+tab":
		m.cycleHighlightedState(-1)
	case "/":
		m.startFilter()
	case "s":
		if m.sortMode == sortAlpha {
			m.sortMode = sortByState
			m.status = "Sort: by state"
		} else {
			m.sortMode = sortAlpha
			m.status = "Sort: alphabetical"
		}
		m.recomputeFiltered()
	}
	return m, nil
}

// setHighlightedState sets the highlighted skill's state, auto-saves, and
// optionally advances the cursor to the next skill. The 1/2/3/4 path advances
// (rapid-fire ergonomics); the tab-to-cycle path doesn't (dwell on one skill).
func (m *model) setHighlightedState(s profile.State, advance bool) {
	if m.profile == nil || len(m.filtered) == 0 {
		return
	}
	idx := m.filtered[m.skillIdx]
	skillName := m.skills[idx].Name
	m.profile.Set(skillName, s)
	if err := m.save(m.profile); err != nil {
		m.err = err
		m.status = "Save failed: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s → %s", skillName, s)

	if advance && m.skillIdx < len(m.filtered)-1 {
		m.skillIdx++
	}

	if m.sortMode == sortByState {
		m.recomputeFiltered()
	}
}

// cycleHighlightedState shifts the highlighted skill's state one step in the
// given direction (+1 forward, -1 back) through AllStates, wrapping at the
// edges. Doesn't advance the cursor — you cycle to find the right state, then
// j/k or 1/2/3/4 to move on.
func (m *model) cycleHighlightedState(dir int) {
	if m.profile == nil || len(m.filtered) == 0 {
		return
	}
	idx := m.filtered[m.skillIdx]
	current := m.profile.Get(m.skills[idx].Name)
	next := stepState(current, dir)
	m.setHighlightedState(next, false)
}

// setAllFiltered sets every skill currently in m.filtered to s, in a single
// save. Honours the filter so a user can scope the bulk-set by typing into the
// filter input first (e.g. /laravel then a4 to off every laravel-* skill).
func (m *model) setAllFiltered(s profile.State) {
	if m.profile == nil || len(m.filtered) == 0 {
		return
	}
	for _, idx := range m.filtered {
		m.profile.Set(m.skills[idx].Name, s)
	}
	if err := m.save(m.profile); err != nil {
		m.err = err
		m.status = "Save failed: " + err.Error()
		return
	}
	scope := "all"
	if m.filter != "" {
		scope = fmt.Sprintf("filtered (%q)", m.filter)
	}
	m.status = fmt.Sprintf("Set %d %s skill(s) to %s", len(m.filtered), scope, s)
	if m.sortMode == sortByState {
		m.recomputeFiltered()
	}
}

// --- filter mode ---

func (m *model) startFilter() {
	m.mode = modeFilter
	m.input.SetValue(m.filter)
	m.input.Placeholder = "filter skills…"
	m.input.Focus()
}

func (m *model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		m.filter = ""
		m.recomputeFiltered()
		return m, nil
	case "enter":
		m.filter = m.input.Value()
		m.mode = modeNormal
		m.input.Blur()
		m.recomputeFiltered()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.filter = m.input.Value()
	m.recomputeFiltered()
	return m, cmd
}

// --- new-profile prompt ---

func (m *model) startNewName() {
	m.mode = modeNewName
	m.input.SetValue("")
	m.input.Placeholder = "profile name (letters, digits, '-' or '_')"
	m.input.Focus()
}

func (m *model) handleNewNameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		m.mode = modeNormal
		m.input.Blur()
		if name == "" {
			return m, nil
		}
		if err := profile.ValidateName(name); err != nil {
			m.status = err.Error()
			return m, nil
		}
		names := make([]string, len(m.skills))
		for i, s := range m.skills {
			names[i] = s.Name
		}
		var globalOverrides map[string]string
		if gp, err := settings.GlobalPath(); err == nil {
			globalOverrides, _ = settings.ReadSkillOverrides(gp)
		}
		p := profile.SeedFromOverrides(names, globalOverrides)
		if err := m.store.Save(name, p, false); err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.profiles = append(m.profiles, name)
		sort.Strings(m.profiles)
		// Focus the new profile.
		for i, n := range m.profiles {
			if n == name {
				if err := m.loadProfile(i); err != nil {
					m.err = err
				}
				break
			}
		}
		m.recomputeFiltered()
		m.focus = paneSkills
		m.status = fmt.Sprintf("Created %q", name)
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// --- apply confirm ---

func (m *model) startConfirmApply() {
	path, err := settings.Default()
	if err != nil {
		m.status = err.Error()
		return
	}
	m.confirm = fmt.Sprintf("Apply %q to %s? (y/n)", m.profileName, path)
	m.mode = modeConfirmApply
}

func (m *model) handleConfirmApply(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		m.mode = modeNormal
		path, err := settings.Default()
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		overrides := m.profile.ToSkillOverrides()
		if err := settings.ApplySkillOverrides(path, overrides); err != nil {
			m.status = "Apply failed: " + err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("Applied %q to %s (%d override(s))", m.profileName, path, len(overrides))
	case "n", "esc":
		m.mode = modeNormal
		m.status = "Cancelled"
	}
	return m, nil
}

// --- delete confirm ---

func (m *model) startConfirmDelete() {
	m.confirm = fmt.Sprintf("Delete profile %q? This cannot be undone. (y/n)", m.profileName)
	m.mode = modeConfirmDelete
}

func (m *model) handleConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		m.mode = modeNormal
		name := m.profileName
		if err := m.store.Delete(name); err != nil {
			m.status = "Delete failed: " + err.Error()
			return m, nil
		}
		// Refresh profile list.
		names, err := m.store.List()
		if err != nil {
			m.err = err
			return m, nil
		}
		m.profiles = names
		if m.profileIdx >= len(m.profiles) {
			m.profileIdx = max0(len(m.profiles) - 1)
		}
		if len(m.profiles) == 0 {
			m.profile = nil
			m.profileName = ""
			m.focus = paneProfiles
		} else {
			if err := m.loadProfile(m.profileIdx); err != nil {
				m.err = err
			}
		}
		m.recomputeFiltered()
		m.status = fmt.Sprintf("Deleted %q", name)
	case "n", "esc":
		m.mode = modeNormal
		m.status = "Cancelled"
	}
	return m, nil
}

// --- external editor ---

// editInExternalEditor returns a tea.Cmd that suspends the TUI, runs $EDITOR
// on the highlighted profile's YAML, then reloads the profile.
func (m *model) editInExternalEditor() tea.Cmd {
	path, err := m.store.Path(m.profileName)
	if err != nil {
		m.status = err.Error()
		return nil
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	name := m.profileName
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{name: name, err: err}
	})
}

type editorFinishedMsg struct {
	name string
	err  error
}

func (m *model) reloadAll() {
	names, err := m.store.List()
	if err != nil {
		m.err = err
		return
	}
	m.profiles = names
	if m.profileIdx >= len(m.profiles) {
		m.profileIdx = max0(len(m.profiles) - 1)
	}
	if len(m.profiles) > 0 {
		if err := m.loadProfile(m.profileIdx); err != nil {
			m.err = err
		}
	}
	m.recomputeFiltered()
	m.status = "Reloaded"
}

func (m *model) clearStatus() {
	m.status = ""
}
