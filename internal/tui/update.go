package tui

import (
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/yeniklas/terdut-tui/internal/api"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildTable()
		m.rebuildArchivedTable()
		m.rebuildScheduleTable()
		m.rebuildUserPickerTable()
		m.rebuildUserManageTable()
		m.detailViewport.Width = m.width
		m.detailViewport.Height = m.detailViewportHeight()
		m.statsViewport.Width = m.width
		m.statsViewport.Height = m.height - 5
		m.refreshDetailContent()
		m.refreshStatsContent()
		return m, nil

	// ── Dashboard messages ────────────────────────────────────────────────

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

	case archivedAlertsFetchedMsg:
		m.archivedAlerts = msg.alerts
		m.archivedLoading = false
		m.rebuildArchivedTable()
		return m, nil

	case alertArchivedMsg:
		m.alerts = msg.alerts
		m.loading = false
		m.rebuildTable()
		m.mode = modeDashboard
		m.statusMsg = "Alert archived"
		return m, clearStatusCmd()

	case alertUnarchivedMsg:
		m.archivedAlerts = msg.alerts
		m.archivedLoading = false
		m.rebuildArchivedTable()
		m.mode = modeDashboard
		m.statusMsg = "Alert unarchived"
		return m, clearStatusCmd()

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

	// ── Detail messages ───────────────────────────────────────────────────

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

	// ── Schedule messages ─────────────────────────────────────────────────

	case scheduleFetchedMsg:
		m.scheduleEntries = msg.entries
		m.currentOnCall = msg.current
		m.scheduleDays = buildScheduleDays(m.scheduleWindow, msg.entries)
		m.scheduleLoading = false
		m.rebuildScheduleTable()
		return m, nil

	case scheduleFetchErrMsg:
		m.scheduleLoading = false
		m.statusMsg = "schedule error: " + msg.err.Error()
		return m, clearStatusCmd()

	case scheduleActionErrMsg:
		m.scheduleLoading = false
		m.statusMsg = "error: " + msg.err.Error()
		return m, clearStatusCmd()

	case usersFetchedMsg:
		m.users = msg.users
		m.usersLoading = false
		m.rebuildUserPickerTable()
		m.rebuildUserManageTable()
		return m, nil

	case apiKeyCreatedMsg:
		m.revealedAPIKey = msg.key
		m.mode = modeAPIKeyReveal
		return m, nil

	case apiKeyRevokedMsg:
		m.statusMsg = "API key revoked"
		m.mode = modeDashboard
		return m, clearStatusCmd()

	case userActionErrMsg:
		m.usersLoading = false
		m.statusMsg = "error: " + msg.err.Error()
		m.mode = modeDashboard
		return m, clearStatusCmd()

	// ── Common ────────────────────────────────────────────────────────────

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		return m.routeKey(msg)
	}

	return m, nil
}

// routeKey passes the key to the active component then to our handler.
func (m Model) routeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.mode {
	case modeDetail:
		var vpCmd tea.Cmd
		m.detailViewport, vpCmd = m.detailViewport.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(vpCmd, ourCmd)

	case modeConfirmDelete:
		// No component to scroll; just handle y/n.
		return m.handleKey(msg)

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

	case modeScheduleUserPicker:
		var tableCmd tea.Cmd
		m.userPickerTable, tableCmd = m.userPickerTable.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(tableCmd, ourCmd)

	case modeUserCreate:
		var inputCmd tea.Cmd
		m.userFormInputs[m.userFormFocus], inputCmd = m.userFormInputs[m.userFormFocus].Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(inputCmd, ourCmd)

	case modeAPIKeyCreate:
		var inputCmd tea.Cmd
		m.apiKeyNameInput, inputCmd = m.apiKeyNameInput.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(inputCmd, ourCmd)

	case modeAPIKeyRevokeByID:
		var inputCmd tea.Cmd
		m.apiKeyRevokeInput, inputCmd = m.apiKeyRevokeInput.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(inputCmd, ourCmd)

	case modeAPIKeyMenu, modeAPIKeyReveal:
		return m.handleKey(msg)

	default: // modeDashboard
		if m.connected {
			switch m.activeSection {
			case sectionAlerts:
				var tableCmd tea.Cmd
				m.alertTable, tableCmd = m.alertTable.Update(msg)
				m2, ourCmd := m.handleKey(msg)
				return m2, tea.Batch(tableCmd, ourCmd)
			case sectionArchived:
				var tableCmd tea.Cmd
				m.archivedTable, tableCmd = m.archivedTable.Update(msg)
				m2, ourCmd := m.handleKey(msg)
				return m2, tea.Batch(tableCmd, ourCmd)
			case sectionSchedule:
				var tableCmd tea.Cmd
				m.scheduleTable, tableCmd = m.scheduleTable.Update(msg)
				m2, ourCmd := m.handleKey(msg)
				return m2, tea.Batch(tableCmd, ourCmd)
			case sectionUsers:
				var tableCmd tea.Cmd
				m.userManageTable, tableCmd = m.userManageTable.Update(msg)
				m2, ourCmd := m.handleKey(msg)
				return m2, tea.Batch(tableCmd, ourCmd)
			}
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
	case modeScheduleUserPicker:
		return m.handleUserPickerKey(msg)
	case modeUserCreate:
		return m.handleUserCreateKey(msg)
	case modeAPIKeyMenu:
		return m.handleAPIKeyMenuKey(msg)
	case modeAPIKeyCreate:
		return m.handleAPIKeyCreateKey(msg)
	case modeAPIKeyReveal:
		return m.handleAPIKeyRevealKey(msg)
	case modeAPIKeyRevokeByID:
		return m.handleAPIKeyRevokeKey(msg)
	default:
		return m.handleDashboardKey(msg)
	}
}

