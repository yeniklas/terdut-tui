package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Tab     key.Binding
	Enter   key.Binding
	Escape  key.Binding
	Refresh key.Binding
	Filter  key.Binding
	Quit    key.Binding
}

var keys = keyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("◀/h", "prev")),
	Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("▶/l", "next")),
	Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch section")),
	Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Escape:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Filter:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.Enter, k.Escape},
		{k.Refresh, k.Filter, k.Quit},
	}
}
