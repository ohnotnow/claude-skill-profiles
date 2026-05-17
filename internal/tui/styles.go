package tui

import "github.com/charmbracelet/lipgloss"

// Theme is the colour and style palette used across the TUI.
type Theme struct {
	Border         lipgloss.Style
	BorderActive   lipgloss.Style
	Title          lipgloss.Style
	TitleActive    lipgloss.Style
	Selected       lipgloss.Style
	SelectedActive lipgloss.Style
	Dim            lipgloss.Style
	Help           lipgloss.Style
	HelpKey        lipgloss.Style
	Status         lipgloss.Style
	Error          lipgloss.Style

	StateEnabled       lipgloss.Style
	StateNameOnly      lipgloss.Style
	StateUserInvocable lipgloss.Style
	StateOff           lipgloss.Style
}

// NewTheme returns the default theme.
func NewTheme() *Theme {
	const (
		colAccent      = "5"   // magenta
		colAccentDim   = "13"  // bright magenta
		colDim         = "240" // grey
		colHelp        = "244" // light grey
		colEnabled     = "2"   // green
		colNameOnly    = "6"   // cyan
		colUserOnly    = "3"   // yellow
		colOff         = "1"   // red
		colError       = "9"   // bright red
		colStatusBG    = "236" // dark grey background
		colStatusFG    = "252" // near-white
	)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colDim)).
		Padding(0, 1)

	borderActive := border.BorderForeground(lipgloss.Color(colAccent))

	return &Theme{
		Border:         border,
		BorderActive:   borderActive,
		Title:          lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)).Bold(true),
		TitleActive:    lipgloss.NewStyle().Foreground(lipgloss.Color(colAccentDim)).Bold(true),
		Selected:       lipgloss.NewStyle().Foreground(lipgloss.Color(colAccent)),
		SelectedActive: lipgloss.NewStyle().Foreground(lipgloss.Color(colAccentDim)).Bold(true),
		Dim:            lipgloss.NewStyle().Foreground(lipgloss.Color(colDim)),
		Help:           lipgloss.NewStyle().Foreground(lipgloss.Color(colHelp)),
		HelpKey:        lipgloss.NewStyle().Foreground(lipgloss.Color(colAccentDim)).Bold(true),
		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colStatusFG)).
			Background(lipgloss.Color(colStatusBG)).
			Padding(0, 1),
		Error:              lipgloss.NewStyle().Foreground(lipgloss.Color(colError)).Bold(true),
		StateEnabled:       lipgloss.NewStyle().Foreground(lipgloss.Color(colEnabled)),
		StateNameOnly:      lipgloss.NewStyle().Foreground(lipgloss.Color(colNameOnly)),
		StateUserInvocable: lipgloss.NewStyle().Foreground(lipgloss.Color(colUserOnly)),
		StateOff:           lipgloss.NewStyle().Foreground(lipgloss.Color(colOff)),
	}
}
