package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dns/lazygithubactions/internal/models"
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

func (c *Client) GetLatestCommit(ctx context.Context, repo string) (*models.Commit, error) {
	// First get default branch
	repoOut, err := c.run(ctx, "api",
		fmt.Sprintf("/repos/%s", repo),
		"--jq", ".default_branch",
	)
	if err != nil {
		return nil, err
	}
	
	defaultBranch := string(repoOut)
	if len(defaultBranch) > 0 && defaultBranch[len(defaultBranch)-1] == '\n' {
		defaultBranch = defaultBranch[:len(defaultBranch)-1]
	}
	
	if defaultBranch == "" {
		defaultBranch = "HEAD"
	}

	// Fetch latest commit on default branch
	out, err := c.run(ctx, "api",
		fmt.Sprintf("/repos/%s/commits/%s", repo, defaultBranch),
	)
	if err != nil {
		return nil, err
	}

	var commitData struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := json.Unmarshal(out, &commitData); err != nil {
		return nil, err
	}

	date, _ := time.Parse(time.RFC3339, commitData.Commit.Author.Date)

	return &models.Commit{
		SHA:     commitData.SHA,
		Message: commitData.Commit.Message,
		Author: struct {
			Name  string    `json:"name"`
			Date  time.Time `json:"date"`
		}{
			Name: commitData.Commit.Author.Name,
			Date: date,
		},
	}, nil
}
