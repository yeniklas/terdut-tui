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
	// Incidents lead: they are the work. Alerts is the raw feed underneath.
	sectionIncidents section = iota
	sectionAlerts
	sectionStats
	sectionArchived
	sectionSchedule
	sectionUsers

	sectionCount = 6
)

type mode int

const (
	modeDashboard mode = iota
	modeIncidentDetail
	modeAlertDetail
	modeNote
	modeSnooze
	modeConfirm
	modeUserPicker
	modeUserCreate
	modeUserNotifyEdit
	modeAPIKeyMenu
	modeAPIKeyCreate
	modeAPIKeyReveal
	modeAPIKeyRevokeByID
)

type confirmTarget int

const (
	confirmDeleteNote confirmTarget = iota
	confirmResolveIncident
	confirmDeleteScheduleEntry
	confirmDeleteUser
	confirmReassignSchedule
)

// pickerTarget says what the user picker is choosing a person for.
type pickerTarget int

const (
	pickerSchedule pickerTarget = iota
	pickerIncidentAssignee
)

// incidentFilters is the cycle the f key walks in the Incidents section. The
// empty string is the server default: open, unsnoozed incidents — the queue.
var incidentFilters = []string{"", api.StatusTriggered, api.StatusAcknowledged, api.StatusResolved, "snoozed"}

// alertFilters is the equivalent cycle for the raw alert feed.
var alertFilters = []string{"firing", "resolved", "", "archived"}

// incidentQuery translates a filter from the cycle into server query terms.
func incidentQuery(filter string) (status string, snoozed bool) {
	if filter == "snoozed" {
		return "", true
	}
	return filter, false
}

// filterLabel renders a filter for the status bar.
func filterLabel(filter string) string {
	if filter == "" {
		return "open"
	}
	return filter
}

// ── Messages ───────────────────────────────────────────────────────────────

// dashboard
type connectedMsg struct{}
type connectErrMsg struct{ err error }
type incidentsFetchedMsg struct{ incidents []api.Incident }
type archivedIncidentsFetchedMsg struct{ incidents []api.Incident }
type incidentActionDoneMsg struct {
	incidents []api.Incident
	status    string
}
type alertsFetchedMsg struct{ alerts []api.Alert }
type statsFetchedMsg struct {
	incidents api.IncidentStats
	alerts    api.AlertStats
}
type fetchDataErrMsg struct{ err error }
type tickMsg time.Time
type clearStatusMsg struct{}

// detail
type incidentDetailFetchedMsg struct {
	incident api.Incident
	timeline []api.IncidentEvent
}
type alertDetailFetchedMsg struct{ alert api.Alert }
type detailErrMsg struct{ err error }
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

// pendingAssign is an on-call assignment held back by the reassignment
// confirmation, because some of its dates belong to somebody else.
type pendingAssign struct {
	userID   int64
	username string
	dates    []string
	// taken are the dates currently held by other people, and holders the
	// distinct names holding them — both only for wording the prompt.
	taken   []string
	holders []string
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
	connected     bool
	loading       bool
	err           error
	statusMsg     string
	incidentStats *api.IncidentStats
	alertStats    *api.AlertStats

	// Incidents
	incidents      []api.Incident
	incidentFilter string
	incidentTable  table.Model

	// Alerts (read-only feed)
	alerts      []api.Alert
	alertFilter string
	alertTable  table.Model

	// Archived incidents
	archivedIncidents []api.Incident
	archivedLoading   bool
	archivedTable     table.Model

	// Incident detail
	selectedIncident api.Incident
	timeline         []api.IncidentEvent
	noteCursor       int
	detailLoading    bool
	detailViewport   viewport.Model

	// Alert detail
	selectedAlert api.Alert

	// Note compose & snooze
	noteInput   textinput.Model
	snoozeInput textinput.Model

	// Confirm
	confirmTarget      confirmTarget
	pendingDeleteID    int64 // note event ID
	pendingDeleteEntry *api.ScheduleEntry
	pendingAssign      *pendingAssign

	// Stats
	topAlerts []api.TopAlert
	hourStats []api.HourStat
	dayStats  []api.DayStat
	// statsLoaded tracks the first fetch separately from emptiness: a server with
	// no alerts yet legitimately returns three empty slices.
	statsLoaded   bool
	statsLoading  bool
	statsViewport viewport.Model

	// Schedule
	scheduleWindow  time.Time
	scheduleEntries []api.ScheduleEntry
	scheduleDays    []scheduleDay
	currentOnCall   *api.ScheduleEntry
	scheduleLoading bool
	scheduleTable   table.Model

	// User picker (schedule assignment and incident assignee)
	users            []api.User
	usersLoading     bool
	userPickerTable  table.Model
	pickerTarget     pickerTarget
	pickerAssignWeek bool

	// User management section
	userManageTable   table.Model
	selectedUser      api.User
	userFormInputs    [2]textinput.Model
	userFormFocus     int
	ntfyTopicInput    textinput.Model
	apiKeyNameInput   textinput.Model
	apiKeyRevokeInput textinput.Model
	revealedAPIKey    api.APIKey

	help help.Model
	keys keyMap
}

