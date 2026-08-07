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
		m.rebuildIncidentTable()
		m.rebuildTable()
		m.rebuildArchivedTable()
		m.rebuildScheduleTable()
		m.rebuildUserPickerTable()
		m.rebuildUserManageTable()
		m.detailViewport.Width = m.width
		m.detailViewport.Height = m.detailViewportHeight()
		m.statsViewport.Width = m.width
		m.statsViewport.Height = m.statsViewportHeight()
		m.refreshDetailContent()
		m.refreshStatsContent()
		return m, nil

	// ── Dashboard messages ────────────────────────────────────────────────

	case connectedMsg:
		m.connected = true
		m.err = nil
		return m, tea.Batch(
			tickCmd(m.refreshInterval),
			fetchIncidentsCmd(m.client, m.incidentFilter),
			fetchStatsCmd(m.client),
		)

	case connectErrMsg:
		m.connected = false
		m.err = msg.err
		return m, nil

	case incidentsFetchedMsg:
		m.incidents = msg.incidents
		m.loading = false
		m.rebuildIncidentTable()
		return m, nil

	case archivedIncidentsFetchedMsg:
		m.archivedIncidents = msg.incidents
		m.archivedLoading = false
		m.rebuildArchivedTable()
		m.mode = modeDashboard
		return m, nil

	case incidentActionDoneMsg:
		m.incidents = msg.incidents
		m.loading = false
		m.rebuildIncidentTable()
		m.mode = modeDashboard
		m.statusMsg = msg.status
		return m, clearStatusCmd()

	case alertsFetchedMsg:
		m.alerts = msg.alerts
		m.loading = false
		m.rebuildTable()
		return m, nil

	case statsFetchedMsg:
		m.incidentStats = &msg.incidents
		m.alertStats = &msg.alerts
		return m, nil

	case fetchDataErrMsg:
		m.loading = false
		m.archivedLoading = false
		m.statusMsg = "refresh error: " + msg.err.Error()
		return m, clearStatusCmd()

	case tickMsg:
		return m, tea.Batch(tickCmd(m.refreshInterval), m.refreshActiveSection())

	// ── Detail messages ───────────────────────────────────────────────────

	case incidentDetailFetchedMsg:
		m.selectedIncident = msg.incident
		m.timeline = msg.timeline
		m.detailLoading = false
		if m.noteCursor >= len(noteEvents(m.timeline)) {
			m.noteCursor = -1
		}
		m.refreshDetailContent()
		return m, nil

	case alertDetailFetchedMsg:
		m.selectedAlert = msg.alert
		m.detailLoading = false
		m.refreshDetailContent()
		return m, nil

	case detailErrMsg:
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
		m.statsLoaded = true
		m.refreshStatsContent()
		return m, nil

	case detailStatsErrMsg:
		m.statsLoading = false
		// Mark it loaded even on failure, so tabbing back in does not re-fire the
		// request every time. The tick and r still retry.
		m.statsLoaded = true
		m.statusMsg = "stats error: " + msg.err.Error()
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

// refreshActiveSection reloads whatever the auto-refresh tick should keep fresh.
// Detail views reload too — an incident someone else acknowledged should stop
// looking unclaimed while you are staring at it.
func (m Model) refreshActiveSection() tea.Cmd {
	switch m.mode {
	case modeIncidentDetail:
		return fetchIncidentDetailCmd(m.client, m.selectedIncident.ID)
	case modeAlertDetail:
		return fetchAlertDetailCmd(m.client, m.selectedAlert.ID)
	case modeDashboard:
		// fall through to the section refresh below
	default:
		// Modal states (compose, pickers, confirmations) are left alone: a
		// refresh underneath a prompt would move the ground under the user.
		return nil
	}

	switch m.activeSection {
	case sectionIncidents:
		return tea.Batch(fetchIncidentsCmd(m.client, m.incidentFilter), fetchStatsCmd(m.client))
	case sectionAlerts:
		return tea.Batch(fetchAlertsCmd(m.client, m.alertFilter), fetchStatsCmd(m.client))
	case sectionStats:
		// Both: fetchStatsCmd feeds the Incident Response block, the other the charts.
		return tea.Batch(fetchStatsCmd(m.client), fetchDetailStatsCmd(m.client))
	case sectionArchived:
		return fetchArchivedIncidentsCmd(m.client)
	case sectionSchedule:
		return fetchScheduleCmd(m.client, m.scheduleWindow, m.scheduleWindow.AddDate(0, 0, 6))
	case sectionUsers:
		return fetchUsersCmd(m.client)
	}
	return nil
}

