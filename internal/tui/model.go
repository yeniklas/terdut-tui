package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yeniklas/terdut-tui/internal/api"
)

// ── Enums ──────────────────────────────────────────────────────────────────

type section int

const (
	sectionAlerts section = iota
	sectionSchedule
	sectionUsers
)

type mode int

const (
	modeDashboard mode = iota
	modeDetail
	modeComment
	modeConfirmDelete
	modeStats
	modeScheduleUserPicker
	modeUserCreate
	modeAPIKeyMenu
	modeAPIKeyCreate
	modeAPIKeyReveal
	modeAPIKeyRevokeByID
)

type confirmTarget int

const (
	confirmDeleteComment confirmTarget = iota
	confirmDeleteScheduleEntry
	confirmDeleteUser
)

// ── Messages ───────────────────────────────────────────────────────────────

// dashboard
type connectedMsg struct{}
type connectErrMsg struct{ err error }
type alertsFetchedMsg struct{ alerts []api.Alert }
type statsFetchedMsg struct{ stats api.AlertStats }
type fetchDataErrMsg struct{ err error }
type tickMsg time.Time
type clearStatusMsg struct{}

// detail
type alertDetailFetchedMsg struct {
	alert    api.Alert
	comments []api.Comment
}
type alertDetailErrMsg struct{ err error }
type actionErrMsg struct{ err error }
type detailStatsFetchedMsg struct {
	top    []api.TopAlert
	byHour []api.HourStat
	byDay  []api.DayStat
}
type detailStatsErrMsg struct{ err error }

// schedule
type scheduleFetchedMsg struct {
	entries []api.ScheduleEntry
	current *api.ScheduleEntry
}
type scheduleFetchErrMsg struct{ err error }
type scheduleActionErrMsg struct{ err error }
type usersFetchedMsg struct{ users []api.User }

// user management
type apiKeyCreatedMsg struct{ key api.APIKey }
type apiKeyRevokedMsg struct{}
type userActionErrMsg struct{ err error }

// ── Model ──────────────────────────────────────────────────────────────────

type scheduleDay struct {
	date  time.Time
	entry *api.ScheduleEntry
}

type Model struct {
	client          *api.Client
	serverURL       string
	refreshInterval time.Duration

	activeSection section
	mode          mode
	width         int
	height        int

	// Connection & dashboard
	connected    bool
	loading      bool
	err          error
	statusMsg    string
	alerts       []api.Alert
	stats        *api.AlertStats
	filterStatus string
	alertTable   table.Model

	// Detail
	selectedAlert  api.Alert
	comments       []api.Comment
	commentCursor  int
	detailLoading  bool
	detailViewport viewport.Model

	// Comment compose
	commentInput textinput.Model

	// Confirm delete
	confirmTarget       confirmTarget
	pendingDeleteID     int64          // comment ID
	pendingDeleteEntry  *api.ScheduleEntry

	// Stats
	topAlerts     []api.TopAlert
	hourStats     []api.HourStat
	dayStats      []api.DayStat
	statsLoading  bool
	statsViewport viewport.Model

	// Schedule
	scheduleWindow  time.Time
	scheduleEntries []api.ScheduleEntry
	scheduleDays    []scheduleDay
	currentOnCall   *api.ScheduleEntry
	scheduleLoading bool
	scheduleTable   table.Model

	// User picker (schedule assignment)
	users           []api.User
	usersLoading    bool
	userPickerTable table.Model

	// User management section
	userManageTable   table.Model
	selectedUser      api.User
	userFormInputs    [2]textinput.Model
	userFormFocus     int
	apiKeyNameInput   textinput.Model
	apiKeyRevokeInput textinput.Model
	revealedAPIKey    api.APIKey

	help help.Model
	keys keyMap
}

