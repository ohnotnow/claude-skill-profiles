package tui

// Refresh screen — flipped-pane view for triaging newly-installed skills.
// See ant ADR-002 (`ant show csp-XKtxA`) for the full design rationale.
//
// Left pane: new skills (those missing from one or more profiles' YAML maps).
// Right pane: every profile, showing the current state of the highlighted
// left-pane skill in that profile. The default for an untouched (skill,
// profile) pair is `user-invocable-only` — safety-first.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"claude-skill-profiles/internal/profile"
)

type refreshFocus int

const (
	refreshFocusSkills refreshFocus = iota
	refreshFocusProfiles
)

type refreshState struct {
	newSkills    []string
	skillIdx     int
	skillOffset  int

	profileNames []string                     // alphabetised
	profiles     map[string]*profile.Profile  // loaded copies, keyed by name
	profileIdx   int
	profileOffset int

	focus      refreshFocus
	pendingAll bool
}

// enterRefresh prepares the refresh state and switches the model into the
// refresh screen. fromMain reports whether the user arrived via the `r`
// keypress (so `esc`/`q` should return to main rather than quit). Returns
// an error if there are no profiles or no skills to triage — the caller is
// expected to handle that case (a friendly message for the cobra command,
// a status-line note for the in-TUI keypress).
func (m *model) enterRefresh(fromMain bool) error {
	if len(m.profiles) == 0 {
		return fmt.Errorf("no profiles exist — try `csp new <name>` first")
	}

	rs := &refreshState{
		profileNames: append([]string(nil), m.profiles...),
		profiles:     map[string]*profile.Profile{},
	}
	sort.Strings(rs.profileNames)
	for _, name := range rs.profileNames {
		p, err := m.store.Load(name)
		if err != nil {
			return fmt.Errorf("loading %s: %w", name, err)
		}
		rs.profiles[name] = p
	}

	installed := make([]string, len(m.skills))
	for i, s := range m.skills {
		installed[i] = s.Name
	}
	rs.newSkills = computeNewSkills(installed, rs.profiles)
	if len(rs.newSkills) == 0 {
		return fmt.Errorf("nothing to triage — every skill is present in every profile")
	}

	m.refresh = rs
	m.screen = screenRefresh
	m.returnsToMain = fromMain
	m.status = ""
	return nil
}

// leaveRefresh exits the refresh screen. If we arrived from the main TUI,
// return there and reload the highlighted profile; otherwise quit.
func (m *model) leaveRefresh() tea.Cmd {
	if !m.returnsToMain {
		m.quitting = true
		return tea.Quit
	}
	m.screen = screenMain
	m.refresh = nil
	m.returnsToMain = false
	// Reload the currently-highlighted profile so any changes the user made
	// in refresh are visible in the main editor.
	if len(m.profiles) > 0 {
		if err := m.loadProfile(m.profileIdx); err != nil {
			m.status = "Reload failed: " + err.Error()
		}
	}
	m.recomputeFiltered()
	return nil
}