// ── Dashboard ─────────────────────────────────────────────────────────────

func (m Model) handleDashboardKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		next := section((int(m.activeSection) + 1) % 4)
		m.activeSection = next
		if next == sectionArchived && len(m.archivedAlerts) == 0 {
			m.archivedLoading = true
			return m, fetchArchivedAlertsCmd(m.client)
		}
		if next == sectionSchedule && len(m.scheduleDays) == 0 {
			m.scheduleLoading = true
			from := m.scheduleWindow
			to := m.scheduleWindow.AddDate(0, 0, 6)
			return m, fetchScheduleCmd(m.client, from, to)
		}
		if next == sectionUsers && len(m.users) == 0 {
			m.usersLoading = true
			return m, fetchUsersCmd(m.client)
		}
		return m, nil

	case "r":
		if !m.connected {
			return m, connectCmd(m.client)
		}
		if m.activeSection == sectionArchived {
			m.archivedLoading = true
			return m, fetchArchivedAlertsCmd(m.client)
		}
		if m.activeSection == sectionSchedule {
			m.scheduleLoading = true
			from := m.scheduleWindow
			to := m.scheduleWindow.AddDate(0, 0, 6)
			return m, fetchScheduleCmd(m.client, from, to)
		}
		if m.activeSection == sectionUsers {
			m.usersLoading = true
			return m, fetchUsersCmd(m.client)
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
		if m.activeSection == sectionAlerts && len(m.alerts) > 0 {
			cursor := m.alertTable.Cursor()
			if cursor >= 0 && cursor < len(m.alerts) {
				m.selectedAlert = m.alerts[cursor]
				m.mode = modeDetail
				m.commentCursor = -1
				m.detailLoading = true
				m.detailViewport = viewport.New(m.width, m.detailViewportHeight())
				return m, fetchAlertDetailCmd(m.client, m.selectedAlert.ID)
			}
		}
		if m.activeSection == sectionArchived && len(m.archivedAlerts) > 0 {
			cursor := m.archivedTable.Cursor()
			if cursor >= 0 && cursor < len(m.archivedAlerts) {
				m.selectedAlert = m.archivedAlerts[cursor]
				m.mode = modeDetail
				m.commentCursor = -1
				m.detailLoading = true
				m.detailViewport = viewport.New(m.width, m.detailViewportHeight())
				return m, fetchAlertDetailCmd(m.client, m.selectedAlert.ID)
			}
		}
		return m, nil

	case "x":
		switch m.activeSection {
		case sectionAlerts:
			cursor := m.alertTable.Cursor()
			if cursor < 0 || cursor >= len(m.alerts) {
				return m, nil
			}
			return m, archiveAlertCmd(m.client, m.alerts[cursor].ID, m.filterStatus)
		case sectionArchived:
			cursor := m.archivedTable.Cursor()
			if cursor < 0 || cursor >= len(m.archivedAlerts) {
				return m, nil
			}
			return m, unarchiveAlertCmd(m.client, m.archivedAlerts[cursor].ID)
		}
		return m, nil

	// Schedule-specific keys
	case "left", "h":
		if m.activeSection == sectionSchedule {
			m.scheduleWindow = m.scheduleWindow.AddDate(0, 0, -7)
			m.scheduleLoading = true
			from := m.scheduleWindow
			to := m.scheduleWindow.AddDate(0, 0, 6)
			return m, fetchScheduleCmd(m.client, from, to)
		}
		return m, nil

	case "right", "l":
		if m.activeSection == sectionSchedule {
			m.scheduleWindow = m.scheduleWindow.AddDate(0, 0, 7)
			m.scheduleLoading = true
			from := m.scheduleWindow
			to := m.scheduleWindow.AddDate(0, 0, 6)
			return m, fetchScheduleCmd(m.client, from, to)
		}
		return m, nil

	case "+":
		if m.activeSection != sectionSchedule || !m.connected {
			return m, nil
		}
		m.pickerAssignWeek = false
		m.mode = modeScheduleUserPicker
		if len(m.users) == 0 {
			m.usersLoading = true
			return m, fetchUsersCmd(m.client)
		}
		m.rebuildUserPickerTable()
		return m, nil

	case "W":
		if m.activeSection != sectionSchedule || !m.connected {
			return m, nil
		}
		m.pickerAssignWeek = true
		m.mode = modeScheduleUserPicker
		if len(m.users) == 0 {
			m.usersLoading = true
			return m, fetchUsersCmd(m.client)
		}
		m.rebuildUserPickerTable()
		return m, nil

	case "d":
		switch m.activeSection {
		case sectionSchedule:
			if !m.connected {
				return m, nil
			}
			cursor := m.scheduleTable.Cursor()
			if cursor >= len(m.scheduleDays) {
				return m, nil
			}
			day := m.scheduleDays[cursor]
			if day.entry == nil {
				m.statusMsg = "no assignment to delete on this date"
				return m, clearStatusCmd()
			}
			m.pendingDeleteEntry = day.entry
			m.confirmTarget = confirmDeleteScheduleEntry
			m.mode = modeConfirmDelete
		case sectionUsers:
			if !m.connected || len(m.users) == 0 {
				return m, nil
			}
			cursor := m.userManageTable.Cursor()
			if cursor >= len(m.users) {
				return m, nil
			}
			m.selectedUser = m.users[cursor]
			m.confirmTarget = confirmDeleteUser
			m.mode = modeConfirmDelete
		}
		return m, nil

	case "n":
		if m.activeSection != sectionUsers || !m.connected {
			return m, nil
		}
		m.userFormInputs[0].Reset()
		m.userFormInputs[1].Reset()
		m.userFormFocus = 0
		m.userFormInputs[0].Focus()
		m.userFormInputs[1].Blur()
		m.mode = modeUserCreate
		return m, nil

	case "k":
		if m.activeSection != sectionUsers || !m.connected || len(m.users) == 0 {
			return m, nil
		}
		cursor := m.userManageTable.Cursor()
		if cursor >= len(m.users) {
			return m, nil
		}
		m.selectedUser = m.users[cursor]
		m.mode = modeAPIKeyMenu
		return m, nil
	}

	return m, nil
}

