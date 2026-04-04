package components

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

// OrgChangedMsg is sent when the user selects a different org.
type OrgChangedMsg struct {
	Org string // "" means All
}

type OrgSelector struct {
	orgs    []string // first entry is always "All"
	cursor  int
	width   int
	height  int
	focused bool
}

func NewOrgSelector() OrgSelector {
	return OrgSelector{
		orgs: []string{"All"},
	}
}

func (o *OrgSelector) SetOrgs(repos []models.Repo) {
	seen := map[string]bool{}
	for _, r := range repos {
		if r.Owner != "" {
			seen[r.Owner] = true
		}
	}
	orgs := make([]string, 0, len(seen))
	for org := range seen {
		orgs = append(orgs, org)
	}
	sort.Strings(orgs)
	o.orgs = append([]string{"All"}, orgs...)
}

func (o *OrgSelector) SetSize(w, h int) {
	o.width = w
	o.height = h
}

func (o *OrgSelector) SetFocused(f bool) {
	o.focused = f
}

func (o *OrgSelector) SelectedOrg() string {
	if o.cursor == 0 || o.cursor >= len(o.orgs) {
		return "" // "All"
	}
	return o.orgs[o.cursor]
}

// SelectOrg sets the cursor to the given org name. Used for loading saved config.
func (o *OrgSelector) OrgCount() int {
	return len(o.orgs)
}

func (o *OrgSelector) SelectOrg(org string) {
	if org == "" {
		o.cursor = 0
		return
	}
	for i, name := range o.orgs {
		if name == org {
			o.cursor = i
			return
		}
	}
	o.cursor = 0
}

func (o *OrgSelector) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if o.cursor > 0 {
				o.cursor--
				return o.emitChange()
			}
		case key.Matches(msg, theme.Keys.Down):
			if o.cursor < len(o.orgs)-1 {
				o.cursor++
				return o.emitChange()
			}
		}
	}
	return nil
}

func (o *OrgSelector) emitChange() tea.Cmd {
	org := o.SelectedOrg()
	return func() tea.Msg { return OrgChangedMsg{Org: org} }
}

func (o *OrgSelector) View() string {
	var b strings.Builder
	title := theme.TitleStyle.Render("Organization")
	b.WriteString(title + "\n")

	visibleHeight := o.height - 3
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	start := 0
	if o.cursor >= visibleHeight {
		start = o.cursor - visibleHeight + 1
	}

	for i := start; i < len(o.orgs) && i < start+visibleHeight; i++ {
		name := o.orgs[i]
		if i == o.cursor && o.focused {
			b.WriteString(theme.SelectedItemStyle.Render("> "+name) + "\n")
		} else if i == o.cursor {
			b.WriteString(theme.NormalItemStyle.Render("> "+name) + "\n")
		} else {
			b.WriteString(theme.NormalItemStyle.Render("  "+name) + "\n")
		}
	}

	style := theme.PanelStyle
	if o.focused {
		style = theme.ActivePanelStyle
	}
	return style.Width(o.width).Height(o.height).Render(b.String())
}