func NewModel(client *api.Client, serverURL string, refreshInterval time.Duration) Model {
	ts := defaultTableStyles()

	incidentT := table.New(table.WithFocused(true))
	incidentT.SetStyles(ts)

	alertT := table.New(table.WithFocused(true))
	alertT.SetStyles(ts)

	archivedT := table.New(table.WithFocused(true))
	archivedT.SetStyles(ts)

	schedT := table.New(table.WithFocused(true))
	schedT.SetStyles(ts)

	pickerT := table.New(table.WithFocused(true))
	pickerT.SetStyles(ts)

	manageT := table.New(table.WithFocused(true))
	manageT.SetStyles(ts)

	// Sized by the first tea.WindowSizeMsg; built here so it carries the default
	// scroll keymap, which the zero value lacks.
	statsVP := viewport.New(0, 0)

	noteIn := textinput.New()
	noteIn.Placeholder = "type your note…"
	noteIn.CharLimit = 1000

	snoozeIn := textinput.New()
	snoozeIn.Placeholder = "duration, e.g. 2h or 30m"
	snoozeIn.CharLimit = 16

	usernameIn := textinput.New()
	usernameIn.Placeholder = "username"
	usernameIn.CharLimit = 64

	emailIn := textinput.New()
	emailIn.Placeholder = "email"
	emailIn.CharLimit = 128

	topicIn := textinput.New()
	topicIn.Placeholder = "ntfy topic — empty clears it"
	topicIn.CharLimit = 128

	keyNameIn := textinput.New()
	keyNameIn.Placeholder = "key name (e.g. laptop)"
	keyNameIn.CharLimit = 64

	revokeIn := textinput.New()
	revokeIn.Placeholder = "integer key ID"
	revokeIn.CharLimit = 20

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7 // ISO: Sunday = 7
	}
	window := today.AddDate(0, 0, -(weekday - 1))

	return Model{
		client:            client,
		serverURL:         serverURL,
		refreshInterval:   refreshInterval,
		activeSection:     sectionIncidents,
		mode:              modeDashboard,
		loading:           true,
		incidentFilter:    "",
		alertFilter:       "firing",
		noteCursor:        -1,
		incidentTable:     incidentT,
		alertTable:        alertT,
		archivedTable:     archivedT,
		statsViewport:     statsVP,
		noteInput:         noteIn,
		snoozeInput:       snoozeIn,
		scheduleWindow:    window,
		scheduleTable:     schedT,
		userPickerTable:   pickerT,
		userManageTable:   manageT,
		userFormInputs:    [2]textinput.Model{usernameIn, emailIn},
		ntfyTopicInput:    topicIn,
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

func (m *Model) rebuildIncidentTable() {
	m.incidentTable.SetColumns(incidentColumns(m.width))
	m.incidentTable.SetRows(incidentRows(m.incidents))
	m.incidentTable.SetHeight(tableHeight(m.height, 8))
}

func (m *Model) rebuildTable() {
	m.alertTable.SetColumns(alertColumns(m.width))
	m.alertTable.SetRows(alertRows(m.alerts))
	m.alertTable.SetHeight(tableHeight(m.height, 8))
}

func (m *Model) rebuildArchivedTable() {
	m.archivedTable.SetColumns(incidentColumns(m.width))
	m.archivedTable.SetRows(incidentRows(m.archivedIncidents))
	m.archivedTable.SetHeight(tableHeight(m.height, 8))
}

func (m *Model) rebuildScheduleTable() {
	m.scheduleTable.SetColumns(scheduleColumns(m.width))
	m.scheduleTable.SetRows(scheduleRows(m.scheduleDays))
	m.scheduleTable.SetHeight(tableHeight(m.height, 10))
}

func (m *Model) rebuildUserPickerTable() {
	m.userPickerTable.SetColumns(userPickerColumns(m.width))
	rows := make([]table.Row, len(m.users))
	for i, u := range m.users {
		rows[i] = table.Row{u.Username, u.Email}
	}
	m.userPickerTable.SetRows(rows)
	m.userPickerTable.SetHeight(tableHeight(m.height, 10))
}

func (m *Model) rebuildUserManageTable() {
	m.userManageTable.SetColumns(userManageColumns(m.width))
	rows := make([]table.Row, len(m.users))
	for i, u := range m.users {
		topic := u.Topic()
		if topic == "" {
			topic = "—"
		}
		rows[i] = table.Row{u.Username, u.Email, topic, u.CreatedAt.UTC().Format("2006-01-02")}
	}
	m.userManageTable.SetRows(rows)
	m.userManageTable.SetHeight(tableHeight(m.height, 10))
}

func tableHeight(windowHeight, chrome int) int {
	h := windowHeight - chrome
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) refreshDetailContent() {
	if m.width == 0 {
		return
	}
	if m.mode == modeAlertDetail {
		m.detailViewport.SetContent(buildAlertDetailContent(m.selectedAlert, m.width))
		return
	}
	m.detailViewport.SetContent(
		buildIncidentDetailContent(m.selectedIncident, m.timeline, m.noteCursor, m.width))
}

func (m *Model) refreshStatsContent() {
	m.statsViewport.SetContent(
		buildStatsContent(m.incidentStats, m.topAlerts, m.hourStats, m.dayStats, m.width))
}

func (m Model) statsViewportHeight() int {
	h := m.height - 5
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) detailViewportHeight() int {
	h := m.height - 5
	if m.mode == modeNote || m.mode == modeSnooze {
		h -= 2
	}
	if h < 1 {
		h = 1
	}
	return h
}

// noteEvents filters a timeline down to the deletable entries, which is what
// the [ and ] cursor walks.
func noteEvents(timeline []api.IncidentEvent) []api.IncidentEvent {
	notes := make([]api.IncidentEvent, 0, len(timeline))
	for _, e := range timeline {
		if e.Type == api.EventNote {
			notes = append(notes, e)
		}
	}
	return notes
}

// ── Column definitions ─────────────────────────────────────────────────────

func incidentColumns(width int) []table.Column {
	const sevW, statusW, timeW = 9, 15, 12
	titleW := width/2 - 10
	if titleW < 20 {
		titleW = 20
	}
	// 10 = bubbles' Padding(0, 1) on each of the five cells.
	assigneeW := width - sevW - titleW - statusW - timeW - 10
	if assigneeW < 8 {
		assigneeW = 8
	}
	return []table.Column{
		{Title: "Sev", Width: sevW},
		{Title: "Incident", Width: titleW},
		{Title: "Status", Width: statusW},
		{Title: "Assignee", Width: assigneeW},
		{Title: "Triggered", Width: timeW},
	}
}

func alertColumns(width int) []table.Column {
	const statusW, timeW = 10, 12
	nameW := width/2 - 14
	if nameW < 20 {
		nameW = 20
	}
	// 10 = bubbles' Padding(0, 1) on each of the five cells.
	incW := width - nameW - statusW - 2*timeW - 10
	if incW < 8 {
		incW = 8
	}
	return []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Status", Width: statusW},
		{Title: "Started", Width: timeW},
		{Title: "Last Seen", Width: timeW},
		{Title: "Incident", Width: incW},
	}
}

