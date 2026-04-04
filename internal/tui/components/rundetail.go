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
}

func NewRunDetail() RunDetail {
	return RunDetail{}
}

func (d *RunDetail) SetDetail(detail *models.RunDetail) {
	d.detail = detail
	d.cursor = 0
}

func (d *RunDetail) SetSize(w, h int) {
	d.width = w
	d.height = h
}

func (d *RunDetail) Update(msg tea.Msg) tea.Cmd {
	if d.detail == nil {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		total := d.totalItems()
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if d.cursor > 0 {
				d.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if d.cursor < total-1 {
				d.cursor++
			}
		}
	}
	return nil
}

func (d *RunDetail) totalItems() int {
	if d.detail == nil {
		return 0
	}
	count := 0
	for _, job := range d.detail.Jobs {
		count++ // job header
		count += len(job.Steps)
	}
	return count
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

	idx := 0
	for _, job := range run.Jobs {
		jobIcon := theme.StatusIcon(job.Status, job.Conclusion)
		jobStyle := theme.StatusStyle(job.Status, job.Conclusion)
		prefix := "  "
		if idx == d.cursor {
			prefix = "> "
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, jobStyle.Render(jobIcon), job.Name))
		idx++

		for _, step := range job.Steps {
			stepIcon := theme.StatusIcon(step.Status, step.Conclusion)
			stepStyle := theme.StatusStyle(step.Status, step.Conclusion)
			prefix = "    "
			if idx == d.cursor {
				prefix = "  > "
			}
			b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, stepStyle.Render(stepIcon), step.Name))
			idx++
		}
		b.WriteString("\n")
	}

	return theme.ActivePanelStyle.Width(d.width).Height(d.height).Render(b.String())
}
