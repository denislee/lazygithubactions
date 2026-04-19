package theme

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
	OpenBrowser key.Binding
}

// Page navigation bindings (not in KeyMap since they're used directly)
var (
	PageDown = key.NewBinding(key.WithKeys("ctrl+f", "pgdown"))
	PageUp   = key.NewBinding(key.WithKeys("ctrl+b", "pgup"))
	HalfDown = key.NewBinding(key.WithKeys("ctrl+d"))
	HalfUp   = key.NewBinding(key.WithKeys("ctrl+u"))
	NextPage = key.NewBinding(key.WithKeys("ctrl+n"))
	PrevPage = key.NewBinding(key.WithKeys("ctrl+p"))
)

var Keys = KeyMap{
	Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch panel")),
	Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Trigger:     key.NewBinding(key.WithKeys("T", "t"), key.WithHelp("t", "trigger")),
	Cancel:      key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "cancel")),
	Rerun:       key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rerun failed")),
	Logs:        key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "view logs")),
	Download:    key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "download artifacts")),
	QuickSwitch: key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("ctrl+k", "quick switch")),
	Filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	OpenBrowser: key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "open browser")),
}