func scheduleColumns(width int) []table.Column {
	dateW := 18
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
	topicW := 22
	// 8 = bubbles' Padding(0, 1) on each of the four cells.
	emailW := width - usernameW - topicW - createdW - 8
	if emailW < 15 {
		emailW = 15
	}
	return []table.Column{
		{Title: "Username", Width: usernameW},
		{Title: "Email", Width: emailW},
		{Title: "Ntfy Topic", Width: topicW},
		{Title: "Created", Width: createdW},
	}
}

// ── Row builders ───────────────────────────────────────────────────────────

func incidentRows(incidents []api.Incident) []table.Row {
	now := time.Now()
	rows := make([]table.Row, len(incidents))
	for i, inc := range incidents {
		severity := inc.Severity
		if severity == "" {
			severity = "—"
		}
		// bubbles' table renders plain strings, so a snoozed incident is marked
		// in the status cell rather than styled.
		status := inc.Status
		if inc.IsSnoozed() {
			status += " (zzz)"
		}
		assignee := inc.AssignedTo
		if assignee == "" {
			assignee = "—"
		}
		rows[i] = table.Row{severity, inc.Title, status, assignee, humanAgo(now, inc.TriggeredAt)}
	}
	return rows
}

func alertRows(alerts []api.Alert) []table.Row {
	now := time.Now()
	rows := make([]table.Row, len(alerts))
	for i, a := range alerts {
		incident := "—"
		if a.IncidentID != nil {
			incident = fmt.Sprintf("#%d", *a.IncidentID)
		}
		rows[i] = table.Row{a.Name, a.Status, humanAgo(now, a.StartsAt), humanAgo(now, a.ReceivedAt), incident}
	}
	return rows
}

