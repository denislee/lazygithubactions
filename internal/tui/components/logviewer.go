package components

import (
	"fmt"
	"strings"

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

var (
	gutterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	cursorGutter   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	cursorLine     = lipgloss.NewStyle().Background(lipgloss.Color("#333355"))
	selectionStyle = lipgloss.NewStyle().Background(lipgloss.Color("#3A3A5C"))
)

type LogViewer struct {
	title   string
	content string
	lines   []string
	width   int
	height  int

	cursor  int // current cursor line
	yOffset int // first visible line

	// Visual selection mode
	visualMode  bool
	selectStart int

	gutterWidth int // width of line number column
}

func NewLogViewer() LogViewer {
	return LogViewer{}
}

func (l *LogViewer) SetContent(title, content string) {
	l.title = title
	l.content = content
	l.lines = strings.Split(content, "\n")
	l.visualMode = false
	l.cursor = 0
	l.yOffset = 0
	l.gutterWidth = len(fmt.Sprintf("%d", len(l.lines))) + 1 // digits + space
	if l.gutterWidth < 4 {
		l.gutterWidth = 4
	}
}

func (l *LogViewer) SetSize(w, h int) {
	l.width = w
	l.height = h
}

func (l *LogViewer) visibleHeight() int {
	h := l.height - 3 // borders (2) + header line (1)
	if h < 1 {
		return 1
	}
	return h
}

// ensureVisible scrolls the viewport just enough to keep the cursor visible.
func (l *LogViewer) ensureVisible() {
	vh := l.visibleHeight()
	if l.cursor < l.yOffset {
		l.yOffset = l.cursor
	}
	if l.cursor >= l.yOffset+vh {
		l.yOffset = l.cursor - vh + 1
	}
	if l.yOffset < 0 {
		l.yOffset = 0
	}
}

func (l *LogViewer) Update(msg tea.Msg) tea.Cmd {
	if len(l.lines) == 0 {
		return nil
	}
	last := len(l.lines) - 1

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		// Line-by-line navigation
		case msg.String() == "j" || msg.String() == "down":
			if l.cursor < last {
				l.cursor++
				l.ensureVisible()
			}
			return nil

		case msg.String() == "k" || msg.String() == "up":
			if l.cursor > 0 {
				l.cursor--
				l.ensureVisible()
			}
			return nil

		// Page navigation
		case msg.String() == "ctrl+f" || msg.String() == "ctrl+n":
			l.cursor += l.visibleHeight()
			if l.cursor > last {
				l.cursor = last
			}
			l.ensureVisible()
			return nil

		case msg.String() == "ctrl+b" || msg.String() == "ctrl+p":
			l.cursor -= l.visibleHeight()
			if l.cursor < 0 {
				l.cursor = 0
			}
			l.ensureVisible()
			return nil

		// Go to top/bottom
		case msg.String() == "g":
			l.cursor = 0
			l.yOffset = 0
			return nil

		case msg.String() == "G":
			l.cursor = last
			l.ensureVisible()
			return nil

		// Copy entire log
		case msg.String() == "y" && !l.visualMode:
			return l.copyToClipboard(l.content, len(l.lines))

		// Visual mode
		case msg.String() == "v":
			if l.visualMode {
				return l.copySelection()
			}
			l.visualMode = true
			l.selectStart = l.cursor
			return nil

		case l.visualMode && msg.String() == "y":
			return l.copySelection()

		case l.visualMode && msg.String() == "esc":
			l.visualMode = false
			return nil
		}
	}

	return nil
}

func (l *LogViewer) copySelection() tea.Cmd {
	start, end := l.selectStart, l.cursor
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

	lineInfo := fmt.Sprintf(" [%d/%d]", l.cursor+1, len(l.lines))
	var modeIndicator string
	if l.visualMode {
		start, end := l.selectStart, l.cursor
		if start > end {
			start, end = end, start
		}
		count := end - start + 1
		modeIndicator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAA00")).
			Bold(true).
			Render(fmt.Sprintf(" VISUAL (%d lines)", count))
	}

	vh := l.visibleHeight()
	contentWidth := l.width - 4 - l.gutterWidth // borders/padding - gutter

	var b strings.Builder
	for i := l.yOffset; i < len(l.lines) && i < l.yOffset+vh; i++ {
		lineText := l.lines[i]
		if len(lineText) > contentWidth {
			lineText = lineText[:contentWidth]
		}

		// Line number gutter
		lineNum := fmt.Sprintf("%*d ", l.gutterWidth-1, i+1)

		isSelected := l.visualMode && l.inSelection(i)
		isCursor := i == l.cursor

		if isCursor && isSelected {
			b.WriteString(cursorGutter.Render(lineNum) + selectionStyle.Render(lineText))
		} else if isCursor {
			b.WriteString(cursorGutter.Render(lineNum) + cursorLine.Render(lineText))
		} else if isSelected {
			b.WriteString(gutterStyle.Render(lineNum) + selectionStyle.Render(lineText))
		} else {
			b.WriteString(gutterStyle.Render(lineNum) + lineText)
		}
		b.WriteString("\n")
	}

	return theme.ActivePanelStyle.Width(l.width).Height(l.height).Render(
		header + theme.NormalItemStyle.Render(lineInfo) + modeIndicator + "\n" + b.String(),
	)
}

func (l *LogViewer) inSelection(line int) bool {
	start, end := l.selectStart, l.cursor
	if start > end {
		start, end = end, start
	}
	return line >= start && line <= end
}

// HelpText returns context-sensitive help for the status bar.
func (l *LogViewer) HelpText() string {
	if l.visualMode {
		return "j/k:select  y:copy  esc:cancel  ctrl+f/b:page  g/G:top/bottom"
	}
	return "j/k:navigate  ctrl+f/b:page  g/G:top/bottom  y:copy all  v:visual select  h:back"
}