func (m *model) handleRefreshKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	rs := m.refresh
	if rs == nil {
		// Defensive — shouldn't happen, but don't lock the user out.
		m.screen = screenMain
		return m, nil
	}

	// Bulk-set prefix: `a` then 1/2/3/4 sets the highlighted skill to that
	// state across every profile in one go.
	if rs.pendingAll {
		rs.pendingAll = false
		switch msg.String() {
		case "1":
			m.refreshBulkSet(profile.StateEnabled)
		case "2":
			m.refreshBulkSet(profile.StateNameOnly)
		case "3":
			m.refreshBulkSet(profile.StateUserInvocable)
		case "4":
			m.refreshBulkSet(profile.StateOff)
		case "esc":
			m.status = "Cancelled"
		default:
			m.status = "Cancelled (a needs 1/2/3/4)"
		}
		return m, nil
	}

	switch msg.String() {
	case "q":
		return m, m.leaveRefresh()
	case "esc":
		return m, m.leaveRefresh()
	case "tab":
		if rs.focus == refreshFocusSkills {
			rs.focus = refreshFocusProfiles
		} else {
			rs.focus = refreshFocusSkills
		}
		return m, nil
	case "shift+tab":
		if rs.focus == refreshFocusProfiles {
			rs.focus = refreshFocusSkills
		} else {
			rs.focus = refreshFocusProfiles
		}
		return m, nil
	case "a":
		m.refresh.pendingAll = true
		m.status = fmt.Sprintf("a → press 1/2/3/4 to set %q across %d profile(s)  (esc cancels)",
			rs.newSkills[rs.skillIdx], len(rs.profileNames))
		return m, nil
	case "enter":
		m.refreshCommitDefaults()
		return m, nil
	case "1":
		m.refreshSetCurrent(profile.StateEnabled)
		return m, nil
	case "2":
		m.refreshSetCurrent(profile.StateNameOnly)
		return m, nil
	case "3":
		m.refreshSetCurrent(profile.StateUserInvocable)
		return m, nil
	case "4":
		m.refreshSetCurrent(profile.StateOff)
		return m, nil
	}

	// Navigation within the focused pane.
	switch rs.focus {
	case refreshFocusSkills:
		switch msg.String() {
		case "j", "down":
			if rs.skillIdx < len(rs.newSkills)-1 {
				rs.skillIdx++
				rs.profileIdx = 0
			}
		case "k", "up":
			if rs.skillIdx > 0 {
				rs.skillIdx--
				rs.profileIdx = 0
			}
		case "g", "home":
			rs.skillIdx = 0
			rs.profileIdx = 0
		case "G", "end":
			rs.skillIdx = len(rs.newSkills) - 1
			rs.profileIdx = 0
		}
	case refreshFocusProfiles:
		switch msg.String() {
		case "j", "down":
			if rs.profileIdx < len(rs.profileNames)-1 {
				rs.profileIdx++
			}
		case "k", "up":
			if rs.profileIdx > 0 {
				rs.profileIdx--
			}
		case "g", "home":
			rs.profileIdx = 0
		case "G", "end":
			rs.profileIdx = len(rs.profileNames) - 1
		}
	}
	return m, nil
}

// The refresh screen never auto-exits. The list of new skills is computed
// once on entry and stays stable until the user leaves with esc/q. This lets
// them review what they've set, change their mind, and explore the screen
// without it disappearing out from under them.

