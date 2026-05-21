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
	right := styleMuted.Render(m.serverURL)
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
	sep := styleMuted.Render(strings.Repeat("─", m.width))
	return strings.Join(tabs, "") + "\n" + sep
}

func (m Model) renderBody() string {
	if m.err != nil {
		return "\n" + styleError.Render(fmt.Sprintf("  Error: %v", m.err)) +
			"\n" + styleMuted.Render("  Press r to retry.")
	}
	if !m.connected {
		return "\n" + styleMuted.Render("  Connecting…")
	}

	switch m.activeSection {
	case sectionAlerts:
		return m.renderAlerts()
	case sectionSchedule:
		return "\n" + styleMuted.Render("  On-call schedule — coming in Stage 4")
	case sectionUsers:
		return "\n" + styleMuted.Render("  User management — coming in Stage 5")
	}
	return ""
}

func (m Model) renderAlerts() string {
	statsBar := m.renderStatsBar()
	var content string
	if m.loading && len(m.alerts) == 0 {
		content = styleMuted.Render("  Loading alerts…")
	} else if len(m.alerts) == 0 {
		label := m.filterStatus
		if label == "" {
			label = "all"
		}
		content = styleMuted.Render(fmt.Sprintf("  No %s alerts.", label))
	} else {
		content = m.alertTable.View()
	}
	return lipgloss.JoinVertical(lipgloss.Left, statsBar, content)
}

func (m Model) renderStatsBar() string {
	total, firing, resolved := 0, 0, 0
	if m.stats != nil {
		total = m.stats.Total
		firing = m.stats.Firing
		resolved = m.stats.Resolved
	}

	filterLabel := m.filterStatus
	if filterLabel == "" {
		filterLabel = "all"
	}

	left := fmt.Sprintf("  Total: %d  %s  %s",
		total,
		styleFiring.Render(fmt.Sprintf("Firing: %d", firing)),
		styleResolved.Render(fmt.Sprintf("Resolved: %d", resolved)),
	)
	right := styleMuted.Render(fmt.Sprintf("filter: %s  [f]  ", filterLabel))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderFooter() string {
	helpView := styleFooter.Render(m.help.ShortHelpView(m.keys.ShortHelp()))
	if m.statusMsg != "" {
		status := styleStatus.Render(m.statusMsg)
		gap := m.width - lipgloss.Width(status) - lipgloss.Width(helpView)
		if gap < 0 {
			gap = 0
		}
		return "\n" + status + strings.Repeat(" ", gap) + helpView
	}
	return "\n" + helpView
}
