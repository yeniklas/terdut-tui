package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"
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
	modeSchedule
	modeUsers
)

// tea.Msg types

type connectedMsg struct{ serverURL string }
type connectErrMsg struct{ err error }
type tickMsg time.Time
type clearStatusMsg struct{}

// Model holds all UI state.
type Model struct {
	client          *api.Client
	serverURL       string
	refreshInterval time.Duration

	activeSection section
	mode          mode
	width         int
	height        int

	connected bool
	err       error
	statusMsg string

	help help.Model
	keys keyMap
}

func NewModel(client *api.Client, serverURL string, refreshInterval time.Duration) Model {
	return Model{
		client:          client,
		serverURL:       serverURL,
		refreshInterval: refreshInterval,
		activeSection:   sectionAlerts,
		mode:            modeDashboard,
		help:            help.New(),
		keys:            keys,
	}
}

func (m Model) Init() tea.Cmd {
	return connectCmd(m.client)
}

func connectCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		if err := client.HealthCheck(); err != nil {
			return connectErrMsg{err}
		}
		return connectedMsg{}
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
