package components

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
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
	logLineNumStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	logCursorLineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	logSelectLineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
	logCursorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	logSelectStyle        = lipgloss.NewStyle().Background(lipgloss.Color("#3A3A5C"))
	logTimestampStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	logSectionStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AAFF"))
	logDimStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	logStatusBarBg        = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e"))
	logModeStyle          = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	logStatusText         = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#AAAAAA"))
	logStatusPos          = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#FFFFFF"))
)

// Matches GitHub Actions timestamp like 2026-04-02T11:18:41.5794341Z
var tsRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z\s*`)

type logLine struct {
	timestamp string
	message   string
	raw       string
	section   string
}

type LogViewer struct {
	title   string
	content string
	lines   []logLine
	width   int
	height  int

	cursor     int
	scroll     int // first visible line
	selectFrom int // -1 if no visual selection

	gutterWidth int
}

func NewLogViewer() LogViewer {
	return LogViewer{selectFrom: -1}
}

func (l *LogViewer) SetContent(title, content string) {
	l.title = title
	l.content = content
	l.selectFrom = -1
	l.cursor = 0
	l.scroll = 0
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

		parts := strings.SplitN(raw, "\t", 3)

		if len(parts) == 3 {
			job := parts[0]
			step := parts[1]
			msg := parts[2]

			currentSection = job + " / " + step

			ts, rest := extractTimestamp(msg)

			l.lines = append(l.lines, logLine{
				timestamp: ts,
				message:   rest,
				raw:       raw,
				section:   currentSection,
			})
		} else {
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

func extractTimestamp(s string) (string, string) {
	if len(s) > 28 && s[4] == '-' && s[7] == '-' && s[10] == 'T' {
		zIdx := strings.IndexByte(s[11:], 'Z')
		if zIdx > 0 {
			zPos := 11 + zIdx + 1
			ts := s[11:19] // HH:MM:SS
			rest := strings.TrimLeft(s[zPos:], " ")
			return ts, rest
		}
	}
	return "", s
}

func (l *LogViewer) SetSize(w, h int) {
	l.width = w
	l.height = h
}

func (l *LogViewer) viewHeight() int {
	// Reserve: 2 border + 1 header + 1 status bar = 4
	h := l.height - 4
	if h < 1 {
		return 1
	}
	return h
}

func (l *LogViewer) ensureVisible() {
	vh := l.viewHeight()
	if l.cursor < l.scroll {
		l.scroll = l.cursor
	} else if l.cursor >= l.scroll+vh {
		l.scroll = l.cursor - vh + 1
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
		case key.Matches(msg, theme.Keys.Down):
			if l.cursor < last {
				l.cursor++
				l.ensureVisible()
			}

		case key.Matches(msg, theme.Keys.Up):
			if l.cursor > 0 {
				l.cursor--
				l.ensureVisible()
			}

		case msg.String() == "g":
			l.cursor = 0
			l.scroll = 0

		case msg.String() == "G":
			l.cursor = last
			l.ensureVisible()

		case key.Matches(msg, theme.HalfDown):
			half := l.viewHeight() / 2
			l.cursor += half
			if l.cursor > last {
				l.cursor = last
			}
			l.ensureVisible()

		case key.Matches(msg, theme.PageDown, theme.NextPage):
			full := l.viewHeight()
			l.cursor += full
			if l.cursor > last {
				l.cursor = last
			}
			l.ensureVisible()

		case key.Matches(msg, theme.HalfUp):
			half := l.viewHeight() / 2
			l.cursor -= half
			if l.cursor < 0 {
				l.cursor = 0
			}
			l.ensureVisible()

		case key.Matches(msg, theme.PageUp, theme.PrevPage):
			full := l.viewHeight()
			l.cursor -= full
			if l.cursor < 0 {
				l.cursor = 0
			}
			l.ensureVisible()

		case msg.String() == "v":
			if l.selectFrom != -1 {
				l.selectFrom = -1 // toggle off
			} else {
				l.selectFrom = l.cursor
			}

		case msg.String() == "y":
			text := l.selectedText()
			count := 1
			if l.selectFrom != -1 {
				lo, hi := l.selectFrom, l.cursor
				if lo > hi {
					lo, hi = hi, lo
				}
				count = hi - lo + 1
				l.selectFrom = -1
			}
			return func() tea.Msg {
				err := clipboard.WriteAll(text)
				return LogCopiedMsg{Lines: count, Err: err}
			}

		case key.Matches(msg, theme.Keys.Back):
			if l.selectFrom != -1 {
				l.selectFrom = -1
				return nil
			}
		}
	}

	return nil
}

func (l *LogViewer) selectedText() string {
	if l.selectFrom == -1 {
		// No selection — copy current line (raw)
		if l.cursor >= 0 && l.cursor < len(l.lines) {
			if l.lines[l.cursor].raw != "" {
				return l.lines[l.cursor].raw
			}
			return l.lines[l.cursor].message
		}
		return ""
	}
	lo, hi := l.selectFrom, l.cursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo < 0 {
		lo = 0
	}
	if hi >= len(l.lines) {
		hi = len(l.lines) - 1
	}
	var sb strings.Builder
	for i := lo; i <= hi; i++ {
		if l.lines[i].raw != "" {
			sb.WriteString(l.lines[i].raw)
		} else {
			sb.WriteString(l.lines[i].message)
		}
		if i < hi {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (l *LogViewer) View() string {
	// Fixed header: title + position + section
	titlePart := theme.TitleStyle.Render("Logs: " + l.title)
	lineInfo := theme.NormalItemStyle.Render(fmt.Sprintf(" [%d/%d]", l.cursor+1, len(l.lines)))

	section := l.currentSection()
	sectionPart := ""
	if section != "" {
		sectionPart = logSectionStyle.Render("  ▸ " + section)
	}

	headerLine := titlePart + lineInfo + sectionPart

	// Content area
	vh := l.viewHeight()
	contentWidth := l.width - 4 - l.gutterWidth - 10 // borders/padding - gutter - timestamp space

	lo, hi := -1, -1
	if l.selectFrom != -1 {
		lo, hi = l.selectFrom, l.cursor
		if lo > hi {
			lo, hi = hi, lo
		}
	}

	var b strings.Builder
	end := l.scroll + vh
	if end > len(l.lines) {
		end = len(l.lines)
	}

	for i := l.scroll; i < end; i++ {
		line := l.lines[i]

		isCursor := i == l.cursor
		isSelected := lo != -1 && i >= lo && i <= hi

		num := fmt.Sprintf("%*d ", l.gutterWidth-1, i+1)

		ts := ""
		if line.timestamp != "" {
			ts = logTimestampStyle.Render(line.timestamp) + " "
		}

		msg := line.message
		if contentWidth > 0 && len(msg) > contentWidth {
			msg = msg[:contentWidth]
		}

		if isCursor {
			b.WriteString(logCursorLineNumStyle.Render(num))
			b.WriteString(ts)
			if isSelected {
				b.WriteString(logSelectStyle.Render(msg))
			} else {
				b.WriteString(logCursorStyle.Render(msg))
			}
		} else if isSelected {
			b.WriteString(logSelectLineNumStyle.Render(num))
			b.WriteString(ts)
			b.WriteString(logSelectStyle.Render(msg))
		} else {
			b.WriteString(logLineNumStyle.Render(num))
			b.WriteString(ts)
			b.WriteString(msg)
		}
		b.WriteString("\n")
	}

	// Pad remaining with ~ lines
	rendered := end - l.scroll
	for rendered < vh {
		b.WriteString(logLineNumStyle.Render(strings.Repeat(" ", l.gutterWidth)))
		b.WriteString(logDimStyle.Render("~"))
		b.WriteString("\n")
		rendered++
	}

	// Status bar
	var mode string
	if l.selectFrom != -1 {
		mode = " VISUAL "
	} else {
		mode = " NORMAL "
	}

	pos := fmt.Sprintf(" %d/%d ", l.cursor+1, len(l.lines))
	help := " [j/k]move [ctrl+d/u]half [ctrl+f/b]page [v]isual [y]ank [g/G]top/bot [h]back "

	left := logModeStyle.Render(mode)
	mid := logStatusText.Render(help)
	right := logStatusPos.Render(pos)
	gap := l.width - 4 - lipgloss.Width(left) - lipgloss.Width(mid) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	pad := logStatusBarBg.Width(gap).Render("")
	bar := logStatusBarBg.Width(l.width - 4).Render(left + mid + pad + right)

	return theme.ActivePanelStyle.Width(l.width).Height(l.height).Render(
		headerLine + "\n" + b.String() + bar,
	)
}

// InVisualMode reports whether visual selection is active.
func (l *LogViewer) InVisualMode() bool {
	return l.selectFrom != -1
}

// HelpText returns help for the app-level status bar (kept minimal since pager has its own).
func (l *LogViewer) HelpText() string {
	return ""
}
