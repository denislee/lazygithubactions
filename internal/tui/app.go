package tui

import (
	"context"
	"fmt"
	"log"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
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

// --- App-local message types ---

// RepoStatusMsg carries a single repo's latest run status from the background poller.
type RepoStatusMsg struct {
	Repo       string
	Status     string
	Conclusion string
}

// ActiveRunDetailMsg carries a polled in-progress run detail for the status bar.
type ActiveRunDetailMsg struct {
	Detail *models.RunDetail
}

// ActiveRunTickMsg triggers the next poll for the active run.
type ActiveRunTickMsg struct{}

// DetailRefreshMsg carries a refreshed run detail for the detail view (preserves cursor).
type DetailRefreshMsg struct {
	Detail *models.RunDetail
	Err    error
}

// RepoDebounceMsg fires after a short delay to load runs for the selected repo.
type RepoDebounceMsg struct {
	Repo string
}

// --- App ---

type App struct {
	client *gh.Client
	dur    Durations

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

	lastRepo           string
	allRepos           []models.Repo // all repos from API
	repos              []models.Repo // filtered by org for QuickSwitch
	selectedOrg        string
	currentRepo        *models.Repo // detected repository for the current directory
	currentRepoChecked bool         // true once loadCurrentRepo has returned (success or nil)

	loadingRepos bool
	loadingRuns  bool
	spinner      spinner.Model
	err          error

	// Debounce: when navigating repos quickly, defer loading runs until cursor settles.
	pendingRepo string
	cancelRuns  context.CancelFunc // cancels the in-flight loadRuns request

	// Active run tracking: when an in-progress run is detected, poll its detail.
	activeRunID   int64
	activeRunRepo string
}

func NewApp() App {
	s := spinner.New()
	s.Spinner = spinner.Dot

	cfg := gh.LoadConfig()

	app := App{
		client:       gh.NewClient(),
		dur:          DefaultDurations,
		orgSelector:  components.NewOrgSelector(),
		repoList:     components.NewRepoList(),
		runList:      components.NewRunList(),
		runDetail:    components.NewRunDetail(),
		logViewer:    components.NewLogViewer(),
		statusBar:    components.NewStatusBar(),
		spinner:      s,
		activePanel:  repoPanel,
		ActiveView:   MainView,
		selectedOrg:  cfg.SelectedOrg,
		loadingRepos: true,
	}
	app.runList.Compact = cfg.CompactView
	return app
}

func (a App) Init() tea.Cmd {
	// Seed repo list with cached run statuses.
	if entries, err := gh.LoadCachedRunStatuses(); err == nil && entries != nil {
		for repo, e := range entries {
			a.repoList.SetRepoStatus(repo, e.Status, e.Conclusion)
		}
	}
	return tea.Batch(
		a.loadCurrentRepo(),
		a.loadRepos(false),
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
		a.layoutComponents()
		return a, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd

	case tea.KeyPressMsg:
		// Ctrl+C always quits, no matter what view.
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

		// Normalize ctrl+[ to esc globally.
		if msg.String() == "ctrl+[" {
			msg = tea.KeyPressMsg{Code: 27} // ESC
		}

		// q quits only outside overlays, and not while searching in the log viewer.
		if msg.String() == "q" && a.ActiveView != QuickSwitchView &&
			a.ActiveView != TriggerView && a.ActiveView != ConfirmView &&
			!(a.ActiveView == LogView && a.logViewer.InSearchMode()) {
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

	case CurrentRepoLoadedMsg:
		a.currentRepoChecked = true
		if msg.Err != nil {
			log.Printf("detect current repo: %v", msg.Err)
		} else if msg.Repo != nil {
			a.currentRepo = msg.Repo
		}
		// If repos are already loaded, trigger the appropriate initial selection now.
		if len(a.allRepos) > 0 {
			if a.currentRepo != nil {
				cmds = append(cmds, a.autoSelectCurrentRepo()...)
			} else if repo := a.repoList.SelectedRepo(); repo != nil && a.lastRepo == "" {
				a.loadingRuns = true
				cmds = append(cmds, a.loadRuns(repo.FullName))
			}
		}

	case ReposLoadedMsg:
		a.loadingRepos = false
		if msg.Err != nil {
			log.Printf("list repos: %v", msg.Err)
			a.err = msg.Err
			a.statusBar.SetMessage("Error: "+msg.Err.Error(), true)
		} else {
			a.allRepos = msg.Repos
			a.orgSelector.SetOrgs(msg.Repos)
			a.orgSelector.SelectOrg(a.selectedOrg)
			a.filterReposByOrg()
			a.statusBar.Clear()

			// Wait for current-repo detection before choosing which repo to load.
			// This avoids racing loadRuns(firstRepo) with loadRuns(currentRepo).
			if a.currentRepoChecked {
				if a.currentRepo != nil {
					cmds = append(cmds, a.autoSelectCurrentRepo()...)
				} else if repo := a.repoList.SelectedRepo(); repo != nil {
					a.loadingRuns = true
					cmds = append(cmds, a.loadRuns(repo.FullName))
				}
			} else {
				a.loadingRuns = true
			}
		}

	case RepoDebounceMsg:
		// Only load if the cursor is still on this repo (debounce).
		if repo := a.repoList.SelectedRepo(); repo != nil && repo.FullName == msg.Repo && a.pendingRepo == msg.Repo {
			a.pendingRepo = ""
			cmds = append(cmds, a.loadRuns(msg.Repo))
		} else if a.pendingRepo != msg.Repo {
			// Cursor moved on; skip this stale debounce.
			a.loadingRuns = false
		}

	case RunsLoadedMsg:
		// Ignore stale responses from repos we've navigated away from.
		if repo := a.repoList.SelectedRepo(); repo != nil && repo.FullName != msg.Repo && msg.Repo != a.lastRepo {
			return a, tea.Batch(cmds...)
		}
		// Silently discard cancelled requests (user navigated away).
		if msg.Err != nil && msg.Err == context.Canceled {
			return a, tea.Batch(cmds...)
		}
		a.loadingRuns = false
		if msg.Err != nil {
			a.err = msg.Err
			a.statusBar.SetMessage("Error: "+msg.Err.Error(), true)
		} else {
			a.runList.SetRuns(msg.Runs, msg.Commit, msg.Repo)
			a.lastRepo = msg.Repo
			// Track last run status for repo icon.
			if len(msg.Runs) > 0 {
				r := msg.Runs[0]
				a.repoList.SetRepoStatus(msg.Repo, r.Status, r.Conclusion)
				// If the latest run is in-progress, start tracking it.
				if isActiveStatus(r.Status) {
					if a.activeRunID != r.ID {
						a.activeRunID = r.ID
						a.activeRunRepo = msg.Repo
						cmds = append(cmds, a.pollActiveRun(), a.activeRunTick())
					}
				} else if a.activeRunRepo == msg.Repo {
					a.activeRunID = 0
					a.activeRunRepo = ""
					a.statusBar.Clear()
				}
			}
			if a.activeRunID == 0 {
				a.statusBar.Clear()
			}
		}

	case RunDetailLoadedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Error loading detail: "+msg.Err.Error(), true)
			a.ActiveView = MainView
		} else {
			a.runDetail.SetDetail(msg.Detail)
			a.ActiveView = DetailView
			a.statusBar.Clear()
			if a.runDetail.HasRunningJobs() {
				cmds = append(cmds, a.detailTick())
			}
		}

	case DetailRefreshMsg:
		if a.ActiveView != DetailView {
			break
		}
		if msg.Err != nil {
			a.statusBar.SetMessage("Refresh failed: "+msg.Err.Error(), true)
		} else {
			a.runDetail.UpdateDetail(msg.Detail)
			a.statusBar.Clear()
			if a.runDetail.HasRunningJobs() {
				cmds = append(cmds, a.detailTick())
			}
		}

	case theme.DetailTickMsg:
		if a.ActiveView == DetailView {
			if run := a.runList.SelectedRun(); run != nil && a.runDetail.HasRunningJobs() {
				cmds = append(cmds, a.refreshRunDetail(run.ID))
			}
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
			a.logViewer.SetSize(a.width, a.height)
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

	case components.TriggerWorkflowSelectedMsg:
		a.statusBar.SetMessage("Loading branches...", false)
		cmds = append(cmds, a.loadBranchesAndInputs(msg.WorkflowPath))

	case theme.BranchesAndInputsLoadedMsg:
		if msg.Err != nil {
			a.statusBar.SetMessage("Error loading branches: "+msg.Err.Error(), true)
			a.ActiveView = MainView
			a.triggerDlg = nil
		} else if a.triggerDlg != nil {
			cmd := a.triggerDlg.SetBranchesAndInputs(msg.Branches, msg.Inputs, a.width)
			cmds = append(cmds, cmd)
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
		// Load runs for first repo in filtered list.
		if repo := a.repoList.SelectedRepo(); repo != nil {
			a.loadingRuns = true
			cmds = append(cmds, a.loadRuns(repo.FullName))
		}

	case components.QuickSwitchResultMsg:
		a.quickSwitch = nil
		if !msg.Cancelled && msg.Repo != nil {
			a.ActiveView = MainView
			a.lastRepo = msg.Repo.FullName
			a.repoList.SelectByName(msg.Repo.FullName)
			a.activePanel = runPanel
			a.loadingRuns = true
			a.runList.SetRuns(nil, nil, msg.Repo.FullName)
			cmds = append(cmds, a.loadRuns(msg.Repo.FullName))
		} else {
			a.ActiveView = a.previousView
		}

	case components.TriggerDialogResultMsg:
		a.ActiveView = MainView
		a.triggerDlg = nil
		if !msg.Cancelled {
			cmds = append(cmds, a.triggerWorkflow(msg.WorkflowFile, msg.Branch, msg.Inputs))
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
		a.statusBar.SetMessage(msg.Message, !msg.Success)
		if msg.Success {
			cmds = append(cmds, a.refreshCurrent())
		}

	case TickMsg:
		if a.lastRepo != "" && a.ActiveView == MainView {
			cmds = append(cmds, a.loadRuns(a.lastRepo))
		}
		cmds = append(cmds, a.tick())

	case RepoStatusMsg:
		a.repoList.SetRepoStatus(msg.Repo, msg.Status, msg.Conclusion)
		gh.SaveRunStatus(msg.Repo, msg.Status, msg.Conclusion)

	case ActiveRunDetailMsg:
		if msg.Detail == nil || a.activeRunID == 0 {
			break
		}
		run := msg.Detail
		if !isActiveStatus(run.Status) {
			// Run finished.
			a.activeRunID = 0
			a.activeRunRepo = ""
			icon := theme.StatusIcon(run.Status, run.Conclusion)
			a.statusBar.SetMessage(
				fmt.Sprintf("%s %s completed: %s", icon, run.WorkflowName, run.Conclusion),
				run.Conclusion == "failure",
			)
			cmds = append(cmds, a.refreshCurrent())
			break
		}
		a.statusBar.SetMessage(formatActiveRun(run), false)

	case ActiveRunTickMsg:
		if a.activeRunID != 0 {
			cmds = append(cmds, a.pollActiveRun(), a.activeRunTick())
		}
	}

	return a, tea.Batch(cmds...)
}

// isActiveStatus reports whether a run is still executing.
func isActiveStatus(s string) bool {
	return s == "in_progress" || s == "queued" || s == "waiting"
}

// formatActiveRun builds the status-bar line for an in-progress run.
func formatActiveRun(run *models.RunDetail) string {
	var activeJob, activeStep string
	for _, job := range run.Jobs {
		if job.Status == "in_progress" {
			activeJob = job.Name
			for _, step := range job.Steps {
				if step.Status == "in_progress" {
					activeStep = step.Name
					break
				}
			}
			break
		}
		if (job.Status == "queued" || job.Status == "waiting") && activeJob == "" {
			activeJob = job.Name
		}
	}
	out := fmt.Sprintf("● %s", run.WorkflowName)
	if activeJob != "" {
		out += fmt.Sprintf(" → %s", activeJob)
	}
	if activeStep != "" {
		out += fmt.Sprintf(" → %s", activeStep)
	}
	return out
}
