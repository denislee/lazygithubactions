package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/gh"
	"github.com/dns/lazygithubactions/internal/models"
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

	// Sub-views
	runDetail   components.RunDetail
	logViewer   components.LogViewer
	quickSwitch *components.QuickSwitch
	triggerDlg  *components.TriggerDialog
	confirmDlg  *components.ConfirmDialog
	statusBar   components.StatusBar

	activePanel panel
	ActiveView  view
	width       int
	height      int

	lastRepo string
	repos    []models.Repo // cached for QuickSwitch
	loading  bool
	err      error
	message  string
}

func NewApp() App {
	return App{
		client:      gh.NewClient(),
		repoList:    components.NewRepoList(),
		runList:     components.NewRunList(),
		runDetail:   components.NewRunDetail(),
		logViewer:   components.NewLogViewer(),
		statusBar:   components.NewStatusBar(),
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
		a.statusBar.SetWidth(a.width)
		return a, nil

	case tea.KeyPressMsg:
		// Ctrl+C always quits, no matter what view
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

		// Normalize ctrl+[ to esc globally
		if msg.String() == "ctrl+[" {
			msg = tea.KeyPressMsg{Code: 27} // ESC
		}

		// q quits only outside overlays
		if msg.String() == "q" && a.ActiveView != QuickSwitchView &&
			a.ActiveView != TriggerView && a.ActiveView != ConfirmView {
			return a, tea.Quit
		}

		switch a.ActiveView {
		case MainView:
			return a.updateMainView(msg)

		case DetailView:
			return a.updateDetailView(msg)

		case LogView:
			return a.updateLogView(msg)

		case QuickSwitchView:
			return a.updateQuickSwitchView(msg)

		case TriggerView:
			return a.updateTriggerView(msg)

		case ConfirmView:
			return a.updateConfirmView(msg)
		}

	case ReposLoadedMsg:
		a.loading = false
		if msg.Err != nil {
			a.err = msg.Err
			a.statusBar.SetMessage("Error: "+msg.Err.Error(), true)
		} else {
			a.repos = msg.Repos
			a.repoList.SetRepos(msg.Repos)
			a.statusBar.Clear()
			if repo := a.repoList.SelectedRepo(); repo != nil {
				cmds = append(cmds, a.loadRuns(repo.FullName))
			}
		}

	case RunsLoadedMsg:
		if msg.Err != nil {
			a.err = msg.Err
			a.statusBar.SetMessage("Error: "+msg.Err.Error(), true)
		} else {
			a.runList.SetRuns(msg.Runs, msg.Repo)
			a.lastRepo = msg.Repo
		}

	case RunDetailLoadedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Error loading detail: "+msg.Err.Error(), true)
			a.ActiveView = MainView
		} else {
			a.runDetail.SetDetail(msg.Detail)
			a.ActiveView = DetailView
		}

	case LogLoadedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Error loading logs: "+msg.Err.Error(), true)
			a.ActiveView = MainView
		} else {
			title := a.lastRepo
			if run := a.runList.SelectedRun(); run != nil {
				title = fmt.Sprintf("%s #%d", run.WorkflowName, run.ID)
			}
			a.logViewer.SetContent(title, msg.Log)
			a.ActiveView = LogView
		}

	case WorkflowsLoadedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Error loading workflows: "+msg.Err.Error(), true)
		} else {
			td := components.NewTriggerDialog(msg.Workflows)
			a.triggerDlg = &td
			a.ActiveView = TriggerView
		}

	case components.QuickSwitchResultMsg:
		a.ActiveView = MainView
		a.quickSwitch = nil
		if !msg.Cancelled && msg.Repo != nil {
			a.lastRepo = msg.Repo.FullName
			a.activePanel = runPanel
			cmds = append(cmds, a.loadRuns(msg.Repo.FullName))
		}

	case components.TriggerDialogResultMsg:
		a.ActiveView = MainView
		a.triggerDlg = nil
		if !msg.Cancelled {
			cmds = append(cmds, a.triggerWorkflow(msg.WorkflowFile, msg.Branch))
		}

	case components.ConfirmDialogResultMsg:
		a.ActiveView = MainView
		a.confirmDlg = nil
		if msg.Confirmed {
			switch msg.Action {
			case "cancel":
				cmds = append(cmds, a.cancelRun(msg.RunID))
			case "rerun":
				cmds = append(cmds, a.rerunFailed(msg.RunID))
			case "download":
				cmds = append(cmds, a.downloadArtifacts(msg.RunID))
			}
		}

	case ActionResultMsg:
		a.message = msg.Message
		a.statusBar.SetMessage(msg.Message, !msg.Success)
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

