package components

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

type RepoList struct {
	repos    []models.Repo
	cursor   int
	width    int
	height   int
	focused  bool
	filter   string
	filtered []models.Repo
}

func NewRepoList() RepoList {
	return RepoList{}
}

func (r *RepoList) SetRepos(repos []models.Repo) {
	r.repos = repos
	r.applyFilter()
}

func (r *RepoList) SetSize(w, h int) {
	r.width = w
	r.height = h
}

func (r *RepoList) SetFocused(f bool) {
	r.focused = f
}

func (r *RepoList) SelectedRepo() *models.Repo {
	if len(r.filtered) == 0 {
		return nil
	}
	if r.cursor >= len(r.filtered) {
		r.cursor = len(r.filtered) - 1
	}
	return &r.filtered[r.cursor]
}

func (r *RepoList) AtTop() bool {
	return r.cursor == 0
}

func (r *RepoList) SetFilter(f string) {
	r.filter = f
	r.applyFilter()
	r.cursor = 0
}

func (r *RepoList) applyFilter() {
	if r.filter == "" {
		r.filtered = r.repos
		return
	}
	lower := strings.ToLower(r.filter)
	r.filtered = nil
	for _, repo := range r.repos {
		if strings.Contains(strings.ToLower(repo.FullName), lower) {
			r.filtered = append(r.filtered, repo)
		}
	}
}

func (r *RepoList) Update(msg tea.Msg) tea.Cmd {
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
			if r.cursor < len(r.filtered)-1 {
				r.cursor++
			}
		case key.Matches(msg, theme.PageDown, theme.NextPage):
			r.cursor += pageSize
			if r.cursor >= len(r.filtered) {
				r.cursor = len(r.filtered) - 1
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

// ViewContent returns the repo list content without borders (for embedding in a combined panel).
func (r *RepoList) ViewContent() string {
	var b strings.Builder
	title := theme.TitleStyle.Render("Repositories")
	b.WriteString(title + "\n")

	visibleHeight := r.height
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	start := 0
	if r.cursor >= visibleHeight {
		start = r.cursor - visibleHeight + 1
	}

	for i := start; i < len(r.filtered) && i < start+visibleHeight; i++ {
		repo := r.filtered[i]
		line := repo.Name
		if i == r.cursor && r.focused {
			line = theme.SelectedItemStyle.Render("> " + line)
		} else if i == r.cursor {
			line = theme.NormalItemStyle.Render("> " + line)
		} else {
			line = theme.NormalItemStyle.Render("  " + line)
		}
		b.WriteString(line + "\n")
	}

	if len(r.filtered) == 0 {
		b.WriteString(theme.NormalItemStyle.Render("  No repositories found"))
	}

	return b.String()
}
