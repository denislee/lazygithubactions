package tui

import (
	"context"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/gh"
	"github.com/dns/lazygithubactions/internal/tui/components"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

type panel int

const (
	repoPanel panel = iota
	runPanel
)

type view int

const (
	MainView view = iota
	DetailView
	LogView
	QuickSwitchView
	TriggerView
	ConfirmView
)

type App struct {
	client   *gh.Client
	repoList components.RepoList
	runList  components.RunList

	activePanel panel
	ActiveView  view
	width       int
	height      int

	lastRepo string
	loading  bool
	err      error
	message  string
}

func NewApp() App {
	return App{
		client:      gh.NewClient(),
		repoList:    components.NewRepoList(),
		runList:     components.NewRunList(),
		activePanel: repoPanel,
		ActiveView:  MainView,
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.loadRepos(),
		a.tick(),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyPressMsg:
		// Global keys
		switch {
		case key.Matches(msg, theme.Keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, theme.Keys.QuickSwitch):
			// TODO: open quick switcher (Task 12)
			return a, nil
		}

		// Main view keys
		if a.ActiveView == MainView {
			switch {
			case key.Matches(msg, theme.Keys.Tab):
				a.togglePanel()
				return a, nil
			case key.Matches(msg, theme.Keys.Refresh):
				return a, a.refreshCurrent()
			case key.Matches(msg, theme.Keys.Enter):
				if a.activePanel == repoPanel {
					return a, a.selectRepo()
				}
				// TODO: drill into run detail (Task 9)
			case key.Matches(msg, theme.Keys.Trigger):
				// TODO: trigger workflow (Task 10)
			case key.Matches(msg, theme.Keys.Cancel):
				if run := a.runList.SelectedRun(); run != nil {
					return a, a.cancelRun(run.ID)
				}
			case key.Matches(msg, theme.Keys.Rerun):
				if run := a.runList.SelectedRun(); run != nil {
					return a, a.rerunFailed(run.ID)
				}
			case key.Matches(msg, theme.Keys.Logs):
				// TODO: view logs (Task 11)
			case key.Matches(msg, theme.Keys.Download):
				if run := a.runList.SelectedRun(); run != nil {
					return a, a.downloadArtifacts(run.ID)
				}
			}

			// Forward to active panel
			if a.activePanel == repoPanel {
				cmd := a.repoList.Update(msg)
				cmds = append(cmds, cmd)
				if repo := a.repoList.SelectedRepo(); repo != nil && repo.FullName != a.lastRepo {
					cmds = append(cmds, a.loadRuns(repo.FullName))
				}
			} else {
				cmd := a.runList.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case ReposLoadedMsg:
		a.loading = false
		if msg.Err != nil {
			a.err = msg.Err
		} else {
			a.repoList.SetRepos(msg.Repos)
			if repo := a.repoList.SelectedRepo(); repo != nil {
				cmds = append(cmds, a.loadRuns(repo.FullName))
			}
		}

	case RunsLoadedMsg:
		if msg.Err != nil {
			a.err = msg.Err
		} else {
			a.runList.SetRuns(msg.Runs, msg.Repo)
			a.lastRepo = msg.Repo
		}

	case ActionResultMsg:
		a.message = msg.Message
		if msg.Success {
			cmds = append(cmds, a.refreshCurrent())
		}

	case TickMsg:
		if a.lastRepo != "" && a.ActiveView == MainView {
			cmds = append(cmds, a.loadRuns(a.lastRepo))
		}
		cmds = append(cmds, a.tick())
	}

	return a, tea.Batch(cmds...)
}

func (a App) View() tea.View {
	if a.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	repoWidth := a.width / 4
	if repoWidth < 20 {
		repoWidth = 20
	}
	runWidth := a.width - repoWidth - 2
	panelHeight := a.height - 3

	a.repoList.SetSize(repoWidth, panelHeight)
	a.repoList.SetFocused(a.activePanel == repoPanel)
	a.runList.SetSize(runWidth, panelHeight)
	a.runList.SetFocused(a.activePanel == runPanel)

	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		a.repoList.View(),
		a.runList.View(),
	)

	help := theme.HelpBarStyle.Width(a.width).Render(
		"tab:switch  j/k:nav  enter:select  T:trigger  C:cancel  R:rerun  L:logs  D:download  r:refresh  ctrl+k:search  q:quit",
	)

	status := ""
	if a.err != nil {
		status = theme.StatusBarStyle.Foreground(lipgloss.Color("#FF4444")).Width(a.width).Render("Error: " + a.err.Error())
	} else if a.message != "" {
		status = theme.StatusBarStyle.Width(a.width).Render(a.message)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, panels, status, help)

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// --- Commands ---

func (a *App) loadRepos() tea.Cmd {
	return func() tea.Msg {
		if repos, err := gh.LoadCachedRepos(); err == nil && repos != nil {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if fresh, err := a.client.ListRepos(ctx); err == nil {
					_ = gh.SaveCachedRepos(fresh)
				}
			}()
			return ReposLoadedMsg{Repos: repos}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repos, err := a.client.ListRepos(ctx)
		if err == nil {
			_ = gh.SaveCachedRepos(repos)
		}
		return ReposLoadedMsg{Repos: repos, Err: err}
	}
}

func (a *App) loadRuns(repo string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		runs, err := a.client.ListRuns(ctx, repo)
		return RunsLoadedMsg{Repo: repo, Runs: runs, Err: err}
	}
}

func (a *App) cancelRun(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := a.client.CancelRun(ctx, repo, runID)
		if err != nil {
			return ActionResultMsg{Action: "cancel", Success: false, Message: "Cancel failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "cancel", Success: true, Message: "Run cancelled"}
	}
}

func (a *App) rerunFailed(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := a.client.RerunFailed(ctx, repo, runID)
		if err != nil {
			return ActionResultMsg{Action: "rerun", Success: false, Message: "Rerun failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "rerun", Success: true, Message: "Re-running failed jobs"}
	}
}

func (a *App) downloadArtifacts(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		err := a.client.DownloadArtifacts(ctx, repo, runID, "")
		if err != nil {
			return ActionResultMsg{Action: "download", Success: false, Message: "Download failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "download", Success: true, Message: "Artifacts downloaded"}
	}
}

func (a *App) refreshCurrent() tea.Cmd {
	if a.lastRepo != "" {
		return a.loadRuns(a.lastRepo)
	}
	return a.loadRepos()
}

func (a *App) selectRepo() tea.Cmd {
	repo := a.repoList.SelectedRepo()
	if repo == nil {
		return nil
	}
	a.activePanel = runPanel
	return a.loadRuns(repo.FullName)
}

func (a *App) togglePanel() {
	if a.activePanel == repoPanel {
		a.activePanel = runPanel
	} else {
		a.activePanel = repoPanel
	}
}

func (a *App) tick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}
