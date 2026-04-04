package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

// Result messages
type ConfirmDialogResultMsg struct {
	Confirmed bool
	Action    string
	RunID     int64
}

type TriggerDialogResultMsg struct {
	Cancelled    bool
	WorkflowFile string
	Branch       string
}

// --- ConfirmDialog ---

type ConfirmDialog struct {
	Title   string
	Message string
	Action  string
	RunID   int64
	width   int
}

func NewConfirmDialog(title, message, action string, runID int64) ConfirmDialog {
	return ConfirmDialog{
		Title:   title,
		Message: message,
		Action:  action,
		RunID:   runID,
		width:   50,
	}
}

func (d ConfirmDialog) Update(msg tea.Msg) (ConfirmDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			return d, func() tea.Msg {
				return ConfirmDialogResultMsg{Confirmed: true, Action: d.Action, RunID: d.RunID}
			}
		case "n", "N", "esc":
			return d, func() tea.Msg {
				return ConfirmDialogResultMsg{Confirmed: false, Action: d.Action, RunID: d.RunID}
			}
		}
	}
	return d, nil
}

func (d ConfirmDialog) View() string {
	content := fmt.Sprintf("%s\n\n%s\n\n[y]es  [n]o", d.Title, d.Message)
	return theme.DialogStyle.Width(d.width).Render(content)
}

// --- TriggerDialog ---

type TriggerDialog struct {
	workflows   []models.Workflow
	cursor      int
	branchInput textinput.Model
	step        int // 0 = pick workflow, 1 = type branch
	width       int
}

func NewTriggerDialog(workflows []models.Workflow) TriggerDialog {
	ti := textinput.New()
	ti.Placeholder = "main"
	ti.SetWidth(30)
	return TriggerDialog{
		workflows:   workflows,
		branchInput: ti,
		step:        0,
		width:       50,
	}
}

func (d TriggerDialog) Update(msg tea.Msg) (TriggerDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return d, func() tea.Msg { return TriggerDialogResultMsg{Cancelled: true} }
		}

		if d.step == 0 {
			switch {
			case key.Matches(msg, theme.Keys.Up):
				if d.cursor > 0 {
					d.cursor--
				}
			case key.Matches(msg, theme.Keys.Down):
				if d.cursor < len(d.workflows)-1 {
					d.cursor++
				}
			case msg.String() == "enter":
				d.step = 1
				return d, d.branchInput.Focus()
			}
		} else {
			if msg.String() == "enter" {
				branch := d.branchInput.Value()
				if branch == "" {
					branch = "main"
				}
				wf := d.workflows[d.cursor]
				return d, func() tea.Msg {
					return TriggerDialogResultMsg{
						WorkflowFile: wf.Path,
						Branch:       branch,
					}
				}
			}
			var cmd tea.Cmd
			d.branchInput, cmd = d.branchInput.Update(msg)
			return d, cmd
		}
	}
	return d, nil
}

func (d TriggerDialog) View() string {
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("Trigger Workflow") + "\n\n")

	if d.step == 0 {
		b.WriteString("Select workflow:\n\n")
		for i, wf := range d.workflows {
			prefix := "  "
			if i == d.cursor {
				prefix = "> "
				b.WriteString(theme.SelectedItemStyle.Render(prefix+wf.Name) + "\n")
			} else {
				b.WriteString(theme.NormalItemStyle.Render(prefix+wf.Name) + "\n")
			}
		}
	} else {
		wf := d.workflows[d.cursor]
		b.WriteString(fmt.Sprintf("Workflow: %s\n\n", wf.Name))
		b.WriteString("Branch: ")
		b.WriteString(d.branchInput.View())
		b.WriteString("\n\nPress enter to trigger, esc to cancel")
	}

	return theme.DialogStyle.Width(d.width).Render(b.String())
}
