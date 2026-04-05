package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

// RepoStatus holds the status/conclusion of a repo's most recent run.
type RepoStatus struct {
	Status     string
	Conclusion string
}

type RepoList struct {
	repos        []models.Repo
	cursor       int
	scrollOffset int
	width        int
	height       int
	focused      bool
	filter       string
	filtered     []models.Repo
	repoStatus   map[string]RepoStatus // fullName -> last run status
}

func NewRepoList() RepoList {
	return RepoList{
		repoStatus: make(map[string]RepoStatus),
	}
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

func (r *RepoList) SetRepoStatus(fullName, status, conclusion string) {
	r.repoStatus[fullName] = RepoStatus{Status: status, Conclusion: conclusion}
}

func (r *RepoList) SelectByName(fullName string) {
	for i, repo := range r.filtered {
		if repo.FullName == fullName {
			r.cursor = i
			return
		}
	}
}

func (r *RepoList) SetFilter(f string) {
	r.filter = f
	r.applyFilter()
	r.cursor = 0
	r.scrollOffset = 0
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
		pageSize := r.height - 2
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
		case key.Matches(msg, theme.HalfDown):
			r.cursor += pageSize / 2
			if r.cursor >= len(r.filtered) {
				r.cursor = len(r.filtered) - 1
			}
		case key.Matches(msg, theme.HalfUp):
			r.cursor -= pageSize / 2
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

	// Adjust scroll offset to keep cursor visible
	if r.cursor < r.scrollOffset {
		r.scrollOffset = r.cursor
	} else if r.cursor >= r.scrollOffset+visibleHeight {
		r.scrollOffset = r.cursor - visibleHeight + 1
	}

	for i := r.scrollOffset; i < len(r.filtered) && i < r.scrollOffset+visibleHeight; i++ {
		repo := r.filtered[i]
		prefix := "  "
		if i == r.cursor {
			prefix = "> "
		}

		// Status icon from last known run
		icon := ""
		if st, ok := r.repoStatus[repo.FullName]; ok {
			ico := theme.StatusIcon(st.Status, st.Conclusion)
			sty := theme.StatusStyle(st.Status, st.Conclusion)
			icon = sty.Render(ico) + " "
		}

		line := fmt.Sprintf("%s%s%s", prefix, icon, repo.Name)
		if i == r.cursor && r.focused {
			line = theme.SelectedItemStyle.Render(prefix) + icon + theme.SelectedItemStyle.Render(repo.Name)
		} else if i == r.cursor {
			line = theme.NormalItemStyle.Render(prefix) + icon + theme.NormalItemStyle.Render(repo.Name)
		} else {
			line = theme.NormalItemStyle.Render(prefix) + icon + theme.NormalItemStyle.Render(repo.Name)
		}
		b.WriteString(line + "\n")
	}

	if len(r.filtered) == 0 {
		b.WriteString(theme.NormalItemStyle.Render("  No repositories found"))
	}

	return b.String()
}
