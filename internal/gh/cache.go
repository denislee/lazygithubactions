package gh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dns/lazygithubactions/internal/models"
)

const cacheTTL = 5 * time.Minute

type repoCache struct {
	Repos     []models.Repo `json:"repos"`
	FetchedAt time.Time     `json:"fetchedAt"`
}

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "lazygithubactions")
}

func cachePath() string {
	return filepath.Join(cacheDir(), "repos.json")
}

func LoadCachedRepos() ([]models.Repo, error) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache repoCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, nil
	}
	if time.Since(cache.FetchedAt) > cacheTTL {
		return nil, nil
	}
	return cache.Repos, nil
}

func SaveCachedRepos(repos []models.Repo) error {
	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return err
	}
	cache := repoCache{
		Repos:     repos,
		FetchedAt: time.Now(),
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(), data, 0o644)
}
