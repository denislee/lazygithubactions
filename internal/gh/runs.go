package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/dns/lazygithubactions/internal/models"
)

// apiRun matches the GitHub REST API workflow run object.
type apiRun struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DisplayTitle string `json:"display_title"`
	Status       string `json:"status"`
	Conclusion   *string `json:"conclusion"`
	HeadBranch   string `json:"head_branch"`
	Event        string `json:"event"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	HTMLURL      string `json:"html_url"`
	RunAttempt   int    `json:"run_attempt"`
	Actor        *struct {
		Login string `json:"login"`
	} `json:"triggering_actor"`
}

func (c *Client) ListRuns(ctx context.Context, repo string) ([]models.WorkflowRun, error) {
	out, err := c.run(ctx, "api",
		fmt.Sprintf("/repos/%s/actions/runs?per_page=30", repo),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Runs []apiRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}

	runs := make([]models.WorkflowRun, len(resp.Runs))
	for i, r := range resp.Runs {
		conclusion := ""
		if r.Conclusion != nil {
			conclusion = *r.Conclusion
		}
		actor := ""
		if r.Actor != nil {
			actor = r.Actor.Login
		}
		created, _ := time.Parse(time.RFC3339, r.CreatedAt)
		updated, _ := time.Parse(time.RFC3339, r.UpdatedAt)

		runs[i] = models.WorkflowRun{
			ID:           r.ID,
			Name:         r.Name,
			DisplayTitle: r.DisplayTitle,
			Status:       r.Status,
			Conclusion:   conclusion,
			Branch:       r.HeadBranch,
			Event:        r.Event,
			CreatedAt:    created,
			UpdatedAt:    updated,
			URL:          r.HTMLURL,
			WorkflowName: r.Name,
			Attempt:      r.RunAttempt,
			Actor:        actor,
		}
	}
	return runs, nil
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

// LatestRunStatus fetches only the most recent workflow run for a repo.
func (c *Client) LatestRunStatus(ctx context.Context, repo string) (status, conclusion string, err error) {
	out, err := c.run(ctx, "api",
		fmt.Sprintf("/repos/%s/actions/runs?per_page=1", repo),
	)
	if err != nil {
		return "", "", err
	}
	var resp struct {
		Runs []apiRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", "", err
	}
	if len(resp.Runs) == 0 {
		return "", "", nil
	}
	r := resp.Runs[0]
	if r.Conclusion != nil {
		conclusion = *r.Conclusion
	}
	return r.Status, conclusion, nil
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
