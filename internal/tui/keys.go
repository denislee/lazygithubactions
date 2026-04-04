package tui

import "charm.land/bubbles/v2/key"

type KeyMap struct {
	Quit        key.Binding
	Up          key.Binding
	Down        key.Binding
	Tab         key.Binding
	Enter       key.Binding
	Back        key.Binding
	Refresh     key.Binding
	Trigger     key.Binding
	Cancel      key.Binding
	Rerun       key.Binding
	Logs        key.Binding
	Download    key.Binding
	QuickSwitch key.Binding
	Filter      key.Binding
	Help        key.Binding
}

var Keys = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select/drill-down"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Trigger: key.NewBinding(
		key.WithKeys("T"),
		key.WithHelp("T", "trigger"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "cancel"),
	),
	Rerun: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "rerun failed"),
	),
	Logs: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "view logs"),
	),
	Download: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "download artifacts"),
	),
	QuickSwitch: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "quick switch"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}