// routeKey passes the key to the active component then to our handler.
func (m Model) routeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.mode {
	case modeIncidentDetail, modeAlertDetail:
		var vpCmd tea.Cmd
		m.detailViewport, vpCmd = m.detailViewport.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(vpCmd, ourCmd)

	case modeConfirm:
		// No component to scroll; just handle y/n.
		return m.handleKey(msg)

	case modeNote:
		var inputCmd tea.Cmd
		m.noteInput, inputCmd = m.noteInput.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(inputCmd, ourCmd)

	case modeSnooze:
		var inputCmd tea.Cmd
		m.snoozeInput, inputCmd = m.snoozeInput.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(inputCmd, ourCmd)

	case modeUserPicker:
		var tableCmd tea.Cmd
		m.userPickerTable, tableCmd = m.userPickerTable.Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(tableCmd, ourCmd)

	case modeUserCreate:
		var inputCmd tea.Cmd
		m.userFormInputs[m.userFormFocus], inputCmd = m.userFormInputs[m.userFormFocus].Update(msg)
		m2, ourCmd := m.handleKey(msg)
		return m2, tea.Batch(inputCmd, ourCmd)

	case modeUserNotifyEdit:
		var inputCmd tea.Cmd
		m.ntfyTopicInput, inputCmd = m.ntfyTopicInput.Update(msg)
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
			case sectionIncidents:
				var tableCmd tea.Cmd
				m.incidentTable, tableCmd = m.incidentTable.Update(msg)
				m2, ourCmd := m.handleKey(msg)
				return m2, tea.Batch(tableCmd, ourCmd)
			case sectionAlerts:
				var tableCmd tea.Cmd
				m.alertTable, tableCmd = m.alertTable.Update(msg)
				m2, ourCmd := m.handleKey(msg)
				return m2, tea.Batch(tableCmd, ourCmd)
			case sectionStats:
				var vpCmd tea.Cmd
				m.statsViewport, vpCmd = m.statsViewport.Update(msg)
				m2, ourCmd := m.handleKey(msg)
				return m2, tea.Batch(vpCmd, ourCmd)
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
	case modeIncidentDetail:
		return m.handleIncidentDetailKey(msg)
	case modeAlertDetail:
		return m.handleAlertDetailKey(msg)
	case modeNote:
		return m.handleNoteKey(msg)
	case modeSnooze:
		return m.handleSnoozeKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	case modeUserPicker:
		return m.handleUserPickerKey(msg)
	case modeUserCreate:
		return m.handleUserCreateKey(msg)
	case modeUserNotifyEdit:
		return m.handleUserNotifyEditKey(msg)
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
		m.activeSection = section((int(m.activeSection) + 1) % sectionCount)
		return m, m.loadSectionIfEmpty()

	case "shift+tab":
		m.activeSection = section((int(m.activeSection) + sectionCount - 1) % sectionCount)
		return m, m.loadSectionIfEmpty()

	case "r":
		if !m.connected {
			return m, connectCmd(m.client)
		}
		m.statusMsg = "Refreshing…"
		return m, tea.Batch(m.refreshActiveSection(), clearStatusCmd())

	case "f":
		if !m.connected {
			return m, nil
		}
		switch m.activeSection {
		case sectionIncidents:
			m.incidentFilter = nextFilter(incidentFilters, m.incidentFilter)
			m.loading = true
			return m, fetchIncidentsCmd(m.client, m.incidentFilter)
		case sectionAlerts:
			m.alertFilter = nextFilter(alertFilters, m.alertFilter)
			m.loading = true
			return m, fetchAlertsCmd(m.client, m.alertFilter)
		}
		return m, nil

	case "enter":
		switch m.activeSection {
		case sectionIncidents:
			if inc, ok := m.incidentAtCursor(); ok {
				return m.openIncident(inc)
			}
		case sectionArchived:
			if i := m.archivedTable.Cursor(); i >= 0 && i < len(m.archivedIncidents) {
				return m.openIncident(m.archivedIncidents[i])
			}
		case sectionAlerts:
			if i := m.alertTable.Cursor(); i >= 0 && i < len(m.alerts) {
				m.selectedAlert = m.alerts[i]
				m.mode = modeAlertDetail
				m.detailLoading = true
				m.detailViewport = viewport.New(m.width, m.detailViewportHeight())
				return m, fetchAlertDetailCmd(m.client, m.selectedAlert.ID)
			}
		}
		return m, nil

	case "x":
		switch m.activeSection {
		case sectionIncidents:
			inc, ok := m.incidentAtCursor()
			if !ok {
				return m, nil
			}
			if inc.IsOpen() {
				m.statusMsg = "resolve the incident before archiving it"
				return m, clearStatusCmd()
			}
			return m, archiveIncidentCmd(m.client, inc.ID, m.incidentFilter)
		case sectionArchived:
			i := m.archivedTable.Cursor()
			if i < 0 || i >= len(m.archivedIncidents) {
				return m, nil
			}
			m.archivedLoading = true
			return m, unarchiveIncidentCmd(m.client, m.archivedIncidents[i].ID)
		}
		return m, nil

	// Schedule-specific keys
	case "left", "h":
		if m.activeSection == sectionSchedule {
			m.scheduleWindow = m.scheduleWindow.AddDate(0, 0, -7)
			m.scheduleLoading = true
			return m, fetchScheduleCmd(m.client, m.scheduleWindow, m.scheduleWindow.AddDate(0, 0, 6))
		}
		return m, nil

	case "right", "l":
		if m.activeSection == sectionSchedule {
			m.scheduleWindow = m.scheduleWindow.AddDate(0, 0, 7)
			m.scheduleLoading = true
			return m, fetchScheduleCmd(m.client, m.scheduleWindow, m.scheduleWindow.AddDate(0, 0, 6))
		}
		return m, nil

	case "+":
		if m.activeSection != sectionSchedule || !m.connected {
			return m, nil
		}
		m.pickerAssignWeek = false
		return m.openUserPicker(pickerSchedule)

	case "W":
		if m.activeSection != sectionSchedule || !m.connected {
			return m, nil
		}
		m.pickerAssignWeek = true
		return m.openUserPicker(pickerSchedule)

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
			m.mode = modeConfirm
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
			m.mode = modeConfirm
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

	case "t":
		if m.activeSection != sectionUsers || !m.connected || len(m.users) == 0 {
			return m, nil
		}
		cursor := m.userManageTable.Cursor()
		if cursor >= len(m.users) {
			return m, nil
		}
		m.selectedUser = m.users[cursor]
		// Prefilled with what they have, so editing a topic does not mean
		// retyping it, and clearing one is a deliberate wipe.
		m.ntfyTopicInput.SetValue(m.selectedUser.Topic())
		m.ntfyTopicInput.CursorEnd()
		m.ntfyTopicInput.Focus()
		m.mode = modeUserNotifyEdit
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

// loadSectionIfEmpty fetches a section's data the first time it is opened.
func (m *Model) loadSectionIfEmpty() tea.Cmd {
	switch m.activeSection {
	case sectionAlerts:
		if len(m.alerts) == 0 {
			m.loading = true
			return fetchAlertsCmd(m.client, m.alertFilter)
		}
	case sectionStats:
		if !m.statsLoaded {
			m.statsLoading = true
			return tea.Batch(fetchStatsCmd(m.client), fetchDetailStatsCmd(m.client))
		}
	case sectionArchived:
		if len(m.archivedIncidents) == 0 {
			m.archivedLoading = true
			return fetchArchivedIncidentsCmd(m.client)
		}
	case sectionSchedule:
		if len(m.scheduleDays) == 0 {
			m.scheduleLoading = true
			return fetchScheduleCmd(m.client, m.scheduleWindow, m.scheduleWindow.AddDate(0, 0, 6))
		}
	case sectionUsers:
		if len(m.users) == 0 {
			m.usersLoading = true
			return fetchUsersCmd(m.client)
		}
	}
	return nil
}

func (m Model) incidentAtCursor() (api.Incident, bool) {
	i := m.incidentTable.Cursor()
	if i < 0 || i >= len(m.incidents) {
		return api.Incident{}, false
	}
	return m.incidents[i], true
}

func (m Model) openIncident(inc api.Incident) (Model, tea.Cmd) {
	m.selectedIncident = inc
	m.mode = modeIncidentDetail
	m.noteCursor = -1
	m.detailLoading = true
	m.detailViewport = viewport.New(m.width, m.detailViewportHeight())
	return m, fetchIncidentDetailCmd(m.client, inc.ID)
}

func (m Model) openUserPicker(target pickerTarget) (Model, tea.Cmd) {
	m.pickerTarget = target
	m.mode = modeUserPicker
	if len(m.users) == 0 {
		m.usersLoading = true
		return m, fetchUsersCmd(m.client)
	}
	m.rebuildUserPickerTable()
	return m, nil
}

// nextFilter advances a filter cycle, wrapping at the end.
func nextFilter(cycle []string, current string) string {
	for i, f := range cycle {
		if f == current {
			return cycle[(i+1)%len(cycle)]
		}
	}
	return cycle[0]
}

// ── Incident detail ───────────────────────────────────────────────────────

func (m Model) handleIncidentDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	inc := m.selectedIncident

	switch msg.String() {
	case "esc", "backspace":
		m.mode = modeDashboard
		m.statusMsg = ""
		return m, nil

	case "a":
		if !inc.IsOpen() {
			return m.rejectClosed()
		}
		if inc.AcknowledgedByID != nil {
			m.statusMsg = "already acknowledged by " + inc.AcknowledgedBy
			return m, clearStatusCmd()
		}
		return m, acknowledgeIncidentCmd(m.client, inc.ID)

	case "A":
		if !inc.IsOpen() {
			return m.rejectClosed()
		}
		if inc.AcknowledgedByID == nil {
			m.statusMsg = "not acknowledged"
			return m, clearStatusCmd()
		}
		return m, unacknowledgeIncidentCmd(m.client, inc.ID)

	case "R":
		if !inc.IsOpen() {
			return m.rejectClosed()
		}
		m.confirmTarget = confirmResolveIncident
		m.mode = modeConfirm
		return m, nil

	case "s":
		if !inc.IsOpen() {
			return m.rejectClosed()
		}
		return m.openUserPicker(pickerIncidentAssignee)

	case "z":
		if !inc.IsOpen() {
			return m.rejectClosed()
		}
		m.mode = modeSnooze
		m.snoozeInput.Reset()
		m.snoozeInput.Focus()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, nil

	case "Z":
		if !inc.IsOpen() {
			return m.rejectClosed()
		}
		if !inc.IsSnoozed() {
			m.statusMsg = "not snoozed"
			return m, clearStatusCmd()
		}
		return m, unsnoozeIncidentCmd(m.client, inc.ID)

	case "x":
		if inc.IsOpen() {
			m.statusMsg = "resolve the incident before archiving it"
			return m, clearStatusCmd()
		}
		if inc.ArchivedAt != nil {
			m.archivedLoading = true
			m.mode = modeDashboard
			return m, unarchiveIncidentCmd(m.client, inc.ID)
		}
		m.mode = modeDashboard
		return m, archiveIncidentCmd(m.client, inc.ID, m.incidentFilter)

	case "c":
		m.mode = modeNote
		m.noteInput.Reset()
		m.noteInput.Focus()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, nil

	case "d":
		notes := noteEvents(m.timeline)
		if m.noteCursor < 0 || m.noteCursor >= len(notes) {
			m.statusMsg = "select a note first with [ / ]"
			return m, clearStatusCmd()
		}
		m.pendingDeleteID = notes[m.noteCursor].ID
		m.confirmTarget = confirmDeleteNote
		m.mode = modeConfirm
		return m, nil

	case "[":
		return m.moveNoteCursor(-1), nil

	case "]":
		return m.moveNoteCursor(1), nil
	}

	return m, nil
}

