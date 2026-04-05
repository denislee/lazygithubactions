package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
	"github.com/sahilm/fuzzy"
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
	Inputs       map[string]string
}

// TriggerWorkflowSelectedMsg is emitted when the user picks a workflow,
// so the app can load branches + inputs in parallel.
type TriggerWorkflowSelectedMsg struct {
	WorkflowPath string
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

// branchSource implements fuzzy.Source for branch name filtering.
type branchSource []string

func (b branchSource) String(i int) string { return b[i] }
func (b branchSource) Len() int            { return len(b) }

type TriggerDialog struct {
	workflows []models.Workflow
	cursor    int
	step      int // 0=workflow, 1=loading, 2=branch, 3=inputs
	width     int

	// Branch picker (step 2)
	branches      []string
	branchInput   textinput.Model
	branchMatches []fuzzy.Match
	branchCursor  int

	// Dispatch inputs (step 3)
	inputs        []models.WorkflowInput
	inputValues   []string          // parallel to inputs: current value per input
	inputModels   []textinput.Model // text inputs for string-type fields (nil for non-string)
	inputCursor   int               // which input field is focused
	choiceCursors []int             // for choice inputs: selected option index
}

func NewTriggerDialog(workflows []models.Workflow) TriggerDialog {
	// Sort deploy workflows to the top
	sorted := make([]models.Workflow, 0, len(workflows))
	var rest []models.Workflow
	for _, wf := range workflows {
		if strings.Contains(strings.ToLower(wf.Name), "deploy") {
			sorted = append(sorted, wf)
		} else {
			rest = append(rest, wf)
		}
	}
	sorted = append(sorted, rest...)

	return TriggerDialog{
		workflows: sorted,
		step:      0,
		width:     80,
	}
}

// SetBranchesAndInputs is called by the app after loading branches and workflow inputs.
// Returns a tea.Cmd to focus the branch search input.
func (d *TriggerDialog) SetBranchesAndInputs(branches []string, inputs []models.WorkflowInput, termWidth int) tea.Cmd {
	d.branches = branches
	d.inputs = inputs
	d.inputValues = make([]string, len(inputs))
	d.choiceCursors = make([]int, len(inputs))
	d.inputModels = make([]textinput.Model, len(inputs))
	for i, inp := range inputs {
		d.inputValues[i] = inp.Default
		switch inp.Type {
		case "boolean":
			if inp.Default == "" {
				d.inputValues[i] = "false"
			}
		case "choice":
			for j, opt := range inp.Options {
				if opt == inp.Default {
					d.choiceCursors[i] = j
					break
				}
			}
			// If default is empty but options exist, select first
			if inp.Default == "" && len(inp.Options) > 0 {
				d.inputValues[i] = inp.Options[0]
			}
		default: // string
			ti := textinput.New()
			ti.Placeholder = inp.Default
			ti.SetWidth(30)
			if inp.Default != "" {
				ti.SetValue(inp.Default)
			}
			d.inputModels[i] = ti
		}
	}
	// Size dialog to fit the longest branch name (prefix "  " + name + dialog padding/border overhead of 6)
	maxWidth := termWidth - 4 // leave some breathing room
	if maxWidth < 80 {
		maxWidth = 80
	}
	needed := 80
	for _, br := range branches {
		if w := len(br) + 8; w > needed { // 2 prefix + 6 padding/border
			needed = w
		}
	}
	if needed > maxWidth {
		needed = maxWidth
	}
	d.width = needed

	d.step = 2
	d.branchInput = textinput.New()
	d.branchInput.Placeholder = "Search branches..."
	d.branchInput.SetWidth(needed - 16) // account for "Branch: " label + dialog chrome
	d.updateBranchMatches()
	return d.branchInput.Focus()
}

func (d *TriggerDialog) updateBranchMatches() {
	q := d.branchInput.Value()
	if q == "" {
		d.branchMatches = make([]fuzzy.Match, len(d.branches))
		for i, name := range d.branches {
			d.branchMatches[i] = fuzzy.Match{Index: i, Str: name}
		}
	} else {
		d.branchMatches = fuzzy.FindFrom(q, branchSource(d.branches))
	}
	d.branchCursor = 0
}

func (d TriggerDialog) selectedWorkflow() models.Workflow {
	return d.workflows[d.cursor]
}

func (d TriggerDialog) selectedBranch() string {
	if len(d.branchMatches) == 0 {
		return "main"
	}
	return d.branchMatches[d.branchCursor].Str
}

func (d TriggerDialog) buildInputs() map[string]string {
	if len(d.inputs) == 0 {
		return nil
	}
	m := make(map[string]string, len(d.inputs))
	for i, inp := range d.inputs {
		var v string
		if inp.Type == "string" || inp.Type == "" {
			v = d.inputModels[i].Value()
		} else {
			v = d.inputValues[i]
		}
		if v != "" || inp.Required {
			m[inp.Name] = v
		}
	}
	return m
}

func (d TriggerDialog) emitResult() tea.Cmd {
	wf := d.selectedWorkflow()
	branch := d.selectedBranch()
	inputs := d.buildInputs()
	return func() tea.Msg {
		return TriggerDialogResultMsg{
			WorkflowFile: wf.Path,
			Branch:       branch,
			Inputs:       inputs,
		}
	}
}

func (d TriggerDialog) Update(msg tea.Msg) (TriggerDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		isEsc := msg.String() == "esc"

		// Step 0: esc or h cancels the dialog
		if d.step == 0 && (isEsc || msg.String() == "h") {
			return d, func() tea.Msg { return TriggerDialogResultMsg{Cancelled: true} }
		}

		// Step 2 (branch) or 3 (inputs): esc goes back to previous step
		if isEsc && d.step == 2 {
			d.step = 0
			d.branchInput.Blur()
			return d, nil
		}
		if isEsc && d.step == 3 {
			d.step = 2
			d.blurCurrentInput()
			return d, d.branchInput.Focus()
		}

		switch d.step {
		case 0: // Pick workflow
			return d.updateWorkflowStep(msg)
		case 2: // Pick branch
			return d.updateBranchStep(msg)
		case 3: // Fill inputs
			return d.updateInputsStep(msg)
		}
	}
	return d, nil
}

