package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

// Re-export styles from theme package for backward compatibility.
func PanelStyle() lipgloss.Style        { return theme.PanelStyle }
func ActivePanelStyle() lipgloss.Style  { return theme.ActivePanelStyle }
func TitleStyle() lipgloss.Style        { return theme.TitleStyle }
func SelectedItemStyle() lipgloss.Style { return theme.SelectedItemStyle }
func NormalItemStyle() lipgloss.Style   { return theme.NormalItemStyle }
func StatusBarStyle() lipgloss.Style    { return theme.StatusBarStyle }
func HelpBarStyle() lipgloss.Style      { return theme.HelpBarStyle }
func OverlayStyle() lipgloss.Style      { return theme.OverlayStyle }
func DialogStyle() lipgloss.Style       { return theme.DialogStyle }
func StatusStyle(status, conclusion string) lipgloss.Style {
	return theme.StatusStyle(status, conclusion)
}
func StatusIcon(status, conclusion string) string {
	return theme.StatusIcon(status, conclusion)
}
