package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/terdut-tui/internal/api"
)

var (
	colorPrimary  = lipgloss.Color("69")  // blue
	colorMuted    = lipgloss.Color("240") // gray
	colorFiring   = lipgloss.Color("196") // red
	colorResolved = lipgloss.Color("70")  // green
	colorAccent   = lipgloss.Color("214") // orange

	colorSevCritical = lipgloss.Color("196") // red
	colorSevError    = lipgloss.Color("202") // dark orange
	colorSevWarning  = lipgloss.Color("214") // orange
	colorSevInfo     = lipgloss.Color("39")  // cyan

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

	styleFiring    = lipgloss.NewStyle().Foreground(colorFiring).Bold(true)
	styleResolved  = lipgloss.NewStyle().Foreground(colorResolved)
	styleMuted     = lipgloss.NewStyle().Foreground(colorMuted)
	styleAlertName = lipgloss.NewStyle().Bold(true)
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleSelected  = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	styleAccent    = lipgloss.NewStyle().Foreground(colorAccent)

	// Incident status. Triggered is unclaimed work and reads as loudly as a
	// firing alert; acknowledged means somebody has it.
	styleTriggered    = lipgloss.NewStyle().Foreground(colorFiring).Bold(true)
	styleAcknowledged = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleSnoozed      = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

	// Severity, over the conventional Alertmanager label values.
	styleSevCritical = lipgloss.NewStyle().Foreground(colorSevCritical).Bold(true)
	styleSevError    = lipgloss.NewStyle().Foreground(colorSevError).Bold(true)
	styleSevWarning  = lipgloss.NewStyle().Foreground(colorSevWarning)
	styleSevInfo     = lipgloss.NewStyle().Foreground(colorSevInfo)
)

// severityStyle picks the style for a severity label, falling back to muted for
// values this client does not recognise rather than dropping them.
func severityStyle(severity string) lipgloss.Style {
	switch strings.ToLower(severity) {
	case "critical":
		return styleSevCritical
	case "error":
		return styleSevError
	case "warning":
		return styleSevWarning
	case "info":
		return styleSevInfo
	default:
		return styleMuted
	}
}

// incidentStatusStyle picks the style for an incident status, falling back to
// muted for statuses added after this client was built.
func incidentStatusStyle(status string) lipgloss.Style {
	switch status {
	case api.StatusTriggered:
		return styleTriggered
	case api.StatusAcknowledged:
		return styleAcknowledged
	case api.StatusResolved:
		return styleResolved
	default:
		return styleMuted
	}
}
