package tui

import (
	"fmt"
	"log"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/gh"
	"github.com/dns/lazygithubactions/internal/tui/components"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

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
		if a.lastRepo != "" {
			return a, tea.Batch(a.loadRepos(true), a.refreshCurrent())
		}
		return a, a.refreshCurrent()

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
		a.previousView = MainView
		a.ActiveView = QuickSwitchView
		return a, a.quickSwitch.Init()

	case key.Matches(msg, theme.Keys.OpenBrowser):
		if repo := a.repoList.SelectedRepo(); repo != nil {
			url := fmt.Sprintf("https://github.com/%s/actions", repo.FullName)
			_ = openBrowser(url)
			a.statusBar.SetMessage(fmt.Sprintf("Opening %s", url), false)
		}
		return a, nil
	}

	// Forward to active panel for navigation, with boundary transitions.
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
			// Cancel any in-flight runs request immediately.
			if a.cancelRuns != nil {
				a.cancelRuns()
				a.cancelRuns = nil
			}
			// Debounce: defer loading runs until cursor settles.
			a.pendingRepo = repo.FullName
			a.loadingRuns = true
			a.runList.SetRuns(nil, nil, repo.FullName) // clear stale data immediately
			cmds = append(cmds, a.debounceRepo(repo.FullName))
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

	case key.Matches(msg, theme.Keys.Refresh):
		if run := a.runList.SelectedRun(); run != nil {
			a.statusBar.SetMessage("Refreshing...", false)
			return a, a.refreshRunDetail(run.ID)
		}

	case key.Matches(msg, theme.Keys.QuickSwitch):
		qs := components.NewQuickSwitch(a.repos)
		a.quickSwitch = &qs
		a.previousView = DetailView
		a.ActiveView = QuickSwitchView
		return a, a.quickSwitch.Init()

	case key.Matches(msg, theme.Keys.OpenBrowser):
		if run := a.runList.SelectedRun(); run != nil {
			url := fmt.Sprintf("https://github.com/%s/actions/runs/%d", a.lastRepo, run.ID)
			if job := a.runDetail.SelectedJob(); job != nil {
				if job.URL != "" {
					url = job.URL
				} else if job.DatabaseID > 0 {
					url = fmt.Sprintf("%s/job/%d", url, job.DatabaseID)
				} else if job.ID > 0 {
					url = fmt.Sprintf("%s/job/%d", url, job.ID)
				}
			}
			_ = openBrowser(url)
			a.statusBar.SetMessage(fmt.Sprintf("Opening %s", url), false)
			return a, nil
		}
	}

	cmd := a.runDetail.Update(msg)
	return a, cmd
}

func (a App) updateLogView(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Ensure log viewer has correct dimensions before processing keys.
	a.logViewer.SetSize(a.width, a.height)

	// Don't intercept keys while searching or in visual mode.
	if !a.logViewer.InSearchMode() {
		// h always goes back
		if msg.String() == "h" {
			a.ActiveView = a.previousView
			return a, nil
		}
		// esc goes back only if not in visual mode
		if key.Matches(msg, theme.Keys.Back) && !a.logViewer.InVisualMode() {
			a.ActiveView = a.previousView
			return a, nil
		}
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

// --- App-level helpers ---

func (a *App) autoSelectCurrentRepo() []tea.Cmd {
	if a.currentRepo == nil {
		return nil
	}

	a.selectedOrg = a.currentRepo.Owner
	a.orgSelector.SelectOrg(a.selectedOrg)
	a.filterReposByOrg()
	a.repoList.SelectByName(a.currentRepo.FullName)
	a.activePanel = runPanel

	if repo := a.repoList.SelectedRepo(); repo != nil && repo.FullName == a.currentRepo.FullName {
		a.loadingRuns = true
		return []tea.Cmd{a.loadRuns(repo.FullName)}
	}

	log.Printf("auto-select: repo %s not found under org %s", a.currentRepo.FullName, a.currentRepo.Owner)
	return nil
}

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

func (a *App) selectRepo() tea.Cmd {
	repo := a.repoList.SelectedRepo()
	if repo == nil {
		return nil
	}
	a.activePanel = runPanel
	a.loadingRuns = true
	a.runList.SetRuns(nil, nil, repo.FullName) // clear stale data
	return a.loadRuns(repo.FullName)
}
