package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var sectionNames = []string{"Alerts", "Schedule", "Users"}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	header := m.renderHeader()
	tabs := m.renderTabs()
	body := m.renderBody()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)
}

func (m Model) renderHeader() string {
	title := styleHeader.Render("terdut-tui")
	right := ""
	if m.serverURL != "" {
		right = styleMuted.Render(m.serverURL)
	}

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return title + strings.Repeat(" ", gap) + right
}

func (m Model) renderTabs() string {
	var tabs []string
	for i, name := range sectionNames {
		if section(i) == m.activeSection {
			tabs = append(tabs, styleTabActive.Render(name))
		} else {
			tabs = append(tabs, styleTabInactive.Render(name))
		}
	}
	line := strings.Repeat("─", m.width)
	return strings.Join(tabs, "") + "\n" + styleMuted.Render(line)
}

func (m Model) renderBody() string {
	bodyHeight := m.height - 5 // header + tabs + separator + footer + help
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var content string
	if m.err != nil {
		content = styleError.Render(fmt.Sprintf("Error: %v", m.err))
	} else if !m.connected {
		content = styleMuted.Render("Connecting…")
	} else if m.statusMsg != "" {
		content = styleStatus.Render(m.statusMsg)
	} else {
		switch m.activeSection {
		case sectionAlerts:
			content = styleMuted.Render("Alert dashboard — coming in Stage 2")
		case sectionSchedule:
			content = styleMuted.Render("On-call schedule — coming in Stage 4")
		case sectionUsers:
			content = styleMuted.Render("User management — coming in Stage 5")
		}
	}

	lines := strings.Split(content, "\n")
	padding := (bodyHeight - len(lines)) / 2
	if padding < 0 {
		padding = 0
	}
	return strings.Repeat("\n", padding) + content
}

func (m Model) renderFooter() string {
	helpView := m.help.ShortHelpView(m.keys.ShortHelp())
	return "\n" + styleFooter.Render(helpView)
}
