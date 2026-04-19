package tui

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/gh"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui/theme"
)

func (a *App) loadRepos(forceRefresh bool) tea.Cmd {
	if !forceRefresh {
		repos, _, err := gh.LoadCachedRepos()
		if err == nil && repos != nil {
			return func() tea.Msg {
				return ReposLoadedMsg{Repos: repos}
			}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.LongTimeout)
		defer cancel()
		repos, err := a.client.ListRepos(ctx)
		if err == nil {
			_ = gh.SaveCachedRepos(repos)
		}
		return ReposLoadedMsg{Repos: repos, Err: err}
	}
}

func (a *App) loadCurrentRepo() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.ShortTimeout)
		defer cancel()
		repo, err := a.client.GetCurrentRepo(ctx)
		return CurrentRepoLoadedMsg{Repo: repo, Err: err}
	}
}

func (a *App) loadRuns(repo string) tea.Cmd {
	// Cancel any in-flight request before starting a new one.
	if a.cancelRuns != nil {
		a.cancelRuns()
	}
	ctx, cancel := context.WithTimeout(context.Background(), a.dur.DefaultTimeout)
	a.cancelRuns = cancel
	return func() tea.Msg {
		defer cancel()

		var runs []models.WorkflowRun
		var runsErr error
		var commit *models.Commit
		var wg sync.WaitGroup

		wg.Add(2)
		go func() {
			defer wg.Done()
			runs, runsErr = a.client.ListRuns(ctx, repo)
		}()
		go func() {
			defer wg.Done()
			// commit errors are non-fatal (empty repo, transient API failure)
			commit, _ = a.client.GetLatestCommit(ctx, repo)
		}()
		wg.Wait()

		if ctx.Err() != nil {
			return RunsLoadedMsg{Repo: repo, Err: ctx.Err()}
		}
		return RunsLoadedMsg{Repo: repo, Runs: runs, Commit: commit, Err: runsErr}
	}
}

func (a *App) loadRunDetail(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.DefaultTimeout)
		defer cancel()
		detail, err := a.client.ViewRun(ctx, repo, runID)
		return RunDetailLoadedMsg{Detail: detail, Err: err}
	}
}

func (a *App) loadRunLog(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.LongTimeout)
		defer cancel()
		log, err := a.client.ViewRunLog(ctx, repo, runID)
		return LogLoadedMsg{Log: log, Err: err}
	}
}

func (a *App) loadWorkflows() tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.DefaultTimeout)
		defer cancel()
		wfs, err := a.client.ListWorkflows(ctx, repo)
		return WorkflowsLoadedMsg{Workflows: wfs, Err: err}
	}
}

func (a *App) cancelRun(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.DefaultTimeout)
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
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.DefaultTimeout)
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
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.DownloadTimeout)
		defer cancel()
		err := a.client.DownloadArtifacts(ctx, repo, runID, "")
		if err != nil {
			return ActionResultMsg{Action: "download", Success: false, Message: "Download failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "download", Success: true, Message: "Artifacts downloaded"}
	}
}

func (a *App) triggerWorkflow(file, branch string, inputs map[string]string) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.DefaultTimeout)
		defer cancel()
		err := a.client.TriggerWorkflow(ctx, repo, file, branch, inputs)
		if err != nil {
			return ActionResultMsg{Action: "trigger", Success: false, Message: "Trigger failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "trigger", Success: true, Message: "Workflow triggered"}
	}
}

func (a *App) loadBranchesAndInputs(workflowPath string) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.DefaultTimeout)
		defer cancel()

		var branches []string
		var inputs []models.WorkflowInput
		var branchErr, inputErr error
		var wg sync.WaitGroup

		wg.Add(2)
		go func() {
			defer wg.Done()
			branches, branchErr = a.client.ListBranches(ctx, repo)
		}()
		go func() {
			defer wg.Done()
			inputs, inputErr = a.client.GetWorkflowInputs(ctx, repo, workflowPath)
		}()
		wg.Wait()

		if branchErr != nil {
			return theme.BranchesAndInputsLoadedMsg{Err: branchErr}
		}
		if inputErr != nil {
			// Non-fatal: workflow may not have inputs
			return theme.BranchesAndInputsLoadedMsg{Branches: branches}
		}
		return theme.BranchesAndInputsLoadedMsg{Branches: branches, Inputs: inputs}
	}
}

func (a *App) refreshCurrent() tea.Cmd {
	if a.lastRepo != "" {
		return a.loadRuns(a.lastRepo)
	}
	return a.loadRepos(true)
}

func (a *App) debounceRepo(repo string) tea.Cmd {
	return tea.Tick(a.dur.RepoDebounceDelay, func(t time.Time) tea.Msg {
		return RepoDebounceMsg{Repo: repo}
	})
}

func (a *App) tick() tea.Cmd {
	return tea.Tick(a.dur.RefreshInterval, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (a *App) activeRunTick() tea.Cmd {
	return tea.Tick(a.dur.ActiveRunInterval, func(t time.Time) tea.Msg {
		return ActiveRunTickMsg{}
	})
}

func (a *App) detailTick() tea.Cmd {
	return tea.Tick(a.dur.DetailInterval, func(t time.Time) tea.Msg {
		return theme.DetailTickMsg{}
	})
}

func (a *App) refreshRunDetail(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.DefaultTimeout)
		defer cancel()
		detail, err := a.client.ViewRun(ctx, repo, runID)
		return DetailRefreshMsg{Detail: detail, Err: err}
	}
}

func (a *App) pollActiveRun() tea.Cmd {
	repo := a.activeRunRepo
	runID := a.activeRunID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), a.dur.PollTimeout)
		defer cancel()
		detail, err := a.client.ViewRun(ctx, repo, runID)
		if err != nil {
			return nil
		}
		return ActiveRunDetailMsg{Detail: detail}
	}
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return errors.New("unsupported platform")
	}
}