// ── Detail ────────────────────────────────────────────────────────────────

func (m Model) handleDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.mode = modeDashboard
		m.statusMsg = ""
		return m, nil

	case "a":
		if m.activeSection != sectionAlerts {
			return m, nil
		}
		if m.selectedAlert.AcknowledgedByID != nil {
			m.statusMsg = "already acknowledged"
			return m, clearStatusCmd()
		}
		return m, acknowledgeCmd(m.client, m.selectedAlert.ID)

	case "A":
		if m.activeSection != sectionAlerts {
			return m, nil
		}
		if m.selectedAlert.AcknowledgedByID == nil {
			m.statusMsg = "not acknowledged"
			return m, clearStatusCmd()
		}
		return m, unacknowledgeCmd(m.client, m.selectedAlert.ID)

	case "x":
		if m.activeSection == sectionAlerts {
			return m, archiveAlertCmd(m.client, m.selectedAlert.ID, m.filterStatus)
		}
		if m.activeSection == sectionArchived {
			return m, unarchiveAlertCmd(m.client, m.selectedAlert.ID)
		}
		return m, nil

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
		m.confirmTarget = confirmDeleteComment
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

// ── Comment compose ───────────────────────────────────────────────────────

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

// ── Confirm delete ────────────────────────────────────────────────────────

func (m Model) handleConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y":
		switch m.confirmTarget {
		case confirmDeleteComment:
			alertID := m.selectedAlert.ID
			commentID := m.pendingDeleteID
			m.mode = modeDetail
			m.commentCursor = -1
			m.pendingDeleteID = 0
			return m, deleteCommentCmd(m.client, alertID, commentID)
		case confirmDeleteScheduleEntry:
			entry := m.pendingDeleteEntry
			m.mode = modeDashboard
			m.pendingDeleteEntry = nil
			m.scheduleLoading = true
			from := m.scheduleWindow
			to := m.scheduleWindow.AddDate(0, 0, 6)
			return m, deleteScheduleEntryCmd(m.client, entry.ID, from, to)
		case confirmDeleteUser:
			userID := m.selectedUser.ID
			m.mode = modeDashboard
			m.usersLoading = true
			return m, deleteUserCmd(m.client, userID)
		}
	default:
		switch m.confirmTarget {
		case confirmDeleteComment:
			m.mode = modeDetail
		case confirmDeleteScheduleEntry:
			m.mode = modeDashboard
		case confirmDeleteUser:
			m.mode = modeDashboard
		}
		m.pendingDeleteID = 0
		m.pendingDeleteEntry = nil
	}
	return m, nil
}

