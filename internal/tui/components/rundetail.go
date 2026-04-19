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
	detail      *models.RunDetail
	cursor      int
	width       int
	height      int
	collapsed   map[int]bool // job index -> collapsed state
	hideSkipped bool         // hide jobs with conclusion "skipped"
}

type visibleLine struct {
	text       string
	isJob      bool
	jobIdx     int // which job this belongs to
	status     string
	conclusion string
	duration   string
	indent     int
	collapsed  bool // only for job lines: whether this job is collapsed
}

func NewRunDetail() RunDetail {
	return RunDetail{
		collapsed:   make(map[int]bool),
		hideSkipped: true,
	}
}

func (d *RunDetail) SetDetail(detail *models.RunDetail) {
	d.detail = detail
	d.cursor = 0
	d.collapsed = make(map[int]bool)
	// Default all jobs to collapsed
	if detail != nil {
		for i := range detail.Jobs {
			d.collapsed[i] = true
		}
	}
}

// UpdateDetail refreshes the data without resetting cursor or collapse state.
func (d *RunDetail) UpdateDetail(detail *models.RunDetail) {
	d.detail = detail
	// Clamp cursor if jobs changed
	lines := d.visibleLines()
	if d.cursor >= len(lines) {
		d.cursor = len(lines) - 1
	}
	if d.cursor < 0 {
		d.cursor = 0
	}
}

// HasRunningJobs returns true if any job is in progress, queued, or waiting.
func (d *RunDetail) HasRunningJobs() bool {
	if d.detail == nil {
		return false
	}
	for _, job := range d.detail.Jobs {
		if job.Status == "in_progress" || job.Status == "queued" || job.Status == "waiting" {
			return true
		}
	}
	return false
}

func (d *RunDetail) SetSize(w, h int) {
	d.width = w
	d.height = h
}

// visibleLines builds the list of currently visible lines (respecting collapsed state).
func (d *RunDetail) visibleLines() []visibleLine {
	if d.detail == nil {
		return nil
	}
	var lines []visibleLine
	for ji, job := range d.detail.Jobs {
		if d.hideSkipped && job.Conclusion == "skipped" {
			continue
		}

		dur := ""
		if !job.StartedAt.IsZero() && !job.CompletedAt.IsZero() {
			dur = duration(job.StartedAt, job.CompletedAt)
		} else if !job.StartedAt.IsZero() && job.Status == "in_progress" {
			dur = "running..."
		}

		isCollapsed := d.collapsed[ji]
		lines = append(lines, visibleLine{
			text:       job.Name,
			isJob:      true,
			jobIdx:     ji,
			status:     job.Status,
			conclusion: job.Conclusion,
			duration:   dur,
			collapsed:  isCollapsed,
		})

		if !isCollapsed {
			for _, step := range job.Steps {
				lines = append(lines, visibleLine{
					text:       step.Name,
					indent:     1,
					jobIdx:     ji,
					status:     step.Status,
					conclusion: step.Conclusion,
				})
			}
		}
	}
	return lines
}

func (d *RunDetail) SelectedJob() *models.Job {
	if d.detail == nil {
		return nil
	}
	lines := d.visibleLines()
	if d.cursor >= 0 && d.cursor < len(lines) {
		line := lines[d.cursor]
		if line.jobIdx >= 0 && line.jobIdx < len(d.detail.Jobs) {
			return &d.detail.Jobs[line.jobIdx]
		}
	}
	return nil
}

func (d *RunDetail) Update(msg tea.Msg) tea.Cmd {
	if d.detail == nil {
		return nil
	}
	lines := d.visibleLines()
	total := len(lines)
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
		case key.Matches(msg, theme.PageDown, theme.NextPage):
			d.cursor += pageSize
			if d.cursor >= total {
				d.cursor = total - 1
			}
		case key.Matches(msg, theme.PageUp, theme.PrevPage):
			d.cursor -= pageSize
			if d.cursor < 0 {
				d.cursor = 0
			}
		case msg.String() == "s":
			d.hideSkipped = !d.hideSkipped
			newLines := d.visibleLines()
			if d.cursor >= len(newLines) {
				d.cursor = len(newLines) - 1
			}
			if d.cursor < 0 {
				d.cursor = 0
			}
		case key.Matches(msg, theme.Keys.Enter) || msg.String() == "l" || msg.String() == "space" || msg.String() == " ":
			// Toggle collapse on job lines
			if d.cursor < total && lines[d.cursor].isJob {
				ji := lines[d.cursor].jobIdx
				d.collapsed[ji] = !d.collapsed[ji]
				// Clamp cursor if it would be past the new end
				newLines := d.visibleLines()
				if d.cursor >= len(newLines) {
					d.cursor = len(newLines) - 1
				}
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

	lines := d.visibleLines()

	visibleHeight := d.height - 5
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	start := 0
	if d.cursor >= visibleHeight {
		start = d.cursor - visibleHeight + 1
	}

	for i := start; i < len(lines) && i < start+visibleHeight; i++ {
		line := lines[i]
		icon := theme.StatusIcon(line.status, line.conclusion)
		style := theme.StatusStyle(line.status, line.conclusion)

		var prefix, text string
		if line.isJob {
			prefix = "  "
			if i == d.cursor {
				prefix = "> "
			}
			// Collapse indicator
			arrow := "▼"
			if line.collapsed {
				arrow = "▶"
			}
			durStr := ""
			if line.duration != "" {
				durStr = "  " + dimStyle.Render(line.duration)
			}
			text = fmt.Sprintf("%s%s %s %s%s", prefix, arrow, style.Render(icon), line.text, durStr)
		} else {
			prefix = "      "
			if i == d.cursor {
				prefix = "    > "
			}
			text = fmt.Sprintf("%s%s %s", prefix, style.Render(icon), line.text)
		}
		b.WriteString(text + "\n")
	}

	if len(lines) == 0 {
		b.WriteString(theme.NormalItemStyle.Render("  No jobs found"))
	}

	if d.hideSkipped && d.detail != nil {
		skipped := 0
		for _, job := range d.detail.Jobs {
			if job.Conclusion == "skipped" {
				skipped++
			}
		}
		if skipped > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("\n  (%d skipped jobs hidden, press s to show)", skipped)))
		}
	}

	return theme.ActivePanelStyle.Width(d.width).Height(d.height).Render(b.String())
}
