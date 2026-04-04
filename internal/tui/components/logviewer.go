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
	gutterStyleNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	gutterStyleCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	lineStyleCursor   = lipgloss.NewStyle().Background(lipgloss.Color("#333355"))
	lineStyleSelect   = lipgloss.NewStyle().Background(lipgloss.Color("#3A3A5C"))
	sectionStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AAFF"))
	timestampStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
)

type logLine struct {
	timestamp string // extracted timestamp (e.g. "2026-04-02T11:18:41")
	message   string // log message without job/step prefix
	raw       string // original line for clipboard copy
	section   string // which job/step section this line belongs to
}

type LogViewer struct {
	title   string
	content string // raw content for full copy
	lines   []logLine
	width   int
	height  int

	cursor  int
	yOffset int

	visualMode  bool
	selectStart int

	gutterWidth int
}

func NewLogViewer() LogViewer {
	return LogViewer{}
}

func (l *LogViewer) SetContent(title, content string) {
	l.title = title
	l.content = content
	l.visualMode = false
	l.cursor = 0
	l.yOffset = 0
	l.parseContent(content)
	l.gutterWidth = len(fmt.Sprintf("%d", len(l.lines))) + 1
	if l.gutterWidth < 4 {
		l.gutterWidth = 4
	}
}

func (l *LogViewer) parseContent(content string) {
	rawLines := strings.Split(content, "\n")
	l.lines = nil

	currentSection := ""

	for _, raw := range rawLines {
		if raw == "" {
			l.lines = append(l.lines, logLine{raw: raw, section: currentSection})
			continue
		}

		// gh log format: "JobName\tStepName\tTimestamp Message"
		parts := strings.SplitN(raw, "\t", 3)

		if len(parts) == 3 {
			job := parts[0]
			step := parts[1]
			msg := parts[2]

			currentSection = job + " / " + step

			// Extract timestamp from message
			ts, rest := extractTimestamp(msg)

			l.lines = append(l.lines, logLine{
				timestamp: ts,
				message:   rest,
				raw:       raw,
				section:   currentSection,
			})
		} else {
			// Not tab-delimited — show as-is
			ts, rest := extractTimestamp(raw)
			l.lines = append(l.lines, logLine{
				timestamp: ts,
				message:   rest,
				raw:       raw,
				section:   currentSection,
			})
		}
	}
}

// extractTimestamp pulls a GitHub Actions timestamp from the start of a line.
// Returns (short timestamp, remaining message).
func extractTimestamp(s string) (string, string) {
	// Format: 2026-04-02T11:18:41.5794341Z ...
	if len(s) > 28 && s[4] == '-' && s[7] == '-' && s[10] == 'T' {
		// Find the Z that ends the timestamp
		zIdx := strings.IndexByte(s[11:], 'Z')
		if zIdx > 0 {
			zPos := 11 + zIdx + 1 // position after 'Z'
			ts := s[11:19]        // just HH:MM:SS
			rest := s[zPos:]
			rest = strings.TrimLeft(rest, " ")
			return ts, rest
		}
	}
	return "", s
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

func (l *LogViewer) currentSection() string {
	if l.cursor >= 0 && l.cursor < len(l.lines) {
		return l.lines[l.cursor].section
	}
	return ""
}

func (l *LogViewer) Update(msg tea.Msg) tea.Cmd {
	if len(l.lines) == 0 {
		return nil
	}
	last := len(l.lines) - 1

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
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

		case msg.String() == "g":
			l.cursor = 0
			l.yOffset = 0
			return nil

		case msg.String() == "G":
			l.cursor = last
			l.ensureVisible()
			return nil

		case msg.String() == "y" && !l.visualMode:
			return l.copyToClipboard(l.content, len(l.lines))

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

	var sb strings.Builder
	for i := start; i <= end; i++ {
		if l.lines[i].raw != "" {
			sb.WriteString(l.lines[i].raw)
		} else {
			sb.WriteString(l.lines[i].message)
		}
		if i < end {
			sb.WriteString("\n")
		}
	}
	count := end - start + 1
	l.visualMode = false
	return l.copyToClipboard(sb.String(), count)
}

func (l *LogViewer) copyToClipboard(text string, lineCount int) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(text)
		return LogCopiedMsg{Lines: lineCount, Err: err}
	}
}

func (l *LogViewer) View() string {
	// Fixed title bar: title + line info + current section
	titlePart := theme.TitleStyle.Render("Logs: " + l.title)
	lineInfo := theme.NormalItemStyle.Render(fmt.Sprintf(" [%d/%d]", l.cursor+1, len(l.lines)))

	section := l.currentSection()
	sectionPart := ""
	if section != "" {
		sectionPart = sectionStyle.Render("  ▸ " + section)
	}

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
			Render(fmt.Sprintf("  VISUAL (%d lines)", count))
	}

	headerLine := titlePart + lineInfo + sectionPart + modeIndicator

	// Render visible log lines
	vh := l.visibleHeight()
	contentWidth := l.width - 4 - l.gutterWidth - 10 // borders/padding - gutter - timestamp

	var b strings.Builder
	for i := l.yOffset; i < len(l.lines) && i < l.yOffset+vh; i++ {
		line := l.lines[i]

		isSelected := l.visualMode && l.inSelection(i)
		isCursor := i == l.cursor

		lineNum := fmt.Sprintf("%*d ", l.gutterWidth-1, i+1)
		ts := ""
		if line.timestamp != "" {
			ts = timestampStyle.Render(line.timestamp) + " "
		}

		msg := line.message
		if len(msg) > contentWidth && contentWidth > 0 {
			msg = msg[:contentWidth]
		}

		var gutter string
		if isCursor {
			gutter = gutterStyleCursor.Render(lineNum)
		} else {
			gutter = gutterStyleNormal.Render(lineNum)
		}

		if isCursor && isSelected {
			b.WriteString(gutter + ts + lineStyleSelect.Render(msg))
		} else if isCursor {
			b.WriteString(gutter + ts + lineStyleCursor.Render(msg))
		} else if isSelected {
			b.WriteString(gutter + ts + lineStyleSelect.Render(msg))
		} else {
			b.WriteString(gutter + ts + msg)
		}
		b.WriteString("\n")
	}

	return theme.ActivePanelStyle.Width(l.width).Height(l.height).Render(
		headerLine + "\n" + b.String(),
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