// ── Stats ─────────────────────────────────────────────────────────────────

func (m Model) handleStatsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.mode = modeDetail
		return m, nil
	}
	return m, nil
}

// ── User picker ───────────────────────────────────────────────────────────

func (m Model) handleUserPickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeDashboard
		return m, nil

	case "enter":
		cursor := m.userPickerTable.Cursor()
		if cursor >= len(m.users) {
			return m, nil
		}
		user := m.users[cursor]
		scheduleCursor := m.scheduleTable.Cursor()
		if scheduleCursor >= len(m.scheduleDays) {
			m.mode = modeDashboard
			return m, nil
		}
		d := m.scheduleDays[scheduleCursor].date
		var dates []string
		if m.pickerAssignWeek {
			weekday := int(d.Weekday())
			if weekday == 0 {
				weekday = 7 // ISO: Sunday = 7
			}
			monday := d.AddDate(0, 0, -(weekday - 1))
			for i := 0; i < 7; i++ {
				dates = append(dates, monday.AddDate(0, 0, i).Format("2006-01-02"))
			}
		} else {
			dates = []string{d.Format("2006-01-02")}
		}
		from := m.scheduleWindow
		to := m.scheduleWindow.AddDate(0, 0, 6)
		m.mode = modeDashboard
		m.scheduleLoading = true
		return m, assignScheduleCmd(m.client, user.ID, dates, from, to)
	}

	return m, nil
}

// ── User management ───────────────────────────────────────────────────────────

func (m Model) handleUserCreateKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.userFormInputs[0].Blur()
		m.userFormInputs[1].Blur()
		m.mode = modeDashboard
		return m, nil

	case "tab", "shift+tab":
		m.userFormInputs[m.userFormFocus].Blur()
		m.userFormFocus = (m.userFormFocus + 1) % 2
		m.userFormInputs[m.userFormFocus].Focus()
		return m, nil

	case "enter":
		username := strings.TrimSpace(m.userFormInputs[0].Value())
		email := strings.TrimSpace(m.userFormInputs[1].Value())
		if username == "" || email == "" {
			m.statusMsg = "username and email are required"
			return m, clearStatusCmd()
		}
		m.userFormInputs[0].Blur()
		m.userFormInputs[1].Blur()
		m.mode = modeDashboard
		m.usersLoading = true
		return m, createUserCmd(m.client, username, email)
	}

	return m, nil
}

func (m Model) handleAPIKeyMenuKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeDashboard
		return m, nil

	case "n":
		m.apiKeyNameInput.Reset()
		m.apiKeyNameInput.Focus()
		m.mode = modeAPIKeyCreate
		return m, nil

	case "r":
		m.apiKeyRevokeInput.Reset()
		m.apiKeyRevokeInput.Focus()
		m.mode = modeAPIKeyRevokeByID
		return m, nil
	}

	return m, nil
}

func (m Model) handleAPIKeyCreateKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.apiKeyNameInput.Blur()
		m.mode = modeAPIKeyMenu
		return m, nil

	case "enter":
		name := strings.TrimSpace(m.apiKeyNameInput.Value())
		if name == "" {
			m.statusMsg = "key name is required"
			return m, clearStatusCmd()
		}
		m.apiKeyNameInput.Blur()
		m.mode = modeDashboard
		return m, createAPIKeyCmd(m.client, m.selectedUser.ID, name)
	}

	return m, nil
}

func (m Model) handleAPIKeyRevealKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		if err := clipboard.WriteAll(m.revealedAPIKey.Key); err != nil {
			m.statusMsg = "clipboard error: " + err.Error()
		} else {
			m.statusMsg = "copied to clipboard"
		}
		return m, clearStatusCmd()

	case "esc", "enter", "q":
		m.revealedAPIKey = api.APIKey{}
		m.mode = modeDashboard
		return m, nil
	}

	return m, nil
}

func (m Model) handleAPIKeyRevokeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.apiKeyRevokeInput.Blur()
		m.mode = modeAPIKeyMenu
		return m, nil

	case "enter":
		raw := strings.TrimSpace(m.apiKeyRevokeInput.Value())
		keyID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || keyID <= 0 {
			m.statusMsg = "invalid key ID — must be a positive integer"
			return m, clearStatusCmd()
		}
		m.apiKeyRevokeInput.Blur()
		m.mode = modeDashboard
		return m, deleteAPIKeyCmd(m.client, m.selectedUser.ID, keyID)
	}

	return m, nil
}