// rejectClosed reports why an action did nothing on a resolved incident. The
// server answers 409 anyway; saying so up front is friendlier than a round trip.
func (m Model) rejectClosed() (Model, tea.Cmd) {
	m.statusMsg = "incident is resolved — a new occurrence opens a new incident"
	return m, clearStatusCmd()
}

func (m Model) moveNoteCursor(delta int) Model {
	notes := noteEvents(m.timeline)
	if len(notes) == 0 {
		return m
	}
	switch {
	case m.noteCursor < 0:
		if delta > 0 {
			m.noteCursor = 0
		} else {
			m.noteCursor = len(notes) - 1
		}
	default:
		m.noteCursor = (m.noteCursor + delta + len(notes)) % len(notes)
	}
	m.refreshDetailContent()
	return m
}

// ── Alert detail (read-only) ──────────────────────────────────────────────

func (m Model) handleAlertDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.mode = modeDashboard
		m.statusMsg = ""
		return m, nil

	case "i":
		// Jump to the incident this alert belongs to — the place where anything
		// can actually be done about it.
		if m.selectedAlert.IncidentID == nil {
			m.statusMsg = "this alert has no incident"
			return m, clearStatusCmd()
		}
		return m.openIncident(api.Incident{ID: *m.selectedAlert.IncidentID})
	}

	return m, nil
}

