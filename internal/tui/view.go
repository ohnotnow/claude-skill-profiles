package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"claude-skill-profiles/internal/profile"
)

const (
	minWidth  = 70
	leftPaneW = 24
)

func (m *model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		// Initial render before WindowSizeMsg arrives.
		return "Loading…"
	}
	if m.width < minWidth {
		return m.theme.Error.Render(fmt.Sprintf("Terminal too narrow (%d cols, need %d)", m.width, minWidth))
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 6 {
		bodyHeight = 6
	}

	left := m.renderProfilePane(bodyHeight)
	right := m.renderSkillPane(m.width-leftPaneW-2, bodyHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// --- header ---

func (m *model) renderHeader() string {
	title := m.theme.TitleActive.Render("csp") +
		m.theme.Dim.Render(" — claude-skill-profiles")
	right := m.theme.Dim.Render(m.skillsDir)
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + right
}

// --- left pane: profiles ---

func (m *model) renderProfilePane(height int) string {
	active := m.focus == paneProfiles
	titleStyle := m.theme.Title
	if active {
		titleStyle = m.theme.TitleActive
	}
	title := titleStyle.Render("Profiles")

	var lines []string
	lines = append(lines, title, "")

	if len(m.profiles) == 0 {
		lines = append(lines, m.theme.Dim.Render("(none yet)"))
		lines = append(lines, "")
		lines = append(lines, m.theme.Help.Render("Press "+m.theme.HelpKey.Render("n")+" to create one"))
	} else {
		for i, name := range m.profiles {
			prefix := "  "
			line := name
			if i == m.profileIdx {
				prefix = m.theme.SelectedActive.Render("▸ ")
				if active {
					line = m.theme.SelectedActive.Render(name)
				} else {
					line = m.theme.Selected.Render(name)
				}
			}
			lines = append(lines, prefix+line)
		}
	}

	// Pad to height.
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

// --- right pane: skills ---

func (m *model) renderSkillPane(width, height int) string {
	active := m.focus == paneSkills
	titleStyle := m.theme.Title
	if active {
		titleStyle = m.theme.TitleActive
	}
	titleText := "Skills"
	if m.profileName != "" {
		titleText = fmt.Sprintf("Skills — %s", m.profileName)
	}
	title := titleStyle.Render(titleText)

	// Sub-header: filter + sort indicator.
	subParts := []string{}
	if m.mode == modeFilter {
		subParts = append(subParts, m.theme.Help.Render("filter: ")+m.input.View())
	} else if m.filter != "" {
		subParts = append(subParts, m.theme.Help.Render("filter: ")+m.filter)
	}
	switch m.sortMode {
	case sortAlpha:
		subParts = append(subParts, m.theme.Help.Render("sort: alpha"))
	case sortByState:
		subParts = append(subParts, m.theme.Help.Render("sort: by state"))
	}
	subline := strings.Join(subParts, "   ")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, subline)
	lines = append(lines, "")

	if m.profile == nil {
		lines = append(lines, m.theme.Dim.Render("No profile selected."))
	} else if len(m.filtered) == 0 {
		if m.filter != "" {
			lines = append(lines, m.theme.Dim.Render("(no skills match filter)"))
		} else {
			lines = append(lines, m.theme.Dim.Render("(no skills discovered under ~/.claude/skills/)"))
		}
	} else {
		// Determine visible window.
		visible := height - 4 // title + subline + blank + bottom border
		if visible < 1 {
			visible = 1
		}
		m.ensureSkillVisible(visible)
		end := m.skillOffset + visible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}
		for i := m.skillOffset; i < end; i++ {
			lines = append(lines, m.renderSkillRow(i, active, width-4))
		}
	}

	// Pad to height.
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

func (m *model) ensureSkillVisible(visible int) {
	if m.skillIdx < m.skillOffset {
		m.skillOffset = m.skillIdx
	} else if m.skillIdx >= m.skillOffset+visible {
		m.skillOffset = m.skillIdx - visible + 1
	}
	if m.skillOffset < 0 {
		m.skillOffset = 0
	}
}

func (m *model) renderSkillRow(rowIdx int, active bool, width int) string {
	skillIdx := m.filtered[rowIdx]
	s := m.skills[skillIdx]
	state := m.stateOf(skillIdx)

	glyph, stateStyle := m.glyphFor(state)
	stateLabel := stateStyle.Render(padRight(string(state), 19))

	name := s.Name
	if width > 0 {
		maxName := width - 24 // 2 marker + 2 + 19 state + 1
		if maxName > 10 && len(name) > maxName {
			name = name[:maxName-1] + "…"
		}
	}

	marker := "  "
	if rowIdx == m.skillIdx {
		if active {
			marker = m.theme.SelectedActive.Render("▸ ")
			name = m.theme.SelectedActive.Render(name)
		} else {
			marker = m.theme.Selected.Render("▸ ")
			name = m.theme.Selected.Render(name)
		}
	}

	return fmt.Sprintf("%s%s %s %s", marker, glyph, stateLabel, name)
}

func (m *model) glyphFor(s profile.State) (string, lipgloss.Style) {
	switch s {
	case profile.StateEnabled:
		return m.theme.StateEnabled.Render("✓"), m.theme.StateEnabled
	case profile.StateNameOnly:
		return m.theme.StateNameOnly.Render("●"), m.theme.StateNameOnly
	case profile.StateUserInvocable:
		return m.theme.StateUserInvocable.Render("○"), m.theme.StateUserInvocable
	case profile.StateOff:
		return m.theme.StateOff.Render("✗"), m.theme.StateOff
	}
	return "?", m.theme.Dim
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

// --- footer ---

func (m *model) renderFooter() string {
	// Status line (if anything to say).
	var statusLine string
	if m.status != "" {
		statusLine = m.theme.Status.Width(m.width).Render(m.status)
	}

	var helpLine string
	switch m.mode {
	case modeFilter:
		helpLine = m.theme.Help.Render("type to filter   ") +
			m.theme.HelpKey.Render("enter") + m.theme.Help.Render(" apply   ") +
			m.theme.HelpKey.Render("esc") + m.theme.Help.Render(" cancel")
	case modeNewName:
		helpLine = m.theme.Help.Render("name the profile   ") +
			m.theme.HelpKey.Render("enter") + m.theme.Help.Render(" create   ") +
			m.theme.HelpKey.Render("esc") + m.theme.Help.Render(" cancel") +
			"\n" + m.theme.Help.Render(m.input.View())
	case modeConfirmApply, modeConfirmDelete:
		helpLine = m.theme.Status.Width(m.width).Render(m.confirm)
	default:
		if m.focus == paneProfiles {
			helpLine = helpJoin(m.theme,
				kv("↑↓/jk", "nav"),
				kv("enter/→", "edit"),
				kv("n", "new"),
				kv("a", "apply"),
				kv("e", "$EDITOR"),
				kv("d", "delete"),
				kv("r", "reload"),
				kv("tab", "switch"),
				kv("q", "quit"),
			)
		} else {
			helpLine = helpJoin(m.theme,
				kv("↑↓/jk", "nav"),
				kv("1", "enabled"),
				kv("2", "name-only"),
				kv("3", "user-only"),
				kv("4", "off"),
				kv("/", "filter"),
				kv("s", "sort"),
				kv("tab/esc", "back"),
				kv("q", "quit"),
			)
		}
	}

	parts := []string{}
	if statusLine != "" {
		parts = append(parts, statusLine)
	}
	parts = append(parts, helpLine)
	return strings.Join(parts, "\n")
}

type kvPair struct{ key, label string }

func kv(key, label string) kvPair { return kvPair{key, label} }

func helpJoin(theme *Theme, pairs ...kvPair) string {
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(theme.HelpKey.Render(p.key))
		b.WriteString(theme.Help.Render(" " + p.label))
	}
	return b.String()
}
