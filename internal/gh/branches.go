package gh

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) ListBranches(ctx context.Context, repo string) ([]string, error) {
	out, err := c.run(ctx, "api",
		fmt.Sprintf("/repos/%s/branches?per_page=100", repo),
	)
	if err != nil {
		return nil, err
	}

	var branches []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &branches); err != nil {
		return nil, err
	}

	// Fetch the default branch name
	defaultBranch := ""
	repoOut, err := c.run(ctx, "api",
		fmt.Sprintf("/repos/%s", repo),
		"--jq", ".default_branch",
	)
	if err == nil {
		defaultBranch = string(repoOut)
		// Trim trailing newline from jq output
		if len(defaultBranch) > 0 && defaultBranch[len(defaultBranch)-1] == '\n' {
			defaultBranch = defaultBranch[:len(defaultBranch)-1]
		}
	}

	// Put default branch first, then the rest
	names := make([]string, 0, len(branches))
	if defaultBranch != "" {
		names = append(names, defaultBranch)
	}
	for _, b := range branches {
		if b.Name != defaultBranch {
			names = append(names, b.Name)
		}
	}
	return names, nil
}