// ── Note compose ──────────────────────────────────────────────────────────

func (m Model) handleNoteKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeIncidentDetail
		m.noteInput.Blur()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, nil

	case "enter":
		content := strings.TrimSpace(m.noteInput.Value())
		if content == "" {
			return m, nil
		}
		m.mode = modeIncidentDetail
		m.noteInput.Blur()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, addNoteCmd(m.client, m.selectedIncident.ID, content)
	}

	return m, nil
}

// ── Snooze ────────────────────────────────────────────────────────────────

func (m Model) handleSnoozeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeIncidentDetail
		m.snoozeInput.Blur()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, nil

	case "enter":
		duration := strings.TrimSpace(m.snoozeInput.Value())
		if duration == "" {
			return m, nil
		}
		m.mode = modeIncidentDetail
		m.snoozeInput.Blur()
		m.detailViewport.Height = m.detailViewportHeight()
		return m, snoozeIncidentCmd(m.client, m.selectedIncident.ID, duration)
	}

	return m, nil
}

// ── Confirm ───────────────────────────────────────────────────────────────

func (m Model) handleConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if strings.ToLower(msg.String()) != "y" {
		switch m.confirmTarget {
		case confirmDeleteNote, confirmResolveIncident:
			m.mode = modeIncidentDetail
		default:
			m.mode = modeDashboard
		}
		m.pendingDeleteID = 0
		m.pendingDeleteEntry = nil
		return m, nil
	}

	switch m.confirmTarget {
	case confirmDeleteNote:
		incidentID := m.selectedIncident.ID
		eventID := m.pendingDeleteID
		m.mode = modeIncidentDetail
		m.noteCursor = -1
		m.pendingDeleteID = 0
		return m, deleteNoteCmd(m.client, incidentID, eventID)

	case confirmResolveIncident:
		m.mode = modeIncidentDetail
		return m, resolveIncidentCmd(m.client, m.selectedIncident.ID)

	case confirmDeleteScheduleEntry:
		entry := m.pendingDeleteEntry
		m.mode = modeDashboard
		m.pendingDeleteEntry = nil
		m.scheduleLoading = true
		return m, deleteScheduleEntryCmd(m.client, entry.ID,
			m.scheduleWindow, m.scheduleWindow.AddDate(0, 0, 6))

	case confirmDeleteUser:
		userID := m.selectedUser.ID
		m.mode = modeDashboard
		m.usersLoading = true
		return m, deleteUserCmd(m.client, userID)
	}

	return m, nil
}

