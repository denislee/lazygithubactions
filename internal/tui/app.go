package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/gh"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/components"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

type panel int

const (
	orgPanel panel = iota
	repoPanel
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
	client      *gh.Client
	orgSelector components.OrgSelector
	repoList    components.RepoList
	runList     components.RunList

	// Sub-views
	runDetail   components.RunDetail
	logViewer   components.LogViewer
	quickSwitch *components.QuickSwitch
	triggerDlg  *components.TriggerDialog
	confirmDlg  *components.ConfirmDialog
	statusBar   components.StatusBar

	activePanel  panel
	ActiveView   view
	previousView view // for back navigation from log view
	width        int
	height       int

	lastRepo    string
	allRepos    []models.Repo // all repos from API
	repos       []models.Repo // filtered by org for QuickSwitch
	selectedOrg string

	loadingRepos bool
	loadingRuns  bool
	spinner      spinner.Model
	err          error
	message      string
}

func NewApp() App {
	s := spinner.New()
	s.Spinner = spinner.Dot

	cfg := gh.LoadConfig()

	app := App{
		client:      gh.NewClient(),
		orgSelector: components.NewOrgSelector(),
		repoList:    components.NewRepoList(),
		runList:     components.NewRunList(),
		runDetail:   components.NewRunDetail(),
		logViewer:   components.NewLogViewer(),
		statusBar:   components.NewStatusBar(),
		spinner:     s,
		activePanel:  repoPanel,
		ActiveView:   MainView,
		selectedOrg:  cfg.SelectedOrg,
		loadingRepos: true,
	}
	app.runList.Compact = cfg.CompactView
	return app
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.loadRepos(),
		a.tick(),
		a.spinner.Tick,
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

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

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
		a.loadingRepos = false
		if msg.Err != nil {
			a.err = msg.Err
			a.statusBar.SetMessage("Error: "+msg.Err.Error(), true)
		} else {
			a.allRepos = msg.Repos
			a.orgSelector.SetOrgs(msg.Repos)
			a.orgSelector.SelectOrg(a.selectedOrg)
			a.filterReposByOrg()
			a.statusBar.Clear()
			if repo := a.repoList.SelectedRepo(); repo != nil {
				a.loadingRuns = true
				cmds = append(cmds, a.loadRuns(repo.FullName))
			}
		}

	case RunsLoadedMsg:
		a.loadingRuns = false
		if msg.Err != nil {
			a.err = msg.Err
			a.statusBar.SetMessage("Error: "+msg.Err.Error(), true)
		} else {
			a.runList.SetRuns(msg.Runs, msg.Repo)
			a.lastRepo = msg.Repo
			a.statusBar.Clear()
		}

	case RunDetailLoadedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Error loading detail: "+msg.Err.Error(), true)
			a.ActiveView = MainView
		} else {
			a.runDetail.SetDetail(msg.Detail)
			a.ActiveView = DetailView
			a.statusBar.Clear()
		}

	case LogLoadedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Error loading logs: "+msg.Err.Error(), true)
			a.ActiveView = a.previousView
		} else {
			title := a.lastRepo
			if run := a.runList.SelectedRun(); run != nil {
				title = fmt.Sprintf("%s #%d", run.WorkflowName, run.ID)
			}
			a.logViewer.SetContent(title, msg.Log)
			a.statusBar.Clear()
			a.ActiveView = LogView
		}

	case WorkflowsLoadedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Error loading workflows: "+msg.Err.Error(), true)
		} else {
			td := components.NewTriggerDialog(msg.Workflows)
			a.triggerDlg = &td
			a.ActiveView = TriggerView
			a.statusBar.Clear()
		}

	case components.LogCopiedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Copy failed: "+msg.Err.Error(), true)
		} else {
			a.statusBar.SetMessage(fmt.Sprintf("Copied %d lines to clipboard", msg.Lines), false)
		}

	case components.OrgChangedMsg:
		a.selectedOrg = msg.Org
		a.filterReposByOrg()
		_ = gh.SaveConfig(gh.UserConfig{SelectedOrg: msg.Org})
		// Load runs for first repo in filtered list
		if repo := a.repoList.SelectedRepo(); repo != nil {
			a.loadingRuns = true
			cmds = append(cmds, a.loadRuns(repo.FullName))
		}

	case components.QuickSwitchResultMsg:
		a.ActiveView = MainView
		a.quickSwitch = nil
		if !msg.Cancelled && msg.Repo != nil {
			a.lastRepo = msg.Repo.FullName
			a.activePanel = runPanel
			a.loadingRuns = true
			a.runList.SetRuns(nil, msg.Repo.FullName)
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
		a.cyclePanel()
		return a, nil

	case key.Matches(msg, theme.Keys.Refresh):
		a.loadingRepos = true
		a.loadingRuns = true
		return a, tea.Batch(a.loadRepos(), a.refreshCurrent())

	case key.Matches(msg, theme.Keys.Enter) || msg.String() == "l":
		if a.activePanel == orgPanel {
			// enter/l on org: move to repo panel
			a.activePanel = repoPanel
			return a, nil
		}
		if a.activePanel == repoPanel {
			repo := a.repoList.SelectedRepo()
			if repo == nil {
				return a, nil
			}
			a.activePanel = runPanel
			// Only reload if switching to a different repo
			if repo.FullName != a.lastRepo {
				return a, a.selectRepo()
			}
			return a, nil
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
		if a.activePanel == repoPanel {
			a.activePanel = orgPanel
			return a, nil
		}

	case msg.String() == "v":
		a.runList.ToggleCompact()
		_ = gh.SaveConfig(gh.UserConfig{SelectedOrg: a.selectedOrg, CompactView: a.runList.Compact})
		return a, nil

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
			a.previousView = MainView
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

	// Forward to active panel for navigation, with boundary transitions
	switch a.activePanel {
	case orgPanel:
		// If at bottom of org list and pressing down/j, move to repo panel
		if a.orgSelector.AtBottom() && key.Matches(msg, theme.Keys.Down) {
			a.activePanel = repoPanel
			return a, nil
		}
		cmd := a.orgSelector.Update(msg)
		cmds = append(cmds, cmd)
	case repoPanel:
		// If at top of repo list and pressing up/k, move to org panel
		if a.repoList.AtTop() && key.Matches(msg, theme.Keys.Up) {
			a.activePanel = orgPanel
			return a, nil
		}
		cmd := a.repoList.Update(msg)
		cmds = append(cmds, cmd)
		if repo := a.repoList.SelectedRepo(); repo != nil && repo.FullName != a.lastRepo {
			a.loadingRuns = true
			a.runList.SetRuns(nil, repo.FullName) // clear stale data immediately
			cmds = append(cmds, a.loadRuns(repo.FullName))
		}
	case runPanel:
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
			a.previousView = DetailView
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

	cmd := a.runDetail.Update(msg)
	return a, cmd
}

func (a App) updateLogView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, theme.Keys.Back) || msg.String() == "h" {
		a.ActiveView = a.previousView
		return a, nil
	}

	cmd := a.logViewer.Update(msg)
	return a, cmd
}