// --- View-specific update handlers ---

func (a App) updateMainView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	case key.Matches(msg, theme.Keys.Tab):
		a.togglePanel()
		return a, nil

	case key.Matches(msg, theme.Keys.Refresh):
		return a, a.refreshCurrent()

	case key.Matches(msg, theme.Keys.Enter) || msg.String() == "l":
		if a.activePanel == repoPanel {
			// l or enter on repo panel: select repo and focus run panel
			a.activePanel = runPanel
			return a, a.selectRepo()
		}
		// l or enter on run panel: drill into run detail
		if run := a.runList.SelectedRun(); run != nil {
			a.statusBar.SetMessage("Loading run details...", false)
			return a, a.loadRunDetail(run.ID)
		}

	case msg.String() == "h":
		if a.activePanel == runPanel {
			a.activePanel = repoPanel
			return a, nil
		}

	case key.Matches(msg, theme.Keys.Trigger):
		if a.lastRepo != "" {
			a.statusBar.SetMessage("Loading workflows...", false)
			return a, a.loadWorkflows()
		}

	case key.Matches(msg, theme.Keys.Cancel):
		if run := a.runList.SelectedRun(); run != nil {
			cd := components.NewConfirmDialog(
				"Cancel Run",
				fmt.Sprintf("Cancel run #%d?", run.ID),
				"cancel",
				run.ID,
			)
			a.confirmDlg = &cd
			a.ActiveView = ConfirmView
			return a, nil
		}

	case key.Matches(msg, theme.Keys.Rerun):
		if run := a.runList.SelectedRun(); run != nil {
			cd := components.NewConfirmDialog(
				"Rerun Failed",
				fmt.Sprintf("Rerun failed jobs for run #%d?", run.ID),
				"rerun",
				run.ID,
			)
			a.confirmDlg = &cd
			a.ActiveView = ConfirmView
			return a, nil
		}

	case key.Matches(msg, theme.Keys.Logs):
		if run := a.runList.SelectedRun(); run != nil {
			a.statusBar.SetMessage("Loading logs...", false)
			return a, a.loadRunLog(run.ID)
		}

	case key.Matches(msg, theme.Keys.Download):
		if run := a.runList.SelectedRun(); run != nil {
			cd := components.NewConfirmDialog(
				"Download Artifacts",
				fmt.Sprintf("Download artifacts for run #%d?", run.ID),
				"download",
				run.ID,
			)
			a.confirmDlg = &cd
			a.ActiveView = ConfirmView
			return a, nil
		}

	case key.Matches(msg, theme.Keys.QuickSwitch):
		qs := components.NewQuickSwitch(a.repos)
		a.quickSwitch = &qs
		a.ActiveView = QuickSwitchView
		return a, a.quickSwitch.Init()
	}

	// Forward to active panel for navigation
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

	return a, tea.Batch(cmds...)
}

func (a App) updateDetailView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, theme.Keys.Back) || msg.String() == "h":
		a.ActiveView = MainView
		return a, nil

	case key.Matches(msg, theme.Keys.Logs) || msg.String() == "l":
		if run := a.runList.SelectedRun(); run != nil {
			a.statusBar.SetMessage("Loading logs...", false)
			return a, a.loadRunLog(run.ID)
		}

	case key.Matches(msg, theme.Keys.Rerun):
		if run := a.runList.SelectedRun(); run != nil {
			cd := components.NewConfirmDialog(
				"Rerun Failed",
				fmt.Sprintf("Rerun failed jobs for run #%d?", run.ID),
				"rerun",
				run.ID,
			)
			a.confirmDlg = &cd
			a.ActiveView = ConfirmView
			return a, nil
		}

	case key.Matches(msg, theme.Keys.Cancel):
		if run := a.runList.SelectedRun(); run != nil {
			cd := components.NewConfirmDialog(
				"Cancel Run",
				fmt.Sprintf("Cancel run #%d?", run.ID),
				"cancel",
				run.ID,
			)
			a.confirmDlg = &cd
			a.ActiveView = ConfirmView
			return a, nil
		}
	}

	// Forward to runDetail for navigation (j/k scrolling)
	cmd := a.runDetail.Update(msg)
	return a, cmd
}

