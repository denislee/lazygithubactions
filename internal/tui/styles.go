package tui

import "charm.land/lipgloss/v2"

var (
	primaryColor    = lipgloss.Color("#7D56F4")
	successColor    = lipgloss.Color("#04B575")
	failureColor    = lipgloss.Color("#FF4444")
	warningColor    = lipgloss.Color("#FFAA00")
	runningColor    = lipgloss.Color("#00AAFF")
	dimColor        = lipgloss.Color("#666666")
	textColor       = lipgloss.Color("#FAFAFA")
	subtextColor    = lipgloss.Color("#AAAAAA")

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dimColor).
		Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
		Foreground(textColor).
		Bold(true)

	normalItemStyle = lipgloss.NewStyle().
		Foreground(subtextColor)

	statusBarStyle = lipgloss.NewStyle().
		Foreground(subtextColor).
		Padding(0, 1)

	helpBarStyle = lipgloss.NewStyle().
		Foreground(dimColor).
		Padding(0, 1)

	overlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Width(60)

	dialogStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(warningColor).
		Padding(1, 2).
		Width(50)
)

func PanelStyle() lipgloss.Style          { return panelStyle }
func ActivePanelStyle() lipgloss.Style    { return activePanelStyle }
func TitleStyle() lipgloss.Style          { return titleStyle }
func SelectedItemStyle() lipgloss.Style   { return selectedItemStyle }
func NormalItemStyle() lipgloss.Style     { return normalItemStyle }
func StatusBarStyle() lipgloss.Style      { return statusBarStyle }
func HelpBarStyle() lipgloss.Style        { return helpBarStyle }
func OverlayStyle() lipgloss.Style        { return overlayStyle }
func DialogStyle() lipgloss.Style         { return dialogStyle }

func StatusStyle(status, conclusion string) lipgloss.Style {
	switch {
	case status == "in_progress" || status == "queued" || status == "waiting":
		return lipgloss.NewStyle().Foreground(runningColor)
	case conclusion == "success":
		return lipgloss.NewStyle().Foreground(successColor)
	case conclusion == "failure":
		return lipgloss.NewStyle().Foreground(failureColor)
	case conclusion == "cancelled":
		return lipgloss.NewStyle().Foreground(dimColor)
	default:
		return lipgloss.NewStyle().Foreground(warningColor)
	}
}

func StatusIcon(status, conclusion string) string {
	switch {
	case status == "in_progress":
		return "●"
	case status == "queued" || status == "waiting":
		return "◷"
	case conclusion == "success":
		return "✓"
	case conclusion == "failure":
		return "✗"
	case conclusion == "cancelled" || conclusion == "skipped":
		return "⊘"
	default:
		return "?"
	}
}
