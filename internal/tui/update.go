package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildTable()
		m.detailViewport.Width = m.width
		m.detailViewport.Height = m.detailViewportHeight()
		m.statsViewport.Width = m.width
		m.statsViewport.Height = m.height - 5
		m.refreshDetailContent()
		m.refreshStatsContent()
		return m, nil

	// Dashboard messages
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
		m.statusMsg = "refresh error: " + msg.err.Error()
		return m, clearStatusCmd()

	case tickMsg:
		return m, tea.Batch(
			tickCmd(m.refreshInterval),
			fetchAlertsCmd(m.client, m.filterStatus),
			fetchStatsCmd(m.client),
		)

	// Detail messages
	case alertDetailFetchedMsg:
		m.selectedAlert = msg.alert
		m.comments = msg.comments
		m.detailLoading = false
		if m.commentCursor >= len(m.comments) {
			m.commentCursor = -1
		}
		m.refreshDetailContent()
		return m, nil

	case alertDetailErrMsg:
		m.detailLoading = false
		m.statusMsg = "error: " + msg.err.Error()
		return m, clearStatusCmd()

	case actionErrMsg:
		m.statusMsg = "error: " + msg.err.Error()
		return m, clearStatusCmd()

	case detailStatsFetchedMsg:
		m.topAlerts = msg.top
		m.hourStats = msg.byHour
		m.dayStats = msg.byDay
		m.statsLoading = false
		m.refreshStatsContent()
		return m, nil

	case detailStatsErrMsg:
		m.statsLoading = false
		m.statusMsg = "stats error: " + msg.err.Error()
		m.mode = modeDetail
		return m, clearStatusCmd()

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		return m.routeKey(msg)
	}

	return m, nil
}

// routeKey passes the key event to the appropriate component then our handler.
func (m Model) routeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.mode {
	case modeDetail, modeConfirmDelete:
		var vpCmd tea.Cmd
		m.detailViewport, vpCmd = m.detailViewport.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(vpCmd, ourCmd)

	case modeComment:
		var inputCmd tea.Cmd
		m.commentInput, inputCmd = m.commentInput.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(inputCmd, ourCmd)

	case modeStats:
		var vpCmd tea.Cmd
		m.statsViewport, vpCmd = m.statsViewport.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(vpCmd, ourCmd)

	default: // modeDashboard
		if m.activeSection == sectionAlerts && m.connected {
			var tableCmd tea.Cmd
			m.alertTable, tableCmd = m.alertTable.Update(msg)
			m2, ourCmd := m.handleKey(msg)
			return m2, tea.Batch(tableCmd, ourCmd)
		}
		return m.handleKey(msg)
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.mode {
	case modeDetail:
		return m.handleDetailKey(msg)
	case modeComment:
		return m.handleCommentKey(msg)
	case modeConfirmDelete:
		return m.handleConfirmKey(msg)
	case modeStats:
		return m.handleStatsKey(msg)
	default:
		return m.handleDashboardKey(msg)
	}
}

func (m Model) handleDashboardKey(msg tea.KeyMsg) (Model, tea.Cmd) {
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

	case "enter":
		if m.activeSection != sectionAlerts || len(m.alerts) == 0 {
			return m, nil
		}
		cursor := m.alertTable.Cursor()
		if cursor >= len(m.alerts) {
			return m, nil
		}
		m.selectedAlert = m.alerts[cursor]
		m.mode = modeDetail
		m.commentCursor = -1
		m.detailLoading = true
		m.detailViewport = viewport.New(m.width, m.detailViewportHeight())
		return m, fetchAlertDetailCmd(m.client, m.selectedAlert.ID)
	}

	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.mode = modeDashboard
		m.statusMsg = ""
		return m, nil

	case "a":
		if m.selectedAlert.AcknowledgedByID != nil {
			m.statusMsg = "already acknowledged"
			return m, clearStatusCmd()
		}
		return m, acknowledgeCmd(m.client, m.selectedAlert.ID)

	case "A":
		if m.selectedAlert.AcknowledgedByID == nil {
			m.statusMsg = "not acknowledged"
			return m, clearStatusCmd()
		}
		return m, unacknowledgeCmd(m.client, m.selectedAlert.ID)

	case "c":
		m.mode = modeComment
		m.commentInput.Reset()
		m.commentInput.Focus()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, nil

	case "d":
		if m.commentCursor < 0 || m.commentCursor >= len(m.comments) {
			m.statusMsg = "select a comment first with [ / ]"
			return m, clearStatusCmd()
		}
		m.pendingDeleteID = m.comments[m.commentCursor].ID
		m.mode = modeConfirmDelete
		return m, nil

	case "s":
		m.statusMsg = "assign not yet supported by server"
		return m, clearStatusCmd()

	case "S":
		m.mode = modeStats
		m.statsLoading = true
		m.statsViewport = viewport.New(m.width, m.height-5)
		return m, fetchDetailStatsCmd(m.client)

	case "[":
		if len(m.comments) == 0 {
			return m, nil
		}
		if m.commentCursor <= 0 {
			m.commentCursor = len(m.comments) - 1
		} else {
			m.commentCursor--
		}
		m.refreshDetailContent()
		return m, nil

	case "]":
		if len(m.comments) == 0 {
			return m, nil
		}
		if m.commentCursor >= len(m.comments)-1 {
			m.commentCursor = 0
		} else {
			m.commentCursor++
		}
		m.refreshDetailContent()
		return m, nil
	}

	return m, nil
}

func (m Model) handleCommentKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeDetail
		m.commentInput.Blur()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, nil

	case "enter":
		content := strings.TrimSpace(m.commentInput.Value())
		if content == "" {
			return m, nil
		}
		m.mode = modeDetail
		m.commentInput.Blur()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, addCommentCmd(m.client, m.selectedAlert.ID, content)
	}

	return m, nil
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		alertID := m.selectedAlert.ID
		commentID := m.pendingDeleteID
		m.mode = modeDetail
		m.commentCursor = -1
		m.pendingDeleteID = 0
		return m, deleteCommentCmd(m.client, alertID, commentID)
	default:
		m.mode = modeDetail
		m.pendingDeleteID = 0
		return m, nil
	}
}

func (m Model) handleStatsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.mode = modeDetail
		return m, nil
	}
	return m, nil
}
