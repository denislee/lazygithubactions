package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
	"github.com/sahilm/fuzzy"
)

type QuickSwitchResultMsg struct {
	Cancelled bool
	Repo      *models.Repo
}

type QuickSwitch struct {
	input   textinput.Model
	repos   []models.Repo
	matches []fuzzy.Match
	cursor  int
	width   int
}

// repoNames implements fuzzy.Source
type repoNames []models.Repo

func (r repoNames) String(i int) string { return r[i].FullName }
func (r repoNames) Len() int            { return len(r) }

func NewQuickSwitch(repos []models.Repo) QuickSwitch {
	ti := textinput.New()
	ti.Placeholder = "Search repositories..."
	ti.SetWidth(50)

	qs := QuickSwitch{
		input: ti,
		repos: repos,
		width: 60,
	}
	qs.updateMatches()
	return qs
}

func (qs *QuickSwitch) Init() tea.Cmd {
	return qs.input.Focus()
}

func (qs *QuickSwitch) updateMatches() {
	query := qs.input.Value()
	if query == "" {
		qs.matches = make([]fuzzy.Match, len(qs.repos))
		for i := range qs.repos {
			qs.matches[i] = fuzzy.Match{Index: i, Str: qs.repos[i].FullName}
		}
	} else {
		qs.matches = fuzzy.FindFrom(query, repoNames(qs.repos))
	}
	qs.cursor = 0
}

func (qs QuickSwitch) Update(msg tea.Msg) (QuickSwitch, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return qs, func() tea.Msg { return QuickSwitchResultMsg{Cancelled: true} }
		case "enter":
			if len(qs.matches) > 0 && qs.cursor < len(qs.matches) {
				repo := qs.repos[qs.matches[qs.cursor].Index]
				return qs, func() tea.Msg { return QuickSwitchResultMsg{Repo: &repo} }
			}
			return qs, nil
		case "up":
			if qs.cursor > 0 {
				qs.cursor--
			}
			return qs, nil
		case "down":
			if qs.cursor < len(qs.matches)-1 {
				qs.cursor++
			}
			return qs, nil
		}
	}

	prevValue := qs.input.Value()
	var cmd tea.Cmd
	qs.input, cmd = qs.input.Update(msg)
	if qs.input.Value() != prevValue {
		qs.updateMatches()
	}
	return qs, cmd
}

func (qs QuickSwitch) View() string {
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("Quick Switch (Ctrl+K)") + "\n\n")
	b.WriteString(qs.input.View() + "\n\n")

	maxVisible := 10
	for i, m := range qs.matches {
		if i >= maxVisible {
			remaining := len(qs.matches) - maxVisible
			b.WriteString(fmt.Sprintf("  ... and %d more\n", remaining))
			break
		}
		prefix := "  "
		if i == qs.cursor {
			b.WriteString(theme.SelectedItemStyle.Render("> "+m.Str) + "\n")
		} else {
			b.WriteString(theme.NormalItemStyle.Render(prefix+m.Str) + "\n")
		}
	}

	if len(qs.matches) == 0 {
		b.WriteString(theme.NormalItemStyle.Render("  No matches"))
	}

	return theme.OverlayStyle.Width(qs.width).Render(b.String())
}