func (a App) updateLogView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, theme.Keys.Back) || msg.String() == "h" {
		a.ActiveView = MainView
		return a, nil
	}

	// Forward to logViewer for scrolling
	cmd := a.logViewer.Update(msg)
	return a, cmd
}

func (a App) updateQuickSwitchView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.quickSwitch == nil {
		a.ActiveView = MainView
		return a, nil
	}
	// Ctrl+K toggles quick switch closed (ctrl+[ is normalized to esc upstream)
	if msg.String() == "ctrl+k" {
		a.ActiveView = MainView
		a.quickSwitch = nil
		return a, nil
	}
	qs, cmd := a.quickSwitch.Update(msg)
	a.quickSwitch = &qs
	return a, cmd
}

func (a App) updateTriggerView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.triggerDlg == nil {
		a.ActiveView = MainView
		return a, nil
	}
	td, cmd := a.triggerDlg.Update(msg)
	a.triggerDlg = &td
	return a, cmd
}

func (a App) updateConfirmView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.confirmDlg == nil {
		a.ActiveView = MainView
		return a, nil
	}
	cd, cmd := a.confirmDlg.Update(msg)
	a.confirmDlg = &cd
	return a, cmd
}

// --- View ---

func (a App) View() tea.View {
	if a.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	var content string

	switch a.ActiveView {
	case DetailView:
		a.runDetail.SetSize(a.width, a.height-2)
		content = lipgloss.JoinVertical(lipgloss.Left,
			a.runDetail.View(),
			a.statusBar.View(),
			components.HelpBar(a.width, "detail"),
		)

	case LogView:
		a.logViewer.SetSize(a.width, a.height-2)
		content = lipgloss.JoinVertical(lipgloss.Left,
			a.logViewer.View(),
			a.statusBar.View(),
			components.HelpBar(a.width, "log"),
		)

	default: // MainView and overlay views
		content = a.renderMainLayout()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a App) renderMainLayout() string {
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

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		panels,
		a.statusBar.View(),
		components.HelpBar(a.width, ""),
	)

	// Overlay dialogs on top of the main layout
	switch {
	case a.ActiveView == QuickSwitchView && a.quickSwitch != nil:
		return a.renderOverlay(mainContent, a.quickSwitch.View())
	case a.ActiveView == TriggerView && a.triggerDlg != nil:
		return a.renderOverlay(mainContent, a.triggerDlg.View())
	case a.ActiveView == ConfirmView && a.confirmDlg != nil:
		return a.renderOverlay(mainContent, a.confirmDlg.View())
	default:
		return mainContent
	}
}

func (a App) renderOverlay(_ string, overlay string) string {
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, overlay)
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

func (a *App) loadRunDetail(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		detail, err := a.client.ViewRun(ctx, repo, runID)
		return RunDetailLoadedMsg{Detail: detail, Err: err}
	}
}

func (a *App) loadRunLog(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		log, err := a.client.ViewRunLog(ctx, repo, runID)
		return LogLoadedMsg{Log: log, Err: err}
	}
}

func (a *App) loadWorkflows() tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		wfs, err := a.client.ListWorkflows(ctx, repo)
		return WorkflowsLoadedMsg{Workflows: wfs, Err: err}
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

func (a *App) triggerWorkflow(file, branch string) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := a.client.TriggerWorkflow(ctx, repo, file, branch)
		if err != nil {
			return ActionResultMsg{Action: "trigger", Success: false, Message: "Trigger failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "trigger", Success: true, Message: "Workflow triggered"}
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
