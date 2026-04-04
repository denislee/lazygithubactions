package components

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

type LogViewer struct {
	viewport viewport.Model
	title    string
	width    int
	height   int
	ready    bool
}

func NewLogViewer() LogViewer {
	return LogViewer{}
}

func (l *LogViewer) SetContent(title, content string) {
	l.title = title
	if !l.ready {
		l.viewport = viewport.New()
		l.ready = true
	}
	l.viewport.SetContent(content)
	l.viewport.GotoTop()
}

func (l *LogViewer) SetSize(w, h int) {
	l.width = w
	l.height = h
	if !l.ready {
		l.viewport = viewport.New()
		l.ready = true
	}
	l.viewport.SetWidth(w - 4)
	l.viewport.SetHeight(h - 6)
}

func (l *LogViewer) Update(msg tea.Msg) tea.Cmd {
	if !l.ready {
		return nil
	}
	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)
	return cmd
}

func (l *LogViewer) View() string {
	header := theme.TitleStyle.Render("Logs: " + l.title)
	footer := theme.HelpBarStyle.Render("↑/↓: scroll  esc: back")
	body := ""
	if l.ready {
		body = l.viewport.View()
	}
	return theme.ActivePanelStyle.Width(l.width).Height(l.height).Render(
		header + "\n" + body + "\n" + footer,
	)
}
