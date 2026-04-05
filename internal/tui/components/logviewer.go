package components

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
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
	logCursorBg           = lipgloss.Color("#2A2A3E")
	logSelectBg           = lipgloss.Color("#3A3A5C")
	logCursorLineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true).Background(logCursorBg)
	logSelectLineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Background(logSelectBg)
	logCursorStyle        = lipgloss.NewStyle().Background(logCursorBg)
	logSelectStyle        = lipgloss.NewStyle().Background(logSelectBg)
	logTimestampStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
	logSectionStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AAFF"))
	logDimStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	logStatusBarBg        = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e"))
	logModeStyle          = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	logStatusText         = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#AAAAAA"))
	logStatusPos          = lipgloss.NewStyle().Background(lipgloss.Color("#1a1a2e")).Foreground(lipgloss.Color("#FFFFFF"))
	logErrorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	logSearchMatchStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#FFAA00")).Foreground(lipgloss.Color("#000000"))
)

// Matches GitHub Actions timestamp like 2026-04-02T11:18:41.5794341Z (fractional seconds optional)
var tsRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\s*`)

var errorRe = regexp.MustCompile(`(?i)error`)

type logLine struct {
	date    string // "YYYY-MM-DD"
	time    string // "HH:MM:SS"
	message string
	raw     string
	section string
}

type LogViewer struct {
	title   string
	content string
	lines   []logLine
	logDate string // date from the first timestamped line, shown in title
	width   int
	height  int

	cursor     int
	scroll     int // first visible line
	selectFrom int // -1 if no visual selection

	gutterWidth int

	// Search
	searching   bool
	searchInput textinput.Model
	searchQuery string
	searchHits  []int // line indices that match
	searchIdx   int   // current position in searchHits
}

func NewLogViewer() LogViewer {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.SetWidth(30)
	return LogViewer{selectFrom: -1, searchInput: ti, searchIdx: -1}
}

func (l *LogViewer) SetContent(title, content string) {
	l.title = title
	l.content = content
	l.selectFrom = -1
	l.cursor = 0
	l.scroll = 0
	l.searching = false
	l.searchQuery = ""
	l.searchHits = nil
	l.searchIdx = -1
	l.parseContent(content)
	l.gutterWidth = len(fmt.Sprintf("%d", len(l.lines))) + 1
	if l.gutterWidth < 4 {
		l.gutterWidth = 4
	}
}

func (l *LogViewer) parseContent(content string) {
	rawLines := strings.Split(content, "\n")
	l.lines = nil
	l.logDate = ""

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

			date, ts, rest := extractTimestamp(msg)
			if l.logDate == "" && date != "" {
				l.logDate = date
			}

			l.lines = append(l.lines, logLine{
				date:    date,
				time:    ts,
				message: rest,
				raw:     raw,
				section: currentSection,
			})
		} else {
			date, ts, rest := extractTimestamp(raw)
			if l.logDate == "" && date != "" {
				l.logDate = date
			}
			l.lines = append(l.lines, logLine{
				date:    date,
				time:    ts,
				message: rest,
				raw:     raw,
				section: currentSection,
			})
		}
	}
}

func extractTimestamp(s string) (date, timeStr, rest string) {
	loc := tsRe.FindStringIndex(s)
	if loc != nil {
		raw := s[loc[0]:loc[1]]
		// raw is like "2026-04-04T17:01:15.5264109Z " or "2026-04-04T17:01:15Z "
		if len(raw) >= 19 {
			date = raw[:10]      // "YYYY-MM-DD"
			timeStr = raw[11:19] // "HH:MM:SS"
			rest = s[loc[1]:]
			return
		}
	}
	return "", "", s
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

func (l *LogViewer) updateSearch() {
	q := l.searchInput.Value()
	l.searchQuery = q
	l.searchHits = nil
	l.searchIdx = -1
	if q == "" {
		return
	}
	lower := strings.ToLower(q)
	for i, line := range l.lines {
		if strings.Contains(strings.ToLower(line.message), lower) {
			l.searchHits = append(l.searchHits, i)
		}
	}
}

func (l *LogViewer) jumpToNextMatch() {
	if len(l.searchHits) == 0 {
		return
	}
	// Find first hit after cursor
	for i, line := range l.searchHits {
		if line > l.cursor {
			l.searchIdx = i
			l.cursor = line
			l.ensureVisible()
			return
		}
	}
	// Wrap around
	l.searchIdx = 0
	l.cursor = l.searchHits[0]
	l.ensureVisible()
}

func (l *LogViewer) jumpToPrevMatch() {
	if len(l.searchHits) == 0 {
		return
	}
	// Find last hit before cursor
	for i := len(l.searchHits) - 1; i >= 0; i-- {
		if l.searchHits[i] < l.cursor {
			l.searchIdx = i
			l.cursor = l.searchHits[i]
			l.ensureVisible()
			return
		}
	}
	// Wrap around
	l.searchIdx = len(l.searchHits) - 1
	l.cursor = l.searchHits[l.searchIdx]
	l.ensureVisible()
}

func (l *LogViewer) Update(msg tea.Msg) tea.Cmd {
	if len(l.lines) == 0 {
		return nil
	}
	last := len(l.lines) - 1

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Search mode: capture input
		if l.searching {
			switch msg.String() {
			case "enter":
				l.searching = false
				l.searchInput.Blur()
				l.updateSearch()
				l.jumpToNextMatch()
			case "esc":
				l.searching = false
				l.searchInput.Blur()
				l.searchQuery = ""
				l.searchHits = nil
				l.searchIdx = -1
			default:
				var cmd tea.Cmd
				l.searchInput, cmd = l.searchInput.Update(msg)
				return cmd
			}
			return nil
		}

		switch {
		case msg.String() == "/":
			l.searching = true
			l.searchInput.SetValue("")
			return l.searchInput.Focus()

		case msg.String() == "n":
			l.jumpToNextMatch()

		case msg.String() == "N":
			l.jumpToPrevMatch()

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

		case msg.String() == "Y":
			count := len(l.lines)
			return func() tea.Msg {
				err := clipboard.WriteAll(l.content)
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

// renderMessage applies error highlighting and search match highlighting to a message string.
func (l *LogViewer) renderMessage(msg string) string {
	if msg == "" {
		return msg
	}

	// Build a list of styled spans
	type span struct {
		start, end int
		style      string // "error" or "search"
	}

	var spans []span

	// Find error matches
	for _, loc := range errorRe.FindAllStringIndex(msg, -1) {
		spans = append(spans, span{loc[0], loc[1], "error"})
	}

	// Find search matches (search takes visual priority over error)
	if l.searchQuery != "" {
		lower := strings.ToLower(msg)
		q := strings.ToLower(l.searchQuery)
		start := 0
		for {
			idx := strings.Index(lower[start:], q)
			if idx == -1 {
				break
			}
			abs := start + idx
			spans = append(spans, span{abs, abs + len(q), "search"})
			start = abs + len(q)
		}
	}

	if len(spans) == 0 {
		return msg
	}

	// Merge spans: search overrides error when they overlap.
	// Simple approach: paint character-by-character.
	charStyle := make([]string, len(msg))
	for _, s := range spans {
		for i := s.start; i < s.end && i < len(msg); i++ {
			if s.style == "search" || charStyle[i] == "" {
				charStyle[i] = s.style
			}
		}
	}

	var sb strings.Builder
	i := 0
	for i < len(msg) {
		if charStyle[i] == "" {
			// Find run of unstyled chars
			j := i
			for j < len(msg) && charStyle[j] == "" {
				j++
			}
			sb.WriteString(msg[i:j])
			i = j
		} else {
			style := charStyle[i]
			j := i
			for j < len(msg) && charStyle[j] == style {
				j++
			}
			chunk := msg[i:j]
			switch style {
			case "error":
				sb.WriteString(logErrorStyle.Render(chunk))
			case "search":
				sb.WriteString(logSearchMatchStyle.Render(chunk))
			}
			i = j
		}
	}
	return sb.String()
}

func (l *LogViewer) isSearchHit(lineIdx int) bool {
	for _, h := range l.searchHits {
		if h == lineIdx {
			return true
		}
		if h > lineIdx {
			break
		}
	}
	return false
}

func (l *LogViewer) View() string {
	innerWidth := l.width - 4 // borders + padding

	// Fixed header: title + section on left, date on right
	leftHeader := theme.TitleStyle.Render("Logs: " + l.title)
	lineInfo := theme.NormalItemStyle.Render(fmt.Sprintf(" [%d/%d]", l.cursor+1, len(l.lines)))
	section := l.currentSection()
	if section != "" {
		leftHeader += lineInfo + logSectionStyle.Render("  ▸ "+section)
	} else {
		leftHeader += lineInfo
	}

	rightHeader := ""
	if l.logDate != "" {
		rightHeader = logTimestampStyle.Render(l.logDate)
	}
	headerGap := innerWidth - lipgloss.Width(leftHeader) - lipgloss.Width(rightHeader)
	if headerGap < 1 {
		headerGap = 1
	}
	headerLine := leftHeader + strings.Repeat(" ", headerGap) + rightHeader

	// Content area
	vh := l.viewHeight()
	tsCol := 10 // "HH:MM:SS" + 2 padding
	contentWidth := innerWidth - l.gutterWidth - tsCol

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

		msg := line.message
		if contentWidth > 0 && len(msg) > contentWidth {
			msg = msg[:contentWidth]
		}

		// Right-align time: pad message to fill content area, then append time
		ts := ""
		msgWidth := lipgloss.Width(msg)
		if line.time != "" {
			padding := contentWidth - msgWidth
			if padding < 1 {
				padding = 1
			}
			ts = strings.Repeat(" ", padding) + line.time
			// no separate styling for time here — it inherits from the line bg
		} else {
			// No timestamp: pad message to fill the full line
			padding := contentWidth + tsCol - msgWidth
			if padding > 0 {
				ts = strings.Repeat(" ", padding)
			}
		}

		if isCursor || isSelected {
			bgStyle := logCursorStyle
			numStyle := logCursorLineNumStyle
			if isSelected {
				bgStyle = logSelectStyle
				numStyle = logSelectLineNumStyle
			}
			// Combine cursor+select: cursor takes precedence for bg
			if isCursor && isSelected {
				bgStyle = logSelectStyle
				numStyle = logSelectLineNumStyle
			}
			lineContent := msg + ts
			b.WriteString(numStyle.Render(num) + bgStyle.Width(innerWidth-l.gutterWidth).Render(lineContent))
		} else {
			// Apply error + search highlighting to message
			styledMsg := l.renderMessage(msg)
			b.WriteString(logLineNumStyle.Render(num))
			b.WriteString(styledMsg)
			if line.time != "" {
				padding := contentWidth - msgWidth
				if padding < 1 {
					padding = 1
				}
				b.WriteString(strings.Repeat(" ", padding) + logTimestampStyle.Render(line.time))
			}
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
	if l.searching {
		mode = " SEARCH "
	} else if l.selectFrom != -1 {
		mode = " VISUAL "
	} else {
		mode = " NORMAL "
	}

	var statusContent string
	if l.searching {
		statusContent = " /" + l.searchInput.View()
	} else {
		help := " [j/k]move [/]search [n/N]next/prev [v]isual [y]ank [Y]ank all [g/G]top/bot [esc]back "
		if l.searchQuery != "" {
			matchInfo := fmt.Sprintf(" [%d matches]", len(l.searchHits))
			help = " [j/k]move [/]search [n/N]next/prev" + matchInfo + " [v]isual [y]ank [Y]ank all [esc]back "
		}
		statusContent = help
	}

	pos := fmt.Sprintf(" %d/%d ", l.cursor+1, len(l.lines))

	left := logModeStyle.Render(mode)
	mid := logStatusText.Render(statusContent)
	right := logStatusPos.Render(pos)
	gap := innerWidth - lipgloss.Width(left) - lipgloss.Width(mid) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	pad := logStatusBarBg.Width(gap).Render("")
	bar := logStatusBarBg.Width(innerWidth).Render(left + mid + pad + right)

	return theme.ActivePanelStyle.Width(l.width).Height(l.height).Render(
		headerLine + "\n" + b.String() + bar,
	)
}

// InVisualMode reports whether visual selection is active.
func (l *LogViewer) InVisualMode() bool {
	return l.selectFrom != -1
}

// InSearchMode reports whether search input is active.
func (l *LogViewer) InSearchMode() bool {
	return l.searching
}

// HelpText returns help for the app-level status bar (kept minimal since pager has its own).
func (l *LogViewer) HelpText() string {
	return ""
}
