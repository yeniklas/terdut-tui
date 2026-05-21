package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
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

// tea.Msg types

type connectedMsg struct{}
type connectErrMsg struct{ err error }
type alertsFetchedMsg struct{ alerts []api.Alert }
type statsFetchedMsg struct{ stats api.AlertStats }
type fetchDataErrMsg struct{ err error }
type tickMsg time.Time
type clearStatusMsg struct{}

// Model holds all UI state.
type Model struct {
	client          *api.Client
	serverURL       string
	refreshInterval time.Duration

	activeSection section
	width         int
	height        int

	connected    bool
	loading      bool
	err          error
	statusMsg    string

	alerts       []api.Alert
	stats        *api.AlertStats
	filterStatus string // "firing", "resolved", or "" (all)

	alertTable table.Model
	help       help.Model
	keys       keyMap
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

	return Model{
		client:          client,
		serverURL:       serverURL,
		refreshInterval: refreshInterval,
		activeSection:   sectionAlerts,
		loading:         true,
		filterStatus:    "firing",
		alertTable:      t,
		help:            help.New(),
		keys:            keys,
	}
}

func (m Model) Init() tea.Cmd {
	return connectCmd(m.client)
}

// rebuildTable updates the alert table columns, rows, and height to match current state.
func (m *Model) rebuildTable() {
	m.alertTable.SetColumns(alertColumns(m.width))
	m.alertTable.SetRows(alertRows(m.alerts))
	h := m.height - 8 // header + tabs + sep + stats + table-header + footer + 2 margins
	if h < 1 {
		h = 1
	}
	m.alertTable.SetHeight(h)
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
		ack := a.AcknowledgedBy
		rows[i] = table.Row{a.Name, a.Status, humanAgo(now, a.StartsAt), ack}
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