func scheduleRows(days []scheduleDay) []table.Row {
	today := time.Now().UTC().Format("2006-01-02")
	rows := make([]table.Row, len(days))
	for i, d := range days {
		dateStr := d.date.Format("2006-01-02")
		showWeek := i == 0 || d.date.Weekday() == time.Monday
		_, week := d.date.ISOWeek()
		weekPrefix := "    "
		if showWeek {
			weekPrefix = fmt.Sprintf("W%02d ", week)
		}
		label := weekPrefix + d.date.Format("Jan 02 Mon")
		if dateStr == today {
			label = weekPrefix + "Today " + d.date.Format("Mon")
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
	days := make([]scheduleDay, 7)
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
	return humanDuration(d) + " ago"
}

// humanUntil renders a future deadline, used for snooze expiry.
func humanUntil(now, t time.Time) string {
	d := t.Sub(now)
	if d <= 0 {
		return "expired"
	}
	return "in " + humanDuration(d)
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "moments"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		if h == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, h)
	}
}

// humanSeconds renders an MTTA/MTTR average.
func humanSeconds(secs *float64) string {
	if secs == nil {
		return "—"
	}
	return humanDuration(time.Duration(*secs) * time.Second)
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

func fetchIncidentsCmd(client *api.Client, filter string) tea.Cmd {
	return func() tea.Msg {
		status, snoozed := incidentQuery(filter)
		incidents, err := client.ListIncidents(status, false, snoozed, 500)
		if err != nil {
			return fetchDataErrMsg{err}
		}
		return incidentsFetchedMsg{incidents}
	}
}

func fetchArchivedIncidentsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		// Archived incidents are all resolved, so the status filter has to be
		// widened past the server's open-only default or nothing comes back.
		incidents, err := client.ListIncidents(api.StatusResolved, true, false, 500)
		if err != nil {
			return fetchDataErrMsg{err}
		}
		return archivedIncidentsFetchedMsg{incidents}
	}
}

func fetchAlertsCmd(client *api.Client, filter string) tea.Cmd {
	return func() tea.Msg {
		status, archived := filter, false
		if filter == "archived" {
			status, archived = "", true
		}
		alerts, err := client.ListAlerts(status, archived, 500)
		if err != nil {
			return fetchDataErrMsg{err}
		}
		return alertsFetchedMsg{alerts}
	}
}

func fetchStatsCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		incidents, err := client.GetIncidentStats()
		if err != nil {
			return fetchDataErrMsg{err}
		}
		alerts, err := client.GetAlertStats()
		if err != nil {
			return fetchDataErrMsg{err}
		}
		return statsFetchedMsg{incidents: *incidents, alerts: *alerts}
	}
}