// ── User picker ───────────────────────────────────────────────────────────

func (m Model) handleUserPickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.pickerTarget == pickerIncidentAssignee {
			m.mode = modeIncidentDetail
		} else {
			m.mode = modeDashboard
		}
		return m, nil

	case "enter":
		cursor := m.userPickerTable.Cursor()
		if cursor < 0 || cursor >= len(m.users) {
			return m, nil
		}
		user := m.users[cursor]

		if m.pickerTarget == pickerIncidentAssignee {
			m.mode = modeIncidentDetail
			return m, assignIncidentCmd(m.client, m.selectedIncident.ID, user.ID)
		}

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
		m.mode = modeDashboard
		m.scheduleLoading = true
		return m, assignScheduleCmd(m.client, user.ID, dates,
			m.scheduleWindow, m.scheduleWindow.AddDate(0, 0, 6))
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

// handleUserNotifyEditKey edits one user's ntfy topic.
//
// Unlike the other forms here, an empty value is not a mistake to reject: it is
// how a topic is cleared, which the server accepts and treats as NULL.
func (m Model) handleUserNotifyEditKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ntfyTopicInput.Blur()
		m.mode = modeDashboard
		return m, nil

	case "enter":
		topic := strings.TrimSpace(m.ntfyTopicInput.Value())
		m.ntfyTopicInput.Blur()
		m.mode = modeDashboard
		m.usersLoading = true
		return m, setUserNotifyTargetCmd(m.client, m.selectedUser.ID, topic)
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
