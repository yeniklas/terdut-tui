package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary  = lipgloss.Color("69")  // blue
	colorMuted    = lipgloss.Color("240") // gray
	colorFiring   = lipgloss.Color("196") // red
	colorResolved = lipgloss.Color("70")  // green
	colorAccent   = lipgloss.Color("214") // orange

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(colorPrimary).
			Padding(0, 2)

	styleTabInactive = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 2)

	styleFooter = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleStatus = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleError = lipgloss.NewStyle().
			Foreground(colorFiring).
			Bold(true)

	styleFiring   = lipgloss.NewStyle().Foreground(colorFiring).Bold(true)
	styleResolved = lipgloss.NewStyle().Foreground(colorResolved)
	styleMuted    = lipgloss.NewStyle().Foreground(colorMuted)
)
