package components

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

var (
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))
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

func (r *RunList) Empty() bool {
	return len(r.runs) == 0
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
		pageSize := r.height / 3 // each run takes ~2-3 lines
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
		case msg.String() == "ctrl+n" || msg.String() == "ctrl+f":
			r.cursor += pageSize
			if r.cursor >= len(r.runs) {
				r.cursor = len(r.runs) - 1
			}
		case msg.String() == "ctrl+p" || msg.String() == "ctrl+b":
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

	// Each run takes 2 lines + 1 blank = 3 lines per entry
	linesPerRun := 3
	visibleRuns := (r.height - 3) / linesPerRun
	if visibleRuns < 1 {
		visibleRuns = 1
	}

	start := 0
	if r.cursor >= visibleRuns {
		start = r.cursor - visibleRuns + 1
	}

	maxWidth := r.width - 6 // account for borders + padding + prefix
	for i := start; i < len(r.runs) && i < start+visibleRuns; i++ {
		run := r.runs[i]
		icon := theme.StatusIcon(run.Status, run.Conclusion)
		stStyle := theme.StatusStyle(run.Status, run.Conclusion)

		// Line 1: status icon, workflow name, conclusion/status, time ago
		conclusion := run.Conclusion
		if conclusion == "" {
			conclusion = run.Status
		}
		ago := timeAgo(run.UpdatedAt)
		dur := duration(run.CreatedAt, run.UpdatedAt)

		prefix := "  "
		if i == r.cursor {
			prefix = "> "
		}

		line1 := fmt.Sprintf("%s%s %s  %s  %s  %s",
			prefix,
			stStyle.Render(icon),
			truncate(run.WorkflowName, min(25, maxWidth/2)),
			stStyle.Render(conclusion),
			dimStyle.Render(dur),
			dimStyle.Render(ago),
		)

		// Line 2: branch, event, commit title
		displayTitle := run.DisplayTitle
		if displayTitle == "" {
			displayTitle = run.Name
		}
		line2 := fmt.Sprintf("    %s %s  %s",
			dimStyle.Render(run.Branch),
			dimStyle.Render("["+run.Event+"]"),
			dimStyle.Render(truncate(displayTitle, min(40, maxWidth-20))),
		)

		if i == r.cursor && r.focused {
			b.WriteString(theme.SelectedItemStyle.Render(line1) + "\n")
			b.WriteString(line2 + "\n")
		} else if i == r.cursor {
			b.WriteString(theme.NormalItemStyle.Render(line1) + "\n")
			b.WriteString(line2 + "\n")
		} else {
			b.WriteString(theme.NormalItemStyle.Render(line1) + "\n")
			b.WriteString(line2 + "\n")
		}
		b.WriteString("\n")
	}

	content := b.String()
	style := theme.PanelStyle
	if r.focused {
		style = theme.ActivePanelStyle
	}
	return style.Width(r.width).Height(r.height).Render(content)
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
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

func duration(start, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return ""
	}
	d := end.Sub(start)
	if d < 0 {
		return ""
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
