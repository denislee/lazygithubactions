package gh

import (
	"context"
	"encoding/json"

	"github.com/dns/lazygithubactions/internal/models"
)

type repoJSON struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (c *Client) ListRepos(ctx context.Context) ([]models.Repo, error) {
	out, err := c.run(ctx, "repo", "list", "--json", "name,owner", "--limit", "100")
	if err != nil {
		return nil, err
	}
	var raw []repoJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	repos := make([]models.Repo, len(raw))
	for i, r := range raw {
		repos[i] = models.Repo{
			Name:     r.Name,
			Owner:    r.Owner.Login,
			FullName: r.Owner.Login + "/" + r.Name,
		}
	}
	return repos, nil
}
