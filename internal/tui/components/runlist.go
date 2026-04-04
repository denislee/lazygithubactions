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
	Compact bool
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

func (r *RunList) Repo() string {
	return r.repo
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

func (r *RunList) ToggleCompact() {
	r.Compact = !r.Compact
}

func (r *RunList) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		linesPerRun := 3
		if r.Compact {
			linesPerRun = 1
		}
		pageSize := (r.height - 3) / linesPerRun
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
		case key.Matches(msg, theme.PageDown, theme.NextPage):
			r.cursor += pageSize
			if r.cursor >= len(r.runs) {
				r.cursor = len(r.runs) - 1
			}
		case key.Matches(msg, theme.PageUp, theme.PrevPage):
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

	linesPerRun := 3
	if r.Compact {
		linesPerRun = 1
	}
	visibleRuns := (r.height - 3) / linesPerRun
	if visibleRuns < 1 {
		visibleRuns = 1
	}

	start := 0
	if r.cursor >= visibleRuns {
		start = r.cursor - visibleRuns + 1
	}

	// Column widths (available = panel width - borders/padding - prefix - icon)
	avail := r.width - 8 // 4 border/padding + 2 prefix + 2 icon+space
	if avail < 30 {
		avail = 30
	}

	// Fixed columns
	agoCol := 10   // "  10d ago" right-aligned
	statusCol := 0 // only in full mode

	if r.Compact {
		// Compact columns: name | branch | actor | duration | age
		durCol := 8
		actorCol := 0

		// Auto-size branch and actor columns from visible runs
		branchCol := 0
		for i := start; i < len(r.runs) && i < start+visibleRuns; i++ {
			if len(r.runs[i].Branch) > branchCol {
				branchCol = len(r.runs[i].Branch)
			}
			if len(r.runs[i].Actor) > actorCol {
				actorCol = len(r.runs[i].Actor)
			}
		}
		branchCol += 1
		actorCol += 1
		if actorCol > 16 {
			actorCol = 16
		}

		fixedCols := agoCol + durCol + actorCol + 4 // 4 gaps
		maxBranch := avail - fixedCols - 10          // reserve name
		if branchCol > maxBranch {
			branchCol = maxBranch
		}
		nameCol := avail - branchCol - fixedCols
		if nameCol < 8 {
			nameCol = 8
		}

		for i := start; i < len(r.runs) && i < start+visibleRuns; i++ {
			run := r.runs[i]
			icon := theme.StatusIcon(run.Status, run.Conclusion)
			stStyle := theme.StatusStyle(run.Status, run.Conclusion)
			ago := timeAgo(run.UpdatedAt)
			dur := duration(run.CreatedAt, run.UpdatedAt)

			prefix := "  "
			if i == r.cursor {
				prefix = "> "
			}

			durPad := fmt.Sprintf("%*s", durCol, dur)
			agoPad := fmt.Sprintf("%*s", agoCol, ago)

			line := fmt.Sprintf("%s%s %-*s %-*s %-*s %s %s",
				prefix,
				stStyle.Render(icon),
				nameCol, truncate(run.WorkflowName, nameCol),
				branchCol, truncate(run.Branch, branchCol),
				actorCol, dimStyle.Render(truncate(run.Actor, actorCol-1)),
				dimStyle.Render(durPad),
				agoPad,
			)

			if i == r.cursor && r.focused {
				b.WriteString(theme.SelectedItemStyle.Render(line) + "\n")
			} else {
				b.WriteString(theme.NormalItemStyle.Render(line) + "\n")
			}
		}
	} else {
		// Full columns: name | status | duration | age
		statusCol = 12
		durCol := 8
		nameCol := avail - statusCol - durCol - agoCol - 3 // 3 for gaps
		if nameCol < 10 {
			nameCol = 10
		}

		for i := start; i < len(r.runs) && i < start+visibleRuns; i++ {
			run := r.runs[i]
			icon := theme.StatusIcon(run.Status, run.Conclusion)
			stStyle := theme.StatusStyle(run.Status, run.Conclusion)

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

			line1 := fmt.Sprintf("%s%s %-*s %-*s %*s %*s",
				prefix,
				stStyle.Render(icon),
				nameCol, truncate(run.WorkflowName, nameCol),
				statusCol, stStyle.Render(truncate(conclusion, statusCol)),
				durCol, dimStyle.Render(dur),
				agoCol, dimStyle.Render(ago),
			)

			displayTitle := run.DisplayTitle
			if displayTitle == "" {
				displayTitle = run.Name
			}
			branchCol := min(20, avail/4)
			eventCol := min(16, avail/5)
			titleCol := avail - branchCol - eventCol - 6
			if titleCol < 10 {
				titleCol = 10
			}
			line2 := fmt.Sprintf("    %-*s %-*s %s",
				branchCol, truncate(run.Branch, branchCol),
				eventCol, dimStyle.Render("["+truncate(run.Event, eventCol-2)+"]"),
				dimStyle.Render(truncate(displayTitle, titleCol)),
			)

			if i == r.cursor && r.focused {
				b.WriteString(theme.SelectedItemStyle.Render(line1) + "\n")
			} else {
				b.WriteString(theme.NormalItemStyle.Render(line1) + "\n")
			}
			b.WriteString(line2 + "\n\n")
		}
	}
	_ = statusCol

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