// refreshSetCurrent writes the given state for the highlighted (skill,
// profile) pair, saves the profile, and advances the profile cursor.
func (m *model) refreshSetCurrent(s profile.State) {
	rs := m.refresh
	if len(rs.newSkills) == 0 || len(rs.profileNames) == 0 {
		return
	}
	skillName := rs.newSkills[rs.skillIdx]
	profileName := rs.profileNames[rs.profileIdx]
	p := rs.profiles[profileName]
	p.Set(skillName, s)
	if err := m.store.Save(profileName, p, true); err != nil {
		m.status = "Save failed: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("%s / %s → %s", skillName, profileName, s)

	if rs.profileIdx < len(rs.profileNames)-1 {
		rs.profileIdx++
	}
}

// refreshBulkSet writes s to every profile for the highlighted skill and
// advances the skill cursor (since the user has expressed a per-skill
// decision, the natural next action is the next skill).
func (m *model) refreshBulkSet(s profile.State) {
	rs := m.refresh
	if len(rs.newSkills) == 0 {
		return
	}
	skillName := rs.newSkills[rs.skillIdx]
	var saved, failed []string
	for _, name := range rs.profileNames {
		p := rs.profiles[name]
		p.Set(skillName, s)
		if err := m.store.Save(name, p, true); err != nil {
			failed = append(failed, name)
			continue
		}
		saved = append(saved, name)
	}
	if len(failed) > 0 {
		m.status = fmt.Sprintf("Set %s = %s; %d profile(s) failed to save: %s",
			skillName, s, len(failed), strings.Join(failed, ", "))
	} else {
		m.status = fmt.Sprintf("Set %s = %s across %d profile(s)", skillName, s, len(saved))
	}
	if rs.skillIdx < len(rs.newSkills)-1 {
		rs.skillIdx++
		rs.profileIdx = 0
	}
}

// refreshCommitDefaults writes user-invocable-only to every profile that
// doesn't yet have an explicit entry for the highlighted skill, then advances
// the skill cursor. "Accept defaults and move on" shortcut.
func (m *model) refreshCommitDefaults() {
	rs := m.refresh
	if len(rs.newSkills) == 0 {
		return
	}
	skillName := rs.newSkills[rs.skillIdx]
	touched := commitDefaults(rs.profiles, skillName)
	for _, name := range touched {
		if err := m.store.Save(name, rs.profiles[name], true); err != nil {
			m.status = "Save failed: " + err.Error()
			return
		}
	}
	if len(touched) == 0 {
		m.status = fmt.Sprintf("%s already explicit in every profile", skillName)
	} else {
		m.status = fmt.Sprintf("%s → user-invocable-only in %d profile(s)", skillName, len(touched))
	}
	if rs.skillIdx < len(rs.newSkills)-1 {
		rs.skillIdx++
		rs.profileIdx = 0
	}
}

// --- view ---

func (m *model) viewRefresh() string {
	if m.refresh == nil {
		return ""
	}
	if m.width == 0 {
		return "Loading…"
	}
	if m.width < minWidth {
		return m.theme.Error.Render(fmt.Sprintf("Terminal too narrow (%d cols, need %d)", m.width, minWidth))
	}

	header := m.renderRefreshHeader()
	footer := m.renderRefreshFooter()
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 6 {
		bodyHeight = 6
	}

	left := m.renderRefreshSkillPane(bodyHeight)
	right := m.renderRefreshProfilePane(m.width-leftPaneW-2, bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *model) renderRefreshHeader() string {
	title := m.theme.TitleActive.Render("csp") +
		m.theme.Dim.Render(" — refresh: new-skill triage")
	right := m.theme.Dim.Render(m.skillsDir)
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + right
}

func (m *model) renderRefreshSkillPane(height int) string {
	rs := m.refresh
	active := rs.focus == refreshFocusSkills

	titleStyle := m.theme.Title
	if active {
		titleStyle = m.theme.TitleActive
	}
	title := titleStyle.Render(fmt.Sprintf("New skills (%d)", len(rs.newSkills)))

	var lines []string
	lines = append(lines, title, "")

	if len(rs.newSkills) == 0 {
		lines = append(lines, m.theme.Dim.Render("(none)"))
	} else {
		visible := height - 4
		if visible < 1 {
			visible = 1
		}
		m.ensureRefreshSkillVisible(visible)
		end := rs.skillOffset + visible
		if end > len(rs.newSkills) {
			end = len(rs.newSkills)
		}
		for i := rs.skillOffset; i < end; i++ {
			name := rs.newSkills[i]
			prefix := "  "
			line := name
			if i == rs.skillIdx {
				if active {
					prefix = m.theme.SelectedActive.Render("▸ ")
					line = m.theme.SelectedActive.Render(name)
				} else {
					prefix = m.theme.Selected.Render("▸ ")
					line = m.theme.Selected.Render(name)
				}
			}
			lines = append(lines, prefix+line)
		}
	}

	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	content := strings.Join(lines, "\n")
	border := m.theme.Border
	if active {
		border = m.theme.BorderActive
	}
	return border.Width(leftPaneW).Height(height - 2).Render(content)
}

func (m *model) renderRefreshProfilePane(width, height int) string {
	rs := m.refresh
	active := rs.focus == refreshFocusProfiles

	titleStyle := m.theme.Title
	if active {
		titleStyle = m.theme.TitleActive
	}
	titleText := "Profiles"
	if len(rs.newSkills) > 0 {
		titleText = fmt.Sprintf("Profiles — for %q", rs.newSkills[rs.skillIdx])
	}
	title := titleStyle.Render(titleText)

	var lines []string
	lines = append(lines, title)
	lines = append(lines, m.renderStateLegend())
	lines = append(lines, "")

	if len(rs.newSkills) == 0 {
		lines = append(lines, m.theme.Dim.Render("Nothing to triage."))
	} else {
		skillName := rs.newSkills[rs.skillIdx]
		visible := height - 5
		if visible < 1 {
			visible = 1
		}
		m.ensureRefreshProfileVisible(visible)
		end := rs.profileOffset + visible
		if end > len(rs.profileNames) {
			end = len(rs.profileNames)
		}
		for i := rs.profileOffset; i < end; i++ {
			name := rs.profileNames[i]
			lines = append(lines, m.renderRefreshProfileRow(i, name, skillName, active, width-4))
		}
	}

	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	content := strings.Join(lines, "\n")
	border := m.theme.Border
	if active {
		border = m.theme.BorderActive
	}
	return border.Width(width).Height(height - 2).Render(content)
}

func (m *model) renderRefreshProfileRow(rowIdx int, profileName, skillName string, active bool, width int) string {
	rs := m.refresh
	p := rs.profiles[profileName]
	state := displayState(p, skillName)
	_, explicit := p.Skills[skillName]

	glyph, stateStyle := m.glyphFor(state)
	label := string(state)
	if !explicit {
		// Dim the label to make it clear this is a not-yet-committed default.
		label += " (default)"
	}
	stateLabel := stateStyle.Render(padRight(label, 32))

	display := profileName
	if width > 0 {
		maxName := width - 38 // 2 marker + 2 + 32 state + 1
		if maxName > 10 && len(display) > maxName {
			display = display[:maxName-1] + "…"
		}
	}

	marker := "  "
	if rowIdx == rs.profileIdx {
		if active {
			marker = m.theme.SelectedActive.Render("▸ ")
			display = m.theme.SelectedActive.Render(display)
		} else {
			marker = m.theme.Selected.Render("▸ ")
			display = m.theme.Selected.Render(display)
		}
	}

	return fmt.Sprintf("%s%s %s %s", marker, glyph, stateLabel, display)
}

func (m *model) ensureRefreshSkillVisible(visible int) {
	rs := m.refresh
	if rs.skillIdx < rs.skillOffset {
		rs.skillOffset = rs.skillIdx
	} else if rs.skillIdx >= rs.skillOffset+visible {
		rs.skillOffset = rs.skillIdx - visible + 1
	}
	if rs.skillOffset < 0 {
		rs.skillOffset = 0
	}
}

func (m *model) ensureRefreshProfileVisible(visible int) {
	rs := m.refresh
	if rs.profileIdx < rs.profileOffset {
		rs.profileOffset = rs.profileIdx
	} else if rs.profileIdx >= rs.profileOffset+visible {
		rs.profileOffset = rs.profileIdx - visible + 1
	}
	if rs.profileOffset < 0 {
		rs.profileOffset = 0
	}
}

func (m *model) renderRefreshFooter() string {
	var statusLine string
	if m.status != "" {
		statusLine = m.theme.Status.Width(m.width).Render(m.status)
	}
	helpLine := helpJoin(m.theme,
		kv("↑↓/jk", "nav"),
		kv("tab", "pane"),
		kv("1-4", "set"),
		kv("a1-4", "all"),
		kv("enter", "default rest"),
		kv("esc/q", exitLabel(m.returnsToMain)),
	)
	parts := []string{}
	if statusLine != "" {
		parts = append(parts, statusLine)
	}
	parts = append(parts, helpLine)
	return strings.Join(parts, "\n")
}

func exitLabel(returnsToMain bool) string {
	if returnsToMain {
		return "back"
	}
	return "quit"
}
