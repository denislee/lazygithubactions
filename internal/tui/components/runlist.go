package components

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

type RunList struct {
	runs    []models.WorkflowRun
	repo    string
	cursor  int
	width   int
	height  int
	focused bool
}

func NewRunList() RunList {
	return RunList{}
}

func (r *RunList) SetRuns(runs []models.WorkflowRun, repo string) {
	r.runs = runs
	r.repo = repo
	if r.cursor >= len(runs) && len(runs) > 0 {
		r.cursor = len(runs) - 1
	}
}

func (r *RunList) SetSize(w, h int) {
	r.width = w
	r.height = h
}

func (r *RunList) SetFocused(f bool) {
	r.focused = f
}

func (r *RunList) SelectedRun() *models.WorkflowRun {
	if len(r.runs) == 0 {
		return nil
	}
	if r.cursor >= len(r.runs) {
		r.cursor = len(r.runs) - 1
	}
	return &r.runs[r.cursor]
}

func (r *RunList) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		pageSize := r.height - 3
		if pageSize < 1 {
			pageSize = 1
		}
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if r.cursor > 0 {
				r.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if r.cursor < len(r.runs)-1 {
				r.cursor++
			}
		case msg.String() == "ctrl+n":
			r.cursor += pageSize
			if r.cursor >= len(r.runs) {
				r.cursor = len(r.runs) - 1
			}
		case msg.String() == "ctrl+p":
			r.cursor -= pageSize
			if r.cursor < 0 {
				r.cursor = 0
			}
		}
	}
	return nil
}

func (r *RunList) View() string {
	var b strings.Builder
	title := theme.TitleStyle.Render(fmt.Sprintf("Workflow Runs — %s", r.repo))
	b.WriteString(title + "\n")

	if len(r.runs) == 0 {
		b.WriteString(theme.NormalItemStyle.Render("  No workflow runs"))
		content := b.String()
		style := theme.PanelStyle
		if r.focused {
			style = theme.ActivePanelStyle
		}
		return style.Width(r.width).Height(r.height).Render(content)
	}

	visibleHeight := r.height - 3
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	start := 0
	if r.cursor >= visibleHeight {
		start = r.cursor - visibleHeight + 1
	}

	for i := start; i < len(r.runs) && i < start+visibleHeight; i++ {
		run := r.runs[i]
		icon := theme.StatusIcon(run.Status, run.Conclusion)
		stStyle := theme.StatusStyle(run.Status, run.Conclusion)
		status := stStyle.Render(icon)

		ago := timeAgo(run.UpdatedAt)
		line := fmt.Sprintf("%s  #%d %-20s %-12s %s",
			status, run.ID, truncate(run.WorkflowName, 20), truncate(run.Branch, 12), ago)

		if i == r.cursor && r.focused {
			line = theme.SelectedItemStyle.Render("> " + line)
		} else if i == r.cursor {
			line = theme.NormalItemStyle.Render("> " + line)
		} else {
			line = theme.NormalItemStyle.Render("  " + line)
		}
		b.WriteString(line + "\n")
	}

	content := b.String()
	style := theme.PanelStyle
	if r.focused {
		style = theme.ActivePanelStyle
	}
	return style.Width(r.width).Height(r.height).Render(content)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
