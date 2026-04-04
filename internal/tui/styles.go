package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var (
	// Colors
	primaryColor    = lipgloss.Color("#7D56F4")
	successColor    = lipgloss.Color("#04B575")
	failureColor    = lipgloss.Color("#FF4444")
	warningColor    = lipgloss.Color("#FFAA00")
	runningColor    = lipgloss.Color("#00AAFF")
	dimColor        = lipgloss.Color("#666666")
	textColor       = lipgloss.Color("#FAFAFA")
	subtextColor    = lipgloss.Color("#AAAAAA")

	// Panel styles
	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dimColor).
		Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 1)

	// Title styles
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Padding(0, 1)

	// Status styles
	statusSuccess   = lipgloss.NewStyle().Foreground(successColor)
	statusFailure   = lipgloss.NewStyle().Foreground(failureColor)
	statusRunning   = lipgloss.NewStyle().Foreground(runningColor)
	statusPending   = lipgloss.NewStyle().Foreground(warningColor)
	statusCancelled = lipgloss.NewStyle().Foreground(dimColor)

	// List item styles
	selectedItemStyle = lipgloss.NewStyle().
		Foreground(textColor).
		Bold(true)

	normalItemStyle = lipgloss.NewStyle().
		Foreground(subtextColor)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
		Foreground(subtextColor).
		Padding(0, 1)

	// Help bar at bottom
	helpBarStyle = lipgloss.NewStyle().
		Foreground(dimColor).
		Padding(0, 1)

	// Overlay (for quick switcher)
	overlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Width(60)

	// Dialog
	dialogStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(warningColor).
		Padding(1, 2).
		Width(50)
)

// Exported accessor functions for use by the components package
func PanelStyle() lipgloss.Style          { return panelStyle }
func ActivePanelStyle() lipgloss.Style    { return activePanelStyle }
func TitleStyle() lipgloss.Style          { return titleStyle }
func SelectedItemStyle() lipgloss.Style   { return selectedItemStyle }
func NormalItemStyle() lipgloss.Style     { return normalItemStyle }
func StatusBarStyle() lipgloss.Style      { return statusBarStyle }
func HelpBarStyle() lipgloss.Style        { return helpBarStyle }
func OverlayStyle() lipgloss.Style        { return overlayStyle }
func DialogStyle() lipgloss.Style         { return dialogStyle }
func FailureColor() color.Color { return failureColor }

// StatusStyle returns the appropriate style for a run status/conclusion.
func StatusStyle(status, conclusion string) lipgloss.Style {
	switch {
	case status == "in_progress" || status == "queued" || status == "waiting":
		return statusRunning
	case conclusion == "success":
		return statusSuccess
	case conclusion == "failure":
		return statusFailure
	case conclusion == "cancelled":
		return statusCancelled
	default:
		return statusPending
	}
}

// StatusIcon returns a unicode icon for a run status/conclusion.
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
	case conclusion == "cancelled":
		return "⊘"
	case conclusion == "skipped":
		return "⊘"
	default:
		return "?"
	}
}