func NewModel(client *api.Client, serverURL string, refreshInterval time.Duration) Model {
	ts := defaultTableStyles()

	alertT := table.New(table.WithFocused(true))
	alertT.SetStyles(ts)

	schedT := table.New(table.WithFocused(true))
	schedT.SetStyles(ts)

	pickerT := table.New(table.WithFocused(true))
	pickerT.SetStyles(ts)

	manageT := table.New(table.WithFocused(true))
	manageT.SetStyles(ts)

	ti := textinput.New()
	ti.Placeholder = "type your comment…"
	ti.CharLimit = 1000

	usernameIn := textinput.New()
	usernameIn.Placeholder = "username"
	usernameIn.CharLimit = 64

	emailIn := textinput.New()
	emailIn.Placeholder = "email"
	emailIn.CharLimit = 128

	keyNameIn := textinput.New()
	keyNameIn.Placeholder = "key name (e.g. laptop)"
	keyNameIn.CharLimit = 64

	revokeIn := textinput.New()
	revokeIn.Placeholder = "integer key ID"
	revokeIn.CharLimit = 20

	now := time.Now().UTC()
	window := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	return Model{
		client:            client,
		serverURL:         serverURL,
		refreshInterval:   refreshInterval,
		activeSection:     sectionAlerts,
		mode:              modeDashboard,
		loading:           true,
		filterStatus:      "firing",
		commentCursor:     -1,
		alertTable:        alertT,
		commentInput:      ti,
		scheduleWindow:    window,
		scheduleTable:     schedT,
		userPickerTable:   pickerT,
		userManageTable:   manageT,
		userFormInputs:    [2]textinput.Model{usernameIn, emailIn},
		apiKeyNameInput:   keyNameIn,
		apiKeyRevokeInput: revokeIn,
		help:              help.New(),
		keys:              keys,
	}
}

func (m Model) Init() tea.Cmd {
	return connectCmd(m.client)
}

// ── Table rebuilders ───────────────────────────────────────────────────────

func defaultTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("0")).
		Background(colorPrimary).
		Bold(true)
	return s
}

func (m *Model) rebuildTable() {
	m.alertTable.SetColumns(alertColumns(m.width))
	m.alertTable.SetRows(alertRows(m.alerts))
	h := m.height - 8
	if h < 1 {
		h = 1
	}
	m.alertTable.SetHeight(h)
}

func (m *Model) rebuildScheduleTable() {
	m.scheduleTable.SetColumns(scheduleColumns(m.width))
	m.scheduleTable.SetRows(scheduleRows(m.scheduleDays))
	h := m.height - 10
	if h < 1 {
		h = 1
	}
	m.scheduleTable.SetHeight(h)
}

func (m *Model) rebuildUserPickerTable() {
	m.userPickerTable.SetColumns(userPickerColumns(m.width))
	rows := make([]table.Row, len(m.users))
	for i, u := range m.users {
		rows[i] = table.Row{u.Username, u.Email}
	}
	m.userPickerTable.SetRows(rows)
	h := m.height - 10
	if h < 1 {
		h = 1
	}
	m.userPickerTable.SetHeight(h)
}

func (m *Model) rebuildUserManageTable() {
	m.userManageTable.SetColumns(userManageColumns(m.width))
	rows := make([]table.Row, len(m.users))
	for i, u := range m.users {
		rows[i] = table.Row{u.Username, u.Email, u.CreatedAt.UTC().Format("2006-01-02")}
	}
	m.userManageTable.SetRows(rows)
	h := m.height - 10
	if h < 1 {
		h = 1
	}
	m.userManageTable.SetHeight(h)
}

func (m *Model) refreshDetailContent() {
	if m.width == 0 {
		return
	}
	m.detailViewport.SetContent(buildDetailContent(m.selectedAlert, m.comments, m.commentCursor, m.width))
}

func (m *Model) refreshStatsContent() {
	m.statsViewport.SetContent(buildStatsContent(m.topAlerts, m.hourStats, m.dayStats, m.width))
}

func (m Model) detailViewportHeight() int {
	h := m.height - 5
	if m.mode == modeComment {
		h -= 2
	}
	if h < 1 {
		h = 1
	}
	return h
}

// ── Column definitions ─────────────────────────────────────────────────────

func alertColumns(width int) []table.Column {
	nameW := width/2 - 8
	if nameW < 20 {
		nameW = 20
	}
	ackW := width - nameW - 10 - 12 - 6
	if ackW < 8 {
		ackW = 8
	}
	return []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Status", Width: 10},
		{Title: "Started", Width: 12},
		{Title: "Ack By", Width: ackW},
	}
}

func scheduleColumns(width int) []table.Column {
	dateW := 16
	onCallW := width - dateW - 6
	if onCallW < 15 {
		onCallW = 15
	}
	return []table.Column{
		{Title: "Date", Width: dateW},
		{Title: "On-Call", Width: onCallW},
	}
}

func userPickerColumns(width int) []table.Column {
	usernameW := 25
	emailW := width - usernameW - 6
	if emailW < 15 {
		emailW = 15
	}
	return []table.Column{
		{Title: "Username", Width: usernameW},
		{Title: "Email", Width: emailW},
	}
}