func (a App) updateQuickSwitchView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.quickSwitch == nil {
		a.ActiveView = MainView
		return a, nil
	}
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
			theme.HelpBarStyle.Width(a.width).Render(a.logViewer.HelpText()),
		)

	default: // MainView and overlay views
		content = a.renderMainLayout()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a App) renderMainLayout() string {
	leftWidth := a.width / 4
	if leftWidth < 20 {
		leftWidth = 20
	}
	runWidth := a.width - leftWidth
	totalHeight := a.height - 2

	// Inner dimensions (subtract border: 2 top/bottom, 2 left/right padding)
	innerWidth := leftWidth - 4

	// Org section height (content lines, no border)
	orgContentLines := min(a.orgSelector.OrgCount(), 5)
	if orgContentLines < 1 {
		orgContentLines = 1
	}
	orgSectionHeight := orgContentLines + 1 // +1 for title line

	// Repo section gets the rest
	// Total inner = totalHeight - 2 (border) - orgSection - 1 (divider)
	repoContentHeight := totalHeight - 2 - orgSectionHeight - 1
	if repoContentHeight < 3 {
		repoContentHeight = 3
	}

	a.orgSelector.SetSize(innerWidth, orgContentLines)
	a.orgSelector.SetFocused(a.activePanel == orgPanel)

	a.repoList.SetSize(innerWidth, repoContentHeight-1) // -1 for title
	a.repoList.SetFocused(a.activePanel == repoPanel)

	a.runList.SetSize(runWidth, totalHeight)
	a.runList.SetFocused(a.activePanel == runPanel)

	// Build left panel content: org + divider + repos in a single frame
	orgContent := a.orgSelector.ViewContent()
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).
		Render(strings.Repeat("─", innerWidth))

	var repoContent string
	if a.loadingRepos {
		repoContent = theme.TitleStyle.Render("Repositories") + "\n"
		repoContent += "  " + a.spinner.View() + " Loading..."
	} else {
		repoContent = a.repoList.ViewContent()
	}

	leftContent := orgContent + divider + "\n" + repoContent

	focused := a.activePanel == orgPanel || a.activePanel == repoPanel
	leftStyle := theme.PanelStyle
	if focused {
		leftStyle = theme.ActivePanelStyle
	}
	leftCol := leftStyle.Width(leftWidth).Height(totalHeight).Render(leftContent)

	runView := a.runList.View()
	if a.loadingRuns {
		runView = a.renderLoadingPanel(fmt.Sprintf("Workflow Runs — %s", a.runList.Repo()), runWidth, totalHeight, a.activePanel == runPanel)
	}

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, runView)

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		panels,
		a.statusBar.View(),
		components.HelpBar(a.width, ""),
	)

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

func (a App) renderLoadingPanel(title string, width, height int, focused bool) string {
	content := theme.TitleStyle.Render(title) + "\n\n"
	content += "  " + a.spinner.View() + " Loading..."
	style := theme.PanelStyle
	if focused {
		style = theme.ActivePanelStyle
	}
	return style.Width(width).Height(height).Render(content)
}

func (a App) renderOverlay(_ string, overlay string) string {
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, overlay)
}

// --- Helpers ---

func (a *App) filterReposByOrg() {
	if a.selectedOrg == "" {
		a.repos = a.allRepos
	} else {
		a.repos = nil
		for _, r := range a.allRepos {
			if r.Owner == a.selectedOrg {
				a.repos = append(a.repos, r)
			}
		}
	}
	a.repoList.SetRepos(a.repos)
}

func (a *App) cyclePanel() {
	switch a.activePanel {
	case orgPanel:
		a.activePanel = repoPanel
	case repoPanel:
		a.activePanel = runPanel
	case runPanel:
		a.activePanel = orgPanel
	}
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
	a.loadingRuns = true
	a.runList.SetRuns(nil, repo.FullName) // clear stale data
	return a.loadRuns(repo.FullName)
}

func (a *App) tick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}
