package components

import (
	"fmt"
	"regexp"
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
	sectionHeader     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	stepHeader        = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AAFF"))

	// Matches GitHub Actions timestamp like 2026-04-02T11:18:41.5794341Z
	timestampRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s*`)
)

type parsedLine struct {
	text      string // cleaned message (no job/step prefix, no timestamp)
	raw       string // original line for clipboard copy
	isHeader  bool   // section header (job/step change)
	headerJob string
}

type LogViewer struct {
	title   string
	content string // raw content for full copy
	lines   []parsedLine
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

	var lastSection string

	for _, raw := range rawLines {
		if raw == "" {
			l.lines = append(l.lines, parsedLine{raw: raw})
			continue
		}

		// gh log format: "JobName\tStepName\tTimestamp Message"
		parts := strings.SplitN(raw, "\t", 3)

		if len(parts) == 3 {
			job := parts[0]
			step := parts[1]
			msg := parts[2]

			// Strip timestamp from message
			msg = timestampRe.ReplaceAllString(msg, "")

			// Insert section header when job/step changes
			section := job + " / " + step
			if section != lastSection {
				lastSection = section
				l.lines = append(l.lines, parsedLine{
					text:      section,
					raw:       "",
					isHeader:  true,
					headerJob: job,
				})
			}

			l.lines = append(l.lines, parsedLine{
				text: msg,
				raw:  raw,
			})
		} else {
			// Not tab-delimited — show as-is (strip timestamp if present)
			msg := timestampRe.ReplaceAllString(raw, "")
			l.lines = append(l.lines, parsedLine{
				text: msg,
				raw:  raw,
			})
		}
	}
}

func (l *LogViewer) SetSize(w, h int) {
	l.width = w
	l.height = h
}

func (l *LogViewer) visibleHeight() int {
	h := l.height - 3
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
			sb.WriteString(l.lines[i].text)
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
	contentWidth := l.width - 4 - l.gutterWidth

	var b strings.Builder
	for i := l.yOffset; i < len(l.lines) && i < l.yOffset+vh; i++ {
		line := l.lines[i]

		isSelected := l.visualMode && l.inSelection(i)
		isCursor := i == l.cursor

		if line.isHeader {
			// Section headers: no line number, styled differently
			gutter := strings.Repeat(" ", l.gutterWidth)
			text := line.text
			if len(text) > contentWidth {
				text = text[:contentWidth]
			}
			headerText := sectionHeader.Render("▸ " + text)
			if isCursor {
				b.WriteString(gutterStyleCursor.Render(gutter) + lineStyleCursor.Render(headerText))
			} else if isSelected {
				b.WriteString(gutter + lineStyleSelect.Render(headerText))
			} else {
				b.WriteString(gutter + headerText)
			}
		} else {
			// Regular line with line number
			lineNum := fmt.Sprintf("%*d ", l.gutterWidth-1, i+1)
			text := line.text
			if len(text) > contentWidth {
				text = text[:contentWidth]
			}

			if isCursor && isSelected {
				b.WriteString(gutterStyleCursor.Render(lineNum) + lineStyleSelect.Render(text))
			} else if isCursor {
				b.WriteString(gutterStyleCursor.Render(lineNum) + lineStyleCursor.Render(text))
			} else if isSelected {
				b.WriteString(gutterStyleNormal.Render(lineNum) + lineStyleSelect.Render(text))
			} else {
				b.WriteString(gutterStyleNormal.Render(lineNum) + text)
			}
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