func userManageColumns(width int) []table.Column {
	createdW := 12
	usernameW := 25
	emailW := width - usernameW - createdW - 8
	if emailW < 15 {
		emailW = 15
	}
	return []table.Column{
		{Title: "Username", Width: usernameW},
		{Title: "Email", Width: emailW},
		{Title: "Created", Width: createdW},
	}
}

// ── Row builders ───────────────────────────────────────────────────────────

func alertRows(alerts []api.Alert) []table.Row {
	now := time.Now()
	rows := make([]table.Row, len(alerts))
	for i, a := range alerts {
		rows[i] = table.Row{a.Name, a.Status, humanAgo(now, a.StartsAt), a.AcknowledgedBy}
	}
	return rows
}

func scheduleRows(days []scheduleDay) []table.Row {
	today := time.Now().UTC().Format("2006-01-02")
	rows := make([]table.Row, len(days))
	for i, d := range days {
		dateStr := d.date.Format("2006-01-02")
		label := d.date.Format("Jan 02  Mon")
		if dateStr == today {
			label = "Today   " + d.date.Format("Mon")
		}
		onCall := "—"
		if d.entry != nil {
			onCall = d.entry.Username
		}
		rows[i] = table.Row{label, onCall}
	}
	return rows
}

func buildScheduleDays(window time.Time, entries []api.ScheduleEntry) []scheduleDay {
	entryMap := make(map[string]api.ScheduleEntry, len(entries))
	for _, e := range entries {
		entryMap[e.Date] = e
	}
	days := make([]scheduleDay, 14)
	for i := range days {
		date := window.AddDate(0, 0, i)
		day := scheduleDay{date: date}
		if e, ok := entryMap[date.Format("2006-01-02")]; ok {
			e2 := e
			day.entry = &e2
		}
		days[i] = day
	}
	return days
}

// ── Helpers ────────────────────────────────────────────────────────────────

func humanAgo(now, t time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh ago", h)
		}
		return fmt.Sprintf("%dh %dm ago", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		if h == 0 {
			return fmt.Sprintf("%dd ago", days)
		}
		return fmt.Sprintf("%dd %dh ago", days, h)
	}
}

// ── Commands ───────────────────────────────────────────────────────────────

func connectCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if err := client.HealthCheck(); err != nil {
			return connectErrMsg{err}
		}
		return connectedMsg{}
	}
}

func fetchAlertsCmd(client *api.Client, status string) tea.Cmd {
	return func() tea.Msg {
		alerts, err := client.ListAlerts(status, 500)
		if err != nil {
			return fetchDataErrMsg{err}
		}
		return alertsFetchedMsg{alerts}
	}
}

func fetchStatsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		stats, err := client.GetAlertStats()
		if err != nil {
			return fetchDataErrMsg{err}
		}
		return statsFetchedMsg{*stats}
	}
}

func fetchAlertDetailCmd(client *api.Client, alertID int64) tea.Cmd {
	return func() tea.Msg {
		alert, err := client.GetAlert(alertID)
		if err != nil {
			return alertDetailErrMsg{err}
		}
		comments, err := client.GetComments(alertID)
		if err != nil {
			return alertDetailErrMsg{err}
		}
		return alertDetailFetchedMsg{alert: *alert, comments: comments}
	}
}

func acknowledgeCmd(client *api.Client, alertID int64) tea.Cmd {
	return func() tea.Msg {
		alert, err := client.AcknowledgeAlert(alertID)
		if err != nil {
			return actionErrMsg{err}
		}
		comments, err := client.GetComments(alertID)
		if err != nil {
			return actionErrMsg{err}
		}
		return alertDetailFetchedMsg{alert: *alert, comments: comments}
	}
}

func unacknowledgeCmd(client *api.Client, alertID int64) tea.Cmd {
	return func() tea.Msg {
		if err := client.UnacknowledgeAlert(alertID); err != nil {
			return actionErrMsg{err}
		}
		alert, err := client.GetAlert(alertID)
		if err != nil {
			return actionErrMsg{err}
		}
		comments, err := client.GetComments(alertID)
		if err != nil {
			return actionErrMsg{err}
		}
		return alertDetailFetchedMsg{alert: *alert, comments: comments}
	}
}

