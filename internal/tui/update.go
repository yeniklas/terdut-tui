package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case connectedMsg:
		m.connected = true
		m.err = nil
		m.statusMsg = "Connected to " + m.serverURL
		return m, tea.Batch(tickCmd(m.refreshInterval), clearStatusCmd())

	case connectErrMsg:
		m.connected = false
		m.err = msg.err
		return m, nil

	case tickMsg:
		return m, tickCmd(m.refreshInterval)

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "q" || msg.String() == "ctrl+c":
		return m, tea.Quit

	case msg.String() == "tab":
		m.activeSection = (m.activeSection + 1) % 3
		return m, nil

	case msg.String() == "r":
		m.statusMsg = "Refreshing…"
		return m, tea.Batch(connectCmd(m.client), clearStatusCmd())
	}

	return m, nil
}
