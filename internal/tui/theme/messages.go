package theme

import (
	"github.com/dns/lazygithubactions/internal/models"
)

type ReposLoadedMsg struct {
	Repos []models.Repo
	Err   error
}

type RunsLoadedMsg struct {
	Repo string
	Runs []models.WorkflowRun
	Err  error
}

type RunDetailLoadedMsg struct {
	Detail *models.RunDetail
	Err    error
}

type LogLoadedMsg struct {
	Log string
	Err error
}

type WorkflowsLoadedMsg struct {
	Workflows []models.Workflow
	Err       error
}

type ActionResultMsg struct {
	Action  string
	Success bool
	Message string
}

type TickMsg struct{}
