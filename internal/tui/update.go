package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildTable()
		return m, nil

	case connectedMsg:
		m.connected = true
		m.err = nil
		return m, tea.Batch(
			tickCmd(m.refreshInterval),
			fetchAlertsCmd(m.client, m.filterStatus),
			fetchStatsCmd(m.client),
		)

	case connectErrMsg:
		m.connected = false
		m.err = msg.err
		return m, nil

	case alertsFetchedMsg:
		m.alerts = msg.alerts
		m.loading = false
		m.rebuildTable()
		return m, nil

	case statsFetchedMsg:
		m.stats = &msg.stats
		return m, nil

	case fetchDataErrMsg:
		m.statusMsg = "error: " + msg.err.Error()
		return m, clearStatusCmd()

	case tickMsg:
		return m, tea.Batch(
			tickCmd(m.refreshInterval),
			fetchAlertsCmd(m.client, m.filterStatus),
			fetchStatsCmd(m.client),
		)

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		if m.activeSection == sectionAlerts && m.connected {
			var tableCmd tea.Cmd
			m.alertTable, tableCmd = m.alertTable.Update(msg)
			m2, ourCmd := m.handleKey(msg)
			return m2, tea.Batch(tableCmd, ourCmd)
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		m.activeSection = (m.activeSection + 1) % 3
		return m, nil

	case "r":
		if !m.connected {
			return m, connectCmd(m.client)
		}
		m.statusMsg = "Refreshing…"
		return m, tea.Batch(
			fetchAlertsCmd(m.client, m.filterStatus),
			fetchStatsCmd(m.client),
			clearStatusCmd(),
		)

	case "f":
		if m.activeSection != sectionAlerts || !m.connected {
			return m, nil
		}
		switch m.filterStatus {
		case "firing":
			m.filterStatus = "resolved"
		case "resolved":
			m.filterStatus = ""
		default:
			m.filterStatus = "firing"
		}
		m.loading = true
		return m, fetchAlertsCmd(m.client, m.filterStatus)
	}

	return m, nil
}
