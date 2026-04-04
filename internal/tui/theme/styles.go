package theme

import "charm.land/lipgloss/v2"

var (
	primaryColor  = lipgloss.Color("#7D56F4")
	successColor  = lipgloss.Color("#04B575")
	failureColor  = lipgloss.Color("#FF4444")
	warningColor  = lipgloss.Color("#FFAA00")
	runningColor  = lipgloss.Color("#00AAFF")
	dimColor      = lipgloss.Color("#666666")
	textColor     = lipgloss.Color("#FAFAFA")
	subtextColor  = lipgloss.Color("#AAAAAA")

	PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dimColor).
		Padding(0, 1)

	ActivePanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(0, 1)

	SelectedItemStyle = lipgloss.NewStyle().
		Foreground(textColor).
		Bold(true)

	NormalItemStyle = lipgloss.NewStyle().
		Foreground(subtextColor)

	StatusBarStyle = lipgloss.NewStyle().
		Foreground(subtextColor).
		Padding(0, 1)

	HelpBarStyle = lipgloss.NewStyle().
		Foreground(dimColor).
		Padding(0, 1)

	OverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Width(60)

	DialogStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(warningColor).
		Padding(1, 2).
		Width(50)
)

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
