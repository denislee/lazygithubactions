package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

// LogCopiedMsg is sent after copying text to clipboard.
type LogCopiedMsg struct {
	Lines int
	Err   error
}

type LogViewer struct {
	viewport viewport.Model
	title    string
	content  string
	lines    []string
	width    int
	height   int
	ready    bool

	// Visual selection mode
	visualMode  bool
	selectStart int // line index where v was pressed
	cursorLine  int // current cursor line in visual mode
}

func NewLogViewer() LogViewer {
	return LogViewer{}
}

func (l *LogViewer) SetContent(title, content string) {
	l.title = title
	l.content = content
	l.lines = strings.Split(content, "\n")
	l.visualMode = false
	if !l.ready {
		l.viewport = viewport.New()
		l.ready = true
	}
	l.viewport.SetContent(content)
	l.viewport.GotoTop()
	l.cursorLine = 0
	l.viewport.StyleLineFunc = nil
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

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		// Page navigation
		case msg.String() == "ctrl+f":
			l.viewport.PageDown()
			if l.visualMode {
				l.cursorLine += l.viewport.Height()
				if l.cursorLine >= len(l.lines) {
					l.cursorLine = len(l.lines) - 1
				}
				l.updateSelectionHighlight()
			}
			return nil

		case msg.String() == "ctrl+b":
			l.viewport.PageUp()
			if l.visualMode {
				l.cursorLine -= l.viewport.Height()
				if l.cursorLine < 0 {
					l.cursorLine = 0
				}
				l.updateSelectionHighlight()
			}
			return nil

		// Copy entire log
		case msg.String() == "y" && !l.visualMode:
			return l.copyToClipboard(l.content, len(l.lines))

		// Visual mode toggle
		case msg.String() == "v":
			if l.visualMode {
				// Copy selection and exit visual mode
				return l.copySelection()
			}
			// Enter visual mode
			l.visualMode = true
			l.cursorLine = l.viewport.YOffset()
			l.selectStart = l.cursorLine
			l.updateSelectionHighlight()
			return nil

		// In visual mode: navigate and select
		case l.visualMode && (msg.String() == "j" || msg.String() == "down"):
			if l.cursorLine < len(l.lines)-1 {
				l.cursorLine++
				// Scroll viewport if cursor goes below visible area
				if l.cursorLine >= l.viewport.YOffset()+l.viewport.Height() {
					l.viewport.ScrollDown(1)
				}
				l.updateSelectionHighlight()
			}
			return nil

		case l.visualMode && (msg.String() == "k" || msg.String() == "up"):
			if l.cursorLine > 0 {
				l.cursorLine--
				if l.cursorLine < l.viewport.YOffset() {
					l.viewport.ScrollUp(1)
				}
				l.updateSelectionHighlight()
			}
			return nil

		case l.visualMode && msg.String() == "y":
			return l.copySelection()

		case l.visualMode && msg.String() == "esc":
			l.visualMode = false
			l.viewport.StyleLineFunc = nil
			return nil
		}
	}

	// Default viewport handling (scrolling with arrows, mouse, etc.)
	if !l.visualMode {
		var cmd tea.Cmd
		l.viewport, cmd = l.viewport.Update(msg)
		return cmd
	}
	return nil
}

func (l *LogViewer) updateSelectionHighlight() {
	start, end := l.selectStart, l.cursorLine
	if start > end {
		start, end = end, start
	}

	selStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#3A3A5C")).
		Foreground(lipgloss.Color("#FFFFFF"))

	l.viewport.StyleLineFunc = func(line int) lipgloss.Style {
		if line >= start && line <= end {
			return selStyle
		}
		return lipgloss.NewStyle()
	}
}

func (l *LogViewer) copySelection() tea.Cmd {
	start, end := l.selectStart, l.cursorLine
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end >= len(l.lines) {
		end = len(l.lines) - 1
	}

	selected := strings.Join(l.lines[start:end+1], "\n")
	count := end - start + 1

	l.visualMode = false
	l.viewport.StyleLineFunc = nil

	return l.copyToClipboard(selected, count)
}

func (l *LogViewer) copyToClipboard(text string, lineCount int) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(text)
		return LogCopiedMsg{Lines: lineCount, Err: err}
	}
}

func (l *LogViewer) View() string {
	header := theme.TitleStyle.Render("Logs: " + l.title)

	var modeIndicator string
	if l.visualMode {
		start, end := l.selectStart, l.cursorLine
		if start > end {
			start, end = end, start
		}
		count := end - start + 1
		modeIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAA00")).
			Bold(true).
			Render(fmt.Sprintf(" — VISUAL (%d lines)", count))
	}

	footer := theme.HelpBarStyle.Render(l.helpText())
	body := ""
	if l.ready {
		body = l.viewport.View()
	}
	return theme.ActivePanelStyle.Width(l.width).Height(l.height).Render(
		header + modeIndicator + "\n" + body + "\n" + footer,
	)
}

func (l *LogViewer) helpText() string {
	if l.visualMode {
		return "j/k:select  y:copy  esc:cancel  ctrl+f/b:page"
	}
	return "↑/↓:scroll  ctrl+f/b:page  y:copy all  v:visual select  h:back"
}
