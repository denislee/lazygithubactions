package gh

import (
	"context"

	"github.com/dns/lazygithubactions/internal/models"
)

func (c *Client) ListWorkflows(ctx context.Context, repo string) ([]models.Workflow, error) {
	var workflows []models.Workflow
	err := c.runJSON(ctx, &workflows, "workflow", "list",
		"-R", repo,
		"--json", "id,name,state,path",
	)
	return workflows, err
}

func (c *Client) TriggerWorkflow(ctx context.Context, repo string, workflowFile string, branch string) error {
	args := []string{"workflow", "run", workflowFile, "-R", repo}
	if branch != "" {
		args = append(args, "--ref", branch)
	}
	_, err := c.run(ctx, args...)
	return err
}