func (d TriggerDialog) updateWorkflowStep(msg tea.KeyPressMsg) (TriggerDialog, tea.Cmd) {
	pageSize := 10
	switch {
	case key.Matches(msg, theme.Keys.Up) || key.Matches(msg, theme.PrevPage):
		if d.cursor > 0 {
			d.cursor--
		} else {
			d.cursor = len(d.workflows) - 1
		}
	case key.Matches(msg, theme.Keys.Down) || key.Matches(msg, theme.NextPage):
		if d.cursor < len(d.workflows)-1 {
			d.cursor++
		} else {
			d.cursor = 0
		}
	case key.Matches(msg, theme.PageDown):
		d.cursor += pageSize
		if d.cursor >= len(d.workflows) {
			d.cursor = len(d.workflows) - 1
		}
	case key.Matches(msg, theme.PageUp):
		d.cursor -= pageSize
		if d.cursor < 0 {
			d.cursor = 0
		}
	case msg.String() == "enter" || msg.String() == "l":
		d.step = 1 // loading
		wf := d.selectedWorkflow()
		return d, func() tea.Msg {
			return TriggerWorkflowSelectedMsg{WorkflowPath: wf.Path}
		}
	}
	return d, nil
}

// arrowUp/arrowDown use only arrow keys, so j/k go to the text input instead.
var (
	arrowUp   = key.NewBinding(key.WithKeys("up"))
	arrowDown = key.NewBinding(key.WithKeys("down"))
)

