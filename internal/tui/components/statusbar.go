package components

import (
	"charm.land/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

type StatusBar struct {
	width   int
	message string
	isError bool
}

func NewStatusBar() StatusBar {
	return StatusBar{}
}

func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

func (s *StatusBar) SetMessage(msg string, isError bool) {
	s.message = msg
	s.isError = isError
}

func (s *StatusBar) Clear() {
	s.message = ""
	s.isError = false
}

func (s StatusBar) View() string {
	if s.message == "" {
		return ""
	}
	style := theme.StatusBarStyle.Width(s.width)
	if s.isError {
		style = style.Foreground(lipgloss.Color("#FF4444"))
	}
	return style.Render(s.message)
}

func HelpBar(width int, activeView string) string {
	var help string
	switch activeView {
	case "detail":
		help = "esc:back  j/k:nav  space:toggle  L:logs  R:rerun  C:cancel  s:skipped  q:quit"
	case "log":
		help = "esc:back  \u2191/\u2193:scroll  q:quit"
	default:
		help = "tab:switch  j/k:nav  enter:select  T:trigger  C:cancel  R:rerun  L:logs  D:download  r:refresh  ctrl+k:search  q:quit"
	}
	return theme.HelpBarStyle.Width(width).Render(help)
}