// incidentDetail reloads an incident and its timeline. Every detail-mode action
// funnels through it so the view always reflects what the server just did.
func incidentDetail(client *api.Client, id int64) tea.Msg {
	incident, err := client.GetIncident(id)
	if err != nil {
		return detailErrMsg{err}
	}
	timeline, err := client.GetIncidentTimeline(id)
	if err != nil {
		return detailErrMsg{err}
	}
	return incidentDetailFetchedMsg{incident: *incident, timeline: timeline}
}

func fetchIncidentDetailCmd(client *api.Client, id int64) tea.Cmd {
	return func() tea.Msg { return incidentDetail(client, id) }
}

// incidentActionCmd performs an action then reloads the detail view, reporting
// the server's error rather than a stale success.
func incidentActionCmd(client *api.Client, id int64, action func() error) tea.Cmd {
	return func() tea.Msg {
		if err := action(); err != nil {
			return actionErrMsg{err}
		}
		return incidentDetail(client, id)
	}
}

func acknowledgeIncidentCmd(client *api.Client, id int64) tea.Cmd {
	return incidentActionCmd(client, id, func() error {
		_, err := client.AcknowledgeIncident(id)
		return err
	})
}

func unacknowledgeIncidentCmd(client *api.Client, id int64) tea.Cmd {
	return incidentActionCmd(client, id, func() error { return client.UnacknowledgeIncident(id) })
}

func resolveIncidentCmd(client *api.Client, id int64) tea.Cmd {
	return incidentActionCmd(client, id, func() error {
		_, err := client.ResolveIncident(id)
		return err
	})
}

func assignIncidentCmd(client *api.Client, id, userID int64) tea.Cmd {
	return incidentActionCmd(client, id, func() error {
		_, err := client.AssignIncident(id, userID)
		return err
	})
}

func snoozeIncidentCmd(client *api.Client, id int64, duration string) tea.Cmd {
	return incidentActionCmd(client, id, func() error {
		_, err := client.SnoozeIncident(id, duration)
		return err
	})
}

func unsnoozeIncidentCmd(client *api.Client, id int64) tea.Cmd {
	return incidentActionCmd(client, id, func() error { return client.UnsnoozeIncident(id) })
}

func addNoteCmd(client *api.Client, id int64, content string) tea.Cmd {
	return incidentActionCmd(client, id, func() error {
		_, err := client.AddNote(id, content)
		return err
	})
}

func deleteNoteCmd(client *api.Client, id, eventID int64) tea.Cmd {
	return incidentActionCmd(client, id, func() error { return client.DeleteNote(id, eventID) })
}

// archiveIncidentCmd archives from the list view, so it reloads the list rather
// than a detail pane.
func archiveIncidentCmd(client *api.Client, id int64, filter string) tea.Cmd {
	return func() tea.Msg {
		if _, err := client.ArchiveIncident(id); err != nil {
			return actionErrMsg{err}
		}
		status, snoozed := incidentQuery(filter)
		incidents, err := client.ListIncidents(status, false, snoozed, 500)
		if err != nil {
			return actionErrMsg{err}
		}
		return incidentActionDoneMsg{incidents: incidents, status: "Incident archived"}
	}
}

func unarchiveIncidentCmd(client *api.Client, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := client.UnarchiveIncident(id); err != nil {
			return actionErrMsg{err}
		}
		incidents, err := client.ListIncidents(api.StatusResolved, true, false, 500)
		if err != nil {
			return actionErrMsg{err}
		}
		return archivedIncidentsFetchedMsg{incidents}
	}
}

func fetchAlertDetailCmd(client *api.Client, alertID int64) tea.Cmd {
	return func() tea.Msg {
		alert, err := client.GetAlert(alertID)
		if err != nil {
			return detailErrMsg{err}
		}
		return alertDetailFetchedMsg{alert: *alert}
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

func assignScheduleCmd(client *api.Client, userID int64, dates []string, replace bool, from, to time.Time) tea.Cmd {
	return func() tea.Msg {
		if _, err := client.AssignSchedule(userID, dates, replace); err != nil {
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

// setUserNotifyTargetCmd points a user's pages at a topic, or clears it when
// topic is empty. It re-lists afterwards so the table shows what the server
// stored rather than what was typed.
func setUserNotifyTargetCmd(client *api.Client, userID int64, topic string) tea.Cmd {
	return func() tea.Msg {
		if _, err := client.SetUserNotifyTarget(userID, topic); err != nil {
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