func (d TriggerDialog) updateBranchStep(msg tea.KeyPressMsg) (TriggerDialog, tea.Cmd) {
	pageSize := 10
	switch {
	case key.Matches(msg, arrowUp) || key.Matches(msg, theme.PrevPage):
		if d.branchCursor > 0 {
			d.branchCursor--
		} else if len(d.branchMatches) > 0 {
			d.branchCursor = len(d.branchMatches) - 1
		}
	case key.Matches(msg, arrowDown) || key.Matches(msg, theme.NextPage):
		if d.branchCursor < len(d.branchMatches)-1 {
			d.branchCursor++
		} else {
			d.branchCursor = 0
		}
	case key.Matches(msg, theme.PageDown):
		d.branchCursor += pageSize
		if d.branchCursor >= len(d.branchMatches) {
			d.branchCursor = len(d.branchMatches) - 1
		}
		if d.branchCursor < 0 {
			d.branchCursor = 0
		}
	case key.Matches(msg, theme.PageUp):
		d.branchCursor -= pageSize
		if d.branchCursor < 0 {
			d.branchCursor = 0
		}
	case msg.String() == "enter":
		if len(d.inputs) > 0 {
			d.step = 3
			d.inputCursor = 0
			return d, d.focusCurrentInput()
		}
		return d, d.emitResult()
	default:
		var cmd tea.Cmd
		d.branchInput, cmd = d.branchInput.Update(msg)
		d.updateBranchMatches()
		return d, cmd
	}
	return d, nil
}

func (d TriggerDialog) focusCurrentInput() tea.Cmd {
	inp := d.inputs[d.inputCursor]
	if inp.Type == "string" || inp.Type == "" {
		return d.inputModels[d.inputCursor].Focus()
	}
	return nil
}

func (d TriggerDialog) blurCurrentInput() {
	inp := d.inputs[d.inputCursor]
	if inp.Type == "string" || inp.Type == "" {
		d.inputModels[d.inputCursor].Blur()
	}
}

func (d TriggerDialog) updateInputsStep(msg tea.KeyPressMsg) (TriggerDialog, tea.Cmd) {
	inp := d.inputs[d.inputCursor]
	isStringInput := inp.Type == "string" || inp.Type == ""

	// Navigation between fields: tab/shift+tab always work;
	// j/k work on non-string inputs (string inputs need them for typing).
	moveNext := msg.String() == "tab" || key.Matches(msg, theme.NextPage) ||
		(!isStringInput && key.Matches(msg, theme.Keys.Down))
	movePrev := msg.String() == "shift+tab" || key.Matches(msg, theme.PrevPage) ||
		(!isStringInput && key.Matches(msg, theme.Keys.Up))

	switch {
	case moveNext:
		d.blurCurrentInput()
		if d.inputCursor < len(d.inputs)-1 {
			d.inputCursor++
		} else {
			d.inputCursor = 0
		}
		return d, d.focusCurrentInput()
	case movePrev:
		d.blurCurrentInput()
		if d.inputCursor > 0 {
			d.inputCursor--
		} else {
			d.inputCursor = len(d.inputs) - 1
		}
		return d, d.focusCurrentInput()
	case msg.String() == "enter":
		if d.inputCursor == len(d.inputs)-1 {
			return d, d.emitResult()
		}
		d.blurCurrentInput()
		d.inputCursor++
		return d, d.focusCurrentInput()
	default:
		switch inp.Type {
		case "boolean":
			switch msg.String() {
			case " ", "x":
				if d.inputValues[d.inputCursor] == "true" {
					d.inputValues[d.inputCursor] = "false"
				} else {
					d.inputValues[d.inputCursor] = "true"
				}
			case "h", "left":
				d.inputValues[d.inputCursor] = "true"
			case "l", "right":
				d.inputValues[d.inputCursor] = "false"
			}
		case "choice":
			if msg.String() == "h" || msg.String() == "left" {
				if d.choiceCursors[d.inputCursor] > 0 {
					d.choiceCursors[d.inputCursor]--
					d.inputValues[d.inputCursor] = inp.Options[d.choiceCursors[d.inputCursor]]
				}
			} else if msg.String() == "l" || msg.String() == "right" {
				if len(inp.Options) > 0 && d.choiceCursors[d.inputCursor] < len(inp.Options)-1 {
					d.choiceCursors[d.inputCursor]++
					d.inputValues[d.inputCursor] = inp.Options[d.choiceCursors[d.inputCursor]]
				}
			}
		default: // string — delegate to textinput.Model
			var cmd tea.Cmd
			d.inputModels[d.inputCursor], cmd = d.inputModels[d.inputCursor].Update(msg)
			return d, cmd
		}
	}
	return d, nil
}

