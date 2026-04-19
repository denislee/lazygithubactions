package gh

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/dns/lazygithubactions/internal/models"
)

type repoJSON struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Archived bool   `json:"archived"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// ListRepos fetches all repos the authenticated user has access to,
// including personal, collaborator, and organization repos.
func (c *Client) ListRepos(ctx context.Context) ([]models.Repo, error) {
	out, err := c.run(ctx, "api",
		"/user/repos?per_page=100&sort=updated&affiliation=owner,collaborator,organization_member",
		"--paginate",
	)
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	// --paginate concatenates JSON arrays: [...][...] — merge into one
	raw = strings.ReplaceAll(raw, "][", ",")

	var repos []repoJSON
	if err := json.Unmarshal([]byte(raw), &repos); err != nil {
		return nil, err
	}

	result := make([]models.Repo, 0, len(repos))
	for _, r := range repos {
		if r.Archived {
			continue
		}
		result = append(result, models.Repo{
			Name:     r.Name,
			Owner:    r.Owner.Login,
			FullName: r.Owner.Login + "/" + r.Name,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].FullName) < strings.ToLower(result[j].FullName)
	})

	return result, nil
}

// GetCurrentRepo returns the repository info for the current directory if it's a GitHub repo.
func (c *Client) GetCurrentRepo(ctx context.Context) (*models.Repo, error) {
	var repo struct {
		Name          string `json:"name"`
		NameWithOwner string `json:"nameWithOwner"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}

	err := c.runJSON(ctx, &repo, "repo", "view", "--json", "owner,name,nameWithOwner")
	if err != nil {
		return nil, err
	}

	return &models.Repo{
		Name:     repo.Name,
		Owner:    repo.Owner.Login,
		FullName: repo.NameWithOwner,
	}, nil
}
