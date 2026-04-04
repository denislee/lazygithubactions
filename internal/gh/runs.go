package gh

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/dns/lazygithubactions/internal/models"
)

func (c *Client) ListRuns(ctx context.Context, repo string) ([]models.WorkflowRun, error) {
	var runs []models.WorkflowRun
	err := c.runJSON(ctx, &runs, "run", "list",
		"-R", repo,
		"--json", "databaseId,name,displayTitle,status,conclusion,headBranch,event,createdAt,updatedAt,url,workflowName,attempt",
		"--limit", "30",
	)
	return runs, err
}

func (c *Client) ViewRun(ctx context.Context, repo string, runID int64) (*models.RunDetail, error) {
	id := strconv.FormatInt(runID, 10)
	out, err := c.run(ctx, "run", "view", id, "-R", repo, "--json",
		"databaseId,name,status,conclusion,headBranch,event,createdAt,updatedAt,url,workflowName,jobs")
	if err != nil {
		return nil, err
	}
	var detail models.RunDetail
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *Client) ViewRunLog(ctx context.Context, repo string, runID int64) (string, error) {
	id := strconv.FormatInt(runID, 10)
	out, err := c.run(ctx, "run", "view", id, "-R", repo, "--log")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (c *Client) CancelRun(ctx context.Context, repo string, runID int64) error {
	id := strconv.FormatInt(runID, 10)
	_, err := c.run(ctx, "run", "cancel", id, "-R", repo)
	return err
}

func (c *Client) RerunFailed(ctx context.Context, repo string, runID int64) error {
	id := strconv.FormatInt(runID, 10)
	_, err := c.run(ctx, "run", "rerun", id, "-R", repo, "--failed")
	return err
}

func (c *Client) DownloadArtifacts(ctx context.Context, repo string, runID int64, dir string) error {
	id := strconv.FormatInt(runID, 10)
	args := []string{"run", "download", id, "-R", repo}
	if dir != "" {
		args = append(args, "-D", dir)
	}
	_, err := c.run(ctx, args...)
	return err
}