func (d TriggerDialog) View() string {
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("Trigger Workflow") + "\n\n")

	switch d.step {
	case 0: // Pick workflow
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

	case 1: // Loading
		wf := d.selectedWorkflow()
		b.WriteString(fmt.Sprintf("Workflow: %s\n\n", wf.Name))
		b.WriteString("Loading branches...")

	case 2: // Pick branch
		wf := d.selectedWorkflow()
		b.WriteString(fmt.Sprintf("Workflow: %s\n\n", wf.Name))
		b.WriteString("Branch: ")
		b.WriteString(d.branchInput.View())
		b.WriteString("\n\n")

		maxVisible := 10
		start := 0
		if d.branchCursor >= maxVisible {
			start = d.branchCursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(d.branchMatches) {
			end = len(d.branchMatches)
		}

		for i := start; i < end; i++ {
			m := d.branchMatches[i]
			prefix := "  "
			if i == d.branchCursor {
				prefix = "> "
				b.WriteString(theme.SelectedItemStyle.Render(prefix+m.Str) + "\n")
			} else {
				b.WriteString(theme.NormalItemStyle.Render(prefix+m.Str) + "\n")
			}
		}

		if len(d.branchMatches) == 0 {
			b.WriteString(theme.NormalItemStyle.Render("  No matching branches") + "\n")
		}

		b.WriteString("\nPress enter to select, esc to cancel")

	case 3: // Fill inputs
		wf := d.selectedWorkflow()
		b.WriteString(fmt.Sprintf("Workflow: %s  Branch: %s\n\n", wf.Name, d.selectedBranch()))
		b.WriteString("Inputs:\n\n")

		for i, inp := range d.inputs {
			isCursor := i == d.inputCursor
			prefix := "  "
			if isCursor {
				prefix = "> "
			}

			label := inp.Name
			if inp.Required {
				label += " *"
			}

			var valueStr string
			switch inp.Type {
			case "boolean":
				if d.inputValues[i] == "true" {
					valueStr = "[true]  false"
				} else {
					valueStr = " true  [false]"
				}
			case "choice":
				var opts []string
				for j, opt := range inp.Options {
					if j == d.choiceCursors[i] {
						opts = append(opts, "["+opt+"]")
					} else {
						opts = append(opts, " "+opt+" ")
					}
				}
				valueStr = strings.Join(opts, " ")
			default: // string — use textinput view
				valueStr = d.inputModels[i].View()
			}

			line := fmt.Sprintf("%s%s: %s", prefix, label, valueStr)
			if isCursor {
				b.WriteString(theme.SelectedItemStyle.Render(line) + "\n")
			} else {
				b.WriteString(theme.NormalItemStyle.Render(line) + "\n")
			}

			if isCursor && inp.Description != "" {
				b.WriteString("    " + theme.NormalItemStyle.Render(inp.Description) + "\n")
			}
		}

		b.WriteString("\n[j/k] navigate  [enter] trigger  [esc] back")
	}

	return theme.DialogStyle.Width(d.width).Render(b.String())
}
