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
)

// tea.Msg types — dashboard

type connectedMsg struct{}
type connectErrMsg struct{ err error }
type alertsFetchedMsg struct{ alerts []api.Alert }
type statsFetchedMsg struct{ stats api.AlertStats }
type fetchDataErrMsg struct{ err error }
type tickMsg time.Time
type clearStatusMsg struct{}

// tea.Msg types — detail

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

// Model holds all UI state.

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

	// Detail view
	selectedAlert  api.Alert
	comments       []api.Comment
	commentCursor  int
	detailLoading  bool
	detailViewport viewport.Model

	// Comment compose
	commentInput textinput.Model

	// Confirm delete
	pendingDeleteID int64

	// Stats view
	topAlerts    []api.TopAlert
	hourStats    []api.HourStat
	dayStats     []api.DayStat
	statsLoading bool
	statsViewport viewport.Model

	help help.Model
	keys keyMap
}

func NewModel(client *api.Client, serverURL string, refreshInterval time.Duration) Model {
	t := table.New(table.WithFocused(true))
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("0")).
		Background(colorPrimary).
		Bold(true)
	t.SetStyles(s)

	ti := textinput.New()
	ti.Placeholder = "type your comment…"
	ti.CharLimit = 1000

	return Model{
		client:          client,
		serverURL:       serverURL,
		refreshInterval: refreshInterval,
		activeSection:   sectionAlerts,
		mode:            modeDashboard,
		loading:         true,
		filterStatus:    "firing",
		commentCursor:   -1,
		alertTable:      t,
		commentInput:    ti,
		help:            help.New(),
		keys:            keys,
	}
}

func (m Model) Init() tea.Cmd {
	return connectCmd(m.client)
}

// rebuildTable updates alert table columns, rows, and height from current state.
func (m *Model) rebuildTable() {
	m.alertTable.SetColumns(alertColumns(m.width))
	m.alertTable.SetRows(alertRows(m.alerts))
	h := m.height - 8
	if h < 1 {
		h = 1
	}
	m.alertTable.SetHeight(h)
}

// refreshDetailContent rebuilds the viewport content from selectedAlert + comments.
func (m *Model) refreshDetailContent() {
	if m.width == 0 {
		return
	}
	content := buildDetailContent(m.selectedAlert, m.comments, m.commentCursor, m.width)
	m.detailViewport.SetContent(content)
}

// refreshStatsContent rebuilds the stats viewport from loaded stats data.
func (m *Model) refreshStatsContent() {
	content := buildStatsContent(m.topAlerts, m.hourStats, m.dayStats, m.width)
	m.statsViewport.SetContent(content)
}

// detailViewportHeight returns the detail viewport height for the current mode.
func (m Model) detailViewportHeight() int {
	h := m.height - 5
	if m.mode == modeComment {
		h -= 2 // separator + input line
	}
	if h < 1 {
		h = 1
	}
	return h
}

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

func alertRows(alerts []api.Alert) []table.Row {
	now := time.Now()
	rows := make([]table.Row, len(alerts))
	for i, a := range alerts {
		rows[i] = table.Row{a.Name, a.Status, humanAgo(now, a.StartsAt), a.AcknowledgedBy}
	}
	return rows
}

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

// Command constructors

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
