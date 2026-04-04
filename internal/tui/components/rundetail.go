package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

type RunDetail struct {
	detail *models.RunDetail
	cursor int
	width  int
	height int
	lines  []detailLine // pre-computed lines for rendering
}

type detailLine struct {
	text    string
	isJob   bool
	indent  int
	status  string
	conclusion string
}

func NewRunDetail() RunDetail {
	return RunDetail{}
}

func (d *RunDetail) SetDetail(detail *models.RunDetail) {
	d.detail = detail
	d.cursor = 0
	d.buildLines()
}

func (d *RunDetail) SetSize(w, h int) {
	d.width = w
	d.height = h
}

func (d *RunDetail) buildLines() {
	d.lines = nil
	if d.detail == nil {
		return
	}
	for _, job := range d.detail.Jobs {
		d.lines = append(d.lines, detailLine{
			text:       job.Name,
			isJob:      true,
			status:     job.Status,
			conclusion: job.Conclusion,
		})
		for _, step := range job.Steps {
			d.lines = append(d.lines, detailLine{
				text:       step.Name,
				indent:     1,
				status:     step.Status,
				conclusion: step.Conclusion,
			})
		}
	}
}

func (d *RunDetail) Update(msg tea.Msg) tea.Cmd {
	if d.detail == nil {
		return nil
	}
	total := len(d.lines)
	if total == 0 {
		return nil
	}
	pageSize := d.height - 5
	if pageSize < 1 {
		pageSize = 1
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if d.cursor > 0 {
				d.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if d.cursor < total-1 {
				d.cursor++
			}
		case msg.String() == "ctrl+n":
			d.cursor += pageSize
			if d.cursor >= total {
				d.cursor = total - 1
			}
		case msg.String() == "ctrl+p":
			d.cursor -= pageSize
			if d.cursor < 0 {
				d.cursor = 0
			}
		}
	}
	return nil
}

func (d *RunDetail) View() string {
	if d.detail == nil {
		return theme.ActivePanelStyle.Width(d.width).Height(d.height).Render("Loading run details...")
	}

	var b strings.Builder
	run := d.detail
	header := fmt.Sprintf("%s  #%d  %s  %s",
		theme.StatusStyle(run.Status, run.Conclusion).Render(theme.StatusIcon(run.Status, run.Conclusion)),
		run.ID, run.WorkflowName, run.Branch)
	b.WriteString(theme.TitleStyle.Render(header) + "\n\n")

	// Scrollable area
	visibleHeight := d.height - 5 // header + border padding
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	start := 0
	if d.cursor >= visibleHeight {
		start = d.cursor - visibleHeight + 1
	}

	for i := start; i < len(d.lines) && i < start+visibleHeight; i++ {
		line := d.lines[i]
		icon := theme.StatusIcon(line.status, line.conclusion)
		style := theme.StatusStyle(line.status, line.conclusion)

		var prefix, text string
		if line.indent == 0 {
			// Job line
			prefix = "  "
			if i == d.cursor {
				prefix = "> "
			}
			text = fmt.Sprintf("%s%s %s", prefix, style.Render(icon), line.text)
		} else {
			// Step line
			prefix = "      "
			if i == d.cursor {
				prefix = "    > "
			}
			text = fmt.Sprintf("%s%s %s", prefix, style.Render(icon), line.text)
		}
		b.WriteString(text + "\n")
	}

	if len(d.lines) == 0 {
		b.WriteString(theme.NormalItemStyle.Render("  No jobs found"))
	}

	return theme.ActivePanelStyle.Width(d.width).Height(d.height).Render(b.String())
}