func addCommentCmd(client *api.Client, alertID int64, content string) tea.Cmd {
	return func() tea.Msg {
		if _, err := client.AddComment(alertID, content); err != nil {
			return actionErrMsg{err}
		}
		alert, err := client.GetAlert(alertID)
		if err != nil {
			return actionErrMsg{err}
		}
		comments, err := client.GetComments(alertID)
		if err != nil {
			return actionErrMsg{err}
		}
		return alertDetailFetchedMsg{alert: *alert, comments: comments}
	}
}

func deleteCommentCmd(client *api.Client, alertID, commentID int64) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteComment(alertID, commentID); err != nil {
			return actionErrMsg{err}
		}
		alert, err := client.GetAlert(alertID)
		if err != nil {
			return actionErrMsg{err}
		}
		comments, err := client.GetComments(alertID)
		if err != nil {
			return actionErrMsg{err}
		}
		return alertDetailFetchedMsg{alert: *alert, comments: comments}
	}
}

func fetchDetailStatsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		top, err := client.GetTopAlerts(10)
		if err != nil {
			return detailStatsErrMsg{err}
		}
		byHour, err := client.GetStatsByHour()
		if err != nil {
			return detailStatsErrMsg{err}
		}
		byDay, err := client.GetStatsByDay()
		if err != nil {
			return detailStatsErrMsg{err}
		}
		return detailStatsFetchedMsg{top: top, byHour: byHour, byDay: byDay}
	}
}

func fetchScheduleCmd(client *api.Client, from, to time.Time) tea.Cmd {
	return func() tea.Msg {
		entries, err := client.GetSchedule(from.Format("2006-01-02"), to.Format("2006-01-02"))
		if err != nil {
			return scheduleFetchErrMsg{err}
		}
		current, err := client.GetCurrentOnCall()
		if err != nil {
			return scheduleFetchErrMsg{err}
		}
		return scheduleFetchedMsg{entries: entries, current: current}
	}
}

func assignScheduleCmd(client *api.Client, userID int64, date string, from, to time.Time) tea.Cmd {
	return func() tea.Msg {
		if _, err := client.AssignSchedule(userID, []string{date}); err != nil {
			return scheduleActionErrMsg{err}
		}
		entries, err := client.GetSchedule(from.Format("2006-01-02"), to.Format("2006-01-02"))
		if err != nil {
			return scheduleActionErrMsg{err}
		}
		current, err := client.GetCurrentOnCall()
		if err != nil {
			return scheduleActionErrMsg{err}
		}
		return scheduleFetchedMsg{entries: entries, current: current}
	}
}

func deleteScheduleEntryCmd(client *api.Client, entryID int64, from, to time.Time) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteScheduleEntry(entryID); err != nil {
			return scheduleActionErrMsg{err}
		}
		entries, err := client.GetSchedule(from.Format("2006-01-02"), to.Format("2006-01-02"))
		if err != nil {
			return scheduleActionErrMsg{err}
		}
		current, err := client.GetCurrentOnCall()
		if err != nil {
			return scheduleActionErrMsg{err}
		}
		return scheduleFetchedMsg{entries: entries, current: current}
	}
}

func fetchUsersCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		users, err := client.ListUsers()
		if err != nil {
			return usersFetchedMsg{} // empty on error, statusMsg set elsewhere
		}
		return usersFetchedMsg{users: users}
	}
}

func createUserCmd(client *api.Client, username, email string) tea.Cmd {
	return func() tea.Msg {
		if _, err := client.CreateUser(username, email); err != nil {
			return userActionErrMsg{err}
		}
		users, err := client.ListUsers()
		if err != nil {
			return userActionErrMsg{err}
		}
		return usersFetchedMsg{users: users}
	}
}

func deleteUserCmd(client *api.Client, userID int64) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteUser(userID); err != nil {
			return userActionErrMsg{err}
		}
		users, err := client.ListUsers()
		if err != nil {
			return userActionErrMsg{err}
		}
		return usersFetchedMsg{users: users}
	}
}

func createAPIKeyCmd(client *api.Client, userID int64, name string) tea.Cmd {
	return func() tea.Msg {
		key, err := client.CreateAPIKey(userID, name)
		if err != nil {
			return userActionErrMsg{err}
		}
		return apiKeyCreatedMsg{key: *key}
	}
}

func deleteAPIKeyCmd(client *api.Client, userID, keyID int64) tea.Cmd {
	return func() tea.Msg {
		if err := client.DeleteAPIKey(userID, keyID); err != nil {
			return userActionErrMsg{err}
		}
		return apiKeyRevokedMsg{}
	}
}

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func clearStatusCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}
